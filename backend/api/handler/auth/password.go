package auth

import (
	"context"
	"log"

	"interview-agents/api/response"
	authpkg "interview-agents/internal/auth"

	"github.com/cloudwego/hertz/pkg/app"
)

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=12"`
}

// ChangePassword 修改密码
func ChangePassword(ctx context.Context, c *app.RequestContext) {
	identity := authpkg.GetIdentity(ctx)

	if identity.UserID == 0 {
		response.Error(ctx, c, 401, "Not authenticated")
		return
	}

	var req ChangePasswordRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.BadRequest(ctx, c, "Invalid request: "+err.Error())
		return
	}

	// 校验新密码强度
	if err := authpkg.ValidatePasswordStrength(req.NewPassword); err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}

	// 获取用户
	user, err := userRepo.GetByID(uint64(identity.UserID))
	if err != nil {
		response.Error(ctx, c, 404, "User not found")
		return
	}

	// 验证旧密码
	if !authpkg.CheckPassword(req.OldPassword, user.PasswordHash) {
		response.Error(ctx, c, 401, "Invalid old password")
		return
	}

	// 哈希新密码
	newHash, err := authpkg.HashPassword(req.NewPassword)
	if err != nil {
		response.InternalServerError(ctx, c, "Failed to process password")
		return
	}

	// 更新密码
	user.PasswordHash = newHash
	if err := userRepo.Update(user); err != nil {
		response.InternalServerError(ctx, c, "Failed to update password")
		return
	}

	log.Printf("[Auth] Password changed: user_id=%d", user.ID)

	response.Success(ctx, c, map[string]string{"message": "Password changed successfully"})
}
