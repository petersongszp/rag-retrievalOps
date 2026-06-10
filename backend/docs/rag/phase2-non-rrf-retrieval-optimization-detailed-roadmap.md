# Phase 2.2 详细功能实现路线（非 RRF 检索优化）

## 1. 文档定位

本文档用于整理本次混合检索排查中暴露出来的、除 `RRF` 融合以外需要优化的功能实现路线。

它不是对 `RRF` 改造文档的替代，而是与 `backend/docs/rag/phase2-rrf-fusion-detailed-roadmap.md` 并列的一份专项执行手册。本文档关注的是：

1. `sparse` 关键词召回底座是否可靠。
2. 当前 `LIKE + 应用内 BM25` 的质量、性能和可解释性问题。
3. 检索日志与指标是否能真实回答“谁参与、谁贡献、谁被丢弃”。
4. `rerank`、查询改写、动态 TopK、离线评测这些链路是否能支撑后续质量优化。
5. 如何把问题拆成可排期、可验收、可回滚的工程任务。

本文档风格与现有路线文档保持一致：

目标 -> 路线 -> 模块任务 -> 验收门禁 -> 下一步入口。

统一口径说明：

1. `dense` 固定指向量召回路由，对应 `backend/internal/milvus/retrieval/search.go`。
2. `sparse` 固定指关键词召回路由，对应 `backend/internal/milvus/retrieval/sparse_search.go`。
3. `当前 sparse 实现` 固定指：从 query 抽取 term -> 对 Milvus collection 执行 `content like "%term%"` 候选查询 -> 在应用内构建临时倒排索引 -> 使用 BM25 排序。
4. `当前 BM25` 固定指 `backend/internal/milvus/retrieval/sparse_inverted_index.go` 内的应用侧 BM25，不是 MySQL 原生能力，也不是独立全文检索引擎能力。
5. `route_hits` 固定指某路召回实际返回候选数。
6. `route_contribution` 在当前代码里更接近“最终结果主路由占比”，不等价于“该路是否参与过最终候选”。
7. 本文档明确不设计 `RRF` 算法本身，`fusion` 策略升级单独走 RRF 专项。

---

## 2. 当前问题基线

本次排查已经确认：

1. 当前系统不是“只走 LIKE，不走向量检索”。
2. 默认混合检索链路会同时执行 `dense + sparse`。
3. `dense` 是真实向量检索，调用 embedding 后在 Milvus 里做向量搜索。
4. `sparse` 是关键词路由，当前以 `content like "%term%"` 从 Milvus 拉候选，再在应用内做 BM25。
5. 线上日志样本显示成功请求中 `dense_hits` 与 `sparse_hits` 都有命中，但 `sparse_contribution` 为 0。
6. `sparse_contribution = 0` 不能直接解释成 `sparse` 没执行，它更接近最终保留结果的主路由统计。

因此，非 RRF 部分的核心问题不是“向量有没有用”，而是：

1. 关键词召回底座弱。
2. BM25 只在候选集内临时计算，统计语义不完整。
3. 分词、术语、缩写、中文 query 支持不足。
4. 日志指标口径容易被误读。
5. reranker 目前偏轻量，不能稳定纠正前序排序问题。
6. 缺少可回放的离线评测门禁，质量优化容易靠主观感受推进。

---

## 3. 范围边界

## 3.1 本专项必须完成

1. 补齐检索日志与 debug trace，让团队能区分 `hits`、`participation`、`primary_route`、`drop_reason`。
2. 改造 `sparse` 关键词召回接口，为后续替换 `LIKE` 底座预留适配层。
3. 优化 term 抽取、分词、停用词、短词、缩写词、中文 query 的处理。
4. 将 BM25 从“临时候选集排序”升级为“可解释、可配置、可评估”的关键词排序能力。
5. 建立 sparse 路由的性能保护：超时、候选上限、降级、熔断、慢查询观测。
6. 梳理 reranker 当前能力边界，形成可替换的重排接口与评测门禁。
7. 建立离线评测数据集与 replay 脚本，用真实 case 验证每次优化是否有效。

## 3.2 本专项明确不做

1. 不实现 `RRF` 算法，RRF 已有独立专项文档。
2. 不一次性重写整条 RAG 链路。
3. 不把所有检索都迁移出 Milvus。
4. 不直接引入不可控的在线学习排序模型。
5. 不把父子块检索、证据拒答、citation consistency 的主逻辑并入本专项。
6. 不在没有评测集和回滚路径的情况下直接替换线上关键词召回底座。

---

## 4. 目标与通过标准（Gate）

本专项通过标准（全满足）：

1. 任意一条检索请求都能从日志中看清：`dense` 是否命中、`sparse` 是否命中、两路是否进入融合前候选、最终主路由是谁、在哪个阶段被截断。
2. `sparse` 路由有独立的耗时、候选数、term 数、BM25 命中数、空结果原因、错误原因指标。
3. `sparse` 候选召回不再被业务代码直接绑定到 `content like "%term%"`，而是通过可替换的 `SparseCandidateProvider` 或等价接口承载。
4. 中文、英文、缩写词、实体词、短 query 场景在离线评测集中都有样例覆盖。
5. BM25 的分词、参数、字段权重、候选统计口径可配置，并能在 debug trace 中解释。
6. 关键词路由优化后，实体词/缩写词场景 Recall@10 或 MRR 相比当前 baseline 有稳定提升。
7. P95 检索延迟不出现不可接受退化；如果退化，能通过配置在 10 分钟内回滚到当前实现。
8. reranker 的启用、失败、超时、fallback 都可观测，并能离线对比 rerank 前后排序变化。

---

## 5. 实现路线总览（L0 -> L8）

本专项按 9 条路线推进：

1. L0：问题口径冻结、baseline 快照与评测集准备
2. L1：检索日志、debug trace 与贡献指标口径修正
3. L2：Sparse 召回接口抽象与 `LIKE` 底座隔离
4. L3：关键词 term 抽取、分词与术语扩展优化
5. L4：BM25 排序能力升级与统计口径修正
6. L5：Sparse 性能保护、超时降级与慢查询治理
7. L6：Reranker 能力边界梳理与可替换重排接口
8. L7：离线评测、回放脚本与质量报告
9. L8：灰度发布、回滚预案与验收收口

建议顺序：

`L0 -> L1 -> L2 -> L3 + L4 -> L5 + L6 -> L7 -> L8`

其中 `L1` 必须优先做，因为没有准确观测，后续所有优化都很难证明收益。

---

## 6. 详细路线拆解

## 6.1 L0 问题口径冻结、baseline 快照与评测集准备

### 目标

先把当前行为固定下来，避免后续优化过程中团队无法判断“到底是 sparse 变好了，还是只是日志口径变了”。

### 功能任务

1. 固定当前检索链路 baseline：
   - `enable_hybrid_retrieval=true`
   - `dense + sparse`
   - `sparse=Milvus content LIKE + app BM25`
   - `rerank=jaccard-v1`
   - `fusion=minmax weighted sum`
2. 固定当前代码入口：
   - `backend/api/handler/kb/handler.go`
   - `backend/api/handler/kb/knowledge_base_binding.go`
   - `backend/internal/milvus/retrieval/hybrid_search.go`
   - `backend/internal/milvus/retrieval/sparse_search.go`
   - `backend/internal/milvus/retrieval/sparse_inverted_index.go`
   - `backend/internal/milvus/retrieval/reranker.go`
3. 建立 baseline SQL：
   - 汇总 `dense_hits/sparse_hits`
   - 汇总 `dense_contribution/sparse_contribution`
   - 按 `query_type`、`kb_id`、`routes`、`result_status` 分组
   - 抽样查看 `debug_trace`
4. 建立第一批评测 case：
   - 英文实体词：如 `JVM`、`Kubernetes`、`Redis`
   - 中文实体词：如业务专有名词、岗位名、公司名
   - 缩写词：如 `GC`、`OOM`、`SLA`
   - 短 query：如 `退款`、`鉴权`、`部署`
   - 长自然语言 query：如“线上 JVM OOM 应该怎么排查”
   - 关键词必须精确命中的 query：如错误码、接口名、配置项
5. 为每个 case 标注期望命中文档或 chunk：
   - `expected_document_id`
   - `expected_chunk_id`
   - `expected_route`
   - `must_contain_terms`
   - `acceptable_answer_evidence`

### 验收

1. 当前 baseline 可通过脚本一键重新跑出。
2. 至少有 30 条覆盖实体词、缩写词、中英文、短 query 的评测样例。
3. 每条样例都能说明“为什么 sparse 应该有价值”或“为什么 dense 应该有价值”。
4. baseline 报告里明确列出当前 `sparse_hits`、`sparse_contribution`、P95 延迟、空结果率。

---

## 6.2 L1 检索日志、debug trace 与贡献指标口径修正

### 目标

解决当前日志最容易误导团队的问题：`sparse_contribution=0` 被误读为 `sparse` 没执行。

### 功能任务

1. 在 `SearchMetrics`、`KBRetrieveLog` 或 `DebugTrace` 中补充更清晰的指标：
   - `dense_hits`
   - `sparse_hits`
   - `dense_participation`
   - `sparse_participation`
   - `primary_dense_count`
   - `primary_sparse_count`
   - `dual_route_final_count`
   - `sparse_candidate_count_before_bm25`
   - `sparse_candidate_count_after_bm25`
2. 调整贡献口径文档：
   - `hits` 表示路由召回数量
   - `participation` 表示该路候选是否进入融合后或最终候选
   - `primary_route` 表示最终结果主路由
   - `contribution` 如果继续保留，必须明确它当前更接近主路由统计
3. 扩展 `debug_trace`：
   - sparse term 列表
   - 每个 term 的候选数
   - BM25 前候选数
   - BM25 后候选数
   - rerank 前排名
   - rerank 后排名
   - filter/truncate 删除原因
4. 扩展日志打印：
   - `sparse_terms`
   - `sparse_candidate_before_bm25`
   - `sparse_candidate_after_bm25`
   - `route_participation`
   - `primary_route_distribution`
5. 在管理后台或 trace API 中展示新增字段，避免只在数据库里可见。

### 验收

1. 可以回答“sparse 有没有执行”。
2. 可以回答“sparse 命中的候选有没有进入最终候选池”。
3. 可以回答“sparse 最后是不是主路由”。
4. 可以回答“一条 sparse 候选在哪个阶段被丢掉”。
5. 旧字段仍兼容，不破坏现有 dashboard 和 API 响应。

---

## 6.3 L2 Sparse 召回接口抽象与 `LIKE` 底座隔离

### 目标

把当前 `sparse_search.go` 中直接拼 `content like "%term%"` 的逻辑隔离出来，为后续替换成真正的全文检索、持久化倒排索引或 Milvus sparse vector 能力留出工程入口。

### 当前问题

1. 业务代码直接依赖 Milvus `Query + LIKE`。
2. `LIKE "%term%"` 不等价于真正的关键词检索引擎。
3. 候选召回能力依赖 Milvus 对字符串谓词的执行方式，不能获得成熟全文检索的倒排索引、分词、短语匹配、字段权重能力。
4. 每个 term 单独查询，term 多时容易增加延迟。
5. 当前候选召回只看 `content`，没有充分利用 `title`、`metadata`、`document_name`、`tags` 等字段。

### 功能任务

1. 新增 sparse 候选召回接口：
   - `SparseCandidateProvider`
   - `SearchCandidates(ctx, req, terms) ([]*schema.Document, SparseCandidateStats, error)`
2. 保留当前实现作为默认 provider：
   - `MilvusLikeCandidateProvider`
   - 行为保持与当前 `content like "%term%"` 一致
3. 在 `SparseRetriever` 中只依赖接口，不直接拼接 Milvus `LIKE`：
   - term 抽取归 term 模块
   - 候选召回归 provider
   - BM25 排序归 ranker
4. 设计后续可插拔 provider：
   - `MilvusLikeCandidateProvider`
   - `FullTextCandidateProvider`
   - `SparseVectorCandidateProvider`
   - `ExternalSearchCandidateProvider`
5. 给 provider 增加统一统计：
   - `provider_name`
   - `provider_version`
   - `term_count`
   - `per_term_limit`
   - `raw_candidate_count`
   - `dedup_candidate_count`
   - `latency_ms`
   - `fallback_reason`
6. 配置化 provider：
   - `rag.phase2.sparse_provider=milvus_like`
   - `rag.phase2.sparse_provider_timeout_ms`
   - `rag.phase2.sparse_provider_max_terms`
   - `rag.phase2.sparse_provider_per_term_limit`

### 验收

1. 默认 provider 下，线上行为与当前 `LIKE` 实现一致。
2. `SparseRetriever` 不再直接散落 Milvus `LIKE` 拼接逻辑。
3. 新增 provider 不需要改动 hybrid 主流程。
4. provider 级别耗时、候选数、错误原因可观测。
5. 可以通过配置在不同 provider 间灰度切换。

---

## 6.4 L3 关键词 term 抽取、分词与术语扩展优化

### 目标

提升 sparse 路由最前面的 query 理解能力，尤其是中文、缩写词、实体词、配置项、错误码场景。

### 当前问题

1. `extractSparseTerms(...)` 主要基于 `unicode.IsLetter/IsNumber` 做简单切分。
2. 英文 stopwords 写死在代码里，中文停用词、业务停用词缺失。
3. 中文连续文本不容易被切成可用词项。
4. 缩写词、大小写、连字符、下划线、错误码、接口路径没有专门处理。
5. term 权重缺失，所有 term 对 sparse 召回影响接近。
6. query 改写与 sparse term 抽取之间缺少明确契约。

### 功能任务

1. 新增 `SparseTermExtractor`：
   - 输入 `original_query/final_query/sparse_query/query_type`
   - 输出 `terms/phrases/entities/boosts`
2. 支持 term 类型：
   - `word`
   - `phrase`
   - `entity`
   - `acronym`
   - `error_code`
   - `config_key`
   - `api_path`
3. 支持基础中文分词策略：
   - 最小版本可以先接入轻量分词库或业务词典最大匹配
   - 后续再评估更完整的中文分词组件
4. 增加停用词与保留词配置：
   - `sparse_stopwords`
   - `sparse_keep_terms`
   - `domain_terms`
   - `acronym_terms`
5. 增加术语扩展：
   - `JVM -> Java Virtual Machine`
   - `OOM -> Out Of Memory`
   - `鉴权 -> authentication/authz`
   - 业务别名 -> 标准术语
6. 为每个 term 记录来源：
   - `original`
   - `rewrite`
   - `domain_dict`
   - `model_rewrite`
7. 将 term 抽取结果写入 debug trace：
   - `sparse_terms`
   - `sparse_phrases`
   - `term_sources`
   - `term_boosts`
   - `dropped_terms`
   - `drop_reason`

### 验收

1. 中文 query 能产生可解释的关键词项。
2. 缩写词 query 能命中扩展后的同义术语。
3. 错误码、配置项、接口路径不会被错误拆碎。
4. 停用词不会进入 Milvus LIKE 或后续全文检索 provider。
5. 每条请求的 term 抽取结果可在 debug trace 中回放。

---

## 6.5 L4 BM25 排序能力升级与统计口径修正

### 目标

把当前“对临时候选集做 BM25 排序”升级为更接近真实关键词检索的排序能力，并让 BM25 分数可解释、可调参、可评估。

### 当前问题

1. 当前 BM25 的 `IDF` 是基于本次候选集计算，不是基于完整知识库语料。
2. 候选集来自 `LIKE`，如果候选阶段漏掉了文档，BM25 没机会补救。
3. 分词逻辑与 term 抽取一样偏简单，中文和业务实体支持不足。
4. `content` 单字段排序，没有 title、metadata、document_name、tags 的字段加权。
5. BM25 参数 `K1/B` 在代码里默认，缺少配置、日志和评测。
6. BM25 分数没有稳定映射到“解释信息”，排查时只能看到一个分数。

### 功能任务

1. 抽象 `SparseRanker`：
   - `Rank(ctx, query, terms, candidates) ([]SparseSearchHit, SparseRankStats, error)`
2. 保留当前实现为：
   - `CandidateBM25Ranker`
3. 增加 BM25 配置：
   - `rag.phase2.bm25_k1`
   - `rag.phase2.bm25_b`
   - `rag.phase2.bm25_topk`
   - `rag.phase2.bm25_min_score`
4. 增加字段权重能力：
   - `content_weight`
   - `title_weight`
   - `metadata_weight`
   - `tag_weight`
5. 增加 BM25 explain 信息：
   - `matched_terms`
   - `term_tf`
   - `term_df`
   - `term_idf`
   - `field_matches`
   - `field_boosts`
   - `bm25_score`
6. 建立持久化语料统计的设计入口：
   - 每个 KB 维护 `doc_count`
   - 每个 term 维护 `df`
   - 每个 chunk 维护 `length`
   - 每个字段维护 `avg_length`
7. 在不马上替换底座的情况下，先支持两种 ranker：
   - `candidate_bm25`：当前候选集内 BM25
   - `corpus_stats_bm25`：基于知识库级统计的 BM25

### 验收

1. BM25 参数可配置，可在启动快照中看到。
2. 每个 sparse 结果能解释为什么得分高。
3. 基于知识库级统计的 BM25 在离线评测中可与当前候选集 BM25 对比。
4. 字段加权对标题、标签、文档名命中场景有可观测收益。
5. BM25 排序失败时可降级为当前候选顺序或原有 ranker。

---

## 6.6 L5 Sparse 性能保护、超时降级与慢查询治理

### 目标

避免关键词路由因为 `LIKE`、term 过多、候选过大导致检索 P95 延迟不可控。

### 当前问题

1. 每个 term 会发一次 Milvus `Query`。
2. `content like "%term%"` 对长文本字段的执行成本可能较高。
3. term 多、知识库大、content 长时，sparse 路由可能成为慢查询来源。
4. 当前对 per-term 候选上限有控制，但缺少更完整的超时、熔断、慢查询审计。
5. sparse 失败后虽然主链路可以继续，但日志层面的降级原因还不够细。

### 功能任务

1. 为 sparse provider 设置独立 timeout：
   - `sparse_timeout_ms`
   - `sparse_per_term_timeout_ms`
2. 增加候选保护：
   - `max_terms`
   - `max_raw_candidates`
   - `max_dedup_candidates`
   - `min_term_length`
   - `max_term_length`
3. 增加慢查询日志：
   - `provider_name`
   - `term`
   - `expr`
   - `limit`
   - `latency_ms`
   - `candidate_count`
4. 增加降级策略：
   - sparse 超时 -> 只返回 dense
   - sparse provider 错误 -> 只返回 dense
   - BM25 ranker 错误 -> 返回 provider 候选或空 sparse
   - term 抽取为空 -> 跳过 sparse
5. 增加熔断策略：
   - 连续超时达到阈值后短时间跳过 sparse
   - 按 KB 维度记录慢查询风险
6. 增加 metrics：
   - `rag_sparse_provider_duration_seconds`
   - `rag_sparse_provider_candidates_total`
   - `rag_sparse_provider_timeout_total`
   - `rag_sparse_provider_degraded_total`
   - `rag_sparse_term_count`

### 验收

1. sparse 慢查询不会拖垮整条 hybrid 请求。
2. sparse 降级原因在日志和 debug trace 中可见。
3. P95 延迟退化超过阈值时可以快速关闭 sparse provider 或切回旧实现。
4. 可以按 KB、query_type、term_count 识别慢查询高风险场景。

---

## 6.7 L6 Reranker 能力边界梳理与可替换重排接口

### 目标

让 reranker 从当前轻量 `Jaccard` 重排，演进为可配置、可替换、可评估的排序层，同时避免它掩盖 sparse 召回问题。

### 当前问题

1. 当前默认 `jaccard-v1` 更偏词面重叠，语义判断能力有限。
2. `OriginalScoreWeight=0.7` 会强依赖前序分数，前序 sparse 被压低时 rerank 很难彻底纠正。
3. reranker 对 dense/sparse 来源、公平性、route balance 没有显式策略。
4. rerank 前后排名变化虽然有 debug trace，但缺少系统化报告。
5. 失败 fallback 存在，但质量侧缺少“fallback 后结果是否明显变差”的观测。

### 功能任务

1. 固化 reranker 接口契约：
   - `Rerank(ctx, query, docs) -> documents + metadata`
2. 增加 rerank 策略配置：
   - `rerank_model`
   - `rerank_timeout_ms`
   - `rerank_topk`
   - `original_score_weight`
   - `route_balance_enabled`
3. 增加 rerank explain：
   - `pre_rerank_rank`
   - `post_rerank_rank`
   - `rerank_score`
   - `original_score`
   - `score_delta`
   - `route`
4. 增加 candidate pool 保护：
   - rerank 输入数量上限
   - dense/sparse 至少保留数量
   - 低分但精确关键词命中的 sparse 候选保护
5. 设计 cross-encoder 或外部 reranker 接入入口：
   - 当前阶段先保留接口和灰度配置
   - 模型接入必须经过离线评测与超时 fallback
6. 建立 rerank 对比报告：
   - rerank 前 Recall@K
   - rerank 后 Recall@K
   - rerank 前 MRR
   - rerank 后 MRR
   - sparse 候选被提升/被压低数量

### 验收

1. rerank 前后排名变化可解释。
2. reranker 超时或失败不会影响主检索可用性。
3. 可以通过配置调整 `original_score_weight` 并离线比较收益。
4. reranker 不再是黑盒排序层，每次排序变化都能在 trace 中定位。

---

## 6.8 L7 离线评测、回放脚本与质量报告

### 目标

把检索优化从“感觉变好了”变成“可复现、可对比、可回滚”的工程流程。

### 功能任务

1. 建立评测数据集表或文件：
   - `case_id`
   - `query`
   - `query_type`
   - `kb_id`
   - `expected_document_id`
   - `expected_chunk_id`
   - `must_contain_terms`
   - `difficulty`
2. 建立 replay 脚本：
   - baseline 策略
   - candidate 策略
   - 输出统一 JSON 报告
3. 评测指标：
   - `Recall@5`
   - `Recall@10`
   - `MRR`
   - `nDCG`
   - `SparseHitRate`
   - `SparseParticipationRate`
   - `PrimarySparseRate`
   - `EmptyRate`
   - `P95 latency`
4. 增加分场景报告：
   - 中文 query
   - 英文 query
   - 缩写词
   - 实体词
   - 短 query
   - 长 query
   - 错误码/配置项
5. 增加失败样例导出：
   - expected 未命中
   - sparse 有命中但最终被丢弃
   - rerank 后正确结果下降
   - token budget 截断掉正确结果
6. 和现有 `kb-l7-retrieval-regression-report-template.md` 对齐，形成固定报告模板。

### 验收

1. 任意检索策略改动都能跑 baseline/candidate 对比。
2. 报告能明确指出收益来自 sparse、rerank、term 扩展还是其他阶段。
3. 如果指标退化，能定位退化 case 和退化阶段。
4. 评测报告可以作为上线门禁附件。

---

## 6.9 L8 灰度发布、回滚预案与验收收口

### 目标

把非 RRF 检索优化按可控方式上线，确保质量收益、性能风险和回滚路径都清楚。

### 功能任务

1. 增加灰度配置：
   - `sparse_provider`
   - `sparse_provider_enabled`
   - `bm25_ranker`
   - `term_extractor_version`
   - `rerank_profile`
2. 发布分阶段：
   - `shadow`：只记录 candidate 结果，不影响线上返回
   - `internal`：内部用户启用
   - `canary`：小流量启用
   - `batch`：扩大流量
   - `stable`：默认策略
3. 回滚策略：
   - provider 回滚到 `milvus_like`
   - ranker 回滚到 `candidate_bm25`
   - term extractor 回滚到旧版本
   - reranker 回滚到 `jaccard-v1` 或关闭高级 rerank
4. 发布观察指标：
   - success rate
   - no result rate
   - dense/sparse hits
   - sparse participation
   - primary sparse rate
   - P95/P99 latency
   - timeout rate
   - fallback rate
5. 验收材料：
   - baseline/candidate 离线报告
   - 线上灰度报告
   - 慢查询报告
   - 失败 case 列表
   - 回滚演练记录

### 验收

1. 每个优化项都有独立开关。
2. 每个优化项都有回滚路径。
3. 灰度期间能清晰观察 sparse 价值是否提升。
4. 出现延迟或质量退化时，能在 10 分钟内恢复旧策略。

---

## 7. 推荐实施顺序

第一批优先做可观测与评测：

1. L0 baseline 快照。
2. L1 日志口径修正。
3. L7 最小评测集与 replay 脚本。

原因：当前最大风险是团队无法准确判断 sparse 是否参与、是否贡献、在哪个阶段被压掉。没有这层能力，后续改 provider、改 BM25、改 rerank 都容易变成“看起来变好了”。

第二批做 sparse 底座解耦：

1. L2 `SparseCandidateProvider` 抽象。
2. L5 sparse timeout、慢查询和降级。

原因：当前 `LIKE` 不是不能保留，而是不能继续让主业务逻辑强依赖它。先隔离底座，后面才能平滑替换。

第三批做关键词质量：

1. L3 term extractor。
2. L4 BM25 ranker。

原因：关键词检索质量很大程度取决于“抽出了什么词”和“这些词如何排序”。这两层必须一起评估。

第四批做 reranker 与灰度：

1. L6 reranker explain 与策略配置。
2. L8 shadow/canary/stable 发布。

原因：reranker 是最后排序层，适合在前面召回质量更可控之后再加强。

---

## 8. 角色分工建议

1. 后端检索负责人：
   - `SparseCandidateProvider`
   - `SparseRanker`
   - provider/ranker 配置接入
2. 平台可观测负责人：
   - `KBRetrieveLog`
   - debug trace
   - metrics
   - dashboard/API 展示
3. 算法/检索质量负责人：
   - term extractor
   - BM25 参数
   - rerank 策略
   - 评测集标注
4. 测试负责人：
   - baseline/candidate replay
   - 回归报告
   - 性能压测
   - 回滚演练
5. 业务知识负责人：
   - 领域词典
   - 缩写词表
   - 同义词表
   - 失败 case 复盘

---

## 9. 评审时建议的讲法

可以用下面这段话开场：

> 这次排查不是证明“向量检索没用”，相反，真实日志说明 dense 一直在跑，并且目前最终结果主要由 dense 主导。我们真正要优化的是 sparse 这条关键词路由：它现在确实能命中，但候选召回依赖 Milvus LIKE，BM25 只在临时候选集里计算，分词和术语能力偏弱，日志也没把参与和主贡献讲清楚。所以 RRF 之外，我们要优先补观测和评测，再把 sparse 召回、BM25、term extractor、reranker 拆开改。

代码 review 顺序建议：

1. `backend/internal/milvus/retrieval/hybrid_search.go`
   - 看 dense/sparse 并发召回与 metrics 汇总。
2. `backend/internal/milvus/retrieval/sparse_search.go`
   - 看 `extractSparseTerms`、`content like "%term%"`、Milvus `Query`、候选合并。
3. `backend/internal/milvus/retrieval/sparse_inverted_index.go`
   - 看当前 BM25 是如何在候选集内临时构建的。
4. `backend/internal/milvus/retrieval/reranker.go`
   - 看当前 `jaccard-v1` 的能力边界。
5. `backend/internal/model/kb_retrieve_log.go`
   - 看当前日志字段为什么容易把 `contribution` 误读成 `participation`。
6. `backend/api/handler/kb/handler.go`
   - 看检索结果如何落日志，以及 dashboard/API 怎么消费这些字段。

---

## 10. 下一步交付物

本专项建议形成以下交付物：

1. `L0`：当前检索 baseline 报告。
2. `L1`：检索日志字段改造 PR。
3. `L2`：`SparseCandidateProvider` 接口与默认 `MilvusLikeCandidateProvider` PR。
4. `L3`：`SparseTermExtractor` 与领域词典 PR。
5. `L4`：`SparseRanker` 与 BM25 explain PR。
6. `L5`：sparse timeout、fallback、slow query metrics PR。
7. `L6`：rerank explain 与策略配置 PR。
8. `L7`：retrieval replay 脚本与离线报告模板。
9. `L8`：灰度发布记录与回滚演练记录。

---

## 11. 维护规则

1. 每次改 sparse 召回，必须同时更新评测报告。
2. 每次改 BM25 参数，必须记录参数快照和指标变化。
3. 每次改 term extractor，必须导出新增/删除 term 的 diff。
4. 每次改 reranker，必须对比 rerank 前后排名变化。
5. 每个新指标都必须明确解释口径，避免再次出现 `sparse_contribution=0` 被误读的问题。
6. 每个优化项必须有关闭开关，不能把实验策略硬编码成默认线上路径。
