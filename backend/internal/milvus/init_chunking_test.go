package milvus

import (
	"testing"

	"interview-agents/internal/milvus/chunking"
)

func TestMilvusManagerReturnsConfiguredChunkingStrategy(t *testing.T) {
	strategy := chunking.NewMarkdownStrategy(nil)
	manager := &MilvusManager{
		ChunkingStrategy: strategy,
	}

	if manager.GetChunkingStrategy() != strategy {
		t.Fatalf("expected configured chunking strategy to be returned")
	}
}
