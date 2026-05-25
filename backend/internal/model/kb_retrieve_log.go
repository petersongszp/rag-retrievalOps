package model

import (
	"time"
)

type RetrieveResultStatus string

const (
	RetrieveResultStatusSuccess RetrieveResultStatus = "success"
	RetrieveResultStatusNoResult RetrieveResultStatus = "no_result"
	RetrieveResultStatusFilteredOut RetrieveResultStatus = "filtered_out"
	RetrieveResultStatusError RetrieveResultStatus = "error"
	RetrieveResultStatusTimeout RetrieveResultStatus = "timeout"
)

var KBRetrieveLogDao _KBRetrieveLog

type (
	_KBRetrieveLog struct{}
	KBRetrieveLog  struct {
		ID              uint64               `json:"id" gorm:"primaryKey;autoIncrement"`
		RequestID       string               `json:"request_id" gorm:"uniqueIndex;size:64;not null"`
		UserID          uint                 `json:"user_id" gorm:"index;not null"`
		KBIDs           string               `json:"kb_ids" gorm:"size:500"`
		Query           string               `json:"query" gorm:"size:2000;not null"`
		Expr            string               `json:"expr" gorm:"size:2000"`
		TopK            int                  `json:"top_k"`
		Rewrite         string               `json:"rewrite" gorm:"size:1000"`
		Routes          string               `json:"routes" gorm:"size:200"`
		Collection      string               `json:"collection" gorm:"size:200"`
		RetrieverVersion string              `json:"retriever_version" gorm:"size:50"`
		FinalCount      int                  `json:"final_count"`
		TruncatedCount  int                  `json:"truncated_count"`
		ResultStatus    RetrieveResultStatus `json:"result_status" gorm:"size:20;not null;default:'success';index"`
		ErrorCode       string               `json:"error_code" gorm:"size:64"`
		ErrorMsg        string               `json:"error_msg" gorm:"size:1000"`
		EmbeddingMs     int64                `json:"embedding_ms"`
		SearchMs        int64                `json:"search_ms"`
		PostprocessMs   int64                `json:"postprocess_ms"`
		DurationMs      int64                `json:"duration_ms"`
		TimeoutMs       int64                `json:"timeout_ms"`
		CreatedAt       time.Time            `json:"created_at" gorm:"autoCreateTime:milli;index"`
	}
)

func (KBRetrieveLog) TableName() string {
	return "kb_retrieve_log"
}

func (d *_KBRetrieveLog) Create(log *KBRetrieveLog) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().Create(log).Error
}

func (d *_KBRetrieveLog) GetByRequestID(requestID string) (*KBRetrieveLog, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var logEntry KBRetrieveLog
	err := getDB().Where("request_id = ?", requestID).First(&logEntry).Error
	if err != nil {
		return nil, err
	}
	return &logEntry, nil
}

func (d *_KBRetrieveLog) ListByUserID(userID uint, page, pageSize int) ([]*KBRetrieveLog, int64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	var list []*KBRetrieveLog
	var total int64

	query := getDB().Model(&KBRetrieveLog{}).Where("user_id = ?", userID)
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

func (d *_KBRetrieveLog) ListByStatus(resultStatus RetrieveResultStatus, page, pageSize int) ([]*KBRetrieveLog, int64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	var list []*KBRetrieveLog
	var total int64

	query := getDB().Model(&KBRetrieveLog{}).Where("result_status = ?", resultStatus)
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

func ParseRetrieveResultStatus(raw string) (RetrieveResultStatus, bool) {
	status := RetrieveResultStatus(raw)
	switch status {
	case RetrieveResultStatusSuccess,
		RetrieveResultStatusNoResult,
		RetrieveResultStatusFilteredOut,
		RetrieveResultStatusError,
		RetrieveResultStatusTimeout:
		return status, true
	default:
		return "", false
	}
}
