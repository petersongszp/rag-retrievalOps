package retrieval

import (
	"encoding/json"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestSearchMetricsFields(t *testing.T) {
	m := SearchMetrics{
		EmbeddingMs:         50,
		SearchMs:            120,
		PostprocessMs:       10,
		HitCount:            8,
		TruncatedCount:      1,
		DenseParticipation:  3,
		SparseParticipation: 2,
		PrimaryDenseCount:   2,
		PrimarySparseCount:  1,
		DualRouteFinalCount: 1,
	}

	if m.EmbeddingMs != 50 {
		t.Errorf("EmbeddingMs = %d, want 50", m.EmbeddingMs)
	}
	if m.SearchMs != 120 {
		t.Errorf("SearchMs = %d, want 120", m.SearchMs)
	}
	if m.PostprocessMs != 10 {
		t.Errorf("PostprocessMs = %d, want 10", m.PostprocessMs)
	}
	if m.HitCount != 8 {
		t.Errorf("HitCount = %d, want 8", m.HitCount)
	}
	if m.TruncatedCount != 1 {
		t.Errorf("TruncatedCount = %d, want 1", m.TruncatedCount)
	}
	if m.DenseParticipation != 3 || m.SparseParticipation != 2 {
		t.Errorf("unexpected participation counts: dense=%d sparse=%d", m.DenseParticipation, m.SparseParticipation)
	}
	if m.PrimaryDenseCount != 2 || m.PrimarySparseCount != 1 || m.DualRouteFinalCount != 1 {
		t.Errorf("unexpected primary/dual stats: %+v", m)
	}
}

func TestSearchResultFields(t *testing.T) {
	sr := SearchResult{
		Documents: nil,
		Metrics: SearchMetrics{
			EmbeddingMs:    30,
			SearchMs:       100,
			PostprocessMs:  5,
			HitCount:       0,
			TruncatedCount: 0,
		},
	}

	if sr.Documents != nil {
		t.Errorf("Documents should be nil for empty result")
	}
	if sr.Metrics.EmbeddingMs != 30 {
		t.Errorf("Metrics.EmbeddingMs = %d, want 30", sr.Metrics.EmbeddingMs)
	}
	if sr.Metrics.SearchMs != 100 {
		t.Errorf("Metrics.SearchMs = %d, want 100", sr.Metrics.SearchMs)
	}
	if sr.Metrics.PostprocessMs != 5 {
		t.Errorf("Metrics.PostprocessMs = %d, want 5", sr.Metrics.PostprocessMs)
	}
}

func TestSearchMetricsJSONSerialization(t *testing.T) {
	m := SearchMetrics{
		EmbeddingMs:    50,
		SearchMs:       120,
		PostprocessMs:  10,
		HitCount:       8,
		TruncatedCount: 1,
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Failed to marshal SearchMetrics: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal SearchMetrics: %v", err)
	}

	requiredFields := []string{"EmbeddingMs", "SearchMs", "PostprocessMs", "HitCount", "TruncatedCount"}
	for _, field := range requiredFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("SearchMetrics JSON missing field: %s", field)
		}
	}
	for _, field := range []string{"DenseParticipation", "SparseParticipation", "PrimaryDenseCount", "PrimarySparseCount", "DualRouteFinalCount"} {
		if _, ok := parsed[field]; !ok {
			t.Errorf("SearchMetrics JSON missing field: %s", field)
		}
	}
}

func TestSearchResultJSONSerialization(t *testing.T) {
	sr := SearchResult{
		Metrics: SearchMetrics{
			EmbeddingMs:   30,
			SearchMs:      100,
			PostprocessMs: 5,
			HitCount:      3,
		},
	}

	data, err := json.Marshal(sr)
	if err != nil {
		t.Fatalf("Failed to marshal SearchResult: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal SearchResult: %v", err)
	}

	if _, ok := parsed["Metrics"]; !ok {
		t.Error("SearchResult JSON missing field: Metrics")
	}
	if _, ok := parsed["Documents"]; !ok {
		t.Error("SearchResult JSON missing field: Documents")
	}
}

func TestSearchMetricsZeroValues(t *testing.T) {
	m := SearchMetrics{}

	if m.EmbeddingMs != 0 || m.SearchMs != 0 || m.PostprocessMs != 0 {
		t.Error("Zero SearchMetrics should have all duration fields as 0")
	}
	if m.HitCount != 0 || m.TruncatedCount != 0 {
		t.Error("Zero SearchMetrics should have all count fields as 0")
	}
}

func TestSearchResultWithDocuments(t *testing.T) {
	sr := SearchResult{
		Documents: nil,
		Metrics: SearchMetrics{
			HitCount: 0,
		},
	}

	if len(sr.Documents) != 0 {
		t.Errorf("Expected nil/empty documents, got %d", len(sr.Documents))
	}

	sr.Documents = make([]*schema.Document, 0)

	if len(sr.Documents) != 0 {
		t.Errorf("Expected empty documents slice, got %d", len(sr.Documents))
	}
}
