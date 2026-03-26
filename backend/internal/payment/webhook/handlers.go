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
func (p *Processor) handleCheckoutCompleted(tx *gorm.DB, provider payment.ProviderName, event *payment.WebhookEvent) error {
	orderNo := event.Data["order_no"]
	if orderNo == "" {
		log.Printf("[webhook] checkout.completed missing order_no, event_id=%s", event.EventID)
		return nil // 非关键错误，不阻塞
	}

	// 更新 payment_attempt（按 order_no 找到尝试记录）
	// 注意：PayPal webhook 的 object_id 未必等于我们本地记录的 ProviderID（可能是 capture id）
	attempts, err := model.PaymentAttemptDao.FindByOrderNo(orderNo)
	if err != nil {
		log.Printf("[webhook] checkout.completed: find attempts failed: provider=%s order_no=%s err=%v", provider, orderNo, err)
	} else {
		for _, a := range attempts {
			_ = model.PaymentAttemptDao.UpdateStatus(tx, a.AttemptNo, model.AttemptStatusProcessing, nil)
		}
	}

	// 更新订单状态为 PROCESSING
	return model.PaymentOrderDao.UpdateStatus(tx, orderNo, model.OrderStatusProcessing, nil)
}

// handlePaymentSucceeded 处理支付成功事件
func (p *Processor) handlePaymentSucceeded(tx *gorm.DB, provider payment.ProviderName, event *payment.WebhookEvent) error {
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

	// 一次性支付上线优先级：只要把订单推进到 PAID 即可“可用”。
	// （权益发放目前代码里是 TODO，且单测期望 PAID；先不推进到 FULFILLED 避免破坏现有行为）
	attempts, err := model.PaymentAttemptDao.FindByOrderNo(orderNo)
	if err != nil {
		log.Printf("[webhook] payment.succeeded: find attempts failed: provider=%s order_no=%s err=%v", provider, orderNo, err)
	} else {
		for _, a := range attempts {
			_ = model.PaymentAttemptDao.UpdateStatus(tx, a.AttemptNo, model.AttemptStatusSucceeded, nil)
		}
	}
	log.Printf("[webhook] order %s paid successfully", orderNo)
	return nil
}

// handlePaymentFailed 处理支付失败事件
func (p *Processor) handlePaymentFailed(tx *gorm.DB, provider payment.ProviderName, event *payment.WebhookEvent) error {
	orderNo := event.Data["order_no"]
	if orderNo == "" {
		log.Printf("[webhook] payment.failed missing order_no, event_id=%s", event.EventID)
		return nil
	}

	log.Printf("[webhook] order %s payment failed, event_id=%s", orderNo, event.EventID)
	if err := model.PaymentOrderDao.UpdateStatus(tx, orderNo, model.OrderStatusFailed, nil); err != nil {
		return err
	}

	// 更新支付尝试状态（用于前端/后台排查）
	failureReason := event.RawType
	if v, ok := event.Data["status"]; ok && v != "" {
		failureReason = v
	}
	attempts, err := model.PaymentAttemptDao.FindByOrderNo(orderNo)
	if err != nil {
		log.Printf("[webhook] payment.failed: find attempts failed: provider=%s order_no=%s err=%v", provider, orderNo, err)
		return nil
	}
	for _, a := range attempts {
		_ = model.PaymentAttemptDao.UpdateStatus(tx, a.AttemptNo, model.AttemptStatusFailed, map[string]interface{}{
			"failure_reason": failureReason,
		})
	}
	return nil
}

// handleSubscriptionCreated 处理订阅创建事件
func (p *Processor) handleSubscriptionCreated(tx *gorm.DB, provider payment.ProviderName, event *payment.WebhookEvent) error {
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
func (p *Processor) handleSubscriptionUpdated(tx *gorm.DB, provider payment.ProviderName, event *payment.WebhookEvent) error {
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
func (p *Processor) handleSubscriptionCanceled(tx *gorm.DB, provider payment.ProviderName, event *payment.WebhookEvent) error {
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

	// 尽量把取消同步到本地 subscription 表（按 provider + provider_sub_id 找到 subscription_no）
	sub, err := model.SubscriptionDao.FindByProviderSubID(string(provider), subID)
	if err != nil {
		log.Printf("[webhook] subscription canceled: find local subscription failed: provider=%s provider_sub_id=%s err=%v", provider, subID, err)
		return nil
	}
	return model.SubscriptionDao.UpdateStatus(tx, sub.SubscriptionNo, model.SubscriptionStatusCanceled, map[string]interface{}{
		"canceled_at":     &now,
		"provider_sub_id": subID,
	})
}

// handleSubscriptionRenewed 处理订阅续费事件
func (p *Processor) handleSubscriptionRenewed(tx *gorm.DB, provider payment.ProviderName, event *payment.WebhookEvent) error {
	subID := event.Data["subscription_id"]
	if subID == "" {
		return nil
	}

	log.Printf("[webhook] subscription renewed: provider_sub_id=%s", subID)
	// TODO: 更新订阅周期、延长权益
	return nil
}
