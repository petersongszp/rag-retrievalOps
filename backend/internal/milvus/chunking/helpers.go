package chunking

import (
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"

	"interview-agents/internal/documentparser"
)

const (
	defaultStructuredChunkBytes = 1000
	defaultMaxChunkContentBytes = 4096
)

type span struct {
	start int
	end   int
}

func validSpan(content string, start, end int) bool {
	return start >= 0 && end > start && end <= len(content)
}

func sliceBySpan(content string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end > len(content) {
		end = len(content)
	}
	if end < start {
		end = start
	}
	return strings.TrimSpace(content[start:end])
}

func sortedValidBlocks(doc *documentparser.NormalizedDocument) []documentparser.NormalizedBlock {
	if doc == nil {
		return nil
	}
	blocks := make([]documentparser.NormalizedBlock, 0, len(doc.Blocks))
	for _, block := range doc.Blocks {
		if validSpan(doc.ContentMarkdown, block.MarkdownStart, block.MarkdownEnd) {
			blocks = append(blocks, block)
		}
	}
	sort.SliceStable(blocks, func(i, j int) bool {
		return blocks[i].MarkdownStart < blocks[j].MarkdownStart
	})
	return blocks
}

func metadataForBlockWindow(req Request, blocks []documentparser.NormalizedBlock, start, end int, strategy, unit string) map[string]interface{} {
	meta := cloneMetadata(req.BaseMeta)
	meta["chunking_strategy"] = strategy
	meta["chunking_unit"] = unit
	meta["normalized_path"] = req.NormalizedPath
	meta["child_start_offset"] = start
	meta["child_end_offset"] = end

	blockIDs := make([]string, 0, len(blocks))
	blockTypes := make([]string, 0)
	seenTypes := map[string]bool{}
	pageStart := 0
	pageEnd := 0
	confidenceSum := 0.0
	confidenceCount := 0

	for _, block := range blocks {
		if strings.TrimSpace(block.ID) != "" {
			blockIDs = append(blockIDs, block.ID)
		}
		blockType := strings.TrimSpace(block.Type)
		if blockType != "" && !seenTypes[blockType] {
			blockTypes = append(blockTypes, blockType)
			seenTypes[blockType] = true
		}
		if block.Page > 0 {
			if pageStart == 0 || block.Page < pageStart {
				pageStart = block.Page
			}
			if block.Page > pageEnd {
				pageEnd = block.Page
			}
		}
		if block.Confidence > 0 {
			confidenceSum += block.Confidence
			confidenceCount++
		}
	}

	if len(blockIDs) > 0 {
		meta["block_ids"] = blockIDs
	}
	if len(blockTypes) > 0 {
		meta["block_types"] = blockTypes
	}
	if pageStart > 0 {
		meta["page_start"] = pageStart
		meta["page_end"] = pageEnd
	}
	if confidenceCount > 0 {
		meta["ocr_confidence"] = round4(confidenceSum / float64(confidenceCount))
	}
	return meta
}

func finalizeChunkIndexes(chunks []*schema.Document) {
	total := len(chunks)
	for i, chunk := range chunks {
		if chunk == nil {
			continue
		}
		if chunk.MetaData == nil {
			chunk.MetaData = map[string]interface{}{}
		}
		chunk.MetaData["chunk_index"] = i
		chunk.MetaData["total_chunks"] = total
	}
}

func chunkSpan(chunk *schema.Document) span {
	if chunk == nil || chunk.MetaData == nil {
		return span{}
	}
	return span{
		start: readIntMetadata(chunk.MetaData, "child_start_offset"),
		end:   readIntMetadata(chunk.MetaData, "child_end_offset"),
	}
}

func readIntMetadata(metadata map[string]interface{}, key string) int {
	switch value := metadata[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

func rangesOverlap(leftStart, leftEnd, rightStart, rightEnd int) bool {
	return leftStart < rightEnd && rightStart < leftEnd
}

func averageBlockConfidence(blocks []documentparser.NormalizedBlock, start, end int) float64 {
	sum := 0.0
	count := 0
	for _, block := range blocks {
		if block.Confidence <= 0 || !rangesOverlap(start, end, block.MarkdownStart, block.MarkdownEnd) {
			continue
		}
		sum += block.Confidence
		count++
	}
	if count == 0 {
		return 0
	}
	return round4(sum / float64(count))
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func splitContentByByteLimit(content string, limit int) []string {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	if limit <= 0 || len(content) <= limit {
		return []string{content}
	}

	segments := make([]string, 0)
	var builder strings.Builder
	flush := func() {
		segment := strings.TrimSpace(builder.String())
		if segment != "" {
			segments = append(segments, segment)
		}
		builder.Reset()
	}

	for _, line := range strings.SplitAfter(content, "\n") {
		if len(line) > limit {
			flush()
			segments = append(segments, splitLongLineByByteLimit(line, limit)...)
			continue
		}
		if builder.Len() > 0 && builder.Len()+len(line) > limit {
			flush()
		}
		builder.WriteString(line)
	}
	flush()
	return segments
}

func splitLongLineByByteLimit(line string, limit int) []string {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	if limit <= 0 || len(line) <= limit {
		return []string{line}
	}

	segments := make([]string, 0)
	var builder strings.Builder
	flush := func() {
		segment := strings.TrimSpace(builder.String())
		if segment != "" {
			segments = append(segments, segment)
		}
		builder.Reset()
	}

	for _, r := range line {
		runeLen := utf8.RuneLen(r)
		if runeLen < 0 {
			runeLen = len(string(r))
		}
		if builder.Len() > 0 && builder.Len()+runeLen > limit {
			flush()
		}
		builder.WriteRune(r)
	}
	flush()
	return segments
}
