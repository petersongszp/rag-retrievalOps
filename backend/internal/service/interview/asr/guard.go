package asr

import (
	"context"
	"errors"
	"fmt"

	"interview-agents/pkg/ratelimiter"

	"github.com/redis/go-redis/v9"
)

const (
	defaultUserRPM     = 6
	defaultProviderRPM = 120
)

type guard struct {
	cfg             ASRConfig
	userLimiter     *ratelimiter.RedisRateLimiter
	providerLimiter *ratelimiter.RedisRateLimiter
}

func newGuard(cfg ASRConfig, redisClient *redis.Client) Guard {
	userLimiter := ratelimiter.NewRedisRateLimiter(redisClient, ratelimiter.Config{
		Enabled:     true,
		DefaultRPM:  defaultUserRPM,
		DefaultTPM:  0,
		FailureMode: ratelimiter.FailureModeClosed,
	})

	providerLimiter := ratelimiter.NewRedisRateLimiter(redisClient, ratelimiter.Config{
		Enabled:     true,
		DefaultRPM:  defaultProviderRPM,
		DefaultTPM:  0,
		FailureMode: ratelimiter.FailureModeClosed,
	})

	return &guard{
		cfg:             cfg,
		userLimiter:     userLimiter,
		providerLimiter: providerLimiter,
	}
}

func (g *guard) CheckCapability(ctx context.Context) (*Capability, error) {
	_ = ctx
	return g.cfg.Capability(), nil
}

func (g *guard) AllowUser(ctx context.Context, userID uint, model string) error {
	if g.userLimiter == nil {
		return NewUnavailableError("", errors.New("user limiter is not initialized"))
	}

	subject := fmt.Sprintf("user:%d", userID)
	return g.mapLimiterError(g.userLimiter.AllowRequest(ctx, subject, "asr:"+model))
}

func (g *guard) AllowProvider(ctx context.Context, provider string, model string) error {
	if g.providerLimiter == nil {
		return NewUnavailableError("", errors.New("provider limiter is not initialized"))
	}

	subject := fmt.Sprintf("provider:%s", provider)
	return g.mapLimiterError(g.providerLimiter.AllowRequest(ctx, subject, "asr:"+model))
}

func (g *guard) mapLimiterError(err error) error {
	if err == nil {
		return nil
	}

	var rateErr *ratelimiter.RateLimitError
	if errors.As(err, &rateErr) {
		return NewRateLimitExceededError(rateErr.RetryAfterSeconds, err)
	}

	var depErr *ratelimiter.DependencyUnavailableError
	if errors.As(err, &depErr) {
		return NewUnavailableError("", err)
	}

	return NewUnavailableError("", err)
}
