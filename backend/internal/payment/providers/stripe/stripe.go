package stripe

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"interview-agents/internal/config"
	"interview-agents/internal/payment"

	stripe "github.com/stripe/stripe-go/v82"
	checkoutsession "github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/refund"
	"github.com/stripe/stripe-go/v82/subscription"
	"github.com/stripe/stripe-go/v82/webhook"
)

// Adapter Stripe 支付适配器
type Adapter struct {
	secretKey     string
	webhookSecret string
}

// NewAdapter 创建 Stripe 适配器
func NewAdapter(cfg config.StripeConfig) *Adapter {
	stripe.Key = cfg.SecretKey
	return &Adapter{
		secretKey:     cfg.SecretKey,
		webhookSecret: cfg.WebhookSecret,
	}
}

func (a *Adapter) Name() payment.ProviderName {
	return payment.ProviderStripe
}

// CreateCheckout 创建 Stripe Checkout Session（一次性支付）
func (a *Adapter) CreateCheckout(ctx context.Context, req *payment.CheckoutRequest) (*payment.CheckoutResult, error) {
	params := &stripe.CheckoutSessionParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{{
			PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
				Currency:   stripe.String(req.Currency),
				UnitAmount: stripe.Int64(req.Amount),
				ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
					Name: stripe.String(req.ProductName),
				},
			},
			Quantity: stripe.Int64(1),
		}},
		SuccessURL: stripe.String(req.SuccessURL),
		CancelURL:  stripe.String(req.CancelURL),
	}

	// 设置 metadata（用于 webhook 回调时关联订单）
	if req.Metadata != nil {
		params.Metadata = req.Metadata
	}
	if req.CustomerEmail != "" {
		params.CustomerEmail = stripe.String(req.CustomerEmail)
	}

	log.Printf("[Stripe] CreateCheckout: order_no=%s amount=%d %s", req.OrderNo, req.Amount, req.Currency)

	session, err := checkoutsession.New(params)
	if err != nil {
		log.Printf("[Stripe] CreateCheckout failed: order_no=%s err=%v", req.OrderNo, err)
		return nil, fmt.Errorf("stripe create checkout session failed: %w", err)
	}

	log.Printf("[Stripe] CreateCheckout success: order_no=%s session_id=%s", req.OrderNo, session.ID)
	return &payment.CheckoutResult{
		ProviderID:  session.ID,
		CheckoutURL: session.URL,
	}, nil
}

// CreateSubscription 创建 Stripe 订阅（通过 Checkout Session subscription 模式）
func (a *Adapter) CreateSubscription(ctx context.Context, req *payment.SubscriptionRequest) (*payment.SubscriptionResult, error) {
	params := &stripe.CheckoutSessionParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{{
			Price:    stripe.String(req.PriceID),
			Quantity: stripe.Int64(1),
		}},
		SuccessURL: stripe.String(req.SuccessURL),
		CancelURL:  stripe.String(req.CancelURL),
	}

	if req.Metadata != nil {
		params.Metadata = req.Metadata
	}
	if req.CustomerEmail != "" {
		params.CustomerEmail = stripe.String(req.CustomerEmail)
	}

	log.Printf("[Stripe] CreateSubscription: order_no=%s price_id=%s", req.OrderNo, req.PriceID)

	session, err := checkoutsession.New(params)
	if err != nil {
		log.Printf("[Stripe] CreateSubscription failed: order_no=%s err=%v", req.OrderNo, err)
		return nil, fmt.Errorf("stripe create subscription checkout failed: %w", err)
	}

	// subscription ID 在 checkout 完成后通过 webhook 获取
	subID := ""
	if session.Subscription != nil {
		subID = session.Subscription.ID
	}

	log.Printf("[Stripe] CreateSubscription success: order_no=%s session_id=%s sub_id=%s", req.OrderNo, session.ID, subID)
	return &payment.SubscriptionResult{
		ProviderSubID: subID,
		ProviderID:    session.ID,
		CheckoutURL:   session.URL,
	}, nil
}

// CancelSubscription 取消 Stripe 订阅
func (a *Adapter) CancelSubscription(ctx context.Context, providerSubID string, immediate bool) error {
	log.Printf("[Stripe] CancelSubscription: sub_id=%s immediate=%v", providerSubID, immediate)

	if immediate {
		_, err := subscription.Cancel(providerSubID, nil)
		if err != nil {
			log.Printf("[Stripe] CancelSubscription failed: sub_id=%s err=%v", providerSubID, err)
			return fmt.Errorf("stripe cancel subscription failed: %w", err)
		}
	} else {
		_, err := subscription.Update(providerSubID, &stripe.SubscriptionParams{
			CancelAtPeriodEnd: stripe.Bool(true),
		})
		if err != nil {
			log.Printf("[Stripe] CancelSubscription schedule failed: sub_id=%s err=%v", providerSubID, err)
			return fmt.Errorf("stripe schedule cancel subscription failed: %w", err)
		}
	}
	log.Printf("[Stripe] CancelSubscription success: sub_id=%s", providerSubID)
	return nil
}

// Refund Stripe 退款
func (a *Adapter) Refund(ctx context.Context, req *payment.RefundRequest) (*payment.RefundResult, error) {
	params := &stripe.RefundParams{
		PaymentIntent: stripe.String(req.ProviderPaymentID),
	}
	if req.Amount > 0 {
		params.Amount = stripe.Int64(req.Amount)
	}
	if req.Reason != "" {
		params.Reason = stripe.String(req.Reason)
	}

	log.Printf("[Stripe] Refund: payment_id=%s amount=%d", req.ProviderPaymentID, req.Amount)

	r, err := refund.New(params)
	if err != nil {
		log.Printf("[Stripe] Refund failed: payment_id=%s err=%v", req.ProviderPaymentID, err)
		return nil, fmt.Errorf("stripe refund failed: %w", err)
	}

	log.Printf("[Stripe] Refund success: payment_id=%s refund_id=%s status=%s", req.ProviderPaymentID, r.ID, r.Status)
	return &payment.RefundResult{
		ProviderRefundID: r.ID,
		Status:           string(r.Status),
	}, nil
}

// VerifyWebhook 使用 stripe-go SDK 验证 webhook 签名并解析事件
func (a *Adapter) VerifyWebhook(ctx context.Context, payload *payment.WebhookPayload) (*payment.WebhookEvent, error) {
	sigHeader := payload.Headers["Stripe-Signature"]
	if sigHeader == "" {
		log.Printf("[Stripe] VerifyWebhook missing Stripe-Signature header")
		return nil, fmt.Errorf("missing Stripe-Signature header")
	}

	// 使用官方 SDK 验签（内置时间窗口校验，默认 5 分钟防重放）
	event, err := webhook.ConstructEvent(payload.Body, sigHeader, a.webhookSecret)
	if err != nil {
		log.Printf("[Stripe] VerifyWebhook signature verification failed: %v", err)
		return nil, fmt.Errorf("stripe webhook signature verification failed: %w", err)
	}

	log.Printf("[Stripe] VerifyWebhook verified: event_id=%s type=%s", event.ID, event.Type)

	// 解析 data.object 中的关键字段
	var raw map[string]interface{}
	if err := json.Unmarshal(payload.Body, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse webhook body: %w", err)
	}

	return &payment.WebhookEvent{
		EventID:   event.ID,
		RawType:   string(event.Type),
		EventType: mapStripeEventType(string(event.Type)),
		Data:      extractStripeData(raw),
		RawJSON:   string(payload.Body),
	}, nil
}

