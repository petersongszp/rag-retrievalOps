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
				QueryID:                     item.ID,
				Query:                       item.Query,
				QueryType:                   item.QueryType,
				Tags:                        item.Tags,
				TopK:                        topK,
				Latency:                     latency,
				RecallAtK:                   computeRecallAtK(item.RelevantIDs, resultIDs, topK),
				MRR:                         computeMRR(item.RelevantIDs, resultIDs, topK),
				NDCG:                        computeNDCG(item.RelevantIDs, resultIDs, topK),
				CitationAccuracy:            computeCitationAccuracy(item.CitationTargets, item.RelevantIDs, outcome.Items, topK),
				CitationPrecision:           computeCitationPrecision(item.CitationTargets, item.RelevantIDs, outcome.Items, topK),
				CitationRecall:              computeCitationRecall(item.CitationTargets, item.RelevantIDs, outcome.Items, topK),
				LongDocCompleteness:         computeLongDocCompleteness(outcome.Items, topK),
				ContextualRecallGain:        outcome.ContextualRecallGain,
				ChunkPurity:                 outcome.ChunkPurity,
				ChunkSelfContainedRate:      outcome.ChunkSelfContained,
				IngestLatencyMS:             outcome.IngestLatencyMS,
				EmbeddingTextLength:         outcome.EmbeddingTextLength,
				ChunksPerDocument:           outcome.ChunksPerDocument,
				Refused:                     outcome.Refused,
				RefusalReason:               outcome.RefusalReason,
				RefusalExpected:             refusalExpected,
				RefusalCorrect:              outcome.Refused == refusalExpected,
				RefusalFalsePositive:        outcome.Refused && !refusalExpected,
				ParentFillCount:             outcome.ParentFillCount,
				RewriteApplied:              outcome.RewriteApplied,
				ModelRewriteApplied:         outcome.ModelRewriteApplied,
				ExpectedPrimaryRoute:        item.ExpectedPrimaryRoute,
				ExpectedParticipatingRoutes: append([]string(nil), item.ExpectedParticipatingRoutes...),
				MustContainTerms:            append([]string(nil), item.MustContainTerms...),
				DenseHits:                   outcome.DenseHits,
				SparseHits:                  outcome.SparseHits,
				DenseParticipation:          outcome.DenseParticipation,
				SparseParticipation:         outcome.SparseParticipation,
				PrimaryDenseCount:           outcome.PrimaryDenseCount,
				PrimarySparseCount:          outcome.PrimarySparseCount,
				DualRouteFinalCount:         outcome.DualRouteFinalCount,
				PrimaryRoute:                resolvePrimaryRoute(outcome),
				EmptyResult:                 len(resultIDs) == 0,
				EmptyReason:                 outcome.EmptyReason,
				DenseContribution:           outcome.DenseContribution,
				SparseContribution:          outcome.SparseContribution,
				SparseCandidateBefore:       outcome.SparseCandidateBefore,
				SparseCandidateAfter:        outcome.SparseCandidateAfter,
				ResultIDs:                   resultIDs,
				RelevantIDs:                 item.RelevantIDs,
				CitationTargets:             item.CitationTargets,
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
		DatasetSize:    len(dataset),
		GeneratedAt:    time.Now(),
		FusionStrategy: resolveReportFusionStrategy(*baseline, *candidate),
		Results:        results,
		Contribution:   buildContribution(results),
		Comparison:     buildComparison(*baseline, *candidate),
		Baseline:       baseline.Strategy.Name,
		Candidate:      candidate.Strategy.Name,
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
			FusionStrategy:            current.Strategy.FusionStrategy,
			ComparedTo:                prev.Strategy.Name,
			ComparedToFusionStrategy:  prev.Strategy.FusionStrategy,
			RecallDelta:               current.Metrics.RecallAtK - prev.Metrics.RecallAtK,
			MRRDelta:                  current.Metrics.MRR - prev.Metrics.MRR,
			NDCGDelta:                 current.Metrics.NDCG - prev.Metrics.NDCG,
			CitationAccuracyDelta:     current.Metrics.CitationAccuracy - prev.Metrics.CitationAccuracy,
			CitationPrecisionDelta:    current.Metrics.CitationPrecision - prev.Metrics.CitationPrecision,
			CitationRecallDelta:       current.Metrics.CitationRecall - prev.Metrics.CitationRecall,
			LongDocCompletenessDelta:  current.Metrics.LongDocCompleteness - prev.Metrics.LongDocCompleteness,
			ContextualRecallGainDelta: current.Metrics.ContextualRecallGain - prev.Metrics.ContextualRecallGain,
			ChunkPurityDelta:          current.Metrics.ChunkPurity - prev.Metrics.ChunkPurity,
			ChunkSelfContainedDelta:   current.Metrics.ChunkSelfContainedRate - prev.Metrics.ChunkSelfContainedRate,
			ParentFillGainDelta:       current.Metrics.ParentFillGain - prev.Metrics.ParentFillGain,
			RefusalFalsePositiveDelta: current.Metrics.RefusalFalsePositiveRate - prev.Metrics.RefusalFalsePositiveRate,
			DenseHitRateDelta:         current.Metrics.DenseHitRate - prev.Metrics.DenseHitRate,
			SparseHitRateDelta:        current.Metrics.SparseHitRate - prev.Metrics.SparseHitRate,
			DenseParticipationDelta:   current.Metrics.DenseParticipationRate - prev.Metrics.DenseParticipationRate,
			SparseParticipationDelta:  current.Metrics.SparseParticipationRate - prev.Metrics.SparseParticipationRate,
			PrimaryDenseRateDelta:     current.Metrics.PrimaryDenseRate - prev.Metrics.PrimaryDenseRate,
			PrimarySparseRateDelta:    current.Metrics.PrimarySparseRate - prev.Metrics.PrimarySparseRate,
			EmptyRateDelta:            current.Metrics.EmptyRate - prev.Metrics.EmptyRate,
			IngestP95DeltaMS:          current.Metrics.IngestP95MS - prev.Metrics.IngestP95MS,
			IngestP95DeltaRatio:       computeDeltaRatio(prev.Metrics.IngestP95MS, current.Metrics.IngestP95MS),
			AvgEmbeddingLengthDelta:   current.Metrics.AvgEmbeddingTextLength - prev.Metrics.AvgEmbeddingTextLength,
			P95EmbeddingLengthDelta:   current.Metrics.P95EmbeddingTextLength - prev.Metrics.P95EmbeddingTextLength,
			AvgChunksPerDocDelta:      current.Metrics.AvgChunksPerDocument - prev.Metrics.AvgChunksPerDocument,
			P95ChunksPerDocDelta:      current.Metrics.P95ChunksPerDocument - prev.Metrics.P95ChunksPerDocument,
			P95LatencyDeltaMS:         current.Metrics.P95LatencyMS - prev.Metrics.P95LatencyMS,
		})
	}
	return deltas
}

func buildComparison(baseline, candidate StrategyResult) ComparisonSummary {
	return ComparisonSummary{
		Baseline:                     baseline.Strategy.Name,
		Candidate:                    candidate.Strategy.Name,
		BaselineFusionStrategy:       baseline.Strategy.FusionStrategy,
		CandidateFusionStrategy:      candidate.Strategy.FusionStrategy,
		RecallDelta:                  candidate.Metrics.RecallAtK - baseline.Metrics.RecallAtK,
		MRRDelta:                     candidate.Metrics.MRR - baseline.Metrics.MRR,
		NDCGDelta:                    candidate.Metrics.NDCG - baseline.Metrics.NDCG,
		CitationAccuracyDelta:        candidate.Metrics.CitationAccuracy - baseline.Metrics.CitationAccuracy,
		CitationPrecisionDelta:       candidate.Metrics.CitationPrecision - baseline.Metrics.CitationPrecision,
		CitationRecallDelta:          candidate.Metrics.CitationRecall - baseline.Metrics.CitationRecall,
		LongDocCompletenessDelta:     candidate.Metrics.LongDocCompleteness - baseline.Metrics.LongDocCompleteness,
		ContextualRecallGainDelta:    candidate.Metrics.ContextualRecallGain - baseline.Metrics.ContextualRecallGain,
		ChunkPurityDelta:             candidate.Metrics.ChunkPurity - baseline.Metrics.ChunkPurity,
		ChunkSelfContainedDelta:      candidate.Metrics.ChunkSelfContainedRate - baseline.Metrics.ChunkSelfContainedRate,
		ParentFillGainDelta:          candidate.Metrics.ParentFillGain - baseline.Metrics.ParentFillGain,
		EvidenceRefusalRateDelta:     candidate.Metrics.EvidenceRefusalRate - baseline.Metrics.EvidenceRefusalRate,
		RefusalFalsePositiveRate:     candidate.Metrics.RefusalFalsePositiveRate,
		RewriteGainDelta:             computeRewriteGainDelta(baseline, candidate),
		DenseRouteContributionDelta:  candidate.Metrics.DenseRouteContribution - baseline.Metrics.DenseRouteContribution,
		SparseRouteContributionDelta: candidate.Metrics.SparseRouteContribution - baseline.Metrics.SparseRouteContribution,
		DenseHitRateDelta:            candidate.Metrics.DenseHitRate - baseline.Metrics.DenseHitRate,
		SparseHitRateDelta:           candidate.Metrics.SparseHitRate - baseline.Metrics.SparseHitRate,
		DenseParticipationRateDelta:  candidate.Metrics.DenseParticipationRate - baseline.Metrics.DenseParticipationRate,
		SparseParticipationRateDelta: candidate.Metrics.SparseParticipationRate - baseline.Metrics.SparseParticipationRate,
		PrimaryDenseRateDelta:        candidate.Metrics.PrimaryDenseRate - baseline.Metrics.PrimaryDenseRate,
		PrimarySparseRateDelta:       candidate.Metrics.PrimarySparseRate - baseline.Metrics.PrimarySparseRate,
		EmptyRateDelta:               candidate.Metrics.EmptyRate - baseline.Metrics.EmptyRate,
		IngestP95DeltaMS:             candidate.Metrics.IngestP95MS - baseline.Metrics.IngestP95MS,
		IngestP95DeltaRatio:          computeDeltaRatio(baseline.Metrics.IngestP95MS, candidate.Metrics.IngestP95MS),
		AvgEmbeddingLengthDelta:      candidate.Metrics.AvgEmbeddingTextLength - baseline.Metrics.AvgEmbeddingTextLength,
		P95EmbeddingLengthDelta:      candidate.Metrics.P95EmbeddingTextLength - baseline.Metrics.P95EmbeddingTextLength,
		AvgChunksPerDocDelta:         candidate.Metrics.AvgChunksPerDocument - baseline.Metrics.AvgChunksPerDocument,
		P95ChunksPerDocDelta:         candidate.Metrics.P95ChunksPerDocument - baseline.Metrics.P95ChunksPerDocument,
		P95LatencyDeltaMS:            candidate.Metrics.P95LatencyMS - baseline.Metrics.P95LatencyMS,
		P95LatencyDeltaRatio:         computeDeltaRatio(baseline.Metrics.P95LatencyMS, candidate.Metrics.P95LatencyMS),
		CandidateModelRewrite:        candidate.Strategy.EnableModelAssistedRewrite,
	}
}

func resolveReportFusionStrategy(baseline, candidate StrategyResult) string {
	if candidate.Strategy.FusionStrategy != "" {
		return candidate.Strategy.FusionStrategy
	}
	return baseline.Strategy.FusionStrategy
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

func resolvePrimaryRoute(outcome SearchOutcome) string {
	switch {
	case outcome.PrimaryDenseCount > 0 && outcome.PrimarySparseCount > 0:
		return "mixed"
	case outcome.PrimaryDenseCount > 0:
		return "dense"
	case outcome.PrimarySparseCount > 0:
		return "sparse"
	default:
		return ""
	}
}

func computeDeltaRatio(baseline, candidate float64) float64 {
	if baseline <= 0 {
		return 0
	}
	return (candidate - baseline) / baseline
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
