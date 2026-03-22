package paypal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"interview-agents/internal/config"
	"interview-agents/internal/payment"

	pp "github.com/plutov/paypal/v4"
)

// Adapter PayPal 支付适配器
type Adapter struct {
	client    *pp.Client
	webhookID string
	sandbox   bool
}

// NewAdapter 创建 PayPal 适配器
func NewAdapter(cfg config.PayPalConfig) (*Adapter, error) {
	apiBase := pp.APIBaseLive
	if cfg.Sandbox {
		apiBase = pp.APIBaseSandBox
	}

	client, err := pp.NewClient(cfg.ClientID, cfg.ClientSecret, apiBase)
	if err != nil {
		return nil, fmt.Errorf("paypal: failed to create client: %w", err)
	}

	// 获取 access token（SDK 会自动刷新）
	if _, err := client.GetAccessToken(context.Background()); err != nil {
		return nil, fmt.Errorf("paypal: failed to get access token: %w", err)
	}

	return &Adapter{
		client:    client,
		webhookID: cfg.WebhookID,
		sandbox:   cfg.Sandbox,
	}, nil
}

func (a *Adapter) Name() payment.ProviderName {
	return payment.ProviderPayPal
}

// CreateCheckout 创建 PayPal 订单（Orders API v2）
// PayPal 的一次性支付流程：创建 Order -> 用户在 PayPal 页面 approve -> 服务端 Capture
func (a *Adapter) CreateCheckout(ctx context.Context, req *payment.CheckoutRequest) (*payment.CheckoutResult, error) {
	// PayPal 金额格式：字符串，单位为标准货币单位（如 "10.00" 表示 10 美元）
	amountStr := formatPayPalAmount(req.Amount, req.Currency)

	log.Printf("[PayPal] CreateCheckout: order_no=%s amount=%s %s", req.OrderNo, amountStr, strings.ToUpper(req.Currency))

	order, err := a.client.CreateOrder(
		ctx,
		pp.OrderIntentCapture,
		[]pp.PurchaseUnitRequest{{
			ReferenceID: req.OrderNo,
			Description: req.ProductName,
			CustomID:    req.OrderNo,
			Amount: &pp.PurchaseUnitAmount{
				Currency: strings.ToUpper(req.Currency),
				Value:    amountStr,
			},
		}},
		nil, // payment source
		&pp.ApplicationContext{
			ReturnURL:          req.SuccessURL,
			CancelURL:          req.CancelURL,
			BrandName:          "AI Interview Platform",
			UserAction:         pp.UserActionPayNow,
			ShippingPreference: pp.ShippingPreferenceNoShipping,
		},
	)
	if err != nil {
		log.Printf("[PayPal] CreateCheckout failed: order_no=%s err=%v", req.OrderNo, err)
		return nil, fmt.Errorf("paypal create order failed: %w", err)
	}

	log.Printf("[PayPal] CreateCheckout order created: order_no=%s paypal_order_id=%s", req.OrderNo, order.ID)

	// 从 links 中提取 approve URL（用户跳转链接）
	approveURL := ""
	for _, link := range order.Links {
		if link.Rel == "approve" {
			approveURL = link.Href
			break
		}
	}

	return &payment.CheckoutResult{
		ProviderID:  order.ID,
		CheckoutURL: approveURL,
	}, nil
}

// CreateSubscription 创建 PayPal 订阅（Subscriptions API）
// PriceID 在 PayPal 中对应 Plan ID
func (a *Adapter) CreateSubscription(ctx context.Context, req *payment.SubscriptionRequest) (*payment.SubscriptionResult, error) {
	log.Printf("[PayPal] CreateSubscription: order_no=%s plan_id=%s", req.OrderNo, req.PriceID)

	sub, err := a.client.CreateSubscription(ctx, pp.SubscriptionBase{
		PlanID:   req.PriceID,
		CustomID: req.OrderNo,
		ApplicationContext: &pp.ApplicationContext{
			ReturnURL:          req.SuccessURL,
			CancelURL:          req.CancelURL,
			BrandName:          "AI Interview Platform",
			UserAction:         pp.UserActionSubscribeNow,
			ShippingPreference: pp.ShippingPreferenceNoShipping,
		},
	})
	if err != nil {
		log.Printf("[PayPal] CreateSubscription failed: order_no=%s err=%v", req.OrderNo, err)
		return nil, fmt.Errorf("paypal create subscription failed: %w", err)
	}

	log.Printf("[PayPal] CreateSubscription created: order_no=%s sub_id=%s", req.OrderNo, sub.ID)

	// 提取 approve URL
	approveURL := ""
	for _, link := range sub.Links {
		if link.Rel == "approve" {
			approveURL = link.Href
			break
		}
	}

	return &payment.SubscriptionResult{
		ProviderSubID: sub.ID,
		ProviderID:    sub.ID,
		CheckoutURL:   approveURL,
	}, nil
}

// CancelSubscription 取消 PayPal 订阅
func (a *Adapter) CancelSubscription(ctx context.Context, providerSubID string, immediate bool) error {
	log.Printf("[PayPal] CancelSubscription: sub_id=%s immediate=%v", providerSubID, immediate)

	// PayPal 订阅取消即为立即取消（不支持"周期结束后取消"，需要用 Suspend 模拟）
	if immediate {
		err := a.client.CancelSubscription(ctx, providerSubID, "User requested cancellation")
		if err != nil {
			log.Printf("[PayPal] CancelSubscription failed: sub_id=%s err=%v", providerSubID, err)
			return fmt.Errorf("paypal cancel subscription failed: %w", err)
		}
	} else {
		// PayPal 没有原生的 cancel_at_period_end，使用 Suspend 暂停续费
		err := a.client.SuspendSubscription(ctx, providerSubID, "Scheduled cancellation at period end")
		if err != nil {
			log.Printf("[PayPal] SuspendSubscription failed: sub_id=%s err=%v", providerSubID, err)
			return fmt.Errorf("paypal suspend subscription failed: %w", err)
		}
	}
	log.Printf("[PayPal] CancelSubscription success: sub_id=%s", providerSubID)
	return nil
}

// Refund PayPal 退款（Captures API）
func (a *Adapter) Refund(ctx context.Context, req *payment.RefundRequest) (*payment.RefundResult, error) {
	log.Printf("[PayPal] Refund: capture_id=%s amount=%d", req.ProviderPaymentID, req.Amount)

	refundReq := pp.RefundCaptureRequest{
		NoteToPayer: req.Reason,
	}

	// 部分退款需要指定金额
	if req.Amount > 0 {
		refundReq.Amount = &pp.Money{
			Currency: "USD", // 默认 USD，实际应从订单中获取
			Value:    formatPayPalAmount(req.Amount, "usd"),
		}
	}

	refundResp, err := a.client.RefundCapture(ctx, req.ProviderPaymentID, refundReq)
	if err != nil {
		log.Printf("[PayPal] Refund failed: capture_id=%s err=%v", req.ProviderPaymentID, err)
		return nil, fmt.Errorf("paypal refund failed: %w", err)
	}

	log.Printf("[PayPal] Refund success: capture_id=%s refund_id=%s status=%s", req.ProviderPaymentID, refundResp.ID, refundResp.Status)
	return &payment.RefundResult{
		ProviderRefundID: refundResp.ID,
		Status:           refundResp.Status,
	}, nil
}

// VerifyWebhook 验证 PayPal webhook 签名并解析事件
func (a *Adapter) VerifyWebhook(ctx context.Context, payload *payment.WebhookPayload) (*payment.WebhookEvent, error) {
	// 构造 http.Request 用于 SDK 验签
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "/", nil)
	if err != nil {
		return nil, fmt.Errorf("paypal: failed to create request for verification: %w", err)
	}
	for k, v := range payload.Headers {
		httpReq.Header.Set(k, v)
	}

	// 调用 PayPal Webhook Signature Verification API
	verifyResp, err := a.client.VerifyWebhookSignature(ctx, httpReq, a.webhookID)
	if err != nil {
		log.Printf("[PayPal] VerifyWebhook signature check failed: %v", err)
		return nil, fmt.Errorf("paypal webhook verification failed: %w", err)
	}
	if verifyResp.VerificationStatus != "SUCCESS" {
		log.Printf("[PayPal] VerifyWebhook signature invalid: status=%s", verifyResp.VerificationStatus)
		return nil, fmt.Errorf("paypal webhook signature invalid: status=%s", verifyResp.VerificationStatus)
	}

	// 解析事件内容
	var raw map[string]interface{}
	if err := json.Unmarshal(payload.Body, &raw); err != nil {
		log.Printf("[PayPal] VerifyWebhook body parse failed: %v", err)
		return nil, fmt.Errorf("failed to parse PayPal webhook body: %w", err)
	}

	eventID, _ := raw["id"].(string)
	eventType, _ := raw["event_type"].(string)

	if eventID == "" || eventType == "" {
		log.Printf("[PayPal] VerifyWebhook missing id or event_type in body")
		return nil, fmt.Errorf("invalid PayPal webhook: missing id or event_type")
	}

	log.Printf("[PayPal] VerifyWebhook verified: event_id=%s type=%s", eventID, eventType)
	return &payment.WebhookEvent{
		EventID:   eventID,
		RawType:   eventType,
		EventType: mapPayPalEventType(eventType),
		Data:      extractPayPalData(raw),
		RawJSON:   string(payload.Body),
	}, nil
}

// formatPayPalAmount 将最小货币单位转换为 PayPal 金额字符串
// 例如：1050 cents -> "10.50"
func formatPayPalAmount(amountInMinorUnits int64, currency string) string {
	// 大多数货币使用 2 位小数（如 USD, EUR）
	// 零小数货币（如 JPY）直接使用整数
	cur := strings.ToUpper(currency)
	zeroDecimalCurrencies := map[string]bool{
		"BIF": true, "CLP": true, "DJF": true, "GNF": true,
		"JPY": true, "KMF": true, "KRW": true, "MGA": true,
		"PYG": true, "RWF": true, "UGX": true, "VND": true,
		"VUV": true, "XAF": true, "XOF": true, "XPF": true,
	}

	if zeroDecimalCurrencies[cur] {
		return strconv.FormatInt(amountInMinorUnits, 10)
	}

	major := amountInMinorUnits / 100
	minor := amountInMinorUnits % 100
	return fmt.Sprintf("%d.%02d", major, minor)
}
