package webhook

import (
	"context"
	"testing"

	"interview-agents/internal/model"
	"interview-agents/internal/payment"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB 创建 SQLite 内存数据库用于测试
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	// 自动迁移支付相关表
	err = db.AutoMigrate(
		&model.PaymentOrder{},
		&model.PaymentAttempt{},
		&model.Subscription{},
		&model.PaymentEvent{},
	)
	if err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	// 设置 model 包的 DB getter
	model.SetDBGetter(func() *gorm.DB { return db })

	return db
}

// TestWebhookEventDedup 测试 webhook 事件去重
func TestWebhookEventDedup(t *testing.T) {
	db := setupTestDB(t)
	processor := NewProcessor(db)
	ctx := context.Background()

	// 先创建一个订单用于测试
	order := &model.PaymentOrder{
		OrderNo:        "PAY20250101000001",
		UserID:         1,
		OrderType:      model.OrderTypeOneTime,
		ProductCode:    "monthly",
		PriceCode:      "monthly_9.99",
		Amount:         999,
		Currency:       "usd",
		Status:         model.OrderStatusPendingPayment,
		Provider:       "stripe",
		IdempotencyKey: "idem_test_001",
	}
	if err := model.PaymentOrderDao.Create(db, order); err != nil {
		t.Fatalf("create test order failed: %v", err)
	}

	event := &payment.WebhookEvent{
		EventID:   "evt_test_001",
		EventType: payment.EventPaymentSucceeded,
		RawType:   "payment_intent.succeeded",
		Data:      map[string]string{"order_no": "PAY20250101000001"},
		RawJSON:   `{"id":"evt_test_001","type":"payment_intent.succeeded"}`,
	}

	// 第一次处理应成功
	err := processor.Process(ctx, payment.ProviderStripe, event)
	if err != nil {
		t.Fatalf("first process should succeed, got: %v", err)
	}

	// 验证事件已落库
	exists, err := model.PaymentEventDao.ExistsBySourceEventID("stripe", "evt_test_001")
	if err != nil {
		t.Fatalf("check event existence failed: %v", err)
	}
	if !exists {
		t.Fatal("event should exist after first process")
	}

	// 第二次处理同一事件应幂等返回成功（不报错）
	err = processor.Process(ctx, payment.ProviderStripe, event)
	if err != nil {
		t.Fatalf("duplicate event should be idempotent, got: %v", err)
	}

	// 验证只有一条事件记录
	var count int64
	db.Model(&model.PaymentEvent{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 event record, got %d", count)
	}
}

// TestPaymentSucceededUpdatesOrder 测试支付成功事件更新订单状态
func TestPaymentSucceededUpdatesOrder(t *testing.T) {
	db := setupTestDB(t)
	processor := NewProcessor(db)
	ctx := context.Background()

	// 创建待支付订单
	order := &model.PaymentOrder{
		OrderNo:        "PAY20250101000002",
		UserID:         1,
		OrderType:      model.OrderTypeOneTime,
		ProductCode:    "monthly",
		PriceCode:      "monthly_9.99",
		Amount:         999,
		Currency:       "usd",
		Status:         model.OrderStatusPendingPayment,
		Provider:       "stripe",
		IdempotencyKey: "idem_test_002",
	}
	if err := model.PaymentOrderDao.Create(db, order); err != nil {
		t.Fatalf("create test order failed: %v", err)
	}

	event := &payment.WebhookEvent{
		EventID:   "evt_test_002",
		EventType: payment.EventPaymentSucceeded,
		RawType:   "payment_intent.succeeded",
		Data:      map[string]string{"order_no": "PAY20250101000002"},
		RawJSON:   `{"id":"evt_test_002","type":"payment_intent.succeeded"}`,
	}

	err := processor.Process(ctx, payment.ProviderStripe, event)
	if err != nil {
		t.Fatalf("process payment succeeded event failed: %v", err)
	}

	// 验证订单状态已更新为 PAID
	updated, err := model.PaymentOrderDao.FindByOrderNo("PAY20250101000002")
	if err != nil {
		t.Fatalf("find order failed: %v", err)
	}
	if updated.Status != model.OrderStatusPaid {
		t.Fatalf("expected order status PAID, got %s", updated.Status)
	}
	if updated.PaidAt == nil {
		t.Fatal("expected paid_at to be set")
	}
}

