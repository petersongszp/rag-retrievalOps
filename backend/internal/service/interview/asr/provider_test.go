package asr

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSiliconFlowProviderSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/audio/transcriptions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("x-siliconcloud-trace-id", "trace-success")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"text":"hello world"}`))
	}))
	defer server.Close()

	provider := NewSiliconFlowProvider(ASRConfig{
		BaseURL:   server.URL,
		APIKey:    "asr-key",
		ModelName: "FunAudioLLM/SenseVoiceSmall",
	})

	result, err := provider.Transcribe(context.Background(), AudioTranscriptionRequest{
		FileName:    "answer.webm",
		ContentType: "audio/webm",
		AudioBytes:  []byte("fake"),
		ModelName:   "FunAudioLLM/SenseVoiceSmall",
	})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if result.Text != "hello world" {
		t.Fatalf("expected text to be returned, got %q", result.Text)
	}
	if result.TraceID != "trace-success" {
		t.Fatalf("expected trace id to be returned, got %q", result.TraceID)
	}
}

func TestSiliconFlowProviderHandles429(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-siliconcloud-trace-id", "trace-429")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"too many"}`))
	}))
	defer server.Close()

	provider := NewSiliconFlowProvider(ASRConfig{
		BaseURL:   server.URL,
		APIKey:    "asr-key",
		ModelName: "FunAudioLLM/SenseVoiceSmall",
	})

	_, err := provider.Transcribe(context.Background(), AudioTranscriptionRequest{
		FileName:    "answer.webm",
		ContentType: "audio/webm",
		AudioBytes:  []byte("fake"),
		ModelName:   "FunAudioLLM/SenseVoiceSmall",
	})
	if err == nil {
		t.Fatal("expected provider error on 429 response")
	}

	var providerErr *ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if providerErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", providerErr.StatusCode)
	}
	if providerErr.TraceID != "trace-429" {
		t.Fatalf("expected trace id, got %q", providerErr.TraceID)
	}
}

func TestSiliconFlowProviderHandlesServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-siliconcloud-trace-id", "trace-500")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer server.Close()

	provider := NewSiliconFlowProvider(ASRConfig{
		BaseURL:   server.URL,
		APIKey:    "asr-key",
		ModelName: "FunAudioLLM/SenseVoiceSmall",
	})

	_, err := provider.Transcribe(context.Background(), AudioTranscriptionRequest{
		FileName:    "answer.webm",
		ContentType: "audio/webm",
		AudioBytes:  []byte("fake"),
		ModelName:   "FunAudioLLM/SenseVoiceSmall",
	})
	if err == nil {
		t.Fatal("expected provider error on 500 response")
	}
}

func TestSiliconFlowProviderHandlesTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"text":"late"}`))
	}))
	defer server.Close()

	provider := NewSiliconFlowProvider(ASRConfig{
		BaseURL:   server.URL,
		APIKey:    "asr-key",
		ModelName: "FunAudioLLM/SenseVoiceSmall",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := provider.Transcribe(ctx, AudioTranscriptionRequest{
		FileName:    "answer.webm",
		ContentType: "audio/webm",
		AudioBytes:  []byte("fake"),
		ModelName:   "FunAudioLLM/SenseVoiceSmall",
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
