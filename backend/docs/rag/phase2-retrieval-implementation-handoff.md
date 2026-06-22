# Phase 2 Retrieval Implementation Handoff

## 1. Completed Commits

1. `158f22a` `Phase2 非RRF L0：冻结检索 baseline 快照`
2. `bcd2f87` `Phase2 非RRF L1：修正 sparse 参与与贡献统计口径`
3. `e7fd629` `Phase2 非RRF L7：补齐最小 replay 与离线评估`
4. `3b3cd7c` `Phase2 RRF L0-L1：补齐开关并实现融合骨架`
5. `d1e2ceb` `Phase2 RRF L2-L5：对齐去重解释并补齐评测测试`
6. `bb3dda4` `Phase2 非RRF L2-L3：抽象 sparse provider 并增强 term 提取`
7. `d3b395a` `Phase2 非RRF L4-L6：补齐 BM25 explain 与降级重排解释`

## 2. What Each Commit Delivered

### 2.1 non-RRF L0

- frozen non-RRF baseline assets
- dedicated dataset/profile files
- baseline snapshot/template assets
- evaluation readme and report template alignment

Key files:

- [baseline_snapshot.json](/d:/Bear/rag-retrievalOps/backend/docs/baseline/phase2/baseline_snapshot.json)
- [dataset.phase2.non_rrf.json](/d:/Bear/rag-retrievalOps/backend/scripts/evaluation/dataset.phase2.non_rrf.json)
- [retrieval_strategy_profiles.phase2.non_rrf.json](/d:/Bear/rag-retrievalOps/backend/scripts/evaluation/retrieval_strategy_profiles.phase2.non_rrf.json)

### 2.2 non-RRF L1

- split `hits / participation / primary_count / contribution`
- fixed sparse observability semantics
- extended retrieve log and debug trace contracts
- recomputed final route metrics after multi-KB merge

Key files:

- [search.go](/d:/Bear/rag-retrievalOps/backend/internal/milvus/retrieval/search.go)
- [hybrid_search.go](/d:/Bear/rag-retrievalOps/backend/internal/milvus/retrieval/hybrid_search.go)
- [handler.go](/d:/Bear/rag-retrievalOps/backend/api/handler/kb/handler.go)
- [retrieval_debug_trace_v2.go](/d:/Bear/rag-retrievalOps/backend/api/handler/kb/retrieval_debug_trace_v2.go)

### 2.3 non-RRF L7

- minimal replay/evaluation loop
- query-level route metrics in evaluation results
- aggregate metrics for sparse hit/participation/primary/empty rate
- report/profile bundle support

Key files:

- [types.go](/d:/Bear/rag-retrievalOps/backend/internal/milvus/evaluation/types.go)
- [runner.go](/d:/Bear/rag-retrievalOps/backend/internal/milvus/evaluation/runner.go)
- [io.go](/d:/Bear/rag-retrievalOps/backend/internal/milvus/evaluation/io.go)

### 2.4 RRF L0-L1

- runtime/eval config for `fusion_strategy` and `rrf_k`
- minmax/rrf profile separation
- real `rrf_v1` fusion path added beside `minmax_v1`
- fusion strategy written into metrics/log/trace

Key files:

- [config.go](/d:/Bear/rag-retrievalOps/backend/internal/config/config.go)
- [fusion.go](/d:/Bear/rag-retrievalOps/backend/internal/milvus/retrieval/fusion.go)
- [init.go](/d:/Bear/rag-retrievalOps/backend/internal/milvus/init.go)

### 2.5 RRF L2-L5

- dedupe now preserves `primary_route`
- added `route_rank` and `route_rrf_contrib`
- report/test coverage for fusion strategy comparison

Key files:

- [dedupe.go](/d:/Bear/rag-retrievalOps/backend/internal/milvus/retrieval/dedupe.go)
- [fusion_test.go](/d:/Bear/rag-retrievalOps/backend/internal/milvus/retrieval/fusion_test.go)
- [runner_test.go](/d:/Bear/rag-retrievalOps/backend/internal/milvus/evaluation/runner_test.go)

### 2.6 non-RRF L2-L3

- extracted `SparseCandidateProvider`
- extracted `SparseRanker`
- default provider remains `MilvusLikeCandidateProvider`
- term extraction now records `source`, `kind`, `dropped terms`

Key files:

- [sparse_search.go](/d:/Bear/rag-retrievalOps/backend/internal/milvus/retrieval/sparse_search.go)
- [sparse_search_test.go](/d:/Bear/rag-retrievalOps/backend/internal/milvus/retrieval/sparse_search_test.go)

### 2.7 non-RRF L4-L6

- BM25 explain payload added
- sparse ranker supports `TopK` and `MinScore`
- sparse provider/ranker failure now degrades instead of breaking hybrid
- reranker explain adds pre/post rank and score delta

Key files:

- [sparse_inverted_index.go](/d:/Bear/rag-retrievalOps/backend/internal/milvus/retrieval/sparse_inverted_index.go)
- [reranker.go](/d:/Bear/rag-retrievalOps/backend/internal/milvus/retrieval/reranker.go)

## 3. Suggested Regression Commands

Use these stable local checks first:

```powershell
cd d:\Bear\rag-retrievalOps\backend
go test ./internal/milvus/retrieval -run 'TestCandidateBM25RankerRanksCandidates|TestSparseRetrieverFallsBackWhenRankerFails|TestBuildSparseInvertedIndexSearchRanksByBM25|TestBuildSparseInvertedIndexAssignsPseudoDocID|TestJaccardRerankerAnnotatesSourceContract|TestConfigurableRerankerFallsBackOnPrimaryError|TestFuseRouteCandidatesAndDedupe|TestFuseRouteCandidatesRRFAnnotatesRanksAndContrib|TestSummarizeFinalRouteStats'
```

```powershell
cd d:\Bear\rag-retrievalOps\backend
go test ./api/handler/kb -run 'TestBuildRetrieveDebugTraceIncludesParentFillDiff|TestBuildRetrievalDebugTraceResponseUsesStructuredDebugTrace|TestBuildRetrievalDebugTraceResponseFallbackMarksContractGaps|TestMergeKnowledgeBaseSearchResultsRecomputesFinalRouteMetrics'
```

```powershell
cd d:\Bear\rag-retrievalOps\backend
go test ./internal/milvus/evaluation -run 'TestRunnerBuildsComparisonAndGate|TestRunnerFlagsRefusalFalsePositiveGate|TestRunnerAggregatesNonRRFRouteMetrics|TestRunnerCarriesFusionStrategyIntoComparisonAndReport'
```

If environment is ready:

```powershell
cd d:\Bear\rag-retrievalOps\backend
go run ./cmd/retrieval-eval -config ./config.yaml -dataset ./scripts/evaluation/dataset.phase2.non_rrf.json -profiles ./scripts/evaluation/retrieval_strategy_profiles.phase2.non_rrf.json -gates ./scripts/evaluation/retrieval_gate_thresholds.example.json -output ./docs/baseline/phase2/non-rrf-baseline
```

## 4. Real-Data Follow-ups Still Needed

1. Replace placeholder chunk IDs in:
   - [non_rrf_dataset.template.json](/d:/Bear/rag-retrievalOps/backend/scripts/evaluation/non_rrf_dataset.template.json)
   - [non_rrf_baseline_snapshot.template.json](/d:/Bear/rag-retrievalOps/backend/docs/baseline/phase2/non_rrf_baseline_snapshot.template.json)
2. Fill real baseline metrics in:
   - [baseline_snapshot.json](/d:/Bear/rag-retrievalOps/backend/docs/baseline/phase2/baseline_snapshot.json)
3. Run real `minmax_v1` vs `rrf_v1` replay with Milvus/embedding/data available.
4. Confirm DB migration on a real database, since new `kb_retrieve_log` columns rely on `AutoMigrate`.
5. If needed, extend Prometheus metrics for fusion strategy and sparse degradation counters.

## 5. Environment Caveats

1. Full `go test ./api/handler/kb` still depends on local MySQL at `127.0.0.1:3307`.
2. Real replay depends on Milvus, embedding config, and populated collections.
3. The two roadmap files below were intentionally left untouched and untracked:
   - `backend/docs/rag/phase2-non-rrf-retrieval-optimization-detailed-roadmap.md`
   - `backend/docs/rag/phase2-rrf-fusion-detailed-roadmap.md`
