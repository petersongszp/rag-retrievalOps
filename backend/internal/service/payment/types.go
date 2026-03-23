package payment

import (
	"interview-agents/internal/model"
	"time"
)

// CreateCheckoutRequest 创建一次性支付请求
type CreateCheckoutRequest struct {
	ProductCode    string `json:"product_code"`
	PriceCode      string `json:"price_code"`
	Provider       string `json:"provider"` // stripe / paypal
	Amount         int64  `json:"amount"`   // 最小货币单位
	Currency       string `json:"currency"`
	ProductName    string `json:"product_name"`
	SuccessURL     string `json:"success_url"`
	CancelURL      string `json:"cancel_url"`
	IdempotencyKey string `json:"idempotency_key"`
}

// CreateCheckoutResponse 创建支付响应
type CreateCheckoutResponse struct {
	OrderNo     string `json:"order_no"`
	CheckoutURL string `json:"checkout_url"`
	ProviderID  string `json:"provider_id"`
}

// CreateSubscriptionRequest 创建订阅请求
type CreateSubscriptionRequest struct {
	ProductCode    string `json:"product_code"`
	PriceCode      string `json:"price_code"`
	Provider       string `json:"provider"`
	PriceID        string `json:"price_id"` // 渠道侧价格 ID
	SuccessURL     string `json:"success_url"`
	CancelURL      string `json:"cancel_url"`
	IdempotencyKey string `json:"idempotency_key"`
}

// CreateSubscriptionResponse 创建订阅响应
type CreateSubscriptionResponse struct {
	OrderNo        string `json:"order_no"`
	SubscriptionNo string `json:"subscription_no"`
	CheckoutURL    string `json:"checkout_url"`
}

// OrderDetail 订单详情
type OrderDetail struct {
	OrderNo     string            `json:"order_no"`
	OrderType   model.OrderType   `json:"order_type"`
	ProductCode string            `json:"product_code"`
	PriceCode   string            `json:"price_code"`
	Amount      int64             `json:"amount"`
	Currency    string            `json:"currency"`
	Status      model.OrderStatus `json:"status"`
	Provider    string            `json:"provider"`
	PaidAt      *time.Time        `json:"paid_at"`
	CreatedAt   time.Time         `json:"created_at"`
}

// OrderListResponse 订单列表响应
type OrderListResponse struct {
	Total  int64          `json:"total"`
	Orders []*OrderDetail `json:"orders"`
}

// SubscriptionDetail 订阅详情
type SubscriptionDetail struct {
	SubscriptionNo     string                   `json:"subscription_no"`
	ProductCode        string                   `json:"product_code"`
	PriceCode          string                   `json:"price_code"`
	Status             model.SubscriptionStatus `json:"status"`
	Provider           string                   `json:"provider"`
	CurrentPeriodStart *time.Time               `json:"current_period_start"`
	CurrentPeriodEnd   *time.Time               `json:"current_period_end"`
	CancelAt           *time.Time               `json:"cancel_at"`
	CreatedAt          time.Time                `json:"created_at"`
}
