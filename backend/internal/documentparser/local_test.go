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
