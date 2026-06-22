package ragqueue

import (
	"context"
	"testing"

	"interview-agents/internal/milvus/chunkmeta"

	"github.com/cloudwego/eino/schema"
)

type fakeKnowledgeSplitter struct {
	splitCalls         int
	splitMarkdownCalls int
}

func (f *fakeKnowledgeSplitter) Split(_ context.Context, docs []*schema.Document) ([]*schema.Document, error) {
	f.splitCalls++
	return docs, nil
}

func (f *fakeKnowledgeSplitter) SplitMarkdownDocument(_ context.Context, doc *schema.Document) ([]*schema.Document, error) {
	f.splitMarkdownCalls++
	return []*schema.Document{doc}, nil
}

func TestSplitKnowledgeDocumentRoutesMarkdownToMarkdownSplitter(t *testing.T) {
	splitter := &fakeKnowledgeSplitter{}
	doc := &schema.Document{Content: "# Guide"}

	_, err := splitKnowledgeDocument(context.Background(), splitter, doc, "markdown")
	if err != nil {
		t.Fatalf("splitKnowledgeDocument failed: %v", err)
	}
	if splitter.splitMarkdownCalls != 1 {
		t.Fatalf("expected markdown splitter to be called once, got %d", splitter.splitMarkdownCalls)
	}
	if splitter.splitCalls != 0 {
		t.Fatalf("expected generic splitter not to be called, got %d", splitter.splitCalls)
	}
}

func TestSplitKnowledgeDocumentRoutesTXTAndPDFToGenericSplitter(t *testing.T) {
	for _, fileType := range []string{"txt", "pdf"} {
		splitter := &fakeKnowledgeSplitter{}
		doc := &schema.Document{Content: "plain text"}

		_, err := splitKnowledgeDocument(context.Background(), splitter, doc, fileType)
		if err != nil {
			t.Fatalf("splitKnowledgeDocument(%s) failed: %v", fileType, err)
		}
		if splitter.splitCalls != 1 {
			t.Fatalf("expected generic splitter for %s, got %d calls", fileType, splitter.splitCalls)
		}
		if splitter.splitMarkdownCalls != 0 {
			t.Fatalf("expected markdown splitter to remain unused for %s, got %d", fileType, splitter.splitMarkdownCalls)
		}
	}
}

func TestSummarizeKnowledgeChunksCapturesChunkingStats(t *testing.T) {
	stats := summarizeKnowledgeChunks([]*schema.Document{
		{
			Content: "alpha chunk",
			MetaData: map[string]interface{}{
				chunkmeta.KeyEmbeddingContent:     "prefix alpha chunk",
				chunkmeta.KeySemanticSplitEnabled: true,
				chunkmeta.KeySplitStrategy:        chunkmeta.SplitStrategyMarkdownV1,
			},
		},
		{
			Content: "beta chunk with more text",
			MetaData: map[string]interface{}{
				chunkmeta.KeyEmbeddingContent: "prefix beta chunk with more text",
				chunkmeta.KeySplitStrategy:    chunkmeta.SplitStrategyRecursiveV1,
			},
		},
	})

	if stats.P95ChunkChars == 0 {
		t.Fatalf("expected p95 chunk chars to be populated")
	}
	if stats.AvgEmbeddingChars == 0 {
		t.Fatalf("expected average embedding chars to be populated")
	}
	if stats.SemanticResplitCount != 1 {
		t.Fatalf("expected semantic resplit count 1, got %d", stats.SemanticResplitCount)
	}
	if stats.MarkdownStructureChunkCount != 1 {
		t.Fatalf("expected markdown structure chunk count 1, got %d", stats.MarkdownStructureChunkCount)
	}
}
