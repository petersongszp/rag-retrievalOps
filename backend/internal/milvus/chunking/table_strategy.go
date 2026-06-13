package chunking

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	"interview-agents/internal/documentparser"
)

type TableAwareStrategy struct {
	delegate Strategy
}

func NewTableAwareStrategy(delegate Strategy) *TableAwareStrategy {
	return &TableAwareStrategy{delegate: delegate}
}

func (s *TableAwareStrategy) Split(ctx context.Context, req Request) ([]*schema.Document, error) {
	if s == nil || s.delegate == nil {
		return nil, fmt.Errorf("table-aware strategy delegate is not initialized")
	}
	if req.Document == nil {
		return nil, fmt.Errorf("normalized document is nil")
	}
	chunks, err := s.delegate.Split(ctx, req)
	if err != nil {
		return nil, err
	}
	for _, table := range req.Document.Tables {
		if !validSpan(req.Document.ContentMarkdown, table.MarkdownStart, table.MarkdownEnd) {
			continue
		}
		content := buildTableRetrievalContent(req.Document.ContentMarkdown, table)
		if strings.TrimSpace(content) == "" {
			continue
		}
		meta := cloneMetadata(req.BaseMeta)
		meta["chunking_strategy"] = StrategyTableAware
		meta["chunking_unit"] = "table"
		meta["normalized_path"] = req.NormalizedPath
		meta["table_ids"] = []string{table.ID}
		meta["child_start_offset"] = table.MarkdownStart
		meta["child_end_offset"] = table.MarkdownEnd
		meta["table_row_count"] = len(table.Rows)
		if table.Page > 0 {
			meta["page_start"] = table.Page
			meta["page_end"] = table.Page
		}
		if table.Quality.Status != "" {
			meta["table_quality_status"] = table.Quality.Status
		}
		if table.MergedCells {
			meta["table_merged_cells"] = true
		}
		if table.Nested {
			meta["table_nested"] = true
		}
		segments := splitContentByByteLimit(content, defaultMaxChunkContentBytes)
		for index, segment := range segments {
			segmentMeta := cloneMetadata(meta)
			if len(segments) > 1 {
				segmentMeta["table_segment_index"] = index
				segmentMeta["table_segment_count"] = len(segments)
			}
			chunks = append(chunks, &schema.Document{
				Content:  segment,
				MetaData: segmentMeta,
			})
		}
	}
	if MatchOCRAware(req.Document) {
		annotateOCRQuality(chunks, req.Document, defaultOCRConfidenceThreshold, false)
	}
	finalizeChunkIndexes(chunks)
	return chunks, nil
}

func buildTableRetrievalContent(markdown string, table documentparser.NormalizedTable) string {
	tableMarkdown := sliceBySpan(markdown, table.MarkdownStart, table.MarkdownEnd)
	rows := renderTableRows(table.Rows)
	parts := []string{fmt.Sprintf("Table %s", table.ID)}
	if tableMarkdown != "" {
		parts = append(parts, tableMarkdown)
	}
	if rows != "" {
		parts = append(parts, "Rows:\n"+rows)
	}
	return strings.Join(parts, "\n\n")
}

func renderTableRows(rows []documentparser.TableRow) string {
	if len(rows) == 0 {
		return ""
	}
	headers := make([]string, 0)
	for _, cell := range rows[0].Cells {
		headers = append(headers, strings.TrimSpace(cell.Text))
	}

	lines := make([]string, 0, len(rows)-1)
	for _, row := range rows[1:] {
		values := make([]string, 0, len(row.Cells))
		for i, cell := range row.Cells {
			text := strings.TrimSpace(cell.Text)
			if i < len(headers) && headers[i] != "" {
				values = append(values, fmt.Sprintf("%s: %s", headers[i], text))
				continue
			}
			values = append(values, text)
		}
		line := strings.TrimSpace(strings.Join(values, " | "))
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}
