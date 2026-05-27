# 基线冻结、策略开关与评测集扩展实现教程

## 1. 背景

这组能力解决的不是一个点，而是一整条“可控演进”链路：

1. `基线冻结` 负责把某个阶段当前可运行、可回滚的参数状态固化下来，避免后续继续开发时把基线悄悄改掉。
2. `策略开关` 负责把高级检索能力拆成独立布尔开关，让我们可以一项一项打开，而不是一次性把全部实验逻辑推到线上。
3. `评测集扩展` 负责把离线评测从“只能跑少量固定样本”升级成“能按场景扩充、能带版本、还能兼容旧格式”的机制。

如果只做检索功能，不做这三件事，团队很快会遇到三个典型问题：

1. 不知道“现在效果变好”到底是因为哪个策略生效了。
2. 回归失败时，不知道应该回滚到哪一版参数。
3. 评测样本越加越多以后，旧脚本、旧报告、旧命令开始互相不兼容。

这篇教程要讲清楚的，就是这三件事在当前仓库里是怎样连起来的。

## 2. 这篇教程会做什么

看完之后，你应该能自己复现下面这条链路：

1. 在配置层定义和校验 RAG 策略开关。
2. 用环境变量在运行时覆盖开关和阈值。
3. 在服务启动时自动写出 Phase 1 与 Phase 2 的基线快照。
4. 用策略 profile 把不同实验组合表达成可比较的离线方案。
5. 用新版本评测集 bundle 结构扩容样本，同时兼容旧的数组格式。
6. 运行 `retrieval-eval`，输出 `baseline vs candidate` 报告，并用 gate 判断是否通过。

这次实现主要涉及这些文件：

1. `backend/internal/config/config.go`
2. `backend/internal/config/config_rag_test.go`
3. `backend/config.example.yaml`
4. `backend/internal/milvus/evaluation/types.go`
5. `backend/internal/milvus/evaluation/io.go`
6. `backend/internal/milvus/evaluation/profiles.go`
7. `backend/internal/milvus/evaluation/gate.go`
8. `backend/internal/milvus/evaluation/runner.go`
9. `backend/cmd/retrieval-eval/main.go`
10. `backend/scripts/evaluation/dataset.json`
11. `backend/scripts/evaluation/retrieval_strategy_profiles.example.json`
12. `backend/scripts/evaluation/retrieval_gate_thresholds.example.json`
13. `backend/scripts/evaluation/evaluate.py`

## 3. 需要先理解的术语

### 3.1 基线冻结

你可以先把“基线冻结”理解成“把当前这一版参数状态写成一份不会被后续启动覆盖的快照”。

这里冻结的不是 Git 分支，也不是数据库镜像，而是：

1. 当前有哪些 feature flag 打开了。
2. 当前阈值和 Phase 2/Phase 3 参数是什么。
3. 当前默认拿哪套评测集、哪组 profile 作为比较基线。

这样做的原因很直接：后面你继续调参时，必须始终能回答“我相对的是哪一个基线”。

### 3.2 策略开关

策略开关就是 `true/false` 的布尔开关，用来决定某个检索能力是否启用。

例如：

1. `enable_parent_child_retrieval` 表示是否打开父子块补全文。
2. `enable_domain_terms` 表示是否打开领域术语扩展。
3. `enable_model_assisted_rewrite` 表示是否打开模型辅助改写。

它的核心作用不是“少写 if”，而是把复杂能力拆成独立实验单元，方便灰度、回滚和归因。

### 3.3 策略 Profile

Profile 可以理解成“一组开关和参数的命名组合”。

比如：

1. `phase2_baseline` 表示当前 Phase 2 参考方案。
2. `parent_child+advanced_rewrite` 表示一个更激进的候选方案。

离线评测不会直接说“把第 7、8、11 个开关打开”，而是说“跑这个 profile”。这样报告更稳定，也更容易对齐团队沟通。

### 3.4 评测集 Bundle

Bundle 就是“给数据集加一层外壳”。

旧格式只有一个数组：

```json
[
  {
    "id": "case-1",
    "query": "goroutine",
    "top_k": 5,
    "relevant_ids": ["chunk-1"]
  }
]
```

新格式则是一个对象，除了 `cases` 之外，还带 `dataset_version` 和 `description`。这能解决两个老问题：

1. 报告里终于能知道这次跑的是哪一版数据集。
2. 数据集扩容后，场景说明不需要散落在别的文档里。

### 3.5 Gate

Gate 就是“自动判定这次候选策略有没有资格继续往前走”的门槛。

比如：

1. `Recall@K` 至少要比基线高 `0.08`。
2. `P95 latency` 回退比例不能超过 `20%`。

这一步非常关键，因为它把“感觉还不错”变成了“有明确定量约束”。

## 4. 整体流程

先看全局，再看代码会更容易。

1. 服务启动时，`LoadConfig` 先读主配置，再读环境 overlay，再吃环境变量覆盖。
2. 配置装载完成后，系统会应用默认值、计算配置版本、写出基线快照，并打印一份 RAG 策略摘要日志。
3. 离线评测启动时，`retrieval-eval` 会加载数据集 bundle、加载策略 profile、加载 gate 阈值。
4. Runner 会让每个 profile 都跑完整个数据集，收集 `Recall@K / MRR / nDCG / Citation Accuracy / P50 / P95`。
5. Runner 再从结果里解析出 baseline 和 candidate，生成 comparison、contribution 和 gate 结果。
6. 最终报告落成 JSON 和 Markdown；如果 gate 不通过，命令会以退出码 `2` 结束。

你可以把这条链路理解成三层：

1. `配置层` 决定系统“有哪些能力”和“默认怎么开”。
2. `实验层` 决定“把哪些能力打包成一套 profile 去比较”。
3. `数据层` 决定“我们拿什么样的样本证明候选方案真的更好”。

## 5. 分步实现

## 第一步：把可控能力正式建模到配置结构里

### 目标

先把“有哪些开关、它们属于哪一层参数”建成明确的数据结构。没有这一步，后面环境覆盖、日志快照、离线 profile 都无从谈起。

### 文件

`backend/internal/config/config.go`

### 完整代码

```go
type RAGConfig struct {
	Enabled      bool             `yaml:"enabled"`
	Environment  string           `yaml:"environment"`
	FeatureFlags RAGFeatureFlags  `yaml:"feature_flags"`
	Thresholds   RAGThresholds    `yaml:"thresholds"`
	Phase2       RAGPhase2Config  `yaml:"phase2"`
	Phase3       RAGPhase3Config  `yaml:"phase3"`
	Release      RAGReleaseConfig `yaml:"release"`
}

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

type RAGThresholds struct {
	MaxRetryCount     int `yaml:"max_retry_count"`
	RetryBackoffMS    int `yaml:"retry_backoff_ms"`
	RetrieveTimeoutMS int `yaml:"retrieve_timeout_ms"`
	UserQPSLimit      int `yaml:"user_qps_limit"`
}
```

### 这段代码在做什么

这一步把 RAG 的控制面拆成了 4 个区：

1. `FeatureFlags` 放布尔开关，回答“这个能力开不开”。
2. `Thresholds` 放安全和运行阈值，回答“系统容忍范围是多少”。
3. `Phase2` 和 `Phase3` 放更细的算法参数，回答“开了以后具体怎么跑”。
4. `Release` 放灰度发布参数，回答“谁能先用、按什么节奏放量”。

### 为什么要这样写

更简单的写法是把所有字段都平铺到 `rag:` 下，但那样会很快失控：

1. 开关和参数混在一起，不容易看出哪些字段是运行态控制，哪些字段是算法调参。
2. 快照里不好按模块比较差异。
3. 新人看到配置文件时，很难快速建立心智模型。

这套分层本质上是在给后面的“快照、覆盖、评测”预留稳定形状。

### 它如何衔接下一步

有了结构体之后，下一步才能把这些字段真正暴露到 `config.example.yaml`。

### 文件

`backend/config.example.yaml`

### 完整代码

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

这份 YAML 是“人类编辑入口”。也就是说，结构体是程序看的，YAML 是工程师改的。

尤其要注意三点：

1. 默认把高级能力全部关掉，只保留 `enable_retrieve_audit: true` 这种低风险能力。
2. Phase 2 和 Phase 3 的参数即使暂时不开，也先有默认值。
3. `release` 单独存在，说明“算法开关”和“放量策略”不是一回事。

### 为什么要这样写

如果只在代码里定义字段，不在示例配置里显式展示，团队成员很难知道：

1. 这个能力有没有默认值。
2. 它的值域大概是什么。
3. 它在配置树里的层级应该放哪。

这份示例配置其实就是一份“可运行合同”。

### 它如何衔接下一步

接下来系统需要把 YAML、环境 overlay、环境变量覆盖合并成最终运行配置。

## 第二步：把策略开关做成可覆盖、可校验、可审计的启动链路

### 目标

这一层要解决的是：同一份代码在不同环境里，怎样安全地打开或关闭策略，而且出问题时能快速看出当时实际生效的是哪套参数。

### 文件

`backend/internal/config/config.go`

### 完整代码

```go
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
	return nil
}
```

### 这段代码在做什么

这段函数把“运行时控制面”完全暴露出来了。也就是说，YAML 只是默认值，真正上线时还可以通过环境变量覆盖。

最重要的不是某一个变量名，而是这套覆盖策略：

1. 先读布尔开关。
2. 再读阈值和 Phase 2 参数。
3. 再读 Phase 3 参数。
4. 最后读 release 放量配置。

### 为什么要这样写

如果只依赖 YAML，灰度环境想临时试一个开关，就必须改配置文件并发版，成本很高。

而环境变量覆盖有三个现实好处：

1. 容器平台非常容易注入。
2. 回滚时只要改变量，不一定要重新构建镜像。
3. 配合快照和日志，事后还能追出“当时实际跑的是哪一套值”。

### 它如何衔接下一步

覆盖之后不能直接相信配置，还必须做 fail-fast 校验。

### 文件

`backend/internal/config/config.go`

### 完整代码

```go
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
```

### 这段代码在做什么

这一步是典型的 fail-fast。也就是系统宁可在启动阶段直接报错，也不要带着无效组合运行到线上再出事故。

例如：

1. 开了 `hybrid`，就必须有合法的 dense/sparse 权重。
2. 开了 `dynamic_topk`，就必须保证 `min_topk <= max_topk <= candidate_topk`。
3. 开了 `citation_consistency`，就必须给出阈值和版本号。

### 为什么要这样写

如果做“宽松容错”，系统会出现一种很麻烦的状态：开关看起来打开了，但某些关键参数其实无效，结果功能半生不熟地运行。

这类问题最难排查，因为日志里只会看到“效果变差了”，而不是“配置根本不合法”。

### 它如何衔接下一步

配置已经能安全装载后，接下来就要把它固化成可追溯的快照。

## 第三步：把基线冻结成首次启动即落盘的快照

### 目标

这一层真正实现“基线冻结”。它要确保：只要配置第一次被加载，对应阶段的快照就会写到 `docs/baseline/...`，后续启动不会覆盖。

### 文件

`backend/internal/config/config.go`

### 完整代码

```go
func (c *Config) buildRAGStrategyDigest() string {
	if c == nil {
		return "unknown"
	}
	payload := map[string]interface{}{
		"enabled":    c.RAG.Enabled,
		"env":        c.RAG.Environment,
		"flags":      c.RAG.FeatureFlags,
		"thresholds": c.RAG.Thresholds,
		"phase2":     c.RAG.Phase2,
		"phase3":     c.RAG.Phase3,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "unknown"
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8])
}

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
			"notes":              "请在完成 Phase 1 基线评测后补齐该字段",
		},
		"evaluation_report": map[string]interface{}{
			"dataset_version": "",
			"report_path":     "",
			"summary":         "请在完成离线评测后补齐该字段",
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
```

### 这段代码在做什么

这里有两个很重要的设计点：

1. 快照只在文件不存在时写入，所以它天然具有“首写冻结”的语义。
2. 快照不只存配置，还预埋了评测路径、基线 profile、候选 profile 和回滚合同。

也就是说，这不是一份“为了好看留个 JSON”，而是一份后面要参与比较、回滚和验收的工程资产。

### 为什么要这样写

更简单的做法是每次启动都重写快照。但那样的坏处非常明显：

1. 你以为自己在对比固定基线，其实基线已经被后续改动覆盖了。
2. 事故追查时无法知道第一次上线时到底是什么参数。
3. 离线评测报告和运行参数之间没有稳定锚点。

`strategy_digest` 也很关键。你可以把它理解成“当前策略状态的短指纹”，用来快速发现两次配置是否已经发生结构性变化。

### 它如何衔接下一步

快照只解决“冻结”，还要有日志解决“运行时可观察”。

### 文件

`backend/internal/config/config.go`

### 完整代码

```go
func (c *Config) LogRAGSnapshot() {
	if c == nil {
		return
	}
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
		strings.Join(c.RAG.Release.InternalRoles, ","),
		c.RAG.Release.CanaryPercent,
		c.RAG.Release.BatchPercent,
		len(c.RAG.Release.UserAllowlist),
		maskAddress(c.Milvus.Address),
		c.Milvus.DatabaseName,
		c.Milvus.CollectionName,
	)
}
```

### 这段代码在做什么

它把“最终生效配置”压缩成一条结构化日志。

这条日志的价值在于：

1. 快速确认某个开关到底有没有被环境变量覆盖。
2. 对照 `strategy_digest` 看线上和快照是否仍是同一套策略。
3. 排查“为什么某个 profile 线下能过、线上效果却不一样”。

### 为什么要这样写

快照是落盘证据，日志是运行证据。两者都要有，排查才闭环。

### 它如何衔接下一步

到这里，配置层已经完成。下一步要把这些能力映射为离线可对比的策略 profile。

## 第四步：把策略开关打包成可比较的评测 Profile

### 目标

这一层不是直接跑线上逻辑，而是把“候选策略组合”明确建模成离线实验对象。

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

### 这段代码在做什么

这个结构体把“一个实验组合需要的全部开关和参数”收口到了一个对象里。

注意它和 `RAGFeatureFlags` 的关系：

1. `RAGFeatureFlags` 是运行配置层。
2. `StrategyProfile` 是离线实验层。

它们字段很像，但角色不同。前者是“服务默认怎么跑”，后者是“评测时我想模拟哪种组合”。

### 为什么要这样写

如果离线评测直接复用运行配置，就会出现两个问题：

1. 线下实验过于依赖当前环境，复现性很差。
2. 两个策略对比时，很难精确表达“到底只差哪几个开关”。

Profile 就是在运行配置之外，再提供一层可复现的实验声明。

### 它如何衔接下一步

有了结构体后，就要给出仓库默认的实验组。

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

### 这段代码在做什么

这组默认 profile 很像一条阶梯：

1. 先从 `phase2_baseline` 开始。
2. 再加 `parent_child`。
3. 再看 `strategic_topk`。
4. 再看 `refusal`。
5. 最后把更激进的 rewrite 能力一起打开。

报告里的 `contribution` 就是按这个顺序做相邻比较的。

### 为什么要这样写

如果只保留 `baseline` 和一个“大杂烩 candidate”，你最多只能回答“最终结果有没有变好”，但回答不了“到底是哪一步带来了收益或副作用”。

这组阶梯 profile 解决的，正是“收益归因”问题。

### 它如何衔接下一步

Profile 只是声明，真正运行时还要把它翻译成搜索器实例。

## 第五步：把 Profile 转成真实搜索行为

### 目标

这一层负责把 profile 里的开关和参数真正喂给检索实现，确保离线评测不是“纸上实验”。

### 文件

`backend/cmd/retrieval-eval/main.go`

### 完整代码

```go
func buildSearcher(cfg *config.Config, manager *milvus.MilvusManager, profile evaluation.StrategyProfile, collection string) (evaluation.Searcher, error) {
	searcher := &retrievalSearcher{
		profile:    profile,
		retriever:  manager.GetRetrieverService(),
		collection: collection,
		timeout:    time.Duration(cfg.RAG.Thresholds.RetrieveTimeoutMS) * time.Millisecond,
	}
	if searcher.timeout <= 0 {
		searcher.timeout = 3 * time.Second
	}

	if strings.EqualFold(profile.Mode, "hybrid") {
		candidateTopK := cfg.RAG.Phase2.CandidateTopK
		if candidateTopK <= 0 {
			candidateTopK = cfg.Milvus.TopK * 2
		}
		if profile.CandidateTopK > 0 {
			candidateTopK = profile.CandidateTopK
		}
		denseWeight := cfg.RAG.Phase2.HybridDenseWeight
		if profile.DenseWeight > 0 {
			denseWeight = profile.DenseWeight
		}
		sparseWeight := cfg.RAG.Phase2.HybridSparseWeight
		if profile.SparseWeight > 0 {
			sparseWeight = profile.SparseWeight
		}
		hybridConfig := &retrieval.HybridRetrieverConfig{
			CandidateTopK: candidateTopK,
			DenseWeight:   denseWeight,
			SparseWeight:  sparseWeight,
			SparseConfig: &retrieval.SparseRetrieverConfig{
				DefaultTopK: candidateTopK,
			},
			DynamicTopK: retrieval.DynamicTopKConfig{
				Enabled:         profile.EnableDynamicTopK,
				MinTopK:         fallbackInt(profile.MinTopK, cfg.RAG.Phase2.MinTopK),
				MaxTopK:         fallbackInt(profile.MaxTopK, cfg.RAG.Phase2.MaxTopK),
				TokenBudget:     fallbackInt(profile.TokenBudget, cfg.RAG.Phase2.TokenBudget),
				MinAnswerChunks: fallbackInt(profile.MinAnswerChunks, cfg.RAG.Phase2.MinAnswerChunks),
			},
		}
		hybridConfig.RerankerImpl = retrieval.NewJaccardReranker(&retrieval.JaccardRerankerConfig{
			TopK:      candidateTopK,
			ModelName: retrieval.DefaultRerankModelJaccardV1,
			Version:   retrieval.DefaultRerankVersion,
		})
		if profile.EnableAdvancedRerank {
			timeout := time.Duration(fallbackInt(profile.RerankTimeoutMS, cfg.RAG.Phase2.RerankTimeoutMS)) * time.Millisecond
			modelName := strings.TrimSpace(profile.RerankModel)
			if modelName == "" {
				modelName = cfg.RAG.Phase2.RerankModel
			}
			hybridConfig.RerankerImpl = retrieval.NewConfigurableReranker(
				modelName,
				timeout,
				retrieval.NewJaccardReranker(&retrieval.JaccardRerankerConfig{
					TopK:      candidateTopK,
					ModelName: modelName,
					Version:   modelName,
				}),
				hybridConfig.RerankerImpl,
			)
		}
		if profile.EnableQueryRewrite {
			hybridConfig.QueryRewriter = retrieval.NewControlledQueryRewriter(&retrieval.QueryRewriterConfig{
				MaxExpansions: fallbackInt(profile.RewriteMaxExpansions, cfg.RAG.Phase2.RewriteMaxExpansions),
			})
		}
		hybridRetriever, err := retrieval.NewHybridRetriever(manager.GetRetrieverService(), hybridConfig)
		if err != nil {
			return nil, err
		}
		searcher.hybrid = hybridRetriever
	}
	return searcher, nil
}
```

### 这段代码在做什么

这段逻辑的核心是“以 profile 为主，以全局配置为兜底”。

比如：

1. `candidateTopK` 先看 profile 有没有显式指定。
2. 没指定就回退到全局 `cfg.RAG.Phase2.CandidateTopK`。
3. 再不行就用 `cfg.Milvus.TopK * 2` 兜底。

这种写法让离线实验具备两个性质：

1. 默认情况下和当前系统配置保持接近。
2. 需要做局部实验时，又能在不改全局配置的前提下覆盖单个参数。

### 为什么要这样写

如果 profile 必须把所有参数都填满，会非常笨重；但如果 profile 不能覆盖关键参数，又无法表达真正的实验差异。

所以这里不是“完全复制一套配置”，而是“只覆盖实验相关项，其他值继承全局配置”。

### 它如何衔接下一步

有了搜索器，接下来就轮到数据集。没有能扩展的数据集，再好的 profile 也无法证明自己。

## 第六步：把评测集从旧数组格式升级为带版本的 Bundle

### 目标

这一层要解决两个问题：

1. 数据集扩容后，怎样给样本分场景、加版本、加描述。
2. 已经存在的旧数组格式，怎样不被新实现直接打断。

### 文件

`backend/internal/milvus/evaluation/types.go`

### 完整代码

```go
type CitationTarget struct {
	DocumentID uint64 `json:"document_id,omitempty"`
	ChunkID    string `json:"chunk_id,omitempty"`
	FileName   string `json:"file_name,omitempty"`
}

type DatasetBundle struct {
	DatasetVersion string        `json:"dataset_version,omitempty"`
	Description    string        `json:"description,omitempty"`
	Cases          []DatasetCase `json:"cases"`
}

type DatasetCase struct {
	ID               string           `json:"id"`
	Question         string           `json:"question,omitempty"`
	Query            string           `json:"query"`
	Context          string           `json:"context,omitempty"`
	GroundTruth      string           `json:"ground_truth,omitempty"`
	TopK             int              `json:"top_k"`
	RelevantIDs      []string         `json:"relevant_ids"`
	CitationTargets  []CitationTarget `json:"citation_targets,omitempty"`
	QueryType        string           `json:"query_type,omitempty"`
	Scenario         string           `json:"scenario,omitempty"`
	ExpectedBehavior string           `json:"expected_behavior,omitempty"`
	Tags             []string         `json:"tags,omitempty"`
	KBIDs            []uint64         `json:"kb_ids,omitempty"`
	Collection       string           `json:"collection,omitempty"`
	Notes            string           `json:"notes,omitempty"`
}
```

### 这段代码在做什么

`DatasetCase` 已经不仅仅是“query + relevant_ids”了，它还显式表达了：

1. 这是哪种查询类型。
2. 这是哪个实验场景。
3. 它期望命中的 citation 目标是什么。
4. 是否限定某个知识库或 collection。

这让同一个数据集既能服务检索评测，也能继续服务旧的 Ragas 评测模式。

### 为什么要这样写

如果只保留最简结构，后面你会很难回答这些问题：

1. 这次 Recall 下降是哪个场景掉的。
2. parent-child 到底该在长文场景里体现收益，还是在实体问答里体现收益。
3. out-of-scope 样本到底是不是该命中。

场景字段本质上是在给后续“拆分报告”和“按能力验收”铺路。

### 它如何衔接下一步

结构体定义好了，接下来真正解决兼容问题的是加载器。

### 文件

`backend/internal/milvus/evaluation/io.go`

### 完整代码

```go
func LoadDataset(path string) ([]DatasetCase, error) {
	bundle, err := LoadDatasetBundle(path)
	if err != nil {
		return nil, err
	}
	return bundle.Cases, nil
}

func LoadDatasetBundle(path string) (DatasetBundle, error) {
	var bundle DatasetBundle
	data, err := os.ReadFile(path)
	if err != nil {
		return bundle, err
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return bundle, fmt.Errorf("parse dataset %s: empty payload", path)
	}
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &bundle.Cases); err != nil {
			return bundle, fmt.Errorf("parse dataset %s: %w", path, err)
		}
		bundle.DatasetVersion = "legacy-array"
		return bundle, nil
	}
	if err := json.Unmarshal(trimmed, &bundle); err != nil {
		return bundle, fmt.Errorf("parse dataset %s: %w", path, err)
	}
	if bundle.DatasetVersion == "" {
		bundle.DatasetVersion = "unspecified"
	}
	return bundle, nil
}
```

### 这段代码在做什么

加载器用了一个非常实用的兼容策略：

1. 如果文件首字符是 `[`，就按旧数组格式解析。
2. 如果不是，就按新 bundle 对象解析。
3. 旧格式自动补一个 `legacy-array` 版本名。
4. 新格式如果没写版本，也会补成 `unspecified`。

### 为什么要这样写

更简单的做法是“一刀切，只支持新格式”。但那样会让已有脚本、数据文件、调试流程全部一起报错。

当前实现选择的是更稳妥的迁移路径：

1. 新能力直接支持。
2. 旧资产先不打断。
3. 报告里还能看出旧数据是不是还没升级。

### 它如何衔接下一步

接下来我们看仓库里的真实数据集长什么样。

### 文件

`backend/scripts/evaluation/dataset.json`

### 完整代码

```json
{
  "dataset_version": "phase3-l0-v1",
  "description": "Phase 3 L0 retrieval regression dataset covering baseline, long-document, multi-evidence, insufficient-evidence, abbreviation, domain-term, and colloquial rewrite scenarios.",
  "cases": [
    {
      "id": "entity-go-goroutine",
      "question": "Go 语言的 goroutine 是什么？",
      "query": "goroutine 是什么",
      "context": "goroutine 是 Go 语言中的轻量级线程，由 Go runtime 调度。",
      "ground_truth": "goroutine 是 Go 的并发执行单元，比操作系统线程更轻量，由 Go runtime 调度。",
      "top_k": 5,
      "relevant_ids": [
        "kb_1_doc_1_chunk_0_1779523924191899824"
      ],
      "citation_targets": [
        {
          "chunk_id": "kb_1_doc_1_chunk_0_1779523924191899824"
        }
      ],
      "query_type": "entity",
      "scenario": "phase2_baseline",
      "expected_behavior": "hit_precise_chunk",
      "tags": ["go", "goroutine", "entity"]
    },
    {
      "id": "longdoc-go-runtime-scheduling",
      "question": "Go runtime 为什么能把 goroutine 调度得比线程更轻量？",
      "query": "Go runtime 调度 goroutine 为什么更轻量",
      "context": "该问题用于覆盖长文档和上下文补全收益场景。",
      "ground_truth": "因为 goroutine 由 Go runtime 调度和管理，开销通常比操作系统线程更小。",
      "top_k": 5,
      "relevant_ids": [
        "kb_1_doc_1_chunk_0_1779523924191899824"
      ],
      "citation_targets": [
        {
          "chunk_id": "kb_1_doc_1_chunk_0_1779523924191899824"
        }
      ],
      "query_type": "long_document",
      "scenario": "parent_child_gain",
      "expected_behavior": "future_parent_child_should_improve_context_completeness",
      "tags": ["go", "runtime", "long_document", "phase3"]
    },
    {
      "id": "entity-react-component",
      "question": "React 中什么是组件？",
      "query": "React 组件",
      "context": "React 组件是 UI 的基础构建块，可以是函数组件或类组件。",
      "ground_truth": "React 组件是可复用的 UI 单元，接收 props 并返回 JSX 描述的界面。",
      "top_k": 5,
      "relevant_ids": [
        "kb_1_doc_2_chunk_0_1779523936358216873"
      ],
      "citation_targets": [
        {
          "chunk_id": "kb_1_doc_2_chunk_0_1779523936358216873"
        }
      ],
      "query_type": "entity",
      "scenario": "phase2_baseline",
      "expected_behavior": "hit_precise_chunk",
      "tags": ["react", "component", "entity"]
    },
    {
      "id": "entity-js-promise",
      "question": "JavaScript 的 Promise 是什么？",
      "query": "JavaScript Promise 异步",
      "context": "Promise 是 JavaScript 处理异步操作的对象，有 pending、fulfilled、rejected 三种状态。",
      "ground_truth": "Promise 用于异步编程，通过 then/catch 链式调用或 async/await 处理异步结果。",
      "top_k": 5,
      "relevant_ids": [
        "kb_1_doc_4_chunk_0_1779731716268688925"
      ],
      "citation_targets": [
        {
          "chunk_id": "kb_1_doc_4_chunk_0_1779731716268688925"
        }
      ],
      "query_type": "entity",
      "scenario": "phase2_baseline",
      "expected_behavior": "hit_precise_chunk",
      "tags": ["javascript", "promise", "async", "entity"]
    },
    {
      "id": "multi-evidence-react-promise",
      "question": "React 组件和 JavaScript Promise 分别解决什么问题？",
      "query": "React 组件 Promise 分别解决什么问题",
      "context": "该样本用于覆盖多段证据整合能力。",
      "ground_truth": "React 组件解决 UI 复用与拆分问题，Promise 解决异步流程表达与结果处理问题。",
      "top_k": 5,
      "relevant_ids": [
        "kb_1_doc_2_chunk_0_1779523936358216873",
        "kb_1_doc_4_chunk_0_1779731716268688925"
      ],
      "citation_targets": [
        {
          "chunk_id": "kb_1_doc_2_chunk_0_1779523936358216873"
        },
        {
          "chunk_id": "kb_1_doc_4_chunk_0_1779731716268688925"
        }
      ],
      "query_type": "multi_hop",
      "scenario": "multi_evidence",
      "expected_behavior": "return_multiple_supporting_chunks",
      "tags": ["react", "javascript", "multi_evidence", "phase3"]
    },
    {
      "id": "abbreviation-gc",
      "question": "gc 在 Go 语言中代表什么？",
      "query": "gc",
      "context": "gc 是 garbage collection（垃圾回收）的缩写。",
      "ground_truth": "gc 是 garbage collection 的缩写，Go 语言内置垃圾回收器自动管理内存。",
      "top_k": 5,
      "relevant_ids": [
        "kb_1_doc_1_chunk_0_1779523924191899824"
      ],
      "citation_targets": [
        {
          "chunk_id": "kb_1_doc_1_chunk_0_1779523924191899824"
        }
      ],
      "query_type": "abbreviation",
      "scenario": "advanced_rewrite",
      "expected_behavior": "rewrite_or_expand_abbreviation",
      "tags": ["go", "gc", "abbreviation", "rewrite"]
    },
    {
      "id": "domain-term-promise-chain",
      "question": "JS 里的 promise 链是干什么的？",
      "query": "js promise 链 干什么",
      "context": "该样本用于覆盖领域术语和口语化表达。",
      "ground_truth": "Promise 链用于串联异步流程，让多个 then/catch 处理步骤按顺序衔接。",
      "top_k": 5,
      "relevant_ids": [
        "kb_1_doc_4_chunk_0_1779731716268688925"
      ],
      "citation_targets": [
        {
          "chunk_id": "kb_1_doc_4_chunk_0_1779731716268688925"
        }
      ],
      "query_type": "domain_term",
      "scenario": "advanced_rewrite",
      "expected_behavior": "benefit_from_domain_terms_or_route_specific_rewrite",
      "tags": ["javascript", "promise", "domain_terms", "colloquial"]
    },
    {
      "id": "typo-js-variable",
      "question": "JavaScript 里怎么声明变量？",
      "query": "javascirpt 变量声明 let const",
      "context": "该样本用于覆盖轻微拼写错误与 rewrite 能力。",
      "ground_truth": "可以使用 var、let 或 const 来声明变量。",
      "top_k": 5,
      "relevant_ids": [
        "kb_1_doc_4_chunk_0_1779731716268688925"
      ],
      "citation_targets": [
        {
          "chunk_id": "kb_1_doc_4_chunk_0_1779731716268688925"
        }
      ],
      "query_type": "typo",
      "scenario": "advanced_rewrite",
      "expected_behavior": "benefit_from_typo_correction",
      "tags": ["javascript", "typo", "rewrite", "route_specific"]
    },
    {
      "id": "insufficient-evidence-out-of-scope",
      "question": "Vue 3 的响应式系统和 React Fiber 的差异是什么？",
      "query": "Vue3 响应式 React Fiber 差异",
      "context": "该样本用于覆盖证据不足或知识库范围外场景。",
      "ground_truth": "当前评测集将它视为证据不足样本，后续用于拒答策略评估。",
      "top_k": 5,
      "relevant_ids": [],
      "citation_targets": [],
      "query_type": "insufficient_evidence",
      "scenario": "evidence_refusal",
      "expected_behavior": "future_evidence_gate_should_refuse_or_degrade_confidence",
      "tags": ["insufficient_evidence", "out_of_scope", "phase3"]
    },
    {
      "id": "insufficient-evidence-ambiguous",
      "question": "那个前端状态方案和异步方案怎么一起配合？",
      "query": "前端 状态方案 异步方案 一起配合",
      "context": "该样本故意保持模糊，用于覆盖弱证据与拒答评测。",
      "ground_truth": "当前评测集将它视为弱证据样本，后续用于 evidence refusal 与 citation consistency 回归。",
      "top_k": 5,
      "relevant_ids": [],
      "citation_targets": [],
      "query_type": "ambiguous",
      "scenario": "evidence_refusal",
      "expected_behavior": "future_evidence_gate_should_avoid_overconfident_answering",
      "tags": ["ambiguous", "weak_evidence", "phase3"]
    }
  ]
}
```

### 这段代码在做什么

这份真实数据集已经明显不是“只有几个实体问答”的简单样本了，它覆盖了：

1. Phase 2 baseline 场景。
2. 长文场景。
3. 多证据场景。
4. 缩写改写。
5. 领域术语与口语表达。
6. 弱证据与越界问题。

这正是“评测集扩展”的核心意义：让新策略不是在单一题型上自嗨，而是在不同失败模式上被真正检验。

### 为什么要这样写

如果 parent-child、rewrite、refusal 这些高级能力还用老的实体问答数据集来评估，那么很多策略即使写对了，也看不出收益；或者反过来，看起来“没有坏”，其实只是因为样本不够能暴露问题。

### 它如何衔接下一步

数据、profile 都有了，接下来就是 Runner 如何把它们跑成报告。

## 第七步：生成 comparison、contribution 和 gate 结果

### 目标

这一层是离线评测的核心执行器。它要把多个策略、多个样本和多个指标组合成一份可决策报告。

### 文件

`backend/internal/milvus/evaluation/runner.go`

### 完整代码

```go
func (r *Runner) Run(ctx context.Context, dataset []DatasetCase, profiles []StrategyProfile, thresholds GateThresholds, baselineName, candidateName string) (*Report, error) {
	if len(dataset) == 0 {
		return nil, fmt.Errorf("evaluation dataset is empty")
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("strategy profiles are empty")
	}
	if r.Factory == nil {
		return nil, fmt.Errorf("searcher factory is required")
	}

	results := make([]StrategyResult, 0, len(profiles))
	for _, profile := range profiles {
		if profile.Name == "" {
			return nil, fmt.Errorf("strategy profile name is required")
		}
		searcher, err := r.Factory(profile)
		if err != nil {
			return nil, fmt.Errorf("create searcher for %s: %w", profile.Name, err)
		}

		queryMetrics := make([]QueryMetrics, 0, len(dataset))
		latencies := make([]time.Duration, 0, len(dataset))
		totalLatency := time.Duration(0)

		for _, item := range dataset {
			topK := item.TopK
			if topK <= 0 {
				topK = 5
			}
			start := time.Now()
			items, err := searcher.Search(ctx, item)
			latency := time.Since(start)
			if err != nil {
				return nil, fmt.Errorf("strategy %s query %s failed: %w", profile.Name, item.ID, err)
			}

			resultIDs := make([]string, 0, len(items))
			for _, result := range items {
				resultIDs = append(resultIDs, result.ResultID)
			}
			queryMetrics = append(queryMetrics, QueryMetrics{
				QueryID:          item.ID,
				Query:            item.Query,
				QueryType:        item.QueryType,
				Tags:             item.Tags,
				TopK:             topK,
				Latency:          latency,
				RecallAtK:        computeRecallAtK(item.RelevantIDs, resultIDs, topK),
				MRR:              computeMRR(item.RelevantIDs, resultIDs, topK),
				NDCG:             computeNDCG(item.RelevantIDs, resultIDs, topK),
				CitationAccuracy: computeCitationAccuracy(item.CitationTargets, item.RelevantIDs, items, topK),
				ResultIDs:        resultIDs,
				RelevantIDs:      item.RelevantIDs,
				CitationTargets:  item.CitationTargets,
			})
			latencies = append(latencies, latency)
			totalLatency += latency
		}

		results = append(results, StrategyResult{
			Strategy: profile,
			Metrics:  aggregateQueryMetrics(queryMetrics, latencies, totalLatency),
			Queries:  queryMetrics,
		})
	}

	baseline := resolveStrategyResult(results, baselineName, true, false)
	candidate := resolveStrategyResult(results, candidateName, false, true)
	if baseline == nil {
		return nil, fmt.Errorf("baseline strategy not found")
	}
	if candidate == nil {
		return nil, fmt.Errorf("candidate strategy not found")
	}

	report := &Report{
		DatasetSize:  len(dataset),
		GeneratedAt:  time.Now(),
		Results:      results,
		Contribution: buildContribution(results),
		Comparison:   buildComparison(*baseline, *candidate),
		Baseline:     baseline.Strategy.Name,
		Candidate:    candidate.Strategy.Name,
	}
	report.Gate = EvaluateGate(report.Comparison, thresholds)
	return report, nil
}
```

### 这段代码在做什么

Runner 做了三件核心事情：

1. 对每个 profile 跑完整个 dataset。
2. 聚合出单策略指标。
3. 再从所有策略里解析出 baseline、candidate 和逐步贡献。

注意这里不是只做“最终比较”，还做了 `Contribution`。这意味着报告不仅能回答“最后赢没赢”，还能回答“中间哪一级开始收益变差了”。

### 为什么要这样写

如果只保留最终 `baseline vs candidate`，当 gate 失败时你只知道“最后不行”，但不知道问题是出在 parent-child、topk 还是 rewrite。

当前实现通过完整保留 `results + contribution + comparison`，让报告既能做发布门禁，也能做调参归因。

### 它如何衔接下一步

Runner 算出 comparison 之后，就要用 gate 把它变成明确的通过或失败。

### 文件

`backend/internal/milvus/evaluation/gate.go`

### 完整代码

```go
func DefaultGateThresholds() GateThresholds {
	return GateThresholds{
		MinRecallDelta:               0.08,
		MinMRRDelta:                  0.00,
		MinNDCGDelta:                 0.00,
		MinCitationAccuracyDelta:     0.00,
		MaxP95LatencyRegressionMS:    0,
		MaxP95LatencyRegressionRatio: 0.20,
	}
}

func EvaluateGate(comparison ComparisonSummary, thresholds GateThresholds) GateResult {
	checks := []GateCheck{
		{
			Name:     "recall_at_k_delta",
			Actual:   comparison.RecallDelta,
			Expected: thresholds.MinRecallDelta,
			Passed:   comparison.RecallDelta >= thresholds.MinRecallDelta,
			Message:  "candidate recall gain must stay above threshold",
		},
		{
			Name:     "mrr_delta",
			Actual:   comparison.MRRDelta,
			Expected: thresholds.MinMRRDelta,
			Passed:   comparison.MRRDelta >= thresholds.MinMRRDelta,
			Message:  "candidate MRR must not regress below threshold",
		},
		{
			Name:     "ndcg_delta",
			Actual:   comparison.NDCGDelta,
			Expected: thresholds.MinNDCGDelta,
			Passed:   comparison.NDCGDelta >= thresholds.MinNDCGDelta,
			Message:  "candidate nDCG must not regress below threshold",
		},
		{
			Name:     "citation_accuracy_delta",
			Actual:   comparison.CitationAccuracyDelta,
			Expected: thresholds.MinCitationAccuracyDelta,
			Passed:   comparison.CitationAccuracyDelta >= thresholds.MinCitationAccuracyDelta,
			Message:  "candidate citation accuracy must not regress below threshold",
		},
		{
			Name:     "p95_latency_regression_ratio",
			Actual:   comparison.P95LatencyDeltaRatio,
			Expected: thresholds.MaxP95LatencyRegressionRatio,
			Passed:   comparison.P95LatencyDeltaRatio <= thresholds.MaxP95LatencyRegressionRatio,
			Message:  "candidate P95 regression ratio must stay within threshold",
		},
	}

	if thresholds.MaxP95LatencyRegressionMS > 0 {
		checks = append(checks, GateCheck{
			Name:     "p95_latency_regression_ms",
			Actual:   comparison.P95LatencyDeltaMS,
			Expected: thresholds.MaxP95LatencyRegressionMS,
			Passed:   comparison.P95LatencyDeltaMS <= thresholds.MaxP95LatencyRegressionMS,
			Message:  "candidate P95 regression ms must stay within threshold",
		})
	}

	passed := true
	for _, check := range checks {
		if !check.Passed {
			passed = false
			break
		}
	}
	return GateResult{
		Passed:     passed,
		Thresholds: thresholds,
		Checks:     checks,
	}
}
```

### 这段代码在做什么

Gate 把“比较结果”转换成“可以被 CI、脚本、发布流程消费的判断结果”。

最重要的是它同时关心两类东西：

1. 质量指标要不退化，最好提升。
2. 延迟不能因为质量提升而无限恶化。

这也是为什么 gate 里既看 `Recall / MRR / nDCG / Citation Accuracy`，也看 `P95`。

### 为什么要这样写

只看质量不看延迟，容易把系统调成“离线很好、线上很慢”。

只看延迟不看质量，又会把 candidate 变成“其实没提升，只是更快”。

所以这一步本质上是在表达一个工程现实：候选策略必须在质量和成本之间达成最低平衡。

### 它如何衔接下一步

最后一步就是把所有东西串成命令入口，真正跑起来。

## 第八步：把数据集、Profile 和 Gate 串成可执行命令

### 目标

这一层让前面的结构不只停留在库代码里，而是变成团队能直接执行的评测命令。

### 文件

`backend/scripts/evaluation/retrieval_strategy_profiles.example.json`

### 完整代码

```json
[
  {
    "name": "phase2_baseline",
    "label": "Phase 2 Baseline",
    "baseline": true,
    "mode": "hybrid",
    "enable_query_rewrite": true,
    "enable_dynamic_topk": true,
    "enable_advanced_rerank": true,
    "candidate_top_k": 10
  },
  {
    "name": "parent_child",
    "label": "Parent Child",
    "mode": "hybrid",
    "enable_query_rewrite": true,
    "enable_dynamic_topk": true,
    "enable_advanced_rerank": true,
    "enable_parent_child_retrieval": true,
    "candidate_top_k": 10
  },
  {
    "name": "parent_child+strategic_topk",
    "label": "Parent Child + Strategic TopK",
    "mode": "hybrid",
    "enable_query_rewrite": true,
    "enable_dynamic_topk": true,
    "enable_advanced_rerank": true,
    "enable_parent_child_retrieval": true,
    "enable_strategic_topk": true,
    "candidate_top_k": 10
  },
  {
    "name": "parent_child+refusal",
    "label": "Parent Child + Refusal",
    "mode": "hybrid",
    "enable_query_rewrite": true,
    "enable_dynamic_topk": true,
    "enable_advanced_rerank": true,
    "enable_parent_child_retrieval": true,
    "enable_evidence_refusal": true,
    "candidate_top_k": 10
  },
  {
    "name": "parent_child+advanced_rewrite",
    "label": "Parent Child + Advanced Rewrite",
    "candidate": true,
    "mode": "hybrid",
    "enable_query_rewrite": true,
    "enable_dynamic_topk": true,
    "enable_advanced_rerank": true,
    "enable_parent_child_retrieval": true,
    "enable_domain_terms": true,
    "enable_route_specific_rewrite": true,
    "enable_model_assisted_rewrite": true,
    "candidate_top_k": 10
  }
]
```

### 这段代码在做什么

这份 JSON 是“可编辑实验面板”。不改 Go 代码，也能新增或重排实验方案。

### 为什么要这样写

这让评测组合的维护成本更低，也方便把某次实验的 profile 文件直接归档进 PR 或发布记录。

### 它如何衔接下一步

Profile 确定后，还需要独立的 gate 阈值文件。

### 文件

`backend/scripts/evaluation/retrieval_gate_thresholds.example.json`

### 完整代码

```json
{
  "min_recall_delta": 0.08,
  "min_mrr_delta": 0.0,
  "min_ndcg_delta": 0.0,
  "min_citation_accuracy_delta": 0.0,
  "max_p95_latency_regression_ratio": 0.2
}
```

### 这段代码在做什么

它把“门槛”从代码里抽出来变成文件，所以不同阶段、不同 CI 流水线都能使用不同阈值。

### 为什么要这样写

如果门槛只能写死在代码里，团队想临时收紧或放宽标准，就必须重新编译工具。

### 它如何衔接下一步

最后由统一入口来调用这些文件。

### 文件

`backend/scripts/evaluation/evaluate.py`

### 完整代码

```python
def run_retrieval_mode(args: argparse.Namespace) -> int:
    command = [
        "go",
        "run",
        "./cmd/retrieval-eval",
        "-config",
        args.config,
        "-dataset",
        args.dataset,
        "-output",
        args.output,
    ]
    if args.profiles:
        command.extend(["-profiles", args.profiles])
    if args.gates:
        command.extend(["-gates", args.gates])
    if args.baseline:
        command.extend(["-baseline", args.baseline])
    if args.candidate:
        command.extend(["-candidate", args.candidate])
    if args.collection:
        command.extend(["-collection", args.collection])

    print("Running retrieval regression:")
    print(" ".join(command))
    completed = subprocess.run(command, cwd=BACKEND_DIR, check=False)
    if completed.returncode == 0:
        print(f"\nRetrieval regression completed. Report prefix: {args.output}")
    elif completed.returncode == 2:
        print(f"\nRetrieval regression completed but gate failed. Report prefix: {args.output}")
    return completed.returncode


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Evaluation entrypoint")
    parser.add_argument("--mode", choices=["retrieval", "ragas"], default="retrieval")
    parser.add_argument("--config", default="config.yaml")
    parser.add_argument("--dataset", default=str(DATASET_FILE))
    parser.add_argument("--profiles", default=str(SCRIPT_DIR / "retrieval_strategy_profiles.example.json"))
    parser.add_argument("--gates", default=str(SCRIPT_DIR / "retrieval_gate_thresholds.example.json"))
    parser.add_argument("--output", default=str(RETRIEVAL_OUTPUT_PREFIX))
    parser.add_argument("--baseline", default="")
    parser.add_argument("--candidate", default="")
    parser.add_argument("--collection", default="")
    parser.add_argument("--no-api", action="store_true", help="Only used in ragas mode")
    parser.add_argument("--ragas-report", default=str(RAGAS_REPORT_FILE))
    return parser
```

### 这段代码在做什么

这个 Python 入口把 Go 评测命令包成了一个更友好的调用层。它的价值不在技术复杂度，而在使用门槛：

1. 默认路径都给好了。
2. 支持单独替换 baseline/candidate。
3. 同一个入口还能兼容 legacy Ragas 模式。

### 为什么要这样写

如果团队每次都要手敲长串 `go run` 参数，评测流程会越来越少人愿意执行，最后工具就会“存在但没人用”。

### 它如何衔接下一步

到这里，整套“基线冻结 -> 策略开关 -> 评测集扩展 -> 报告输出”的链路就打通了。

## 6. 如何验证

你至少应该验证下面几件事。

### 6.1 配置与快照

在 `backend` 目录启动服务或加载配置后，检查：

1. `backend/docs/baseline/phase1/baseline_snapshot.json`
2. `backend/docs/baseline/phase2/baseline_snapshot.json`

确认它们首次生成后不会被重复启动覆盖。

### 6.2 环境变量覆盖

可以用类似下面的变量做一次覆盖启动：

```powershell
$env:RAG_ENABLE_PARENT_CHILD_RETRIEVAL="true"
$env:RAG_PARENT_CHILD_FILL_STRATEGY="section_window"
$env:RAG_RELEASE_ENABLED="true"
$env:RAG_RELEASE_STAGE="small_flow"
```

然后检查启动日志里的 `[RAG:L0] snapshot ... flags=... release=...` 是否反映了真实生效值。

### 6.3 离线评测

在 `backend` 目录运行：

```powershell
python scripts/evaluation/evaluate.py
```

或直接运行：

```powershell
go run ./cmd/retrieval-eval `
  -config ./config.yaml `
  -dataset ./scripts/evaluation/dataset.json `
  -profiles ./scripts/evaluation/retrieval_strategy_profiles.example.json `
  -gates ./scripts/evaluation/retrieval_gate_thresholds.example.json `
  -output ./docs/retrieval-regression-report
```

成功后应该看到：

1. `backend/docs/retrieval-regression-report.json`
2. `backend/docs/retrieval-regression-report.md`

### 6.4 测试

配置层至少有这些测试覆盖：

`backend/internal/config/config_rag_test.go`

这份测试重点验证：

1. 非法权重和非法范围会失败。
2. overlay 与环境变量覆盖会生效。
3. `LoadConfig` 后 Phase 1 与 Phase 2 快照会被创建。

## 7. 取舍与后续优化

当前实现已经很实用，但也有一些明确取舍。

### 7.1 当前版本优化了什么

1. 优先保证迁移平滑，所以数据集加载器兼容旧数组格式。
2. 优先保证可回滚，所以快照首次写入后不覆盖。
3. 优先保证可归因，所以 profile 采用阶梯式实验组，而不是只比较最终 candidate。

### 7.2 当前版本暂时没有解决什么

1. 快照是首次冻结，不包含后续“人工确认基线升级”的工作流。
2. 数据集虽然有 `scenario` 和 `tags`，但报告还没有按场景自动拆分子报表。
3. Gate 仍是全局阈值，暂时没有“某个场景单独设门槛”的能力。

### 7.3 下一步最自然的演进方向

1. 给快照增加“人工确认升级基线”的命令，而不是只靠首次写入。
2. 在报告里加入按 `scenario`、`query_type`、`tags` 的切片统计。
3. 给 Gate 增加场景级阈值，例如“弱证据样本允许 Recall 下降，但要求拒答率提升”。
4. 把 `strategy_digest` 写入评测报告，直接把运行态和离线态串起来。

这套设计最重要的成果，不是多了几个 JSON 文件，而是把高级检索迭代变成了一条可冻结、可开关、可评测、可回滚的工程链路。你可以把它理解成：功能开发只是前半段，这条链路才是让功能真正可交付的后半段。
