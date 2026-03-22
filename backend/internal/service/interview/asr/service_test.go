package asr

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
)

type stubGuard struct {
	capability  *Capability
	checkErr    error
	userErr     error
	providerErr error
}

func (g *stubGuard) CheckCapability(ctx context.Context) (*Capability, error) {
	return g.capability, g.checkErr
}

func (g *stubGuard) AllowUser(ctx context.Context, userID uint, model string) error {
	return g.userErr
}

func (g *stubGuard) AllowProvider(ctx context.Context, provider string, model string) error {
	return g.providerErr
}

type stubProvider struct {
	result *AudioTranscriptionResult
	err    error
}

func (p *stubProvider) Transcribe(ctx context.Context, req AudioTranscriptionRequest) (*AudioTranscriptionResult, error) {
	return p.result, p.err
}

type stubModifier struct {
	text      string
	err       error
	waitForCT bool
	callCount int
	lastReq   TranscriptModifyRequest
}

func (m *stubModifier) Modify(ctx context.Context, req TranscriptModifyRequest) (string, error) {
	m.callCount++
	m.lastReq = req
	if m.waitForCT {
		<-ctx.Done()
		return "", ctx.Err()
	}
	if m.err != nil {
		return "", m.err
	}
	return m.text, nil
}

func enabledCapability() *Capability {
	return &Capability{
		Enabled:  true,
		Provider: ProviderSiliconFlow,
		Model:    "FunAudioLLM/SenseVoiceSmall",
	}
}

func baseServiceConfig() ASRConfig {
	return ASRConfig{
		BaseURL:   "https://api.siliconflow.cn/v1",
		APIKey:    "asr-key",
		ModelName: "FunAudioLLM/SenseVoiceSmall",
	}
}

func TestServiceTranscribeReturnsOriginalWhenModifierDisabled(t *testing.T) {
	cfg := baseServiceConfig()
	guard := &stubGuard{capability: enabledCapability()}
	provider := &stubProvider{
		result: &AudioTranscriptionResult{Text: "go0"},
	}
	modifier := &stubModifier{text: "go-zero"}

	service := NewServiceWithDependencies(cfg, guard, provider, modifier)
	result, err := service.Transcribe(context.Background(), 7, AudioTranscriptionRequest{
		QuestionText: "请介绍一下 go-zero",
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if result.Text != "go0" {
		t.Fatalf("expected original transcript, got %q", result.Text)
	}
	if modifier.callCount != 0 {
		t.Fatalf("expected modifier to stay disabled, got %d calls", modifier.callCount)
	}
}

func TestServiceTranscribeAppliesModifiedText(t *testing.T) {
	cfg := baseServiceConfig()
	cfg.ModifyLLMModel = "Qwen/Qwen3.5-4B"

	guard := &stubGuard{capability: enabledCapability()}
	provider := &stubProvider{
		result: &AudioTranscriptionResult{Text: "go0"},
	}
	modifier := &stubModifier{text: "go-zero"}

	service := NewServiceWithDependencies(cfg, guard, provider, modifier)
	result, err := service.Transcribe(context.Background(), 7, AudioTranscriptionRequest{
		SessionID:    "session-1",
		QuestionText: "请介绍一下 go-zero",
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if result.Text != "go-zero" {
		t.Fatalf("expected modified transcript, got %q", result.Text)
	}
	if modifier.callCount != 1 {
		t.Fatalf("expected modifier to be called once, got %d", modifier.callCount)
	}
	if modifier.lastReq.QuestionText != "请介绍一下 go-zero" {
		t.Fatalf("unexpected modifier question text: %q", modifier.lastReq.QuestionText)
	}
	if modifier.lastReq.Transcript != "go0" {
		t.Fatalf("unexpected modifier transcript: %q", modifier.lastReq.Transcript)
	}
}

func TestServiceTranscribeFallsBackOnModifierTimeout(t *testing.T) {
	cfg := baseServiceConfig()
	cfg.ModifyLLMModel = "Qwen/Qwen3.5-4B"

	guard := &stubGuard{capability: enabledCapability()}
	provider := &stubProvider{
		result: &AudioTranscriptionResult{Text: "go0"},
	}
	modifier := &stubModifier{waitForCT: true}

	service := NewServiceWithDependencies(cfg, guard, provider, modifier)
	result, err := service.Transcribe(context.Background(), 7, AudioTranscriptionRequest{
		SessionID:    "session-2",
		QuestionText: "请介绍一下 go-zero",
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if result.Text != "go0" {
		t.Fatalf("expected original transcript after timeout fallback, got %q", result.Text)
	}
}

func TestServiceTranscribeFallsBackOnEmptyModifierText(t *testing.T) {
	cfg := baseServiceConfig()
	cfg.ModifyLLMModel = "Qwen/Qwen3.5-4B"

	guard := &stubGuard{capability: enabledCapability()}
	provider := &stubProvider{
		result: &AudioTranscriptionResult{Text: "I think,嗯, the answer is go0"},
	}
	modifier := &stubModifier{text: "   "}

	service := NewServiceWithDependencies(cfg, guard, provider, modifier)
	result, err := service.Transcribe(context.Background(), 7, AudioTranscriptionRequest{
		SessionID:    "session-3",
		QuestionText: "Please explain go-zero",
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if result.Text != "I think,嗯, the answer is go0" {
		t.Fatalf("expected original transcript after empty modifier result, got %q", result.Text)
	}
}

func TestServiceTranscribeProviderErrorReturnsUnavailable(t *testing.T) {
	cfg := baseServiceConfig()
	cfg.ModifyLLMModel = "Qwen/Qwen3.5-4B"

	guard := &stubGuard{capability: enabledCapability()}
	provider := &stubProvider{
		err: &ProviderError{Err: errors.New("boom")},
	}
	modifier := &stubModifier{text: "go-zero"}

	service := NewServiceWithDependencies(cfg, guard, provider, modifier)
	_, err := service.Transcribe(context.Background(), 7, AudioTranscriptionRequest{
		QuestionText: "请介绍一下 go-zero",
	})
	if err == nil {
		t.Fatal("expected error when provider fails")
	}

	var serviceErr *ServiceError
	if !errors.As(err, &serviceErr) {
		t.Fatalf("expected ServiceError, got %T", err)
	}
	if serviceErr.Code != ErrorCodeUnavailable {
		t.Fatalf("expected %q, got %q", ErrorCodeUnavailable, serviceErr.Code)
	}
	if modifier.callCount != 0 {
		t.Fatalf("expected modifier to be skipped when ASR fails, got %d calls", modifier.callCount)
	}
}

func TestServiceTranscribeLogsRawTranscriptWhenModifierDisabled(t *testing.T) {
	cfg := baseServiceConfig()
	guard := &stubGuard{capability: enabledCapability()}
	provider := &stubProvider{
		result: &AudioTranscriptionResult{
			Text:     "go0",
			Provider: ProviderSiliconFlow,
			Model:    "FunAudioLLM/SenseVoiceSmall",
			TraceID:  "trace-raw",
		},
	}
	modifier := &stubModifier{text: "go-zero"}

	logs := captureTestLogs(t)

	service := NewServiceWithDependencies(cfg, guard, provider, modifier)
	_, err := service.Transcribe(context.Background(), 7, AudioTranscriptionRequest{
		SessionID:    "session-raw",
		QuestionText: "请介绍一下 go-zero",
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	logOutput := logs.String()
	if !strings.Contains(logOutput, `[ASR] raw transcript user_id=7 session_id=session-raw provider=siliconflow model=FunAudioLLM/SenseVoiceSmall trace_id=trace-raw text="go0"`) {
		t.Fatalf("expected raw transcript log, got %q", logOutput)
	}
	if strings.Contains(logOutput, "[ASR] modified transcript") {
		t.Fatalf("did not expect modified transcript log when modifier disabled, got %q", logOutput)
	}
}

func TestServiceTranscribeLogsModifiedTranscriptWhenModifierSucceeds(t *testing.T) {
	cfg := baseServiceConfig()
	cfg.ModifyLLMModel = "Qwen/Qwen3.5-4B"

	guard := &stubGuard{capability: enabledCapability()}
	provider := &stubProvider{
		result: &AudioTranscriptionResult{
			Text:     "go0",
			Provider: ProviderSiliconFlow,
			Model:    "FunAudioLLM/SenseVoiceSmall",
			TraceID:  "trace-modified",
		},
	}
	modifier := &stubModifier{text: "go-zero"}

	logs := captureTestLogs(t)

	service := NewServiceWithDependencies(cfg, guard, provider, modifier)
	_, err := service.Transcribe(context.Background(), 7, AudioTranscriptionRequest{
		SessionID:    "session-modified",
		QuestionText: "请介绍一下 go-zero",
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	logOutput := logs.String()
	if !strings.Contains(logOutput, `[ASR] raw transcript user_id=7 session_id=session-modified provider=siliconflow model=FunAudioLLM/SenseVoiceSmall trace_id=trace-modified text="go0"`) {
		t.Fatalf("expected raw transcript log, got %q", logOutput)
	}
	if !strings.Contains(logOutput, `[ASR] modified transcript user_id=7 session_id=session-modified model=Qwen/Qwen3.5-4B original_text="go0" modified_text="go-zero"`) {
		t.Fatalf("expected modified transcript log, got %q", logOutput)
	}
}

func TestServiceTranscribeLogsFallbackWithoutModifiedTranscriptOnModifierError(t *testing.T) {
	cfg := baseServiceConfig()
	cfg.ModifyLLMModel = "Qwen/Qwen3.5-4B"

	guard := &stubGuard{capability: enabledCapability()}
	provider := &stubProvider{
		result: &AudioTranscriptionResult{
			Text:     "go0",
			Provider: ProviderSiliconFlow,
			Model:    "FunAudioLLM/SenseVoiceSmall",
			TraceID:  "trace-fallback",
		},
	}
	modifier := &stubModifier{err: errors.New("modifier boom")}

	logs := captureTestLogs(t)

	service := NewServiceWithDependencies(cfg, guard, provider, modifier)
	_, err := service.Transcribe(context.Background(), 7, AudioTranscriptionRequest{
		SessionID:    "session-fallback",
		QuestionText: "请介绍一下 go-zero",
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	logOutput := logs.String()
	if !strings.Contains(logOutput, `[ASR] raw transcript user_id=7 session_id=session-fallback provider=siliconflow model=FunAudioLLM/SenseVoiceSmall trace_id=trace-fallback text="go0"`) {
		t.Fatalf("expected raw transcript log, got %q", logOutput)
	}
	if !strings.Contains(logOutput, "[ASR] transcript modifier fallback session_id=session-fallback err=modifier boom") {
		t.Fatalf("expected fallback log, got %q", logOutput)
	}
	if strings.Contains(logOutput, "[ASR] modified transcript") {
		t.Fatalf("did not expect modified transcript success log on fallback, got %q", logOutput)
	}
}

func captureTestLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buffer bytes.Buffer
	oldWriter := log.Writer()
	oldFlags := log.Flags()
	log.SetOutput(&buffer)
	log.SetFlags(0)

	t.Cleanup(func() {
		log.SetOutput(oldWriter)
		log.SetFlags(oldFlags)
	})

	return &buffer
}
