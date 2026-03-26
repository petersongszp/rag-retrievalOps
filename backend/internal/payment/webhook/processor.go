package webhook

import (
	"context"
	"fmt"
	"log"

	"interview-agents/internal/model"
	"interview-agents/internal/payment"

	"gorm.io/gorm"
)

// Processor webhook 事件处理器
type Processor struct {
	db *gorm.DB
}

// NewProcessor 创建 webhook 处理器
func NewProcessor(db *gorm.DB) *Processor {
	return &Processor{db: db}
}

// Process 处理 webhook 事件（验签由 Provider 完成，这里做去重 + 落库 + 业务处理）
func (p *Processor) Process(ctx context.Context, provider payment.ProviderName, event *payment.WebhookEvent) error {
	source := string(provider)

	// 1. 去重：检查事件是否已处理
	exists, err := model.PaymentEventDao.ExistsBySourceEventID(source, event.EventID)
	if err != nil {
		return fmt.Errorf("check event existence failed: %w", err)
	}
	if exists {
		log.Printf("[webhook] duplicate event ignored: source=%s event_id=%s", source, event.EventID)
		return nil // 幂等返回成功
	}

	// 2. 落库保存原始事件
	eventRecord := &model.PaymentEvent{
		Source:        source,
		SourceEventID: event.EventID,
		EventType:     event.EventType,
		Status:        model.EventStatusVerified,
		RawPayload:    event.RawJSON,
	}

	if err := model.PaymentEventDao.Create(p.db, eventRecord); err != nil {
		// 唯一索引冲突 = 并发去重，视为成功
		log.Printf("[webhook] event create conflict (concurrent dedup): %v", err)
		return nil
	}

	// 3. 在事务内处理业务逻辑
	processErr := model.WithTransaction(ctx, func(tx *gorm.DB) error {
		return p.handleEvent(tx, provider, event)
	})

	// 4. 更新事件处理状态
	if processErr != nil {
		log.Printf("[webhook] event process failed: source=%s event_id=%s err=%v", source, event.EventID, processErr)
		_ = model.PaymentEventDao.UpdateStatus(p.db, eventRecord.ID, model.EventStatusFailed, processErr.Error())
		return processErr
	}

	_ = model.PaymentEventDao.UpdateStatus(p.db, eventRecord.ID, model.EventStatusProcessed, "ok")
	return nil
}

// handleEvent 根据事件类型分发处理
func (p *Processor) handleEvent(tx *gorm.DB, provider payment.ProviderName, event *payment.WebhookEvent) error {
	switch event.EventType {
	case payment.EventCheckoutCompleted:
		return p.handleCheckoutCompleted(tx, provider, event)
	case payment.EventPaymentSucceeded:
		return p.handlePaymentSucceeded(tx, provider, event)
	case payment.EventPaymentFailed:
		return p.handlePaymentFailed(tx, provider, event)
	case payment.EventSubscriptionCreated:
		return p.handleSubscriptionCreated(tx, provider, event)
	case payment.EventSubscriptionUpdated:
		return p.handleSubscriptionUpdated(tx, provider, event)
	case payment.EventSubscriptionCanceled:
		return p.handleSubscriptionCanceled(tx, provider, event)
	case payment.EventSubscriptionRenewed:
		return p.handleSubscriptionRenewed(tx, provider, event)
	default:
		log.Printf("[webhook] unhandled event type: %s (raw: %s)", event.EventType, event.RawType)
		return nil
	}
}
