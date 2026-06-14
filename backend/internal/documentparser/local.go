package documentparser

import (
	"context"
	"fmt"
	"strings"
)

type LocalRequest struct {
	FileName   string
	FileType   string
	SourcePath string
	Content    []byte
}

func NormalizeLocal(ctx context.Context, req LocalRequest) (*NormalizedDocument, error) {
	_ = ctx

	fileType := NormalizeFileType(req.FileType)
	if !IsLocalType(fileType) {
		return nil, fmt.Errorf("local normalizer does not support file type: %s", fileType)
	}

	content := strings.TrimSpace(string(req.Content))
	if content == "" {
		return nil, fmt.Errorf("empty text content")
	}
	tables := []NormalizedTable(nil)
	if fileType == "md" || fileType == "markdown" {
		content, tables = NormalizeMarkdownPipeTables(content)
	}

	doc := &NormalizedDocument{
		ContentMarkdown: content,
		Source: NormalizedSource{
			FileName:   req.FileName,
			FileType:   fileType,
			SourcePath: req.SourcePath,
		},
		Quality: ParseQuality{
			Status: "ok",
			Score:  1,
		},
		Extractor: ExtractorInfo{
			Provider: "local",
			Version:  NormalizerVersion,
		},
	}

	if fileType == "md" || fileType == "markdown" {
		doc.Tables = tables
	}

	if err := doc.Validate(); err != nil {
		return nil, err
	}
	return doc, nil
}

// NormalizeMarkdownPipeTables canonicalizes Markdown pipe tables and returns
// table spans against the canonical Markdown. It is shared by local Markdown
// ingestion and parser-provider outputs so different source formats converge
// before chunking.
func NormalizeMarkdownPipeTables(markdown string) (string, []NormalizedTable) {
	lines := splitMarkdownLines(markdown)
	var out strings.Builder
	tables := make([]NormalizedTable, 0)

	for i := 0; i < len(lines); i++ {
		if !looksLikePipeRow(lines[i].Text) {
			out.WriteString(lines[i].Raw())
			continue
		}
		if i+1 >= len(lines) || !looksLikeSeparatorRow(lines[i+1].Text) {
			out.WriteString(lines[i].Raw())
			continue
		}

		j := i + 2
		for ; j < len(lines); j++ {
			if !looksLikePipeRow(lines[j].Text) {
				break
			}
		}

		tableLines := make([]string, 0, j-i)
		for _, line := range lines[i:j] {
			tableLines = append(tableLines, line.Text)
		}
		rows := pipeTableRows(tableLines)
		if len(rows) > 0 {
			start := out.Len()
			canonical := renderMarkdownPipeTable(rows)
			out.WriteString(canonical)
			if lines[j-1].HasNewline {
				out.WriteByte('\n')
			}
			end := start + len(canonical)
			tables = append(tables, NormalizedTable{
				ID:            fmt.Sprintf("table-%03d", len(tables)+1),
				MarkdownStart: start,
				MarkdownEnd:   end,
				Rows:          rows,
				Quality: TableQuality{
					Status: "ok",
				},
			})
		}
		i = j - 1
	}

	return strings.TrimSpace(out.String()), tables
}

type markdownLine struct {
	Text       string
	HasNewline bool
}

func (l markdownLine) Raw() string {
	if l.HasNewline {
		return l.Text + "\n"
	}
	return l.Text
}

func splitMarkdownLines(markdown string) []markdownLine {
	rawLines := strings.SplitAfter(markdown, "\n")
	lines := make([]markdownLine, 0, len(rawLines))
	for _, raw := range rawLines {
		if raw == "" {
			continue
		}
		hasNewline := strings.HasSuffix(raw, "\n")
		text := strings.TrimSuffix(raw, "\n")
		text = strings.TrimSuffix(text, "\r")
		lines = append(lines, markdownLine{Text: text, HasNewline: hasNewline})
	}
	if len(lines) == 0 && markdown != "" {
		lines = append(lines, markdownLine{Text: markdown})
	}
	return lines
}

func looksLikePipeRow(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.Count(trimmed, "|") >= 2 && strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|")
}

func looksLikeSeparatorRow(line string) bool {
	if !looksLikePipeRow(line) {
		return false
	}
	cells := splitPipeCells(line)
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		cleaned := strings.TrimSpace(strings.ReplaceAll(cell, ":", ""))
		if cleaned == "" || strings.Trim(cleaned, "-") != "" {
			return false
		}
	}
	return true
}

func pipeTableRows(lines []string) []TableRow {
	if len(lines) < 2 {
		return nil
	}

	header := splitPipeCells(lines[0])
	rows := make([]TableRow, 0, len(lines)-1)
	rows = append(rows, TableRow{Cells: cellsFromStrings(header, true)})

	for _, line := range lines[2:] {
		cells := splitPipeCells(line)
		if len(cells) == 0 {
			continue
		}
		rows = append(rows, TableRow{Cells: cellsFromStrings(cells, false)})
	}
	return rows
}

func splitPipeCells(line string) []string {
	trimmed := strings.Trim(strings.TrimSpace(line), "|")
	parts := strings.Split(trimmed, "|")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(part))
	}
	return cells
}

func cellsFromStrings(values []string, header bool) []TableCell {
	cells := make([]TableCell, 0, len(values))
	for _, value := range values {
		cells = append(cells, TableCell{
			Text:     cleanTableCellText(value),
			IsHeader: header,
		})
	}
	return cells
}

func cleanTableCellText(value string) string {
	cleaned := strings.TrimSpace(value)
	for {
		next := strings.TrimSpace(cleaned)
		next = strings.Trim(next, "*_`")
		next = strings.TrimSpace(next)
		if next == cleaned {
			break
		}
		cleaned = next
	}
	return strings.Join(strings.Fields(cleaned), " ")
}

func renderMarkdownPipeTable(rows []TableRow) string {
	if len(rows) == 0 {
		return ""
	}
	columnCount := 0
	for _, row := range rows {
		if len(row.Cells) > columnCount {
			columnCount = len(row.Cells)
		}
	}
	if columnCount == 0 {
		return ""
	}

	var builder strings.Builder
	writeRow := func(cells []TableCell) {
		builder.WriteString("|")
		for i := 0; i < columnCount; i++ {
			text := ""
			if i < len(cells) {
				text = cleanTableCellText(cells[i].Text)
			}
			builder.WriteString(" ")
			builder.WriteString(text)
			builder.WriteString(" |")
		}
		builder.WriteByte('\n')
	}

	writeRow(rows[0].Cells)
	builder.WriteString("|")
	for i := 0; i < columnCount; i++ {
		builder.WriteString(" --- |")
	}
	builder.WriteByte('\n')
	for _, row := range rows[1:] {
		writeRow(row.Cells)
	}
	return strings.TrimRight(builder.String(), "\n")
}
