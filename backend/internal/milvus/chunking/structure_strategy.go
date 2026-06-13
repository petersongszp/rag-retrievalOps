package chunking

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	"interview-agents/internal/documentparser"
)

type StructureAwareStrategy struct {
	fallback      Strategy
	maxChunkBytes int
}

func NewStructureAwareStrategy(fallback Strategy) *StructureAwareStrategy {
	return &StructureAwareStrategy{
		fallback:      fallback,
		maxChunkBytes: defaultStructuredChunkBytes,
	}
}

func (s *StructureAwareStrategy) Split(ctx context.Context, req Request) ([]*schema.Document, error) {
	if req.Document == nil {
		return nil, fmt.Errorf("normalized document is nil")
	}
	blocks := sortedValidBlocks(req.Document)
	if len(blocks) == 0 {
		if s == nil || s.fallback == nil {
			return nil, fmt.Errorf("structure-aware strategy has no valid blocks and no fallback")
		}
		return s.fallback.Split(ctx, req)
	}

	maxChunkBytes := defaultStructuredChunkBytes
	if s != nil && s.maxChunkBytes > 0 {
		maxChunkBytes = s.maxChunkBytes
	}

	chunks := make([]*schema.Document, 0)
	windowBlocks := make([]documentparser.NormalizedBlock, 0)
	windowStart := 0
	windowEnd := 0

	flush := func() {
		if len(windowBlocks) == 0 || windowEnd <= windowStart {
			return
		}
		content := sliceBySpan(req.Document.ContentMarkdown, windowStart, windowEnd)
		if strings.TrimSpace(content) != "" {
			chunks = append(chunks, &schema.Document{
				Content:  content,
				MetaData: metadataForBlockWindow(req, windowBlocks, windowStart, windowEnd, StrategyStructureAware, "block_window"),
			})
		}
		windowBlocks = windowBlocks[:0]
		windowStart = 0
		windowEnd = 0
	}

	for _, block := range blocks {
		if len(windowBlocks) == 0 {
			windowStart = block.MarkdownStart
			windowEnd = block.MarkdownEnd
			windowBlocks = append(windowBlocks, block)
			continue
		}

		candidateEnd := block.MarkdownEnd
		if candidateEnd-windowStart > maxChunkBytes {
			flush()
			windowStart = block.MarkdownStart
			windowEnd = block.MarkdownEnd
			windowBlocks = append(windowBlocks, block)
			continue
		}
		if block.MarkdownEnd > windowEnd {
			windowEnd = block.MarkdownEnd
		}
		windowBlocks = append(windowBlocks, block)
	}
	flush()

	if len(chunks) == 0 {
		if s == nil || s.fallback == nil {
			return nil, fmt.Errorf("structure-aware strategy produced no chunks and has no fallback")
		}
		return s.fallback.Split(ctx, req)
	}
	finalizeChunkIndexes(chunks)
	return chunks, nil
}
