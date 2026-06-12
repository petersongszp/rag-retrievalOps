package evaluation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadGateThresholdsMergesDefaults(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "gates.json")
	payload := `{"min_recall_delta":0.12}`
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	thresholds, err := LoadGateThresholds(path)
	if err != nil {
		t.Fatalf("LoadGateThresholds failed: %v", err)
	}
	if thresholds.MinRecallDelta != 0.12 {
		t.Fatalf("MinRecallDelta = %.2f, want 0.12", thresholds.MinRecallDelta)
	}
	if thresholds.MaxP95LatencyRegressionRatio != DefaultGateThresholds().MaxP95LatencyRegressionRatio {
		t.Fatalf("expected default MaxP95LatencyRegressionRatio to be preserved, got %.2f", thresholds.MaxP95LatencyRegressionRatio)
	}
}

func TestRenderMarkdownReportIncludesChunkingSections(t *testing.T) {
	report := &Report{
		DatasetSize:    1,
		DatasetVersion: "chunking-v1",
		ProfileVersion: "profiles-v1",
		GeneratedAt:    time.Unix(0, 0),
		Baseline:       "baseline_recursive",
		Candidate:      "semantic_resplit",
		Results: []StrategyResult{
			{
				Strategy: StrategyProfile{Name: "baseline_recursive"},
				Metrics: AggregateMetrics{
					RecallAtK:              0.5,
					ChunkPurity:            0.8,
					ChunkSelfContainedRate: 0.7,
					ParentFillGain:         0.1,
					ContextualRecallGain:   0.2,
					AvgEmbeddingTextLength: 123,
					P95EmbeddingTextLength: 180,
					AvgChunksPerDocument:   4,
					P95ChunksPerDocument:   6,
					IngestP95MS:            25,
				},
			},
		},
		Comparison: ComparisonSummary{
			ContextualRecallGainDelta: 0.1,
			ChunkPurityDelta:          0.05,
			ChunkSelfContainedDelta:   0.04,
			IngestP95DeltaMS:          3,
		},
		Gate: GateResult{Passed: true},
	}

	rendered := RenderMarkdownReport(report)
	for _, needle := range []string{
		"## Chunking Stats",
		"Chunk Purity",
		"Contextual Recall Gain Delta",
		"Avg Embedding Text Length Delta",
	} {
		if !strings.Contains(rendered, needle) {
			t.Fatalf("rendered report missing %q", needle)
		}
	}
}
