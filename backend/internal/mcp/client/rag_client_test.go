package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientRetrieveCallsV1RetrieveWithBearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/retrieve" {
			t.Fatalf("path = %q, want /v1/retrieve", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer rag_test_token" {
			t.Fatalf("Authorization = %q", got)
		}
		var req RetrieveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Query != "hello" || len(req.KBIDs) != 1 || req.KBIDs[0] != 7 || req.TopK != 3 {
			t.Fatalf("unexpected request: %+v", req)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": 200,
			"message": "Success",
			"data": {
				"request_id": "req-1",
				"items": [{
					"content": "answer",
					"score": 0.91,
					"citation": {"kb_id": 7, "document_id": 8, "chunk_id": "c1", "file_name": "a.md", "chunk_index": 2},
					"source": {"route": "dense", "collection": "kb_7", "retriever_version": "phase1-dense-v1"}
				}],
				"evidence_gate_result": "pass"
			}
		}`))
	}))
	defer server.Close()

	client, err := New(server.URL, "rag_test_token", time.Second)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	resp, err := client.Retrieve(context.Background(), RetrieveRequest{
		Query: "hello",
		KBIDs: []uint64{7},
		TopK:  3,
	})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if resp.RequestID != "req-1" || len(resp.Items) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Items[0].Citation.FileName != "a.md" || resp.Items[0].Source.Route != "dense" {
		t.Fatalf("unexpected item: %+v", resp.Items[0])
	}
	if resp.EvidenceGateResult != "pass" {
		t.Fatalf("EvidenceGateResult = %q", resp.EvidenceGateResult)
	}
}

func TestClientRetrieveSupportsBareResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"request_id":"req-bare","items":[]}`))
	}))
	defer server.Close()

	client, err := New(server.URL, "rag_test_token", time.Second)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	resp, err := client.Retrieve(context.Background(), RetrieveRequest{Query: "q", KBIDs: []uint64{1}, TopK: 1})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if resp.RequestID != "req-bare" || resp.Items == nil {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestClientRetrieveMapsUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Permission denied","request_id":"req-denied"}`))
	}))
	defer server.Close()

	client, err := New(server.URL, "rag_test_token", time.Second)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = client.Retrieve(context.Background(), RetrieveRequest{Query: "q", KBIDs: []uint64{1}, TopK: 1})
	if err == nil {
		t.Fatal("Retrieve() error = nil, want error")
	}
	upstreamErr, ok := err.(*UpstreamError)
	if !ok {
		t.Fatalf("error type = %T, want *UpstreamError", err)
	}
	if upstreamErr.Code != "forbidden" || upstreamErr.Retryable || upstreamErr.RequestID != "req-denied" {
		t.Fatalf("unexpected upstream error: %+v", upstreamErr)
	}
}

func TestMapStatus(t *testing.T) {
	tests := []struct {
		status    int
		code      string
		retryable bool
	}{
		{status: http.StatusBadRequest, code: "invalid_request"},
		{status: http.StatusUnauthorized, code: "unauthorized"},
		{status: http.StatusForbidden, code: "forbidden"},
		{status: http.StatusNotFound, code: "not_found"},
		{status: http.StatusTooManyRequests, code: "rate_limited", retryable: true},
		{status: http.StatusServiceUnavailable, code: "backend_unavailable", retryable: true},
		{status: http.StatusGatewayTimeout, code: "backend_timeout", retryable: true},
		{status: http.StatusInternalServerError, code: "backend_error", retryable: true},
	}

	for _, tt := range tests {
		code, retryable := mapStatus(tt.status)
		if code != tt.code || retryable != tt.retryable {
			t.Fatalf("mapStatus(%d) = (%q, %t), want (%q, %t)", tt.status, code, retryable, tt.code, tt.retryable)
		}
	}
}

func TestClientRetrieveMapsTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(server.URL, "rag_test_token", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = client.Retrieve(context.Background(), RetrieveRequest{Query: "q", KBIDs: []uint64{1}, TopK: 1})
	if err == nil {
		t.Fatal("Retrieve() error = nil, want error")
	}
	upstreamErr, ok := err.(*UpstreamError)
	if !ok {
		t.Fatalf("error type = %T, want *UpstreamError", err)
	}
	if upstreamErr.Code != "backend_timeout" || !upstreamErr.Retryable {
		t.Fatalf("unexpected upstream error: %+v", upstreamErr)
	}
}
