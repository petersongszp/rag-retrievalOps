package canonical_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"interview-agents/internal/documentparser"
	"interview-agents/internal/documentparser/canonical"
)

func TestNormalizeConvergesRocketMQVariants(t *testing.T) {
	variants := map[string]string{
		"markdown": "## **1.3 消息收发TPS计算规则**\n\n• 消息大小以4 KB为最小计量单位，不足4 KB按照4 KB计算。例如，消息收发TPS为（16/4）×（5000+5000）=40000次/秒。",
		"html":     "## 1.3 消息收发 TPS 计算规则\n\n• 消息大小以4 KB为最小计量单位，不足4 KB按照4 KB计算。例如，消息收发TPS为（16/4）×（5000+5000）=40000次/秒。",
		"pdf":      "## 1.3 TPS\n\n## 消息收发 计算规则\n\n- 消息大小以 4 KB 为最小计量单位，不足 4 KB 按照 4 KB 计算。例如，消息收发 TPS 为（ 16/4 ） × （ 5000+5000 ） =40000 次 / 秒。",
		"ocr":      "## 1.3消息收发TPS计算规则\n\n- ·消息大小以4KB为最小计量单位，不足4KB按照4KB计算。例如，消息收发TPS为 （16/4）×（5000+5000）=40000次/秒。每秒钟发送10条事务消息，则消息发送TPS为10x5=50次/ 秒。",
	}

	for name, input := range variants {
		t.Run(name, func(t *testing.T) {
			doc := normalizedDoc(name, input)
			got, err := canonical.Normalize(doc)
			if err != nil {
				t.Fatalf("Normalize returned error: %v", err)
			}

			if !hasHeadingWithAll(got.ContentMarkdown, "1.3", "消息收发", "TPS", "计算规则") {
				t.Fatalf("canonical markdown missing equivalent 1.3 TPS heading:\n%s", got.ContentMarkdown)
			}
			if got.ContentMarkdownRaw != input {
				t.Fatalf("ContentMarkdownRaw = %q, want original input", got.ContentMarkdownRaw)
			}
			if got.Canonicalization.Version == "" {
				t.Fatalf("canonicalization version should be recorded")
			}
			if got.Canonicalization.RawSHA1 == "" || got.Canonicalization.CanonicalSHA1 == "" {
				t.Fatalf("canonicalization hashes should be recorded: %+v", got.Canonicalization)
			}
		})
	}
}

func TestNormalizeMergesHeadingContinuationButNotNumberedSibling(t *testing.T) {
	doc := normalizedDoc("pdf", "## 云消息队列 RocketMQ 版消息收发计算规格\n\n## 计费说明\n\n正文\n\n## 1. 计算规格说明\n\n## 1.1 计算规格能力约束\n\n内容")
	got, err := canonical.Normalize(doc)
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}

	if !strings.Contains(got.ContentMarkdown, "## 云消息队列 RocketMQ 版消息收发计算规格计费说明") {
		t.Fatalf("expected long heading continuation to merge, got:\n%s", got.ContentMarkdown)
	}
	if strings.Contains(got.ContentMarkdown, "## 1. 计算规格说明1.1 计算规格能力约束") {
		t.Fatalf("numbered sibling headings should not merge:\n%s", got.ContentMarkdown)
	}
	if !strings.Contains(got.ContentMarkdown, "## 1. 计算规格说明\n\n## 1.1 计算规格能力约束") {
		t.Fatalf("expected numbered sibling headings to stay separate, got:\n%s", got.ContentMarkdown)
	}
}

func TestNormalizeDoesNotRewriteDomainTermsOrUnits(t *testing.T) {
	doc := normalizedDoc("md", "## 2.1 CPU 使用率\n\n缓存小于128MB按照128MB计算，延迟低于20ms。RocketMQ 版保持产品名空格。")
	got, err := canonical.Normalize(doc)
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}

	required := []string{
		"## 2.1 CPU 使用率",
		"128MB",
		"20ms",
		"RocketMQ 版",
	}
	for _, term := range required {
		if !strings.Contains(got.ContentMarkdown, term) {
			t.Fatalf("canonical markdown missing %q:\n%s", term, got.ContentMarkdown)
		}
	}
	if strings.Contains(got.ContentMarkdown, "RocketMQ版") {
		t.Fatalf("camel-case product names should not be glued to following CJK text:\n%s", got.ContentMarkdown)
	}
	if strings.Contains(got.ContentMarkdown, "128 MB") || strings.Contains(got.ContentMarkdown, "20 ms") {
		t.Fatalf("canonicalizer should not rewrite unit lexemes:\n%s", got.ContentMarkdown)
	}
}

func TestNormalizeMergesNumberedHeadingWithGenericAcronymFragment(t *testing.T) {
	doc := normalizedDoc("pdf", "## 2.7 CPU\n\n## 使用率告警规则\n\n正文")
	got, err := canonical.Normalize(doc)
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}

	if !strings.Contains(got.ContentMarkdown, "## 2.7 CPU使用率告警规则") {
		t.Fatalf("expected acronym heading fragment to merge before continuation title, got:\n%s", got.ContentMarkdown)
	}
}

func TestNormalizeFormulaSpacingDoesNotCorruptMarkdownEmphasis(t *testing.T) {
	doc := normalizedDoc("md", "- **重要提示**: 保留强调格式\n\n公式为（ 16 / 4 ） × （ 5000 + 5000 ） = 40000 次 / 秒。")
	got, err := canonical.Normalize(doc)
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}

	if !strings.Contains(got.ContentMarkdown, "- **重要提示**") {
		t.Fatalf("markdown emphasis should not be treated as a formula operator, got:\n%s", got.ContentMarkdown)
	}
	if !strings.Contains(got.ContentMarkdown, "公式为( 16 / 4 ) × ( 5000 + 5000 ) = 40000 次 / 秒") {
		t.Fatalf("formula lexemes should be preserved by structural canonicalization, got:\n%s", got.ContentMarkdown)
	}
}

func TestNormalizeRebasesTableSpansAfterUnicodeNormalization(t *testing.T) {
	input := strings.TrimSpace(`
说明（小时）

| 项目 | 按量付费 |
| --- | --- |
| 计费公式 | 服务时长（小时） |
`)
	start := strings.Index(input, "| 项目 |")
	if start < 0 {
		t.Fatal("test table start not found")
	}
	doc := normalizedDoc("docx", input)
	doc.Tables = []documentparser.NormalizedTable{
		{
			ID:            "table-001",
			MarkdownStart: start,
			MarkdownEnd:   len(input),
			Rows: []documentparser.TableRow{
				{Cells: []documentparser.TableCell{{Text: "项目", IsHeader: true}, {Text: "按量付费", IsHeader: true}}},
				{Cells: []documentparser.TableCell{{Text: "计费公式"}, {Text: "服务时长（小时）"}}},
			},
			Quality: documentparser.TableQuality{Status: "ok"},
		},
	}

	got, err := canonical.Normalize(doc)
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}
	if len(got.Tables) != 1 {
		t.Fatalf("expected one table, got %d", len(got.Tables))
	}
	table := got.Tables[0]
	if table.MarkdownStart < 0 || table.MarkdownEnd > len(got.ContentMarkdown) || table.MarkdownStart >= table.MarkdownEnd {
		t.Fatalf("invalid table span after canonicalization: %d-%d len=%d", table.MarkdownStart, table.MarkdownEnd, len(got.ContentMarkdown))
	}
	tableMarkdown := got.ContentMarkdown[table.MarkdownStart:table.MarkdownEnd]
	if !utf8.ValidString(tableMarkdown) {
		t.Fatalf("table span must not split UTF-8 runes: %q", tableMarkdown)
	}
	if !strings.HasPrefix(tableMarkdown, "| 项目 | 按量付费 |") {
		t.Fatalf("table span should be rebased to the canonical table, got %q", tableMarkdown)
	}
}

func TestNormalizeIsIdempotent(t *testing.T) {
	input := "## 1.3 TPS\n\n## 消息收发 计算规则\n\n- 消息大小以4KB为最小计量单位，不足4KB按照4KB计算。"
	first, err := canonical.Normalize(normalizedDoc("pdf", input))
	if err != nil {
		t.Fatalf("first Normalize returned error: %v", err)
	}
	second, err := canonical.Normalize(first)
	if err != nil {
		t.Fatalf("second Normalize returned error: %v", err)
	}

	if second.ContentMarkdown != first.ContentMarkdown {
		t.Fatalf("Normalize should be idempotent\nfirst:\n%s\nsecond:\n%s", first.ContentMarkdown, second.ContentMarkdown)
	}
	if second.ContentMarkdownRaw != input {
		t.Fatalf("second raw content should preserve original raw parser output")
	}
	if second.Canonicalization.CanonicalSHA1 != first.Canonicalization.CanonicalSHA1 {
		t.Fatalf("canonical hash changed across idempotent normalization")
	}
}

func normalizedDoc(fileType, content string) *documentparser.NormalizedDocument {
	return &documentparser.NormalizedDocument{
		ContentMarkdown: content,
		Source: documentparser.NormalizedSource{
			FileName: "sample." + fileType,
			FileType: fileType,
		},
		Quality: documentparser.ParseQuality{Status: "ok", Score: 1},
		Extractor: documentparser.ExtractorInfo{
			Provider: "test",
			Version:  "v1",
		},
	}
}

func hasHeadingWithAll(markdown string, terms ...string) bool {
	for _, line := range strings.Split(markdown, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		matched := true
		for _, term := range terms {
			if !strings.Contains(line, term) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}
