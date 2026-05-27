# KB L5 引用一致性校验与 Citation 质量增强实现教程

## 背景

这次 L5 的核心结论可以先说在前面：它不是“给检索结果多加几个字段”，而是在检索链路里补上一层“引用到底能不能支撑当前问题”的校验能力。

如果没有这层能力，系统很容易出现两类问题：

1. 文档确实召回到了，但引用片段和用户问题并不真正匹配，模型仍然可能据此硬答。
2. 检索结果里虽然有 `document_id`、`chunk_id` 这样的定位信息，但我们并不知道这些 citation 对答案到底有没有支撑作用，前端和日志也就只能“展示引用”，不能“判断引用质量”。

所以 L5 实际上做了三件彼此配合的事：

1. 在检索层新增 `CitationConsistencyChecker`，对 query 和候选 chunk 的匹配度打分。
2. 把引用支撑结果写回文档元数据、SearchMetrics、审计日志和 API 响应。
3. 让 evidence gate 能利用这份结果进一步做拒答或降级判断，从“有 citation”升级到“citation 质量可判断”。

## 这篇教程会做什么

看完这篇教程之后，你应该能从头复现这样一条链路：

1. 在配置层打开 `enable_citation_consistency`，并设置阈值与版本号。
2. 在 Milvus 初始化时，把 citation checker 配置注入 `HybridRetriever`。
3. 在混合检索主链路里，对候选文档做 claim 拆分、匹配打分、文档标注和二次收敛。
4. 把引用质量结果继续送入 evidence gate，作为拒答判断的一部分。
5. 在 API、日志和数据库模型里暴露 `citation_supported`、`citation_support_score`、`unsupported_claim_count`、`citation_check_version` 等字段。
6. 用单元测试验证配置、checker 行为和对外 JSON 契约都稳定可用。

这篇教程主要覆盖这些文件：

1. `backend/internal/config/config.go`
2. `backend/config.example.yaml`
3. `backend/internal/milvus/init.go`
4. `backend/internal/milvus/retrieval/citation_consistency.go`
5. `backend/internal/milvus/retrieval/hybrid_search.go`
6. `backend/internal/milvus/retrieval/evidence_gate.go`
7. `backend/api/handler/kb/handler.go`
8. `backend/api/handler/kb/handler_refusal.go`
9. `backend/internal/model/kb_retrieve_log.go`
10. `backend/internal/milvus/retrieval/citation_consistency_test.go`
11. `backend/api/handler/kb/handler_refusal_test.go`
12. `backend/internal/config/config_rag_test.go`

如果先用一句人话概括最终控制流，可以这样理解：

1. 混合检索先找出候选文档。
2. citation checker 判断这些文档能不能真正支撑 query 里的关键 claim。
3. 结果被写入文档元数据和 metrics。
4. evidence gate 再决定这批证据是“可回答”还是“应该拒答”。
5. API 把这套内部结果翻译成稳定的对外响应与审计日志。

## 需要先理解的术语

### 什么是引用一致性校验

引用一致性校验，英文常叫 `citation consistency`，你可以先把它理解成：

“用户问的关键点，是否真的能在当前 citation 指向的 chunk 里找到支撑。”

这里它不是用大模型做复杂判题，而是先用一套可解释的启发式规则做快速判断：

1. 先从 query 里拆出几个关键 claim。
2. 再把每个 claim 和每个候选文档做词项覆盖、实体覆盖、字符片段覆盖比较。
3. 计算一个 0 到 1 的 support score。
4. 用阈值判断每个 claim 是否被支持。

### 什么是 claim

`claim` 可以先理解成“当前问题里需要被证据支撑的关键语义片段”。

例如 query 是：

`How does the Go scheduler multiplex goroutines?`

这里系统可能抽出像 `Go scheduler`、`multiplex goroutines` 这样更适合和文档做比对的片段。

这一步很重要，因为如果我们直接拿整句 query 去做全文包含判断，往往会过于严格，也不利于处理问句前缀、标点和复合问法。

### 什么是 support score

`support score` 就是 citation 对 claim 的支撑强度分数，范围是 `0 ~ 1`。

在这套实现里，它由三部分加权组成：

1. 词项覆盖率 `lexicalCoverage`
2. 实体词覆盖率 `entityCoverage`
3. 字符 n-gram 覆盖率 `charCoverage`

你可以把它理解成一把“多指标混合尺子”。它不是语义理解的最终形态，但足够稳定、便宜、可调参，也方便先落地。

### 什么是 unsupported claims

`unsupported_claims` 指的是“这次 query 里没有被当前证据充分支撑的 claim 列表”。

这比单独给一个布尔值更有用，因为：

1. API 可以直接把问题点暴露给前端或调试页。
2. evidence gate 可以用 unsupported claim 数量来决定是否拒答。
3. 后续如果要换成更强的 judge 模型，这个结构也还能继续复用。

### 初学者最容易误解的地方

这里最容易“看起来懂了，但实际没串起来”的地方有三个：

1. `citation consistency` 不是替代 `evidence gate`，而是给 `evidence gate` 提供更强的输入。
2. `citation_supported` 不是“这个文档有 citation 字段”，而是“这个文档对当前问题的支撑分数过线”。
3. L5 不只改检索算法，它还改了配置、主链路、日志模型和 API 契约，所以它本质上是一个跨层能力。

## 整体流程

先看总流程，再进代码会更顺。

1. 服务启动时，`config.go` 读取 `enable_citation_consistency`、`citation_check_threshold` 和 `citation_check_version`。
2. `InitMilvusManager` 创建 `HybridRetrieverConfig` 时，把 citation checker 配置注入到 `HybridRetriever`。
3. 混合检索执行到 rerank、parent-child fill、token budget guard 之后，进入 citation consistency 评估阶段。
4. `CitationConsistencyChecker.Check` 从 query 抽 claim，对每个 claim 在候选文档里找最佳支撑分数。
5. checker 把结果写回每个文档的 `MetaData` 和 `source` 字段里，同时返回总体 `CitationConsistencyOutcome`。
6. `HybridRetriever` 如果发现有些文档属于低支撑 citation，还会尝试只保留“通过校验”的文档再做一次 refine。
7. refine 后的结果继续送到 `EvaluateEvidenceGate`，让拒答逻辑能利用 citation support 信息。
8. `handler.go` 把这些 metrics 翻译成 `citation_check` 响应对象，并写入 `kb_retrieve_log`。

如果你只记一个顺序，就记这个：

先判断“citation 是否真的支撑问题”，再决定“系统是否应该放心回答”。

## 分步实现

## 第 1 步：先把开关、阈值和版本号配置好

### 目标

把 citation consistency 做成可开关、可校验、可灰度的系统能力，而不是把阈值硬编码在算法文件里。

### 文件

`backend/internal/config/config.go`

`backend/config.example.yaml`

`backend/internal/config/config_rag_test.go`

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

if c.RAG.FeatureFlags.EnableCitationConsistency {
	if !isNormalizedRatio(c.RAG.Phase3.CitationCheckThreshold) {
		return fmt.Errorf("rag citation consistency enabled but rag.phase3.citation_check_threshold must be within [0,1], got %.4f", c.RAG.Phase3.CitationCheckThreshold)
	}
	if strings.TrimSpace(c.RAG.Phase3.CitationCheckVersion) == "" {
		return fmt.Errorf("rag citation consistency enabled but rag.phase3.citation_check_version is empty")
	}
}

if c.RAG.Phase3.CitationCheckThreshold <= 0 {
	c.RAG.Phase3.CitationCheckThreshold = 0.7
}
if strings.TrimSpace(c.RAG.Phase3.CitationCheckVersion) == "" {
	c.RAG.Phase3.CitationCheckVersion = "phase3-citation-v1"
}

if value, ok, err := readEnvBool("RAG_ENABLE_CITATION_CONSISTENCY"); err != nil {
	return err
} else if ok {
	c.RAG.FeatureFlags.EnableCitationConsistency = value
}

if value, ok, err := readEnvFloat64("RAG_CITATION_CHECK_THRESHOLD"); err != nil {
	return err
} else if ok {
	c.RAG.Phase3.CitationCheckThreshold = value
}

if value, ok := os.LookupEnv("RAG_CITATION_CHECK_VERSION"); ok {
	c.RAG.Phase3.CitationCheckVersion = strings.TrimSpace(value)
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
```

文件：`backend/internal/config/config_rag_test.go`

```go
func TestValidateRAGPrerequisites_CitationConsistencyMissingVersion(t *testing.T) {
	cfg := baseValidRAGConfig()
	cfg.RAG.FeatureFlags.EnableCitationConsistency = true
	cfg.RAG.Phase3.CitationCheckThreshold = 0.8
	cfg.RAG.Phase3.CitationCheckVersion = ""
	if err := cfg.ValidateRAGPrerequisites(); err == nil {
		t.Fatal("expected error when citation consistency version is missing")
	}
}
```

### 这段代码在做什么

这一层做了三件基础但很重要的事：

1. 定义 feature flag：`EnableCitationConsistency`
2. 定义两个关键参数：阈值 `CitationCheckThreshold` 和版本号 `CitationCheckVersion`
3. 在配置校验和环境变量覆盖里，把这套能力变成可治理的线上配置

### 为什么要这样做

最简单的写法当然是直接在 `citation_consistency.go` 里写死：

1. 阈值就是 `0.7`
2. 版本固定叫 `phase3-citation-v1`
3. 永远启用

但这样做后面会很难受：

1. 线上发现误判时，不能快速调阈值。
2. 算法版本升级后，日志里分不清新旧行为。
3. 如果有人忘了传版本号，系统会悄悄跑一套“不可审计”的策略。

所以这里的重点不是“多写一点配置代码”，而是提前把算法能力做成可灰度、可观测、可回滚的系统能力。

### 它如何衔接下一步

有了这组配置之后，下一步就可以在 Milvus 初始化阶段，把 citation checker 真正注入 `HybridRetriever`。

## 第 2 步：把 citation checker 接进 HybridRetriever

### 目标

让混合检索主链路天然拥有 citation consistency 能力，而不是在 API 层临时补一段后处理。

### 文件

`backend/internal/milvus/init.go`

`backend/internal/milvus/retrieval/hybrid_search.go`

### 完整代码

文件：`backend/internal/milvus/init.go`

```go
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
	CitationCheck: retrieval.CitationConsistencyConfig{
		Enabled:   cfg.RAG.FeatureFlags.EnableCitationConsistency,
		Threshold: cfg.RAG.Phase3.CitationCheckThreshold,
		Version:   cfg.RAG.Phase3.CitationCheckVersion,
	},
}
```

文件：`backend/internal/milvus/retrieval/hybrid_search.go`

```go
type HybridRetriever struct {
	retriever       *RetrieverService
	sparseRetriever *SparseRetriever
	reranker        Reranker
	queryRewriter   QueryRewriter
	parentChild     *parentChildPostProcessor
	citationChecker *CitationConsistencyChecker
	config          *HybridRetrieverConfig
}

type HybridRetrieverConfig struct {
	CandidateTopK int
	DenseWeight   float64
	SparseWeight  float64
	SparseConfig  *SparseRetrieverConfig
	RerankerImpl  Reranker
	QueryRewriter QueryRewriter
	DynamicTopK   DynamicTopKConfig
	ParentChild   ParentChildConfig
	EvidenceGate  EvidenceGateConfig
	CitationCheck CitationConsistencyConfig
}

func NewHybridRetriever(retriever *RetrieverService, config *HybridRetrieverConfig) (*HybridRetriever, error) {
	if retriever == nil {
		return nil, fmt.Errorf("retriever service is nil")
	}
	if config == nil {
		config = &HybridRetrieverConfig{CandidateTopK: 10}
	}
	if config.CandidateTopK <= 0 {
		config.CandidateTopK = 10
	}
	if config.DenseWeight <= 0 {
		config.DenseWeight = 0.7
	}
	if config.SparseWeight <= 0 {
		config.SparseWeight = 0.3
	}

	sparseRetriever, err := NewSparseRetriever(retriever.client, retriever.config.Collection, config.SparseConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to init sparse retriever: %w", err)
	}

	hr := &HybridRetriever{
		retriever:       retriever,
		sparseRetriever: sparseRetriever,
		queryRewriter:   config.QueryRewriter,
		parentChild:     newParentChildPostProcessor(retriever.client, retriever.config.Collection, config.ParentChild),
		citationChecker: NewCitationConsistencyChecker(config.CitationCheck),
		config:          config,
	}
	if config.RerankerImpl != nil {
		hr.reranker = config.RerankerImpl
	} else {
		hr.reranker = NewJaccardReranker(nil)
	}
	return hr, nil
}
```

### 这段代码在做什么

这里的核心角色是 `HybridRetrieverConfig`。你可以把它理解成“混合检索主链路的总配电箱”。

L5 把 citation checker 放进这个总配电箱，有两个直接效果：

1. citation consistency 不再是一个零散后处理，而是检索主链路的一部分。
2. 它可以天然访问 rerank、parent-child、dynamic topK 之后的最终候选集，而不是看一份过早、过粗的原始结果。

### 为什么要这样做

更简单的替代方案，是在 `handler.go` 里拿到最终 `items` 之后再额外跑一遍 citation 校验。

但那样会有两个问题：

1. handler 已经失去了不少检索链路里的中间状态，很难做更精细的 refine。
2. citation checker 的结果就没法自然进入 evidence gate、搜索日志和 SearchMetrics。

所以真正重要的不是“checker 放在哪里都能跑”，而是“它应该放在最懂检索上下文的那一层去跑”。

### 它如何衔接下一步

接下来就要看 L5 的核心算法文件 `citation_consistency.go`，也就是这套校验到底怎么做。

## 第 3 步：实现 CitationConsistencyChecker

### 目标

实现一个可解释、可测试、可回写元数据的 citation checker，用来判断候选 chunk 是否真的支撑 query 里的关键 claim。

### 文件

`backend/internal/milvus/retrieval/citation_consistency.go`

### 完整代码

文件：`backend/internal/milvus/retrieval/citation_consistency.go`

```go
package retrieval

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"
)

const defaultCitationCheckThreshold = 0.7

type CitationConsistencyConfig struct {
	Enabled   bool
	Threshold float64
	Version   string
}

type CitationConsistencyOutcome struct {
	Supported         bool
	SupportScore      float64
	UnsupportedClaims []string
	Version           string
	Latency           time.Duration
	Error             string
}

type CitationConsistencyChecker struct {
	config CitationConsistencyConfig
}

func NewCitationConsistencyChecker(config CitationConsistencyConfig) *CitationConsistencyChecker {
	if config.Threshold <= 0 || config.Threshold > 1 {
		config.Threshold = defaultCitationCheckThreshold
	}
	return &CitationConsistencyChecker{config: config}
}

func (c *CitationConsistencyChecker) Check(query string, docs []*schema.Document) CitationConsistencyOutcome {
	start := time.Now()
	if c == nil || !c.config.Enabled {
		return CitationConsistencyOutcome{}
	}
	if strings.TrimSpace(c.config.Version) == "" {
		return CitationConsistencyOutcome{
			Supported: true,
			Latency:   time.Since(start),
			Error:     ErrInvalidCitationConsistencyConfig.Error(),
		}
	}

	claims := extractCitationClaims(query)
	if len(claims) == 0 {
		trimmed := strings.TrimSpace(query)
		if trimmed != "" {
			claims = []string{trimmed}
		}
	}
	if len(claims) == 0 {
		return CitationConsistencyOutcome{
			Supported: true,
			Version:   c.config.Version,
			Latency:   time.Since(start),
		}
	}

	docScores := make([]float64, len(docs))
	unsupportedClaims := make([]string, 0, len(claims))
	totalScore := 0.0
	for _, claim := range claims {
		bestScore := 0.0
		for idx, doc := range docs {
			score := scoreCitationClaimAgainstDocument(claim, doc)
			if score > docScores[idx] {
				docScores[idx] = score
			}
			if score > bestScore {
				bestScore = score
			}
		}
		totalScore += bestScore
		if bestScore < c.config.Threshold {
			unsupportedClaims = append(unsupportedClaims, claim)
		}
	}

	supportScore := 0.0
	if len(claims) > 0 {
		supportScore = totalScore / float64(len(claims))
	}
	c.annotateDocuments(docs, docScores)

	return CitationConsistencyOutcome{
		Supported:         len(unsupportedClaims) == 0,
		SupportScore:      supportScore,
		UnsupportedClaims: unsupportedClaims,
		Version:           c.config.Version,
		Latency:           time.Since(start),
	}
}

func (c *CitationConsistencyChecker) annotateDocuments(docs []*schema.Document, docScores []float64) {
	for idx, doc := range docs {
		if doc == nil {
			continue
		}
		score := 0.0
		if idx < len(docScores) {
			score = docScores[idx]
		}
		supported := score >= c.config.Threshold

		if doc.MetaData == nil {
			doc.MetaData = make(map[string]interface{})
		}
		doc.MetaData["citation_supported"] = supported
		doc.MetaData["citation_support_score"] = score
		doc.MetaData["citation_check_version"] = c.config.Version
		doc.MetaData["low_support_citation"] = !supported

		source := ensureSourceMetadata(doc)
		source["citation_supported"] = supported
		source["citation_support_score"] = score
		source["citation_check_version"] = c.config.Version
		source["low_support_citation"] = !supported
		doc.MetaData["source"] = source
	}
}

func extractCitationClaims(query string) []string {
	normalized := strings.TrimSpace(query)
	if normalized == "" {
		return nil
	}

	parts := strings.FieldsFunc(normalized, func(r rune) bool {
		return r == '\n' || r == '\r' || unicode.IsPunct(r)
	})
	claims := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		for _, candidate := range splitCompositeClaim(part) {
			candidate = normalizeClaim(candidate)
			if !isMeaningfulClaim(candidate) {
				continue
			}
			key := strings.ToLower(candidate)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			claims = append(claims, candidate)
			if len(claims) >= 4 {
				return claims
			}
		}
	}
	if len(claims) == 0 {
		return []string{normalizeClaim(normalized)}
	}
	return claims
}

func splitCompositeClaim(claim string) []string {
	candidates := []string{claim}
	connectors := []string{" and ", " vs ", " versus ", " compare "}
	for _, connector := range connectors {
		next := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			if utf8.RuneCountInString(candidate) < 12 {
				next = append(next, candidate)
				continue
			}
			parts := strings.Split(candidate, connector)
			if len(parts) == 1 {
				next = append(next, candidate)
				continue
			}
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part != "" {
					next = append(next, part)
				}
			}
		}
		candidates = next
	}
	return candidates
}

func normalizeClaim(claim string) string {
	claim = strings.TrimSpace(claim)
	claim = strings.Trim(claim, `"'[](){}<>`)
	claim = strings.Join(strings.Fields(claim), " ")
	lower := strings.ToLower(claim)
	for _, prefix := range []string{
		"how does ", "how do ", "what is ", "what are ", "why does ", "why do ",
		"explain ", "describe ", "tell me ", "show me ",
	} {
		if strings.HasPrefix(lower, prefix) {
			claim = strings.TrimSpace(claim[len(prefix):])
			break
		}
	}
	return strings.TrimSpace(claim)
}

func isMeaningfulClaim(claim string) bool {
	if claim == "" {
		return false
	}
	tokens := meaningfulTokens(claim)
	if len(tokens) >= 2 {
		return true
	}
	return utf8.RuneCountInString(claim) >= 4
}

func scoreCitationClaimAgainstDocument(claim string, doc *schema.Document) float64 {
	if strings.TrimSpace(claim) == "" || doc == nil {
		return 0
	}
	content := strings.TrimSpace(doc.Content)
	if content == "" {
		return 0
	}

	normalizedClaim := normalizeForComparison(claim)
	normalizedContent := normalizeForComparison(content)
	if normalizedClaim == "" || normalizedContent == "" {
		return 0
	}
	if strings.Contains(normalizedContent, normalizedClaim) {
		return 0.98
	}

	claimTokens := meaningfulTokens(claim)
	contentTokens := meaningfulTokens(content)
	lexicalCoverage := tokenCoverage(claimTokens, contentTokens)

	claimEntities := extractEntityTerms(claim)
	contentEntities := extractEntityTerms(content)
	entityCoverage := tokenCoverage(claimEntities, contentEntities)

	charCoverage := characterNGramCoverage(normalizedClaim, normalizedContent, 3)
	score := (lexicalCoverage * 0.5) + (entityCoverage * 0.2) + (charCoverage * 0.3)

	switch {
	case lexicalCoverage >= 1 && len(claimTokens) > 0:
		score = maxCitationFloat64(score, 0.9)
	case lexicalCoverage >= 0.8 && entityCoverage >= 0.8:
		score = maxCitationFloat64(score, 0.85)
	case entityCoverage >= 1 && len(claimEntities) > 0:
		score = maxCitationFloat64(score, 0.82)
	}

	if score > 1 {
		return 1
	}
	return score
}

func meaningfulTokens(text string) []string {
	rawTokens := splitIntoTerms(text)
	if len(rawTokens) == 0 {
		return nil
	}
	result := make([]string, 0, len(rawTokens))
	seen := make(map[string]struct{}, len(rawTokens))
	for _, token := range rawTokens {
		token = strings.ToLower(strings.TrimSpace(token))
		if token == "" || isStopToken(token) {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		result = append(result, token)
	}
	return result
}

func extractEntityTerms(text string) []string {
	terms := splitIntoTerms(text)
	result := make([]string, 0, len(terms))
	seen := make(map[string]struct{}, len(terms))
	for _, term := range terms {
		normalized := strings.ToLower(strings.TrimSpace(term))
		if normalized == "" {
			continue
		}
		if !looksLikeEntityTerm(term) {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	sort.Strings(result)
	return result
}

func splitIntoTerms(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return strings.FieldsFunc(text, func(r rune) bool {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
		switch r {
		case '_', '-', '.', '/', '#', '+':
			return false
		default:
			return true
		}
	})
}

func looksLikeEntityTerm(term string) bool {
	runeCount := utf8.RuneCountInString(term)
	if runeCount >= 2 && containsDigit(term) {
		return true
	}
	if runeCount >= 3 && strings.ContainsAny(term, "_-/#+.") {
		return true
	}
	if runeCount >= 3 && hasUppercase(term) {
		return true
	}
	if runeCount >= 4 {
		return true
	}
	return false
}

func tokenCoverage(claimTokens, contentTokens []string) float64 {
	if len(claimTokens) == 0 {
		return 0
	}
	contentSet := make(map[string]struct{}, len(contentTokens))
	for _, token := range contentTokens {
		contentSet[token] = struct{}{}
	}
	matched := 0
	for _, token := range claimTokens {
		if _, ok := contentSet[token]; ok {
			matched++
		}
	}
	return float64(matched) / float64(len(claimTokens))
}

func characterNGramCoverage(claim, content string, n int) float64 {
	claimGrams := buildCharacterNGrams(claim, n)
	if len(claimGrams) == 0 {
		return 0
	}
	contentGrams := buildCharacterNGrams(content, n)
	if len(contentGrams) == 0 {
		return 0
	}
	matched := 0
	for gram := range claimGrams {
		if _, ok := contentGrams[gram]; ok {
			matched++
		}
	}
	return float64(matched) / float64(len(claimGrams))
}

func buildCharacterNGrams(text string, n int) map[string]struct{} {
	if n <= 0 {
		return nil
	}
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}
	if len(runes) < n {
		return map[string]struct{}{string(runes): {}}
	}
	grams := make(map[string]struct{}, len(runes)-n+1)
	for i := 0; i <= len(runes)-n; i++ {
		grams[string(runes[i:i+n])] = struct{}{}
	}
	return grams
}

func normalizeForComparison(text string) string {
	var builder strings.Builder
	builder.Grow(len(text))
	for _, r := range strings.ToLower(text) {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			continue
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

func containsDigit(text string) bool {
	for _, r := range text {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func hasUppercase(text string) bool {
	for _, r := range text {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

func isStopToken(token string) bool {
	switch token {
	case "", "the", "a", "an", "is", "are", "was", "were", "what", "how", "why", "when", "where", "which", "who",
		"to", "of", "for", "in", "on", "at", "by", "with", "about", "and", "or", "vs", "versus":
		return true
	default:
		return false
	}
}

func maxCitationFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

var ErrInvalidCitationConsistencyConfig = fmt.Errorf("invalid citation consistency config")
```

### 这段代码在做什么

先说结论，这个文件的职责不是“神奇地判断对错”，而是把 citation support 这件事拆成几个可解释步骤：

1. `Check` 是总入口。
2. `extractCitationClaims` 负责把 query 变成最多 4 个可比较的 claim。
3. `scoreCitationClaimAgainstDocument` 负责给“单个 claim 对单个文档”的匹配打分。
4. `annotateDocuments` 负责把分数和布尔结果写回文档元数据。

这里最关键的设计点有两个。

第一，系统不是拿“整句 query”去硬比对整段文档，而是先拆 claim。

这样做的好处是：

1. 能去掉 `how does`、`what is` 这类问句前缀噪声。
2. 能把复合问法拆开，比如 `A vs B`、`compare A and B`。
3. 能更准确地找出到底是哪一部分没被支撑。

第二，系统不是只返回一个总体布尔值，而是同时保留：

1. 总体 `Supported`
2. 平均 `SupportScore`
3. `UnsupportedClaims`
4. 每个文档自己的 `citation_supported` / `citation_support_score`

这意味着它既能支持“最终是否拒答”的粗粒度判断，也能支持前端调试和日志分析的细粒度观察。

### 为什么要这样做

更简单的方案有两种，但都有明显问题。

第一种简单方案，是只做字符串包含。

问题是：

1. 问句里有很多噪声词。
2. 文档写法稍有变化就会匹配不到。
3. 无法给出连续分数，只能二元判断。

第二种简单方案，是直接接一个大模型做 citation judge。

这条路后面当然可以走，但在 L5 这个阶段它有几个现实成本：

1. 每次检索都多一次模型调用，时延和成本都会上来。
2. judge 自身也需要 prompt、版本管理和回归评测。
3. 排查线上问题时，可解释性反而更差。

所以当前版本的思路很务实：先用启发式方案把引用质量能力落地，得到稳定结构、指标和审计链路，再决定是否要升级更强的 judge。

### 它如何衔接下一步

有了 checker 之后，下一步不是单独调用它，而是把它塞进混合检索主链路里，成为 rerank、token guard 之后的正式阶段。

## 第 4 步：把 checker 接到混合检索主链路，并做 refine

### 目标

在检索主链路里真正执行 citation consistency，并在必要时剔除低支撑 citation。

### 文件

`backend/internal/milvus/retrieval/hybrid_search.go`

### 完整代码

文件：`backend/internal/milvus/retrieval/hybrid_search.go`

```go
func (h *HybridRetriever) SearchWithRequestAndMetrics(ctx context.Context, req *HybridSearchRequest) (*SearchResult, error) {
	if req == nil {
		return nil, fmt.Errorf("hybrid search request is nil")
	}
	if strings.TrimSpace(req.Query) == "" {
		return nil, fmt.Errorf("query is empty")
	}
	if strings.TrimSpace(req.OriginalQuery) == "" {
		req.OriginalQuery = strings.TrimSpace(req.Query)
	}
	if strings.TrimSpace(req.FinalQuery) == "" {
		req.FinalQuery = req.OriginalQuery
	}
	if strings.TrimSpace(req.RequestID) == "" {
		req.RequestID = fmt.Sprintf("hyb-%d", time.Now().UnixNano())
	}
	if req.CandidateTopK <= 0 {
		req.CandidateTopK = h.config.CandidateTopK
	}
	if req.TopK <= 0 {
		req.TopK = req.CandidateTopK
	}
	if strings.TrimSpace(req.Collection) == "" {
		req.Collection = h.retriever.config.Collection
	}

	req.applyControlledRewrite(ctx, h.queryRewriter)
	topKDecision := DecideDynamicTopK(req.FinalQuery, req.CandidateTopK, req.TopK, h.config.DynamicTopK)

	start := time.Now()
	type routeResult struct {
		route    string
		docs     []*schema.Document
		metrics  SearchMetrics
		err      error
		duration time.Duration
	}

	resultCh := make(chan routeResult, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		routeStart := time.Now()
		denseResult, err := h.searchDenseWithMetrics(ctx, req)
		routeRes := routeResult{route: routeDense, err: err, duration: time.Since(routeStart)}
		if denseResult != nil {
			routeRes.docs = denseResult.Documents
			routeRes.metrics = denseResult.Metrics
		}
		resultCh <- routeRes
	}()

	go func() {
		defer wg.Done()
		routeStart := time.Now()
		docs, err := h.sparseRetriever.Search(ctx, req)
		resultCh <- routeResult{
			route:    routeSparse,
			docs:     docs,
			err:      err,
			duration: time.Since(routeStart),
			metrics: SearchMetrics{
				SparseHits: len(docs),
			},
		}
	}()

	wg.Wait()
	close(resultCh)

	var (
		denseDocs   []*schema.Document
		sparseDocs  []*schema.Document
		denseErr    error
		sparseErr   error
		denseMS     int64
		sparseMS    int64
		denseMetric SearchMetrics
	)
	for routeRes := range resultCh {
		switch routeRes.route {
		case routeDense:
			denseDocs = routeRes.docs
			denseErr = routeRes.err
			denseMS = routeRes.duration.Milliseconds()
			denseMetric = routeRes.metrics
		case routeSparse:
			sparseDocs = routeRes.docs
			sparseErr = routeRes.err
			sparseMS = routeRes.duration.Milliseconds()
		}
		h.observeRouteMetric(routeRes.route, routeRes.duration, routeRes.err, len(routeRes.docs))
	}

	if denseErr != nil && sparseErr != nil {
		totalMS := time.Since(start).Milliseconds()
		log.Printf(
			"[RAG:L1] request_id=%s query=%q rewrite=%q final_query=%q rewrite_strategy=%q rewrite_applied=%t expr=%q candidate_topk=%d final_topk=%d token_budget=%d truncate_reason=%q topk_policy_version=%q score_distribution=%q rerank_gap=%.4f evidence_density=%.4f topk_decision_reason=%q routes=%s route_hits={dense:0,sparse:0} final_count=0 empty_reason=empty-after-retrieve duration_ms=%d dense_error=%q sparse_error=%q",
			req.RequestID, req.OriginalQuery, req.RewriteQuery, req.FinalQuery, req.RewriteStrategy, req.RewriteApplied, req.Expr, topKDecision.CandidateTopK, topKDecision.FinalTopK, topKDecision.TokenBudget, topKDecision.TruncateReason, topKDecision.PolicyVersion, topKDecision.ScoreDistribution, topKDecision.RerankGap, topKDecision.EvidenceDensity, topKDecision.DecisionReason, "dense+sparse", totalMS, denseErr.Error(), sparseErr.Error(),
		)
		return nil, fmt.Errorf("hybrid retrieval failed: dense=%v sparse=%v", denseErr, sparseErr)
	}

	fused := FuseRouteCandidates(denseDocs, sparseDocs, FusionConfig{
		DenseWeight:  h.config.DenseWeight,
		SparseWeight: h.config.SparseWeight,
	})
	merged := DeduplicateFusedDocuments(fused)

	rerankModel := ""
	rerankVersion := ""
	rerankFallback := false
	rerankReason := ""
	var rerankMS int64
	if h.reranker != nil {
		reranked, err := h.reranker.Rerank(ctx, req.OriginalQuery, merged)
		if err != nil {
			log.Printf("[RAG:L5] request_id=%s rerank_failed=true rerank_error=%q", req.RequestID, err.Error())
		} else if reranked != nil {
			rerankModel = reranked.Model
			rerankVersion = reranked.Version
			rerankFallback = reranked.Fallback
			rerankReason = reranked.Reason
			rerankMS = reranked.Latency.Milliseconds()
			if len(reranked.Documents) > 0 {
				merged = reranked.Documents
			}
		}
	}
	topKDecision = DecideStrategicTopK(req.FinalQuery, req.CandidateTopK, req.TopK, merged, h.config.DynamicTopK)

	parentFillStats := ParentChildFillStats{}
	if h.parentChild != nil {
		merged, parentFillStats = h.parentChild.Fill(ctx, merged)
	}

	beforeTruncateCount := len(merged)
	merged, topKDecision = ApplyTokenBudgetGuard(merged, topKDecision, h.config.DynamicTopK)
	truncatedCount := beforeTruncateCount - len(merged)

	citationCandidateCount := len(merged)
	citationOutcome := h.evaluateCitationConsistency(req.FinalQuery, merged)
	merged, citationOutcome = h.tryRefineUnsupportedCitations(req.FinalQuery, merged, citationOutcome)
	truncatedCount += citationCandidateCount - len(merged)
	evidenceOutcome := h.evaluateEvidenceGate(req.FinalQuery, merged, topKDecision, citationOutcome)
	totalMS := time.Since(start).Milliseconds()

	log.Printf(
		"[RAG:L2] request_id=%s query=%q rewrite=%q final_query=%q rewrite_strategy=%q rewrite_applied=%t expr=%q candidate_topk=%d final_topk=%d token_budget=%d token_budget_remaining=%d context_tokens=%d truncate_reason=%q topk_policy_version=%q score_distribution=%q rerank_gap=%.4f evidence_density=%.4f topk_decision_reason=%q evidence_gate_result=%q refusal_reason=%q citation_supported=%t citation_support_score=%.4f unsupported_claim_count=%d citation_check_version=%q citation_check_latency_ms=%d citation_check_error=%q evidence_gate_error=%q routes=%s route_hits={dense:%d,sparse:%d} final_count=%d truncated_count=%d empty_reason=%s parent_fill_strategy=%q parent_fill_count=%d parent_fill_fallback=%d parent_fill_tokens=%d duration_ms=%d dense_ms=%d sparse_ms=%d rerank_ms=%d rerank_model=%q rerank_version=%q rerank_fallback=%t rerank_reason=%q dense_error=%q sparse_error=%q",
		req.RequestID,
		req.OriginalQuery,
		req.RewriteQuery,
		req.FinalQuery,
		req.RewriteStrategy,
		req.RewriteApplied,
		req.Expr,
		topKDecision.CandidateTopK,
		topKDecision.FinalTopK,
		topKDecision.TokenBudget,
		topKDecision.TokenBudgetRemaining,
		topKDecision.EstimatedContextTokens,
		topKDecision.TruncateReason,
		topKDecision.PolicyVersion,
		topKDecision.ScoreDistribution,
		topKDecision.RerankGap,
		topKDecision.EvidenceDensity,
		topKDecision.DecisionReason,
		evidenceOutcome.Result,
		evidenceOutcome.RefusalReason,
		citationOutcome.Supported,
		evidenceOutcome.CitationSupportScore,
		len(citationOutcome.UnsupportedClaims),
		citationOutcome.Version,
		citationOutcome.Latency.Milliseconds(),
		citationOutcome.Error,
		evidenceOutcome.Error,
		"dense+sparse",
		len(denseDocs),
		len(sparseDocs),
		len(merged),
		truncatedCount,
		emptyReason,
		parentFillStats.Strategy,
		parentFillStats.FilledCount,
		parentFillStats.FallbackCount,
		parentFillStats.FilledTokens,
		totalMS,
		denseMS,
		sparseMS,
		rerankMS,
		rerankModel,
		rerankVersion,
		rerankFallback,
		rerankReason,
		toLogError(denseErr),
		toLogError(sparseErr),
	)

	result := h.buildHybridResultMetrics(req, denseMetric, len(denseDocs), len(sparseDocs), sparseMS, topKDecision, totalMS, merged, emptyReason, evidenceOutcome)
	result.Metrics.CitationSupported = citationOutcome.Supported
	result.Metrics.UnsupportedClaims = append([]string(nil), citationOutcome.UnsupportedClaims...)
	result.Metrics.UnsupportedClaimCount = len(citationOutcome.UnsupportedClaims)
	result.Metrics.CitationCheckVersion = citationOutcome.Version
	result.Metrics.CitationCheckLatencyMs = citationOutcome.Latency.Milliseconds()
	result.Metrics.CitationCheckError = citationOutcome.Error
	result.Metrics.TruncatedCount = truncatedCount
	result.Metrics.RerankMs = rerankMS
	result.Metrics.RerankModel = rerankModel
	result.Metrics.RerankVersion = rerankVersion
	result.Metrics.RerankFallback = rerankFallback
	result.Metrics.RerankReason = rerankReason
	return result, nil
}

func (h *HybridRetriever) evaluateCitationConsistency(query string, docs []*schema.Document) CitationConsistencyOutcome {
	if h == nil || h.citationChecker == nil {
		return CitationConsistencyOutcome{}
	}
	return h.citationChecker.Check(query, docs)
}

func (h *HybridRetriever) tryRefineUnsupportedCitations(query string, docs []*schema.Document, current CitationConsistencyOutcome) ([]*schema.Document, CitationConsistencyOutcome) {
	if h == nil || h.citationChecker == nil || !h.config.CitationCheck.Enabled {
		return docs, current
	}
	if current.Supported || len(docs) <= 1 {
		return docs, current
	}

	supportedDocs := make([]*schema.Document, 0, len(docs))
	for _, doc := range docs {
		if doc == nil || doc.MetaData == nil {
			continue
		}
		if value, ok := doc.MetaData["citation_supported"]; ok && castBool(value) {
			supportedDocs = append(supportedDocs, doc)
		}
	}
	if len(supportedDocs) == 0 || len(supportedDocs) == len(docs) {
		return docs, current
	}

	refined := h.citationChecker.Check(query, supportedDocs)
	if refined.SupportScore > current.SupportScore || (refined.Supported && !current.Supported) {
		return supportedDocs, refined
	}
	return docs, current
}
```

### 这段代码在做什么

这一步真正把 L5 放进了主链路，而且顺序很讲究：

1. 先 dense/sparse 并发召回。
2. 再 fuse + dedupe。
3. 再 rerank。
4. 再 parent-child fill。
5. 再 token budget guard。
6. 然后才做 citation consistency。
7. 最后把 citation 结果送到 evidence gate。

这个顺序非常重要。因为 citation checker 要看的，应该是“最接近最终答案候选集”的文档，而不是早期粗召回结果。

另外，`tryRefineUnsupportedCitations` 是 L5 很有价值的一步。你可以把它理解成：

“如果混进来几条低支撑 citation，那就试试只保留通过校验的文档，看看总体 support score 能不能变好。”

这一步不是必须的，但它让系统不仅会“报告问题”，还会“主动收敛问题”。

### 为什么要这样做

如果我们省掉 refine，系统当然也能工作，但会有一个常见坏现象：

1. 候选集里混了几条低质量 chunk。
2. 这些 chunk 拉低了整体 citation support。
3. evidence gate 更容易判成拒答。

也就是说，明明“有足够好的证据”，只是被坏证据拖累了。

`tryRefineUnsupportedCitations` 解决的就是这个很具体的工程问题。它不是在“加复杂度”，而是在减少误杀。

### 它如何衔接下一步

有了 citation outcome 之后，下一步就是让 evidence gate 真正用上这些结果，形成“引用质量影响最终能否回答”的闭环。

## 第 5 步：让 evidence gate 利用 citation quality 做最终判断

### 目标

把 citation consistency 从“一个附加指标”升级为 evidence gate 的正式输入。

### 文件

`backend/internal/milvus/retrieval/evidence_gate.go`

`backend/internal/milvus/retrieval/hybrid_search.go`

### 完整代码

文件：`backend/internal/milvus/retrieval/evidence_gate.go`

```go
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

	citationSupportScore, citationSupported, citationChecked, unsupportedClaimCount := resolveCitationSupport(metrics, docs)
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

	if citationChecked && (!citationSupported || unsupportedClaimCount > 0) {
		outcome.Result = EvidenceGateResultRefused
		outcome.RefusalReason = RefusalReasonInsufficientCitationCover
		return outcome
	}

	if citationSupportScore < thresholds.MinCitationCoverage {
		outcome.Result = EvidenceGateResultRefused
		outcome.RefusalReason = RefusalReasonInsufficientCitationCover
		return outcome
	}

	return outcome
}

func resolveCitationSupport(metrics SearchMetrics, docs []*schema.Document) (float64, bool, bool, int) {
	if metrics.CitationCheckVersion != "" {
		return metrics.CitationSupportScore, metrics.CitationSupported, true, metrics.UnsupportedClaimCount
	}
	score := metrics.CitationSupportScore
	if score <= 0 {
		score = computeCitationSupportScore(docs)
	}
	return score, score > 0, false, 0
}
```

文件：`backend/internal/milvus/retrieval/hybrid_search.go`

```go
func (h *HybridRetriever) evaluateEvidenceGate(query string, docs []*schema.Document, topKDecision TopKDecision, citationOutcome CitationConsistencyOutcome) EvidenceGateOutcome {
	if h == nil {
		return EvidenceGateOutcome{Result: EvidenceGateResultDisabled}
	}
	return EvaluateEvidenceGate(query, docs, SearchMetrics{
		EvidenceDensity:        topKDecision.EvidenceDensity,
		CitationSupportScore:   citationOutcome.SupportScore,
		CitationSupported:      citationOutcome.Supported,
		UnsupportedClaims:      append([]string(nil), citationOutcome.UnsupportedClaims...),
		UnsupportedClaimCount:  len(citationOutcome.UnsupportedClaims),
		CitationCheckVersion:   citationOutcome.Version,
		CitationCheckLatencyMs: citationOutcome.Latency.Milliseconds(),
		CitationCheckError:     citationOutcome.Error,
	}, h.config.EvidenceGate)
}
```

### 这段代码在做什么

这里最重要的设计点是：evidence gate 不再只看“召回了没有”“分数高不高”，还会看“citation 是否真的支撑 query”。

具体来说，它分两层判断：

1. 如果 citation checker 已经运行过，就优先使用 `CitationCheckVersion`、`CitationSupported`、`UnsupportedClaimCount` 这套更强信号。
2. 如果 citation checker 没跑，就退回到 `computeCitationSupportScore` 这样的弱规则兜底。

这意味着 L5 对旧链路是兼容的，对新链路又能提供更高质量的判断。

### 为什么要这样做

如果 evidence gate 完全不关心 citation quality，那么系统仍然可能出现这种情况：

1. rerank 分很高。
2. evidence density 也不错。
3. 但引用内容其实没支撑住关键 claim。

这种回答看上去最危险，因为它“很像对的”，却缺少真正可核验的依据。

所以 L5 真正增强的不是“展示更多字段”，而是把“有证据”升级为“证据真的支撑当前问题”。

### 它如何衔接下一步

有了 citation outcome 和 evidence gate outcome，接下来就要把这些结果真正暴露给 API、数据库日志和前端。

## 第 6 步：把 citation 结果写进 API 响应和检索日志

### 目标

让这套能力不仅在检索内部有效，也能被前端、调试页、质量监控和审计日志消费。

### 文件

`backend/api/handler/kb/handler.go`

`backend/internal/model/kb_retrieve_log.go`

`backend/api/handler/kb/handler_refusal.go`

### 完整代码

文件：`backend/api/handler/kb/handler.go`

```go
type source struct {
	Route                string  `json:"route"`
	Collection           string  `json:"collection"`
	RetrieverVersion     string  `json:"retriever_version"`
	ParentID             string  `json:"parent_id"`
	ChildID              string  `json:"child_id"`
	SectionTitle         string  `json:"section_title"`
	HierarchyPath        string  `json:"hierarchy_path"`
	ParentFillStrategy   string  `json:"parent_fill_strategy"`
	ParentFillTokens     int     `json:"parent_fill_tokens"`
	CitationSupported    bool    `json:"citation_supported"`
	CitationSupportScore float64 `json:"citation_support_score"`
	CitationCheckVersion string  `json:"citation_check_version"`
	LowSupportCitation   bool    `json:"low_support_citation"`
}

type citationCheckResponse struct {
	Supported             bool     `json:"supported"`
	SupportScore          float64  `json:"support_score"`
	UnsupportedClaims     []string `json:"unsupported_claims,omitempty"`
	UnsupportedClaimCount int      `json:"unsupported_claim_count"`
	Version               string   `json:"version,omitempty"`
	LatencyMs             int64    `json:"latency_ms,omitempty"`
	Error                 string   `json:"error,omitempty"`
}

type retrieveResponse struct {
	RequestID          string                 `json:"request_id"`
	Items              []retrieveItem         `json:"items"`
	EvidenceGateResult string                 `json:"evidence_gate_result,omitempty"`
	CitationCheck      *citationCheckResponse `json:"citation_check,omitempty"`
	Refusal            *refusalPayload        `json:"refusal,omitempty"`
}

func buildCitationCheckResponse(metrics retrieval.SearchMetrics) *citationCheckResponse {
	if metrics.CitationCheckVersion == "" && metrics.CitationCheckLatencyMs == 0 && metrics.CitationCheckError == "" && metrics.UnsupportedClaimCount == 0 {
		return nil
	}
	return &citationCheckResponse{
		Supported:             metrics.CitationSupported,
		SupportScore:          metrics.CitationSupportScore,
		UnsupportedClaims:     append([]string(nil), metrics.UnsupportedClaims...),
		UnsupportedClaimCount: metrics.UnsupportedClaimCount,
		Version:               metrics.CitationCheckVersion,
		LatencyMs:             metrics.CitationCheckLatencyMs,
		Error:                 metrics.CitationCheckError,
	}
}
```

文件：`backend/api/handler/kb/handler.go`

```go
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
		Route:                route,
		Collection:           firstNonEmptyString(getStringMetadata(doc.MetaData, "collection"), collection),
		RetrieverVersion:     firstNonEmptyString(getStringMetadata(doc.MetaData, "retriever_version"), searchMetrics.RetrieverVersion),
		ParentID:             getStringMetadata(doc.MetaData, "parent_id"),
		ChildID:              firstNonEmptyString(getStringMetadata(doc.MetaData, "child_id"), firstNonEmptyString(doc.ID, getStringMetadata(doc.MetaData, "chunk_id"))),
		SectionTitle:         getStringMetadata(doc.MetaData, "section_title"),
		HierarchyPath:        getStringMetadata(doc.MetaData, "hierarchy_path"),
		ParentFillStrategy:   getStringMetadata(doc.MetaData, "parent_fill_strategy"),
		ParentFillTokens:     getIntMetadata(doc.MetaData, "parent_fill_tokens"),
		CitationSupported:    getBoolMetadata(doc.MetaData, "citation_supported"),
		CitationSupportScore: getFloat64Metadata(doc.MetaData, "citation_support_score"),
		CitationCheckVersion: getStringMetadata(doc.MetaData, "citation_check_version"),
		LowSupportCitation:   getBoolMetadata(doc.MetaData, "low_support_citation"),
	},
})

citationCheck := buildCitationCheckResponse(searchMetrics)

response.Success(ctx, c, retrieveResponse{
	RequestID:          requestID,
	Items:              items,
	EvidenceGateResult: searchMetrics.EvidenceGateResult,
	CitationCheck:      citationCheck,
	Refusal:            refusal,
})
```

文件：`backend/internal/model/kb_retrieve_log.go`

```go
type KBRetrieveLog struct {
	ID                     uint64               `json:"id" gorm:"primaryKey;autoIncrement"`
	RequestID              string               `json:"request_id" gorm:"uniqueIndex;size:64;not null"`
	UserID                 uint                 `json:"user_id" gorm:"index;not null"`
	KBIDs                  string               `json:"kb_ids" gorm:"size:500"`
	Query                  string               `json:"query" gorm:"size:2000;not null"`
	FinalQuery             string               `json:"final_query" gorm:"size:2000"`
	Expr                   string               `json:"expr" gorm:"size:2000"`
	TopK                   int                  `json:"top_k"`
	CandidateTopK          int                  `json:"candidate_topk"`
	FinalTopK              int                  `json:"final_topk"`
	TokenBudget            int                  `json:"token_budget"`
	TruncateReason         string               `json:"truncate_reason" gorm:"size:64"`
	Rewrite                string               `json:"rewrite" gorm:"size:1000"`
	RewriteStrategy        string               `json:"rewrite_strategy" gorm:"size:255"`
	RewriteApplied         bool                 `json:"rewrite_applied"`
	Strategy               string               `json:"strategy" gorm:"size:20;index"`
	ReleaseStage           string               `json:"release_stage" gorm:"size:32;index"`
	ReleaseReason          string               `json:"release_reason" gorm:"size:255"`
	Routes                 string               `json:"routes" gorm:"size:200"`
	Collection             string               `json:"collection" gorm:"size:200"`
	RetrieverVersion       string               `json:"retriever_version" gorm:"size:50"`
	EmptyReason            string               `json:"empty_reason" gorm:"size:64;index"`
	EvidenceGateResult     string               `json:"evidence_gate_result" gorm:"size:32;index"`
	RefusalReason          string               `json:"refusal_reason" gorm:"size:64;index"`
	CitationSupported      bool                 `json:"citation_supported"`
	CitationSupportScore   float64              `json:"citation_support_score"`
	UnsupportedClaimCount  int                  `json:"unsupported_claim_count"`
	CitationCheckVersion   string               `json:"citation_check_version" gorm:"size:64"`
	CitationCheckLatencyMs int64                `json:"citation_check_latency_ms"`
	EvidenceGateError      string               `json:"evidence_gate_error" gorm:"size:500"`
	CitationCheckError     string               `json:"citation_check_error" gorm:"size:500"`
	FinalCount             int                  `json:"final_count"`
	TruncatedCount         int                  `json:"truncated_count"`
	DenseHits              int                  `json:"dense_hits"`
	SparseHits             int                  `json:"sparse_hits"`
	DenseContribution      int                  `json:"dense_contribution"`
	SparseContribution     int                  `json:"sparse_contribution"`
	ResultStatus           RetrieveResultStatus `json:"result_status" gorm:"size:20;not null;default:'success';index"`
	ErrorCode              string               `json:"error_code" gorm:"size:64"`
	ErrorMsg               string               `json:"error_msg" gorm:"size:1000"`
	EmbeddingMs            int64                `json:"embedding_ms"`
	SearchMs               int64                `json:"search_ms"`
	PostprocessMs          int64                `json:"postprocess_ms"`
	RerankMs               int64                `json:"rerank_ms"`
	RerankModel            string               `json:"rerank_model" gorm:"size:128"`
	DurationMs             int64                `json:"duration_ms"`
	TimeoutMs              int64                `json:"timeout_ms"`
	CreatedAt              time.Time            `json:"created_at" gorm:"autoCreateTime:milli;index"`
}
```

文件：`backend/api/handler/kb/handler_refusal.go`

```go
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
```

### 这段代码在做什么

这一层做的事情，可以概括成一句话：

把“内部校验结果”翻译成“外部可消费契约”。

具体包括三部分：

1. 每条 item 的 `source` 带上文档级 citation 质量信息。
2. 整个响应新增顶层 `citation_check`，暴露总体结果。
3. 审计日志模型把 citation 相关字段持久化，方便后续排查和做质量看板。

### 为什么要这样做

如果只在内部 metrics 里保留 citation 结果，会出现两个很现实的问题：

1. 前端只能看到“有 citation”，看不到“citation 质量如何”。
2. 线上排查只能翻检索代码，很难直接从日志里看出问题。

把结果同时放在文档级、请求级和日志级，实际上是在建立三层可观测性：

1. 文档级看“哪条引用弱”。
2. 请求级看“这次总体支撑如何”。
3. 日志级看“线上长期趋势如何”。

### 它如何衔接下一步

最后一步就是把测试补齐，确保配置、checker 行为和对外 JSON 契约都不会被后续改动悄悄破坏。

## 第 7 步：用测试把行为和契约锁住

### 目标

保证后续调阈值、换算法、重构 handler 时，不会悄悄破坏 citation consistency 能力。

### 文件

`backend/internal/milvus/retrieval/citation_consistency_test.go`

`backend/api/handler/kb/handler_refusal_test.go`

`backend/internal/config/config_rag_test.go`

### 完整代码

文件：`backend/internal/milvus/retrieval/citation_consistency_test.go`

```go
func TestCitationConsistencyCheckerSupported(t *testing.T) {
	checker := NewCitationConsistencyChecker(CitationConsistencyConfig{
		Enabled:   true,
		Threshold: 0.7,
		Version:   "phase3-citation-v1",
	})
	docs := []*schema.Document{
		{
			ID:      "chunk-1",
			Content: "The Go scheduler multiplexes goroutines onto system threads with work-stealing.",
			MetaData: map[string]interface{}{
				"document_id": uint64(7),
				"chunk_id":    "chunk-1",
			},
		},
	}

	outcome := checker.Check("How does the Go scheduler multiplex goroutines?", docs)
	if !outcome.Supported {
		t.Fatalf("supported = false, want true; outcome=%+v", outcome)
	}
	if outcome.SupportScore < 0.7 {
		t.Fatalf("support_score = %.3f, want >= 0.7", outcome.SupportScore)
	}
	if docs[0].MetaData["citation_supported"] != true {
		t.Fatalf("doc citation_supported = %v, want true", docs[0].MetaData["citation_supported"])
	}
}

func TestCitationConsistencyCheckerUnsupportedClaim(t *testing.T) {
	checker := NewCitationConsistencyChecker(CitationConsistencyConfig{
		Enabled:   true,
		Threshold: 0.72,
		Version:   "phase3-citation-v1",
	})
	docs := []*schema.Document{
		{
			ID:      "chunk-2",
			Content: "The storage layer persists vectors and metadata for retrieval.",
			MetaData: map[string]interface{}{
				"document_id": uint64(8),
				"chunk_id":    "chunk-2",
			},
		},
	}

	outcome := checker.Check("How does the Go scheduler multiplex goroutines?", docs)
	if outcome.Supported {
		t.Fatalf("supported = true, want false; outcome=%+v", outcome)
	}
	if len(outcome.UnsupportedClaims) == 0 {
		t.Fatalf("unsupported_claims empty, want at least one")
	}
	if docs[0].MetaData["low_support_citation"] != true {
		t.Fatalf("doc low_support_citation = %v, want true", docs[0].MetaData["low_support_citation"])
	}
}

func TestCitationConsistencyCheckerMissingVersionDegrades(t *testing.T) {
	checker := NewCitationConsistencyChecker(CitationConsistencyConfig{
		Enabled:   true,
		Threshold: 0.7,
	})

	outcome := checker.Check("go scheduler", nil)
	if outcome.Error == "" {
		t.Fatal("expected error when citation checker version is missing")
	}
	if !outcome.Supported {
		t.Fatalf("supported = false, want degraded true outcome; outcome=%+v", outcome)
	}
}
```

文件：`backend/api/handler/kb/handler_refusal_test.go`

```go
func TestRetrieveResponseWithRefusalJSON(t *testing.T) {
	resp := retrieveResponse{
		RequestID:          "req-1",
		Items:              []retrieveItem{},
		EvidenceGateResult: retrieval.EvidenceGateResultRefused,
		CitationCheck: &citationCheckResponse{
			Supported:             false,
			SupportScore:          0.3,
			UnsupportedClaims:     []string{"scheduler detail"},
			UnsupportedClaimCount: 1,
			Version:               "phase3-citation-v1",
			LatencyMs:             8,
		},
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
	citationCheck, ok := parsed["citation_check"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected citation_check object, got %T", parsed["citation_check"])
	}
	if citationCheck["unsupported_claim_count"] != float64(1) {
		t.Fatalf("unsupported_claim_count = %v, want 1", citationCheck["unsupported_claim_count"])
	}
}
```

文件：`backend/internal/config/config_rag_test.go`

```go
func TestValidateRAGPrerequisites_CitationConsistencyMissingVersion(t *testing.T) {
	cfg := baseValidRAGConfig()
	cfg.RAG.FeatureFlags.EnableCitationConsistency = true
	cfg.RAG.Phase3.CitationCheckThreshold = 0.8
	cfg.RAG.Phase3.CitationCheckVersion = ""
	if err := cfg.ValidateRAGPrerequisites(); err == nil {
		t.Fatal("expected error when citation consistency version is missing")
	}
}
```

### 这段代码在做什么

这几类测试分别锁住了不同风险：

1. `citation_consistency_test.go` 锁住算法行为。
2. `config_rag_test.go` 锁住配置前置条件。
3. `handler_refusal_test.go` 锁住对外 JSON 契约。

如果只测最终接口，很难知道问题到底出在：

1. claim 抽取逻辑
2. support score 计算
3. 配置校验
4. handler 序列化

把这些层拆开测，后续维护成本会低很多。

### 为什么要这样做

L5 这种跨层能力最怕的，不是“代码一开始写不出来”，而是后续有人改了其中一层，看起来一切还能跑，但质量能力已经悄悄失真。

这些测试本质上是在保护三件事：

1. 算法规则不被无意改坏。
2. 配置不会进入不可审计状态。
3. 前端和调试页依赖的响应结构保持稳定。

### 它如何衔接下一步

到这里，L5 的实现闭环就完整了。接下来就应该进入验证、观测与演进阶段。

## 如何验证

建议至少从下面三类验证做起。

### 1. 跑单元测试

在 `backend` 目录运行：

```powershell
go test ./internal/milvus/retrieval ./api/handler/kb ./internal/config ./internal/model
```

你应该重点确认这些结果：

1. 支撑充分的 query 会得到 `citation_supported=true`。
2. 不匹配的 query 会出现 `unsupported_claims`。
3. 版本号缺失时会进入带 `error` 的降级路径，而不是静默成功。
4. API 响应里会稳定带上 `citation_check` 结构。

### 2. 看检索日志

一次真实请求之后，重点看这些字段：

1. `citation_supported`
2. `citation_support_score`
3. `unsupported_claim_count`
4. `citation_check_version`
5. `citation_check_latency_ms`
6. `citation_check_error`

一个比较典型的低质量引用信号是：

1. `citation_supported=false`
2. `unsupported_claim_count > 0`
3. `evidence_gate_result=refused`
4. `refusal_reason=Insufficient-Citation-Coverage`

### 3. 用真实 query 做烟雾验证

你可以准备两类 query：

1. 知识库里明确有对应章节、chunk 也比较聚焦的问题。
2. 关键词部分匹配，但真正答案不在该 chunk 里的问题。

预期结果通常是：

1. 第一类更容易得到较高 `citation_support_score`。
2. 第二类更容易出现 `unsupported_claims` 或 `low_support_citation=true`。
3. 如果 evidence gate 开启，第二类请求更可能被拒答。

## 取舍与后续优化

### 这一版优化了什么

L5 当前版本最核心的价值有四个：

1. 把“citation 是否真的支撑问题”从感性判断变成结构化信号。
2. 把这个信号贯穿到检索、拒答、日志和 API 全链路。
3. 在不引入额外模型调用的前提下，先建立一套可解释、可调参、可审计的基础能力。
4. 通过 refine 机制减少低质量 citation 对整体结果的拖累。

### 这一版刻意没有解决什么

这版也有明确边界：

1. 它主要是启发式匹配，不是完整语义 judge。
2. claim 拆分规则还比较轻量，复杂长问句可能还会继续演进。
3. `support_score` 是工程上可用的代理指标，不等于真正的事实性判定。

这些都不是问题被忽略了，而是刻意控制实现复杂度的结果。L5 先把“可观测、可配置、可拒答、可回放”打通，比一开始就上重模型更重要。

### 下一步自然演进方向

如果后面继续升级 L5/L6，一般会沿着这几个方向演进：

1. 用更强的 query claim 拆分器，提升复杂问句的稳定性。
2. 引入模型辅助 judge，把启发式分数和语义判断结合起来。
3. 在质量监控页上聚合 `unsupported_claim_count`、`citation_support_score` 分布和版本对比。
4. 为不同知识库类型配置不同的 citation threshold，而不是全局一刀切。

如果你想用一句话记住这次实现，可以记成：

L5 的本质不是“多返回了 citation 字段”，而是把“引用是否真的支撑当前答案”做成了检索系统里可配置、可审计、可用于拒答决策的正式能力。
