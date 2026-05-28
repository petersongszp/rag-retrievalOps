package governance

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

const (
	FlagCostGovernance   = "RAG_ENABLE_COST_GOVERNANCE"
	FlagAuditCenter      = "RAG_ENABLE_AUDIT_CENTER"
	FlagVectorOps        = "RAG_ENABLE_VECTOR_OPS"
	FlagGovernanceAlerts = "RAG_ENABLE_GOVERNANCE_ALERTS"
	FlagWeeklyReport     = "RAG_ENABLE_WEEKLY_REPORT"

	FlagExperimentPlatform    = "RAG_ENABLE_EXPERIMENT_PLATFORM"
	FlagIndexLifecycle        = "RAG_ENABLE_INDEX_LIFECYCLE"
	FlagCostDashboard         = "RAG_ENABLE_COST_DASHBOARD"
	FlagComplianceAudit       = "RAG_ENABLE_COMPLIANCE_AUDIT"
	FlagMilvusOpsTooling      = "RAG_ENABLE_MILVUS_OPS_TOOLING"
	FlagCollectionSwitchGuard = "RAG_ENABLE_COLLECTION_SWITCH_GUARD"
)

const (
	MetricQualityScore       = "quality_score"
	MetricCostPer1KQueries   = "cost_per_1k_queries"
	MetricAvgContextTokens   = "avg_context_tokens"
	MetricStrategyRegression = "strategy_regression_rate"
	MetricRollbackSuccess    = "rollback_success_rate"
	MetricAuditCoverage      = "audit_coverage_rate"
	MetricCollectionHealth   = "collection_health_score"
)

const (
	ActionKBCreate           = "kb_create"
	ActionDocumentUpload     = "document_upload"
	ActionDocumentDelete     = "document_delete"
	ActionIngestRetry        = "ingest_retry"
	ActionIngestCancel       = "ingest_cancel"
	ActionRetrieveQuery      = "retrieve_query"
	ActionTraceView          = "trace_view"
	ActionEvalRunCreate      = "eval_run_create"
	ActionReportExport       = "report_export"
	ActionStrategyFlagUpdate = "strategy_flag_update"
	ActionStrategyRollback   = "strategy_rollback"
	ActionExperimentUpdate   = "experiment_update"
	ActionExperimentRollback = "experiment_rollback"
	ActionCollectionRebuild  = "collection_rebuild"
	ActionCollectionSwitch   = "collection_switch"
	ActionCollectionRollback = "collection_rollback"
	ActionAlertAck           = "alert_ack"
	ActionAlertResolve       = "alert_resolve"
	ActionPermissionChange   = "permission_change"
)

const (
	CostAPIPrefix       = "/api/admin/kb/cost"
	VectorAPIPrefix     = "/api/admin/kb/vector"
	AuditAPIPrefix      = "/api/admin/kb/audit"
	AlertsAPIPrefix     = "/api/admin/kb/alerts"
	ReportsAPIPrefix    = "/api/admin/kb/reports"
	GovernanceAPIPrefix = "/api/admin/kb/governance"
)

var phase4FeatureFlags = []string{
	FlagCostGovernance,
	FlagAuditCenter,
	FlagVectorOps,
	FlagGovernanceAlerts,
	FlagWeeklyReport,
}

var phase4MetricKeys = []string{
	MetricQualityScore,
	MetricCostPer1KQueries,
	MetricAvgContextTokens,
	MetricStrategyRegression,
	MetricRollbackSuccess,
	MetricAuditCoverage,
	MetricCollectionHealth,
}

type TraceContext struct {
	ExperimentID      string `json:"experiment_id"`
	StrategyVersion   string `json:"strategy_version"`
	IndexVersion      string `json:"index_version"`
	CollectionVersion string `json:"collection_version"`
	CostTraceID       string `json:"cost_trace_id"`
	AuditTraceID      string `json:"audit_trace_id"`
	ReleaseID         string `json:"release_id"`
}

type CompensationTask struct {
	ID        string                 `json:"id"`
	Scope     string                 `json:"scope"`
	TraceID   string                 `json:"trace_id"`
	RequestID string                 `json:"request_id"`
	Reason    string                 `json:"reason"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

var (
	compensationMu      sync.Mutex
	compensationCounter int
	compensationTasks   []CompensationTask
)

func FeatureFlagKeys() []string {
	return append([]string(nil), phase4FeatureFlags...)
}

func MetricKeys() []string {
	return append([]string(nil), phase4MetricKeys...)
}

func IsFeatureFlag(flagKey string) bool {
	for _, candidate := range phase4FeatureFlags {
		if candidate == flagKey {
			return true
		}
	}
	return false
}

func EnqueueCompensation(scope, traceID, requestID, reason string, payload map[string]interface{}) CompensationTask {
	compensationMu.Lock()
	defer compensationMu.Unlock()

	compensationCounter++
	task := CompensationTask{
		ID:        fmt.Sprintf("governance-comp-%d", compensationCounter),
		Scope:     scope,
		TraceID:   traceID,
		RequestID: requestID,
		Reason:    reason,
		Payload:   clonePayload(payload),
		CreatedAt: time.Now().UTC(),
	}
	compensationTasks = append([]CompensationTask{task}, compensationTasks...)
	if len(compensationTasks) > 256 {
		compensationTasks = compensationTasks[:256]
	}
	return task
}

func SnapshotCompensationTasks() []CompensationTask {
	compensationMu.Lock()
	defer compensationMu.Unlock()

	items := make([]CompensationTask, 0, len(compensationTasks))
	for _, item := range compensationTasks {
		item.Payload = clonePayload(item.Payload)
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return items
}

func ResetCompensationTasks() {
	compensationMu.Lock()
	defer compensationMu.Unlock()

	compensationCounter = 0
	compensationTasks = nil
}

func clonePayload(payload map[string]interface{}) map[string]interface{} {
	if len(payload) == 0 {
		return nil
	}
	cloned := make(map[string]interface{}, len(payload))
	for key, value := range payload {
		cloned[key] = value
	}
	return cloned
}
