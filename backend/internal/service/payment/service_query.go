package payment

import (
	"context"
	"errors"
	"fmt"
	"log"

	"interview-agents/internal/model"
	paymentpkg "interview-agents/internal/payment"

	"gorm.io/gorm"
)

// GetOrderByNo 查询订单详情
func (s *paymentService) GetOrderByNo(ctx context.Context, userID uint, orderNo string) (*OrderDetail, error) {
	order, err := model.PaymentOrderDao.FindByOrderNo(orderNo)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("order not found")
		}
		return nil, err
	}

	// 校验订单归属
	if order.UserID != userID {
		return nil, fmt.Errorf("order not found")
	}

	return toOrderDetail(order), nil
}

// ListOrders 查询用户订单列表
func (s *paymentService) ListOrders(ctx context.Context, userID uint, page, pageSize int) (*OrderListResponse, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 50 {
		pageSize = 50
	}

	orders, total, err := model.PaymentOrderDao.FindByUserID(userID, page, pageSize)
	if err != nil {
		return nil, err
	}

	details := make([]*OrderDetail, 0, len(orders))
	for _, o := range orders {
		details = append(details, toOrderDetail(o))
	}

	return &OrderListResponse{
		Total:  total,
		Orders: details,
	}, nil
}

// CancelSubscription 取消订阅
func (s *paymentService) CancelSubscription(ctx context.Context, userID uint, subscriptionNo string, immediate bool) error {
	sub, err := model.SubscriptionDao.FindBySubscriptionNo(subscriptionNo)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("subscription not found")
		}
		return err
	}

	if sub.UserID != userID {
		return fmt.Errorf("subscription not found")
	}

	if sub.Status != model.SubscriptionStatusActive && sub.Status != model.SubscriptionStatusCancelScheduled {
		log.Printf("[Payment] CancelSubscription invalid status: user_id=%d sub_no=%s current_status=%s", userID, subscriptionNo, sub.Status)
		return fmt.Errorf("subscription cannot be canceled in current status: %s", sub.Status)
	}

	// 调用渠道取消
	provider, err := paymentpkg.GetProviderByString(sub.Provider)
	if err != nil {
		return err
	}

	log.Printf("[Payment] CancelSubscription calling provider: user_id=%d sub_no=%s provider=%s provider_sub_id=%s immediate=%v", userID, subscriptionNo, sub.Provider, sub.ProviderSubID, immediate)

	if err := provider.CancelSubscription(ctx, sub.ProviderSubID, immediate); err != nil {
		log.Printf("[Payment] CancelSubscription provider call failed: user_id=%d sub_no=%s err=%v", userID, subscriptionNo, err)
		return fmt.Errorf("provider cancel subscription failed: %w", err)
	}

	// 更新本地状态
	newStatus := model.SubscriptionStatusCancelScheduled
	if immediate {
		newStatus = model.SubscriptionStatusCanceled
	}
	log.Printf("[Payment] CancelSubscription success: user_id=%d sub_no=%s new_status=%s", userID, subscriptionNo, newStatus)
	return model.SubscriptionDao.UpdateStatus(nil, subscriptionNo, newStatus, nil)
}

// GetActiveSubscription 查询用户当前生效订阅
func (s *paymentService) GetActiveSubscription(ctx context.Context, userID uint) (*SubscriptionDetail, error) {
	sub, err := model.SubscriptionDao.FindActiveByUserID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // 无生效订阅
		}
		return nil, err
	}

	return &SubscriptionDetail{
		SubscriptionNo:     sub.SubscriptionNo,
		ProductCode:        sub.ProductCode,
		PriceCode:          sub.PriceCode,
		Status:             sub.Status,
		Provider:           sub.Provider,
		CurrentPeriodStart: sub.CurrentPeriodStart,
		CurrentPeriodEnd:   sub.CurrentPeriodEnd,
		CancelAt:           sub.CancelAt,
		CreatedAt:          sub.CreatedAt,
	}, nil
}

// toOrderDetail 转换订单模型为详情
func toOrderDetail(o *model.PaymentOrder) *OrderDetail {
	return &OrderDetail{
		OrderNo:     o.OrderNo,
		OrderType:   o.OrderType,
		ProductCode: o.ProductCode,
		PriceCode:   o.PriceCode,
		Amount:      o.Amount,
		Currency:    o.Currency,
		Status:      o.Status,
		Provider:    o.Provider,
		PaidAt:      o.PaidAt,
		CreatedAt:   o.CreatedAt,
	}
}
