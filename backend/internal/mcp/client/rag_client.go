package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxResponseBodyBytes = 8 << 20

type RetrieveRequest struct {
	Query           string                 `json:"query"`
	KBIDs           []uint64               `json:"kb_ids"`
	TopK            int                    `json:"top_k"`
	StrategyProfile string                 `json:"strategy_profile,omitempty"`
	MetadataFilter  map[string]interface{} `json:"metadata_filter,omitempty"`
}

type RetrieveResponse struct {
	RequestID          string         `json:"request_id"`
	Items              []RetrieveItem `json:"items"`
	StrategyVersion    string         `json:"strategy_version,omitempty"`
	RequestCost        interface{}    `json:"request_cost,omitempty"`
	CitationCheck      interface{}    `json:"citation_check,omitempty"`
	Refusal            interface{}    `json:"refusal,omitempty"`
	EvidenceGateResult string         `json:"evidence_gate_result,omitempty"`
}

type RetrieveItem struct {
	Content  string   `json:"content"`
	Score    float64  `json:"score"`
	Citation Citation `json:"citation"`
	Source   Source   `json:"source"`
}

type Citation struct {
	KBID       uint64 `json:"kb_id"`
	DocumentID uint64 `json:"document_id"`
	ChunkID    string `json:"chunk_id"`
	FileName   string `json:"file_name"`
	ChunkIndex int    `json:"chunk_index"`
}

type Source struct {
	Route            string `json:"route"`
	Collection       string `json:"collection"`
	RetrieverVersion string `json:"retriever_version"`
}

type UpstreamError struct {
	StatusCode int
	Code       string
	Message    string
	RequestID  string
	Retryable  bool
}

func (e *UpstreamError) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("%s: %s (request_id=%s)", e.Code, e.Message, e.RequestID)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

type Client struct {
	endpoint      string
	authorization string
	httpClient    *http.Client
}

func New(baseURL, token string, timeout time.Duration) (*Client, error) {
	return NewWithAuthorization(baseURL, "Bearer "+strings.TrimSpace(token), timeout)
}

func NewWithAuthorization(baseURL, authorization string, timeout time.Duration) (*Client, error) {
	baseURL = strings.TrimSpace(baseURL)
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid RAG base URL")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("timeout must be positive")
	}
	return &Client{
		endpoint:      strings.TrimRight(baseURL, "/") + "/v1/retrieve",
		authorization: strings.TrimSpace(authorization),
		httpClient:    &http.Client{Timeout: timeout},
	}, nil
}

func (c *Client) Retrieve(ctx context.Context, req RetrieveRequest) (*RetrieveResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal retrieve request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create retrieve request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if c.authorization != "" {
		httpReq.Header.Set("Authorization", c.authorization)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, &UpstreamError{
				Code:      "backend_timeout",
				Message:   "RAG retrieval timed out",
				Retryable: true,
			}
		}
		return nil, &UpstreamError{
			Code:      "backend_unavailable",
			Message:   "RAG service is unavailable",
			Retryable: true,
		}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read retrieve response: %w", err)
	}
	if len(respBody) > maxResponseBodyBytes {
		return nil, &UpstreamError{
			StatusCode: resp.StatusCode,
			Code:       "backend_error",
			Message:    "RAG response exceeded the size limit",
			Retryable:  true,
		}
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, decodeUpstreamError(resp.StatusCode, respBody)
	}

	result, err := decodeRetrieveResponse(respBody)
	if err != nil {
		return nil, &UpstreamError{
			StatusCode: resp.StatusCode,
			Code:       "backend_error",
			Message:    "RAG service returned an invalid response",
			Retryable:  true,
		}
	}
	return result, nil
}

type responseEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func decodeRetrieveResponse(body []byte) (*RetrieveResponse, error) {
	var envelope responseEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}

	payload := body
	if envelope.Data != nil {
		if envelope.Code != 0 && envelope.Code != http.StatusOK {
			return nil, fmt.Errorf("RAG API returned code %d", envelope.Code)
		}
		payload = envelope.Data
	}

	var result RetrieveResponse
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, err
	}
	if result.Items == nil {
		result.Items = []RetrieveItem{}
	}
	return &result, nil
}

func decodeUpstreamError(statusCode int, body []byte) *UpstreamError {
	var envelope struct {
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
		Data      struct {
			RequestID string `json:"request_id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(body, &envelope)

	message := strings.TrimSpace(envelope.Message)
	if message == "" {
		message = http.StatusText(statusCode)
	}
	requestID := envelope.RequestID
	if requestID == "" {
		requestID = envelope.Data.RequestID
	}

	code, retryable := mapStatus(statusCode)
	return &UpstreamError{
		StatusCode: statusCode,
		Code:       code,
		Message:    message,
		RequestID:  requestID,
		Retryable:  retryable,
	}
}

func mapStatus(statusCode int) (string, bool) {
	switch statusCode {
	case http.StatusBadRequest:
		return "invalid_request", false
	case http.StatusUnauthorized:
		return "unauthorized", false
	case http.StatusForbidden:
		return "forbidden", false
	case http.StatusNotFound:
		return "not_found", false
	case http.StatusTooManyRequests:
		return "rate_limited", true
	case http.StatusServiceUnavailable:
		return "backend_unavailable", true
	case http.StatusGatewayTimeout:
		return "backend_timeout", true
	default:
		return "backend_error", statusCode >= http.StatusInternalServerError
	}
}
