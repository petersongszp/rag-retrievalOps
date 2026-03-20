package payment

import (
	handler "interview-agents/api/handler/payment"

	"github.com/cloudwego/hertz/pkg/app/server"
)

// Register 注册支付相关路由
// 所有接口统一使用 POST 方法
func Register(r *server.Hertz) {
	paymentGroup := r.Group("/api/payment")
	{
		// 一次性支付
		checkout := paymentGroup.Group("/checkout")
		{
			checkout.POST("/create", handler.CreateCheckout)
		}

		// 订阅
		subscription := paymentGroup.Group("/subscription")
		{
			subscription.POST("/create", handler.CreateSubscription)
			subscription.POST("/cancel", handler.CancelSubscription)
			subscription.POST("/active", handler.GetActiveSubscription)
		}

		// 订单
		order := paymentGroup.Group("/order")
		{
			order.POST("/query", handler.GetOrder)
			order.POST("/list", handler.ListOrders)
		}

		// Webhook（公开接口，不需要 JWT）
		paymentGroup.POST("/webhook/:provider", handler.WebhookHandler)
	}
}

