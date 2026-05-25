package benchmark

import (
	"context"
	"strings"
	"testing"
)

type fakeSearcher struct {
	results map[string][]string
}

func (f *fakeSearcher) Search(_ context.Context, query string, _ int) ([]string, error) {
	return f.results[query], nil
}

func TestRunnerComputesAggregateMetricsAndRecommendation(t *testing.T) {
	dataset := []QueryCase{
		{ID: "q1", Query: "goroutine", TopK: 3, RelevantIDs: []string{"doc-1"}},
		{ID: "q2", Query: "mysql mvcc", TopK: 3, RelevantIDs: []string{"doc-2"}},
	}
	profiles := []IndexProfile{
		{
			Name:       "baseline",
			Family:     IndexFamilyHNSW,
			MetricType: "COSINE",
			IsBaseline: true,
			HNSW:       &HNSWParams{M: 16, EfConstruction: 200, EfSearch: 64},
		},
		{
			Name:       "candidate",
			Family:     IndexFamilyHNSW,
			MetricType: "COSINE",
			HNSW:       &HNSWParams{M: 24, EfConstruction: 320, EfSearch: 96},
		},
	}

	searchers := map[string]*fakeSearcher{
		"baseline": {results: map[string][]string{
			"goroutine":  {"doc-x", "doc-1", "doc-9"},
			"mysql mvcc": {"doc-2", "doc-x", "doc-y"},
		}},
		"candidate": {results: map[string][]string{
			"goroutine":  {"doc-1", "doc-x", "doc-9"},
			"mysql mvcc": {"doc-2", "doc-x", "doc-y"},
		}},
	}

	runner := &Runner{
		Factory: func(profile IndexProfile) (Searcher, error) {
			return searchers[profile.Name], nil
		},
	}

	report, err := runner.Run(context.Background(), dataset, profiles)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(report.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(report.Results))
	}
	if report.Recommendation.RecommendedProfile != "candidate" {
		t.Fatalf("expected candidate recommendation, got %s", report.Recommendation.RecommendedProfile)
	}
	if report.Results[0].Metrics.RecallAtK < report.Results[1].Metrics.RecallAtK {
		t.Fatalf("expected results sorted by descending quality")
	}
}

func TestRenderMarkdownReportIncludesRollbackSection(t *testing.T) {
	report := &Report{
		DatasetSize:     1,
		ProfilesScanned: []string{"baseline", "candidate"},
		Results: []ProfileResult{
			{
				Profile: IndexProfile{Name: "candidate", Family: IndexFamilyHNSW},
				Metrics: AggregateMetrics{RecallAtK: 1, MRR: 1, NDCG: 1},
			},
		},
		Recommendation: Recommendation{
			BaselineProfile:    "baseline",
			RecommendedProfile: "candidate",
			RollbackSteps:      []string{"switch back to baseline"},
		},
	}
	markdown := RenderMarkdownReport(report)
	if !strings.Contains(markdown, "## 回滚清单") {
		t.Fatalf("expected rollback section in markdown")
	}
	if !strings.Contains(markdown, "candidate") {
		t.Fatalf("expected recommended profile in markdown")
	}
}
