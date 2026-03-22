package payment

import (
	"context"
	paymentpkg "interview-agents/internal/payment"
)

// Manager 支付服务接口
type Manager interface {
	// CreateCheckout 创建一次性支付
	CreateCheckout(ctx context.Context, userID uint, req *CreateCheckoutRequest) (*CreateCheckoutResponse, error)

	// CreateSubscriptionCheckout 创建订阅支付
	CreateSubscriptionCheckout(ctx context.Context, userID uint, req *CreateSubscriptionRequest) (*CreateSubscriptionResponse, error)

	// HandleWebhook 处理 webhook 回调
	HandleWebhook(ctx context.Context, provider string, payload *paymentpkg.WebhookPayload) error

	// GetOrderByNo 查询订单
	GetOrderByNo(ctx context.Context, userID uint, orderNo string) (*OrderDetail, error)

	// ListOrders 查询用户订单列表
	ListOrders(ctx context.Context, userID uint, page, pageSize int) (*OrderListResponse, error)

	// CancelSubscription 取消订阅
	CancelSubscription(ctx context.Context, userID uint, subscriptionNo string, immediate bool) error

	// GetActiveSubscription 查询用户当前生效订阅
	GetActiveSubscription(ctx context.Context, userID uint) (*SubscriptionDetail, error)
}

// NewManager 创建支付服务管理器
func NewManager() Manager {
	return newPaymentService()
}
