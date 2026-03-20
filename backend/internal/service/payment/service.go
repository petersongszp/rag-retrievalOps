package payment

import (
	"context"
	"errors"
	"fmt"
	"log"

	"interview-agents/internal/model"
	paymentpkg "interview-agents/internal/payment"
	"interview-agents/internal/payment/webhook"
	"interview-agents/internal/repository"

	"gorm.io/gorm"
)

type paymentService struct {
	webhookProcessor *webhook.Processor
}

func newPaymentService() *paymentService {
	return &paymentService{
		webhookProcessor: webhook.NewProcessor(repository.GetDB()),
	}
}

// CreateCheckout 创建一次性支付
func (s *paymentService) CreateCheckout(ctx context.Context, userID uint, req *CreateCheckoutRequest) (*CreateCheckoutResponse, error) {
	// 1. 幂等检查
	if req.IdempotencyKey != "" {
		existing, err := model.PaymentOrderDao.FindByIdempotencyKey(req.IdempotencyKey)
		if err == nil && existing != nil {
			log.Printf("[Payment] CreateCheckout idempotent hit: user_id=%d idempotency_key=%s existing_order=%s", userID, req.IdempotencyKey, existing.OrderNo)
			// 已存在，查找对应的 attempt 返回 checkout URL
			attempts, _ := model.PaymentAttemptDao.FindByOrderNo(existing.OrderNo)
			checkoutURL := ""
			providerID := ""
			if len(attempts) > 0 {
				checkoutURL = attempts[0].CheckoutURL
				providerID = attempts[0].ProviderID
			}
			return &CreateCheckoutResponse{
				OrderNo:     existing.OrderNo,
				CheckoutURL: checkoutURL,
				ProviderID:  providerID,
			}, nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[Payment] CreateCheckout idempotency check error: user_id=%d key=%s err=%v", userID, req.IdempotencyKey, err)
			return nil, fmt.Errorf("idempotency check failed: %w", err)
		}
	}

	// 2. 获取 provider
	provider, err := paymentpkg.GetProviderByString(req.Provider)
	if err != nil {
		return nil, err
	}

	// 3. 生成订单号
	orderNo := generateOrderNo()

	log.Printf("[Payment] CreateCheckout creating order: user_id=%d provider=%s product=%s amount=%d %s", userID, req.Provider, req.ProductCode, req.Amount, req.Currency)

	// 4. 在事务中创建订单和支付尝试
	var checkoutResult *paymentpkg.CheckoutResult
	var attemptNo string

	txErr := model.WithTransaction(ctx, func(tx *gorm.DB) error {
		// 创建订单
		order := &model.PaymentOrder{
			OrderNo:        orderNo,
			UserID:         userID,
			OrderType:      model.OrderTypeOneTime,
			ProductCode:    req.ProductCode,
			PriceCode:      req.PriceCode,
			Amount:         req.Amount,
			Currency:       req.Currency,
			Status:         model.OrderStatusCreated,
			Provider:       req.Provider,
			IdempotencyKey: req.IdempotencyKey,
			SuccessURL:     req.SuccessURL,
			CancelURL:      req.CancelURL,
		}
		if err := model.PaymentOrderDao.Create(tx, order); err != nil {
			return fmt.Errorf("create order failed: %w", err)
		}

		// 调用渠道创建 checkout
		checkoutResult, err = provider.CreateCheckout(ctx, &paymentpkg.CheckoutRequest{
			OrderNo:     orderNo,
			Amount:      req.Amount,
			Currency:    req.Currency,
			ProductName: req.ProductName,
			SuccessURL:  req.SuccessURL,
			CancelURL:   req.CancelURL,
			Metadata:    map[string]string{"order_no": orderNo},
		})
		if err != nil {
			return fmt.Errorf("provider create checkout failed: %w", err)
		}

		// 创建支付尝试
		attemptNo = generateAttemptNo()
		attempt := &model.PaymentAttempt{
			AttemptNo:   attemptNo,
			OrderNo:     orderNo,
			Provider:    req.Provider,
			ProviderID:  checkoutResult.ProviderID,
			Status:      model.AttemptStatusInitiated,
			CheckoutURL: checkoutResult.CheckoutURL,
			Amount:      req.Amount,
			Currency:    req.Currency,
		}
		if err := model.PaymentAttemptDao.Create(tx, attempt); err != nil {
			return fmt.Errorf("create attempt failed: %w", err)
		}

		// 更新订单状态
		return model.PaymentOrderDao.UpdateStatus(tx, orderNo, model.OrderStatusPendingPayment, nil)
	})

	if txErr != nil {
		log.Printf("[Payment] CreateCheckout transaction failed: user_id=%d order_no=%s err=%v", userID, orderNo, txErr)
		return nil, txErr
	}

	log.Printf("[Payment] CreateCheckout completed: user_id=%d order_no=%s provider_id=%s", userID, orderNo, checkoutResult.ProviderID)
	return &CreateCheckoutResponse{
		OrderNo:     orderNo,
		CheckoutURL: checkoutResult.CheckoutURL,
		ProviderID:  checkoutResult.ProviderID,
	}, nil
}

