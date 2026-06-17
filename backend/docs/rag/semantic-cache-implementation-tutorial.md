# Semantic Cache（语义缓存）详细实现教程

## 1. 文档定位

本文档是 Semantic Cache 的实现大纲与执行手册。
目标不是立刻写完功能代码，而是先把这项能力拆成可以分阶段落地、验证、回滚和提交的实施路线。

它有四个用途：

1. 作为 Semantic Cache 功能的统一设计入口。
2. 作为后续 `L0 -> L6` 逐层实现的执行参考。
3. 作为测试、验收、回滚和灰度发布的边界定义。
4. 作为团队协作和分工的冻结文档。

本文档风格与现有实现教程保持一致：

`背景 -> 范围 -> 路线 -> 模块任务 -> Gate -> 测试 -> 回滚 -> 协作`

统一口径说明：

1. `Semantic Cache` 固定指：对“语义相近且上下文边界一致”的检索请求，直接复用历史检索结果，而不是重新执行完整检索链路。
2. `缓存命中` 固定指：在隔离边界、策略边界、知识库边界都满足的前提下，查询 embedding 与历史缓存项相似度达到阈值，并成功返回缓存结果。
3. `缓存边界` 固定指：至少按 `tenant_id + kb_ids + strategy_version + query_scope` 隔离，禁止跨租户、跨知识库、跨策略误命中。
4. `缓存结果` 在本阶段固定指：`retrieve` 结果，而不是最终 LLM 回答。
5. `缓存回填` 固定指：请求未命中时，在正常检索成功后把本次结果写入语义缓存。

---

## 2. 背景

当前项目已经具备比较完整的检索主链路：

1. Dense / Sparse 混合检索
2. Fusion / Dedupe / Rerank
3. Query Rewrite
4. Dynamic TopK
5. Parent-Child
6. Evidence Gate
7. Citation Consistency

同时项目也已经具备三类对 Semantic Cache 很重要的基础设施：

1. Redis 客户端和基础 KV 能力
2. 检索请求日志 `KBRetrieveLog`
3. 成本追踪 `KBCostTrace`

这意味着我们不需要从零开始搭一个缓存系统，而是可以在现有平台上补出一条新的“缓存短路路径”：

`请求进入 -> 语义缓存判定 -> 命中则直接返回缓存结果 -> 未命中则走原检索链路 -> 成功后回填缓存`

Semantic Cache 的价值主要有三点：

1. 降低重复相似 query 的检索延迟。
2. 降低重复请求带来的检索和重排成本。
3. 在高频问题场景下提升系统稳定性和吞吐。

---

## 3. 当前现状与接入点

基于现有仓库，Semantic Cache 最适合接在以下位置：

1. `backend/api/handler/kb/handler.go`
   - 检索请求主入口
   - 适合做缓存查找、命中短路、未命中回填
2. `backend/internal/milvus/retrieval/search.go`
   - 已有统一的 `SearchMetrics` / `SearchResult`
   - 适合扩展缓存相关指标字段
3. `backend/internal/repository/redis.go`
   - 已有 Redis 初始化和基础读写能力
   - 可作为缓存存储的最小基础设施入口

现状限制也很明确：

1. 现在没有现成的 Semantic Cache 模块。
2. 现在的 Redis 能力偏基础，没有缓存索引协议。
3. 当前检索日志与成本日志里还没有缓存命中字段。
4. 当前没有现成的缓存失效和缓存清理治理逻辑。

---

## 4. 设计目标

Semantic Cache 本阶段目标不是“做一个无限复杂的缓存平台”，而是先做出一个可上线、可观察、可回滚的最小可用版本。

本阶段目标：

1. 为 `/api/kb/retrieve` 和复用同链路的检索请求增加语义缓存能力。
2. 命中时直接返回缓存结果，绕过后续昂贵检索链路。
3. 未命中时正常执行原流程，并把成功结果写回缓存。
4. 整个能力必须可开关、可观测、可回滚。
5. 必须保证多租户、知识库、策略版本之间严格隔离。

非目标：

1. 不做最终 LLM Answer Cache。
2. 不做跨服务共享的复杂多级缓存体系。
3. 不做自动学习阈值。
4. 不做无门禁的全量灰度。
5. 不做缓存替代评测门禁。

---

## 5. 技术方案总览

本次建议采用：

`Redis 存 payload + Redis 存轻量索引 + 应用侧计算相似度`

原因：

1. 仓库里已经稳定使用 Redis。
2. 不新增新的基础设施依赖，实施成本低。
3. 便于先做小范围可控版本。
4. 适合先验证收益，再决定是否升级为更重的向量缓存索引。

### 5.1 缓存项建议结构

每条缓存项最小包含：

1. `cache_entry_id`
2. `tenant_id`
3. `kb_scope`
4. `kb_ids`
5. `strategy_version`
6. `retriever_version`
7. `query`
8. `query_embedding`
9. `response_payload`
10. `top_k`
11. `created_at`
12. `expires_at`
13. `hit_count`
14. `last_hit_at`

### 5.2 建议命中流程

1. 请求进入检索入口。
2. 判断当前是否允许缓存：
   - 开关是否开启
   - query 是否为空
   - 当前请求是否属于可缓存范围
3. 生成当前 query embedding。
4. 从当前隔离边界对应的候选缓存集合中拉取最近 N 条缓存项。
5. 在应用侧计算 cosine similarity。
6. 找到最高相似且超过阈值的缓存项。
7. 命中则反序列化结果，补齐 metrics/log 并直接返回。
8. 未命中则走正常检索链路。
9. 正常检索成功后，把结果写回缓存。

### 5.3 建议隔离维度

缓存命中至少受以下字段约束：

1. `tenant_id`
2. `kb_ids`
3. `strategy_version`
4. `query_type`
5. `top_k` 或可兼容的 TopK 范围

### 5.4 建议失效策略

最小版本先采用：

1. TTL 过期
2. 按知识库变更主动失效
3. 按策略版本变化隔离失效

---

## 6. 范围边界

## 6.1 本阶段必须完成

1. 语义缓存 Feature Flag 与配置接入。
2. 检索入口缓存命中和未命中回填主链路。
3. Redis 存储协议和候选索引协议。
4. 检索日志、成本日志、调试信息补充缓存字段。
5. 缓存命中/未命中/误命中风险测试。
6. 缓存失效与回滚策略。

## 6.2 本阶段明确不做

1. LLM 最终回答缓存。
2. 多级缓存（内存 + Redis + 向量库）联动。
3. 自适应阈值学习。
4. 基于用户反馈自动淘汰缓存。
5. 独立的缓存后台管理页面。

---

## 7. 实现路线总览（L0 -> L6）

建议按 7 条路线推进：

1. L0：配置、边界和命中契约冻结
2. L1：缓存数据结构与 Redis 协议
3. L2：检索入口缓存查询与短路返回
4. L3：未命中回填、失效和清理策略
5. L4：日志、指标、成本与调试链路
6. L5：测试、Gate、灰度与回滚
7. L6：文档收口与验收报告

建议顺序：

`L0 -> L1 -> L2 -> L3 -> L4 -> L5 -> L6`

---

## 8. 详细路线拆解

## 8.1 L0 配置、边界和命中契约冻结

### 目标

先把 Semantic Cache 的边界说清楚，避免后续实现中出现跨租户误命中、跨策略污染或命中规则漂移。

### 功能任务

1. 在配置层增加 Semantic Cache 开关和参数：
   - `enable_semantic_cache`
   - `semantic_cache_similarity_threshold`
   - `semantic_cache_ttl_seconds`
   - `semantic_cache_max_candidates`
   - `semantic_cache_max_entries_per_scope`
2. 冻结缓存隔离边界：
   - `tenant_id`
   - `kb_ids`
   - `strategy_version`
   - `query_type`
3. 冻结缓存命中契约：
   - 命中阈值定义
   - TopK 兼容策略
   - 结果 payload 契约
4. 冻结哪些请求不允许缓存：·  
   - query 为空
   - 显式 debug 模式
   - 权限结果非正常
   - 高风险试验流量

### 验收

1. 配置缺失或阈值非法时 fail-fast。
2. 命中契约有明确文档定义。
3. 开关关闭时行为与当前版本完全一致。

## 8.2 L1 缓存数据结构与 Redis 协议

### 目标

建立一个最小但可维护的缓存存储协议。

### 功能任务

1. 新增语义缓存模块目录，例如：
   - `backend/internal/cache/semantic/`
2. 定义缓存实体：
   - `SemanticCacheEntry`
   - `SemanticCacheLookupResult`
3. 设计 Redis key 规则：
   - scope key
   - entry key
   - optional index key
4. 定义 payload 序列化格式：
   - query
   - embedding
   - retrieve response
   - metrics snapshot
5. 定义淘汰规则：
   - TTL
   - 最近最少使用近似策略
   - 超量裁剪

### 验收

1. 可正确写入、读取、删除缓存项。
2. Redis key 结构稳定、可追踪。
3. 超量时能按规则裁剪旧缓存项。

## 8.3 L2 检索入口缓存查询与短路返回

### 目标

在真正检索前先做语义缓存判定，命中时直接短路返回。

### 功能任务

1. 在 `handler.go` 检索主入口增加缓存查询阶段。
2. 生成当前 query embedding。
3. 拉取当前 scope 下最近 N 个候选缓存项。
4. 计算相似度并判定是否命中。
5. 命中时：
   - 构造 retrieve response
   - 标记缓存命中 metrics
   - 写入 retrieve log
   - 返回结果
6. 未命中时继续原始链路。

### 验收

1. 相同 query 能稳定命中缓存。
2. 相似 query 在超过阈值时可命中。
3. 不同租户 / 不同知识库请求不会误命中。

## 8.4 L3 未命中回填、失效和清理策略

### 目标

让缓存不仅能查，还能持续被安全回填和失效。

### 功能任务

1. 在正常检索成功后写回缓存。
2. 对以下情况禁止回填：
   - 检索失败
   - 空结果
   - Evidence Gate 拒答
   - 权限异常
3. 建立主动失效规则：
   - 文档上传 / 删除后，按 KB 作用域失效
   - 策略版本切换后按版本隔离
4. 建立缓存清理任务：
   - TTL 扫描
   - 超量清理

### 验收

1. 未命中后再次请求可命中。
2. 知识库内容变化后旧缓存不会继续污染结果。
3. 无效结果不会写入缓存。

## 8.5 L4 日志、指标、成本与调试链路

### 目标

让 Semantic Cache 成为一个可观测能力，而不是黑盒优化。

### 功能任务

1. 扩展 `SearchMetrics`：
   - `SemanticCacheEnabled`
   - `SemanticCacheHit`
   - `SemanticCacheLookupMs`
   - `SemanticCacheSimilarity`
   - `SemanticCacheEntryID`
2. 扩展 `KBRetrieveLog`：
   - `semantic_cache_hit`
   - `semantic_cache_similarity`
   - `semantic_cache_lookup_ms`
   - `semantic_cache_reason`
3. 扩展 `KBCostTrace`：
   - `cache_saved_retrieval_cost`
   - `cache_saved_rerank_cost`
4. 扩展调试视图：
   - 是否命中缓存
   - 命中阈值
   - 候选数量
   - 最高相似度

### 验收

1. 命中率、节省成本、平均查找耗时可观测。
2. retrieve log 能区分缓存命中与真实检索。
3. 调试视图可解释为什么命中或未命中。

## 8.6 L5 测试、Gate、灰度与回滚

### 目标

确保语义缓存不会因为“命中很快”而牺牲正确性和安全边界。

### 功能任务

1. 单元测试：
   - 相似度计算
   - 阈值判定
   - scope key 生成
   - Redis payload 读写
2. 集成测试：
   - 命中短路
   - 未命中回填
   - 文档变更后失效
3. 回归测试：
   - 关闭缓存后原链路不变
   - 日志与成本链路不破坏
4. 灰度发布：
   - 内部环境
   - 小流量灰度
   - 逐步放量
5. 回滚方案：
   - 关闭 `enable_semantic_cache`
   - 停止回填
   - 保留 Redis 数据但不再读取

### 验收

1. 命中率有收益且误命中可控。
2. P95 延迟下降或高频 query 显著改善。
3. 关闭开关后系统回到现有行为。
4. 回滚路径 10 分钟内可执行。

## 8.7 L6 文档收口与验收报告

### 目标

把 Semantic Cache 从“做完一堆代码”收口成“能交付、能复盘、能继续迭代”的完整能力。

### 功能任务

1. 输出实现教程正式版。
2. 输出测试报告。
3. 输出灰度与回滚记录。
4. 输出收益总结：
   - 命中率
   - 平均耗时收益
   - 成本收益
   - 风险样本

### 验收

1. 新同学能按文档复盘完整实现路径。
2. 验收报告可支撑后续继续扩展 Answer Cache 或多级缓存。

### 当前落地产物

当前代码和文档侧已经补齐以下 L6 产物：

1. 管理接口：
   - `GET /api/admin/kb/semantic-cache/report`
2. 验收报告：
   - `backend/docs/rag/semantic-cache-l6-acceptance-report.md`
3. 会议讲稿：
   - `backend/docs/rag/semantic-cache-l0-to-l6-meeting-brief.md`

---

## 9. Gate

Semantic Cache 功能通过标准：

1. 缓存开关关闭时，行为与当前版本完全一致。
2. 缓存命中只发生在正确隔离边界内。
3. 相同或高相似 query 命中后，结果契约稳定不变。
4. 知识库变更后，旧缓存不会继续污染新结果。
5. 检索日志、成本日志、调试视图都能解释缓存行为。
6. 出现异常时可一键关闭缓存并恢复原链路。

---

## 10. 测试方案

## 10.1 单元测试

1. 相似度计算正确性
2. 命中阈值边界
3. Redis key 协议
4. payload 序列化 / 反序列化
5. 淘汰和 TTL 行为

## 10.2 集成测试

1. 首次请求未命中 -> 正常检索 -> 回填成功
2. 第二次请求命中 -> 直接返回缓存
3. 不同 tenant / kb / strategy 不互串
4. 文档上传或删除后缓存失效

## 10.3 回归测试

1. 不影响当前 Hybrid / Parent-Child / Evidence Gate 主链路
2. 不破坏 retrieve log / cost trace / debug trace
3. 关闭开关后全部回到原链路

## 10.4 验收指标

1. `semantic_cache_hit_rate`
2. `semantic_cache_lookup_p95_ms`
3. `semantic_cache_false_hit_count`
4. `semantic_cache_saved_retrieval_cost`
5. `semantic_cache_saved_rerank_cost`

---

## 11. 风险点

Semantic Cache 的风险比普通 KV Cache 更高，必须提前写清楚。

关键风险包括：

1. 跨租户误命中
2. 跨知识库误命中
3. 策略版本变化后命中旧结果
4. 文档更新后命中陈旧结果
5. 相似度阈值过低导致错误结果被复用
6. 缓存查找本身太慢，收益被抵消

对应原则：

1. 先保守隔离，再追求命中率。
2. 先保守阈值，再追求覆盖面。
3. 先可回滚，再追求激进优化。

---

## 12. 协作分工建议

为了避免上下文爆炸和实现串线，建议拆成 3 条并行线：

1. Worker A：缓存核心模块
   - 负责 `internal/cache/semantic`
   - 负责 Redis 协议、相似度判定、读写封装
2. Worker B：检索入口集成
   - 负责 `handler.go`、metrics、日志、cost trace 集成
   - 负责命中短路与回填主链路
3. Worker C：测试与文档
   - 负责测试用例、验收脚本、教程与报告

主线程负责：

1. 冻结契约
2. 合并改动
3. 跑验证
4. 每个 L 层完成后提交中文 commit

---

## 13. 实施与提交节奏

后续真正开始写功能代码时，我会按下面的方式执行：

1. 先完成一个 L 层。
2. 跑对应验证。
3. 验证通过后再本地 `git commit`。
4. 提交信息用中文。
5. 不推送远程仓库。

建议提交节奏如下：

1. `冻结 Semantic Cache 配置与命中契约（L0）`
2. `实现 Semantic Cache Redis 协议与缓存实体（L1）`
3. `接入检索入口语义缓存查询与短路返回（L2）`
4. `补充语义缓存回填与失效策略（L3）`
5. `接入语义缓存日志指标与成本追踪（L4）`
6. `完成语义缓存测试灰度与回滚验证（L5）`
7. `完善语义缓存教程与验收报告（L6）`

---

## 14. 当前结论

Semantic Cache 可以在这个仓库里完整落地，而且适合按分层需求推进。

当前最合理的推进方式不是立刻写功能代码，而是：

1. 先冻结这份大纲和阶段边界。
2. 再从 `L0` 开始逐层实现。
3. 每层单独验证、单独提交。
4. 用多 agent 拆开“缓存核心 / 主链路集成 / 测试文档”三条线，控制上下文规模。

这份文档之后，正式实现应以本文件为总参考。
