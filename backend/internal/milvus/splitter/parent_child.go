package splitter

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"math"
	"strings"

	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
	"github.com/cloudwego/eino/schema"
)

const (
	parentChildMetadataVersion = "phase3-parent-child-v1"
	parentStrategyHeading      = "heading_section"
	parentStrategyHeadingWin   = "heading_section_window"
	parentStrategyParagraph    = "paragraph_window"
	defaultSectionTitle        = "Document"
)

type textSpan struct {
	Start int
	End   int
}

type headingSection struct {
	Level         int
	Title         string
	HierarchyPath string
	Start         int
	End           int
}

type parentBlock struct {
	ID            string
	SectionTitle  string
	HierarchyPath string
	Start         int
	End           int
	Strategy      string
	TokenCount    int
	Truncated     bool
}

func (s *DocumentSplitterService) annotateSplitChunks(original *schema.Document, chunks []*schema.Document) []*schema.Document {
	if len(chunks) == 0 {
		return chunks
	}

	originalContent := ""
	baseMeta := map[string]interface{}{}
	if original != nil {
		originalContent = original.Content
		baseMeta = cloneMetadataMap(original.MetaData)
	}

	chunkSpans := locateChunkOffsets(originalContent, chunks, s.config)
	blocks := buildParentBlocks(originalContent, baseMeta, chunkSpans, s.config)
	documentKey := resolveDocumentKey(baseMeta, originalContent)

	for i, chunk := range chunks {
		if chunk == nil {
			continue
		}

		mergedMeta := cloneMetadataMap(baseMeta)
		for key, value := range cloneMetadataMap(chunk.MetaData) {
			mergedMeta[key] = value
		}

		chunkID := fmt.Sprintf("%s-child-%03d", documentKey, i)
		block := pickParentBlock(blocks, chunkSpans[i])
		parentID := ""
		sectionTitle := resolveBaseTitle(baseMeta)
		hierarchyPath := sectionTitle
		parentStart := 0
		parentEnd := 0
		parentTokenCount := 0
		parentStrategy := parentStrategyParagraph
		parentTruncated := false
		parentAvailable := false

		if block != nil {
			parentID = block.ID
			sectionTitle = block.SectionTitle
			hierarchyPath = block.HierarchyPath
			parentStart = block.Start
			parentEnd = block.End
			parentTokenCount = block.TokenCount
			parentStrategy = block.Strategy
			parentTruncated = block.Truncated
			parentAvailable = true
		}

		mergedMeta["chunk_index"] = i
		mergedMeta["total_chunks"] = len(chunks)
		mergedMeta["chunk_id"] = chunkID
		mergedMeta["child_id"] = chunkID
		mergedMeta["child_start_offset"] = chunkSpans[i].Start
		mergedMeta["child_end_offset"] = chunkSpans[i].End
		mergedMeta["section_title"] = sectionTitle
		mergedMeta["hierarchy_path"] = hierarchyPath
		mergedMeta["parent_child_available"] = parentAvailable
		mergedMeta["parent_build_version"] = parentChildMetadataVersion
		mergedMeta["parent_build_strategy"] = parentStrategy
		mergedMeta["parent_truncated"] = parentTruncated

		if parentAvailable {
			mergedMeta["parent_id"] = parentID
			mergedMeta["parent_start_offset"] = parentStart
			mergedMeta["parent_end_offset"] = parentEnd
			mergedMeta["parent_token_count"] = parentTokenCount
		}

		chunk.MetaData = mergedMeta
		if strings.TrimSpace(chunk.ID) == "" {
			chunk.ID = chunkID
		}
	}

	return chunks
}

func locateChunkOffsets(content string, chunks []*schema.Document, cfg *recursive.Config) []textSpan {
	spans := make([]textSpan, len(chunks))
	if strings.TrimSpace(content) == "" {
		return spans
	}

	searchFrom := 0
	lastStart := 0
	overlapHint := 0
	if cfg != nil && cfg.OverlapSize > 0 {
		overlapHint = cfg.OverlapSize
	}

	for i, chunk := range chunks {
		if chunk == nil {
			continue
		}
		piece := chunk.Content
		start := findChunkOffset(content, piece, searchFrom)
		if start < 0 && i > 0 {
			retryFrom := maxInt(0, spans[i-1].End-overlapHint-len(piece))
			start = findChunkOffset(content, piece, retryFrom)
		}
		if start < 0 {
			start = findChunkOffset(content, piece, 0)
		}
		if start < 0 {
			start = minInt(searchFrom, len(content))
		}

		end := start + len(piece)
		if end > len(content) {
			end = len(content)
		}
		if end < start {
			end = start
		}

		spans[i] = textSpan{Start: start, End: end}
		lastStart = start
		searchFrom = maxInt(lastStart+1, end-overlapHint)
	}

	return spans
}

func findChunkOffset(content, piece string, from int) int {
	if strings.TrimSpace(content) == "" || strings.TrimSpace(piece) == "" {
		return -1
	}
	if from < 0 {
		from = 0
	}
	if from >= len(content) {
		from = len(content) - 1
	}
	if from < 0 {
		from = 0
	}

	if idx := strings.Index(content[from:], piece); idx >= 0 {
		return from + idx
	}

	trimmed := strings.TrimSpace(piece)
	if trimmed != "" {
		if idx := strings.Index(content[from:], trimmed); idx >= 0 {
			return from + idx
		}
	}

	return -1
}

func buildParentBlocks(content string, baseMeta map[string]interface{}, childSpans []textSpan, cfg *recursive.Config) []*parentBlock {
	if strings.TrimSpace(content) == "" {
		return nil
	}

	parentCap := resolveParentCharCap(cfg)
	paragraphs := extractParagraphSpans(content)
	sections := extractHeadingSections(content)
	blocks := make([]*parentBlock, 0)
	documentKey := resolveDocumentKey(baseMeta, content)
	baseTitle := resolveBaseTitle(baseMeta)

	if len(sections) == 0 {
		baseBlock := splitRegionIntoParentBlocks(
			documentKey,
			baseTitle,
			baseTitle,
			parentStrategyParagraph,
			textSpan{Start: 0, End: len(content)},
			paragraphs,
			parentCap,
			content,
		)
		return baseBlock
	}

	for _, section := range sections {
		sectionSpan := textSpan{Start: section.Start, End: section.End}
		sectionParagraphs := filterParagraphsWithin(paragraphs, sectionSpan)
		strategy := parentStrategyHeading
		if section.End-section.Start > parentCap {
			strategy = parentStrategyHeadingWin
		}
		sectionBlocks := splitRegionIntoParentBlocks(
			documentKey,
			section.Title,
			section.HierarchyPath,
			strategy,
			sectionSpan,
			sectionParagraphs,
			parentCap,
			content,
		)
		blocks = append(blocks, sectionBlocks...)
	}

	if len(blocks) == 0 {
		return splitRegionIntoParentBlocks(
			documentKey,
			baseTitle,
			baseTitle,
			parentStrategyParagraph,
			textSpan{Start: 0, End: len(content)},
			paragraphs,
			parentCap,
			content,
		)
	}
	return blocks
}

func splitRegionIntoParentBlocks(
	documentKey string,
	sectionTitle string,
	hierarchyPath string,
	strategy string,
	region textSpan,
	paragraphs []textSpan,
	parentCap int,
	content string,
) []*parentBlock {
	if region.End < region.Start {
		region.End = region.Start
	}
	if parentCap <= 0 {
		parentCap = region.End - region.Start
	}

	var spans []textSpan
	if len(paragraphs) == 0 || region.End-region.Start <= parentCap {
		spans = []textSpan{region}
	} else {
		blockStart := region.Start
		blockEnd := region.Start
		for _, paragraph := range paragraphs {
			candidateEnd := paragraph.End
			if candidateEnd-blockStart > parentCap && blockEnd > blockStart {
				spans = append(spans, textSpan{Start: blockStart, End: blockEnd})
				blockStart = paragraph.Start
			}
			blockEnd = candidateEnd
		}
		if blockEnd <= blockStart {
			blockEnd = region.End
		}
		spans = append(spans, textSpan{Start: blockStart, End: minInt(blockEnd, region.End)})
	}

	blocks := make([]*parentBlock, 0, len(spans))
	for idx, span := range spans {
		if span.Start < region.Start {
			span.Start = region.Start
		}
		if span.End > region.End {
			span.End = region.End
		}
		if span.End <= span.Start {
			span.End = minInt(region.End, span.Start)
		}
		spanContent := sliceContent(content, span)
		blockID := fmt.Sprintf("%s-parent-%s-%03d", documentKey, shortHash(fmt.Sprintf("%s:%d:%d", hierarchyPath, span.Start, span.End)), idx)
		blocks = append(blocks, &parentBlock{
			ID:            blockID,
			SectionTitle:  firstNonEmptyString(sectionTitle, defaultSectionTitle),
			HierarchyPath: firstNonEmptyString(hierarchyPath, sectionTitle, defaultSectionTitle),
			Start:         span.Start,
			End:           span.End,
			Strategy:      strategy,
			TokenCount:    approximateTokenCount(spanContent),
			Truncated:     len(spans) > 1 || (region.End-region.Start) > (span.End-span.Start),
		})
	}

	return blocks
}

func pickParentBlock(blocks []*parentBlock, child textSpan) *parentBlock {
	if len(blocks) == 0 {
		return nil
	}

	var bestContaining *parentBlock
	bestContainingWidth := math.MaxInt
	for _, block := range blocks {
		if block == nil {
			continue
		}
		if child.Start >= block.Start && child.Start < block.End {
			width := block.End - block.Start
			if width < bestContainingWidth {
				bestContaining = block
				bestContainingWidth = width
			}
		}
	}
	if bestContaining != nil {
		return bestContaining
	}

	childCenter := child.Start + (child.End-child.Start)/2
	best := blocks[0]
	bestDistance := math.MaxInt
	for _, block := range blocks {
		if block == nil {
			continue
		}
		distance := 0
		switch {
		case childCenter < block.Start:
			distance = block.Start - childCenter
		case childCenter > block.End:
			distance = childCenter - block.End
		default:
			distance = 0
		}
		if distance < bestDistance {
			best = block
			bestDistance = distance
		}
	}
	return best
}

func extractHeadingSections(content string) []headingSection {
	type heading struct {
		level int
		title string
		start int
		path  string
	}

	headings := make([]heading, 0)
	stack := make([]heading, 0, 6)
	forEachLine(content, func(start, _ int, line string) {
		level, title, ok := parseMarkdownHeading(line)
		if !ok {
			return
		}
		for len(stack) > 0 && stack[len(stack)-1].level >= level {
			stack = stack[:len(stack)-1]
		}
		entry := heading{
			level: level,
			title: title,
			start: start,
		}
		stack = append(stack, entry)
		pathParts := make([]string, 0, len(stack))
		for _, item := range stack {
			pathParts = append(pathParts, item.title)
		}
		entry.path = strings.Join(pathParts, " > ")
		headings = append(headings, entry)
		stack[len(stack)-1] = entry
	})

	sections := make([]headingSection, 0, len(headings))
	for i, item := range headings {
		end := len(content)
		for j := i + 1; j < len(headings); j++ {
			if headings[j].level <= item.level {
				end = headings[j].start
				break
			}
		}
		sections = append(sections, headingSection{
			Level:         item.level,
			Title:         item.title,
			HierarchyPath: item.path,
			Start:         item.start,
			End:           end,
		})
	}
	return sections
}

func parseMarkdownHeading(line string) (int, string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "#") {
		return 0, "", false
	}

	level := 0
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level > 6 {
		return 0, "", false
	}
	if len(trimmed) <= level || trimmed[level] != ' ' {
		return 0, "", false
	}

	title := strings.TrimSpace(trimmed[level:])
	if title == "" {
		return 0, "", false
	}
	return level, title, true
}

func extractParagraphSpans(content string) []textSpan {
	paragraphs := make([]textSpan, 0)
	inParagraph := false
	start := 0
	end := 0

	forEachLine(content, func(lineStart, lineEnd int, line string) {
		if strings.TrimSpace(line) == "" {
			if inParagraph {
				paragraphs = append(paragraphs, textSpan{Start: start, End: end})
				inParagraph = false
			}
			return
		}
		if !inParagraph {
			start = lineStart
			inParagraph = true
		}
		end = lineEnd
	})

	if inParagraph {
		paragraphs = append(paragraphs, textSpan{Start: start, End: end})
	}
	if len(paragraphs) == 0 && strings.TrimSpace(content) != "" {
		return []textSpan{{Start: 0, End: len(content)}}
	}
	return paragraphs
}

func filterParagraphsWithin(paragraphs []textSpan, region textSpan) []textSpan {
	out := make([]textSpan, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		if paragraph.End <= region.Start {
			continue
		}
		if paragraph.Start >= region.End {
			continue
		}
		start := maxInt(paragraph.Start, region.Start)
		end := minInt(paragraph.End, region.End)
		if end > start {
			out = append(out, textSpan{Start: start, End: end})
		}
	}
	return out
}

func forEachLine(content string, fn func(start, end int, line string)) {
	start := 0
	for start <= len(content) {
		if start == len(content) {
			break
		}
		remaining := content[start:]
		idx := strings.IndexByte(remaining, '\n')
		if idx < 0 {
			end := len(content)
			fn(start, end, strings.TrimSuffix(content[start:end], "\r"))
			break
		}
		end := start + idx + 1
		fn(start, end, strings.TrimSuffix(content[start:end], "\r\n"))
		start = end
	}
}

func resolveParentCharCap(cfg *recursive.Config) int {
	chunkSize := 1000
	if cfg != nil && cfg.ChunkSize > 0 {
		chunkSize = cfg.ChunkSize
	}
	return maxInt(chunkSize*3, chunkSize)
}

func resolveDocumentKey(meta map[string]interface{}, content string) string {
	if meta != nil {
		for _, key := range []string{"document_id", "doc_id"} {
			if value := strings.TrimSpace(fmt.Sprint(meta[key])); value != "" && value != "<nil>" {
				return "doc-" + sanitizeIDComponent(value)
			}
		}
		for _, key := range []string{"file_name", "title"} {
			if value := strings.TrimSpace(fmt.Sprint(meta[key])); value != "" && value != "<nil>" {
				return sanitizeIDComponent(value) + "-" + shortHash(content)
			}
		}
	}
	if strings.TrimSpace(content) == "" {
		return "document"
	}
	return "document-" + shortHash(content)
}

func resolveBaseTitle(meta map[string]interface{}) string {
	if meta != nil {
		for _, key := range []string{"title", "file_name"} {
			if value := strings.TrimSpace(fmt.Sprint(meta[key])); value != "" && value != "<nil>" {
				return value
			}
		}
	}
	return defaultSectionTitle
}

func cloneMetadataMap(input map[string]interface{}) map[string]interface{} {
	if len(input) == 0 {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
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
	cleaned := replacer.Replace(trimmed)
	cleaned = strings.Trim(cleaned, "-")
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
	runeCount := len([]rune(trimmed))
	return maxInt(1, int(math.Ceil(float64(runeCount)/4.0)))
}

func sliceContent(content string, span textSpan) string {
	if span.Start < 0 {
		span.Start = 0
	}
	if span.End > len(content) {
		span.End = len(content)
	}
	if span.End < span.Start {
		span.End = span.Start
	}
	return content[span.Start:span.End]
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
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
