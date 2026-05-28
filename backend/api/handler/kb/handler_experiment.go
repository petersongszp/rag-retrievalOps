package kb

import (
	"context"
	"strings"
	"time"

	"interview-agents/api/response"
	"interview-agents/internal/config"
	myerrors "interview-agents/internal/errors"
	"interview-agents/internal/model"
	"interview-agents/internal/rag/experiment"
	"interview-agents/internal/rag/governance"

	"github.com/cloudwego/hertz/pkg/app"
)

type saveExperimentRequest struct {
	ExperimentID      string   `json:"experiment_id"`
	ExperimentName    string   `json:"experiment_name"`
	StrategyType      string   `json:"strategy_type"`
	BaselineVersion   string   `json:"baseline_version"`
	CandidateVersion  string   `json:"candidate_version"`
	TrafficRatio      int      `json:"traffic_ratio"`
	TargetKBIDs       []uint64 `json:"target_kb_ids"`
	TargetQueryTypes  []string `json:"target_query_types"`
	TargetEnvironment string   `json:"target_environment"`
	ShadowMode        bool     `json:"shadow_mode"`
	StartTime         string   `json:"start_time"`
	EndTime           string   `json:"end_time"`
	Owner             string   `json:"owner"`
	Status            string   `json:"status"`
}

type rollbackExperimentRequest struct {
	Reason string `json:"reason"`
}

type experimentListResponse struct {
	Items []experiment.ConfigRecord `json:"items"`
}

type experimentSummaryResponse struct {
	Items []experiment.Summary `json:"items"`
}

func ListExperiments(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}
	response.Success(ctx, c, experimentListResponse{Items: experiment.List()})
}

func SaveExperiment(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	var req saveExperimentRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.BadRequest(ctx, c, "invalid request: "+err.Error())
		return
	}

	startTime, err := parseOptionalExperimentTime(req.StartTime)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewValidationError(err.Error()))
		return
	}
	endTime, err := parseOptionalExperimentTime(req.EndTime)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewValidationError(err.Error()))
		return
	}

	record, err := experiment.Save(experiment.ConfigRecord{
		ExperimentID:      strings.TrimSpace(req.ExperimentID),
		ExperimentName:    strings.TrimSpace(req.ExperimentName),
		StrategyType:      strings.TrimSpace(req.StrategyType),
		BaselineVersion:   strings.TrimSpace(req.BaselineVersion),
		CandidateVersion:  strings.TrimSpace(req.CandidateVersion),
		TrafficRatio:      req.TrafficRatio,
		TargetKBIDs:       append([]uint64(nil), req.TargetKBIDs...),
		TargetQueryTypes:  append([]string(nil), req.TargetQueryTypes...),
		TargetEnvironment: strings.TrimSpace(req.TargetEnvironment),
		ShadowMode:        req.ShadowMode,
		StartTime:         startTime,
		EndTime:           endTime,
		Owner:             strings.TrimSpace(req.Owner),
		Status:            strings.TrimSpace(req.Status),
	})
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewValidationError(err.Error()))
		return
	}
	persistAuditEvent(&model.KBAuditEvent{
		AuditTraceID: "audit-experiment-" + record.ExperimentID,
		Action:       governance.ActionExperimentUpdate,
		ResourceType: "experiment",
		ResourceID:   record.ExperimentID,
		AfterData:    record.CandidateVersion,
		Result:       record.Status,
		Reason:       strings.TrimSpace(req.Owner),
	})
	response.Success(ctx, c, record)
}

func RollbackExperiment(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}
	experimentID := strings.TrimSpace(c.Param("experiment_id"))
	if experimentID == "" {
		response.BadRequest(ctx, c, "experiment_id is required")
		return
	}
	var req rollbackExperimentRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.BadRequest(ctx, c, "invalid request: "+err.Error())
		return
	}
	record, err := experiment.Rollback(ctx, &config.Global, experimentID, strings.TrimSpace(req.Reason))
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewValidationError(err.Error()))
		return
	}
	persistAuditEvent(&model.KBAuditEvent{
		AuditTraceID: "audit-experiment-rollback-" + record.ExperimentID,
		Action:       governance.ActionExperimentRollback,
		ResourceType: "experiment",
		ResourceID:   record.ExperimentID,
		AfterData:    record.Status,
		Result:       record.Status,
		Reason:       strings.TrimSpace(req.Reason),
	})
	response.Success(ctx, c, record)
}

func GetExperimentSummaries(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}
	logs, _, err := model.KBRetrieveLogDao.ListWithFilter(model.KBRetrieveLogListFilter{
		Page:     1,
		PageSize: 500,
	})
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to load retrieve logs", err))
		return
	}
	items := make([]experiment.RetrieveLogLike, 0, len(logs))
	for _, item := range logs {
		if item != nil {
			items = append(items, retrieveLogAdapter{item})
		}
	}
	response.Success(ctx, c, experimentSummaryResponse{
		Items: experiment.BuildSummary(items),
	})
}

type retrieveLogAdapter struct {
	entry *model.KBRetrieveLog
}

func (a retrieveLogAdapter) GetExperimentID() string {
	if a.entry == nil {
		return ""
	}
	return a.entry.ExperimentID
}

func (a retrieveLogAdapter) GetExperimentGroup() string {
	if a.entry == nil {
		return ""
	}
	return a.entry.ExperimentGroup
}

func (a retrieveLogAdapter) GetDurationMS() int64 {
	if a.entry == nil {
		return 0
	}
	return a.entry.DurationMs
}

func (a retrieveLogAdapter) GetContextTokens() int {
	if a.entry == nil {
		return 0
	}
	return a.entry.ContextTokens
}

func (a retrieveLogAdapter) GetResultStatus() string {
	if a.entry == nil {
		return ""
	}
	return string(a.entry.ResultStatus)
}

func parseOptionalExperimentTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, myerrors.NewValidationError("time must use RFC3339")
	}
	return value.UTC(), nil
}
