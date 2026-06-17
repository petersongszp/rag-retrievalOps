package documentparser

import (
	"context"
	"strings"
	"testing"
)

func TestNormalizeLocalText(t *testing.T) {
	doc, err := NormalizeLocal(context.Background(), LocalRequest{
		FileName: "notes.txt",
		FileType: "txt",
		Content:  []byte("hello world\n"),
	})
	if err != nil {
		t.Fatalf("NormalizeLocal returned error: %v", err)
	}
	if doc.ContentMarkdown != "hello world" {
		t.Fatalf("ContentMarkdown = %q", doc.ContentMarkdown)
	}
	if doc.Source.FileType != "txt" {
		t.Fatalf("Source.FileType = %q", doc.Source.FileType)
	}
	if doc.Extractor.Provider != "local" {
		t.Fatalf("Extractor.Provider = %q", doc.Extractor.Provider)
	}
}

func TestNormalizeMarkdownPipeTable(t *testing.T) {
	input := []byte(strings.Join([]string{
		"# API",
		"",
		"| 字段 | 含义 |",
		"|---|---|",
		"| kb_id | 知识库 ID |",
		"| doc_id | 文档 ID |",
		"",
		"结束",
	}, "\n"))

	doc, err := NormalizeLocal(context.Background(), LocalRequest{
		FileName: "api.md",
		FileType: "md",
		Content:  input,
	})
	if err != nil {
		t.Fatalf("NormalizeLocal returned error: %v", err)
	}
	if len(doc.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(doc.Tables))
	}
	table := doc.Tables[0]
	if table.ID != "table-001" {
		t.Fatalf("table.ID = %q", table.ID)
	}
	if len(table.Rows) != 3 {
		t.Fatalf("expected header plus 2 rows, got %d", len(table.Rows))
	}
	if !table.Rows[0].Cells[0].IsHeader {
		t.Fatalf("first row should be header")
	}
	if table.Rows[1].Cells[0].Text != "kb_id" {
		t.Fatalf("first data cell = %q", table.Rows[1].Cells[0].Text)
	}
	if table.MarkdownStart <= 0 || table.MarkdownEnd <= table.MarkdownStart {
		t.Fatalf("invalid markdown range: %d-%d", table.MarkdownStart, table.MarkdownEnd)
	}
}

func TestNormalizeMarkdownCanonicalizesPipeTableCells(t *testing.T) {
	input := []byte(strings.Join([]string{
		"## 计费规则",
		"",
		"| **项目** | **按量付费** | **包年包月** |",
		"|----|----|----|",
		"| **计费公式** | 计算规格费用 = 服务时长（小时） × 规格单价（元/小时） | 计算规格费用 = 购买时长（月） × 规格单价（元/月） |",
	}, "\n"))

	doc, err := NormalizeLocal(context.Background(), LocalRequest{
		FileName: "billing.md",
		FileType: "markdown",
		Content:  input,
	})
	if err != nil {
		t.Fatalf("NormalizeLocal returned error: %v", err)
	}
	if !strings.Contains(doc.ContentMarkdown, "| 项目 | 按量付费 | 包年包月 |") {
		t.Fatalf("expected canonical table header, got %q", doc.ContentMarkdown)
	}
	if !strings.Contains(doc.ContentMarkdown, "| --- | --- | --- |") {
		t.Fatalf("expected canonical separator row, got %q", doc.ContentMarkdown)
	}
	if len(doc.Tables) != 1 {
		t.Fatalf("expected one parsed table, got %d", len(doc.Tables))
	}
	table := doc.Tables[0]
	if got := table.Rows[0].Cells[0].Text; got != "项目" {
		t.Fatalf("header cell text = %q", got)
	}
	if got := table.Rows[1].Cells[0].Text; got != "计费公式" {
		t.Fatalf("data cell text = %q", got)
	}
	if table.MarkdownStart <= 0 || table.MarkdownEnd > len(doc.ContentMarkdown) {
		t.Fatalf("invalid markdown range %d-%d for content length %d", table.MarkdownStart, table.MarkdownEnd, len(doc.ContentMarkdown))
	}
	if got := doc.ContentMarkdown[table.MarkdownStart:table.MarkdownEnd]; !strings.Contains(got, "计费公式") {
		t.Fatalf("table span does not point at canonical table: %q", got)
	}
}

func TestNormalizeLocalRejectsEmptyContent(t *testing.T) {
	_, err := NormalizeLocal(context.Background(), LocalRequest{
		FileName: "empty.md",
		FileType: "md",
		Content:  []byte("   \n"),
	})
	if err == nil {
		t.Fatalf("empty content should return error")
	}
}
