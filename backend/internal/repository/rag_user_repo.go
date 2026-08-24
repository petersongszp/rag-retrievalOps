package repository

import (
	"errors"
	"interview-agents/internal/model"

	"gorm.io/gorm"
)

type RAGUserRepository struct {
	db *gorm.DB
}

func NewRAGUserRepository(db *gorm.DB) *RAGUserRepository {
	return &RAGUserRepository{db: db}
}

func (r *RAGUserRepository) Create(user *model.RAGUser) error {
	return r.db.Create(user).Error
}

func (r *RAGUserRepository) GetByID(id uint64) (*model.RAGUser, error) {
	var user model.RAGUser
	err := r.db.Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *RAGUserRepository) GetByEmail(email string) (*model.RAGUser, error) {
	var user model.RAGUser
	err := r.db.Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *RAGUserRepository) Update(user *model.RAGUser) error {
	return r.db.Save(user).Error
}

func (r *RAGUserRepository) UpdateLastLogin(userID uint64) error {
	return r.db.Model(&model.RAGUser{}).Where("id = ?", userID).Update("last_login_at", gorm.Expr("NOW()")).Error
}

func (r *RAGUserRepository) ListByTenantID(tenantID uint64) ([]*model.RAGUser, error) {
	var users []*model.RAGUser
	err := r.db.Where("tenant_id = ?", tenantID).Find(&users).Error
	return users, err
}
