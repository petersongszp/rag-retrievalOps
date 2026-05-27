# KB L4 证据不足拒答策略与标准拒答模板实现教程

## 背景

这层功能先说结论：它不是简单地在“没搜到结果”时返回空数组，而是在检索链路里明确判断“当前证据够不够支撑回答”，如果不够，就统一返回标准拒答模板。

如果没有这层机制，系统会有两个很典型的问题：

1. 检索命中了几条内容，但这些内容分数很弱、引用信息不完整，模型还是可能“硬答”。
2. 同样都是“答不了”，有时返回空结果，有时返回几条模糊片段，有时前端根本不知道该怎么提示用户。

所以这里其实做了两件彼此配合的事：

1. 在检索层增加“证据门禁”（Evidence Gate），判断这次请求是否应该拒答。
2. 在 API 层增加“标准拒答模板”，把拒答原因、提示文案和建议动作统一包装成稳定 JSON。

## 这篇教程会做什么

看完这篇教程，你应该能从头复现这样一条链路：

1. 在配置里打开 `enable_evidence_refusal`，并设置三类阈值。
2. 在 `HybridRetriever` 初始化时把这些阈值注入到检索器。
3. 在检索主链路里，对召回结果做证据充足性判断。
4. 把拒答结果写入统一的 `SearchMetrics`，继续往上层传递。
5. 在知识库检索 API 中，把拒答结果转换成标准 JSON 模板。
6. 用单元测试验证“会拒答”“不会误拒答”“响应结构稳定”。

这篇教程主要覆盖这些文件：

1. `backend/internal/config/config.go`
2. `backend/config.example.yaml`
3. `backend/internal/milvus/init.go`
4. `backend/internal/milvus/retrieval/evidence_gate.go`
5. `backend/internal/milvus/retrieval/hybrid_search.go`
6. `backend/internal/milvus/retrieval/search.go`
7. `backend/api/handler/kb/handler_refusal.go`
8. `backend/api/handler/kb/handler.go`
9. `backend/internal/milvus/retrieval/evidence_gate_test.go`
10. `backend/api/handler/kb/handler_refusal_test.go`

如果先用一句人话概括最终控制流，可以这样理解：

1. 检索器先把文档找出来。
2. 证据门禁判断“这些文档能不能支撑回答”。
3. 如果不能，就把原因写进指标。
4. API 层看到这个结果后，不再返回证据列表，而是返回标准拒答模板。

## 需要先理解的术语

这里先把初学者最容易卡住的地方说清楚。真正难的不是 if/else，而是几个“听起来像算法词”的概念。

### 什么是证据门禁

证据门禁（Evidence Gate）可以先理解成“回答前的最后一道安检”。

它不关心模型能不能组织语言，而是先问一个更基础的问题：当前检索到的证据，是否足够让系统负责任地回答。

比如：

1. 只搜到 1 条特别泛的文档片段，这通常不够。
2. 搜到的文档没有 `document_id` 或 `chunk_id`，说明无法稳定引用来源，也不够。
3. 两条高分文档在同一个问题上出现明显矛盾，这时宁可拒答，也不要硬生成。

### 什么是证据密度

证据密度（`evidence_density`）指的是：当前候选文档里，高质量证据占比有多高。

举个小例子：

1. 一共 5 条文档。
2. 其中 4 条分数都超过阈值。
3. 那么证据密度就是 `4 / 5 = 0.8`。

这比单看“最高分是多少”更稳，因为最高分高，不代表整体证据都靠谱。

### 什么是引用覆盖率

引用覆盖率（这里代码里叫 `citation_support_score`）可以先理解成“前几条关键证据里，有多少条真的带着可引用来源”。

在这套实现里，系统会检查前 3 条文档：

1. 有没有 `document_id`
2. 有没有 `chunk_id`，或者至少文档本身有 `ID`

如果前 3 条里只有 1 条满足，就得到 `1 / 3 = 0.333...`。这通常说明回答虽然可能“看起来像有依据”，但引用落地能力不够。

### 什么是标准拒答模板

标准拒答模板不是一句固定文案，而是一个结构化响应对象，里面至少有这些信息：

1. `reason`：拒答原因代码
2. `message`：面向用户的标准提示
3. `suggestions`：下一步建议
4. `citation_support_score`：这次证据支持度

这样前端、日志、离线评估和后续运营分析都可以基于同一份数据工作。

### 初学者最容易误解的三个点

1. “搜到了文档”不等于“可以安全回答”。这正是证据门禁存在的原因。
2. 拒答模板不放在检索层里，是因为检索层负责判定，API 层负责对外协议。
3. 这不是在“增加复杂度”，而是在解决一个很具体的工程问题：避免模型在证据不足时乱答。

## 整体流程

在看代码前，先抓住总流程：

1. 服务启动时读取 RAG Phase 3 配置，并校验拒答阈值是否合法。
2. `InitMilvusManager` 创建 `HybridRetriever` 时，把 `EvidenceGateConfig` 注入进去。
3. 混合检索完成后，`HybridRetriever` 调用 `EvaluateEvidenceGate` 计算门禁结果。
4. 检索结果和门禁结果一起写入 `SearchMetrics`。
5. `Retrieve` API 调用 `resolveEvidenceGateOutcome`，决定是否生成标准拒答模板。
6. 如果生成了拒答模板，原始 `items` 会被清空，响应里改为返回 `evidence_gate_result + refusal`。

如果你只记一个顺序，可以记这个：

先判定“能不能答”，再决定“怎么返回”。

## 分步实现

## 第 1 步：先把拒答开关和阈值配置补齐

### 目标

让这套能力可以被安全地打开、关闭、调参和校验，而不是把阈值写死在代码里。

### 文件

`backend/internal/config/config.go`

`backend/config.example.yaml`

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

if c.RAG.Phase3.EvidenceMinRerankScore <= 0 {
	c.RAG.Phase3.EvidenceMinRerankScore = 0.55
}
if c.RAG.Phase3.EvidenceMinDensity <= 0 {
	c.RAG.Phase3.EvidenceMinDensity = 0.2
}
if c.RAG.Phase3.EvidenceMinCitationCoverage <= 0 {
	c.RAG.Phase3.EvidenceMinCitationCoverage = 0.5
}

if value, ok, err := readEnvBool("RAG_ENABLE_EVIDENCE_REFUSAL"); err != nil {
	return err
} else if ok {
	c.RAG.FeatureFlags.EnableEvidenceRefusal = value
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
```

文件：`backend/config.example.yaml`

```yaml
rag:
  enabled: false
  environment: dev
  feature_flags:
    enable_prod_guard: false
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
  release:
    enabled: false
    stage: full
    internal_roles: ["admin"]
    canary_percent: 5
    batch_percent: 25
    user_allowlist: []
```

### 这段代码在做什么

这一层做了四件事：

1. 定义开关 `enable_evidence_refusal`。
2. 定义三类阈值：最低重排分、最低证据密度、最低引用覆盖率。
3. 在配置校验阶段，强制要求这些阈值都在 `[0,1]` 区间。
4. 给出默认值和环境变量覆盖入口，方便线上灰度调参。

### 为什么要这样做

最简单的写法当然是把阈值写死在 `evidence_gate.go` 里，但那会有三个问题：

1. 不同环境不能独立调参。
2. 线上出现误拒答时，没法快速降阈值或关开关。
3. 非法阈值要等到请求来了才暴露，排障会更痛苦。

这里把校验前置到配置层，本质上是在把“算法策略”变成“可治理的系统能力”。

### 它如何衔接下一步

有了这组配置之后，下一步就可以在检索器初始化阶段，把这些参数真正注入到 `HybridRetriever`。

## 第 2 步：在检索器初始化阶段注入证据门禁配置

### 目标

让混合检索器拿到完整的 `EvidenceGateConfig`，这样拒答判定才能进入主检索链路。

### 文件

`backend/internal/milvus/init.go`

### 完整代码

文件：`backend/internal/milvus/init.go`

```go
candidateTopK := cfg.Milvus.TopK * 2
if cfg.RAG.Phase2.CandidateTopK > 0 {
	candidateTopK = cfg.RAG.Phase2.CandidateTopK
}
hybridConfig := &retrieval.HybridRetrieverConfig{
	CandidateTopK: candidateTopK,
	DenseWeight:   cfg.RAG.Phase2.HybridDenseWeight,
	SparseWeight:  cfg.RAG.Phase2.HybridSparseWeight,
	SparseConfig: &retrieval.SparseRetrieverConfig{
		DefaultTopK: candidateTopK,
	},
	DynamicTopK: retrieval.DynamicTopKConfig{
		Enabled:              cfg.RAG.FeatureFlags.EnableDynamicTopK,
		MinTopK:              cfg.RAG.Phase2.MinTopK,
		MaxTopK:              cfg.RAG.Phase2.MaxTopK,
		TokenBudget:          cfg.RAG.Phase2.TokenBudget,
		MinAnswerChunks:      cfg.RAG.Phase2.MinAnswerChunks,
		StrategicEnabled:     cfg.RAG.FeatureFlags.EnableStrategicTopK,
		StrategicMinTopK:     cfg.RAG.Phase3.StrategicTopKMinK,
		StrategicMaxTopK:     cfg.RAG.Phase3.StrategicTopKMaxK,
		StrategicBudgetRatio: cfg.RAG.Phase3.StrategicTopKBudgetRatio,
	},
	ParentChild: retrieval.ParentChildConfig{
		Enabled:      cfg.RAG.FeatureFlags.EnableParentChildRetrieval,
		FillStrategy: cfg.RAG.Phase3.ParentChildFillStrategy,
		WindowSize:   cfg.RAG.Phase3.ParentChildWindowSize,
		MaxTokens:    cfg.RAG.Phase3.ParentChildMaxTokens,
	},
	EvidenceGate: retrieval.EvidenceGateConfig{
		Enabled:             cfg.RAG.FeatureFlags.EnableEvidenceRefusal,
		MinRerankScore:      cfg.RAG.Phase3.EvidenceMinRerankScore,
		MinEvidenceDensity:  cfg.RAG.Phase3.EvidenceMinDensity,
		MinCitationCoverage: cfg.RAG.Phase3.EvidenceMinCitationCoverage,
	},
}
fallbackReranker := retrieval.NewJaccardReranker(&retrieval.JaccardRerankerConfig{
	TopK:      candidateTopK,
	ModelName: retrieval.DefaultRerankModelJaccardV1,
	Version:   retrieval.DefaultRerankVersion,
})
if cfg.RAG.FeatureFlags.EnableAdvancedRerank {
	timeout := time.Duration(cfg.RAG.Phase2.RerankTimeoutMS) * time.Millisecond
	hybridConfig.RerankerImpl = retrieval.NewConfigurableReranker(
		cfg.RAG.Phase2.RerankModel,
		timeout,
		retrieval.NewJaccardReranker(&retrieval.JaccardRerankerConfig{
			TopK:      candidateTopK,
			ModelName: cfg.RAG.Phase2.RerankModel,
			Version:   cfg.RAG.Phase2.RerankModel,
		}),
		fallbackReranker,
	)
} else {
	hybridConfig.RerankerImpl = fallbackReranker
}
if cfg.RAG.FeatureFlags.EnableQueryRewrite {
	hybridConfig.QueryRewriter = retrieval.NewControlledQueryRewriter(&retrieval.QueryRewriterConfig{
		MaxExpansions: cfg.RAG.Phase2.RewriteMaxExpansions,
	})
}
hybridRetriever, err := retrieval.NewHybridRetriever(retrieverService, hybridConfig)
if err != nil {
	return nil, fmt.Errorf("failed to initialize hybrid retriever: %w", err)
}
manager.HybridRetriever = hybridRetriever
```

### 这段代码在做什么

这一步的核心作用是“接线”：

1. 创建 `HybridRetrieverConfig`。
2. 把动态 TopK、父子块补全文、拒答门禁等 Phase 3 能力统一挂进去。
3. 让同一个 `HybridRetriever` 在运行时可以访问这些配置。

### 为什么要这样做

如果把拒答逻辑直接写进 handler 层，看起来更简单，但会有两个问题：

1. handler 看不到完整的检索中间态，比如真实的 `evidence_density`。
2. 不同调用方如果都要复用检索器，就会重复写一套门禁逻辑。

把配置注入到检索器，意味着“是否应该拒答”成为检索层职责，而不是某个 API 的临时判断。

### 它如何衔接下一步

接线完成后，真正的核心逻辑就会发生在 `EvaluateEvidenceGate` 里，也就是下一步要看的证据门禁实现。

## 第 3 步：实现证据门禁本身

### 目标

把“证据不足时拒答”的规则封装成一个独立、可测试、可复用的检索层模块。

### 文件

`backend/internal/milvus/retrieval/evidence_gate.go`

### 完整代码

文件：`backend/internal/milvus/retrieval/evidence_gate.go`

```go
package retrieval

import (
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"
)

const (
	EvidenceGateResultDisabled     = "disabled"
	EvidenceGateResultPass         = "pass"
	EvidenceGateResultRefused      = "refused"
	EvidenceGateResultDegradedPass = "degraded_pass"

	RefusalReasonNoRetrievalHit            = "No-Retrieval-Hit"
	RefusalReasonLowRerankConfidence       = "Low-Rerank-Confidence"
	RefusalReasonInsufficientCitationCover = "Insufficient-Citation-Coverage"
	RefusalReasonContradictoryEvidence     = "Contradictory-Evidence"
	RefusalReasonOutOfKBScope              = "Out-Of-KB-Scope"
	EmptyReasonEvidenceRefusal             = "Evidence-Refusal"
)

type EvidenceGateConfig struct {
	Enabled             bool
	MinRerankScore      float64
	MinEvidenceDensity  float64
	MinCitationCoverage float64
}

type EvidenceGateOutcome struct {
	Result               string
	RefusalReason        string
	CitationSupportScore float64
	Error                string
}

func (o EvidenceGateOutcome) Refused() bool {
	return strings.EqualFold(strings.TrimSpace(o.Result), EvidenceGateResultRefused)
}

func EvaluateEvidenceGate(query string, docs []*schema.Document, metrics SearchMetrics, cfg EvidenceGateConfig) EvidenceGateOutcome {
	if !cfg.Enabled {
		return EvidenceGateOutcome{Result: EvidenceGateResultDisabled}
	}

	thresholds, err := resolveEvidenceThresholds(strings.TrimSpace(query), cfg)
	if err != nil {
		return EvidenceGateOutcome{
			Result: EvidenceGateResultDegradedPass,
			Error:  err.Error(),
		}
	}

	if len(docs) == 0 {
		return EvidenceGateOutcome{
			Result:        EvidenceGateResultRefused,
			RefusalReason: RefusalReasonNoRetrievalHit,
		}
	}

	citationSupportScore := computeCitationSupportScore(docs)
	maxRerankScore := computeMaxEvidenceScore(docs)
	evidenceDensity := metrics.EvidenceDensity
	if evidenceDensity <= 0 {
		evidenceDensity = computeFallbackEvidenceDensity(docs, thresholds.MinRerankScore)
	}

	outcome := EvidenceGateOutcome{
		Result:               EvidenceGateResultPass,
		CitationSupportScore: citationSupportScore,
	}

	if hasContradictoryEvidence(query, docs, thresholds.MinRerankScore) {
		outcome.Result = EvidenceGateResultRefused
		outcome.RefusalReason = RefusalReasonContradictoryEvidence
		return outcome
	}

	if isLikelyOutOfKBScope(query, docs, maxRerankScore, evidenceDensity, thresholds) {
		outcome.Result = EvidenceGateResultRefused
		outcome.RefusalReason = RefusalReasonOutOfKBScope
		return outcome
	}

	if maxRerankScore < thresholds.MinRerankScore || evidenceDensity < thresholds.MinEvidenceDensity {
		outcome.Result = EvidenceGateResultRefused
		outcome.RefusalReason = RefusalReasonLowRerankConfidence
		return outcome
	}

	if citationSupportScore < thresholds.MinCitationCoverage {
		outcome.Result = EvidenceGateResultRefused
		outcome.RefusalReason = RefusalReasonInsufficientCitationCover
		return outcome
	}

	return outcome
}

type evidenceThresholds struct {
	MinRerankScore      float64
	MinEvidenceDensity  float64
	MinCitationCoverage float64
}

func resolveEvidenceThresholds(query string, cfg EvidenceGateConfig) (evidenceThresholds, error) {
	minRerank := clampNormalizedRatio(cfg.MinRerankScore)
	minDensity := clampNormalizedRatio(cfg.MinEvidenceDensity)
	minCitation := clampNormalizedRatio(cfg.MinCitationCoverage)
	if minRerank <= 0 || minDensity <= 0 || minCitation <= 0 {
		return evidenceThresholds{}, ErrInvalidEvidenceGateConfig
	}

	multiplier := evidenceSensitivityMultiplier(query)
	return evidenceThresholds{
		MinRerankScore:      clampNormalizedRatio(minRerank * multiplier),
		MinEvidenceDensity:  clampNormalizedRatio(minDensity * multiplier),
		MinCitationCoverage: clampNormalizedRatio(minCitation * multiplier),
	}, nil
}

func evidenceSensitivityMultiplier(query string) float64 {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return 1
	}

	multiplier := 1.0
	termCount := len(strings.Fields(trimmed))
	runeCount := utf8.RuneCountInString(trimmed)

	if isBroadQuery(trimmed) {
		multiplier += 0.12
	}
	if runeCount <= 12 || termCount <= 2 {
		multiplier += 0.05
	}
	if containsAnyFold(trimmed, "how", "why", "compare", "difference", "区分", "对比", "怎么", "如何", "为什么") {
		multiplier += 0.05
	}

	return math.Min(multiplier, 1.25)
}

func computeCitationSupportScore(docs []*schema.Document) float64 {
	if len(docs) == 0 {
		return 0
	}

	limit := minInt(len(docs), 3)
	citableCount := 0
	for i := 0; i < limit; i++ {
		doc := docs[i]
		if doc == nil {
			continue
		}
		documentID := getFloatMetadataValue(doc, "document_id")
		chunkID := strings.TrimSpace(firstNonEmptyMetadata(doc, "chunk_id"))
		if documentID > 0 && (chunkID != "" || strings.TrimSpace(doc.ID) != "") {
			citableCount++
		}
	}
	return float64(citableCount) / float64(limit)
}

func computeMaxEvidenceScore(docs []*schema.Document) float64 {
	maxScore := 0.0
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		score := getFloatMetadataValue(doc, "rerank_score")
		if score <= 0 {
			score = getFloatMetadataValue(doc, "score")
		}
		if score > maxScore {
			maxScore = score
		}
	}
	return maxScore
}

func computeFallbackEvidenceDensity(docs []*schema.Document, minScore float64) float64 {
	if len(docs) == 0 {
		return 0
	}
	strongCount := 0
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		score := getFloatMetadataValue(doc, "rerank_score")
		if score <= 0 {
			score = getFloatMetadataValue(doc, "score")
		}
		if score >= minScore {
			strongCount++
		}
	}
	return float64(strongCount) / float64(len(docs))
}

func hasContradictoryEvidence(query string, docs []*schema.Document, minScore float64) bool {
	if len(docs) < 2 {
		return false
	}

	candidates := make([]*schema.Document, 0, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		score := getFloatMetadataValue(doc, "rerank_score")
		if score <= 0 {
			score = getFloatMetadataValue(doc, "score")
		}
		if score >= minScore {
			candidates = append(candidates, doc)
		}
	}
	if len(candidates) < 2 {
		return false
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return computeMaxEvidenceScore([]*schema.Document{candidates[i]}) > computeMaxEvidenceScore([]*schema.Document{candidates[j]})
	})

	queryTokens := tokenize(query)
	first := candidates[0]
	second := candidates[1]
	if !docTouchesQuery(first, queryTokens) || !docTouchesQuery(second, queryTokens) {
		return false
	}

	return containsNegationCue(first.Content) != containsNegationCue(second.Content)
}

func isLikelyOutOfKBScope(query string, docs []*schema.Document, maxScore, evidenceDensity float64, thresholds evidenceThresholds) bool {
	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return false
	}

	coverage := computeQueryCoverage(queryTokens, docs)
	if coverage >= 0.2 {
		return false
	}

	return maxScore < thresholds.MinRerankScore || evidenceDensity < thresholds.MinEvidenceDensity
}

func computeQueryCoverage(queryTokens map[string]struct{}, docs []*schema.Document) float64 {
	if len(queryTokens) == 0 {
		return 0
	}
	matched := 0
	for token := range queryTokens {
		if token == "" {
			continue
		}
		for _, doc := range docs {
			if doc == nil {
				continue
			}
			contentTokens := tokenize(doc.Content)
			if _, ok := contentTokens[token]; ok {
				matched++
				break
			}
		}
	}
	return float64(matched) / float64(len(queryTokens))
}

func docTouchesQuery(doc *schema.Document, queryTokens map[string]struct{}) bool {
	if doc == nil || len(queryTokens) == 0 {
		return false
	}
	contentTokens := tokenize(doc.Content)
	for token := range queryTokens {
		if _, ok := contentTokens[token]; ok {
			return true
		}
	}
	return false
}

func containsNegationCue(content string) bool {
	return containsAnyFold(content, " not ", " no ", " never ", " without ", " cannot ", " can't ", "禁止", "不能", "不支持", "不会", "无", "未")
}

func containsAnyFold(content string, keywords ...string) bool {
	lower := strings.ToLower(content)
	for _, keyword := range keywords {
		if strings.Contains(lower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func getFloatMetadataValue(doc *schema.Document, key string) float64 {
	if doc == nil || doc.MetaData == nil {
		return 0
	}
	if value, ok := doc.MetaData[key]; ok {
		if score, ok := castScore(value); ok {
			return score
		}
	}
	return 0
}

func firstNonEmptyMetadata(doc *schema.Document, keys ...string) string {
	if doc == nil || doc.MetaData == nil {
		return ""
	}
	for _, key := range keys {
		if value := strings.TrimSpace(getStringMetadata(doc.MetaData, key)); value != "" {
			return value
		}
	}
	return ""
}

func clampNormalizedRatio(value float64) float64 {
	switch {
	case value < 0:
		return 0
	case value > 1:
		return 1
	default:
		return value
	}
}

var ErrInvalidEvidenceGateConfig = errInvalidEvidenceGateConfig("invalid evidence gate config")

type errInvalidEvidenceGateConfig string

func (e errInvalidEvidenceGateConfig) Error() string {
	return string(e)
}
```

### 这段代码在做什么

这份文件做的不是“一个判断”，而是一整套门禁策略：

1. 如果功能没打开，直接返回 `disabled`。
2. 如果配置非法，不阻断主链路，而是返回 `degraded_pass`。
3. 如果没有任何召回文档，返回 `No-Retrieval-Hit`。
4. 如果高分证据互相矛盾，返回 `Contradictory-Evidence`。
5. 如果问题很可能超出知识库范围，返回 `Out-Of-KB-Scope`。
6. 如果分数或证据密度不足，返回 `Low-Rerank-Confidence`。
7. 如果引用支撑不够，返回 `Insufficient-Citation-Coverage`。

### 为什么要这样做

这里最关键的设计，不是阈值本身，而是“分原因拒答”。

如果我们只返回一个布尔值，比如 `should_refuse = true`，后面会立刻遇到几个问题：

1. 前端不知道应该怎么提示用户。
2. 运营看日志时不知道是“没召回到”还是“引用不完整”。
3. 后续调参时无法区分是哪种问题最常见。

所以当前实现不是只有“拒答/不拒答”，而是同时给出“为什么拒答”。这会让后续的产品提示和离线分析都清晰很多。

另一个容易忽略的点是 `degraded_pass`。它的意思不是“配置错了也无所谓”，而是“门禁自身出故障时，不要因为门禁把整个检索服务打死”。这是一个典型的工程兜底思路。

### 它如何衔接下一步

到这里为止，检索器已经能算出门禁结果了。下一步要解决的是：这些结果怎么沿着检索链路传到 API 层。

## 第 4 步：把门禁结果挂到检索指标上

### 目标

让上层代码拿到的不只是文档列表，还能拿到门禁结果、拒答原因和引用支持分数。

### 文件

`backend/internal/milvus/retrieval/search.go`

`backend/internal/milvus/retrieval/hybrid_search.go`

### 完整代码

文件：`backend/internal/milvus/retrieval/search.go`

```go
type SearchMetrics struct {
	EmbeddingMs          int64
	SearchMs             int64
	PostprocessMs        int64
	HitCount             int
	TruncatedCount       int
	CandidateTopK        int
	FinalTopK            int
	TokenBudget          int
	TruncateReason       string
	Strategy             string
	ReleaseStage         string
	ReleaseReason        string
	RetrieverVersion     string
	RewriteApplied       bool
	EmptyReason          string
	RerankMs             int64
	RerankModel          string
	RerankVersion        string
	RerankFallback       bool
	RerankReason         string
	DenseHits            int
	SparseHits           int
	DenseContribution    int
	SparseContribution   int
	TopKPolicyVersion    string
	ScoreDistribution    string
	RerankGap            float64
	EvidenceDensity      float64
	TopKDecisionReason   string
	TokenBudgetRemain    int
	ContextTokens        int
	EvidenceGateResult   string
	RefusalReason        string
	CitationSupportScore float64
	EvidenceGateError    string
}

type SearchResult struct {
	Documents []*schema.Document
	Metrics   SearchMetrics
}
```

文件：`backend/internal/milvus/retrieval/hybrid_search.go`

```go
func (h *HybridRetriever) buildHybridResultMetrics(
	req *HybridSearchRequest,
	denseMetric SearchMetrics,
	denseHits, sparseHits int,
	sparseMS int64,
	topKDecision TopKDecision,
	totalMS int64,
	docs []*schema.Document,
	emptyReason string,
	evidenceOutcome EvidenceGateOutcome,
) *SearchResult {
	denseContribution, sparseContribution := countRouteContributions(docs)
	searchStageMS := denseMetric.SearchMs + sparseMS
	return &SearchResult{
		Documents: docs,
		Metrics: SearchMetrics{
			EmbeddingMs:          denseMetric.EmbeddingMs,
			SearchMs:             searchStageMS,
			PostprocessMs:        maxInt64(0, totalMS-denseMetric.EmbeddingMs-searchStageMS),
			HitCount:             denseHits + sparseHits,
			CandidateTopK:        topKDecision.CandidateTopK,
			FinalTopK:            len(docs),
			TokenBudget:          topKDecision.TokenBudget,
			TruncateReason:       topKDecision.TruncateReason,
			Strategy:             resolveRetrieveStrategy(h.parentChild),
			RetrieverVersion:     HybridRetrieverVersion,
			RewriteApplied:       req.RewriteApplied,
			EmptyReason:          emptyReason,
			DenseHits:            denseHits,
			SparseHits:           sparseHits,
			DenseContribution:    denseContribution,
			SparseContribution:   sparseContribution,
			TopKPolicyVersion:    topKDecision.PolicyVersion,
			ScoreDistribution:    topKDecision.ScoreDistribution,
			RerankGap:            topKDecision.RerankGap,
			EvidenceDensity:      topKDecision.EvidenceDensity,
			TopKDecisionReason:   topKDecision.DecisionReason,
			TokenBudgetRemain:    topKDecision.TokenBudgetRemaining,
			ContextTokens:        topKDecision.EstimatedContextTokens,
			EvidenceGateResult:   evidenceOutcome.Result,
			RefusalReason:        evidenceOutcome.RefusalReason,
			CitationSupportScore: evidenceOutcome.CitationSupportScore,
			EvidenceGateError:    evidenceOutcome.Error,
		},
	}
}

func (h *HybridRetriever) evaluateEvidenceGate(query string, docs []*schema.Document, topKDecision TopKDecision) EvidenceGateOutcome {
	if h == nil {
		return EvidenceGateOutcome{Result: EvidenceGateResultDisabled}
	}
	return EvaluateEvidenceGate(query, docs, SearchMetrics{
		EvidenceDensity: topKDecision.EvidenceDensity,
	}, h.config.EvidenceGate)
}
```

### 这段代码在做什么

这一层做的是“上传门禁结果”：

1. 在 `SearchMetrics` 里增加四个门禁字段。
2. 在 `HybridRetriever` 完成检索后调用 `evaluateEvidenceGate`。
3. 把门禁结果放进 `SearchResult.Metrics`，继续往上层传。

### 为什么要这样做

如果门禁结果只出现在 `evidence_gate.go` 的局部变量里，上层就根本用不到。那样的话：

1. handler 无法决定是否返回拒答模板。
2. 审计日志无法记录拒答原因。
3. 监控面板也看不到拒答率和原因分布。

所以 `SearchMetrics` 在这里起的是“跨层协议”作用。它把检索层算出来的事实，变成了 API 层可以消费的数据。

### 它如何衔接下一步

现在上层已经拿到“这次该不该拒答”以及“为什么拒答”。下一步就是把这些内部结果转换成对外的标准响应模板。

## 第 5 步：把门禁结果转换成标准拒答模板

### 目标

让 API 返回一个对前端稳定、对用户友好的拒答结构，而不是只暴露底层枚举值。

### 文件

`backend/api/handler/kb/handler_refusal.go`

### 完整代码

文件：`backend/api/handler/kb/handler_refusal.go`

```go
package kb

import (
	"fmt"
	"strings"

	"interview-agents/internal/config"
	"interview-agents/internal/milvus/retrieval"

	"github.com/cloudwego/eino/schema"
)

func resolveEvidenceGateOutcome(query string, docs []*schema.Document, metrics retrieval.SearchMetrics) retrieval.EvidenceGateOutcome {
	if strings.TrimSpace(metrics.EvidenceGateResult) != "" {
		return retrieval.EvidenceGateOutcome{
			Result:               metrics.EvidenceGateResult,
			RefusalReason:        metrics.RefusalReason,
			CitationSupportScore: metrics.CitationSupportScore,
			Error:                metrics.EvidenceGateError,
		}
	}

	return retrieval.EvaluateEvidenceGate(query, docs, metrics, retrieval.EvidenceGateConfig{
		Enabled:             config.Global.RAG.FeatureFlags.EnableEvidenceRefusal,
		MinRerankScore:      config.Global.RAG.Phase3.EvidenceMinRerankScore,
		MinEvidenceDensity:  config.Global.RAG.Phase3.EvidenceMinDensity,
		MinCitationCoverage: config.Global.RAG.Phase3.EvidenceMinCitationCoverage,
	})
}

func buildStandardRefusalPayload(outcome retrieval.EvidenceGateOutcome) *refusalPayload {
	if !outcome.Refused() {
		return nil
	}

	reasonLabel := normalizeRefusalReason(outcome.RefusalReason)
	return &refusalPayload{
		Reason:               outcome.RefusalReason,
		Message:              fmt.Sprintf("当前知识库证据不足，暂时不能可靠回答这个问题。触发原因：%s。", reasonLabel),
		Suggestions:          refusalSuggestions(outcome.RefusalReason),
		CitationSupportScore: outcome.CitationSupportScore,
	}
}

func normalizeRefusalReason(reason string) string {
	switch strings.TrimSpace(reason) {
	case retrieval.RefusalReasonNoRetrievalHit:
		return "未检索到可用证据"
	case retrieval.RefusalReasonLowRerankConfidence:
		return "候选证据置信度不足"
	case retrieval.RefusalReasonInsufficientCitationCover:
		return "引用覆盖不足"
	case retrieval.RefusalReasonContradictoryEvidence:
		return "候选证据存在冲突"
	case retrieval.RefusalReasonOutOfKBScope:
		return "问题超出知识库覆盖范围"
	default:
		if strings.TrimSpace(reason) == "" {
			return "证据校验未通过"
		}
		return strings.TrimSpace(reason)
	}
}

func refusalSuggestions(reason string) []string {
	base := []string{
		"补充更相关的知识库文档后再试。",
		"把问题缩小到更具体的模块、版本或场景。",
	}

	switch strings.TrimSpace(reason) {
	case retrieval.RefusalReasonNoRetrievalHit, retrieval.RefusalReasonOutOfKBScope:
		return append(base, "确认问题是否属于当前知识库范围。")
	case retrieval.RefusalReasonInsufficientCitationCover:
		return append(base, "优先上传包含明确章节、文件名或原始出处的材料。")
	case retrieval.RefusalReasonContradictoryEvidence:
		return append(base, "补充权威版本文档，或明确你希望采用的规范来源。")
	default:
		return base
	}
}
```

### 这段代码在做什么

这一层做了两件很关键的事：

1. `resolveEvidenceGateOutcome` 负责统一拿到门禁结果。
2. `buildStandardRefusalPayload` 负责把内部结果翻译成面向用户的模板。

这里有一个很重要的细节：如果下游检索器已经把门禁结果写进 `SearchMetrics`，handler 就直接复用；如果没有，再在这里兜底调用一次 `EvaluateEvidenceGate`。

### 为什么要这样做

对初学者来说，这一步最容易觉得“多余”。为什么不直接在 `evidence_gate.go` 里拼接最终提示文案？

原因是职责不同：

1. 检索层负责判断事实。
2. handler 层负责对外协议和文案。

如果把文案写进检索层，会让底层算法代码开始依赖 API 表达方式，后面一旦前端改协议，检索层也要跟着改，这会把耦合做得很重。

### 它如何衔接下一步

有了拒答模板构造函数后，最后一步就是在真正的 `Retrieve` API 里接上它，让正常结果和拒答结果走到同一条响应链路。

## 第 6 步：在知识库检索 API 中落地标准拒答响应

### 目标

当证据不足时，不再返回模糊结果，而是明确返回 `refusal` 对象和门禁状态。

### 文件

`backend/api/handler/kb/handler.go`

### 完整代码

文件：`backend/api/handler/kb/handler.go`

```go
type refusalPayload struct {
	Reason               string   `json:"reason"`
	Message              string   `json:"message"`
	Suggestions          []string `json:"suggestions,omitempty"`
	CitationSupportScore float64  `json:"citation_support_score,omitempty"`
}

type retrieveResponse struct {
	RequestID          string          `json:"request_id"`
	Items              []retrieveItem  `json:"items"`
	EvidenceGateResult string          `json:"evidence_gate_result,omitempty"`
	Refusal            *refusalPayload `json:"refusal,omitempty"`
}
```

文件：`backend/api/handler/kb/handler.go`

```go
	queryLower := strings.ToLower(req.Query)
	items := make([]retrieveItem, 0, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}

		documentID := getUint64Metadata(doc.MetaData, "document_id")
		if documentID == 0 {
			continue
		}
		storedDoc, err := model.KBDocumentDao.GetByID(documentID)
		if err != nil || storedDoc == nil {
			continue
		}
		if _, ok := allowedKBs[storedDoc.KbID]; !ok {
			continue
		}

		route := firstNonEmptyString(getStringMetadata(doc.MetaData, "route"), resolvePrimaryRoute(useHybrid))
		items = append(items, retrieveItem{
			Content: doc.Content,
			Score:   getFloat64Metadata(doc.MetaData, "score"),
			Citation: citation{
				KBID:          storedDoc.KbID,
				DocumentID:    documentID,
				ChunkID:       firstNonEmptyString(doc.ID, getStringMetadata(doc.MetaData, "chunk_id")),
				FileName:      firstNonEmptyString(getStringMetadata(doc.MetaData, "file_name"), storedDoc.FileName),
				ChunkIndex:    getIntMetadata(doc.MetaData, "chunk_index"),
				SnippetOffset: computeSnippetOffset(doc.Content, queryLower),
			},
			Source: source{
				Route:            route,
				Collection:       firstNonEmptyString(getStringMetadata(doc.MetaData, "collection"), collection),
				RetrieverVersion: firstNonEmptyString(getStringMetadata(doc.MetaData, "retriever_version"), searchMetrics.RetrieverVersion),
			},
		})
	}

	evidenceOutcome := resolveEvidenceGateOutcome(req.Query, docs, searchMetrics)
	searchMetrics.EvidenceGateResult = evidenceOutcome.Result
	searchMetrics.RefusalReason = evidenceOutcome.RefusalReason
	searchMetrics.CitationSupportScore = evidenceOutcome.CitationSupportScore
	searchMetrics.EvidenceGateError = evidenceOutcome.Error
	refusal := buildStandardRefusalPayload(evidenceOutcome)

	resultStatus := model.RetrieveResultStatusSuccess
	emptyReason := searchMetrics.EmptyReason
	if refusal != nil {
		items = []retrieveItem{}
		resultStatus = model.RetrieveResultStatusFilteredOut
		emptyReason = retrieval.EmptyReasonEvidenceRefusal
	} else if len(items) == 0 {
		if searchMetrics.HitCount > 0 {
			resultStatus = model.RetrieveResultStatusFilteredOut
			emptyReason = firstNonEmptyString(emptyReason, retrieval.EmptyReasonAfterFilter)
		} else {
			resultStatus = model.RetrieveResultStatusNoResult
			emptyReason = firstNonEmptyString(emptyReason, retrieval.EmptyReasonAfterRetrieve)
		}
	} else {
		emptyReason = firstNonEmptyString(emptyReason, retrieval.EmptyReasonNone)
	}
	searchMetrics.EmptyReason = emptyReason

	retrieveLog := &model.KBRetrieveLog{
		RequestID:            requestID,
		UserID:               userID,
		KBIDs:                formatKBIDs(kbIDs),
		Query:                req.Query,
		FinalQuery:           firstNonEmptyString(extractFinalQuery(docs), req.Query),
		Expr:                 expr,
		TopK:                 topK,
		CandidateTopK:        searchMetrics.CandidateTopK,
		FinalTopK:            searchMetrics.FinalTopK,
		TokenBudget:          searchMetrics.TokenBudget,
		TruncateReason:       searchMetrics.TruncateReason,
		Rewrite:              extractRewriteQuery(docs),
		RewriteStrategy:      extractRewriteStrategy(docs),
		RewriteApplied:       searchMetrics.RewriteApplied || extractRewriteApplied(docs),
		Strategy:             searchMetrics.Strategy,
		ReleaseStage:         searchMetrics.ReleaseStage,
		ReleaseReason:        searchMetrics.ReleaseReason,
		Routes:               resolveRetrieveRoutes(useHybrid),
		Collection:           collection,
		RetrieverVersion:     searchMetrics.RetrieverVersion,
		EmptyReason:          emptyReason,
		FinalCount:           len(items),
		TruncatedCount:       searchMetrics.TruncatedCount,
		DenseHits:            searchMetrics.DenseHits,
		SparseHits:           searchMetrics.SparseHits,
		DenseContribution:    searchMetrics.DenseContribution,
		SparseContribution:   searchMetrics.SparseContribution,
		EvidenceGateResult:   searchMetrics.EvidenceGateResult,
		RefusalReason:        searchMetrics.RefusalReason,
		CitationSupportScore: searchMetrics.CitationSupportScore,
		EvidenceGateError:    searchMetrics.EvidenceGateError,
		ResultStatus:         resultStatus,
		EmbeddingMs:          searchMetrics.EmbeddingMs,
		SearchMs:             searchMetrics.SearchMs,
		PostprocessMs:        searchMetrics.PostprocessMs,
		RerankMs:             searchMetrics.RerankMs,
		RerankModel:          searchMetrics.RerankModel,
		DurationMs:           durationMs,
		TimeoutMs:            retrieveTimeout.Milliseconds(),
	}
	persistRetrieveLog(retrieveLog)

	response.Success(ctx, c, retrieveResponse{
		RequestID:          requestID,
		Items:              items,
		EvidenceGateResult: searchMetrics.EvidenceGateResult,
		Refusal:            refusal,
	})
```

### 这段代码在做什么

这一层真正把“拒答策略”变成了“对外行为”：

1. 先按正常流程把文档转换成 `items`。
2. 再根据 `evidenceOutcome` 决定是否应该拒答。
3. 如果拒答，就清空 `items`，同时把 `result_status` 标记为 `filtered_out`。
4. 最后把 `evidence_gate_result` 和 `refusal` 一起返回给调用方。

### 为什么要这样做

这里一个很重要的工程选择是：拒答时把 `items` 清空，而不是把弱证据也一起返回。

如果做更简单的实现，比如“既返回文档，又返回 refusal”，前端和模型调用方很容易出现歧义：

1. 前端不知道是优先展示拒答，还是优先展示片段。
2. 有的调用方可能忽略 `refusal`，继续把 `items` 当作可回答证据使用。

所以当前实现故意做得更强约束：一旦拒答，返回结构就明确进入“拒答模式”。

### 它如何衔接下一步

主链路打通后，最后要做的就是用测试把“会拒答”“不会误伤”“响应结构正确”这三件事锁住。

## 第 7 步：用单元测试锁住策略行为和响应结构

### 目标

保证后续调整阈值、重构 handler 或改日志字段时，不会悄悄破坏拒答能力。

### 文件

`backend/internal/milvus/retrieval/evidence_gate_test.go`

`backend/api/handler/kb/handler_refusal_test.go`

### 完整代码

文件：`backend/internal/milvus/retrieval/evidence_gate_test.go`

```go
package retrieval

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestEvaluateEvidenceGateDisabled(t *testing.T) {
	outcome := EvaluateEvidenceGate("what is go", nil, SearchMetrics{}, EvidenceGateConfig{})
	if outcome.Result != EvidenceGateResultDisabled {
		t.Fatalf("result = %q, want %q", outcome.Result, EvidenceGateResultDisabled)
	}
}

func TestEvaluateEvidenceGateNoRetrievalHit(t *testing.T) {
	outcome := EvaluateEvidenceGate("what is go", nil, SearchMetrics{}, EvidenceGateConfig{
		Enabled:             true,
		MinRerankScore:      0.55,
		MinEvidenceDensity:  0.2,
		MinCitationCoverage: 0.5,
	})
	if outcome.Result != EvidenceGateResultRefused {
		t.Fatalf("result = %q, want %q", outcome.Result, EvidenceGateResultRefused)
	}
	if outcome.RefusalReason != RefusalReasonNoRetrievalHit {
		t.Fatalf("refusal_reason = %q, want %q", outcome.RefusalReason, RefusalReasonNoRetrievalHit)
	}
}

func TestEvaluateEvidenceGateLowConfidence(t *testing.T) {
	docs := []*schema.Document{
		{
			ID:      "chunk-1",
			Content: "The Go scheduler runs goroutines, but this snippet is too generic to explain how scheduling actually works.",
			MetaData: map[string]interface{}{
				"document_id":  1,
				"chunk_id":     "chunk-1",
				"rerank_score": 0.32,
			},
		},
	}

	outcome := EvaluateEvidenceGate("how does go scheduler work", docs, SearchMetrics{
		EvidenceDensity: 0.1,
	}, EvidenceGateConfig{
		Enabled:             true,
		MinRerankScore:      0.55,
		MinEvidenceDensity:  0.2,
		MinCitationCoverage: 0.5,
	})
	if outcome.Result != EvidenceGateResultRefused {
		t.Fatalf("result = %q, want %q", outcome.Result, EvidenceGateResultRefused)
	}
	if outcome.RefusalReason != RefusalReasonLowRerankConfidence {
		t.Fatalf("refusal_reason = %q, want %q", outcome.RefusalReason, RefusalReasonLowRerankConfidence)
	}
}

func TestEvaluateEvidenceGateCitationCoverage(t *testing.T) {
	docs := []*schema.Document{
		{
			Content: "Go uses goroutines and a work-stealing scheduler.",
			MetaData: map[string]interface{}{
				"rerank_score": 0.91,
			},
		},
	}

	outcome := EvaluateEvidenceGate("go scheduler", docs, SearchMetrics{
		EvidenceDensity: 0.9,
	}, EvidenceGateConfig{
		Enabled:             true,
		MinRerankScore:      0.55,
		MinEvidenceDensity:  0.2,
		MinCitationCoverage: 0.5,
	})
	if outcome.Result != EvidenceGateResultRefused {
		t.Fatalf("result = %q, want %q", outcome.Result, EvidenceGateResultRefused)
	}
	if outcome.RefusalReason != RefusalReasonInsufficientCitationCover {
		t.Fatalf("refusal_reason = %q, want %q", outcome.RefusalReason, RefusalReasonInsufficientCitationCover)
	}
}

func TestEvaluateEvidenceGatePass(t *testing.T) {
	docs := []*schema.Document{
		{
			ID:      "chunk-1",
			Content: "The Go scheduler multiplexes goroutines onto system threads.",
			MetaData: map[string]interface{}{
				"document_id":  42,
				"chunk_id":     "chunk-1",
				"rerank_score": 0.92,
			},
		},
	}

	outcome := EvaluateEvidenceGate("go scheduler goroutines", docs, SearchMetrics{
		EvidenceDensity: 0.8,
	}, EvidenceGateConfig{
		Enabled:             true,
		MinRerankScore:      0.55,
		MinEvidenceDensity:  0.2,
		MinCitationCoverage: 0.5,
	})
	if outcome.Result != EvidenceGateResultPass {
		t.Fatalf("result = %q, want %q", outcome.Result, EvidenceGateResultPass)
	}
	if outcome.RefusalReason != "" {
		t.Fatalf("refusal_reason = %q, want empty", outcome.RefusalReason)
	}
}
```

文件：`backend/api/handler/kb/handler_refusal_test.go`

```go
package kb

import (
	"encoding/json"
	"testing"

	"interview-agents/internal/milvus/retrieval"
)

func TestBuildStandardRefusalPayload(t *testing.T) {
	payload := buildStandardRefusalPayload(retrieval.EvidenceGateOutcome{
		Result:               retrieval.EvidenceGateResultRefused,
		RefusalReason:        retrieval.RefusalReasonOutOfKBScope,
		CitationSupportScore: 0.25,
	})
	if payload == nil {
		t.Fatal("expected refusal payload")
	}
	if payload.Reason != retrieval.RefusalReasonOutOfKBScope {
		t.Fatalf("reason = %q, want %q", payload.Reason, retrieval.RefusalReasonOutOfKBScope)
	}
	if payload.CitationSupportScore != 0.25 {
		t.Fatalf("citation_support_score = %.2f, want 0.25", payload.CitationSupportScore)
	}
	if len(payload.Suggestions) == 0 {
		t.Fatal("expected suggestions")
	}
}

func TestRetrieveResponseWithRefusalJSON(t *testing.T) {
	resp := retrieveResponse{
		RequestID:          "req-1",
		Items:              []retrieveItem{},
		EvidenceGateResult: retrieval.EvidenceGateResultRefused,
		Refusal: &refusalPayload{
			Reason:               retrieval.RefusalReasonLowRerankConfidence,
			Message:              "evidence too weak",
			Suggestions:          []string{"narrow the question"},
			CitationSupportScore: 0.3,
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if parsed["evidence_gate_result"] != retrieval.EvidenceGateResultRefused {
		t.Fatalf("evidence_gate_result = %v, want %q", parsed["evidence_gate_result"], retrieval.EvidenceGateResultRefused)
	}
	refusal, ok := parsed["refusal"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected refusal object, got %T", parsed["refusal"])
	}
	if refusal["reason"] != retrieval.RefusalReasonLowRerankConfidence {
		t.Fatalf("reason = %v, want %q", refusal["reason"], retrieval.RefusalReasonLowRerankConfidence)
	}
}
```

### 这段代码在做什么

这些测试分别锁住了两类风险：

1. `evidence_gate_test.go` 锁住策略行为。
2. `handler_refusal_test.go` 锁住对外 JSON 协议。

前者保证算法逻辑没跑偏，后者保证前端拿到的结构没变形。

### 为什么要这样做

如果只测最终接口，很容易出现一种情况：

1. 某次重构把拒答原因码改了。
2. 接口仍然返回了一个 `refusal` 对象。
3. 但运营依赖的原因分类统计已经失真。

把“门禁行为”和“响应结构”拆开测，可以更快定位问题到底在检索层还是 handler 层。

### 它如何衔接下一步

到这里，功能本身已经闭环。最后一节我们看怎么验证、有哪些取舍、以及下一步可以怎么演进。

## 如何验证

建议至少从下面三类验证做起。

### 1. 跑单元测试

在 `backend` 目录运行：

```powershell
go test ./internal/milvus/retrieval ./api/handler/kb ./internal/config
```

成功时，你应该重点确认：

1. 空结果会触发 `No-Retrieval-Hit`。
2. 高分但无引用信息的结果会触发 `Insufficient-Citation-Coverage`。
3. 高分且有引用信息的结果能正常通过。
4. JSON 响应里会带 `evidence_gate_result` 和 `refusal`。

### 2. 看检索日志

当 `enable_retrieve_audit` 打开后，检索日志里应该能看到这些字段：

1. `evidence_gate_result`
2. `refusal_reason`
3. `citation_support_score`
4. `evidence_gate_error`
5. `empty_reason`

一个典型的成功拒答信号是：

1. `evidence_gate_result=refused`
2. `result_status=filtered_out`
3. `empty_reason=Evidence-Refusal`

### 3. 实际打一个问题做验收

你可以准备两类问题：

1. 明显超出知识库范围的问题。
2. 知识库里有材料，但材料缺少明确出处的问题。

预期结果通常是：

1. 第一类更容易触发 `Out-Of-KB-Scope` 或 `No-Retrieval-Hit`。
2. 第二类更容易触发 `Insufficient-Citation-Coverage`。
3. 响应里的 `items` 应为空，`refusal.message` 则给出标准提示。

## 取舍与后续优化

### 这一版优化了什么

这版实现最核心的价值有三个：

1. 把“证据不足时不要乱答”做成了真正的系统能力。
2. 把拒答原因做成了结构化枚举，而不是一条模糊文案。
3. 把内部门禁结果和外部响应模板清晰分层，便于长期维护。

### 这一版刻意没有解决什么

这一版也有明确边界：

1. 冲突检测还是启发式规则，不是深度语义判断。
2. 引用覆盖率只看前 3 条证据，不是完整的答案级引用校验。
3. `degraded_pass` 只是兜底，不代表门禁异常可以长期忽略。

这些都不是缺点，而是当前阶段有意控制复杂度的结果。先把“拒答机制、标准模板、日志链路、调参入口”打通，比一开始就做成重模型判定更重要。

### 下一步自然演进方向

如果后面继续做 L4/L5 演进，比较自然的方向是：

1. 把冲突检测从否定词启发式升级成更稳的语义一致性判断。
2. 把门禁结果接入监控面板，长期观察拒答率和原因分布。
3. 把当前的引用覆盖率检查进一步升级到答案级引用一致性校验。
4. 按知识库类型或场景拆分不同的门禁阈值模板。

如果你想用一句话记住这一层，可以记成：

这套实现的本质不是“多返回一个拒答字段”，而是把“什么时候必须克制不回答”做成了可配置、可审计、可复用的工程能力。
