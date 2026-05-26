package model

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type KBIngestJobStatus string

const (
	KBIngestJobStatusPending    KBIngestJobStatus = "pending"
	KBIngestJobStatusProcessing KBIngestJobStatus = "processing"
	KBIngestJobStatusCompleted  KBIngestJobStatus = "completed"
	KBIngestJobStatusFailed     KBIngestJobStatus = "failed"
	KBIngestJobStatusRetrying   KBIngestJobStatus = "retrying"
	KBIngestJobStatusDead       KBIngestJobStatus = "dead"
	KBIngestJobStatusCanceled   KBIngestJobStatus = "canceled"
)

var KBIngestJobDao _KBIngestJob
var ErrInvalidKBIngestJobTransition = errors.New("invalid kb ingest job status transition")

var kbIngestJobTransitions = map[KBIngestJobStatus]map[KBIngestJobStatus]struct{}{
	KBIngestJobStatusPending: {
		KBIngestJobStatusProcessing: {},
		KBIngestJobStatusFailed:     {},
		KBIngestJobStatusCanceled:   {},
	},
	KBIngestJobStatusProcessing: {
		KBIngestJobStatusCompleted: {},
		KBIngestJobStatusFailed:    {},
		KBIngestJobStatusRetrying:  {},
		KBIngestJobStatusCanceled:  {},
	},
	KBIngestJobStatusFailed: {
		KBIngestJobStatusRetrying: {},
		KBIngestJobStatusDead:     {},
		KBIngestJobStatusCanceled: {},
	},
	KBIngestJobStatusRetrying: {
		KBIngestJobStatusProcessing: {},
		KBIngestJobStatusFailed:     {},
		KBIngestJobStatusDead:       {},
		KBIngestJobStatusCanceled:   {},
	},
	KBIngestJobStatusDead: {
		KBIngestJobStatusRetrying: {},
	},
}

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
		OperatorID      uint              `json:"operator_id" gorm:"index;default:0"`
		Operation       string            `json:"operation" gorm:"size:64"`
		OperationReason string            `json:"operation_reason" gorm:"size:500"`
		OperatedAt      *time.Time        `json:"operated_at"`
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

func (d *_KBIngestJob) ListByKbID(kbID uint64, status *KBIngestJobStatus, page, pageSize int) ([]*KBIngestJob, int64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	var list []*KBIngestJob
	var total int64

	query := getDB().Model(&KBIngestJob{}).Where("kb_id = ?", kbID)
	if status != nil {
		query = query.Where("status = ?", *status)
	}
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

func (d *_KBIngestJob) ListByUserID(userID uint, status *KBIngestJobStatus, page, pageSize int) ([]*KBIngestJob, int64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	var list []*KBIngestJob
	var total int64

	query := getDB().Model(&KBIngestJob{}).Where("user_id = ?", userID)
	if status != nil {
		query = query.Where("status = ?", *status)
	}

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

func (d *_KBIngestJob) List(status *KBIngestJobStatus, page, pageSize int) ([]*KBIngestJob, int64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	var list []*KBIngestJob
	var total int64

	query := getDB().Model(&KBIngestJob{})
	if status != nil {
		query = query.Where("status = ?", *status)
	}

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

func (d *_KBIngestJob) ListByCreatedAt(startTime, endTime time.Time, kbID *uint64) ([]*KBIngestJob, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}

	var list []*KBIngestJob
	query := getDB().Model(&KBIngestJob{}).
		Where("created_at >= ? AND created_at <= ?", startTime, endTime)
	if kbID != nil {
		query = query.Where("kb_id = ?", *kbID)
	}

	err := query.Order("created_at ASC").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
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
	_, err := d.updateStatusWithGuard(id, status, errorMsg, nil)
	return err
}

func (d *_KBIngestJob) UpdateStatusFrom(id uint64, status KBIngestJobStatus, errorMsg string, fromStatuses ...KBIngestJobStatus) (bool, error) {
	return d.updateStatusWithGuard(id, status, errorMsg, fromStatuses)
}

func (d *_KBIngestJob) MarkRetrying(id uint64, operatorID uint, reason string) (bool, error) {
	now := time.Now()
	updates := map[string]interface{}{
		"status":            KBIngestJobStatusRetrying,
		"next_retry_at":     nil,
		"error_msg":         "",
		"last_error_code":   "",
		"last_error_detail": "",
		"started_at":        nil,
		"finished_at":       nil,
		"operator_id":       operatorID,
		"operation":         "retry",
		"operation_reason":  reason,
		"operated_at":       now,
	}
	return d.updateWithCustomFields(
		id,
		[]KBIngestJobStatus{KBIngestJobStatusFailed, KBIngestJobStatusDead},
		KBIngestJobStatusRetrying,
		updates,
	)
}

func (d *_KBIngestJob) MarkCanceled(id uint64, operatorID uint, reason string, fromStatuses ...KBIngestJobStatus) (bool, error) {
	now := time.Now()
	updates := map[string]interface{}{
		"status":            KBIngestJobStatusCanceled,
		"next_retry_at":     nil,
		"error_msg":         firstNonEmpty(reason, "canceled by operator"),
		"last_error_code":   "canceled_by_operator",
		"last_error_detail": firstNonEmpty(reason, "canceled by operator"),
		"operator_id":       operatorID,
		"operation":         "cancel",
		"operation_reason":  reason,
		"operated_at":       now,
		"finished_at":       now,
	}
	if len(fromStatuses) == 0 {
		fromStatuses = []KBIngestJobStatus{
			KBIngestJobStatusPending,
			KBIngestJobStatusRetrying,
			KBIngestJobStatusProcessing,
			KBIngestJobStatusFailed,
		}
	}
	return d.updateWithCustomFields(id, fromStatuses, KBIngestJobStatusCanceled, updates)
}

func (d *_KBIngestJob) ClaimForProcessing(id uint64) (bool, error) {
	return d.UpdateStatusFrom(id, KBIngestJobStatusProcessing, "", KBIngestJobStatusPending, KBIngestJobStatusRetrying)
}

func (d *_KBIngestJob) updateStatusWithGuard(id uint64, status KBIngestJobStatus, errorMsg string, fromStatuses []KBIngestJobStatus) (bool, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}

	if !isKnownKBIngestJobStatus(status) {
		return false, fmt.Errorf("unsupported kb ingest job status: %s", status)
	}

	if len(fromStatuses) == 0 {
		fromStatuses = transitionSourcesTo(status)
	}
	if len(fromStatuses) == 0 {
		return false, ErrInvalidKBIngestJobTransition
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
	case KBIngestJobStatusCompleted, KBIngestJobStatusFailed, KBIngestJobStatusDead, KBIngestJobStatusCanceled:
		updates["finished_at"] = now
	case KBIngestJobStatusRetrying:
		updates["started_at"] = nil
		updates["finished_at"] = nil
		updates["next_retry_at"] = nil
	}
	return d.updateWithCustomFields(id, fromStatuses, status, updates)
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

	if status != KBIngestJobStatusFailed && status != KBIngestJobStatusDead && status != KBIngestJobStatusRetrying {
		return fmt.Errorf("failure state must be failed/dead/retrying, got: %s", status)
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
	_, err := d.updateWithCustomFields(
		id,
		[]KBIngestJobStatus{KBIngestJobStatusPending, KBIngestJobStatusProcessing, KBIngestJobStatusRetrying},
		status,
		updates,
	)
	return err
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
		Where("status = ? AND next_retry_at IS NOT NULL AND next_retry_at <= ?", KBIngestJobStatusRetrying, now).
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
		Where("id = ? AND status = ? AND next_retry_at IS NOT NULL AND next_retry_at <= ?", id, KBIngestJobStatusRetrying, now).
		Updates(map[string]interface{}{
			"status":        KBIngestJobStatusRetrying,
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

func (d *_KBIngestJob) updateWithCustomFields(id uint64, fromStatuses []KBIngestJobStatus, toStatus KBIngestJobStatus, updates map[string]interface{}) (bool, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	if len(fromStatuses) == 0 {
		return false, ErrInvalidKBIngestJobTransition
	}
	allowedFromStatuses := make([]KBIngestJobStatus, 0, len(fromStatuses))
	for _, from := range fromStatuses {
		if !canKBIngestJobTransition(from, toStatus) {
			continue
		}
		allowedFromStatuses = append(allowedFromStatuses, from)
	}
	if len(allowedFromStatuses) == 0 {
		return false, ErrInvalidKBIngestJobTransition
	}

	tx := getDB().Model(&KBIngestJob{}).
		Where("id = ? AND status IN ?", id, allowedFromStatuses).
		Updates(updates)
	if tx.Error != nil {
		return false, tx.Error
	}
	if tx.RowsAffected > 0 {
		return true, nil
	}
	job, err := d.GetByID(id)
	if err != nil {
		return false, err
	}
	return false, fmt.Errorf("%w: current=%s target=%s", ErrInvalidKBIngestJobTransition, job.Status, toStatus)
}

func transitionSourcesTo(toStatus KBIngestJobStatus) []KBIngestJobStatus {
	allowed := make([]KBIngestJobStatus, 0)
	for from, targets := range kbIngestJobTransitions {
		if _, ok := targets[toStatus]; ok {
			allowed = append(allowed, from)
		}
	}
	return allowed
}

func canKBIngestJobTransition(from, to KBIngestJobStatus) bool {
	targets, ok := kbIngestJobTransitions[from]
	if !ok {
		return false
	}
	_, ok = targets[to]
	return ok
}

func isKnownKBIngestJobStatus(status KBIngestJobStatus) bool {
	switch status {
	case KBIngestJobStatusPending,
		KBIngestJobStatusProcessing,
		KBIngestJobStatusCompleted,
		KBIngestJobStatusFailed,
		KBIngestJobStatusRetrying,
		KBIngestJobStatusDead,
		KBIngestJobStatusCanceled:
		return true
	default:
		return false
	}
}

func ParseKBIngestJobStatus(raw string) (KBIngestJobStatus, bool) {
	status := KBIngestJobStatus(raw)
	return status, isKnownKBIngestJobStatus(status)
}

func (d *_KBIngestJob) CountByStatuses(statuses []KBIngestJobStatus) (int64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var count int64
	err := getDB().Model(&KBIngestJob{}).Where("status IN ?", statuses).Count(&count).Error
	return count, err
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
