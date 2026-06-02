package model

import "time"

// RAGTenant 租户模型
type RAGTenant struct {
	ID                uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	Name              string    `json:"name" gorm:"size:128;not null"`
	Slug              string    `json:"slug" gorm:"size:64;uniqueIndex;not null"`
	Plan              string    `json:"plan" gorm:"size:32;default:free"`
	Status            string    `json:"status" gorm:"size:16;default:active;index"`
	MaxKBCount        int       `json:"max_kb_count" gorm:"default:5"`
	MaxDocCount       int       `json:"max_doc_count" gorm:"default:100"`
	MaxStorageMB      int       `json:"max_storage_mb" gorm:"default:1024"`
	MaxAPICallsPerDay int       `json:"max_api_calls_per_day" gorm:"default:10000"`
	CreatedAt         time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (RAGTenant) TableName() string {
	return "rag_tenant"
}
