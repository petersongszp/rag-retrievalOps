package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"golang.org/x/time/rate"
)

// IPRateLimiter IP 限流器
type IPRateLimiter struct {
	ips   sync.Map
	rate  rate.Limit
	burst int
	mu    sync.Mutex
	// 简单的清理机制
	lastSeen sync.Map
}

// NewIPRateLimiter 创建新的 IP 限流器
// r: 每秒允许的请求数 (RPS)
// b: 允许的突发请求数
func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	limiter := &IPRateLimiter{
		rate:  r,
		burst: b,
	}

	// 启动清理协程，防止内存泄漏
	go limiter.cleanupLoop()

	return limiter
}

// getLimiter 获取指定 IP 的限流器
func (i *IPRateLimiter) getLimiter(ip string) *rate.Limiter {
	// 更新最后访问时间
	i.lastSeen.Store(ip, time.Now())

	if v, ok := i.ips.Load(ip); ok {
		return v.(*rate.Limiter)
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	// 双重检查
	if v, ok := i.ips.Load(ip); ok {
		return v.(*rate.Limiter)
	}

	limiter := rate.NewLimiter(i.rate, i.burst)
	i.ips.Store(ip, limiter)
	return limiter
}

// cleanupLoop 定期清理过期的限流器
func (i *IPRateLimiter) cleanupLoop() {
	for {
		time.Sleep(1 * time.Minute)
		i.lastSeen.Range(func(key, value interface{}) bool {
			lastTime := value.(time.Time)
			// 如果超过 5 分钟没有访问，清理掉
			if time.Since(lastTime) > 5*time.Minute {
				i.ips.Delete(key)
				i.lastSeen.Delete(key)
			}
			return true
		})
	}
}

// Handler Hertz 中间件处理函数
func (i *IPRateLimiter) Handler() app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		ip := ctx.ClientIP()
		limiter := i.getLimiter(ip)

		if !limiter.Allow() {
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, map[string]string{
				"error":   "Too Many Requests",
				"message": "您的请求过于频繁，请稍后再试",
			})
			return
		}

		ctx.Next(c)
	}
}
