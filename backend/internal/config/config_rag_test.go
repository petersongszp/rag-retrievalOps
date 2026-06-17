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
	cfg.RAG.DocumentParser.Engine = ""
	cfg.RAG.DocumentParser.TimeoutMS = 0
	cfg.RAG.DocumentParser.StrictMode = false
	cfg.RAG.DocumentParser.SaveSidecar = false

	cfg.applyRAGDefaults()

	if cfg.RAG.DocumentParser.Provider != "http" {
		t.Fatalf("Provider = %q", cfg.RAG.DocumentParser.Provider)
	}
	if cfg.RAG.DocumentParser.Engine != "docling" {
		t.Fatalf("Engine = %q", cfg.RAG.DocumentParser.Engine)
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

func TestLoadConfig_DocumentParserEnvOverrides(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	baseConfig := `
rag:
  enabled: true
  environment: dev
  feature_flags:
    enable_prod_guard: false
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
	if err := os.WriteFile(configPath, []byte(baseConfig), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	t.Setenv("DOCUMENT_PARSER_PROVIDER", "http")
	t.Setenv("DOCUMENT_PARSER_ENGINE", "docling")
	t.Setenv("DOCUMENT_PARSER_ENDPOINT", "http://parser-provider:9000/parse")
	t.Setenv("DOCUMENT_PARSER_TIMEOUT_MS", "120000")
	t.Setenv("DOCUMENT_PARSER_STRICT_MODE", "false")
	t.Setenv("DOCUMENT_PARSER_SAVE_SIDECAR", "false")
	t.Setenv("OCR_PROVIDER", "http")
	t.Setenv("OCR_ENDPOINT", "http://paddleocr:9000/ocr")
	t.Setenv("OCR_TIMEOUT_MS", "45000")

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	parser := cfg.RAG.DocumentParser
	if parser.Provider != "http" {
		t.Fatalf("Provider = %q", parser.Provider)
	}
	if parser.Engine != "docling" {
		t.Fatalf("Engine = %q", parser.Engine)
	}
	if parser.Endpoint != "http://parser-provider:9000/parse" {
		t.Fatalf("Endpoint = %q", parser.Endpoint)
	}
	if parser.TimeoutMS != 120000 {
		t.Fatalf("TimeoutMS = %d", parser.TimeoutMS)
	}
	if parser.StrictMode {
		t.Fatalf("StrictMode should be overridden to false")
	}
	if parser.SaveSidecar {
		t.Fatalf("SaveSidecar should be overridden to false")
	}
	if parser.OCR.Provider != "http" {
		t.Fatalf("OCR.Provider = %q", parser.OCR.Provider)
	}
	if parser.OCR.Endpoint != "http://paddleocr:9000/ocr" {
		t.Fatalf("OCR.Endpoint = %q", parser.OCR.Endpoint)
	}
	if parser.OCR.TimeoutMS != 45000 {
		t.Fatalf("OCR.TimeoutMS = %d", parser.OCR.TimeoutMS)
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

func TestValidateRAGPrerequisites_SemanticCacheDisabledAllowsZeroValueConfig(t *testing.T) {
	cfg := baseValidRAGConfig()
	cfg.Redis.Addr = ""
	cfg.RAG.SemanticCache = RAGSemanticCacheConfig{}
	if err := cfg.ValidateRAGPrerequisites(); err != nil {
		t.Fatalf("expected semantic cache zero values to be ignored when feature is disabled, got: %v", err)
	}
}

func TestValidateRAGPrerequisites_SemanticCacheMissingRedis(t *testing.T) {
	cfg := baseValidRAGConfig()
	cfg.RAG.FeatureFlags.EnableSemanticCache = true
	cfg.Redis.Addr = ""
	if err := cfg.ValidateRAGPrerequisites(); err == nil {
		t.Fatal("expected error when semantic cache is enabled and redis addr is empty")
	}
}

func TestValidateRAGPrerequisites_SemanticCacheThresholdInvalid(t *testing.T) {
	cfg := baseValidRAGConfig()
	cfg.RAG.FeatureFlags.EnableSemanticCache = true
	cfg.Redis.Addr = "localhost:6379"
	cfg.RAG.SemanticCache = RAGSemanticCacheConfig{
		SimilarityThreshold: 1.2,
		TTLSeconds:          900,
		MaxCandidates:       20,
		MaxEntriesPerScope:  200,
	}
	if err := cfg.ValidateRAGPrerequisites(); err == nil {
		t.Fatal("expected error when semantic cache similarity threshold is invalid")
	}
}

func TestValidateRAGPrerequisites_SemanticCacheTTLInvalid(t *testing.T) {
	cfg := baseValidRAGConfig()
	cfg.RAG.FeatureFlags.EnableSemanticCache = true
	cfg.Redis.Addr = "localhost:6379"
	cfg.RAG.SemanticCache = RAGSemanticCacheConfig{
		SimilarityThreshold: 0.92,
		TTLSeconds:          0,
		MaxCandidates:       20,
		MaxEntriesPerScope:  200,
	}
	if err := cfg.ValidateRAGPrerequisites(); err == nil {
		t.Fatal("expected error when semantic cache ttl is invalid")
	}
}

func TestValidateRAGPrerequisites_SemanticCacheMaxCandidatesInvalid(t *testing.T) {
	cfg := baseValidRAGConfig()
	cfg.RAG.FeatureFlags.EnableSemanticCache = true
	cfg.Redis.Addr = "localhost:6379"
	cfg.RAG.SemanticCache = RAGSemanticCacheConfig{
		SimilarityThreshold: 0.92,
		TTLSeconds:          900,
		MaxCandidates:       0,
		MaxEntriesPerScope:  200,
	}
	if err := cfg.ValidateRAGPrerequisites(); err == nil {
		t.Fatal("expected error when semantic cache max_candidates is invalid")
	}
}

func TestValidateRAGPrerequisites_SemanticCacheMaxEntriesPerScopeInvalid(t *testing.T) {
	cfg := baseValidRAGConfig()
	cfg.RAG.FeatureFlags.EnableSemanticCache = true
	cfg.Redis.Addr = "localhost:6379"
	cfg.RAG.SemanticCache = RAGSemanticCacheConfig{
		SimilarityThreshold: 0.92,
		TTLSeconds:          900,
		MaxCandidates:       20,
		MaxEntriesPerScope:  0,
	}
	if err := cfg.ValidateRAGPrerequisites(); err == nil {
		t.Fatal("expected error when semantic cache max_entries_per_scope is invalid")
	}
}

func TestValidateRAGPrerequisites_SemanticCacheValid(t *testing.T) {
	cfg := baseValidRAGConfig()
	cfg.RAG.FeatureFlags.EnableSemanticCache = true
	cfg.Redis.Addr = "localhost:6379"
	cfg.RAG.SemanticCache = RAGSemanticCacheConfig{
		SimilarityThreshold: 0.92,
		TTLSeconds:          900,
		MaxCandidates:       20,
		MaxEntriesPerScope:  200,
	}
	if err := cfg.ValidateRAGPrerequisites(); err != nil {
		t.Fatalf("expected valid semantic cache config, got: %v", err)
	}
}

func TestValidateRAGPrerequisites_EmbeddingCacheDisabledAllowsZeroValueConfig(t *testing.T) {
	cfg := baseValidRAGConfig()
	cfg.Embedding.EnableCache = false
	cfg.Embedding.CacheTTLSeconds = 0
	cfg.Embedding.CacheMaxEntries = 0
	if err := cfg.ValidateRAGPrerequisites(); err != nil {
		t.Fatalf("expected embedding cache zero values to be ignored when feature is disabled, got: %v", err)
	}
}

func TestValidateRAGPrerequisites_EmbeddingCacheTTLInvalid(t *testing.T) {
	cfg := baseValidRAGConfig()
	cfg.Embedding.EnableCache = true
	cfg.Embedding.CacheTTLSeconds = -1
	cfg.Embedding.CacheMaxEntries = 1000
	if err := cfg.ValidateRAGPrerequisites(); err == nil {
		t.Fatal("expected error when embedding cache ttl is invalid")
	}
}

func TestValidateRAGPrerequisites_EmbeddingCacheMaxEntriesInvalid(t *testing.T) {
	cfg := baseValidRAGConfig()
	cfg.Embedding.EnableCache = true
	cfg.Embedding.CacheTTLSeconds = 1800
	cfg.Embedding.CacheMaxEntries = -1
	if err := cfg.ValidateRAGPrerequisites(); err == nil {
		t.Fatal("expected error when embedding cache max entries is invalid")
	}
}

func TestApplyRAGDefaults_EmbeddingCacheDefaultsWhenEnabled(t *testing.T) {
	cfg := baseValidRAGConfig()
	cfg.Embedding.EnableCache = true
	cfg.Embedding.CacheTTLSeconds = 0
	cfg.Embedding.CacheMaxEntries = 0

	cfg.applyRAGDefaults()

	if cfg.Embedding.CacheTTLSeconds != 1800 {
		t.Fatalf("expected embedding cache ttl default to 1800, got %d", cfg.Embedding.CacheTTLSeconds)
	}
	if cfg.Embedding.CacheMaxEntries != 2000 {
		t.Fatalf("expected embedding cache max entries default to 2000, got %d", cfg.Embedding.CacheMaxEntries)
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
    enable_semantic_cache: false
  thresholds:
    max_retry_count: 3
    retry_backoff_ms: 500
    retrieve_timeout_ms: 3000
    user_qps_limit: 20
  semantic_cache:
    similarity_threshold: 0.92
    ttl_seconds: 900
    max_candidates: 20
    max_entries_per_scope: 200
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
	t.Setenv("RAG_ENABLE_SEMANTIC_CACHE", "true")
	t.Setenv("RAG_SEMANTIC_CACHE_SIMILARITY_THRESHOLD", "0.95")
	t.Setenv("RAG_SEMANTIC_CACHE_TTL_SECONDS", "1200")
	t.Setenv("RAG_SEMANTIC_CACHE_MAX_CANDIDATES", "16")
	t.Setenv("RAG_SEMANTIC_CACHE_MAX_ENTRIES_PER_SCOPE", "80")

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
	if !cfg.RAG.FeatureFlags.EnableSemanticCache {
		t.Fatalf("expected semantic cache flag to be enabled from env override")
	}
	if cfg.RAG.SemanticCache.SimilarityThreshold != 0.95 ||
		cfg.RAG.SemanticCache.TTLSeconds != 1200 ||
		cfg.RAG.SemanticCache.MaxCandidates != 16 ||
		cfg.RAG.SemanticCache.MaxEntriesPerScope != 80 {
		t.Fatalf("expected semantic cache env overrides to apply, got %+v", cfg.RAG.SemanticCache)
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
	t.Setenv("ARK_EMBEDDING_API_TYPE", "multimodal")
	t.Setenv("EMBEDDING_BASE_URL", "https://example.com/v1")

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Embedding.Provider != "openai" {
		t.Fatalf("expected embedding provider to expand to openai, got %q", cfg.Embedding.Provider)
	}
	if cfg.Embedding.ArkAPIType != "" {
		t.Fatalf("expected openai provider to ignore ark API type env, got %q", cfg.Embedding.ArkAPIType)
	}
}

func TestLoadConfig_UsesArkEmbeddingAPITypeEnv(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	baseConfig := `
rag:
  enabled: false
  environment: dev
Embedding:
  Provider: ark
  APIKey: test-key
  Model: doubao-embedding-vision-251215
  BaseURL: https://ark.cn-beijing.volces.com/api/v3
  Dimensions: 2048
`
	if err := os.WriteFile(configPath, []byte(baseConfig), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	t.Setenv("ARK_EMBEDDING_API_TYPE", "multimodal")

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Embedding.ArkAPIType != "multimodal" {
		t.Fatalf("expected ark embedding API type to come from ARK_EMBEDDING_API_TYPE, got %q", cfg.Embedding.ArkAPIType)
	}
}

func TestLoadConfig_IgnoresGenericEmbeddingAPIType(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	baseConfig := `
rag:
  enabled: false
  environment: dev
Embedding:
  Provider: ark
  APIKey: test-key
  Model: doubao-embedding-vision-251215
  BaseURL: https://ark.cn-beijing.volces.com/api/v3
  Dimensions: 2048
`
	if err := os.WriteFile(configPath, []byte(baseConfig), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	t.Setenv("EMBEDDING_API_TYPE", "multi_modal")
	t.Setenv("ARK_EMBEDDING_API_TYPE", "")

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Embedding.ArkAPIType != "" {
		t.Fatalf("expected generic EMBEDDING_API_TYPE to be ignored, got %q", cfg.Embedding.ArkAPIType)
	}
}

func TestLoadConfig_EmbeddingCacheEnvOverrides(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	baseConfig := `
rag:
  enabled: false
  environment: dev
Embedding:
  APIKey: "test-key"
  Model: "bge-m3"
  BaseURL: "https://example.com/v1"
  Dimensions: 1024
  EnableCache: false
  CacheTTLSeconds: 300
  CacheMaxEntries: 50
`
	if err := os.WriteFile(configPath, []byte(baseConfig), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	t.Setenv("EMBEDDING_ENABLE_CACHE", "true")
	t.Setenv("EMBEDDING_CACHE_TTL_SECONDS", "2400")
	t.Setenv("EMBEDDING_CACHE_MAX_ENTRIES", "1500")

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if !cfg.Embedding.EnableCache {
		t.Fatalf("expected embedding cache flag to be enabled from env override")
	}
	if cfg.Embedding.CacheTTLSeconds != 2400 {
		t.Fatalf("expected embedding cache ttl override to apply, got %d", cfg.Embedding.CacheTTLSeconds)
	}
	if cfg.Embedding.CacheMaxEntries != 1500 {
		t.Fatalf("expected embedding cache max entries override to apply, got %d", cfg.Embedding.CacheMaxEntries)
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

func TestSemanticCacheContract(t *testing.T) {
	cfg := baseValidRAGConfig()
	contract := cfg.SemanticCacheContract()
	if contract.ResultPayload != "retrieve_result_only" {
		t.Fatalf("expected retrieve result payload contract, got %q", contract.ResultPayload)
	}
	if contract.TopKPolicy != "exact_topk_only" {
		t.Fatalf("expected exact topk policy, got %q", contract.TopKPolicy)
	}
	if len(contract.ScopeDimensions) != 4 {
		t.Fatalf("expected 4 scope dimensions, got %d", len(contract.ScopeDimensions))
	}
	if len(contract.BypassReasons) != 4 {
		t.Fatalf("expected 4 bypass reasons, got %d", len(contract.BypassReasons))
	}
}
