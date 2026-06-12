package evaluation

import (
	"fmt"
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

func computeCitationPrecision(expected []CitationTarget, relevantIDs []string, results []RetrievedItem, k int) float64 {
	if k <= 0 || len(results) == 0 {
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
	limit := min(k, len(results))
	for i := 0; i < limit; i++ {
		for _, target := range targets {
			if citationMatches(target, results[i]) {
				matched++
				break
			}
		}
	}
	return float64(matched) / float64(limit)
}

func computeCitationRecall(expected []CitationTarget, relevantIDs []string, results []RetrievedItem, k int) float64 {
	return computeCitationAccuracy(expected, relevantIDs, results, k)
}

func computeLongDocCompleteness(results []RetrievedItem, k int) float64 {
	if k <= 0 || len(results) == 0 {
		return 0
	}
	limit := min(k, len(results))
	total := 0.0
	for i := 0; i < limit; i++ {
		before := readRawString(results[i].RawFields, "original_child_content")
		after := readRawString(results[i].RawFields, "content")
		if after == "" {
			if value, ok := results[i].RawFields["content"].(string); ok {
				after = value
			}
		}
		if after == "" {
			after = readRawString(results[i].Source, "content")
		}
		beforeLen := len([]rune(before))
		afterLen := len([]rune(after))
		switch {
		case beforeLen <= 0 && afterLen <= 0:
			total += 0
		case beforeLen <= 0:
			total += 1
		default:
			ratio := float64(afterLen) / float64(beforeLen)
			if ratio < 1 {
				ratio = 1
			}
			if ratio > 3 {
				ratio = 3
			}
			total += ratio
		}
	}
	return total / float64(limit)
}

func aggregateQueryMetrics(queryMetrics []QueryMetrics, latencies []time.Duration, totalLatency time.Duration) AggregateMetrics {
	recall := 0.0
	mrr := 0.0
	ndcg := 0.0
	citationAccuracy := 0.0
	citationPrecision := 0.0
	citationRecall := 0.0
	longDocCompleteness := 0.0
	contextualRecallGain := 0.0
	chunkPurity := 0.0
	chunkSelfContainedRate := 0.0
	refusalCount := 0.0
	refusalFalsePositive := 0.0
	parentFillGain := 0.0
	rewriteApplied := 0.0
	modelRewrite := 0.0
	denseHitRate := 0.0
	sparseHitRate := 0.0
	denseParticipationRate := 0.0
	sparseParticipationRate := 0.0
	primaryDenseRate := 0.0
	primarySparseRate := 0.0
	dualRouteRate := 0.0
	emptyRate := 0.0
	denseContribution := 0.0
	sparseContribution := 0.0
	ingestLatencies := make([]float64, 0, len(queryMetrics))
	embeddingTextLengths := make([]int, 0, len(queryMetrics))
	chunksPerDocument := make([]int, 0, len(queryMetrics))
	for _, metric := range queryMetrics {
		recall += metric.RecallAtK
		mrr += metric.MRR
		ndcg += metric.NDCG
		citationAccuracy += metric.CitationAccuracy
		citationPrecision += metric.CitationPrecision
		citationRecall += metric.CitationRecall
		longDocCompleteness += metric.LongDocCompleteness
		contextualRecallGain += metric.ContextualRecallGain
		chunkPurity += metric.ChunkPurity
		chunkSelfContainedRate += metric.ChunkSelfContainedRate
		if metric.Refused {
			refusalCount++
		}
		if metric.RefusalFalsePositive {
			refusalFalsePositive++
		}
		if metric.ParentFillCount > 0 && metric.LongDocCompleteness > 1 {
			parentFillGain += metric.LongDocCompleteness - 1
		}
		if metric.RewriteApplied {
			rewriteApplied++
		}
		if metric.ModelRewriteApplied {
			modelRewrite++
		}
		if metric.DenseHits > 0 {
			denseHitRate++
		}
		if metric.SparseHits > 0 {
			sparseHitRate++
		}
		if metric.DenseParticipation > 0 {
			denseParticipationRate++
		}
		if metric.SparseParticipation > 0 {
			sparseParticipationRate++
		}
		if metric.PrimaryDenseCount > 0 {
			primaryDenseRate += float64(metric.PrimaryDenseCount) / float64(maxInt(metric.TopK, 1))
		}
		if metric.PrimarySparseCount > 0 {
			primarySparseRate += float64(metric.PrimarySparseCount) / float64(maxInt(metric.TopK, 1))
		}
		if metric.DualRouteFinalCount > 0 {
			dualRouteRate += float64(metric.DualRouteFinalCount) / float64(maxInt(metric.TopK, 1))
		}
		if metric.EmptyResult || len(metric.ResultIDs) == 0 {
			emptyRate++
		}
		totalRouteContribution := metric.DenseContribution + metric.SparseContribution
		if totalRouteContribution > 0 {
			denseContribution += float64(metric.DenseContribution) / float64(totalRouteContribution)
			sparseContribution += float64(metric.SparseContribution) / float64(totalRouteContribution)
		}
		ingestLatencies = append(ingestLatencies, metric.IngestLatencyMS)
		embeddingTextLengths = append(embeddingTextLengths, metric.EmbeddingTextLength)
		chunksPerDocument = append(chunksPerDocument, metric.ChunksPerDocument)
	}
	count := float64(len(queryMetrics))
	if count == 0 {
		count = 1
	}
	return AggregateMetrics{
		RecallAtK:                recall / count,
		MRR:                      mrr / count,
		NDCG:                     ndcg / count,
		CitationAccuracy:         citationAccuracy / count,
		CitationPrecision:        citationPrecision / count,
		CitationRecall:           citationRecall / count,
		LongDocCompleteness:      longDocCompleteness / count,
		ContextualRecallGain:     contextualRecallGain / count,
		ChunkPurity:              chunkPurity / count,
		ChunkSelfContainedRate:   chunkSelfContainedRate / count,
		EvidenceRefusalRate:      refusalCount / count,
		RefusalFalsePositiveRate: refusalFalsePositive / count,
		ParentFillGain:           parentFillGain / count,
		RewriteAppliedRate:       rewriteApplied / count,
		ModelRewriteRate:         modelRewrite / count,
		DenseHitRate:             denseHitRate / count,
		SparseHitRate:            sparseHitRate / count,
		DenseParticipationRate:   denseParticipationRate / count,
		SparseParticipationRate:  sparseParticipationRate / count,
		PrimaryDenseRate:         primaryDenseRate / count,
		PrimarySparseRate:        primarySparseRate / count,
		DualRouteRate:            dualRouteRate / count,
		EmptyRate:                emptyRate / count,
		DenseRouteContribution:   denseContribution / count,
		SparseRouteContribution:  sparseContribution / count,
		IngestP95MS:              percentileFloat64(ingestLatencies, 95),
		AvgEmbeddingTextLength:   averageInt(embeddingTextLengths),
		P95EmbeddingTextLength:   percentileInt(embeddingTextLengths, 95),
		AvgChunksPerDocument:     averageInt(chunksPerDocument),
		P95ChunksPerDocument:     percentileInt(chunksPerDocument, 95),
		P50LatencyMS:             percentileLatency(latencies, 50),
		P95LatencyMS:             percentileLatency(latencies, 95),
		AvgLatencyMS:             averageLatency(latencies),
		TotalLatency:             totalLatency,
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

func percentileFloat64(values []float64, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)
	index := int(math.Ceil((percentile/100.0)*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func percentileInt(values []int, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]int, len(values))
	copy(sorted, values)
	sort.Ints(sorted)
	index := int(math.Ceil((percentile/100.0)*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return float64(sorted[index])
}

func averageInt(values []int) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0
	for _, value := range values {
		total += value
	}
	return float64(total) / float64(len(values))
}

func normalizeID(id string) string {
	return strings.TrimSpace(strings.ToLower(id))
}

func readRawString(values map[string]interface{}, key string) string {
	if values == nil {
		return ""
	}
	if value, ok := values[key]; ok && value != nil {
		switch typed := value.(type) {
		case string:
			return strings.TrimSpace(typed)
		default:
			return strings.TrimSpace(fmt.Sprint(typed))
		}
	}
	return ""
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
