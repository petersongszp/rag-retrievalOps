package documentparser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

type ProviderRequest struct {
	FileName string
	FileType string
	Content  []byte
	Options  map[string]interface{}
}

type HTTPProviderConfig struct {
	Endpoint string
	Timeout  time.Duration
	Client   *http.Client
}

type HTTPProvider struct {
	endpoint string
	client   *http.Client
}

func NewHTTPProvider(cfg HTTPProviderConfig) *HTTPProvider {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	return &HTTPProvider{
		endpoint: cfg.Endpoint,
		client:   client,
	}
}

func (p *HTTPProvider) Parse(ctx context.Context, req ProviderRequest) (*NormalizedDocument, error) {
	if p == nil || p.endpoint == "" {
		return nil, fmt.Errorf("parser provider endpoint is empty")
	}
	if !IsProviderType(req.FileType) {
		return nil, fmt.Errorf("http parser provider does not support file type: %s", req.FileType)
	}
	if len(req.Content) == 0 {
		return nil, fmt.Errorf("provider request content is empty")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fileWriter, err := writer.CreateFormFile("file", req.FileName)
	if err != nil {
		return nil, fmt.Errorf("create provider file field: %w", err)
	}
	if _, err := fileWriter.Write(req.Content); err != nil {
		return nil, fmt.Errorf("write provider file field: %w", err)
	}
	if err := writer.WriteField("file_type", NormalizeFileType(req.FileType)); err != nil {
		return nil, fmt.Errorf("write provider file_type field: %w", err)
	}
	if req.Options != nil {
		options, err := json.Marshal(req.Options)
		if err != nil {
			return nil, fmt.Errorf("marshal provider options: %w", err)
		}
		if err := writer.WriteField("options", string(options)); err != nil {
			return nil, fmt.Errorf("write provider options field: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close provider multipart body: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, &body)
	if err != nil {
		return nil, fmt.Errorf("create provider request: %w", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call parser provider: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read parser provider response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var providerErr ProviderError
		if err := json.Unmarshal(respBody, &providerErr); err == nil && providerErr.Code != "" {
			return nil, &providerErr
		}
		return nil, fmt.Errorf("parser provider returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var doc NormalizedDocument
	if err := json.Unmarshal(respBody, &doc); err != nil {
		return nil, fmt.Errorf("decode parser provider response: %w", err)
	}
	if err := doc.Validate(); err != nil {
		return nil, fmt.Errorf("invalid parser provider response: %w", err)
	}
	return &doc, nil
}
