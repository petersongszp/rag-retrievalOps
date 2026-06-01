package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// RAGRetrieveConfig 配置 RAG Platform 连接信息
type RAGRetrieveConfig struct {
	BaseURL      string   `json:"base_url"`       // e.g. "http://localhost:8899"
	APIKey       string   `json:"api_key"`         // API Key (第一版可为空)
	AppID        string   `json:"app_id"`          // e.g. "interview-agent"
	DefaultKBIDs []uint64 `json:"default_kb_ids"` // 默认知识库 ID
}

// ragRetrieveTool 实现 eino tool.InvokableTool 接口
type ragRetrieveTool struct {
	config     RAGRetrieveConfig
	httpClient *http.Client
}

// NewRAGRetrieveTool 创建 RAG Retrieve Tool
func NewRAGRetrieveTool(cfg RAGRetrieveConfig) *ragRetrieveTool {
	return &ragRetrieveTool{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Info 返回工具信息
func (t *ragRetrieveTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "rag_retrieve",
		Desc: "从知识库中检索相关文档。使用此工具可以查找面试题、技术知识点、项目经验等信息。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Type:     schema.String,
				Desc:     "检索查询内容",
				Required: true,
			},
			"kb_ids": {
				Type:     schema.Array,
				Desc:     "知识库 ID 列表（可选，不传则使用默认值）",
				Required: false,
				ElemInfo: &schema.ParameterInfo{
					Type: schema.Integer,
				},
			},
		}),
	}, nil
}

// InvokableRun 执行检索
func (t *ragRetrieveTool) InvokableRun(ctx context.Context, arguments string, opts ...tool.Option) (string, error) {
	// 解析参数
	var params struct {
		Query string   `json:"query"`
		KBIDs []uint64 `json:"kb_ids"`
	}
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	if params.Query == "" {
		return "", fmt.Errorf("query is required")
	}

	kbIDs := params.KBIDs
	if len(kbIDs) == 0 {
		kbIDs = t.config.DefaultKBIDs
	}

	// 构建请求
	reqBody := map[string]interface{}{
		"app_id": t.config.AppID,
		"query":  params.Query,
		"kb_ids": kbIDs,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	url := strings.TrimRight(t.config.BaseURL, "/") + "/v1/retrieve"
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if t.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+t.config.APIKey)
	}

	log.Printf("[RAG Retrieve Tool] calling %s query=%q kb_ids=%v", url, params.Query, kbIDs)

	// 发送请求
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request RAG platform: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("RAG platform returned %d: %s", resp.StatusCode, string(respBody))
	}

	log.Printf("[RAG Retrieve Tool] response status=%d body_len=%d", resp.StatusCode, len(respBody))

	// 返回结果
	return string(respBody), nil
}

// ragRetrieveToolInstance 全局实例
var ragRetrieveToolInstance *ragRetrieveTool

// InitRAGRetrieveTool 初始化 RAG Retrieve Tool
func InitRAGRetrieveTool(cfg RAGRetrieveConfig) {
	ragRetrieveToolInstance = NewRAGRetrieveTool(cfg)
	log.Printf("[RAG Retrieve Tool] initialized base_url=%s app_id=%s default_kb_ids=%v", cfg.BaseURL, cfg.AppID, cfg.DefaultKBIDs)
}

// GetRAGRetrieveTool 获取 RAG Retrieve Tool 实例
func GetRAGRetrieveTool() tool.InvokableTool {
	return ragRetrieveToolInstance
}
