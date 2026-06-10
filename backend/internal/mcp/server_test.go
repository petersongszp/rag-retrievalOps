package mcp

import (
	"context"
	"strings"
	"testing"

	ragclient "interview-agents/internal/mcp/client"
	"interview-agents/internal/mcp/tools"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type protocolRetriever struct {
	req ragclient.RetrieveRequest
}

func (r *protocolRetriever) Retrieve(ctx context.Context, req ragclient.RetrieveRequest) (*ragclient.RetrieveResponse, error) {
	r.req = req
	return &ragclient.RetrieveResponse{
		RequestID: "req-protocol",
		Items: []ragclient.RetrieveItem{
			{
				Content:  "protocol result",
				Score:    0.88,
				Citation: ragclient.Citation{KBID: 9, DocumentID: 10, FileName: "protocol.md", ChunkIndex: 1},
				Source:   ragclient.Source{Route: "dense", Collection: "kb_9", RetrieverVersion: "phase1-dense-v1"},
			},
		},
	}, nil
}

func TestServerListsAndCallsRetrieveKnowledge(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	retriever := &protocolRetriever{}
	server, err := NewServer(retriever)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Run(ctx, serverTransport)
	}()

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	defer session.Close()

	toolsResult, err := session.ListTools(ctx, &mcpsdk.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	if len(toolsResult.Tools) != 1 || toolsResult.Tools[0].Name != tools.RetrieveKnowledgeName {
		t.Fatalf("tools = %+v", toolsResult.Tools)
	}

	callResult, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: tools.RetrieveKnowledgeName,
		Arguments: map[string]interface{}{
			"query":  " protocol query ",
			"kb_ids": []uint64{9},
			"top_k":  2,
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if callResult.IsError {
		t.Fatalf("CallTool() returned tool error: %+v", callResult.Content)
	}
	if retriever.req.Query != "protocol query" || retriever.req.TopK != 2 {
		t.Fatalf("unexpected retrieve request: %+v", retriever.req)
	}
	if len(callResult.Content) != 1 {
		t.Fatalf("content length = %d, want 1", len(callResult.Content))
	}
	text := callResult.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "protocol.md") {
		t.Fatalf("readable text = %q, want protocol.md", text)
	}
	if callResult.StructuredContent == nil {
		t.Fatal("StructuredContent = nil")
	}
	structured, ok := callResult.StructuredContent.(map[string]interface{})
	if !ok {
		t.Fatalf("StructuredContent = %T, want map[string]interface{}", callResult.StructuredContent)
	}
	if structured["request_id"] != "req-protocol" {
		t.Fatalf("request_id = %v", structured["request_id"])
	}

	cancel()
	_ = session.Close()
	<-errCh
}

func TestServerRejectsForbiddenIdentityArguments(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server, err := NewServer(&protocolRetriever{})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Run(ctx, serverTransport)
	}()

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	defer session.Close()

	callResult, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: tools.RetrieveKnowledgeName,
		Arguments: map[string]interface{}{
			"query":   "q",
			"kb_ids":  []uint64{1},
			"api_key": "rag_xxx",
		},
	})
	if err != nil {
		t.Fatalf("CallTool() protocol error = %v", err)
	}
	if !callResult.IsError {
		t.Fatalf("IsError = false, want true")
	}

	cancel()
	_ = session.Close()
	<-errCh
}
