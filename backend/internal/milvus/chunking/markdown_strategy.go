package chunking

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	"interview-agents/internal/documentparser"
)

type MarkdownSplitter interface {
	SplitMarkdownDocument(ctx context.Context, doc *schema.Document) ([]*schema.Document, error)
}

type MarkdownStrategy struct {
	splitter MarkdownSplitter
}

func NewMarkdownStrategy(splitter MarkdownSplitter) *MarkdownStrategy {
	return &MarkdownStrategy{splitter: splitter}
}

func (s *MarkdownStrategy) Split(ctx context.Context, req Request) ([]*schema.Document, error) {
	if s == nil || s.splitter == nil {
		return nil, fmt.Errorf("markdown chunking strategy is not initialized")
	}
	if req.Document == nil {
		return nil, fmt.Errorf("normalized document is nil")
	}
	if strings.TrimSpace(req.Document.ContentMarkdown) == "" {
		return nil, fmt.Errorf("normalized markdown content is empty")
	}

	doc := &schema.Document{
		Content:  req.Document.ContentMarkdown,
		MetaData: cloneMetadata(req.BaseMeta),
	}
	chunks, err := s.splitter.SplitMarkdownDocument(ctx, doc)
	if err != nil {
		return nil, err
	}
	documentparser.AnnotateChunksWithProvenance(chunks, req.Document, req.NormalizedPath)
	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}
		if chunk.MetaData == nil {
			chunk.MetaData = map[string]interface{}{}
		}
		chunk.MetaData["chunking_strategy"] = StrategyMarkdown
		chunk.MetaData["chunking_unit"] = "markdown_recursive"
	}
	return chunks, nil
}

func cloneMetadata(input map[string]interface{}) map[string]interface{} {
	if len(input) == 0 {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
