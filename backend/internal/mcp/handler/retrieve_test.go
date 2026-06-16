package handler

import (
	"context"
	"strings"
	"testing"

	ragclient "interview-agents/internal/mcp/client"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeRetriever struct {
	req  ragclient.RetrieveRequest
	resp *ragclient.RetrieveResponse
	err  error
}

func (f *fakeRetriever) Retrieve(ctx context.Context, req ragclient.RetrieveRequest) (*ragclient.RetrieveResponse, error) {
	f.req = req
	return f.resp, f.err
}

func TestNormalizeInputTrimsMergesAndDefaults(t *testing.T) {
	req, err := normalizeInput(RetrieveKnowledgeInput{
		Query: "  explain mvcc  ",
		KBIDs: []uint64{1, 2, 1},
		KBID:  3,
	})
	if err != nil {
		t.Fatalf("normalizeInput() error = %v", err)
	}
	if req.Query != "explain mvcc" {
		t.Fatalf("Query = %q", req.Query)
	}
	if got, want := req.KBIDs, []uint64{1, 2, 3}; !equalUint64s(got, want) {
		t.Fatalf("KBIDs = %v, want %v", got, want)
	}
	if req.TopK != defaultTopK {
		t.Fatalf("TopK = %d, want %d", req.TopK, defaultTopK)
	}
}

func TestNormalizeInputRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name  string
		input RetrieveKnowledgeInput
		want  string
	}{
		{name: "empty query", input: RetrieveKnowledgeInput{Query: " ", KBIDs: []uint64{1}}, want: "query is required"},
		{name: "missing kb", input: RetrieveKnowledgeInput{Query: "q"}, want: "kb_id or kb_ids is required"},
		{name: "zero kb", input: RetrieveKnowledgeInput{Query: "q", KBIDs: []uint64{0}}, want: "positive integers"},
		{name: "bad top k", input: RetrieveKnowledgeInput{Query: "q", KBIDs: []uint64{1}, TopK: 21}, want: "top_k"},
		{name: "oversized metadata", input: RetrieveKnowledgeInput{
			Query:          "q",
			KBIDs:          []uint64{1},
			MetadataFilter: map[string]interface{}{"payload": strings.Repeat("x", maxMetadataFilterBytes)},
		}, want: "metadata_filter exceeds"},
		{name: "deep metadata", input: RetrieveKnowledgeInput{
			Query:          "q",
			KBIDs:          []uint64{1},
			MetadataFilter: nestedFilter(maxMetadataFilterDepth + 1),
		}, want: "metadata_filter nesting"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeInput(tt.input)
			if err == nil {
				t.Fatal("normalizeInput() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want contains %q", err.Error(), tt.want)
			}
		})
	}
}

func TestRetrieveHandlerReturnsReadableTextAndStructuredOutput(t *testing.T) {
	retriever := &fakeRetriever{
		resp: &ragclient.RetrieveResponse{
			RequestID: "req-1",
			Items: []ragclient.RetrieveItem{
				{
					Content: "JVM heap sizing depends on workload.",
					Score:   0.92,
					Citation: ragclient.Citation{
						KBID:       1,
						DocumentID: 10,
						FileName:   "jvm.md",
						ChunkIndex: 3,
					},
					Source: ragclient.Source{Route: "dense"},
				},
			},
		},
	}
	h := NewRetrieveHandler(RetrieverFactoryFunc(func(*mcp.CallToolRequest) (Retriever, error) {
		return retriever, nil
	}))
	result, output, err := h.Handle(context.Background(), nil, RetrieveKnowledgeInput{
		Query: "jvm tuning",
		KBID:  1,
		TopK:  1,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if retriever.req.Query != "jvm tuning" || retriever.req.TopK != 1 || len(retriever.req.KBIDs) != 1 {
		t.Fatalf("unexpected upstream request: %+v", retriever.req)
	}
	if output.RequestID != "req-1" || len(output.Items) != 1 {
		t.Fatalf("unexpected output: %+v", output)
	}
	if len(result.Content) != 1 {
		t.Fatalf("Content length = %d, want 1", len(result.Content))
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Content[0] = %T, want *mcp.TextContent", result.Content[0])
	}
	if !strings.Contains(text.Text, "Retrieved 1 item(s).") || !strings.Contains(text.Text, "jvm.md") {
		t.Fatalf("unexpected readable text: %s", text.Text)
	}
}

func equalUint64s(a, b []uint64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func nestedFilter(depth int) map[string]interface{} {
	root := map[string]interface{}{}
	current := root
	for i := 0; i < depth; i++ {
		next := map[string]interface{}{}
		current["child"] = next
		current = next
	}
	return root
}
