package benchmark

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Searcher interface {
	Search(ctx context.Context, query string, topK int) ([]string, error)
}

type SearcherFactory func(profile IndexProfile) (Searcher, error)

type ProfileApplier interface {
	ApplyProfile(ctx context.Context, profile IndexProfile) error
}

type Runner struct {
	Factory SearcherFactory
	Applier ProfileApplier
	Warmup  int
}

func (r *Runner) Run(ctx context.Context, dataset []QueryCase, profiles []IndexProfile) (*Report, error) {
	if len(dataset) == 0 {
		return nil, fmt.Errorf("benchmark dataset is empty")
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("benchmark profiles are empty")
	}
	if r.Factory == nil {
		return nil, fmt.Errorf("searcher factory is required")
	}

	results := make([]ProfileResult, 0, len(profiles))
	scanned := make([]string, 0, len(profiles))

	for _, profile := range profiles {
		if err := ValidateProfile(profile); err != nil {
			return nil, err
		}
		if r.Applier != nil {
			if err := r.Applier.ApplyProfile(ctx, profile); err != nil {
				return nil, fmt.Errorf("apply profile %s: %w", profile.Name, err)
			}
		}
		searcher, err := r.Factory(profile)
		if err != nil {
			return nil, fmt.Errorf("create searcher for %s: %w", profile.Name, err)
		}

		for i := 0; i < r.Warmup && i < len(dataset); i++ {
			_, _ = searcher.Search(ctx, dataset[i].Query, normalizedTopK(dataset[i]))
		}

		before := captureResourceSnapshot()
		queryMetrics := make([]QueryMetrics, 0, len(dataset))
		latencies := make([]time.Duration, 0, len(dataset))
		totalLatency := time.Duration(0)

		for _, item := range dataset {
			topK := normalizedTopK(item)
			start := time.Now()
			resultIDs, err := searcher.Search(ctx, item.Query, topK)
			latency := time.Since(start)
			if err != nil {
				return nil, fmt.Errorf("profile %s query %s failed: %w", profile.Name, item.ID, err)
			}

			queryMetrics = append(queryMetrics, QueryMetrics{
				QueryID:    item.ID,
				Query:      item.Query,
				TopK:       topK,
				Latency:    latency,
				RecallAtK:  computeRecallAtK(item.RelevantIDs, resultIDs, topK),
				MRR:        computeMRR(item.RelevantIDs, resultIDs, topK),
				NDCG:       computeNDCG(item.RelevantIDs, resultIDs, topK),
				ResultIDs:  resultIDs,
				RelevantID: item.RelevantIDs,
			})
			latencies = append(latencies, latency)
			totalLatency += latency
		}
		after := captureResourceSnapshot()

		results = append(results, ProfileResult{
			Profile: profile,
			Metrics: aggregateQueryMetrics(queryMetrics, latencies, totalLatency, before, after),
			Queries: queryMetrics,
		})
		scanned = append(scanned, profile.Name)
	}

	SortResults(results)
	report := &Report{
		DatasetSize:     len(dataset),
		GeneratedAt:     time.Now(),
		Results:         results,
		ProfilesScanned: scanned,
	}
	report.Recommendation = BuildRecommendation(results)
	return report, nil
}

func aggregateQueryMetrics(queryMetrics []QueryMetrics, latencies []time.Duration, totalLatency time.Duration, before, after resourceSnapshot) AggregateMetrics {
	recall := 0.0
	mrr := 0.0
	ndcg := 0.0
	for _, metric := range queryMetrics {
		recall += metric.RecallAtK
		mrr += metric.MRR
		ndcg += metric.NDCG
	}
	count := float64(len(queryMetrics))
	if count == 0 {
		count = 1
	}
	return AggregateMetrics{
		RecallAtK:    recall / count,
		MRR:          mrr / count,
		NDCG:         ndcg / count,
		P50LatencyMS: percentileLatency(latencies, 50),
		P95LatencyMS: percentileLatency(latencies, 95),
		AvgLatencyMS: averageLatency(latencies),
		TotalLatency: totalLatency,
		Resources: ResourceUsage{
			ProcessCPUUserMS:   maxInt64(0, after.userMS-before.userMS),
			ProcessCPUSystemMS: maxInt64(0, after.sysMS-before.sysMS),
			HeapAllocMB:        after.allocMB,
			HeapSysMB:          after.sysMB,
		},
	}
}

func BuildRecommendation(results []ProfileResult) Recommendation {
	recommendation := Recommendation{}
	if len(results) == 0 {
		return recommendation
	}

	var baseline ProfileResult
	baselineFound := false
	for _, result := range results {
		if result.Profile.IsBaseline {
			baseline = result
			baselineFound = true
			break
		}
	}
	if !baselineFound {
		baseline = results[len(results)-1]
	}

	recommended := results[0]
	recommendation.RecommendedProfile = recommended.Profile.Name
	recommendation.BaselineProfile = baseline.Profile.Name

	qualityGain := recommendationScore(recommended) - recommendationScore(baseline)
	latencyDelta := recommended.Metrics.P95LatencyMS - baseline.Metrics.P95LatencyMS
	recommendation.Reasons = []string{
		fmt.Sprintf("Top overall quality score profile is `%s`.", recommended.Profile.Name),
		fmt.Sprintf("Relative to baseline, quality score delta is %.4f and P95 latency delta is %.2f ms.", qualityGain, latencyDelta),
		fmt.Sprintf("Recommended family is `%s`, which keeps MetricType `%s`.", recommended.Profile.Family, strings.ToUpper(recommended.Profile.MetricType)),
	}
	if latencyDelta > 0 {
		recommendation.Risks = append(recommendation.Risks, "P95 latency increased versus baseline; verify online traffic headroom before rollout.")
	}
	if recommended.Profile.Family != baseline.Profile.Family {
		recommendation.Risks = append(recommendation.Risks, "Index family changed; rehearse full reindex and load timeline before production switch.")
	}
	recommendation.RollbackSteps = []string{
		fmt.Sprintf("Release collection and re-apply baseline profile `%s`.", baseline.Profile.Name),
		"Reload collection and run the offline benchmark smoke subset.",
		"Switch runtime search profile back to baseline and monitor Recall proxy plus P95 latency.",
	}
	return recommendation
}

func normalizedTopK(item QueryCase) int {
	if item.TopK > 0 {
		return item.TopK
	}
	return 5
}

func maxInt64(a, b int64) int64 {
	if a < b {
		return b
	}
	return a
}
