# KB L6 高级查询改写（领域词表、Route-Specific、模型辅助灰度）实现教程

## 背景

这次 L6 的核心结论可以先说在前面：它不是“把 Phase 2 的规则改写表再补几个词”这么简单，而是把查询改写从单一路径升级成了一套可分层增强、可灰度、可回退、可观测的检索能力。

如果没有这一步，我们会一直遇到三类问题：

1. 规则版 rewrite 能处理常见缩写和别名，但对更强的领域术语场景不够敏感，比如 `juc`、`gmp`、`dlq` 这种词，在不同知识库和语言上下文里的含义并不一样。
2. dense 检索和 sparse 检索共用同一条改写 query，往往会互相牵制。dense route 更适合保守改写，sparse route 更适合积极扩展，混在一起会同时损失两边效果。
3. 长尾 query 靠纯规则很难覆盖，但如果直接让模型“重写整句”，风险又太大，容易引入语义漂移，甚至把危险输入带进召回链路。

所以这次实现做了三件彼此配合的事：

1. 增加领域词表能力，让 rewrite 可以按 `kb_id / kb_scope / language / category / global` 分层命中术语。
2. 增加 route-specific rewrite，让 dense 和 sparse 各自拿到不同强度的最终 query。
3. 增加模型辅助 rewrite 的 shadow 灰度机制，但模型只能补充结构化术语，不能替换原始 query。

这三个点放在一起，才是这个版本真正建立起来的“高级查询改写”。

## 这篇教程会做什么

看完这篇教程以后，你应该能从头复现这样一条链路：

1. 在配置层打开 `enable_domain_terms`、`enable_route_specific_rewrite`、`enable_model_assisted_rewrite`。
2. 在 Milvus 初始化时，把这些 Phase 3 参数注入 `ControlledQueryRewriter`。
3. 在 rewrite 层先做规则改写，再按领域词表补术语，最后在满足条件时做模型辅助 shadow 扩展。
4. 让 dense route 和 sparse route 使用不同的最终 query，并把每路策略写进文档元数据。
5. 用日志、元数据和评测 profile 把这套能力变成可验证、可灰度、可回滚的系统能力。

这篇教程主要覆盖这些文件：

1. `backend/internal/config/config.go`
2. `backend/config.example.yaml`
3. `backend/config.yaml`
4. `backend/internal/milvus/init.go`
5. `backend/internal/milvus/retrieval/rewrite.go`
6. `backend/internal/milvus/retrieval/rewrite_sources.go`
7. `backend/internal/milvus/retrieval/hybrid_search.go`
8. `backend/internal/milvus/retrieval/rewrite_test.go`
9. `backend/internal/milvus/evaluation/profiles.go`
10. `backend/internal/milvus/evaluation/types.go`

如果先用一句人话概括最终控制流，可以这样理解：

1. 请求先进入 `HybridSearchRequest`。
2. `applyControlledRewrite` 调用高级改写器，产出原始 query、dense query、sparse query、策略信息和灰度信息。
3. dense route 拿更保守的 query，sparse route 拿更积极的 query。
4. 每个召回文档都挂上 rewrite 元数据，后面的 handler、日志、评测就能看到“到底改了什么”。

## 需要先理解的术语

### 什么是领域词表

领域词表可以先理解成“按知识域组织的一组专业词扩展规则”。

比如 `juc` 在 Java 面试知识库里，经常对应 `java.util.concurrent`、`abstract queued synchronizer`。这类词不是通用缩写表能稳定覆盖的，所以要额外按领域管理。

在这次实现里，领域词表由 `DomainTermProvider` 提供，默认实现是 `StaticDomainTermProvider`。

### 什么是 Route-Specific Rewrite

`route-specific rewrite` 的意思是：不同召回路线，不一定使用同一条改写后的 query。

这里的 `route` 指检索路线：

1. `dense`：向量检索，更看重整体语义。
2. `sparse`：BM25 一类稀疏检索，更吃关键词覆盖。

所以这次实现里：

1. dense route 走保守策略，只拿少量高置信扩展。
2. sparse route 走积极策略，可以带更多别名、缩写展开和领域词。

这样做不是为了“设计更复杂”，而是因为两条路线的工作原理本来就不同。

### 什么是模型辅助灰度

模型辅助灰度不是“让模型重写 query 再直接上线”，而是：

1. 只在规则和领域词表覆盖不足时触发。
2. 只让模型输出结构化术语建议，比如 `normalized_terms`、`aliases`、`must_keep_terms`。
3. 只以 shadow 方式参与扩展，不替换原始 query。
4. 通过采样比例控制流量，比如 `0.1` 代表约 10% 请求进入模型辅助实验。

你可以把它理解成“模型只提供候选补充词，最后是否进入召回还要经过规则和风险控制”。

### 什么是 Shadow

`shadow` 可以先理解成“影子实验”。

也就是功能真的执行、真的产出结果、真的写日志，但它不改变系统最核心的安全前提。在这里，这个前提就是：

1. 原 query 必须保留。
2. 模型不能替换整句 query。
3. 高风险建议不能进召回。

### 初学者最容易误解的地方

这里最容易“看起来懂了，但实现时写偏”的地方有四个：

1. 高级 rewrite 不是替代规则 rewrite，而是建立在规则 rewrite 之后。
2. route-specific 不是再造两个 rewriter，而是在同一次 rewrite 里产出两条 route query。
3. 模型辅助不是自由生成文本，而是受结构化输出和风险等级约束。
4. 领域词表不是全局唯一大字典，而是按 scope 逐层命中，最后才兜底到 `global`。

## 整体流程

先看总流程，再进代码会更顺。

1. `config.go` 读取高级 rewrite 的开关、超时和 shadow 比例。
2. `InitMilvusManager` 在创建 `HybridRetriever` 时，把这些参数组装进 `QueryRewriterConfig`。
3. `ControlledQueryRewriter.Rewrite` 先做规则版 rewrite。
4. 如果开启了领域词表，就按请求上下文解析 scope，并补充高价值术语。
5. 如果还满足模型辅助条件，就调用 `ModelRewriteAssistant`，只收结构化建议。
6. rewriter 同时生成 `DenseQuery` 和 `SparseQuery`，并为每条 route 生成独立策略说明。
7. `HybridSearchRequest.applyControlledRewrite` 把结果写回请求对象。
8. `attachRewriteMetadata` 把这些信息挂到每个文档上，供后续日志、API、评测和回放使用。

如果你只记一个顺序，就记这个：

先做安全的规则改写，再做可解释的领域增强，最后才做受控的模型灰度补充。

## 分步实现

## 第 1 步：先把高级 rewrite 做成可配置能力

### 目标

先把“是否启用领域词表、是否按 route 分化、是否做模型辅助灰度”从代码逻辑里抽出来，变成可配置、可校验、可通过环境变量覆盖的能力。

### 文件

1. `backend/internal/config/config.go`
2. `backend/config.example.yaml`
3. `backend/config.yaml`

### 完整代码

文件：`backend/internal/config/config.go`

```go
type RAGFeatureFlags struct {
	EnableProdGuard            bool `yaml:"enable_prod_guard"`
	EnableIngestRetry          bool `yaml:"enable_ingest_retry"`
	EnableRetrieveAudit        bool `yaml:"enable_retrieve_audit"`
	EnableHybridRetrieval      bool `yaml:"enable_hybrid_retrieval"`
	EnableQueryRewrite         bool `yaml:"enable_query_rewrite"`
	EnableDynamicTopK          bool `yaml:"enable_dynamic_topk"`
	EnableAdvancedRerank       bool `yaml:"enable_advanced_rerank"`
	EnableParentChildRetrieval bool `yaml:"enable_parent_child_retrieval"`
	EnableStrategicTopK        bool `yaml:"enable_strategic_topk"`
	EnableEvidenceRefusal      bool `yaml:"enable_evidence_refusal"`
	EnableCitationConsistency  bool `yaml:"enable_citation_consistency"`
	EnableDomainTerms          bool `yaml:"enable_domain_terms"`
	EnableRouteSpecificRewrite bool `yaml:"enable_route_specific_rewrite"`
	EnableModelAssistedRewrite bool `yaml:"enable_model_assisted_rewrite"`
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
```

```go
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
```

```go
if c.RAG.Phase3.DomainTermTimeoutMS <= 0 {
	c.RAG.Phase3.DomainTermTimeoutMS = 80
}
if c.RAG.Phase3.ModelRewriteTimeoutMS <= 0 {
	c.RAG.Phase3.ModelRewriteTimeoutMS = 150
}
if c.RAG.Phase3.ModelRewriteShadowRatio <= 0 {
	c.RAG.Phase3.ModelRewriteShadowRatio = 0.1
}
```

```go
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
```

```go
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
```

```go
log.Printf(
	"[RAG:L0] snapshot version=%s strategy_digest=%s env=%s enabled=%t flags={prod_guard:%t ingest_retry:%t retrieve_audit:%t hybrid:%t rewrite:%t dynamic_topk:%t adv_rerank:%t parent_child:%t strategic_topk:%t evidence_refusal:%t citation_consistency:%t domain_terms:%t route_specific_rewrite:%t model_assisted_rewrite:%t} thresholds={max_retry_count:%d retry_backoff_ms:%d retrieve_timeout_ms:%d user_qps_limit:%d} phase2={hybrid_dense_weight:%.3f hybrid_sparse_weight:%.3f candidate_topk:%d min_topk:%d max_topk:%d token_budget:%d min_answer_chunks:%d rewrite_timeout_ms:%d rewrite_max_expansions:%d rerank_timeout_ms:%d rerank_model:%s} phase3={parent_child_fill_strategy:%s parent_child_window_size:%d parent_child_max_tokens:%d strategic_topk_min_k:%d strategic_topk_max_k:%d strategic_topk_budget_ratio:%.3f evidence_min_rerank_score:%.3f evidence_min_density:%.3f evidence_min_citation_coverage:%.3f citation_check_threshold:%.3f citation_check_version:%s domain_term_timeout_ms:%d model_rewrite_timeout_ms:%d model_rewrite_shadow_ratio:%.3f} release={enabled:%t stage:%s internal_roles:%s canary_percent:%d batch_percent:%d allowlist_count:%d} milvus={address:%s database:%s collection:%s}",
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
	c.RAG.Thresholds.MaxRetryCount,
	c.RAG.Thresholds.RetryBackoffMS,
	c.RAG.Thresholds.RetrieveTimeoutMS,
	c.RAG.Thresholds.UserQPSLimit,
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
)
```

文件：`backend/config.example.yaml`

```yaml
    enable_ingest_retry: false
    enable_retrieve_audit: true
    enable_hybrid_retrieval: false
    enable_query_rewrite: false
    enable_dynamic_topk: false
    enable_advanced_rerank: false
    enable_parent_child_retrieval: false
    enable_strategic_topk: false
    enable_evidence_refusal: false
    enable_citation_consistency: false
    enable_domain_terms: false
    enable_route_specific_rewrite: false
    enable_model_assisted_rewrite: false
  thresholds:
    max_retry_count: 3
    retry_backoff_ms: 500
    retrieve_timeout_ms: 3000
    user_qps_limit: 20
  phase2:
    hybrid_dense_weight: 0.7
    hybrid_sparse_weight: 0.3
    candidate_topk: 10
    min_topk: 3
    max_topk: 8
    token_budget: 0
    min_answer_chunks: 2
    rewrite_timeout_ms: 120
    rewrite_max_expansions: 3
    rerank_timeout_ms: 250
    rerank_model: "jaccard-v1"
  phase3:
    parent_child_fill_strategy: section_window
    parent_child_window_size: 1
    parent_child_max_tokens: 1200
    strategic_topk_min_k: 2
    strategic_topk_max_k: 8
    strategic_topk_budget_ratio: 0.6
    evidence_min_rerank_score: 0.55
    evidence_min_density: 0.2
    evidence_min_citation_coverage: 0.5
    citation_check_threshold: 0.7
    citation_check_version: "phase3-citation-v1"
    domain_term_timeout_ms: 80
    model_rewrite_timeout_ms: 150
    model_rewrite_shadow_ratio: 0.1
```

### 这段代码在做什么

这一步建立了三个非常重要的工程前提：

1. 功能开关独立存在，说明高级 rewrite 可以单独开启和关闭。
2. 超时和比例不是硬编码，说明它可以在线上根据反馈快速调整。
3. 配置快照会打印到日志里，说明一次请求背后的策略状态是可追溯的。

### 为什么要这样做

如果我们把这些值直接写在 `rewrite.go` 里，短期看起来省事，但后面会立刻遇到两个问题：

1. 不同环境无法做灰度，只能一起变。
2. 效果变差时无法快速确认到底是开关、比例还是超时引起的。

高级 rewrite 本质上是一种实验型检索增强功能，所以它从第一天就必须是可配置的，而不是写死的。

### 它如何衔接下一步

现在配置层已经能表达“要不要启用 L6”，下一步就可以在初始化阶段把这些配置注入真正的 `QueryRewriter` 实例。

## 第 2 步：在 Milvus 初始化里装配高级改写器

### 目标

把 Phase 3 的高级 rewrite 参数真正传进检索系统，而不是只停留在配置对象里。

### 文件

`backend/internal/milvus/init.go`

### 完整代码

```go
if cfg.RAG.FeatureFlags.EnableQueryRewrite {
	hybridConfig.QueryRewriter = retrieval.NewControlledQueryRewriter(&retrieval.QueryRewriterConfig{
		MaxExpansions:              cfg.RAG.Phase2.RewriteMaxExpansions,
		EnableDomainTerms:          cfg.RAG.FeatureFlags.EnableDomainTerms,
		EnableRouteSpecificRewrite: cfg.RAG.FeatureFlags.EnableRouteSpecificRewrite,
		EnableModelAssistedRewrite: cfg.RAG.FeatureFlags.EnableModelAssistedRewrite,
		DomainTermTimeout:          time.Duration(cfg.RAG.Phase3.DomainTermTimeoutMS) * time.Millisecond,
		ModelRewriteTimeout:        time.Duration(cfg.RAG.Phase3.ModelRewriteTimeoutMS) * time.Millisecond,
		ModelRewriteShadowRatio:    cfg.RAG.Phase3.ModelRewriteShadowRatio,
	})
}
```

### 这段代码在做什么

这段代码完成了真正的“能力装配”：

1. 只有 `EnableQueryRewrite` 打开时，系统才创建 rewriter。
2. Phase 2 的 `RewriteMaxExpansions` 继续作为扩展词预算。
3. Phase 3 再往上叠加领域词表、route-specific、模型辅助灰度这三项高级能力。

### 为什么要这样做

这里有一个很关键的设计：L6 并没有新造一个完全独立的高级 rewriter，而是继续扩展 `ControlledQueryRewriter`。

这样做的好处是：

1. Phase 2 和 Phase 3 共用同一条 rewrite 主链路。
2. 关闭高级能力后，会自然回退到规则版 rewrite。
3. 后续日志和元数据结构也不用拆成两套。

换句话说，L6 不是推倒重来，而是在原来的受控 rewrite 上加更高层的增强。

### 它如何衔接下一步

现在 rewriter 已经被成功注入 `HybridRetriever`，下一步就要看这个 rewriter 自己到底是怎么工作的。

## 第 3 步：先定义高级 rewrite 的输入、输出和控制面

### 目标

在正式写算法之前，先把 rewrite 的请求形状、返回形状和依赖接口定义清楚。

### 文件

`backend/internal/milvus/retrieval/rewrite.go`

### 完整代码

```go
const (
	RewriteStrategyNone                = "none"
	RewriteStrategyBlacklist           = "blacklist"
	RewriteStrategyRuleBased           = "rule_based"
	RewriteStrategyTimeout             = "timeout_fallback"
	RewriteStrategyDomainTerms         = "domain_terms"
	RewriteStrategyRouteSpecific       = "route_specific"
	RewriteStrategyModelAssistedShadow = "model_assisted_shadow"
	defaultDomainDictionaryVersion     = "term-dict-v1"
	defaultModelRewriteShadowRatio     = 0.1
	defaultDomainTermTimeout           = 120 * time.Millisecond
	defaultModelRewriteTimeout         = 150 * time.Millisecond
	modelRewriteRiskLow                = "low"
	modelRewriteRiskMedium             = "medium"
	modelRewriteRiskHigh               = "high"
)

type QueryRewriteRequest struct {
	Query         string
	KBID          uint64
	KBScope       string
	Language      DocumentLanguage
	Category      DocumentCategory
	Collection    string
	RequestID     string
	CandidateTopK int
}

type QueryRewriteResult struct {
	OriginalQuery         string
	RewriteQuery          string
	FinalQuery            string
	DenseQuery            string
	SparseQuery           string
	Strategy              string
	Applied               bool
	Skipped               bool
	ExpansionTerms        []string
	CorrectedTerms        []string
	BlockedByPolicy       bool
	RouteQueries          map[string]string
	RouteStrategies       map[string]string
	TermDictScope         string
	TermDictVersion       string
	TermHits              []string
	ModelRewriteApplied   bool
	ModelRewriteShadow    bool
	ModelRewriteRiskLevel string
	ModelRewriteTerms     []string
}

type QueryRewriter interface {
	Rewrite(ctx context.Context, request QueryRewriteRequest) QueryRewriteResult
}

type QueryRewriterConfig struct {
	MaxExpansions              int
	EnableDomainTerms          bool
	EnableRouteSpecificRewrite bool
	EnableModelAssistedRewrite bool
	DomainTermTimeout          time.Duration
	ModelRewriteTimeout        time.Duration
	ModelRewriteShadowRatio    float64
	DomainTerms                DomainTermProvider
	ModelAssistant             ModelRewriteAssistant
}

type DomainTermProvider interface {
	Resolve(ctx context.Context, request QueryRewriteRequest) DomainTermResolution
}

type DomainTermResolution struct {
	Scope    string
	Version  string
	Terms    map[string][]string
	HitTerms []string
}

type ModelRewriteAssistant interface {
	Assist(ctx context.Context, request ModelRewriteRequest) (ModelRewriteSuggestion, error)
}

type ModelRewriteRequest struct {
	Query           string
	Context         QueryRewriteRequest
	ExistingTerms   []string
	ExpansionBudget int
}

type ModelRewriteSuggestion struct {
	NormalizedTerms []string
	Aliases         []string
	Abbreviations   []string
	MustKeepTerms   []string
	RiskLevel       string
}
```

### 这段代码在做什么

这里最重要的是 `QueryRewriteResult`。它不只返回一条“改写后的 query”，而是返回一整组 rewrite 产物：

1. 原始 query 是什么。
2. dense route 要用什么。
3. sparse route 要用什么。
4. 这次命中了哪些领域词。
5. 模型辅助有没有触发，风险等级是多少。

### 为什么要这样做

如果这里只返回一个 `string`，那后面很多事情都会做不了：

1. 你无法知道 dense 和 sparse 到底用了什么。
2. 你无法知道是规则命中的，还是词表命中的。
3. 你无法把灰度信息打进日志和元数据。

所以这一步的本质，是把 rewrite 从“黑盒字符串处理”升级成“可解释的结构化阶段”。

### 它如何衔接下一步

输入输出协议定义好以后，下一步就可以实现真正的 `ControlledQueryRewriter` 主算法。

## 第 4 步：实现受控高级改写器主算法

### 目标

把规则改写、领域词表扩展、模型辅助灰度和 route-specific 分化放进同一次 rewrite 执行里。

### 文件

`backend/internal/milvus/retrieval/rewrite.go`

### 完整代码

```go
type ControlledQueryRewriter struct {
	config          QueryRewriterConfig
	abbreviations   map[string][]string
	aliases         map[string][]string
	typoCorrections map[string]string
	blacklist       []string
	domainTerms     DomainTermProvider
	modelAssistant  ModelRewriteAssistant
}

func NewControlledQueryRewriter(cfg *QueryRewriterConfig) *ControlledQueryRewriter {
	config := QueryRewriterConfig{
		MaxExpansions:           3,
		DomainTermTimeout:       defaultDomainTermTimeout,
		ModelRewriteTimeout:     defaultModelRewriteTimeout,
		ModelRewriteShadowRatio: defaultModelRewriteShadowRatio,
	}
	if cfg != nil {
		if cfg.MaxExpansions > 0 {
			config.MaxExpansions = cfg.MaxExpansions
		}
		config.EnableDomainTerms = cfg.EnableDomainTerms
		config.EnableRouteSpecificRewrite = cfg.EnableRouteSpecificRewrite
		config.EnableModelAssistedRewrite = cfg.EnableModelAssistedRewrite
		if cfg.DomainTermTimeout > 0 {
			config.DomainTermTimeout = cfg.DomainTermTimeout
		}
		if cfg.ModelRewriteTimeout > 0 {
			config.ModelRewriteTimeout = cfg.ModelRewriteTimeout
		}
		if cfg.ModelRewriteShadowRatio >= 0 && cfg.ModelRewriteShadowRatio <= 1 {
			config.ModelRewriteShadowRatio = cfg.ModelRewriteShadowRatio
		}
		config.DomainTerms = cfg.DomainTerms
		config.ModelAssistant = cfg.ModelAssistant
	}

	domainTerms := config.DomainTerms
	if config.EnableDomainTerms && domainTerms == nil {
		domainTerms = NewStaticDomainTermProvider(defaultDomainDictionaryVersion)
	}
	modelAssistant := config.ModelAssistant
	if config.EnableModelAssistedRewrite && modelAssistant == nil {
		modelAssistant = NewHeuristicModelRewriteAssistant()
	}

	return &ControlledQueryRewriter{
		config: config,
		abbreviations: map[string][]string{
			"jvm":   {"java virtual machine"},
			"gc":    {"garbage collection"},
			"rpc":   {"remote procedure call"},
			"mq":    {"message queue", "message broker"},
			"orm":   {"object relational mapping"},
			"ioc":   {"inversion of control"},
			"aop":   {"aspect oriented programming"},
			"ddl":   {"data definition language"},
			"dml":   {"data manipulation language"},
			"mvcc":  {"multi version concurrency control"},
			"mysql": {"my sql"},
			"k8s":   {"kubernetes"},
			"cas":   {"compare and swap"},
			"stw":   {"stop the world"},
		},
		aliases: map[string][]string{
			"golang":       {"go"},
			"go":           {"golang"},
			"redis":        {"redis cache"},
			"es":           {"elasticsearch"},
			"spring":       {"spring framework"},
			"springboot":   {"spring boot"},
			"microservice": {"microservices", "distributed service"},
			"middleware":   {"middle ware"},
			"rabbitmq":     {"rabbit mq"},
			"rocketmq":     {"rocket mq"},
			"kubernetes":   {"k8s"},
		},
		typoCorrections: map[string]string{
			"sprinboot":    "springboot",
			"spingboot":    "springboot",
			"javva":        "java",
			"golnag":       "golang",
			"redsi":        "redis",
			"kafak":        "kafka",
			"elaticsearch": "elasticsearch",
			"kubenetes":    "kubernetes",
			"gpm":          "gmp",
		},
		blacklist: []string{
			"\"",
			"'",
			"`",
			"site:",
			"http://",
			"https://",
			"select ",
			"update ",
			"delete ",
			"insert ",
			"drop ",
			"truncate ",
			"ignore previous",
			"system prompt",
		},
		domainTerms:    domainTerms,
		modelAssistant: modelAssistant,
	}
}
```

```go
func (r *ControlledQueryRewriter) Rewrite(ctx context.Context, request QueryRewriteRequest) QueryRewriteResult {
	trimmed := strings.TrimSpace(request.Query)
	result := QueryRewriteResult{
		OriginalQuery:   trimmed,
		FinalQuery:      trimmed,
		DenseQuery:      trimmed,
		SparseQuery:     trimmed,
		Strategy:        RewriteStrategyNone,
		RouteQueries:    map[string]string{routeDense: trimmed, routeSparse: trimmed},
		RouteStrategies: map[string]string{routeDense: RewriteStrategyNone, routeSparse: RewriteStrategyNone},
	}
	if trimmed == "" {
		result.Skipped = true
		return result
	}

	select {
	case <-ctx.Done():
		result.Strategy = RewriteStrategyTimeout
		result.Skipped = true
		return result
	default:
	}

	lowerQuery := strings.ToLower(trimmed)
	for _, token := range r.blacklist {
		if strings.Contains(lowerQuery, token) {
			result.Strategy = RewriteStrategyBlacklist
			result.Skipped = true
			result.BlockedByPolicy = true
			return result
		}
	}

	tokens := tokenizeRewriteTerms(trimmed)
	if len(tokens) == 0 {
		result.Skipped = true
		return result
	}

	strategyParts := make([]string, 0, 4)
	if len(tokens) > 0 {
		strategyParts = append(strategyParts, RewriteStrategyRuleBased)
	}

	denseTerms := make([]string, 0, len(tokens)+2)
	sparseTerms := make([]string, 0, len(tokens)+r.config.MaxExpansions)
	denseSeen := make(map[string]struct{}, len(tokens)+r.config.MaxExpansions)
	sparseSeen := make(map[string]struct{}, len(tokens)+r.config.MaxExpansions)

	addTerm := func(target *[]string, seen map[string]struct{}, term string) bool {
		normalized := normalizeRewriteTerm(term)
		if normalized == "" {
			return false
		}
		if _, exists := seen[normalized]; exists {
			return false
		}
		seen[normalized] = struct{}{}
		*target = append(*target, term)
		return true
	}

	correctedTerms := make([]string, 0, 2)
	expansions := make([]string, 0, r.config.MaxExpansions)
	expansionCount := 0
	addExpansion := func(term string, includeDense bool) bool {
		if expansionCount >= r.config.MaxExpansions {
			return false
		}
		if !addTerm(&sparseTerms, sparseSeen, term) {
			return false
		}
		expansions = append(expansions, term)
		expansionCount++
		if includeDense {
			addTerm(&denseTerms, denseSeen, term)
		}
		return true
	}

	for _, token := range tokens {
		select {
		case <-ctx.Done():
			result.Strategy = RewriteStrategyTimeout
			result.Skipped = true
			result.RewriteQuery = ""
			result.FinalQuery = trimmed
			result.DenseQuery = trimmed
			result.SparseQuery = trimmed
			result.RouteQueries = map[string]string{routeDense: trimmed, routeSparse: trimmed}
			result.RouteStrategies = map[string]string{routeDense: RewriteStrategyTimeout, routeSparse: RewriteStrategyTimeout}
			return result
		default:
		}

		normalized := normalizeRewriteTerm(token)
		if corrected, ok := r.typoCorrections[normalized]; ok {
			if addTerm(&denseTerms, denseSeen, corrected) {
				addTerm(&sparseTerms, sparseSeen, corrected)
				correctedTerms = append(correctedTerms, corrected)
			}
			normalized = normalizeRewriteTerm(corrected)
		}
		addTerm(&denseTerms, denseSeen, token)
		addTerm(&sparseTerms, sparseSeen, token)

		ruleCandidates := make([]string, 0, 4)
		if values, ok := r.abbreviations[normalized]; ok {
			ruleCandidates = append(ruleCandidates, values...)
		}
		if values, ok := r.aliases[normalized]; ok {
			ruleCandidates = append(ruleCandidates, values...)
		}
		for index, value := range ruleCandidates {
			addExpansion(value, r.config.EnableRouteSpecificRewrite && index == 0)
		}
	}

	if len(correctedTerms) == 0 && len(expansions) == 0 {
		strategyParts = strategyParts[:0]
	}

	domainResolution := DomainTermResolution{}
	if r.config.EnableDomainTerms && r.domainTerms != nil {
		domainCtx, cancel := context.WithTimeout(ctx, r.config.DomainTermTimeout)
		domainResolution = r.domainTerms.Resolve(domainCtx, request)
		cancel()
		if len(domainResolution.HitTerms) > 0 {
			strategyParts = appendIfMissing(strategyParts, RewriteStrategyDomainTerms)
			result.TermDictScope = domainResolution.Scope
			result.TermDictVersion = domainResolution.Version
			result.TermHits = append([]string(nil), domainResolution.HitTerms...)
			for _, hit := range domainResolution.HitTerms {
				candidates := domainResolution.Terms[hit]
				for index, candidate := range candidates {
					addExpansion(candidate, r.config.EnableRouteSpecificRewrite && index == 0)
				}
			}
		}
	}

	modelSuggestion := ModelRewriteSuggestion{}
	if r.shouldApplyModelAssist(request, tokens, correctedTerms, expansions, domainResolution) {
		modelCtx, cancel := context.WithTimeout(ctx, r.config.ModelRewriteTimeout)
		suggestion, err := r.modelAssistant.Assist(modelCtx, ModelRewriteRequest{
			Query:           trimmed,
			Context:         request,
			ExistingTerms:   append([]string(nil), sparseTerms...),
			ExpansionBudget: max(0, r.config.MaxExpansions-expansionCount),
		})
		cancel()
		if err == nil {
			modelSuggestion = sanitizeModelSuggestion(suggestion)
			result.ModelRewriteRiskLevel = modelSuggestion.RiskLevel
			if modelSuggestion.RiskLevel != modelRewriteRiskHigh {
				modelTerms := append([]string(nil), modelSuggestion.NormalizedTerms...)
				modelTerms = append(modelTerms, modelSuggestion.MustKeepTerms...)
				modelTerms = append(modelTerms, modelSuggestion.Aliases...)
				modelTerms = append(modelTerms, modelSuggestion.Abbreviations...)
				for index, term := range dedupeStrings(modelTerms) {
					if addExpansion(term, r.config.EnableRouteSpecificRewrite && index == 0) {
						result.ModelRewriteTerms = append(result.ModelRewriteTerms, term)
					}
				}
				if len(result.ModelRewriteTerms) > 0 {
					result.ModelRewriteApplied = true
					result.ModelRewriteShadow = true
					strategyParts = appendIfMissing(strategyParts, RewriteStrategyModelAssistedShadow)
				}
			}
		}
	}

	if !r.config.EnableRouteSpecificRewrite {
		denseTerms = append([]string(nil), sparseTerms...)
		denseSeen = cloneSet(sparseSeen)
	} else if !strings.EqualFold(strings.TrimSpace(strings.Join(denseTerms, " ")), strings.TrimSpace(strings.Join(sparseTerms, " "))) {
		strategyParts = appendIfMissing(strategyParts, RewriteStrategyRouteSpecific)
	}

	denseQuery := strings.TrimSpace(strings.Join(denseTerms, " "))
	sparseQuery := strings.TrimSpace(strings.Join(sparseTerms, " "))
	if denseQuery == "" {
		denseQuery = trimmed
	}
	if sparseQuery == "" {
		sparseQuery = trimmed
	}

	result.CorrectedTerms = correctedTerms
	result.ExpansionTerms = expansions
	result.DenseQuery = denseQuery
	result.SparseQuery = sparseQuery
	result.RouteQueries = map[string]string{
		routeDense:  denseQuery,
		routeSparse: sparseQuery,
	}
	result.RouteStrategies = map[string]string{
		routeDense:  resolveRouteRewriteStrategy(r.config.EnableRouteSpecificRewrite, denseQuery, trimmed, correctedTerms, result.TermHits, result.ModelRewriteApplied, true),
		routeSparse: resolveRouteRewriteStrategy(r.config.EnableRouteSpecificRewrite, sparseQuery, trimmed, correctedTerms, result.TermHits, result.ModelRewriteApplied, false),
	}

	result.Strategy = RewriteStrategyNone
	if len(strategyParts) > 0 {
		result.Strategy = strings.Join(dedupeStrings(strategyParts), "+")
	}

	finalQuery := sparseQuery
	if !r.config.EnableRouteSpecificRewrite {
		finalQuery = denseQuery
	}
	if finalQuery == "" {
		finalQuery = trimmed
	}
	if strings.EqualFold(finalQuery, trimmed) && strings.EqualFold(denseQuery, trimmed) && strings.EqualFold(sparseQuery, trimmed) {
		result.Skipped = true
		result.FinalQuery = trimmed
		result.RewriteQuery = ""
		result.Strategy = firstNonEmpty(result.Strategy, RewriteStrategyNone)
		return result
	}

	result.Applied = true
	result.FinalQuery = finalQuery
	result.RewriteQuery = finalQuery
	if strings.EqualFold(strings.TrimSpace(result.RewriteQuery), trimmed) && !strings.EqualFold(strings.TrimSpace(denseQuery), trimmed) {
		result.RewriteQuery = denseQuery
	}
	return result
}
```

### 这段代码在做什么

这一步是整套能力的核心。它做了五层控制：

1. 空 query、超时、黑名单 query 直接跳过。
2. 先做规则级改写，包括错拼修正、缩写展开、别名扩展。
3. 再按领域词表补专业术语。
4. 只有在收益可能更高、风险可控时，才触发模型辅助建议。
5. 最后根据是否启用 route-specific，决定 dense 和 sparse 是否分化。

### 为什么要这样做

这个顺序不是随便排的。

如果先跑模型，再跑规则，会有两个问题：

1. 很多本来可以靠规则稳定解决的 query，被没必要地送去模型辅助。
2. 模型补出来的词可能会挤占 `MaxExpansions` 预算，反而影响更确定的规则词。

所以这里先规则、再词表、再模型，是一种“从最确定到最不确定”的排序。

### 它如何衔接下一步

主算法已经有了，但它还依赖两个外部能力：领域词表提供者和模型辅助建议者。下一步就看这两个来源怎么实现。

## 第 5 步：实现领域词表提供者和模型辅助来源

### 目标

给主改写器提供两个可插拔的数据来源：

1. 领域词表来源。
2. 模型辅助建议来源。

### 文件

`backend/internal/milvus/retrieval/rewrite_sources.go`

### 完整代码

```go
type StaticDomainTermProvider struct {
	mu      sync.RWMutex
	version string
	scopes  map[string]map[string][]string
}

func NewStaticDomainTermProvider(version string) *StaticDomainTermProvider {
	if strings.TrimSpace(version) == "" {
		version = defaultDomainDictionaryVersion
	}
	provider := &StaticDomainTermProvider{
		version: version,
		scopes:  make(map[string]map[string][]string),
	}
	provider.RegisterScope("global", map[string][]string{
		"iac":   {"infrastructure as code"},
		"sso":   {"single sign on"},
		"ci":    {"continuous integration"},
		"cd":    {"continuous delivery"},
		"oauth": {"open authorization"},
	})
	provider.RegisterScope("language:java", map[string][]string{
		"juc":  {"java.util.concurrent", "concurrent utilities", "aqs"},
		"jmm":  {"java memory model"},
		"aqs":  {"abstract queued synchronizer"},
		"jfr":  {"java flight recorder"},
		"tlab": {"thread local allocation buffer"},
	})
	provider.RegisterScope("language:golang", map[string][]string{
		"gmp":     {"goroutine machine processor scheduler", "go scheduler"},
		"pprof":   {"performance profiling"},
		"gctrace": {"garbage collector trace"},
		"csp":     {"communicating sequential processes"},
	})
	provider.RegisterScope("language:middleware", map[string][]string{
		"dlq": {"dead letter queue"},
		"isr": {"in sync replica"},
		"osr": {"out of sync replica"},
		"eos": {"exactly once semantics"},
	})
	return provider
}

func (p *StaticDomainTermProvider) RegisterScope(scope string, terms map[string][]string) {
	scope = strings.TrimSpace(strings.ToLower(scope))
	if scope == "" || len(terms) == 0 {
		return
	}
	normalized := make(map[string][]string, len(terms))
	for key, values := range terms {
		termKey := normalizeRewriteTerm(key)
		if termKey == "" {
			continue
		}
		expansions := make([]string, 0, len(values))
		for _, value := range values {
			if normalizeRewriteTerm(value) == "" {
				continue
			}
			expansions = append(expansions, strings.TrimSpace(value))
		}
		if len(expansions) == 0 {
			continue
		}
		normalized[termKey] = expansions
	}
	if len(normalized) == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.scopes[scope] = normalized
}

func (p *StaticDomainTermProvider) Resolve(ctx context.Context, request QueryRewriteRequest) DomainTermResolution {
	select {
	case <-ctx.Done():
		return DomainTermResolution{Version: p.version}
	default:
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	scopeKeys := resolveDomainScopeKeys(request)
	lowerQuery := strings.ToLower(strings.TrimSpace(request.Query))
	tokenSet := make(map[string]struct{}, 8)
	for _, token := range tokenizeRewriteTerms(request.Query) {
		tokenSet[token] = struct{}{}
	}

	resolution := DomainTermResolution{
		Version: p.version,
		Terms:   make(map[string][]string),
	}
	appliedScopes := make([]string, 0, len(scopeKeys))
	hitTerms := make([]string, 0, 4)
	for _, scope := range scopeKeys {
		terms, ok := p.scopes[scope]
		if !ok {
			continue
		}
		appliedScopes = append(appliedScopes, scope)
		for key, values := range terms {
			select {
			case <-ctx.Done():
				resolution.Scope = strings.Join(appliedScopes, ">")
				resolution.HitTerms = dedupeStrings(hitTerms)
				return resolution
			default:
			}

			if !matchesRewriteKey(lowerQuery, tokenSet, key) {
				continue
			}
			resolution.Terms[key] = append([]string(nil), values...)
			hitTerms = append(hitTerms, key)
		}
	}
	resolution.Scope = strings.Join(appliedScopes, ">")
	resolution.HitTerms = dedupeStrings(hitTerms)
	return resolution
}
```

```go
type HeuristicModelRewriteAssistant struct {
	knowledge map[string]ModelRewriteSuggestion
}

func NewHeuristicModelRewriteAssistant() *HeuristicModelRewriteAssistant {
	return &HeuristicModelRewriteAssistant{
		knowledge: map[string]ModelRewriteSuggestion{
			"aqs": {
				NormalizedTerms: []string{"abstract queued synchronizer"},
				Aliases:         []string{"queue synchronizer"},
				MustKeepTerms:   []string{"aqs"},
				RiskLevel:       modelRewriteRiskLow,
			},
			"cas": {
				NormalizedTerms: []string{"compare and swap"},
				Aliases:         []string{"atomic compare swap"},
				MustKeepTerms:   []string{"cas"},
				RiskLevel:       modelRewriteRiskLow,
			},
			"gmp": {
				NormalizedTerms: []string{"go scheduler"},
				Aliases:         []string{"goroutine scheduler"},
				MustKeepTerms:   []string{"gmp"},
				RiskLevel:       modelRewriteRiskLow,
			},
			"stw": {
				NormalizedTerms: []string{"stop the world"},
				Aliases:         []string{"gc pause"},
				MustKeepTerms:   []string{"stw"},
				RiskLevel:       modelRewriteRiskLow,
			},
			"tlab": {
				NormalizedTerms: []string{"thread local allocation buffer"},
				MustKeepTerms:   []string{"tlab"},
				RiskLevel:       modelRewriteRiskLow,
			},
		},
	}
}

func (a *HeuristicModelRewriteAssistant) Assist(ctx context.Context, request ModelRewriteRequest) (ModelRewriteSuggestion, error) {
	select {
	case <-ctx.Done():
		return ModelRewriteSuggestion{RiskLevel: modelRewriteRiskMedium}, ctx.Err()
	default:
	}

	tokens := tokenizeRewriteTerms(request.Query)
	if len(tokens) == 0 {
		return ModelRewriteSuggestion{RiskLevel: modelRewriteRiskMedium}, nil
	}

	collected := ModelRewriteSuggestion{RiskLevel: modelRewriteRiskMedium}
	for _, token := range tokens {
		select {
		case <-ctx.Done():
			return ModelRewriteSuggestion{RiskLevel: modelRewriteRiskMedium}, ctx.Err()
		default:
		}

		suggestion, ok := a.knowledge[token]
		if !ok {
			continue
		}
		collected.NormalizedTerms = append(collected.NormalizedTerms, suggestion.NormalizedTerms...)
		collected.Aliases = append(collected.Aliases, suggestion.Aliases...)
		collected.Abbreviations = append(collected.Abbreviations, suggestion.Abbreviations...)
		collected.MustKeepTerms = append(collected.MustKeepTerms, suggestion.MustKeepTerms...)
		collected.RiskLevel = suggestion.RiskLevel
	}
	return sanitizeModelSuggestion(collected), nil
}
```

```go
func resolveDomainScopeKeys(request QueryRewriteRequest) []string {
	keys := make([]string, 0, 5)
	if request.KBID > 0 {
		keys = append(keys, "kb:"+uint64ToString(request.KBID))
	}
	if value := strings.TrimSpace(strings.ToLower(request.KBScope)); value != "" {
		keys = append(keys, "kb_scope:"+value)
	}
	if value := strings.TrimSpace(strings.ToLower(string(request.Language))); value != "" {
		keys = append(keys, "language:"+value)
	}
	if value := strings.TrimSpace(strings.ToLower(string(request.Category))); value != "" {
		keys = append(keys, "category:"+value)
	}
	keys = append(keys, "global")
	return uniqueStringsPreserveOrder(keys)
}

func matchesRewriteKey(lowerQuery string, tokens map[string]struct{}, key string) bool {
	key = normalizeRewriteTerm(key)
	if key == "" {
		return false
	}
	if _, ok := tokens[key]; ok {
		return true
	}
	return strings.Contains(lowerQuery, key)
}
```

### 这段代码在做什么

这里做了两件事：

1. 用 `StaticDomainTermProvider` 解决“领域词从哪里来”的问题。
2. 用 `HeuristicModelRewriteAssistant` 解决“模型辅助建议长什么样”的问题。

尤其是 `resolveDomainScopeKeys` 很关键。它告诉我们一次请求会按什么顺序找词表：

1. 先找 `kb:<id>`。
2. 再找 `kb_scope:<scope>`。
3. 再找 `language:<language>`。
4. 再找 `category:<category>`。
5. 最后兜底到 `global`。

### 为什么要这样做

如果只做一个全局大词表，会有两个明显问题：

1. 同一个缩写在不同知识域里可能含义不同。
2. 词表会越来越大，误命中也会越来越多。

分 scope 的本质，不是为了层次更漂亮，而是为了把“词义上下文”带进 rewrite 阶段。

模型辅助这边也是同理。它没有返回一整句文本，而是返回结构化字段。这样做的好处是：

1. 风险更低。
2. 更容易审计。
3. 更容易和规则、词表合并。

### 它如何衔接下一步

现在 rewriter 的外部依赖已经齐了，下一步要补上剩下几个关键辅助函数，让“何时触发模型辅助、如何采样、如何格式化策略”全部闭环。

## 第 6 步：补齐模型灰度、采样和策略格式化

### 目标

让模型辅助 rewrite 真正成为“受控实验”，而不是只多了一个接口。

### 文件

`backend/internal/milvus/retrieval/rewrite.go`

### 完整代码

```go
func (r *ControlledQueryRewriter) shouldApplyModelAssist(
	request QueryRewriteRequest,
	tokens []string,
	correctedTerms []string,
	expansions []string,
	domainResolution DomainTermResolution,
) bool {
	if r == nil || !r.config.EnableModelAssistedRewrite || r.modelAssistant == nil {
		return false
	}
	query := strings.TrimSpace(request.Query)
	if query == "" || isHighRiskModelRewriteQuery(query) {
		return false
	}
	if !sampleModelRewriteShadow(request, r.config.ModelRewriteShadowRatio) {
		return false
	}
	if len(tokens) >= 10 {
		return false
	}
	return len(correctedTerms) == 0 || (len(expansions) == 0 && len(domainResolution.HitTerms) == 0)
}

func sampleModelRewriteShadow(request QueryRewriteRequest, ratio float64) bool {
	if ratio <= 0 {
		return false
	}
	if ratio >= 1 {
		return true
	}
	key := strings.ToLower(strings.TrimSpace(request.Query)) + "|" + strings.TrimSpace(request.KBScope) + "|" + request.Collection
	if request.KBID > 0 {
		key += "|" + strconv.FormatUint(request.KBID, 10)
	}
	sum := sha1.Sum([]byte(key))
	bucket := binary.BigEndian.Uint32(sum[:4]) % 10000
	return float64(bucket) < ratio*10000
}

func sanitizeModelSuggestion(suggestion ModelRewriteSuggestion) ModelRewriteSuggestion {
	suggestion.NormalizedTerms = dedupeStrings(suggestion.NormalizedTerms)
	suggestion.Aliases = dedupeStrings(suggestion.Aliases)
	suggestion.Abbreviations = dedupeStrings(suggestion.Abbreviations)
	suggestion.MustKeepTerms = dedupeStrings(suggestion.MustKeepTerms)
	switch strings.ToLower(strings.TrimSpace(suggestion.RiskLevel)) {
	case modelRewriteRiskLow:
		suggestion.RiskLevel = modelRewriteRiskLow
	case modelRewriteRiskHigh:
		suggestion.RiskLevel = modelRewriteRiskHigh
	default:
		suggestion.RiskLevel = modelRewriteRiskMedium
	}
	return suggestion
}

func isHighRiskModelRewriteQuery(query string) bool {
	lower := strings.ToLower(strings.TrimSpace(query))
	if lower == "" {
		return true
	}
	riskyMarkers := []string{
		"site:",
		"http://",
		"https://",
		"ignore previous",
		"system prompt",
		"```",
		"select ",
		"delete ",
		"drop ",
	}
	for _, marker := range riskyMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return len(tokenizeRewriteTerms(query)) > 12
}
```

```go
func formatRewriteStrategy(result QueryRewriteResult) string {
	if result.Strategy == "" {
		return RewriteStrategyNone
	}
	parts := []string{result.Strategy}
	if len(result.RouteStrategies) > 0 {
		parts = append(parts, "routes="+formatRouteStrategyPairs(result.RouteStrategies))
	}
	if result.TermDictScope != "" {
		parts = append(parts, "term_scope="+result.TermDictScope)
	}
	if result.TermDictVersion != "" {
		parts = append(parts, "term_version="+result.TermDictVersion)
	}
	if len(result.TermHits) > 0 {
		parts = append(parts, "term_hits="+strings.Join(dedupeStrings(result.TermHits), "|"))
	}
	if result.ModelRewriteApplied {
		parts = append(parts, "model_shadow=true")
	}
	if result.ModelRewriteRiskLevel != "" {
		parts = append(parts, "model_risk="+result.ModelRewriteRiskLevel)
	}
	if len(result.ModelRewriteTerms) > 0 {
		parts = append(parts, "model_terms="+strings.Join(dedupeStrings(result.ModelRewriteTerms), "|"))
	}
	if len(result.CorrectedTerms) > 0 {
		parts = append(parts, "corrected="+strings.Join(dedupeStrings(result.CorrectedTerms), "|"))
	}
	if len(result.ExpansionTerms) > 0 {
		parts = append(parts, "expanded="+strings.Join(dedupeStrings(result.ExpansionTerms), "|"))
	}
	return strings.Join(parts, ";")
}

func resolveRouteRewriteStrategy(enableRouteSpecific bool, routeQuery, originalQuery string, correctedTerms, termHits []string, modelApplied bool, dense bool) string {
	if strings.EqualFold(strings.TrimSpace(routeQuery), strings.TrimSpace(originalQuery)) {
		return RewriteStrategyNone
	}
	parts := make([]string, 0, 5)
	parts = append(parts, RewriteStrategyRuleBased)
	if enableRouteSpecific {
		if dense {
			parts = append(parts, RewriteStrategyRouteSpecific+":conservative")
		} else {
			parts = append(parts, RewriteStrategyRouteSpecific+":aggressive")
		}
	}
	if len(termHits) > 0 {
		parts = append(parts, RewriteStrategyDomainTerms)
	}
	if modelApplied {
		parts = append(parts, RewriteStrategyModelAssistedShadow)
	}
	if len(correctedTerms) > 0 {
		parts = append(parts, "typo_corrected")
	}
	return strings.Join(dedupeStrings(parts), "+")
}
```

### 这段代码在做什么

这里真正把“灰度”落成了代码：

1. `shouldApplyModelAssist` 负责决定要不要试。
2. `sampleModelRewriteShadow` 负责稳定采样。
3. `sanitizeModelSuggestion` 负责把模型输出收口成安全格式。
4. `formatRewriteStrategy` 和 `resolveRouteRewriteStrategy` 负责把本次改写解释清楚。

### 为什么要这样做

最值得注意的是稳定采样。

这里不是简单 `rand.Float64() < ratio`，而是对 `query + kb_scope + collection + kb_id` 做哈希采样。这样同一个请求在一段时间内会稳定地落到同一个 bucket，方便做 A/B 对比和问题复现。

如果直接随机采样，会出现一个很麻烦的问题：同一个 query 今天进实验，明天不进实验，回放和排障都很痛苦。

### 它如何衔接下一步

高级 rewrite 已经能产出结果了，下一步要把这些结果真正送进检索请求和文档元数据里，否则后面的链路根本看不到这些变化。

## 第 7 步：把高级 rewrite 接进检索请求和文档元数据

### 目标

让 `HybridRetriever` 真的消费高级 rewrite 的结果，并把这些结果传给后续链路。

### 文件

`backend/internal/milvus/retrieval/hybrid_search.go`

### 完整代码

```go
type HybridSearchRequest struct {
	Query                 string
	OriginalQuery         string
	RewriteQuery          string
	FinalQuery            string
	DenseQuery            string
	SparseQuery           string
	RewriteStrategy       string
	RewriteApplied        bool
	RouteRewriteStrategy  map[string]string
	Expr                  string
	TopK                  int
	KBScope               string
	KBID                  uint64
	Language              DocumentLanguage
	Category              DocumentCategory
	RequestID             string
	Collection            string
	CandidateTopK         int
	TermDictScope         string
	TermDictVersion       string
	TermHits              []string
	ModelRewriteApplied   bool
	ModelRewriteShadow    bool
	ModelRewriteRiskLevel string
	ModelRewriteTerms     []string
}
```

```go
func (req *HybridSearchRequest) applyControlledRewrite(ctx context.Context, rewriter QueryRewriter) {
	if req == nil {
		return
	}
	if strings.TrimSpace(req.OriginalQuery) == "" {
		req.OriginalQuery = strings.TrimSpace(req.Query)
	}
	if rewriter == nil {
		req.Query = req.OriginalQuery
		req.FinalQuery = req.OriginalQuery
		req.DenseQuery = req.OriginalQuery
		req.SparseQuery = req.OriginalQuery
		req.RewriteQuery = ""
		req.RewriteStrategy = RewriteStrategyNone
		req.RewriteApplied = false
		req.RouteRewriteStrategy = map[string]string{}
		return
	}

	result := rewriter.Rewrite(ctx, QueryRewriteRequest{
		Query:         req.OriginalQuery,
		KBID:          req.KBID,
		KBScope:       req.KBScope,
		Language:      req.Language,
		Category:      req.Category,
		Collection:    req.Collection,
		RequestID:     req.RequestID,
		CandidateTopK: req.CandidateTopK,
	})
	req.Query = req.OriginalQuery
	req.RewriteQuery = strings.TrimSpace(result.RewriteQuery)
	req.FinalQuery = strings.TrimSpace(result.FinalQuery)
	if req.FinalQuery == "" {
		req.FinalQuery = req.OriginalQuery
	}
	req.DenseQuery = firstNonEmptyQuery(result.DenseQuery, req.FinalQuery, req.OriginalQuery)
	req.SparseQuery = firstNonEmptyQuery(result.SparseQuery, req.FinalQuery, req.OriginalQuery)
	req.RewriteStrategy = formatRewriteStrategy(result)
	req.RewriteApplied = result.Applied
	req.RouteRewriteStrategy = cloneStringMap(result.RouteStrategies)
	req.TermDictScope = result.TermDictScope
	req.TermDictVersion = result.TermDictVersion
	req.TermHits = append([]string(nil), result.TermHits...)
	req.ModelRewriteApplied = result.ModelRewriteApplied
	req.ModelRewriteShadow = result.ModelRewriteShadow
	req.ModelRewriteRiskLevel = result.ModelRewriteRiskLevel
	req.ModelRewriteTerms = append([]string(nil), result.ModelRewriteTerms...)
	if !req.RewriteApplied {
		req.RewriteQuery = ""
		req.FinalQuery = req.OriginalQuery
		req.DenseQuery = req.OriginalQuery
		req.SparseQuery = req.OriginalQuery
	}
}
```

```go
func attachRewriteMetadata(doc *schema.Document, req *HybridSearchRequest, route string) {
	if doc == nil || req == nil {
		return
	}
	if doc.MetaData == nil {
		doc.MetaData = make(map[string]interface{})
	}
	doc.MetaData["original_query"] = req.OriginalQuery
	doc.MetaData["rewrite_query"] = req.RewriteQuery
	doc.MetaData["final_query"] = req.FinalQuery
	doc.MetaData["dense_query"] = req.DenseQuery
	doc.MetaData["sparse_query"] = req.SparseQuery
	doc.MetaData["route_final_query"] = resolveRouteQuery(req, route)
	doc.MetaData["route_rewrite_strategy"] = resolveRouteRewriteStrategyFromRequest(req, route)
	doc.MetaData["rewrite_strategy"] = req.RewriteStrategy
	doc.MetaData["rewrite_applied"] = req.RewriteApplied
	if req.TermDictScope != "" {
		doc.MetaData["term_dict_scope"] = req.TermDictScope
	}
	if req.TermDictVersion != "" {
		doc.MetaData["term_dict_version"] = req.TermDictVersion
	}
	if len(req.TermHits) > 0 {
		doc.MetaData["term_hits"] = append([]string(nil), req.TermHits...)
	}
	if req.ModelRewriteApplied {
		doc.MetaData["model_rewrite_applied"] = true
	}
	if req.ModelRewriteShadow {
		doc.MetaData["model_rewrite_shadow"] = true
	}
	if req.ModelRewriteRiskLevel != "" {
		doc.MetaData["model_rewrite_risk_level"] = req.ModelRewriteRiskLevel
	}
	if len(req.ModelRewriteTerms) > 0 {
		doc.MetaData["model_rewrite_terms"] = append([]string(nil), req.ModelRewriteTerms...)
	}
}
```

```go
func resolveRouteQuery(req *HybridSearchRequest, route string) string {
	if req == nil {
		return ""
	}
	switch strings.TrimSpace(strings.ToLower(route)) {
	case routeSparse:
		return firstNonEmptyQuery(req.SparseQuery, req.FinalQuery, req.OriginalQuery)
	default:
		return firstNonEmptyQuery(req.DenseQuery, req.FinalQuery, req.OriginalQuery)
	}
}

func resolveRouteRewriteStrategyFromRequest(req *HybridSearchRequest, route string) string {
	if req == nil || len(req.RouteRewriteStrategy) == 0 {
		return ""
	}
	return strings.TrimSpace(req.RouteRewriteStrategy[strings.TrimSpace(strings.ToLower(route))])
}
```

### 这段代码在做什么

这一层解决的是“改写结果怎么流到后面”。

它把 L6 产出的信息分成了两份：

1. 请求级数据，放在 `HybridSearchRequest` 上。
2. 文档级数据，挂在每个 `schema.Document.MetaData` 上。

### 为什么要这样做

这一步非常重要，因为高级 rewrite 不是只影响“搜什么”，还影响“怎么解释这次为什么搜成这样”。

如果没有这层元数据，后面你看到一条召回结果时，只知道“它命中了”，但你不知道：

1. 它是用原 query 命中的，还是用领域词命中的。
2. 它命中的是 dense route 还是 sparse route。
3. 它有没有受到模型辅助 shadow 的影响。

所以文档元数据不是附属信息，它是整个高级 rewrite 可观测性的基础。

### 它如何衔接下一步

现在检索链路已经真正消费了 L6 的结果，最后一步就是补上测试和评测 profile，确保这套能力能长期稳定演进。

## 第 8 步：用测试和评测 profile 把这套能力固定下来

### 目标

证明这套高级 rewrite 不是“写出来看起来对”，而是能被测试、能被评测、能被灰度比较。

### 文件

1. `backend/internal/milvus/retrieval/rewrite_test.go`
2. `backend/internal/milvus/evaluation/profiles.go`
3. `backend/internal/milvus/evaluation/types.go`

### 完整代码

文件：`backend/internal/milvus/retrieval/rewrite_test.go`

```go
type stubModelRewriteAssistant struct {
	suggestion ModelRewriteSuggestion
	err        error
}

func (s stubModelRewriteAssistant) Assist(ctx context.Context, request ModelRewriteRequest) (ModelRewriteSuggestion, error) {
	return s.suggestion, s.err
}

func TestControlledQueryRewriterExpandsAbbreviationAndAlias(t *testing.T) {
	rewriter := NewControlledQueryRewriter(&QueryRewriterConfig{MaxExpansions: 3})

	result := rewriter.Rewrite(context.Background(), QueryRewriteRequest{Query: "jvm gc"})

	if !result.Applied {
		t.Fatalf("expected rewrite to be applied")
	}
	if result.Strategy != RewriteStrategyRuleBased {
		t.Fatalf("strategy = %q, want %q", result.Strategy, RewriteStrategyRuleBased)
	}
	if !strings.Contains(result.FinalQuery, "java virtual machine") {
		t.Fatalf("expected final query to contain abbreviation expansion, got %q", result.FinalQuery)
	}
	if !strings.Contains(result.FinalQuery, "garbage collection") {
		t.Fatalf("expected final query to contain alias expansion, got %q", result.FinalQuery)
	}
}

func TestControlledQueryRewriterHonorsExpansionLimit(t *testing.T) {
	rewriter := NewControlledQueryRewriter(&QueryRewriterConfig{MaxExpansions: 1})

	result := rewriter.Rewrite(context.Background(), QueryRewriteRequest{Query: "mq rpc"})

	if len(result.ExpansionTerms) != 1 {
		t.Fatalf("expansions = %d, want 1", len(result.ExpansionTerms))
	}
}

func TestControlledQueryRewriterCorrectsTypos(t *testing.T) {
	rewriter := NewControlledQueryRewriter(nil)

	result := rewriter.Rewrite(context.Background(), QueryRewriteRequest{Query: "sprinboot interview"})

	if !result.Applied {
		t.Fatalf("expected typo correction to apply")
	}
	if !strings.Contains(result.FinalQuery, "springboot") {
		t.Fatalf("final query = %q, want springboot correction", result.FinalQuery)
	}
}

func TestControlledQueryRewriterBlacklistSkipsRewrite(t *testing.T) {
	rewriter := NewControlledQueryRewriter(nil)

	result := rewriter.Rewrite(context.Background(), QueryRewriteRequest{Query: `site:example.com "jvm"`})

	if result.Applied {
		t.Fatalf("expected blacklist query to skip rewrite")
	}
	if result.Strategy != RewriteStrategyBlacklist {
		t.Fatalf("strategy = %q, want %q", result.Strategy, RewriteStrategyBlacklist)
	}
	if result.FinalQuery != `site:example.com "jvm"` {
		t.Fatalf("final query = %q, want original query", result.FinalQuery)
	}
}

func TestControlledQueryRewriterTimeoutFallsBack(t *testing.T) {
	rewriter := NewControlledQueryRewriter(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := rewriter.Rewrite(ctx, QueryRewriteRequest{Query: "jvm"})

	if result.Applied {
		t.Fatalf("expected canceled context to skip rewrite")
	}
	if result.Strategy != RewriteStrategyTimeout {
		t.Fatalf("strategy = %q, want %q", result.Strategy, RewriteStrategyTimeout)
	}
}

func TestControlledQueryRewriterAppliesDomainTermsAndRouteSpecificQueries(t *testing.T) {
	provider := NewStaticDomainTermProvider("test-v1")
	provider.RegisterScope("language:java", map[string][]string{
		"juc": {"java.util.concurrent", "abstract queued synchronizer"},
	})
	rewriter := NewControlledQueryRewriter(&QueryRewriterConfig{
		MaxExpansions:              4,
		EnableDomainTerms:          true,
		EnableRouteSpecificRewrite: true,
		DomainTerms:                provider,
	})

	result := rewriter.Rewrite(context.Background(), QueryRewriteRequest{
		Query:    "juc lock",
		Language: LanguageJava,
	})

	if !result.Applied {
		t.Fatalf("expected domain term rewrite to apply")
	}
	if result.TermDictVersion != "test-v1" {
		t.Fatalf("term dict version = %q, want test-v1", result.TermDictVersion)
	}
	if !strings.Contains(result.DenseQuery, "java.util.concurrent") {
		t.Fatalf("dense query = %q, want canonical domain term", result.DenseQuery)
	}
	if !strings.Contains(result.SparseQuery, "abstract queued synchronizer") {
		t.Fatalf("sparse query = %q, want aggressive domain term expansion", result.SparseQuery)
	}
	if result.DenseQuery == result.SparseQuery {
		t.Fatalf("expected route-specific queries to differ, got %q", result.DenseQuery)
	}
}

func TestControlledQueryRewriterAppliesModelAssistedShadow(t *testing.T) {
	rewriter := NewControlledQueryRewriter(&QueryRewriterConfig{
		MaxExpansions:              4,
		EnableRouteSpecificRewrite: true,
		EnableModelAssistedRewrite: true,
		ModelRewriteShadowRatio:    1,
		ModelAssistant: stubModelRewriteAssistant{
			suggestion: ModelRewriteSuggestion{
				NormalizedTerms: []string{"compare and swap"},
				Aliases:         []string{"atomic compare swap"},
				MustKeepTerms:   []string{"cas"},
				RiskLevel:       modelRewriteRiskLow,
			},
		},
	})

	result := rewriter.Rewrite(context.Background(), QueryRewriteRequest{Query: "cas retry"})

	if !result.ModelRewriteApplied {
		t.Fatalf("expected model-assisted rewrite to be applied")
	}
	if !result.ModelRewriteShadow {
		t.Fatalf("expected model-assisted rewrite to stay in shadow mode")
	}
	if !strings.Contains(result.SparseQuery, "compare and swap") {
		t.Fatalf("sparse query = %q, want model expansion", result.SparseQuery)
	}
	if !strings.Contains(result.Strategy, RewriteStrategyModelAssistedShadow) {
		t.Fatalf("strategy = %q, want %q marker", result.Strategy, RewriteStrategyModelAssistedShadow)
	}
}

func TestHybridSearchRequestApplyControlledRewriteFallback(t *testing.T) {
	req := &HybridSearchRequest{
		Query:         "mq rpc",
		OriginalQuery: "mq rpc",
	}

	req.applyControlledRewrite(context.Background(), NewControlledQueryRewriter(&QueryRewriterConfig{
		MaxExpansions:              3,
		EnableRouteSpecificRewrite: true,
	}))

	if !req.RewriteApplied {
		t.Fatalf("expected request rewrite to be applied")
	}
	if req.FinalQuery == req.OriginalQuery {
		t.Fatalf("expected final query to differ from original query")
	}
	if req.DenseQuery == "" || req.SparseQuery == "" {
		t.Fatalf("expected route queries to be populated")
	}
	if req.DenseQuery == req.SparseQuery {
		t.Fatalf("expected dense and sparse route queries to differ for route-specific rewrite")
	}
}
```

文件：`backend/internal/milvus/evaluation/profiles.go`

```go
{
	Name:                       "parent_child+advanced_rewrite",
	Label:                      "Parent Child + Advanced Rewrite",
	Candidate:                  true,
	Mode:                       "hybrid",
	EnableQueryRewrite:         true,
	EnableDynamicTopK:          true,
	EnableAdvancedRerank:       true,
	EnableParentChildRetrieval: true,
	EnableDomainTerms:          true,
	EnableRouteSpecificRewrite: true,
	EnableModelAssistedRewrite: true,
	CandidateTopK:              10,
},
```

文件：`backend/internal/milvus/evaluation/types.go`

```go
EnableQueryRewrite          bool    `json:"enable_query_rewrite,omitempty"`
EnableDynamicTopK           bool    `json:"enable_dynamic_topk,omitempty"`
EnableAdvancedRerank        bool    `json:"enable_advanced_rerank,omitempty"`
EnableParentChildRetrieval  bool    `json:"enable_parent_child_retrieval,omitempty"`
EnableStrategicTopK         bool    `json:"enable_strategic_topk,omitempty"`
EnableEvidenceRefusal       bool    `json:"enable_evidence_refusal,omitempty"`
EnableCitationConsistency   bool    `json:"enable_citation_consistency,omitempty"`
EnableDomainTerms           bool    `json:"enable_domain_terms,omitempty"`
EnableRouteSpecificRewrite  bool    `json:"enable_route_specific_rewrite,omitempty"`
EnableModelAssistedRewrite  bool    `json:"enable_model_assisted_rewrite,omitempty"`
CandidateTopK               int     `json:"candidate_top_k,omitempty"`
DenseWeight                 float64 `json:"dense_weight,omitempty"`
SparseWeight                float64 `json:"sparse_weight,omitempty"`
MinTopK                     int     `json:"min_top_k,omitempty"`
MaxTopK                     int     `json:"max_top_k,omitempty"`
TokenBudget                 int     `json:"token_budget,omitempty"`
MinAnswerChunks             int     `json:"min_answer_chunks,omitempty"`
RewriteMaxExpansions        int     `json:"rewrite_max_expansions,omitempty"`
RerankTimeoutMS             int     `json:"rerank_timeout_ms,omitempty"`
RerankModel                 string  `json:"rerank_model,omitempty"`
ParentChildFillStrategy     string  `json:"parent_child_fill_strategy,omitempty"`
ParentChildWindowSize       int     `json:"parent_child_window_size,omitempty"`
ParentChildMaxTokens        int     `json:"parent_child_max_tokens,omitempty"`
StrategicTopKMinK           int     `json:"strategic_topk_min_k,omitempty"`
StrategicTopKMaxK           int     `json:"strategic_topk_max_k,omitempty"`
StrategicTopKBudgetRatio    float64 `json:"strategic_topk_budget_ratio,omitempty"`
EvidenceMinRerankScore      float64 `json:"evidence_min_rerank_score,omitempty"`
EvidenceMinDensity          float64 `json:"evidence_min_density,omitempty"`
EvidenceMinCitationCoverage float64 `json:"evidence_min_citation_coverage,omitempty"`
CitationCheckThreshold      float64 `json:"citation_check_threshold,omitempty"`
CitationCheckVersion        string  `json:"citation_check_version,omitempty"`
DomainTermTimeoutMS         int     `json:"domain_term_timeout_ms,omitempty"`
ModelRewriteTimeoutMS       int     `json:"model_rewrite_timeout_ms,omitempty"`
ModelRewriteShadowRatio     float64 `json:"model_rewrite_shadow_ratio,omitempty"`
```

### 这段代码在做什么

测试和 profile 一起做了两件事：

1. 单测保证行为不回退。
2. 评测 profile 保证实验有独立身份，可以单独对比收益。

### 为什么要这样做

如果只有代码没有 profile，这个功能上线以后很容易变成“夹在别的策略里一起观察”。那样即使效果有变化，也不知道到底是 parent-child、dynamic topK 还是 advanced rewrite 带来的。

`parent_child+advanced_rewrite` 这个候选 profile 的价值就在这里：它给 L6 一张独立的成绩单。

### 它如何衔接下一步

到这里，L6 的实现已经完整闭环。剩下的事情就不是“怎么写代码”，而是“怎么验证它真的工作正常”。

## 如何验证

建议至少按下面顺序验证：

1. 跑单测：

```bash
cd backend
go test ./internal/milvus/retrieval ./internal/config ./internal/milvus/evaluation
```

2. 检查配置快照日志里是否出现这些字段：
   `domain_terms`、`route_specific_rewrite`、`model_assisted_rewrite`、`domain_term_timeout_ms`、`model_rewrite_timeout_ms`、`model_rewrite_shadow_ratio`

3. 用几个典型 query 做人工验证：
   `juc lock`
   `gmp 调度`
   `cas retry`
   `site:example.com "jvm"`

4. 观察文档元数据是否出现这些键：
   `route_final_query`
   `route_rewrite_strategy`
   `term_dict_scope`
   `term_dict_version`
   `term_hits`
   `model_rewrite_applied`
   `model_rewrite_shadow`
   `model_rewrite_risk_level`
   `model_rewrite_terms`

5. 跑 retrieval 评测时，确认 `parent_child+advanced_rewrite` 能作为独立 candidate 输出结果。

成功时你应该能看到这些现象：

1. 领域词命中时，`term_hits` 和 `term_dict_version` 会被写出来。
2. 开启 route-specific 时，`DenseQuery` 和 `SparseQuery` 在部分 query 上会不同。
3. 模型辅助触发时，策略串里会带上 `model_assisted_shadow` 和 `model_risk=...`。
4. 高风险 query 或黑名单 query 不会被模型辅助接管。

常见失败信号通常有这些：

1. `EnableModelAssistedRewrite=true` 但没有任何 shadow 命中，通常要先检查 `model_rewrite_shadow_ratio` 是否过低。
2. `DenseQuery` 和 `SparseQuery` 总是一样，通常说明 `EnableRouteSpecificRewrite` 没打开，或者扩展预算过小。
3. `term_hits` 为空但你明明配置了术语，通常要检查 `language/category/kb_scope` 是否和 scope key 对得上。

## 取舍与后续优化

这个版本优先优化的是四件事：

1. 安全性，尤其是模型辅助只做结构化 shadow。
2. 可解释性，尤其是 route 级策略和词表命中记录。
3. 可回退性，关闭高级开关后自然退回规则版 rewrite。
4. 可评测性，能够单独输出 advanced rewrite 的收益。

它暂时没有解决的点也很明确：

1. 领域词表现在默认还是静态提供者，还不是真正的热更新服务。
2. 模型辅助目前是启发式 assistant，真实接大模型时还需要更严格的契约和限流。
3. route-specific 目前只分 dense 和 sparse，两路之外的更多 route 还没展开。

后续最自然的演进方向通常是这三个：

1. 把 `StaticDomainTermProvider` 换成可热更新、可版本回滚的词表服务。
2. 给模型辅助增加更明确的观测指标，比如 shadow 命中率、低风险通过率、Rewrite Gain。
3. 把 route-specific 的收益继续细分到日志和 dashboard，真正看清每条 route 的贡献。

如果你读到这里，最应该记住的一句话是：

L6 的重点不是“把 query 改得更花”，而是把 rewrite 变成一套能安全增强召回、还能解释清楚每一步为什么这样做的工程能力。
