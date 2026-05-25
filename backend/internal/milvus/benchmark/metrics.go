package benchmark

import (
	"math"
	"sort"
	"time"
)

func computeRecallAtK(relevantIDs, resultIDs []string, k int) float64 {
	if len(relevantIDs) == 0 || k <= 0 {
		return 0
	}
	rel := make(map[string]struct{}, len(relevantIDs))
	for _, id := range relevantIDs {
		rel[id] = struct{}{}
	}
	hits := 0
	limit := min(k, len(resultIDs))
	for i := 0; i < limit; i++ {
		if _, ok := rel[resultIDs[i]]; ok {
			hits++
		}
	}
	return float64(hits) / float64(len(relevantIDs))
}

func computeMRR(relevantIDs, resultIDs []string, k int) float64 {
	if len(relevantIDs) == 0 || k <= 0 {
		return 0
	}
	rel := make(map[string]struct{}, len(relevantIDs))
	for _, id := range relevantIDs {
		rel[id] = struct{}{}
	}
	limit := min(k, len(resultIDs))
	for i := 0; i < limit; i++ {
		if _, ok := rel[resultIDs[i]]; ok {
			return 1.0 / float64(i+1)
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
		rel[id] = struct{}{}
	}

	dcg := 0.0
	limit := min(k, len(resultIDs))
	for i := 0; i < limit; i++ {
		if _, ok := rel[resultIDs[i]]; ok {
			dcg += 1.0 / math.Log2(float64(i+2))
		}
	}

	idcg := 0.0
	ideal := min(k, len(relevantIDs))
	for i := 0; i < ideal; i++ {
		idcg += 1.0 / math.Log2(float64(i+2))
	}
	if idcg == 0 {
		return 0
	}
	return dcg / idcg
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
