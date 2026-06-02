package auth

// 注册请求
type RegisterRequest struct {
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required,min=12"`
	Name       string `json:"name" binding:"required"`
	TenantName string `json:"tenant_name"`
}

// 注册响应
type RegisterResponse struct {
	UserID   uint   `json:"user_id"`
	Email    string `json:"email"`
	TenantID uint64 `json:"tenant_id"`
}

// 登录请求
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// 登录响应
type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	UserID       uint   `json:"user_id"`
	Role         string `json:"role"`
	TenantID     uint64 `json:"tenant_id"`
}

// API Key 创建请求
type CreateAPIKeyRequest struct {
	Name        string   `json:"name" binding:"required"`
	AppID       string   `json:"app_id" binding:"required"`
	Permissions []string `json:"permissions"`
	ExpiresIn   int      `json:"expires_in"` // 秒，0 表示永不过期
}

// API Key 创建响应（只在创建时返回完整 key）
type CreateAPIKeyResponse struct {
	ID        uint64 `json:"id"`
	Name      string `json:"name"`
	AppID     string `json:"app_id"`
	Key       string `json:"key"`       // 只在创建时返回
	KeyPrefix string `json:"key_prefix"`
	CreatedAt string `json:"created_at"`
}

// API Key 列表项（不包含完整 key）
type APIKeyItem struct {
	ID          uint64   `json:"id"`
	Name        string   `json:"name"`
	AppID       string   `json:"app_id"`
	KeyPrefix   string   `json:"key_prefix"`
	Permissions []string `json:"permissions"`
	Status      string   `json:"status"`
	LastUsedAt  string   `json:"last_used_at"`
	CreatedAt   string   `json:"created_at"`
}

// 错误码定义
const (
	ErrCodeInvalidCredentials = "INVALID_CREDENTIALS"
	ErrCodeEmailExists        = "EMAIL_ALREADY_EXISTS"
	ErrCodeWeakPassword       = "WEAK_PASSWORD"
	ErrCodeInvalidAPIKey      = "INVALID_API_KEY"
	ErrCodeAPIKeyRevoked      = "API_KEY_REVOKED"
	ErrCodeAPIKeyExpired      = "API_KEY_EXPIRED"
	ErrCodePermissionDenied   = "PERMISSION_DENIED"
	ErrCodeTenantNotFound     = "TENANT_NOT_FOUND"
	ErrCodeTenantDisabled     = "TENANT_DISABLED"
	ErrCodeQuotaExceeded      = "QUOTA_EXCEEDED"
	ErrCodeTokenExpired       = "TOKEN_EXPIRED"
	ErrCodeTokenInvalid       = "TOKEN_INVALID"
)
