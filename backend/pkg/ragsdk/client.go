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

// Client RAG Platform SDK 客户端
type Client struct {
	BaseURL    string
	APIKey     string
	AppID      string
	HTTPClient *http.Client
}

// ClientConfig SDK 配置
type ClientConfig struct {
	BaseURL string // e.g. "http://localhost:8081"
	APIKey  string
	AppID   string
	Timeout time.Duration // default 10s
}

// NewClient 创建 SDK 客户端
func NewClient(cfg ClientConfig) *Client {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		BaseURL: cfg.BaseURL,
		APIKey:  cfg.APIKey,
		AppID:   cfg.AppID,
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// RetrieveRequest 检索请求
type RetrieveRequest struct {
	Query           string                 `json:"query"`
	KBIDs           []uint64               `json:"kb_ids,omitempty"`
	TopK            int                    `json:"top_k,omitempty"`
	StrategyProfile string                 `json:"strategy_profile,omitempty"`
	MetadataFilter  map[string]interface{} `json:"metadata_filter,omitempty"`
}

// RetrieveResponse 检索响应
type RetrieveResponse struct {
	RequestID string         `json:"request_id"`
	Items     []RetrieveItem `json:"items"`
}

// RetrieveItem 检索结果项
type RetrieveItem struct {
	Content  string      `json:"content"`
	Score    float64     `json:"score"`
	Citation interface{} `json:"citation"`
	Source   interface{} `json:"source"`
}

// Retrieve 执行检索
func (c *Client) Retrieve(ctx context.Context, req RetrieveRequest) (*RetrieveResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := c.BaseURL + "/v1/retrieve"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
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
		return nil, fmt.Errorf("RAG API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result RetrieveResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &result, nil
}
