package ragqueue

import (
	"context"
	"testing"

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
