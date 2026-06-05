# Semantic Cache L1 数据结构与 Redis 协议设计说明

## 1. 文档目的

这份文档解释 Semantic Cache 的 `L1：缓存数据结构与 Redis 协议` 是怎么实现的，以及为什么要这样设计。

它重点回答 4 个问题：

1. 这次 L1 改了哪些代码
2. 为什么要先做数据结构和 Redis 协议，而不是直接接到检索入口
3. L1 和 L0 是怎么衔接起来的
4. 这一步完成后，后续 L2/L3 能直接复用什么

---

## 2. L1 的目标是什么

L0 解决的是“规则先冻结”。

L1 要解决的是另一个问题：

如果后面真的要把 Semantic Cache 接进主链路，那么缓存里到底存什么、Redis key 长什么样、scope 如何映射到 key、payload 如何序列化、容量超限时怎么裁剪，这些都必须先定下来。

所以 L1 的目标不是让缓存“开始命中”，而是让缓存“有一个稳定可维护的存储协议”。

可以把 L1 理解成一句话：

先把缓存仓库和货架搭好，再考虑什么时候开始往里面放货、什么时候开始从里面取货。

---

## 3. 这次具体改了什么

本次 L1 新增了一个独立模块：

`backend/internal/cache/semantic/`

主要文件有：

1. [types.go](/d:/RAG/rag-retrievalOps/backend/internal/cache/semantic/types.go)
2. [keys.go](/d:/RAG/rag-retrievalOps/backend/internal/cache/semantic/keys.go)
3. [codec.go](/d:/RAG/rag-retrievalOps/backend/internal/cache/semantic/codec.go)
4. [store.go](/d:/RAG/rag-retrievalOps/backend/internal/cache/semantic/store.go)
5. [store_test.go](/d:/RAG/rag-retrievalOps/backend/internal/cache/semantic/store_test.go)

### 3.1 定义了 Scope

`Scope` 是缓存隔离边界的代码表达。

```go
type Scope struct {
    TenantID        uint64
    KBIDs           []uint64
    StrategyVersion string
    QueryType       string
}
```

这个结构和 L0 的契约完全对齐，表示 Redis 里的缓存一定是按这几个维度隔离的。

### 3.2 定义了 Entry

`Entry` 是单条语义缓存记录。

里面包含：

1. `EntryID`
2. `TenantID`
3. `KBScope`
4. `KBIDs`
5. `StrategyVersion`
6. `RetrieverVersion`
7. `QueryType`
8. `Query`
9. `QueryEmbedding`
10. `ResponsePayload`
11. `ResultPayload`
12. `TopK`
13. `CreatedAt`
14. `ExpiresAt`
15. `HitCount`
16. `LastHitAt`

这基本就是把教程里 L1 提到的缓存实体正式代码化了。

### 3.3 定义了 LookupResult

`LookupResult` 用于表达“某个 scope 下拿到的一批候选缓存项”。

它不是最终命中结果，而是给后续 L2 做候选比对用的：

```go
type LookupResult struct {
    Scope          Scope
    Candidates     []*Entry
    CandidateCount int
}
```

### 3.4 定义了 Redis Key 规则

在 `keys.go` 里定义了两类 key：

1. `scope key`
2. `entry key`

#### scope key

示例格式：

```text
rag:semantic_cache:scope:t12:k2-4-9:sphase4-semantic-v1:qretrieve
```

它表示某个租户、某组知识库、某个策略版本、某种 query type 下面的候选缓存集合。

#### entry key

示例格式：

```text
rag:semantic_cache:entry:<entry_id>
```

它表示某一条具体缓存记录。

### 3.5 定义了稳定的 EntryID 生成规则

`BuildEntryID()` 会根据下面这些内容生成稳定 ID：

1. `tenant_id`
2. `kb_ids`
3. `strategy_version`
4. `query_type`
5. `top_k`
6. `query`

然后做 hash，得到固定 entry id。

这意味着：

同一 scope、同一 query、同一 top_k，会映射到同一条缓存记录。

### 3.6 定义了序列化协议

在 `codec.go` 里实现了：

1. `EncodeEntry()`
2. `DecodeEntry()`

也就是说，L1 把“Entry 在 Redis 里长什么样”固定成了 JSON 协议。

### 3.7 定义了 Store

`Store` 是 Redis 读写的统一封装层。

它现在提供了 4 个核心能力：

1. `Put()`：写入缓存项
2. `GetCandidates()`：按 scope 取候选缓存项
3. `Touch()`：命中后更新 hit_count / last_hit_at / ttl
4. `DeleteScope()`：按 scope 删除缓存

---

## 4. Redis 协议是怎么设计的

L1 不是简单地用一个 `GET/SET` 把整个 cache 做掉，而是用了两层结构：

1. `entry key` 存 payload
2. `scope key` 存候选索引

### 4.1 entry key 存 payload

每条缓存记录会以 JSON 形式落到：

```text
rag:semantic_cache:entry:<entry_id>
```

里面存完整的 `Entry`。

这样做的好处是：

1. 单条记录结构完整
2. 序列化稳定
3. 后续想扩字段更容易

### 4.2 scope key 存候选索引

每个 scope 对应一个 Redis ZSet。

ZSet 的 member 是 `entry_id`，score 是 `created_at.Unix()`。

这样做后，我们就可以：

1. 先按 scope 拿最近 N 条候选缓存项
2. 再逐条读 entry payload
3. 最后在应用层做相似度比较

这正好贴合 Semantic Cache 的使用模式。

---

## 5. 为什么不是直接用一个大 JSON 列表

这是一个很典型的问题。

为什么不直接一个 key 下面塞一个大数组，里面放所有候选项？

因为那样会带来几个问题：

1. 每次写入都要整体反序列化再整体回写
2. 单条记录无法单独更新 hit_count
3. 容量一大之后，scope 级 payload 会越来越重
4. 后续清理和淘汰会很难做

用“索引 + 明细”的两层结构以后：

1. 候选集合管理更轻
2. 单条 entry 能独立更新
3. 后续扩展统计字段更方便
4. 更适合后面做淘汰和主动失效

---

## 6. 为什么 scope 要先 normalize

`Scope.Normalize()` 做了两件事：

1. 对 `KBIDs` 排序
2. 对 `KBIDs` 去重

原因是：

同一个逻辑 scope 不能因为参数顺序不同而变成两个 Redis key。

例如：

1. `kb_ids = [5,1]`
2. `kb_ids = [1,5]`

这两个请求在语义上是同一个 scope，如果不 normalize，就会把同一个缓存空间切成两份。

这会导致：

1. 命中率被人为打散
2. 容量管理混乱
3. 排查问题时难以理解

所以 normalize 不是优化，而是协议稳定性的必要步骤。

---

## 7. 为什么 Put 时要重新回填 scope 信息

在 `Store.Put()` 里，写入前会把 entry 的以下字段按 scope 强制对齐：

1. `TenantID`
2. `KBIDs`
3. `KBScope`
4. `StrategyVersion`
5. `QueryType`

原因是：

缓存 entry 的边界必须以调用时传入的 scope 为准，而不是信任外部传进来的任意 entry 字段。

这样做能避免：

1. entry 自己带的 scope 信息和真实 scope 不一致
2. 后续开发误把不同 scope 的 entry 写进同一个空间
3. 产生难排查的数据污染

---

## 8. 为什么要单独存 `ResponsePayload`

`ResponsePayload` 目前是 `json.RawMessage`。

这样设计是为了满足两件事：

1. L1 不提前耦合具体 retrieve response 结构
2. L2 可以直接把真实检索结果序列化后塞进来

这意味着 L1 只负责“存储协议”，不负责“业务结构解释”。

这是刻意的分层。

如果 L1 现在就强绑定某个 handler response 结构，会出现两个问题：

1. 缓存层和 API 层耦合太早
2. 后面一改返回结构，缓存模块也要跟着动

---

## 9. 为什么 `ResultPayload` 要固定成 `retrieve_result_only`

这一步是和 L0 强绑定的。

L0 已经明确约束：当前阶段只缓存 retrieve 结果，不缓存最终 LLM answer。

所以 L1 在 `Entry.Validate()` 里也把这个约束落成代码：

1. 如果为空，补默认值 `retrieve_result_only`
2. 如果不是这个值，直接报错

这代表：

L0 的规则不只是文档约定，L1 的存储层也在强制执行。

这样后面即使有人误把 answer payload 写进来，也会在缓存层被挡住。

---

## 10. 为什么要做 Touch

`Touch()` 的作用是：

1. 增加 `hit_count`
2. 更新 `last_hit_at`
3. 刷新 entry TTL

这一步虽然真正会在 L2 命中时使用，但 L1 先把能力准备好很重要。

因为后面很多治理逻辑都依赖这些字段：

1. 哪些缓存经常命中
2. 哪些缓存长期没人用
3. 哪些 entry 值得保留

换句话说，`Touch()` 是后续缓存治理的基础动作。

---

## 11. 为什么要在 Put 里做超量裁剪

L0 里已经引入了 `max_entries_per_scope`。

L1 不能只把这个字段留在配置里而不落地，所以 `Put()` 会在写完之后检查 scope 的候选数量，如果超限，就裁剪最旧的 entry。

这么做有三个好处：

1. scope 容量受控
2. Redis 空间不会无限增长
3. 候选集合查找成本可控

这里先做的是最简单、最保守的策略：

按 `created_at` 淘汰最旧的记录。

这不一定是未来最终最优策略，但它是一个稳定可解释的起点。

---

## 12. 为什么 L1 不直接实现“命中判断”

因为命中判断属于 L2，不属于 L1。

L1 只负责：

1. 数据怎么存
2. 候选怎么取
3. 淘汰怎么做
4. entry 怎么更新

而命中判断还需要：

1. query embedding
2. 候选向量比对
3. similarity threshold
4. topk 匹配规则
5. 命中后的 handler 短路返回

这些都属于主链路逻辑，不应该提前塞进存储层。

这样分层后，L1 更稳定，L2 也更容易开发。

---

## 13. L1 和 L0 是怎么衔接的

这是你汇报时最值得讲清楚的一部分。

### 13.1 L0 负责“规则冻结”

L0 给出的东西包括：

1. `enable_semantic_cache`
2. `similarity_threshold`
3. `ttl_seconds`
4. `max_candidates`
5. `max_entries_per_scope`
6. scope 契约
7. bypass 契约
8. payload 契约
9. TopK 契约

### 13.2 L1 负责“把规则变成存储协议”

L1 对 L0 的承接关系是：

1. L0 的 scope 契约 -> L1 的 `Scope`
2. L0 的 payload 契约 -> L1 的 `ResultPayload`
3. L0 的 `ttl_seconds` -> L1 的 entry TTL / scope TTL
4. L0 的 `max_candidates` -> L1 的 `GetCandidates()`
5. L0 的 `max_entries_per_scope` -> L1 的裁剪逻辑

可以这样理解：

L0 决定缓存该遵守什么规则。
L1 决定这些规则在 Redis 里怎么真正落地。

### 13.3 为什么说 L1 是 L0 的“执行层”

因为如果只有 L0，没有 L1，那些配置和契约还只是“系统知道该怎么做”。

而 L1 做完后，系统才真正具备了：

1. 生成 scope key 的能力
2. 落 entry payload 的能力
3. 按 scope 拿候选的能力
4. 根据容量做裁剪的能力

也就是说，L1 让 L0 从“规则定义”走到了“协议可执行”。

---

## 14. 这次为什么单元测试很重要

L1 的单测重点不在业务语义，而在协议稳定性。

这次测试覆盖了：

1. `ScopeKey()` 是否会做排序去重
2. `BuildEntryID()` 是否稳定
3. `EncodeEntry()/DecodeEntry()` 是否对称
4. `Put()` 是否能正确写入
5. `GetCandidates()` 是否能正确返回候选
6. `Touch()` 是否能更新 hit_count
7. `DeleteScope()` 是否能清空 scope
8. scope 满时是否会裁剪最旧 entry

这些测试的意义在于：

后面就算 L2/L3 接入主链路，底层协议也不会轻易被改坏。

---

## 15. 怎么向组长汇报

你可以这样说：

> L0 我先把配置和命中契约冻结了，L1 我把这套规则真正落成了独立的缓存存储协议。  
> 这次新建了 `internal/cache/semantic` 模块，定义了 scope、entry、lookup result、Redis key 规则、entry 编解码和 store 封装。  
> Redis 协议采用“scope 索引 + entry payload”两层结构：scope 用 ZSet 管理候选 entry，entry 用 JSON 存完整缓存内容。  
> 这样做能保证 scope 隔离稳定、单条记录可独立更新、容量可控，并且能直接支撑后续 L2 做候选拉取和命中判断。  
> 另外我把 L0 的 `ttl_seconds`、`max_candidates`、`max_entries_per_scope` 都落到了存储层，并补齐了单测验证协议稳定性。

如果组长追问“为什么不直接接 handler”，你可以补一句：

> 因为如果存储协议没先定下来，L2 一接主链路就会边开发边改 Redis 结构，后面返工成本会很高。现在先把 L1 收敛住，L2 只需要专注候选查询和命中判断。

---

## 16. 当前结论

L1 的价值，不是“缓存已经能命中”。

L1 的价值是：

系统已经具备了稳定的 Semantic Cache 存储骨架。

更准确地说，L0 解决了“应该怎么做”，L1 解决了“在 Redis 里具体怎么放”。

这样到了 L2，我们就不需要再争论 entry 长什么样、scope key 怎么拼、候选从哪拿，而是可以直接开始做真正的缓存查询与短路逻辑。
