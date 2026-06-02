package model

import "time"

// RAGUser 用户模型
type RAGUser struct {
	ID           uint64     `json:"id" gorm:"primaryKey;autoIncrement"`
	TenantID     uint64     `json:"tenant_id" gorm:"index;not null"`
	Email        string     `json:"email" gorm:"size:255;uniqueIndex;not null"`
	PasswordHash string     `json:"-" gorm:"size:255;not null"`
	Name         string     `json:"name" gorm:"size:128;not null"`
	Role         string     `json:"role" gorm:"size:32;default:member;index"`
	Status       string     `json:"status" gorm:"size:16;default:active;index"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time  `json:"updated_at" gorm:"autoUpdateTime"`

	// 关联
	Tenant *RAGTenant `json:"tenant,omitempty" gorm:"foreignKey:TenantID"`
}

func (RAGUser) TableName() string {
	return "rag_user"
}
