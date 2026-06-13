package chunking

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
	"github.com/cloudwego/eino/schema"

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

func TestMarkdownStrategyKeepsFormulaHeadingAsHardBoundary(t *testing.T) {
	splitterService, err := splitter.NewDocumentSplitterService(context.Background(), &recursive.Config{
		ChunkSize:   1000,
		OverlapSize: 200,
		KeepType:    0,
	})
	if err != nil {
		t.Fatalf("NewDocumentSplitterService returned error: %v", err)
	}

	normalized := &documentparser.NormalizedDocument{
		ContentMarkdown: "## 在线客户端数量费用\n\n### 在线客户端数量计数规则\n\n1个生产者或1个消费者对象计数为1个在线客户端。\n\n### **计费公式**\n\n当实例的在线客户端数量超过免费额度时，超出部分将被视为付费连接，需按照以下方式进行计费。不足1小时，按1小时计算。\n\n`在线客户端数量费用＝付费在线客户端数量（个）×在线客户端数量单价（元/个/小时）`\n",
		Source: documentparser.NormalizedSource{
			FileName: "billing.md",
			FileType: "md",
		},
		Quality:   documentparser.ParseQuality{Status: "ok", Score: 1},
		Extractor: documentparser.ExtractorInfo{Provider: "local", Version: documentparser.NormalizerVersion},
	}

	strategy := NewMarkdownStrategy(splitterService)
	chunks, err := strategy.Split(context.Background(), Request{
		Document:       normalized,
		BaseMeta:       map[string]interface{}{"document_id": uint64(7), "title": "计费文档"},
		NormalizedPath: "/tmp/billing.md.normalized.json",
	})
	if err != nil {
		t.Fatalf("Split returned error: %v", err)
	}

	foundFormulaChunk := false
	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}
		content := chunk.Content
		if strings.Contains(content, "计数规则") && strings.Contains(content, "计费公式") {
			t.Fatalf("expected heading boundary to be hard, got merged chunk: %q", content)
		}
		if strings.Contains(content, "计费公式") {
			foundFormulaChunk = true
			if !strings.Contains(content, "在线客户端数量费用＝付费在线客户端数量") {
				t.Fatalf("formula chunk is missing formula expression: %q", content)
			}
			if got := chunk.MetaData["chunking_unit"]; got != "formula" {
				t.Fatalf("formula chunking_unit = %v, want formula", got)
			}
			if got := chunk.MetaData["section_title"]; got != "计费公式" {
				t.Fatalf("formula section_title = %v, want 计费公式", got)
			}
			if got := chunk.MetaData["hierarchy_path"]; got != "在线客户端数量费用 > 计费公式" {
				t.Fatalf("formula hierarchy_path = %v", got)
			}
			assertParentChildMetadata(t, chunk)
		}
	}
	if !foundFormulaChunk {
		t.Fatalf("expected formula chunk, got %d chunks", len(chunks))
	}
}

func assertParentChildMetadata(t *testing.T, chunk *schema.Document) {
	t.Helper()
	if chunk == nil {
		t.Fatalf("chunk is nil")
	}
	if chunk.MetaData == nil {
		t.Fatalf("chunk metadata is nil")
	}
	chunkID, ok := chunk.MetaData["chunk_id"].(string)
	if !ok || strings.TrimSpace(chunkID) == "" {
		t.Fatalf("chunk_id = %v, want non-empty string", chunk.MetaData["chunk_id"])
	}
	if got := chunk.MetaData["child_id"]; got != chunkID {
		t.Fatalf("child_id = %v, want %q", got, chunkID)
	}
	parentID, ok := chunk.MetaData["parent_id"].(string)
	if !ok || strings.TrimSpace(parentID) == "" {
		t.Fatalf("parent_id = %v, want non-empty string", chunk.MetaData["parent_id"])
	}
	if got := chunk.MetaData["parent_child_available"]; got != true {
		t.Fatalf("parent_child_available = %v, want true", got)
	}

	childStart := readIntMetadata(chunk.MetaData, "child_start_offset")
	childEnd := readIntMetadata(chunk.MetaData, "child_end_offset")
	parentStart := readIntMetadata(chunk.MetaData, "parent_start_offset")
	parentEnd := readIntMetadata(chunk.MetaData, "parent_end_offset")
	if parentStart > childStart || parentEnd < childEnd {
		t.Fatalf("parent offsets %d/%d do not cover child offsets %d/%d", parentStart, parentEnd, childStart, childEnd)
	}
	if got := chunk.MetaData["parent_build_version"]; got != "phase3-parent-child-v1" {
		t.Fatalf("parent_build_version = %v", got)
	}
}
