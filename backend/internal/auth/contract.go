package auth

import "time"

type RegisterRequest struct {
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required,min=12"`
	Name       string `json:"name" binding:"required"`
	TenantName string `json:"tenant_name"`
}

type RegisterResponse struct {
	UserID   uint   `json:"user_id"`
	Email    string `json:"email"`
	TenantID uint64 `json:"tenant_id"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	UserID       uint   `json:"user_id"`
	Role         string `json:"role"`
	TenantID     uint64 `json:"tenant_id"`
}

type CreateAPIKeyRequest struct {
	Name        string   `json:"name" binding:"required"`
	AppID       string   `json:"app_id" binding:"required"`
	Permissions []string `json:"permissions"`
	ExpiresIn   int      `json:"expires_in"`
}

type CreateAPIKeyResponse struct {
	ID          uint64   `json:"id"`
	Name        string   `json:"name"`
	AppID       string   `json:"app_id"`
	Key         string   `json:"key"`
	KeyPrefix   string   `json:"key_prefix"`
	Permissions []string `json:"permissions"`
	CreatedAt   string   `json:"created_at"`
}

type APIKeyItem struct {
	ID          uint64   `json:"id"`
	Name        string   `json:"name"`
	AppID       string   `json:"app_id"`
	KeyPrefix   string   `json:"key_prefix"`
	Permissions []string `json:"permissions"`
	Status      string   `json:"status"`
	LastUsedAt  string   `json:"last_used_at"`
	ExpiresAt   string   `json:"expires_at"`
	CreatedAt   string   `json:"created_at"`
}

type UpdateAPIKeyRequest struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

type RotateAPIKeyResponse struct {
	ID        uint64 `json:"id"`
	Key       string `json:"key"`
	KeyPrefix string `json:"key_prefix"`
	CreatedAt string `json:"created_at"`
}

type TenantResponse struct {
	TenantID          uint64    `json:"tenant_id"`
	Name              string    `json:"name"`
	Slug              string    `json:"slug"`
	Plan              string    `json:"plan"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	MaxKBCount        int       `json:"max_kb_count"`
	MaxDocCount       int       `json:"max_doc_count"`
	MaxStorageMB      int       `json:"max_storage_mb"`
	MaxAPICallsPerDay int       `json:"max_api_calls_per_day"`
}

type TenantUsageLimits struct {
	MaxKBCount        int `json:"max_kb_count"`
	MaxDocCount       int `json:"max_doc_count"`
	MaxStorageMB      int `json:"max_storage_mb"`
	MaxAPICallsPerDay int `json:"max_api_calls_per_day"`
}

type TenantUsageResponse struct {
	APICallsToday int               `json:"api_calls_today"`
	KBCount       int               `json:"kb_count"`
	DocCount      int               `json:"doc_count"`
	StorageMB     int64             `json:"storage_mb"`
	Limits        TenantUsageLimits `json:"limits"`
}

const (
	ErrCodeInvalidCredentials = "INVALID_CREDENTIALS"
	ErrCodeEmailExists        = "EMAIL_ALREADY_EXISTS"
	ErrCodeWeakPassword       = "WEAK_PASSWORD"
	ErrCodeInvalidAPIKey      = "INVALID_API_KEY"
	ErrCodeAPIKeyRevoked      = "API_KEY_REVOKED"
	ErrCodeAPIKeyExpired      = "API_KEY_EXPIRED"
	ErrCodeAPIKeyInactive     = "API_KEY_INACTIVE"
	ErrCodePermissionDenied   = "PERMISSION_DENIED"
	ErrCodeTenantNotFound     = "TENANT_NOT_FOUND"
	ErrCodeTenantDisabled     = "TENANT_DISABLED"
	ErrCodeQuotaExceeded      = "QUOTA_EXCEEDED"
	ErrCodeTokenExpired       = "TOKEN_EXPIRED"
	ErrCodeTokenInvalid       = "TOKEN_INVALID"
)
