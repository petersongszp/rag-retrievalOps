package ratelimiter

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	return client, mr
}

func TestAllowRequest_RPM_Basic(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	rl := NewRedisRateLimiter(client, Config{
		Enabled:    true,
		DefaultRPM: 3,
		DefaultTPM: 100000,
	})

	ctx := context.Background()
	apiKey := "sk-test-key-12345"
	model := "gpt-4"

	// First 3 requests should succeed
	for i := 0; i < 3; i++ {
		err := rl.AllowRequest(ctx, apiKey, model)
		if err != nil {
			t.Fatalf("request %d should be allowed, got: %v", i+1, err)
		}
	}

	// 4th request should be denied
	err := rl.AllowRequest(ctx, apiKey, model)
	if err == nil {
		t.Fatal("4th request should be denied, but was allowed")
	}

	rateLimitErr, ok := err.(*RateLimitError)
	if !ok {
		t.Fatalf("expected *RateLimitError, got %T", err)
	}
	if rateLimitErr.LimitType != "rpm" {
		t.Errorf("expected limit type 'rpm', got %q", rateLimitErr.LimitType)
	}
	if rateLimitErr.LimitRPM != 3 {
		t.Errorf("expected limit RPM 3, got %d", rateLimitErr.LimitRPM)
	}
	if rateLimitErr.RetryAfterSeconds <= 0 {
		t.Errorf("expected positive retry_after_seconds, got %d", rateLimitErr.RetryAfterSeconds)
	}
	t.Logf("Rate limit error: %v", rateLimitErr)
}

func TestAllowRequest_RPM_WindowExpiry(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	rl := NewRedisRateLimiter(client, Config{
		Enabled:    true,
		DefaultRPM: 2,
		DefaultTPM: 100000,
	})

	ctx := context.Background()
	apiKey := "sk-test-key"
	model := "gpt-4"

	// Fill the limit
	for i := 0; i < 2; i++ {
		if err := rl.AllowRequest(ctx, apiKey, model); err != nil {
			t.Fatalf("request %d should be allowed: %v", i+1, err)
		}
	}

	// Should be denied
	if err := rl.AllowRequest(ctx, apiKey, model); err == nil {
		t.Fatal("should be denied after limit reached")
	}

	// Simulate window expiry: remove all members from the sorted set
	// In production, ZREMRANGEBYSCORE handles this automatically as time passes.
	// miniredis.FastForward only advances TTL, not our Go time.Now() used for window calc.
	key := buildKey(apiKey, model, "rpm")
	client.Del(ctx, key)

	// Should be allowed again after "window expires"
	if err := rl.AllowRequest(ctx, apiKey, model); err != nil {
		t.Fatalf("should be allowed after window expires: %v", err)
	}
}

func TestPreAllocateTokens_TPM_Basic(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	rl := NewRedisRateLimiter(client, Config{
		Enabled:    true,
		DefaultRPM: 100,
		DefaultTPM: 1000, // 1000 tokens per minute
	})

	ctx := context.Background()
	apiKey := "sk-test-key"
	model := "gpt-4"

	// Pre-allocate 400 tokens - should succeed
	member1, err := rl.PreAllocateTokens(ctx, apiKey, model, 400)
	if err != nil {
		t.Fatalf("first pre-allocate should succeed: %v", err)
	}
	if member1 == "" {
		t.Fatal("member should not be empty")
	}

	// Pre-allocate another 400 tokens - should succeed (total 800)
	_, err = rl.PreAllocateTokens(ctx, apiKey, model, 400)
	if err != nil {
		t.Fatalf("second pre-allocate should succeed: %v", err)
	}

	// Pre-allocate 300 more tokens - should fail (would be 1100 > 1000)
	_, err = rl.PreAllocateTokens(ctx, apiKey, model, 300)
	if err == nil {
		t.Fatal("third pre-allocate should be denied (would exceed 1000)")
	}

	rateLimitErr, ok := err.(*RateLimitError)
	if !ok {
		t.Fatalf("expected *RateLimitError, got %T", err)
	}
	if rateLimitErr.LimitType != "tpm" {
		t.Errorf("expected limit type 'tpm', got %q", rateLimitErr.LimitType)
	}
}

func TestAdjustTokens(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	rl := NewRedisRateLimiter(client, Config{
		Enabled:    true,
		DefaultRPM: 100,
		DefaultTPM: 1000,
	})

	ctx := context.Background()
	apiKey := "sk-test-key"
	model := "gpt-4"

	// Pre-allocate 500 tokens
	member, err := rl.PreAllocateTokens(ctx, apiKey, model, 500)
	if err != nil {
		t.Fatalf("pre-allocate should succeed: %v", err)
	}

	// Adjust to actual 200 tokens (lower than estimated)
	rl.AdjustTokens(ctx, apiKey, model, member, 200)

	// Now we should have ~200 tokens used. Pre-allocating 700 more should succeed (total ~900 < 1000)
	_, err = rl.PreAllocateTokens(ctx, apiKey, model, 700)
	if err != nil {
		t.Fatalf("should succeed after adjustment freed up tokens: %v", err)
	}
}

func TestIsolation_DifferentKeysAndModels(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	rl := NewRedisRateLimiter(client, Config{
		Enabled:    true,
		DefaultRPM: 2,
		DefaultTPM: 100000,
	})

	ctx := context.Background()

	// Fill limit for userA + gpt-4
	for i := 0; i < 2; i++ {
		rl.AllowRequest(ctx, "key-A", "gpt-4")
	}

	// userA + gpt-4 should be denied
	if err := rl.AllowRequest(ctx, "key-A", "gpt-4"); err == nil {
		t.Error("key-A gpt-4 should be denied")
	}

	// userB + gpt-4 should still be allowed (different key)
	if err := rl.AllowRequest(ctx, "key-B", "gpt-4"); err != nil {
		t.Errorf("key-B gpt-4 should be allowed: %v", err)
	}

	// userA + deepseek should still be allowed (different model)
	if err := rl.AllowRequest(ctx, "key-A", "deepseek-chat"); err != nil {
		t.Errorf("key-A deepseek should be allowed: %v", err)
	}
}

func TestGetUsage(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	rl := NewRedisRateLimiter(client, Config{
		Enabled:    true,
		DefaultRPM: 100,
		DefaultTPM: 100000,
	})

	ctx := context.Background()
	apiKey := "sk-test-key"
	model := "gpt-4"

	// Make 3 requests
	for i := 0; i < 3; i++ {
		rl.AllowRequest(ctx, apiKey, model)
	}

	// Pre-allocate 500 tokens
	rl.PreAllocateTokens(ctx, apiKey, model, 500)

	rpm, tpm, err := rl.GetUsage(ctx, apiKey, model)
	if err != nil {
		t.Fatalf("GetUsage failed: %v", err)
	}

	if rpm != 3 {
		t.Errorf("expected RPM = 3, got %d", rpm)
	}
	if tpm != 500 {
		t.Errorf("expected TPM = 500, got %d", tpm)
	}

	t.Logf("Usage: RPM=%d, TPM=%d", rpm, tpm)
}

func TestModelSpecificConfig(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	rl := NewRedisRateLimiter(client, Config{
		Enabled:    true,
		DefaultRPM: 100,
		DefaultTPM: 100000,
		Models: map[string]ModelRateConfig{
			"gpt-4": {RPM: 2, TPM: 500},
		},
	})

	ctx := context.Background()
	apiKey := "sk-test-key"

	// gpt-4 should be limited to 2 RPM
	for i := 0; i < 2; i++ {
		if err := rl.AllowRequest(ctx, apiKey, "gpt-4"); err != nil {
			t.Fatalf("gpt-4 request %d should be allowed: %v", i+1, err)
		}
	}
	if err := rl.AllowRequest(ctx, apiKey, "gpt-4"); err == nil {
		t.Error("gpt-4 3rd request should be denied (limit=2)")
	}

	// deepseek-chat should use default (100 RPM), so 3 requests should be fine
	for i := 0; i < 3; i++ {
		if err := rl.AllowRequest(ctx, apiKey, "deepseek-chat"); err != nil {
			t.Fatalf("deepseek request %d should be allowed: %v", i+1, err)
		}
	}
}

func TestDisabled(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	rl := NewRedisRateLimiter(client, Config{
		Enabled:    false,
		DefaultRPM: 1,
		DefaultTPM: 1,
	})

	ctx := context.Background()

	// All requests should be allowed when disabled
	for i := 0; i < 100; i++ {
		if err := rl.AllowRequest(ctx, "key", "model"); err != nil {
			t.Fatalf("request should be allowed when disabled: %v", err)
		}
	}
}

func TestHashAPIKey(t *testing.T) {
	// Same key should produce same hash
	h1 := hashAPIKey("sk-abc123")
	h2 := hashAPIKey("sk-abc123")
	if h1 != h2 {
		t.Errorf("same key should produce same hash: %s != %s", h1, h2)
	}

	// Different keys should produce different hashes
	h3 := hashAPIKey("sk-different-key")
	if h1 == h3 {
		t.Error("different keys should produce different hashes")
	}

	// Hash should be 16 chars
	if len(h1) != 16 {
		t.Errorf("hash should be 16 chars, got %d: %s", len(h1), h1)
	}

	t.Logf("Hash of 'sk-abc123': %s", h1)
}

func TestAllowRequest_RedisFailureModeOpen(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer client.Close()
	defer mr.Close()

	rl := NewRedisRateLimiter(client, Config{
		Enabled:     true,
		DefaultRPM:  1,
		DefaultTPM:  1000,
		FailureMode: FailureModeOpen,
	})

	mr.Close()

	if err := rl.AllowRequest(context.Background(), "key", "model"); err != nil {
		t.Fatalf("expected fail-open behavior, got error: %v", err)
	}
}

func TestAllowRequest_RedisFailureModeClosed(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer client.Close()
	defer mr.Close()

	rl := NewRedisRateLimiter(client, Config{
		Enabled:     true,
		DefaultRPM:  1,
		DefaultTPM:  1000,
		FailureMode: FailureModeClosed,
	})

	mr.Close()

	err := rl.AllowRequest(context.Background(), "key", "model")
	if err == nil {
		t.Fatal("expected dependency unavailable error, got nil")
	}

	var depErr *DependencyUnavailableError
	if !errors.As(err, &depErr) {
		t.Fatalf("expected DependencyUnavailableError, got %T", err)
	}
}
