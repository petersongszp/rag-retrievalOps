package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	internalmcp "interview-agents/internal/mcp"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestHTTPTransportMapsUpstreamUnauthorizedAndForbidden(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantCode   string
	}{
		{
			name:       "unauthorized",
			statusCode: http.StatusUnauthorized,
			body:       `{"message":"API key invalid","request_id":"req-401"}`,
			wantCode:   "unauthorized",
		},
		{
			name:       "forbidden",
			statusCode: http.StatusForbidden,
			body:       `{"message":"Permission denied","request_id":"req-403"}`,
			wantCode:   "forbidden",
		},
		{
			name:       "unavailable",
			statusCode: http.StatusServiceUnavailable,
			body:       `{"message":"upstream down","request_id":"req-503"}`,
			wantCode:   "backend_unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer upstream.Close()

			session := connectHTTPTestClient(t, internalmcp.Config{
				Enabled:        true,
				Transport:      "http",
				Host:           "127.0.0.1",
				Port:           8898,
				Endpoint:       "/mcp",
				AllowedOrigins: []string{"https://agent.example.com"},
				RAGBaseURL:     upstream.URL,
				Timeout:        time.Second,
				SessionTimeout: time.Minute,
			}, "Bearer rag_http_token", nil)
			defer session.Close()

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
			if !callResult.IsError {
				t.Fatalf("IsError = false, want true")
			}
			if len(callResult.Content) != 1 {
				t.Fatalf("content length = %d, want 1", len(callResult.Content))
			}
			text := callResult.Content[0].(*mcp.TextContent).Text
			if !strings.Contains(text, `"`+"code"+`":"`+tt.wantCode+`"`) {
				t.Fatalf("error text = %s, want code %s", text, tt.wantCode)
			}
		})
	}
}

func TestHTTPTransportMapsUpstreamTimeout(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	session := connectHTTPTestClient(t, internalmcp.Config{
		Enabled:        true,
		Transport:      "http",
		Host:           "127.0.0.1",
		Port:           8898,
		Endpoint:       "/mcp",
		AllowedOrigins: []string{"https://agent.example.com"},
		RAGBaseURL:     upstream.URL,
		Timeout:        10 * time.Millisecond,
		SessionTimeout: time.Minute,
	}, "Bearer rag_http_token", nil)
	defer session.Close()

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
	if !callResult.IsError {
		t.Fatalf("IsError = false, want true")
	}
	text := callResult.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, `"code":"backend_timeout"`) {
		t.Fatalf("error text = %s", text)
	}
}

func TestHTTPTransportLogsRedactedAuthorization(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"message":"Success","data":{"request_id":"req-log","items":[]}}`))
	}))
	defer upstream.Close()

	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	session := connectHTTPTestClient(t, internalmcp.Config{
		Enabled:        true,
		Transport:      "http",
		Host:           "127.0.0.1",
		Port:           8898,
		Endpoint:       "/mcp",
		AllowedOrigins: []string{"https://agent.example.com"},
		RAGBaseURL:     upstream.URL,
		Timeout:        time.Second,
		SessionTimeout: time.Minute,
	}, "Bearer rag_secret_token", logger)
	defer session.Close()

	_, err := session.CallTool(context.Background(), &mcp.CallToolParams{
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
	logOutput := buf.String()
	if strings.Contains(logOutput, "rag_secret_token") {
		t.Fatalf("log leaked token: %s", logOutput)
	}
	if !strings.Contains(logOutput, "auth=sha256:") {
		t.Fatalf("log missing redacted fingerprint: %s", logOutput)
	}
}

func TestHTTPTransportConcurrentCallsDoNotMixAuthorization(t *testing.T) {
	type record struct {
		query string
		auth  string
	}
	var (
		mu      sync.Mutex
		records []record
	)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query string `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		mu.Lock()
		records = append(records, record{query: req.Query, auth: r.Header.Get("Authorization")})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"message":"Success","data":{"request_id":"req-concurrent","items":[]}}`))
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

	runCall := func(query, authz string) error {
		client := mcp.NewClient(&mcp.Implementation{Name: "http-test-client", Version: "0.1.0"}, nil)
		baseClient := httpServer.Client()
		session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
			Endpoint: httpServer.URL + cfg.Endpoint,
			HTTPClient: &http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					req.Header.Set("Origin", "https://agent.example.com")
					req.Header.Set("Authorization", authz)
					return baseClient.Transport.RoundTrip(req)
				}),
			},
		}, nil)
		if err != nil {
			return err
		}
		defer session.Close()
		_, err = session.CallTool(context.Background(), &mcp.CallToolParams{
			Name: "retrieve_knowledge",
			Arguments: map[string]interface{}{
				"query":  query,
				"kb_ids": []uint64{11},
				"top_k":  1,
			},
		})
		return err
	}

	errCh := make(chan error, 2)
	go func() { errCh <- runCall("query-a", "Bearer token-a") }()
	go func() { errCh <- runCall("query-b", "Bearer token-b") }()
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("runCall() error = %v", err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(records) != 2 {
		t.Fatalf("records = %d, want 2", len(records))
	}
	expected := map[string]string{
		"query-a": "Bearer token-a",
		"query-b": "Bearer token-b",
	}
	for _, rec := range records {
		if expected[rec.query] != rec.auth {
			t.Fatalf("query=%s auth=%s expected=%s", rec.query, rec.auth, expected[rec.query])
		}
	}
}

func connectHTTPTestClient(t *testing.T, cfg internalmcp.Config, authorization string, logger *log.Logger) *mcp.ClientSession {
	t.Helper()
	server, err := internalmcp.NewServerFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewServerFromConfig() error = %v", err)
	}
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	httpServer := httptest.NewServer(NewHTTPHandler(cfg, server, logger))
	t.Cleanup(httpServer.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "http-test-client", Version: "0.1.0"}, nil)
	httpClient := httpServer.Client()
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint: httpServer.URL + cfg.Endpoint,
		HTTPClient: &http.Client{
			Timeout: httpClient.Timeout,
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				req.Header.Set("Origin", "https://agent.example.com")
				req.Header.Set("Authorization", authorization)
				return httpClient.Transport.RoundTrip(req)
			}),
		},
	}, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	return session
}
