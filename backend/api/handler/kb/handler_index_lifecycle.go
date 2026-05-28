package kb

import (
	"context"
	"strings"
	"time"

	"interview-agents/api/response"
	"interview-agents/internal/config"
	myerrors "interview-agents/internal/errors"
	"interview-agents/internal/middleware"
	"interview-agents/internal/milvus/benchmark"
	"interview-agents/internal/model"
	"interview-agents/internal/rag/governance"
	"interview-agents/internal/rag/indexlifecycle"

	"github.com/cloudwego/hertz/pkg/app"
)

type buildCandidateIndexRequest struct {
	IndexVersion string `json:"index_version"`
	ProfileName  string `json:"profile_name"`
	Reason       string `json:"reason"`
}

type indexRegistryListResponse struct {
	Items []*model.KBIndexRegistry `json:"items"`
}

type indexOperationListResponse struct {
	Items []*model.KBIndexOperationLog `json:"items"`
}

func ListIndexRegistry(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}
	items, err := indexlifecycle.ListRegistry()
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to list index registry", err))
		return
	}
	response.Success(ctx, c, indexRegistryListResponse{Items: items})
}

func RegisterIndexFromConfig(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}
	record, err := indexlifecycle.RegisterFromConfig(&config.Global, "admin")
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewValidationError(err.Error()))
		return
	}
	persistAuditEvent(&model.KBAuditEvent{
		AuditTraceID: "audit-index-register-" + record.IndexVersion,
		Action:       governance.ActionCollectionRebuild,
		ResourceType: "index_registry",
		ResourceID:   record.IndexVersion,
		AfterData:    record.CollectionName,
		Result:       string(record.BuildStatus),
		Reason:       "register_from_config",
	})
	response.Success(ctx, c, record)
}

func BuildCandidateIndex(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}
	var req buildCandidateIndexRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.BadRequest(ctx, c, "invalid request: "+err.Error())
		return
	}
	profile, ok := benchmarkProfileByName(strings.TrimSpace(req.ProfileName))
	if !ok {
		response.ErrorFromErr(ctx, c, myerrors.NewValidationError("unknown profile_name"))
		return
	}
	indexVersion := strings.TrimSpace(req.IndexVersion)
	if indexVersion == "" {
		indexVersion = "idx-" + time.Now().UTC().Format("20060102-150405")
	}
	record, err := indexlifecycle.BuildCandidate(ctx, &config.Global, indexVersion, profile, middleware.GetUserID(c), strings.TrimSpace(req.Reason))
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewValidationError(err.Error()))
		return
	}
	persistAuditEvent(&model.KBAuditEvent{
		AuditTraceID: "audit-index-build-" + record.IndexVersion,
		Action:       governance.ActionCollectionRebuild,
		ResourceType: "index_registry",
		ResourceID:   record.IndexVersion,
		AfterData:    record.CollectionName,
		Result:       string(record.BuildStatus),
		Reason:       strings.TrimSpace(req.Reason),
	})
	response.Success(ctx, c, record)
}

func GetIndexHealth(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}
	indexVersion := strings.TrimSpace(c.Param("index_version"))
	if indexVersion == "" {
		response.BadRequest(ctx, c, "index_version is required")
		return
	}
	report, err := indexlifecycle.HealthCheck(ctx, &config.Global, indexVersion)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewValidationError(err.Error()))
		return
	}
	response.Success(ctx, c, report)
}

func SwitchActiveIndex(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}
	indexVersion := strings.TrimSpace(c.Param("index_version"))
	if indexVersion == "" {
		response.BadRequest(ctx, c, "index_version is required")
		return
	}
	result, err := indexlifecycle.SwitchActive(ctx, &config.Global, indexVersion, middleware.GetUserID(c), getOperationReason(c))
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewValidationError(err.Error()))
		return
	}
	persistAuditEvent(&model.KBAuditEvent{
		AuditTraceID: "audit-index-switch-" + result.ActiveIndexVersion,
		Action:       governance.ActionCollectionSwitch,
		ResourceType: "index_registry",
		ResourceID:   result.ActiveIndexVersion,
		AfterData:    result.PreviousIndexVersion,
		Result:       "switched",
		Reason:       getOperationReason(c),
	})
	response.Success(ctx, c, result)
}

func RollbackActiveIndex(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}
	result, err := indexlifecycle.RollbackActive(ctx, &config.Global, middleware.GetUserID(c), getOperationReason(c))
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewValidationError(err.Error()))
		return
	}
	persistAuditEvent(&model.KBAuditEvent{
		AuditTraceID: "audit-index-rollback-" + result.ActiveIndexVersion,
		Action:       governance.ActionCollectionRollback,
		ResourceType: "index_registry",
		ResourceID:   result.ActiveIndexVersion,
		AfterData:    result.PreviousIndexVersion,
		Result:       "rolled_back",
		Reason:       getOperationReason(c),
	})
	response.Success(ctx, c, result)
}

func ListIndexOperations(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}
	items, err := indexlifecycle.ListOperations(100)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to list index operations", err))
		return
	}
	response.Success(ctx, c, indexOperationListResponse{Items: items})
}

func benchmarkProfileByName(name string) (benchmark.IndexProfile, bool) {
	for _, profile := range benchmark.DefaultProfiles() {
		if strings.EqualFold(strings.TrimSpace(profile.Name), name) {
			return profile, true
		}
	}
	return benchmark.IndexProfile{}, false
}
