package kb

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"interview-agents/api/response"
	"interview-agents/internal/model"
	"interview-agents/internal/rag/governance"
	"interview-agents/internal/repository"

	"github.com/cloudwego/hertz/pkg/app"
)

type governanceAlertResponse struct {
	ID             string     `json:"id"`
	Title          string     `json:"title"`
	Category       string     `json:"category"`
	Severity       string     `json:"severity"`
	Status         string     `json:"status"`
	KBID           *uint64    `json:"kb_id,omitempty"`
	TargetType     string     `json:"target_type,omitempty"`
	TargetID       string     `json:"target_id,omitempty"`
	Summary        string     `json:"summary,omitempty"`
	MetricKey      string     `json:"metric_key,omitempty"`
	MetricValue    *float64   `json:"metric_value,omitempty"`
	Threshold      *float64   `json:"threshold,omitempty"`
	RequestID      string     `json:"request_id,omitempty"`
	TraceID        string     `json:"trace_id,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	ContractGaps   []string   `json:"contract_gaps,omitempty"`
}

type governanceAlertListResponse struct {
	Items    []governanceAlertResponse `json:"items"`
	Total    int                       `json:"total"`
	Page     int                       `json:"page"`
	PageSize int                       `json:"page_size"`
}

type alertMutationRequest struct {
	Reason string `json:"reason"`
}

type alertStateRecord struct {
	Status         string
	AcknowledgedAt *time.Time
	ResolvedAt     *time.Time
}

var (
	alertStateMu sync.Mutex
	alertStates  = map[string]alertStateRecord{}
)

func ListAlerts(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	statusFilter := strings.TrimSpace(string(c.Query("status")))
	severityFilter := strings.TrimSpace(string(c.Query("severity")))
	categoryFilter := strings.TrimSpace(string(c.Query("category")))
	page, pageSize := getPagination(c)

	alerts := deriveGovernanceAlerts()
	filtered := make([]governanceAlertResponse, 0, len(alerts))
	for _, item := range alerts {
		if statusFilter != "" && !strings.EqualFold(item.Status, statusFilter) {
			continue
		}
		if severityFilter != "" && !strings.EqualFold(item.Severity, severityFilter) {
			continue
		}
		if categoryFilter != "" && !strings.EqualFold(item.Category, categoryFilter) {
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

	response.Success(ctx, c, governanceAlertListResponse{
		Items:    filtered[start:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func AckAlert(ctx context.Context, c *app.RequestContext) {
	mutateAlertState(ctx, c, "acknowledged", governance.ActionAlertAck)
}

func ResolveAlert(ctx context.Context, c *app.RequestContext) {
	mutateAlertState(ctx, c, "resolved", governance.ActionAlertResolve)
}

func mutateAlertState(ctx context.Context, c *app.RequestContext, nextStatus string, action string) {
	if !requireAdmin(ctx, c) {
		return
	}

	alertID := strings.TrimSpace(c.Param("alert_id"))
	if alertID == "" {
		response.BadRequest(ctx, c, "alert_id is required")
		return
	}
	var req alertMutationRequest
	_ = c.BindAndValidate(&req)

	now := time.Now().UTC()
	alertStateMu.Lock()
	current := alertStates[alertID]
	current.Status = nextStatus
	if nextStatus == "acknowledged" {
		current.AcknowledgedAt = &now
	} else if nextStatus == "resolved" {
		current.ResolvedAt = &now
		if current.AcknowledgedAt == nil {
			current.AcknowledgedAt = &now
		}
	}
	alertStates[alertID] = current
	alertStateMu.Unlock()

	persistAuditEvent(&model.KBAuditEvent{
		AuditTraceID: fmt.Sprintf("audit-alert-%s-%d", alertID, now.Unix()),
		Action:       action,
		ResourceType: "alert",
		ResourceID:   alertID,
		Reason:       strings.TrimSpace(req.Reason),
		Result:       nextStatus,
		CreatedAt:    now,
	})

	response.Success(ctx, c, map[string]interface{}{
		"alert_id": alertID,
		"status":   nextStatus,
	})
}

func deriveGovernanceAlerts() []governanceAlertResponse {
	now := time.Now().UTC()
	alerts := make([]governanceAlertResponse, 0, 8)

	if safeToComputeGovernanceGate() {
		gate := newGateRecorder().compute(context.Background(), &app.RequestContext{})
		if !gate.CostGuardPassed {
			alerts = append(alerts, newDerivedAlert("cost-budget", "成本预算超阈值", "cost", "high", now, "cost_per_1k_queries", gate.CostPer1KQueries, 25, "governance_gate", "cost"))
		}
		if !gate.AuditGuardPassed {
			alerts = append(alerts, newDerivedAlert("audit-coverage", "审计覆盖率不足", "audit", "high", now, "audit_coverage_rate", gate.AuditCoverageRate, 0.6, "governance_gate", "audit"))
		}
		if !gate.IndexGuardPassed {
			alerts = append(alerts, newDerivedAlert("capacity-index-health", "Collection 健康分过低", "capacity", "medium", now, "collection_health_score", gate.CollectionHealthScore, 0.5, "governance_gate", "collection"))
		}
	}

	for _, task := range governance.SnapshotCompensationTasks() {
		createdAt := task.CreatedAt
		alerts = append(alerts, governanceAlertResponse{
			ID:           task.ID,
			Title:        "治理补偿任务待处理",
			Category:     "audit",
			Severity:     "medium",
			Status:       resolveAlertStatus(task.ID),
			TargetType:   task.Scope,
			TargetID:     task.TraceID,
			Summary:      task.Reason,
			RequestID:    task.RequestID,
			TraceID:      task.TraceID,
			CreatedAt:    createdAt,
			ContractGaps: []string{"kb_id"},
		})
	}

	for i := range alerts {
		applyAlertState(&alerts[i])
	}
	return alerts
}

func safeToComputeGovernanceGate() bool {
	return repository.GetDB() != nil
}

func newDerivedAlert(id, title, category, severity string, createdAt time.Time, metricKey string, metricValue float64, threshold float64, targetType string, targetID string) governanceAlertResponse {
	return governanceAlertResponse{
		ID:          id,
		Title:       title,
		Category:    category,
		Severity:    severity,
		Status:      resolveAlertStatus(id),
		TargetType:  targetType,
		TargetID:    targetID,
		MetricKey:   metricKey,
		MetricValue: float64Ptr(metricValue),
		Threshold:   float64Ptr(threshold),
		CreatedAt:   createdAt,
	}
}

func resolveAlertStatus(id string) string {
	alertStateMu.Lock()
	defer alertStateMu.Unlock()

	if state, ok := alertStates[id]; ok && strings.TrimSpace(state.Status) != "" {
		return state.Status
	}
	return "open"
}

func applyAlertState(item *governanceAlertResponse) {
	if item == nil {
		return
	}
	alertStateMu.Lock()
	defer alertStateMu.Unlock()

	state, ok := alertStates[item.ID]
	if !ok {
		return
	}
	if strings.TrimSpace(state.Status) != "" {
		item.Status = state.Status
	}
	item.AcknowledgedAt = state.AcknowledgedAt
	item.ResolvedAt = state.ResolvedAt
}
