package kb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"interview-agents/api/response"
	myerrors "interview-agents/internal/errors"
	"interview-agents/internal/middleware"
	"interview-agents/internal/milvus/evaluation"
	"interview-agents/internal/model"
	kbservice "interview-agents/internal/service/kb"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var evalRunExecutor = kbservice.NewEvalRunExecutor()

type createEvalRunRequest struct {
	DatasetID        uint64                       `json:"dataset_id"`
	BaselineProfile  string                       `json:"baseline_profile"`
	CandidateProfile string                       `json:"candidate_profile"`
	Profiles         []evaluation.StrategyProfile `json:"profiles"`
	GateThresholds   evaluation.GateThresholds    `json:"gate_thresholds"`
}

type evalRunListResponse struct {
	Items    []*model.KBEvalRun `json:"items"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

func ListEvalRuns(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	page, pageSize := getPagination(c)
	filter := model.KBEvalRunListFilter{
		Page:     page,
		PageSize: pageSize,
	}

	if raw := strings.TrimSpace(string(c.Query("dataset_id"))); raw != "" {
		datasetID, err := parseUint64(raw, "dataset_id")
		if err != nil {
			response.BadRequest(ctx, c, err.Error())
			return
		}
		filter.DatasetID = &datasetID
	}

	if raw := strings.TrimSpace(string(c.Query("status"))); raw != "" {
		status, ok := model.ParseKBEvalRunStatus(raw)
		if !ok {
			response.BadRequest(ctx, c, "invalid run status")
			return
		}
		filter.Status = &status
	}

	items, total, err := model.KBEvalRunDao.ListWithFilter(filter)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to list evaluation runs", err))
		return
	}

	response.Success(ctx, c, evalRunListResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func CreateEvalRun(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	var req createEvalRunRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.BadRequest(ctx, c, "invalid request: "+err.Error())
		return
	}

	dataset, err := mustEvalDatasetExist(req.DatasetID)
	if err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}
	if dataset.Status != model.KBEvalDatasetStatusReady {
		response.BadRequest(ctx, c, "only ready datasets can start evaluation runs")
		return
	}

	datasetCases, err := model.KBEvalCaseDao.ListAllByDatasetID(req.DatasetID)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to load evaluation cases", err))
		return
	}
	if len(datasetCases) == 0 {
		response.BadRequest(ctx, c, "evaluation dataset is empty")
		return
	}

	profiles := req.Profiles
	if len(profiles) == 0 {
		profiles = evaluation.DefaultProfiles()
	}

	baselineProfile, err := resolveRunProfileName(profiles, strings.TrimSpace(req.BaselineProfile), true)
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}
	candidateProfile, err := resolveRunProfileName(profiles, strings.TrimSpace(req.CandidateProfile), false)
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}

	thresholds := req.GateThresholds
	if isZeroGateThresholds(thresholds) {
		thresholds = evaluation.DefaultGateThresholds()
	}

	run := &model.KBEvalRun{
		RunID:            uuid.NewString(),
		DatasetID:        req.DatasetID,
		BaselineProfile:  baselineProfile,
		CandidateProfile: candidateProfile,
		Profiles:         model.EvalStrategyProfileList(profiles),
		GateThresholds:   model.EvalGateThresholds(thresholds),
		Status:           model.KBEvalRunStatusPending,
		Progress:         0,
		CaseTotal:        len(datasetCases),
		CaseFinished:     0,
		CreatedBy:        middleware.GetUserID(c),
	}
	if err := model.KBEvalRunDao.Create(run); err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to create evaluation run", err))
		return
	}

	go executeEvalRun(run.RunID)
	response.Success(ctx, c, run)
}

func GetEvalRun(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	runID := strings.TrimSpace(c.Param("run_id"))
	if runID == "" {
		response.BadRequest(ctx, c, "run_id is required")
		return
	}

	run, err := model.KBEvalRunDao.GetByRunID(runID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(ctx, c, "evaluation run not found")
			return
		}
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to get evaluation run", err))
		return
	}

	response.Success(ctx, c, run)
}

func executeEvalRun(runID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	run, err := model.KBEvalRunDao.GetByRunID(runID)
	if err != nil {
		return
	}

	startedAt := time.Now()
	if err := model.KBEvalRunDao.UpdateByRunID(runID, map[string]interface{}{
		"status":        model.KBEvalRunStatusRunning,
		"progress":      1,
		"error_msg":     "",
		"started_at":    startedAt,
		"finished_at":   nil,
		"case_total":    run.CaseTotal,
		"case_finished": 0,
	}); err != nil {
		return
	}

	cases, err := model.KBEvalCaseDao.ListAllByDatasetID(run.DatasetID)
	if err != nil {
		failEvalRun(runID, fmt.Errorf("load dataset cases: %w", err))
		return
	}
	if len(cases) == 0 {
		failEvalRun(runID, fmt.Errorf("evaluation dataset is empty"))
		return
	}

	dataset := make([]evaluation.DatasetCase, 0, len(cases))
	for _, item := range cases {
		dataset = append(dataset, model.KBEvalCaseDao.ToDatasetCase(item))
	}

	report, err := evalRunExecutor.Execute(ctx, run, dataset)
	if err != nil {
		failEvalRun(runID, err)
		return
	}

	reportPath, err := persistEvalRunReport(runID, report)
	if err != nil {
		failEvalRun(runID, err)
		return
	}

	finishedAt := time.Now()
	_ = model.KBEvalRunDao.UpdateByRunID(runID, map[string]interface{}{
		"status":        model.KBEvalRunStatusSucceeded,
		"progress":      100,
		"case_finished": run.CaseTotal,
		"report_path":   reportPath,
		"error_msg":     "",
		"finished_at":   finishedAt,
	})
}

func failEvalRun(runID string, err error) {
	finishedAt := time.Now()
	_ = model.KBEvalRunDao.UpdateByRunID(runID, map[string]interface{}{
		"status":      model.KBEvalRunStatusFailed,
		"progress":    100,
		"error_msg":   err.Error(),
		"finished_at": finishedAt,
	})
}

func resolveRunProfileName(profiles []evaluation.StrategyProfile, explicit string, baseline bool) (string, error) {
	explicit = strings.TrimSpace(explicit)
	if explicit != "" {
		for _, profile := range profiles {
			if strings.EqualFold(profile.Name, explicit) {
				return profile.Name, nil
			}
		}
		return "", fmt.Errorf("profile %q not found", explicit)
	}

	for _, profile := range profiles {
		if baseline && profile.Baseline {
			return profile.Name, nil
		}
		if !baseline && profile.Candidate {
			return profile.Name, nil
		}
	}

	if len(profiles) == 0 {
		return "", fmt.Errorf("profiles are required")
	}
	if baseline {
		return profiles[0].Name, nil
	}
	return profiles[len(profiles)-1].Name, nil
}

func isZeroGateThresholds(thresholds evaluation.GateThresholds) bool {
	return thresholds == (evaluation.GateThresholds{})
}

func persistEvalRunReport(runID string, report *evaluation.Report) (string, error) {
	backendRoot, err := resolveBackendRoot()
	if err != nil {
		return "", err
	}
	reportDir := filepath.Join(backendRoot, "runtime", "eval-reports")
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		return "", err
	}
	reportPath := filepath.Join(reportDir, runID+".json")
	if err := evaluation.SaveReportJSON(reportPath, report); err != nil {
		return "", err
	}
	return reportPath, nil
}

func resolveBackendRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	current := wd
	for {
		if _, statErr := os.Stat(filepath.Join(current, "go.mod")); statErr == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("backend go.mod not found from %s", wd)
		}
		current = parent
	}
}
