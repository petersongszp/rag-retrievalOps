package repository

import (
	"interview-agents/internal/model"

	"gorm.io/gorm"
)

type AuditEventRepository struct {
	db *gorm.DB
}

func NewAuditEventRepository(db *gorm.DB) *AuditEventRepository {
	return &AuditEventRepository{db: db}
}

func (r *AuditEventRepository) ListByTenant(tenantID uint64, page, pageSize int) ([]*model.KBAuditEvent, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	query := r.db.Model(&model.KBAuditEvent{}).Where("tenant_id = ?", tenantID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var events []*model.KBAuditEvent
	err := query.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&events).Error
	if err != nil {
		return nil, 0, err
	}

	return events, total, nil
}
