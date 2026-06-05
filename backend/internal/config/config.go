package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"interview-agents/internal/rag/governance"
	"interview-agents/internal/rag/phase3"

	"gopkg.in/yaml.v3"
)

// Config 搴旂敤绋嬪簭閰嶇疆缁撴瀯
type Config struct {
	Host             string             `yaml:"host"`
	Port             int                `yaml:"port"`
	Database         DatabaseConfig     `yaml:"database"`
	Redis            RedisConfig        `yaml:"redis"`
	Hertz            HertzConfig        `yaml:"hertz"`
	Eino             EinoConfig         `yaml:"eino"`
	Interview        InterviewConfig    `yaml:"interview"`
	Security         SecurityConfig     `yaml:"security"`
	GoogleSearch     GoogleConfig       `yaml:"google_search"`
	OpenAI           OpenAIConfig       `yaml:"openai"`
	LLM              LLMConfig          `yaml:"llm"`
	RAG              RAGConfig          `yaml:"rag"`
	Embedding        EmbeddingConfig    `yaml:"Embedding"`
	Milvus           MilvusConfig       `yaml:"Milvus"`
	DocumentSplitter SplitterConfig     `yaml:"DocumentSplitter"`
	Wechat           WechatConfig       `yaml:"wechat"`       // 寰俊閰嶇疆
	GitHub           GitHubConfig       `yaml:"github"`       // GitHub OAuth 閰嶇疆
	GoogleOAuth      GoogleOAuthConfig  `yaml:"google_oauth"` // Google OAuth 閰嶇疆锛堥偖绠辩櫥褰曪級
	Feishu           FeishuConfig       `yaml:"feishu"`       // 椋炰功閰嶇疆
	Email            EmailConfig        `yaml:"email"`        // 閭欢閰嶇疆
	RateLimit        LLMRateLimitConfig `yaml:"rate_limit"`   // LLM API 闄愭祦閰嶇疆
	Payment          PaymentConfig      `yaml:"payment"`
	RAGPlatform      RAGPlatformConfig  `yaml:"rag_platform"` // RAG Platform client config
	ConfigVersion    string             `yaml:"-"`
}

// RAGPlatformConfig RAG Platform client config
type RAGPlatformConfig struct {
	Enabled      bool     `yaml:"enabled"`        // enable RAG Platform calls
	BaseURL      string   `yaml:"base_url"`       // RAG Platform address
	APIKey       string   `yaml:"api_key"`        // API Key
	AppID        string   `yaml:"app_id"`         // client app ID
	DefaultKBIDs []uint64 `yaml:"default_kb_ids"` // default knowledge base IDs
}

// PaymentConfig 鏀粯閰嶇疆
type PaymentConfig struct {
	Stripe            StripeConfig `yaml:"stripe"`
	PayPal            PayPalConfig `yaml:"paypal"`
	WebhookTimeout    string       `yaml:"webhook_timeout"`     // webhook 浜嬩欢鏃堕棿绐楀彛锛屽 "5m"
	AllowedReturnURLs []string     `yaml:"allowed_return_urls"` // success/cancel URL 鐧藉悕鍗?
}

// StripeConfig Stripe 閰嶇疆
type StripeConfig struct {
	SecretKey      string `yaml:"secret_key"`
	WebhookSecret  string `yaml:"webhook_secret"`
	PublishableKey string `yaml:"publishable_key"`
}

// PayPalConfig PayPal 閰嶇疆
type PayPalConfig struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	WebhookID    string `yaml:"webhook_id"`
	Sandbox      bool   `yaml:"sandbox"`
}

// GitHubConfig GitHub OAuth 閰嶇疆
type GitHubConfig struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	RedirectURL  string `yaml:"redirect_url"`
}

// GoogleOAuthConfig Google OAuth 閰嶇疆锛堥偖绠辩櫥褰曪級
type GoogleOAuthConfig struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	RedirectURL  string `yaml:"redirect_url"`
}

// EmailConfig 閭欢閰嶇疆
type EmailConfig struct {
	SMTPHost  string `yaml:"smtp_host"`
	SMTPPort  int    `yaml:"smtp_port"`
	SMTPUser  string `yaml:"smtp_user"`
	SMTPPass  string `yaml:"smtp_pass"`
	FromEmail string `yaml:"from_email"`
}

// WechatConfig 寰俊閰嶇疆
type WechatConfig struct {
	AppID       string `yaml:"app_id"`
	AppSecret   string `yaml:"app_secret"`
	RedirectURL string `yaml:"redirect_url"`
}

// FeishuConfig 椋炰功閰嶇疆
type FeishuConfig struct {
	WebhookURL string `yaml:"webhook_url"` // 椋炰功鏈哄櫒浜?Webhook URL
	Enabled    bool   `yaml:"enabled"`     // 鏄惁鍚敤椋炰功鍛婅
}

// CORSConfig CORS閰嶇疆
type CORSConfig struct {
	AllowOrigins     []string `yaml:"allow_origins"`
	AllowMethods     []string `yaml:"allow_methods"`
	AllowHeaders     []string `yaml:"allow_headers"`
	ExposeHeaders    []string `yaml:"expose_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
}

// DatabaseConfig 鏁版嵁搴撻厤缃?
type DatabaseConfig struct {
	Driver          string `yaml:"driver"`
	DSN             string `yaml:"dsn"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime string `yaml:"conn_max_lifetime"`
}

// RedisConfig Redis閰嶇疆
type RedisConfig struct {
	Addr         string `yaml:"addr"`
	Password     string `yaml:"password"`
	DB           int    `yaml:"db"`
	DialTimeout  string `yaml:"dial_timeout"`
	ReadTimeout  string `yaml:"read_timeout"`
	WriteTimeout string `yaml:"write_timeout"`
	PoolSize     int    `yaml:"pool_size"`
	MinIdleConns int    `yaml:"min_idle_conns"`
}

// HertzConfig Hertz妗嗘灦閰嶇疆锛堝惈 11.1 楂樺苟鍙戠浉鍏筹級
type HertzConfig struct {
	LogLevel     string          `yaml:"log_level"`
	LogPath      string          `yaml:"log_path"`
	ReadTimeout  string          `yaml:"read_timeout"`
	WriteTimeout string          `yaml:"write_timeout"`
	IdleTimeout  string          `yaml:"idle_timeout"`
	KeepAlive    bool            `yaml:"keep_alive"` // 11.1.1 杩炴帴姹狅細鏄惁寮€鍚?KeepAlive
	GOMAXPROCS   int             `yaml:"gomaxprocs"` // 11.1.2 鍗忕▼妯″瀷锛歅 鏁伴噺锛? 琛ㄧず涓嶈缃紙浣跨敤榛樿锛?
	RateLimit    RateLimitConfig `yaml:"rate_limit"` // 11.1.3 闄愭祦锛氭敮鎸佸垎甯冨紡 Redis 闄愭祦
}

// RateLimitConfig 闄愭祦閰嶇疆
type RateLimitConfig struct {
	RPS            int    `yaml:"rps"`              // 姣忕璇锋眰鏁?
	Burst          int    `yaml:"burst"`            // 绐佸彂瀹归噺
	UseRedis       bool   `yaml:"use_redis"`        // 鏄惁浣跨敤 Redis 鍒嗗竷寮忛檺娴?
	RedisKeyPrefix string `yaml:"redis_key_prefix"` // Redis 閿墠缂€锛岀┖鍒欑敤 "rl:"
}

// EinoConfig Eino妗嗘灦閰嶇疆
type EinoConfig struct {
	Model       string  `yaml:"model"`
	APIKey      string  `yaml:"api_key"`
	BaseURL     string  `yaml:"base_url"`
	MaxTokens   int     `yaml:"max_tokens"`
	Temperature float64 `yaml:"temperature"`
	RetryCount  int     `yaml:"retry_count"`
	RetryDelay  string  `yaml:"retry_delay"`
	// 11.2.3 鎺ㄧ悊鎴愭湰鎺у埗锛氭瘡鐢ㄦ埛姣忔棩 Token 閰嶉锛堝惈 Reasoning Model R1/o3 绛夛級锛? 琛ㄧず涓嶉檺鍒?
	TokenQuotaPerUserPerDay int `yaml:"token_quota_per_user_per_day"`
}

// InterviewConfig 闈㈣瘯绯荤粺閰嶇疆
type InterviewConfig struct {
	MaxDuration     string `yaml:"max_duration"`
	QuestionTimeout string `yaml:"question_timeout"`
	MaxQuestions    int    `yaml:"max_questions"`
	MinQuestions    int    `yaml:"min_questions"`
}

// SecurityConfig 瀹夊叏鎬ч厤缃?
type SecurityConfig struct {
	JWTSecret     string     `yaml:"jwt_secret"`
	JWTExpiration string     `yaml:"jwt_expiration"`
	CORS          CORSConfig `yaml:"cors"`
}

// GoogleConfig Google鎼滅储閰嶇疆
type GoogleConfig struct {
	APIKey         string `yaml:"api_key"`
	SearchEngineID string `yaml:"search_engine_id"`
}

// OpenAIConfig OpenAI閰嶇疆
type OpenAIConfig struct {
	APIKey    string `yaml:"api_key"`
	ModelName string `yaml:"model_name"`
	BaseURL   string `yaml:"base_url"`
}

// LLMConfig LLM 澶фā鍨嬮厤缃紙鍏ㄥ眬缁熶竴浣跨敤锛?
type LLMConfig struct {
	APIKey       string `yaml:"api_key"`
	BaseURL      string `yaml:"base_url"`
	ModelName    string `yaml:"model_name"`
	ProviderName string `yaml:"provider_name"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	DevAdminBypassEnabled  bool   `yaml:"dev_admin_bypass_enabled" env:"RAG_DEV_ADMIN_BYPASS_ENABLED"`
	JWTSecret              string `yaml:"jwt_secret" env:"JWT_SECRET"`
	AccessTokenTTL         string `yaml:"access_token_ttl" env:"ACCESS_TOKEN_TTL"`   // e.g. "2h"
	RefreshTokenTTL        string `yaml:"refresh_token_ttl" env:"REFRESH_TOKEN_TTL"` // e.g. "168h"
	BootstrapEnabled       bool   `yaml:"bootstrap_enabled" env:"BOOTSTRAP_ENABLED"`
	BootstrapAdminEmail    string `yaml:"bootstrap_admin_email" env:"BOOTSTRAP_ADMIN_EMAIL"`
	BootstrapAdminPassword string `yaml:"bootstrap_admin_password" env:"BOOTSTRAP_ADMIN_PASSWORD"`
	BootstrapAdminName     string `yaml:"bootstrap_admin_name" env:"BOOTSTRAP_ADMIN_NAME"`
	BootstrapTenantName    string `yaml:"bootstrap_tenant_name" env:"BOOTSTRAP_TENANT_NAME"`
}

// RAGConfig RAG 鑳藉姏鎬诲紑鍏?
type RAGConfig struct {
	Enabled       bool                   `yaml:"enabled"`
	Environment   string                 `yaml:"environment"`
	Auth          AuthConfig             `yaml:"auth"`
	FeatureFlags  RAGFeatureFlags        `yaml:"feature_flags"`
	Thresholds    RAGThresholds          `yaml:"thresholds"`
	SemanticCache RAGSemanticCacheConfig `yaml:"semantic_cache"`
	Phase2        RAGPhase2Config        `yaml:"phase2"`
	Phase3        RAGPhase3Config        `yaml:"phase3"`
	Release       RAGReleaseConfig       `yaml:"release"`
}

type RAGFeatureFlags struct {
	EnableProdGuard             bool `yaml:"enable_prod_guard"`
	EnableIngestRetry           bool `yaml:"enable_ingest_retry"`
	EnableRetrieveAudit         bool `yaml:"enable_retrieve_audit"`
	EnableHybridRetrieval       bool `yaml:"enable_hybrid_retrieval"`
	EnableQueryRewrite          bool `yaml:"enable_query_rewrite"`
	EnableDynamicTopK           bool `yaml:"enable_dynamic_topk"`
	EnableAdvancedRerank        bool `yaml:"enable_advanced_rerank"`
	EnableParentChildRetrieval  bool `yaml:"enable_parent_child_retrieval"`
	EnableStrategicTopK         bool `yaml:"enable_strategic_topk"`
	EnableEvidenceRefusal       bool `yaml:"enable_evidence_refusal"`
	EnableCitationConsistency   bool `yaml:"enable_citation_consistency"`
	EnableDomainTerms           bool `yaml:"enable_domain_terms"`
	EnableRouteSpecificRewrite  bool `yaml:"enable_route_specific_rewrite"`
	EnableModelAssistedRewrite  bool `yaml:"enable_model_assisted_rewrite"`
	EnableExperimentPlatform    bool `yaml:"enable_experiment_platform"`
	EnableIndexLifecycle        bool `yaml:"enable_index_lifecycle"`
	EnableCostDashboard         bool `yaml:"enable_cost_dashboard"`
	EnableComplianceAudit       bool `yaml:"enable_compliance_audit"`
	EnableWeeklyReport          bool `yaml:"enable_weekly_report"`
	EnableMilvusOpsTooling      bool `yaml:"enable_milvus_ops_tooling"`
	EnableCollectionSwitchGuard bool `yaml:"enable_collection_switch_guard"`
	EnableCostGovernance        bool `yaml:"enable_cost_governance"`
	EnableAuditCenter           bool `yaml:"enable_audit_center"`
	EnableVectorOps             bool `yaml:"enable_vector_ops"`
	EnableGovernanceAlerts      bool `yaml:"enable_governance_alerts"`
	EnableSemanticCache         bool `yaml:"enable_semantic_cache"`
}

type RAGSemanticCacheConfig struct {
	SimilarityThreshold float64 `yaml:"similarity_threshold"`
	TTLSeconds          int     `yaml:"ttl_seconds"`
	MaxCandidates       int     `yaml:"max_candidates"`
	MaxEntriesPerScope  int     `yaml:"max_entries_per_scope"`
}

type RAGSemanticCacheContract struct {
	ScopeDimensions []string `json:"scope_dimensions"`
	BypassReasons   []string `json:"bypass_reasons"`
	ResultPayload   string   `json:"result_payload"`
	TopKPolicy      string   `json:"topk_policy"`
}

func (f RAGFeatureFlags) Phase3StrategyFlags() map[string]bool {
	return map[string]bool{
		phase3.FlagParentChildRetrieval: f.EnableParentChildRetrieval,
		phase3.FlagStrategicTopK:        f.EnableStrategicTopK,
		phase3.FlagEvidenceRefusal:      f.EnableEvidenceRefusal,
		phase3.FlagCitationConsistency:  f.EnableCitationConsistency,
		phase3.FlagDomainTerms:          f.EnableDomainTerms,
		phase3.FlagRouteSpecificRewrite: f.EnableRouteSpecificRewrite,
		phase3.FlagModelAssistedRewrite: f.EnableModelAssistedRewrite,
	}
}

func (f RAGFeatureFlags) GetPhase3StrategyFlag(flagKey string) (bool, bool) {
	switch flagKey {
	case phase3.FlagParentChildRetrieval:
		return f.EnableParentChildRetrieval, true
	case phase3.FlagStrategicTopK:
		return f.EnableStrategicTopK, true
	case phase3.FlagEvidenceRefusal:
		return f.EnableEvidenceRefusal, true
	case phase3.FlagCitationConsistency:
		return f.EnableCitationConsistency, true
	case phase3.FlagDomainTerms:
		return f.EnableDomainTerms, true
	case phase3.FlagRouteSpecificRewrite:
		return f.EnableRouteSpecificRewrite, true
	case phase3.FlagModelAssistedRewrite:
		return f.EnableModelAssistedRewrite, true
	default:
		return false, false
	}
}

func (f *RAGFeatureFlags) normalizePhase4Aliases() {
	if f == nil {
		return
	}

	if f.EnableCostGovernance {
		f.EnableCostDashboard = true
	}
	if f.EnableCostDashboard {
		f.EnableCostGovernance = true
	}

	if f.EnableAuditCenter {
		f.EnableComplianceAudit = true
	}
	if f.EnableComplianceAudit {
		f.EnableAuditCenter = true
	}

	if f.EnableVectorOps {
		f.EnableMilvusOpsTooling = true
		f.EnableIndexLifecycle = true
		f.EnableCollectionSwitchGuard = true
	}
	if f.EnableMilvusOpsTooling || f.EnableIndexLifecycle || f.EnableCollectionSwitchGuard {
		f.EnableVectorOps = true
	}

	if f.EnableGovernanceAlerts {
		f.EnableExperimentPlatform = true
	}
	if f.EnableExperimentPlatform {
		f.EnableGovernanceAlerts = true
	}
}

func (f *RAGFeatureFlags) SetPhase3StrategyFlag(flagKey string, enabled bool) bool {
	if f == nil {
		return false
	}
	switch flagKey {
	case phase3.FlagParentChildRetrieval:
		f.EnableParentChildRetrieval = enabled
	case phase3.FlagStrategicTopK:
		f.EnableStrategicTopK = enabled
	case phase3.FlagEvidenceRefusal:
		f.EnableEvidenceRefusal = enabled
	case phase3.FlagCitationConsistency:
		f.EnableCitationConsistency = enabled
	case phase3.FlagDomainTerms:
		f.EnableDomainTerms = enabled
	case phase3.FlagRouteSpecificRewrite:
		f.EnableRouteSpecificRewrite = enabled
	case phase3.FlagModelAssistedRewrite:
		f.EnableModelAssistedRewrite = enabled
	default:
		return false
	}
	return true
}

type RAGThresholds struct {
	MaxRetryCount     int `yaml:"max_retry_count"`
	RetryBackoffMS    int `yaml:"retry_backoff_ms"`
	RetrieveTimeoutMS int `yaml:"retrieve_timeout_ms"`
	UserQPSLimit      int `yaml:"user_qps_limit"`
}

type RAGPhase2Config struct {
	HybridDenseWeight    float64 `yaml:"hybrid_dense_weight"`
	HybridSparseWeight   float64 `yaml:"hybrid_sparse_weight"`
	CandidateTopK        int     `yaml:"candidate_topk"`
	MinTopK              int     `yaml:"min_topk"`
	MaxTopK              int     `yaml:"max_topk"`
	TokenBudget          int     `yaml:"token_budget"`
	MinAnswerChunks      int     `yaml:"min_answer_chunks"`
	RewriteTimeoutMS     int     `yaml:"rewrite_timeout_ms"`
	RewriteMaxExpansions int     `yaml:"rewrite_max_expansions"`
	RerankTimeoutMS      int     `yaml:"rerank_timeout_ms"`
	RerankModel          string  `yaml:"rerank_model"`
}

type RAGPhase3Config struct {
	ParentChildFillStrategy     string  `yaml:"parent_child_fill_strategy"`
	ParentChildWindowSize       int     `yaml:"parent_child_window_size"`
	ParentChildMaxTokens        int     `yaml:"parent_child_max_tokens"`
	StrategicTopKMinK           int     `yaml:"strategic_topk_min_k"`
	StrategicTopKMaxK           int     `yaml:"strategic_topk_max_k"`
	StrategicTopKBudgetRatio    float64 `yaml:"strategic_topk_budget_ratio"`
	EvidenceMinRerankScore      float64 `yaml:"evidence_min_rerank_score"`
	EvidenceMinDensity          float64 `yaml:"evidence_min_density"`
	EvidenceMinCitationCoverage float64 `yaml:"evidence_min_citation_coverage"`
	CitationCheckThreshold      float64 `yaml:"citation_check_threshold"`
	CitationCheckVersion        string  `yaml:"citation_check_version"`
	DomainTermTimeoutMS         int     `yaml:"domain_term_timeout_ms"`
	ModelRewriteTimeoutMS       int     `yaml:"model_rewrite_timeout_ms"`
	ModelRewriteShadowRatio     float64 `yaml:"model_rewrite_shadow_ratio"`
}

type RAGReleaseConfig struct {
	Enabled       bool     `yaml:"enabled"`
	Stage         string   `yaml:"stage"`
	InternalRoles []string `yaml:"internal_roles"`
	CanaryPercent int      `yaml:"canary_percent"`
	BatchPercent  int      `yaml:"batch_percent"`
	UserAllowlist []uint   `yaml:"user_allowlist"`
}

// RateLimitModelConfig 鍗曚釜妯″瀷鐨勯檺娴侀厤缃?
type RateLimitModelConfig struct {
	RPM int `yaml:"rpm"` // 姣忓垎閽熸渶澶ц姹傛暟
	TPM int `yaml:"tpm"` // 姣忓垎閽熸渶澶?Token 鏁?
}

// LLMRateLimitConfig LLM API 闄愭祦閰嶇疆锛堜笌 Hertz 鐨?RateLimitConfig 鍖哄垎锛?
type LLMRateLimitConfig struct {
	Enabled    bool                            `yaml:"enabled"`
	DefaultRPM int                             `yaml:"default_rpm"` // 榛樿 RPM 闄愬埗
	DefaultTPM int                             `yaml:"default_tpm"` // 榛樿 TPM 闄愬埗
	Models     map[string]RateLimitModelConfig `yaml:"models"`      // 鎸夋ā鍨嬭嚜瀹氫箟闄愬埗
}

// Global 鍏ㄥ眬閰嶇疆瀹炰緥
var Global Config

// LoadConfig 浠庢枃浠跺姞杞介厤缃?
// 鍏堝 YAML 鍘熸枃鍋氱幆澧冨彉閲忔浛鎹紙${VAR} / $VAR锛夛紝鍐嶅弽搴忓垪鍖栧埌缁撴瀯浣撱€?
// 杩欐牱鎵€鏈夐厤缃瓧娈佃嚜鍔ㄦ敮鎸佺幆澧冨彉閲忔敞鍏ワ紝鏃犻渶鍦?ExpandEnv 涓€愬瓧娈电淮鎶ゃ€?
func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	expanded := expandEnvInBytes(data)

	var cfg Config
	if err = yaml.Unmarshal(expanded, &cfg); err != nil {
		return nil, err
	}

	env := normalizeRAGEnv(strings.TrimSpace(os.Getenv("APP_ENV")))
	if env == "" {
		env = normalizeRAGEnv(strings.TrimSpace(os.Getenv("RAG_ENV")))
	}
	if env == "" {
		env = normalizeRAGEnv(strings.TrimSpace(cfg.RAG.Environment))
	}
	if env == "" {
		env = "dev"
	}
	cfg.RAG.Environment = env

	loadedPaths := []string{configPath}
	for _, overlayPath := range buildConfigOverlayPaths(configPath, env) {
		overlayData, readErr := os.ReadFile(overlayPath)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				continue
			}
			return nil, fmt.Errorf("failed to read overlay config %s: %w", overlayPath, readErr)
		}
		overlayExpanded := expandEnvInBytes(overlayData)
		if err = yaml.Unmarshal(overlayExpanded, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse overlay config %s: %w", overlayPath, err)
		}
		loadedPaths = append(loadedPaths, overlayPath)
	}
	cfg.RAG.Environment = env

	if err := cfg.applyRAGEnvOverrides(); err != nil {
		return nil, err
	}
	cfg.RAG.FeatureFlags.normalizePhase4Aliases()
	cfg.applyRAGDefaults()
	cfg.ConfigVersion = cfg.buildConfigVersion()
	if err := cfg.writePhase1BaselineSnapshot(configPath); err != nil {
		return nil, err
	}
	if err := cfg.writePhase2BaselineSnapshot(configPath); err != nil {
		return nil, err
	}
	if err := cfg.writePhase3BaselineSnapshot(configPath); err != nil {
		return nil, err
	}

	Global = cfg
	log.Printf("閰嶇疆鍔犺浇鎴愬姛 env=%s files=%s version=%s", cfg.RAG.Environment, strings.Join(loadedPaths, ","), cfg.ConfigVersion)
	cfg.LogRAGSnapshot()
	return &cfg, nil
}

// ValidateRAGPrerequisites 鏍￠獙 RAG 鍚敤鏃剁殑鏈€灏忓繀濉」
func (c *Config) ValidateRAGPrerequisites() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	if !c.RAG.Enabled {
		return nil
	}

	if !isValidRAGEnv(c.RAG.Environment) {
		return fmt.Errorf("rag environment must be one of dev/staging/prod, got: %q", c.RAG.Environment)
	}

	if strings.TrimSpace(c.Milvus.Address) == "" {
		return fmt.Errorf("rag enabled but Milvus.Address is empty")
	}
	if strings.TrimSpace(c.Milvus.CollectionName) == "" {
		return fmt.Errorf("rag enabled but Milvus.CollectionName is empty")
	}
	if strings.TrimSpace(c.Embedding.Model) == "" {
		return fmt.Errorf("rag enabled but Embedding.Model is empty")
	}
	if strings.TrimSpace(c.Embedding.BaseURL) == "" {
		return fmt.Errorf("rag enabled but Embedding.BaseURL is empty")
	}
	if strings.TrimSpace(c.Embedding.APIKey) == "" &&
		(strings.TrimSpace(c.Embedding.AccessKey) == "" || strings.TrimSpace(c.Embedding.SecretKey) == "") {
		return fmt.Errorf("rag enabled but Embedding credential is missing: provide APIKey or AccessKey+SecretKey")
	}
	if c.Embedding.Dimensions <= 0 {
		return fmt.Errorf("rag enabled but Embedding.Dimensions must be > 0")
	}

	if c.RAG.FeatureFlags.EnableProdGuard {
		if c.RAG.Thresholds.MaxRetryCount <= 0 {
			return fmt.Errorf("rag prod guard enabled but rag.thresholds.max_retry_count must be > 0")
		}
		if c.RAG.Thresholds.RetryBackoffMS <= 0 {
			return fmt.Errorf("rag prod guard enabled but rag.thresholds.retry_backoff_ms must be > 0")
		}
		if c.RAG.Thresholds.RetrieveTimeoutMS <= 0 {
			return fmt.Errorf("rag prod guard enabled but rag.thresholds.retrieve_timeout_ms must be > 0")
		}
		if c.RAG.Thresholds.UserQPSLimit <= 0 {
			return fmt.Errorf("rag prod guard enabled but rag.thresholds.user_qps_limit must be > 0")
		}
	}
	if c.RAG.FeatureFlags.EnableHybridRetrieval {
		if c.RAG.Phase2.HybridDenseWeight <= 0 {
			return fmt.Errorf("rag phase2 hybrid enabled but rag.phase2.hybrid_dense_weight must be > 0")
		}
		if c.RAG.Phase2.HybridSparseWeight <= 0 {
			return fmt.Errorf("rag phase2 hybrid enabled but rag.phase2.hybrid_sparse_weight must be > 0")
		}
		weightSum := c.RAG.Phase2.HybridDenseWeight + c.RAG.Phase2.HybridSparseWeight
		if weightSum < 0.999 || weightSum > 1.001 {
			return fmt.Errorf("rag phase2 hybrid enabled but dense+sparse weight must be 1.0, got %.4f", weightSum)
		}
		if c.RAG.Phase2.CandidateTopK <= 0 {
			return fmt.Errorf("rag phase2 hybrid enabled but rag.phase2.candidate_topk must be > 0")
		}
	}
	if c.RAG.FeatureFlags.EnableDynamicTopK {
		if c.RAG.Phase2.MinTopK <= 0 {
			return fmt.Errorf("rag dynamic topk enabled but rag.phase2.min_topk must be > 0")
		}
		if c.RAG.Phase2.MaxTopK <= 0 {
			return fmt.Errorf("rag dynamic topk enabled but rag.phase2.max_topk must be > 0")
		}
		if c.RAG.Phase2.MinTopK > c.RAG.Phase2.MaxTopK {
			return fmt.Errorf("rag dynamic topk enabled but rag.phase2.min_topk (%d) > rag.phase2.max_topk (%d)", c.RAG.Phase2.MinTopK, c.RAG.Phase2.MaxTopK)
		}
		if c.RAG.Phase2.CandidateTopK < c.RAG.Phase2.MaxTopK {
			return fmt.Errorf("rag dynamic topk enabled but rag.phase2.candidate_topk (%d) < rag.phase2.max_topk (%d)", c.RAG.Phase2.CandidateTopK, c.RAG.Phase2.MaxTopK)
		}
		if c.RAG.Phase2.TokenBudget < 0 {
			return fmt.Errorf("rag dynamic topk enabled but rag.phase2.token_budget must be >= 0")
		}
		if c.RAG.Phase2.MinAnswerChunks <= 0 {
			return fmt.Errorf("rag dynamic topk enabled but rag.phase2.min_answer_chunks must be > 0")
		}
	}
	if c.RAG.FeatureFlags.EnableQueryRewrite {
		if c.RAG.Phase2.RewriteTimeoutMS <= 0 {
			return fmt.Errorf("rag query rewrite enabled but rag.phase2.rewrite_timeout_ms must be > 0")
		}
		if c.RAG.Phase2.RewriteMaxExpansions <= 0 {
			return fmt.Errorf("rag query rewrite enabled but rag.phase2.rewrite_max_expansions must be > 0")
		}
	}
	if c.RAG.FeatureFlags.EnableAdvancedRerank {
		if c.RAG.Phase2.RerankTimeoutMS <= 0 {
			return fmt.Errorf("rag advanced rerank enabled but rag.phase2.rerank_timeout_ms must be > 0")
		}
		if strings.TrimSpace(c.RAG.Phase2.RerankModel) == "" {
			return fmt.Errorf("rag advanced rerank enabled but rag.phase2.rerank_model is empty")
		}
	}
	if c.RAG.FeatureFlags.EnableParentChildRetrieval {
		if !isValidParentChildFillStrategy(c.RAG.Phase3.ParentChildFillStrategy) {
			return fmt.Errorf("rag parent-child retrieval enabled but rag.phase3.parent_child_fill_strategy must be one of parent_only/sibling_window/section_window/child_first_with_parent_summary, got %q", c.RAG.Phase3.ParentChildFillStrategy)
		}
		if c.RAG.Phase3.ParentChildWindowSize < 0 {
			return fmt.Errorf("rag parent-child retrieval enabled but rag.phase3.parent_child_window_size must be >= 0")
		}
		if c.RAG.Phase3.ParentChildMaxTokens <= 0 {
			return fmt.Errorf("rag parent-child retrieval enabled but rag.phase3.parent_child_max_tokens must be > 0")
		}
	}
	if c.RAG.FeatureFlags.EnableStrategicTopK {
		if c.RAG.Phase3.StrategicTopKMinK <= 0 {
			return fmt.Errorf("rag strategic topk enabled but rag.phase3.strategic_topk_min_k must be > 0")
		}
		if c.RAG.Phase3.StrategicTopKMaxK <= 0 {
			return fmt.Errorf("rag strategic topk enabled but rag.phase3.strategic_topk_max_k must be > 0")
		}
		if c.RAG.Phase3.StrategicTopKMinK > c.RAG.Phase3.StrategicTopKMaxK {
			return fmt.Errorf("rag strategic topk enabled but rag.phase3.strategic_topk_min_k (%d) > rag.phase3.strategic_topk_max_k (%d)", c.RAG.Phase3.StrategicTopKMinK, c.RAG.Phase3.StrategicTopKMaxK)
		}
		if c.RAG.Phase3.StrategicTopKBudgetRatio <= 0 || c.RAG.Phase3.StrategicTopKBudgetRatio > 1 {
			return fmt.Errorf("rag strategic topk enabled but rag.phase3.strategic_topk_budget_ratio must be within (0,1], got %.4f", c.RAG.Phase3.StrategicTopKBudgetRatio)
		}
	}
	if c.RAG.FeatureFlags.EnableEvidenceRefusal {
		if !isNormalizedRatio(c.RAG.Phase3.EvidenceMinRerankScore) {
			return fmt.Errorf("rag evidence refusal enabled but rag.phase3.evidence_min_rerank_score must be within [0,1], got %.4f", c.RAG.Phase3.EvidenceMinRerankScore)
		}
		if !isNormalizedRatio(c.RAG.Phase3.EvidenceMinDensity) {
			return fmt.Errorf("rag evidence refusal enabled but rag.phase3.evidence_min_density must be within [0,1], got %.4f", c.RAG.Phase3.EvidenceMinDensity)
		}
		if !isNormalizedRatio(c.RAG.Phase3.EvidenceMinCitationCoverage) {
			return fmt.Errorf("rag evidence refusal enabled but rag.phase3.evidence_min_citation_coverage must be within [0,1], got %.4f", c.RAG.Phase3.EvidenceMinCitationCoverage)
		}
	}
	if c.RAG.FeatureFlags.EnableCitationConsistency {
		if !isNormalizedRatio(c.RAG.Phase3.CitationCheckThreshold) {
			return fmt.Errorf("rag citation consistency enabled but rag.phase3.citation_check_threshold must be within [0,1], got %.4f", c.RAG.Phase3.CitationCheckThreshold)
		}
		if strings.TrimSpace(c.RAG.Phase3.CitationCheckVersion) == "" {
			return fmt.Errorf("rag citation consistency enabled but rag.phase3.citation_check_version is empty")
		}
	}
	if c.RAG.FeatureFlags.EnableDomainTerms {
		if c.RAG.Phase3.DomainTermTimeoutMS <= 0 {
			return fmt.Errorf("rag domain terms enabled but rag.phase3.domain_term_timeout_ms must be > 0")
		}
	}
	if c.RAG.FeatureFlags.EnableModelAssistedRewrite {
		if c.RAG.Phase3.ModelRewriteTimeoutMS <= 0 {
			return fmt.Errorf("rag model-assisted rewrite enabled but rag.phase3.model_rewrite_timeout_ms must be > 0")
		}
		if c.RAG.Phase3.ModelRewriteShadowRatio < 0 || c.RAG.Phase3.ModelRewriteShadowRatio > 1 {
			return fmt.Errorf("rag model-assisted rewrite enabled but rag.phase3.model_rewrite_shadow_ratio must be within [0,1], got %.4f", c.RAG.Phase3.ModelRewriteShadowRatio)
		}
	}
	if c.RAG.FeatureFlags.EnableSemanticCache {
		if strings.TrimSpace(c.Redis.Addr) == "" {
			return fmt.Errorf("rag semantic cache enabled but redis.addr is empty")
		}
		if c.RAG.SemanticCache.SimilarityThreshold <= 0 || c.RAG.SemanticCache.SimilarityThreshold > 1 {
			return fmt.Errorf("rag semantic cache enabled but rag.semantic_cache.similarity_threshold must be within (0,1], got %.4f", c.RAG.SemanticCache.SimilarityThreshold)
		}
		if c.RAG.SemanticCache.TTLSeconds <= 0 {
			return fmt.Errorf("rag semantic cache enabled but rag.semantic_cache.ttl_seconds must be > 0")
		}
		if c.RAG.SemanticCache.MaxCandidates <= 0 {
			return fmt.Errorf("rag semantic cache enabled but rag.semantic_cache.max_candidates must be > 0")
		}
		if c.RAG.SemanticCache.MaxEntriesPerScope <= 0 {
			return fmt.Errorf("rag semantic cache enabled but rag.semantic_cache.max_entries_per_scope must be > 0")
		}
	}
	if c.RAG.Release.Enabled {
		if !isValidRAGReleaseStage(c.RAG.Release.Stage) {
			return fmt.Errorf("rag release enabled but rag.release.stage must be one of phase1/internal/small_flow/batch/full, got %q", c.RAG.Release.Stage)
		}
		if c.RAG.Release.CanaryPercent < 0 || c.RAG.Release.CanaryPercent > 100 {
			return fmt.Errorf("rag release enabled but rag.release.canary_percent must be within [0,100], got %d", c.RAG.Release.CanaryPercent)
		}
		if c.RAG.Release.BatchPercent < 0 || c.RAG.Release.BatchPercent > 100 {
			return fmt.Errorf("rag release enabled but rag.release.batch_percent must be within [0,100], got %d", c.RAG.Release.BatchPercent)
		}
		if normalizeRAGReleaseStage(c.RAG.Release.Stage) == "small_flow" && c.RAG.Release.CanaryPercent <= 0 {
			return fmt.Errorf("rag release enabled but rag.release.canary_percent must be > 0 when stage=small_flow")
		}
		if normalizeRAGReleaseStage(c.RAG.Release.Stage) == "batch" && c.RAG.Release.BatchPercent <= 0 {
			return fmt.Errorf("rag release enabled but rag.release.batch_percent must be > 0 when stage=batch")
		}
	}

	return nil
}

func (c *Config) applyRAGDefaults() {
	if c == nil {
		return
	}
	if strings.TrimSpace(c.RAG.Environment) == "" {
		c.RAG.Environment = "dev"
	}
	if !c.RAG.FeatureFlags.EnableProdGuard || c.RAG.Environment != "prod" {
		if c.RAG.Thresholds.MaxRetryCount <= 0 {
			c.RAG.Thresholds.MaxRetryCount = 3
		}
		if c.RAG.Thresholds.RetryBackoffMS <= 0 {
			c.RAG.Thresholds.RetryBackoffMS = 500
		}
		if c.RAG.Thresholds.RetrieveTimeoutMS <= 0 {
			c.RAG.Thresholds.RetrieveTimeoutMS = 3000
		}
		if c.RAG.Thresholds.UserQPSLimit <= 0 {
			c.RAG.Thresholds.UserQPSLimit = 20
		}
	}
	if c.RAG.Phase2.HybridDenseWeight <= 0 {
		c.RAG.Phase2.HybridDenseWeight = 0.7
	}
	if c.RAG.Phase2.HybridSparseWeight <= 0 {
		c.RAG.Phase2.HybridSparseWeight = 0.3
	}
	if c.RAG.Phase2.CandidateTopK <= 0 {
		c.RAG.Phase2.CandidateTopK = 10
	}
	if c.RAG.Phase2.MinTopK <= 0 {
		c.RAG.Phase2.MinTopK = 3
	}
	if c.RAG.Phase2.MaxTopK <= 0 {
		c.RAG.Phase2.MaxTopK = 8
	}
	if c.RAG.Phase2.TokenBudget < 0 {
		c.RAG.Phase2.TokenBudget = 0
	}
	if c.RAG.Phase2.MinAnswerChunks <= 0 {
		c.RAG.Phase2.MinAnswerChunks = 2
	}
	if c.RAG.Phase2.RewriteTimeoutMS <= 0 {
		c.RAG.Phase2.RewriteTimeoutMS = 120
	}
	if c.RAG.Phase2.RewriteMaxExpansions <= 0 {
		c.RAG.Phase2.RewriteMaxExpansions = 3
	}
	if c.RAG.Phase2.RerankTimeoutMS <= 0 {
		c.RAG.Phase2.RerankTimeoutMS = 250
	}
	if strings.TrimSpace(c.RAG.Phase2.RerankModel) == "" {
		c.RAG.Phase2.RerankModel = "jaccard-v1"
	}
	if c.RAG.SemanticCache.SimilarityThreshold <= 0 {
		c.RAG.SemanticCache.SimilarityThreshold = 0.92
	}
	if c.RAG.SemanticCache.TTLSeconds <= 0 {
		c.RAG.SemanticCache.TTLSeconds = 900
	}
	if c.RAG.SemanticCache.MaxCandidates <= 0 {
		c.RAG.SemanticCache.MaxCandidates = 20
	}
	if c.RAG.SemanticCache.MaxEntriesPerScope <= 0 {
		c.RAG.SemanticCache.MaxEntriesPerScope = 200
	}
	if strings.TrimSpace(c.RAG.Release.Stage) == "" {
		c.RAG.Release.Stage = "full"
	}
	if len(c.RAG.Release.InternalRoles) == 0 {
		c.RAG.Release.InternalRoles = []string{"admin"}
	}
	if c.RAG.Release.CanaryPercent <= 0 {
		c.RAG.Release.CanaryPercent = 5
	}
	if c.RAG.Release.BatchPercent <= 0 {
		c.RAG.Release.BatchPercent = 25
	}
	if strings.TrimSpace(c.RAG.Phase3.ParentChildFillStrategy) == "" {
		c.RAG.Phase3.ParentChildFillStrategy = "section_window"
	}
	if c.RAG.Phase3.ParentChildWindowSize < 0 {
		c.RAG.Phase3.ParentChildWindowSize = 0
	}
	if c.RAG.Phase3.ParentChildMaxTokens <= 0 {
		c.RAG.Phase3.ParentChildMaxTokens = 1200
	}
	if c.RAG.Phase3.StrategicTopKMinK <= 0 {
		c.RAG.Phase3.StrategicTopKMinK = 2
	}
	if c.RAG.Phase3.StrategicTopKMaxK <= 0 {
		c.RAG.Phase3.StrategicTopKMaxK = 8
	}
	if c.RAG.Phase3.StrategicTopKBudgetRatio <= 0 {
		c.RAG.Phase3.StrategicTopKBudgetRatio = 0.6
	}
	if c.RAG.Phase3.EvidenceMinRerankScore <= 0 {
		c.RAG.Phase3.EvidenceMinRerankScore = 0.55
	}
	if c.RAG.Phase3.EvidenceMinDensity <= 0 {
		c.RAG.Phase3.EvidenceMinDensity = 0.2
	}
	if c.RAG.Phase3.EvidenceMinCitationCoverage <= 0 {
		c.RAG.Phase3.EvidenceMinCitationCoverage = 0.5
	}
	if c.RAG.Phase3.CitationCheckThreshold <= 0 {
		c.RAG.Phase3.CitationCheckThreshold = 0.7
	}
	if strings.TrimSpace(c.RAG.Phase3.CitationCheckVersion) == "" {
		c.RAG.Phase3.CitationCheckVersion = "phase3-citation-v1"
	}
	// Auth TTL defaults
	if strings.TrimSpace(c.RAG.Auth.AccessTokenTTL) == "" {
		c.RAG.Auth.AccessTokenTTL = "2h"
	}
	if strings.TrimSpace(c.RAG.Auth.RefreshTokenTTL) == "" {
		c.RAG.Auth.RefreshTokenTTL = "168h"
	}
	if c.RAG.Phase3.DomainTermTimeoutMS <= 0 {
		c.RAG.Phase3.DomainTermTimeoutMS = 80
	}
	if c.RAG.Phase3.ModelRewriteTimeoutMS <= 0 {
		c.RAG.Phase3.ModelRewriteTimeoutMS = 150
	}
	if c.RAG.Phase3.ModelRewriteShadowRatio <= 0 {
		c.RAG.Phase3.ModelRewriteShadowRatio = 0.1
	}
}

func (c *Config) applyRAGEnvOverrides() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	if value, ok, err := readEnvBool("RAG_ENABLED"); err != nil {
		return err
	} else if ok {
		c.RAG.Enabled = value
	}
	if value, ok, err := readEnvBool("RAG_ENABLE_PROD_GUARD"); err != nil {
		return err
	} else if ok {
		c.RAG.FeatureFlags.EnableProdGuard = value
	}
	if value, ok, err := readEnvBool("RAG_ENABLE_INGEST_RETRY"); err != nil {
		return err
	} else if ok {
		c.RAG.FeatureFlags.EnableIngestRetry = value
	}
	if value, ok, err := readEnvBool("RAG_ENABLE_RETRIEVE_AUDIT"); err != nil {
		return err
	} else if ok {
		c.RAG.FeatureFlags.EnableRetrieveAudit = value
	}
	if value, ok, err := readEnvBool("RAG_ENABLE_HYBRID_RETRIEVAL"); err != nil {
		return err
	} else if ok {
		c.RAG.FeatureFlags.EnableHybridRetrieval = value
	}
	if value, ok, err := readEnvBool("RAG_ENABLE_QUERY_REWRITE"); err != nil {
		return err
	} else if ok {
		c.RAG.FeatureFlags.EnableQueryRewrite = value
	}
	if value, ok, err := readEnvBool("RAG_ENABLE_DYNAMIC_TOPK"); err != nil {
		return err
	} else if ok {
		c.RAG.FeatureFlags.EnableDynamicTopK = value
	}
	if value, ok, err := readEnvBool("RAG_ENABLE_ADVANCED_RERANK"); err != nil {
		return err
	} else if ok {
		c.RAG.FeatureFlags.EnableAdvancedRerank = value
	}
	if value, ok, err := readEnvBool("RAG_ENABLE_PARENT_CHILD_RETRIEVAL"); err != nil {
		return err
	} else if ok {
		c.RAG.FeatureFlags.EnableParentChildRetrieval = value
	}
	if value, ok, err := readEnvBool("RAG_ENABLE_STRATEGIC_TOPK"); err != nil {
		return err
	} else if ok {
		c.RAG.FeatureFlags.EnableStrategicTopK = value
	}
	if value, ok, err := readEnvBool("RAG_ENABLE_EVIDENCE_REFUSAL"); err != nil {
		return err
	} else if ok {
		c.RAG.FeatureFlags.EnableEvidenceRefusal = value
	}
	if value, ok, err := readEnvBool("RAG_ENABLE_CITATION_CONSISTENCY"); err != nil {
		return err
	} else if ok {
		c.RAG.FeatureFlags.EnableCitationConsistency = value
	}
	if value, ok, err := readEnvBool("RAG_ENABLE_DOMAIN_TERMS"); err != nil {
		return err
	} else if ok {
		c.RAG.FeatureFlags.EnableDomainTerms = value
	}
	if value, ok, err := readEnvBool("RAG_ENABLE_ROUTE_SPECIFIC_REWRITE"); err != nil {
		return err
	} else if ok {
		c.RAG.FeatureFlags.EnableRouteSpecificRewrite = value
	}
	if value, ok, err := readEnvBool("RAG_ENABLE_MODEL_ASSISTED_REWRITE"); err != nil {
		return err
	} else if ok {
		c.RAG.FeatureFlags.EnableModelAssistedRewrite = value
	}
	if value, ok, err := readEnvBoolAny(governance.FlagGovernanceAlerts, governance.FlagExperimentPlatform); err != nil {
		return err
	} else if ok {
		c.RAG.FeatureFlags.EnableExperimentPlatform = value
		c.RAG.FeatureFlags.EnableGovernanceAlerts = value
	}
	if value, ok, err := readEnvBoolAny(governance.FlagVectorOps, governance.FlagIndexLifecycle); err != nil {
		return err
	} else if ok {
		c.RAG.FeatureFlags.EnableIndexLifecycle = value
		c.RAG.FeatureFlags.EnableVectorOps = value
	}
	if value, ok, err := readEnvBoolAny(governance.FlagCostGovernance, governance.FlagCostDashboard); err != nil {
		return err
	} else if ok {
		c.RAG.FeatureFlags.EnableCostDashboard = value
		c.RAG.FeatureFlags.EnableCostGovernance = value
	}
	if value, ok, err := readEnvBoolAny(governance.FlagAuditCenter, governance.FlagComplianceAudit); err != nil {
		return err
	} else if ok {
		c.RAG.FeatureFlags.EnableComplianceAudit = value
		c.RAG.FeatureFlags.EnableAuditCenter = value
	}
	if value, ok, err := readEnvBool(governance.FlagWeeklyReport); err != nil {
		return err
	} else if ok {
		c.RAG.FeatureFlags.EnableWeeklyReport = value
	}
	if value, ok, err := readEnvBoolAny(governance.FlagVectorOps, governance.FlagMilvusOpsTooling); err != nil {
		return err
	} else if ok {
		c.RAG.FeatureFlags.EnableMilvusOpsTooling = value
		c.RAG.FeatureFlags.EnableVectorOps = value
	}
	if value, ok, err := readEnvBoolAny(governance.FlagVectorOps, governance.FlagCollectionSwitchGuard); err != nil {
		return err
	} else if ok {
		c.RAG.FeatureFlags.EnableCollectionSwitchGuard = value
		c.RAG.FeatureFlags.EnableVectorOps = value
	}
	if value, ok, err := readEnvBool("RAG_ENABLE_SEMANTIC_CACHE"); err != nil {
		return err
	} else if ok {
		c.RAG.FeatureFlags.EnableSemanticCache = value
	}
	if value, ok, err := readEnvInt("RAG_MAX_RETRY_COUNT"); err != nil {
		return err
	} else if ok {
		c.RAG.Thresholds.MaxRetryCount = value
	}
	if value, ok, err := readEnvInt("RAG_RETRY_BACKOFF_MS"); err != nil {
		return err
	} else if ok {
		c.RAG.Thresholds.RetryBackoffMS = value
	}
	if value, ok, err := readEnvInt("RAG_RETRIEVE_TIMEOUT_MS"); err != nil {
		return err
	} else if ok {
		c.RAG.Thresholds.RetrieveTimeoutMS = value
	}
	if value, ok, err := readEnvInt("RAG_USER_QPS_LIMIT"); err != nil {
		return err
	} else if ok {
		c.RAG.Thresholds.UserQPSLimit = value
	}
	if value, ok, err := readEnvFloat64("RAG_SEMANTIC_CACHE_SIMILARITY_THRESHOLD"); err != nil {
		return err
	} else if ok {
		c.RAG.SemanticCache.SimilarityThreshold = value
	}
	if value, ok, err := readEnvInt("RAG_SEMANTIC_CACHE_TTL_SECONDS"); err != nil {
		return err
	} else if ok {
		c.RAG.SemanticCache.TTLSeconds = value
	}
	if value, ok, err := readEnvInt("RAG_SEMANTIC_CACHE_MAX_CANDIDATES"); err != nil {
		return err
	} else if ok {
		c.RAG.SemanticCache.MaxCandidates = value
	}
	if value, ok, err := readEnvInt("RAG_SEMANTIC_CACHE_MAX_ENTRIES_PER_SCOPE"); err != nil {
		return err
	} else if ok {
		c.RAG.SemanticCache.MaxEntriesPerScope = value
	}
	if value, ok, err := readEnvFloat64("RAG_HYBRID_DENSE_WEIGHT"); err != nil {
		return err
	} else if ok {
		c.RAG.Phase2.HybridDenseWeight = value
	}
	if value, ok, err := readEnvFloat64("RAG_HYBRID_SPARSE_WEIGHT"); err != nil {
		return err
	} else if ok {
		c.RAG.Phase2.HybridSparseWeight = value
	}
	if value, ok, err := readEnvInt("RAG_CANDIDATE_TOPK"); err != nil {
		return err
	} else if ok {
		c.RAG.Phase2.CandidateTopK = value
	}
	if value, ok, err := readEnvInt("RAG_MIN_TOPK"); err != nil {
		return err
	} else if ok {
		c.RAG.Phase2.MinTopK = value
	}
	if value, ok, err := readEnvInt("RAG_MAX_TOPK"); err != nil {
		return err
	} else if ok {
		c.RAG.Phase2.MaxTopK = value
	}
	if value, ok, err := readEnvInt("RAG_TOKEN_BUDGET"); err != nil {
		return err
	} else if ok {
		c.RAG.Phase2.TokenBudget = value
	}
	if value, ok, err := readEnvInt("RAG_MIN_ANSWER_CHUNKS"); err != nil {
		return err
	} else if ok {
		c.RAG.Phase2.MinAnswerChunks = value
	}
	if value, ok, err := readEnvInt("RAG_REWRITE_TIMEOUT_MS"); err != nil {
		return err
	} else if ok {
		c.RAG.Phase2.RewriteTimeoutMS = value
	}
	if value, ok, err := readEnvInt("RAG_REWRITE_MAX_EXPANSIONS"); err != nil {
		return err
	} else if ok {
		c.RAG.Phase2.RewriteMaxExpansions = value
	}
	if value, ok, err := readEnvInt("RAG_RERANK_TIMEOUT_MS"); err != nil {
		return err
	} else if ok {
		c.RAG.Phase2.RerankTimeoutMS = value
	}
	if value, ok := os.LookupEnv("RAG_RERANK_MODEL"); ok {
		c.RAG.Phase2.RerankModel = strings.TrimSpace(value)
	}
	if value, ok := os.LookupEnv("RAG_PARENT_CHILD_FILL_STRATEGY"); ok {
		c.RAG.Phase3.ParentChildFillStrategy = strings.TrimSpace(value)
	}
	if value, ok, err := readEnvInt("RAG_PARENT_CHILD_WINDOW_SIZE"); err != nil {
		return err
	} else if ok {
		c.RAG.Phase3.ParentChildWindowSize = value
	}
	if value, ok, err := readEnvInt("RAG_PARENT_CHILD_MAX_TOKENS"); err != nil {
		return err
	} else if ok {
		c.RAG.Phase3.ParentChildMaxTokens = value
	}
	if value, ok, err := readEnvInt("RAG_STRATEGIC_TOPK_MIN_K"); err != nil {
		return err
	} else if ok {
		c.RAG.Phase3.StrategicTopKMinK = value
	}
	if value, ok, err := readEnvInt("RAG_STRATEGIC_TOPK_MAX_K"); err != nil {
		return err
	} else if ok {
		c.RAG.Phase3.StrategicTopKMaxK = value
	}
	if value, ok, err := readEnvFloat64("RAG_STRATEGIC_TOPK_BUDGET_RATIO"); err != nil {
		return err
	} else if ok {
		c.RAG.Phase3.StrategicTopKBudgetRatio = value
	}
	if value, ok, err := readEnvFloat64("RAG_EVIDENCE_MIN_RERANK_SCORE"); err != nil {
		return err
	} else if ok {
		c.RAG.Phase3.EvidenceMinRerankScore = value
	}
	if value, ok, err := readEnvFloat64("RAG_EVIDENCE_MIN_DENSITY"); err != nil {
		return err
	} else if ok {
		c.RAG.Phase3.EvidenceMinDensity = value
	}
	if value, ok, err := readEnvFloat64("RAG_EVIDENCE_MIN_CITATION_COVERAGE"); err != nil {
		return err
	} else if ok {
		c.RAG.Phase3.EvidenceMinCitationCoverage = value
	}
	if value, ok, err := readEnvFloat64("RAG_CITATION_CHECK_THRESHOLD"); err != nil {
		return err
	} else if ok {
		c.RAG.Phase3.CitationCheckThreshold = value
	}
	if value, ok := os.LookupEnv("RAG_CITATION_CHECK_VERSION"); ok {
		c.RAG.Phase3.CitationCheckVersion = strings.TrimSpace(value)
	}
	if value, ok, err := readEnvInt("RAG_DOMAIN_TERM_TIMEOUT_MS"); err != nil {
		return err
	} else if ok {
		c.RAG.Phase3.DomainTermTimeoutMS = value
	}
	if value, ok, err := readEnvInt("RAG_MODEL_REWRITE_TIMEOUT_MS"); err != nil {
		return err
	} else if ok {
		c.RAG.Phase3.ModelRewriteTimeoutMS = value
	}
	if value, ok, err := readEnvFloat64("RAG_MODEL_REWRITE_SHADOW_RATIO"); err != nil {
		return err
	} else if ok {
		c.RAG.Phase3.ModelRewriteShadowRatio = value
	}
	if value, ok, err := readEnvBool("RAG_RELEASE_ENABLED"); err != nil {
		return err
	} else if ok {
		c.RAG.Release.Enabled = value
	}
	if value, ok := os.LookupEnv("RAG_RELEASE_STAGE"); ok {
		c.RAG.Release.Stage = strings.TrimSpace(value)
	}
	if value, ok, err := readEnvInt("RAG_RELEASE_CANARY_PERCENT"); err != nil {
		return err
	} else if ok {
		c.RAG.Release.CanaryPercent = value
	}
	if value, ok, err := readEnvInt("RAG_RELEASE_BATCH_PERCENT"); err != nil {
		return err
	} else if ok {
		c.RAG.Release.BatchPercent = value
	}
	if value, ok := os.LookupEnv("RAG_RELEASE_INTERNAL_ROLES"); ok {
		c.RAG.Release.InternalRoles = readEnvCSVStrings(value)
	}
	if values, ok, err := readEnvUintSlice("RAG_RELEASE_USER_ALLOWLIST"); err != nil {
		return err
	} else if ok {
		c.RAG.Release.UserAllowlist = values
	}
	// Auth env overrides
	if value, ok, err := readEnvBool("RAG_DEV_ADMIN_BYPASS_ENABLED"); err != nil {
		return err
	} else if ok {
		c.RAG.Auth.DevAdminBypassEnabled = value
	}
	if value, ok := os.LookupEnv("JWT_SECRET"); ok {
		c.RAG.Auth.JWTSecret = strings.TrimSpace(value)
	}
	if value, ok := os.LookupEnv("ACCESS_TOKEN_TTL"); ok {
		c.RAG.Auth.AccessTokenTTL = strings.TrimSpace(value)
	}
	if value, ok := os.LookupEnv("REFRESH_TOKEN_TTL"); ok {
		c.RAG.Auth.RefreshTokenTTL = strings.TrimSpace(value)
	}
	if value, ok, err := readEnvBool("BOOTSTRAP_ENABLED"); err != nil {
		return err
	} else if ok {
		c.RAG.Auth.BootstrapEnabled = value
	}
	if value, ok := os.LookupEnv("BOOTSTRAP_ADMIN_EMAIL"); ok {
		c.RAG.Auth.BootstrapAdminEmail = strings.TrimSpace(value)
	}
	if value, ok := os.LookupEnv("BOOTSTRAP_ADMIN_PASSWORD"); ok {
		c.RAG.Auth.BootstrapAdminPassword = strings.TrimSpace(value)
	}
	if value, ok := os.LookupEnv("BOOTSTRAP_ADMIN_NAME"); ok {
		c.RAG.Auth.BootstrapAdminName = strings.TrimSpace(value)
	}
	if value, ok := os.LookupEnv("BOOTSTRAP_TENANT_NAME"); ok {
		c.RAG.Auth.BootstrapTenantName = strings.TrimSpace(value)
	}
	return nil
}

// GetAccessTokenTTL 解析 access token TTL 配置，返回 time.Duration
func (c *AuthConfig) GetAccessTokenTTL() time.Duration {
	ttl := strings.TrimSpace(c.AccessTokenTTL)
	if ttl == "" {
		ttl = "2h"
	}
	d, err := time.ParseDuration(ttl)
	if err != nil {
		return 2 * time.Hour
	}
	return d
}

// GetRefreshTokenTTL 解析 refresh token TTL 配置，返回 time.Duration
func (c *AuthConfig) GetRefreshTokenTTL() time.Duration {
	ttl := strings.TrimSpace(c.RefreshTokenTTL)
	if ttl == "" {
		ttl = "168h"
	}
	d, err := time.ParseDuration(ttl)
	if err != nil {
		return 168 * time.Hour
	}
	return d
}

// ValidateAuthProductionSafety 生产环境认证安全检查
func (c *Config) ValidateAuthProductionSafety() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	if c.RAG.Environment != "prod" {
		return nil
	}
	if c.RAG.Auth.DevAdminBypassEnabled {
		return fmt.Errorf("[FATAL] Dev admin bypass cannot be enabled in production")
	}
	if c.RAG.Auth.JWTSecret == "" || c.RAG.Auth.JWTSecret == "your-jwt-secret-key-here" {
		return fmt.Errorf("[FATAL] JWT secret must be set in production")
	}
	if c.RAG.Auth.BootstrapEnabled {
		return fmt.Errorf("[FATAL] Bootstrap cannot be enabled in production")
	}
	return nil
}

func (c *Config) buildConfigVersion() string {
	data, err := yaml.Marshal(c)
	if err != nil {
		return "unknown"
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8])
}

func (c *Config) LogRAGSnapshot() {
	if c == nil {
		return
	}
	semanticCacheContract := c.SemanticCacheContract()
	log.Printf(
		"[RAG:L0] snapshot version=%s strategy_digest=%s env=%s enabled=%t flags={prod_guard:%t ingest_retry:%t retrieve_audit:%t hybrid:%t rewrite:%t dynamic_topk:%t adv_rerank:%t parent_child:%t strategic_topk:%t evidence_refusal:%t citation_consistency:%t domain_terms:%t route_specific_rewrite:%t model_assisted_rewrite:%t experiment_platform:%t index_lifecycle:%t cost_dashboard:%t compliance_audit:%t weekly_report:%t milvus_ops_tooling:%t collection_switch_guard:%t cost_governance:%t audit_center:%t vector_ops:%t governance_alerts:%t semantic_cache:%t} thresholds={max_retry_count:%d retry_backoff_ms:%d retrieve_timeout_ms:%d user_qps_limit:%d} semantic_cache={threshold:%.3f ttl_seconds:%d max_candidates:%d max_entries_per_scope:%d scope:%s bypass:%s payload:%s topk:%s} phase2={hybrid_dense_weight:%.3f hybrid_sparse_weight:%.3f candidate_topk:%d min_topk:%d max_topk:%d token_budget:%d min_answer_chunks:%d rewrite_timeout_ms:%d rewrite_max_expansions:%d rerank_timeout_ms:%d rerank_model:%s} phase3={parent_child_fill_strategy:%s parent_child_window_size:%d parent_child_max_tokens:%d strategic_topk_min_k:%d strategic_topk_max_k:%d strategic_topk_budget_ratio:%.3f evidence_min_rerank_score:%.3f evidence_min_density:%.3f evidence_min_citation_coverage:%.3f citation_check_threshold:%.3f citation_check_version:%s domain_term_timeout_ms:%d model_rewrite_timeout_ms:%d model_rewrite_shadow_ratio:%.3f} release={enabled:%t stage:%s internal_roles:%s canary_percent:%d batch_percent:%d allowlist_count:%d} phase4_metrics=%s milvus={address:%s database:%s collection:%s}",
		c.ConfigVersion,
		c.buildRAGStrategyDigest(),
		c.RAG.Environment,
		c.RAG.Enabled,
		c.RAG.FeatureFlags.EnableProdGuard,
		c.RAG.FeatureFlags.EnableIngestRetry,
		c.RAG.FeatureFlags.EnableRetrieveAudit,
		c.RAG.FeatureFlags.EnableHybridRetrieval,
		c.RAG.FeatureFlags.EnableQueryRewrite,
		c.RAG.FeatureFlags.EnableDynamicTopK,
		c.RAG.FeatureFlags.EnableAdvancedRerank,
		c.RAG.FeatureFlags.EnableParentChildRetrieval,
		c.RAG.FeatureFlags.EnableStrategicTopK,
		c.RAG.FeatureFlags.EnableEvidenceRefusal,
		c.RAG.FeatureFlags.EnableCitationConsistency,
		c.RAG.FeatureFlags.EnableDomainTerms,
		c.RAG.FeatureFlags.EnableRouteSpecificRewrite,
		c.RAG.FeatureFlags.EnableModelAssistedRewrite,
		c.RAG.FeatureFlags.EnableExperimentPlatform,
		c.RAG.FeatureFlags.EnableIndexLifecycle,
		c.RAG.FeatureFlags.EnableCostDashboard,
		c.RAG.FeatureFlags.EnableComplianceAudit,
		c.RAG.FeatureFlags.EnableWeeklyReport,
		c.RAG.FeatureFlags.EnableMilvusOpsTooling,
		c.RAG.FeatureFlags.EnableCollectionSwitchGuard,
		c.RAG.FeatureFlags.EnableCostGovernance,
		c.RAG.FeatureFlags.EnableAuditCenter,
		c.RAG.FeatureFlags.EnableVectorOps,
		c.RAG.FeatureFlags.EnableGovernanceAlerts,
		c.RAG.FeatureFlags.EnableSemanticCache,
		c.RAG.Thresholds.MaxRetryCount,
		c.RAG.Thresholds.RetryBackoffMS,
		c.RAG.Thresholds.RetrieveTimeoutMS,
		c.RAG.Thresholds.UserQPSLimit,
		c.RAG.SemanticCache.SimilarityThreshold,
		c.RAG.SemanticCache.TTLSeconds,
		c.RAG.SemanticCache.MaxCandidates,
		c.RAG.SemanticCache.MaxEntriesPerScope,
		strings.Join(semanticCacheContract.ScopeDimensions, ","),
		strings.Join(semanticCacheContract.BypassReasons, ","),
		semanticCacheContract.ResultPayload,
		semanticCacheContract.TopKPolicy,
		c.RAG.Phase2.HybridDenseWeight,
		c.RAG.Phase2.HybridSparseWeight,
		c.RAG.Phase2.CandidateTopK,
		c.RAG.Phase2.MinTopK,
		c.RAG.Phase2.MaxTopK,
		c.RAG.Phase2.TokenBudget,
		c.RAG.Phase2.MinAnswerChunks,
		c.RAG.Phase2.RewriteTimeoutMS,
		c.RAG.Phase2.RewriteMaxExpansions,
		c.RAG.Phase2.RerankTimeoutMS,
		c.RAG.Phase2.RerankModel,
		c.RAG.Phase3.ParentChildFillStrategy,
		c.RAG.Phase3.ParentChildWindowSize,
		c.RAG.Phase3.ParentChildMaxTokens,
		c.RAG.Phase3.StrategicTopKMinK,
		c.RAG.Phase3.StrategicTopKMaxK,
		c.RAG.Phase3.StrategicTopKBudgetRatio,
		c.RAG.Phase3.EvidenceMinRerankScore,
		c.RAG.Phase3.EvidenceMinDensity,
		c.RAG.Phase3.EvidenceMinCitationCoverage,
		c.RAG.Phase3.CitationCheckThreshold,
		c.RAG.Phase3.CitationCheckVersion,
		c.RAG.Phase3.DomainTermTimeoutMS,
		c.RAG.Phase3.ModelRewriteTimeoutMS,
		c.RAG.Phase3.ModelRewriteShadowRatio,
		c.RAG.Release.Enabled,
		normalizeRAGReleaseStage(c.RAG.Release.Stage),
		strings.Join(c.RAG.Release.InternalRoles, ","),
		c.RAG.Release.CanaryPercent,
		c.RAG.Release.BatchPercent,
		len(c.RAG.Release.UserAllowlist),
		strings.Join(governance.MetricKeys(), ","),
		maskAddress(c.Milvus.Address),
		c.Milvus.DatabaseName,
		c.Milvus.CollectionName,
	)
}

func buildConfigOverlayPaths(baseConfigPath, env string) []string {
	if env == "" {
		return nil
	}
	baseDir := filepath.Dir(baseConfigPath)
	return []string{
		filepath.Join(baseDir, fmt.Sprintf("config.%s.yaml", env)),
		filepath.Join(baseDir, fmt.Sprintf("config.%s.local.yaml", env)),
	}
}

func readEnvBool(key string) (bool, bool, error) {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return false, false, nil
	}
	value, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false, true, fmt.Errorf("invalid bool env %s=%q: %w", key, raw, err)
	}
	return value, true, nil
}

func readEnvBoolAny(keys ...string) (bool, bool, error) {
	for _, key := range keys {
		if value, ok, err := readEnvBool(key); err != nil {
			return false, false, err
		} else if ok {
			return value, true, nil
		}
	}
	return false, false, nil
}

func readEnvInt(key string) (int, bool, error) {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return 0, false, nil
	}
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, true, fmt.Errorf("invalid int env %s=%q: %w", key, raw, err)
	}
	return value, true, nil
}

func readEnvFloat64(key string) (float64, bool, error) {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return 0, false, nil
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, true, fmt.Errorf("invalid float env %s=%q: %w", key, raw, err)
	}
	return value, true, nil
}

func readEnvUintSlice(key string) ([]uint, bool, error) {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return nil, false, nil
	}
	values := readEnvCSVStrings(raw)
	if len(values) == 0 {
		return []uint{}, true, nil
	}
	result := make([]uint, 0, len(values))
	for _, value := range values {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return nil, true, fmt.Errorf("invalid uint list env %s=%q: %w", key, raw, err)
		}
		result = append(result, uint(parsed))
	}
	return result, true, nil
}

func readEnvCSVStrings(raw string) []string {
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func (c *Config) buildRAGStrategyDigest() string {
	if c == nil {
		return "unknown"
	}
	payload := map[string]interface{}{
		"enabled":        c.RAG.Enabled,
		"env":            c.RAG.Environment,
		"flags":          c.RAG.FeatureFlags,
		"thresholds":     c.RAG.Thresholds,
		"semantic_cache": c.RAG.SemanticCache,
		"phase2":         c.RAG.Phase2,
		"phase3":         c.RAG.Phase3,
		"phase4": map[string]interface{}{
			"experiment_platform":     c.RAG.FeatureFlags.EnableExperimentPlatform,
			"index_lifecycle":         c.RAG.FeatureFlags.EnableIndexLifecycle,
			"cost_dashboard":          c.RAG.FeatureFlags.EnableCostDashboard,
			"compliance_audit":        c.RAG.FeatureFlags.EnableComplianceAudit,
			"weekly_report":           c.RAG.FeatureFlags.EnableWeeklyReport,
			"milvus_ops_tooling":      c.RAG.FeatureFlags.EnableMilvusOpsTooling,
			"collection_switch_guard": c.RAG.FeatureFlags.EnableCollectionSwitchGuard,
			"cost_governance":         c.RAG.FeatureFlags.EnableCostGovernance,
			"audit_center":            c.RAG.FeatureFlags.EnableAuditCenter,
			"vector_ops":              c.RAG.FeatureFlags.EnableVectorOps,
			"governance_alerts":       c.RAG.FeatureFlags.EnableGovernanceAlerts,
			"metric_keys":             governance.MetricKeys(),
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "unknown"
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8])
}

func (c *Config) SemanticCacheContract() RAGSemanticCacheContract {
	return RAGSemanticCacheContract{
		ScopeDimensions: []string{"tenant_id", "kb_ids", "strategy_version", "query_type"},
		BypassReasons:   []string{"empty_query", "debug_request", "authorization_abnormal", "high_risk_experiment"},
		ResultPayload:   "retrieve_result_only",
		TopKPolicy:      "exact_topk_only",
	}
}

// writePhase1BaselineSnapshot 固定 Phase 1 基线快照（配置 + 指标 + 评测报告占位）。
// 首次启动时落盘，后续启动不会覆盖，避免基线在开发过程中被误修改。
func (c *Config) writePhase1BaselineSnapshot(configPath string) error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	baseDir := filepath.Dir(configPath)
	snapshotDir := filepath.Join(baseDir, "docs", "baseline", "phase1")
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return fmt.Errorf("failed to create phase1 baseline dir: %w", err)
	}
	snapshotPath := filepath.Join(snapshotDir, "baseline_snapshot.json")
	if _, err := os.Stat(snapshotPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check phase1 baseline snapshot: %w", err)
	}

	payload := map[string]interface{}{
		"snapshot_type":   "phase1_baseline",
		"generated_at":    time.Now().UTC().Format(time.RFC3339),
		"config_version":  c.ConfigVersion,
		"strategy_digest": c.buildRAGStrategyDigest(),
		"rag": map[string]interface{}{
			"enabled":       c.RAG.Enabled,
			"environment":   c.RAG.Environment,
			"feature_flags": c.RAG.FeatureFlags,
			"thresholds":    c.RAG.Thresholds,
			"phase2":        c.RAG.Phase2,
			"release":       c.RAG.Release,
		},
		"metrics_snapshot": map[string]interface{}{
			"recall_at_10":       nil,
			"mrr":                nil,
			"ndcg":               nil,
			"citation_accuracy":  nil,
			"retrieval_p95_ms":   nil,
			"context_avg_tokens": nil,
			"notes":              "请在完成 Phase 1 基线评测后补齐该节",
		},
		"evaluation_report": map[string]interface{}{
			"dataset_version": "",
			"report_path":     "",
			"summary":         "请在完成离线评测后补齐该节",
		},
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal phase1 baseline snapshot: %w", err)
	}
	if err := os.WriteFile(snapshotPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write phase1 baseline snapshot: %w", err)
	}
	log.Printf("[RAG:L0] phase1 baseline snapshot created: %s", snapshotPath)
	return nil
}

func (c *Config) writePhase2BaselineSnapshot(configPath string) error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	baseDir := filepath.Dir(configPath)
	snapshotDir := filepath.Join(baseDir, "docs", "baseline", "phase2")
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return fmt.Errorf("failed to create phase2 baseline dir: %w", err)
	}
	snapshotPath := filepath.Join(snapshotDir, "baseline_snapshot.json")
	if _, err := os.Stat(snapshotPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check phase2 baseline snapshot: %w", err)
	}

	payload := map[string]interface{}{
		"snapshot_type":   "phase2_baseline",
		"generated_at":    time.Now().UTC().Format(time.RFC3339),
		"config_version":  c.ConfigVersion,
		"strategy_digest": c.buildRAGStrategyDigest(),
		"rag": map[string]interface{}{
			"enabled":       c.RAG.Enabled,
			"environment":   c.RAG.Environment,
			"feature_flags": c.RAG.FeatureFlags,
			"thresholds":    c.RAG.Thresholds,
			"phase2":        c.RAG.Phase2,
			"phase3":        c.RAG.Phase3,
			"release":       c.RAG.Release,
		},
		"evaluation_baseline": map[string]interface{}{
			"dataset_path":      "scripts/evaluation/dataset.json",
			"profile_path":      "scripts/evaluation/retrieval_strategy_profiles.example.json",
			"baseline_profile":  "phase2_baseline",
			"candidate_profile": "parent_child+advanced_rewrite",
			"experiment_groups": []string{
				"phase2_baseline",
				"parent_child",
				"parent_child+strategic_topk",
				"parent_child+refusal",
				"parent_child+advanced_rewrite",
			},
		},
		"metrics_snapshot": map[string]interface{}{
			"recall_at_k":        nil,
			"mrr":                nil,
			"ndcg":               nil,
			"citation_precision": nil,
			"retrieval_p95_ms":   nil,
			"context_avg_tokens": nil,
			"notes":              "Fill in after the frozen Phase 2 regression run completes.",
		},
		"rollback_contract": map[string]interface{}{
			"phase2_main_path_unchanged": true,
			"phase3_flags_independent":   true,
			"rollback_target":            "phase2_baseline",
		},
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal phase2 baseline snapshot: %w", err)
	}
	if err := os.WriteFile(snapshotPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write phase2 baseline snapshot: %w", err)
	}
	log.Printf("[RAG:L0] phase2 baseline snapshot created: %s", snapshotPath)
	return nil
}

func (c *Config) writePhase3BaselineSnapshot(configPath string) error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	baseDir := filepath.Dir(configPath)
	snapshotDir := filepath.Join(baseDir, "docs", "baseline", "phase3")
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return fmt.Errorf("failed to create phase3 baseline dir: %w", err)
	}
	snapshotPath := filepath.Join(snapshotDir, "baseline_snapshot.json")
	if _, err := os.Stat(snapshotPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check phase3 baseline snapshot: %w", err)
	}

	payload := map[string]interface{}{
		"snapshot_type":   "phase3_baseline",
		"generated_at":    time.Now().UTC().Format(time.RFC3339),
		"config_version":  c.ConfigVersion,
		"strategy_digest": c.buildRAGStrategyDigest(),
		"rag": map[string]interface{}{
			"enabled":       c.RAG.Enabled,
			"environment":   c.RAG.Environment,
			"feature_flags": c.RAG.FeatureFlags,
			"thresholds":    c.RAG.Thresholds,
			"phase2":        c.RAG.Phase2,
			"phase3":        c.RAG.Phase3,
			"release":       c.RAG.Release,
		},
		"phase4_governance": map[string]interface{}{
			"feature_flags": governance.FeatureFlagKeys(),
			"metric_keys":   governance.MetricKeys(),
			"trace_fields": []string{
				"experiment_id",
				"strategy_version",
				"index_version",
				"collection_version",
				"cost_trace_id",
				"audit_trace_id",
				"release_id",
			},
			"degrade_contract": []string{
				"governance collection failures do not block retrieval",
				"audit persistence failures emit alert and enter compensation queue",
				"cost collection failures affect dashboard only",
				"collection switch guard failures must block switching",
			},
		},
		"metrics_snapshot": map[string]interface{}{
			"recall_at_k":              nil,
			"mrr":                      nil,
			"ndcg":                     nil,
			"citation_precision":       nil,
			"citation_support_score":   nil,
			"retrieval_p95_ms":         nil,
			"avg_context_tokens":       nil,
			"quality_score":            nil,
			"cost_per_1k_queries":      nil,
			"strategy_regression_rate": nil,
			"rollback_success_rate":    nil,
			"audit_coverage_rate":      nil,
			"collection_health_score":  nil,
			"notes":                    "Fill in after the frozen Phase 3 regression run completes.",
		},
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal phase3 baseline snapshot: %w", err)
	}
	if err := os.WriteFile(snapshotPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write phase3 baseline snapshot: %w", err)
	}
	log.Printf("[RAG:L0] phase3 baseline snapshot created: %s", snapshotPath)
	return nil
}

func isValidParentChildFillStrategy(strategy string) bool {
	switch strings.TrimSpace(strategy) {
	case "parent_only", "sibling_window", "section_window", "child_first_with_parent_summary":
		return true
	default:
		return false
	}
}

func isNormalizedRatio(value float64) bool {
	return value >= 0 && value <= 1
}

func normalizeRAGEnv(env string) string {
	normalized := strings.ToLower(strings.TrimSpace(env))
	switch normalized {
	case "prod", "production":
		return "prod"
	case "staging", "stage":
		return "staging"
	case "dev", "development", "local":
		return "dev"
	case "":
		return ""
	default:
		return normalized
	}
}

func isValidRAGEnv(env string) bool {
	switch normalizeRAGEnv(env) {
	case "dev", "staging", "prod":
		return true
	default:
		return false
	}
}

func normalizeRAGReleaseStage(stage string) string {
	switch strings.ToLower(strings.TrimSpace(stage)) {
	case "", "full", "all":
		return "full"
	case "phase1", "rollback", "off":
		return "phase1"
	case "internal":
		return "internal"
	case "small_flow", "small-flow", "canary":
		return "small_flow"
	case "batch", "batch_flow", "batch-flow":
		return "batch"
	default:
		return strings.ToLower(strings.TrimSpace(stage))
	}
}

func isValidRAGReleaseStage(stage string) bool {
	switch normalizeRAGReleaseStage(stage) {
	case "phase1", "internal", "small_flow", "batch", "full":
		return true
	default:
		return false
	}
}

func maskAddress(addr string) string {
	trimmed := strings.TrimSpace(addr)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 6 {
		return "***"
	}
	return trimmed[:3] + "***" + trimmed[len(trimmed)-3:]
}

// expandEnvInBytes 瀵瑰瓧鑺傚垏鐗囦腑鐨?${VAR_NAME} / $VAR_NAME 鍋氱幆澧冨彉閲忔浛鎹?
func expandEnvInBytes(data []byte) []byte {
	re := regexp.MustCompile(`\$\{([^}]+)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)
	result := re.ReplaceAllFunc(data, func(match []byte) []byte {
		varName := ""
		if bytes.HasPrefix(match, []byte("${")) {
			varName = string(match[2 : len(match)-1])
		} else {
			varName = string(match[1:])
		}
		if val, ok := os.LookupEnv(varName); ok {
			return []byte(val)
		}
		return match
	})
	return result
}

// EmbeddingConfig Embedding鏈嶅姟閰嶇疆
type EmbeddingConfig struct {
	// 认证配置（二选一）
	APIKey    string `yaml:"APIKey"`    // 使用 API Key 认证
	AccessKey string `yaml:"AccessKey"` // 使用 AK 认证 (ark 专用)
	SecretKey string `yaml:"SecretKey"` // 使用 SK 认证 (ark 专用)

	// 服务配置
	Provider string `yaml:"Provider"` // 向量模型提供商: ark, openai, ollama (默认 ark)
	Model    string `yaml:"Model"`    // 模型 ID
	BaseURL  string `yaml:"BaseURL"`  // API 基础 URL
	Region   string `yaml:"Region"`   // 服务区域 (ark 专用)

	// 高级配置
	Timeout    time.Duration `yaml:"Timeout"`    // 请求超时时间
	RetryTimes int           `yaml:"RetryTimes"` // 重试次数
	Dimensions int           `yaml:"Dimensions"` // 输出向量维度
	BatchSize  int           `yaml:"BatchSize"`  // 单次 embedding 请求最大文本数，0=使用默认策略
	User       string        `yaml:"User"`       // 用户标识
}

// MilvusConfig Milvus鍚戦噺鏁版嵁搴撻厤缃?
type MilvusConfig struct {
	// 杩炴帴閰嶇疆
	Address        string `yaml:"Address"`        // Milvus 鏈嶅姟鍦板潃
	Username       string `yaml:"Username"`       // 鐢ㄦ埛鍚嶏紙鍙€夛級
	Password       string `yaml:"Password"`       // 瀵嗙爜锛堝彲閫夛級
	DatabaseName   string `yaml:"DatabaseName"`   // 鏁版嵁搴撳悕绉?
	CollectionName string `yaml:"CollectionName"` // 榛樿闆嗗悎鍚嶇О

	// 澶氶泦鍚堥厤缃?
	Collections map[string]string `yaml:"Collections"` // 澶氫釜闆嗗悎鐨勫懡鍚嶆槧灏?

	// 妫€绱㈤厤缃?
	TopK       int    `yaml:"TopK"`       // 杩斿洖鐨勬渶鐩镐技鏂囨。鏁伴噺
	MetricType string `yaml:"MetricType"` // 璺濈搴﹂噺绫诲瀷: L2, IP, COSINE

	// 瓒呮椂閰嶇疆
	ConnectTimeout time.Duration `yaml:"ConnectTimeout"` // 杩炴帴瓒呮椂
	SearchTimeout  time.Duration `yaml:"SearchTimeout"`  // 鎼滅储瓒呮椂
}

// GetCollection 鑾峰彇鎸囧畾鍚嶇О鐨勯泦鍚堬紝濡傛灉涓嶅瓨鍦ㄥ垯杩斿洖榛樿闆嗗悎
func (c *MilvusConfig) GetCollection(name string) string {
	if c.Collections != nil {
		if col, ok := c.Collections[name]; ok {
			return col
		}
	}
	return c.CollectionName
}

// SplitterConfig 鏂囨。鍒嗗壊鍣ㄩ厤缃?
type SplitterConfig struct {
	ChunkSize   int      `yaml:"ChunkSize"`   // 鐩爣鐗囨澶у皬锛堝瓧绗︽暟锛?
	OverlapSize int      `yaml:"OverlapSize"` // 鐗囨閲嶅彔澶у皬锛堝瓧绗︽暟锛?
	Separators  []string `yaml:"Separators"`  // 鍒嗛殧绗﹀垪琛?
	KeepType    int      `yaml:"KeepType"`    // 鍒嗛殧绗︿繚鐣欑瓥鐣ワ細0=涓嶄繚鐣? 1=淇濈暀鍦ㄥ紑澶? 2=淇濈暀鍦ㄧ粨灏?
}

// ExpandEnv 灞曞紑閰嶇疆涓殑鐜鍙橀噺寮曠敤
// 鏀寔 ${VAR_NAME} 鍜?$VAR_NAME 涓ょ璇硶
func (c *Config) ExpandEnv() {
	// 灞曞紑 Embedding 閰嶇疆
	c.Embedding.APIKey = expandEnvVar(c.Embedding.APIKey)
	c.Embedding.AccessKey = expandEnvVar(c.Embedding.AccessKey)
	c.Embedding.SecretKey = expandEnvVar(c.Embedding.SecretKey)
	c.Embedding.Provider = expandEnvVar(c.Embedding.Provider)
	c.Embedding.Model = expandEnvVar(c.Embedding.Model)
	c.Embedding.BaseURL = expandEnvVar(c.Embedding.BaseURL)
	c.Embedding.Region = expandEnvVar(c.Embedding.Region)
	c.Embedding.User = expandEnvVar(c.Embedding.User)

	// 灞曞紑 Database 閰嶇疆
	c.Database.DSN = expandEnvVar(c.Database.DSN)
	c.Database.Driver = expandEnvVar(c.Database.Driver)

	// 灞曞紑 Redis 閰嶇疆
	c.Redis.Addr = expandEnvVar(c.Redis.Addr)
	c.Redis.Password = expandEnvVar(c.Redis.Password)

	// 灞曞紑 Security 閰嶇疆
	c.Security.JWTSecret = expandEnvVar(c.Security.JWTSecret)

	// 灞曞紑 OpenAI 閰嶇疆
	c.OpenAI.APIKey = expandEnvVar(c.OpenAI.APIKey)
	c.OpenAI.BaseURL = expandEnvVar(c.OpenAI.BaseURL)
	c.OpenAI.ModelName = expandEnvVar(c.OpenAI.ModelName)

	// 灞曞紑 LLM 閰嶇疆
	c.LLM.APIKey = expandEnvVar(c.LLM.APIKey)
	c.LLM.BaseURL = expandEnvVar(c.LLM.BaseURL)
	c.LLM.ModelName = expandEnvVar(c.LLM.ModelName)
	c.LLM.ProviderName = expandEnvVar(c.LLM.ProviderName)

	// 灞曞紑 Google 閰嶇疆
	c.GoogleSearch.APIKey = expandEnvVar(c.GoogleSearch.APIKey)
	c.GoogleSearch.SearchEngineID = expandEnvVar(c.GoogleSearch.SearchEngineID)

	// 灞曞紑 Milvus 閰嶇疆
	c.Milvus.Address = expandEnvVar(c.Milvus.Address)
	c.Milvus.Username = expandEnvVar(c.Milvus.Username)
	c.Milvus.Password = expandEnvVar(c.Milvus.Password)
	c.Milvus.DatabaseName = expandEnvVar(c.Milvus.DatabaseName)
	c.Milvus.CollectionName = expandEnvVar(c.Milvus.CollectionName)
	c.Milvus.MetricType = expandEnvVar(c.Milvus.MetricType)

	// 灞曞紑 Feishu 閰嶇疆
	c.Feishu.WebhookURL = expandEnvVar(c.Feishu.WebhookURL)

	// 灞曞紑 GitHub 閰嶇疆
	c.GitHub.ClientID = expandEnvVar(c.GitHub.ClientID)
	c.GitHub.ClientSecret = expandEnvVar(c.GitHub.ClientSecret)
	c.GitHub.RedirectURL = expandEnvVar(c.GitHub.RedirectURL)

	// 灞曞紑 Payment 閰嶇疆
	c.Payment.Stripe.SecretKey = expandEnvVar(c.Payment.Stripe.SecretKey)
	c.Payment.Stripe.WebhookSecret = expandEnvVar(c.Payment.Stripe.WebhookSecret)
	c.Payment.Stripe.PublishableKey = expandEnvVar(c.Payment.Stripe.PublishableKey)
	c.Payment.PayPal.ClientID = expandEnvVar(c.Payment.PayPal.ClientID)
	c.Payment.PayPal.ClientSecret = expandEnvVar(c.Payment.PayPal.ClientSecret)
	c.Payment.PayPal.WebhookID = expandEnvVar(c.Payment.PayPal.WebhookID)
	// 灞曞紑 Google OAuth 閰嶇疆
	c.GoogleOAuth.ClientID = expandEnvVar(c.GoogleOAuth.ClientID)
	c.GoogleOAuth.ClientSecret = expandEnvVar(c.GoogleOAuth.ClientSecret)
	c.GoogleOAuth.RedirectURL = expandEnvVar(c.GoogleOAuth.RedirectURL)
}

// expandEnvVar 灞曞紑瀛楃涓蹭腑鐨勭幆澧冨彉閲忓紩鐢?
// 鏀寔 ${VAR_NAME} 鍜?$VAR_NAME 涓ょ璇硶
func expandEnvVar(s string) string {
	if s == "" {
		return s
	}

	// 鍖归厤 ${VAR_NAME} 鎴?$VAR_NAME
	re := regexp.MustCompile(`\$\{([^}]+)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

	result := re.ReplaceAllStringFunc(s, func(match string) string {
		// 鎻愬彇鍙橀噺鍚?
		varName := ""
		if strings.HasPrefix(match, "${") {
			// ${VAR_NAME} 鏍煎紡
			varName = match[2 : len(match)-1]
		} else {
			// $VAR_NAME 鏍煎紡
			varName = match[1:]
		}

		// 鑾峰彇鐜鍙橀噺鍊?
		value := os.Getenv(varName)
		if value != "" {
			return value
		}

		// 濡傛灉鐜鍙橀噺涓嶅瓨鍦紝淇濇寔鍘熸牱
		return match
	})

	return result
}
