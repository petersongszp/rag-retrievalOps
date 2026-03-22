package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"interview-agents/internal/model"
	paymentpkg "interview-agents/internal/payment"
	"interview-agents/internal/repository"

	"gorm.io/gorm"
)

// CreateSubscriptionCheckout 创建订阅支付
func (s *paymentService) CreateSubscriptionCheckout(ctx context.Context, userID uint, req *CreateSubscriptionRequest) (*CreateSubscriptionResponse, error) {
	// 幂等检查
	if req.IdempotencyKey != "" {
		existing, err := model.PaymentOrderDao.FindByIdempotencyKey(req.IdempotencyKey)
		if err == nil && existing != nil {
			log.Printf("[Payment] CreateSubscription idempotent hit: user_id=%d idempotency_key=%s existing_order=%s", userID, req.IdempotencyKey, existing.OrderNo)
			subs, _ := model.SubscriptionDao.FindByUserID(userID)
			subNo := ""
			if len(subs) > 0 {
				subNo = subs[0].SubscriptionNo
			}
			attempts, _ := model.PaymentAttemptDao.FindByOrderNo(existing.OrderNo)
			checkoutURL := ""
			if len(attempts) > 0 {
				checkoutURL = attempts[0].CheckoutURL
			}
			return &CreateSubscriptionResponse{
				OrderNo:        existing.OrderNo,
				SubscriptionNo: subNo,
				CheckoutURL:    checkoutURL,
			}, nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[Payment] CreateSubscription idempotency check error: user_id=%d key=%s err=%v", userID, req.IdempotencyKey, err)
			return nil, fmt.Errorf("idempotency check failed: %w", err)
		}
	}

	provider, err := paymentpkg.GetProviderByString(req.Provider)
	if err != nil {
		log.Printf("[Payment] CreateSubscription unknown provider: user_id=%d provider=%s", userID, req.Provider)
		return nil, err
	}

	orderNo := generateOrderNo()
	subscriptionNo := generateSubscriptionNo()
	log.Printf("[Payment] CreateSubscription creating: user_id=%d provider=%s product=%s order_no=%s sub_no=%s", userID, req.Provider, req.ProductCode, orderNo, subscriptionNo)

	var subResult *paymentpkg.SubscriptionResult

	txErr := model.WithTransaction(ctx, func(tx *gorm.DB) error {
		order := &model.PaymentOrder{
			OrderNo:        orderNo,
			UserID:         userID,
			OrderType:      model.OrderTypeSubscription,
			ProductCode:    req.ProductCode,
			PriceCode:      req.PriceCode,
			Amount:         0, // 订阅金额由渠道管理
			Currency:       "usd",
			Status:         model.OrderStatusCreated,
			Provider:       req.Provider,
			IdempotencyKey: req.IdempotencyKey,
			SuccessURL:     req.SuccessURL,
			CancelURL:      req.CancelURL,
		}
		if err := model.PaymentOrderDao.Create(tx, order); err != nil {
			return fmt.Errorf("create order failed: %w", err)
		}

		subResult, err = provider.CreateSubscription(ctx, &paymentpkg.SubscriptionRequest{
			OrderNo:    orderNo,
			PriceID:    req.PriceID,
			SuccessURL: req.SuccessURL,
			CancelURL:  req.CancelURL,
			Metadata:   map[string]string{"order_no": orderNo},
		})
		if err != nil {
			return fmt.Errorf("provider create subscription failed: %w", err)
		}

		sub := &model.Subscription{
			SubscriptionNo: subscriptionNo,
			UserID:         userID,
			OrderNo:        orderNo,
			Provider:       req.Provider,
			ProviderSubID:  subResult.ProviderSubID,
			ProductCode:    req.ProductCode,
			PriceCode:      req.PriceCode,
			Status:         model.SubscriptionStatusInit,
		}
		if err := model.SubscriptionDao.Create(tx, sub); err != nil {
			return fmt.Errorf("create subscription failed: %w", err)
		}

		attempt := &model.PaymentAttempt{
			AttemptNo:   generateAttemptNo(),
			OrderNo:     orderNo,
			Provider:    req.Provider,
			ProviderID:  subResult.ProviderID,
			Status:      model.AttemptStatusInitiated,
			CheckoutURL: subResult.CheckoutURL,
			Amount:      0,
			Currency:    "usd",
		}
		if err := model.PaymentAttemptDao.Create(tx, attempt); err != nil {
			return fmt.Errorf("create attempt failed: %w", err)
		}

		return model.PaymentOrderDao.UpdateStatus(tx, orderNo, model.OrderStatusPendingPayment, nil)
	})

	if txErr != nil {
		log.Printf("[Payment] CreateSubscription transaction failed: user_id=%d order_no=%s err=%v", userID, orderNo, txErr)
		return nil, txErr
	}

	log.Printf("[Payment] CreateSubscription completed: user_id=%d order_no=%s sub_no=%s", userID, orderNo, subscriptionNo)
	return &CreateSubscriptionResponse{
		OrderNo:        orderNo,
		SubscriptionNo: subscriptionNo,
		CheckoutURL:    subResult.CheckoutURL,
	}, nil
}

// HandleWebhook 处理 webhook 回调
func (s *paymentService) HandleWebhook(ctx context.Context, providerStr string, payload *paymentpkg.WebhookPayload) error {
	startTime := time.Now()
	db := repository.GetDB()

	// 1. 先将原始回调记录落库（无论后续验签是否成功）
	headersJSON, _ := json.Marshal(payload.Headers)
	callbackRecord := &model.PaymentCallback{
		Provider:   providerStr,
		RawHeaders: string(headersJSON),
		RawBody:    string(payload.Body),
		Status:     model.CallbackStatusReceived,
	}
	if err := model.PaymentCallbackDao.Create(db, callbackRecord); err != nil {
		log.Printf("[Payment] Webhook callback record save failed: provider=%s err=%v", providerStr, err)
		// 记录失败不阻塞主流程，仅打日志
	}

	// 2. 获取支付渠道
	provider, err := paymentpkg.GetProviderByString(providerStr)
	if err != nil {
		log.Printf("[Payment] Webhook unknown provider: %s", providerStr)
		if callbackRecord.ID > 0 {
			elapsed := time.Since(startTime).Milliseconds()
			_ = model.PaymentCallbackDao.UpdateStatus(db, callbackRecord.ID, model.CallbackStatusFailed, fmt.Sprintf("unknown provider: %s", providerStr), elapsed)
		}
		return fmt.Errorf("unknown provider: %s", providerStr)
	}

	// 3. 验签 + 解析
	event, err := provider.VerifyWebhook(ctx, payload)
	if err != nil {
		log.Printf("[Payment] Webhook verification failed: provider=%s err=%v", providerStr, err)
		if callbackRecord.ID > 0 {
			elapsed := time.Since(startTime).Milliseconds()
			_ = model.PaymentCallbackDao.UpdateStatus(db, callbackRecord.ID, model.CallbackStatusFailed, fmt.Sprintf("verification failed: %v", err), elapsed)
		}
		return fmt.Errorf("webhook verification failed: %w", err)
	}

	// 4. 验签成功，更新回调记录的事件信息
	log.Printf("[Payment] Webhook verified: provider=%s event_type=%s event_id=%s raw_type=%s", providerStr, event.EventType, event.EventID, event.RawType)
	if callbackRecord.ID > 0 {
		_ = model.PaymentCallbackDao.UpdateEventInfo(db, callbackRecord.ID, event.EventID, event.EventType)
		_ = model.PaymentCallbackDao.UpdateStatus(db, callbackRecord.ID, model.CallbackStatusVerified, "", 0)
	}

	// 5. 去重 + 落库 + 业务处理
	if err := s.webhookProcessor.Process(ctx, paymentpkg.ProviderName(providerStr), event); err != nil {
		log.Printf("[Payment] Webhook processing error: provider=%s event_id=%s err=%v", providerStr, event.EventID, err)
		if callbackRecord.ID > 0 {
			elapsed := time.Since(startTime).Milliseconds()
			_ = model.PaymentCallbackDao.UpdateStatus(db, callbackRecord.ID, model.CallbackStatusFailed, fmt.Sprintf("process error: %v", err), elapsed)
		}
		return err
	}

	// 6. 处理完成，更新回调记录状态
	log.Printf("[Payment] Webhook processed: provider=%s event_id=%s event_type=%s", providerStr, event.EventID, event.EventType)
	if callbackRecord.ID > 0 {
		elapsed := time.Since(startTime).Milliseconds()
		_ = model.PaymentCallbackDao.UpdateStatus(db, callbackRecord.ID, model.CallbackStatusProcessed, "ok", elapsed)
	}
	return nil
}
