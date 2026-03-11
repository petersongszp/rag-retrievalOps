package circuitbreaker

import (
	"errors"
	"fmt"
	"time"

	"github.com/sony/gobreaker"
)

var (
	ErrOpen     = gobreaker.ErrOpenState
	ErrTooMany  = gobreaker.ErrTooManyRequests
	ErrHalfOpen = errors.New("circuit breaker is half-open")
)

// CircuitBreaker 简单的熔断器封装
type CircuitBreaker struct {
	cb *gobreaker.CircuitBreaker
}

// Settings 熔断器配置
type Settings struct {
	Name        string
	MaxRequests uint32
	Interval    time.Duration
	Timeout     time.Duration
}

// NewCircuitBreaker 创建一个新的熔断器
// name: 熔断器名称
func NewCircuitBreaker(name string) *CircuitBreaker {
	settings := gobreaker.Settings{
		Name:        name,
		MaxRequests: 3,                // 半开状态下允许的最大请求数
		Interval:    60 * time.Second, // 统计周期，在这个周期内失败次数达到阈值则触发熔断
		Timeout:     30 * time.Second, // 熔断后等待多久进入半开状态
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// 连续失败 5 次，则熔断
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 5 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			fmt.Printf("[CircuitBreaker] %s state changed from %s to %s\n", name, from, to)
		},
	}

	return &CircuitBreaker{
		cb: gobreaker.NewCircuitBreaker(settings),
	}
}

// Execute 执行受熔断器保护的函数
func (c *CircuitBreaker) Execute(req func() (interface{}, error)) (interface{}, error) {
	return c.cb.Execute(req)
}
