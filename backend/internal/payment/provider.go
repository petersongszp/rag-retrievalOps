package payment

import "context"

// ProviderName 支付渠道名称
type ProviderName string

const (
	ProviderStripe ProviderName = "stripe"
	ProviderPayPal ProviderName = "paypal"
)

// CheckoutRequest 创建支付请求
type CheckoutRequest struct {
	OrderNo       string
	Amount        int64  // 最小货币单位
	Currency      string // e.g. "usd"
	ProductName   string
	SuccessURL    string
	CancelURL     string
	CustomerEmail string
	Metadata      map[string]string
}

// CheckoutResult 创建支付结果
type CheckoutResult struct {
	ProviderID  string // 渠道侧 session/order ID
	CheckoutURL string // 用户跳转支付链接
}

// SubscriptionRequest 创建订阅请求
type SubscriptionRequest struct {
	OrderNo       string
	PriceID       string // 渠道侧价格 ID
	SuccessURL    string
	CancelURL     string
	CustomerEmail string
	Metadata      map[string]string
}

// SubscriptionResult 创建订阅结果
type SubscriptionResult struct {
	ProviderSubID string // 渠道侧订阅 ID
	ProviderID    string // checkout session ID
	CheckoutURL   string
}

// RefundRequest 退款请求
type RefundRequest struct {
	ProviderPaymentID string // 渠道侧支付 ID
	Amount            int64  // 退款金额，0 表示全额
	Reason            string
}

// RefundResult 退款结果
type RefundResult struct {
	ProviderRefundID string
	Status           string
}

// WebhookPayload webhook 原始数据
type WebhookPayload struct {
	Headers map[string]string
	Body    []byte
}

// WebhookEvent 标准化后的 webhook 事件
type WebhookEvent struct {
	EventID   string            // 渠道事件 ID
	EventType string            // 标准化事件类型
	RawType   string            // 渠道原始事件类型
	Data      map[string]string // 关键字段
	RawJSON   string            // 原始 JSON
}

// 标准化事件类型
const (
	EventCheckoutCompleted    = "checkout.completed"
	EventPaymentSucceeded     = "payment.succeeded"
	EventPaymentFailed        = "payment.failed"
	EventSubscriptionCreated  = "subscription.created"
	EventSubscriptionUpdated  = "subscription.updated"
	EventSubscriptionCanceled = "subscription.canceled"
	EventSubscriptionRenewed  = "subscription.renewed"
	EventRefundCompleted      = "refund.completed"
)

// Provider 支付渠道统一接口
type Provider interface {
	// Name 渠道名称
	Name() ProviderName

	// CreateCheckout 创建一次性支付
	CreateCheckout(ctx context.Context, req *CheckoutRequest) (*CheckoutResult, error)

	// CreateSubscription 创建订阅
	CreateSubscription(ctx context.Context, req *SubscriptionRequest) (*SubscriptionResult, error)

	// CancelSubscription 取消订阅
	CancelSubscription(ctx context.Context, providerSubID string, immediate bool) error

	// Refund 退款
	Refund(ctx context.Context, req *RefundRequest) (*RefundResult, error)

	// VerifyWebhook 验证 webhook 签名并解析事件
	VerifyWebhook(ctx context.Context, payload *WebhookPayload) (*WebhookEvent, error)
}
