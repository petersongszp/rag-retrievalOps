package kb

import (
	"encoding/json"
	"testing"

	"interview-agents/internal/model"
)

func TestComputeSnippetOffset(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		queryLower  string
		expected    int
	}{
		{
			name:       "exact match at start",
			content:    "Go语言特点",
			queryLower: "go语言特点",
			expected:   0,
		},
		{
			name:       "partial match in middle",
			content:    "Go语言有并发特性",
			queryLower: "并发",
			expected:   11,
		},
		{
			name:       "no match",
			content:    "Java虚拟机",
			queryLower: "go",
			expected:   -1,
		},
		{
			name:       "empty query",
			content:    "任意内容",
			queryLower: "",
			expected:   -1,
		},
		{
			name:       "empty content",
			content:    "",
			queryLower: "go",
			expected:   -1,
		},
		{
			name:       "both empty",
			content:    "",
			queryLower: "",
			expected:   -1,
		},
		{
			name:       "multi-word fallback matches first word",
			content:    "Go语言并发模型",
			queryLower: "并发 模型",
			expected:   8,
		},
		{
			name:       "multi-word fallback no match at all",
			content:    "Python解释器",
			queryLower: "go java",
			expected:   -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeSnippetOffset(tt.content, tt.queryLower)
			if result != tt.expected {
				t.Errorf("computeSnippetOffset(%q, %q) = %d, want %d", tt.content, tt.queryLower, result, tt.expected)
			}
		})
	}
}

func TestClassifyRetrieveResultStatus(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected model.RetrieveResultStatus
	}{
		{
			name:     "timeout status",
			input:    "timeout",
			expected: model.RetrieveResultStatusTimeout,
		},
		{
			name:     "error status",
			input:    "error",
			expected: model.RetrieveResultStatusError,
		},
		{
			name:     "success status",
			input:    "success",
			expected: model.RetrieveResultStatusSuccess,
		},
		{
			name:     "unknown falls back to success",
			input:    "unknown",
			expected: model.RetrieveResultStatusSuccess,
		},
		{
			name:     "empty string falls back to success",
			input:    "",
			expected: model.RetrieveResultStatusSuccess,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyRetrieveResultStatus(tt.input)
			if result != tt.expected {
				t.Errorf("classifyRetrieveResultStatus(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatKBIDs(t *testing.T) {
	tests := []struct {
		name     string
		input    []uint64
		expected string
	}{
		{
			name:     "empty slice",
			input:    []uint64{},
			expected: "",
		},
		{
			name:     "single id",
			input:    []uint64{1},
			expected: "1",
		},
		{
			name:     "multiple ids",
			input:    []uint64{1, 2, 3},
			expected: "1,2,3",
		},
		{
			name:     "large ids",
			input:    []uint64{100, 200, 300},
			expected: "100,200,300",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatKBIDs(tt.input)
			if result != tt.expected {
				t.Errorf("formatKBIDs(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCitationJSONSerialization(t *testing.T) {
	c := citation{
		KBID:          1,
		DocumentID:    42,
		ChunkID:       "chunk_001",
		FileName:      "intro.md",
		ChunkIndex:    3,
		SnippetOffset: 128,
	}

	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Failed to marshal citation: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal citation: %v", err)
	}

	requiredFields := []string{"kb_id", "document_id", "chunk_id", "file_name", "chunk_index", "snippet_offset"}
	for _, field := range requiredFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("citation JSON missing field: %s", field)
		}
	}

	if parsed["kb_id"].(float64) != 1 {
		t.Errorf("citation kb_id = %v, want 1", parsed["kb_id"])
	}
	if parsed["document_id"].(float64) != 42 {
		t.Errorf("citation document_id = %v, want 42", parsed["document_id"])
	}
	if parsed["chunk_id"].(string) != "chunk_001" {
		t.Errorf("citation chunk_id = %v, want chunk_001", parsed["chunk_id"])
	}
	if parsed["file_name"].(string) != "intro.md" {
		t.Errorf("citation file_name = %v, want intro.md", parsed["file_name"])
	}
	if parsed["chunk_index"].(float64) != 3 {
		t.Errorf("citation chunk_index = %v, want 3", parsed["chunk_index"])
	}
	if parsed["snippet_offset"].(float64) != 128 {
		t.Errorf("citation snippet_offset = %v, want 128", parsed["snippet_offset"])
	}
}

func TestCitationSnippetOffsetOmitEmpty(t *testing.T) {
	c := citation{
		KBID:       1,
		DocumentID: 42,
		ChunkID:    "chunk_001",
		FileName:   "intro.md",
		ChunkIndex: 3,
	}

	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Failed to marshal citation: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal citation: %v", err)
	}

	if _, ok := parsed["snippet_offset"]; ok {
		t.Errorf("citation JSON should omit snippet_offset when zero, but got: %v", parsed["snippet_offset"])
	}
}

func TestSourceJSONSerialization(t *testing.T) {
	s := source{
		Route:            "dense",
		Collection:       "knowledge_collection",
		RetrieverVersion: "v1",
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Failed to marshal source: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal source: %v", err)
	}

	requiredFields := []string{"route", "collection", "retriever_version"}
	for _, field := range requiredFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("source JSON missing field: %s", field)
		}
	}

	if parsed["route"].(string) != "dense" {
		t.Errorf("source route = %v, want dense", parsed["route"])
	}
	if parsed["collection"].(string) != "knowledge_collection" {
		t.Errorf("source collection = %v, want knowledge_collection", parsed["collection"])
	}
	if parsed["retriever_version"].(string) != "v1" {
		t.Errorf("source retriever_version = %v, want v1", parsed["retriever_version"])
	}
}

func TestRetrieveResponseJSONSerialization(t *testing.T) {
	resp := retrieveResponse{
		RequestID: "550e8400-e29b-41d4-a716-446655440000",
		Items: []retrieveItem{
			{
				Content: "Go语言有并发特性",
				Score:   0.85,
				Citation: citation{
					KBID:          1,
					DocumentID:    42,
					ChunkID:       "chunk_001",
					FileName:      "intro.md",
					ChunkIndex:    3,
					SnippetOffset: 0,
				},
				Source: source{
					Route:            "dense",
					Collection:       "knowledge_collection",
					RetrieverVersion: "v1",
				},
			},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal retrieveResponse: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal retrieveResponse: %v", err)
	}

	if _, ok := parsed["request_id"]; !ok {
		t.Error("retrieveResponse JSON missing field: request_id")
	}
	if parsed["request_id"].(string) != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("retrieveResponse request_id = %v, want 550e8400-e29b-41d4-a716-446655440000", parsed["request_id"])
	}

	items, ok := parsed["items"].([]interface{})
	if !ok {
		t.Fatal("retrieveResponse items is not an array")
	}
	if len(items) != 1 {
		t.Fatalf("retrieveResponse items length = %d, want 1", len(items))
	}

	item := items[0].(map[string]interface{})
	topLevelFields := []string{"content", "score", "citation", "source"}
	for _, field := range topLevelFields {
		if _, ok := item[field]; !ok {
			t.Errorf("retrieveItem JSON missing field: %s", field)
		}
	}

	cit := item["citation"].(map[string]interface{})
	citationFields := []string{"kb_id", "document_id", "chunk_id", "file_name", "chunk_index"}
	for _, field := range citationFields {
		if _, ok := cit[field]; !ok {
			t.Errorf("citation JSON missing field: %s", field)
		}
	}

	src := item["source"].(map[string]interface{})
	sourceFields := []string{"route", "collection", "retriever_version"}
	for _, field := range sourceFields {
		if _, ok := src[field]; !ok {
			t.Errorf("source JSON missing field: %s", field)
		}
	}
}

func TestRetrieveResponseEmptyItems(t *testing.T) {
	resp := retrieveResponse{
		RequestID: "test-req-id",
		Items:     []retrieveItem{},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if parsed["request_id"].(string) != "test-req-id" {
		t.Errorf("request_id = %v, want test-req-id", parsed["request_id"])
	}

	items, ok := parsed["items"].([]interface{})
	if !ok {
		t.Fatal("items is not an array")
	}
	if len(items) != 0 {
		t.Errorf("items length = %d, want 0", len(items))
	}
}
