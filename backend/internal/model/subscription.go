package model

import (
	"time"

	"gorm.io/gorm"
)

// SubscriptionStatus 订阅状态
type SubscriptionStatus string

const (
	SubscriptionStatusInit            SubscriptionStatus = "INIT"
	SubscriptionStatusActive          SubscriptionStatus = "ACTIVE"
	SubscriptionStatusPastDue         SubscriptionStatus = "PAST_DUE"
	SubscriptionStatusCancelScheduled SubscriptionStatus = "CANCEL_SCHEDULED"
	SubscriptionStatusCanceled        SubscriptionStatus = "CANCELED"
	SubscriptionStatusExpired         SubscriptionStatus = "EXPIRED"
)

var SubscriptionDao _Subscription

type _Subscription struct{}

// Subscription 订阅记录
type Subscription struct {
	ID                 uint64             `json:"id" gorm:"primaryKey;autoIncrement"`
	SubscriptionNo     string             `json:"subscription_no" gorm:"uniqueIndex;size:64;not null;comment:订阅编号"`
	UserID             uint               `json:"user_id" gorm:"index;not null;comment:用户ID"`
	OrderNo            string             `json:"order_no" gorm:"index;size:64;not null;comment:关联首次订单号"`
	Provider           string             `json:"provider" gorm:"size:20;not null;comment:支付渠道"`
	ProviderSubID      string             `json:"provider_sub_id" gorm:"size:128;index;comment:渠道订阅ID"`
	ProductCode        string             `json:"product_code" gorm:"size:64;not null;comment:商品编码"`
	PriceCode          string             `json:"price_code" gorm:"size:64;not null;comment:价格编码"`
	Status             SubscriptionStatus `json:"status" gorm:"size:30;not null;default:'INIT';index;comment:订阅状态"`
	CurrentPeriodStart *time.Time         `json:"current_period_start" gorm:"comment:当前周期开始"`
	CurrentPeriodEnd   *time.Time         `json:"current_period_end" gorm:"comment:当前周期结束"`
	CancelAt           *time.Time         `json:"cancel_at" gorm:"comment:计划取消时间"`
	CanceledAt         *time.Time         `json:"canceled_at" gorm:"comment:实际取消时间"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
}

func (Subscription) TableName() string {
	return "subscription"
}

// Create 创建订阅
func (d *_Subscription) Create(tx *gorm.DB, sub *Subscription) error {
	return tx.Create(sub).Error
}

// FindBySubscriptionNo 根据订阅编号查询
func (d *_Subscription) FindBySubscriptionNo(subNo string) (*Subscription, error) {
	var sub Subscription
	err := getDB().Where("subscription_no = ?", subNo).First(&sub).Error
	return &sub, err
}

// FindByProviderSubID 根据渠道订阅ID查询
func (d *_Subscription) FindByProviderSubID(provider, providerSubID string) (*Subscription, error) {
	var sub Subscription
	err := getDB().Where("provider = ? AND provider_sub_id = ?", provider, providerSubID).First(&sub).Error
	return &sub, err
}

// UpdateStatus 更新订阅状态
func (d *_Subscription) UpdateStatus(tx *gorm.DB, subNo string, status SubscriptionStatus, updates map[string]interface{}) error {
	if updates == nil {
		updates = make(map[string]interface{})
	}
	updates["status"] = status
	return tx.Model(&Subscription{}).Where("subscription_no = ?", subNo).Updates(updates).Error
}

// FindActiveByUserID 查询用户生效中的订阅
func (d *_Subscription) FindActiveByUserID(userID uint) (*Subscription, error) {
	var sub Subscription
	err := getDB().Where("user_id = ? AND status IN ?", userID,
		[]SubscriptionStatus{SubscriptionStatusActive, SubscriptionStatusCancelScheduled}).
		Order("created_at DESC").First(&sub).Error
	return &sub, err
}

// FindByUserID 查询用户所有订阅
func (d *_Subscription) FindByUserID(userID uint) ([]*Subscription, error) {
	var subs []*Subscription
	err := getDB().Where("user_id = ?", userID).Order("created_at DESC").Find(&subs).Error
	return subs, err
}

