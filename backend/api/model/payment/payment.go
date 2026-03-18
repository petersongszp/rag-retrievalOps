package payment

// CreateCheckoutRequest 创建一次性支付请求 DTO
type CreateCheckoutRequest struct {
	ProductCode string `json:"product_code" vd:"len($)>0"`
	PriceCode   string `json:"price_code" vd:"len($)>0"`
	Provider    string `json:"provider" vd:"len($)>0"` // stripe / paypal
	Amount      int64  `json:"amount" vd:"$>0"`        // 最小货币单位
	Currency    string `json:"currency" vd:"len($)>0"` // usd
	ProductName string `json:"product_name" vd:"len($)>0"`
	//SuccessURL     string `json:"success_url" vd:"len($)>0"`
	//CancelURL      string `json:"cancel_url" vd:"len($)>0"`
	//IdempotencyKey string `json:"idempotency_key" vd:"len($)>0"`
}

// CreateSubscriptionRequest 创建订阅请求 DTO
type CreateSubscriptionRequest struct {
	ProductCode    string `json:"product_code" vd:"len($)>0"`
	PriceCode      string `json:"price_code" vd:"len($)>0"`
	Provider       string `json:"provider" vd:"len($)>0"`
	PriceID        string `json:"price_id" vd:"len($)>0"` // 渠道侧价格 ID
	SuccessURL     string `json:"success_url" vd:"len($)>0"`
	CancelURL      string `json:"cancel_url" vd:"len($)>0"`
	IdempotencyKey string `json:"idempotency_key" vd:"len($)>0"`
}

// CancelSubscriptionRequest 取消订阅请求 DTO
type CancelSubscriptionRequest struct {
	SubscriptionNo string `json:"subscription_no" vd:"len($)>0"`
	Immediate      bool   `json:"immediate"` // true=立即取消, false=周期结束取消
}

// OrderQueryRequest 订单查询请求 DTO
type OrderQueryRequest struct {
	OrderNo string `json:"order_no" vd:"len($)>0"`
}

// OrderListRequest 订单列表请求 DTO
type OrderListRequest struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}
