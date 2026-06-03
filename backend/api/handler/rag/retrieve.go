package rag

import (
	"context"
	"errors"
	"log"
	"strings"

	kb "interview-agents/api/handler/kb"
	"interview-agents/api/response"
	auth "interview-agents/internal/auth"
	"interview-agents/internal/repository"
	"interview-agents/internal/service"

	"github.com/cloudwego/hertz/pkg/app"
	"gorm.io/gorm"
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

const (
	systemTenantID uint64 = 1
	systemUserID   uint   = 1
)

var (
	kbTenantRepo  *repository.KBTenantRepository
	kbPermService *service.KBPermissionService
)

func getAPIKeyRepo() *repository.RAGAPIKeyRepository {
	db := repository.GetDB()
	if db == nil {
		return nil
	}
	return repository.NewRAGAPIKeyRepository(db)
}

func initRetrieveDependencies() bool {
	db := repository.GetDB()
	if db == nil {
		return false
	}
	if kbTenantRepo == nil {
		kbTenantRepo = repository.NewKBTenantRepository(db)
	}
	if kbPermService == nil {
		kbPermService = service.NewKBPermissionService(
			repository.NewRAGTenantKBPermissionRepository(db),
		)
	}
	return true
}

func setIdentityContext(ctx context.Context, c *app.RequestContext, identity *auth.Identity) context.Context {
	ctx = auth.WithIdentity(ctx, identity)
	c.Set("auth_type", string(identity.AuthType))
	c.Set("user_id", identity.UserID)
	c.Set("role", identity.Role)
	c.Set("tenant_id", identity.TenantID)
	c.Set("app_id", identity.AppID)
	c.Set("api_key_id", identity.APIKeyID)
	c.Set("is_legacy", identity.IsLegacy)
	return ctx
}

func resolveRequestedKBIDs(req RAGRetrieveRequest) []uint64 {
	ids := make([]uint64, 0, len(req.KBIDs)+1)
	seen := make(map[uint64]struct{}, len(req.KBIDs)+1)

	for _, kbID := range req.KBIDs {
		if kbID == 0 {
			continue
		}
		if _, ok := seen[kbID]; ok {
			continue
		}
		seen[kbID] = struct{}{}
		ids = append(ids, kbID)
	}

	if req.KBID > 0 {
		if _, ok := seen[req.KBID]; !ok {
			ids = append(ids, req.KBID)
		}
	}

	return ids
}

func authorizeRetrieveKBIDs(ctx context.Context, c *app.RequestContext, tenantID uint64, req RAGRetrieveRequest) bool {
	if tenantID == 0 {
		response.Error(ctx, c, 401, "Authentication required")
		return false
	}
	if !initRetrieveDependencies() {
		response.Error(ctx, c, 500, "Database is not initialized")
		return false
	}

	requestedKBIDs := resolveRequestedKBIDs(req)
	if len(requestedKBIDs) == 0 {
		response.BadRequest(ctx, c, "kb_id or kb_ids is required")
		return false
	}

	for _, kbID := range requestedKBIDs {
		if _, err := kbTenantRepo.GetByIDForTenant(tenantID, kbID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				response.Error(ctx, c, 404, "Knowledge base not found")
				return false
			}
			response.Error(ctx, c, 500, "Failed to validate knowledge base access")
			return false
		}

		allowed, err := kbPermService.CheckPermission(tenantID, kbID, "read")
		if err != nil {
			response.Error(ctx, c, 500, "Failed to validate knowledge base permission")
			return false
		}
		if !allowed {
			response.Error(ctx, c, 403, "Permission denied")
			return false
		}
	}

	return true
}

// allowedAppIDs is a static whitelist for legacy compatibility.
// NOTE: This is a legacy auth path. Will be fully removed after Phase 2 migration.
var allowedAppIDs = map[string]string{
	"interview-agent": "interview-agent",
	"mianshiba-web":   "mianshiba-web",
	"mianshiba-admin": "mianshiba-admin",
}

func isLegacyAppID(appID string) bool {
	_, ok := allowedAppIDs[appID]
	return ok
}

// Retrieve is the v1 public API handler for RAG retrieval.
// Auth priority: API Key (header) > JWT (header) > Legacy app_id (body).
func Retrieve(ctx context.Context, c *app.RequestContext) {
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

	identity := auth.GetIdentity(ctx)
	if identity.AuthType == "" {
		authHeader := string(c.GetHeader("Authorization"))
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if auth.ValidateAPIKeyFormat(token) {
				apiKeyRepo := getAPIKeyRepo()
				if apiKeyRepo != nil {
					keyHash := auth.HashAPIKey(token)
					apiKey, err := apiKeyRepo.GetByKeyHash(keyHash)
					if err == nil {
						if apiKey.Status == "revoked" {
							response.Error(ctx, c, 401, "API key is revoked")
							return
						}
						if auth.IsAPIKeyExpired(apiKey.ExpiresAt) {
							response.Error(ctx, c, 401, "API key is expired")
							return
						}

						identity = &auth.Identity{
							AuthType:    auth.AuthTypeAPIKey,
							TenantID:    apiKey.TenantID,
							UserID:      uint(apiKey.UserID),
							Role:        "member",
							AppID:       apiKey.AppID,
							APIKeyID:    apiKey.ID,
							Permissions: auth.ParsePermissions(apiKey.Permissions),
						}
						ctx = setIdentityContext(ctx, c, identity)
						apiKeyRepo.UpdateLastUsed(apiKey.ID)
					}
				}
			}
		}
	}

	switch identity.AuthType {
	case auth.AuthTypeAPIKey:
		req.AppID = identity.AppID
		ctx = setIdentityContext(ctx, c, identity)
		if !authorizeRetrieveKBIDs(ctx, c, identity.TenantID, req) {
			return
		}

	case auth.AuthTypeJWT:
		if req.AppID == "" {
			req.AppID = identity.AppID
		}
		ctx = setIdentityContext(ctx, c, identity)
		if !authorizeRetrieveKBIDs(ctx, c, identity.TenantID, req) {
			return
		}

	case auth.AuthTypeLegacyAppID, "":
		if req.AppID == "" {
			response.BadRequest(ctx, c, "app_id is required for legacy access")
			return
		}
		if !isLegacyAppID(req.AppID) {
			response.Error(ctx, c, 403, "Invalid app_id")
			return
		}

		identity = &auth.Identity{
			AuthType: auth.AuthTypeLegacyAppID,
			TenantID: systemTenantID,
			UserID:   systemUserID,
			Role:     "member",
			AppID:    req.AppID,
			IsLegacy: true,
		}
		ctx = setIdentityContext(ctx, c, identity)
		log.Printf("[Auth] Legacy app_id=%s, deprecated after Phase 2", req.AppID)
		if !authorizeRetrieveKBIDs(ctx, c, identity.TenantID, req) {
			return
		}

	default:
		response.Error(ctx, c, 401, "Authentication required")
		return
	}

	log.Printf(
		"[RAG Public API] source_api=v1 auth_type=%s tenant_id=%d app_id=%s query=%q top_k=%d",
		identity.AuthType,
		identity.TenantID,
		req.AppID,
		req.Query,
		req.TopK,
	)

	kb.Retrieve(ctx, c)
}
