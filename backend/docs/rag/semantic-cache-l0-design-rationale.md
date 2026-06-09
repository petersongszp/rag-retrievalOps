# Semantic Cache L0 设计说明

## 1. 文档目的

这份文档专门解释 Semantic Cache 的 `L0：配置、边界和命中契约冻结` 为什么要这样做。

它不负责讲完整实现细节，也不负责替代后续 `L1/L2/L3` 的开发教程。它只回答三个问题：

1. 这次到底改了什么
2. 为什么第一步先改这些，而不是直接写缓存读写逻辑
3. 这些改动在团队协作里解决了什么问题

---

## 2. L0 的核心目标

L0 不是“做出 Semantic Cache 功能”，而是“先把 Semantic Cache 的口径冻住”。

这里的“冻住”有四层意思：

1. 冻住配置入口
2. 冻住命中边界
3. 冻住结果契约
4. 冻住错误配置时的失败方式

原因很简单：Semantic Cache 不是普通 KV 缓存，它会直接影响检索结果。如果边界不清楚，后面最容易出现的不是“功能没做完”，而是“做出来但结果不可信”。

所以 L0 的原则不是先追求快，而是先追求稳。

---

## 3. 这次具体改了什么

本次 L0 主要修改了 3 个文件：

1. [config.go](/d:/RAG/rag-retrievalOps/backend/internal/config/config.go)
2. [config_rag_test.go](/d:/RAG/rag-retrievalOps/backend/internal/config/config_rag_test.go)
3. [config.rag.example.yaml](/d:/RAG/rag-retrievalOps/backend/config.rag.example.yaml)

### 3.1 新增 Semantic Cache 配置块

在 `RAGConfig` 中新增了：

```go
SemanticCache RAGSemanticCacheConfig `yaml:"semantic_cache"`
```

并新增了配置结构：

```go
type RAGSemanticCacheConfig struct {
    SimilarityThreshold float64 `yaml:"similarity_threshold"`
    TTLSeconds          int     `yaml:"ttl_seconds"`
    MaxCandidates       int     `yaml:"max_candidates"`
    MaxEntriesPerScope  int     `yaml:"max_entries_per_scope"`
}
```

同时增加了 feature flag：

```go
EnableSemanticCache bool `yaml:"enable_semantic_cache"`
```

### 3.2 新增默认值

在 `applyRAGDefaults()` 中补了默认值：

1. `similarity_threshold = 0.92`
2. `ttl_seconds = 900`
3. `max_candidates = 20`
4. `max_entries_per_scope = 200`

### 3.3 新增环境变量覆盖

在 `applyRAGEnvOverrides()` 中补了这些环境变量：

1. `RAG_ENABLE_SEMANTIC_CACHE`
2. `RAG_SEMANTIC_CACHE_SIMILARITY_THRESHOLD`
3. `RAG_SEMANTIC_CACHE_TTL_SECONDS`
4. `RAG_SEMANTIC_CACHE_MAX_CANDIDATES`
5. `RAG_SEMANTIC_CACHE_MAX_ENTRIES_PER_SCOPE`

### 3.4 新增 fail-fast 校验

在 `ValidateRAGPrerequisites()` 里增加了 Semantic Cache 的启动校验。

只有在 `enable_semantic_cache=true` 时才会强制检查：

1. `redis.addr` 不能为空
2. `similarity_threshold` 必须在 `(0,1]`
3. `ttl_seconds > 0`
4. `max_candidates > 0`
5. `max_entries_per_scope > 0`

### 3.5 冻结命中契约

新增了：

```go
func (c *Config) SemanticCacheContract() RAGSemanticCacheContract
```

用于显式返回当前 L0 约定：

1. scope 维度：`tenant_id`, `kb_ids`, `strategy_version`, `query_type`
2. bypass 原因：`empty_query`, `debug_request`, `authorization_abnormal`, `high_risk_experiment`
3. payload 类型：`retrieve_result_only`
4. TopK 策略：`exact_topk_only`

### 3.6 新增快照日志

在 `LogRAGSnapshot()` 里把 Semantic Cache 的开关、参数和契约打进启动日志。

这样做以后，线上如果出现问题，我们能快速知道：

1. 当时缓存有没有开
2. 阈值配的是多少
3. 作用域按什么口径隔离
4. 命中的到底是哪种结果契约

### 3.7 新增单元测试

在 `config_rag_test.go` 中补了以下测试：

1. 开关关闭时允许零值配置
2. 开关开启但 Redis 缺失时报错
3. 相似度阈值非法时报错
4. TTL 非法时报错
5. `max_candidates` 非法时报错
6. `max_entries_per_scope` 非法时报错
7. 合法配置可以通过
8. 环境变量覆盖可以生效
9. `SemanticCacheContract()` 返回的口径符合预期

---

## 4. 为什么要先改配置，而不是先写缓存逻辑

很多初学者看到缓存需求，第一反应是：

“先把 Redis 读写写出来不就行了吗？”

但 Semantic Cache 和普通缓存不一样，原因在于它缓存的是“检索结果”，不是“页面片段”或者“对象快照”。

这意味着它的风险点在于“命中错了”，而不是“没命中”。

没命中，最多是慢一点。
误命中，返回的就是错误结果。

所以第一步必须先做这三件事：

1. 先定义什么情况下允许命中
2. 先定义什么情况下绝对不能命中
3. 先定义命中后返回的到底是什么

只有这些先定死，后面的 Redis 协议、候选召回、相似度计算、回填逻辑才不会反复返工。

---

## 5. 为什么要加 feature flag

`enable_semantic_cache` 的意义，不只是“开关功能”。

它同时承担了 3 个职责：

1. 灰度入口
2. 回滚入口
3. 回归对照开关

### 5.1 灰度入口

后续接入真实缓存链路后，我们不一定敢一次全量开启。

有了 feature flag，就可以先在开发环境、测试环境、小流量环境里逐步放量。

### 5.2 回滚入口

如果线上发现误命中、命中率异常、性能收益不达预期，最安全的方式不是删代码，而是直接关掉开关，恢复原检索路径。

### 5.3 回归对照开关

测试时也需要能快速对比：

1. 开缓存前的行为
2. 开缓存后的行为

没有开关，就很难做 A/B 对比和快速排障。

---

## 6. 为什么要加这些参数

### 6.1 `similarity_threshold`

这是最核心的安全阀。

语义缓存最大的风险是“两个问题看起来像，但其实不是一回事”。阈值越低，命中率可能更高，但误命中风险也更高。

L0 阶段先把它显式配置化，有两个目的：

1. 以后可以调参，而不是写死在代码里
2. 让团队对“安全优先还是召回优先”有共同口径

### 6.2 `ttl_seconds`

缓存不能永久有效，因为知识库内容会变。

TTL 的作用是给缓存一个自然过期边界，避免旧检索结果无限期复用。

即使以后会做主动失效，TTL 也依然是最基础的保护网。

### 6.3 `max_candidates`

语义缓存通常不是只查一条，而是先拿一批候选，再在应用层做相似度比较。

这个参数控制的是查找成本上限，避免“为了找缓存反而比正常检索更慢”。

### 6.4 `max_entries_per_scope`

scope 内的缓存条目不能无限膨胀，否则 Redis 会持续堆积，查找和清理都会越来越重。

这个参数是后续做淘汰和治理的前提。

---

## 7. 为什么要做 fail-fast 校验

这里的思路是：

如果配置错了，就在启动时失败，而不是在运行中悄悄出错。

这样做有 4 个原因。

### 7.1 提前暴露风险

如果 `enable_semantic_cache=true`，但 Redis 地址没配，那说明系统根本不具备工作条件。

这种情况下继续启动，只会把错误拖到运行期。

### 7.2 防止隐性错误

比如阈值配成 `1.5`，程序也许还能跑，但语义上完全错误。

这种错误最危险，因为系统“看起来能用”，但结果已经不可信。

### 7.3 保护后续开发

L1/L2 开始接缓存链路后，如果没有前置校验，开发和测试阶段会出现很多“这段逻辑为什么不生效”的伪问题。

其实根源只是配置坏了。

### 7.4 统一团队预期

fail-fast 本身也是契约的一部分：团队都知道，只要开了 Semantic Cache，就必须满足这些前提条件。

---

## 8. 为什么要冻结命中契约

这是 L0 最关键的部分。

### 8.1 为什么 scope 要包含 `tenant_id`

这是多租户系统最基本的隔离要求。

如果不包含 `tenant_id`，最坏情况就是 A 租户的问题命中了 B 租户的结果，这是严重数据污染。

### 8.2 为什么 scope 要包含 `kb_ids`

即使是同一个租户，不同知识库的语义上下文也可能完全不同。

不按知识库隔离，就会出现“问题类似，但答案来自错误知识库”的情况。

### 8.3 为什么 scope 要包含 `strategy_version`

检索策略一旦变化，旧结果的生成逻辑就和新请求不一致了。

如果不把策略版本纳入 scope，就会出现“新策略命中旧策略结果”的问题。

### 8.4 为什么 scope 要包含 `query_type`

不同 query type 往往代表不同处理语义，比如 debug 查询、普通查询、特殊路由查询，它们的候选生成和后续判断可能不同。

提前纳入 scope，能减少后面扩展时的兼容风险。

### 8.5 为什么 payload 固定为 `retrieve_result_only`

这次只缓存检索结果，不缓存最终 LLM 回答，是为了控制复杂度。

因为回答层缓存会引入更多变量：

1. prompt 版本
2. model 版本
3. citation 约束
4. answer style

如果 L0 就把 answer cache 一起放进来，边界会立刻变复杂，团队很难统一口径。

### 8.6 为什么 TopK 先固定成 `exact_topk_only`

TopK 兼容策略本身是个容易争议的点。

例如请求 `top_k=5`，能不能命中 `top_k=8` 的缓存后再裁剪？

这件事不是不能做，而是 L0 不应该先放开。因为它会立刻带来：

1. 结果一致性争议
2. 裁剪顺序争议
3. rerank 后稳定性争议

所以先固定成精确匹配，后面如果要放开，再在更高层级里显式设计。

---

## 9. 为什么要把契约写进代码，而不是只写在文档里

只写文档的问题是，文档会被忘、会过期、会和代码分离。

把契约写进 `SemanticCacheContract()` 有三个好处：

1. 代码可引用
2. 日志可观测
3. 测试可校验

这样后续无论谁接手，都能明确看到系统当前认定的口径，而不是靠口头同步。

---

## 10. 为什么要写到启动快照日志里

因为线上排查问题时，最怕的是“不知道当时配置是什么”。

把 Semantic Cache 参数和契约打进快照日志，相当于给每次启动留下一个“现场记录”。

以后如果发现命中率异常或者结果污染，可以先排查：

1. 当时 Semantic Cache 是否开启
2. 阈值是不是被调过
3. scope 维度是什么
4. topk 策略是什么

这会显著降低排障成本。

---

## 11. 为什么要补这么多单测

L0 的本质不是业务功能测试，而是“规则测试”。

这里的测试重点不是“缓存命中了没有”，而是：

1. 规则有没有被冻结
2. 错配置能不能被拦住
3. 环境变量和 YAML 会不会互相打架
4. 约定输出会不会被后续改坏

这些测试越早补，后面越省事。

因为一旦进入 `L1/L2/L3`，系统复杂度会迅速上升。如果 L0 没守住，后面每一层都会反复背锅。

---

## 12. 这次改动解决了什么团队问题

从团队协作角度看，这次 L0 主要解决了 5 个问题。

### 12.1 解决“大家嘴上理解不一样”

现在 Semantic Cache 的配置、边界和契约都已经代码化，减少了口头理解偏差。

### 12.2 解决“后面写着写着边界漂移”

L0 先冻住 scope、payload、TopK 规则，后续开发就不会随手放开。

### 12.3 解决“线上出问题不好回滚”

feature flag 已经准备好了，后面一旦接入主链路就具备一键关闭能力。

### 12.4 解决“调试时不知道配置是否生效”

现在环境变量覆盖、默认值和快照日志都已经补齐，配置生效路径清晰很多。

### 12.5 解决“不同开发者并行时互相踩边界”

L0 做完后，后面可以相对稳定地拆分：

1. 一个人做 Redis 协议
2. 一个人做检索入口接入
3. 一个人做日志和测试

因为最容易争议的契约部分已经先锁定了。

---

## 13. 初学者最容易误解的点

### 13.1 误解一：L0 没写读写逻辑，是不是没做事

不是。

L0 做的是“后面所有实现的边界基础”。这种工作看起来不炫，但非常重要。

如果没有 L0，后面的功能很可能越做越乱。

### 13.2 误解二：默认值是不是随便给的

不是完全随便，但也不是最终最优值。

L0 的默认值是“保守可用的起点”，目的是让系统先具备稳定的初始行为，后面可以根据实测再调。

### 13.3 误解三：契约为什么这么保守

因为语义缓存最怕错命中。

在这类能力里，第一阶段宁愿保守，也不要一开始就追求覆盖率。

---

## 14. 可以怎么向组长汇报

可以直接用下面这段：

> 我先完成了 Semantic Cache 的 L0，不接入真实缓存读写，只冻结配置、命中边界和结果契约。  
> 目前已经补齐 feature flag、语义缓存参数、默认值、环境变量覆盖和 fail-fast 校验；同时把 scope 隔离维度、bypass 场景、payload 类型和 TopK 策略固化成代码契约，并写入启动快照日志。  
> 这样后面做 Redis 协议和检索入口接入时，团队不会在“什么能命中、命中什么结果、出了问题怎么回滚”这些基础问题上反复返工。  
> 这一步已经通过 `go test ./internal/config` 验证，并完成本地提交。

如果组长继续追问“这一步的价值是什么”，你可以补一句：

> 这一步的价值不是增加功能点，而是降低后续实现风险，特别是跨租户误命中、策略漂移和配置失控这几类高风险问题。

---

## 15. 当前结论

这次 L0 的本质不是“把 Semantic Cache 做出来”，而是“把 Semantic Cache 做对的前提先搭起来”。

它的价值可以概括成一句话：

先把会影响正确性的规则锁死，再去做会影响性能的实现。

这也是为什么本次优先改的是配置、校验、契约和测试，而不是直接写 Redis 查询逻辑。
