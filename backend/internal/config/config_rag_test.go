package config

import "testing"

func baseValidRAGConfig() *Config {
	return &Config{
		RAG: RAGConfig{Enabled: true},
		Milvus: MilvusConfig{
			Address:        "localhost:19530",
			CollectionName: "documents",
		},
		Embedding: EmbeddingConfig{
			APIKey:     "test-key",
			Model:      "bge-m3",
			BaseURL:    "https://example.com/v1",
			Dimensions: 1024,
		},
	}
}

func TestValidateRAGPrerequisites_Disabled(t *testing.T) {
	cfg := &Config{
		RAG: RAGConfig{Enabled: false},
	}
	if err := cfg.ValidateRAGPrerequisites(); err != nil {
		t.Fatalf("expected nil error when rag is disabled, got: %v", err)
	}
}

func TestValidateRAGPrerequisites_MissingMilvusAddress(t *testing.T) {
	cfg := baseValidRAGConfig()
	cfg.Milvus.Address = ""
	if err := cfg.ValidateRAGPrerequisites(); err == nil {
		t.Fatal("expected error when Milvus.Address is empty")
	}
}

func TestValidateRAGPrerequisites_Valid(t *testing.T) {
	cfg := baseValidRAGConfig()
	if err := cfg.ValidateRAGPrerequisites(); err != nil {
		t.Fatalf("expected nil error for valid rag config, got: %v", err)
	}
}
