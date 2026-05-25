package evaluation

import (
	"context"
	"testing"
)

type fakeSearcher struct {
	results map[string][]RetrievedItem
}

func (f *fakeSearcher) Search(_ context.Context, item DatasetCase) ([]RetrievedItem, error) {
	return f.results[item.Query], nil
}

func TestRunnerBuildsComparisonAndGate(t *testing.T) {
	dataset := []DatasetCase{
		{
			ID:          "abbr",
			Query:       "mvc",
			TopK:        3,
			RelevantIDs: []string{"chunk-mvcc"},
			CitationTargets: []CitationTarget{
				{ChunkID: "chunk-mvcc"},
			},
		},
		{
			ID:          "entity",
			Query:       "redis persistence",
			TopK:        3,
			RelevantIDs: []string{"chunk-redis"},
		},
	}
	profiles := []StrategyProfile{
		{Name: "dense_only", Baseline: true, Mode: "dense"},
		{Name: "hybrid", Candidate: true, Mode: "hybrid"},
	}
	searchers := map[string]*fakeSearcher{
		"dense_only": {
			results: map[string][]RetrievedItem{
				"mvc":               {{ResultID: "noise-1"}},
				"redis persistence": {{ResultID: "chunk-redis", Citation: CitationTarget{ChunkID: "chunk-redis"}}},
			},
		},
		"hybrid": {
			results: map[string][]RetrievedItem{
				"mvc":               {{ResultID: "chunk-mvcc", Citation: CitationTarget{ChunkID: "chunk-mvcc"}}},
				"redis persistence": {{ResultID: "chunk-redis", Citation: CitationTarget{ChunkID: "chunk-redis"}}},
			},
		},
	}

	runner := &Runner{
		Factory: func(profile StrategyProfile) (Searcher, error) {
			return searchers[profile.Name], nil
		},
	}

	report, err := runner.Run(context.Background(), dataset, profiles, GateThresholds{MinRecallDelta: 0.4, MaxP95LatencyRegressionRatio: 1}, "", "")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if report.Baseline != "dense_only" {
		t.Fatalf("baseline = %s, want dense_only", report.Baseline)
	}
	if report.Candidate != "hybrid" {
		t.Fatalf("candidate = %s, want hybrid", report.Candidate)
	}
	if report.Comparison.RecallDelta <= 0 {
		t.Fatalf("expected positive recall delta, got %.4f", report.Comparison.RecallDelta)
	}
	if !report.Gate.Passed {
		t.Fatalf("expected gate to pass")
	}
}
