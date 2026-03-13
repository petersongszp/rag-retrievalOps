package model

import (
	"time"

	"gorm.io/gorm"
)

// EventProcessStatus webhook 事件处理状态
type EventProcessStatus string

const (
	EventStatusReceived  EventProcessStatus = "RECEIVED"
	EventStatusVerified  EventProcessStatus = "VERIFIED"
	EventStatusProcessed EventProcessStatus = "PROCESSED"
	EventStatusFailed    EventProcessStatus = "FAILED"
)

var PaymentEventDao _PaymentEvent

type _PaymentEvent struct{}

// PaymentEvent webhook 事件记录
type PaymentEvent struct {
	ID            uint64             `json:"id" gorm:"primaryKey;autoIncrement"`
	Source        string             `json:"source" gorm:"size:20;not null;uniqueIndex:idx_source_event;comment:来源 stripe/paypal"`
	SourceEventID string             `json:"source_event_id" gorm:"size:128;not null;uniqueIndex:idx_source_event;comment:渠道事件ID"`
	EventType     string             `json:"event_type" gorm:"size:64;not null;comment:事件类型"`
	Status        EventProcessStatus `json:"status" gorm:"size:20;not null;default:'RECEIVED';comment:处理状态"`
	RawPayload    string             `json:"raw_payload" gorm:"type:mediumtext;comment:原始请求体"`
	ProcessResult string             `json:"process_result" gorm:"type:text;comment:处理结果/错误信息"`
	RetryCount    int                `json:"retry_count" gorm:"not null;default:0;comment:重试次数"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
}

func (PaymentEvent) TableName() string {
	return "payment_event"
}

// Create 创建事件记录
func (d *_PaymentEvent) Create(tx *gorm.DB, event *PaymentEvent) error {
	return tx.Create(event).Error
}

// ExistsBySourceEventID 检查事件是否已存在（去重）
func (d *_PaymentEvent) ExistsBySourceEventID(source, sourceEventID string) (bool, error) {
	var count int64
	err := getDB().Model(&PaymentEvent{}).
		Where("source = ? AND source_event_id = ?", source, sourceEventID).
		Count(&count).Error
	return count > 0, err
}

// UpdateStatus 更新事件处理状态
func (d *_PaymentEvent) UpdateStatus(tx *gorm.DB, id uint64, status EventProcessStatus, result string) error {
	return tx.Model(&PaymentEvent{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":         status,
			"process_result": result,
		}).Error
}

// FindUnprocessed 查询未处理/失败的事件（用于补偿重试）
func (d *_PaymentEvent) FindUnprocessed(limit int) ([]*PaymentEvent, error) {
	var events []*PaymentEvent
	err := getDB().Where("status IN ? AND retry_count < ?",
		[]EventProcessStatus{EventStatusReceived, EventStatusVerified, EventStatusFailed}, 5).
		Order("created_at ASC").Limit(limit).Find(&events).Error
	return events, err
}

// IncrRetryCount 增加重试次数
func (d *_PaymentEvent) IncrRetryCount(tx *gorm.DB, id uint64) error {
	return tx.Model(&PaymentEvent{}).Where("id = ?", id).
		UpdateColumn("retry_count", gorm.Expr("retry_count + 1")).Error
}

