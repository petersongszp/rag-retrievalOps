package repository

import (
	"interview-agents/internal/model"

	"gorm.io/gorm"
)

type RAGTenantUsageRepository struct {
	db *gorm.DB
}

func NewRAGTenantUsageRepository(db *gorm.DB) *RAGTenantUsageRepository {
	return &RAGTenantUsageRepository{db: db}
}

func (r *RAGTenantUsageRepository) CountKnowledgeBases(tenantID uint64) (int64, error) {
	var count int64
	err := r.db.Model(&model.KBKnowledgeBase{}).
		Where("tenant_id = ?", tenantID).
		Count(&count).Error
	return count, err
}

func (r *RAGTenantUsageRepository) CountDocuments(tenantID uint64) (int64, error) {
	var count int64
	err := r.db.Model(&model.KBDocument{}).
		Where("tenant_id = ? AND deleted = 0", tenantID).
		Count(&count).Error
	return count, err
}

func (r *RAGTenantUsageRepository) SumStorageBytes(tenantID uint64) (int64, error) {
	var totalBytes int64
	err := r.db.Model(&model.KBDocument{}).
		Where("tenant_id = ? AND deleted = 0", tenantID).
		Select("COALESCE(SUM(file_size), 0)").
		Scan(&totalBytes).Error
	return totalBytes, err
}
