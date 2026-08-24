package auth

import (
	"context"
	"log"
	"time"

	"interview-agents/api/response"
	authpkg "interview-agents/internal/auth"

	"github.com/cloudwego/hertz/pkg/app"
)

// Login 用户登录
func Login(ctx context.Context, c *app.RequestContext) {
	var req authpkg.LoginRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.BadRequest(ctx, c, "Invalid request: "+err.Error())
		return
	}

	// 查找用户
	user, err := userRepo.GetByEmail(req.Email)
	if err != nil || user == nil {
		response.Error(ctx, c, 401, "Invalid email or password")
		return
	}

	// 检查用户状态
	if user.Status != "active" {
		response.Error(ctx, c, 401, "Account is disabled")
		return
	}

	// 检查租户状态
	tenant, err := tenantRepo.GetByID(user.TenantID)
	if err != nil || tenant.Status != "active" {
		response.Error(ctx, c, 401, "Tenant is disabled")
		return
	}

	// 验证密码
	if !authpkg.CheckPassword(req.Password, user.PasswordHash) {
		response.Error(ctx, c, 401, "Invalid email or password")
		return
	}

	// 生成 Token
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

	// 更新最后登录时间
	userRepo.UpdateLastLogin(user.ID)

	log.Printf("[Auth] User logged in: email=%s, user_id=%d", req.Email, user.ID)

	response.Success(ctx, c, authpkg.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int((2 * time.Hour).Seconds()),
		UserID:       uint(user.ID),
		Role:         user.Role,
		TenantID:     user.TenantID,
	})
}
