package stripe

import (
	"interview-agents/internal/payment"
)

// mapStripeEventType 将 Stripe 事件类型映射为标准事件类型
func mapStripeEventType(stripeType string) string {
	switch stripeType {
	case "checkout.session.completed":
		return payment.EventCheckoutCompleted
	case "payment_intent.succeeded":
		return payment.EventPaymentSucceeded
	case "payment_intent.payment_failed":
		return payment.EventPaymentFailed
	case "customer.subscription.created":
		return payment.EventSubscriptionCreated
	case "customer.subscription.updated":
		return payment.EventSubscriptionUpdated
	case "customer.subscription.deleted":
		return payment.EventSubscriptionCanceled
	case "invoice.paid":
		return payment.EventSubscriptionRenewed
	case "charge.refunded":
		return payment.EventRefundCompleted
	default:
		return stripeType
	}
}

// extractStripeData 从 Stripe 事件中提取关键字段
func extractStripeData(raw map[string]interface{}) map[string]string {
	data := make(map[string]string)

	obj, _ := raw["data"].(map[string]interface{})
	if obj == nil {
		return data
	}
	inner, _ := obj["object"].(map[string]interface{})
	if inner == nil {
		return data
	}

	// 提取常用字段
	if id, ok := inner["id"].(string); ok {
		data["object_id"] = id
	}
	if status, ok := inner["status"].(string); ok {
		data["status"] = status
	}
	if mode, ok := inner["mode"].(string); ok {
		data["mode"] = mode
	}
	if sub, ok := inner["subscription"].(string); ok {
		data["subscription_id"] = sub
	}
	if pi, ok := inner["payment_intent"].(string); ok {
		data["payment_intent_id"] = pi
	}

	// 提取 metadata 中的 order_no
	if metadata, ok := inner["metadata"].(map[string]interface{}); ok {
		if orderNo, ok := metadata["order_no"].(string); ok {
			data["order_no"] = orderNo
		}
	}

	return data
}
