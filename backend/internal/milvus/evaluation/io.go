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
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var profiles []StrategyProfile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return nil, fmt.Errorf("parse profiles %s: %w", path, err)
	}
	return profiles, nil
}

func LoadGateThresholds(path string) (GateThresholds, error) {
	var thresholds GateThresholds
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
	buf.WriteString("# L7 检索离线回归报告\n\n")
	buf.WriteString(fmt.Sprintf("- 生成时间: `%s`\n", report.GeneratedAt.Format("2006-01-02 15:04:05")))
	buf.WriteString(fmt.Sprintf("- 评测样本数: `%d`\n", report.DatasetSize))
	if strings.TrimSpace(report.DatasetVersion) != "" {
		buf.WriteString(fmt.Sprintf("- Dataset Version: `%s`\n", report.DatasetVersion))
	}
	buf.WriteString(fmt.Sprintf("- Baseline: `%s`\n", report.Baseline))
	buf.WriteString(fmt.Sprintf("- Candidate: `%s`\n\n", report.Candidate))

	buf.WriteString("## 指标对比\n\n")
	buf.WriteString("| Strategy | Recall@K | MRR | nDCG | Citation Accuracy | P50(ms) | P95(ms) |\n")
	buf.WriteString("| --- | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, result := range report.Results {
		buf.WriteString(fmt.Sprintf(
			"| %s | %.4f | %.4f | %.4f | %.4f | %.2f | %.2f |\n",
			result.Strategy.Name,
			result.Metrics.RecallAtK,
			result.Metrics.MRR,
			result.Metrics.NDCG,
			result.Metrics.CitationAccuracy,
			result.Metrics.P50LatencyMS,
			result.Metrics.P95LatencyMS,
		))
	}

	buf.WriteString("\n## Baseline vs Candidate\n\n")
	buf.WriteString(fmt.Sprintf("- Recall@K Delta: `%.4f`\n", report.Comparison.RecallDelta))
	buf.WriteString(fmt.Sprintf("- MRR Delta: `%.4f`\n", report.Comparison.MRRDelta))
	buf.WriteString(fmt.Sprintf("- nDCG Delta: `%.4f`\n", report.Comparison.NDCGDelta))
	buf.WriteString(fmt.Sprintf("- Citation Accuracy Delta: `%.4f`\n", report.Comparison.CitationAccuracyDelta))
	buf.WriteString(fmt.Sprintf("- P95 Latency Delta: `%.2f ms` (`%.2f%%`)\n\n", report.Comparison.P95LatencyDeltaMS, report.Comparison.P95LatencyDeltaRatio*100))

	buf.WriteString("## 策略贡献分析\n\n")
	if len(report.Contribution) == 0 {
		buf.WriteString("- 未生成贡献度链路，请检查 profile 配置顺序。\n")
	} else {
		for _, delta := range report.Contribution {
			buf.WriteString(fmt.Sprintf(
				"- `%s` vs `%s`: Recall `%.4f`, MRR `%.4f`, nDCG `%.4f`, Citation `%.4f`, P95 `%.2f ms`\n",
				delta.Strategy, delta.ComparedTo, delta.RecallDelta, delta.MRRDelta, delta.NDCGDelta, delta.CitationAccuracyDelta, delta.P95LatencyDeltaMS,
			))
		}
	}

	buf.WriteString("\n## 门禁结果\n\n")
	buf.WriteString(fmt.Sprintf("- 总体结果: `%t`\n", report.Gate.Passed))
	for _, check := range report.Gate.Checks {
		status := "FAIL"
		if check.Passed {
			status = "PASS"
		}
		buf.WriteString(fmt.Sprintf("- [%s] %s: actual=`%.4f` expected=`%.4f` %s\n", status, check.Name, check.Actual, check.Expected, strings.TrimSpace(check.Message)))
	}
	return buf.String()
}
