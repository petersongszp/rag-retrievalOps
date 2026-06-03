# KB L3 查询改写与术语扩展实现教程

## 背景

这一层功能的结论先讲在前面：它不是为了“让 query 看起来更聪明”，而是为了在不改动用户原始问题含义的前提下，尽量补齐缩写、别名和明显拼写错误，从而提高检索命中率。

在只有原始 query 的时候，下面几类问题很容易召回不稳：

1. 用户直接输入缩写，比如 `jvm gc`、`mq rpc`
2. 用户输入的是常见别名，比如 `golang` 和 `go`
3. 用户有轻微拼写错误，比如 `sprinboot`
4. 用户问题很短，向量检索可利用的信息不够多

这套实现的目标不是替换原始 query，而是做一个“受控增强层”：

1. 能开关控制
2. 能限制扩展数量
3. 能超时回退
4. 能记录 rewrite 前后结果
5. 能通过日志和测试验证效果

## 这篇教程会做什么

看完这篇文档，你会明白这个 L3 功能是怎样从配置一路接到检索主链路里的。

这篇教程主要覆盖这些文件：

1. `backend/internal/milvus/retrieval/rewrite.go`
2. `backend/internal/milvus/retrieval/hybrid_search.go`
3. `backend/internal/milvus/init.go`
4. `backend/internal/config/config.go`
5. `backend/api/handler/kb/handler.go`
6. `backend/internal/model/kb_retrieve_log.go`
7. `backend/internal/milvus/retrieval/rewrite_test.go`
8. `backend/config.yaml`
9. `backend/config.example.yaml`

最终链路可以先用一句人话理解：

1. 请求进入知识库检索接口
2. 如果开启了查询改写，就先对原始 query 做受控 rewrite
3. rewrite 结果进入 hybrid retriever 的 dense+sparse 召回
4. 每个结果文档都挂上 rewrite 元数据
5. handler 把 rewrite 信息写入审计日志，方便排查和对比效果

## 需要先理解的术语

### 查询改写

你可以先把“查询改写”理解成：在真正检索前，先把用户问题整理成更适合检索系统理解的样子。

比如用户输入：

`jvm gc`

系统会把它扩成更完整的检索词：

`jvm java virtual machine gc garbage collection`

这里要注意，当前实现不是“改写成另一个完全不同的问题”，而是“保留原词，再补充解释词”。

### 术语扩展

“术语扩展”可以先理解成：给一个词补上它的别名、全称或者常见另一种写法。

比如：

1. `rpc` 扩成 `remote procedure call`
2. `k8s` 扩成 `kubernetes`
3. `springboot` 和 `spring boot` 互相补充

这一步的价值在于，知识库里的文档不一定和用户用同一种叫法。扩展术语，就是在帮 query 和文档“对齐语言”。

### 受控改写

“受控”是这次实现里最重要的词。它的意思是：改写不是随便做，而是要被明确约束。

这里的约束主要有四种：

1. 只有 feature flag 打开时才启用
2. 只做规则型扩展，不做不可解释的自由生成
3. 最多只补有限个扩展词
4. 命中黑名单或上下文超时时，直接回退原 query

### 黑名单

黑名单的意思是：有些 query 一看就不适合做 rewrite，系统要立刻跳过。

比如：

1. 包含引号的精确短语
2. 包含 `site:` 这样的搜索语法
3. 包含 `select`、`drop` 这样的类 SQL 文本
4. 包含 URL

如果这些 query 也被强行拆词和扩展，反而可能破坏用户本来的意图。

### Rewrite 元数据

rewrite 元数据就是跟着检索结果一起往后传的附加信息，主要包括：

1. `original_query`
2. `rewrite_query`
3. `final_query`
4. `rewrite_strategy`
5. `rewrite_applied`

它的作用不是给用户展示，而是让后面的 handler、日志和审计表能知道这次请求到底改没改、怎么改的。

## 整体流程

先看全局，再看代码，会更容易。

1. 服务启动时读取 `config.yaml`，确定是否开启 `enable_query_rewrite`
2. `InitMilvusManager` 创建 `HybridRetriever` 时，把 `ControlledQueryRewriter` 注入进去
3. 请求进入 `Retrieve` 接口后，如果当前走 hybrid 检索，就会调用 `HybridRetriever.Search`
4. `HybridRetriever.SearchWithRequest` 在真正检索前先执行 `applyControlledRewrite`
5. rewriter 输出 `RewriteQuery`、`FinalQuery`、`Strategy` 等结果
6. dense route 用 `FinalQuery` 去检索，并把 rewrite 元数据挂到每个文档上
7. handler 从文档元数据里提取 rewrite 信息，写入 `kb_retrieve_log`
8. 测试用例验证缩写扩展、扩展上限、拼写修正、黑名单跳过和超时回退

如果你想抓住最核心的一点，可以记住：

这次设计把“怎么改 query”和“怎么做检索”分开了。改写器只负责产出 rewrite 结果，检索器只负责消费这个结果并继续执行召回。

## 分步实现

## 第 1 步：先把配置开关和参数定义好

### 目标

先把这个功能做成可配置、可关闭、可校验的能力，而不是把规则硬编码死在检索流程里。

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
	RewriteTimeoutMS     int     `yaml:"rewrite_timeout_ms"`
	RewriteMaxExpansions int     `yaml:"rewrite_max_expansions"`
	RerankTimeoutMS      int     `yaml:"rerank_timeout_ms"`
	RerankModel          string  `yaml:"rerank_model"`
}

if c.RAG.FeatureFlags.EnableQueryRewrite {
	if c.RAG.Phase2.RewriteTimeoutMS <= 0 {
		return fmt.Errorf("rag query rewrite enabled but rag.phase2.rewrite_timeout_ms must be > 0")
	}
	if c.RAG.Phase2.RewriteMaxExpansions <= 0 {
		return fmt.Errorf("rag query rewrite enabled but rag.phase2.rewrite_max_expansions must be > 0")
	}
}

if c.RAG.Phase2.RewriteTimeoutMS <= 0 {
	c.RAG.Phase2.RewriteTimeoutMS = 120
}
if c.RAG.Phase2.RewriteMaxExpansions <= 0 {
	c.RAG.Phase2.RewriteMaxExpansions = 3
}

if value, ok, err := readEnvBool("RAG_ENABLE_QUERY_REWRITE"); err != nil {
	return err
} else if ok {
	c.RAG.FeatureFlags.EnableQueryRewrite = value
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
```

配置文件里对应的是：

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

这一层做了三件事：

1. 定义 rewrite 的开关和参数
2. 在开关打开时做参数合法性校验
3. 给 rewrite 参数设置默认值，并支持环境变量覆盖

### 为什么要这样做

更简单的写法当然可以是：直接在 `rewrite.go` 里写死 `maxExpansions := 3`，然后默认总是开启。

但这样做有几个问题：

1. 出线上问题时没法快速关闭
2. 不同环境没法用不同参数
3. 你不知道当前 rewrite 是按什么配置跑的
4. 后面做 A/B 或灰度时会非常别扭

所以这一步本质上是在建立“可控上线”的前提。

### 它如何衔接下一步

有了配置之后，下一步才能在启动时按配置决定要不要创建 query rewriter。

## 第 2 步：定义 rewrite 的输入输出契约

### 目标

把改写器做成一个明确的接口，而不是散落在检索逻辑里的几个字符串处理函数。

### 文件

`backend/internal/milvus/retrieval/rewrite.go`

### 完整代码

```go
const (
	RewriteStrategyNone      = "none"
	RewriteStrategyBlacklist = "blacklist"
	RewriteStrategyRuleBased = "rule_based"
	RewriteStrategyTimeout   = "timeout_fallback"
)

type QueryRewriteResult struct {
	OriginalQuery   string
	RewriteQuery    string
	FinalQuery      string
	Strategy        string
	Applied         bool
	Skipped         bool
	ExpansionTerms  []string
	CorrectedTerms  []string
	BlockedByPolicy bool
}

type QueryRewriter interface {
	Rewrite(ctx context.Context, query string) QueryRewriteResult
}

type QueryRewriterConfig struct {
	MaxExpansions int
}
```

### 这段代码在做什么

它定义了这套功能最核心的契约：

1. 输入是什么：一个 `query`
2. 输出是什么：一个结构化的 `QueryRewriteResult`
3. 改写器长什么样：实现 `Rewrite(ctx, query)` 方法

### 为什么要这样做

如果没有这层接口，最直接的写法就是在 `HybridRetriever` 里直接写很多 `if strings.Contains(...)`。

短期看能跑，长期会有几个问题：

1. 很难单独测试 rewrite 逻辑
2. 很难替换成别的 rewrite 实现
3. 检索器会知道太多术语规则细节
4. 日志层拿不到统一结构的 rewrite 结果

所以这一步的本质是在做职责拆分：改写器负责“生成 rewrite 结果”，检索器负责“使用 rewrite 结果”。

### 它如何衔接下一步

有了接口和结果结构后，下一步就能写真正的规则型改写器。

## 第 3 步：实现受控规则改写器

### 目标

实现一版可解释、可测试、可回退的规则型 query rewriter。

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
}

func NewControlledQueryRewriter(cfg *QueryRewriterConfig) *ControlledQueryRewriter {
	config := QueryRewriterConfig{
		MaxExpansions: 3,
	}
	if cfg != nil && cfg.MaxExpansions > 0 {
		config.MaxExpansions = cfg.MaxExpansions
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
		},
		aliases: map[string][]string{
			"golang":       {"go"},
			"go":           {"golang"},
			"redis":        {"redis cache"},
			"es":           {"elasticsearch"},
			"spring":       {"spring framework"},
			"springboot":   {"spring boot"},
			"spring boot":  {"springboot"},
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
		},
	}
}
```

### 这段代码在做什么

这个构造函数把规则表一次性准备好，规则分成四类：

1. 缩写扩展
2. 别名补充
3. 拼写纠正
4. 黑名单跳过

### 为什么要这样做

这里最值得注意的是：当前实现没有引入模型来做 rewrite，而是先用规则。

原因很实际：

1. 规则型 rewrite 可解释
2. 规则型 rewrite 便于测试
3. 规则型 rewrite 不会引入额外模型延迟
4. 在术语型场景里，规则往往已经能覆盖大量高频问题

这一步不是说模型改写不好，而是说在 L3 第一版里，规则版是更稳的落地方式。

### 它如何衔接下一步

接下来要把这些规则真正跑起来，也就是实现 `Rewrite` 方法。

## 第 4 步：实现 rewrite 主流程

### 目标

让改写器能在一次请求里完成跳过、纠错、扩展、去重、限流和回退。

### 文件

`backend/internal/milvus/retrieval/rewrite.go`

### 完整代码

```go
func (r *ControlledQueryRewriter) Rewrite(ctx context.Context, query string) QueryRewriteResult {
	trimmed := strings.TrimSpace(query)
	result := QueryRewriteResult{
		OriginalQuery: trimmed,
		FinalQuery:    trimmed,
		Strategy:      RewriteStrategyNone,
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

	expansionLimit := r.config.MaxExpansions
	rewriteTerms := make([]string, 0, len(tokens)+expansionLimit)
	seen := make(map[string]struct{}, len(tokens)+expansionLimit)
	expansions := make([]string, 0, expansionLimit)
	correctedTerms := make([]string, 0, 2)

	addTerm := func(term string) bool {
		normalized := normalizeRewriteTerm(term)
		if normalized == "" {
			return false
		}
		if _, exists := seen[normalized]; exists {
			return false
		}
		seen[normalized] = struct{}{}
		rewriteTerms = append(rewriteTerms, term)
		return true
	}
	addExpansion := func(term string) {
		if len(expansions) >= expansionLimit {
			return
		}
		if addTerm(term) {
			expansions = append(expansions, term)
		}
	}

	for _, token := range tokens {
		select {
		case <-ctx.Done():
			result.Strategy = RewriteStrategyTimeout
			result.Skipped = true
			result.RewriteQuery = ""
			result.FinalQuery = trimmed
			result.Applied = false
			return result
		default:
		}

		normalized := normalizeRewriteTerm(token)
		if corrected, ok := r.typoCorrections[normalized]; ok {
			if addTerm(corrected) {
				correctedTerms = append(correctedTerms, corrected)
			}
		}
		addTerm(token)
		if values, ok := r.abbreviations[normalized]; ok {
			for _, value := range values {
				addExpansion(value)
			}
		}
		if values, ok := r.aliases[normalized]; ok {
			for _, value := range values {
				addExpansion(value)
			}
		}
	}

	finalQuery := strings.TrimSpace(strings.Join(rewriteTerms, " "))
	if finalQuery == "" || strings.EqualFold(finalQuery, trimmed) {
		result.Skipped = true
		result.FinalQuery = trimmed
		result.RewriteQuery = ""
		result.CorrectedTerms = correctedTerms
		result.ExpansionTerms = expansions
		return result
	}

	result.Strategy = RewriteStrategyRuleBased
	result.Applied = true
	result.RewriteQuery = finalQuery
	result.FinalQuery = finalQuery
	result.ExpansionTerms = expansions
	result.CorrectedTerms = correctedTerms
	return result
}
```

### 这段代码在做什么

这段逻辑的执行顺序很关键：

1. 先处理空 query
2. 再处理 context 超时
3. 再检查黑名单
4. 再拆词
5. 对每个 term 做拼写纠正、缩写扩展、别名扩展
6. 用 `seen` 去重
7. 用 `MaxExpansions` 控制扩展上限
8. 如果最终 query 和原 query 一样，就视为未生效

### 为什么要这样做

这里有三个很重要的设计点。

第一，原词要保留。

如果只保留扩展词，不保留原词，用户原始表达可能反而丢掉。现在的做法是“原词 + 补充词”一起保留，更稳。

第二，要限制扩展数量。

如果不限制，越补越长，dense 检索和 sparse 检索都可能被噪声拖垮。当前的 `rewrite_max_expansions` 就是在控制这个风险。

第三，要允许优雅回退。

如果超时、黑名单命中，或者最后没有产生有效变化，就直接用原 query。这样 rewrite 是增强项，不是单点风险源。

### 它如何衔接下一步

有了 `Rewrite` 主流程后，还需要把 token 化、归一化和策略串格式化这些辅助逻辑补齐。

## 第 5 步：补齐 token、归一化和策略格式化

### 目标

把 rewrite 的辅助规则做完整，避免主逻辑里塞太多细节。

### 文件

`backend/internal/milvus/retrieval/rewrite.go`

### 完整代码

```go
func tokenizeRewriteTerms(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	parts := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return false
		}
		return true
	})
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		normalized := normalizeRewriteTerm(part)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func normalizeRewriteTerm(term string) string {
	trimmed := strings.TrimSpace(strings.ToLower(term))
	if len([]rune(trimmed)) < 2 {
		return ""
	}
	return trimmed
}

func formatRewriteStrategy(result QueryRewriteResult) string {
	if result.Strategy == "" {
		return RewriteStrategyNone
	}
	if len(result.ExpansionTerms) == 0 && len(result.CorrectedTerms) == 0 {
		return result.Strategy
	}
	parts := []string{result.Strategy}
	if len(result.CorrectedTerms) > 0 {
		sorted := append([]string(nil), result.CorrectedTerms...)
		sort.Strings(sorted)
		parts = append(parts, "corrected="+strings.Join(sorted, "|"))
	}
	if len(result.ExpansionTerms) > 0 {
		sorted := append([]string(nil), result.ExpansionTerms...)
		sort.Strings(sorted)
		parts = append(parts, "expanded="+strings.Join(sorted, "|"))
	}
	return strings.Join(parts, ";")
}
```

### 这段代码在做什么

这几个辅助函数分别负责：

1. 把 query 拆成 term
2. 把 term 统一成稳定格式
3. 把 rewrite 结果压成可记录的策略字符串

### 为什么要这样做

如果这些逻辑直接混在 `Rewrite` 里，代码会更难读，也更难单测。

尤其是 `formatRewriteStrategy` 很有价值，因为它把“这次到底纠正了什么、扩展了什么”直接编码进日志字符串里，后面查线上问题会方便很多。

### 它如何衔接下一步

现在改写器已经完整了，下一步要把它接入 HybridRetriever 的请求生命周期。

## 第 6 步：在混合检索请求上挂 rewrite 字段

### 目标

给 hybrid 检索请求增加承载 rewrite 结果的字段。

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
}
```

### 这段代码在做什么

这个结构体现在不只保存“检索参数”，还保存“rewrite 之后的状态”。

### 为什么要这样做

一个更简单的写法是只把 `query string` 一路往下传。

但这样后面会立刻遇到几个问题：

1. 你不知道原始 query 和最终 query 哪个是哪个
2. 你没法把 rewrite 信息传给日志层
3. dense route 和 handler 会拿不到 rewrite 元数据

所以这里专门把 `OriginalQuery`、`RewriteQuery`、`FinalQuery` 拆开，是为了让后面的链路保持清晰。

### 它如何衔接下一步

下一步要在这个请求对象上真正执行 rewrite，并把字段填好。

## 第 7 步：在检索前执行受控 rewrite

### 目标

让 HybridRetriever 在真正召回前先完成 query rewrite，同时保证没有 rewriter 时也能自然回退。

### 文件

`backend/internal/milvus/retrieval/hybrid_search.go`

### 完整代码

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
		req.RewriteQuery = ""
		req.RewriteStrategy = RewriteStrategyNone
		req.RewriteApplied = false
		return
	}

	result := rewriter.Rewrite(ctx, req.OriginalQuery)
	req.Query = req.OriginalQuery
	req.RewriteQuery = strings.TrimSpace(result.RewriteQuery)
	req.FinalQuery = strings.TrimSpace(result.FinalQuery)
	if req.FinalQuery == "" {
		req.FinalQuery = req.OriginalQuery
	}
	req.RewriteStrategy = formatRewriteStrategy(result)
	req.RewriteApplied = result.Applied && !strings.EqualFold(req.FinalQuery, req.OriginalQuery)
	if !req.RewriteApplied {
		req.RewriteQuery = ""
		req.FinalQuery = req.OriginalQuery
	}
}
```

调用位置是：

```go
req.applyControlledRewrite(ctx, h.queryRewriter)
```

### 这段代码在做什么

这段逻辑把 rewriter 产出的结果安全地写回 `HybridSearchRequest`：

1. 没有 rewriter 就直接退回原 query
2. 有 rewriter 就写入 rewrite 字段
3. 如果最终没有产生有效变化，就把 `RewriteApplied` 置为 false

### 为什么要这样做

这里的关键不是“能改写”，而是“能正确表达这次是否真的改写生效”。

因为后面日志分析时，`rewrite_applied=true` 和 `rewrite_strategy=rule_based;expanded=...` 会被拿来做效果回放。如果状态判断不严谨，后面的审计数据就会失真。

### 它如何衔接下一步

有了 `FinalQuery` 后，下一步就能让真正的 dense 检索使用它。

## 第 8 步：让 dense route 使用 final query 并附加 rewrite 元数据

### 目标

真正让 rewrite 结果影响检索，并把过程信息挂到每个文档上，方便后续提取。

### 文件

`backend/internal/milvus/retrieval/hybrid_search.go`

### 完整代码

```go
func (h *HybridRetriever) searchDense(ctx context.Context, req *HybridSearchRequest) ([]*schema.Document, error) {
	opts := &RetrieveOptions{
		Expr:             req.Expr,
		TopK:             req.TopK,
		Collection:       req.Collection,
		KBScope:          req.KBScope,
		ActiveGlobalKBID: req.KBID,
		RequestID:        req.RequestID,
		OriginalQuery:    req.OriginalQuery,
		RewriteQuery:     req.RewriteQuery,
		FinalQuery:       req.FinalQuery,
		RewriteStrategy:  req.RewriteStrategy,
		RewriteApplied:   req.RewriteApplied,
	}
	docs, err := h.retriever.RetrieveWithOptions(ctx, req.FinalQuery, opts)
	if err != nil {
		return nil, err
	}
	for _, doc := range docs {
		if doc.MetaData == nil {
			doc.MetaData = make(map[string]interface{})
		}
		doc.MetaData["route"] = routeDense
		doc.MetaData["dense_score"] = readDocScore(doc)
		attachRewriteMetadata(doc, req)
	}
	return docs, nil
}

func attachRewriteMetadata(doc *schema.Document, req *HybridSearchRequest) {
	if doc == nil || req == nil {
		return
	}
	if doc.MetaData == nil {
		doc.MetaData = make(map[string]interface{})
	}
	doc.MetaData["original_query"] = req.OriginalQuery
	doc.MetaData["rewrite_query"] = req.RewriteQuery
	doc.MetaData["final_query"] = req.FinalQuery
	doc.MetaData["rewrite_strategy"] = req.RewriteStrategy
	doc.MetaData["rewrite_applied"] = req.RewriteApplied
}
```

### 这段代码在做什么

这里发生了两件事：

1. dense 检索正式使用 `req.FinalQuery`
2. 每个文档都带上 rewrite 元数据

### 为什么要这样做

如果只是在内存里改了 query，但不把 rewrite 信息附到文档上，后面的 handler 就拿不到这些信息。

这会导致两个后果：

1. 审计日志里只能看到原始 query，看不到 rewrite 后的 query
2. 排查“为什么这次召回变了”时没有证据链

所以这一步本质上是在建立从检索层到日志层的数据传递通道。

### 它如何衔接下一步

现在 rewrite 信息已经挂在文档上了，下一步要在服务启动时把 rewriter 接进 HybridRetriever。

## 第 9 步：在启动阶段把 rewriter 注入 HybridRetriever

### 目标

让 rewrite 成为启动配置控制的一部分，而不是运行时到处手工判断。

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

服务启动时，它会：

1. 组装 hybrid retriever 的配置
2. 如果开了 `enable_query_rewrite`，就创建 `ControlledQueryRewriter`
3. 把 rewriter 注入 `HybridRetriever`

### 为什么要这样做

更简单的做法是每次请求来时再临时 `NewControlledQueryRewriter(...)`。

但那样会有两个不好的地方：

1. 生命周期不清晰，配置来源也不集中
2. 检索链路里会混入过多“如果开启就 new 一个对象”的装配逻辑

启动时装配对象，运行时只消费对象，是更稳定的工程形态。

### 它如何衔接下一步

到这里检索层已经打通了，下一步要把 rewrite 信息写进接口层的审计日志。

## 第 10 步：在 handler 层提取 rewrite 信息并落审计日志

### 目标

让这次 rewrite 不是“只在检索层内部生效”，而是能被业务接口、数据库日志和排查工具看见。

### 文件

`backend/api/handler/kb/handler.go`

### 完整代码

```go
func extractFinalQuery(docs []*schema.Document) string {
	return getStringMetadataFromDocs(docs, "final_query")
}

func extractRewriteQuery(docs []*schema.Document) string {
	return getStringMetadataFromDocs(docs, "rewrite_query")
}

func extractRewriteStrategy(docs []*schema.Document) string {
	return getStringMetadataFromDocs(docs, "rewrite_strategy")
}

func extractRewriteApplied(docs []*schema.Document) bool {
	for _, doc := range docs {
		if doc == nil || doc.MetaData == nil {
			continue
		}
		if value, ok := doc.MetaData["rewrite_applied"]; ok {
			switch v := value.(type) {
			case bool:
				return v
			case string:
				return strings.EqualFold(strings.TrimSpace(v), "true")
			}
		}
	}
	return false
}

func getStringMetadataFromDocs(docs []*schema.Document, key string) string {
	for _, doc := range docs {
		if value := getStringMetadata(doc.MetaData, key); value != "" {
			return value
		}
	}
	return ""
}
```

审计日志组装是：

```go
retrieveLog := &model.KBRetrieveLog{
	RequestID:        requestID,
	UserID:           userID,
	KBIDs:            formatKBIDs(kbIDs),
	Query:            req.Query,
	FinalQuery:       firstNonEmptyString(extractFinalQuery(docs), req.Query),
	Expr:             expr,
	TopK:             topK,
	Rewrite:          extractRewriteQuery(docs),
	RewriteStrategy:  extractRewriteStrategy(docs),
	RewriteApplied:   extractRewriteApplied(docs),
	Routes:           resolveRetrieveRoutes(useHybrid),
	Collection:       collection,
	RetrieverVersion: "v1",
	FinalCount:       len(items),
	TruncatedCount:   searchMetrics.TruncatedCount,
	ResultStatus:     resultStatus,
	ErrorCode:        "",
	ErrorMsg:         "",
	EmbeddingMs:      searchMetrics.EmbeddingMs,
	SearchMs:         searchMetrics.SearchMs,
	PostprocessMs:    searchMetrics.PostprocessMs,
	DurationMs:       durationMs,
	TimeoutMs:        retrieveTimeout.Milliseconds(),
}
```

审计日志打印是：

```go
log.Printf(
	"[KB Retrieve] request_id=%s query=%q final_query=%q rewrite=%q rewrite_strategy=%q rewrite_applied=%t user_id=%d kb_ids=%v kb_scope=%q expr=%q topk=%d routes=%q final_count=%d hit_count=%d truncated_count=%d duration_ms=%d embedding_ms=%d search_ms=%d postprocess_ms=%d timeout_ms=%d result_status=%s",
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
```

### 这段代码在做什么

它从文档元数据里把 rewrite 信息提出来，然后写到接口级别的审计日志里。

### 为什么要这样做

很多系统的问题不是“功能没实现”，而是“实现了但无法观察”。

这里如果不写审计日志，后面很难回答这些问题：

1. 这次命中提升是不是 rewrite 带来的
2. 某个 query 为什么被改写了
3. 哪类 query 最常触发黑名单
4. rewrite 是否在某些请求上引入了副作用

所以这一步是在补可观测性，而不是在补装饰性日志。

### 它如何衔接下一步

日志要落库，下一步自然要看数据库模型怎么设计。

## 第 11 步：给检索审计表增加 rewrite 字段

### 目标

让 rewrite 的关键数据能持久化保存，不只存在于瞬时日志里。

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

它把 rewrite 相关信息和一次检索请求的其他上下文一起落进 `kb_retrieve_log` 表。

### 为什么要这样做

只看控制台日志只能临时排障，无法做长期分析。把这些字段存数据库后，后面就能做：

1. rewrite 生效率统计
2. 不同 rewrite 策略的命中效果对比
3. 问题回放
4. 灰度期间的线上效果复盘

### 它如何衔接下一步

最后还差一块，就是测试。没有测试，这些规则将来很容易被改坏。

## 第 12 步：用测试把 rewrite 的核心行为钉住

### 目标

确保缩写扩展、上限控制、拼写修正、黑名单和超时回退这些关键行为以后不会被无意破坏。

### 文件

`backend/internal/milvus/retrieval/rewrite_test.go`

### 完整代码

```go
func TestControlledQueryRewriterExpandsAbbreviationAndAlias(t *testing.T) {
	rewriter := NewControlledQueryRewriter(&QueryRewriterConfig{MaxExpansions: 3})

	result := rewriter.Rewrite(context.Background(), "jvm gc")

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

	result := rewriter.Rewrite(context.Background(), "mq rpc")

	if len(result.ExpansionTerms) != 1 {
		t.Fatalf("expansions = %d, want 1", len(result.ExpansionTerms))
	}
}

func TestControlledQueryRewriterCorrectsTypos(t *testing.T) {
	rewriter := NewControlledQueryRewriter(nil)

	result := rewriter.Rewrite(context.Background(), "sprinboot interview")

	if !result.Applied {
		t.Fatalf("expected typo correction to apply")
	}
	if !strings.Contains(result.FinalQuery, "springboot") {
		t.Fatalf("final query = %q, want springboot correction", result.FinalQuery)
	}
}

func TestControlledQueryRewriterBlacklistSkipsRewrite(t *testing.T) {
	rewriter := NewControlledQueryRewriter(nil)

	result := rewriter.Rewrite(context.Background(), `site:example.com "jvm"`)

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

	result := rewriter.Rewrite(ctx, "jvm")

	if result.Applied {
		t.Fatalf("expected canceled context to skip rewrite")
	}
	if result.Strategy != RewriteStrategyTimeout {
		t.Fatalf("strategy = %q, want %q", result.Strategy, RewriteStrategyTimeout)
	}
}

func TestHybridSearchRequestApplyControlledRewriteFallback(t *testing.T) {
	req := &HybridSearchRequest{
		Query:         "jvm",
		OriginalQuery: "jvm",
	}

	req.applyControlledRewrite(context.Background(), NewControlledQueryRewriter(nil))

	if !req.RewriteApplied {
		t.Fatalf("expected request rewrite to be applied")
	}
	if req.FinalQuery == req.OriginalQuery {
		t.Fatalf("expected final query to differ from original query")
	}
}
```

### 这段代码在做什么

这些测试覆盖的是 rewrite 最容易出问题的边界：

1. 能不能正确扩展
2. 会不会无限扩展
3. 拼写纠正有没有生效
4. 不该改的 query 会不会被误改
5. 超时后能不能安全回退
6. HybridSearchRequest 能不能正确接住 rewrite 结果

### 为什么要这样做

rewrite 这种功能特别容易“越改规则越多，最后没人敢动”。测试的价值就在于，把关键行为变成明确契约。

### 它如何衔接下一步

代码到这里已经完整，最后就要说怎么验收。

## 如何验证

### 1. 跑单元测试

在 `backend` 目录执行：

```powershell
go test ./internal/milvus/retrieval -run Rewrite
```

如果你想把 hybrid 相关一起跑掉，也可以直接：

```powershell
go test ./internal/milvus/retrieval
```

### 2. 打开功能开关做接口验证

把下面两个开关打开：

1. `rag.feature_flags.enable_hybrid_retrieval=true`
2. `rag.feature_flags.enable_query_rewrite=true`

然后确认：

1. `rewrite_timeout_ms` 大于 0
2. `rewrite_max_expansions` 大于 0

### 3. 用典型 query 验证行为

建议至少测这几类：

1. `jvm gc`
2. `mq rpc`
3. `sprinboot interview`
4. `site:example.com "jvm"`

你应该重点观察：

1. `final_query` 是否被扩展
2. `rewrite_strategy` 是否符合预期
3. `rewrite_applied` 是否正确
4. 黑名单 query 是否保持原样

### 4. 看检索审计日志

打开 `enable_retrieve_audit` 后，查看 `[KB Retrieve]` 日志，重点看这些字段：

1. `query`
2. `final_query`
3. `rewrite`
4. `rewrite_strategy`
5. `rewrite_applied`
6. `routes`
7. `result_status`

### 5. 看数据库审计表

检查 `kb_retrieve_log` 表里是否已经写入：

1. `rewrite`
2. `rewrite_strategy`
3. `rewrite_applied`
4. `final_query`

如果这些字段是空的，通常说明问题在两种地方：

1. HybridRetriever 没有挂 rewriter
2. 文档元数据没有正确透传到 handler

## 取舍与后续优化

### 这版实现刻意优化了什么

这版实现明显偏向“稳定和可观测”：

1. 规则型 rewrite 可解释
2. 可以随时关掉
3. 有扩展上限
4. 有黑名单保护
5. 有日志和数据库审计

### 这版实现故意没有解决什么

它还没有解决下面这些更复杂的问题：

1. 没有引入模型辅助 rewrite
2. 没有做按领域动态加载术语表
3. 没有对不同 route 分别做 rewrite 策略
4. 没有做 rewrite 增益的离线评测闭环

### 后续最自然的演进方向

如果后面要继续做，可以按这个顺序推进：

1. 给术语表做外部配置，而不是继续只写在代码里
2. 增加按知识库或行业定制的 alias 词典
3. 统计 `rewrite_applied=true` 请求的命中收益
4. 在保留原 query 的前提下，引入受控的模型辅助 rewrite

这篇教程最重要的不是让你背住每个 map 里有哪些术语，而是让你建立一个正确心智模型：

L3 查询改写不是“改写一下字符串”这么简单，它实际上是在检索链路里增加了一个受控增强层。这个增强层必须同时满足可配置、可回退、可观察、可测试，才适合上线。
