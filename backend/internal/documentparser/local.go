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
		doc.Tables = parseMarkdownPipeTables(content)
	}

	if err := doc.Validate(); err != nil {
		return nil, err
	}
	return doc, nil
}

func parseMarkdownPipeTables(markdown string) []NormalizedTable {
	lines := strings.Split(markdown, "\n")
	lineStarts := make([]int, len(lines))
	offset := 0
	for i, line := range lines {
		lineStarts[i] = offset
		offset += len(line) + 1
	}

	tables := make([]NormalizedTable, 0)
	for i := 0; i < len(lines); i++ {
		if !looksLikePipeRow(lines[i]) {
			continue
		}
		if i+1 >= len(lines) || !looksLikeSeparatorRow(lines[i+1]) {
			continue
		}

		j := i + 2
		for ; j < len(lines); j++ {
			if !looksLikePipeRow(lines[j]) {
				break
			}
		}

		tableLines := lines[i:j]
		rows := pipeTableRows(tableLines)
		if len(rows) > 0 {
			lastLine := j - 1
			tables = append(tables, NormalizedTable{
				ID:            fmt.Sprintf("table-%03d", len(tables)+1),
				MarkdownStart: lineStarts[i],
				MarkdownEnd:   lineStarts[lastLine] + len(lines[lastLine]),
				Rows:          rows,
				Quality: TableQuality{
					Status: "ok",
				},
			})
		}
		i = j - 1
	}

	return tables
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
			Text:     value,
			IsHeader: header,
		})
	}
	return cells
}
