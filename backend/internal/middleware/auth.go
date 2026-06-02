package middleware

import (
	"context"
	"strings"

	authpkg "interview-agents/internal/auth"

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
