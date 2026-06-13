package chunking

import (
	"context"
	"testing"

	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"

	"interview-agents/internal/documentparser"
	"interview-agents/internal/milvus/splitter"
)

func TestMarkdownStrategySplitsMarkdownAndAnnotatesProvenance(t *testing.T) {
	splitterService, err := splitter.NewDocumentSplitterService(context.Background(), &recursive.Config{
		ChunkSize:   80,
		OverlapSize: 8,
		KeepType:    0,
	})
	if err != nil {
		t.Fatalf("NewDocumentSplitterService returned error: %v", err)
	}

	normalized := &documentparser.NormalizedDocument{
		ContentMarkdown: "# Guide\n\n## API\nThe API layer handles routing and validation.\n\n| 字段 | 含义 |\n|---|---|\n| id | 编号 |\n",
		Source: documentparser.NormalizedSource{
			FileName: "guide.md",
			FileType: "md",
		},
		Blocks: []documentparser.NormalizedBlock{
			{ID: "p1-b1", Page: 1, MarkdownStart: 0, MarkdownEnd: 64, Confidence: 0.99},
		},
		Tables: []documentparser.NormalizedTable{
			{ID: "t-001", Page: 1, MarkdownStart: 65, MarkdownEnd: 108},
		},
		Quality:   documentparser.ParseQuality{Status: "ok", Score: 1},
		Extractor: documentparser.ExtractorInfo{Provider: "local", Version: documentparser.NormalizerVersion},
	}

	strategy := NewMarkdownStrategy(splitterService)
	chunks, err := strategy.Split(context.Background(), Request{
		Document:       normalized,
		BaseMeta:       map[string]interface{}{"document_id": uint64(42), "title": "Guide"},
		NormalizedPath: "/tmp/guide.md.normalized.json",
	})
	if err != nil {
		t.Fatalf("Split returned error: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatalf("expected chunks, got 0")
	}

	first := chunks[0]
	if first.MetaData["document_id"] != uint64(42) {
		t.Fatalf("document_id metadata = %v", first.MetaData["document_id"])
	}
	if first.MetaData["normalized_path"] != "/tmp/guide.md.normalized.json" {
		t.Fatalf("normalized_path metadata = %v", first.MetaData["normalized_path"])
	}
	if first.MetaData["page_start"] != 1 || first.MetaData["page_end"] != 1 {
		t.Fatalf("page metadata = %v/%v", first.MetaData["page_start"], first.MetaData["page_end"])
	}
	if got := first.MetaData["block_ids"].([]string)[0]; got != "p1-b1" {
		t.Fatalf("block_ids[0] = %q", got)
	}

	foundTableChunk := false
	for _, chunk := range chunks {
		if chunk == nil || chunk.MetaData == nil {
			continue
		}
		tableIDs, ok := chunk.MetaData["table_ids"].([]string)
		if ok && len(tableIDs) > 0 && tableIDs[0] == "t-001" {
			foundTableChunk = true
		}
	}
	if !foundTableChunk {
		t.Fatalf("expected at least one chunk to carry table provenance")
	}
}
