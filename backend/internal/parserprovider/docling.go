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
	FileName    string `json:"filename"`
	MDContent   string `json:"md_content"`
	TextContent string `json:"text_content"`
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
	}
	if err := normalized.Validate(); err != nil {
		return nil, fmt.Errorf("invalid normalized docling document: %w", err)
	}
	return normalized, nil
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
