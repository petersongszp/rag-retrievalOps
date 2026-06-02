package rag

import (
	"context"
	"log"
	"strings"

	kb "interview-agents/api/handler/kb"
	"interview-agents/api/response"

	"github.com/cloudwego/hertz/pkg/app"
)

// RAGRetrieveRequest is the v1 public API request contract.
type RAGRetrieveRequest struct {
	AppID           string                 `json:"app_id"`
	KBID            uint64                 `json:"kb_id"`
	KBIDs           []uint64               `json:"kb_ids"`
	Query           string                 `json:"query" binding:"required"`
	TopK            int                    `json:"top_k"`
	StrategyProfile string                 `json:"strategy_profile"`
	MetadataFilter  map[string]interface{} `json:"metadata_filter"`
}

// RAGRetrieveResponse is the v1 public API response contract.
type RAGRetrieveResponse struct {
	RequestID       string            `json:"request_id"`
	Items           []RAGRetrieveItem `json:"items"`
	StrategyVersion string            `json:"strategy_version"`
	RequestCost     RAGRequestCost    `json:"request_cost"`
}

// RAGRetrieveItem represents a single retrieval result in the v1 API.
type RAGRetrieveItem struct {
	Content  string      `json:"content"`
	Score    float64     `json:"score"`
	Citation RAGCitation `json:"citation"`
	Source   RAGSource   `json:"source"`
}

// RAGCitation contains citation information for a retrieval result.
type RAGCitation struct {
	KBID       uint64 `json:"kb_id"`
	DocumentID uint64 `json:"document_id"`
	ChunkID    string `json:"chunk_id"`
	FileName   string `json:"file_name"`
	ChunkIndex int    `json:"chunk_index"`
}

// RAGSource contains source metadata for a retrieval result.
type RAGSource struct {
	Route            string `json:"route"`
	Collection       string `json:"collection"`
	RetrieverVersion string `json:"retriever_version"`
}

// RAGRequestCost contains cost information for the request.
type RAGRequestCost struct {
	EstimatedCost float64 `json:"estimated_cost"`
}

// allowedAppIDs is a static whitelist for the first version.
// TODO: move to config or database in later versions.
// NOTE: This is a legacy auth path. Will be replaced by API Key in Phase 2+.
var allowedAppIDs = map[string]string{
	"interview-agent":   "interview-agent",
	"mianshiba-web":     "mianshiba-web",
	"mianshiba-admin":   "mianshiba-admin",
}

// isLegacyAppID 检查是否是旧白名单 app_id
func isLegacyAppID(appID string) bool {
	_, ok := allowedAppIDs[appID]
	return ok
}

// Retrieve is the v1 public API handler for RAG retrieval.
// It validates app_id, then delegates to the existing kb.Retrieve handler
// to reuse the entire retrieval chain without duplicating logic.
func Retrieve(ctx context.Context, c *app.RequestContext) {
	// Parse v1 request to extract app_id for validation.
	var req RAGRetrieveRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.BadRequest(ctx, c, "Invalid request: "+err.Error())
		return
	}

	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		response.BadRequest(ctx, c, "query is required")
		return
	}

	// Validate app_id against whitelist.
	appID := strings.TrimSpace(req.AppID)
	if appID == "" {
		response.BadRequest(ctx, c, "app_id is required")
		return
	}
	if !isLegacyAppID(appID) {
		response.Forbidden(ctx, c, "invalid app_id")
		return
	}

	// 标记为 legacy 认证路径，便于 Phase 2 迁移时区分来源
	if isLegacyAppID(appID) {
		c.Set("auth_type", "legacy_app_id")
		c.Set("app_id", appID)
		c.Set("is_legacy", true)
		log.Printf("[Auth] Legacy app_id=%s, will be replaced by API Key in Phase 2", appID)
	}

	log.Printf("[RAG Public API] source_api=v1 app_id=%s query=%q top_k=%d", appID, req.Query, req.TopK)

	// Delegate to the existing kb.Retrieve handler to reuse the full retrieval chain.
	// The response format from kb.Retrieve already contains request_id, items, citation, source.
	kb.Retrieve(ctx, c)
}
