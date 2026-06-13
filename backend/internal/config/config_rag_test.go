package config

import (
	"os"
	"path/filepath"
	"testing"

	"interview-agents/internal/rag/phase3"
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

func TestRAGDocumentParserDefaults(t *testing.T) {
	cfg := baseValidRAGConfig()
	cfg.RAG.DocumentParser.Provider = ""
	cfg.RAG.DocumentParser.TimeoutMS = 0
	cfg.RAG.DocumentParser.StrictMode = false
	cfg.RAG.DocumentParser.SaveSidecar = false

	cfg.applyRAGDefaults()

	if cfg.RAG.DocumentParser.Provider != "http" {
		t.Fatalf("Provider = %q", cfg.RAG.DocumentParser.Provider)
	}
	if cfg.RAG.DocumentParser.TimeoutMS != 60000 {
		t.Fatalf("TimeoutMS = %d", cfg.RAG.DocumentParser.TimeoutMS)
	}
	if !cfg.RAG.DocumentParser.StrictMode {
		t.Fatalf("StrictMode should default to true")
	}
	if !cfg.RAG.DocumentParser.SaveSidecar {
		t.Fatalf("SaveSidecar should default to true")
	}
	if cfg.RAG.DocumentParser.OCR.Provider != "http" {
		t.Fatalf("OCR.Provider = %q", cfg.RAG.DocumentParser.OCR.Provider)
	}
	if cfg.RAG.DocumentParser.OCR.TimeoutMS != 30000 {
		t.Fatalf("OCR.TimeoutMS = %d", cfg.RAG.DocumentParser.OCR.TimeoutMS)
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

func TestValidateRAGPrerequisites_StrategicTopKRangeInvalid(t *testing.T) {
	cfg := baseValidRAGConfig()
	cfg.RAG.FeatureFlags.EnableStrategicTopK = true
	cfg.RAG.Phase3.StrategicTopKMinK = 9
	cfg.RAG.Phase3.StrategicTopKMaxK = 5
	cfg.RAG.Phase3.StrategicTopKBudgetRatio = 0.6
	if err := cfg.ValidateRAGPrerequisites(); err == nil {
		t.Fatal("expected error when strategic topk min/max range is invalid")
	}
}

func TestValidateRAGPrerequisites_CitationConsistencyMissingVersion(t *testing.T) {
	cfg := baseValidRAGConfig()
	cfg.RAG.FeatureFlags.EnableCitationConsistency = true
	cfg.RAG.Phase3.CitationCheckThreshold = 0.8
	cfg.RAG.Phase3.CitationCheckVersion = ""
	if err := cfg.ValidateRAGPrerequisites(); err == nil {
		t.Fatal("expected error when citation consistency version is missing")
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
	t.Setenv("RAG_ENABLE_PARENT_CHILD_RETRIEVAL", "true")
	t.Setenv("RAG_PARENT_CHILD_FILL_STRATEGY", "section_window")
	t.Setenv("RAG_STRATEGIC_TOPK_BUDGET_RATIO", "0.75")

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
	if !cfg.RAG.FeatureFlags.EnableParentChildRetrieval {
		t.Fatalf("expected parent-child retrieval flag to be enabled from env override")
	}
	if cfg.RAG.Phase3.ParentChildFillStrategy != "section_window" {
		t.Fatalf("expected parent child fill strategy override to apply, got %s", cfg.RAG.Phase3.ParentChildFillStrategy)
	}
	if cfg.RAG.Phase3.StrategicTopKBudgetRatio != 0.75 {
		t.Fatalf("expected strategic topk budget ratio override to apply, got %.2f", cfg.RAG.Phase3.StrategicTopKBudgetRatio)
	}

	snapshotPath := filepath.Join(tempDir, "docs", "baseline", "phase1", "baseline_snapshot.json")
	if _, err := os.Stat(snapshotPath); err != nil {
		t.Fatalf("expected phase1 baseline snapshot to be created, got err: %v", err)
	}
	phase2SnapshotPath := filepath.Join(tempDir, "docs", "baseline", "phase2", "baseline_snapshot.json")
	if _, err := os.Stat(phase2SnapshotPath); err != nil {
		t.Fatalf("expected phase2 baseline snapshot to be created, got err: %v", err)
	}
	phase3SnapshotPath := filepath.Join(tempDir, "docs", "baseline", "phase3", "baseline_snapshot.json")
	if _, err := os.Stat(phase3SnapshotPath); err != nil {
		t.Fatalf("expected phase3 baseline snapshot to be created, got err: %v", err)
	}
}

func TestLoadConfig_ExpandsEmbeddingProvider(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	baseConfig := `
rag:
  enabled: false
  environment: dev
Embedding:
  Provider: "${EMBEDDING_PROVIDER}"
  APIKey: "${EMBEDDING_API_KEY}"
  Model: "${EMBEDDING_MODEL}"
  BaseURL: "${EMBEDDING_BASE_URL}"
  Dimensions: 1024
`
	if err := os.WriteFile(configPath, []byte(baseConfig), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	t.Setenv("EMBEDDING_PROVIDER", "openai")
	t.Setenv("EMBEDDING_API_KEY", "test-key")
	t.Setenv("EMBEDDING_MODEL", "bge-m3")
	t.Setenv("EMBEDDING_BASE_URL", "https://example.com/v1")

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Embedding.Provider != "openai" {
		t.Fatalf("expected embedding provider to expand to openai, got %q", cfg.Embedding.Provider)
	}
}

func TestRAGFeatureFlagsPhase3StrategyFlags(t *testing.T) {
	flags := RAGFeatureFlags{
		EnableParentChildRetrieval: true,
		EnableStrategicTopK:        true,
		EnableEvidenceRefusal:      false,
		EnableCitationConsistency:  true,
		EnableDomainTerms:          false,
		EnableRouteSpecificRewrite: true,
		EnableModelAssistedRewrite: false,
	}

	got := flags.Phase3StrategyFlags()
	if len(got) != len(phase3.ManagedFeatureFlags()) {
		t.Fatalf("Phase3StrategyFlags len = %d, want %d", len(got), len(phase3.ManagedFeatureFlags()))
	}
	if !got[phase3.FlagParentChildRetrieval] || !got[phase3.FlagStrategicTopK] || !got[phase3.FlagCitationConsistency] {
		t.Fatalf("Phase3StrategyFlags missing enabled flags: %+v", got)
	}
	if got[phase3.FlagEvidenceRefusal] || got[phase3.FlagDomainTerms] || got[phase3.FlagModelAssistedRewrite] {
		t.Fatalf("Phase3StrategyFlags unexpected enabled flags: %+v", got)
	}
}

func TestRAGFeatureFlagsGetAndSetPhase3StrategyFlag(t *testing.T) {
	var flags RAGFeatureFlags

	if ok := flags.SetPhase3StrategyFlag(phase3.FlagModelAssistedRewrite, true); !ok {
		t.Fatal("SetPhase3StrategyFlag should accept managed flag")
	}
	value, ok := flags.GetPhase3StrategyFlag(phase3.FlagModelAssistedRewrite)
	if !ok || !value {
		t.Fatalf("GetPhase3StrategyFlag(%q) = (%t, %t), want (true, true)", phase3.FlagModelAssistedRewrite, value, ok)
	}

	if ok := flags.SetPhase3StrategyFlag("RAG_UNKNOWN_FLAG", true); ok {
		t.Fatal("SetPhase3StrategyFlag should reject unmanaged flag")
	}
	if _, ok := flags.GetPhase3StrategyFlag("RAG_UNKNOWN_FLAG"); ok {
		t.Fatal("GetPhase3StrategyFlag should reject unmanaged flag")
	}
}

func TestLoadConfig_Phase4FeatureFlagsFromEnv(t *testing.T) {
	tempDir := t.TempDir()
	basePath := filepath.Join(tempDir, "config.yaml")

	baseConfig := `
rag:
  enabled: true
  environment: dev
Milvus:
  Address: localhost:19530
  CollectionName: documents
Embedding:
  APIKey: test-key
  Model: bge-m3
  BaseURL: https://example.com/v1
  Dimensions: 1024
`
	if err := os.WriteFile(basePath, []byte(baseConfig), 0644); err != nil {
		t.Fatalf("failed to write base config: %v", err)
	}

	t.Setenv("RAG_ENABLE_EXPERIMENT_PLATFORM", "true")
	t.Setenv("RAG_ENABLE_INDEX_LIFECYCLE", "true")
	t.Setenv("RAG_ENABLE_COST_DASHBOARD", "true")
	t.Setenv("RAG_ENABLE_COMPLIANCE_AUDIT", "true")
	t.Setenv("RAG_ENABLE_WEEKLY_REPORT", "true")
	t.Setenv("RAG_ENABLE_MILVUS_OPS_TOOLING", "true")
	t.Setenv("RAG_ENABLE_COLLECTION_SWITCH_GUARD", "true")

	cfg, err := LoadConfig(basePath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if !cfg.RAG.FeatureFlags.EnableExperimentPlatform ||
		!cfg.RAG.FeatureFlags.EnableIndexLifecycle ||
		!cfg.RAG.FeatureFlags.EnableCostDashboard ||
		!cfg.RAG.FeatureFlags.EnableComplianceAudit ||
		!cfg.RAG.FeatureFlags.EnableWeeklyReport ||
		!cfg.RAG.FeatureFlags.EnableMilvusOpsTooling ||
		!cfg.RAG.FeatureFlags.EnableCollectionSwitchGuard {
		t.Fatalf("expected phase4 governance flags to load from env, got %+v", cfg.RAG.FeatureFlags)
	}
}

func TestLoadConfig_Phase4CanonicalFeatureFlagsFromEnv(t *testing.T) {
	tempDir := t.TempDir()
	basePath := filepath.Join(tempDir, "config.yaml")

	baseConfig := `
rag:
  enabled: true
  environment: dev
Milvus:
  Address: localhost:19530
  CollectionName: documents
Embedding:
  APIKey: test-key
  Model: bge-m3
  BaseURL: https://example.com/v1
  Dimensions: 1024
`
	if err := os.WriteFile(basePath, []byte(baseConfig), 0644); err != nil {
		t.Fatalf("failed to write base config: %v", err)
	}

	t.Setenv("RAG_ENABLE_COST_GOVERNANCE", "true")
	t.Setenv("RAG_ENABLE_AUDIT_CENTER", "true")
	t.Setenv("RAG_ENABLE_VECTOR_OPS", "true")
	t.Setenv("RAG_ENABLE_GOVERNANCE_ALERTS", "true")
	t.Setenv("RAG_ENABLE_WEEKLY_REPORT", "true")

	cfg, err := LoadConfig(basePath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if !cfg.RAG.FeatureFlags.EnableCostGovernance ||
		!cfg.RAG.FeatureFlags.EnableAuditCenter ||
		!cfg.RAG.FeatureFlags.EnableVectorOps ||
		!cfg.RAG.FeatureFlags.EnableGovernanceAlerts ||
		!cfg.RAG.FeatureFlags.EnableWeeklyReport {
		t.Fatalf("expected canonical phase4 flags to load from env, got %+v", cfg.RAG.FeatureFlags)
	}
	if !cfg.RAG.FeatureFlags.EnableCostDashboard ||
		!cfg.RAG.FeatureFlags.EnableComplianceAudit ||
		!cfg.RAG.FeatureFlags.EnableMilvusOpsTooling ||
		!cfg.RAG.FeatureFlags.EnableExperimentPlatform {
		t.Fatalf("expected canonical flags to normalize legacy aliases, got %+v", cfg.RAG.FeatureFlags)
	}
}
