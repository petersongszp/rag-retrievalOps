package splitter

import (
	"context"
	"strings"
	"testing"

	"interview-agents/internal/milvus/chunkmeta"

	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
)

func TestSplitMarkdownDocumentAnnotatesHeadingHierarchy(t *testing.T) {
	service := mustNewTestSplitter(t, 90, 10)
	doc := &schema.Document{
		Content: "# Handbook\n\n## API Layer\nThe API layer handles routing, validation, authentication, throttling, and audit logging for every request.\nIt also normalizes request metadata before dispatch.\n\n## Storage Layer\nThe storage layer persists vectors and metadata for retrieval.\n",
		MetaData: map[string]interface{}{
			"document_id":      uint64(42),
			"title":            "Handbook",
			"source_file_type": chunkmeta.SourceFileTypeMarkdown,
		},
	}

	chunks, err := service.SplitMarkdownDocument(context.Background(), doc)
	if err != nil {
		t.Fatalf("SplitMarkdownDocument failed: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatalf("expected markdown chunks, got 0")
	}

	foundStructuredChunk := false
	for _, chunk := range chunks {
		if chunk == nil || chunk.MetaData == nil {
			t.Fatalf("expected chunk metadata to be present")
		}
		if chunk.MetaData["document_id"] != uint64(42) {
			t.Fatalf("expected document_id to be preserved, got %v", chunk.MetaData["document_id"])
		}
		if chunk.MetaData[chunkmeta.KeySplitStrategy] != chunkmeta.SplitStrategyMarkdownV1 {
			t.Fatalf("expected markdown split strategy, got %v", chunk.MetaData[chunkmeta.KeySplitStrategy])
		}
		if chunk.MetaData[chunkmeta.KeySplitVersion] != "v1" {
			t.Fatalf("expected split version v1, got %v", chunk.MetaData[chunkmeta.KeySplitVersion])
		}
		if chunk.MetaData[chunkmeta.KeyEmbeddingBuildStrategy] != chunkmeta.EmbeddingBuildStrategyRaw {
			t.Fatalf("expected raw embedding build strategy, got %v", chunk.MetaData[chunkmeta.KeyEmbeddingBuildStrategy])
		}
		if chunk.MetaData[chunkmeta.KeyContextVersion] != chunkmeta.ContextVersionRawContent {
			t.Fatalf("expected raw context version, got %v", chunk.MetaData[chunkmeta.KeyContextVersion])
		}
		if chunk.MetaData[chunkmeta.KeySourceFileType] != chunkmeta.SourceFileTypeMarkdown {
			t.Fatalf("expected markdown source file type, got %v", chunk.MetaData[chunkmeta.KeySourceFileType])
		}
		if chunk.MetaData["child_id"] == "" || chunk.MetaData["parent_id"] == "" {
			t.Fatalf("expected child_id/parent_id to be populated, got child=%v parent=%v", chunk.MetaData["child_id"], chunk.MetaData["parent_id"])
		}
		if chunk.MetaData["chunk_id"] != chunk.MetaData["child_id"] {
			t.Fatalf("expected chunk_id to align with child_id")
		}
		if available, ok := chunk.MetaData["parent_child_available"].(bool); !ok || !available {
			t.Fatalf("expected parent_child_available=true, got %v", chunk.MetaData["parent_child_available"])
		}
		if section := asString(t, chunk.MetaData["section_title"]); section == "API Layer" {
			foundStructuredChunk = true
			if got := asString(t, chunk.MetaData["hierarchy_path"]); got != "Handbook > API Layer" {
				t.Fatalf("expected hierarchy path for API chunk, got %q", got)
			}
		}
	}

	if !foundStructuredChunk {
		t.Fatalf("expected at least one chunk to inherit API Layer hierarchy metadata")
	}
}

func TestSplitPreservesChildParentOffsets(t *testing.T) {
	service := mustNewTestSplitter(t, 70, 8)
	content := "First paragraph introduces the document.\n\nSecond paragraph contains the most relevant evidence.\n\nThird paragraph closes the explanation."
	doc := &schema.Document{
		Content: content,
		MetaData: map[string]interface{}{
			"document_id": uint64(1001),
			"title":       "Offset Spec",
		},
	}

	chunks, err := service.Split(context.Background(), []*schema.Document{doc})
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatalf("expected chunks, got 0")
	}

	for _, chunk := range chunks {
		if chunk == nil || chunk.MetaData == nil {
			t.Fatalf("expected chunk metadata")
		}
		childStart := asInt(t, chunk.MetaData["child_start_offset"])
		childEnd := asInt(t, chunk.MetaData["child_end_offset"])
		parentStart := asInt(t, chunk.MetaData["parent_start_offset"])
		parentEnd := asInt(t, chunk.MetaData["parent_end_offset"])

		if childStart < 0 || childEnd < childStart || childEnd > len(content) {
			t.Fatalf("invalid child offsets: start=%d end=%d len=%d", childStart, childEnd, len(content))
		}
		if chunk.MetaData[chunkmeta.KeySplitStrategy] != chunkmeta.SplitStrategyRecursiveV1 {
			t.Fatalf("expected recursive split strategy, got %v", chunk.MetaData[chunkmeta.KeySplitStrategy])
		}
		if chunk.MetaData[chunkmeta.KeyEmbeddingBuildStrategy] != chunkmeta.EmbeddingBuildStrategyRaw {
			t.Fatalf("expected raw embedding build strategy, got %v", chunk.MetaData[chunkmeta.KeyEmbeddingBuildStrategy])
		}
		if parentStart > childStart || parentEnd < childEnd {
			t.Fatalf("expected parent span to contain child span, parent=[%d,%d) child=[%d,%d)", parentStart, parentEnd, childStart, childEnd)
		}

		childSlice := content[childStart:childEnd]
		if !strings.Contains(childSlice, strings.TrimSpace(chunk.Content)) && !strings.Contains(chunk.Content, strings.TrimSpace(childSlice)) {
			t.Fatalf("expected chunk content to align with located child offsets, chunk=%q child_slice=%q", chunk.Content, childSlice)
		}
	}
}

func TestLongHeadingSectionUsesTruncatedParentWindow(t *testing.T) {
	service := mustNewTestSplitter(t, 40, 5)
	content := "# Guide\n\n## Deep Dive\nParagraph one explains the retrieval model in detail.\n\nParagraph two keeps adding more structured evidence for the same section.\n\nParagraph three extends the section so the parent block must be windowed.\n\nParagraph four pushes the section well beyond the configured parent window.\n"
	doc := &schema.Document{
		Content: content,
		MetaData: map[string]interface{}{
			"document_id": uint64(77),
			"title":       "Guide",
		},
	}

	chunks, err := service.SplitMarkdownDocument(context.Background(), doc)
	if err != nil {
		t.Fatalf("SplitMarkdownDocument failed: %v", err)
	}

	foundTruncated := false
	for _, chunk := range chunks {
		if chunk == nil || chunk.MetaData == nil {
			continue
		}
		if truncated, ok := chunk.MetaData["parent_truncated"].(bool); ok && truncated {
			foundTruncated = true
			if got := asString(t, chunk.MetaData["parent_build_strategy"]); got != parentStrategyHeadingWin {
				t.Fatalf("expected heading window strategy, got %q", got)
			}
			parentWidth := asInt(t, chunk.MetaData["parent_end_offset"]) - asInt(t, chunk.MetaData["parent_start_offset"])
			if parentWidth >= len(content) {
				t.Fatalf("expected truncated parent window smaller than full document, got %d vs %d", parentWidth, len(content))
			}
		}
	}

	if !foundTruncated {
		t.Fatalf("expected at least one chunk to record a truncated parent window")
	}
}

func TestSemanticSecondarySplitMarksLongBlocks(t *testing.T) {
	service := mustNewTestSplitter(t, 500, 20)
	service.ConfigureSemanticSplit(SemanticSplitConfig{
		Enabled:              true,
		MinBlockSize:         80,
		TargetChunkSize:      60,
		MaxChunkSize:         90,
		BreakpointPercentile: 50,
		MinSentencesPerChunk: 2,
		Embedder:             &fakeSemanticEmbedder{},
	})

	doc := &schema.Document{
		Content: "Alpha topic introduces the incident and the first mitigation. Alpha topic continues with more operational detail. Beta topic switches to cost controls and exception handling. Beta topic adds alerts and escalation steps.",
		MetaData: map[string]interface{}{
			"document_id": uint64(501),
			"title":       "Semantic Guide",
		},
	}

	chunks, err := service.Split(context.Background(), []*schema.Document{doc})
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected semantic split to produce multiple chunks, got %d", len(chunks))
	}

	foundSemanticChunk := false
	for _, chunk := range chunks {
		if chunk.MetaData[chunkmeta.KeySemanticSplitEnabled] == true {
			foundSemanticChunk = true
			if chunk.MetaData[chunkmeta.KeySemanticBreakpointMethod] != chunkmeta.SemanticBreakpointEmbeddingV1 {
				t.Fatalf("expected semantic breakpoint method, got %v", chunk.MetaData[chunkmeta.KeySemanticBreakpointMethod])
			}
		}
	}
	if !foundSemanticChunk {
		t.Fatalf("expected at least one chunk to record semantic split metadata")
	}
}

func mustNewTestSplitter(t *testing.T, chunkSize, overlap int) *DocumentSplitterService {
	t.Helper()

	service, err := NewDocumentSplitterService(context.Background(), &recursive.Config{
		ChunkSize:   chunkSize,
		OverlapSize: overlap,
		Separators:  []string{"\n\n", "\n", ". ", " "},
	})
	if err != nil {
		t.Fatalf("NewDocumentSplitterService failed: %v", err)
	}
	return service
}

type fakeSemanticEmbedder struct{}

func (f *fakeSemanticEmbedder) EmbedStrings(_ context.Context, texts []string, _ ...embedding.Option) ([][]float64, error) {
	vectors := make([][]float64, 0, len(texts))
	for idx := range texts {
		switch {
		case idx < 2:
			vectors = append(vectors, []float64{1, 0})
		default:
			vectors = append(vectors, []float64{0, 1})
		}
	}
	return vectors, nil
}

func asString(t *testing.T, value interface{}) string {
	t.Helper()
	if value == nil {
		t.Fatalf("expected string value, got nil")
	}
	stringValue, ok := value.(string)
	if !ok {
		t.Fatalf("expected string value, got %T (%v)", value, value)
	}
	return strings.TrimSpace(stringValue)
}

func asInt(t *testing.T, value interface{}) int {
	t.Helper()
	intValue, ok := value.(int)
	if !ok {
		t.Fatalf("expected int value, got %T (%v)", value, value)
	}
	return intValue
}
