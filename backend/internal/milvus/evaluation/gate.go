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
		MaxP95LatencyRegressionMS:    0,
		MaxP95LatencyRegressionRatio: 0.20,
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
