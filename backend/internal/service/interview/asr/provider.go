package asr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"sync"
	"time"

	"interview-agents/pkg/circuitbreaker"
)

var (
	asrTransport     *http.Transport
	asrTransportOnce sync.Once
)

type ProviderError struct {
	StatusCode int
	TraceID    string
	Err        error
}

func (e *ProviderError) Error() string {
	if e.Err == nil {
		return "provider error"
	}
	return e.Err.Error()
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

type siliconFlowProvider struct {
	endpoint string
	apiKey   string
	model    string
	client   *http.Client
	breaker  *circuitbreaker.CircuitBreaker
}

func NewSiliconFlowProvider(cfg ASRConfig) AudioTranscriptionProvider {
	return &siliconFlowProvider{
		endpoint: cfg.BaseURL + "/audio/transcriptions",
		apiKey:   cfg.APIKey,
		model:    cfg.ModelName,
		client: &http.Client{
			Timeout:   120 * time.Second,
			Transport: getASRTransport(),
		},
		breaker: circuitbreaker.NewCircuitBreaker("asr-siliconflow-" + cfg.ModelName),
	}
}

func getASRTransport() *http.Transport {
	asrTransportOnce.Do(func() {
		asrTransport = &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			MaxIdleConns:          20,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			DisableKeepAlives:     false,
			ForceAttemptHTTP2:     false,
		}
	})

	return asrTransport
}

func (p *siliconFlowProvider) Transcribe(ctx context.Context, req AudioTranscriptionRequest) (*AudioTranscriptionResult, error) {
	result, err := p.breaker.Execute(func() (interface{}, error) {
		return p.doTranscribe(ctx, req)
	})
	if err != nil {
		var providerErr *ProviderError
		if errors.As(err, &providerErr) {
			return nil, providerErr
		}
		if err == circuitbreaker.ErrOpen || err == circuitbreaker.ErrTooMany {
			return nil, &ProviderError{Err: err}
		}
		return nil, &ProviderError{Err: err}
	}

	return result.(*AudioTranscriptionResult), nil
}

func (p *siliconFlowProvider) doTranscribe(ctx context.Context, req AudioTranscriptionRequest) (*AudioTranscriptionResult, error) {
	body, contentType, err := p.buildRequestBody(req)
	if err != nil {
		return nil, &ProviderError{Err: err}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, &ProviderError{Err: err}
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", contentType)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, &ProviderError{Err: err}
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("[ASR] failed to close upstream response body: %v", closeErr)
		}
	}()

	traceID := resp.Header.Get("x-siliconcloud-trace-id")
	responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return nil, &ProviderError{
			StatusCode: resp.StatusCode,
			TraceID:    traceID,
			Err:        readErr,
		}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &ProviderError{
			StatusCode: resp.StatusCode,
			TraceID:    traceID,
			Err:        fmt.Errorf("upstream ASR request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(responseBody))),
		}
	}

	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, &ProviderError{
			StatusCode: resp.StatusCode,
			TraceID:    traceID,
			Err:        err,
		}
	}

	return &AudioTranscriptionResult{
		Text:     payload.Text,
		Provider: ProviderSiliconFlow,
		Model:    req.ModelName,
		TraceID:  traceID,
	}, nil
}

func (p *siliconFlowProvider) buildRequestBody(req AudioTranscriptionRequest) ([]byte, string, error) {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)

	if err := writer.WriteField("model", req.ModelName); err != nil {
		return nil, "", err
	}

	fileName := req.FileName
	if strings.TrimSpace(fileName) == "" {
		fileName = "audio.webm"
	}

	part, err := writer.CreatePart(buildFileHeader(fileName, req.ContentType))
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(req.AudioBytes); err != nil {
		return nil, "", err
	}

	if err := writer.Close(); err != nil {
		return nil, "", err
	}

	return buffer.Bytes(), writer.FormDataContentType(), nil
}

func buildFileHeader(fileName, contentType string) textproto.MIMEHeader {
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, strings.ReplaceAll(fileName, `"`, "")))
	header.Set("Content-Type", contentType)
	return header
}
