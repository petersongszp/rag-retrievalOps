package interview

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"interview-agents/internal/config"
	asrservice "interview-agents/internal/service/interview/asr"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
)

type fakeASRService struct {
	capability *asrservice.Capability
	capErr     error
	result     *asrservice.AudioTranscriptionResult
	transErr   error
	lastReq    asrservice.AudioTranscriptionRequest
}

func (f *fakeASRService) GetCapability(ctx context.Context, userID uint) (*asrservice.Capability, error) {
	return f.capability, f.capErr
}

func (f *fakeASRService) Transcribe(ctx context.Context, userID uint, req asrservice.AudioTranscriptionRequest) (*asrservice.AudioTranscriptionResult, error) {
	f.lastReq = req
	return f.result, f.transErr
}

func TestGetASRCapabilityUnauthorized(t *testing.T) {
	h := server.Default()
	h.GET("/api/interview/asr/capability", GetASRCapability)

	resp := ut.PerformRequest(h.Engine, http.MethodGet, "/api/interview/asr/capability", nil).Result()
	if resp.StatusCode() != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode())
	}
}

func TestGetASRCapabilityConfiguredFalse(t *testing.T) {
	oldFactory := getASRService
	defer func() { getASRService = oldFactory }()
	getASRService = func() asrservice.Service {
		return &fakeASRService{
			capability: &asrservice.Capability{
				Enabled: false,
				Reason:  asrservice.CapabilityReasonNotConfigured,
			},
		}
	}

	h := server.Default()
	h.Use(func(ctx context.Context, c *app.RequestContext) {
		c.Set("user_id", uint(7))
		c.Next(ctx)
	})
	h.GET("/api/interview/asr/capability", GetASRCapability)

	recorder := ut.PerformRequest(h.Engine, http.MethodGet, "/api/interview/asr/capability", nil)
	resp := recorder.Result()
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode())
	}

	var payload struct {
		Code int `json:"code"`
		Data struct {
			Enabled bool   `json:"enabled"`
			Reason  string `json:"reason"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Code != 200 || payload.Data.Enabled {
		t.Fatalf("expected disabled capability payload, got %+v", payload)
	}
}

func TestTranscribeInterviewAudioNotConfigured(t *testing.T) {
	oldFactory := getASRService
	defer func() { getASRService = oldFactory }()
	getASRService = func() asrservice.Service {
		return &fakeASRService{
			transErr: asrservice.NewNotConfiguredError(),
		}
	}

	h := newAuthenticatedASRTestServer()
	h.POST("/api/interview/asr/transcribe", TranscribeInterviewAudio)

	body, contentType := buildMultipartAudioBody(t, nil)
	resp := ut.PerformRequest(
		h.Engine,
		http.MethodPost,
		"/api/interview/asr/transcribe",
		&ut.Body{Body: bytes.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: contentType},
	).Result()

	if resp.StatusCode() != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode())
	}
}

func TestTranscribeInterviewAudioRateLimited(t *testing.T) {
	oldFactory := getASRService
	defer func() { getASRService = oldFactory }()
	getASRService = func() asrservice.Service {
		return &fakeASRService{
			transErr: asrservice.NewRateLimitExceededError(30, errors.New("limit reached")),
		}
	}

	h := newAuthenticatedASRTestServer()
	h.POST("/api/interview/asr/transcribe", TranscribeInterviewAudio)

	body, contentType := buildMultipartAudioBody(t, nil)
	resp := ut.PerformRequest(
		h.Engine,
		http.MethodPost,
		"/api/interview/asr/transcribe",
		&ut.Body{Body: bytes.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: contentType},
	).Result()

	if resp.StatusCode() != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp.StatusCode())
	}
}

func TestTranscribeInterviewAudioPassesQuestionText(t *testing.T) {
	oldFactory := getASRService
	defer func() { getASRService = oldFactory }()

	fake := &fakeASRService{
		result: &asrservice.AudioTranscriptionResult{
			Text:     "gozero",
			Provider: asrservice.ProviderSiliconFlow,
			Model:    "FunAudioLLM/SenseVoiceSmall",
		},
	}
	getASRService = func() asrservice.Service { return fake }

	h := newAuthenticatedASRTestServer()
	h.POST("/api/interview/asr/transcribe", TranscribeInterviewAudio)

	body, contentType := buildMultipartAudioBody(t, map[string]string{
		"question_text":  "请你介绍一下 go-zero 的服务治理能力",
		"session_id":     "session-1",
		"interview_type": "专项面试",
		"domain":         "go",
	})
	resp := ut.PerformRequest(
		h.Engine,
		http.MethodPost,
		"/api/interview/asr/transcribe",
		&ut.Body{Body: bytes.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: contentType},
	).Result()

	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode())
	}
	if fake.lastReq.QuestionText != "请你介绍一下 go-zero 的服务治理能力" {
		t.Fatalf("expected question_text to be forwarded, got %q", fake.lastReq.QuestionText)
	}
	if fake.lastReq.SessionID != "session-1" {
		t.Fatalf("expected session_id to be forwarded, got %q", fake.lastReq.SessionID)
	}
}

func TestTranscribeInterviewAudioLogsFinalText(t *testing.T) {
	oldFactory := getASRService
	defer func() { getASRService = oldFactory }()

	fake := &fakeASRService{
		result: &asrservice.AudioTranscriptionResult{
			Text:     "go-zero",
			Provider: asrservice.ProviderSiliconFlow,
			Model:    "FunAudioLLM/SenseVoiceSmall",
			TraceID:  "trace-final",
		},
	}
	getASRService = func() asrservice.Service { return fake }

	var logBuffer bytes.Buffer
	oldWriter := log.Writer()
	oldFlags := log.Flags()
	log.SetOutput(&logBuffer)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(oldWriter)
		log.SetFlags(oldFlags)
	}()

	h := newAuthenticatedASRTestServer()
	h.POST("/api/interview/asr/transcribe", TranscribeInterviewAudio)

	body, contentType := buildMultipartAudioBody(t, map[string]string{
		"session_id": "session-final",
	})
	resp := ut.PerformRequest(
		h.Engine,
		http.MethodPost,
		"/api/interview/asr/transcribe",
		&ut.Body{Body: bytes.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: contentType},
	).Result()

	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode())
	}
	if !strings.Contains(logBuffer.String(), `[ASR] transcription success user_id=7 session_id=session-final provider=siliconflow model=FunAudioLLM/SenseVoiceSmall trace_id=trace-final final_text="go-zero"`) {
		t.Fatalf("expected final_text log, got %q", logBuffer.String())
	}
}

func newAuthenticatedASRTestServer() *server.Hertz {
	config.Global.Security.JWTSecret = "test-secret"

	h := server.Default()
	h.Use(func(ctx context.Context, c *app.RequestContext) {
		c.Set("user_id", uint(7))
		c.Next(ctx)
	})

	return h
}

func buildMultipartAudioBody(t *testing.T, extraFields map[string]string) ([]byte, string) {
	t.Helper()

	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	for key, value := range extraFields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("failed to write form field %q: %v", key, err)
		}
	}
	part, err := writer.CreateFormFile("file", "answer.webm")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := part.Write([]byte("audio")); err != nil {
		t.Fatalf("failed to write audio bytes: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	return buffer.Bytes(), writer.FormDataContentType()
}
