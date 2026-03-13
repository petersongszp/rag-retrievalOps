package model

import (
	"time"

	"gorm.io/gorm"
)

// AttemptStatus 支付尝试状态
type AttemptStatus string

const (
	AttemptStatusInitiated      AttemptStatus = "INITIATED"
	AttemptStatusRequiresAction AttemptStatus = "REQUIRES_ACTION"
	AttemptStatusProcessing     AttemptStatus = "PROCESSING"
	AttemptStatusSucceeded      AttemptStatus = "SUCCEEDED"
	AttemptStatusFailed         AttemptStatus = "FAILED"
	AttemptStatusCanceled       AttemptStatus = "CANCELED"
	AttemptStatusExpired        AttemptStatus = "EXPIRED"
)

var PaymentAttemptDao _PaymentAttempt

type _PaymentAttempt struct{}

// PaymentAttempt 支付尝试
type PaymentAttempt struct {
	ID            uint64        `json:"id" gorm:"primaryKey;autoIncrement"`
	AttemptNo     string        `json:"attempt_no" gorm:"uniqueIndex;size:64;not null;comment:支付尝试编号"`
	OrderNo       string        `json:"order_no" gorm:"index;size:64;not null;comment:关联订单号"`
	Provider      string        `json:"provider" gorm:"size:20;not null;comment:支付渠道"`
	ProviderID    string        `json:"provider_id" gorm:"size:128;comment:渠道侧ID(checkout session/order id)"`
	Status        AttemptStatus `json:"status" gorm:"size:30;not null;default:'INITIATED';comment:状态"`
	CheckoutURL   string        `json:"checkout_url" gorm:"size:1024;comment:支付链接"`
	Amount        int64         `json:"amount" gorm:"not null;comment:金额(最小货币单位)"`
	Currency      string        `json:"currency" gorm:"size:10;not null;default:'usd'"`
	FailureReason string        `json:"failure_reason" gorm:"size:512;comment:失败原因"`
	ProviderData  string        `json:"provider_data" gorm:"type:text;comment:渠道原始响应(脱敏)"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

func (PaymentAttempt) TableName() string {
	return "payment_attempt"
}

// Create 创建支付尝试
func (d *_PaymentAttempt) Create(tx *gorm.DB, attempt *PaymentAttempt) error {
	return tx.Create(attempt).Error
}

// FindByAttemptNo 根据尝试编号查询
func (d *_PaymentAttempt) FindByAttemptNo(attemptNo string) (*PaymentAttempt, error) {
	var attempt PaymentAttempt
	err := getDB().Where("attempt_no = ?", attemptNo).First(&attempt).Error
	return &attempt, err
}

// FindByProviderID 根据渠道ID查询
func (d *_PaymentAttempt) FindByProviderID(provider, providerID string) (*PaymentAttempt, error) {
	var attempt PaymentAttempt
	err := getDB().Where("provider = ? AND provider_id = ?", provider, providerID).First(&attempt).Error
	return &attempt, err
}

// UpdateStatus 更新支付尝试状态
func (d *_PaymentAttempt) UpdateStatus(tx *gorm.DB, attemptNo string, status AttemptStatus, updates map[string]interface{}) error {
	if updates == nil {
		updates = make(map[string]interface{})
	}
	updates["status"] = status
	return tx.Model(&PaymentAttempt{}).Where("attempt_no = ?", attemptNo).Updates(updates).Error
}

// FindByOrderNo 查询订单的所有支付尝试
func (d *_PaymentAttempt) FindByOrderNo(orderNo string) ([]*PaymentAttempt, error) {
	var attempts []*PaymentAttempt
	err := getDB().Where("order_no = ?", orderNo).Order("created_at DESC").Find(&attempts).Error
	return attempts, err
}

