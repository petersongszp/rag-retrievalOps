package documentparser

import (
	"math"

	"github.com/cloudwego/eino/schema"
)

func AnnotateChunksWithProvenance(chunks []*schema.Document, normalized *NormalizedDocument, normalizedPath string) {
	if normalized == nil {
		return
	}
	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}
		if chunk.MetaData == nil {
			chunk.MetaData = map[string]interface{}{}
		}
		chunk.MetaData["normalized_path"] = normalizedPath

		start := readIntMetadata(chunk.MetaData, "child_start_offset")
		end := readIntMetadata(chunk.MetaData, "child_end_offset")
		if end < start {
			end = start
		}

		blockIDs, pageStart, pageEnd, confidence := overlappingBlocks(normalized.Blocks, start, end)
		if len(blockIDs) > 0 {
			chunk.MetaData["block_ids"] = blockIDs
		}
		if pageStart > 0 {
			chunk.MetaData["page_start"] = pageStart
			chunk.MetaData["page_end"] = pageEnd
		}
		if confidence > 0 {
			chunk.MetaData["ocr_confidence"] = confidence
		}

		tableIDs := overlappingTables(normalized.Tables, start, end)
		if len(tableIDs) > 0 {
			chunk.MetaData["table_ids"] = tableIDs
		}
	}
}

func overlappingBlocks(blocks []NormalizedBlock, start, end int) ([]string, int, int, float64) {
	ids := make([]string, 0)
	pageStart := 0
	pageEnd := 0
	confidenceSum := 0.0
	confidenceCount := 0

	for _, block := range blocks {
		if !rangesOverlap(start, end, block.MarkdownStart, block.MarkdownEnd) {
			continue
		}
		ids = append(ids, block.ID)
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

	confidence := 0.0
	if confidenceCount > 0 {
		confidence = math.Round((confidenceSum/float64(confidenceCount))*10000) / 10000
	}
	return ids, pageStart, pageEnd, confidence
}

func overlappingTables(tables []NormalizedTable, start, end int) []string {
	ids := make([]string, 0)
	for _, table := range tables {
		if rangesOverlap(start, end, table.MarkdownStart, table.MarkdownEnd) {
			ids = append(ids, table.ID)
		}
	}
	return ids
}

func rangesOverlap(leftStart, leftEnd, rightStart, rightEnd int) bool {
	return leftStart <= rightEnd && rightStart <= leftEnd
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
