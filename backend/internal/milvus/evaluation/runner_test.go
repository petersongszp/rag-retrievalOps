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

func TestRunnerAggregatesNonRRFRouteMetrics(t *testing.T) {
	dataset := []DatasetCase{
		{
			ID:                          "dense-case",
			Query:                       "dense dominant query",
			TopK:                        4,
			ExpectedPrimaryRoute:        "dense",
			ExpectedParticipatingRoutes: []string{"dense", "sparse"},
		},
		{
			ID:                          "sparse-case",
			Query:                       "sparse dominant query",
			TopK:                        4,
			ExpectedPrimaryRoute:        "sparse",
			ExpectedParticipatingRoutes: []string{"sparse"},
		},
	}
	profiles := []StrategyProfile{
		{Name: "baseline", Baseline: true, Mode: "hybrid"},
		{Name: "candidate", Candidate: true, Mode: "hybrid"},
	}
	searchers := map[string]*fakeSearcher{
		"baseline": {
			results: map[string]SearchOutcome{
				"dense dominant query": {
					Items:               []RetrievedItem{{ResultID: "doc-1"}},
					DenseHits:           2,
					SparseHits:          1,
					DenseParticipation:  1,
					SparseParticipation: 1,
					PrimaryDenseCount:   1,
					DualRouteFinalCount: 1,
				},
				"sparse dominant query": {
					Items:                 []RetrievedItem{{ResultID: "doc-2"}},
					SparseHits:            2,
					SparseParticipation:   1,
					PrimarySparseCount:    1,
					SparseCandidateBefore: 3,
					SparseCandidateAfter:  2,
				},
			},
		},
		"candidate": {
			results: map[string]SearchOutcome{
				"dense dominant query": {
					Items:               []RetrievedItem{{ResultID: "doc-1"}},
					DenseHits:           2,
					SparseHits:          2,
					DenseParticipation:  1,
					SparseParticipation: 1,
					PrimaryDenseCount:   1,
					DualRouteFinalCount: 1,
				},
				"sparse dominant query": {
					Items:                 []RetrievedItem{{ResultID: "doc-2"}},
					SparseHits:            3,
					SparseParticipation:   1,
					PrimarySparseCount:    1,
					SparseCandidateBefore: 5,
					SparseCandidateAfter:  3,
				},
			},
		},
	}

	runner := &Runner{
		Factory: func(profile StrategyProfile) (Searcher, error) {
			return searchers[profile.Name], nil
		},
	}

	report, err := runner.Run(context.Background(), dataset, profiles, GateThresholds{MaxP95LatencyRegressionRatio: 1}, "", "")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if report.Results[0].Queries[0].PrimaryRoute != "dense" {
		t.Fatalf("primary route = %q, want dense", report.Results[0].Queries[0].PrimaryRoute)
	}
	if report.Results[0].Queries[1].PrimaryRoute != "sparse" {
		t.Fatalf("primary route = %q, want sparse", report.Results[0].Queries[1].PrimaryRoute)
	}
	if report.Results[0].Metrics.DenseHitRate <= 0 {
		t.Fatalf("expected dense hit rate > 0")
	}
	if report.Results[0].Metrics.SparseParticipationRate <= 0 {
		t.Fatalf("expected sparse participation rate > 0")
	}
	if report.Results[0].Metrics.PrimarySparseRate <= 0 {
		t.Fatalf("expected primary sparse rate > 0")
	}
}
