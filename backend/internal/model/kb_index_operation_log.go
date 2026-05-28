package model

import "time"

var KBIndexOperationLogDao _KBIndexOperationLog

type (
	_KBIndexOperationLog struct{}
	KBIndexOperationLog  struct {
		ID              uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
		IndexVersion    string    `json:"index_version" gorm:"size:128;index;not null"`
		CollectionName  string    `json:"collection_name" gorm:"size:255;index;not null"`
		Operation       string    `json:"operation" gorm:"size:64;index;not null"`
		FromRole        string    `json:"from_role" gorm:"size:32"`
		ToRole          string    `json:"to_role" gorm:"size:32"`
		OperatorID      uint      `json:"operator_id" gorm:"index"`
		OperationReason string    `json:"operation_reason" gorm:"size:512"`
		HealthStatus    string    `json:"health_status" gorm:"size:64"`
		RollbackTarget  string    `json:"rollback_target" gorm:"size:128"`
		CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime:milli;index"`
	}
)

func (KBIndexOperationLog) TableName() string {
	return "kb_index_operation_log"
}

func (d *_KBIndexOperationLog) Create(record *KBIndexOperationLog) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().Create(record).Error
}

func (d *_KBIndexOperationLog) List(limit int) ([]*KBIndexOperationLog, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	if limit <= 0 {
		limit = 50
	}
	var records []*KBIndexOperationLog
	err := getDB().Model(&KBIndexOperationLog{}).Order("created_at DESC").Limit(limit).Find(&records).Error
	return records, err
}
