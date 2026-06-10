# Phase2 检索功能代码改动复盘与 Review 指南

## 1. 背景

这次 Phase2 的核心目标不是简单增加一个检索算法，而是把混合检索链路从“能跑”推进到“能解释、能切换、能评测、能回滚”。

改动前主要有几个问题：

1. `dense` 和 `sparse` 都在跑，但日志里的 `sparse_contribution=0` 容易被误读成 sparse 没有执行。
2. sparse 关键词召回逻辑直接和 Milvus `content like "%term%"` 绑在一起，后续替换全文检索或 sparse vector 不方便。
3. BM25 只有一个排序分数，缺少 matched term、TF、DF、IDF 等解释信息，不方便排查为什么某个 chunk 排名前。
4. minmax 融合是唯一主路径，无法用配置切换到 RRF 做对照实验。
5. 去重后缺少 route rank、RRF 贡献、primary route 等解释字段，debug trace 和审计日志不够完整。
6. 离线评测没有把 dense/sparse 命中、参与、主路由贡献、融合策略差异统一纳入报告。

这次改动围绕两条线展开：

1. 非 RRF 检索优化：修正 sparse 观测口径，抽象 sparse provider/ranker，补齐 BM25 explain、降级和 rerank explain。
2. RRF 融合能力：增加 `minmax_v1` / `rrf_v1` 配置开关，实现 RRF 融合、去重解释、评测对比和回滚路径。

## 2. 主要改动总览

### 2.1 配置层增加融合策略开关

相关文件：

1. `backend/internal/config/config.go`
2. `backend/config.yaml`
3. `backend/config.example.yaml`
4. `backend/config.rag.example.yaml`

新增和补齐的 Phase2 配置包括：

```yaml
rag:
  phase2:
    fusion_strategy: minmax_v1
    rrf_k: 60
    rrf_dense_weight: 0.7
    rrf_sparse_weight: 0.3
    hybrid_dense_weight: 0.7
    hybrid_sparse_weight: 0.3
```

这里的 `fusion_strategy` 是本次最重要的回滚开关：

1. `minmax_v1`：保留原来的 min-max 归一化加权融合。
2. `rrf_v1`：启用 RRF 排名融合。

代码里对配置做了校验：

1. hybrid 开启时，`fusion_strategy` 必须是 `minmax_v1` 或 `rrf_v1`。
2. dense/sparse 权重和必须接近 1。
3. RRF 启用时，`rrf_k` 必须大于 0。
4. RRF dense/sparse 权重要么都不配，走默认；要么一起配，并且权重和必须是 1。

这解决的问题是：RRF 不是硬切上线，而是通过配置切换。线上如果发现质量或性能风险，可以把 `fusion_strategy` 改回 `minmax_v1` 并重启服务。

### 2.2 混合检索主流程接入融合策略

相关文件：

1. `backend/internal/milvus/retrieval/hybrid_search.go`
2. `backend/internal/milvus/retrieval/fusion.go`
3. `backend/internal/milvus/retrieval/dedupe.go`

主流程现在大致是：

1. dense 路由做向量召回。
2. sparse 路由做关键词候选召回和 BM25 排序。
3. `FuseRouteCandidates` 根据配置选择 `minmax_v1` 或 `rrf_v1`。
4. `DeduplicateFusedDocuments` 按 `document_id + chunk_id` 去重。
5. reranker 对融合后的候选再排序。
6. 生成最终 metrics、debug trace 和审计日志。

重点代码入口：

```go
fused := FuseRouteCandidates(denseDocs, sparseDocs, FusionConfig{
    FusionStrategy: h.config.FusionStrategy,
    RRFK:           h.config.RRFK,
    DenseWeight:    h.config.DenseWeight,
    SparseWeight:   h.config.SparseWeight,
    RRFDenseWeight: h.config.RRFDenseWeight,
    RRFSparseWeight: h.config.RRFSparseWeight,
})
```

这段代码的意义是把融合策略从固定逻辑变成运行时配置。以后做 A/B、灰度、离线对比，都不需要再改主流程代码。

### 2.3 实现 `minmax_v1` 和 `rrf_v1` 两种融合

相关文件：

1. `backend/internal/milvus/retrieval/fusion.go`
2. `backend/internal/milvus/retrieval/fusion_test.go`

`minmax_v1` 的逻辑：

1. 分别收集 dense 和 sparse 的原始分数。
2. 对每条路由内部做 min-max 归一化。
3. 再乘以 dense/sparse 权重。
4. 写入 `fusion_score`、`route_score`、`route_contrib` 等字段。

`rrf_v1` 的逻辑：

1. 每条路由内部先按原始分数排序。
2. 记录当前文档在每条路由里的名次 `route_rank`。
3. 使用公式 `weight / (rrf_k + rank)` 计算 RRF 贡献。
4. 写入 `rrf_score`、`route_rrf_contrib`、`route_rank`。

RRF 的好处是它更关注“排名”而不是“原始分数大小”。这对 dense 和 sparse 尤其重要，因为两条路由的分数体系天然不一样：dense 是向量相似度，sparse 是 BM25 关键词相关性，直接比较原始分数不公平。

### 2.4 去重后保留解释信息

相关文件：

1. `backend/internal/milvus/retrieval/dedupe.go`
2. `backend/internal/milvus/retrieval/fusion_test.go`

去重逻辑现在不只是保留最高分文档，还会合并每条 route 的解释信息：

1. `primary_route`：最终主路由是谁。
2. `fusion_strategy`：本次使用的是 `minmax_v1` 还是 `rrf_v1`。
3. `route_contrib`：每条路由对最终文档的贡献。
4. `route_raw_scores`：每条路由原始分数。
5. `route_rank`：RRF 下每条路由里的名次。
6. `route_rrf_contrib`：RRF 下每条路由贡献分。

这个改动修复了一个 review 时很关键的问题：同一个 chunk 可能同时被 dense 和 sparse 找到。以前去重后容易只看见“最终保留的是谁”，看不清“它其实被哪些路由共同命中过”。现在 debug trace 和 metadata 里能解释这个过程。

### 2.5 修正 sparse 参与度和贡献口径

相关文件：

1. `backend/internal/milvus/retrieval/hybrid_search.go`
2. `backend/internal/milvus/retrieval/search.go`
3. `backend/internal/model/kb_retrieve_log.go`
4. `backend/api/handler/kb/handler.go`
5. `backend/api/handler/kb/retrieval_debug_trace_v2.go`
6. `backend/api/handler/kb/knowledge_base_binding.go`

这次新增和统一了几类指标：

1. `dense_hits` / `sparse_hits`：路由原始召回数量。
2. `dense_participation` / `sparse_participation`：最终结果里，有多少文档曾被该路由参与命中过。
3. `primary_dense_count` / `primary_sparse_count`：最终主路由分别属于 dense/sparse 的数量。
4. `dual_route_final_count`：最终结果里，同时被 dense 和 sparse 命中的数量。
5. `dense_contribution` / `sparse_contribution`：当前等同于最终主路由贡献数量。

最重要的修复是：`sparse_contribution=0` 不再被当成 sparse 没执行。

正确理解应该是：

1. `sparse_hits > 0`：说明 sparse 路由确实召回到了候选。
2. `sparse_participation > 0`：说明 sparse 候选进入了最终结果解释链路。
3. `primary_sparse_count = 0`：说明最终主路由没有由 sparse 主导。
4. `sparse_contribution = 0`：说明按当前主路由统计，sparse 没有成为最终主贡献方。

这个口径修复对排查很重要。它让团队可以回答三个问题：sparse 有没有跑、有没有命中、最后有没有主导结果。

### 2.6 多知识库合并后重新计算最终 route 指标

相关文件：

1. `backend/api/handler/kb/knowledge_base_binding.go`
2. `backend/api/handler/kb/handler_l7_test.go`

之前每个 KB 的检索结果有自己的 route 指标，多 KB 合并后如果直接累加，可能和最终返回给用户的文档不一致。

现在合并后会基于最终文档重新计算：

1. dense/sparse participation。
2. primary dense/sparse count。
3. dual route final count。
4. dense/sparse contribution。

这修复的是“日志指标和真实返回结果不一致”的问题。review 时可以重点看 `mergeKnowledgeBaseSearchResults` 附近的逻辑和 `TestMergeKnowledgeBaseSearchResultsRecomputesFinalRouteMetrics`。

### 2.7 抽象 sparse provider 和 ranker

相关文件：

1. `backend/internal/milvus/retrieval/sparse_search.go`
2. `backend/internal/milvus/retrieval/sparse_search_test.go`

这次把 sparse 拆成两层：

1. `SparseCandidateProvider`：负责根据 term 找候选文档。
2. `SparseRanker`：负责对候选文档排序。

默认实现仍然保持原行为：

1. `MilvusLikeCandidateProvider`：继续用 Milvus `content like "%term%"` 找候选。
2. `CandidateBM25Ranker`：继续在候选集合内构建临时 BM25 倒排索引并排序。

这不是为了马上替换 Milvus LIKE，而是先把替换点留出来。后续如果要换成 Elasticsearch、数据库全文索引、Milvus sparse vector 或外部搜索服务，只需要替换 provider/ranker，不需要改 hybrid 主链路。

### 2.8 增强 sparse term 提取和可观测字段

相关文件：

1. `backend/internal/milvus/retrieval/sparse_search.go`
2. `backend/internal/milvus/retrieval/debug_trace.go`
3. `backend/internal/milvus/retrieval/hybrid_search.go`

term 提取现在会记录：

1. `sparse_terms`：最终参与 sparse 检索的词。
2. `term_sources`：词来源，目前主要是 `original`。
3. `dropped_terms`：被丢弃的词和原因，例如 too short、stopword、duplicate。
4. `per_term_candidate_count`：每个 term 召回了多少候选。
5. `sparse_candidate_before_bm25` / `sparse_candidate_after_bm25`：BM25 前后候选数量。

这解决的问题是：以前 sparse 命中不好时，很难知道是 query 没抽出词、term 被停用词过滤了、LIKE 没查到，还是 BM25 排序后被截断了。现在每个阶段都能看见。

### 2.9 补齐 BM25 explain

相关文件：

1. `backend/internal/milvus/retrieval/sparse_inverted_index.go`
2. `backend/internal/milvus/retrieval/sparse_inverted_index_test.go`
3. `backend/internal/milvus/retrieval/sparse_search_test.go`

BM25 排序现在返回 `SparseBM25Explain`：

1. `matched_terms`：命中的 term。
2. `tf`：term 在当前文档中的出现次数。
3. `df`：term 在候选集合中出现于多少文档。
4. `idf`：term 的区分度。
5. `bm25_score`：最终 BM25 分数。

这让 sparse 的排序从“只看到一个分数”变成“能解释为什么这个 chunk 排名前面”。

### 2.10 reranker 增加可替换接口、fallback 和解释字段

相关文件：

1. `backend/internal/milvus/retrieval/reranker.go`
2. `backend/internal/milvus/retrieval/reranker_test.go`
3. `backend/internal/milvus/retrieval/hybrid_search.go`
4. `backend/internal/milvus/retrieval/debug_trace.go`

现在 reranker 有统一接口：

```go
type Reranker interface {
    Rerank(ctx context.Context, query string, docs []*schema.Document) (*RerankResult, error)
}
```

默认还是 `jaccard-v1`，但外面包了一层 `ConfigurableReranker`，支持：

1. primary reranker 执行。
2. timeout 或 error 后 fallback。
3. fallback reason 记录。
4. `pre_rerank_rank`、`post_rerank_rank`、`score_delta` 等解释字段。

这为后续接 cross-encoder 或外部 rerank 服务留了接口，同时保证高级 reranker 出问题时不会拖垮主检索链路。

### 2.11 离线评测和报告能力增强

相关文件：

1. `backend/cmd/retrieval-eval/main.go`
2. `backend/internal/milvus/evaluation/types.go`
3. `backend/internal/milvus/evaluation/runner.go`
4. `backend/internal/milvus/evaluation/metrics.go`
5. `backend/internal/milvus/evaluation/io.go`
6. `backend/internal/milvus/evaluation/runner_test.go`
7. `backend/scripts/evaluation/dataset.phase2.non_rrf.json`
8. `backend/scripts/evaluation/retrieval_strategy_profiles.phase2.non_rrf.json`
9. `backend/docs/baseline/phase2/baseline_snapshot.json`

评测报告现在不仅看 recall、MRR、nDCG、latency，也会看：

1. dense/sparse hit rate。
2. dense/sparse participation rate。
3. primary dense/sparse rate。
4. dual route rate。
5. dense/sparse route contribution。
6. baseline/candidate fusion strategy。
7. `minmax_v1` 到 `rrf_v1` 的 delta。

这让 RRF 是否值得上线不再靠单次请求观感，而是能跑 baseline/candidate 对照。

## 3. 修复了哪些问题

### 3.1 修复 sparse 统计被误读的问题

以前看到 `sparse_contribution=0` 容易误判为 sparse 没参与。现在通过 `hits / participation / primary_count / contribution` 四层口径拆开，能精确说明 sparse 到底停在哪一步。

### 3.2 修复多 KB 合并后指标不准确的问题

多 KB 检索后，最终返回结果可能经过再次排序和截断。现在合并后基于最终文档重新计算 route 指标，避免日志统计和用户看到的结果不一致。

### 3.3 修复融合和去重后解释信息丢失的问题

同一 chunk 被 dense 和 sparse 同时召回时，去重后仍保留 route 贡献、原始分数、RRF rank、RRF 贡献。debug trace 能解释“它为什么留下来”。

### 3.4 修复 sparse 排序不可解释的问题

BM25 不再只是一个黑盒分数，能看到每个 term 的 TF、DF、IDF 和最终 `bm25_score`。

### 3.5 修复 rerank 失败影响主链路的问题

高级 reranker 如果 timeout 或报错，可以 fallback 到可用排序，并在 trace 中记录 fallback 原因。

### 3.6 修复评测无法覆盖 route 质量的问题

评测报告新增 dense/sparse 参与、主路由、贡献和融合策略字段，能判断优化到底影响了哪一层。

## 4. 实现了哪些功能

1. 支持 `fusion_strategy=minmax_v1/rrf_v1` 配置切换。
2. 实现 RRF 融合算法，支持 `rrf_k`、dense/sparse RRF 权重。
3. 融合结果写入 `fusion_strategy`、`rrf_score`、`route_rank`、`route_rrf_contrib`。
4. 去重后保留多路由贡献解释。
5. 新增 dense/sparse participation、primary count、dual route final count 等指标。
6. debug trace 展示 route hits、fusion results、dedupe results、rerank before/after。
7. sparse 召回拆成 provider 和 ranker 两层。
8. 保留默认 Milvus LIKE provider，保证行为兼容。
9. BM25 排序支持 explain、TopK、MinScore。
10. sparse provider/ranker 失败时降级为空 sparse 结果，不中断 dense 主链路。
11. reranker 支持可替换接口、timeout、fallback 和排序变化解释。
12. 离线评测支持 `minmax_v1` vs `rrf_v1` 对照报告。

## 5. 建议代码 Review 顺序

### 第一步：先看配置和入口

先看：

1. `backend/config.yaml`
2. `backend/internal/config/config.go`
3. `backend/internal/milvus/init.go`

重点确认：

1. 当前默认策略是 `minmax_v1`。
2. `rrf_v1` 可以通过配置打开。
3. RRF 参数有默认值和校验。
4. 配置最终传入 `HybridRetrieverConfig`。

### 第二步：看混合检索主流程

看：

1. `backend/internal/milvus/retrieval/hybrid_search.go`

重点确认：

1. dense 和 sparse 的结果如何进入融合。
2. `FuseRouteCandidates` 如何被调用。
3. `DeduplicateFusedDocuments` 在什么阶段执行。
4. rerank、parent fill、topK 截断在融合之后如何衔接。
5. `buildHybridResultMetrics` 如何计算最终 route 指标。

### 第三步：看融合算法

看：

1. `backend/internal/milvus/retrieval/fusion.go`
2. `backend/internal/milvus/retrieval/fusion_test.go`

重点确认：

1. `minmax_v1` 是否保持老行为。
2. `rrf_v1` 是否按 `weight / (rrf_k + rank)` 计算。
3. dense/sparse 权重是否归一化。
4. 排序是否稳定。
5. metadata 是否写入完整解释字段。

### 第四步：看去重解释

看：

1. `backend/internal/milvus/retrieval/dedupe.go`

重点确认：

1. 去重 key 是否优先使用 `document_id + chunk_id`。
2. 同一文档的 route 贡献是否合并。
3. `primary_route` 是否保留最终最高分 route。
4. `source` metadata 是否同步更新。

### 第五步：看 sparse provider/ranker

看：

1. `backend/internal/milvus/retrieval/sparse_search.go`
2. `backend/internal/milvus/retrieval/sparse_inverted_index.go`
3. `backend/internal/milvus/retrieval/sparse_search_test.go`

重点确认：

1. `SparseCandidateProvider` 是否隔离了 Milvus LIKE。
2. `CandidateBM25Ranker` 是否只负责候选排序。
3. provider/ranker error 是否会 fallback，而不是让整个 hybrid 失败。
4. term source、dropped term、candidate before/after 是否被记录。
5. BM25 explain 是否返回 matched terms 和分数构成。

### 第六步：看 API、日志和 debug trace

看：

1. `backend/api/handler/kb/handler.go`
2. `backend/api/handler/kb/retrieval_debug_trace_v2.go`
3. `backend/internal/model/kb_retrieve_log.go`
4. `backend/api/handler/kb/knowledge_base_binding.go`

重点确认：

1. 新指标是否写入审计日志。
2. debug trace 是否能展示 fusion、dedupe、rerank。
3. 多 KB 合并后是否重新计算最终 route 指标。
4. API response 是否兼容旧字段。

### 第七步：看评测管线

看：

1. `backend/cmd/retrieval-eval/main.go`
2. `backend/internal/milvus/evaluation/runner.go`
3. `backend/internal/milvus/evaluation/metrics.go`
4. `backend/internal/milvus/evaluation/io.go`
5. `backend/internal/milvus/evaluation/runner_test.go`

重点确认：

1. baseline/candidate 是否能携带 fusion strategy。
2. route 指标是否进入 report。
3. gate 是否还能根据 recall、latency、refusal false positive 判断。
4. 报告里是否能看出 `minmax_v1` 和 `rrf_v1` 的差异。

## 6. 验证结果

本次已有的本地验证覆盖了核心链路：

1. fusion 和去重：`TestFuseRouteCandidatesAndDedupe`
2. RRF 字段解释：`TestFuseRouteCandidatesRRFAnnotatesRanksAndContrib`
3. 最终 route 指标：`TestSummarizeFinalRouteStats`
4. sparse BM25：`TestCandidateBM25RankerRanksCandidates`
5. sparse 降级：`TestSparseRetrieverFallsBackWhenRankerFails`
6. 多 KB 指标重算：`TestMergeKnowledgeBaseSearchResultsRecomputesFinalRouteMetrics`
7. debug trace 合约：`TestBuildRetrievalDebugTraceResponseUsesStructuredDebugTrace`
8. 评测聚合：`TestRunnerAggregatesNonRRFRouteMetrics`
9. 策略对比报告：`TestRunnerCarriesFusionStrategyIntoComparisonAndReport`

建议 review 前跑：

```powershell
cd d:\Bear\rag-retrievalOps\backend
go test ./internal/milvus/retrieval ./internal/milvus/evaluation ./api/handler/kb
```

如果本地 MySQL、Milvus、embedding 环境不完整，可以先跑更小范围的核心单测：

```powershell
cd d:\Bear\rag-retrievalOps\backend
go test ./internal/milvus/retrieval -run "TestFuseRouteCandidates|TestSummarizeFinalRouteStats|TestCandidateBM25Ranker|TestSparseRetrieverFallsBack"
go test ./internal/milvus/evaluation -run "TestRunnerAggregatesNonRRFRouteMetrics|TestRunnerCarriesFusionStrategyIntoComparisonAndReport"
```

## 7. 晚上分享时可以这样讲

可以按这个顺序讲：

1. 先说明原问题：不是 dense 没跑，也不是 sparse 没跑，而是观测口径不清楚，导致 `sparse_contribution=0` 被误读。
2. 再说明第一层修复：把 hits、participation、primary count、contribution 拆开。
3. 然后说明第二层改造：把 sparse 拆成 provider 和 ranker，让 LIKE 只是默认实现，不再绑死主链路。
4. 接着说明第三层能力：BM25 explain 和 rerank explain 让排序过程可解释。
5. 最后说明 RRF：通过 `fusion_strategy` 开关引入 `rrf_v1`，能和 `minmax_v1` 做离线对比，也能随时回滚。

一句话总结可以这样说：

> Phase2 这次不是只加 RRF，而是把混合检索链路补成了一个可观测、可解释、可评测、可回滚的工程闭环。RRF 是其中的融合策略升级，sparse 指标修正、provider 抽象、BM25 explain、debug trace 和离线评测才是支撑它能被安全 review 和上线的基础。

## 8. 需要继续关注的风险点

1. 当前默认配置仍是 `minmax_v1`，RRF 需要通过配置切到 `rrf_v1`。
2. sparse 默认 provider 仍然是 Milvus LIKE，不是真正的全文检索引擎。
3. 当前 BM25 是候选集合内 BM25，不是基于完整 KB 语料的持久化 BM25。
4. 中文 query、缩写词、业务术语的 term 提取还只是基础版本，后续需要词典和真实评测集继续优化。
5. 评测数据需要使用真实中文 query 和真实 KB 内容，否则容易出现空结果率偏高，无法准确判断 RRF 收益。
6. 完整端到端验证依赖 Milvus、MySQL、Redis、embedding 服务和真实知识库数据。

