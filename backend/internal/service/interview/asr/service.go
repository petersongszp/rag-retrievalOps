package asr

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/redis/go-redis/v9"
)

type service struct {
	cfg      ASRConfig
	guard    Guard
	provider AudioTranscriptionProvider
	modifier TranscriptModifier
}

func NewService(cfg ASRConfig, redisClient *redis.Client) Service {
	return &service{
		cfg:      cfg,
		guard:    newGuard(cfg, redisClient),
		provider: newProviderForConfig(cfg),
		modifier: newModifierForConfig(cfg),
	}
}

func NewServiceWithDependencies(cfg ASRConfig, guard Guard, provider AudioTranscriptionProvider, modifier TranscriptModifier) Service {
	return &service{
		cfg:      cfg,
		guard:    guard,
		provider: provider,
		modifier: modifier,
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

	log.Printf(
		"[ASR] raw transcript user_id=%d session_id=%s provider=%s model=%s trace_id=%s text=%q",
		userID,
		req.SessionID,
		result.Provider,
		result.Model,
		result.TraceID,
		result.Text,
	)

	s.modifyTranscriptIfNeeded(ctx, userID, req, result)

	return result, nil
}

func newProviderForConfig(cfg ASRConfig) AudioTranscriptionProvider {
	if cfg.Capability().Enabled {
		return NewSiliconFlowProvider(cfg)
	}
	return nil
}

func newModifierForConfig(cfg ASRConfig) TranscriptModifier {
	if cfg.ModifierEnabled() {
		return NewOpenAITranscriptModifier(cfg)
	}
	return nil
}

func (s *service) modifyTranscriptIfNeeded(ctx context.Context, userID uint, req AudioTranscriptionRequest, result *AudioTranscriptionResult) {
	if result == nil || s.modifier == nil || !s.cfg.ModifierEnabled() {
		return
	}

	questionText := strings.TrimSpace(req.QuestionText)
	originalText := strings.TrimSpace(result.Text)
	if questionText == "" || originalText == "" {
		return
	}

	modifyCtx, cancel := context.WithTimeout(ctx, transcriptModifierTimeout)
	defer cancel()

	modifiedText, err := s.modifier.Modify(modifyCtx, TranscriptModifyRequest{
		QuestionText: questionText,
		Transcript:   originalText,
	})
	if err != nil {
		log.Printf("[ASR] transcript modifier fallback session_id=%s err=%v", req.SessionID, err)
		return
	}

	modifiedText = strings.TrimSpace(modifiedText)
	if modifiedText == "" {
		log.Printf("[ASR] transcript modifier returned empty text session_id=%s", req.SessionID)
		return
	}

	log.Printf(
		"[ASR] modified transcript user_id=%d session_id=%s model=%s original_text=%q modified_text=%q",
		userID,
		req.SessionID,
		s.cfg.ModifyLLMModel,
		originalText,
		modifiedText,
	)

	result.Text = modifiedText
}
