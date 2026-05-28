package governance

import "testing"

func TestFeatureFlagKeys(t *testing.T) {
	keys := FeatureFlagKeys()
	if len(keys) != 7 {
		t.Fatalf("FeatureFlagKeys len = %d, want 7", len(keys))
	}
	if !IsFeatureFlag(FlagComplianceAudit) {
		t.Fatal("expected FlagComplianceAudit to be managed")
	}
	if IsFeatureFlag("RAG_UNKNOWN_PHASE4_FLAG") {
		t.Fatal("unexpected unknown phase4 flag")
	}
}

func TestMetricKeys(t *testing.T) {
	keys := MetricKeys()
	if len(keys) != 7 {
		t.Fatalf("MetricKeys len = %d, want 7", len(keys))
	}
	if keys[0] != MetricQualityScore {
		t.Fatalf("MetricKeys[0] = %q, want %q", keys[0], MetricQualityScore)
	}
}

func TestCompensationQueue(t *testing.T) {
	ResetCompensationTasks()

	task := EnqueueCompensation("audit", "trace-1", "req-1", "persist_failed", map[string]interface{}{
		"request_id": "req-1",
	})
	if task.ID == "" {
		t.Fatal("expected compensation task id")
	}

	items := SnapshotCompensationTasks()
	if len(items) != 1 {
		t.Fatalf("SnapshotCompensationTasks len = %d, want 1", len(items))
	}
	if items[0].TraceID != "trace-1" || items[0].RequestID != "req-1" {
		t.Fatalf("unexpected compensation task: %+v", items[0])
	}

	ResetCompensationTasks()
}
