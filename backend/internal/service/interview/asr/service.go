package asr

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
)

type service struct {
	cfg      ASRConfig
	guard    Guard
	provider AudioTranscriptionProvider
}

func NewService(cfg ASRConfig, redisClient *redis.Client) Service {
	return &service{
		cfg:      cfg,
		guard:    newGuard(cfg, redisClient),
		provider: newProviderForConfig(cfg),
	}
}

func NewServiceWithDependencies(cfg ASRConfig, guard Guard, provider AudioTranscriptionProvider) Service {
	return &service{
		cfg:      cfg,
		guard:    guard,
		provider: provider,
	}
}

func (s *service) GetCapability(ctx context.Context, userID uint) (*Capability, error) {
	_ = userID
	return s.guard.CheckCapability(ctx)
}

func (s *service) Transcribe(ctx context.Context, userID uint, req AudioTranscriptionRequest) (*AudioTranscriptionResult, error) {
	capability, err := s.guard.CheckCapability(ctx)
	if err != nil {
		return nil, err
	}
	if capability == nil || !capability.Enabled {
		return nil, NewNotConfiguredError()
	}
	if userID == 0 {
		return nil, NewUnavailableError("", errors.New("user id is required"))
	}
	if s.provider == nil {
		return nil, NewUnavailableError("", errors.New("asr provider is not initialized"))
	}

	if err := s.guard.AllowUser(ctx, userID, capability.Model); err != nil {
		return nil, err
	}
	if err := s.guard.AllowProvider(ctx, capability.Provider, capability.Model); err != nil {
		return nil, err
	}

	req.ModelName = capability.Model

	result, err := s.provider.Transcribe(ctx, req)
	if err != nil {
		var providerErr *ProviderError
		if errors.As(err, &providerErr) {
			return nil, NewUnavailableError(providerErr.TraceID, providerErr)
		}
		return nil, NewUnavailableError("", err)
	}

	if result.Provider == "" {
		result.Provider = capability.Provider
	}
	if result.Model == "" {
		result.Model = capability.Model
	}

	return result, nil
}

func newProviderForConfig(cfg ASRConfig) AudioTranscriptionProvider {
	if cfg.Capability().Enabled {
		return NewSiliconFlowProvider(cfg)
	}
	return nil
}
