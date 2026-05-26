# Phase 2 检索质量优化项目复盘分享讲稿

## 1. 分享开场

大家好，今天我想分享的是我们最近这一轮 RAG 项目迭代，也就是 Phase 2 检索质量优化的完整过程。这个分享我会重点讲六件事：

1. 我们为什么要做这一轮迭代。
2. 这一轮到底做了哪些功能，代码是怎么落下去的。
3. 过程中踩过哪些坑，后来是怎么修正的。
4. 我们是怎么做调优，而不是凭感觉改参数的。
5. 我们是怎么先写文档，再基于文档用 vibe coding 把代码实现出来的。
6. 最后如果把这段经历放到面试里，应该怎么讲，才能体现工程能力而不只是“我写了几个功能”。

如果用一句话概括这次迭代，我觉得最合适的表达是：

**我们不是在给 RAG 多塞几个策略，而是在把“检索”从一个能跑的功能，升级成一个可灰度、可回滚、可评测、可解释的工程系统。**

---

## 2. 项目背景

在 Phase 1 的时候，我们的检索主链路本质上还是单路 dense 检索。它能解决一部分语义相近的问题，但一到下面这些场景就容易掉点：

1. 实体词、缩写词，比如 `MVCC`、`JVM GC`、`RPC`。
2. 别名和不同写法，比如 `golang` 和 `go`，`spring boot` 和 `springboot`。
3. 用户 query 很短，但是问得很准，这时 dense 召回容易信息不足。
4. 用户 query 很长、很泛，这时固定 TopK 又会把很多无效上下文塞给大模型，成本和噪音都上来。

所以到了 Phase 2，我们的目标已经不是“把 topK 调大一点”这么简单，而是系统性解决四类问题：

1. 召回率不够。
2. 结果排序不稳。
3. token 成本不可控。
4. 策略上线缺少评测和回滚闭环。

这也是为什么我们的 Phase 2 路线图不是一篇泛泛而谈的方案文档，而是拆成了 L0 到 L8 九条路线，覆盖配置开关、混合检索、融合去重、改写、动态 TopK、重排、索引调优、离线评测、灰度回滚。

---

## 3. 这轮迭代的主线

如果让我把整个迭代过程浓缩成一条主线，我会这样讲：

第一步，不是先写策略，而是先把安全底座补齐。也就是先做 L0，让所有新策略都能独立开关、能打印策略快照、能启动校验 fail-fast、能保留 Phase 1 基线快照。这个能力主要落在 `backend/internal/config/config.go` 和 `backend/cmd/server/main.go`。

第二步，我们把单路 dense 检索升级成标准混合检索流水线，也就是 `dense + sparse -> fusion -> dedupe -> rerank -> filter/truncate`。这条链路的核心入口在 `backend/internal/milvus/retrieval/hybrid_search.go`。

第三步，我们补上质量增强策略，但所有增强都要求“受控”。所以 query rewrite 不是任意改写，而是可关闭、可限流、可超时回退、可黑名单跳过。动态 TopK 也不是拍脑袋，而是规则决策加 token 预算守卫。

第四步，我们不允许策略改完直接上线，所以又补了 L6 和 L7：一边做索引参数扫描，一边做离线评测和门禁，让召回率、MRR、nDCG、Citation Accuracy 和 P95 延迟都能对比。

第五步，我们把发布思路做成可回滚的工程动作，而不是“上线后看运气”。所以这一轮的关键词其实不是某个单点算法，而是“质量工程化”。

---

## 4. 核心功能是怎么实现的

下面我会用比较适合会议分享，也适合面试表达的方式，把几个核心功能串起来讲。

### 4.1 L0：先做策略开关、配置校验、快照和回滚基础

这一层很容易被忽略，但我反而会把它放在前面强调，因为它最体现工程成熟度。

在 `backend/internal/config/config.go` 里，我们给 Phase 2 增加了四个核心 Feature Flag：

1. `RAG_ENABLE_HYBRID_RETRIEVAL`
2. `RAG_ENABLE_QUERY_REWRITE`
3. `RAG_ENABLE_DYNAMIC_TOPK`
4. `RAG_ENABLE_ADVANCED_RERANK`

同时补了参数区，比如：

1. `hybrid_dense_weight` 和 `hybrid_sparse_weight`
2. `candidate_topk`、`min_topk`、`max_topk`
3. `token_budget`、`min_answer_chunks`
4. `rewrite_timeout_ms`、`rewrite_max_expansions`
5. `rerank_timeout_ms`、`rerank_model`

更关键的是，这些配置不是只读进来就算了，我们还做了三件很值钱的事：

1. 启动前校验。比如混合检索权重必须加起来接近 1，动态 TopK 的 `min_topk` 不能大于 `max_topk`。
2. 启动日志快照。在 `backend/cmd/server/main.go` 和 `config.go` 里会打印 `[RAG:L0]` 的策略摘要，方便线上排障时追溯“当时到底开了哪些开关”。
3. Phase 1 基线快照。配置首次加载时会自动写出 `backend/docs/baseline/phase1/baseline_snapshot.json`，给后续对比留锚点。

这个阶段最大的价值不是业务功能，而是为后面所有实验留出“可回退、可解释、可复盘”的空间。

### 4.2 L1：把单路 dense 升级成 hybrid retrieval

这一层的主入口在 `backend/internal/milvus/init.go` 和 `backend/internal/milvus/retrieval/hybrid_search.go`。

初始化阶段，`InitMilvusManager` 会把 dense 检索器、sparse 检索器、reranker、query rewriter、dynamic topK 配置统一注入到 `HybridRetriever` 里。也就是说，从架构上我们不是在原 dense 检索器上打补丁，而是单独引入了一层“混合检索编排器”。

真正执行请求时，`HybridRetriever.SearchWithRequest` 会做几件事：

1. 先构造 `HybridSearchRequest`，把 query、expr、kb_scope、kb_id、request_id 这些统一起来。
2. 如果开了 rewrite，就先执行受控改写。
3. 并发跑 dense route 和 sparse route。
4. 路由级打点，记录耗时、命中数、错误。
5. 任一路由失败都不拖垮全链路，只在两路都失败时才整体报错。

这里的 sparse 检索不是拍脑袋造一套完全独立的数据结构，而是在 `backend/internal/milvus/retrieval/sparse_search.go` 里走“先拉候选，再在应用侧构建显式倒排并做 BM25 排序”的方案。这么做的好处是：

1. 改动范围小。
2. 容易灰度。
3. 对实体词、缩写词、错拼词这种 dense 天生不占优的场景，有明显补位价值。

### 4.3 L2：融合、去重、统一打分

这一层在我看来是 Phase 2 的技术核心之一，代码主要在：

1. `backend/internal/milvus/retrieval/fusion.go`
2. `backend/internal/milvus/retrieval/dedupe.go`

为什么这一层重要？因为 dense 分和 sparse 分根本不是一套量纲，不能直接相加。

所以我们做法是：

1. 每条 route 先在自己内部做分数归一化。
2. 再按配置做加权融合，默认是 dense 0.7、sparse 0.3。
3. 再按 `document_id + chunk_id` 去重。
4. 对重复命中的 chunk，保留更优主分，同时保留多路贡献信息。

最终我们把统一结果契约固定成 `content / score / citation / source`，并补充：

1. `route`
2. `route_contrib`
3. `route_raw_scores`
4. `retriever_version`

这一步特别适合在分享里强调一句：

**如果没有统一打分和统一结果契约，后面的 rerank、日志、评测、前端展示都会越做越乱。**

### 4.4 L3：受控 query rewrite 和术语扩展

这一层落在 `backend/internal/milvus/retrieval/rewrite.go`。

我们没有一上来就上大模型 query rewrite，而是先做规则版、受控版。受控的意思是：

1. 只扩展，不替换用户原意。
2. 有扩展上限。
3. 有黑名单。
4. 上下文取消或超时直接回退。
5. 是否启用由开关控制。

当前 rewrite 主要做三类事：

1. 缩写扩展，比如 `jvm -> java virtual machine`，`rpc -> remote procedure call`。
2. 别名补充，比如 `golang <-> go`，`kubernetes <-> k8s`。
3. 轻量错拼纠正，比如 `sprinboot -> springboot`。

这里我很建议在会上强调一个认知：

**rewrite 不是为了让 query 看起来更聪明，而是为了减少 query 表达和知识库表达不一致带来的召回损失。**

同时我们保留了 `original_query`、`rewrite_query`、`final_query`、`rewrite_strategy`、`rewrite_applied` 这些元数据，后续在 handler 和审计表里都能追到。

### 4.5 L4：动态 TopK 和 token 预算守卫

这一层主要在 `backend/internal/milvus/retrieval/topk_policy.go`。

这里我会建议大家讲清楚两个概念：

1. `candidate_topk` 是为了给后续融合、去重、重排留候选空间。
2. `final_topk` 是最后真正准备喂给大模型的结果数。

这两个值必须解耦，否则很多优化做不出来。

动态 TopK 的规则目前比较工程化，核心看几类信号：

1. query 是否很长。
2. query term 数是否很多。
3. query 是否明显是宽泛问题。
4. query 是否短且精确。

决策完 `final_topk` 以后，我们再做 token guard。也就是即使条数没超，如果内容太长，仍然可以按预算截断，但同时保留最少回答块数 `min_answer_chunks`，避免为了省 token 把回答证据裁得太狠。

这一层很适合讲一个工程经验：

**只做动态 TopK，不做 token 守卫，本质上还是成本不可控。只做 token 守卫，不做动态 TopK，又容易把宽问题的证据截没。**

### 4.6 L5：重排升级和可追踪结果

这一层在 `backend/internal/milvus/retrieval/reranker.go`。

我们没有把 rerank 写死成一个实现，而是抽象了 `Reranker` 接口，再做了两个层次：

1. `JaccardReranker` 作为轻量 fallback。
2. `ConfigurableReranker` 作为带超时和降级能力的外层包装。

这背后的思路是，重排是高价值层，但它不能成为系统单点风险。

所以重排阶段除了输出文档顺序，我们还把这些信息补到了元数据和 `source` 里：

1. `rerank_score`
2. `rerank_version`
3. `rerank_latency_ms`

并且如果主重排失败，会回退到 fallback，保证系统仍然可用。

这里还有一个非常适合面试讲的点：

**rerank 一定要放在融合和去重之后，因为它应该面对的是统一候选池，而不是各路各自先排完再拼。**

### 4.7 L6：索引参数优化和基准扫描

这一层代码主要在：

1. `backend/cmd/retrieval-benchmark/main.go`
2. `backend/internal/milvus/benchmark/`
3. `backend/docs/kb-l6-index-benchmark-report-template.md`

这里我们不是靠感觉说“这个参数应该更好”，而是把 HNSW 和 IVF 的参数组合做成 profile。

默认 profile 里已经包含：

1. Phase 1 HNSW baseline：`M=16, efConstruction=200, efSearch=64`
2. Phase 2 HNSW balanced：`M=24, efConstruction=320, efSearch=96`
3. Phase 2 HNSW high recall：`M=32, efConstruction=360, efSearch=128`
4. Phase 2 IVF balanced：`nlist=2048, nprobe=32`
5. Phase 2 IVF low latency：`nlist=1024, nprobe=16`

这一步的价值在于，我们终于能把“召回率-延迟-资源消耗”放在一张表里讨论，而不是靠个人体感争论。

### 4.8 L7：离线评测、门禁和贡献度分析

这一层的核心在：

1. `backend/cmd/retrieval-eval/main.go`
2. `backend/internal/milvus/evaluation/`
3. `backend/scripts/evaluation/dataset.json`
4. `backend/scripts/evaluation/retrieval_strategy_profiles.example.json`
5. `backend/scripts/evaluation/retrieval_gate_thresholds.example.json`
6. `backend/docs/kb-l7-retrieval-regression-report-template.md`

我们定义了四组策略画像：

1. `dense_only`
2. `hybrid`
3. `hybrid_rewrite`
4. `hybrid_rewrite_dynamic_topk`

这样每次跑评测，不只是知道“变好了”，还知道到底是哪条策略带来的收益。

门禁阈值也明确写出来了，比如：

1. `Recall` 相对提升至少 `0.08`
2. `P95` 延迟退化比例不能超过 `0.2`

这套机制非常适合在分享里强调，因为它说明我们从“做功能”进化到了“做质量回归系统”。

---

## 5. 这次迭代踩过的坑

下面这部分我建议会上讲得真实一点，因为这往往比“我做了什么”更有价值。

### 5.1 不能把 dense 分和 sparse 分直接加起来

这是混合检索里最容易踩的坑。因为 dense score 和 BM25 score 完全不是同一套分布，如果直接相加，某一路会长期压制另一条路，最后看起来像混合了，实际上还是单路在主导。

所以我们后来改成了“路由内归一化，再加权融合”。这一步就是 `fusion.go` 的核心价值。

### 5.2 candidate_topk 和 final_topk 一开始如果不拆开，后面很难调优

刚开始很多人会觉得 TopK 不就是一个数吗？但实际上不是。

如果候选池太小，融合和重排没有空间。
如果最终返回太多，token 成本会上去。

所以这次最重要的一个工程抽象，就是把“召回多少”和“最终保留多少”拆开。

### 5.3 rewrite 如果做成“强改写”，风险很大

一开始最容易冲动的做法，就是把用户 query 直接改成系统更喜欢的 query。但这样一旦改偏，就不是优化，而是在篡改用户意图。

所以我们最后坚持几条原则：

1. 原 query 永远保留。
2. rewrite 默认是受控扩展，不是自由改写。
3. 黑名单 query 直接跳过。
4. 超时直接回退。

### 5.4 动态 TopK 没有 token guard，等于只做了一半

很多团队做动态 TopK 只关心“结果条数”，但真实成本在 chunk 长度，不在条数本身。

所以这次一个很重要的经验是：

**TopK 解决的是“取几条”，token guard 解决的是“这些条到底有多贵”。**

### 5.5 rerank 放错位置会把整条链路逻辑搞乱

如果每个 route 先各自 rerank，再去融合，后面你其实是在拼两份已经各自偏过的排序，根本不是在统一候选池上比较。

所以我们的顺序必须固定成：

`dense + sparse -> fusion -> dedupe -> rerank -> truncate`

这个顺序一旦定下来，后续日志、评测、调试才有稳定口径。

### 5.6 评测数据如果还是占位值，门禁就是假的

这里我要特别讲一个很现实的坑。

当前 `backend/scripts/evaluation/dataset.json` 里还能看到 `replace-with-real-chunk-id-xxx` 这种占位值。这说明评测框架和报告产出链路已经搭起来了，但如果真实 `chunk_id` 没补进去，那离线评测跑出来的数就没有真实参考意义。

这个点很适合拿出来提醒团队：

**评测系统最难的部分不只是脚本，而是高质量评测集。**

---

## 6. 我们是怎么做调优的

这次我最想强调的是，我们尽量避免“凭感觉调参”。

我们的调优方法基本分三层。

第一层是策略层调优，也就是 Hybrid、Rewrite、Dynamic TopK、Rerank 的开关组合。这个由 L7 的 profile 对比来做。

第二层是索引层调优，也就是 HNSW 和 IVF 参数扫描。这个由 `retrieval-benchmark` 来做。

第三层是线上工程层调优，也就是看日志、看 route 命中、看空结果原因、看 P95 延迟、看 token 成本。

也就是说，真正有效的调优应该回答四个问题：

1. 召回是不是更好了。
2. 排序是不是更稳了。
3. 延迟是不是还在可接受范围内。
4. 成本是不是被控制住了。

如果只回答第一个问题，那不叫完整调优，只能叫单指标优化。

---

## 7. 这轮项目最值得分享的方法论：文档先行

下面这部分，是我觉得最适合做团队分享，也最适合面试加分的部分。

这次我们不是先上手写代码，而是先把文档体系搭出来。大致分三层：

1. 总路线图：`backend/docs/phase2-retrieval-quality-detailed-roadmap.md`
2. 实现教程：比如 L1、L2、L3、L4 的实现教程文档
3. 验证和报告模板：比如 `phase2-l0-l1-validation-tutorial.md`、L6/L7 报告模板

这三层文档分别解决的是三个不同问题：

1. 路线图解决“要做什么、边界是什么、门禁是什么”。
2. 实现教程解决“代码该怎么拆、每层职责是什么、为什么这么写”。
3. 验证模板解决“做完以后怎么证明它真的行”。

我认为文档先行最大的价值，不是让文档看起来专业，而是把讨论从“我感觉这样行”变成“我们按统一契约推进”。

比如这次路线图里一开始就把下面这些口径写死了：

1. 统一检索结果契约是 `content/score/citation/source`
2. 混合检索标准流水线固定为 `dense + sparse -> fusion -> dedupe -> rerank -> filter/truncate`
3. 结构化日志至少要有哪些字段
4. 离线评测一定比较哪些指标

当这些口径先冻结下来以后，后面写代码的人就不会一人一个版本。

---

## 8. 怎么写这种文档，才真的能指导实现

如果让我总结这次文档写法，我会给出五条经验。

第一，文档不要写成愿景，要写成执行手册。

像这次 Phase 2 路线图，不是写“我们要提升检索质量”，而是逐条写清楚：

1. 目标是什么
2. 做哪些功能任务
3. 验收标准是什么
4. 不做什么

第二，文档一定要先写边界，不然需求会无限膨胀。

比如我们明确写了 Phase 2 不做：

1. Parent-Child Retrieval
2. 学习型动态 TopK
3. AB 自动实验平台
4. 索引生命周期自动治理

这一步特别重要，因为它保证团队不会边做边加范围。

第三，文档里要先冻结统一契约。

比如先明确结果字段、日志字段、空结果原因分类。这样后面的 handler、评测脚本、前端展示、调试页面才不会各写各的。

第四，文档里要有 Gate。

也就是不要只写“做好了”，而要写“做到什么程度才算过关”。这次我们就明确了 Recall 提升、P95 回退比例、token 成本、回滚能力这些门槛。

第五，文档一定要能反推代码模块。

也就是说，看完文档后，工程师应该能自然映射出：

1. 哪些是 config 层改动
2. 哪些是 retrieval 层改动
3. 哪些是 handler 和 audit 层改动
4. 哪些是 benchmark 和 evaluation 层改动

如果文档做不到这一点，那它更像汇报材料，不像实施文档。

---

## 9. 怎么通过文档做 vibe coding

这一段我会建议作为分享亮点来讲，因为它很有当下工程实践的代表性。

我理解的“通过文档 vibe coding 实现功能代码”，不是把需求往 AI 一丢，然后期待它自动产出完美系统，而是把 AI 当成一个执行力很强、但是需要上下文约束的工程搭档。

这次最有效的做法，其实是四步。

第一步，先用路线图把上下文喂完整。

路线图里已经告诉 AI：

1. 本阶段做什么，不做什么
2. 标准链路是什么
3. 必须保留哪些字段
4. 日志、回滚、评测门禁怎么做

第二步，再把路线图拆成单层教程或单层任务。

比如只让它做 L3 query rewrite，或者只让它做 L4 dynamic TopK。这样上下文边界清晰，代码更不容易漂。

第三步，要求它按真实代码结构落地，而不是写伪代码。

这次很多实现都能明显看出来是“文档驱动的模块化落地”：

1. 配置进 `config.go`
2. 初始化进 `init.go`
3. 核心逻辑进 `retrieval/*`
4. 审计与 API 进 `handler.go`
5. 验证进 `cmd/*` 和 `evaluation/*`

第四步，写完代码以后，不是马上结束，而是回到文档检查三件事：

1. 功能是否覆盖了路线图中的任务项
2. 字段和日志是否满足契约
3. 是否补了验证入口、模板或测试

所以我更愿意把这种方式叫做：

**文档先冻结设计意图，AI 再按这个意图高速铺代码，人来做验收和纠偏。**

这才是靠谱的 vibe coding，而不是无约束生成。

---

## 10. 如果放到面试里，应该怎么讲

如果这是我在面试里讲，我不会只说“我做了混合检索、query rewrite、dynamic TopK”，因为这样听起来像功能列表。

我会按下面这条逻辑讲：

第一，先讲问题。Phase 1 单路 dense 在实体词、缩写词、短 query、长 query 场景下质量和成本都有明显瓶颈。

第二，讲方案设计。我们把检索链路升级成标准流水线：`dense + sparse -> fusion -> dedupe -> rerank -> truncate`，同时通过 feature flag、日志快照、离线评测、回滚机制把策略工程化。

第三，讲核心难点。真正难的不是多加几个策略，而是：

1. 不同 route 分数不统一
2. rewrite 可能破坏用户意图
3. TopK 和 token 成本存在耦合
4. 没有评测集就无法证明收益

第四，讲你是怎么解决的。比如：

1. 用归一化和加权融合统一分数
2. 用受控 rewrite 和黑名单降低风险
3. 把 `candidate_topk` 和 `final_topk` 解耦
4. 用评测 profile 和门禁脚本固化收益验证

第五，讲结果。结果不一定非得报具体业务数字，但一定要讲“系统能力提升了什么”：

1. 检索质量优化从手工试错变成可回归比较
2. 新策略可以灰度、回滚、审计
3. 质量、延迟、成本第一次能放到同一个闭环里看

如果能按这条线讲，面试官通常会觉得你做的不是一个点功能，而是一整套质量工程建设。

---

## 11. 我认为这轮迭代最有价值的三个结论

第一个结论，RAG 检索优化不是单纯算法题，而是工程题。

真正让系统变强的，不只是混合检索本身，而是开关、日志、评测、回滚、模板、验证这些基础设施。

第二个结论，文档不是交付的附属品，而是实现的前置约束。

这次路线图、实现教程、验证文档、报告模板连起来以后，开发效率和一致性都明显更高，也更适合 AI 参与代码实现。

第三个结论，vibe coding 想靠谱，前提一定是上下文和边界足够清晰。

路线图写不清、契约没冻结、门禁没定义，AI 只会越写越散。反过来，只要文档先把问题拆准，它就很适合帮助我们快速铺实现、补样板、补测试和补工具链。

---

## 12. 收尾

最后我想用一句话收尾这次分享：

**这一轮 Phase 2，我们做的不是几个检索优化技巧的堆叠，而是把检索系统从“能用”推进到了“可控、可评、可回滚、可复盘”。**

如果后面继续进入 Phase 3，我觉得最自然的延续方向会是：

1. Parent-Child Retrieval
2. 更智能的动态 TopK
3. 证据不足时的拒答策略
4. 更完整的灰度实验和 AB 平台

这说明我们这次不是做了一次性功能，而是在搭一个可以持续演进的检索质量平台。

谢谢大家。

---

## 13. 可直接加在分享最后的问答补充

如果有人问“这轮最难的点是什么”，可以回答：

最难的不是把某个策略写出来，而是让策略之间能共存，而且每个策略都能被开关、日志、评测和回滚体系接住。

如果有人问“为什么先写文档”，可以回答：

因为检索优化天然容易发散，如果不先冻结链路、字段和门禁，最后代码一定会碎，评测也无法对齐。

如果有人问“vibe coding 到底有没有价值”，可以回答：

有价值，但前提是文档已经把设计约束、模块边界和验收标准写清楚。这样 AI 负责提速，人负责定方向和验收，效率是很高的。
