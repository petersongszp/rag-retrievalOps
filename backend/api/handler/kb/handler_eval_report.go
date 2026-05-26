package kb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"interview-agents/api/response"
	myerrors "interview-agents/internal/errors"
	"interview-agents/internal/milvus/evaluation"
	"interview-agents/internal/model"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"gorm.io/gorm"
)

type evalFailureMetrics struct {
	RecallAtK        float64 `json:"recall_at_k"`
	MRR              float64 `json:"mrr"`
	NDCG             float64 `json:"ndcg"`
	CitationAccuracy float64 `json:"citation_accuracy"`
	LatencyMS        int64   `json:"latency_ms"`
}

type evalFailureDelta struct {
	RecallDelta           float64 `json:"recall_delta"`
	MRRDelta              float64 `json:"mrr_delta"`
	NDCGDelta             float64 `json:"ndcg_delta"`
	CitationAccuracyDelta float64 `json:"citation_accuracy_delta"`
	LatencyDeltaMS        int64   `json:"latency_delta_ms"`
}

type evalFailureCase struct {
	CaseID             string             `json:"case_id"`
	Query              string             `json:"query"`
	QueryType          string             `json:"query_type,omitempty"`
	Tags               []string           `json:"tags,omitempty"`
	FailureReason      string             `json:"failure_reason"`
	BaselineMetrics    evalFailureMetrics `json:"baseline_metrics"`
	CandidateMetrics   evalFailureMetrics `json:"candidate_metrics"`
	Delta              evalFailureDelta   `json:"delta"`
	BaselineRequestID  string             `json:"baseline_request_id,omitempty"`
	CandidateRequestID string             `json:"candidate_request_id,omitempty"`
}

type evalFailureCaseListResponse struct {
	Items    []evalFailureCase `json:"items"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

func GetEvalReport(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	run, err := mustSucceededEvalRun(strings.TrimSpace(c.Param("run_id")))
	if err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}

	report, err := loadEvalReport(run.ReportPath)
	if err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}

	response.Success(ctx, c, report)
}

func ListEvalFailureCases(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	run, err := mustSucceededEvalRun(strings.TrimSpace(c.Param("run_id")))
	if err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}

	report, err := loadEvalReport(run.ReportPath)
	if err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}

	baselineResult, candidateResult, err := resolveBaselineCandidateResults(report)
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}

	page, pageSize := getPagination(c)
	failureReasonFilter := strings.TrimSpace(string(c.Query("failure_reason")))
	queryTypeFilter := strings.TrimSpace(string(c.Query("query_type")))
	tagFilter := strings.TrimSpace(string(c.Query("tag")))

	baselineTraceMap := buildTraceAvailabilityMap(run.BaselineProfile, baselineResult.Queries)
	candidateTraceMap := buildTraceAvailabilityMap(run.CandidateProfile, candidateResult.Queries)

	items := make([]evalFailureCase, 0)
	baselineByID := make(map[string]evaluation.QueryMetrics, len(baselineResult.Queries))
	for _, query := range baselineResult.Queries {
		baselineByID[query.QueryID] = query
	}

	for _, candidateQuery := range candidateResult.Queries {
		baselineQuery, ok := baselineByID[candidateQuery.QueryID]
		if !ok {
			continue
		}

		failureReason := classifyFailureReason(report, baselineQuery, candidateQuery)
		if failureReason == "" {
			continue
		}
		if failureReasonFilter != "" && failureReason != failureReasonFilter {
			continue
		}
		if queryTypeFilter != "" && !strings.EqualFold(candidateQuery.QueryType, queryTypeFilter) {
			continue
		}
		if tagFilter != "" && !stringSliceContains(candidateQuery.Tags, tagFilter) {
			continue
		}

		item := evalFailureCase{
			CaseID:        candidateQuery.QueryID,
			Query:         candidateQuery.Query,
			QueryType:     candidateQuery.QueryType,
			Tags:          append([]string(nil), candidateQuery.Tags...),
			FailureReason: failureReason,
			BaselineMetrics: evalFailureMetrics{
				RecallAtK:        baselineQuery.RecallAtK,
				MRR:              baselineQuery.MRR,
				NDCG:             baselineQuery.NDCG,
				CitationAccuracy: baselineQuery.CitationAccuracy,
				LatencyMS:        baselineQuery.Latency.Milliseconds(),
			},
			CandidateMetrics: evalFailureMetrics{
				RecallAtK:        candidateQuery.RecallAtK,
				MRR:              candidateQuery.MRR,
				NDCG:             candidateQuery.NDCG,
				CitationAccuracy: candidateQuery.CitationAccuracy,
				LatencyMS:        candidateQuery.Latency.Milliseconds(),
			},
			Delta: evalFailureDelta{
				RecallDelta:           candidateQuery.RecallAtK - baselineQuery.RecallAtK,
				MRRDelta:              candidateQuery.MRR - baselineQuery.MRR,
				NDCGDelta:             candidateQuery.NDCG - baselineQuery.NDCG,
				CitationAccuracyDelta: candidateQuery.CitationAccuracy - baselineQuery.CitationAccuracy,
				LatencyDeltaMS:        candidateQuery.Latency.Milliseconds() - baselineQuery.Latency.Milliseconds(),
			},
		}
		if baselineTraceMap[item.CaseID] {
			item.BaselineRequestID = buildEvalRequestID(run.BaselineProfile, item.CaseID)
		}
		if candidateTraceMap[item.CaseID] {
			item.CandidateRequestID = buildEvalRequestID(run.CandidateProfile, item.CaseID)
		}
		items = append(items, item)
	}

	total := int64(len(items))
	start := (page - 1) * pageSize
	if start > len(items) {
		start = len(items)
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}

	response.Success(ctx, c, evalFailureCaseListResponse{
		Items:    items[start:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func ExportEvalReport(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	run, err := mustSucceededEvalRun(strings.TrimSpace(c.Param("run_id")))
	if err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}

	report, err := loadEvalReport(run.ReportPath)
	if err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}

	format := strings.TrimSpace(string(c.Query("format")))
	switch format {
	case "", "json":
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.json\"", run.RunID))
		c.JSON(consts.StatusOK, report)
	case "markdown":
		c.Header("Content-Type", "text/markdown; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.md\"", run.RunID))
		c.String(consts.StatusOK, evaluation.RenderMarkdownReport(report))
	default:
		response.BadRequest(ctx, c, "format must be json or markdown")
	}
}

func mustSucceededEvalRun(runID string) (*model.KBEvalRun, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, myerrors.NewInvalidParamError("run_id is required")
	}

	run, err := model.KBEvalRunDao.GetByRunID(runID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, myerrors.NewNotFoundError("evaluation run")
		}
		return nil, myerrors.NewDBError("failed to get evaluation run", err)
	}
	if run.Status != model.KBEvalRunStatusSucceeded {
		return nil, myerrors.NewInvalidParamError("evaluation report is not ready")
	}
	if strings.TrimSpace(run.ReportPath) == "" {
		return nil, myerrors.NewNotFoundError("evaluation report")
	}
	return run, nil
}

func loadEvalReport(reportPath string) (*evaluation.Report, error) {
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return nil, myerrors.NewInternalError("failed to read evaluation report", err)
	}

	var report evaluation.Report
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, myerrors.NewInternalError("failed to parse evaluation report", err)
	}
	return &report, nil
}

func resolveBaselineCandidateResults(report *evaluation.Report) (*evaluation.StrategyResult, *evaluation.StrategyResult, error) {
	var baselineResult *evaluation.StrategyResult
	var candidateResult *evaluation.StrategyResult
	for i := range report.Results {
		if report.Results[i].Strategy.Name == report.Baseline {
			baselineResult = &report.Results[i]
		}
		if report.Results[i].Strategy.Name == report.Candidate {
			candidateResult = &report.Results[i]
		}
	}
	if baselineResult == nil || candidateResult == nil {
		return nil, nil, fmt.Errorf("baseline or candidate strategy results are missing")
	}
	return baselineResult, candidateResult, nil
}

func classifyFailureReason(report *evaluation.Report, baseline evaluation.QueryMetrics, candidate evaluation.QueryMetrics) string {
	switch {
	case candidate.RecallAtK < baseline.RecallAtK:
		return "recall_miss"
	case candidate.CitationAccuracy < baseline.CitationAccuracy:
		return "citation_miss"
	case candidate.MRR < baseline.MRR:
		return "mrr_drop"
	case candidate.NDCG < baseline.NDCG:
		return "ndcg_drop"
	case candidate.Latency > baseline.Latency:
		return "latency_regression"
	case report.Gate.Passed == false:
		return "gate_failed"
	default:
		return ""
	}
}

func buildEvalRequestID(profileName string, caseID string) string {
	return fmt.Sprintf("eval-%s-%s", profileName, caseID)
}

func buildTraceAvailabilityMap(profileName string, queries []evaluation.QueryMetrics) map[string]bool {
	requestIDs := make([]string, 0, len(queries))
	caseToRequestID := make(map[string]string, len(queries))
	for _, query := range queries {
		requestID := buildEvalRequestID(profileName, query.QueryID)
		requestIDs = append(requestIDs, requestID)
		caseToRequestID[query.QueryID] = requestID
	}

	available := make(map[string]bool, len(queries))
	if len(requestIDs) == 0 {
		return available
	}

	existing := findExistingRetrieveLogs(requestIDs)
	for caseID, requestID := range caseToRequestID {
		available[caseID] = existing[requestID]
	}
	return available
}

func findExistingRetrieveLogs(requestIDs []string) map[string]bool {
	result := make(map[string]bool, len(requestIDs))
	if len(requestIDs) == 0 {
		return result
	}

	db := model.GetDB()
	if db == nil {
		return result
	}

	var rows []struct {
		RequestID string `gorm:"column:request_id"`
	}
	if err := db.Model(&model.KBRetrieveLog{}).Where("request_id IN ?", requestIDs).Find(&rows).Error; err != nil {
		return result
	}
	for _, row := range rows {
		result[row.RequestID] = true
	}
	return result
}

func stringSliceContains(items []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), target) {
			return true
		}
	}
	return false
}
