package ragsdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("RAG API returned %d: %s", e.StatusCode, e.Body)
}

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

type ClientConfig struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

func NewClient(cfg ClientConfig) *Client {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	return &Client{
		BaseURL: cfg.BaseURL,
		APIKey:  cfg.APIKey,
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
	}
}

type RetrieveRequest struct {
	Query           string                 `json:"query"`
	KBIDs           []uint64               `json:"kb_ids,omitempty"`
	TopK            int                    `json:"top_k,omitempty"`
	StrategyProfile string                 `json:"strategy_profile,omitempty"`
	MetadataFilter  map[string]interface{} `json:"metadata_filter,omitempty"`
}

type RetrieveResponse struct {
	RequestID string         `json:"request_id"`
	Items     []RetrieveItem `json:"items"`
}

type RetrieveItem struct {
	Content  string      `json:"content"`
	Score    float64     `json:"score"`
	Citation interface{} `json:"citation"`
	Source   interface{} `json:"source"`
}

func (c *Client) Retrieve(ctx context.Context, req RetrieveRequest) (*RetrieveResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := c.BaseURL + "/v1/retrieve"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
	}

	var result RetrieveResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &result, nil
}
