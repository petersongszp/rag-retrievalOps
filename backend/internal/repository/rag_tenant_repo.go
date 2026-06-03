package repository

import (
	"interview-agents/internal/model"

	"gorm.io/gorm"
)

type RAGTenantRepository struct {
	db *gorm.DB
}

func NewRAGTenantRepository(db *gorm.DB) *RAGTenantRepository {
	return &RAGTenantRepository{db: db}
}

func (r *RAGTenantRepository) Create(tenant *model.RAGTenant) error {
	return r.db.Create(tenant).Error
}

func (r *RAGTenantRepository) GetByID(id uint64) (*model.RAGTenant, error) {
	var tenant model.RAGTenant
	err := r.db.Where("id = ?", id).First(&tenant).Error
	return &tenant, err
}

func (r *RAGTenantRepository) GetBySlug(slug string) (*model.RAGTenant, error) {
	var tenant model.RAGTenant
	err := r.db.Where("slug = ?", slug).First(&tenant).Error
	return &tenant, err
}

func (r *RAGTenantRepository) Update(tenant *model.RAGTenant) error {
	return r.db.Save(tenant).Error
}
