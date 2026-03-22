package webhook

import (
	"testing"

	"interview-agents/internal/model"
)

// TestIdempotencyKeyPreventsDoubleOrder 测试幂等键防止重复创建订单
func TestIdempotencyKeyPreventsDoubleOrder(t *testing.T) {
	db := setupTestDB(t)

	// 创建第一个订单
	order1 := &model.PaymentOrder{
		OrderNo:        "PAY20250101000010",
		UserID:         1,
		OrderType:      model.OrderTypeOneTime,
		ProductCode:    "monthly",
		PriceCode:      "monthly_9.99",
		Amount:         999,
		Currency:       "usd",
		Status:         model.OrderStatusCreated,
		Provider:       "stripe",
		IdempotencyKey: "idem_unique_001",
	}
	err := model.PaymentOrderDao.Create(db, order1)
	if err != nil {
		t.Fatalf("create first order failed: %v", err)
	}

	// 尝试用相同幂等键创建第二个订单，应失败（唯一索引冲突）
	order2 := &model.PaymentOrder{
		OrderNo:        "PAY20250101000011",
		UserID:         1,
		OrderType:      model.OrderTypeOneTime,
		ProductCode:    "monthly",
		PriceCode:      "monthly_9.99",
		Amount:         999,
		Currency:       "usd",
		Status:         model.OrderStatusCreated,
		Provider:       "stripe",
		IdempotencyKey: "idem_unique_001", // 相同幂等键
	}
	err = model.PaymentOrderDao.Create(db, order2)
	if err == nil {
		t.Fatal("expected error for duplicate idempotency_key, got nil")
	}

	// 验证通过幂等键查询能找到原始订单
	found, err := model.PaymentOrderDao.FindByIdempotencyKey("idem_unique_001")
	if err != nil {
		t.Fatalf("find by idempotency key failed: %v", err)
	}
	if found.OrderNo != "PAY20250101000010" {
		t.Fatalf("expected order PAY20250101000010, got %s", found.OrderNo)
	}
}

// TestOrderStatusTransitions 测试订单状态流转
func TestOrderStatusTransitions(t *testing.T) {
	db := setupTestDB(t)

	order := &model.PaymentOrder{
		OrderNo:        "PAY20250101000020",
		UserID:         1,
		OrderType:      model.OrderTypeOneTime,
		ProductCode:    "monthly",
		PriceCode:      "monthly_9.99",
		Amount:         999,
		Currency:       "usd",
		Status:         model.OrderStatusCreated,
		Provider:       "stripe",
		IdempotencyKey: "idem_state_001",
	}
	if err := model.PaymentOrderDao.Create(db, order); err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	// CREATED -> PENDING_PAYMENT
	err := model.PaymentOrderDao.UpdateStatus(db, "PAY20250101000020", model.OrderStatusPendingPayment, nil)
	if err != nil {
		t.Fatalf("update to PENDING_PAYMENT failed: %v", err)
	}
	o, _ := model.PaymentOrderDao.FindByOrderNo("PAY20250101000020")
	if o.Status != model.OrderStatusPendingPayment {
		t.Fatalf("expected PENDING_PAYMENT, got %s", o.Status)
	}

	// PENDING_PAYMENT -> PROCESSING
	err = model.PaymentOrderDao.UpdateStatus(db, "PAY20250101000020", model.OrderStatusProcessing, nil)
	if err != nil {
		t.Fatalf("update to PROCESSING failed: %v", err)
	}
	o, _ = model.PaymentOrderDao.FindByOrderNo("PAY20250101000020")
	if o.Status != model.OrderStatusProcessing {
		t.Fatalf("expected PROCESSING, got %s", o.Status)
	}

	// PROCESSING -> PAID
	err = model.PaymentOrderDao.UpdateStatus(db, "PAY20250101000020", model.OrderStatusPaid, nil)
	if err != nil {
		t.Fatalf("update to PAID failed: %v", err)
	}
	o, _ = model.PaymentOrderDao.FindByOrderNo("PAY20250101000020")
	if o.Status != model.OrderStatusPaid {
		t.Fatalf("expected PAID, got %s", o.Status)
	}

	// PAID -> FULFILLED
	err = model.PaymentOrderDao.UpdateStatus(db, "PAY20250101000020", model.OrderStatusFulfilled, nil)
	if err != nil {
		t.Fatalf("update to FULFILLED failed: %v", err)
	}
	o, _ = model.PaymentOrderDao.FindByOrderNo("PAY20250101000020")
	if o.Status != model.OrderStatusFulfilled {
		t.Fatalf("expected FULFILLED, got %s", o.Status)
	}
}

// TestSubscriptionStatusTransitions 测试订阅状态流转
func TestSubscriptionStatusTransitions(t *testing.T) {
	db := setupTestDB(t)

	sub := &model.Subscription{
		SubscriptionNo: "SUB20250101000001",
		UserID:         1,
		OrderNo:        "PAY20250101000030",
		Provider:       "stripe",
		ProviderSubID:  "sub_stripe_001",
		ProductCode:    "pro_monthly",
		PriceCode:      "pro_monthly_19.99",
		Status:         model.SubscriptionStatusInit,
	}
	if err := model.SubscriptionDao.Create(db, sub); err != nil {
		t.Fatalf("create subscription failed: %v", err)
	}

	// INIT -> ACTIVE
	err := model.SubscriptionDao.UpdateStatus(db, "SUB20250101000001", model.SubscriptionStatusActive, nil)
	if err != nil {
		t.Fatalf("update to ACTIVE failed: %v", err)
	}
	s, _ := model.SubscriptionDao.FindBySubscriptionNo("SUB20250101000001")
	if s.Status != model.SubscriptionStatusActive {
		t.Fatalf("expected ACTIVE, got %s", s.Status)
	}

	// ACTIVE -> CANCEL_SCHEDULED
	err = model.SubscriptionDao.UpdateStatus(db, "SUB20250101000001", model.SubscriptionStatusCancelScheduled, nil)
	if err != nil {
		t.Fatalf("update to CANCEL_SCHEDULED failed: %v", err)
	}

	// CANCEL_SCHEDULED -> CANCELED
	err = model.SubscriptionDao.UpdateStatus(db, "SUB20250101000001", model.SubscriptionStatusCanceled, nil)
	if err != nil {
		t.Fatalf("update to CANCELED failed: %v", err)
	}
	s, _ = model.SubscriptionDao.FindBySubscriptionNo("SUB20250101000001")
	if s.Status != model.SubscriptionStatusCanceled {
		t.Fatalf("expected CANCELED, got %s", s.Status)
	}
}
