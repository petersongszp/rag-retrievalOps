package webhook

import (
	"fmt"
	"log"
	"time"

	"interview-agents/internal/model"
	"interview-agents/internal/payment"

	"gorm.io/gorm"
)

// handleCheckoutCompleted 处理 checkout 完成事件
func (p *Processor) handleCheckoutCompleted(tx *gorm.DB, event *payment.WebhookEvent) error {
	orderNo := event.Data["order_no"]
	if orderNo == "" {
		log.Printf("[webhook] checkout.completed missing order_no, event_id=%s", event.EventID)
		return nil // 非关键错误，不阻塞
	}

	providerID := event.Data["object_id"]

	// 更新 payment_attempt
	if providerID != "" {
		_ = model.PaymentAttemptDao.UpdateStatus(tx, "", model.AttemptStatusProcessing, map[string]interface{}{
			"provider_id": providerID,
		})
	}

	// 更新订单状态为 PROCESSING
	return model.PaymentOrderDao.UpdateStatus(tx, orderNo, model.OrderStatusProcessing, nil)
}

// handlePaymentSucceeded 处理支付成功事件
func (p *Processor) handlePaymentSucceeded(tx *gorm.DB, event *payment.WebhookEvent) error {
	orderNo := event.Data["order_no"]
	if orderNo == "" {
		log.Printf("[webhook] payment.succeeded missing order_no, event_id=%s", event.EventID)
		return nil
	}

	now := time.Now()
	err := model.PaymentOrderDao.UpdateStatus(tx, orderNo, model.OrderStatusPaid, map[string]interface{}{
		"paid_at": &now,
	})
	if err != nil {
		return fmt.Errorf("update order to PAID failed: %w", err)
	}

	// TODO: 在这里触发权益发放逻辑（如更新用户会员状态）
	log.Printf("[webhook] order %s paid successfully", orderNo)
	return nil
}

// handlePaymentFailed 处理支付失败事件
func (p *Processor) handlePaymentFailed(tx *gorm.DB, event *payment.WebhookEvent) error {
	orderNo := event.Data["order_no"]
	if orderNo == "" {
		log.Printf("[webhook] payment.failed missing order_no, event_id=%s", event.EventID)
		return nil
	}

	log.Printf("[webhook] order %s payment failed, event_id=%s", orderNo, event.EventID)
	return model.PaymentOrderDao.UpdateStatus(tx, orderNo, model.OrderStatusFailed, nil)
}

// handleSubscriptionCreated 处理订阅创建事件
func (p *Processor) handleSubscriptionCreated(tx *gorm.DB, event *payment.WebhookEvent) error {
	subID := event.Data["subscription_id"]
	if subID == "" {
		subID = event.Data["object_id"]
	}
	if subID == "" {
		return nil
	}

	orderNo := event.Data["order_no"]
	log.Printf("[webhook] subscription created: provider_sub_id=%s order_no=%s", subID, orderNo)

	// 如果订阅记录已存在，更新状态；否则仅记录日志
	// 实际的订阅记录应在 checkout 流程中创建
	return nil
}

// handleSubscriptionUpdated 处理订阅更新事件
func (p *Processor) handleSubscriptionUpdated(tx *gorm.DB, event *payment.WebhookEvent) error {
	subID := event.Data["subscription_id"]
	if subID == "" {
		subID = event.Data["object_id"]
	}
	if subID == "" {
		return nil
	}

	status := event.Data["status"]
	log.Printf("[webhook] subscription updated: provider_sub_id=%s status=%s", subID, status)

	// 根据渠道状态映射更新本地订阅状态
	// 具体映射逻辑取决于业务需求
	return nil
}

// handleSubscriptionCanceled 处理订阅取消事件
func (p *Processor) handleSubscriptionCanceled(tx *gorm.DB, event *payment.WebhookEvent) error {
	subID := event.Data["subscription_id"]
	if subID == "" {
		subID = event.Data["object_id"]
	}
	if subID == "" {
		log.Printf("[webhook] subscription.canceled missing subscription_id, event_id=%s", event.EventID)
		return nil
	}

	log.Printf("[webhook] subscription canceled: provider_sub_id=%s event_id=%s", subID, event.EventID)
	now := time.Now()
	return model.SubscriptionDao.UpdateStatus(tx, "", model.SubscriptionStatusCanceled, map[string]interface{}{
		"canceled_at":    &now,
		"provider_sub_id": subID,
	})
}

// handleSubscriptionRenewed 处理订阅续费事件
func (p *Processor) handleSubscriptionRenewed(tx *gorm.DB, event *payment.WebhookEvent) error {
	subID := event.Data["subscription_id"]
	if subID == "" {
		return nil
	}

	log.Printf("[webhook] subscription renewed: provider_sub_id=%s", subID)
	// TODO: 更新订阅周期、延长权益
	return nil
}

