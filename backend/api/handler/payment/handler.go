package payment

import (
	"context"
	"io"
	"log"

	paymentDTO "interview-agents/api/model/payment"
	"interview-agents/api/response"
	"interview-agents/internal/middleware"
	paymentpkg "interview-agents/internal/payment"
	paymentService "interview-agents/internal/service/payment"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/google/uuid"
)

// CreateCheckout 创建一次性支付
// @router /api/payment/checkout/create [POST]
func CreateCheckout(ctx context.Context, c *app.RequestContext) {
	var req paymentDTO.CreateCheckoutRequest
	if err := c.BindAndValidate(&req); err != nil {
		log.Printf("[Payment] CreateCheckout bad request: %v", err)
		response.BadRequest(ctx, c, "Invalid request: "+err.Error())
		return
	}

	userID := middleware.GetUserID(c)
	if userID == 0 {
		log.Printf("[Payment] CreateCheckout unauthorized access attempt")
		response.Unauthorized(ctx, c, "Unauthorized")
		return
	}

	log.Printf("[Payment] CreateCheckout start: user_id=%d provider=%s product=%s idempotency_key=%s", userID, req.Provider, req.ProductCode)

	svc := paymentService.NewManager()
	result, err := svc.CreateCheckout(ctx, userID, &paymentService.CreateCheckoutRequest{
		ProductCode: req.ProductCode,
		PriceCode:   req.PriceCode,
		Provider:    req.Provider,
		Amount:      req.Amount,
		Currency:    req.Currency,
		ProductName: req.ProductName,
		//SuccessURL:     req.SuccessURL,
		//CancelURL:      req.CancelURL,
		IdempotencyKey: uuid.New().String(),
	})
	if err != nil {
		log.Printf("[Payment] CreateCheckout failed: user_id=%d err=%v", userID, err)
		response.InternalServerError(ctx, c, err.Error())
		return
	}

	log.Printf("[Payment] CreateCheckout success: user_id=%d order_no=%s", userID, result.OrderNo)
	response.Success(ctx, c, result)
}

// CreateSubscription 创建订阅支付
// @router /api/payment/subscription/create [POST]
func CreateSubscription(ctx context.Context, c *app.RequestContext) {
	var req paymentDTO.CreateSubscriptionRequest
	if err := c.BindAndValidate(&req); err != nil {
		log.Printf("[Payment] CreateSubscription bad request: %v", err)
		response.BadRequest(ctx, c, "Invalid request: "+err.Error())
		return
	}

	userID := middleware.GetUserID(c)
	if userID == 0 {
		log.Printf("[Payment] CreateSubscription unauthorized access attempt")
		response.Unauthorized(ctx, c, "Unauthorized")
		return
	}

	log.Printf("[Payment] CreateSubscription start: user_id=%d provider=%s product=%s idempotency_key=%s", userID, req.Provider, req.ProductCode, req.IdempotencyKey)

	svc := paymentService.NewManager()
	result, err := svc.CreateSubscriptionCheckout(ctx, userID, &paymentService.CreateSubscriptionRequest{
		ProductCode:    req.ProductCode,
		PriceCode:      req.PriceCode,
		Provider:       req.Provider,
		PriceID:        req.PriceID,
		SuccessURL:     req.SuccessURL,
		CancelURL:      req.CancelURL,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		log.Printf("[Payment] CreateSubscription failed: user_id=%d err=%v", userID, err)
		response.InternalServerError(ctx, c, err.Error())
		return
	}

	log.Printf("[Payment] CreateSubscription success: user_id=%d order_no=%s sub_no=%s", userID, result.OrderNo, result.SubscriptionNo)
	response.Success(ctx, c, result)
}

// WebhookHandler 处理支付渠道 webhook 回调
// @router /api/payment/webhook/:provider [POST]
func WebhookHandler(ctx context.Context, c *app.RequestContext) {
	provider := c.Param("provider")
	if provider == "" {
		log.Printf("[Payment] Webhook missing provider param")
		c.String(400, "missing provider")
		return
	}

	body, err := io.ReadAll(c.GetRequest().BodyStream())
	if err != nil {
		// 如果 BodyStream 为空，尝试直接获取 body
		body = c.GetRequest().Body()
	}

	log.Printf("[Payment] Webhook received: provider=%s body_size=%d", provider, len(body))

	headers := make(map[string]string)
	c.Request.Header.VisitAll(func(key, value []byte) {
		headers[string(key)] = string(value)
	})

	svc := paymentService.NewManager()
	if err := svc.HandleWebhook(ctx, provider, &paymentpkg.WebhookPayload{
		Headers: headers,
		Body:    body,
	}); err != nil {
		log.Printf("[Payment] Webhook processing failed: provider=%s err=%v", provider, err)
		c.String(400, "webhook processing failed")
		return
	}

	log.Printf("[Payment] Webhook processed successfully: provider=%s", provider)
	// Webhook 必须返回 200，否则渠道会重试
	c.String(200, "ok")
}
