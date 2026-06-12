package retrieval

import (
	"encoding/json"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestSearchMetricsFields(t *testing.T) {
	m := SearchMetrics{
		EmbeddingMs:            50,
		SearchMs:               120,
		PostprocessMs:          10,
		HitCount:               8,
		TruncatedCount:         1,
		EmbeddingCacheEnabled:  true,
		EmbeddingCacheHit:      true,
		EmbeddingCacheLookupMs: 3,
		EmbeddingCacheReason:   "hit",
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
	if !m.EmbeddingCacheEnabled || !m.EmbeddingCacheHit {
		t.Errorf("unexpected embedding cache flags: enabled=%v hit=%v", m.EmbeddingCacheEnabled, m.EmbeddingCacheHit)
	}
	if m.EmbeddingCacheLookupMs != 3 || m.EmbeddingCacheReason != "hit" {
		t.Errorf("unexpected embedding cache metrics: %+v", m)
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
		EmbeddingMs:            50,
		SearchMs:               120,
		PostprocessMs:          10,
		HitCount:               8,
		TruncatedCount:         1,
		EmbeddingCacheEnabled:  true,
		EmbeddingCacheHit:      true,
		EmbeddingCacheLookupMs: 3,
		EmbeddingCacheReason:   "hit",
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Failed to marshal SearchMetrics: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal SearchMetrics: %v", err)
	}

	requiredFields := []string{"EmbeddingMs", "SearchMs", "PostprocessMs", "HitCount", "TruncatedCount", "EmbeddingCacheEnabled", "EmbeddingCacheHit", "EmbeddingCacheLookupMs", "EmbeddingCacheReason"}
	for _, field := range requiredFields {
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
