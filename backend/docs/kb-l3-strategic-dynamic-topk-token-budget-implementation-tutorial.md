# KB L3 策略版动态 TopK 与 Token 预算联动实现教程

## 背景

这层功能先说结论：它不是把 `TopK` 从固定值改成一个“更灵活”的数字这么简单，而是把“要不要多拿一些证据”和“上下文还能塞下多少内容”放进同一个决策里。

如果只做规则版动态 `TopK`，系统已经能根据 query 长短和宽泛程度，在 `min_topk` 到 `max_topk` 之间选一个值。但这还不够，因为真实检索结果还会遇到另外三类问题：

1. 同样是 5 条文档，文档长度可能差很多，token 成本完全不同。
2. 有些 query 的前几条分数出现明显断崖，这时继续放大 `TopK` 往往只会把噪声带进来。
3. 有些 query 的候选结果分数很平、父文档来源也很多，这时固定保守的 `TopK` 又容易漏证据。

所以这一版实现做了两件事：

1. 先在规则版动态 `TopK` 的基础上，加一层“策略版”判断，看候选结果本身的分布来决定该收紧还是放宽。
2. 再把 token 预算直接接进这个判断过程里，让“能拿多少条”不再只看条数，还看上下文成本。

你可以把它理解成：规则版动态 `TopK` 只看“问题长什么样”，策略版动态 `TopK` 再多看一步“证据长什么样”。

## 这篇教程会做什么

看完之后，你应该能自己复现这条链路：

1. 在配置层定义策略版动态 `TopK` 所需的开关和参数。
2. 在 Milvus 初始化阶段把这些参数注入 `HybridRetriever`。
3. 在混合检索主链路里，先跑规则版动态 `TopK`，再在 rerank 后跑策略版动态 `TopK`。
4. 根据分数分布、父文档覆盖度、证据密度和 token 预算，算出最终 `TopK`。
5. 用 token 守卫对最终结果做兜底截断，并把决策原因写进日志和指标。
6. 用单元测试和离线评测 profile 验证这套策略是不是按预期工作。

这次会涉及这些文件：

1. `backend/internal/config/config.go`
2. `backend/internal/milvus/init.go`
3. `backend/internal/milvus/retrieval/topk_policy.go`
4. `backend/internal/milvus/retrieval/hybrid_search.go`
5. `backend/internal/milvus/retrieval/search.go`
6. `backend/internal/milvus/retrieval/topk_policy_test.go`
7. `backend/internal/milvus/evaluation/types.go`
8. `backend/internal/milvus/evaluation/profiles.go`
9. `backend/cmd/retrieval-eval/main.go`

## 需要先理解的术语

### 什么是 TopK

`TopK` 的意思是“最多保留前 K 条结果”。

例如：

1. `TopK = 3`，表示最多返回前 3 条。
2. `TopK = 6`，表示最多返回前 6 条。

它不是“必须凑满 K 条”，而是“最多留 K 条最相关结果”。

### 什么是候选 TopK

候选 `TopK`（`candidate_topk`）是“先捞上来多少候选文档”。

比如最终可能只保留 4 条结果，但为了让 rerank 和后续策略有更多信息可看，系统会先取 10 条候选。前者是“最终保留多少”，后者是“前面先看看多少”。

### 什么是规则版动态 TopK

规则版动态 `TopK`（`DecideDynamicTopK`）是第一层决策。它只看 query 自身特征，比如：

1. query 很宽泛，就倾向取更大的 `TopK`。
2. query 很短很准，就倾向取更小的 `TopK`。
3. query 介于两者之间，就取中间值。

这层解决的是“不同 query 不该一刀切”。

### 什么是策略版动态 TopK

策略版动态 `TopK`（`DecideStrategicTopK`）是在规则版之上再做一层判断。它不只看 query，还看 rerank 之后的文档分布，比如：

1. 分数是不是前高后低，像断崖一样。
2. 多条结果是不是都来自同一个父文档。
3. 高分证据是不是足够密集。
4. 这些候选如果都塞进上下文，大概要花多少 token。

这层解决的是“同样的 query，证据质量不同，保留条数也应该不同”。

### 什么是 token 预算

token 预算（`token_budget`）就是“这一轮检索结果最多允许占用多少上下文成本”。

这里用一个近似估算方法：把 chunk 内容按 “4 个字符约等于 1 个 token” 粗估。这个估算不追求和模型计数器完全一致，重点是足够稳定，能做检索层的前置限流。

### 什么是预算联动

预算联动不是“最后超了再砍”，而是“在算 `TopK` 的时候就把预算考虑进去”。

也就是说：

1. 策略版 `TopK` 会先根据预算比例，估算在当前文档集合下最多能放多少条。
2. 即使前面决策已经得出一个更大的 `TopK`，预算仍然可以把它压下来。
3. 最后 `ApplyTokenBudgetGuard` 还会再做一次兜底，防止上下文成本失控。

这就是“策略版动态 `TopK` 与 token 预算联动”的核心含义。

## 整体流程

先看全局，再看代码会更容易：

1. 配置层在 `config.go` 里定义 `EnableStrategicTopK`、`StrategicTopKMinK`、`StrategicTopKMaxK`、`StrategicTopKBudgetRatio`。
2. `InitMilvusManager` 启动时，把这些参数装进 `retrieval.DynamicTopKConfig`，注入 `HybridRetriever`。
3. `HybridRetriever.SearchWithRequestAndMetrics` 在真正检索前，先调用一次 `DecideDynamicTopK`，得到规则版 `TopK`。
4. dense 和 sparse 两路召回完成后，系统会做 fusion、dedupe 和 rerank。
5. rerank 之后，系统拿着已经排序好的文档列表，调用 `DecideStrategicTopK`，根据分数分布、父文档多样性、证据密度和预算，重新算最终 `TopK`。
6. 之后进入 parent-child 补全文本，再执行 `ApplyTokenBudgetGuard`，用预算做最后兜底截断。
7. 最终结果和整个决策过程会写进 `SearchMetrics` 和日志，包括：
   `topk_policy_version`、`score_distribution`、`rerank_gap`、`evidence_density`、`topk_decision_reason`、`token_budget_remaining`、`context_tokens`。

这里最容易让初学者困惑的地方有三个：

1. 为什么要先算一次规则版，再算一次策略版。
2. 为什么预算既在策略决策里参与一次，又在最后守卫里再参与一次。
3. 为什么策略版要放在 rerank 之后，而不是召回之前。

答案其实都指向同一个设计目标：前面先做便宜、粗粒度的判断，后面再用更贵但更准确的信息做精修，而且最后还要有兜底。

## 分步实现

## 第 1 步：把策略版动态 TopK 参数接进配置层

### 目标

先把“这个功能能不能开”“开了之后边界是多少”“预算要按多少比例参与决策”定义成正式配置，而不是写死在代码里。

### 文件

`backend/internal/config/config.go`

### 完整代码

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

if c.RAG.Phase3.StrategicTopKMinK <= 0 {
	c.RAG.Phase3.StrategicTopKMinK = 2
}
if c.RAG.Phase3.StrategicTopKMaxK <= 0 {
	c.RAG.Phase3.StrategicTopKMaxK = 8
}
if c.RAG.Phase3.StrategicTopKBudgetRatio <= 0 {
	c.RAG.Phase3.StrategicTopKBudgetRatio = 0.6
}

if value, ok, err := readEnvBool("RAG_ENABLE_STRATEGIC_TOPK"); err != nil {
	return err
} else if ok {
	c.RAG.FeatureFlags.EnableStrategicTopK = value
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
```

### 这段代码在做什么

这一步做了四件事：

1. 加 feature flag，允许我们独立开关策略版 `TopK`。
2. 加 Phase 3 参数，定义策略版 `TopK` 的上下界和预算比例。
3. 在配置校验里保证参数合法，避免线上带着错误边界启动。
4. 给默认值和环境变量覆盖通路，这样开发、评测、灰度都能单独调。

### 为什么要这样写

更简单的做法是把这些数值直接写在 `topk_policy.go` 里，但那样有三个问题：

1. 评测时没法快速切 profile。
2. 线上灰度时没法通过环境变量单独调。
3. 出问题时日志里只能看到结果，看不到当时策略的真实配置。

所以这里的核心不是“多几行配置”，而是让这个策略具备可调、可控、可回滚的工程属性。

### 它如何衔接下一步

配置定义好之后，下一步就是把这些参数真正注入检索器，否则它们还只是静态配置。

## 第 2 步：在初始化阶段把策略参数注入 Hybrid Retriever

### 目标

让 `HybridRetriever` 拿到完整的 `DynamicTopKConfig`，这样策略版 `TopK` 才能在检索主链路里生效。

### 文件

`backend/internal/milvus/init.go`

### 完整代码

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

这一步把三层能力一起装进了 `HybridRetriever`：

1. 基础混合检索参数：`CandidateTopK`、dense/sparse 权重。
2. 动态 `TopK` 参数：规则版和策略版都在 `DynamicTopKConfig` 里。
3. 上游依赖：reranker、query rewriter、parent-child 补全器。

其中最重要的是这一点：策略版 `TopK` 不是单独的新模块，而是复用原来的 `DynamicTopKConfig`，只是新增了更高一层的决策字段。

### 为什么要这样写

更简单的写法是给 `HybridRetriever` 再塞一份独立的 `StrategicTopKConfig`。但那样会把“规则版动态 `TopK`”和“策略版动态 `TopK`”切成两套不相干的配置，后面判断边界时就容易不一致。

当前实现把它们放在一个配置对象里，有两个好处：

1. 规则版和策略版天然共享同一套 token 预算和最小回答条数语义。
2. 检索主链路在调用时只需要传一个配置对象，逻辑更集中。

### 它如何衔接下一步

配置注入完成后，下一步就是真正定义“规则版”和“策略版”这两层决策函数。

## 第 3 步：定义规则版与策略版动态 TopK 的核心策略

### 目标

先把“怎么算 `TopK`”本身写清楚，并且让这个决策能解释自己为什么得出这个值。

### 文件

`backend/internal/milvus/retrieval/topk_policy.go`

### 完整代码

```go
const (
	TruncateReasonNone        = ""
	TruncateReasonFinalTopK   = "final_topk"
	TruncateReasonTokenBudget = "token_budget"

	TopKPolicyVersionRule      = "phase2-rule-v1"
	TopKPolicyVersionStrategic = "phase3-strategic-v1"
)

type DynamicTopKConfig struct {
	Enabled              bool
	MinTopK              int
	MaxTopK              int
	TokenBudget          int
	MinAnswerChunks      int
	StrategicEnabled     bool
	StrategicMinTopK     int
	StrategicMaxTopK     int
	StrategicBudgetRatio float64
}

type TopKDecision struct {
	CandidateTopK          int
	RequestedTopK          int
	FinalTopK              int
	TokenBudget            int
	TokenBudgetRemaining   int
	EstimatedContextTokens int
	TruncateReason         string
	PolicyVersion          string
	ScoreDistribution      string
	RerankGap              float64
	EvidenceDensity        float64
	DecisionReason         string
}

func DecideDynamicTopK(query string, candidateTopK int, requestedTopK int, cfg DynamicTopKConfig) TopKDecision {
	minTopK, maxTopK := resolveTopKBounds(cfg.MinTopK, cfg.MaxTopK, candidateTopK)

	finalTopK := requestedTopK
	if finalTopK <= 0 {
		finalTopK = maxTopK
	}
	if !cfg.Enabled {
		finalTopK = clampInt(finalTopK, minTopK, maxTopK)
		return TopKDecision{
			CandidateTopK:  candidateTopK,
			RequestedTopK:  requestedTopK,
			FinalTopK:      finalTopK,
			TokenBudget:    cfg.TokenBudget,
			PolicyVersion:  TopKPolicyVersionRule,
			DecisionReason: "dynamic_topk_disabled",
		}
	}

	queryTrimmed := strings.TrimSpace(query)
	runeCount := utf8.RuneCountInString(queryTrimmed)
	termCount := len(strings.Fields(queryTrimmed))

	ruleTopK := minTopK
	reasons := make([]string, 0, 2)
	switch {
	case isBroadQuery(queryTrimmed):
		ruleTopK = maxTopK
		reasons = append(reasons, "broad_query")
	case runeCount >= 48 || termCount >= 8:
		ruleTopK = maxTopK
		reasons = append(reasons, "long_query")
	case runeCount >= 24 || termCount >= 5:
		ruleTopK = minTopK + (maxTopK-minTopK)/2 + 1
		reasons = append(reasons, "medium_query")
	case isShortPreciseQuery(queryTrimmed):
		ruleTopK = minTopK
		reasons = append(reasons, "short_precise_query")
	default:
		ruleTopK = minTopK + (maxTopK-minTopK)/2
		reasons = append(reasons, "default_mid_range")
	}

	finalTopK = clampInt(ruleTopK, minTopK, maxTopK)
	if requestedTopK > 0 && requestedTopK < finalTopK {
		finalTopK = clampInt(requestedTopK, minTopK, maxTopK)
		reasons = append(reasons, "requested_cap")
	}

	return TopKDecision{
		CandidateTopK:  candidateTopK,
		RequestedTopK:  requestedTopK,
		FinalTopK:      finalTopK,
		TokenBudget:    cfg.TokenBudget,
		PolicyVersion:  TopKPolicyVersionRule,
		DecisionReason: strings.Join(reasons, "+"),
	}
}

func DecideStrategicTopK(query string, candidateTopK int, requestedTopK int, docs []*schema.Document, cfg DynamicTopKConfig) (decision TopKDecision) {
	base := DecideDynamicTopK(query, candidateTopK, requestedTopK, cfg)
	if !cfg.StrategicEnabled || len(docs) == 0 {
		return base
	}
	decision = base

	defer func() {
		if recovered := recover(); recovered != nil {
			decision = base
			decision.PolicyVersion = TopKPolicyVersionRule
			decision.DecisionReason = strings.Trim(strings.Join([]string{
				strings.TrimSpace(base.DecisionReason),
				fmt.Sprintf("strategic_fallback:%v", recovered),
			}, "+"), "+")
		}
	}()

	minTopK, maxTopK := resolveTopKBounds(cfg.StrategicMinTopK, cfg.StrategicMaxTopK, candidateTopK)
	if maxTopK < minTopK {
		maxTopK = minTopK
	}
	signals := analyzeStrategicTopKSignals(docs)
	reasons := []string{"strategic_enabled"}
	finalTopK := clampInt(base.FinalTopK, minTopK, maxTopK)

	switch signals.Distribution {
	case "cliff":
		delta := 1
		if signals.RerankGap >= 0.18 {
			delta = 2
		}
		finalTopK -= delta
		reasons = append(reasons, "score_cliff")
	case "flat":
		finalTopK += 2
		reasons = append(reasons, "flat_distribution")
	default:
		reasons = append(reasons, "balanced_distribution")
	}

	if signals.ParentDiversity >= minInt(len(docs), 3) && signals.DominantParentShare < 0.7 {
		finalTopK++
		reasons = append(reasons, "diverse_parent_coverage")
	} else if signals.DominantParentShare >= 0.75 {
		finalTopK--
		reasons = append(reasons, "single_parent_concentration")
	}

	if signals.EvidenceDensity < 0.34 {
		finalTopK = minInt(finalTopK, base.FinalTopK)
		reasons = append(reasons, "low_evidence_density_no_expand")
	} else if signals.EvidenceDensity >= 0.7 && signals.RerankGap < 0.08 {
		finalTopK++
		reasons = append(reasons, "dense_evidence")
	}

	effectiveBudget := resolveStrategicTokenBudget(cfg.TokenBudget, cfg.StrategicBudgetRatio)
	estimatedTokens := estimateTopKTokens(docs, finalTopK)
	if effectiveBudget > 0 {
		budgetTopK, budgetTokens := estimateBudgetCappedTopK(docs, effectiveBudget, cfg.MinAnswerChunks, minTopK, maxTopK)
		if budgetTopK < finalTopK {
			finalTopK = budgetTopK
			estimatedTokens = budgetTokens
			reasons = append(reasons, "token_budget_cap")
		}
	}

	finalTopK = clampInt(finalTopK, minTopK, maxTopK)
	if requestedTopK > 0 && requestedTopK < finalTopK {
		finalTopK = clampInt(requestedTopK, minTopK, maxTopK)
		reasons = append(reasons, "requested_cap")
	}
	estimatedTokens = estimateTopKTokens(docs, finalTopK)

	tokenBudgetRemaining := 0
	if effectiveBudget > 0 && estimatedTokens < effectiveBudget {
		tokenBudgetRemaining = effectiveBudget - estimatedTokens
	}

	decision = TopKDecision{
		CandidateTopK:          candidateTopK,
		RequestedTopK:          requestedTopK,
		FinalTopK:              finalTopK,
		TokenBudget:            effectiveBudget,
		TokenBudgetRemaining:   tokenBudgetRemaining,
		EstimatedContextTokens: estimatedTokens,
		PolicyVersion:          TopKPolicyVersionStrategic,
		ScoreDistribution:      signals.Summary,
		RerankGap:              signals.RerankGap,
		EvidenceDensity:        signals.EvidenceDensity,
		DecisionReason:         strings.Join(reasons, "+"),
	}
	return decision
}
```

### 这段代码在做什么

这一段就是整个功能的“大脑”。

你可以把它分成两层理解：

1. `DecideDynamicTopK` 是第一层，只看 query。
2. `DecideStrategicTopK` 是第二层，看 rerank 之后的证据分布和预算。

策略版的调节逻辑主要有四组：

1. 分数断崖就收紧。
2. 分数平、证据分散就放宽。
3. 单一父文档过度集中就收紧，多父文档覆盖好就放宽。
4. 预算装不下时，再把 `TopK` 压回安全范围。

另外它还会把原因拼成 `DecisionReason`，比如：

`strategic_enabled+flat_distribution+diverse_parent_coverage+token_budget_cap`

这在排查线上行为时非常有用，因为我们不只知道“最后是 4 条”，还知道“为什么是 4 条”。

### 为什么要这样写

如果只根据 query 长度来调 `TopK`，会漏掉一个关键信息：同样一句 query，在不同知识库、不同召回结果下，最佳 `TopK` 其实可能不同。

策略版放在 rerank 之后，恰好能利用已经更可信的排序结果做判断：

1. 这时分数更稳定，更适合看断崖和平坦分布。
2. 这时文档顺序更接近最终上下文顺序，更适合估算 token 成本。
3. 这时能看出父文档集中度，判断是否“表面很多条，其实都在重复同一份证据”。

这一步解决的不是“让策略更花哨”，而是让 `TopK` 真正跟证据质量挂钩。

### 它如何衔接下一步

上面已经决定了一个“目标 `TopK`”。下一步要做的是把这些信号具体算出来，并把 token 预算落实成可执行的条数上限。

## 第 4 步：计算信号分布，并把预算变成可落地的条数限制

### 目标

把“分数断崖”“证据密度”“预算最多能装几条”这些抽象概念变成具体函数。

### 文件

`backend/internal/milvus/retrieval/topk_policy.go`

### 完整代码

```go
type strategicTopKSignals struct {
	Distribution        string
	Summary             string
	RerankGap           float64
	EvidenceDensity     float64
	ParentDiversity     int
	DominantParentShare float64
}

func analyzeStrategicTopKSignals(docs []*schema.Document) strategicTopKSignals {
	scores := make([]float64, 0, len(docs))
	strongHits := 0
	parentCounts := make(map[string]int, len(docs))
	maxParentHits := 0

	for _, doc := range docs {
		if doc == nil {
			continue
		}
		score := readStrategicScore(doc)
		scores = append(scores, score)

		parentKey := strings.TrimSpace(readMetadataString(doc, "parent_id"))
		if parentKey == "" {
			parentKey = buildDedupeKey(doc)
		}
		parentCounts[parentKey]++
		if parentCounts[parentKey] > maxParentHits {
			maxParentHits = parentCounts[parentKey]
		}
	}

	if len(scores) == 0 {
		return strategicTopKSignals{
			Distribution: "empty",
			Summary:      "empty",
		}
	}

	topScore := scores[0]
	for _, score := range scores {
		if topScore <= 0 {
			if score > 0 {
				strongHits++
			}
			continue
		}
		if score >= topScore*0.82 {
			strongHits++
		}
	}

	rerankGap := 0.0
	if len(scores) > 1 {
		rerankGap = maxFloat64(0, scores[0]-scores[1])
	}

	distribution := classifyScoreDistribution(scores, rerankGap)
	evidenceDensity := float64(strongHits) / float64(len(scores))
	dominantParentShare := 1.0
	if len(scores) > 0 {
		dominantParentShare = float64(maxParentHits) / float64(len(scores))
	}

	return strategicTopKSignals{
		Distribution:        distribution,
		Summary:             fmt.Sprintf("%s(top=%.3f,gap=%.3f,strong=%d/%d)", distribution, scores[0], rerankGap, strongHits, len(scores)),
		RerankGap:           rerankGap,
		EvidenceDensity:     evidenceDensity,
		ParentDiversity:     len(parentCounts),
		DominantParentShare: dominantParentShare,
	}
}

func classifyScoreDistribution(scores []float64, rerankGap float64) string {
	if len(scores) <= 1 {
		return "single"
	}
	normalized := normalizeScoreWindow(scores)
	spread := scores[0] - scores[len(scores)-1]
	avgAdjGap := 0.0
	for idx := 1; idx < len(normalized); idx++ {
		avgAdjGap += math.Abs(normalized[idx-1] - normalized[idx])
	}
	avgAdjGap = avgAdjGap / float64(len(normalized)-1)

	if spread <= 0.08 {
		return "flat"
	}
	if rerankGap >= 0.15 || (spread > 0.12 && normalized[0]-normalized[1] >= 0.22) {
		return "cliff"
	}
	if avgAdjGap <= 0.08 {
		return "flat"
	}
	return "balanced"
}

func estimateTopKTokens(docs []*schema.Document, limit int) int {
	if len(docs) == 0 || limit <= 0 {
		return 0
	}
	if limit > len(docs) {
		limit = len(docs)
	}
	total := 0
	for idx := 0; idx < limit; idx++ {
		total += estimateDocumentTokens(docs[idx])
	}
	return total
}

func estimateBudgetCappedTopK(docs []*schema.Document, budget int, minAnswerChunks int, minTopK int, maxTopK int) (int, int) {
	if len(docs) == 0 {
		return minTopK, 0
	}
	if budget <= 0 {
		return clampInt(len(docs), minTopK, maxTopK), estimateTopKTokens(docs, clampInt(len(docs), minTopK, maxTopK))
	}
	if minAnswerChunks <= 0 {
		minAnswerChunks = 1
	}

	totalTokens := 0
	allowedTopK := 0
	for idx, doc := range docs {
		if idx >= maxTopK {
			break
		}
		docTokens := estimateDocumentTokens(doc)
		if idx < minAnswerChunks {
			totalTokens += docTokens
			allowedTopK++
			continue
		}
		if totalTokens+docTokens > budget {
			break
		}
		totalTokens += docTokens
		allowedTopK++
	}

	if allowedTopK == 0 {
		allowedTopK = 1
		totalTokens = estimateDocumentTokens(docs[0])
	}
	allowedTopK = clampInt(allowedTopK, minTopK, maxTopK)
	return allowedTopK, totalTokens
}

func resolveStrategicTokenBudget(baseBudget int, ratio float64) int {
	if baseBudget <= 0 {
		return 0
	}
	if ratio <= 0 || ratio > 1 {
		ratio = 1
	}
	budget := int(math.Floor(float64(baseBudget) * ratio))
	if budget < 1 {
		return 1
	}
	return budget
}

func estimateDocumentTokens(doc *schema.Document) int {
	if doc == nil {
		return 0
	}
	content := strings.TrimSpace(doc.Content)
	if content == "" {
		return 0
	}
	runes := utf8.RuneCountInString(content)
	tokens := runes / 4
	if runes%4 != 0 {
		tokens++
	}
	if tokens < 1 {
		tokens = 1
	}
	return tokens
}
```

### 这段代码在做什么

这一段把“策略信号”拆成了几个可解释的指标：

1. `Distribution`：分数是 `flat`、`cliff` 还是 `balanced`。
2. `RerankGap`：第一名和第二名之间差多大。
3. `EvidenceDensity`：高质量证据有多密集。
4. `ParentDiversity` 和 `DominantParentShare`：结果是不是都来自同一个父文档。
5. `estimateBudgetCappedTopK`：在给定预算里，最多能容纳多少条。

这里有一个很重要的心智模型：

不是先精确算 token，再反推 `TopK`，而是先按当前排序顺序逐条累加，看到第几条会超预算。

因为真实系统里，最终上下文也是按这个顺序拼接的，所以这种“逐条装箱”的估算方式更贴近实际。

### 为什么要这样写

更简单的做法是：

1. 只看 `scores[0] - scores[1]`。
2. 或者只按平均 chunk 长度估算预算。

但这两种做法都太粗：

1. 只看前两名，忽略了整体分布。
2. 只看平均长度，忽略了长尾 chunk 对预算的突然挤压。

现在的实现虽然还是启发式规则，但已经足够贴近真实工程问题，而且解释成本低，便于后续调参。

### 它如何衔接下一步

有了完整的决策函数和信号函数，下一步就是把它们嵌进真实的混合检索主链路。

## 第 5 步：把策略版动态 TopK 接进混合检索主链路

### 目标

让策略版 `TopK` 真正在 dense + sparse + fusion + rerank 的完整链路里生效，而不是停留在一个孤立函数里。

### 文件

`backend/internal/milvus/retrieval/hybrid_search.go`

### 完整代码

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

	if h.reranker != nil {
		reranked, err := h.reranker.Rerank(ctx, req.OriginalQuery, merged)
		if err == nil && reranked != nil && len(reranked.Documents) > 0 {
			merged = reranked.Documents
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

	totalMS := time.Since(start).Milliseconds()
	log.Printf(
		"[RAG:L2] request_id=%s query=%q rewrite=%q final_query=%q rewrite_strategy=%q rewrite_applied=%t expr=%q candidate_topk=%d final_topk=%d token_budget=%d token_budget_remaining=%d context_tokens=%d truncate_reason=%q topk_policy_version=%q score_distribution=%q rerank_gap=%.4f evidence_density=%.4f topk_decision_reason=%q routes=%s route_hits={dense:%d,sparse:%d} final_count=%d truncated_count=%d empty_reason=%s parent_fill_strategy=%q parent_fill_count=%d parent_fill_fallback=%d parent_fill_tokens=%d duration_ms=%d dense_ms=%d sparse_ms=%d rerank_ms=%d rerank_model=%q rerank_version=%q rerank_fallback=%t rerank_reason=%q dense_error=%q sparse_error=%q",
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

	result := h.buildHybridResultMetrics(req, denseMetric, len(denseDocs), len(sparseDocs), sparseMS, topKDecision, totalMS, merged, emptyReason)
	result.Metrics.TruncatedCount = truncatedCount
	result.Metrics.RerankMs = rerankMS
	result.Metrics.RerankModel = rerankModel
	result.Metrics.RerankVersion = rerankVersion
	result.Metrics.RerankFallback = rerankFallback
	result.Metrics.RerankReason = rerankReason
	return result, nil
}
```

### 这段代码在做什么

这一段最重要的不是每一行细节，而是两个决策点的位置：

1. `DecideDynamicTopK` 放在 rewrite 之后、召回之前。
2. `DecideStrategicTopK` 放在 rerank 之后、token guard 之前。

这两个位置不是随便选的：

1. 前一个位置只有 query 信息，适合做规则版粗调。
2. 后一个位置已经有排序好的证据列表，适合做策略版精调。

最后 `ApplyTokenBudgetGuard` 再补一层硬约束，保证不管策略怎么算，最终上下文都不会完全失控。

### 为什么要这样写

如果把策略版 `TopK` 放在 rerank 前，系统看不到真正可靠的分数分布，策略判断会很虚。

如果完全不在前面做规则版 `TopK`，又会让召回阶段一直按最大规模跑，既浪费成本，也让后续模块压力变大。

现在的顺序相当于：

1. 先用便宜规则做第一轮过滤。
2. 再用高质量证据做第二轮修正。
3. 最后用预算守卫做硬兜底。

这是一个很典型的“多层渐进式决策”设计。

### 它如何衔接下一步

主链路里已经用到了 `TopKDecision`，接下来要把这些决策细节暴露到指标结构里，方便观察和排查。

## 第 6 步：把策略决策写进指标与日志

### 目标

让我们不只拿到“最后返回了几条”，还拿到“它为什么这样做”和“上下文预算还剩多少”。

### 文件

`backend/internal/milvus/retrieval/search.go`

### 完整代码

```go
type SearchMetrics struct {
	EmbeddingMs        int64
	SearchMs           int64
	PostprocessMs      int64
	HitCount           int
	TruncatedCount     int
	CandidateTopK      int
	FinalTopK          int
	TokenBudget        int
	TruncateReason     string
	Strategy           string
	ReleaseStage       string
	ReleaseReason      string
	RetrieverVersion   string
	RewriteApplied     bool
	EmptyReason        string
	RerankMs           int64
	RerankModel        string
	RerankVersion      string
	RerankFallback     bool
	RerankReason       string
	DenseHits          int
	SparseHits         int
	DenseContribution  int
	SparseContribution int
	TopKPolicyVersion  string
	ScoreDistribution  string
	RerankGap          float64
	EvidenceDensity    float64
	TopKDecisionReason string
	TokenBudgetRemain  int
	ContextTokens      int
}
```

### 这段代码在做什么

这些字段把策略版 `TopK` 的结果展开成了可观察数据：

1. `TopKPolicyVersion` 区分当前走的是规则版还是策略版。
2. `ScoreDistribution`、`RerankGap`、`EvidenceDensity` 说明证据分布长什么样。
3. `TopKDecisionReason` 说明为什么会扩容或收缩。
4. `TokenBudgetRemain` 和 `ContextTokens` 说明预算用了多少，还剩多少。

这意味着后面无论是日志平台、Grafana 还是离线分析，都可以从“结果现象”回到“策略原因”。

### 为什么要这样写

没有这些字段时，线上看到的现象通常只有一句：

“这次怎么只返回了 3 条？”

但有了这些字段，我们能继续追问：

1. 是不是 `score_cliff` 导致缩小了。
2. 是不是 `token_budget_cap` 压住了扩容。
3. 是不是 `single_parent_concentration` 说明结果都来自同一个父文档。

这让调试从猜测变成了可证据化分析。

### 它如何衔接下一步

可观测性接好之后，下一步就是通过测试验证这套策略的几个关键分支。

## 第 7 步：用单元测试验证收缩、扩容和预算联动

### 目标

证明这套策略不是“看起来合理”，而是在关键场景下真的按预期行为工作。

### 文件

`backend/internal/milvus/retrieval/topk_policy_test.go`

### 完整代码

```go
func TestDecideStrategicTopK_CliffDistributionShrinksTopK(t *testing.T) {
	cfg := DynamicTopKConfig{
		Enabled:              true,
		MinTopK:              3,
		MaxTopK:              8,
		TokenBudget:          0,
		MinAnswerChunks:      1,
		StrategicEnabled:     true,
		StrategicMinTopK:     2,
		StrategicMaxTopK:     6,
		StrategicBudgetRatio: 0.6,
	}
	docs := []*schema.Document{
		makeStrategicDoc("doc-1", "p-1", 0.93, 120),
		makeStrategicDoc("doc-2", "p-1", 0.62, 120),
		makeStrategicDoc("doc-3", "p-1", 0.41, 120),
		makeStrategicDoc("doc-4", "p-1", 0.39, 120),
	}

	ruleDecision := DecideDynamicTopK("compare go interface design tradeoff overview", 8, 0, cfg)
	decision := DecideStrategicTopK("compare go interface design tradeoff overview", 8, 0, docs, cfg)

	if decision.PolicyVersion != TopKPolicyVersionStrategic {
		t.Fatalf("expected strategic policy version, got %q", decision.PolicyVersion)
	}
	if decision.FinalTopK >= ruleDecision.FinalTopK {
		t.Fatalf("expected strategic topk to shrink below rule decision, got strategic=%d rule=%d", decision.FinalTopK, ruleDecision.FinalTopK)
	}
	if decision.FinalTopK > 4 {
		t.Fatalf("expected cliff distribution to keep topk tight, got %d", decision.FinalTopK)
	}
	if !strings.Contains(decision.DecisionReason, "score_cliff") {
		t.Fatalf("expected decision reason to include score_cliff, got %q", decision.DecisionReason)
	}
}

func TestDecideStrategicTopK_FlatDistributionExpandsWithinBudgetRatio(t *testing.T) {
	cfg := DynamicTopKConfig{
		Enabled:              true,
		MinTopK:              3,
		MaxTopK:              8,
		TokenBudget:          200,
		MinAnswerChunks:      2,
		StrategicEnabled:     true,
		StrategicMinTopK:     2,
		StrategicMaxTopK:     7,
		StrategicBudgetRatio: 0.5,
	}
	docs := []*schema.Document{
		makeStrategicDoc("doc-1", "p-1", 0.82, 80),
		makeStrategicDoc("doc-2", "p-2", 0.81, 80),
		makeStrategicDoc("doc-3", "p-3", 0.80, 80),
		makeStrategicDoc("doc-4", "p-4", 0.79, 80),
		makeStrategicDoc("doc-5", "p-5", 0.78, 80),
	}

	ruleDecision := DecideDynamicTopK("go interface", 8, 0, cfg)
	decision := DecideStrategicTopK("go interface", 8, 0, docs, cfg)

	if decision.PolicyVersion != TopKPolicyVersionStrategic {
		t.Fatalf("expected strategic policy version, got %q", decision.PolicyVersion)
	}
	if decision.TokenBudget != 100 {
		t.Fatalf("expected strategic token budget ratio to apply, got %d", decision.TokenBudget)
	}
	if decision.FinalTopK < ruleDecision.FinalTopK {
		t.Fatalf("expected flat diverse evidence to keep or expand topk, got strategic=%d rule=%d", decision.FinalTopK, ruleDecision.FinalTopK)
	}
	if !strings.Contains(decision.ScoreDistribution, "flat") {
		t.Fatalf("expected flat score distribution summary, got %q", decision.ScoreDistribution)
	}
	if decision.TokenBudgetRemaining != 0 {
		t.Fatalf("expected flat case to consume the strategic token budget, got remaining=%d", decision.TokenBudgetRemaining)
	}
	if decision.EstimatedContextTokens != 100 {
		t.Fatalf("expected estimated context tokens to match the effective strategic budget, got %d", decision.EstimatedContextTokens)
	}
}

func TestApplyTokenBudgetGuard_TracksRemainingBudget(t *testing.T) {
	cfg := DynamicTopKConfig{
		MinAnswerChunks: 1,
	}
	docs := []*schema.Document{
		makeStrategicDoc("doc-1", "p-1", 0.91, 40),
		makeStrategicDoc("doc-2", "p-2", 0.88, 40),
		makeStrategicDoc("doc-3", "p-3", 0.80, 40),
	}
	decision := TopKDecision{
		FinalTopK:   3,
		TokenBudget: 25,
	}

	budgeted, out := ApplyTokenBudgetGuard(docs, decision, cfg)
	if len(budgeted) != 2 {
		t.Fatalf("expected two docs to fit the token budget, got %d", len(budgeted))
	}
	if out.TruncateReason != TruncateReasonTokenBudget {
		t.Fatalf("expected token budget truncate reason, got %q", out.TruncateReason)
	}
	if out.TokenBudgetRemaining != 5 {
		t.Fatalf("expected token budget remaining to be 5, got %d", out.TokenBudgetRemaining)
	}
	if out.EstimatedContextTokens != 20 {
		t.Fatalf("expected estimated context tokens to be 20, got %d", out.EstimatedContextTokens)
	}
}
```

### 这段代码在做什么

这三组测试分别覆盖了最重要的三种行为：

1. `cliff` 场景会收紧 `TopK`。
2. `flat + diverse` 场景会放宽 `TopK`，但仍然受预算比例约束。
3. 最后 token guard 会正确更新剩余预算和截断原因。

### 为什么要这样写

如果只测“函数能跑通”，却不测“策略方向对不对”，那这类启发式规则很容易在后续重构里慢慢漂移。

这里的测试重点不是每个常量值本身，而是几个核心语义不能变：

1. 分数断崖应倾向收缩。
2. 平坦且多源证据应允许扩容。
3. 预算是硬约束，最后必须体现在输出指标里。

### 它如何衔接下一步

单元测试证明局部规则没问题，下一步还要把它接进离线评测 profile，观察在真实数据集上的整体收益。

## 第 8 步：把策略版动态 TopK 接进离线评测 profile

### 目标

让这个功能不只在代码里存在，还能作为一组独立策略参加回归评测。

### 文件

`backend/internal/milvus/evaluation/types.go`

### 完整代码

```go
type StrategyProfile struct {
	Name                        string  `json:"name"`
	Label                       string  `json:"label,omitempty"`
	Baseline                    bool    `json:"baseline,omitempty"`
	Candidate                   bool    `json:"candidate,omitempty"`
	Mode                        string  `json:"mode"`
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
}
```

### 文件

`backend/internal/milvus/evaluation/profiles.go`

### 完整代码

```go
func DefaultProfiles() []StrategyProfile {
	return []StrategyProfile{
		{
			Name:                 "phase2_baseline",
			Label:                "Phase 2 Baseline",
			Baseline:             true,
			Mode:                 "hybrid",
			EnableQueryRewrite:   true,
			EnableDynamicTopK:    true,
			EnableAdvancedRerank: true,
			CandidateTopK:        10,
		},
		{
			Name:                       "parent_child",
			Label:                      "Parent Child",
			Mode:                       "hybrid",
			EnableQueryRewrite:         true,
			EnableDynamicTopK:          true,
			EnableAdvancedRerank:       true,
			EnableParentChildRetrieval: true,
			CandidateTopK:              10,
		},
		{
			Name:                       "parent_child+strategic_topk",
			Label:                      "Parent Child + Strategic TopK",
			Mode:                       "hybrid",
			EnableQueryRewrite:         true,
			EnableDynamicTopK:          true,
			EnableAdvancedRerank:       true,
			EnableParentChildRetrieval: true,
			EnableStrategicTopK:        true,
			CandidateTopK:              10,
		},
		{
			Name:                       "parent_child+refusal",
			Label:                      "Parent Child + Refusal",
			Mode:                       "hybrid",
			EnableQueryRewrite:         true,
			EnableDynamicTopK:          true,
			EnableAdvancedRerank:       true,
			EnableParentChildRetrieval: true,
			EnableEvidenceRefusal:      true,
			CandidateTopK:              10,
		},
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
	}
}
```

### 文件

`backend/cmd/retrieval-eval/main.go`

### 完整代码

```go
DynamicTopK: retrieval.DynamicTopKConfig{
	Enabled:              profile.EnableDynamicTopK,
	MinTopK:              fallbackInt(profile.MinTopK, cfg.RAG.Phase2.MinTopK),
	MaxTopK:              fallbackInt(profile.MaxTopK, cfg.RAG.Phase2.MaxTopK),
	TokenBudget:          fallbackInt(profile.TokenBudget, cfg.RAG.Phase2.TokenBudget),
	MinAnswerChunks:      fallbackInt(profile.MinAnswerChunks, cfg.RAG.Phase2.MinAnswerChunks),
	StrategicEnabled:     profile.EnableStrategicTopK,
	StrategicMinTopK:     fallbackInt(profile.StrategicTopKMinK, cfg.RAG.Phase3.StrategicTopKMinK),
	StrategicMaxTopK:     fallbackInt(profile.StrategicTopKMaxK, cfg.RAG.Phase3.StrategicTopKMaxK),
	StrategicBudgetRatio: fallbackFloat(profile.StrategicTopKBudgetRatio, cfg.RAG.Phase3.StrategicTopKBudgetRatio),
},
```

### 这段代码在做什么

这一层让策略版 `TopK` 能以 profile 的形式参与实验：

1. `StrategyProfile` 给 profile 加上策略版相关字段。
2. `DefaultProfiles` 增加了 `parent_child+strategic_topk` 这个实验组。
3. `retrieval-eval` 在构造 `HybridRetrieverConfig` 时，允许 profile 覆盖默认配置。

这意味着我们可以直接比较：

1. `phase2_baseline`
2. `parent_child`
3. `parent_child+strategic_topk`

从而判断“新增的策略版 `TopK`”到底带来了什么变化。

### 为什么要这样写

如果没有独立 profile，这个功能就只能作为“跟其他改动一起混着上线”的一部分存在。那一旦效果变好或变差，我们很难知道是 parent-child、rewrite 还是 strategic `TopK` 贡献的。

把它拆成单独 profile，本质上是在做可归因实验设计。

### 它如何衔接下一步

代码和评测入口都接好了，最后一步就是验证整套链路。

## 如何验证

### 1. 跑单元测试

在 `backend` 目录运行：

```powershell
go test ./internal/milvus/retrieval -run StrategicTopK
```

如果你想把 token guard 的测试也一起带上，可以直接运行：

```powershell
go test ./internal/milvus/retrieval
```

成功信号：

1. `TestDecideStrategicTopK_CliffDistributionShrinksTopK` 通过。
2. `TestDecideStrategicTopK_FlatDistributionExpandsWithinBudgetRatio` 通过。
3. `TestApplyTokenBudgetGuard_TracksRemainingBudget` 通过。

### 2. 看启动配置是否正确注入

启动服务时确认以下配置已经生效：

1. `rag.feature_flags.enable_dynamic_topk`
2. `rag.feature_flags.enable_strategic_topk`
3. `rag.phase3.strategic_topk_min_k`
4. `rag.phase3.strategic_topk_max_k`
5. `rag.phase3.strategic_topk_budget_ratio`

如果用环境变量覆盖，可以检查类似：

```powershell
$env:RAG_ENABLE_DYNAMIC_TOPK="true"
$env:RAG_ENABLE_STRATEGIC_TOPK="true"
$env:RAG_STRATEGIC_TOPK_MIN_K="2"
$env:RAG_STRATEGIC_TOPK_MAX_K="6"
$env:RAG_STRATEGIC_TOPK_BUDGET_RATIO="0.5"
```

### 3. 看检索日志是否带上策略信号

一条成功的混合检索日志里，至少应该能看到这些字段：

1. `topk_policy_version`
2. `score_distribution`
3. `rerank_gap`
4. `evidence_density`
5. `topk_decision_reason`
6. `token_budget_remaining`
7. `context_tokens`

判断策略版是否生效，一个很直接的信号是：

`topk_policy_version="phase3-strategic-v1"`

### 4. 跑离线评测

在 `backend` 目录运行：

```powershell
go run ./cmd/retrieval-eval `
  -config ./config.yaml `
  -dataset ./scripts/evaluation/dataset.json `
  -output ./docs/retrieval-regression-report
```

然后重点比较：

1. `phase2_baseline`
2. `parent_child`
3. `parent_child+strategic_topk`

这里最值得看的不是单一指标，而是三个方向是否同时合理：

1. Recall 或 nDCG 有没有提升。
2. P95 延迟有没有失控。
3. `ContextTokens` 是否因为策略版预算联动而更稳定。

## 取舍与后续优化

### 当前版本优化了什么

这版实现主要优化的是“解释性”和“工程可控性”：

1. 规则版和策略版分层清晰。
2. token 预算既参与策略决策，也参与最后兜底。
3. 每个决策都有结构化原因，便于线上排查和离线归因。

### 当前版本还没有解决什么

这版仍然是启发式策略，不是学习型策略，所以有几个天然限制：

1. 阈值是人工经验值，不是从数据自动学习出来的。
2. token 估算是近似值，不是模型真实 tokenizer 的精确结果。
3. `flat`、`cliff`、`balanced` 的分类规则对不同数据集可能需要再调。

### 下一步最自然的演进方向

后续最自然的演进有三条：

1. 把 `score_distribution`、`evidence_density` 等信号长期打点，基于真实数据回调阈值。
2. 用真实 tokenizer 替换当前的粗估算，减少预算偏差。
3. 在评测报告里增加 `ContextTokens`、`TopKPolicyVersion`、`TopKDecisionReason` 的聚合分析，观察不同 query 类型下策略是否稳定。

这一步真正建立起来的，不只是一个“会变大变小的 `TopK`”，而是一套能根据证据质量和上下文成本联动决策的检索控制层。你可以把它理解成：系统开始不只问“还能不能多拿几条”，而是开始问“多拿这几条值不值、装不装得下、会不会把噪声也带进来”。
