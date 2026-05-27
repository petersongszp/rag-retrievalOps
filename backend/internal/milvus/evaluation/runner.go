package evaluation

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Searcher interface {
	Search(ctx context.Context, item DatasetCase) (SearchOutcome, error)
}

type SearcherFactory func(profile StrategyProfile) (Searcher, error)

type Runner struct {
	Factory SearcherFactory
}

func (r *Runner) Run(ctx context.Context, dataset []DatasetCase, profiles []StrategyProfile, thresholds GateThresholds, baselineName, candidateName string) (*Report, error) {
	if len(dataset) == 0 {
		return nil, fmt.Errorf("evaluation dataset is empty")
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("strategy profiles are empty")
	}
	if r.Factory == nil {
		return nil, fmt.Errorf("searcher factory is required")
	}

	results := make([]StrategyResult, 0, len(profiles))
	for _, profile := range profiles {
		if profile.Name == "" {
			return nil, fmt.Errorf("strategy profile name is required")
		}
		searcher, err := r.Factory(profile)
		if err != nil {
			return nil, fmt.Errorf("create searcher for %s: %w", profile.Name, err)
		}

		queryMetrics := make([]QueryMetrics, 0, len(dataset))
		latencies := make([]time.Duration, 0, len(dataset))
		totalLatency := time.Duration(0)

		for _, item := range dataset {
			topK := item.TopK
			if topK <= 0 {
				topK = 5
			}
			start := time.Now()
			outcome, err := searcher.Search(ctx, item)
			latency := time.Since(start)
			if err != nil {
				return nil, fmt.Errorf("strategy %s query %s failed: %w", profile.Name, item.ID, err)
			}

			resultIDs := make([]string, 0, len(outcome.Items))
			for _, result := range outcome.Items {
				resultIDs = append(resultIDs, result.ResultID)
			}
			refusalExpected := expectsRefusal(item)
			queryMetrics = append(queryMetrics, QueryMetrics{
				QueryID:              item.ID,
				Query:                item.Query,
				QueryType:            item.QueryType,
				Tags:                 item.Tags,
				TopK:                 topK,
				Latency:              latency,
				RecallAtK:            computeRecallAtK(item.RelevantIDs, resultIDs, topK),
				MRR:                  computeMRR(item.RelevantIDs, resultIDs, topK),
				NDCG:                 computeNDCG(item.RelevantIDs, resultIDs, topK),
				CitationAccuracy:     computeCitationAccuracy(item.CitationTargets, item.RelevantIDs, outcome.Items, topK),
				CitationPrecision:    computeCitationPrecision(item.CitationTargets, item.RelevantIDs, outcome.Items, topK),
				CitationRecall:       computeCitationRecall(item.CitationTargets, item.RelevantIDs, outcome.Items, topK),
				LongDocCompleteness:  computeLongDocCompleteness(outcome.Items, topK),
				Refused:              outcome.Refused,
				RefusalReason:        outcome.RefusalReason,
				RefusalExpected:      refusalExpected,
				RefusalCorrect:       outcome.Refused == refusalExpected,
				RefusalFalsePositive: outcome.Refused && !refusalExpected,
				ParentFillCount:      outcome.ParentFillCount,
				RewriteApplied:       outcome.RewriteApplied,
				ModelRewriteApplied:  outcome.ModelRewriteApplied,
				DenseContribution:    outcome.DenseContribution,
				SparseContribution:   outcome.SparseContribution,
				ResultIDs:            resultIDs,
				RelevantIDs:          item.RelevantIDs,
				CitationTargets:      item.CitationTargets,
			})
			latencies = append(latencies, latency)
			totalLatency += latency
		}

		results = append(results, StrategyResult{
			Strategy: profile,
			Metrics:  aggregateQueryMetrics(queryMetrics, latencies, totalLatency),
			Queries:  queryMetrics,
		})
	}

	baseline := resolveStrategyResult(results, baselineName, true, false)
	candidate := resolveStrategyResult(results, candidateName, false, true)
	if baseline == nil {
		return nil, fmt.Errorf("baseline strategy not found")
	}
	if candidate == nil {
		return nil, fmt.Errorf("candidate strategy not found")
	}

	report := &Report{
		DatasetSize:  len(dataset),
		GeneratedAt:  time.Now(),
		Results:      results,
		Contribution: buildContribution(results),
		Comparison:   buildComparison(*baseline, *candidate),
		Baseline:     baseline.Strategy.Name,
		Candidate:    candidate.Strategy.Name,
	}
	report.Gate = EvaluateGate(report.Comparison, thresholds)
	return report, nil
}

func buildContribution(results []StrategyResult) []StrategyDelta {
	if len(results) < 2 {
		return nil
	}
	deltas := make([]StrategyDelta, 0, len(results)-1)
	for i := 1; i < len(results); i++ {
		current := results[i]
		prev := results[i-1]
		deltas = append(deltas, StrategyDelta{
			Strategy:                  current.Strategy.Name,
			ComparedTo:                prev.Strategy.Name,
			RecallDelta:               current.Metrics.RecallAtK - prev.Metrics.RecallAtK,
			MRRDelta:                  current.Metrics.MRR - prev.Metrics.MRR,
			NDCGDelta:                 current.Metrics.NDCG - prev.Metrics.NDCG,
			CitationAccuracyDelta:     current.Metrics.CitationAccuracy - prev.Metrics.CitationAccuracy,
			CitationPrecisionDelta:    current.Metrics.CitationPrecision - prev.Metrics.CitationPrecision,
			CitationRecallDelta:       current.Metrics.CitationRecall - prev.Metrics.CitationRecall,
			LongDocCompletenessDelta:  current.Metrics.LongDocCompleteness - prev.Metrics.LongDocCompleteness,
			ParentFillGainDelta:       current.Metrics.ParentFillGain - prev.Metrics.ParentFillGain,
			RefusalFalsePositiveDelta: current.Metrics.RefusalFalsePositiveRate - prev.Metrics.RefusalFalsePositiveRate,
			P95LatencyDeltaMS:         current.Metrics.P95LatencyMS - prev.Metrics.P95LatencyMS,
		})
	}
	return deltas
}

func buildComparison(baseline, candidate StrategyResult) ComparisonSummary {
	ratio := 0.0
	if baseline.Metrics.P95LatencyMS > 0 {
		ratio = (candidate.Metrics.P95LatencyMS - baseline.Metrics.P95LatencyMS) / baseline.Metrics.P95LatencyMS
	}
	return ComparisonSummary{
		Baseline:                     baseline.Strategy.Name,
		Candidate:                    candidate.Strategy.Name,
		RecallDelta:                  candidate.Metrics.RecallAtK - baseline.Metrics.RecallAtK,
		MRRDelta:                     candidate.Metrics.MRR - baseline.Metrics.MRR,
		NDCGDelta:                    candidate.Metrics.NDCG - baseline.Metrics.NDCG,
		CitationAccuracyDelta:        candidate.Metrics.CitationAccuracy - baseline.Metrics.CitationAccuracy,
		CitationPrecisionDelta:       candidate.Metrics.CitationPrecision - baseline.Metrics.CitationPrecision,
		CitationRecallDelta:          candidate.Metrics.CitationRecall - baseline.Metrics.CitationRecall,
		LongDocCompletenessDelta:     candidate.Metrics.LongDocCompleteness - baseline.Metrics.LongDocCompleteness,
		ParentFillGainDelta:          candidate.Metrics.ParentFillGain - baseline.Metrics.ParentFillGain,
		EvidenceRefusalRateDelta:     candidate.Metrics.EvidenceRefusalRate - baseline.Metrics.EvidenceRefusalRate,
		RefusalFalsePositiveRate:     candidate.Metrics.RefusalFalsePositiveRate,
		RewriteGainDelta:             computeRewriteGainDelta(baseline, candidate),
		DenseRouteContributionDelta:  candidate.Metrics.DenseRouteContribution - baseline.Metrics.DenseRouteContribution,
		SparseRouteContributionDelta: candidate.Metrics.SparseRouteContribution - baseline.Metrics.SparseRouteContribution,
		P95LatencyDeltaMS:            candidate.Metrics.P95LatencyMS - baseline.Metrics.P95LatencyMS,
		P95LatencyDeltaRatio:         ratio,
		CandidateModelRewrite:        candidate.Strategy.EnableModelAssistedRewrite,
	}
}

func expectsRefusal(item DatasetCase) bool {
	if strings.Contains(strings.ToLower(strings.TrimSpace(item.ExpectedBehavior)), "refus") {
		return true
	}
	for _, tag := range item.Tags {
		switch strings.ToLower(strings.TrimSpace(tag)) {
		case "refusal_expected", "expect_refusal", "out_of_scope", "insufficient_evidence":
			return true
		}
	}
	return false
}

func computeRewriteGainDelta(baseline, candidate StrategyResult) float64 {
	if len(candidate.Queries) == 0 {
		return 0
	}
	baselineByID := make(map[string]QueryMetrics, len(baseline.Queries))
	for _, query := range baseline.Queries {
		baselineByID[query.QueryID] = query
	}
	total := 0.0
	count := 0
	for _, query := range candidate.Queries {
		if !query.RewriteApplied {
			continue
		}
		base := baselineByID[query.QueryID]
		total += query.RecallAtK - base.RecallAtK
		count++
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

func resolveStrategyResult(results []StrategyResult, explicitName string, preferBaseline, preferCandidate bool) *StrategyResult {
	if explicitName != "" {
		for i := range results {
			if results[i].Strategy.Name == explicitName {
				return &results[i]
			}
		}
	}
	for i := range results {
		if preferBaseline && results[i].Strategy.Baseline {
			return &results[i]
		}
		if preferCandidate && results[i].Strategy.Candidate {
			return &results[i]
		}
	}
	if preferCandidate && len(results) > 0 {
		return &results[len(results)-1]
	}
	if len(results) > 0 {
		return &results[0]
	}
	return nil
}
