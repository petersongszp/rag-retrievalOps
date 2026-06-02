package model

import "time"

const (
	RAGTenantKBPermissionRead  = "read"
	RAGTenantKBPermissionWrite = "write"
	RAGTenantKBPermissionAdmin = "admin"
)

type RAGTenantKBPermission struct {
	ID         uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	TenantID   uint64    `json:"tenant_id" gorm:"index;uniqueIndex:uk_tenant_kb_permission,priority:1;not null"`
	KBID       uint64    `json:"kb_id" gorm:"index;uniqueIndex:uk_tenant_kb_permission,priority:2;not null"`
	Permission string    `json:"permission" gorm:"size:16;not null;default:'read'"`
	CreatedAt  time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (RAGTenantKBPermission) TableName() string {
	return "rag_tenant_kb_permission"
}
