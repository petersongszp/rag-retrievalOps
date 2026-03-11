package ratelimiter

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

// loadEnv loads the .env file from the project root
func loadEnv(t *testing.T) {
	t.Helper()
	envFile := "/Users/lucas/work/code/go/go-eino-interview-agent-co-write/.env"
	if err := godotenv.Load(envFile); err != nil {
		t.Skipf("Skipping integration test: failed to load .env: %v", err)
	}
}

// connectRealRedis connects to Redis using .env config
func connectRealRedis(t *testing.T) *redis.Client {
	t.Helper()

	host := os.Getenv("REDIS_HOST")
	port := os.Getenv("REDIS_PORT")
	password := os.Getenv("REDIS_PASSWORD")

	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "6379"
	}

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: password,
		DB:       1, // Use DB 1 for tests to avoid polluting DB 0
	})

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Skipping integration test: Redis not available at %s:%s: %v", host, port, err)
	}

	// Clean up test keys on test finish
	t.Cleanup(func() {
		// Delete all test keys with our prefix
		//ctx := context.Background()
		//iter := client.Scan(ctx, 0, "ratelimit:*", 100).Iterator()
		//for iter.Next(ctx) {
		//	client.Del(ctx, iter.Val())
		//}
		//client.Close()
	})

	return client
}

// TestIntegration_RPM_WithRealRedis tests RPM rate limiting with a real Redis instance.
// Requires Redis to be running (reads config from .env).
func TestIntegration_RPM_WithRealRedis(t *testing.T) {
	loadEnv(t)
	client := connectRealRedis(t)

	apiKey := os.Getenv("TEST_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping: TEST_API_KEY not set in .env")
	}

	rl := NewRedisRateLimiter(client, Config{
		Enabled:    true,
		DefaultRPM: 5, // Low limit for testing
		DefaultTPM: 100000,
	})

	ctx := context.Background()
	model := "test-model"

	// Send 5 requests (should all be allowed)
	for i := 0; i < 5; i++ {
		err := rl.AllowRequest(ctx, apiKey, model)
		if err != nil {
			t.Fatalf("Request %d should be allowed, got: %v", i+1, err)
		}
		t.Logf("Request %d: ALLOWED", i+1)
	}

	// 6th request should be denied
	err := rl.AllowRequest(ctx, apiKey, model)
	if err == nil {
		t.Fatal("Request 6 should be DENIED, but was allowed")
	}

	rateLimitErr, ok := err.(*RateLimitError)
	if !ok {
		t.Fatalf("Expected *RateLimitError, got %T: %v", err, err)
	}

	t.Logf("Request 6: DENIED ✓")
	t.Logf("  Message: %s", rateLimitErr.Message)
	t.Logf("  Current RPM: %d / %d", rateLimitErr.CurrentRPM, rateLimitErr.LimitRPM)
	t.Logf("  Retry After: %ds", rateLimitErr.RetryAfterSeconds)

	// Verify usage via GetUsage
	rpm, tpm, err := rl.GetUsage(ctx, apiKey, model)
	if err != nil {
		t.Fatalf("GetUsage failed: %v", err)
	}
	t.Logf("  Current Usage: RPM=%d, TPM=%d", rpm, tpm)

	if rpm != 5 {
		t.Errorf("Expected RPM usage = 5, got %d", rpm)
	}
}

// TestIntegration_TPM_PreAllocAndAdjust tests TPM pre-allocation and adjustment with real Redis.
func TestIntegration_TPM_PreAllocAndAdjust(t *testing.T) {
	loadEnv(t)
	client := connectRealRedis(t)

	apiKey := os.Getenv("TEST_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping: TEST_API_KEY not set in .env")
	}

	rl := NewRedisRateLimiter(client, Config{
		Enabled:    true,
		DefaultRPM: 100,
		DefaultTPM: 2000, // 2000 tokens per minute
	})

	ctx := context.Background()
	model := "test-model"

	// Step 1: Pre-allocate 1000 tokens (estimated)
	member, err := rl.PreAllocateTokens(ctx, apiKey, model, 1000)
	if err != nil {
		t.Fatalf("Pre-allocate 1000 tokens should succeed: %v", err)
	}
	t.Logf("Step 1: Pre-allocated 1000 tokens (member: %s)", member[:20]+"...")

	// Step 2: Pre-allocate another 800 tokens (total 1800, still under 2000)
	member2, err := rl.PreAllocateTokens(ctx, apiKey, model, 800)
	if err != nil {
		t.Fatalf("Pre-allocate 800 more tokens should succeed (total 1800 < 2000): %v", err)
	}
	t.Logf("Step 2: Pre-allocated 800 more tokens")

	// Step 3: Pre-allocate 300 more should FAIL (would be 2100 > 2000)
	_, err = rl.PreAllocateTokens(ctx, apiKey, model, 300)
	if err == nil {
		t.Fatal("Pre-allocate 300 more tokens should FAIL (total would be 2100 > 2000)")
	}
	t.Logf("Step 3: Pre-allocate 300 more DENIED ✓ (would exceed limit)")

	// Step 4: Adjust first pre-allocation from 1000 → 200 (actual was much less)
	rl.AdjustTokens(ctx, apiKey, model, member, 200)
	t.Logf("Step 4: Adjusted first pre-alloc from 1000 → 200 (saved 800 tokens)")

	// Step 5: Now pre-allocate 300 should succeed (200 + 800 + 300 = 1300 < 2000)
	_, err = rl.PreAllocateTokens(ctx, apiKey, model, 300)
	if err != nil {
		t.Fatalf("Pre-allocate 300 should now succeed after adjustment: %v", err)
	}
	t.Logf("Step 5: Pre-allocate 300 now ALLOWED ✓ (after adjustment)")

	// Verify final usage
	_, tpm, err := rl.GetUsage(ctx, apiKey, model)
	if err != nil {
		t.Fatalf("GetUsage failed: %v", err)
	}
	t.Logf("Final TPM usage: %d", tpm)

	_ = member2 // used above
}

// TestIntegration_ModelSpecificLimits tests per-model rate limit overrides.
func TestIntegration_ModelSpecificLimits(t *testing.T) {
	loadEnv(t)
	client := connectRealRedis(t)

	apiKey := os.Getenv("TEST_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping: TEST_API_KEY not set in .env")
	}

	rl := NewRedisRateLimiter(client, Config{
		Enabled:    true,
		DefaultRPM: 100,
		DefaultTPM: 100000,
		Models: map[string]ModelRateConfig{
			"expensive-model": {RPM: 3, TPM: 1000},
		},
	})

	ctx := context.Background()

	// expensive-model: limited to 3 RPM
	t.Log("--- expensive-model (limit: 3 RPM) ---")
	for i := 0; i < 3; i++ {
		if err := rl.AllowRequest(ctx, apiKey, "expensive-model"); err != nil {
			t.Fatalf("expensive-model request %d should be allowed: %v", i+1, err)
		}
		t.Logf("  Request %d: ALLOWED", i+1)
	}
	if err := rl.AllowRequest(ctx, apiKey, "expensive-model"); err == nil {
		t.Fatal("expensive-model request 4 should be denied")
	} else {
		t.Logf("  Request 4: DENIED ✓")
	}

	// cheap-model: uses default (100 RPM), 4 requests should all pass
	t.Log("--- cheap-model (limit: 100 RPM default) ---")
	for i := 0; i < 4; i++ {
		if err := rl.AllowRequest(ctx, apiKey, "cheap-model"); err != nil {
			t.Fatalf("cheap-model request %d should be allowed: %v", i+1, err)
		}
		t.Logf("  Request %d: ALLOWED", i+1)
	}

	t.Log("Per-model isolation verified ✓")
}
