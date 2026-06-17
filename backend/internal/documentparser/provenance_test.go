package documentparser

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestAnnotateChunksWithProvenance(t *testing.T) {
	doc := &NormalizedDocument{
		Blocks: []NormalizedBlock{
			{ID: "p1-b1", Page: 1, MarkdownStart: 0, MarkdownEnd: 20, Confidence: 0.95},
			{ID: "p2-b1", Page: 2, MarkdownStart: 21, MarkdownEnd: 50, Confidence: 0.85},
		},
		Tables: []NormalizedTable{
			{ID: "t-001", Page: 2, MarkdownStart: 22, MarkdownEnd: 45},
		},
	}
	chunks := []*schema.Document{
		{Content: "first chunk", MetaData: map[string]interface{}{"child_start_offset": 0, "child_end_offset": 20}},
		{Content: "second chunk", MetaData: map[string]interface{}{"child_start_offset": 21, "child_end_offset": 50}},
	}

	AnnotateChunksWithProvenance(chunks, doc, "/tmp/doc.pdf.normalized.json")

	if chunks[0].MetaData["normalized_path"] != "/tmp/doc.pdf.normalized.json" {
		t.Fatalf("normalized_path missing")
	}
	if chunks[0].MetaData["page_start"] != 1 {
		t.Fatalf("page_start = %v", chunks[0].MetaData["page_start"])
	}
	if chunks[1].MetaData["page_start"] != 2 || chunks[1].MetaData["page_end"] != 2 {
		t.Fatalf("page metadata = %v/%v", chunks[1].MetaData["page_start"], chunks[1].MetaData["page_end"])
	}
	if chunks[1].MetaData["table_ids"].([]string)[0] != "t-001" {
		t.Fatalf("table_ids = %v", chunks[1].MetaData["table_ids"])
	}
}
