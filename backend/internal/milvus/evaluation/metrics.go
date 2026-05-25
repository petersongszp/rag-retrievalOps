package evaluation

import (
	"math"
	"sort"
	"strings"
	"time"
)

func computeRecallAtK(relevantIDs, resultIDs []string, k int) float64 {
	if len(relevantIDs) == 0 || k <= 0 {
		return 0
	}
	rel := make(map[string]struct{}, len(relevantIDs))
	for _, id := range relevantIDs {
		if normalized := normalizeID(id); normalized != "" {
			rel[normalized] = struct{}{}
		}
	}
	hits := 0
	limit := min(k, len(resultIDs))
	for i := 0; i < limit; i++ {
		if _, ok := rel[normalizeID(resultIDs[i])]; ok {
			hits++
		}
	}
	return float64(hits) / float64(len(rel))
}

func computeMRR(relevantIDs, resultIDs []string, k int) float64 {
	if len(relevantIDs) == 0 || k <= 0 {
		return 0
	}
	rel := make(map[string]struct{}, len(relevantIDs))
	for _, id := range relevantIDs {
		if normalized := normalizeID(id); normalized != "" {
			rel[normalized] = struct{}{}
		}
	}
	limit := min(k, len(resultIDs))
	for i := 0; i < limit; i++ {
		if _, ok := rel[normalizeID(resultIDs[i])]; ok {
			return 1 / float64(i+1)
		}
	}
	return 0
}

func computeNDCG(relevantIDs, resultIDs []string, k int) float64 {
	if len(relevantIDs) == 0 || k <= 0 {
		return 0
	}
	rel := make(map[string]struct{}, len(relevantIDs))
	for _, id := range relevantIDs {
		if normalized := normalizeID(id); normalized != "" {
			rel[normalized] = struct{}{}
		}
	}

	dcg := 0.0
	limit := min(k, len(resultIDs))
	for i := 0; i < limit; i++ {
		if _, ok := rel[normalizeID(resultIDs[i])]; ok {
			dcg += 1.0 / math.Log2(float64(i+2))
		}
	}

	idcg := 0.0
	ideal := min(k, len(rel))
	for i := 0; i < ideal; i++ {
		idcg += 1.0 / math.Log2(float64(i+2))
	}
	if idcg == 0 {
		return 0
	}
	return dcg / idcg
}

func computeCitationAccuracy(expected []CitationTarget, relevantIDs []string, results []RetrievedItem, k int) float64 {
	if k <= 0 {
		return 0
	}
	targets := expected
	if len(targets) == 0 && len(relevantIDs) > 0 {
		targets = make([]CitationTarget, 0, len(relevantIDs))
		for _, id := range relevantIDs {
			if normalized := normalizeID(id); normalized != "" {
				targets = append(targets, CitationTarget{ChunkID: normalized})
			}
		}
	}
	if len(targets) == 0 {
		return 0
	}

	matched := 0
	used := make([]bool, len(targets))
	limit := min(k, len(results))
	for i := 0; i < limit; i++ {
		for idx, target := range targets {
			if used[idx] {
				continue
			}
			if citationMatches(target, results[i]) {
				used[idx] = true
				matched++
				break
			}
		}
	}
	return float64(matched) / float64(len(targets))
}

func aggregateQueryMetrics(queryMetrics []QueryMetrics, latencies []time.Duration, totalLatency time.Duration) AggregateMetrics {
	recall := 0.0
	mrr := 0.0
	ndcg := 0.0
	citationAccuracy := 0.0
	for _, metric := range queryMetrics {
		recall += metric.RecallAtK
		mrr += metric.MRR
		ndcg += metric.NDCG
		citationAccuracy += metric.CitationAccuracy
	}
	count := float64(len(queryMetrics))
	if count == 0 {
		count = 1
	}
	return AggregateMetrics{
		RecallAtK:        recall / count,
		MRR:              mrr / count,
		NDCG:             ndcg / count,
		CitationAccuracy: citationAccuracy / count,
		P50LatencyMS:     percentileLatency(latencies, 50),
		P95LatencyMS:     percentileLatency(latencies, 95),
		AvgLatencyMS:     averageLatency(latencies),
		TotalLatency:     totalLatency,
	}
}

func percentileLatency(latencies []time.Duration, percentile float64) float64 {
	if len(latencies) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	index := int(math.Ceil((percentile/100.0)*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return float64(sorted[index].Microseconds()) / 1000.0
}

func averageLatency(latencies []time.Duration) float64 {
	if len(latencies) == 0 {
		return 0
	}
	var total time.Duration
	for _, latency := range latencies {
		total += latency
	}
	return float64(total.Microseconds()) / 1000.0 / float64(len(latencies))
}

func normalizeID(id string) string {
	return strings.TrimSpace(strings.ToLower(id))
}

func citationMatches(target CitationTarget, result RetrievedItem) bool {
	targetChunk := normalizeID(target.ChunkID)
	resultChunk := normalizeID(result.Citation.ChunkID)
	if targetChunk != "" && targetChunk == resultChunk {
		return true
	}

	if target.DocumentID > 0 && target.DocumentID == result.Citation.DocumentID {
		return true
	}

	targetFile := normalizeID(target.FileName)
	resultFile := normalizeID(result.Citation.FileName)
	return targetFile != "" && targetFile == resultFile
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
