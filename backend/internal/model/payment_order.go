package model

import (
	"time"

	"gorm.io/gorm"
)

// OrderStatus 订单状态
type OrderStatus string

const (
	OrderStatusCreated           OrderStatus = "CREATED"
	OrderStatusPendingPayment    OrderStatus = "PENDING_PAYMENT"
	OrderStatusProcessing        OrderStatus = "PROCESSING"
	OrderStatusPaid              OrderStatus = "PAID"
	OrderStatusFulfilled         OrderStatus = "FULFILLED"
	OrderStatusFailed            OrderStatus = "FAILED"
	OrderStatusCanceled          OrderStatus = "CANCELED"
	OrderStatusExpired           OrderStatus = "EXPIRED"
	OrderStatusRefunding         OrderStatus = "REFUNDING"
	OrderStatusRefunded          OrderStatus = "REFUNDED"
	OrderStatusPartiallyRefunded OrderStatus = "PARTIALLY_REFUNDED"
)

// OrderType 订单类型
type OrderType string

const (
	OrderTypeOneTime      OrderType = "one_time"
	OrderTypeSubscription OrderType = "subscription"
)

var PaymentOrderDao _PaymentOrder

type _PaymentOrder struct{}

// PaymentOrder 支付订单
type PaymentOrder struct {
	ID             uint64         `json:"id" gorm:"primaryKey;autoIncrement"`
	OrderNo        string         `json:"order_no" gorm:"uniqueIndex;size:64;not null;comment:业务订单号"`
	UserID         uint           `json:"user_id" gorm:"index;not null;comment:用户ID"`
	OrderType      OrderType      `json:"order_type" gorm:"size:20;not null;comment:one_time/subscription"`
	ProductCode    string         `json:"product_code" gorm:"size:64;not null;comment:商品编码"`
	PriceCode      string         `json:"price_code" gorm:"size:64;not null;comment:价格编码"`
	Amount         int64          `json:"amount" gorm:"not null;comment:金额(最小货币单位)"`
	Currency       string         `json:"currency" gorm:"size:10;not null;default:'usd';comment:币种"`
	Status         OrderStatus    `json:"status" gorm:"size:30;not null;default:'CREATED';index;comment:订单状态"`
	Provider       string         `json:"provider" gorm:"size:20;not null;comment:支付渠道 stripe/paypal"`
	IdempotencyKey string         `json:"idempotency_key" gorm:"uniqueIndex;size:128;not null;comment:幂等键"`
	SuccessURL     string         `json:"success_url" gorm:"size:512;comment:支付成功跳转"`
	CancelURL      string         `json:"cancel_url" gorm:"size:512;comment:支付取消跳转"`
	Metadata       string         `json:"metadata" gorm:"type:text;comment:扩展信息JSON"`
	PaidAt         *time.Time     `json:"paid_at" gorm:"comment:支付成功时间"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

func (PaymentOrder) TableName() string {
	return "payment_order"
}

// Create 创建订单
func (d *_PaymentOrder) Create(tx *gorm.DB, order *PaymentOrder) error {
	return tx.Create(order).Error
}

// FindByOrderNo 根据订单号查询
func (d *_PaymentOrder) FindByOrderNo(orderNo string) (*PaymentOrder, error) {
	var order PaymentOrder
	err := getDB().Where("order_no = ?", orderNo).First(&order).Error
	return &order, err
}

// FindByIdempotencyKey 根据幂等键查询
func (d *_PaymentOrder) FindByIdempotencyKey(key string) (*PaymentOrder, error) {
	var order PaymentOrder
	err := getDB().Where("idempotency_key = ?", key).First(&order).Error
	return &order, err
}

// UpdateStatus 更新订单状态
func (d *_PaymentOrder) UpdateStatus(tx *gorm.DB, orderNo string, status OrderStatus, updates map[string]interface{}) error {
	if updates == nil {
		updates = make(map[string]interface{})
	}
	updates["status"] = status
	return tx.Model(&PaymentOrder{}).Where("order_no = ?", orderNo).Updates(updates).Error
}

// FindByUserID 查询用户订单列表
func (d *_PaymentOrder) FindByUserID(userID uint, page, pageSize int) ([]*PaymentOrder, int64, error) {
	var orders []*PaymentOrder
	var total int64
	query := getDB().Model(&PaymentOrder{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Offset((page - 1) * pageSize).Limit(pageSize).Order("created_at DESC").Find(&orders).Error
	return orders, total, err
}
