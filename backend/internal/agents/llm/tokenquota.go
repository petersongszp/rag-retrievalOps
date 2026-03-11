package llm

import (
	"context"
	"fmt"
	"time"

	"interview-agents/pkg/eino/callbacks"

	"github.com/redis/go-redis/v9"
)

const (
	tokenUsageKeyPrefix = "token_usage:user:"
	tokenUsageKeyTTL    = 48 * time.Hour // 保留 2 天便于统计
)

// tokenRecorder 11.2.3 推理成本控制：基于 Redis 的每用户每日 Token 消耗记录与配额检查（含 R1/o3 等 Reasoning Model）
type tokenRecorder struct {
	redis *redis.Client
	limit int // 每用户每日上限，0 表示不限制
}

// 确保实现接口
var _ callbacks.TokenRecorder = (*tokenRecorder)(nil)

// NewTokenRecorder 创建 Token 记录/配额器，limit 为每用户每日上限（0 表示不限制）
func NewTokenRecorder(redisClient *redis.Client, limit int) callbacks.TokenRecorder {
	if redisClient == nil {
		return nil
	}
	return &tokenRecorder{redis: redisClient, limit: limit}
}

func (r *tokenRecorder) key(userID uint) string {
	return tokenUsageKeyPrefix + fmt.Sprintf("%d", userID) + ":" + time.Now().Format("2006-01-02")
}

// Record 记录本次调用的 Token 消耗（OnEnd 时由 monitoring 回调）
func (r *tokenRecorder) Record(ctx context.Context, userID uint, promptTokens, completionTokens, totalTokens int64) {
	if r == nil || r.redis == nil || totalTokens <= 0 {
		return
	}
	k := r.key(userID)
	pipe := r.redis.Pipeline()
	pipe.IncrBy(ctx, k, totalTokens)
	pipe.Expire(ctx, k, tokenUsageKeyTTL)
	_, _ = pipe.Exec(ctx)
}

// CheckQuota 在发起推理前调用，若用户当日已超配额则返回 error（用于 Reasoning Model 等成本控制）
func (r *tokenRecorder) CheckQuota(ctx context.Context, userID uint) error {
	if r == nil || r.redis == nil || r.limit <= 0 {
		return nil
	}
	k := r.key(userID)
	n, err := r.redis.Get(ctx, k).Int64()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		return err
	}
	if n >= int64(r.limit) {
		return fmt.Errorf("当日 Token 配额已用尽（已用 %d，上限 %d），明日再试或联系管理员", n, r.limit)
	}
	return nil
}
