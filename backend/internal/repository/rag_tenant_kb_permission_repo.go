package repository

import (
	"interview-agents/internal/model"

	"gorm.io/gorm"
)

type RAGTenantKBPermissionRepository struct {
	db *gorm.DB
}

func NewRAGTenantKBPermissionRepository(db *gorm.DB) *RAGTenantKBPermissionRepository {
	return &RAGTenantKBPermissionRepository{db: db}
}

func (r *RAGTenantKBPermissionRepository) Create(permission *model.RAGTenantKBPermission) error {
	return r.db.Create(permission).Error
}

func (r *RAGTenantKBPermissionRepository) GetByTenantAndKB(tenantID, kbID uint64) (*model.RAGTenantKBPermission, error) {
	var permission model.RAGTenantKBPermission
	err := r.db.Where("tenant_id = ? AND kb_id = ?", tenantID, kbID).First(&permission).Error
	return &permission, err
}

func (r *RAGTenantKBPermissionRepository) ListByTenantID(tenantID uint64) ([]*model.RAGTenantKBPermission, error) {
	var permissions []*model.RAGTenantKBPermission
	err := r.db.Where("tenant_id = ?", tenantID).Order("created_at DESC").Find(&permissions).Error
	return permissions, err
}

func (r *RAGTenantKBPermissionRepository) ListByKBID(kbID uint64) ([]*model.RAGTenantKBPermission, error) {
	var permissions []*model.RAGTenantKBPermission
	err := r.db.Where("kb_id = ?", kbID).Order("created_at DESC").Find(&permissions).Error
	return permissions, err
}

func (r *RAGTenantKBPermissionRepository) Update(permission *model.RAGTenantKBPermission) error {
	return r.db.Save(permission).Error
}

func (r *RAGTenantKBPermissionRepository) Delete(tenantID, kbID uint64) error {
	return r.db.Where("tenant_id = ? AND kb_id = ?", tenantID, kbID).Delete(&model.RAGTenantKBPermission{}).Error
}
