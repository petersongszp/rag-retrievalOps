package asr

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupGuardTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
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

func testASRConfig() ASRConfig {
	return ASRConfig{
		BaseURL:   "https://api.siliconflow.cn/v1",
		APIKey:    "asr-key",
		ModelName: "FunAudioLLM/SenseVoiceSmall",
	}
}

func TestGuardUserLimit(t *testing.T) {
	client, mr := setupGuardTestRedis(t)
	defer client.Close()
	defer mr.Close()

	guard := newGuard(testASRConfig(), client)
	ctx := context.Background()

	for i := 0; i < defaultUserRPM; i++ {
		if err := guard.AllowUser(ctx, 1, "FunAudioLLM/SenseVoiceSmall"); err != nil {
			t.Fatalf("request %d should be allowed: %v", i+1, err)
		}
	}

	err := guard.AllowUser(ctx, 1, "FunAudioLLM/SenseVoiceSmall")
	if err == nil {
		t.Fatal("expected user limiter to reject extra request")
	}

	var serviceErr *ServiceError
	if !errors.As(err, &serviceErr) {
		t.Fatalf("expected ServiceError, got %T", err)
	}
	if serviceErr.Code != ErrorCodeRateLimitExceeded {
		t.Fatalf("expected %q, got %q", ErrorCodeRateLimitExceeded, serviceErr.Code)
	}
}

func TestGuardProviderLimit(t *testing.T) {
	client, mr := setupGuardTestRedis(t)
	defer client.Close()
	defer mr.Close()

	guard := newGuard(testASRConfig(), client)
	ctx := context.Background()

	for i := 0; i < defaultProviderRPM; i++ {
		if err := guard.AllowProvider(ctx, ProviderSiliconFlow, "FunAudioLLM/SenseVoiceSmall"); err != nil {
			t.Fatalf("provider request %d should be allowed: %v", i+1, err)
		}
	}

	err := guard.AllowProvider(ctx, ProviderSiliconFlow, "FunAudioLLM/SenseVoiceSmall")
	if err == nil {
		t.Fatal("expected provider limiter to reject extra request")
	}
}

func TestGuardRedisFailureReturnsUnavailable(t *testing.T) {
	client, mr := setupGuardTestRedis(t)
	defer client.Close()
	defer mr.Close()

	guard := newGuard(testASRConfig(), client)
	mr.Close()

	err := guard.AllowUser(context.Background(), 1, "FunAudioLLM/SenseVoiceSmall")
	if err == nil {
		t.Fatal("expected unavailable error when redis is down")
	}

	var serviceErr *ServiceError
	if !errors.As(err, &serviceErr) {
		t.Fatalf("expected ServiceError, got %T", err)
	}
	if serviceErr.Code != ErrorCodeUnavailable {
		t.Fatalf("expected %q, got %q", ErrorCodeUnavailable, serviceErr.Code)
	}
}
