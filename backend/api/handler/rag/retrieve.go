package rag

import (
	"context"
	"log"
	"strings"

	kb "interview-agents/api/handler/kb"
	"interview-agents/api/response"
	auth "interview-agents/internal/auth"

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

// allowedAppIDs is a static whitelist for legacy compatibility.
// NOTE: This is a legacy auth path. Will be fully removed after Phase 2 migration.
var allowedAppIDs = map[string]string{
	"interview-agent": "interview-agent",
	"mianshiba-web":   "mianshiba-web",
	"mianshiba-admin": "mianshiba-admin",
}

// isLegacyAppID 检查是否是旧白名单 app_id
func isLegacyAppID(appID string) bool {
	_, ok := allowedAppIDs[appID]
	return ok
}

// Retrieve is the v1 public API handler for RAG retrieval.
// Auth priority: API Key (middleware injected) > JWT > Legacy app_id whitelist.
func Retrieve(ctx context.Context, c *app.RequestContext) {
	// 解析请求
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

	// 获取身份（优先从 API Key 中间件注入）
	identity := auth.GetIdentity(ctx)

	switch identity.AuthType {
	case auth.AuthTypeAPIKey:
		// API Key 认证：app_id 从 Key 推导，不需要请求体
		// 如果请求体带了 app_id，以 Key 绑定的为准
		req.AppID = identity.AppID

	case auth.AuthTypeJWT:
		// JWT 认证：管理端调用
		if req.AppID == "" {
			req.AppID = identity.AppID
		}

	case auth.AuthTypeLegacyAppID, "":
		// Legacy 兼容：旧 app_id 白名单
		if req.AppID == "" {
			response.BadRequest(ctx, c, "app_id is required for legacy access")
			return
		}

		if !isLegacyAppID(req.AppID) {
			response.Error(ctx, c, 403, "Invalid app_id")
			return
		}

		// 标记为 legacy
		identity = &auth.Identity{
			AuthType: auth.AuthTypeLegacyAppID,
			AppID:    req.AppID,
			IsLegacy: true,
		}
		ctx = auth.WithIdentity(ctx, identity)
		c.Set("auth_type", "legacy_app_id")
		c.Set("app_id", req.AppID)
		c.Set("is_legacy", true)
		log.Printf("[Auth] Legacy app_id=%s, deprecated after Phase 2", req.AppID)

	default:
		response.Error(ctx, c, 401, "Authentication required")
		return
	}

	log.Printf("[RAG Public API] source_api=v1 auth_type=%s app_id=%s query=%q top_k=%d", identity.AuthType, req.AppID, req.Query, req.TopK)

	// Delegate to the existing kb.Retrieve handler to reuse the full retrieval chain.
	kb.Retrieve(ctx, c)
}
