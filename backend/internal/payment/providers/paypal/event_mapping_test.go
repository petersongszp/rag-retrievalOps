package paypal

import (
	"interview-agents/internal/payment"
	"testing"
)

func TestMapPayPalEventType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"CHECKOUT.ORDER.APPROVED", payment.EventCheckoutCompleted},
		{"PAYMENT.CAPTURE.COMPLETED", payment.EventPaymentSucceeded},
		{"PAYMENT.CAPTURE.DENIED", payment.EventPaymentFailed},
		{"BILLING.SUBSCRIPTION.CREATED", payment.EventSubscriptionCreated},
		{"BILLING.SUBSCRIPTION.UPDATED", payment.EventSubscriptionUpdated},
		{"BILLING.SUBSCRIPTION.CANCELLED", payment.EventSubscriptionCanceled},
		{"PAYMENT.SALE.COMPLETED", payment.EventSubscriptionRenewed},
		{"PAYMENT.CAPTURE.REFUNDED", payment.EventRefundCompleted},
		{"UNKNOWN.EVENT", "UNKNOWN.EVENT"}, // 未知事件原样返回
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapPayPalEventType(tt.input)
			if result != tt.expected {
				t.Errorf("mapPayPalEventType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractPayPalData(t *testing.T) {
	raw := map[string]interface{}{
		"id":         "WH-123",
		"event_type": "PAYMENT.CAPTURE.COMPLETED",
		"resource": map[string]interface{}{
			"id":                   "CAP_001",
			"status":               "COMPLETED",
			"custom_id":            "PAY20250101000001",
			"billing_agreement_id": "BA_001",
		},
	}

	data := extractPayPalData(raw)

	expected := map[string]string{
		"object_id":       "CAP_001",
		"status":          "COMPLETED",
		"order_no":        "PAY20250101000001",
		"subscription_id": "BA_001",
	}

	for k, v := range expected {
		if data[k] != v {
			t.Errorf("extractPayPalData[%q] = %q, want %q", k, data[k], v)
		}
	}
}

func TestExtractPayPalDataEmpty(t *testing.T) {
	raw := map[string]interface{}{}
	data := extractPayPalData(raw)
	if len(data) != 0 {
		t.Errorf("expected empty data, got %v", data)
	}
}

