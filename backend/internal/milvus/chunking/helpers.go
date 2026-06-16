package chunking

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
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

	chunkingParentChildMetadataVersion = "phase3-parent-child-v1"
	parentStrategyHeading              = "heading_section"
	parentStrategyTable                = "table"
	parentStrategyDocument             = "paragraph_window"
	defaultSectionTitle                = "Document"
)

type span struct {
	start int
	end   int
}

type chunkParentContext struct {
	id            string
	sectionTitle  string
	hierarchyPath string
	start         int
	end           int
	strategy      string
	tokenCount    int
	truncated     bool
}

func validSpan(content string, start, end int) bool {
	return start >= 0 && end > start && end <= len(content)
}

func validUTF8Span(content string, start, end int) bool {
	return validSpan(content, start, end) && utf8Boundary(content, start) && utf8Boundary(content, end)
}

func utf8Boundary(content string, offset int) bool {
	return offset == 0 || offset == len(content) || (offset > 0 && offset < len(content) && utf8.RuneStart(content[offset]))
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
	for start < end && !utf8Boundary(content, start) {
		start++
	}
	for end > start && !utf8Boundary(content, end) {
		end--
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

func finalizeChunks(req Request, chunks []*schema.Document) {
	if len(chunks) == 0 {
		return
	}
	content := ""
	if req.Document != nil {
		content = req.Document.ContentMarkdown
	}
	sections := markdownHeadingSections(content)

	for i, chunk := range chunks {
		if chunk == nil {
			continue
		}
		if chunk.MetaData == nil {
			chunk.MetaData = map[string]interface{}{}
		}
		for key, value := range cloneMetadata(req.BaseMeta) {
			if _, exists := chunk.MetaData[key]; !exists {
				chunk.MetaData[key] = value
			}
		}
		if req.NormalizedPath != "" {
			chunk.MetaData["normalized_path"] = req.NormalizedPath
		}

		child := resolveChunkChildSpan(content, chunk)
		chunk.MetaData["chunk_index"] = i
		chunk.MetaData["total_chunks"] = len(chunks)
		chunk.MetaData["child_start_offset"] = child.start
		chunk.MetaData["child_end_offset"] = child.end

		documentKey := resolveChunkDocumentKey(chunk.MetaData, content)
		chunkID := strings.TrimSpace(fmt.Sprint(chunk.MetaData["chunk_id"]))
		if chunkID == "" || chunkID == "<nil>" {
			chunkID = fmt.Sprintf("%s-child-%03d", documentKey, i)
			chunk.MetaData["chunk_id"] = chunkID
		}
		if strings.TrimSpace(fmt.Sprint(chunk.MetaData["child_id"])) == "" || strings.TrimSpace(fmt.Sprint(chunk.MetaData["child_id"])) == "<nil>" {
			chunk.MetaData["child_id"] = chunkID
		}
		if strings.TrimSpace(chunk.ID) == "" {
			chunk.ID = chunkID
		}

		parent := resolveChunkParentContext(content, chunk.MetaData, child, sections, documentKey)
		if parent.id == "" {
			chunk.MetaData["parent_child_available"] = false
			continue
		}

		chunk.MetaData["parent_id"] = parent.id
		chunk.MetaData["parent_start_offset"] = parent.start
		chunk.MetaData["parent_end_offset"] = parent.end
		chunk.MetaData["parent_child_available"] = true
		chunk.MetaData["parent_build_version"] = chunkingParentChildMetadataVersion
		chunk.MetaData["parent_build_strategy"] = parent.strategy
		chunk.MetaData["parent_token_count"] = parent.tokenCount
		chunk.MetaData["parent_truncated"] = parent.truncated
		if parent.sectionTitle != "" {
			chunk.MetaData["section_title"] = parent.sectionTitle
		}
		if parent.hierarchyPath != "" {
			chunk.MetaData["hierarchy_path"] = parent.hierarchyPath
		}
	}
}

func resolveChunkChildSpan(content string, chunk *schema.Document) span {
	if chunk == nil {
		return span{}
	}
	start := readIntMetadata(chunk.MetaData, "child_start_offset")
	end := readIntMetadata(chunk.MetaData, "child_end_offset")
	if validSpan(content, start, end) || (start == 0 && end == len(content) && len(content) > 0) {
		return span{start: start, end: end}
	}

	if strings.TrimSpace(content) != "" && strings.TrimSpace(chunk.Content) != "" {
		if idx := strings.Index(content, chunk.Content); idx >= 0 {
			return span{start: idx, end: idx + len(chunk.Content)}
		}
		trimmed := strings.TrimSpace(chunk.Content)
		if idx := strings.Index(content, trimmed); idx >= 0 {
			return span{start: idx, end: idx + len(trimmed)}
		}
	}
	if len(content) == 0 {
		return span{}
	}
	return span{start: 0, end: minInt(len(content), len(strings.TrimSpace(chunk.Content)))}
}

func resolveChunkParentContext(content string, metadata map[string]interface{}, child span, sections []markdownHeadingSection, documentKey string) chunkParentContext {
	if len(content) == 0 {
		return chunkParentContext{}
	}

	unit := strings.TrimSpace(fmt.Sprint(metadata["chunking_unit"]))
	section := findHeadingSectionForSpan(sections, child)
	parent := chunkParentContext{
		start:         0,
		end:           len(content),
		sectionTitle:  resolveChunkBaseTitle(metadata),
		hierarchyPath: resolveChunkBaseTitle(metadata),
		strategy:      parentStrategyDocument,
	}
	if section != nil {
		parent.start = section.start
		parent.end = section.end
		parent.sectionTitle = firstNonEmpty(readStringMetadata(metadata, "section_title"), section.title, parent.sectionTitle)
		parent.hierarchyPath = firstNonEmpty(readStringMetadata(metadata, "hierarchy_path"), strings.Join(section.hierarchy, " > "), parent.sectionTitle)
		parent.strategy = parentStrategyHeading
	}
	if unit == "table" && child.end > child.start {
		parent.start = child.start
		parent.end = child.end
		parent.strategy = parentStrategyTable
		parent.sectionTitle = firstNonEmpty(readStringMetadata(metadata, "section_title"), tableParentTitle(metadata), parent.sectionTitle)
		parent.hierarchyPath = firstNonEmpty(readStringMetadata(metadata, "hierarchy_path"), sectionHierarchy(section), parent.sectionTitle)
	}

	if parent.end <= parent.start {
		parent.start = child.start
		parent.end = child.end
	}
	if parent.end <= parent.start {
		return chunkParentContext{}
	}
	parent.id = fmt.Sprintf("%s-parent-%s-000", documentKey, shortHash(fmt.Sprintf("%s:%d:%d", parent.hierarchyPath, parent.start, parent.end)))
	parent.tokenCount = approximateTokenCount(sliceBySpan(content, parent.start, parent.end))
	parent.truncated = (parent.end - parent.start) > defaultStructuredChunkBytes
	return parent
}

func findHeadingSectionForSpan(sections []markdownHeadingSection, child span) *markdownHeadingSection {
	var best *markdownHeadingSection
	bestWidth := math.MaxInt
	for i := range sections {
		section := &sections[i]
		if section.start <= child.start && child.start < section.end {
			width := section.end - section.start
			if width < bestWidth {
				best = section
				bestWidth = width
			}
		}
	}
	return best
}

func tableParentTitle(metadata map[string]interface{}) string {
	if ids, ok := metadata["table_ids"].([]string); ok && len(ids) > 0 && strings.TrimSpace(ids[0]) != "" {
		return "Table " + strings.TrimSpace(ids[0])
	}
	if id := readStringMetadata(metadata, "table_id"); id != "" {
		return "Table " + id
	}
	return ""
}

func sectionHierarchy(section *markdownHeadingSection) string {
	if section == nil {
		return ""
	}
	return strings.Join(section.hierarchy, " > ")
}

func readStringMetadata(metadata map[string]interface{}, key string) string {
	if metadata == nil {
		return ""
	}
	value := strings.TrimSpace(fmt.Sprint(metadata[key]))
	if value == "<nil>" {
		return ""
	}
	return value
}

func resolveChunkDocumentKey(metadata map[string]interface{}, content string) string {
	for _, key := range []string{"document_id", "doc_id"} {
		if value := readStringMetadata(metadata, key); value != "" {
			return "doc-" + sanitizeIDComponent(value)
		}
	}
	for _, key := range []string{"file_name", "title"} {
		if value := readStringMetadata(metadata, key); value != "" {
			return sanitizeIDComponent(value) + "-" + shortHash(content)
		}
	}
	if strings.TrimSpace(content) == "" {
		return "document"
	}
	return "document-" + shortHash(content)
}

func resolveChunkBaseTitle(metadata map[string]interface{}) string {
	for _, key := range []string{"title", "file_name"} {
		if value := readStringMetadata(metadata, key); value != "" {
			return value
		}
	}
	return defaultSectionTitle
}

func sanitizeIDComponent(input string) string {
	trimmed := strings.TrimSpace(strings.ToLower(input))
	if trimmed == "" {
		return "document"
	}
	replacer := strings.NewReplacer(
		" ", "-",
		"/", "-",
		"\\", "-",
		":", "-",
		".", "-",
		"#", "-",
		">", "-",
	)
	cleaned := strings.Trim(replacer.Replace(trimmed), "-")
	if cleaned == "" {
		return "document"
	}
	return cleaned
}

func shortHash(input string) string {
	sum := sha1.Sum([]byte(input))
	return hex.EncodeToString(sum[:4])
}

func approximateTokenCount(content string) int {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return 0
	}
	fields := strings.Fields(trimmed)
	if len(fields) > 1 {
		return len(fields)
	}
	return maxInt(1, int(math.Ceil(float64(len([]rune(trimmed)))/4.0)))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
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
