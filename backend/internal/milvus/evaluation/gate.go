package evaluation

import "fmt"

func DefaultGateThresholds() GateThresholds {
	return GateThresholds{
		MinRecallDelta:               0.08,
		MinMRRDelta:                  0.00,
		MinNDCGDelta:                 0.00,
		MinCitationAccuracyDelta:     0.00,
		MinCitationPrecisionDelta:    0.00,
		MinCitationRecallDelta:       0.00,
		MinContextualRecallGainDelta: 0.00,
		MinChunkPurityDelta:          0.00,
		MinChunkSelfContainedDelta:   0.00,
		MinParentFillGainDelta:       0.00,
		MaxP95LatencyRegressionMS:    0,
		MaxP95LatencyRegressionRatio: 0.20,
		MaxIngestP95RegressionMS:     0,
		MaxIngestP95RegressionRatio:  0,
		MaxAvgEmbeddingLengthDelta:   0,
		MaxP95EmbeddingLengthDelta:   0,
		MaxAvgChunksPerDocDelta:      0,
		MaxP95ChunksPerDocDelta:      0,
		MaxRefusalFalsePositiveRate:  0.05,
		MinRewriteGainDelta:          0.00,
	}
}

func EvaluateGate(comparison ComparisonSummary, thresholds GateThresholds) GateResult {
	checks := []GateCheck{
		{
			Name:     "recall_at_k_delta",
			Actual:   comparison.RecallDelta,
			Expected: thresholds.MinRecallDelta,
			Passed:   comparison.RecallDelta >= thresholds.MinRecallDelta,
			Message:  "candidate recall gain must stay above threshold",
		},
		{
			Name:     "mrr_delta",
			Actual:   comparison.MRRDelta,
			Expected: thresholds.MinMRRDelta,
			Passed:   comparison.MRRDelta >= thresholds.MinMRRDelta,
			Message:  "candidate MRR must not regress below threshold",
		},
		{
			Name:     "ndcg_delta",
			Actual:   comparison.NDCGDelta,
			Expected: thresholds.MinNDCGDelta,
			Passed:   comparison.NDCGDelta >= thresholds.MinNDCGDelta,
			Message:  "candidate nDCG must not regress below threshold",
		},
		{
			Name:     "citation_accuracy_delta",
			Actual:   comparison.CitationAccuracyDelta,
			Expected: thresholds.MinCitationAccuracyDelta,
			Passed:   comparison.CitationAccuracyDelta >= thresholds.MinCitationAccuracyDelta,
			Message:  "candidate citation accuracy must not regress below threshold",
		},
		{
			Name:     "citation_precision_delta",
			Actual:   comparison.CitationPrecisionDelta,
			Expected: thresholds.MinCitationPrecisionDelta,
			Passed:   comparison.CitationPrecisionDelta >= thresholds.MinCitationPrecisionDelta,
			Message:  "candidate citation precision must not regress below threshold",
		},
		{
			Name:     "citation_recall_delta",
			Actual:   comparison.CitationRecallDelta,
			Expected: thresholds.MinCitationRecallDelta,
			Passed:   comparison.CitationRecallDelta >= thresholds.MinCitationRecallDelta,
			Message:  "candidate citation recall must not regress below threshold",
		},
		{
			Name:     "contextual_recall_gain_delta",
			Actual:   comparison.ContextualRecallGainDelta,
			Expected: thresholds.MinContextualRecallGainDelta,
			Passed:   comparison.ContextualRecallGainDelta >= thresholds.MinContextualRecallGainDelta,
			Message:  "candidate contextual recall gain must not regress below threshold",
		},
		{
			Name:     "chunk_purity_delta",
			Actual:   comparison.ChunkPurityDelta,
			Expected: thresholds.MinChunkPurityDelta,
			Passed:   comparison.ChunkPurityDelta >= thresholds.MinChunkPurityDelta,
			Message:  "candidate chunk purity must not regress below threshold",
		},
		{
			Name:     "chunk_self_contained_rate_delta",
			Actual:   comparison.ChunkSelfContainedDelta,
			Expected: thresholds.MinChunkSelfContainedDelta,
			Passed:   comparison.ChunkSelfContainedDelta >= thresholds.MinChunkSelfContainedDelta,
			Message:  "candidate chunk self-contained rate must not regress below threshold",
		},
		{
			Name:     "parent_fill_gain_delta",
			Actual:   comparison.ParentFillGainDelta,
			Expected: thresholds.MinParentFillGainDelta,
			Passed:   comparison.ParentFillGainDelta >= thresholds.MinParentFillGainDelta,
			Message:  "candidate parent fill gain must not regress below threshold",
		},
		{
			Name:     "p95_latency_regression_ratio",
			Actual:   comparison.P95LatencyDeltaRatio,
			Expected: thresholds.MaxP95LatencyRegressionRatio,
			Passed:   comparison.P95LatencyDeltaRatio <= thresholds.MaxP95LatencyRegressionRatio,
			Message:  "candidate P95 regression ratio must stay within threshold",
		},
		{
			Name:     "refusal_false_positive_rate",
			Actual:   comparison.RefusalFalsePositiveRate,
			Expected: thresholds.MaxRefusalFalsePositiveRate,
			Passed:   comparison.RefusalFalsePositiveRate <= thresholds.MaxRefusalFalsePositiveRate,
			Message:  "candidate refusal false positive rate must stay within threshold",
		},
	}

	if comparison.CandidateModelRewrite || thresholds.MinRewriteGainDelta > 0 {
		checks = append(checks, GateCheck{
			Name:     "rewrite_gain_delta",
			Actual:   comparison.RewriteGainDelta,
			Expected: thresholds.MinRewriteGainDelta,
			Passed:   comparison.RewriteGainDelta >= thresholds.MinRewriteGainDelta,
			Message:  "model-assisted rewrite must show non-negative gain before broader rollout",
		})
	}

	if thresholds.MaxP95LatencyRegressionMS > 0 {
		checks = append(checks, GateCheck{
			Name:     "p95_latency_regression_ms",
			Actual:   comparison.P95LatencyDeltaMS,
			Expected: thresholds.MaxP95LatencyRegressionMS,
			Passed:   comparison.P95LatencyDeltaMS <= thresholds.MaxP95LatencyRegressionMS,
			Message:  "candidate P95 regression ms must stay within threshold",
		})
	}

	if thresholds.MaxIngestP95RegressionRatio > 0 {
		checks = append(checks, GateCheck{
			Name:     "ingest_p95_regression_ratio",
			Actual:   comparison.IngestP95DeltaRatio,
			Expected: thresholds.MaxIngestP95RegressionRatio,
			Passed:   comparison.IngestP95DeltaRatio <= thresholds.MaxIngestP95RegressionRatio,
			Message:  "candidate ingest P95 regression ratio must stay within threshold",
		})
	}

	if thresholds.MaxIngestP95RegressionMS > 0 {
		checks = append(checks, GateCheck{
			Name:     "ingest_p95_regression_ms",
			Actual:   comparison.IngestP95DeltaMS,
			Expected: thresholds.MaxIngestP95RegressionMS,
			Passed:   comparison.IngestP95DeltaMS <= thresholds.MaxIngestP95RegressionMS,
			Message:  "candidate ingest P95 regression ms must stay within threshold",
		})
	}

	if thresholds.MaxAvgEmbeddingLengthDelta > 0 {
		checks = append(checks, GateCheck{
			Name:     "avg_embedding_text_length_delta",
			Actual:   comparison.AvgEmbeddingLengthDelta,
			Expected: thresholds.MaxAvgEmbeddingLengthDelta,
			Passed:   comparison.AvgEmbeddingLengthDelta <= thresholds.MaxAvgEmbeddingLengthDelta,
			Message:  "candidate average embedding text growth must stay within threshold",
		})
	}

	if thresholds.MaxP95EmbeddingLengthDelta > 0 {
		checks = append(checks, GateCheck{
			Name:     "p95_embedding_text_length_delta",
			Actual:   comparison.P95EmbeddingLengthDelta,
			Expected: thresholds.MaxP95EmbeddingLengthDelta,
			Passed:   comparison.P95EmbeddingLengthDelta <= thresholds.MaxP95EmbeddingLengthDelta,
			Message:  "candidate P95 embedding text growth must stay within threshold",
		})
	}

	if thresholds.MaxAvgChunksPerDocDelta > 0 {
		checks = append(checks, GateCheck{
			Name:     "avg_chunks_per_document_delta",
			Actual:   comparison.AvgChunksPerDocDelta,
			Expected: thresholds.MaxAvgChunksPerDocDelta,
			Passed:   comparison.AvgChunksPerDocDelta <= thresholds.MaxAvgChunksPerDocDelta,
			Message:  "candidate average chunks per document growth must stay within threshold",
		})
	}

	if thresholds.MaxP95ChunksPerDocDelta > 0 {
		checks = append(checks, GateCheck{
			Name:     "p95_chunks_per_document_delta",
			Actual:   comparison.P95ChunksPerDocDelta,
			Expected: thresholds.MaxP95ChunksPerDocDelta,
			Passed:   comparison.P95ChunksPerDocDelta <= thresholds.MaxP95ChunksPerDocDelta,
			Message:  "candidate P95 chunks per document growth must stay within threshold",
		})
	}

	passed := true
	for _, check := range checks {
		if !check.Passed {
			passed = false
			break
		}
	}
	return GateResult{
		Passed:     passed,
		Thresholds: thresholds,
		Checks:     checks,
	}
}

func (g GateResult) Error() error {
	if g.Passed {
		return nil
	}
	for _, check := range g.Checks {
		if !check.Passed {
			return fmt.Errorf("gate failed on %s: actual=%.4f expected=%.4f", check.Name, check.Actual, check.Expected)
		}
	}
	return fmt.Errorf("gate failed")
}
