package transport

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	internalmcp "interview-agents/internal/mcp"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestHTTPTransportSupportsInitializeListAndCall(t *testing.T) {
	var capturedAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": 200,
			"message": "Success",
			"data": {
				"request_id": "req-http",
				"items": [{
					"content": "http evidence",
					"score": 0.88,
					"citation": {"kb_id": 11, "document_id": 101, "chunk_id": "chunk-http", "file_name": "http.md", "chunk_index": 1},
					"source": {"route": "dense", "collection": "kb_11", "retriever_version": "phase2-dense-v1"}
				}]
			}
		}`))
	}))
	defer upstream.Close()

	cfg := internalmcp.Config{
		Enabled:        true,
		Transport:      "http",
		Host:           "127.0.0.1",
		Port:           8898,
		Endpoint:       "/mcp",
		AllowedOrigins: []string{"https://agent.example.com"},
		RAGBaseURL:     upstream.URL,
		Timeout:        time.Second,
		SessionTimeout: time.Minute,
	}
	server, err := internalmcp.NewServerFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewServerFromConfig() error = %v", err)
	}

	httpServer := httptest.NewServer(NewHTTPHandler(cfg, server, log.New(io.Discard, "", 0)))
	defer httpServer.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "http-test-client", Version: "0.1.0"}, nil)
	httpClient := httpServer.Client()
	transport := &mcp.StreamableClientTransport{
		Endpoint: httpServer.URL + cfg.Endpoint,
		HTTPClient: &http.Client{
			Timeout: httpClient.Timeout,
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				req.Header.Set("Origin", "https://agent.example.com")
				req.Header.Set("Authorization", "Bearer rag_http_token")
				return httpClient.Transport.RoundTrip(req)
			}),
		},
	}

	session, err := client.Connect(context.Background(), transport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	defer session.Close()

	toolsResult, err := session.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	if len(toolsResult.Tools) != 1 {
		t.Fatalf("tools length = %d, want 1", len(toolsResult.Tools))
	}

	callResult, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "retrieve_knowledge",
		Arguments: map[string]interface{}{
			"query":  "http smoke",
			"kb_ids": []uint64{11},
			"top_k":  1,
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if callResult.IsError {
		t.Fatalf("CallTool() returned error result: %+v", callResult.StructuredContent)
	}
	if capturedAuth != "Bearer rag_http_token" {
		t.Fatalf("Authorization = %q, want Bearer rag_http_token", capturedAuth)
	}
}

func TestHTTPTransportRejectsMissingAuthorization(t *testing.T) {
	cfg := internalmcp.Config{
		Enabled:        true,
		Transport:      "http",
		Host:           "127.0.0.1",
		Port:           8898,
		Endpoint:       "/mcp",
		AllowedOrigins: []string{"https://agent.example.com"},
		RAGBaseURL:     "http://127.0.0.1:9",
		Timeout:        time.Second,
		SessionTimeout: time.Minute,
	}
	server, err := internalmcp.NewServerFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewServerFromConfig() error = %v", err)
	}
	httpServer := httptest.NewServer(NewHTTPHandler(cfg, server, log.New(io.Discard, "", 0)))
	defer httpServer.Close()

	req, err := http.NewRequest(http.MethodPost, httpServer.URL+cfg.Endpoint, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Origin", "https://agent.example.com")
	resp, err := httpServer.Client().Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 401 body=%s", resp.StatusCode, string(body))
	}
}

func TestHTTPTransportRejectsOriginOutsideAllowlist(t *testing.T) {
	cfg := internalmcp.Config{
		Enabled:        true,
		Transport:      "http",
		Host:           "127.0.0.1",
		Port:           8898,
		Endpoint:       "/mcp",
		AllowedOrigins: []string{"https://agent.example.com"},
		RAGBaseURL:     "http://127.0.0.1:9",
		Timeout:        time.Second,
		SessionTimeout: time.Minute,
	}
	server, err := internalmcp.NewServerFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewServerFromConfig() error = %v", err)
	}
	httpServer := httptest.NewServer(NewHTTPHandler(cfg, server, log.New(io.Discard, "", 0)))
	defer httpServer.Close()

	req, err := http.NewRequest(http.MethodOptions, httpServer.URL+cfg.Endpoint, nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.Header.Set("Origin", "https://blocked.example.com")
	resp, err := httpServer.Client().Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestReadyzReflectsUpstreamAvailability(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/readyz" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"degraded"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	cfg := internalmcp.Config{
		Enabled:        true,
		Transport:      "http",
		Host:           "127.0.0.1",
		Port:           8898,
		Endpoint:       "/mcp",
		RAGBaseURL:     upstream.URL,
		Timeout:        time.Second,
		SessionTimeout: time.Minute,
		DisableHTTPAuth: true,
	}
	server, err := internalmcp.NewServerFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewServerFromConfig() error = %v", err)
	}
	httpServer := httptest.NewServer(NewHTTPHandler(cfg, server, log.New(io.Discard, "", 0)))
	defer httpServer.Close()

	resp, err := httpServer.Client().Get(httpServer.URL + "/readyz")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if payload["status"] != "degraded" {
		t.Fatalf("payload = %+v", payload)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
