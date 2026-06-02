package auth

import "context"

// AuthType 认证类型
type AuthType string

const (
	AuthTypeJWT            AuthType = "jwt"
	AuthTypeAPIKey         AuthType = "api_key"
	AuthTypeLegacyAppID    AuthType = "legacy_app_id"
	AuthTypeDevAdminBypass AuthType = "dev_admin_bypass"
	AuthTypeBootstrap      AuthType = "bootstrap"
)

// Identity 统一身份上下文
type Identity struct {
	AuthType    AuthType `json:"auth_type"`
	TenantID    uint64   `json:"tenant_id"`
	UserID      uint     `json:"user_id"`
	Role        string   `json:"role"`
	AppID       string   `json:"app_id"`
	APIKeyID    uint64   `json:"api_key_id"`
	Permissions []string `json:"permissions"`
	IsLegacy    bool     `json:"is_legacy"`
}

type contextKey struct{}

// WithIdentity 将身份信息注入 context
func WithIdentity(ctx context.Context, identity *Identity) context.Context {
	return context.WithValue(ctx, contextKey{}, identity)
}

// GetIdentity 从 context 获取身份信息
func GetIdentity(ctx context.Context) *Identity {
	if v, ok := ctx.Value(contextKey{}).(*Identity); ok {
		return v
	}
	return &Identity{}
}

// GetUserID 从 context 获取 user_id（向后兼容）
func GetUserID(ctx context.Context) uint {
	return GetIdentity(ctx).UserID
}

// GetTenantID 从 context 获取 tenant_id
func GetTenantID(ctx context.Context) uint64 {
	return GetIdentity(ctx).TenantID
}

// GetRole 从 context 获取 role
func GetRole(ctx context.Context) string {
	return GetIdentity(ctx).Role
}

// GetAppID 从 context 获取 app_id
func GetAppID(ctx context.Context) string {
	return GetIdentity(ctx).AppID
}

// IsAdmin 检查是否是管理员
func IsAdmin(ctx context.Context) bool {
	role := GetRole(ctx)
	return role == "admin" || role == "owner"
}

// IsLegacy 检查是否是旧 app_id 认证
func IsLegacy(ctx context.Context) bool {
	return GetIdentity(ctx).IsLegacy
}
