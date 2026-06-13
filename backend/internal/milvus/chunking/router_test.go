package chunking

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"

	"interview-agents/internal/documentparser"
)

func TestStrategyRouterSelectsFirstMatchingStrategy(t *testing.T) {
	tableStrategy := &recordingStrategy{name: "table-aware"}
	defaultStrategy := &recordingStrategy{name: "markdown"}
	router := NewStrategyRouter(defaultStrategy, []RoutedStrategy{
		{
			Name:     "table-aware",
			Match:    MatchHasTables,
			Strategy: tableStrategy,
		},
	})

	chunks, err := router.Split(context.Background(), Request{
		Document: &documentparser.NormalizedDocument{
			ContentMarkdown: "| A | B |\n|---|---|\n| x | y |\n",
			Source:          documentparser.NormalizedSource{FileName: "table.pdf", FileType: "pdf"},
			Tables:          []documentparser.NormalizedTable{{ID: "t-001", MarkdownStart: 0, MarkdownEnd: 31}},
			Quality:         documentparser.ParseQuality{Status: "ok", Score: 1},
			Extractor:       documentparser.ExtractorInfo{Provider: "test", Version: "v1"},
		},
	})
	if err != nil {
		t.Fatalf("Split returned error: %v", err)
	}
	if tableStrategy.calls != 1 {
		t.Fatalf("expected table strategy to be called once, got %d", tableStrategy.calls)
	}
	if defaultStrategy.calls != 0 {
		t.Fatalf("expected default strategy not to be called, got %d", defaultStrategy.calls)
	}
	if chunks[0].MetaData["chunking_route"] != "table-aware" {
		t.Fatalf("chunking_route = %v", chunks[0].MetaData["chunking_route"])
	}
}

func TestStrategyRouterFallsBackToDefaultStrategy(t *testing.T) {
	tableStrategy := &recordingStrategy{name: "table-aware"}
	defaultStrategy := &recordingStrategy{name: "markdown"}
	router := NewStrategyRouter(defaultStrategy, []RoutedStrategy{
		{
			Name:     "table-aware",
			Match:    MatchHasTables,
			Strategy: tableStrategy,
		},
	})

	chunks, err := router.Split(context.Background(), Request{
		Document: &documentparser.NormalizedDocument{
			ContentMarkdown: "plain text",
			Source:          documentparser.NormalizedSource{FileName: "plain.md", FileType: "md"},
			Quality:         documentparser.ParseQuality{Status: "ok", Score: 1},
			Extractor:       documentparser.ExtractorInfo{Provider: "test", Version: "v1"},
		},
	})
	if err != nil {
		t.Fatalf("Split returned error: %v", err)
	}
	if tableStrategy.calls != 0 {
		t.Fatalf("expected table strategy not to be called, got %d", tableStrategy.calls)
	}
	if defaultStrategy.calls != 1 {
		t.Fatalf("expected default strategy to be called once, got %d", defaultStrategy.calls)
	}
	if chunks[0].MetaData["chunking_route"] != "markdown" {
		t.Fatalf("chunking_route = %v", chunks[0].MetaData["chunking_route"])
	}
}

type recordingStrategy struct {
	name  string
	calls int
}

func (s *recordingStrategy) Split(_ context.Context, req Request) ([]*schema.Document, error) {
	s.calls++
	return []*schema.Document{
		{
			Content: req.Document.ContentMarkdown,
			MetaData: map[string]interface{}{
				"chunking_strategy": s.name,
			},
		},
	}, nil
}
