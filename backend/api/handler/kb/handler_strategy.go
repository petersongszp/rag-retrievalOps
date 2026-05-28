package kb

import (
	"context"
	"strings"

	"interview-agents/api/response"
	"interview-agents/internal/config"
	myerrors "interview-agents/internal/errors"
	"interview-agents/internal/middleware"
	"interview-agents/internal/model"
	"interview-agents/internal/rag/phase3"
	"interview-agents/internal/rag/phase3admin"

	"github.com/cloudwego/hertz/pkg/app"
)

type strategyFlagListResponse struct {
	Items []phase3admin.FlagState `json:"items"`
}

type updateStrategyFlagRequest struct {
	Enabled           bool   `json:"enabled"`
	Status            string `json:"status"`
	RolloutPercentage int    `json:"rollout_percentage"`
	Reason            string `json:"reason"`
}

type strategyVersionListResponse struct {
	Items    []phase3admin.VersionRecord `json:"items"`
	Total    int                         `json:"total"`
	Page     int                         `json:"page"`
	PageSize int                         `json:"page_size"`
}

type rollbackStrategyRequest struct {
	TargetVersion string   `json:"target_version"`
	FlagKeys      []string `json:"flag_keys"`
	Reason        string   `json:"reason"`
}

func ListStrategyFlags(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	response.Success(ctx, c, strategyFlagListResponse{
		Items: phase3admin.ListFlags(&config.Global),
	})
}

func UpdateStrategyFlag(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	flagKey := strings.TrimSpace(c.Param("flag_key"))
	if flagKey == "" {
		response.BadRequest(ctx, c, "flag_key is required")
		return
	}

	var req updateStrategyFlagRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.BadRequest(ctx, c, "invalid request: "+err.Error())
		return
	}

	state, err := phase3admin.UpdateFlag(
		ctx,
		&config.Global,
		flagKey,
		req.Enabled,
		strings.TrimSpace(req.Status),
		req.RolloutPercentage,
		strings.TrimSpace(req.Reason),
		middleware.GetUserID(c),
	)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewValidationError(err.Error()))
		return
	}
	persistAuditEvent(&model.KBAuditEvent{
		AuditTraceID: "audit-strategy-" + flagKey,
		OperatorID:   middleware.GetUserID(c),
		UserID:       middleware.GetUserID(c),
		Action:       "StrategyChanged",
		ResourceType: "strategy_flag",
		ResourceID:   flagKey,
		AfterData:    state.StrategyVersion,
		Result:       state.Status,
		Reason:       strings.TrimSpace(req.Reason),
	})

	response.Success(ctx, c, state)
}

func ListStrategyVersions(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	page, pageSize := getPagination(c)
	flagKey := strings.TrimSpace(string(c.Query("flag_key")))
	if flagKey != "" && !phase3.IsManagedFeatureFlag(flagKey) {
		response.ErrorFromErr(ctx, c, myerrors.NewValidationError("unsupported flag_key: "+flagKey))
		return
	}

	items := phase3admin.ListVersions(&config.Global, flagKey)
	total := len(items)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	response.Success(ctx, c, strategyVersionListResponse{
		Items:    items[start:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func GetStrategyVersion(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	versionID := strings.TrimSpace(c.Param("version_id"))
	if versionID == "" {
		response.BadRequest(ctx, c, "version_id is required")
		return
	}

	record, ok := phase3admin.GetVersion(&config.Global, versionID)
	if !ok {
		response.NotFound(ctx, c, "strategy version not found")
		return
	}

	response.Success(ctx, c, record)
}

func RollbackStrategy(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	var req rollbackStrategyRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.BadRequest(ctx, c, "invalid request: "+err.Error())
		return
	}

	targetVersion := strings.TrimSpace(req.TargetVersion)
	if targetVersion == "" {
		targetVersion = phase3.StrategyTargetPhase2Baseline
	}

	result, err := phase3admin.Rollback(
		ctx,
		&config.Global,
		targetVersion,
		req.FlagKeys,
		strings.TrimSpace(req.Reason),
		middleware.GetUserID(c),
	)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewValidationError(err.Error()))
		return
	}
	persistAuditEvent(&model.KBAuditEvent{
		AuditTraceID: "audit-strategy-rollback-" + targetVersion,
		OperatorID:   middleware.GetUserID(c),
		UserID:       middleware.GetUserID(c),
		Action:       "StrategyChanged",
		ResourceType: "strategy_flag",
		ResourceID:   targetVersion,
		AfterData:    result.Status,
		Result:       result.Status,
		Reason:       strings.TrimSpace(req.Reason),
	})

	response.Success(ctx, c, result)
}
