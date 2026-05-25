package benchmark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func LoadDataset(path string) ([]QueryCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var dataset []QueryCase
	if err := json.Unmarshal(data, &dataset); err != nil {
		return nil, fmt.Errorf("parse dataset %s: %w", path, err)
	}
	return dataset, nil
}

func LoadProfiles(path string) ([]IndexProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var profiles []IndexProfile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return nil, fmt.Errorf("parse profiles %s: %w", path, err)
	}
	return profiles, nil
}

func SaveReportJSON(path string, report *Report) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func RenderMarkdownReport(report *Report) string {
	var buf bytes.Buffer
	buf.WriteString("# L6 索引参数优化基准报告\n\n")
	buf.WriteString(fmt.Sprintf("- 生成时间: `%s`\n", report.GeneratedAt.Format("2006-01-02 15:04:05")))
	buf.WriteString(fmt.Sprintf("- 评测样本数: `%d`\n", report.DatasetSize))
	buf.WriteString(fmt.Sprintf("- 扫描 profile: `%s`\n\n", strings.Join(report.ProfilesScanned, "`, `")))

	buf.WriteString("## 参数对比\n\n")
	buf.WriteString("| Profile | Family | Recall@K | MRR | nDCG | P50(ms) | P95(ms) | CPU User(ms) | HeapAlloc(MB) |\n")
	buf.WriteString("| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, result := range report.Results {
		buf.WriteString(fmt.Sprintf(
			"| %s | %s | %.4f | %.4f | %.4f | %.2f | %.2f | %d | %.2f |\n",
			result.Profile.Name,
			result.Profile.Family,
			result.Metrics.RecallAtK,
			result.Metrics.MRR,
			result.Metrics.NDCG,
			result.Metrics.P50LatencyMS,
			result.Metrics.P95LatencyMS,
			result.Metrics.Resources.ProcessCPUUserMS,
			result.Metrics.Resources.HeapAllocMB,
		))
	}

	buf.WriteString("\n## 推荐参数\n\n")
	buf.WriteString(fmt.Sprintf("- Baseline: `%s`\n", report.Recommendation.BaselineProfile))
	buf.WriteString(fmt.Sprintf("- Recommended: `%s`\n", report.Recommendation.RecommendedProfile))
	for _, reason := range report.Recommendation.Reasons {
		buf.WriteString(fmt.Sprintf("- %s\n", reason))
	}

	buf.WriteString("\n## 风险说明\n\n")
	if len(report.Recommendation.Risks) == 0 {
		buf.WriteString("- 当前离线结果未发现额外风险，但仍建议在灰度流量下验证。\n")
	} else {
		for _, risk := range report.Recommendation.Risks {
			buf.WriteString(fmt.Sprintf("- %s\n", risk))
		}
	}

	buf.WriteString("\n## 回滚清单\n\n")
	for idx, step := range report.Recommendation.RollbackSteps {
		buf.WriteString(fmt.Sprintf("%d. %s\n", idx+1, step))
	}
	return buf.String()
}

func SaveReportMarkdown(path string, report *Report) error {
	return os.WriteFile(path, []byte(RenderMarkdownReport(report)), 0o644)
}
