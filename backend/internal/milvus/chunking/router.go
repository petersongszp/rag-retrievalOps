package chunking

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	"interview-agents/internal/documentparser"
)

const (
	StrategyMarkdown       = "markdown"
	StrategyTableAware     = "table-aware"
	StrategyStructureAware = "structure-aware"
	StrategyOCRAware       = "ocr-aware"
)

type Matcher func(*documentparser.NormalizedDocument) bool

type RoutedStrategy struct {
	Name     string
	Match    Matcher
	Strategy Strategy
}

type StrategyRouter struct {
	defaultStrategy Strategy
	routes          []RoutedStrategy
}

func NewStrategyRouter(defaultStrategy Strategy, routes []RoutedStrategy) *StrategyRouter {
	return &StrategyRouter{
		defaultStrategy: defaultStrategy,
		routes:          routes,
	}
}

func (r *StrategyRouter) Split(ctx context.Context, req Request) ([]*schema.Document, error) {
	if r == nil || r.defaultStrategy == nil {
		return nil, fmt.Errorf("chunking strategy router has no default strategy")
	}
	for _, route := range r.routes {
		if route.Strategy == nil || route.Match == nil || !route.Match(req.Document) {
			continue
		}
		chunks, err := route.Strategy.Split(ctx, req)
		if err != nil {
			return nil, err
		}
		tagChunkingRoute(chunks, firstNonEmpty(route.Name, StrategyMarkdown))
		return chunks, nil
	}

	chunks, err := r.defaultStrategy.Split(ctx, req)
	if err != nil {
		return nil, err
	}
	tagChunkingRoute(chunks, StrategyMarkdown)
	return chunks, nil
}

func MatchHasTables(doc *documentparser.NormalizedDocument) bool {
	return doc != nil && len(doc.Tables) > 0
}

func MatchOCRAware(doc *documentparser.NormalizedDocument) bool {
	if doc == nil || documentparser.NormalizeFileType(doc.Source.FileType) != "pdf" {
		return false
	}
	if strings.Contains(strings.ToLower(strings.Join(doc.Quality.Warnings, " ")), "ocr") {
		return true
	}
	for _, block := range doc.Blocks {
		if block.Confidence > 0 {
			return true
		}
	}
	return false
}

func MatchHasStructure(doc *documentparser.NormalizedDocument) bool {
	if doc == nil || len(doc.Blocks) == 0 {
		return false
	}
	switch documentparser.NormalizeFileType(doc.Source.FileType) {
	case "docx", "pdf", "html", "htm":
		return true
	default:
		return false
	}
}

func tagChunkingRoute(chunks []*schema.Document, route string) {
	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}
		if chunk.MetaData == nil {
			chunk.MetaData = map[string]interface{}{}
		}
		chunk.MetaData["chunking_route"] = route
	}
}
