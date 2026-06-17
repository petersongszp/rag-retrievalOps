package chunking

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"
	"github.com/yuin/goldmark"
	goldmarkast "github.com/yuin/goldmark/ast"
	goldmarktext "github.com/yuin/goldmark/text"

	"interview-agents/internal/documentparser"
)

type MarkdownSplitter interface {
	SplitMarkdownDocument(ctx context.Context, doc *schema.Document) ([]*schema.Document, error)
}

type MarkdownStrategy struct {
	splitter MarkdownSplitter
}

type markdownHeadingSection struct {
	start     int
	end       int
	level     int
	title     string
	hierarchy []string
}

type markdownSegment struct {
	content string
	start   int
	end     int
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

	if chunks, ok := splitMarkdownByHeadingStructure(req); ok {
		return chunks, nil
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
	finalizeChunks(req, chunks)
	return chunks, nil
}

func splitMarkdownByHeadingStructure(req Request) ([]*schema.Document, bool) {
	content := req.Document.ContentMarkdown
	sections := markdownHeadingSections(content)
	if len(sections) == 0 {
		return nil, false
	}

	chunks := make([]*schema.Document, 0, len(sections))
	for _, section := range sections {
		if section.start < 0 || section.end > len(content) || section.start >= section.end {
			continue
		}
		rawSection := content[section.start:section.end]
		if !markdownSectionHasIndexableBody(rawSection) {
			continue
		}

		segments := splitMarkdownSectionByByteLimit(rawSection, section.start, defaultStructuredChunkBytes)
		for segmentIndex, segment := range segments {
			if strings.TrimSpace(segment.content) == "" {
				continue
			}
			meta := cloneMetadata(req.BaseMeta)
			meta["chunking_strategy"] = StrategyMarkdown
			meta["chunking_unit"] = markdownChunkingUnit(section.title)
			meta["child_start_offset"] = segment.start
			meta["child_end_offset"] = segment.end
			if req.NormalizedPath != "" {
				meta["normalized_path"] = req.NormalizedPath
			}
			if section.title != "" {
				meta["section_title"] = section.title
			}
			if len(section.hierarchy) > 0 {
				meta["hierarchy_path"] = strings.Join(section.hierarchy, " > ")
			}
			if section.level > 0 {
				meta["heading_level"] = section.level
			}
			if len(segments) > 1 {
				meta["section_segment_index"] = segmentIndex
				meta["section_segment_count"] = len(segments)
			}
			chunks = append(chunks, &schema.Document{
				Content:  segment.content,
				MetaData: meta,
			})
		}
	}
	if len(chunks) == 0 {
		return nil, false
	}

	documentparser.AnnotateChunksWithProvenance(chunks, req.Document, req.NormalizedPath)
	finalizeChunks(req, chunks)
	return chunks, true
}

func markdownHeadingSections(content string) []markdownHeadingSection {
	source := []byte(content)
	root := goldmark.New().Parser().Parse(goldmarktext.NewReader(source))
	sections := make([]markdownHeadingSection, 0)
	hierarchy := make([]string, 6)

	_ = goldmarkast.Walk(root, func(node goldmarkast.Node, entering bool) (goldmarkast.WalkStatus, error) {
		if !entering {
			return goldmarkast.WalkContinue, nil
		}
		heading, ok := node.(*goldmarkast.Heading)
		if !ok {
			return goldmarkast.WalkContinue, nil
		}
		start := headingSourceStart(content, heading)
		if len(sections) == 0 && start > 0 && strings.TrimSpace(content[:start]) != "" {
			sections = append(sections, markdownHeadingSection{start: 0, end: start})
		}
		if len(sections) > 0 {
			sections[len(sections)-1].end = start
		}

		level := heading.Level
		title := normalizeMarkdownHeadingTitle(string(heading.Text(source)))
		if title == "" {
			level, title, ok = parseMarkdownHeading(sliceMarkdownLineAt(content, start))
			if !ok {
				return goldmarkast.WalkContinue, nil
			}
		}

		if level <= len(hierarchy) {
			hierarchy[level-1] = title
			for i := level; i < len(hierarchy); i++ {
				hierarchy[i] = ""
			}
		}

		path := make([]string, 0, level)
		for i := 0; i < level && i < len(hierarchy); i++ {
			if hierarchy[i] != "" {
				path = append(path, hierarchy[i])
			}
		}
		sections = append(sections, markdownHeadingSection{
			start:     start,
			level:     level,
			title:     title,
			hierarchy: path,
		})
		return goldmarkast.WalkSkipChildren, nil
	})

	if len(sections) == 0 {
		return nil
	}
	sections[len(sections)-1].end = len(content)
	return sections
}

func headingSourceStart(content string, heading *goldmarkast.Heading) int {
	if heading == nil || heading.Lines() == nil || heading.Lines().Len() == 0 {
		return 0
	}
	start := heading.Lines().At(0).Start
	if start < 0 {
		start = 0
	}
	if start > len(content) {
		start = len(content)
	}
	for start > 0 && content[start-1] != '\n' {
		start--
	}
	return start
}

func sliceMarkdownLineAt(content string, start int) string {
	if start < 0 {
		start = 0
	}
	if start >= len(content) {
		return ""
	}
	end := strings.IndexByte(content[start:], '\n')
	if end < 0 {
		return content[start:]
	}
	return content[start : start+end]
}

func markdownHeadingSectionsByLineScan(content string) []markdownHeadingSection {
	sections := make([]markdownHeadingSection, 0)
	hierarchy := make([]string, 6)
	var current *markdownHeadingSection

	forEachMarkdownLine(content, func(start, end int, line string) {
		level, title, ok := parseMarkdownHeading(line)
		if !ok {
			return
		}

		if current == nil && start > 0 && strings.TrimSpace(content[:start]) != "" {
			sections = append(sections, markdownHeadingSection{
				start: 0,
				end:   start,
			})
		}
		if current != nil {
			current.end = start
			sections = append(sections, *current)
		}

		if level <= len(hierarchy) {
			hierarchy[level-1] = title
			for i := level; i < len(hierarchy); i++ {
				hierarchy[i] = ""
			}
		}

		path := make([]string, 0, level)
		for i := 0; i < level && i < len(hierarchy); i++ {
			if hierarchy[i] != "" {
				path = append(path, hierarchy[i])
			}
		}
		current = &markdownHeadingSection{
			start:     start,
			end:       end,
			level:     level,
			title:     title,
			hierarchy: path,
		}
	})

	if current != nil {
		current.end = len(content)
		sections = append(sections, *current)
	}
	return sections
}

func forEachMarkdownLine(content string, fn func(start, end int, line string)) {
	for start := 0; start < len(content); {
		newline := strings.IndexByte(content[start:], '\n')
		if newline < 0 {
			fn(start, len(content), content[start:])
			return
		}
		end := start + newline + 1
		fn(start, end, content[start:end])
		start = end
	}
}

func parseMarkdownHeading(line string) (int, string, bool) {
	trimmedRight := strings.TrimRight(line, "\r\n")
	trimmedLeft := strings.TrimLeft(trimmedRight, " \t")
	if len(trimmedRight)-len(trimmedLeft) > 3 || !strings.HasPrefix(trimmedLeft, "#") {
		return 0, "", false
	}

	level := 0
	for level < len(trimmedLeft) && trimmedLeft[level] == '#' {
		level++
	}
	if level == 0 || level > 6 {
		return 0, "", false
	}
	if len(trimmedLeft) > level && trimmedLeft[level] != ' ' && trimmedLeft[level] != '\t' {
		return 0, "", false
	}

	rawTitle := trimMarkdownClosingHashes(trimmedLeft[level:])
	title := normalizeMarkdownHeadingTitle(rawTitle)
	if title == "" {
		return 0, "", false
	}
	return level, title, true
}

func trimMarkdownClosingHashes(title string) string {
	title = strings.TrimSpace(title)
	hashStart := len(title)
	for hashStart > 0 && title[hashStart-1] == '#' {
		hashStart--
	}
	if hashStart == len(title) || hashStart == 0 {
		return title
	}
	if title[hashStart-1] != ' ' && title[hashStart-1] != '\t' {
		return title
	}
	return strings.TrimSpace(title[:hashStart-1])
}

func normalizeMarkdownHeadingTitle(title string) string {
	for {
		normalized := strings.TrimSpace(title)
		normalized = strings.Trim(normalized, "*_`")
		normalized = strings.TrimSpace(normalized)
		if normalized == title {
			break
		}
		title = normalized
	}
	return strings.Join(strings.Fields(title), " ")
}

func markdownSectionHasIndexableBody(content string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return false
	}
	newline := strings.IndexByte(trimmed, '\n')
	if newline < 0 {
		_, _, isHeading := parseMarkdownHeading(trimmed)
		return !isHeading
	}
	firstLine := trimmed[:newline]
	if _, _, isHeading := parseMarkdownHeading(firstLine); !isHeading {
		return true
	}
	return strings.TrimSpace(trimmed[newline+1:]) != ""
}

func markdownChunkingUnit(title string) string {
	if strings.Contains(title, "计费公式") {
		return "formula"
	}
	return "markdown_heading_section"
}

func splitMarkdownSectionByByteLimit(content string, absoluteStart int, limit int) []markdownSegment {
	content = strings.TrimRight(content, "\x00")
	if strings.TrimSpace(content) == "" {
		return nil
	}
	if limit <= 0 {
		limit = defaultMaxChunkContentBytes
	}

	segments := make([]markdownSegment, 0)
	var builder strings.Builder
	builderStart := 0

	flush := func() {
		if builder.Len() == 0 {
			return
		}
		raw := builder.String()
		trimmed := strings.TrimSpace(raw)
		if trimmed != "" {
			leading := strings.Index(raw, trimmed)
			if leading < 0 {
				leading = 0
			}
			start := absoluteStart + builderStart + leading
			segments = append(segments, markdownSegment{
				content: trimmed,
				start:   start,
				end:     start + len(trimmed),
			})
		}
		builder.Reset()
		builderStart = 0
	}

	appendPiece := func(piece string, start, end int) {
		if piece == "" {
			return
		}
		if builder.Len() == 0 {
			builderStart = start
		}
		builder.WriteString(piece)
	}

	forEachMarkdownLine(content, func(start, end int, line string) {
		if len(line) > limit {
			flush()
			segments = append(segments, splitLongMarkdownLineByByteLimit(line, absoluteStart+start, limit)...)
			return
		}
		if builder.Len() > 0 && builder.Len()+len(line) > limit {
			flush()
		}
		appendPiece(line, start, end)
	})
	flush()
	return segments
}

func splitLongMarkdownLineByByteLimit(line string, absoluteStart int, limit int) []markdownSegment {
	segments := make([]markdownSegment, 0)
	var builder strings.Builder
	builderStart := 0
	offset := 0

	flush := func(end int) {
		if builder.Len() == 0 {
			return
		}
		raw := builder.String()
		trimmed := strings.TrimSpace(raw)
		if trimmed != "" {
			leading := strings.Index(raw, trimmed)
			if leading < 0 {
				leading = 0
			}
			start := absoluteStart + builderStart + leading
			segments = append(segments, markdownSegment{
				content: trimmed,
				start:   start,
				end:     start + len(trimmed),
			})
		}
		builder.Reset()
		builderStart = end
	}

	for _, r := range line {
		runeLen := utf8.RuneLen(r)
		if runeLen < 0 {
			runeLen = len(string(r))
		}
		if builder.Len() > 0 && builder.Len()+runeLen > limit {
			flush(offset)
		}
		if builder.Len() == 0 {
			builderStart = offset
		}
		builder.WriteRune(r)
		offset += runeLen
	}
	flush(offset)
	return segments
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
