package config

import (
	"os"
	"path/filepath"
	"testing"
)

func baseValidRAGConfig() *Config {
	return &Config{
		RAG: RAGConfig{
			Enabled:     true,
			Environment: "dev",
			Thresholds: RAGThresholds{
				MaxRetryCount:     3,
				RetryBackoffMS:    500,
				RetrieveTimeoutMS: 3000,
				UserQPSLimit:      20,
			},
		},
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

func TestValidateRAGPrerequisites_ProdGuardThresholds(t *testing.T) {
	cfg := baseValidRAGConfig()
	cfg.RAG.FeatureFlags.EnableProdGuard = true
	cfg.RAG.Thresholds.RetrieveTimeoutMS = 0
	if err := cfg.ValidateRAGPrerequisites(); err == nil {
		t.Fatal("expected error when prod guard enabled and threshold is missing")
	}
}

func TestValidateRAGPrerequisites_Valid(t *testing.T) {
	cfg := baseValidRAGConfig()
	if err := cfg.ValidateRAGPrerequisites(); err != nil {
		t.Fatalf("expected nil error for valid rag config, got: %v", err)
	}
}

func TestValidateRAGPrerequisites_HybridWeightInvalid(t *testing.T) {
	cfg := baseValidRAGConfig()
	cfg.RAG.FeatureFlags.EnableHybridRetrieval = true
	cfg.RAG.Phase2.HybridDenseWeight = 0.9
	cfg.RAG.Phase2.HybridSparseWeight = 0.3
	cfg.RAG.Phase2.CandidateTopK = 10
	if err := cfg.ValidateRAGPrerequisites(); err == nil {
		t.Fatal("expected error when hybrid dense+sparse weights are invalid")
	}
}

func TestValidateRAGPrerequisites_DynamicTopKRangeInvalid(t *testing.T) {
	cfg := baseValidRAGConfig()
	cfg.RAG.FeatureFlags.EnableDynamicTopK = true
	cfg.RAG.Phase2.CandidateTopK = 6
	cfg.RAG.Phase2.MinTopK = 8
	cfg.RAG.Phase2.MaxTopK = 7
	if err := cfg.ValidateRAGPrerequisites(); err == nil {
		t.Fatal("expected error when dynamic topk min/max range is invalid")
	}
}

func TestValidateRAGPrerequisites_ReleaseStageInvalid(t *testing.T) {
	cfg := baseValidRAGConfig()
	cfg.RAG.Release.Enabled = true
	cfg.RAG.Release.Stage = "unknown"
	if err := cfg.ValidateRAGPrerequisites(); err == nil {
		t.Fatal("expected error when release stage is invalid")
	}
}

func TestLoadConfig_EnvOverlayAndOverride(t *testing.T) {
	tempDir := t.TempDir()
	basePath := filepath.Join(tempDir, "config.yaml")
	overlayPath := filepath.Join(tempDir, "config.staging.yaml")

	baseConfig := `
rag:
  enabled: true
  environment: dev
  feature_flags:
    enable_prod_guard: false
    enable_ingest_retry: false
    enable_retrieve_audit: true
  thresholds:
    max_retry_count: 3
    retry_backoff_ms: 500
    retrieve_timeout_ms: 3000
    user_qps_limit: 20
Milvus:
  Address: localhost:19530
  CollectionName: documents
Embedding:
  APIKey: test-key
  Model: bge-m3
  BaseURL: https://example.com/v1
  Dimensions: 1024
`
	overlayConfig := `
rag:
  thresholds:
    retrieve_timeout_ms: 9000
  feature_flags:
    enable_retrieve_audit: false
`

	if err := os.WriteFile(basePath, []byte(baseConfig), 0644); err != nil {
		t.Fatalf("failed to write base config: %v", err)
	}
	if err := os.WriteFile(overlayPath, []byte(overlayConfig), 0644); err != nil {
		t.Fatalf("failed to write overlay config: %v", err)
	}

	t.Setenv("APP_ENV", "staging")
	t.Setenv("RAG_RETRIEVE_TIMEOUT_MS", "7000")
	t.Setenv("RAG_RELEASE_ENABLED", "true")
	t.Setenv("RAG_RELEASE_STAGE", "small_flow")
	t.Setenv("RAG_RELEASE_CANARY_PERCENT", "15")
	t.Setenv("RAG_RELEASE_INTERNAL_ROLES", "admin,staff")
	t.Setenv("RAG_RELEASE_USER_ALLOWLIST", "1,3,5")

	cfg, err := LoadConfig(basePath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.RAG.Environment != "staging" {
		t.Fatalf("expected staging env, got %s", cfg.RAG.Environment)
	}
	if cfg.RAG.Thresholds.RetrieveTimeoutMS != 7000 {
		t.Fatalf("expected retrieve_timeout_ms override to 7000, got %d", cfg.RAG.Thresholds.RetrieveTimeoutMS)
	}
	if cfg.RAG.FeatureFlags.EnableRetrieveAudit {
		t.Fatalf("expected retrieve_audit to be false from overlay")
	}
	if !cfg.RAG.Release.Enabled || cfg.RAG.Release.Stage != "small_flow" || cfg.RAG.Release.CanaryPercent != 15 {
		t.Fatalf("expected release env overrides to apply, got %+v", cfg.RAG.Release)
	}
	if len(cfg.RAG.Release.InternalRoles) != 2 || len(cfg.RAG.Release.UserAllowlist) != 3 {
		t.Fatalf("expected release role/allowlist overrides to apply, got %+v", cfg.RAG.Release)
	}

	snapshotPath := filepath.Join(tempDir, "docs", "baseline", "phase1", "baseline_snapshot.json")
	if _, err := os.Stat(snapshotPath); err != nil {
		t.Fatalf("expected phase1 baseline snapshot to be created, got err: %v", err)
	}
}
