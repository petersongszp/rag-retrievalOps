package model

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

var KBAuditEventDao _KBAuditEvent

type KBAuditEventListFilter struct {
	Action       *string
	ResourceType *string
	ActorID      *uint
	KBID         *uint64
	RequestID    *string
	DocumentID   *uint64
	StartTime    *time.Time
	EndTime      *time.Time
	Page         int
	PageSize     int
}

type (
	_KBAuditEvent struct{}
	KBAuditEvent  struct {
		ID                    uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
		AuditTraceID          string    `json:"audit_trace_id" gorm:"size:128;index;not null"`
		RequestID             string    `json:"request_id" gorm:"size:64;index"`
		OperatorID            uint      `json:"operator_id" gorm:"index"`
		UserID                uint      `json:"user_id" gorm:"index"`
		KBID                  uint64    `json:"kb_id" gorm:"index"`
		DocumentID            uint64    `json:"document_id" gorm:"index"`
		Action                string    `json:"action" gorm:"size:64;index;not null"`
		ResourceType          string    `json:"resource_type" gorm:"size:64;index;not null"`
		ResourceID            string    `json:"resource_id" gorm:"size:128;index"`
		BeforeData            string    `json:"before" gorm:"type:text"`
		AfterData             string    `json:"after" gorm:"type:text"`
		Result                string    `json:"result" gorm:"size:64"`
		Reason                string    `json:"reason" gorm:"size:512"`
		ActorName             string    `json:"actor_name" gorm:"size:128"`
		IP                    string    `json:"ip" gorm:"size:128"`
		UserAgent             string    `json:"user_agent" gorm:"size:512"`
		SensitiveFieldsMasked string    `json:"sensitive_fields_masked" gorm:"type:text"`
		CreatedAt             time.Time `json:"created_at" gorm:"autoCreateTime:milli;index"`
	}
)

func (KBAuditEvent) TableName() string {
	return "kb_audit_event"
}

func (d *_KBAuditEvent) Create(record *KBAuditEvent) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().Create(record).Error
}

func (d *_KBAuditEvent) List(limit int) ([]*KBAuditEvent, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	if limit <= 0 {
		limit = 100
	}
	var records []*KBAuditEvent
	err := getDB().Model(&KBAuditEvent{}).Order("created_at DESC").Limit(limit).Find(&records).Error
	return records, err
}

func (d *_KBAuditEvent) ListWithFilter(filter KBAuditEventListFilter) ([]*KBAuditEvent, int64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}

	query := applyKBAuditEventFilters(getDB().Model(&KBAuditEvent{}), filter)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []*KBAuditEvent
	if err := query.Order("created_at DESC").
		Offset((filter.Page - 1) * filter.PageSize).
		Limit(filter.PageSize).
		Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

func (d *_KBAuditEvent) GetByID(id uint64) (*KBAuditEvent, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var record KBAuditEvent
	if err := getDB().Where("id = ?", id).First(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func applyKBAuditEventFilters(query *gorm.DB, filter KBAuditEventListFilter) *gorm.DB {
	if filter.Action != nil && strings.TrimSpace(*filter.Action) != "" {
		query = query.Where("action = ?", strings.TrimSpace(*filter.Action))
	}
	if filter.ResourceType != nil && strings.TrimSpace(*filter.ResourceType) != "" {
		query = query.Where("resource_type = ?", strings.TrimSpace(*filter.ResourceType))
	}
	if filter.ActorID != nil {
		query = query.Where("operator_id = ? OR user_id = ?", *filter.ActorID, *filter.ActorID)
	}
	if filter.KBID != nil {
		query = query.Where("kb_id = ?", *filter.KBID)
	}
	if filter.RequestID != nil && strings.TrimSpace(*filter.RequestID) != "" {
		query = query.Where("request_id = ?", strings.TrimSpace(*filter.RequestID))
	}
	if filter.DocumentID != nil {
		query = query.Where("document_id = ?", *filter.DocumentID)
	}
	if filter.StartTime != nil {
		query = query.Where("created_at >= ?", *filter.StartTime)
	}
	if filter.EndTime != nil {
		query = query.Where("created_at <= ?", *filter.EndTime)
	}
	return query
}
