package evaluation

import (
	"context"
	"testing"
)

type fakeSearcher struct {
	results map[string]SearchOutcome
}

func (f *fakeSearcher) Search(_ context.Context, item DatasetCase) (SearchOutcome, error) {
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
			results: map[string]SearchOutcome{
				"mvc":               {Items: []RetrievedItem{{ResultID: "noise-1"}}},
				"redis persistence": {Items: []RetrievedItem{{ResultID: "chunk-redis", Citation: CitationTarget{ChunkID: "chunk-redis"}}}},
			},
		},
		"hybrid": {
			results: map[string]SearchOutcome{
				"mvc":               {Items: []RetrievedItem{{ResultID: "chunk-mvcc", Citation: CitationTarget{ChunkID: "chunk-mvcc"}}}, RewriteApplied: true},
				"redis persistence": {Items: []RetrievedItem{{ResultID: "chunk-redis", Citation: CitationTarget{ChunkID: "chunk-redis"}}}},
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

func TestRunnerFlagsRefusalFalsePositiveGate(t *testing.T) {
	dataset := []DatasetCase{
		{
			ID:               "scope",
			Query:            "totally unrelated topic",
			TopK:             3,
			ExpectedBehavior: "answer normally",
		},
	}
	profiles := []StrategyProfile{
		{Name: "baseline", Baseline: true, Mode: "dense"},
		{Name: "candidate", Candidate: true, Mode: "hybrid", EnableModelAssistedRewrite: true},
	}
	searchers := map[string]*fakeSearcher{
		"baseline": {
			results: map[string]SearchOutcome{
				"totally unrelated topic": {Items: []RetrievedItem{{ResultID: "chunk-1"}}},
			},
		},
		"candidate": {
			results: map[string]SearchOutcome{
				"totally unrelated topic": {Refused: true, RefusalReason: "No-Retrieval-Hit", RewriteApplied: true},
			},
		},
	}

	runner := &Runner{
		Factory: func(profile StrategyProfile) (Searcher, error) {
			return searchers[profile.Name], nil
		},
	}

	report, err := runner.Run(context.Background(), dataset, profiles, GateThresholds{
		MaxP95LatencyRegressionRatio: 1,
		MaxRefusalFalsePositiveRate:  0,
	}, "", "")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if report.Gate.Passed {
		t.Fatalf("expected gate to fail on refusal false positive")
	}
	if report.Comparison.RefusalFalsePositiveRate <= 0 {
		t.Fatalf("expected refusal false positive rate > 0")
	}
}
