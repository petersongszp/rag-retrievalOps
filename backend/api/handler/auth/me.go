package auth

import (
	"context"

	"interview-agents/api/response"
	authpkg "interview-agents/internal/auth"

	"github.com/cloudwego/hertz/pkg/app"
)

// Me 获取当前用户信息
func Me(ctx context.Context, c *app.RequestContext) {
	identity := authpkg.GetIdentity(ctx)

	if identity.UserID == 0 {
		response.Error(ctx, c, 401, "Not authenticated")
		return
	}

	user, err := userRepo.GetByID(uint64(identity.UserID))
	if err != nil {
		response.Error(ctx, c, 404, "User not found")
		return
	}

	tenant, _ := tenantRepo.GetByID(user.TenantID)

	tenantName := ""
	if tenant != nil {
		tenantName = tenant.Name
	}

	response.Success(ctx, c, map[string]interface{}{
		"user_id":     user.ID,
		"email":       user.Email,
		"name":        user.Name,
		"role":        user.Role,
		"tenant_id":   user.TenantID,
		"tenant_name": tenantName,
		"created_at":  user.CreatedAt,
	})
}
