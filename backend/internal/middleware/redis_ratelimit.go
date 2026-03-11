package middleware

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/redis/go-redis/v9"
)

// RedisRateLimiter 基于 Redis 的分布式 IP 限流器（固定窗口，原子 INCR+EXPIRE）
// 适用于多实例部署时统一限流（11.1.3 分布式限流）
type RedisRateLimiter struct {
	client *redis.Client
	prefix string
	burst  int
}

// redisFixedWindowLua 原子：INCR key，首次则 EXPIRE 2s，返回当前计数
const redisFixedWindowLua = `
local k = KEYS[1]
local c = redis.call('INCR', k)
if c == 1 then
  redis.call('EXPIRE', k, 2)
end
return c
`

// NewRedisRateLimiter 创建 Redis 分布式限流器
// prefix: Redis 键前缀，如 "rl:"
// burst: 每窗口内允许的最大请求数（与配置的 rps 一致时即为每秒请求数）
func NewRedisRateLimiter(client *redis.Client, prefix string, burst int) *RedisRateLimiter {
	if client == nil {
		panic("RedisRateLimiter: redis client is nil")
	}
	if prefix == "" {
		prefix = "rl:"
	}
	return &RedisRateLimiter{
		client: client,
		prefix: prefix,
		burst:  burst,
	}
}

// Handler 返回 Hertz 限流中间件
func (r *RedisRateLimiter) Handler() app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		ip := ctx.ClientIP()
		windowKey := strconv.FormatInt(time.Now().Unix(), 10)
		key := r.prefix + ip + ":" + windowKey

		n, err := r.client.Eval(c, redisFixedWindowLua, []string{key}).Int()
		if err != nil {
			// Redis 不可用时放行并记录，避免雪崩
			ctx.Next(c)
			return
		}

		if n > r.burst {
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, map[string]string{
				"error":   "Too Many Requests",
				"message": "您的请求过于频繁，请稍后再试",
			})
			return
		}
		ctx.Next(c)
	}
}
