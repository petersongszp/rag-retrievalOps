package kb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"interview-agents/api/response"
	"interview-agents/internal/config"
	myerrors "interview-agents/internal/errors"
	"interview-agents/internal/middleware"
	"interview-agents/internal/model"
	"interview-agents/internal/rag/governance"
	"interview-agents/internal/rag/indexlifecycle"

	"github.com/cloudwego/hertz/pkg/app"
)

type vectorCollectionListItemResponse struct {
	CollectionName     string     `json:"collection_name"`
	KBID               *uint64    `json:"kb_id,omitempty"`
	Active             bool       `json:"active"`
	Status             string     `json:"status"`
	HealthStatus       string     `json:"health_status"`
	EntityCount        *int64     `json:"entity_count,omitempty"`
	CapacityBytes      *int64     `json:"capacity_bytes,omitempty"`
	IndexStatus        string     `json:"index_status"`
	IndexVersion       string     `json:"index_version"`
	SchemaVersion      string     `json:"schema_version,omitempty"`
	LastRebuildAt      *time.Time `json:"last_rebuild_at,omitempty"`
	LastSwitchAt       *time.Time `json:"last_switch_at,omitempty"`
	RollbackCollection string     `json:"rollback_collection,omitempty"`
	ContractGaps       []string   `json:"contract_gaps,omitempty"`
}

type vectorCollectionListResponse struct {
	Items []vectorCollectionListItemResponse `json:"items"`
}

type vectorCollectionHealthResponse struct {
	CollectionName      string    `json:"collection_name"`
	IndexVersion        string    `json:"index_version"`
	LoadState           string    `json:"load_state"`
	IndexBuildProgress  int       `json:"index_build_progress"`
	QueryLatencyP95Ms   *int64    `json:"query_latency_p95_ms,omitempty"`
	InsertLatencyP95Ms  *int64    `json:"insert_latency_p95_ms,omitempty"`
	SegmentCount        *int      `json:"segment_count,omitempty"`
	SealedSegmentCount  *int      `json:"sealed_segment_count,omitempty"`
	GrowingSegmentCount *int      `json:"growing_segment_count,omitempty"`
	LastError           string    `json:"last_error,omitempty"`
	CollectionExists    bool      `json:"collection_exists"`
	DimensionMatch      bool      `json:"dimension_match"`
	MetricTypeMatch     bool      `json:"metric_type_match"`
	QuerySmokeHealthy   bool      `json:"query_smoke_healthy"`
	CheckedAt           time.Time `json:"checked_at"`
	ContractGaps        []string  `json:"contract_gaps,omitempty"`
}

type vectorCollectionCapacityResponse struct {
	CollectionName string   `json:"collection_name"`
	CapacityBytes  *int64   `json:"capacity_bytes,omitempty"`
	EntityCount    *int64   `json:"entity_count,omitempty"`
	ContractGaps   []string `json:"contract_gaps,omitempty"`
}

type rebuildVectorCollectionRequest struct {
	TargetIndexVersion string `json:"target_index_version"`
	Reason             string `json:"reason"`
	DryRun             bool   `json:"dry_run"`
}

type switchVectorCollectionRequest struct {
	Reason string `json:"reason"`
}

type rollbackVectorCollectionRequest struct {
	Reason string `json:"reason"`
}

type vectorOperationListResponse struct {
	Items    []*model.KBIndexOperationLog `json:"items"`
	Total    int                          `json:"total"`
	Page     int                          `json:"page"`
	PageSize int                          `json:"page_size"`
}

func ListVectorCollections(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	statusFilter := strings.TrimSpace(string(c.Query("status")))
	collectionNameFilter := strings.TrimSpace(string(c.Query("collection_name")))

	registry, err := indexlifecycle.ListRegistry()
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to list vector collections", err))
		return
	}

	operations, err := indexlifecycle.ListOperations(500)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to list vector operations", err))
		return
	}
	rollbackByCollection := buildRollbackCollectionMap(registry)
	lastSwitchAt, lastRebuildAt := buildVectorOperationTimestamps(operations)

	items := make([]vectorCollectionListItemResponse, 0, len(registry))
	for _, item := range registry {
		if item == nil {
			continue
		}
		if collectionNameFilter != "" && !strings.EqualFold(strings.TrimSpace(item.CollectionName), collectionNameFilter) {
			continue
		}
		status := string(item.BuildStatus)
		if statusFilter != "" && !strings.EqualFold(status, statusFilter) {
			continue
		}
		healthStatus := deriveVectorHealthStatus(item.BuildStatus)
		respItem := vectorCollectionListItemResponse{
			CollectionName:     item.CollectionName,
			Active:             item.CollectionRole == model.CollectionRoleActive,
			Status:             status,
			HealthStatus:       healthStatus,
			IndexStatus:        status,
			IndexVersion:       item.IndexVersion,
			SchemaVersion:      item.MetricType,
			LastRebuildAt:      lastRebuildAt[item.IndexVersion],
			LastSwitchAt:       lastSwitchAt[item.IndexVersion],
			RollbackCollection: rollbackByCollection[item.CollectionName],
			ContractGaps:       buildVectorListContractGaps(item, registry),
		}
		items = append(items, respItem)
	}

	response.Success(ctx, c, vectorCollectionListResponse{Items: items})
}

func GetVectorCollectionHealth(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	record, err := findVectorRegistryByCollectionName(strings.TrimSpace(c.Param("name")))
	if err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}

	report, err := indexlifecycle.HealthCheck(ctx, &config.Global, record.IndexVersion)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewValidationError(err.Error()))
		return
	}

	loadState := "not_loaded"
	if report.LoadHealthy {
		loadState = "loaded"
	}
	progress := 0
	if report.CollectionExists && report.DimensionMatch && report.MetricTypeMatch {
		progress = 100
	} else if report.CollectionExists {
		progress = 50
	}

	resp := vectorCollectionHealthResponse{
		CollectionName:     record.CollectionName,
		IndexVersion:       record.IndexVersion,
		LoadState:          loadState,
		IndexBuildProgress: progress,
		CollectionExists:   report.CollectionExists,
		DimensionMatch:     report.DimensionMatch,
		MetricTypeMatch:    report.MetricTypeMatch,
		QuerySmokeHealthy:  report.QuerySmokeHealthy,
		CheckedAt:          report.CheckedAt,
		LastError:          report.Message,
		ContractGaps:       []string{"query_latency_p95_ms", "insert_latency_p95_ms", "segment_count", "sealed_segment_count", "growing_segment_count"},
	}
	response.Success(ctx, c, resp)
}

func GetVectorCollectionCapacity(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	record, err := findVectorRegistryByCollectionName(strings.TrimSpace(c.Param("name")))
	if err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}
	resp := vectorCollectionCapacityResponse{
		CollectionName: record.CollectionName,
		ContractGaps:   []string{"capacity_bytes", "entity_count"},
	}
	response.Success(ctx, c, resp)
}

func RebuildVectorCollection(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	collectionName := strings.TrimSpace(c.Param("name"))
	record, err := findVectorRegistryByCollectionName(collectionName)
	if err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}

	var req rebuildVectorCollectionRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.BadRequest(ctx, c, "invalid request: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Reason) == "" {
		response.BadRequest(ctx, c, "reason is required")
		return
	}

	targetVersion := strings.TrimSpace(req.TargetIndexVersion)
	if targetVersion == "" {
		targetVersion = fmt.Sprintf("%s-%d", record.IndexVersion, time.Now().UTC().Unix())
	}
	if req.DryRun {
		response.Success(ctx, c, map[string]interface{}{
			"collection_name":      record.CollectionName,
			"target_index_version": targetVersion,
			"reason":               strings.TrimSpace(req.Reason),
			"dry_run":              true,
			"contract_gaps":        []string{"data_source_validation", "document_count_validation"},
		})
		return
	}

	buildReq := buildCandidateIndexRequest{
		IndexVersion: targetVersion,
		ProfileName:  "phase2-hnsw-balanced",
		Reason:       strings.TrimSpace(req.Reason),
	}
	result, err := buildCandidateIndex(ctx, c, buildReq)
	if err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}
	persistAuditEvent(&model.KBAuditEvent{
		AuditTraceID: "audit-vector-rebuild-" + targetVersion,
		OperatorID:   middleware.GetUserID(c),
		UserID:       middleware.GetUserID(c),
		Action:       governance.ActionCollectionRebuild,
		ResourceType: "collection",
		ResourceID:   collectionName,
		AfterData:    targetVersion,
		Result:       string(result.BuildStatus),
		Reason:       strings.TrimSpace(req.Reason),
	})
	response.Success(ctx, c, result)
}

func SwitchVectorCollection(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	record, err := findVectorRegistryByCollectionName(strings.TrimSpace(c.Param("name")))
	if err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}

	var req switchVectorCollectionRequest
	_ = c.BindAndValidate(&req)
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = getOperationReason(c)
	}
	if reason == "" {
		response.BadRequest(ctx, c, "reason is required")
		return
	}

	health, err := indexlifecycle.HealthCheck(ctx, &config.Global, record.IndexVersion)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewValidationError(err.Error()))
		return
	}
	if !health.CollectionExists || !health.DimensionMatch || !health.MetricTypeMatch || !health.QuerySmokeHealthy {
		response.ErrorFromErr(ctx, c, myerrors.NewValidationError("target collection health check did not pass"))
		return
	}

	result, err := indexlifecycle.SwitchActive(ctx, &config.Global, record.IndexVersion, middleware.GetUserID(c), reason)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewValidationError(err.Error()))
		return
	}
	persistAuditEvent(&model.KBAuditEvent{
		AuditTraceID: "audit-vector-switch-" + record.IndexVersion,
		OperatorID:   middleware.GetUserID(c),
		UserID:       middleware.GetUserID(c),
		Action:       governance.ActionCollectionSwitch,
		ResourceType: "collection",
		ResourceID:   record.CollectionName,
		BeforeData:   result.PreviousIndexVersion,
		AfterData:    result.ActiveIndexVersion,
		Result:       "switched",
		Reason:       reason,
	})
	response.Success(ctx, c, result)
}

func RollbackVectorCollection(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	record, err := findVectorRegistryByCollectionName(strings.TrimSpace(c.Param("name")))
	if err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}

	var req rollbackVectorCollectionRequest
	_ = c.BindAndValidate(&req)
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = getOperationReason(c)
	}
	if reason == "" {
		response.BadRequest(ctx, c, "reason is required")
		return
	}

	result, err := indexlifecycle.RollbackActive(ctx, &config.Global, middleware.GetUserID(c), reason)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewValidationError(err.Error()))
		return
	}
	persistAuditEvent(&model.KBAuditEvent{
		AuditTraceID: "audit-vector-rollback-" + record.IndexVersion,
		OperatorID:   middleware.GetUserID(c),
		UserID:       middleware.GetUserID(c),
		Action:       governance.ActionCollectionRollback,
		ResourceType: "collection",
		ResourceID:   record.CollectionName,
		BeforeData:   result.PreviousIndexVersion,
		AfterData:    result.ActiveIndexVersion,
		Result:       "rolled_back",
		Reason:       reason,
	})
	response.Success(ctx, c, result)
}

func ListVectorOperations(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	page, pageSize := getPagination(c)
	collectionName := strings.TrimSpace(string(c.Query("collection_name")))

	items, err := indexlifecycle.ListOperations(500)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to list vector operations", err))
		return
	}

	filtered := make([]*model.KBIndexOperationLog, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		if collectionName != "" && !strings.EqualFold(strings.TrimSpace(item.CollectionName), collectionName) {
			continue
		}
		filtered = append(filtered, item)
	}

	total := len(filtered)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	response.Success(ctx, c, vectorOperationListResponse{
		Items:    filtered[start:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func buildCandidateIndex(ctx context.Context, c *app.RequestContext, req buildCandidateIndexRequest) (*model.KBIndexRegistry, error) {
	profile, ok := benchmarkProfileByName(strings.TrimSpace(req.ProfileName))
	if !ok {
		if fallback, found := benchmarkProfileByName("balanced"); found {
			profile = fallback
		} else {
			return nil, myerrors.NewValidationError("unknown profile_name")
		}
	}
	indexVersion := strings.TrimSpace(req.IndexVersion)
	if indexVersion == "" {
		indexVersion = "idx-" + time.Now().UTC().Format("20060102-150405")
	}
	record, err := indexlifecycle.BuildCandidate(ctx, &config.Global, indexVersion, profile, middleware.GetUserID(c), strings.TrimSpace(req.Reason))
	if err != nil {
		return nil, myerrors.NewValidationError(err.Error())
	}
	return record, nil
}

func findVectorRegistryByCollectionName(name string) (*model.KBIndexRegistry, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, myerrors.NewValidationError("collection name is required")
	}
	items, err := indexlifecycle.ListRegistry()
	if err != nil {
		return nil, myerrors.NewDBError("failed to list vector collections", err)
	}
	var match *model.KBIndexRegistry
	for _, item := range items {
		if item == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.IndexVersion), name) {
			return item, nil
		}
		if strings.EqualFold(strings.TrimSpace(item.CollectionName), name) {
			if match != nil {
				return nil, myerrors.NewValidationError("collection name is not unique, use index_version instead")
			}
			match = item
		}
	}
	if match != nil {
		return match, nil
	}
	return nil, myerrors.NewValidationError("collection not found")
}

func buildRollbackCollectionMap(items []*model.KBIndexRegistry) map[string]string {
	result := make(map[string]string)
	for _, item := range items {
		if item == nil {
			continue
		}
		if item.CollectionRole == model.CollectionRoleRollback {
			result[item.CollectionName] = item.IndexVersion
		}
	}
	return result
}

func buildVectorOperationTimestamps(items []*model.KBIndexOperationLog) (map[string]*time.Time, map[string]*time.Time) {
	lastSwitch := make(map[string]*time.Time)
	lastRebuild := make(map[string]*time.Time)
	for _, item := range items {
		if item == nil {
			continue
		}
		ts := item.CreatedAt
		switch strings.TrimSpace(item.Operation) {
		case "switch_active":
			if lastSwitch[item.IndexVersion] == nil || lastSwitch[item.IndexVersion].Before(ts) {
				copy := ts
				lastSwitch[item.IndexVersion] = &copy
			}
		case "build_candidate", "register":
			if lastRebuild[item.IndexVersion] == nil || lastRebuild[item.IndexVersion].Before(ts) {
				copy := ts
				lastRebuild[item.IndexVersion] = &copy
			}
		}
	}
	return lastSwitch, lastRebuild
}

func deriveVectorHealthStatus(status model.IndexBuildStatus) string {
	switch status {
	case model.IndexBuildStatusReady, model.IndexBuildStatusSwitched:
		return "healthy"
	case model.IndexBuildStatusBuilding, model.IndexBuildStatusPending:
		return "degraded"
	case model.IndexBuildStatusFailed:
		return "unhealthy"
	default:
		return "unknown"
	}
}

func buildVectorListContractGaps(item *model.KBIndexRegistry, registry []*model.KBIndexRegistry) []string {
	gaps := []string{"kb_id", "entity_count", "capacity_bytes"}
	if item == nil {
		return gaps
	}
	duplicateCollections := 0
	for _, candidate := range registry {
		if candidate != nil && strings.EqualFold(strings.TrimSpace(candidate.CollectionName), strings.TrimSpace(item.CollectionName)) {
			duplicateCollections++
		}
	}
	if duplicateCollections > 1 {
		gaps = append(gaps, "collection_name_not_unique_use_index_version")
	}
	return gaps
}
