package ratelimiter

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// ---- Lua Scripts ----

// rpmCheckScript atomically checks and records a request in a sliding window for RPM.
// Uses timestamp as score so ZREMRANGEBYSCORE can clean expired entries.
// KEYS[1] = sorted set key (e.g. ratelimit:{hash}:{model}:rpm)
// ARGV[1] = current timestamp in microseconds
// ARGV[2] = window start timestamp in microseconds
// ARGV[3] = RPM limit
// ARGV[4] = unique member ID
// ARGV[5] = TTL in seconds
//
// Returns: {currentCount, allowed(1/0)}
var rpmCheckScript = redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window_start = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local member = ARGV[4]
local ttl = tonumber(ARGV[5])

-- Remove entries outside the window
redis.call('ZREMRANGEBYSCORE', key, '-inf', window_start)

-- Count current requests in the window
local current = redis.call('ZCARD', key)

-- Check limit
if current >= limit then
    redis.call('EXPIRE', key, ttl)
    return {current, 0}
end

-- Add new entry with timestamp as score
redis.call('ZADD', key, now, member)
redis.call('EXPIRE', key, ttl)

return {current + 1, 1}
`)

// tpmCheckScript atomically checks and pre-allocates tokens in a sliding window for TPM.
// Each member stores "tokens:{member_id}" to encode the token count in the member name.
// Score is the timestamp for window management.
// We parse member names to sum token usage.
// KEYS[1] = sorted set key (e.g. ratelimit:{hash}:{model}:tpm)
// ARGV[1] = current timestamp in microseconds
// ARGV[2] = window start timestamp in microseconds
// ARGV[3] = TPM limit
// ARGV[4] = unique member (format: "tokens:{id}")
// ARGV[5] = token count for this request
// ARGV[6] = TTL in seconds
//
// Returns: {currentTotal, allowed(1/0)}
var tpmCheckScript = redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window_start = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local member = ARGV[4]
local tokens = tonumber(ARGV[5])
local ttl = tonumber(ARGV[6])

-- Remove entries outside the window
redis.call('ZREMRANGEBYSCORE', key, '-inf', window_start)

-- Sum current token usage by parsing member names
-- Members are stored as "{tokenCount}:{uniqueId}"
local members = redis.call('ZRANGE', key, 0, -1)
local current = 0
for i = 1, #members do
    local sep = string.find(members[i], ':', 1, true)
    if sep then
        current = current + tonumber(string.sub(members[i], 1, sep - 1))
    end
end

-- Check limit
if current + tokens > limit then
    redis.call('EXPIRE', key, ttl)
    return {current, 0}
end

-- Add new entry: member = "{tokens}:{id}", score = timestamp
redis.call('ZADD', key, now, member)
redis.call('EXPIRE', key, ttl)

return {current + tokens, 1}
`)

// tpmAdjustScript adjusts a previously pre-allocated token entry with actual usage.
// KEYS[1] = sorted set key (tpm key)
// ARGV[1] = old member to remove
// ARGV[2] = new member (with corrected token count in name)
// ARGV[3] = timestamp score for new member
//
// Returns: 1 on success
var tpmAdjustScript = redis.NewScript(`
local key = KEYS[1]
local old_member = ARGV[1]
local new_member = ARGV[2]
local score = tonumber(ARGV[3])

redis.call('ZREM', key, old_member)
redis.call('ZADD', key, score, new_member)

return 1
`)

// tpmGetUsageScript gets the current TPM usage within the sliding window.
// KEYS[1] = sorted set key
// ARGV[1] = window start timestamp in microseconds
//
// Returns: total token count
var tpmGetUsageScript = redis.NewScript(`
local key = KEYS[1]
local window_start = tonumber(ARGV[1])

redis.call('ZREMRANGEBYSCORE', key, '-inf', window_start)

local members = redis.call('ZRANGE', key, 0, -1)
local total = 0
for i = 1, #members do
    local sep = string.find(members[i], ':', 1, true)
    if sep then
        total = total + tonumber(string.sub(members[i], 1, sep - 1))
    end
end

return total
`)

// rpmGetUsageScript gets the current RPM usage within the sliding window.
// KEYS[1] = sorted set key
// ARGV[1] = window start timestamp in microseconds
//
// Returns: request count
var rpmGetUsageScript = redis.NewScript(`
local key = KEYS[1]
local window_start = tonumber(ARGV[1])

redis.call('ZREMRANGEBYSCORE', key, '-inf', window_start)
return redis.call('ZCARD', key)
`)

// ---- Config Types ----

// ModelRateConfig defines rate limits for a specific model.
type ModelRateConfig struct {
	RPM int `yaml:"rpm" json:"rpm"` // Requests per minute
	TPM int `yaml:"tpm" json:"tpm"` // Tokens per minute
}

// Config holds the overall rate limiter configuration.
type Config struct {
	Enabled    bool                       `yaml:"enabled" json:"enabled"`
	DefaultRPM int                        `yaml:"default_rpm" json:"default_rpm"`
	DefaultTPM int                        `yaml:"default_tpm" json:"default_tpm"`
	Models     map[string]ModelRateConfig `yaml:"models" json:"models"`
}

// ---- Error Types ----

// RateLimitError contains detailed information about a rate limit violation.
type RateLimitError struct {
	Message           string `json:"message"`
	RetryAfterSeconds int    `json:"retry_after_seconds"`
	CurrentRPM        int64  `json:"current_rpm,omitempty"`
	LimitRPM          int    `json:"limit_rpm,omitempty"`
	CurrentTPM        int64  `json:"current_tpm,omitempty"`
	LimitTPM          int    `json:"limit_tpm,omitempty"`
	LimitType         string `json:"limit_type"` // "rpm" or "tpm"
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded (%s): %s (retry after %ds)",
		e.LimitType, e.Message, e.RetryAfterSeconds)
}

// ---- Rate Limiter ----

// RedisRateLimiter implements distributed rate limiting using Redis sliding windows.
type RedisRateLimiter struct {
	client *redis.Client
	config Config
	mu     sync.RWMutex
	// counter for generating unique member IDs within the same microsecond
	counter uint64
}

// NewRedisRateLimiter creates a new Redis-based rate limiter.
func NewRedisRateLimiter(client *redis.Client, cfg Config) *RedisRateLimiter {
	return &RedisRateLimiter{
		client: client,
		config: cfg,
	}
}

// hashAPIKey produces a short, safe hash from the API key for use in Redis keys.
func hashAPIKey(apiKey string) string {
	h := sha256.Sum256([]byte(apiKey))
	return fmt.Sprintf("%x", h[:8]) // 16 hex chars
}

// getModelConfig returns the rate config for a given model, falling back to defaults.
func (r *RedisRateLimiter) getModelConfig(model string) ModelRateConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if cfg, ok := r.config.Models[model]; ok {
		return ModelRateConfig{
			RPM: orDefault(cfg.RPM, r.config.DefaultRPM),
			TPM: orDefault(cfg.TPM, r.config.DefaultTPM),
		}
	}
	return ModelRateConfig{
		RPM: r.config.DefaultRPM,
		TPM: r.config.DefaultTPM,
	}
}

func orDefault(val, def int) int {
	if val > 0 {
		return val
	}
	return def
}

// buildKey constructs the Redis key for a given apiKey, model, and metric type.
func buildKey(apiKey, model, metricType string) string {
	return fmt.Sprintf("ratelimit:%s:%s:%s", hashAPIKey(apiKey), model, metricType)
}

// generateMember creates a unique member ID for sorted set entries.
func (r *RedisRateLimiter) generateMember(prefix string) string {
	r.mu.Lock()
	r.counter++
	c := r.counter
	r.mu.Unlock()
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), c)
}

// AllowRequest checks if a request is allowed under RPM limits.
// Returns nil if allowed, *RateLimitError if rate limited.
func (r *RedisRateLimiter) AllowRequest(ctx context.Context, apiKey, model string) error {
	if !r.config.Enabled {
		return nil
	}

	cfg := r.getModelConfig(model)
	if cfg.RPM <= 0 {
		return nil // No RPM limit configured
	}

	key := buildKey(apiKey, model, "rpm")
	now := time.Now().UnixMicro()
	windowStart := now - 60*1e6 // 1 minute window in microseconds
	member := r.generateMember("req")

	result, err := rpmCheckScript.Run(ctx, r.client, []string{key},
		now, windowStart, cfg.RPM, member, 120, // TTL = 120s (2x window for safety)
	).Int64Slice()

	if err != nil {
		// On Redis error, fail open (allow the request) to avoid blocking all traffic
		fmt.Printf("[RateLimiter] Redis error on RPM check (failing open): %v\n", err)
		return nil
	}

	currentCount := result[0]
	allowed := result[1]

	if allowed == 0 {
		return &RateLimitError{
			Message:           fmt.Sprintf("RPM limit exceeded for model %s", model),
			RetryAfterSeconds: r.estimateRetryAfter(ctx, key),
			CurrentRPM:        currentCount,
			LimitRPM:          cfg.RPM,
			LimitType:         "rpm",
		}
	}

	return nil
}

// PreAllocateTokens pre-allocates estimated tokens against the TPM limit.
// Returns the member ID used for pre-allocation (needed for AdjustTokens).
func (r *RedisRateLimiter) PreAllocateTokens(ctx context.Context, apiKey, model string, estimatedTokens int) (string, error) {
	if !r.config.Enabled || estimatedTokens <= 0 {
		return "", nil
	}

	cfg := r.getModelConfig(model)
	if cfg.TPM <= 0 {
		return "", nil // No TPM limit configured
	}

	key := buildKey(apiKey, model, "tpm")
	now := time.Now().UnixMicro()
	windowStart := now - 60*1e6
	uid := r.generateMember("tok")
	// Member format: "{tokens}:{uniqueId}" — token count encoded in the member name
	member := fmt.Sprintf("%d:%s", estimatedTokens, uid)

	result, err := tpmCheckScript.Run(ctx, r.client, []string{key},
		now, windowStart, cfg.TPM, member, estimatedTokens, 120,
	).Int64Slice()

	if err != nil {
		fmt.Printf("[RateLimiter] Redis error on TPM pre-allocate (failing open): %v\n", err)
		return member, nil
	}

	currentCount := result[0]
	allowed := result[1]

	if allowed == 0 {
		return "", &RateLimitError{
			Message:           fmt.Sprintf("TPM limit exceeded for model %s", model),
			RetryAfterSeconds: r.estimateRetryAfter(ctx, key),
			CurrentTPM:        currentCount,
			LimitTPM:          cfg.TPM,
			LimitType:         "tpm",
		}
	}

	return member, nil
}

// tpmMemberWithTokens creates a new member name with updated token count.
func tpmMemberWithTokens(oldMember string, newTokens int) string {
	// Old member format: "{oldTokens}:{uniqueId}"
	// Find first colon and replace the prefix
	for i, c := range oldMember {
		if c == ':' {
			return fmt.Sprintf("%d%s", newTokens, oldMember[i:])
		}
	}
	return fmt.Sprintf("%d:%s", newTokens, oldMember)
}

// AdjustTokens corrects a previously pre-allocated token entry with the actual usage.
func (r *RedisRateLimiter) AdjustTokens(ctx context.Context, apiKey, model string, preAllocMember string, actualTokens int) {
	if !r.config.Enabled || preAllocMember == "" {
		return
	}

	key := buildKey(apiKey, model, "tpm")
	newMember := tpmMemberWithTokens(preAllocMember, actualTokens)
	now := time.Now().UnixMicro()

	_, err := tpmAdjustScript.Run(ctx, r.client, []string{key},
		preAllocMember, newMember, now,
	).Result()

	if err != nil {
		fmt.Printf("[RateLimiter] Redis error on TPM adjust (non-critical): %v\n", err)
	}
}

// GetUsage returns the current RPM and TPM usage for a given apiKey and model.
func (r *RedisRateLimiter) GetUsage(ctx context.Context, apiKey, model string) (rpm int64, tpm int64, err error) {
	rpmKey := buildKey(apiKey, model, "rpm")
	tpmKey := buildKey(apiKey, model, "tpm")
	windowStart := time.Now().UnixMicro() - 60*1e6

	rpmResult, err := rpmGetUsageScript.Run(ctx, r.client, []string{rpmKey}, windowStart).Int64()
	if err != nil && err != redis.Nil {
		return 0, 0, fmt.Errorf("failed to get RPM usage: %w", err)
	}

	tpmResult, rErr := tpmGetUsageScript.Run(ctx, r.client, []string{tpmKey}, windowStart).Int64()
	if rErr != nil && rErr != redis.Nil {
		return 0, 0, fmt.Errorf("failed to get TPM usage: %w", rErr)
	}

	return rpmResult, tpmResult, nil
}

// GetLimits returns the configured limits for a given model.
func (r *RedisRateLimiter) GetLimits(model string) ModelRateConfig {
	return r.getModelConfig(model)
}

// estimateRetryAfter estimates how many seconds until the oldest entry expires from the window.
func (r *RedisRateLimiter) estimateRetryAfter(ctx context.Context, key string) int {
	// Get the oldest entry's score (timestamp) in the sorted set
	members, err := r.client.ZRangeWithScores(ctx, key, 0, 0).Result()
	if err != nil || len(members) == 0 {
		return 10 // Default: suggest retry after 10 seconds
	}

	oldestTimestamp := int64(members[0].Score)
	// The oldest entry will expire at oldestTimestamp + 60s window
	expiresAt := oldestTimestamp + 60*1e6 // in microseconds
	now := time.Now().UnixMicro()
	retryAfter := (expiresAt - now) / 1e6 // convert to seconds

	if retryAfter <= 0 {
		return 1
	}
	if retryAfter > 60 {
		return 60
	}
	return int(retryAfter)
}
