package repository

import (
	"interview-agents/internal/model"

	"gorm.io/gorm"
)

type KBTenantRepository struct {
	db *gorm.DB
}

func NewKBTenantRepository(db *gorm.DB) *KBTenantRepository {
	return &KBTenantRepository{db: db}
}

func (r *KBTenantRepository) ListByTenant(tenantID uint64, page, pageSize int) ([]*model.KBKnowledgeBase, error) {
	var knowledgeBases []*model.KBKnowledgeBase

	query := r.db.Where("tenant_id = ?", tenantID).Order("created_at DESC")
	if page > 0 && pageSize > 0 {
		query = query.Offset((page - 1) * pageSize).Limit(pageSize)
	}

	err := query.Find(&knowledgeBases).Error
	return knowledgeBases, err
}

func (r *KBTenantRepository) GetByIDForTenant(tenantID, kbID uint64) (*model.KBKnowledgeBase, error) {
	var knowledgeBase model.KBKnowledgeBase
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, kbID).First(&knowledgeBase).Error
	return &knowledgeBase, err
}

func (r *KBTenantRepository) CreateForTenant(tenantID uint64, kb *model.KBKnowledgeBase) error {
	kb.TenantID = tenantID
	return r.db.Create(kb).Error
}

func (r *KBTenantRepository) UpdateByIDForTenant(tenantID, kbID uint64, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	safeUpdates := make(map[string]interface{}, len(updates)+1)
	for key, value := range updates {
		if key == "tenant_id" {
			continue
		}
		safeUpdates[key] = value
	}

	return r.db.Model(&model.KBKnowledgeBase{}).
		Where("tenant_id = ? AND id = ?", tenantID, kbID).
		Updates(safeUpdates).Error
}

func (r *KBTenantRepository) DeleteByIDForTenant(tenantID, kbID uint64) error {
	return r.db.Where("tenant_id = ? AND id = ?", tenantID, kbID).Delete(&model.KBKnowledgeBase{}).Error
}

func (r *KBTenantRepository) CountByTenant(tenantID uint64) (int64, error) {
	var count int64
	err := r.db.Model(&model.KBKnowledgeBase{}).Where("tenant_id = ?", tenantID).Count(&count).Error
	return count, err
}
