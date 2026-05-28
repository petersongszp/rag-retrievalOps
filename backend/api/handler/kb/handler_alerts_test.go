package kb

import (
	"testing"
	"time"

	"interview-agents/internal/rag/governance"
)

func TestApplyAlertState(t *testing.T) {
	alertStateMu.Lock()
	alertStates = map[string]alertStateRecord{}
	alertStateMu.Unlock()

	now := time.Now().UTC()
	alertStateMu.Lock()
	alertStates["alert-1"] = alertStateRecord{
		Status:         "acknowledged",
		AcknowledgedAt: &now,
	}
	alertStateMu.Unlock()

	item := &governanceAlertResponse{ID: "alert-1", Status: "open"}
	applyAlertState(item)
	if item.Status != "acknowledged" || item.AcknowledgedAt == nil {
		t.Fatalf("unexpected alert state: %#v", item)
	}
}

func TestNewDerivedAlert(t *testing.T) {
	now := time.Now().UTC()
	item := newDerivedAlert("cost-1", "成本异常", "cost", "high", now, "cost_per_1k_queries", 12.3, 10, "gate", "cost")
	if item.ID != "cost-1" || item.MetricValue == nil || *item.MetricValue != 12.3 {
		t.Fatalf("unexpected derived alert: %#v", item)
	}
}

func TestDerivedGovernanceAlertsIncludeCompensationTasks(t *testing.T) {
	governance.ResetCompensationTasks()
	governance.EnqueueCompensation("audit_event", "trace-1", "req-1", "persist_failed", nil)
	alerts := deriveGovernanceAlerts()
	found := false
	for _, item := range alerts {
		if item.ID == "governance-comp-1" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("alerts = %#v, want compensation-derived alert", alerts)
	}
	governance.ResetCompensationTasks()
}
