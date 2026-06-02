package auth

import (
	"context"
	"log"
	"strconv"
	"time"

	"interview-agents/api/response"
	authpkg "interview-agents/internal/auth"
	"interview-agents/internal/model"
	"interview-agents/internal/repository"

	"github.com/cloudwego/hertz/pkg/app"
	"gorm.io/gorm"
)

var apiKeyRepo *repository.RAGAPIKeyRepository

// InitAPIKeyHandler 初始化 API Key handler 依赖
func InitAPIKeyHandler(db *gorm.DB) {
	apiKeyRepo = repository.NewRAGAPIKeyRepository(db)
}

// ListAPIKeys 获取 API Key 列表
func ListAPIKeys(ctx context.Context, c *app.RequestContext) {
	identity := authpkg.GetIdentity(ctx)
	if identity.UserID == 0 {
		response.Error(ctx, c, 401, "Not authenticated")
		return
	}

	keys, err := apiKeyRepo.ListByTenantID(identity.TenantID)
	if err != nil {
		response.InternalServerError(ctx, c, "Failed to list API keys")
		return
	}

	items := make([]authpkg.APIKeyItem, 0, len(keys))
	for _, key := range keys {
		items = append(items, authpkg.APIKeyItem{
			ID:          key.ID,
			Name:        key.Name,
			AppID:       key.AppID,
			KeyPrefix:   key.KeyPrefix,
			Permissions: authpkg.ParsePermissions(key.Permissions),
			Status:      key.Status,
			LastUsedAt:  formatTime(key.LastUsedAt),
			ExpiresAt:   formatTime(key.ExpiresAt),
			CreatedAt:   key.CreatedAt.Format(time.RFC3339),
		})
	}

	response.Success(ctx, c, map[string]interface{}{
		"items": items,
		"total": len(items),
	})
}

// CreateAPIKey 创建 API Key
func CreateAPIKey(ctx context.Context, c *app.RequestContext) {
	identity := authpkg.GetIdentity(ctx)
	if identity.UserID == 0 {
		response.Error(ctx, c, 401, "Not authenticated")
		return
	}

	var req authpkg.CreateAPIKeyRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.BadRequest(ctx, c, "Invalid request: "+err.Error())
		return
	}

	// 生成 API Key
	key, keyHash, keyPrefix, err := authpkg.GenerateAPIKey()
	if err != nil {
		log.Printf("[Auth] Generate API key failed: %v", err)
		response.InternalServerError(ctx, c, "Failed to generate API key")
		return
	}

	// 计算过期时间
	var expiresAt *time.Time
	if req.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(req.ExpiresIn) * time.Second)
		expiresAt = &t
	}

	// 创建记录
	apiKey := &model.RAGAPIKey{
		TenantID:    identity.TenantID,
		UserID:      uint64(identity.UserID),
		AppID:       req.AppID,
		KeyHash:     keyHash,
		KeyPrefix:   keyPrefix,
		Name:        req.Name,
		Permissions: authpkg.FormatPermissions(req.Permissions),
		Status:      "active",
		ExpiresAt:   expiresAt,
	}

	if err := apiKeyRepo.Create(apiKey); err != nil {
		log.Printf("[Auth] Create API key failed: %v", err)
		response.InternalServerError(ctx, c, "Failed to create API key")
		return
	}

	log.Printf("[Auth] API key created: id=%d, app_id=%s, tenant_id=%d", apiKey.ID, req.AppID, identity.TenantID)

	response.Success(ctx, c, authpkg.CreateAPIKeyResponse{
		ID:          apiKey.ID,
		Name:        apiKey.Name,
		AppID:       apiKey.AppID,
		Key:         key, // 只在创建时返回明文
		KeyPrefix:   keyPrefix,
		Permissions: req.Permissions,
		CreatedAt:   apiKey.CreatedAt.Format(time.RFC3339),
	})
}

// UpdateAPIKey 更新 API Key
func UpdateAPIKey(ctx context.Context, c *app.RequestContext) {
	identity := authpkg.GetIdentity(ctx)
	if identity.UserID == 0 {
		response.Error(ctx, c, 401, "Not authenticated")
		return
	}

	id, err := parseUintParam(c, "id")
	if err != nil {
		response.BadRequest(ctx, c, "Invalid API key ID")
		return
	}

	var req authpkg.UpdateAPIKeyRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.BadRequest(ctx, c, "Invalid request: "+err.Error())
		return
	}

	apiKey, err := apiKeyRepo.GetByID(id)
	if err != nil {
		response.Error(ctx, c, 404, "API key not found")
		return
	}

	// 检查权限
	if apiKey.TenantID != identity.TenantID {
		response.Error(ctx, c, 403, "Access denied")
		return
	}

	// 更新字段
	if req.Name != "" {
		apiKey.Name = req.Name
	}
	if req.Permissions != nil {
		apiKey.Permissions = authpkg.FormatPermissions(req.Permissions)
	}

	if err := apiKeyRepo.Update(apiKey); err != nil {
		response.InternalServerError(ctx, c, "Failed to update API key")
		return
	}

	response.Success(ctx, c, map[string]string{"message": "API key updated"})
}

// DeleteAPIKey 吊销 API Key
func DeleteAPIKey(ctx context.Context, c *app.RequestContext) {
	identity := authpkg.GetIdentity(ctx)
	if identity.UserID == 0 {
		response.Error(ctx, c, 401, "Not authenticated")
		return
	}

	id, err := parseUintParam(c, "id")
	if err != nil {
		response.BadRequest(ctx, c, "Invalid API key ID")
		return
	}

	apiKey, err := apiKeyRepo.GetByID(id)
	if err != nil {
		response.Error(ctx, c, 404, "API key not found")
		return
	}

	// 检查权限
	if apiKey.TenantID != identity.TenantID {
		response.Error(ctx, c, 403, "Access denied")
		return
	}

	if err := apiKeyRepo.Delete(id); err != nil {
		response.InternalServerError(ctx, c, "Failed to revoke API key")
		return
	}

	log.Printf("[Auth] API key revoked: id=%d, tenant_id=%d", id, identity.TenantID)

	response.Success(ctx, c, map[string]string{"message": "API key revoked"})
}

// 辅助函数

func parseUintParam(c *app.RequestContext, name string) (uint64, error) {
	val := c.Param(name)
	return strconv.ParseUint(val, 10, 64)
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
