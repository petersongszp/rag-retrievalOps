# KB L4 动态 TopK（规则版）与 Token 守卫实现教程

## 背景

这一层功能先说结论：它不是单纯把 `TopK` 从固定值改成“看起来更聪明”的动态值，而是在**召回质量**和**上下文成本**之间加一层可控策略。

如果一直使用固定 `TopK`，会遇到两类典型问题：

1. 问题很宽泛时，`TopK = 5` 可能不够，证据不全，回答容易漏点。
2. 问题很短很准时，`TopK = 5` 又可能太多，把无关 chunk 也塞进上下文，白白浪费 token。

更麻烦的是，哪怕我们把 `TopK` 调成动态值，也还会遇到另一个问题：**文档条数相同，不代表 token 成本相同**。  
比如都是 5 个 chunk：

1. 一组 chunk 每条只有 80 token，总共约 400 token。
2. 另一组 chunk 每条接近 500 token，总共就可能到 2500 token。

所以 L4 做了两件事：

1. 先根据 query 规则决定一个更合适的 `final_topk`
2. 再用 token 预算守卫把最终上下文成本卡在预算内

这就是“动态 TopK + token 守卫”的完整含义。

## 这篇教程会做什么

看完这篇教程，你应该能从零复现这一条链路：

1. 在配置里打开 `enable_dynamic_topk`
2. 配置 `candidate_topk / min_topk / max_topk / token_budget / min_answer_chunks`
3. 在 Hybrid Retriever 初始化时注入 `DynamicTopKConfig`
4. 在检索主链路里先决策 `final_topk`，再执行 token 守卫
5. 把 `candidate_topk / final_topk / token_budget / truncate_reason / truncated_count` 写进日志和审计表

这篇教程主要覆盖这些文件：

1. `backend/internal/config/config.go`
2. `backend/internal/milvus/init.go`
3. `backend/internal/milvus/retrieval/topk_policy.go`
4. `backend/internal/milvus/retrieval/hybrid_search.go`
5. `backend/internal/milvus/retrieval/search.go`
6. `backend/api/handler/kb/handler.go`
7. `backend/internal/model/kb_retrieve_log.go`
8. `backend/cmd/server/main.go`
9. `backend/internal/config/config_rag_test.go`

如果先用一句人话概括最终链路，可以这样理解：

1. API 收到知识库检索请求
2. Hybrid Retriever 先确定候选池大小 `candidate_topk`
3. 规则策略根据 query 宽泛程度算出 `final_topk`
4. dense 和 sparse 召回、融合、去重、重排
5. token 守卫按预算继续截断结果
6. handler 把最终结果和 L4 指标写入响应日志与审计表

## 需要先理解的术语

### 什么是 TopK

`TopK` 的意思是“最多取前 K 条结果”。

比如：

1. `TopK = 3`，就是最多返回前 3 条
2. `TopK = 8`，就是最多返回前 8 条

在检索系统里，`TopK` 不是“必须返回 K 条”，而是“最多保留 K 条最相关结果”。

### 什么是候选 TopK

这里的 `candidate_topk` 可以先理解成：**为了给后续融合、去重、重排留空间，先多拿一些候选结果**。

比如最终只想返回 5 条，但 dense 和 sparse 两条路都可能各自命中一些好结果。  
这时候如果一开始就只拿 5 条候选，后面可选择空间太小，融合效果会受限。

所以这里把两个概念拆开了：

1. `candidate_topk`：先捞多少候选
2. `final_topk`：最后最多返回多少条

这一步非常重要，因为 L4 的核心不是“把固定 5 改成动态 6”，而是**把候选池大小和最终输出大小解耦**。

### 什么是动态 TopK

动态 TopK（Dynamic TopK）指的是：**不是所有 query 都用同一个 K，而是按 query 特征选一个更合适的 K**。

当前这版是“规则版”，不是学习型策略。它主要看这些信号：

1. query 是否明显属于宽泛问题
2. query 的长度是否较长
3. query 的 term 数是否较多
4. query 是否很短且很精确

### 什么是 token 守卫

token 守卫可以先理解成：**在最终把 chunk 交给 LLM 前，再做一次上下文成本检查**。

原因很简单：

1. 动态 TopK 解决的是“条数是否合适”
2. token 守卫解决的是“这些条内容加起来会不会太贵”

如果只有动态 TopK，没有 token 守卫，那么一个宽泛 query 可能拿到很多长 chunk，最终上下文成本还是会失控。

### 什么是最少回答块数

`min_answer_chunks` 的意思是：**即使 token 预算比较紧，也至少保留这么多条 chunk**。

这是一条“不要截得太狠”的保护线。  
否则预算一紧，系统可能只剩 1 条证据，回答会很脆弱。

## 整体流程

在看代码前，先把整体流程抓住：

1. 服务启动时读取 RAG Phase2 配置，并校验 L4 参数是否合法。
2. `InitMilvusManager` 创建 `HybridRetriever` 时，把动态 TopK 配置注入进去。
3. 请求进入 `HybridRetriever.SearchWithRequest`。
4. 在真正召回前，调用 `DecideDynamicTopK` 算出这次请求的 `final_topk`。
5. dense 与 sparse 并发召回，后面做融合、去重、可选重排。
6. 合并后的结果进入 `ApplyTokenBudgetGuard`，按 token 预算做最终截断。
7. 检索结果回到 handler，handler 把 `candidate_topk / final_topk / token_budget / truncate_reason / truncated_count` 落到审计日志。

如果你只记一件事，可以记这个顺序：

先决定“理论上最多保留几条”，再决定“预算上实际能保留几条”。

## 分步实现

## 第 1 步：先把配置开关和参数补齐

### 目标

让 L4 能被安全地打开、关闭、调参和校验，而不是把规则硬编码在检索流程里。

### 文件

`backend/internal/config/config.go`

### 完整代码

```go
type RAGFeatureFlags struct {
	EnableProdGuard       bool `yaml:"enable_prod_guard"`
	EnableIngestRetry     bool `yaml:"enable_ingest_retry"`
	EnableRetrieveAudit   bool `yaml:"enable_retrieve_audit"`
	EnableHybridRetrieval bool `yaml:"enable_hybrid_retrieval"`
	EnableQueryRewrite    bool `yaml:"enable_query_rewrite"`
	EnableDynamicTopK     bool `yaml:"enable_dynamic_topk"`
	EnableAdvancedRerank  bool `yaml:"enable_advanced_rerank"`
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

if value, ok, err := readEnvBool("RAG_ENABLE_DYNAMIC_TOPK"); err != nil {
	return err
} else if ok {
	c.RAG.FeatureFlags.EnableDynamicTopK = value
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
```

配置文件示例：

文件：`backend/config.yaml`

```yaml
rag:
  enabled: true
  environment: dev
  feature_flags:
    enable_prod_guard: false
    enable_ingest_retry: false
    enable_retrieve_audit: true
    enable_hybrid_retrieval: false
    enable_query_rewrite: false
    enable_dynamic_topk: false
    enable_advanced_rerank: false
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
    rewrite_timeout_ms: 120
    rewrite_max_expansions: 3
    rerank_timeout_ms: 250
    rerank_model: "jaccard-v1"
```

### 这段代码在做什么

这一层做了四件事：

1. 定义 L4 需要的配置项
2. 在开关开启时校验参数组合是否合法
3. 提供默认值，避免缺省配置直接把功能跑坏
4. 支持用环境变量覆盖，方便灰度和线上调参

### 为什么要这样做

最简单的写法当然是直接在策略代码里写：

1. `minTopK := 3`
2. `maxTopK := 8`
3. `tokenBudget := 1200`

但这样会有三个问题：

1. 线上出问题时没法快速关
2. 不同环境没法独立调参
3. 参数之间的非法组合不能提前拦截

尤其是 `candidate_topk < max_topk` 这一条，如果不在配置层先挡住，后面策略再聪明也没有意义，因为候选池本来就不够大。

### 它如何衔接下一步

有了这组配置，下一步就可以在初始化 Hybrid Retriever 时，把 L4 的策略参数真正注入检索链路。

## 第 2 步：在检索初始化阶段注入动态 TopK 配置

### 目标

让 Hybrid Retriever 拿到完整的 `DynamicTopKConfig`，这样策略逻辑就能在检索主链路里生效。

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
		Enabled:         cfg.RAG.FeatureFlags.EnableDynamicTopK,
		MinTopK:         cfg.RAG.Phase2.MinTopK,
		MaxTopK:         cfg.RAG.Phase2.MaxTopK,
		TokenBudget:     cfg.RAG.Phase2.TokenBudget,
		MinAnswerChunks: cfg.RAG.Phase2.MinAnswerChunks,
	},
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

这一步做的是“接线”：

1. 先决定候选池默认大小 `candidateTopK`
2. 再把动态 TopK 的参数整体放进 `HybridRetrieverConfig`
3. 最后用这份配置创建 `HybridRetriever`

### 为什么要这样做

这里最容易让初学者觉得“有点绕”的地方是：为什么 `CandidateTopK` 和 `DynamicTopK` 要同时存在？

原因是它们解决的是两个不同问题：

1. `CandidateTopK` 决定召回阶段能捞多少候选
2. `DynamicTopK` 决定最终最多应该保留多少结果

如果把这两个概念混成一个值，融合和重排阶段就没有缓冲空间了。

### 它如何衔接下一步

配置注入完成后，真正的 L4 决策就会发生在 `HybridRetriever.SearchWithRequest` 里。

## 第 3 步：定义动态 TopK 决策与 token 守卫

### 目标

把 L4 核心策略封装到一个独立文件里，让“如何算 K”和“如何做预算保护”都能单独阅读、测试和迭代。

### 文件

`backend/internal/milvus/retrieval/topk_policy.go`

### 完整代码

```go
package retrieval

import (
	"strings"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"
)

const (
	TruncateReasonNone        = ""
	TruncateReasonFinalTopK   = "final_topk"
	TruncateReasonTokenBudget = "token_budget"
)

type DynamicTopKConfig struct {
	Enabled         bool
	MinTopK         int
	MaxTopK         int
	TokenBudget     int
	MinAnswerChunks int
}

type TopKDecision struct {
	CandidateTopK  int
	RequestedTopK  int
	FinalTopK      int
	TokenBudget    int
	TruncateReason string
}

func DecideDynamicTopK(query string, candidateTopK int, requestedTopK int, cfg DynamicTopKConfig) TopKDecision {
	minTopK := cfg.MinTopK
	if minTopK <= 0 {
		minTopK = 1
	}
	maxTopK := cfg.MaxTopK
	if maxTopK < minTopK {
		maxTopK = minTopK
	}
	if candidateTopK > 0 && maxTopK > candidateTopK {
		maxTopK = candidateTopK
	}

	finalTopK := requestedTopK
	if finalTopK <= 0 {
		finalTopK = maxTopK
	}
	if !cfg.Enabled {
		finalTopK = clampInt(finalTopK, minTopK, maxTopK)
		return TopKDecision{
			CandidateTopK: candidateTopK,
			RequestedTopK: requestedTopK,
			FinalTopK:     finalTopK,
			TokenBudget:   cfg.TokenBudget,
		}
	}

	queryTrimmed := strings.TrimSpace(query)
	runeCount := utf8.RuneCountInString(queryTrimmed)
	termCount := len(strings.Fields(queryTrimmed))

	ruleTopK := minTopK
	switch {
	case isBroadQuery(queryTrimmed):
		ruleTopK = maxTopK
	case runeCount >= 48 || termCount >= 8:
		ruleTopK = maxTopK
	case runeCount >= 24 || termCount >= 5:
		ruleTopK = minTopK + (maxTopK-minTopK)/2 + 1
	case isShortPreciseQuery(queryTrimmed):
		ruleTopK = minTopK
	default:
		ruleTopK = minTopK + (maxTopK-minTopK)/2
	}

	finalTopK = clampInt(ruleTopK, minTopK, maxTopK)
	if requestedTopK > 0 && requestedTopK < finalTopK {
		finalTopK = clampInt(requestedTopK, minTopK, maxTopK)
	}

	return TopKDecision{
		CandidateTopK: candidateTopK,
		RequestedTopK: requestedTopK,
		FinalTopK:     finalTopK,
		TokenBudget:   cfg.TokenBudget,
	}
}

func ApplyTokenBudgetGuard(docs []*schema.Document, decision TopKDecision, cfg DynamicTopKConfig) ([]*schema.Document, TopKDecision) {
	if len(docs) == 0 {
		return docs, decision
	}

	guardTopK := decision.FinalTopK
	if guardTopK <= 0 || guardTopK > len(docs) {
		guardTopK = len(docs)
	}
	if guardTopK < 1 {
		guardTopK = 1
	}
	if cfg.MinAnswerChunks <= 0 {
		cfg.MinAnswerChunks = 1
	}
	if cfg.MinAnswerChunks > len(docs) {
		cfg.MinAnswerChunks = len(docs)
	}
	if guardTopK < cfg.MinAnswerChunks {
		guardTopK = cfg.MinAnswerChunks
	}

	truncated := docs
	if len(truncated) > guardTopK {
		truncated = truncated[:guardTopK]
		decision.TruncateReason = TruncateReasonFinalTopK
	}

	if decision.TokenBudget <= 0 {
		decision.FinalTopK = len(truncated)
		return truncated, decision
	}

	totalTokens := 0
	budgeted := make([]*schema.Document, 0, len(truncated))
	for idx, doc := range truncated {
		docTokens := estimateDocumentTokens(doc)
		if idx < cfg.MinAnswerChunks {
			budgeted = append(budgeted, doc)
			totalTokens += docTokens
			continue
		}
		if totalTokens+docTokens > decision.TokenBudget {
			decision.TruncateReason = TruncateReasonTokenBudget
			break
		}
		budgeted = append(budgeted, doc)
		totalTokens += docTokens
	}

	if len(budgeted) == 0 {
		budgeted = truncated[:1]
		totalTokens = estimateDocumentTokens(budgeted[0])
	}
	decision.FinalTopK = len(budgeted)
	return budgeted, decision
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

func isBroadQuery(query string) bool {
	lower := strings.ToLower(query)
	keywords := []string{
		"区别", "对比", "总结", "全面", "原理", "设计", "最佳实践",
		"difference", "compare", "overview", "design", "tradeoff", "best practice",
	}
	for _, keyword := range keywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func isShortPreciseQuery(query string) bool {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return false
	}
	return utf8.RuneCountInString(trimmed) <= 12 && len(strings.Fields(trimmed)) <= 3
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
```

### 这段代码在做什么

这份文件其实分成两层：

1. `DecideDynamicTopK` 负责“按规则先算一个目标 K”
2. `ApplyTokenBudgetGuard` 负责“按预算再做一次收口”

你可以把它理解成：

1. 第一次截断是“策略截断”
2. 第二次截断是“成本截断”

### 为什么这样设计

如果我们只做其中一个，会各有缺陷：

1. 只有动态 TopK，没有 token 守卫：条数合理，但总 token 可能失控
2. 只有 token 守卫，没有动态 TopK：虽然成本安全，但对宽泛 query 没有召回补偿

所以这版实现是“两段式”：

1. 先按 query 类型调 K
2. 再按 token 成本守预算

### 这一版规则是怎么工作的

`DecideDynamicTopK` 的规则很朴素，但很好解释：

1. query 包含“区别、对比、总结、design、tradeoff”这种宽泛词，直接倾向 `max_topk`
2. query 很长，或者 term 数很多，也倾向更大的 K
3. query 很短很准，就倾向 `min_topk`
4. 其他中间情况落到中间值

再加上一条保护：

1. 就算规则算出来很大，`requestedTopK` 如果更小，也要尊重请求方的更严格上限

### token 守卫为什么这样写

这里最关键的设计点有两个：

1. 先保证 `min_answer_chunks`
2. 再从第 `min_answer_chunks` 之后开始看预算

这表示系统在成本和回答完整性之间做了一个明确取舍：  
哪怕预算比较紧，也尽量先保住最少证据数量。

### 它如何衔接下一步

下一步就要把这两个函数真正接进 `HybridRetriever.SearchWithRequest` 主链路，不然它们只是“存在于代码里”。

## 第 4 步：在混合检索主链路里调用动态 TopK 和 token 守卫

### 目标

把 L4 策略插到真实召回流程里，而且要放在正确的位置。

### 文件

`backend/internal/milvus/retrieval/hybrid_search.go`

### 完整代码

```go
type HybridSearchRequest struct {
	Query           string
	OriginalQuery   string
	RewriteQuery    string
	FinalQuery      string
	RewriteStrategy string
	RewriteApplied  bool
	Expr            string
	TopK            int
	KBScope         string
	KBID            uint64
	RequestID       string
	Collection      string
	CandidateTopK   int
}

type HybridRetrieverConfig struct {
	CandidateTopK int
	DenseWeight   float64
	SparseWeight  float64
	SparseConfig  *SparseRetrieverConfig
	RerankerImpl  Reranker
	QueryRewriter QueryRewriter
	DynamicTopK   DynamicTopKConfig
}

func (h *HybridRetriever) SearchWithRequest(ctx context.Context, req *HybridSearchRequest) ([]*schema.Document, error) {
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
		err      error
		duration time.Duration
	}

	resultCh := make(chan routeResult, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		routeStart := time.Now()
		docs, err := h.searchDense(ctx, req)
		resultCh <- routeResult{route: routeDense, docs: docs, err: err, duration: time.Since(routeStart)}
	}()

	go func() {
		defer wg.Done()
		routeStart := time.Now()
		docs, err := h.sparseRetriever.Search(ctx, req)
		resultCh <- routeResult{route: routeSparse, docs: docs, err: err, duration: time.Since(routeStart)}
	}()

	wg.Wait()
	close(resultCh)

	var (
		denseDocs  []*schema.Document
		sparseDocs []*schema.Document
		denseErr   error
		sparseErr  error
		denseMS    int64
		sparseMS   int64
	)
	for routeRes := range resultCh {
		switch routeRes.route {
		case routeDense:
			denseDocs = routeRes.docs
			denseErr = routeRes.err
			denseMS = routeRes.duration.Milliseconds()
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
			"[RAG:L1] request_id=%s query=%q rewrite=%q final_query=%q rewrite_strategy=%q rewrite_applied=%t expr=%q candidate_topk=%d final_topk=%d token_budget=%d truncate_reason=%q routes=%s route_hits={dense:0,sparse:0} final_count=0 empty_reason=empty-after-retrieve duration_ms=%d dense_error=%q sparse_error=%q",
			req.RequestID, req.OriginalQuery, req.RewriteQuery, req.FinalQuery, req.RewriteStrategy, req.RewriteApplied, req.Expr, topKDecision.CandidateTopK, topKDecision.FinalTopK, topKDecision.TokenBudget, topKDecision.TruncateReason, "dense+sparse", totalMS, denseErr.Error(), sparseErr.Error(),
		)
		return nil, fmt.Errorf("hybrid retrieval failed: dense=%v sparse=%v", denseErr, sparseErr)
	}

	rawCandidateCount := len(denseDocs) + len(sparseDocs)
	if rawCandidateCount == 0 {
		totalMS := time.Since(start).Milliseconds()
		log.Printf(
			"[RAG:L2] request_id=%s query=%q rewrite=%q final_query=%q rewrite_strategy=%q rewrite_applied=%t expr=%q candidate_topk=%d final_topk=%d token_budget=%d truncate_reason=%q routes=%s route_hits={dense:%d,sparse:%d} final_count=0 empty_reason=%s duration_ms=%d dense_ms=%d sparse_ms=%d dense_error=%q sparse_error=%q",
			req.RequestID, req.OriginalQuery, req.RewriteQuery, req.FinalQuery, req.RewriteStrategy, req.RewriteApplied, req.Expr, topKDecision.CandidateTopK, topKDecision.FinalTopK, topKDecision.TokenBudget, topKDecision.TruncateReason, "dense+sparse", len(denseDocs), len(sparseDocs), EmptyReasonAfterRetrieve, totalMS, denseMS, sparseMS, toLogError(denseErr), toLogError(sparseErr),
		)
		return []*schema.Document{}, nil
	}

	fused := FuseRouteCandidates(denseDocs, sparseDocs, FusionConfig{
		DenseWeight:  h.config.DenseWeight,
		SparseWeight: h.config.SparseWeight,
	})
	if len(fused) == 0 {
		totalMS := time.Since(start).Milliseconds()
		log.Printf(
			"[RAG:L2] request_id=%s query=%q rewrite=%q final_query=%q rewrite_strategy=%q rewrite_applied=%t expr=%q candidate_topk=%d final_topk=%d token_budget=%d truncate_reason=%q routes=%s route_hits={dense:%d,sparse:%d} final_count=0 empty_reason=%s duration_ms=%d dense_ms=%d sparse_ms=%d dense_error=%q sparse_error=%q",
			req.RequestID, req.OriginalQuery, req.RewriteQuery, req.FinalQuery, req.RewriteStrategy, req.RewriteApplied, req.Expr, topKDecision.CandidateTopK, topKDecision.FinalTopK, topKDecision.TokenBudget, topKDecision.TruncateReason, "dense+sparse", len(denseDocs), len(sparseDocs), EmptyReasonAfterFusion, totalMS, denseMS, sparseMS, toLogError(denseErr), toLogError(sparseErr),
		)
		return []*schema.Document{}, nil
	}

	merged := DeduplicateFusedDocuments(fused)
	if len(merged) == 0 {
		totalMS := time.Since(start).Milliseconds()
		log.Printf(
			"[RAG:L2] request_id=%s query=%q rewrite=%q final_query=%q rewrite_strategy=%q rewrite_applied=%t expr=%q candidate_topk=%d final_topk=%d token_budget=%d truncate_reason=%q routes=%s route_hits={dense:%d,sparse:%d} final_count=0 empty_reason=%s duration_ms=%d dense_ms=%d sparse_ms=%d dense_error=%q sparse_error=%q",
			req.RequestID, req.OriginalQuery, req.RewriteQuery, req.FinalQuery, req.RewriteStrategy, req.RewriteApplied, req.Expr, topKDecision.CandidateTopK, topKDecision.FinalTopK, topKDecision.TokenBudget, topKDecision.TruncateReason, "dense+sparse", len(denseDocs), len(sparseDocs), EmptyReasonAfterFusion, totalMS, denseMS, sparseMS, toLogError(denseErr), toLogError(sparseErr),
		)
		return []*schema.Document{}, nil
	}

	if h.reranker != nil {
		reranked, err := h.reranker.Rerank(ctx, req.OriginalQuery, merged)
		if err == nil && len(reranked) > 0 {
			merged = reranked
		}
	}

	emptyReason := EmptyReasonNone
	if len(merged) == 0 {
		emptyReason = EmptyReasonAfterFilter
	}

	beforeTruncateCount := len(merged)
	merged, topKDecision = ApplyTokenBudgetGuard(merged, topKDecision, h.config.DynamicTopK)
	truncatedCount := beforeTruncateCount - len(merged)

	totalMS := time.Since(start).Milliseconds()
	log.Printf(
		"[RAG:L2] request_id=%s query=%q rewrite=%q final_query=%q rewrite_strategy=%q rewrite_applied=%t expr=%q candidate_topk=%d final_topk=%d token_budget=%d truncate_reason=%q routes=%s route_hits={dense:%d,sparse:%d} final_count=%d truncated_count=%d empty_reason=%s duration_ms=%d dense_ms=%d sparse_ms=%d dense_error=%q sparse_error=%q",
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
		topKDecision.TruncateReason,
		"dense+sparse",
		len(denseDocs),
		len(sparseDocs),
		len(merged),
		truncatedCount,
		emptyReason,
		totalMS,
		denseMS,
		sparseMS,
		toLogError(denseErr),
		toLogError(sparseErr),
	)
	return merged, nil
}
```

### 这段代码在做什么

这里最关键的是两行：

```go
topKDecision := DecideDynamicTopK(req.FinalQuery, req.CandidateTopK, req.TopK, h.config.DynamicTopK)
merged, topKDecision = ApplyTokenBudgetGuard(merged, topKDecision, h.config.DynamicTopK)
```

第一行发生在**真正召回前**，第二行发生在**融合、去重、重排后**。

### 为什么要放在这个位置

这一步如果放错位置，效果会完全不一样：

1. 如果在 dense/sparse 召回前就用 `final_topk` 限死候选池，会损失融合空间
2. 如果在融合前做 token 守卫，根本不知道最终保留的是哪些结果
3. 如果在 handler 层再做 token 守卫，说明前面已经浪费掉了检索和重排成本

所以正确位置是：

1. 候选召回阶段用 `candidate_topk`
2. 合并完成后再对最终结果做 `final_topk + token_budget` 收口

### 它如何衔接下一步

到这一步，检索链路里已经有了 L4 行为。下一步要解决的是：这些信息怎么带给上层，让 API 日志和数据库审计能看见它。

## 第 5 步：扩展搜索指标结构，让 L4 指标能往上传

### 目标

让上层 handler 能拿到 L4 的关键指标，而不是只知道“返回了几条文档”。

### 文件

`backend/internal/milvus/retrieval/search.go`

### 完整代码

```go
type SearchMetrics struct {
	EmbeddingMs    int64
	SearchMs       int64
	PostprocessMs  int64
	HitCount       int
	TruncatedCount int
	CandidateTopK  int
	FinalTopK      int
	TokenBudget    int
	TruncateReason string
}

type SearchResult struct {
	Documents []*schema.Document
	Metrics   SearchMetrics
}
```

### 这段代码在做什么

它把 L4 指标统一放进 `SearchMetrics`，这样上层无论是：

1. API handler
2. 结构化日志
3. 审计表落库
4. 后续监控面板

都可以走同一份指标结构。

### 为什么要这样做

如果这些字段只出现在 `HybridRetriever` 内部日志里，就只能“人肉看日志”，没法稳定做审计和统计。

把它们放进 `SearchMetrics`，本质上是在建立一个检索层和 API 层之间的指标契约。

### 它如何衔接下一步

有了 `SearchMetrics` 之后，handler 就能把这些值安全地带到业务日志和数据库表里。

## 第 6 步：在 API 层把动态 TopK 和 token 守卫结果写入审计日志

### 目标

让请求级审计真正能看见 L4 是否生效、为什么截断、最后截成了多少条。

### 文件

`backend/api/handler/kb/handler.go`

### 完整代码

```go
topK := clampTopK(req.TopK)

if useHybrid {
	docs, hybridErr := manager.GetHybridRetriever().Search(retrieveCtx, req.Query, &retrieval.RetrieveOptions{
		TopK:             topK,
		Collection:       collection,
		KBScope:          "global",
		ActiveGlobalKBID: activeGlobalKBID,
		RequestID:        requestID,
		OriginalQuery:    req.Query,
	})
	searchResult = &retrieval.SearchResult{
		Documents: docs,
		Metrics: retrieval.SearchMetrics{
			SearchMs:      time.Since(start).Milliseconds(),
			HitCount:      len(docs),
			FinalTopK:     topK,
			CandidateTopK: topK,
		},
	}
	searchErr = hybridErr
} else {
	searchResult, searchErr = retriever.RetrieveKnowledgeWithMetrics(retrieveCtx, req.Query, activeGlobalKBID, topK, collection)
}

searchMetrics := searchResult.Metrics

retrieveLog := &model.KBRetrieveLog{
	RequestID:        requestID,
	UserID:           userID,
	KBIDs:            formatKBIDs(kbIDs),
	Query:            req.Query,
	FinalQuery:       firstNonEmptyString(extractFinalQuery(docs), req.Query),
	Expr:             expr,
	TopK:             topK,
	CandidateTopK:    searchMetrics.CandidateTopK,
	FinalTopK:        searchMetrics.FinalTopK,
	TokenBudget:      searchMetrics.TokenBudget,
	TruncateReason:   searchMetrics.TruncateReason,
	Rewrite:          extractRewriteQuery(docs),
	RewriteStrategy:  extractRewriteStrategy(docs),
	RewriteApplied:   extractRewriteApplied(docs),
	Routes:           resolveRetrieveRoutes(useHybrid),
	Collection:       collection,
	RetrieverVersion: "v1",
	FinalCount:       len(items),
	TruncatedCount:   searchMetrics.TruncatedCount,
	ResultStatus:     resultStatus,
	EmbeddingMs:      searchMetrics.EmbeddingMs,
	SearchMs:         searchMetrics.SearchMs,
	PostprocessMs:    searchMetrics.PostprocessMs,
	DurationMs:       durationMs,
	TimeoutMs:        retrieveTimeout.Milliseconds(),
}
persistRetrieveLog(retrieveLog)

if config.Global.RAG.FeatureFlags.EnableRetrieveAudit {
	log.Printf(
		"[KB Retrieve] request_id=%s query=%q final_query=%q rewrite=%q rewrite_strategy=%q rewrite_applied=%t user_id=%d kb_ids=%v kb_scope=%q expr=%q topk=%d candidate_topk=%d final_topk=%d token_budget=%d truncate_reason=%q routes=%q final_count=%d hit_count=%d truncated_count=%d duration_ms=%d embedding_ms=%d search_ms=%d postprocess_ms=%d timeout_ms=%d result_status=%s",
		requestID,
		req.Query,
		retrieveLog.FinalQuery,
		retrieveLog.Rewrite,
		retrieveLog.RewriteStrategy,
		retrieveLog.RewriteApplied,
		userID,
		kbIDs,
		"global",
		expr,
		topK,
		searchMetrics.CandidateTopK,
		searchMetrics.FinalTopK,
		searchMetrics.TokenBudget,
		searchMetrics.TruncateReason,
		retrieveLog.Routes,
		len(items),
		searchMetrics.HitCount,
		searchMetrics.TruncatedCount,
		durationMs,
		searchMetrics.EmbeddingMs,
		searchMetrics.SearchMs,
		searchMetrics.PostprocessMs,
		retrieveTimeout.Milliseconds(),
		string(resultStatus),
	)
}
```

### 这段代码在做什么

这一层把 L4 结果变成了“可审计数据”：

1. 写入 `KBRetrieveLog`
2. 打印结构化检索日志

这里最值得注意的字段有：

1. `candidate_topk`
2. `final_topk`
3. `token_budget`
4. `truncate_reason`
5. `truncated_count`

### 为什么要这样做

如果没有这些字段，线上看到“结果只有 3 条”时，你根本不知道是因为：

1. query 本来就只命中 3 条
2. 动态 TopK 规则把它定成了 3
3. token 预算把它从 6 截成了 3

而这些原因，对调参和排障完全不是一回事。

### 它如何衔接下一步

日志打通后，最后还差一件事：数据库模型也要能存这些字段，不然结构化日志能看，历史审计查不到。

## 第 7 步：扩展检索审计表模型

### 目标

让 L4 指标不仅能打印出来，还能稳定落库，支持后续查询、比对和报表统计。

### 文件

`backend/internal/model/kb_retrieve_log.go`

### 完整代码

```go
type KBRetrieveLog struct {
	ID               uint64               `json:"id" gorm:"primaryKey;autoIncrement"`
	RequestID        string               `json:"request_id" gorm:"uniqueIndex;size:64;not null"`
	UserID           uint                 `json:"user_id" gorm:"index;not null"`
	KBIDs            string               `json:"kb_ids" gorm:"size:500"`
	Query            string               `json:"query" gorm:"size:2000;not null"`
	FinalQuery       string               `json:"final_query" gorm:"size:2000"`
	Expr             string               `json:"expr" gorm:"size:2000"`
	TopK             int                  `json:"top_k"`
	CandidateTopK    int                  `json:"candidate_topk"`
	FinalTopK        int                  `json:"final_topk"`
	TokenBudget      int                  `json:"token_budget"`
	TruncateReason   string               `json:"truncate_reason" gorm:"size:64"`
	Rewrite          string               `json:"rewrite" gorm:"size:1000"`
	RewriteStrategy  string               `json:"rewrite_strategy" gorm:"size:255"`
	RewriteApplied   bool                 `json:"rewrite_applied"`
	Routes           string               `json:"routes" gorm:"size:200"`
	Collection       string               `json:"collection" gorm:"size:200"`
	RetrieverVersion string               `json:"retriever_version" gorm:"size:50"`
	FinalCount       int                  `json:"final_count"`
	TruncatedCount   int                  `json:"truncated_count"`
	ResultStatus     RetrieveResultStatus `json:"result_status" gorm:"size:20;not null;default:'success';index"`
	ErrorCode        string               `json:"error_code" gorm:"size:64"`
	ErrorMsg         string               `json:"error_msg" gorm:"size:1000"`
	EmbeddingMs      int64                `json:"embedding_ms"`
	SearchMs         int64                `json:"search_ms"`
	PostprocessMs    int64                `json:"postprocess_ms"`
	DurationMs       int64                `json:"duration_ms"`
	TimeoutMs        int64                `json:"timeout_ms"`
	CreatedAt        time.Time            `json:"created_at" gorm:"autoCreateTime:milli;index"`
}
```

### 这段代码在做什么

它把 L4 指标纳入了知识库检索审计模型，让每次请求都能留下完整上下文：

1. 请求想要多少条 `top_k`
2. 候选池拿了多少条 `candidate_topk`
3. 最后决定保留多少条 `final_topk`
4. token 预算是多少 `token_budget`
5. 截断原因是什么 `truncate_reason`
6. 最终被截掉多少条 `truncated_count`

### 为什么要这样做

这一步本质上是在给后续运营和调参留抓手。

比如你后面想回答这些问题：

1. 哪类 query 最容易触发 `token_budget`
2. `max_topk = 8` 是否真的带来了更高 `final_count`
3. 宽泛 query 是否经常因为预算又被压回去

没有落库字段，这些分析几乎做不了。

## 第 8 步：在启动日志里打印 L4 配置快照

### 目标

让服务启动时就能知道当前实例到底用了什么 L4 配置。

### 文件

`backend/cmd/server/main.go`

### 完整代码

```go
log.Printf("[RAG:L0] phase2_flags hybrid=%t rewrite=%t dynamic_topk=%t advanced_rerank=%t",
	cfg.RAG.FeatureFlags.EnableHybridRetrieval,
	cfg.RAG.FeatureFlags.EnableQueryRewrite,
	cfg.RAG.FeatureFlags.EnableDynamicTopK,
	cfg.RAG.FeatureFlags.EnableAdvancedRerank,
)
log.Printf("[RAG:L0] phase2_params dense_weight=%.3f sparse_weight=%.3f candidate_topk=%d min_topk=%d max_topk=%d token_budget=%d min_answer_chunks=%d rewrite_timeout_ms=%d rewrite_max_expansions=%d rerank_timeout_ms=%d rerank_model=%s",
	cfg.RAG.Phase2.HybridDenseWeight,
	cfg.RAG.Phase2.HybridSparseWeight,
	cfg.RAG.Phase2.CandidateTopK,
	cfg.RAG.Phase2.MinTopK,
	cfg.RAG.Phase2.MaxTopK,
	cfg.RAG.Phase2.TokenBudget,
	cfg.RAG.Phase2.MinAnswerChunks,
	cfg.RAG.Phase2.RewriteTimeoutMS,
	cfg.RAG.Phase2.RewriteMaxExpansions,
	cfg.RAG.Phase2.RerankTimeoutMS,
	cfg.RAG.Phase2.RerankModel,
)
```

### 这段代码在做什么

它在服务启动时把 L4 关键开关和参数打印出来。

### 为什么要这样做

如果线上日志只打印请求级结果，但不打印实例级配置，你会遇到一个很常见的问题：

同样一条 query，在两个环境表现不同，但你不知道到底是代码不同，还是配置不同。

启动快照能先把这个问题解决掉。

## 如何验证

这一层建议至少从三类验证做起。

### 1. 配置校验验证

文件：`backend/internal/config/config_rag_test.go`

这里已经有一个直接覆盖 L4 参数非法组合的测试：

```go
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
```

这个测试验证的是最基础的一件事：  
参数非法时，系统应该在启动前就报错，而不是把错误留到请求期。

### 2. 检索日志字段验证

文件：`backend/internal/milvus/retrieval/search.go`

确认 `SearchMetrics` 已包含这些字段：

1. `CandidateTopK`
2. `FinalTopK`
3. `TokenBudget`
4. `TruncateReason`
5. `TruncatedCount`

这一步虽然看起来简单，但很重要，因为 handler 和数据库落库都依赖这层契约。

### 3. 请求级日志验证

发起知识库检索请求后，重点观察日志里这些字段是否符合预期：

1. `candidate_topk`
2. `final_topk`
3. `token_budget`
4. `truncate_reason`
5. `truncated_count`

你可以用两个简单场景验证：

1. 宽泛 query，比如“Java 锁的区别和最佳实践总结”
2. 短而准的 query，比如“CAS 原理”

预期现象通常是：

1. 宽泛 query 的 `final_topk` 更接近 `max_topk`
2. 短精确 query 的 `final_topk` 更接近 `min_topk`
3. 如果 token 预算很小，`truncate_reason` 会变成 `token_budget`

## 取舍与后续优化

### 这版优化了什么

这一版最核心的收益有三个：

1. 不再让所有 query 共用同一个固定 K
2. 把候选池大小和最终输出大小拆开了
3. 在最终上下文进入 LLM 前加了一道成本守卫

### 这版故意没有做什么

这版还是明确偏“规则版”，所以它没有解决这些问题：

1. 不看检索分数分布，只看 query 规则
2. token 估算是近似值，不是真实 tokenizer 计数
3. 不会根据不同知识库类型自动切换策略

这不是缺点，而是当前阶段有意为之。  
因为 L4 先要解决的是“先把机制做对、日志打通、参数可控”，而不是一开始就把策略做得非常复杂。

### 下一步自然演进方向

如果后面继续做 L4/L5 演进，比较自然的方向是：

1. 让动态 TopK 参考融合后的分数分布，而不只看 query 文字特征
2. 把 token 估算从粗略的“4 个字符约 1 token”升级成真实 tokenizer
3. 让不同知识库、不同 query 类型采用不同策略模板
4. 把 `truncate_reason` 和 `final_topk` 拉进监控面板，做长期观察

如果你想用一句话记住这一层，可以记成：

L4 不是“多返回几条文档”，而是把“返回几条”和“最多花多少上下文成本”都变成了可控策略。
