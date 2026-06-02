package model

import "time"

// RAGAPIKey API Key 模型
type RAGAPIKey struct {
	ID          uint64     `json:"id" gorm:"primaryKey;autoIncrement"`
	TenantID    uint64     `json:"tenant_id" gorm:"index;not null"`
	UserID      uint64     `json:"user_id" gorm:"index;not null"`
	AppID       string     `json:"app_id" gorm:"size:64;not null"`
	KeyHash     string     `json:"-" gorm:"size:255;not null"`
	KeyPrefix   string     `json:"key_prefix" gorm:"size:16;not null"`
	Name        string     `json:"name" gorm:"size:128"`
	Permissions string     `json:"-" gorm:"type:text"` // JSON string
	Status      string     `json:"status" gorm:"size:16;default:active;index"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	ExpiresAt   *time.Time `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"autoUpdateTime"`

	// 关联
	Tenant *RAGTenant `json:"tenant,omitempty" gorm:"foreignKey:TenantID"`
	User   *RAGUser   `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

func (RAGAPIKey) TableName() string {
	return "rag_api_key"
}
