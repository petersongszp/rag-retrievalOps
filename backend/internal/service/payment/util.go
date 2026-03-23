package payment

import (
	"fmt"
	"math/rand"
	"time"
)

// generateOrderNo 生成订单号: PAY + 时间戳 + 随机数
func generateOrderNo() string {
	return fmt.Sprintf("PAY%s%04d", time.Now().Format("20060102150405"), rand.Intn(10000))
}

// generateAttemptNo 生成支付尝试编号
func generateAttemptNo() string {
	return fmt.Sprintf("ATT%s%04d", time.Now().Format("20060102150405"), rand.Intn(10000))
}

// generateSubscriptionNo 生成订阅编号
func generateSubscriptionNo() string {
	return fmt.Sprintf("SUB%s%04d", time.Now().Format("20060102150405"), rand.Intn(10000))
}
