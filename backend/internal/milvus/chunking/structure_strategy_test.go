package chunking

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"

	"interview-agents/internal/documentparser"
)

func TestStructureAwareStrategyBuildsBlockWindowChunks(t *testing.T) {
	doc := &documentparser.NormalizedDocument{
		ContentMarkdown: "# API\n\nParagraph one explains routing.\n\nParagraph two explains validation.",
		Source:          documentparser.NormalizedSource{FileName: "api.docx", FileType: "docx"},
		Blocks: []documentparser.NormalizedBlock{
			{ID: "b-title", Type: "heading", Page: 1, MarkdownStart: 0, MarkdownEnd: 6},
			{ID: "b-p1", Type: "paragraph", Page: 1, MarkdownStart: 8, MarkdownEnd: 39, Confidence: 0.98},
			{ID: "b-p2", Type: "paragraph", Page: 1, MarkdownStart: 41, MarkdownEnd: 74, Confidence: 0.96},
		},
		Quality:   documentparser.ParseQuality{Status: "ok", Score: 1},
		Extractor: documentparser.ExtractorInfo{Provider: "docling", Version: "v1"},
	}

	strategy := NewStructureAwareStrategy(&recordingStrategy{name: "markdown"})
	chunks, err := strategy.Split(context.Background(), Request{
		Document:       doc,
		BaseMeta:       map[string]interface{}{"document_id": uint64(7)},
		NormalizedPath: "/tmp/api.docx.normalized.json",
	})
	if err != nil {
		t.Fatalf("Split returned error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected one block-window chunk, got %d", len(chunks))
	}

	chunk := chunks[0]
	if chunk.MetaData["chunking_strategy"] != "structure-aware" {
		t.Fatalf("chunking_strategy = %v", chunk.MetaData["chunking_strategy"])
	}
	if chunk.MetaData["chunking_unit"] != "block_window" {
		t.Fatalf("chunking_unit = %v", chunk.MetaData["chunking_unit"])
	}
	if chunk.MetaData["document_id"] != uint64(7) {
		t.Fatalf("document_id = %v", chunk.MetaData["document_id"])
	}
	if chunk.MetaData["page_start"] != 1 || chunk.MetaData["page_end"] != 1 {
		t.Fatalf("page metadata = %v/%v", chunk.MetaData["page_start"], chunk.MetaData["page_end"])
	}
	if got := chunk.MetaData["block_ids"].([]string); len(got) != 3 || got[0] != "b-title" || got[2] != "b-p2" {
		t.Fatalf("block_ids = %v", got)
	}
	if got := chunk.MetaData["block_types"].([]string); len(got) != 2 || got[0] != "heading" || got[1] != "paragraph" {
		t.Fatalf("block_types = %v", got)
	}
	if chunk.MetaData["normalized_path"] != "/tmp/api.docx.normalized.json" {
		t.Fatalf("normalized_path = %v", chunk.MetaData["normalized_path"])
	}
}

func TestStructureAwareStrategyFallsBackWhenBlocksAreMissing(t *testing.T) {
	fallback := &recordingStrategy{name: "markdown"}
	strategy := NewStructureAwareStrategy(fallback)

	chunks, err := strategy.Split(context.Background(), Request{
		Document: &documentparser.NormalizedDocument{
			ContentMarkdown: "plain markdown",
			Source:          documentparser.NormalizedSource{FileName: "plain.html", FileType: "html"},
			Quality:         documentparser.ParseQuality{Status: "ok", Score: 1},
			Extractor:       documentparser.ExtractorInfo{Provider: "docling", Version: "v1"},
		},
	})
	if err != nil {
		t.Fatalf("Split returned error: %v", err)
	}
	if fallback.calls != 1 {
		t.Fatalf("expected fallback to be called once, got %d", fallback.calls)
	}
	if chunks[0].MetaData["chunking_strategy"] != "markdown" {
		t.Fatalf("chunking_strategy = %v", chunks[0].MetaData["chunking_strategy"])
	}
}

var _ Strategy = (*recordingStrategy)(nil)
var _ = schema.Document{}
