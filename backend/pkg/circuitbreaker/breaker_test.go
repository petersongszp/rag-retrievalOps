package circuitbreaker

import (
	"errors"
	"testing"
)

func TestCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker("test-breaker")

	// 1. 正常请求
	_, err := cb.Execute(func() (interface{}, error) {
		return "success", nil
	})
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// 2. 模拟失败，触发熔断
	// 我们设置了连续 5 次失败触发熔断
	for i := 0; i < 5; i++ {
		cb.Execute(func() (interface{}, error) {
			return nil, errors.New("simulated error")
		})
	}

	// 3. 此时应该熔断开启
	_, err = cb.Execute(func() (interface{}, error) {
		return "success", nil
	})
	if err != ErrOpen {
		t.Errorf("Expected ErrOpen, got %v", err)
	} else {
		t.Log("Circuit breaker successfully opened")
	}

	// 注意：由于 gbr.Interval 设置为 60s，Timeout 为 30s，这里很难在单元测试中等待半开状态。
	// 但我们可以确认它确实 Open 了。
}
