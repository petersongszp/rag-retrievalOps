# L1 混合检索召回链路实现教程

本文对应当前仓库里已经落地的 L1 混合检索召回链路实现，目标是让后续同学不只是“知道代码在哪里”，而是能理解这套实现为什么这么拆、每一步要解决什么问题、按什么顺序写会比较稳。

这篇文档默认读者可能是第一次接触 RAG，所以文中会尽量把术语解释成人话。如果你以前没有做过检索系统，也可以顺着看下去。

这篇教程覆盖的主链路是：

`dense + sparse -> merge/dedupe -> rerank -> truncate -> log/metrics`

对应文件主要在：

- `backend/internal/milvus/retrieval/options.go`
- `backend/internal/milvus/retrieval/filter.go`
- `backend/internal/milvus/retrieval/hybrid_search.go`
- `backend/internal/milvus/retrieval/sparse_search.go`
- `backend/internal/milvus/retrieval/sparse_inverted_index.go`
- `backend/internal/milvus/retrieval/sparse_inverted_index_test.go`
- `backend/internal/milvus/init.go`
- `backend/internal/agents/tools/milvus_retriever_tool.go`
- `backend/internal/observability/metrics/rag_metrics.go`

---

## 1. 先讲清楚：L1 到底要解决什么问题

在只有 dense 向量检索的时候，系统对“语义相近”的问题通常还可以，但对下面几类 query 容易不稳定：

1. 实体词很强，例如产品名、组件名、缩写词。
2. 用户 query 很短，向量表达不充分。
3. 错拼、混拼、中英夹杂导致 embedding 语义不够稳。
4. 某些知识点必须靠显式词命中，纯语义近邻容易漏掉。

所以 L1 的目标不是“把原来的 dense 替换掉”，而是把检索从单路升级成多路召回：

1. dense 继续负责语义召回。
2. sparse 负责显式关键词命中。
3. 两路结果统一合并。
4. 单路失败时尽量降级，不拖垮整条链路。
5. 整个过程要可观测，后面才能继续做 Phase 2 的优化。

这里最重要的工程判断是：**先把召回链路编排起来，再追求更复杂的 BM25 或 learned sparse**。因为在业务早期，最先缺的通常不是更复杂的算法，而是一个可解释、可回滚、可迭代的骨架。

---

## 1.1 先补几个最容易卡住的基础词

如果你刚接触 RAG，下面几个词一定先看懂，后面读起来会轻松很多。

### 什么是 RAG

RAG 可以先简单理解成一条两段式流程：

1. 先从知识库里把“可能相关”的资料找出来，这一步叫“检索”。
2. 再把这些资料连同用户问题一起交给大模型生成答案。

所以 RAG 的第一关不是“模型会不会回答”，而是“你能不能先找对资料”。

### 什么是召回

“召回”这个词可以先理解成：**把可能相关的候选文档先找回来**。

注意，召回阶段不要求一上来就排得特别准，它更像“先把候选人名单拉出来”。  
后面还会有排序、重排、截断这些步骤，继续从候选里挑最合适的结果。

### 什么是 dense

`dense` 可以先理解成“向量检索”。

它的基本思路是：

1. 把用户问题转成一个向量。
2. 把知识库文档也转成向量。
3. 去向量库里找“离这个问题最近”的文档。

它擅长找“意思接近”的内容，所以我们常说它偏语义检索。

比如用户问：

`Go 里面协程调度是怎么做的？`

就算文档里没有完全一样的句子，只要文档里写的是 “goroutine 调度模型”“runtime scheduler”，dense 也可能把它找出来。

### 什么是 sparse

`sparse` 可以先理解成“关键词检索”。

它的基本思路是：

1. 从 query 里拆出几个关键词。
2. 先找出包含这些词的文档。
3. 再根据词命中情况给这些文档排序。

它擅长找“字面上真的出现过这个词”的内容，所以对缩写词、组件名、专业名词特别有帮助。

### 什么是 rerank

`rerank` 可以先理解成“重排”。

也就是：

1. 先把一批候选找回来。
2. 再用另一套更细的比较方法，把这些候选重新排一遍顺序。

你可以把它理解成：召回阶段是“先把相关的人叫进会议室”，重排阶段是“再决定谁坐前排”。

### 什么是 route

这里的 `route` 不是 HTTP 路由，而是“检索路线”或者“检索通道”。

比如：

1. dense route：走向量检索这条路
2. sparse route：走关键词检索这条路

混合检索的意思，就是同一个 query 同时走多条 route，再把结果合起来。

### 什么是 term

`term` 可以简单理解成“从 query 里拆出来的一个关键词”。

比如 query 是：

`golang runtime scheduler`

那拆出来的 term 可能就是：

- `golang`
- `runtime`
- `scheduler`

所以文中说“每个 term”时，你可以直接理解成“每个关键词”。

### 什么是 TopK

`TopK` 的意思是：**最多取前 K 个结果**。

比如：

- `TopK = 5`，就是最多拿前 5 条结果
- `TopK = 10`，就是最多拿前 10 条结果

在检索系统里，`TopK` 很常见，因为我们通常不需要把所有结果都返回，只需要前面最有可能相关的那几条。

---

## 1.2 再补一个贯穿全文的小例子

假设用户问：

`什么是 goroutine scheduler`

系统可能会这样工作：

1. dense route 去找“语义接近”的文档。
2. sparse route 去找包含 `goroutine`、`scheduler` 这些词的文档。
3. 两边各自找回一批候选。
4. 把这些候选合并。
5. 去重。
6. 再重排。
7. 最后只返回前几条。

所以整篇文档其实就在回答一件事：  
**怎么把上面这 7 步写成一条稳定、可维护的代码链路。**

---

## 2. 设计原则：为什么不是“直接塞一个 BM25 包”就结束

这次实现没有走“一把梭接个第三方 BM25 搜索器”的路线，原因有三个：

### 2.1 我们优先解决的是链路问题，不只是打分问题

真正要上线的是“混合检索召回链路”，不是“BM25 算法演示”。上线链路至少要包含：

1. 统一入参。
2. dense / sparse 两路编排。
3. 单路错误容错。
4. 合并去重。
5. 统一 score 读取。
6. route 级监控与日志。

如果只把 BM25 接进来，但没有这些工程约束，后面会很快长成一堆 if/else，最后没法维护。

### 2.2 先用 Milvus 现有能力做候选，再在应用层做显式排序

当前 sparse 路线采用的是两段式：

1. 先用 Milvus `Query + like` 做关键词候选集筛出。
2. 再在应用层基于候选集构建显式倒排索引，用 BM25 重新排序。

这样做的好处是：

1. 不需要一开始就改动底层存储结构。
2. 候选召回范围仍然受现有 metadata/filter 约束。
3. BM25 排序过程完全在应用层，可测试、可替换、可调参。

这是一个很典型的“先把系统长出来，再逐步升级局部能力”的做法。

### 2.3 对已有调用方保持兼容

老调用方已经在使用：

```go
HybridRetriever.Search(ctx, query, opts)
```

所以实现上保留了旧方法签名，同时在内部升级为新的统一请求结构。这一点非常关键，因为教程面对的是接手同学，不是重写项目的人。能平滑升级，比一次性改得“很干净”更重要。

---

## 3. 最终代码结构长什么样

建议你先建立整个脑图，再开始写文件：

### 3.1 契约层

- `RetrieveOptions`
- `HybridSearchRequest`

### 3.2 sparse 路由层

- `SparseRetriever`
- `extractSparseTerms`
- `parseQueryResultSet`
- `BuildSparseInvertedIndex`
- `SparseInvertedIndex.Search`

### 3.3 编排层

- `HybridRetriever.Search`
- `HybridRetriever.SearchWithRequest`
- `searchDense`
- `mergeRouteCandidates`

### 3.4 接入层

- `InitMilvusManager` 初始化 hybrid retriever
- `milvus_retriever_tool.go` 根据 feature flag 切流

### 3.5 观测层

- `ObserveRetrieveRoute`
- 结构化日志 `[RAG:L1] ...`

---

## 4. 第一步：先扩展统一检索参数

文件：`backend/internal/milvus/retrieval/options.go`

参考当前位置：`options.go:21-37`

现在的 `RetrieveOptions` 不再只服务 dense 检索，它要承担混合检索入口的公共参数。

核心新增字段：

```go
type RetrieveOptions struct {
    Language         DocumentLanguage
    Category         DocumentCategory
    Expr             string
    TopK             int
    Database         string
    Collection       string
    RequestID        string
    KBScope          string
    ActiveGlobalKBID uint64
}
```

### 为什么这里一定要加 `RequestID`

因为混合检索会拆成多路执行。只要一拆路，你就会立刻遇到两个问题：

1. 同一请求里的 dense 和 sparse 日志怎么串起来？
2. 哪一次空结果、超时、单路失败，对应的是哪次用户请求？

`RequestID` 不是“锦上添花”的字段，而是多路链路的最小排障凭据。

### 为什么 `KBScope / ActiveGlobalKBID / Collection` 要留在公共层

因为过滤条件不能只对 dense 生效，sparse 也必须遵守相同的知识库边界。否则就会出现：

1. dense 在 A 知识库里找。
2. sparse 却从全量集合里捞。
3. 最终 merge 出来的结果跨库污染。

混合检索里，**统一过滤边界比统一算法更重要**。

---

## 5. 第二步：把过滤表达式统一起来

文件：`backend/internal/milvus/retrieval/filter.go`

参考位置：`filter.go:8-47`

这里的 `BuildFilterExpr` 不是新写的，但在 L1 里它的角色变重要了。它负责把业务过滤条件压成 Milvus 可执行表达式：

```go
func BuildFilterExpr(opts *RetrieveOptions) string {
    if opts == nil {
        return ""
    }
    if opts.Expr != "" {
        return opts.Expr
    }

    var conditions []string
    if opts.KBScope != "" {
        conditions = append(conditions, fmt.Sprintf("metadata[\"kb_scope\"] == '%s'", opts.KBScope))
    }
    if opts.ActiveGlobalKBID > 0 {
        conditions = append(conditions, fmt.Sprintf("metadata[\"kb_id\"] == %d", opts.ActiveGlobalKBID))
    }
    ...
    return strings.Join(conditions, " && ")
}
```

### 为什么不能让 dense/sparse 各自拼表达式

因为一旦两边各自拼，迟早会出这些问题：

1. 过滤逻辑不一致。
2. 某边漏了字段。
3. 一边支持自定义 `Expr`，另一边没支持。
4. 后续新增 filter 条件时改漏。

所以正确做法是：**过滤规则只定义一次，路由方只消费结果。**

---

## 6. 第三步：定义 L1 统一请求结构 `HybridSearchRequest`

文件：`backend/internal/milvus/retrieval/hybrid_search.go`

参考位置：`hybrid_search.go:22-32`

代码：

```go
type HybridSearchRequest struct {
    Query      string
    Expr       string
    TopK       int
    KBScope    string
    KBID       uint64
    RequestID  string
    Collection string
}
```

### 为什么已经有 `RetrieveOptions` 了，还要再加一层 `HybridSearchRequest`

这一段第一次看确实容易绕，所以这里不用抽象术语，直接用“这两层分别在干什么”来讲。

先说结论：

- `RetrieveOptions` 更像“某一个具体检索器执行查询时要用的参数”
- `HybridSearchRequest` 更像“混合检索总控器拿到的一次任务单”

它们看起来都像“检索参数”，但服务的对象不是同一层。

#### 先看以前只有 dense 检索时是什么情况

以前链路比较简单：

1. 上层传进来一个 query。
2. 再带上一些检索参数，比如 `TopK`、`Expr`、`Collection`。
3. `RetrieverService` 直接拿这些参数去 Milvus 查。

这个时候 `RetrieveOptions` 就够了，因为系统里基本只有一个主要执行者，就是 dense 检索器。

换句话说，`RetrieveOptions` 最早更像是给“具体干活的人”准备的。

#### 现在为什么不够了

现在不是只有一条 dense 路线了，而是变成了：

1. 先由 `HybridRetriever` 接住整个请求。
2. 它决定同时走 dense 和 sparse 两条路。
3. 等两边都回来。
4. 再做合并、去重、rerank。
5. 最后给出最终结果。

这时候 `HybridRetriever` 的角色已经不是“具体查一次库”，而是“组织整条混合检索流程”。

所以这时就出现了两种不同层次的参数：

1. 一种是“底层某条路具体怎么查”的参数。
2. 另一种是“整条混合检索任务怎么组织”的参数。

这两类参数会有重叠，但不应该完全混成一个结构体。

#### `RetrieveOptions` 更像什么

`RetrieveOptions` 更像底层执行参数。它关心的是：

1. 这次查多少条，也就是 `TopK`
2. 过滤表达式是什么，也就是 `Expr`
3. 去哪个 `Collection`
4. 语言、分类这些过滤项是什么

这些都偏“我这一次具体怎么查”。

所以它更适合给 `RetrieverService` 这种底层检索器使用。

#### `HybridSearchRequest` 更像什么

`HybridSearchRequest` 更像编排层任务参数。它关心的是：

1. 这次混合检索的 query 是什么
2. 请求 ID 是什么，方便串日志和打点
3. 检索范围是什么，比如 `KBScope`、`KBID`
4. 最终希望拿多少结果
5. 这次任务要交给哪几条 route 去执行

如果讲得更白一点：

- `RetrieveOptions` 像“发给某个执行同学的操作单”
- `HybridSearchRequest` 像“发给组长的任务单”

组长拿到任务单之后，再决定：

1. 哪些信息发给 dense route
2. 哪些信息发给 sparse route
3. 两边结果回来后怎么汇总

所以这两层虽然都带参数，但职责不一样。

#### 如果不加这层，会有什么问题

如果没有 `HybridSearchRequest`，最直接的做法就是：

让 `HybridRetriever` 继续直接收 `RetrieveOptions`，然后把它一路往下传。

这样短期也能跑，但长期会有几个问题。

##### 问题 1：编排层会被旧 dense 接口绑住

因为 `RetrieveOptions` 是历史上偏 dense 检索接口的一套设计。

如果 `HybridRetriever` 也完全围着它转，就等于混合检索总控器必须按“老 dense 检索器的习惯”来组织工作。

这就会让编排层没有自己的边界。

##### 问题 2：以后新增编排字段时，会把 `RetrieveOptions` 越塞越胖

比如以后你可能想加这些字段：

1. rewrite 后的 query
2. 是否开启 dense route
3. 是否开启 sparse route
4. sparse 专用的 term boost
5. dense 和 sparse 不同的 TopK
6. 实验分组信息

这些字段很多并不是底层 dense 检索器必须关心的，它们更像“混合检索总控器”的控制参数。

如果你没有 `HybridSearchRequest`，就只能把这些东西硬塞进 `RetrieveOptions`。

最后 `RetrieveOptions` 就会变成一个很臃肿的结构体：

1. 有些字段 dense 用不到
2. 有些字段 sparse 用不到
3. 有些字段只是编排层想记录
4. 但大家都被迫共用这一个结构

这就是职责污染。

##### 问题 3：上层和底层会耦得越来越紧

如果没有中间这层标准请求结构，那么：

1. 上层调用方会越来越依赖底层 retriever 的字段细节
2. 编排层也会越来越依赖历史接口风格
3. 以后你想换底层实现，改动面会很大

所以中间单独加一个 `HybridSearchRequest`，本质上是在做“隔离”。

#### 加了 `HybridSearchRequest` 之后，链路变成什么样

现在的逻辑其实是：

1. 外部调用方仍然可以继续传 `RetrieveOptions`
2. `HybridRetriever.Search()` 先把它整理成 `HybridSearchRequest`
3. 从这一步开始，混合检索编排层统一只认 `HybridSearchRequest`
4. 真正调用 dense 或 sparse 时，再按各自需要转成底层参数

也就是说：

- `Search()` 是兼容旧接口的入口
- `SearchWithRequest()` 是混合检索真正工作的标准入口

这相当于在中间加了一层“翻译”：

1. 把旧世界的调用方式接住
2. 翻译成新世界统一的编排请求
3. 后面整条混合检索链路都按自己的标准来走

#### 为什么说这是“给未来扩展留空间”

因为以后如果混合检索想继续升级，很多新增字段其实都更适合放在编排层，而不是底层检索器参数里。

比如：

1. `EnableDense`
2. `EnableSparse`
3. `RewriteQuery`
4. `OriginalQuery`
5. `DenseTopK`
6. `SparseTopK`

这些字段明显更像“总控器怎么安排这次任务”，而不是“某一个底层检索器怎么查 Milvus”。

有了 `HybridSearchRequest`，这些扩展就有自然的落点，不会把老的 `RetrieveOptions` 搅乱。

#### 最后用一句最简单的话总结

为什么已经有 `RetrieveOptions`，还要再加一层 `HybridSearchRequest`？

因为：

**`RetrieveOptions` 解决的是“某条检索路怎么查”，`HybridSearchRequest` 解决的是“整条混合检索链路怎么组织”。**

前者偏执行细节，后者偏总控编排。

它们有重叠很正常，但不应该混成一个东西。

---

## 7. 第四步：扩展 `HybridRetriever`，让它真正负责“编排”

文件：`backend/internal/milvus/retrieval/hybrid_search.go`

参考位置：

- `hybrid_search.go:34-47`
- `hybrid_search.go:49-76`

结构体：

```go
type HybridRetriever struct {
    retriever       *RetrieverService
    sparseRetriever *SparseRetriever
    reranker        Reranker
    config          *HybridRetrieverConfig
}

type HybridRetrieverConfig struct {
    CandidateTopK int
    SparseConfig  *SparseRetrieverConfig
    RerankerImpl  Reranker
}
```

### 这里的关键思想

以前 `HybridRetriever` 更像“dense + rerank”。这句话如果换成人话，就是：

1. 以前它主要还是“先做向量检索”。
2. 然后把向量检索出来的结果再重排一下。
3. 它还不算一个真正意义上的“多路检索总控器”。

现在它升级为真正的路由编排器：

1. 手里有 dense route：`RetrieverService`
2. 手里有 sparse route：`SparseRetriever`
3. 手里有统一后处理：`Reranker`

如果把它讲得更白一点，可以理解成：

1. `RetrieverService` 负责“走向量检索这条路”。
2. `SparseRetriever` 负责“走关键词检索这条路”。
3. `Reranker` 负责“把多条路找回来的结果再统一排一遍”。

所以现在的 `HybridRetriever` 已经不只是一个“检索一下再排个序”的对象，而是一个“统一调度多条检索路线”的对象。

这意味着后续如果再加第三路，比如规则召回、tag route、query rewrite route，也有明确扩展点。

### 初始化时为什么就把 `SparseRetriever` 建好

因为混合检索属于热路径，不适合每次请求临时组装依赖。初始化时就把对象图准备好，请求路径只做执行，不做依赖拼装，延迟和复杂度都会更稳。

---

## 8. 第五步：保留旧接口，但内部升级为统一请求

文件：`backend/internal/milvus/retrieval/hybrid_search.go`

参考位置：`hybrid_search.go:78-115`

关键逻辑是 `Search` 方法：

```go
func (h *HybridRetriever) Search(ctx context.Context, query string, opts *RetrieveOptions) ([]*schema.Document, error) {
    if strings.TrimSpace(query) == "" {
        return nil, fmt.Errorf("query is empty")
    }

    requestID := ""
    if opts != nil {
        requestID = strings.TrimSpace(opts.RequestID)
    }
    if requestID == "" {
        requestID = fmt.Sprintf("hyb-%d", time.Now().UnixNano())
    }

    topK := h.config.CandidateTopK
    if opts != nil && opts.TopK > 0 {
        topK = opts.TopK
    }

    req := &HybridSearchRequest{...}
    ...
    return h.SearchWithRequest(ctx, req)
}
```

### 为什么这里不直接让所有调用方都改成 `SearchWithRequest`

因为那样改动面太大，而且风险不集中。正确顺序是：

1. 先在内部新增标准入口。
2. 再用老接口把参数转过去。
3. 跑稳后再决定是否推动上游逐步切换。

这是典型的增量重构路径。它看起来不如一次性重写“整洁”，但在真实业务里可靠得多。

### 这里为什么先算 `req.Expr`

注意这段：

```go
req.Expr = strings.TrimSpace(BuildFilterExpr(opts))
if req.Expr == "" {
    req.Expr = strings.TrimSpace(opts.Expr)
}
```

这里的意图是：

1. 优先走统一表达式构造。
2. 如果构不出来，再兜底使用已有 `opts.Expr`。

本质上是在做“规范输入优先，历史输入兼容兜底”。

---

## 9. 第六步：写真正的 sparse 路由

文件：`backend/internal/milvus/retrieval/sparse_search.go`

参考位置：

- `sparse_search.go:14-27`
- `sparse_search.go:29-63`
- `sparse_search.go:65-170`

### 9.1 先定义配置

```go
type SparseRetrieverConfig struct {
    DefaultTopK   int
    MaxTerms      int
    PerTermFactor int
    MinPerTermK   int
}
```

这几个参数都不是随便起的。

#### `MaxTerms`

控制从 query 里最多抽多少个关键词。太少，召回面不够；太多，Milvus `like` 查询次数和噪音都变大。

#### `PerTermFactor`

这里最容易让初学者困惑，所以展开讲一下。

先假设：

- 用户最终只想要 `TopK = 5` 条结果
- query 被拆成了 3 个 term，也就是 3 个关键词

如果我们对每个 term 都只查 5 条候选，看起来好像够了，但其实通常不够，原因有三个：

1. 不同 term 查出来的结果可能有重复。
2. 有些结果只是“碰巧包含这个词”，相关性并不高。
3. 后面我们还要做 BM25 排序，如果候选太少，排序器没有发挥空间。

所以工程上常见做法是：  
**最终只想返回 5 条，但先多拿一些候选回来，比如每个 term 先拿 20 条。**

`PerTermFactor` 就是在控制这个“先多拿多少”的比例。

比如：

- `TopK = 5`
- `PerTermFactor = 4`

那每个 term 的候选上限就是：

`5 * 4 = 20`

也就是说：

1. 最终结果只要 5 条。
2. 但每个关键词先去 Milvus 里查 20 条候选。
3. 后面合并、去重、排序之后，再选出最好的 5 条。

所以文中的 `topK * factor`，你可以直接理解成：

**“最终想要的结果数” × “候选放大倍数”**

#### `MinPerTermK`

这也是一个“为了别查得太少”的保护值。

比如：

- `TopK = 3`
- `PerTermFactor = 4`

那算出来每个 term 只查：

`3 * 4 = 12`

12 不一定够用。因为：

1. 3 本来就很小。
2. 12 条里还可能重复。
3. 还要给后面的排序留空间。

所以我们再设一个下限，比如 `MinPerTermK = 20`，意思是：

1. 正常按 `TopK * PerTermFactor` 算。
2. 但如果算出来太小，就至少查 20 条。

所以它本质上是在说：

**“别因为用户只想拿 3 条结果，我们就把前面的候选查得太窄。”**

这几个配置的本质，是在平衡：

1. 召回面
2. 排序空间
3. 查询次数
4. 延迟成本

---

## 10. 第七步：先从 query 里提取 sparse term

文件：`backend/internal/milvus/retrieval/sparse_search.go`

参考位置：`sparse_search.go:242-277`

实现：

```go
func extractSparseTerms(query string, maxTerms int) []string
```

它做了几件事：

1. 全部转小写。
2. 按非字母数字切词。
3. 过滤长度过短的 term。
4. 过滤一批英文停用词。
5. 去重。
6. 截断到 `maxTerms`。

### 为什么 L1 只做这么朴素的切词

因为这一层目标不是把 sparse 检索做到“学术最优”，而是尽快得到一个可工作的 route：

1. 对英文技术词、缩写词，简单切词已经有明显收益。
2. 这段逻辑足够透明，线上问题容易解释。
3. 以后要升级分词器、同义词扩展、拼写纠错，也有明确替换点。

工程上最忌讳的一种情况是：一开始就上复杂 NLP 处理，结果召回问题一来，没人能判断是召回、分词还是排序出了问题。

---

## 11. 第八步：用 Milvus `Query + like` 先拿到 sparse 候选集

文件：`backend/internal/milvus/retrieval/sparse_search.go`

参考位置：`sparse_search.go:83-139`

核心逻辑：

```go
for _, term := range terms {
    likeExpr := fmt.Sprintf("content like \"%%%s%%\"", escapeLikeValue(term))
    expr := likeExpr
    if baseExpr != "" {
        expr = fmt.Sprintf("(%s) && (%s)", baseExpr, likeExpr)
    }

    resultSet, err := s.client.Query(
        ctx,
        collection,
        nil,
        expr,
        []string{"id", "content", "metadata"},
        milvusClient.WithLimit(int64(perTermLimit)),
    )
    ...
}
```

### 为什么 sparse route 先查候选，而不是直接全量 BM25

因为当前数据在 Milvus 里，应用侧并没有一个常驻的全量倒排索引存储。要在不改底层存储架构的前提下落地 sparse 检索，最现实的方式就是：

1. 用 Milvus 过滤能力先缩小文档集合。
2. 在这个集合上做应用层精排。

这样能把实现难度压到一个可上线的范围。

### 为什么 `baseExpr` 要跟 `likeExpr` 拼在一起

因为 sparse route 和 dense route 必须遵守同一过滤边界。这里的：

```go
expr = fmt.Sprintf("(%s) && (%s)", baseExpr, likeExpr)
```

意思就是：

1. 先在指定知识库范围内。
2. 再要求 content 命中关键词。

这是保证混合检索“不串库、不越界”的关键。

### 为什么要做 `escapeLikeValue`

文件位置：`sparse_search.go:279-287`

如果 term 里有 `%`、`_`、引号、反斜杠，不转义会直接把表达式搞坏，严重时还会让查询语义跑偏。所以别觉得这是小细节，实际它是构造表达式时很必要的安全处理。

---

## 12. 第九步：把 Milvus 查询结果转成统一文档结构

文件：`backend/internal/milvus/retrieval/sparse_search.go`

参考位置：`sparse_search.go:173-223`

函数：

```go
func parseQueryResultSet(rs milvusClient.ResultSet) []*schema.Document
```

它主要做三件事：

1. 从结果集取出 `id`、`content`、`metadata`。
2. 还原为 `schema.Document`。
3. 把 metadata JSON 反序列化成 map。

### 为什么 sparse route 也一定要输出 `schema.Document`

因为最终 merge、rerank、tool 输出都已经围绕 `schema.Document` 在工作。如果 sparse route 自己发明另一套结构，后面每一层都要写适配代码，复杂度会迅速扩散。

这里遵循的是一个很重要的约束：**不同召回路由可以有自己的内部实现，但对外输出必须统一。**

---

## 13. 第十步：给没有稳定 ID 的文档补 pseudo ID

文件：`backend/internal/milvus/retrieval/sparse_search.go`

参考位置：`sparse_search.go:225-240`

函数：

```go
func buildPseudoDocID(doc *schema.Document) string
```

策略是：

1. 优先用 `metadata.document_id + chunk_id`
2. 其次用 `document_id`
3. 最后退化为 `content`

### 为什么这一步很重要

因为多路召回最终一定要去重，而去重的前提是“同一文档要有稳定 key”。如果某些结果没有 `doc.ID`：

1. dense 一条、sparse 一条，其实是同一 chunk。
2. merge 阶段却识别不出来。
3. 最终结果重复，rerank 和 topK 也会被污染。

所以 pseudo ID 的本质不是“补个字段”，而是在给整条链路建立去重基础。

---

## 14. 第十一步：在应用层实现显式倒排索引 + BM25

文件：`backend/internal/milvus/retrieval/sparse_inverted_index.go`

参考位置：

- `sparse_inverted_index.go:11-37`
- `sparse_inverted_index.go:39-111`
- `sparse_inverted_index.go:113-173`

### 14.1 数据结构

```go
type SparseInvertedIndex struct {
    postings     map[string][]sparsePosting
    documents    map[string]*schema.Document
    docLengths   map[string]int
    avgDocLength float64
    totalDocs    float64
    config       SparseIndexConfig
}
```

这就是一个标准的轻量倒排索引：

1. `postings` 记录 term -> doc 列表
2. `docLengths` 记录文档长度
3. `avgDocLength` 给 BM25 做归一化
4. `documents` 保留文档对象，最终返回时直接拿

### 14.2 建索引

```go
func BuildSparseInvertedIndex(docs []*schema.Document, cfg *SparseIndexConfig) *SparseInvertedIndex
```

这里对每篇文档做：

1. 计算 term frequency
2. 统计文档长度
3. 写入 posting list

`tokenizeWithFreq` 在 `sparse_search.go:289-308`，跟 query 切词保持同类规则，这一点很重要。如果 query 切词方式和 doc 切词方式差异过大，BM25 的得分就会很飘。

### 14.3 排序

```go
func (idx *SparseInvertedIndex) Search(queryTerms []string, topK int) []SparseSearchHit
```

BM25 公式核心实现是：

```go
idf := math.Log(1 + (idx.totalDocs-df+0.5)/(df+0.5))
norm := idx.config.K1 * (1 - idx.config.B + idx.config.B*(docLength/idx.avgDocLength))
scores[posting.docID] += idf * (tf * (idx.config.K1 + 1) / (tf + norm))
```

### 为什么要自己写这个 BM25，而不是只按命中次数排序

如果只按命中次数排序，会有几个明显问题：

1. 长文档天然占优，噪音更大。
2. 高频词的区分度不够。
3. 多 term query 时，相关性排序太粗糙。

BM25 至少把这些基础问题兜住了，而且实现复杂度不高，非常适合作为 L1 的 sparse rerank。

---

## 15. 第十二步：把 sparse route 串起来

文件：`backend/internal/milvus/retrieval/sparse_search.go`

参考位置：`sparse_search.go:141-170`

核心收尾逻辑：

```go
candidates := make([]*schema.Document, 0, len(merged))
for _, doc := range merged {
    candidates = append(candidates, doc)
}
index := BuildSparseInvertedIndex(candidates, nil)
hits := index.Search(terms, topK)
...
doc.MetaData["route"] = routeSparse
doc.MetaData["sparse_score"] = hit.Score
doc.MetaData["score"] = hit.Score
```

### 为什么要把 `sparse_score` 同时写到 `score`

因为后面的 merge 阶段需要统一读分数。如果 sparse route 只写 `sparse_score`，那 merge 就必须知道“这个文档来自 sparse，所以读 A 字段；另一个来自 dense，所以读 B 字段”。这样逻辑会越来越散。

所以这里做了两层处理：

1. 保留 `sparse_score`，便于分析和调试。
2. 同时写标准 `score`，便于公共流程统一读取。

这是很实用的工程手法：**既保留 route 特有信息，又提供公共消费接口。**

---

## 16. 第十三步：真正执行 dense + sparse 并发召回

文件：`backend/internal/milvus/retrieval/hybrid_search.go`

参考位置：`hybrid_search.go:117-245`

这里是 L1 的核心入口 `SearchWithRequest`。

并发执行部分：

```go
resultCh := make(chan routeResult, 2)
var wg sync.WaitGroup
wg.Add(2)

go func() {
    defer wg.Done()
    docs, err := h.searchDense(ctx, req)
    resultCh <- routeResult{route: routeDense, docs: docs, err: err, ...}
}()

go func() {
    defer wg.Done()
    docs, err := h.sparseRetriever.Search(ctx, req)
    resultCh <- routeResult{route: routeSparse, docs: docs, err: err, ...}
}()
```

### 为什么这里必须并发

因为 dense 和 sparse 的输入相同、依赖独立，没有必要串行等。串行只会平白增加延迟。

更重要的是，并发之后整条链路的认知模型会更清楚：

1. route 独立执行
2. route 独立产出结果
3. route 独立打点
4. 编排层统一汇总

这是后面继续扩多路召回的基础形状。

### 为什么用 channel 收集结果，而不是共享变量加锁

因为这里天然就是“固定数量子任务 -> 汇总结果”的模型，用 channel 表达更直观，也更不容易把路由结果和状态耦死在共享内存上。

---

## 17. 第十四步：允许单路失败，不能整链路一挂全挂

文件：`backend/internal/milvus/retrieval/hybrid_search.go`

参考位置：`hybrid_search.go:196-212`

关键逻辑：

```go
if denseErr != nil && sparseErr != nil {
    return nil, fmt.Errorf("hybrid retrieval failed: dense=%v sparse=%v", denseErr, sparseErr)
}

merged := mergeRouteCandidates(denseDocs, sparseDocs)
if len(merged) == 0 {
    return []*schema.Document{}, nil
}
```

### 为什么只在“两路都失败”时才报错

因为混合检索的目标之一，就是提升系统鲁棒性。如果 sparse 一时失败，但 dense 还能工作，这个请求从用户视角仍然可能是成功的。反过来也一样。

这类设计在生产里很关键：**多路召回不只是为了提高命中率，也是为了提高可用性。**

如果你要求任何一路失败就整个报错，那混合检索反而把系统可用性做差了。

---

## 18. 第十五步：实现多路结果合并与去重

文件：`backend/internal/milvus/retrieval/hybrid_search.go`

参考位置：`hybrid_search.go:280-325`

实现：

```go
func mergeRouteCandidates(denseDocs, sparseDocs []*schema.Document) []*schema.Document
```

逻辑分三步：

1. 先把两路结果拼到一起。
2. 用 `doc.ID` 或 pseudo ID 做唯一键。
3. 对同 key 的文档保留分数更高的一份。

对应关键代码：

```go
score := readDocScore(doc)
existing, ok := bestByKey[key]
if !ok || score > existing.score {
    bestByKey[key] = rankedDoc{doc: doc, score: score}
}
```

### 为什么这里选“保留高分版本”

因为同一文档可能被不同 route 命中，而 route 会写入不同 metadata，比如：

1. dense 命中时有 dense score
2. sparse 命中时有 sparse score

当前 L1 没做更复杂的 score fusion，所以最简单稳妥的策略就是：**同文档只保留当前分数更高的一份**。

这不是最终形态，但它足够简单，并且不会阻碍后面做加权融合。

### 为什么 `readDocScore` 同时读 `score` 和 `sparse_score`

参考位置：`hybrid_search.go:327-350`

这是为了兼容不同 route 的写分习惯，让 merge 层只关心“有没有统一可比较分数”，不关心它来自哪条路。

---

## 19. 第十六步：Rerank 放在 merge 之后，不放在 route 内部

文件：`backend/internal/milvus/retrieval/hybrid_search.go`

参考位置：`hybrid_search.go:215-224`

```go
if h.reranker != nil {
    reranked, err := h.reranker.Rerank(ctx, req.Query, merged)
    if err == nil && len(reranked) > 0 {
        merged = reranked
    }
}
if len(merged) > req.TopK {
    merged = merged[:req.TopK]
}
```

### 为什么 rerank 一定要在 merge 之后

因为 rerank 的目标是对“统一候选池”重新比较。若你在各 route 内部先 rerank：

1. dense 内部得到一个排序
2. sparse 内部得到一个排序
3. 但两者之间仍然没有统一比较标准

所以正确顺序是：

1. 先多路召回
2. 再统一 merge
3. 再统一 rerank
4. 最后 truncate

这正是标准混合检索流水线的核心次序。

---

## 20. 第十七步：给 dense route 也补 route 标识

文件：`backend/internal/milvus/retrieval/hybrid_search.go`

参考位置：`hybrid_search.go:248-267`

```go
for _, doc := range docs {
    if doc.MetaData == nil {
        doc.MetaData = make(map[string]interface{})
    }
    doc.MetaData["route"] = routeDense
}
```

### 为什么这一步不能省

因为后面你总会问这些问题：

1. 这个最终命中的文档是 dense 来的还是 sparse 来的？
2. 某一类 query 主要靠哪条 route 在救？
3. 为什么这次 sparse 打到 20 条但最后一条没留？

没有 route 标识，排障只能靠猜。

---

## 21. 第十八步：把 route 级指标补齐

文件：`backend/internal/observability/metrics/rag_metrics.go`

参考位置：

- `rag_metrics.go:21-23`
- `rag_metrics.go:63-85`
- `rag_metrics.go:161-174`

新增指标：

1. `rag_retrieve_route_requests_total`
2. `rag_retrieve_route_duration_seconds`
3. `rag_retrieve_route_hits`

暴露函数：

```go
func ObserveRetrieveRoute(route string, duration time.Duration, status, errorCode string, hitCount int)
```

### 为什么 L1 阶段就要做 route 级监控

因为混合检索如果没有 route 维度数据，后面几乎没法调：

1. 你看见整体 P95 变慢了，但不知道慢在 dense 还是 sparse。
2. 你看见命中率上升了，但不知道收益来自哪条 route。
3. 你想调 `MaxTerms`、`PerTermFactor`，却没有 sparse hit/latency 基线。

所以 route 级指标不是“Phase 3 再做”的锦上添花，它是 L1 能否继续演进的前提。

---

## 22. 第十九步：写结构化日志，而不是只打零散 log

文件：`backend/internal/milvus/retrieval/hybrid_search.go`

参考位置：

- `hybrid_search.go:198-201`
- `hybrid_search.go:208-211`
- `hybrid_search.go:227-244`

日志格式大致像这样：

```go
log.Printf(
    "[RAG:L1] request_id=%s query=%q final_query=%q expr=%q topk=%d routes=%s route_hits={dense:%d,sparse:%d} final_count=%d ...",
    ...
)
```

### 为什么这类日志要一次性把关键信息打全

因为检索问题通常不是单变量问题。一次排障经常要同时看：

1. 原始 query
2. 最终 query
3. 过滤表达式
4. topK
5. 每路命中数量
6. 每路耗时
7. 最终结果数
8. 是否有单路报错

如果这些信息分散在 6 条日志里，排障体验会很差。结构化长日志虽然看起来啰嗦，但在召回链路里非常值。

---

## 23. 第二十步：在初始化阶段把 HybridRetriever 装配起来

文件：`backend/internal/milvus/init.go`

参考位置：`init.go:149-159`

```go
candidateTopK := cfg.Milvus.TopK * 2
if cfg.RAG.Phase2.CandidateTopK > 0 {
    candidateTopK = cfg.RAG.Phase2.CandidateTopK
}
hybridConfig := &retrieval.HybridRetrieverConfig{
    CandidateTopK: candidateTopK,
    SparseConfig: &retrieval.SparseRetrieverConfig{
        DefaultTopK: candidateTopK,
    },
}
hybridRetriever, err := retrieval.NewHybridRetriever(retrieverService, hybridConfig)
```

### 为什么默认 `CandidateTopK = Milvus.TopK * 2`

因为最终返回结果的 `TopK` 和候选召回数不是一个概念。要让 rerank 和 merge 有空间，候选池通常应大于最终返回数。

乘 2 不是唯一答案，但作为 L1 默认值足够合理：

1. 比直接等于最终 TopK 更有余量。
2. 又不至于把候选拉得太大，延迟暴涨。

同时这里保留了 `cfg.RAG.Phase2.CandidateTopK` 的覆盖能力，方便后续调参。

---

## 24. 第二十一步：在工具层做 feature flag 切流

文件：`backend/internal/agents/tools/milvus_retriever_tool.go`

参考位置：

- `milvus_retriever_tool.go:56-67`
- `milvus_retriever_tool.go:93-114`

关键逻辑：

```go
useHybrid := mgr.Config != nil &&
    mgr.Config.RAG.FeatureFlags.EnableHybridRetrieval &&
    mgr.HybridRetriever != nil

documents, err := retrieveDocuments(ctxWithTimeout, mgr, query, useHybrid)
```

以及：

```go
if useHybrid {
    return mgr.HybridRetriever.Search(ctx, query, &retrieval.RetrieveOptions{
        TopK:      topK,
        RequestID: fmt.Sprintf("tool-%d", time.Now().UnixNano()),
    })
}
```

### 为什么一定要通过 feature flag 切

因为检索链路属于高影响面能力。你很难靠本地测试就完全确定线上所有 query 行为都更好，所以最稳的方式永远是：

1. 代码先具备双实现。
2. 配置决定走旧链路还是新链路。
3. 新链路出问题时可以秒回滚。

这也是为什么 L1 阶段要把兼容性保留得比较完整。

---

## 25. 第二十二步：补最小可用测试

文件：`backend/internal/milvus/retrieval/sparse_inverted_index_test.go`

参考位置：`sparse_inverted_index_test.go:9-56`

当前测试覆盖了三件关键事情：

### 25.1 BM25 排序结果是合理的

```go
func TestBuildSparseInvertedIndexSearchRanksByBM25(t *testing.T)
```

这个测试验证：

1. 命中数量正确。
2. 更相关的文档排在前面。
3. 第一名分数高于第二名。

### 25.2 没有原始 ID 时 pseudo ID 会被补上

```go
func TestBuildSparseInvertedIndexAssignsPseudoDocID(t *testing.T)
```

这个测试不是小题大做。它保证 merge/dedupe 的基础设施是成立的。

### 25.3 空输入不会 panic，也不会返回脏数据

```go
func TestBuildSparseInvertedIndexSearchHandlesEmptyInput(t *testing.T)
```

L1 阶段测试不用一口气铺满所有集成场景，但一定要守住这些核心不变量。

---

## 26. 建议你按什么顺序自己实现

如果后面的同学要从头照着做，最稳的顺序不是按文件名来，而是按依赖关系来：

1. 扩展 `RetrieveOptions`
2. 确认 `BuildFilterExpr` 能提供统一过滤表达式
3. 新增 `HybridSearchRequest`
4. 写 `SparseRetrieverConfig` 和 `SparseRetriever`
5. 写 `extractSparseTerms / parseQueryResultSet / buildPseudoDocID`
6. 写 `BuildSparseInvertedIndex` 和 `Search`
7. 把 sparse route 跑通
8. 改造 `HybridRetriever.Search`
9. 新增 `SearchWithRequest`
10. 加并发执行 dense + sparse
11. 加 merge/dedupe
12. 接入 rerank
13. 加 route metrics 和结构化日志
14. 在 `init.go` 装配
15. 在 tool 层用 feature flag 接入
16. 补单测

这个顺序的核心原则是：**先建能力，再建编排，最后再接流量入口。**

---

## 27. 这版实现的优点和边界

### 优点

1. 改动集中，风险可控。
2. 保留旧接口，兼容性好。
3. sparse 路由完全应用层可控，便于测试和迭代。
4. 已经具备多路召回的标准骨架。
5. 日志和监控提前到位，后面调优有抓手。

### 边界

1. sparse 候选仍依赖 Milvus `like`，不是原生全文索引。
2. 目前 merge 只是“高分保留”，还不是严格 score fusion。
3. 分词逻辑较朴素，中文和复杂 query 还有限制。
4. rerank 目前还是已有实现，没有针对 mixed score 做更细致处理。

这些边界不是问题本身，它们只是说明：**L1 的任务是把路修出来，不是一次性把效果打满。**

---

## 28. 后续可以怎么继续演进

基于这版代码，后面继续做优化时，路径会比较自然：

1. `mergeRouteCandidates` 升级为加权融合或 RRF。
2. `extractSparseTerms` 升级为更好的分词、同义词、缩写扩展。
3. `SparseRetriever` 升级为更高效的候选获取方式。
4. rerank 阶段加入更多 route 特征。
5. 结合 route metrics 做离线评测和在线调参。

也就是说，这版代码的价值不只在“它现在能跑”，更在于“它给后面的 Phase 2 留了稳定演进面”。

---

## 30. 如果你是第一次接触这套实现，建议先记住这 5 句话

最后再给小白版总结一下。你只要先记住下面 5 句话，再回头看代码就不会那么晕：

1. `dense` 是“按语义找相近内容”的向量检索。
2. `sparse` 是“按关键词命中找内容”的关键词检索。
3. `term` 就是从 query 里拆出来的关键词。
4. `TopK` 是最终想返回多少条结果。
5. `topK * factor` 是“为了后面排序更稳，前面先多拿一些候选”。

如果把整条链路讲成一句最朴素的话，就是：

**先让向量检索和关键词检索各自去找一批可能相关的文档，再把这些文档合并、去重、重排，最后返回最好的几条。**

---

## 29. 给接手同学的落地提醒

最后给几个非常实际的提醒：

1. 不要一上来就改调用方接口，先保兼容。
2. 不要让 sparse route 绕开统一 filter。
3. 不要把 route 特有字段和公共字段混成一团，像 `route`、`sparse_score`、`score` 这种分层要保留。
4. 不要省掉 `RequestID`、metrics、结构化日志，这些不是“后面再补”的装饰。
5. 不要把 mixed retrieval 做成“任何一路失败就整体失败”。

如果照着这份教程实现，最终你拿到的就不只是一个“能搜到点东西”的 sparse 功能，而是一条可以上线、可以观察、可以继续迭代的 L1 混合检索召回链路。
