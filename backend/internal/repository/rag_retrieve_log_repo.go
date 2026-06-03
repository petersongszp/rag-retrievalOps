package repository

import (
	"interview-agents/internal/model"

	"gorm.io/gorm"
)

type RAGRetrieveLogRepository struct {
	db *gorm.DB
}

func NewRAGRetrieveLogRepository(db *gorm.DB) *RAGRetrieveLogRepository {
	return &RAGRetrieveLogRepository{db: db}
}

func (r *RAGRetrieveLogRepository) ListByTenant(tenantID uint64, page, pageSize int) ([]*model.KBRetrieveLog, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	query := r.db.Model(&model.KBRetrieveLog{}).Where("tenant_id = ?", tenantID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []*model.KBRetrieveLog
	err := query.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

func (r *RAGRetrieveLogRepository) GetByRequestIDForTenant(tenantID uint64, requestID string) (*model.KBRetrieveLog, error) {
	var logEntry model.KBRetrieveLog
	err := r.db.Where("tenant_id = ? AND request_id = ?", tenantID, requestID).First(&logEntry).Error
	if err != nil {
		return nil, err
	}
	return &logEntry, nil
}
