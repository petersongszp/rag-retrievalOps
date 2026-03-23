package payment

import (
	"context"
	"log"

	paymentDTO "interview-agents/api/model/payment"
	"interview-agents/api/response"
	"interview-agents/internal/middleware"
	paymentService "interview-agents/internal/service/payment"

	"github.com/cloudwego/hertz/pkg/app"
)

// GetOrder 查询订单详情
// @router /api/payment/order/query [POST]
func GetOrder(ctx context.Context, c *app.RequestContext) {
	var req paymentDTO.OrderQueryRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.BadRequest(ctx, c, "Invalid request: "+err.Error())
		return
	}

	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(ctx, c, "Unauthorized")
		return
	}

	svc := paymentService.NewManager()
	result, err := svc.GetOrderByNo(ctx, userID, req.OrderNo)
	if err != nil {
		log.Printf("[Payment] GetOrder failed: user_id=%d order_no=%s err=%v", userID, req.OrderNo, err)
		response.InternalServerError(ctx, c, err.Error())
		return
	}

	response.Success(ctx, c, result)
}

// ListOrders 查询用户订单列表
// @router /api/payment/order/list [POST]
func ListOrders(ctx context.Context, c *app.RequestContext) {
	var req paymentDTO.OrderListRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.BadRequest(ctx, c, "Invalid request: "+err.Error())
		return
	}

	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(ctx, c, "Unauthorized")
		return
	}

	svc := paymentService.NewManager()
	result, err := svc.ListOrders(ctx, userID, req.Page, req.PageSize)
	if err != nil {
		log.Printf("[Payment] ListOrders failed: user_id=%d err=%v", userID, err)
		response.InternalServerError(ctx, c, err.Error())
		return
	}

	response.Success(ctx, c, result)
}

// CancelSubscription 取消订阅
// @router /api/payment/subscription/cancel [POST]
func CancelSubscription(ctx context.Context, c *app.RequestContext) {
	var req paymentDTO.CancelSubscriptionRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.BadRequest(ctx, c, "Invalid request: "+err.Error())
		return
	}

	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(ctx, c, "Unauthorized")
		return
	}

	log.Printf("[Payment] CancelSubscription start: user_id=%d sub_no=%s immediate=%v", userID, req.SubscriptionNo, req.Immediate)

	svc := paymentService.NewManager()
	if err := svc.CancelSubscription(ctx, userID, req.SubscriptionNo, req.Immediate); err != nil {
		log.Printf("[Payment] CancelSubscription failed: user_id=%d sub_no=%s err=%v", userID, req.SubscriptionNo, err)
		response.InternalServerError(ctx, c, err.Error())
		return
	}

	log.Printf("[Payment] CancelSubscription success: user_id=%d sub_no=%s", userID, req.SubscriptionNo)
	response.Success(ctx, c, nil)
}

// GetActiveSubscription 查询当前生效订阅
// @router /api/payment/subscription/active [POST]
func GetActiveSubscription(ctx context.Context, c *app.RequestContext) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(ctx, c, "Unauthorized")
		return
	}

	svc := paymentService.NewManager()
	result, err := svc.GetActiveSubscription(ctx, userID)
	if err != nil {
		log.Printf("[Payment] GetActiveSubscription failed: user_id=%d err=%v", userID, err)
		response.InternalServerError(ctx, c, err.Error())
		return
	}

	response.Success(ctx, c, result)
}
