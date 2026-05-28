package model

import "time"

var KBAuditEventDao _KBAuditEvent

type (
	_KBAuditEvent struct{}
	KBAuditEvent  struct {
		ID           uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
		AuditTraceID string    `json:"audit_trace_id" gorm:"size:128;index;not null"`
		RequestID    string    `json:"request_id" gorm:"size:64;index"`
		OperatorID   uint      `json:"operator_id" gorm:"index"`
		UserID       uint      `json:"user_id" gorm:"index"`
		KBID         uint64    `json:"kb_id" gorm:"index"`
		DocumentID   uint64    `json:"document_id" gorm:"index"`
		Action       string    `json:"action" gorm:"size:64;index;not null"`
		ResourceType string    `json:"resource_type" gorm:"size:64;index;not null"`
		ResourceID   string    `json:"resource_id" gorm:"size:128;index"`
		BeforeData   string    `json:"before" gorm:"type:text"`
		AfterData    string    `json:"after" gorm:"type:text"`
		Result       string    `json:"result" gorm:"size:64"`
		Reason       string    `json:"reason" gorm:"size:512"`
		CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime:milli;index"`
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
