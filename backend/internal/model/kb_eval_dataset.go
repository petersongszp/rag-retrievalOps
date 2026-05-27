package model

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type KBEvalDatasetStatus string

const (
	KBEvalDatasetStatusDraft    KBEvalDatasetStatus = "draft"
	KBEvalDatasetStatusReady    KBEvalDatasetStatus = "ready"
	KBEvalDatasetStatusInvalid  KBEvalDatasetStatus = "invalid"
	KBEvalDatasetStatusArchived KBEvalDatasetStatus = "archived"
)

var KBEvalDatasetDao _KBEvalDataset

type KBEvalDatasetListFilter struct {
	KBID     *uint64
	Status   *KBEvalDatasetStatus
	Keyword  *string
	Page     int
	PageSize int
}

type (
	_KBEvalDataset struct{}
	KBEvalDataset  struct {
		ID          uint64              `json:"id" gorm:"primaryKey;autoIncrement"`
		Name        string              `json:"name" gorm:"size:255;not null;index"`
		Description string              `json:"description" gorm:"size:1000"`
		KbID        *uint64             `json:"kb_id" gorm:"index"`
		CaseCount   int                 `json:"case_count" gorm:"default:0"`
		Status      KBEvalDatasetStatus `json:"status" gorm:"size:20;not null;default:'draft';index"`
		CreatedBy   uint                `json:"created_by" gorm:"index;not null"`
		CreatedAt   time.Time           `json:"created_at" gorm:"autoCreateTime:milli"`
		UpdatedAt   time.Time           `json:"updated_at" gorm:"autoUpdateTime:milli"`
	}
)

func (KBEvalDataset) TableName() string {
	return "kb_eval_dataset"
}

func ParseKBEvalDatasetStatus(raw string) (KBEvalDatasetStatus, bool) {
	status := KBEvalDatasetStatus(strings.TrimSpace(raw))
	switch status {
	case KBEvalDatasetStatusDraft,
		KBEvalDatasetStatusReady,
		KBEvalDatasetStatusInvalid,
		KBEvalDatasetStatusArchived:
		return status, true
	default:
		return "", false
	}
}

func (d *_KBEvalDataset) Create(dataset *KBEvalDataset) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().Create(dataset).Error
}

func (d *_KBEvalDataset) GetByID(id uint64) (*KBEvalDataset, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var dataset KBEvalDataset
	if err := getDB().Where("id = ?", id).First(&dataset).Error; err != nil {
		return nil, err
	}
	return &dataset, nil
}

func (d *_KBEvalDataset) ListWithFilter(filter KBEvalDatasetListFilter) ([]*KBEvalDataset, int64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 10
	}

	var list []*KBEvalDataset
	var total int64

	query := applyKBEvalDatasetFilters(getDB().Model(&KBEvalDataset{}), filter)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Offset((filter.Page - 1) * filter.PageSize).
		Limit(filter.PageSize).
		Order("updated_at DESC").
		Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (d *_KBEvalDataset) UpdateByID(id uint64, updates map[string]interface{}) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	if len(updates) == 0 {
		return nil
	}
	return getDB().Model(&KBEvalDataset{}).Where("id = ?", id).Updates(updates).Error
}

func (d *_KBEvalDataset) UpdateCaseCount(id uint64, caseCount int) error {
	return d.UpdateByID(id, map[string]interface{}{"case_count": caseCount})
}

func (d *_KBEvalDataset) UpdateStatus(id uint64, status KBEvalDatasetStatus) error {
	return d.UpdateByID(id, map[string]interface{}{"status": status})
}

func applyKBEvalDatasetFilters(query *gorm.DB, filter KBEvalDatasetListFilter) *gorm.DB {
	if filter.KBID != nil {
		query = query.Where("kb_id = ?", *filter.KBID)
	}
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}
	if filter.Keyword != nil {
		keyword := strings.TrimSpace(*filter.Keyword)
		if keyword != "" {
			query = query.Where("name LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
		}
	}
	return query
}
