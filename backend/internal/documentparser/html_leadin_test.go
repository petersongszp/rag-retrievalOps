package documentparser

import (
	"strings"
	"testing"
)

// TestRestoreHTMLLeadInSkipsNonHTMLFileTypes 验证非 HTML 文件类型直接返回原始 markdown，
// 不触发 HTML 解析逻辑。这是数据校验的守卫路径，必须保持稳定。
func TestRestoreHTMLLeadInSkipsNonHTMLFileTypes(t *testing.T) {
	cases := map[string]string{
		"pdf":  "pdf",
		"docx": "docx",
		"txt":  "txt",
		"md":   "md",
		"":     "",
	}
	for fileType, label := range cases {
		t.Run(label, func(t *testing.T) {
			markdown := "# 标题\n\n正文"
			// 即使传入可解析的 HTML 字节，非 HTML 类型也不应改动 markdown
			got := RestoreHTMLLeadInBeforeFirstHeading(fileType, []byte("<p>不应出现</p>"), markdown)
			if got != markdown {
				t.Fatalf("非 HTML 类型 %q 应原样返回 markdown，got=%q", fileType, got)
			}
		})
	}
}

// TestRestoreHTMLLeadInEmptySourceReturnsMarkdown 验证空 HTML 源不会产生前缀，
// 直接返回原始 markdown。覆盖边界条件。
func TestRestoreHTMLLeadInEmptySourceReturnsMarkdown(t *testing.T) {
	markdown := "# 标题\n\n正文"
	for _, source := range [][]byte{nil, {}, []byte("   \n\t "), []byte("<html></html>")} {
		got := RestoreHTMLLeadInBeforeFirstHeading("html", source, markdown)
		if got != markdown {
			t.Fatalf("空 HTML 源应原样返回 markdown，source=%q got=%q", source, got)
		}
	}
}

// TestRestoreHTMLLeadInHeadingFirstReturnsMarkdown 验证当 HTML 第一个元素即为标题时，
// 没有前导段落可恢复，应原样返回 markdown。
func TestRestoreHTMLLeadInHeadingFirstReturnsMarkdown(t *testing.T) {
	source := []byte(`<html><body><h1>首个标题</h1><p>正文段落</p></body></html>`)
	markdown := "# 首个标题\n\n正文段落"
	got := RestoreHTMLLeadInBeforeFirstHeading("html", source, markdown)
	if got != markdown {
		t.Fatalf("首个元素为标题时不应添加前缀，got=%q", got)
	}
}

// TestRestoreHTMLLeadInPrependsMissingParagraphs 验证核心路径：HTML 中位于首个标题之前的
// 段落若未出现在 markdown 中，应被前置到 markdown 开头，顺序与 HTML 一致。
func TestRestoreHTMLLeadInPrependsMissingParagraphs(t *testing.T) {
	source := []byte(`<html><body>` +
		`<p>第一段前导说明</p>` +
		`<p>第二段前导说明</p>` +
		`<h1>1. 章节标题</h1>` +
		`<p>章节正文</p>` +
		`</body></html>`)
	markdown := "# 1. 章节标题\n\n章节正文"
	got := RestoreHTMLLeadInBeforeFirstHeading("html", source, markdown)

	if !strings.HasPrefix(got, "第一段前导说明") {
		t.Fatalf("应以第一段前导开头，got=%q", got)
	}
	if !strings.Contains(got, "第二段前导说明") {
		t.Fatalf("应包含第二段前导，got=%q", got)
	}
	// 前导应出现在原标题之前
	firstLeadIdx := strings.Index(got, "第一段前导说明")
	headingIdx := strings.Index(got, "# 1. 章节标题")
	if firstLeadIdx < 0 || headingIdx < 0 || firstLeadIdx > headingIdx {
		t.Fatalf("前导应位于标题之前，firstLeadIdx=%d headingIdx=%d", firstLeadIdx, headingIdx)
	}
}

// TestRestoreHTMLLeadInDoesNotDuplicateExistingText 验证当 markdown 已包含前导段落文本时，
// 不会重复添加。这是去重逻辑的关键边界条件。
func TestRestoreHTMLLeadInDoesNotDuplicateExistingText(t *testing.T) {
	source := []byte(`<html><body>` +
		`<p>已存在的导语</p>` +
		`<p>缺失的导语</p>` +
		`<h1>标题</h1>` +
		`</body></html>`)
	// markdown 中已包含"已存在的导语"，但缺少"缺失的导语"
	markdown := "已存在的导语\n\n# 标题\n\n正文"
	got := RestoreHTMLLeadInBeforeFirstHeading("html", source, markdown)

	// "已存在的导语"不应被重复添加
	if strings.Count(got, "已存在的导语") != 1 {
		t.Fatalf("已存在的文本不应被重复添加，出现次数=%d\ngot=%q", strings.Count(got, "已存在的导语"), got)
	}
	// "缺失的导语"应被前置
	if !strings.Contains(got, "缺失的导语") {
		t.Fatalf("缺失的导语应被补充，got=%q", got)
	}
	// 原始 markdown 内容应保留
	if !strings.Contains(got, "# 标题") {
		t.Fatalf("原始标题应保留，got=%q", got)
	}
}

// TestRestoreHTMLLeadInStrongTagProducesBoldMarkdown 验证 <strong> 标签会被识别为强调信号，
// 在恢复的段落两侧添加 ** 标记。
func TestRestoreHTMLLeadInStrongTagProducesBoldMarkdown(t *testing.T) {
	source := []byte(`<html><body>` +
		`<p><strong>重要提示</strong></p>` +
		`<h1>正文标题</h1>` +
		`</body></html>`)
	markdown := "# 正文标题\n\n正文"
	got := RestoreHTMLLeadInBeforeFirstHeading("html", source, markdown)
	if !strings.Contains(got, "**重要提示**") {
		t.Fatalf("<strong> 应产生 ** 包裹的强调文本，got=%q", got)
	}
}

// TestRestoreHTMLLeadInBoldTagProducesBoldMarkdown 验证 <b> 标签同样被识别为强调信号。
func TestRestoreHTMLLeadInBoldTagProducesBoldMarkdown(t *testing.T) {
	source := []byte(`<html><body>` +
		`<p><b>加粗内容</b></p>` +
		`<h1>标题</h1>` +
		`</body></html>`)
	markdown := "# 标题"
	got := RestoreHTMLLeadInBeforeFirstHeading("html", source, markdown)
	if !strings.Contains(got, "**加粗内容**") {
		t.Fatalf("<b> 应产生 ** 包裹的强调文本，got=%q", got)
	}
}

// TestRestoreHTMLLeadInInlineBoldStyleProducesBoldMarkdown 验证内联 style 中的
// font-weight:bold/700/800/900 会被识别为强调信号。这是 CSS 解析逻辑的边界条件。
func TestRestoreHTMLLeadInInlineBoldStyleProducesBoldMarkdown(t *testing.T) {
	weights := []string{"bold", "700", "800", "900"}
	for _, weight := range weights {
		t.Run(weight, func(t *testing.T) {
			source := []byte(`<html><body>` +
				`<p><span style="font-weight:` + weight + `">样式加粗</span></p>` +
				`<h1>标题</h1>` +
				`</body></html>`)
			markdown := "# 标题"
			got := RestoreHTMLLeadInBeforeFirstHeading("html", source, markdown)
			if !strings.Contains(got, "**样式加粗**") {
				t.Fatalf("font-weight:%s 应产生强调文本，got=%q", weight, got)
			}
		})
	}
}

// TestRestoreHTMLLeadInNonBoldStyleDoesNotProduceBold 验证非粗体的 font-weight 值
// 不会误判为强调，避免假阳性。
func TestRestoreHTMLLeadInNonBoldStyleDoesNotProduceBold(t *testing.T) {
	source := []byte(`<html><body>` +
		`<p><span style="font-weight:normal">普通文本</span></p>` +
		`<h1>标题</h1>` +
		`</body></html>`)
	markdown := "# 标题"
	got := RestoreHTMLLeadInBeforeFirstHeading("html", source, markdown)
	if strings.Contains(got, "**普通文本**") {
		t.Fatalf("font-weight:normal 不应产生强调文本，got=%q", got)
	}
	if !strings.Contains(got, "普通文本") {
		t.Fatalf("普通文本应被恢复（无强调标记），got=%q", got)
	}
}

// TestRestoreHTMLLeadInSkipsIgnoredContainers 验证 script/style/noscript/template
// 容器内的文本不会被当作前导段落恢复。这是解析逻辑中的数据校验边界。
func TestRestoreHTMLLeadInSkipsIgnoredContainers(t *testing.T) {
	source := []byte(`<html><body>` +
		`<script>var x = "脚本内容";</script>` +
		`<style>p { color: red; }</style>` +
		`<noscript>请启用 JavaScript</noscript>` +
		`<template>模板内容</template>` +
		`<p>真实前导段落</p>` +
		`<h1>标题</h1>` +
		`</body></html>`)
	markdown := "# 标题"
	got := RestoreHTMLLeadInBeforeFirstHeading("html", source, markdown)
	if strings.Contains(got, "脚本内容") {
		t.Fatalf("script 内容不应被恢复，got=%q", got)
	}
	if strings.Contains(got, "color: red") {
		t.Fatalf("style 内容不应被恢复，got=%q", got)
	}
	if strings.Contains(got, "请启用 JavaScript") {
		t.Fatalf("noscript 内容不应被恢复，got=%q", got)
	}
	if strings.Contains(got, "模板内容") {
		t.Fatalf("template 内容不应被恢复，got=%q", got)
	}
	if !strings.Contains(got, "真实前导段落") {
		t.Fatalf("真实前导段落应被恢复，got=%q", got)
	}
}

// TestRestoreHTMLLeadInStopsAtAnyHeadingLevel 验证 h1-h6 任一级别的标题都会停止前导收集。
func TestRestoreHTMLLeadInStopsAtAnyHeadingLevel(t *testing.T) {
	levels := []string{"h1", "h2", "h3", "h4", "h5", "h6"}
	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			source := []byte(`<html><body>` +
				`<p>前导段落</p>` +
				`<` + level + `>标题</` + level + `>` +
				`<p>标题后段落</p>` +
				`</body></html>`)
			markdown := "<" + level + ">标题</" + level + ">"
			// 将标题转为 markdown 形式以模拟解析器输出
			markdown = "# 标题"
			got := RestoreHTMLLeadInBeforeFirstHeading("html", source, markdown)
			if !strings.Contains(got, "前导段落") {
				t.Fatalf("%s 前应收集前导段落，got=%q", level, got)
			}
			if strings.Contains(got, "标题后段落") {
				t.Fatalf("%s 后的段落不应被收集为前导，got=%q", level, got)
			}
		})
	}
}

// TestRestoreHTMLLeadInEmptyMarkdownReturnsPrefix 验证当 markdown 为空时，
// 仅返回前导段落，不产生多余的分隔符。
func TestRestoreHTMLLeadInEmptyMarkdownReturnsPrefix(t *testing.T) {
	source := []byte(`<html><body>` +
		`<p>前导段落</p>` +
		`<h1>标题</h1>` +
		`</body></html>`)
	got := RestoreHTMLLeadInBeforeFirstHeading("html", source, "")
	if got != "前导段落" {
		t.Fatalf("空 markdown 应仅返回前导段落，got=%q", got)
	}
}

// TestRestoreHTMLLeadInNormalizesNBSP 验证不间断空格（\u00a0）会被规范化为普通空格，
// 不会破坏文本匹配。这是文本规范化逻辑的边界条件。
func TestRestoreHTMLLeadInNormalizesNBSP(t *testing.T) {
	// 使用 NBSP 分隔的文本，验证规范化后能与 markdown 匹配
	source := []byte(`<html><body>` +
		`<p>包含\u00a0不间断空格的段落</p>` +
		`<h1>标题</h1>` +
		`</body></html>`)
	// 注意：上面字符串中的 \u00a0 是字面量，需要替换为真实 NBSP
	source = []byte(strings.ReplaceAll(string(source), `\u00a0`, "\u00a0"))
	markdown := "# 标题"
	got := RestoreHTMLLeadInBeforeFirstHeading("html", source, markdown)
	// NBSP 应被规范化为普通空格，文本仍应被恢复
	if !strings.Contains(got, "包含 不间断空格的段落") {
		t.Fatalf("NBSP 应被规范化为普通空格，got=%q", got)
	}
}

// TestRestoreHTMLLeadInStopsAtByteLimit 验证当累计前导字节数达到上限时停止收集，
// 避免超大 HTML 导致过长的前导恢复。这是边界条件与安全防护。
func TestRestoreHTMLLeadInStopsAtByteLimit(t *testing.T) {
	// 构造超过 maxHTMLLeadInBytes (4096) 的前导内容，共 20 个段落
	var builder strings.Builder
	builder.WriteString("<html><body>")
	for i := 0; i < 20; i++ {
		builder.WriteString("<p>段落")
		for j := 0; j < 60; j++ {
			builder.WriteString("内容填充")
		}
		builder.WriteString("</p>")
	}
	builder.WriteString("<h1>标题</h1></body></html>")
	source := []byte(builder.String())
	markdown := "# 标题"
	got := RestoreHTMLLeadInBeforeFirstHeading("html", source, markdown)

	// 标题应保留
	headingIdx := strings.Index(got, "# 标题")
	if headingIdx < 0 {
		t.Fatalf("应保留原始 markdown 标题，got=%q", got)
	}
	prefix := got[:headingIdx]
	// 字节上限是软上限：收集会在超过 maxHTMLLeadInBytes 后停止，
	// 因此前导不会包含全部 20 个段落。验证收集确实被截断。
	// 每个段落约 720 字节，20 个段落约 14400 字节；若上限生效，前导应远小于此。
	if len(prefix) >= 14000 {
		t.Fatalf("前导恢复应受字节上限约束，prefixLen=%d（不应接近全部 20 段落的总量）", len(prefix))
	}
	// 同时验证并非全部段落都被收集（上限生效的正面证明）
	count := strings.Count(prefix, "段落")
	if count >= 20 {
		t.Fatalf("字节上限应阻止收集全部 20 个段落，实际收集 %d 个", count)
	}
	if count == 0 {
		t.Fatalf("应至少收集部分前导段落")
	}
}
