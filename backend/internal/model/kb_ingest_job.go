package model

import (
	"time"

	"gorm.io/gorm"
)

type KBIngestJobStatus string

const (
	KBIngestJobStatusPending    KBIngestJobStatus = "pending"
	KBIngestJobStatusProcessing KBIngestJobStatus = "processing"
	KBIngestJobStatusCompleted  KBIngestJobStatus = "completed"
	KBIngestJobStatusFailed     KBIngestJobStatus = "failed"
	KBIngestJobStatusDead       KBIngestJobStatus = "dead"
)

var KBIngestJobDao _KBIngestJob

type (
	_KBIngestJob struct{}
	KBIngestJob  struct {
		ID              uint64            `json:"id" gorm:"primaryKey;autoIncrement"`
		KbID            uint64            `json:"kb_id" gorm:"index;not null"`
		DocumentID      uint64            `json:"document_id" gorm:"index;not null"`
		UserID          uint              `json:"user_id" gorm:"index;not null"`
		Status          KBIngestJobStatus `json:"status" gorm:"size:20;not null;default:'pending';index"`
		RetryCount      int               `json:"retry_count" gorm:"default:0"`
		ErrorMsg        string            `json:"error_msg" gorm:"size:500"`
		NextRetryAt     *time.Time        `json:"next_retry_at" gorm:"index"`
		LastErrorCode   string            `json:"last_error_code" gorm:"size:64;index"`
		LastErrorDetail string            `json:"last_error_detail" gorm:"size:1000"`
		StartedAt       *time.Time        `json:"started_at"`
		FinishedAt      *time.Time        `json:"finished_at"`
		CreatedAt       time.Time         `json:"created_at" gorm:"autoCreateTime:milli"`
		UpdatedAt       time.Time         `json:"updated_at" gorm:"autoUpdateTime:milli"`
	}
)

func (KBIngestJob) TableName() string {
	return "kb_ingest_job"
}

func (d *_KBIngestJob) Create(job *KBIngestJob) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().Create(job).Error
}

func (d *_KBIngestJob) GetByID(id uint64) (*KBIngestJob, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var job KBIngestJob
	err := getDB().Where("id = ?", id).First(&job).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (d *_KBIngestJob) GetLatestByDocumentID(documentID uint64) (*KBIngestJob, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var job KBIngestJob
	err := getDB().Where("document_id = ?", documentID).
		Order("created_at DESC").
		First(&job).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (d *_KBIngestJob) ListByKbID(kbID uint64, page, pageSize int) ([]*KBIngestJob, int64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var list []*KBIngestJob
	var total int64

	query := getDB().Model(&KBIngestJob{}).Where("kb_id = ?", kbID)
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

func (d *_KBIngestJob) ListPendingJobs(limit int) ([]*KBIngestJob, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var list []*KBIngestJob
	err := getDB().Where("status = ?", KBIngestJobStatusPending).
		Order("created_at ASC").
		Limit(limit).
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (d *_KBIngestJob) UpdateStatus(id uint64, status KBIngestJobStatus, errorMsg string) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	updates := map[string]interface{}{
		"status":    status,
		"error_msg": errorMsg,
	}
	now := time.Now()
	switch status {
	case KBIngestJobStatusProcessing:
		updates["started_at"] = now
		updates["finished_at"] = nil
	case KBIngestJobStatusCompleted, KBIngestJobStatusFailed, KBIngestJobStatusDead:
		updates["finished_at"] = now
	}
	return getDB().Model(&KBIngestJob{}).Where("id = ?", id).Updates(updates).Error
}

func (d *_KBIngestJob) IncrementRetry(id uint64) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().Model(&KBIngestJob{}).Where("id = ?", id).
		Update("retry_count", gorm.Expr("retry_count + 1")).Error
}

func (d *_KBIngestJob) UpdateFailureState(id uint64, status KBIngestJobStatus, errorMsg, errorCode, errorDetail string, nextRetryAt *time.Time, incrementRetry bool) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}

	updates := map[string]interface{}{
		"status":            status,
		"error_msg":         errorMsg,
		"last_error_code":   errorCode,
		"last_error_detail": errorDetail,
		"next_retry_at":     nextRetryAt,
		"finished_at":       time.Now(),
	}
	if incrementRetry {
		updates["retry_count"] = gorm.Expr("retry_count + 1")
	}
	return getDB().Model(&KBIngestJob{}).Where("id = ?", id).Updates(updates).Error
}

func (d *_KBIngestJob) ListRetryDueJobs(limit int, now time.Time) ([]*KBIngestJob, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	if limit <= 0 {
		limit = 50
	}

	var list []*KBIngestJob
	err := getDB().
		Where("status = ? AND next_retry_at IS NOT NULL AND next_retry_at <= ?", KBIngestJobStatusFailed, now).
		Order("next_retry_at ASC, id ASC").
		Limit(limit).
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (d *_KBIngestJob) MarkPendingForRetry(id uint64, now time.Time) (bool, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	tx := getDB().Model(&KBIngestJob{}).
		Where("id = ? AND status = ? AND next_retry_at IS NOT NULL AND next_retry_at <= ?", id, KBIngestJobStatusFailed, now).
		Updates(map[string]interface{}{
			"status":        KBIngestJobStatusPending,
			"next_retry_at": nil,
			"error_msg":     "",
			"started_at":    nil,
			"finished_at":   nil,
		})
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}
