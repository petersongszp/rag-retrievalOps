package milvus

import (
	"strings"
	"testing"
)

func TestBuildDimensionScopedCollectionName(t *testing.T) {
	name := buildDimensionScopedCollectionName("documents", "doubao-embedding-text-240715", 2560)
	want := "documents_doubao_embedding_text_240715_dim2560"
	if name != want {
		t.Fatalf("expected %s, got %s", want, name)
	}
}

func TestBuildDimensionScopedCollectionNameLongModel(t *testing.T) {
	longModel := strings.Repeat("model-", 80)
	name := buildDimensionScopedCollectionName("knowledge_base", longModel, 3072)
	if len(name) > maxCollectionNameLen {
		t.Fatalf("expected collection name length <= %d, got %d", maxCollectionNameLen, len(name))
	}
	if !strings.Contains(name, "_dim3072") {
		t.Fatalf("expected collection name to contain dimension suffix, got %s", name)
	}
}

func TestSanitizeCollectionName(t *testing.T) {
	name := sanitizeCollectionName(" Knowledge Base/V2 ")
	if name != "knowledge_base_v2" {
		t.Fatalf("expected sanitized name knowledge_base_v2, got %s", name)
	}
}
