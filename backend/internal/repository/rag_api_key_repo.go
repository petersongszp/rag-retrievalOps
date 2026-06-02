package repository

import (
	"interview-agents/internal/model"

	"gorm.io/gorm"
)

type RAGAPIKeyRepository struct {
	db *gorm.DB
}

func NewRAGAPIKeyRepository(db *gorm.DB) *RAGAPIKeyRepository {
	return &RAGAPIKeyRepository{db: db}
}

func (r *RAGAPIKeyRepository) Create(key *model.RAGAPIKey) error {
	return r.db.Create(key).Error
}

func (r *RAGAPIKeyRepository) GetByID(id uint64) (*model.RAGAPIKey, error) {
	var key model.RAGAPIKey
	err := r.db.Where("id = ?", id).First(&key).Error
	return &key, err
}

func (r *RAGAPIKeyRepository) GetByIDForTenant(tenantID, id uint64) (*model.RAGAPIKey, error) {
	var key model.RAGAPIKey
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).First(&key).Error
	return &key, err
}

func (r *RAGAPIKeyRepository) GetByKeyHash(keyHash string) (*model.RAGAPIKey, error) {
	var key model.RAGAPIKey
	err := r.db.Where("key_hash = ?", keyHash).First(&key).Error
	return &key, err
}

func (r *RAGAPIKeyRepository) ListByTenantID(tenantID uint64) ([]*model.RAGAPIKey, error) {
	var keys []*model.RAGAPIKey
	err := r.db.Where("tenant_id = ?", tenantID).Order("created_at DESC").Find(&keys).Error
	return keys, err
}

func (r *RAGAPIKeyRepository) ListByUserID(userID uint64) ([]*model.RAGAPIKey, error) {
	var keys []*model.RAGAPIKey
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&keys).Error
	return keys, err
}

func (r *RAGAPIKeyRepository) Update(key *model.RAGAPIKey) error {
	return r.db.Save(key).Error
}

func (r *RAGAPIKeyRepository) UpdateLastUsed(id uint64) error {
	return r.db.Model(&model.RAGAPIKey{}).Where("id = ?", id).Update("last_used_at", gorm.Expr("NOW()")).Error
}

func (r *RAGAPIKeyRepository) Delete(id uint64) error {
	return r.db.Model(&model.RAGAPIKey{}).Where("id = ?", id).Update("status", "revoked").Error
}
