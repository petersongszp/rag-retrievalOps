package parserprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	"interview-agents/internal/documentparser"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestDoclingClientParseUploadsFileAndNormalizesMarkdown(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPost {
				t.Fatalf("method = %s", req.Method)
			}
			if req.URL.String() != "http://docling:5001/v1/convert/file" {
				t.Fatalf("url = %s", req.URL.String())
			}
			if !strings.HasPrefix(req.Header.Get("Content-Type"), "multipart/form-data;") {
				t.Fatalf("Content-Type = %q", req.Header.Get("Content-Type"))
			}
			if err := req.ParseMultipartForm(1 << 20); err != nil {
				t.Fatalf("ParseMultipartForm: %v", err)
			}
			if got := req.FormValue("to_formats"); got != "md" {
				t.Fatalf("to_formats = %q", got)
			}
			file, header, err := req.FormFile("files")
			if err != nil {
				t.Fatalf("FormFile(files): %v", err)
			}
			defer file.Close()
			content, err := io.ReadAll(file)
			if err != nil {
				t.Fatalf("ReadAll(file): %v", err)
			}
			if header.Filename != "guide.pdf" {
				t.Fatalf("filename = %q", header.Filename)
			}
			if string(content) != "%PDF" {
				t.Fatalf("content = %q", string(content))
			}

			body := `{"status":"success","document":{"filename":"guide.pdf","md_content":"# Guide\n\nHello"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	}
	adapter := NewDoclingClient(DoclingConfig{
		BaseURL: "http://docling:5001",
		Timeout: 2 * time.Second,
		Client:  client,
	})

	doc, err := adapter.Parse(context.Background(), ParseRequest{
		FileName: "guide.pdf",
		FileType: "pdf",
		Content:  []byte("%PDF"),
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if doc.ContentMarkdown != "# Guide\n\nHello" {
		t.Fatalf("ContentMarkdown = %q", doc.ContentMarkdown)
	}
	if doc.Source.FileName != "guide.pdf" || doc.Source.FileType != "pdf" {
		t.Fatalf("Source = %+v", doc.Source)
	}
	if doc.Extractor.Provider != "docling" {
		t.Fatalf("Extractor.Provider = %q", doc.Extractor.Provider)
	}
}

func TestDoclingClientParseExtractsMarkdownTables(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"status":"success","document":{"filename":"billing.pdf","md_content":"## 计费规则\n\n| **项目** | **按量付费** | **包年包月** |\n|----|----|----|\n| **计费公式** | 计算规格费用 = 服务时长（小时） × 规格单价（元/小时） | 计算规格费用 = 购买时长（月） × 规格单价（元/月） |"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	}
	adapter := NewDoclingClient(DoclingConfig{
		BaseURL: "http://docling:5001",
		Timeout: 2 * time.Second,
		Client:  client,
	})

	doc, err := adapter.Parse(context.Background(), ParseRequest{
		FileName: "billing.pdf",
		FileType: "pdf",
		Content:  []byte("%PDF"),
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !strings.Contains(doc.ContentMarkdown, "| 项目 | 按量付费 | 包年包月 |") {
		t.Fatalf("expected canonical table header, got %q", doc.ContentMarkdown)
	}
	if len(doc.Tables) != 1 {
		t.Fatalf("expected one table, got %d", len(doc.Tables))
	}
	table := doc.Tables[0]
	if got := table.Rows[0].Cells[0].Text; got != "项目" {
		t.Fatalf("first header cell = %q", got)
	}
	if got := table.Rows[1].Cells[0].Text; got != "计费公式" {
		t.Fatalf("first data cell = %q", got)
	}
	if table.MarkdownStart <= 0 || table.MarkdownEnd > len(doc.ContentMarkdown) {
		t.Fatalf("invalid markdown range %d-%d for content length %d", table.MarkdownStart, table.MarkdownEnd, len(doc.ContentMarkdown))
	}
}

func TestDoclingClientParseExtractsStructuredJSONTables(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if err := req.ParseMultipartForm(1 << 20); err != nil {
				t.Fatalf("ParseMultipartForm: %v", err)
			}
			formats := req.MultipartForm.Value["to_formats"]
			if len(formats) != 2 || formats[0] != "md" || formats[1] != "json" {
				t.Fatalf("to_formats = %v", formats)
			}
			body := `{
				"status": "success",
				"document": {
					"filename": "billing.pdf",
					"md_content": "## 计费规则\n\n项目 按量付费 包年包月\n计费公式 计算规格费用 = 实例购买后的服务时长（小时） × 规格单价（元/小时） 计算规格费用 = 购买时长（月） × 规格单价（元/月）",
					"json_content": {
						"tables": [
							{
								"self_ref": "#/tables/0",
								"prov": [{"page_no": 2}],
								"data": {
									"num_rows": 2,
									"num_cols": 3,
									"table_cells": [
										{"text": "项目", "start_row_offset_idx": 0, "end_row_offset_idx": 1, "start_col_offset_idx": 0, "end_col_offset_idx": 1, "column_header": true},
										{"text": "按量付费", "start_row_offset_idx": 0, "end_row_offset_idx": 1, "start_col_offset_idx": 1, "end_col_offset_idx": 2, "column_header": true},
										{"text": "包年包月", "start_row_offset_idx": 0, "end_row_offset_idx": 1, "start_col_offset_idx": 2, "end_col_offset_idx": 3, "column_header": true},
										{"text": "计费公式", "start_row_offset_idx": 1, "end_row_offset_idx": 2, "start_col_offset_idx": 0, "end_col_offset_idx": 1},
										{"text": "计算规格费用 = 实例购买后的服务时长（小时） × 规格单价（元/小时）", "start_row_offset_idx": 1, "end_row_offset_idx": 2, "start_col_offset_idx": 1, "end_col_offset_idx": 2},
										{"text": "计算规格费用 = 购买时长（月） × 规格单价（元/月）", "start_row_offset_idx": 1, "end_row_offset_idx": 2, "start_col_offset_idx": 2, "end_col_offset_idx": 3}
									]
								}
							}
						]
					}
				}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	}
	adapter := NewDoclingClient(DoclingConfig{
		BaseURL: "http://docling:5001",
		Timeout: 2 * time.Second,
		Client:  client,
	})

	doc, err := adapter.Parse(context.Background(), ParseRequest{
		FileName: "billing.pdf",
		FileType: "pdf",
		Content:  []byte("%PDF"),
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(doc.Tables) != 1 {
		t.Fatalf("expected one structured table, got %d", len(doc.Tables))
	}
	table := doc.Tables[0]
	if table.Page != 2 {
		t.Fatalf("table page = %d", table.Page)
	}
	if got := table.Rows[0].Cells[0].Text; got != "项目" {
		t.Fatalf("first header cell = %q", got)
	}
	if got := table.Rows[1].Cells[1].Text; got != "计算规格费用 = 实例购买后的服务时长（小时） × 规格单价（元/小时）" {
		t.Fatalf("pay-as-you-go cell = %q", got)
	}
	if !strings.Contains(doc.ContentMarkdown, "| 项目 | 按量付费 | 包年包月 |") {
		t.Fatalf("expected structured table markdown to be appended, got %q", doc.ContentMarkdown)
	}
	if table.MarkdownStart <= 0 || table.MarkdownEnd > len(doc.ContentMarkdown) {
		t.Fatalf("invalid markdown range %d-%d for content length %d", table.MarkdownStart, table.MarkdownEnd, len(doc.ContentMarkdown))
	}
}

func TestDoclingClientParseExtendsFragmentedPDFTableSpan(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"status": "success",
				"document": {
					"filename": "billing.pdf",
					"md_content": "## 2. 计费规则\n\n| 项目 | 按量付费 | 包年包月 |\n| --- | --- | --- |\n| 计费公式 | 计算规格费用 = 服务时长（小时） × 规格单价（元/小时） | 计算规格费用 = 购买时长（月） × 规格单价（元/月） |\n\n### 计费周期 按小时计费\n\n当前计费周期内，若实例的服务时间不足 1 小时按照 1 小时计算。\n\n例如，实例购买时间为 10 点 30 分，则 10 点到 11 点这一计费周期内，计算规格费用为 1 小时 × 规格单价，而非 0.5 小时 × 规格单价。\n\n## 3. 常见问题\n\n实例购买后立即开始计费。"
				}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	}
	adapter := NewDoclingClient(DoclingConfig{
		BaseURL: "http://docling:5001",
		Timeout: 2 * time.Second,
		Client:  client,
	})

	doc, err := adapter.Parse(context.Background(), ParseRequest{
		FileName: "billing.pdf",
		FileType: "pdf",
		Content:  []byte("%PDF"),
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(doc.Tables) != 1 {
		t.Fatalf("expected one table, got %d", len(doc.Tables))
	}
	table := doc.Tables[0]
	tableContent := doc.ContentMarkdown[table.MarkdownStart:table.MarkdownEnd]
	if !strings.Contains(tableContent, "10 点 30 分") {
		t.Fatalf("expected table span to include fragmented billing-cycle row, got %q", tableContent)
	}
	if strings.Contains(tableContent, "## 3. 常见问题") {
		t.Fatalf("table span should stop before next numbered section, got %q", tableContent)
	}
}

func TestDoclingClientParseDoesNotExtendPDFTableSpanIntoUnrelatedUnnumberedSection(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"status": "success",
				"document": {
					"filename": "billing.pdf",
					"md_content": "## 2. 计费规则\n\n| 项目 | 按量付费 |\n| --- | --- |\n| 计费公式 | 计算规格费用 = 服务时长 × 规格单价 |\n\n## 注意事项\n\n请确保账号余额充足。\n\n上传资料前请阅读开通说明。\n\n## 3. 下一章\n\n后续正文。"
				}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	}
	adapter := NewDoclingClient(DoclingConfig{
		BaseURL: "http://docling:5001",
		Timeout: 2 * time.Second,
		Client:  client,
	})

	doc, err := adapter.Parse(context.Background(), ParseRequest{
		FileName: "billing.pdf",
		FileType: "pdf",
		Content:  []byte("%PDF"),
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(doc.Tables) != 1 {
		t.Fatalf("expected one table, got %d", len(doc.Tables))
	}
	table := doc.Tables[0]
	tableContent := doc.ContentMarkdown[table.MarkdownStart:table.MarkdownEnd]
	if strings.Contains(tableContent, "注意事项") || strings.Contains(tableContent, "账号余额") {
		t.Fatalf("table span should not include unrelated unnumbered section, got %q", tableContent)
	}
}

func TestDoclingClientParseStopsFragmentedPDFTableSpanBeforeNextUnnumberedSection(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"status": "success",
				"document": {
					"filename": "billing.pdf",
					"md_content": "## 2. 计费规则\n\n| 项目 | 按量付费 |\n| --- | --- |\n| 计费公式 | 计算规格费用 = 服务时长 × 规格单价 |\n\n### 计费周期 按小时计费\n\n例如，当前计费周期内计算规格费用为 1 小时 × 规格单价。\n\n## 注意事项\n\n规格单价可能在普通说明里再次出现，但这里不是表格续接内容。\n\n## 3. 下一章\n\n后续正文。"
				}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	}
	adapter := NewDoclingClient(DoclingConfig{
		BaseURL: "http://docling:5001",
		Timeout: 2 * time.Second,
		Client:  client,
	})

	doc, err := adapter.Parse(context.Background(), ParseRequest{
		FileName: "billing.pdf",
		FileType: "pdf",
		Content:  []byte("%PDF"),
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(doc.Tables) != 1 {
		t.Fatalf("expected one table, got %d", len(doc.Tables))
	}
	table := doc.Tables[0]
	tableContent := doc.ContentMarkdown[table.MarkdownStart:table.MarkdownEnd]
	if !strings.Contains(tableContent, "当前计费周期") {
		t.Fatalf("expected table span to include first continuation block, got %q", tableContent)
	}
	if strings.Contains(tableContent, "注意事项") || strings.Contains(tableContent, "普通说明") {
		t.Fatalf("table span should stop before the next unnumbered section, got %q", tableContent)
	}
}

func TestDoclingClientParseDoesNotExtendPDFTableSpanIntoSameLevelUnnumberedHeading(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"status": "success",
				"document": {
					"filename": "billing.pdf",
					"md_content": "## 2. 计费规则\n\n| 项目 | 按量付费 |\n| --- | --- |\n| 计费公式 | 计算规格费用 = 服务时长 × 规格单价 |\n\n## 使用方式\n\n调用接口时仍会提到计算规格费用，但这里已经是同级新章节。"
				}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	}
	adapter := NewDoclingClient(DoclingConfig{
		BaseURL: "http://docling:5001",
		Timeout: 2 * time.Second,
		Client:  client,
	})

	doc, err := adapter.Parse(context.Background(), ParseRequest{
		FileName: "billing.pdf",
		FileType: "pdf",
		Content:  []byte("%PDF"),
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(doc.Tables) != 1 {
		t.Fatalf("expected one table, got %d", len(doc.Tables))
	}
	table := doc.Tables[0]
	tableContent := doc.ContentMarkdown[table.MarkdownStart:table.MarkdownEnd]
	if strings.Contains(tableContent, "使用方式") || strings.Contains(tableContent, "同级新章节") {
		t.Fatalf("table span should stop before same-level unnumbered heading, got %q", tableContent)
	}
}

func TestDoclingFragmentedTableExtensionRequiresASCIIKeywordBoundary(t *testing.T) {
	content := "\n\n## Notes\n\nThe xa1by identifier is unrelated prose."
	end := doclingFragmentedTableExtensionEnd(content, 0, 0, len(content), []documentparser.TableRow{
		{Cells: []documentparser.TableCell{{Text: "a1b", IsHeader: true}}},
	})
	if end != 0 {
		t.Fatalf("expected embedded ASCII keyword not to extend table span, got end=%d content=%q", end, content[:end])
	}
}

func TestDoclingTableRowsRejectsOversizedDimensions(t *testing.T) {
	rows, merged := doclingTableRows(map[string]interface{}{
		"data": map[string]interface{}{
			"num_rows": float64(100000),
			"num_cols": float64(2),
			"table_cells": []interface{}{
				map[string]interface{}{
					"text":                 "项目",
					"start_row_offset_idx": float64(0),
					"end_row_offset_idx":   float64(1),
					"start_col_offset_idx": float64(0),
					"end_col_offset_idx":   float64(1),
					"column_header":        true,
				},
			},
		},
	})
	if len(rows) != 0 || merged {
		t.Fatalf("expected oversized table to be discarded, got rows=%d merged=%v", len(rows), merged)
	}
}

func TestDoclingTableRowsDoesNotTreatRowHeaderAsColumnHeader(t *testing.T) {
	rows, _ := doclingTableRows(map[string]interface{}{
		"data": map[string]interface{}{
			"num_rows": float64(2),
			"num_cols": float64(2),
			"table_cells": []interface{}{
				map[string]interface{}{
					"text":                 "地域",
					"start_row_offset_idx": float64(0),
					"end_row_offset_idx":   float64(1),
					"start_col_offset_idx": float64(0),
					"end_col_offset_idx":   float64(1),
					"row_header":           true,
				},
				map[string]interface{}{
					"text":                 "单价",
					"start_row_offset_idx": float64(0),
					"end_row_offset_idx":   float64(1),
					"start_col_offset_idx": float64(1),
					"end_col_offset_idx":   float64(2),
				},
				map[string]interface{}{
					"text":                 "北京",
					"start_row_offset_idx": float64(1),
					"end_row_offset_idx":   float64(2),
					"start_col_offset_idx": float64(0),
					"end_col_offset_idx":   float64(1),
					"row_header":           true,
				},
				map[string]interface{}{
					"text":                 "1.00",
					"start_row_offset_idx": float64(1),
					"end_row_offset_idx":   float64(2),
					"start_col_offset_idx": float64(1),
					"end_col_offset_idx":   float64(2),
				},
			},
		},
	})
	if len(rows) != 2 || len(rows[0].Cells) != 2 {
		t.Fatalf("expected 2x2 rows, got %+v", rows)
	}
	if !rows[0].Cells[0].IsHeader || !rows[0].Cells[1].IsHeader {
		t.Fatalf("expected first row to be promoted as fallback column header, got %+v", rows[0].Cells)
	}
	if rows[1].Cells[0].IsHeader {
		t.Fatalf("row_header should not mark body row cells as column headers, got %+v", rows[1].Cells)
	}
}

func TestParseHandlerReturnsNormalizedDocument(t *testing.T) {
	upstream := &fakeParser{
		doc: &documentparser.NormalizedDocument{
			ContentMarkdown: "# Parsed",
			Source: documentparser.NormalizedSource{
				FileName: "scan.pdf",
				FileType: "pdf",
			},
			Quality:   documentparser.ParseQuality{Status: "ok", Score: 1},
			Extractor: documentparser.ExtractorInfo{Provider: "docling", Version: DoclingAdapterVersion},
		},
	}
	handler := NewParseHandler(upstream)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fileWriter, err := writer.CreateFormFile("file", "scan.pdf")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fileWriter.Write([]byte("%PDF")); err != nil {
		t.Fatalf("Write file: %v", err)
	}
	if err := writer.WriteField("file_type", "pdf"); err != nil {
		t.Fatalf("WriteField: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, "/parse", &body)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := &responseRecorder{header: make(http.Header)}

	handler.ServeHTTP(rec, req)

	if rec.status != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.status, rec.body.String())
	}
	var got map[string]interface{}
	if err := json.Unmarshal(rec.body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got["content_markdown"] != "# Parsed" {
		t.Fatalf("content_markdown = %v", got["content_markdown"])
	}
	if upstream.req.FileName != "scan.pdf" || upstream.req.FileType != "pdf" || string(upstream.req.Content) != "%PDF" {
		t.Fatalf("upstream request = %+v", upstream.req)
	}
}

type fakeParser struct {
	req ParseRequest
	doc *documentparser.NormalizedDocument
}

func (p *fakeParser) Parse(ctx context.Context, req ParseRequest) (*documentparser.NormalizedDocument, error) {
	_ = ctx
	p.req = req
	return p.doc, nil
}

type responseRecorder struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(data)
}
