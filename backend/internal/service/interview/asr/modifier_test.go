package asr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOpenAITranscriptModifierSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var payload struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Model != "Qwen/Qwen3.5-4B" {
			t.Fatalf("unexpected model: %s", payload.Model)
		}
		if len(payload.Messages) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(payload.Messages))
		}
		if !strings.Contains(payload.Messages[1].Content, "go0") {
			t.Fatalf("expected transcript to be present in prompt, got %q", payload.Messages[1].Content)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"go-zero"}}]}`))
	}))
	defer server.Close()

	modifier := NewOpenAITranscriptModifier(ASRConfig{
		BaseURL:        server.URL,
		APIKey:         "asr-key",
		ModifyLLMModel: "Qwen/Qwen3.5-4B",
	})

	text, err := modifier.Modify(context.Background(), TranscriptModifyRequest{
		QuestionText: "请介绍一下 go-zero",
		Transcript:   "go0",
	})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if text != "go-zero" {
		t.Fatalf("expected modified text, got %q", text)
	}
}

func TestOpenAITranscriptModifierHandlesServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer server.Close()

	modifier := NewOpenAITranscriptModifier(ASRConfig{
		BaseURL:        server.URL,
		APIKey:         "asr-key",
		ModifyLLMModel: "Qwen/Qwen3.5-4B",
	})

	_, err := modifier.Modify(context.Background(), TranscriptModifyRequest{
		QuestionText: "请介绍一下 go-zero",
		Transcript:   "go0",
	})
	if err == nil {
		t.Fatal("expected error on 500 response")
	}
}

func TestOpenAITranscriptModifierHandlesInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer server.Close()

	modifier := NewOpenAITranscriptModifier(ASRConfig{
		BaseURL:        server.URL,
		APIKey:         "asr-key",
		ModifyLLMModel: "Qwen/Qwen3.5-4B",
	})

	_, err := modifier.Modify(context.Background(), TranscriptModifyRequest{
		QuestionText: "请介绍一下 go-zero",
		Transcript:   "go0",
	})
	if err == nil {
		t.Fatal("expected error on invalid JSON response")
	}
}

func TestOpenAITranscriptModifierHandlesTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"go-zero"}}]}`))
	}))
	defer server.Close()

	modifier := NewOpenAITranscriptModifier(ASRConfig{
		BaseURL:        server.URL,
		APIKey:         "asr-key",
		ModifyLLMModel: "Qwen/Qwen3.5-4B",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := modifier.Modify(ctx, TranscriptModifyRequest{
		QuestionText: "请介绍一下 go-zero",
		Transcript:   "go0",
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
