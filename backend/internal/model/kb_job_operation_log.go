package model

import "time"

var KBJobOperationLogDao _KBJobOperationLog

type (
	_KBJobOperationLog struct{}
	KBJobOperationLog  struct {
		ID              uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
		TenantID        uint64    `json:"tenant_id" gorm:"index"`
		JobID           uint64    `json:"job_id" gorm:"index;not null"`
		OperatorID      uint      `json:"operator_id" gorm:"index;not null"`
		Operation       string    `json:"operation" gorm:"size:64;not null;index"`
		OperationReason string    `json:"operation_reason" gorm:"size:500"`
		FromStatus      string    `json:"from_status" gorm:"size:20;not null"`
		ToStatus        string    `json:"to_status" gorm:"size:20;not null"`
		CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime:milli"`
	}
)

func (KBJobOperationLog) TableName() string {
	return "kb_job_operation_log"
}

func (d *_KBJobOperationLog) Create(record *KBJobOperationLog) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().Create(record).Error
}

func (d *_KBJobOperationLog) ListByJobID(jobID uint64) ([]*KBJobOperationLog, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}

	var records []*KBJobOperationLog
	err := getDB().
		Model(&KBJobOperationLog{}).
		Where("job_id = ?", jobID).
		Order("created_at ASC, id ASC").
		Find(&records).Error
	if err != nil {
		return nil, err
	}
	return records, nil
}
