package parserprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"interview-agents/internal/documentparser"
)

const DoclingAdapterVersion = "docling-serve-adapter-v1"

type ParseRequest struct {
	FileName string
	FileType string
	Content  []byte
	Options  map[string]interface{}
}

type Parser interface {
	Parse(ctx context.Context, req ParseRequest) (*documentparser.NormalizedDocument, error)
}

type DoclingConfig struct {
	BaseURL string
	Path    string
	Timeout time.Duration
	Client  *http.Client
}

type DoclingClient struct {
	baseURL string
	path    string
	client  *http.Client
}

func NewDoclingClient(cfg DoclingConfig) *DoclingClient {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = "http://localhost:5001"
	}
	path := strings.TrimSpace(cfg.Path)
	if path == "" {
		path = "/v1/convert/file"
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	return &DoclingClient{
		baseURL: baseURL,
		path:    path,
		client:  client,
	}
}

func (c *DoclingClient) Parse(ctx context.Context, req ParseRequest) (*documentparser.NormalizedDocument, error) {
	if c == nil {
		return nil, fmt.Errorf("docling client is nil")
	}
	if len(req.Content) == 0 {
		return nil, &documentparser.ProviderError{
			Code:      "empty_file",
			Message:   "source file content is empty",
			Stage:     "parse",
			Retryable: false,
		}
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fileWriter, err := writer.CreateFormFile("files", req.FileName)
	if err != nil {
		return nil, fmt.Errorf("create docling file field: %w", err)
	}
	if _, err := fileWriter.Write(req.Content); err != nil {
		return nil, fmt.Errorf("write docling file field: %w", err)
	}
	if err := writer.WriteField("to_formats", "md"); err != nil {
		return nil, fmt.Errorf("write docling to_formats field: %w", err)
	}
	if err := writer.WriteField("to_formats", "json"); err != nil {
		return nil, fmt.Errorf("write docling json to_formats field: %w", err)
	}
	if err := writer.WriteField("image_export_mode", "placeholder"); err != nil {
		return nil, fmt.Errorf("write docling image_export_mode field: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close docling multipart body: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.convertURL(), &body)
	if err != nil {
		return nil, fmt.Errorf("create docling request: %w", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call docling serve: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read docling response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &documentparser.ProviderError{
			Code:      "docling_http_error",
			Message:   fmt.Sprintf("docling returned status %d: %s", resp.StatusCode, truncate(strings.TrimSpace(string(respBody)), 300)),
			Stage:     "parse",
			Retryable: resp.StatusCode >= 500,
		}
	}

	var parsed doclingConvertResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("decode docling response: %w", err)
	}
	return normalizeDoclingResponse(parsed, req)
}

func (c *DoclingClient) convertURL() string {
	path := c.path
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return c.baseURL + path
}

type doclingConvertResponse struct {
	Status    string            `json:"status"`
	Document  doclingDocument   `json:"document"`
	Documents []doclingDocument `json:"documents"`
	Errors    []doclingError    `json:"errors"`
}

type doclingDocument struct {
	FileName    string          `json:"filename"`
	MDContent   string          `json:"md_content"`
	TextContent string          `json:"text_content"`
	JSONContent json.RawMessage `json:"json_content"`
}

type doclingError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func normalizeDoclingResponse(resp doclingConvertResponse, req ParseRequest) (*documentparser.NormalizedDocument, error) {
	doc := resp.Document
	if strings.TrimSpace(doc.MDContent) == "" && strings.TrimSpace(doc.TextContent) == "" && len(resp.Documents) > 0 {
		doc = resp.Documents[0]
	}

	content := strings.TrimSpace(doc.MDContent)
	if content == "" {
		content = strings.TrimSpace(doc.TextContent)
	}
	if content == "" {
		return nil, &documentparser.ProviderError{
			Code:      "empty_docling_result",
			Message:   "docling returned empty markdown content",
			Stage:     "parse",
			Retryable: false,
		}
	}
	content, tables := documentparser.NormalizeMarkdownPipeTables(content)
	if len(tables) == 0 {
		jsonTables := extractDoclingStructuredTables(doc.JSONContent)
		if len(jsonTables) > 0 {
			content, tables = appendStructuredTablesToMarkdown(content, jsonTables)
		}
	}

	fileName := strings.TrimSpace(req.FileName)
	if fileName == "" {
		fileName = strings.TrimSpace(doc.FileName)
	}
	fileType := documentparser.NormalizeFileType(req.FileType)
	if fileType == "" {
		fileType = documentparser.NormalizeFileType(filepath.Ext(fileName))
	}

	normalized := &documentparser.NormalizedDocument{
		ContentMarkdown: content,
		Source: documentparser.NormalizedSource{
			FileName: fileName,
			FileType: fileType,
		},
		Quality: documentparser.ParseQuality{
			Status:   "ok",
			Score:    1,
			Warnings: doclingWarnings(resp.Errors),
		},
		Extractor: documentparser.ExtractorInfo{
			Provider: "docling",
			Version:  DoclingAdapterVersion,
		},
		Tables: tables,
	}
	if err := normalized.Validate(); err != nil {
		return nil, fmt.Errorf("invalid normalized docling document: %w", err)
	}
	return normalized, nil
}

func extractDoclingStructuredTables(raw json.RawMessage) []documentparser.NormalizedTable {
	raw = normalizeDoclingJSONContent(raw)
	if len(raw) == 0 {
		return nil
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}

	tableValues := asList(payload["tables"])
	tables := make([]documentparser.NormalizedTable, 0, len(tableValues))
	for _, tableValue := range tableValues {
		tableMap := asMap(tableValue)
		if tableMap == nil {
			continue
		}
		rows, merged := doclingTableRows(tableMap)
		if len(rows) == 0 {
			continue
		}
		tables = append(tables, documentparser.NormalizedTable{
			ID:          fmt.Sprintf("table-%03d", len(tables)+1),
			Page:        doclingTablePage(tableMap),
			MergedCells: merged,
			Rows:        rows,
			Quality: documentparser.TableQuality{
				Status: "ok",
			},
		})
	}
	return tables
}

func normalizeDoclingJSONContent(raw json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	if trimmed[0] != '"' {
		return trimmed
	}
	var encoded string
	if err := json.Unmarshal(trimmed, &encoded); err != nil {
		return nil
	}
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return nil
	}
	return json.RawMessage(encoded)
}

func doclingTableRows(tableMap map[string]interface{}) ([]documentparser.TableRow, bool) {
	data := asMap(tableMap["data"])
	if data == nil {
		return nil, false
	}

	cellValues := asList(data["table_cells"])
	if len(cellValues) == 0 {
		cellValues = asList(data["cells"])
	}
	if len(cellValues) == 0 {
		return nil, false
	}

	numRows, _ := intFromAny(data, "num_rows", "nrows", "rows")
	numCols, _ := intFromAny(data, "num_cols", "ncols", "cols")
	parsedCells := make([]doclingParsedCell, 0, len(cellValues))
	hasHeader := false
	merged := false

	for _, cellValue := range cellValues {
		cellMap := asMap(cellValue)
		if cellMap == nil {
			continue
		}
		row, okRow := intFromAny(cellMap, "start_row_offset_idx", "row_idx", "row")
		col, okCol := intFromAny(cellMap, "start_col_offset_idx", "col_idx", "col", "column")
		if !okRow || !okCol || row < 0 || col < 0 {
			continue
		}
		rowSpan := spanFromCell(cellMap, row, "end_row_offset_idx", "row_span", "rowspan")
		colSpan := spanFromCell(cellMap, col, "end_col_offset_idx", "col_span", "colspan")
		if rowSpan > 1 || colSpan > 1 {
			merged = true
		}
		isHeader := boolFromAny(cellMap, "column_header", "row_header", "is_header", "header")
		if isHeader {
			hasHeader = true
		}
		text := normalizeDoclingTableCellText(stringFromAny(cellMap, "text", "content"))
		parsedCells = append(parsedCells, doclingParsedCell{
			row:      row,
			col:      col,
			rowSpan:  rowSpan,
			colSpan:  colSpan,
			text:     text,
			isHeader: isHeader,
		})
		if row+rowSpan > numRows {
			numRows = row + rowSpan
		}
		if col+colSpan > numCols {
			numCols = col + colSpan
		}
	}
	if len(parsedCells) == 0 || numRows <= 0 || numCols <= 0 {
		return nil, false
	}

	rows := make([]documentparser.TableRow, numRows)
	for r := range rows {
		rows[r].Cells = make([]documentparser.TableCell, numCols)
	}
	for _, cell := range parsedCells {
		if cell.row >= numRows || cell.col >= numCols {
			continue
		}
		rows[cell.row].Cells[cell.col] = documentparser.TableCell{
			Text:     cell.text,
			Rowspan:  omitDefaultSpan(cell.rowSpan),
			Colspan:  omitDefaultSpan(cell.colSpan),
			IsHeader: cell.isHeader,
		}
	}
	if !hasHeader && len(rows) > 0 {
		for i := range rows[0].Cells {
			rows[0].Cells[i].IsHeader = true
		}
	}
	return trimEmptyDoclingRows(rows), merged
}

type doclingParsedCell struct {
	row      int
	col      int
	rowSpan  int
	colSpan  int
	text     string
	isHeader bool
}

func doclingTablePage(tableMap map[string]interface{}) int {
	for _, provValue := range asList(tableMap["prov"]) {
		prov := asMap(provValue)
		if prov == nil {
			continue
		}
		if page, ok := intFromAny(prov, "page_no", "page"); ok && page > 0 {
			return page
		}
	}
	if page, ok := intFromAny(tableMap, "page_no", "page"); ok && page > 0 {
		return page
	}
	return 0
}

func appendStructuredTablesToMarkdown(content string, tables []documentparser.NormalizedTable) (string, []documentparser.NormalizedTable) {
	var builder strings.Builder
	content = strings.TrimSpace(content)
	if content != "" {
		builder.WriteString(content)
		builder.WriteString("\n\n")
	}
	builder.WriteString("## Extracted Tables\n\n")

	appended := make([]documentparser.NormalizedTable, 0, len(tables))
	for _, table := range tables {
		markdown := renderStructuredTableMarkdown(table.Rows)
		if strings.TrimSpace(markdown) == "" {
			continue
		}
		start := builder.Len()
		builder.WriteString(markdown)
		end := start + len(markdown)
		table.ID = fmt.Sprintf("table-%03d", len(appended)+1)
		table.MarkdownStart = start
		table.MarkdownEnd = end
		appended = append(appended, table)
		builder.WriteString("\n\n")
	}
	return strings.TrimSpace(builder.String()), appended
}

func renderStructuredTableMarkdown(rows []documentparser.TableRow) string {
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
	writeRow := func(cells []documentparser.TableCell) {
		builder.WriteString("|")
		for i := 0; i < columnCount; i++ {
			text := ""
			if i < len(cells) {
				text = escapeStructuredTableCell(cells[i].Text)
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

func trimEmptyDoclingRows(rows []documentparser.TableRow) []documentparser.TableRow {
	trimmed := rows[:0]
	for _, row := range rows {
		empty := true
		for _, cell := range row.Cells {
			if strings.TrimSpace(cell.Text) != "" {
				empty = false
				break
			}
		}
		if !empty {
			trimmed = append(trimmed, row)
		}
	}
	return trimmed
}

func spanFromCell(cell map[string]interface{}, start int, endKey string, spanKeys ...string) int {
	if end, ok := intFromAny(cell, endKey); ok && end > start {
		return end - start
	}
	if span, ok := intFromAny(cell, spanKeys...); ok && span > 0 {
		return span
	}
	return 1
}

func omitDefaultSpan(span int) int {
	if span <= 1 {
		return 0
	}
	return span
}

func normalizeDoclingTableCellText(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func escapeStructuredTableCell(value string) string {
	value = normalizeDoclingTableCellText(value)
	value = strings.ReplaceAll(value, "|", "\\|")
	return value
}

func asMap(value interface{}) map[string]interface{} {
	if mapped, ok := value.(map[string]interface{}); ok {
		return mapped
	}
	return nil
}

func asList(value interface{}) []interface{} {
	if list, ok := value.([]interface{}); ok {
		return list
	}
	return nil
}

func intFromAny(values map[string]interface{}, keys ...string) (int, bool) {
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case int:
			return typed, true
		case int64:
			return int(typed), true
		case float64:
			return int(typed), true
		case json.Number:
			integer, err := typed.Int64()
			if err == nil {
				return int(integer), true
			}
		}
	}
	return 0, false
}

func boolFromAny(values map[string]interface{}, keys ...string) bool {
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		if typed, ok := value.(bool); ok {
			return typed
		}
	}
	return false
}

func stringFromAny(values map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		if typed, ok := value.(string); ok {
			return typed
		}
	}
	return ""
}

func doclingWarnings(errors []doclingError) []string {
	warnings := make([]string, 0, len(errors))
	for _, err := range errors {
		message := strings.TrimSpace(err.Message)
		if message == "" {
			message = strings.TrimSpace(err.Code)
		}
		if message != "" {
			warnings = append(warnings, message)
		}
	}
	return warnings
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
