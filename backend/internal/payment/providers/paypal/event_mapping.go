package paypal

import (
	"interview-agents/internal/payment"
)

// mapPayPalEventType 将 PayPal 事件类型映射为标准事件类型
func mapPayPalEventType(paypalType string) string {
	switch paypalType {
	case "CHECKOUT.ORDER.APPROVED":
		return payment.EventCheckoutCompleted
	case "PAYMENT.CAPTURE.COMPLETED":
		return payment.EventPaymentSucceeded
	case "PAYMENT.CAPTURE.DENIED":
		return payment.EventPaymentFailed
	case "BILLING.SUBSCRIPTION.CREATED":
		return payment.EventSubscriptionCreated
	case "BILLING.SUBSCRIPTION.UPDATED":
		return payment.EventSubscriptionUpdated
	case "BILLING.SUBSCRIPTION.CANCELLED":
		return payment.EventSubscriptionCanceled
	case "PAYMENT.SALE.COMPLETED":
		return payment.EventSubscriptionRenewed
	case "PAYMENT.CAPTURE.REFUNDED":
		return payment.EventRefundCompleted
	default:
		return paypalType
	}
}

// extractPayPalData 从 PayPal 事件中提取关键字段
func extractPayPalData(raw map[string]interface{}) map[string]string {
	data := make(map[string]string)

	resource, _ := raw["resource"].(map[string]interface{})
	if resource == nil {
		return data
	}

	if id, ok := resource["id"].(string); ok {
		data["object_id"] = id
	}
	if status, ok := resource["status"].(string); ok {
		data["status"] = status
	}

	// 提取 custom_id（我们在创建时传入 order_no）
	if customID, ok := resource["custom_id"].(string); ok {
		data["order_no"] = customID
	}

	// 订阅相关
	if billingAgreementID, ok := resource["billing_agreement_id"].(string); ok {
		data["subscription_id"] = billingAgreementID
	}

	return data
}
