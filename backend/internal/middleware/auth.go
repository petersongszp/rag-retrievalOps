package middleware

import (
	"context"
	"strings"

	authpkg "interview-agents/internal/auth"
	"interview-agents/internal/repository"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
)

// JWTAuth JWT 认证中间件，解析 Bearer Token 并注入 auth.Identity 到 context
func JWTAuth(jwtManager *authpkg.JWTManager) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		// 从 Header 获取 Token
		authHeader := c.GetHeader("Authorization")
		if len(authHeader) == 0 {
			c.JSON(401, utils.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// 解析 Bearer Token
		tokenString := strings.TrimPrefix(string(authHeader), "Bearer ")
		if tokenString == string(authHeader) {
			c.JSON(401, utils.H{"error": "Invalid authorization format"})
			c.Abort()
			return
		}

		// 验证 Token
		claims, err := jwtManager.ValidateToken(tokenString)
		if err != nil {
			c.JSON(401, utils.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// 验证 token 类型（只接受 access token）
		if claims.TokenType != "access" {
			c.JSON(401, utils.H{"error": "Invalid token type"})
			c.Abort()
			return
		}

		// 注入统一身份上下文
		identity := &authpkg.Identity{
			AuthType: authpkg.AuthTypeJWT,
			UserID:   uint(claims.UserID),
			TenantID: claims.TenantID,
			Role:     claims.Role,
		}

		ctx = authpkg.WithIdentity(ctx, identity)
		c.Set("auth_type", "jwt")
		c.Set("user_id", claims.UserID)
		c.Set("tenant_id", claims.TenantID)
		c.Set("role", claims.Role)

		c.Next(ctx)
	}
}

// RequireRole 角色检查中间件，需配合 JWTAuth 使用（依赖 context 中的 Identity）
func RequireRole(roles ...string) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity := authpkg.GetIdentity(ctx)

		allowed := false
		for _, role := range roles {
			if identity.Role == role {
				allowed = true
				break
			}
		}

		if !allowed {
			c.JSON(403, utils.H{"error": "Permission denied"})
			c.Abort()
			return
		}

		c.Next(ctx)
	}
}

// RequirePermission 权限检查中间件
func RequirePermission(permission string) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity := authpkg.GetIdentity(ctx)

		if !authpkg.HasPermission(identity.Role, permission) {
			c.JSON(403, utils.H{
				"error":    "Permission denied",
				"required": permission,
				"role":     identity.Role,
			})
			c.Abort()
			return
		}

		c.Next(ctx)
	}
}

// RequireTenantActive 租户状态检查中间件
func RequireTenantActive(tenantRepo *repository.RAGTenantRepository) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity := authpkg.GetIdentity(ctx)

		if identity.TenantID == 0 {
			c.Next(ctx)
			return
		}

		tenant, err := tenantRepo.GetByID(identity.TenantID)
		if err != nil {
			c.JSON(404, utils.H{"error": "Tenant not found"})
			c.Abort()
			return
		}

		if tenant.Status != "active" {
			c.JSON(403, utils.H{
				"error":  "Tenant is not active",
				"status": tenant.Status,
			})
			c.Abort()
			return
		}

		c.Next(ctx)
	}
}

// APIKeyAuth API Key 认证中间件
func APIKeyAuth(apiKeyRepo *repository.RAGAPIKeyRepository, tenantRepo *repository.RAGTenantRepository) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		// 从 Header 获取 Token
		authHeader := c.GetHeader("Authorization")
		if len(authHeader) == 0 {
			c.JSON(401, utils.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// 解析 Bearer Token
		tokenString := strings.TrimPrefix(string(authHeader), "Bearer ")
		if tokenString == string(authHeader) {
			c.JSON(401, utils.H{"error": "Invalid authorization format"})
			c.Abort()
			return
		}

		// 检查是否是 API Key 格式
		if !authpkg.ValidateAPIKeyFormat(tokenString) {
			c.JSON(401, utils.H{"error": "Invalid API key format"})
			c.Abort()
			return
		}

		// 计算 hash
		keyHash := authpkg.HashAPIKey(tokenString)

		// 查询 API Key
		apiKey, err := apiKeyRepo.GetByKeyHash(keyHash)
		if err != nil {
			c.JSON(401, utils.H{"error": "Invalid API key"})
			c.Abort()
			return
		}

		// 检查状态
		if apiKey.Status != "active" {
			c.JSON(401, utils.H{"error": "API key is revoked", "code": authpkg.ErrCodeAPIKeyRevoked})
			c.Abort()
			return
		}

		// 检查过期
		if authpkg.IsAPIKeyExpired(apiKey.ExpiresAt) {
			c.JSON(401, utils.H{"error": "API key is expired", "code": authpkg.ErrCodeAPIKeyExpired})
			c.Abort()
			return
		}

		// 检查租户状态
		tenant, err := tenantRepo.GetByID(apiKey.TenantID)
		if err != nil || tenant.Status != "active" {
			c.JSON(401, utils.H{"error": "Tenant is disabled"})
			c.Abort()
			return
		}

		// 更新最后使用时间
		apiKeyRepo.UpdateLastUsed(apiKey.ID)

		// 注入身份上下文
		identity := &authpkg.Identity{
			AuthType:    authpkg.AuthTypeAPIKey,
			TenantID:    apiKey.TenantID,
			UserID:      uint(apiKey.UserID),
			Role:        "member", // API Key 默认 member 角色
			AppID:       apiKey.AppID,
			APIKeyID:    apiKey.ID,
			Permissions: authpkg.ParsePermissions(apiKey.Permissions),
			IsLegacy:    false,
		}

		ctx = authpkg.WithIdentity(ctx, identity)
		c.Set("auth_type", "api_key")
		c.Set("tenant_id", apiKey.TenantID)
		c.Set("user_id", apiKey.UserID)
		c.Set("app_id", apiKey.AppID)
		c.Set("api_key_id", apiKey.ID)

		c.Next(ctx)
	}
}
