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
