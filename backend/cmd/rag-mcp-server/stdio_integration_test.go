package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"interview-agents/internal/mcp/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestStdioServerProcessListsAndCallsRetrieveKnowledge(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stdio process smoke test in short mode")
	}

	var capturedAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/retrieve" {
			t.Fatalf("path = %q, want /v1/retrieve", r.URL.Path)
		}
		capturedAuth = r.Header.Get("Authorization")
		var req struct {
			Query string   `json:"query"`
			KBIDs []uint64 `json:"kb_ids"`
			TopK  int      `json:"top_k"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Query != "stdio smoke" || len(req.KBIDs) != 1 || req.KBIDs[0] != 42 || req.TopK != 1 {
			t.Fatalf("unexpected retrieve request: %+v", req)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": 200,
			"message": "Success",
			"data": {
				"request_id": "req-stdio",
				"items": [{
					"content": "stdio evidence",
					"score": 0.77,
					"citation": {"kb_id": 42, "document_id": 100, "chunk_id": "chunk-stdio", "file_name": "stdio.md", "chunk_index": 5},
					"source": {"route": "dense", "collection": "kb_42", "retriever_version": "phase1-dense-v1"}
				}]
			}
		}`))
	}))
	defer upstream.Close()

	binaryPath := buildTestBinary(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "--transport", "stdio")
	cmd.Env = append(os.Environ(),
		"RAG_BASE_URL="+upstream.URL,
		"RAG_ACCESS_TOKEN=rag_stdio_token",
	)
	stderr := new(strings.Builder)
	cmd.Stderr = stderr

	client := mcp.NewClient(&mcp.Implementation{Name: "stdio-test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{
		Command:           cmd,
		TerminateDuration: time.Second,
	}, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v stderr=%s", err, stderr.String())
	}
	defer session.Close()

	listResult, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools() error = %v stderr=%s", err, stderr.String())
	}
	if len(listResult.Tools) != 1 || listResult.Tools[0].Name != tools.RetrieveKnowledgeName {
		t.Fatalf("tools = %+v", listResult.Tools)
	}

	callResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: tools.RetrieveKnowledgeName,
		Arguments: map[string]interface{}{
			"query":  "stdio smoke",
			"kb_ids": []uint64{42},
			"top_k":  1,
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v stderr=%s", err, stderr.String())
	}
	if callResult.IsError {
		t.Fatalf("CallTool() returned tool error: %+v", callResult.Content)
	}
	if capturedAuth != "Bearer rag_stdio_token" {
		t.Fatalf("Authorization = %q", capturedAuth)
	}
	if len(callResult.Content) != 1 {
		t.Fatalf("content length = %d, want 1", len(callResult.Content))
	}
	text := callResult.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "stdio.md") || !strings.Contains(text, "stdio evidence") {
		t.Fatalf("readable text = %q", text)
	}
	if callResult.StructuredContent == nil {
		t.Fatal("StructuredContent = nil")
	}
	if strings.Contains(stderr.String(), "rag_stdio_token") {
		t.Fatalf("stderr leaked access token: %s", stderr.String())
	}
}

func TestStdioServerProcessFailsWithoutRequiredEnv(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stdio process smoke test in short mode")
	}

	binaryPath := buildTestBinary(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "--transport", "stdio")
	cmd.Env = filteredEnv(os.Environ(), "RAG_BASE_URL", "RAG_ACCESS_TOKEN")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("command succeeded, want failure; output=%s", string(output))
	}
	if !strings.Contains(string(output), "RAG_BASE_URL is required") {
		t.Fatalf("output = %q, want missing RAG_BASE_URL message", string(output))
	}
}

func buildTestBinary(t *testing.T) string {
	t.Helper()
	name := "rag-mcp-server-test"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binaryPath := filepath.Join(t.TempDir(), name)
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, string(output))
	}
	return binaryPath
}

func filteredEnv(env []string, names ...string) []string {
	blocked := make(map[string]struct{}, len(names))
	for _, name := range names {
		blocked[name] = struct{}{}
	}
	filtered := env[:0]
	for _, item := range env {
		name, _, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		if _, drop := blocked[name]; drop {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}
