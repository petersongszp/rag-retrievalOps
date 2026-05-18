package model

import (
	"time"
)

type KBKnowledgeBaseStatus string

const (
	KBKnowledgeBaseStatusActive   KBKnowledgeBaseStatus = "active"
	KBKnowledgeBaseStatusDisabled KBKnowledgeBaseStatus = "disabled"
)

var KBKnowledgeBaseDao _KBKnowledgeBase

type (
	_KBKnowledgeBase struct{}
	KBKnowledgeBase  struct {
		ID          uint64                `json:"id" gorm:"primaryKey;autoIncrement"`
		UserID      uint                  `json:"user_id" gorm:"index;not null"`
		Name        string                `json:"name" gorm:"size:255;not null"`
		Description string                `json:"description" gorm:"size:1000"`
		Status      KBKnowledgeBaseStatus `json:"status" gorm:"size:20;not null;default:'active';index"`
		CreatedAt   time.Time             `json:"created_at" gorm:"autoCreateTime:milli"`
		UpdatedAt   time.Time             `json:"updated_at" gorm:"autoUpdateTime:milli"`
	}
)

func (KBKnowledgeBase) TableName() string {
	return "kb_knowledge_base"
}

func (d *_KBKnowledgeBase) Create(kb *KBKnowledgeBase) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().Create(kb).Error
}

func (d *_KBKnowledgeBase) GetByID(id uint64) (*KBKnowledgeBase, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var kb KBKnowledgeBase
	err := getDB().Where("id = ?", id).First(&kb).Error
	if err != nil {
		return nil, err
	}
	return &kb, nil
}

func (d *_KBKnowledgeBase) GetByUserIDAndName(userID uint, name string) (*KBKnowledgeBase, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var kb KBKnowledgeBase
	err := getDB().Where("user_id = ? AND name = ?", userID, name).First(&kb).Error
	if err != nil {
		return nil, err
	}
	return &kb, nil
}

func (d *_KBKnowledgeBase) ListByUserID(userID uint, page, pageSize int) ([]*KBKnowledgeBase, int64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var list []*KBKnowledgeBase
	var total int64

	query := getDB().Model(&KBKnowledgeBase{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Offset((page - 1) * pageSize).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&list).Error
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (d *_KBKnowledgeBase) UpdateByID(id uint64, updates map[string]interface{}) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	if len(updates) == 0 {
		return nil
	}
	return getDB().Model(&KBKnowledgeBase{}).Where("id = ?", id).Updates(updates).Error
}

func (d *_KBKnowledgeBase) DeleteByID(id uint64) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().Where("id = ?", id).Delete(&KBKnowledgeBase{}).Error
}
