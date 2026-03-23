package stripe

import (
	"interview-agents/internal/payment"
	"testing"
)

func TestMapStripeEventType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"checkout.session.completed", payment.EventCheckoutCompleted},
		{"payment_intent.succeeded", payment.EventPaymentSucceeded},
		{"payment_intent.payment_failed", payment.EventPaymentFailed},
		{"customer.subscription.created", payment.EventSubscriptionCreated},
		{"customer.subscription.updated", payment.EventSubscriptionUpdated},
		{"customer.subscription.deleted", payment.EventSubscriptionCanceled},
		{"invoice.paid", payment.EventSubscriptionRenewed},
		{"charge.refunded", payment.EventRefundCompleted},
		{"unknown.event.type", "unknown.event.type"}, // 未知事件原样返回
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapStripeEventType(tt.input)
			if result != tt.expected {
				t.Errorf("mapStripeEventType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractStripeData(t *testing.T) {
	raw := map[string]interface{}{
		"id":   "evt_123",
		"type": "checkout.session.completed",
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"id":             "cs_test_abc",
				"status":         "complete",
				"mode":           "payment",
				"subscription":   "sub_xyz",
				"payment_intent": "pi_def",
				"metadata": map[string]interface{}{
					"order_no": "PAY20250101000001",
				},
			},
		},
	}

	data := extractStripeData(raw)

	expected := map[string]string{
		"object_id":         "cs_test_abc",
		"status":            "complete",
		"mode":              "payment",
		"subscription_id":   "sub_xyz",
		"payment_intent_id": "pi_def",
		"order_no":          "PAY20250101000001",
	}

	for k, v := range expected {
		if data[k] != v {
			t.Errorf("extractStripeData[%q] = %q, want %q", k, data[k], v)
		}
	}
}

func TestExtractStripeDataEmpty(t *testing.T) {
	// 空 data
	raw := map[string]interface{}{}
	data := extractStripeData(raw)
	if len(data) != 0 {
		t.Errorf("expected empty data, got %v", data)
	}

	// data 但无 object
	raw2 := map[string]interface{}{
		"data": map[string]interface{}{},
	}
	data2 := extractStripeData(raw2)
	if len(data2) != 0 {
		t.Errorf("expected empty data, got %v", data2)
	}
}
