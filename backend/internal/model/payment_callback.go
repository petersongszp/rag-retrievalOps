package model

import (
	"time"

	"gorm.io/gorm"
)

// CallbackStatus 回调处理状态
type CallbackStatus string

const (
	CallbackStatusReceived  CallbackStatus = "RECEIVED"
	CallbackStatusVerified  CallbackStatus = "VERIFIED"
	CallbackStatusProcessed CallbackStatus = "PROCESSED"
	CallbackStatusFailed    CallbackStatus = "FAILED"
)

var PaymentCallbackDao _PaymentCallback

type _PaymentCallback struct{}

// PaymentCallback 支付回调记录（记录每一次渠道回调的原始数据）
type PaymentCallback struct {
	ID            uint64         `json:"id" gorm:"primaryKey;autoIncrement"`
	Provider      string         `json:"provider" gorm:"size:20;not null;index;comment:支付渠道 stripe/paypal"`
	RawHeaders    string         `json:"raw_headers" gorm:"type:text;comment:原始请求头JSON"`
	RawBody       string         `json:"raw_body" gorm:"type:mediumtext;comment:原始请求体"`
	EventID       string         `json:"event_id" gorm:"size:128;index;comment:解析后的渠道事件ID"`
	EventType     string         `json:"event_type" gorm:"size:64;comment:解析后的事件类型"`
	Status        CallbackStatus `json:"status" gorm:"size:20;not null;default:'RECEIVED';index;comment:处理状态"`
	ErrorMessage  string         `json:"error_message" gorm:"type:text;comment:错误信息"`
	ProcessTimeMs int64          `json:"process_time_ms" gorm:"comment:处理耗时(毫秒)"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

func (PaymentCallback) TableName() string {
	return "payment_callback"
}

// Create 创建回调记录
func (d *_PaymentCallback) Create(tx *gorm.DB, callback *PaymentCallback) error {
	return tx.Create(callback).Error
}

// UpdateStatus 更新回调处理状态
func (d *_PaymentCallback) UpdateStatus(tx *gorm.DB, id uint64, status CallbackStatus, errMsg string, processTimeMs int64) error {
	updates := map[string]interface{}{
		"status":          status,
		"error_message":   errMsg,
		"process_time_ms": processTimeMs,
	}
	return tx.Model(&PaymentCallback{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateEventInfo 更新解析后的事件信息
func (d *_PaymentCallback) UpdateEventInfo(tx *gorm.DB, id uint64, eventID, eventType string) error {
	return tx.Model(&PaymentCallback{}).Where("id = ?", id).Updates(map[string]interface{}{
		"event_id":   eventID,
		"event_type": eventType,
	}).Error
}

// FindByProvider 按渠道查询回调记录
func (d *_PaymentCallback) FindByProvider(provider string, page, pageSize int) ([]*PaymentCallback, int64, error) {
	var callbacks []*PaymentCallback
	var total int64
	query := getDB().Model(&PaymentCallback{}).Where("provider = ?", provider)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Offset((page - 1) * pageSize).Limit(pageSize).Order("created_at DESC").Find(&callbacks).Error
	return callbacks, total, err
}

// FindByStatus 按状态查询回调记录
func (d *_PaymentCallback) FindByStatus(status CallbackStatus, limit int) ([]*PaymentCallback, error) {
	var callbacks []*PaymentCallback
	err := getDB().Where("status = ?", status).
		Order("created_at ASC").Limit(limit).Find(&callbacks).Error
	return callbacks, err
}

