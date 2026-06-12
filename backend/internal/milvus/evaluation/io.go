package evaluation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func LoadDataset(path string) ([]DatasetCase, error) {
	bundle, err := LoadDatasetBundle(path)
	if err != nil {
		return nil, err
	}
	return bundle.Cases, nil
}

func LoadDatasetBundle(path string) (DatasetBundle, error) {
	var bundle DatasetBundle
	data, err := os.ReadFile(path)
	if err != nil {
		return bundle, err
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return bundle, fmt.Errorf("parse dataset %s: empty payload", path)
	}
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &bundle.Cases); err != nil {
			return bundle, fmt.Errorf("parse dataset %s: %w", path, err)
		}
		bundle.DatasetVersion = "legacy-array"
		return bundle, nil
	}
	if err := json.Unmarshal(trimmed, &bundle); err != nil {
		return bundle, fmt.Errorf("parse dataset %s: %w", path, err)
	}
	if bundle.DatasetVersion == "" {
		bundle.DatasetVersion = "unspecified"
	}
	return bundle, nil
}

func LoadProfiles(path string) ([]StrategyProfile, error) {
	bundle, err := LoadProfileBundle(path)
	if err != nil {
		return nil, err
	}
	return bundle.Profiles, nil
}

func LoadProfileBundle(path string) (ProfileBundle, error) {
	var bundle ProfileBundle
	data, err := os.ReadFile(path)
	if err != nil {
		return bundle, err
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return bundle, fmt.Errorf("parse profiles %s: empty payload", path)
	}
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &bundle.Profiles); err != nil {
			return bundle, fmt.Errorf("parse profiles %s: %w", path, err)
		}
		bundle.ProfileVersion = "legacy-array"
		return bundle, nil
	}
	if err := json.Unmarshal(trimmed, &bundle); err != nil {
		return bundle, fmt.Errorf("parse profiles %s: %w", path, err)
	}
	if bundle.ProfileVersion == "" {
		bundle.ProfileVersion = "unspecified"
	}
	return bundle, nil
}

func LoadGateThresholds(path string) (GateThresholds, error) {
	thresholds := DefaultGateThresholds()
	data, err := os.ReadFile(path)
	if err != nil {
		return thresholds, err
	}
	if err := json.Unmarshal(data, &thresholds); err != nil {
		return thresholds, fmt.Errorf("parse gate thresholds %s: %w", path, err)
	}
	return thresholds, nil
}

func SaveReportJSON(path string, report *Report) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func SaveReportMarkdown(path string, report *Report) error {
	return os.WriteFile(path, []byte(RenderMarkdownReport(report)), 0o644)
}

func RenderMarkdownReport(report *Report) string {
	var buf bytes.Buffer
	buf.WriteString("# L7 Retrieval Regression Report\n\n")
	buf.WriteString(fmt.Sprintf("- Generated At: `%s`\n", report.GeneratedAt.Format("2006-01-02 15:04:05")))
	buf.WriteString(fmt.Sprintf("- Dataset Size: `%d`\n", report.DatasetSize))
	if strings.TrimSpace(report.DatasetVersion) != "" {
		buf.WriteString(fmt.Sprintf("- Dataset Version: `%s`\n", report.DatasetVersion))
	}
	if strings.TrimSpace(report.ProfileVersion) != "" {
		buf.WriteString(fmt.Sprintf("- Profile Version: `%s`\n", report.ProfileVersion))
	}
	buf.WriteString(fmt.Sprintf("- Baseline: `%s`\n", report.Baseline))
	buf.WriteString(fmt.Sprintf("- Candidate: `%s`\n\n", report.Candidate))
	if strings.TrimSpace(report.FusionStrategy) != "" {
		buf.WriteString(fmt.Sprintf("- Report Fusion Strategy: `%s`\n\n", report.FusionStrategy))
	}

	buf.WriteString("## Metrics\n\n")
	buf.WriteString("| Strategy | Fusion | Recall@K | MRR | nDCG | Citation Acc | Chunk Purity | Self-contained | Parent Fill | Context Gain | P50(ms) | P95(ms) |\n")
	buf.WriteString("| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, result := range report.Results {
		buf.WriteString(fmt.Sprintf(
			"| %s | %s | %.4f | %.4f | %.4f | %.4f | %.4f | %.4f | %.4f | %.4f | %.2f | %.2f |\n",
			result.Strategy.Name,
			formatFusionStrategy(result.Strategy),
			result.Metrics.RecallAtK,
			result.Metrics.MRR,
			result.Metrics.NDCG,
			result.Metrics.CitationAccuracy,
			result.Metrics.ChunkPurity,
			result.Metrics.ChunkSelfContainedRate,
			result.Metrics.ParentFillGain,
			result.Metrics.ContextualRecallGain,
			result.Metrics.P50LatencyMS,
			result.Metrics.P95LatencyMS,
		))
	}

	buf.WriteString("\n## Chunking Stats\n\n")
	buf.WriteString("| Strategy | Avg Embedding Len | P95 Embedding Len | Avg Chunks/Doc | P95 Chunks/Doc | Ingest P95(ms) |\n")
	buf.WriteString("| --- | ---: | ---: | ---: | ---: | ---: |\n")
	for _, result := range report.Results {
		buf.WriteString(fmt.Sprintf(
			"| %s | %.2f | %.2f | %.2f | %.2f | %.2f |\n",
			result.Strategy.Name,
			result.Metrics.AvgEmbeddingTextLength,
			result.Metrics.P95EmbeddingTextLength,
			result.Metrics.AvgChunksPerDocument,
			result.Metrics.P95ChunksPerDocument,
			result.Metrics.IngestP95MS,
		))
	}

	buf.WriteString("\n## Baseline vs Candidate\n\n")
	if strings.TrimSpace(report.Comparison.BaselineFusionStrategy) != "" || strings.TrimSpace(report.Comparison.CandidateFusionStrategy) != "" {
		buf.WriteString(fmt.Sprintf("- Fusion Strategy: `%s` -> `%s`\n", report.Comparison.BaselineFusionStrategy, report.Comparison.CandidateFusionStrategy))
	}
	buf.WriteString(fmt.Sprintf("- Recall@K Delta: `%.4f`\n", report.Comparison.RecallDelta))
	buf.WriteString(fmt.Sprintf("- MRR Delta: `%.4f`\n", report.Comparison.MRRDelta))
	buf.WriteString(fmt.Sprintf("- nDCG Delta: `%.4f`\n", report.Comparison.NDCGDelta))
	buf.WriteString(fmt.Sprintf("- Citation Accuracy Delta: `%.4f`\n", report.Comparison.CitationAccuracyDelta))
	buf.WriteString(fmt.Sprintf("- Citation Precision Delta: `%.4f`\n", report.Comparison.CitationPrecisionDelta))
	buf.WriteString(fmt.Sprintf("- Citation Recall Delta: `%.4f`\n", report.Comparison.CitationRecallDelta))
	buf.WriteString(fmt.Sprintf("- Parent Fill Gain Delta: `%.4f`\n", report.Comparison.ParentFillGainDelta))
	buf.WriteString(fmt.Sprintf("- Contextual Recall Gain Delta: `%.4f`\n", report.Comparison.ContextualRecallGainDelta))
	buf.WriteString(fmt.Sprintf("- Chunk Purity Delta: `%.4f`\n", report.Comparison.ChunkPurityDelta))
	buf.WriteString(fmt.Sprintf("- Chunk Self-contained Rate Delta: `%.4f`\n", report.Comparison.ChunkSelfContainedDelta))
	buf.WriteString(fmt.Sprintf("- Rewrite Gain Delta: `%.4f`\n", report.Comparison.RewriteGainDelta))
	buf.WriteString(fmt.Sprintf("- Dense Hit Rate Delta: `%.4f`\n", report.Comparison.DenseHitRateDelta))
	buf.WriteString(fmt.Sprintf("- Sparse Hit Rate Delta: `%.4f`\n", report.Comparison.SparseHitRateDelta))
	buf.WriteString(fmt.Sprintf("- Dense Participation Delta: `%.4f`\n", report.Comparison.DenseParticipationRateDelta))
	buf.WriteString(fmt.Sprintf("- Sparse Participation Delta: `%.4f`\n", report.Comparison.SparseParticipationRateDelta))
	buf.WriteString(fmt.Sprintf("- Primary Dense Rate Delta: `%.4f`\n", report.Comparison.PrimaryDenseRateDelta))
	buf.WriteString(fmt.Sprintf("- Primary Sparse Rate Delta: `%.4f`\n", report.Comparison.PrimarySparseRateDelta))
	buf.WriteString(fmt.Sprintf("- Empty Rate Delta: `%.4f`\n", report.Comparison.EmptyRateDelta))
	buf.WriteString(fmt.Sprintf("- Dense Route Contribution Delta: `%.4f`\n", report.Comparison.DenseRouteContributionDelta))
	buf.WriteString(fmt.Sprintf("- Sparse Route Contribution Delta: `%.4f`\n", report.Comparison.SparseRouteContributionDelta))
	buf.WriteString(fmt.Sprintf("- Refusal False Positive Rate: `%.4f`\n", report.Comparison.RefusalFalsePositiveRate))
	buf.WriteString(fmt.Sprintf("- Ingest P95 Delta: `%.2f ms` (`%.2f%%`)\n", report.Comparison.IngestP95DeltaMS, report.Comparison.IngestP95DeltaRatio*100))
	buf.WriteString(fmt.Sprintf("- Avg Embedding Text Length Delta: `%.2f`\n", report.Comparison.AvgEmbeddingLengthDelta))
	buf.WriteString(fmt.Sprintf("- P95 Embedding Text Length Delta: `%.2f`\n", report.Comparison.P95EmbeddingLengthDelta))
	buf.WriteString(fmt.Sprintf("- Avg Chunks Per Document Delta: `%.2f`\n", report.Comparison.AvgChunksPerDocDelta))
	buf.WriteString(fmt.Sprintf("- P95 Chunks Per Document Delta: `%.2f`\n", report.Comparison.P95ChunksPerDocDelta))
	buf.WriteString(fmt.Sprintf("- P95 Latency Delta: `%.2f ms` (`%.2f%%`)\n\n", report.Comparison.P95LatencyDeltaMS, report.Comparison.P95LatencyDeltaRatio*100))

	buf.WriteString("## Contribution\n\n")
	if len(report.Contribution) == 0 {
		buf.WriteString("- No contribution chain generated.\n")
	} else {
		for _, delta := range report.Contribution {
			buf.WriteString(fmt.Sprintf(
				"- `%s` (%s) vs `%s` (%s): Recall `%.4f`, MRR `%.4f`, nDCG `%.4f`, Citation Acc `%.4f`, Chunk Purity `%.4f`, Self-contained `%.4f`, Dense/Sparse Hit `%.4f / %.4f`, Dense/Sparse Part. `%.4f / %.4f`, Primary Sparse `%.4f`, Empty `%.4f`, Ingest P95 `%.2f ms`, Query P95 `%.2f ms`\n",
				delta.Strategy,
				blankAsNA(delta.FusionStrategy),
				delta.ComparedTo,
				blankAsNA(delta.ComparedToFusionStrategy),
				delta.RecallDelta,
				delta.MRRDelta,
				delta.NDCGDelta,
				delta.CitationAccuracyDelta,
				delta.ChunkPurityDelta,
				delta.ChunkSelfContainedDelta,
				delta.DenseHitRateDelta,
				delta.SparseHitRateDelta,
				delta.DenseParticipationDelta,
				delta.SparseParticipationDelta,
				delta.PrimarySparseRateDelta,
				delta.EmptyRateDelta,
				delta.IngestP95DeltaMS,
				delta.P95LatencyDeltaMS,
			))
		}
	}

	buf.WriteString("\n## Gate\n\n")
	buf.WriteString(fmt.Sprintf("- Passed: `%t`\n", report.Gate.Passed))
	for _, check := range report.Gate.Checks {
		status := "FAIL"
		if check.Passed {
			status = "PASS"
		}
		buf.WriteString(fmt.Sprintf("- [%s] %s: actual=`%.4f` expected=`%.4f` %s\n", status, check.Name, check.Actual, check.Expected, strings.TrimSpace(check.Message)))
	}
	return buf.String()
}

func formatFusionStrategy(profile StrategyProfile) string {
	fusion := blankAsNA(profile.FusionStrategy)
	if profile.RRFK > 0 {
		return fmt.Sprintf("%s(k=%d)", fusion, profile.RRFK)
	}
	return fusion
}

func blankAsNA(value string) string {
	if strings.TrimSpace(value) == "" {
		return "n/a"
	}
	return strings.TrimSpace(value)
}
