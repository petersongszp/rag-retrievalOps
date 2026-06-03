package auth

import (
	"context"
	"log"

	"interview-agents/api/response"
	authpkg "interview-agents/internal/auth"

	"github.com/cloudwego/hertz/pkg/app"
)

// RefreshRequest 刷新 Token 请求
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Refresh 刷新 Token
func Refresh(ctx context.Context, c *app.RequestContext) {
	var req RefreshRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.BadRequest(ctx, c, "Invalid request: "+err.Error())
		return
	}

	// 解析 refresh token
	claims, err := jwtManager.ParseTokenClaims(req.RefreshToken)
	if err != nil {
		response.Error(ctx, c, 401, "Invalid refresh token")
		return
	}

	// 验证 token 类型
	if claims.TokenType != "refresh" {
		response.Error(ctx, c, 401, "Invalid token type")
		return
	}

	// 验证用户存在且活跃
	user, err := userRepo.GetByID(claims.UserID)
	if err != nil || user.Status != "active" {
		response.Error(ctx, c, 401, "User not found or disabled")
		return
	}

	// 生成新 Token
	accessToken, err := jwtManager.GenerateAccessToken(user.ID, user.TenantID, user.Role)
	if err != nil {
		log.Printf("[Auth] Generate access token failed: %v", err)
		response.InternalServerError(ctx, c, "Failed to generate token")
		return
	}

	refreshToken, err := jwtManager.GenerateRefreshToken(user.ID, user.TenantID, user.Role)
	if err != nil {
		log.Printf("[Auth] Generate refresh token failed: %v", err)
		response.InternalServerError(ctx, c, "Failed to generate token")
		return
	}

	log.Printf("[Auth] Token refreshed: user_id=%d", user.ID)

	response.Success(ctx, c, authpkg.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    7200,
		UserID:       uint(user.ID),
		Role:         user.Role,
		TenantID:     user.TenantID,
	})
}
