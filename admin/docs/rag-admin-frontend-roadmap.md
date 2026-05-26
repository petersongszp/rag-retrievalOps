# RAG 管理后台前端功能实现路线图（监控看板 + 检索调试 + 后端契约）

## 1. 文档目的与使用方式

本文档基于 `backend/docs/rag-enterprise-fusion-roadmap.md`，专门拆解 `admin` 管理后台的前端建设路线。

当前诉求不是只做一个“能上传文档的后台”，而是把 RAG 企业级能力可视化出来，让学员和项目使用者能看到：

1. 文档从上传、解析、切块、入库到可检索的完整状态。
2. RAG 查询链路里的召回、融合、rerank、过滤、引用、拒答等关键过程。
3. Recall@K、MRR、nDCG、Citation Precision、Empty-After-Filter Rate、Route Contribution、Rewrite Gain 等质量指标。
4. 线上日志、评测结果、策略开关、灰度发布、成本与告警。
5. 后端虽然已经记录日志，但前端要能通过稳定 API 拉取、筛选、聚合、对比和回放。

使用方式：

1. 按 P0 -> P4 分阶段做，不要一开始就做全量大屏。
2. 每个阶段先冻结前后端契约，再实现页面。
3. 每个页面必须绑定一个后端数据来源，不能只做静态展示。
4. 所有检索优化必须能在前端看到“指标变化 + 链路原因 + 回滚入口”。

---

## 2. 当前 `admin` 前端现状

## 2.1 已有能力

基于当前代码扫描，`admin` 已具备以下最小管理能力：

1. Next.js 14 + React 18 + TypeScript + Ant Design 5。
2. 单页知识库管理入口：`admin/src/app/page.tsx`。
3. 知识库列表、创建知识库。
4. 文档上传、文档列表、文档删除。
5. 入库任务列表、失败任务重试、任务取消。
6. 检索测试弹窗，能展示 `content/score/citation/source`。
7. API 客户端统一封装：`admin/src/services/api/client.ts`。
8. 基础类型定义：`admin/src/types/kb.ts`。

## 2.2 关键缺口

1. 页面结构仍是单页，后续监控、评测、调试能力会挤在一起，必须拆成管理台信息架构。
2. 没有 RAG 监控总览页，学员看不到入库成功率、检索 P95、空结果率、召回率趋势。
3. 没有检索链路日志页，无法按 `request_id/query/kb_id/route/status` 检索与回放。
4. 没有检索调试视图，无法展示 `dense + sparse -> fusion -> dedupe -> rerank -> filter/truncate` 的完整链路。
5. 没有离线评测和 A/B 对比页，Recall@10、MRR、nDCG、Rewrite Gain 等指标无法可视化。
6. 没有引用质量页，Citation Precision、Citation Support Score、unsupported claims 无处展示。
7. 没有策略开关和灰度页，Phase 2/3 的 rewrite、dynamic topK、parent-child、evidence refusal 无法在前端管理。
8. 没有成本、告警、审计、Milvus/Collection 运维页，不满足企业级项目展示要求。

---

## 3. 前端建设目标

## 3.1 业务目标

1. 让知识库管理从“上传文件”升级为“可观测的 RAG 管理台”。
2. 让学员能直观看到 RAG 质量优化为什么有效，而不是只看后端日志。
3. 让企业级项目具备质量、稳定性、成本、审计、回滚的可视化闭环。

## 3.2 技术目标

1. 前端页面按业务域拆分，避免单页继续膨胀。
2. 所有表格、趋势图、调试视图都依赖后端 API，不硬编码假数据。
3. 指标支持时间范围、知识库、策略版本、实验组、query_type 维度筛选。
4. 检索日志支持从“聚合指标”下钻到“单次 request trace”。
5. 每个高级策略都能看到收益、风险、延迟、成本和开关状态。

## 3.3 第一版核心展示指标

1. 入库成功率：`ingest_success_rate`。
2. 平均入库耗时：`avg_ingest_duration_ms`。
3. 检索 P50/P95：`retrieval_p50_ms`、`retrieval_p95_ms`。
4. 空结果率：`empty_result_rate`。
5. Recall@10：`recall_at_10`。
6. MRR：`mrr`。
7. nDCG@10：`ndcg_at_10`。
8. 引用准确率：`citation_precision`。
9. Citation Support Score：`citation_support_score`。
10. Empty-After-Filter Rate：`empty_after_filter_rate`。
11. Route Contribution：`dense/sparse/rewrite/rerank` 贡献占比。
12. Rewrite Gain：rewrite 开启前后的召回增益。
13. Score Completeness：最终结果 `score` 字段完整率。
14. 查询链路成本：`embedding_tokens/llm_tokens/retrieval_candidates`。

---

## 4. 前端能力全景

## 4.1 信息架构建议

建议把 `admin` 从单页升级为以下导航结构：

1. `Dashboard`：RAG 运营总览。
2. `Knowledge Bases`：知识库、文档、入库任务管理。
3. `Retrieval Lab`：检索测试、引用展示、链路调试。
4. `Trace Logs`：结构化检索日志、入库日志、异常日志。
5. `Evaluation`：离线评测集、评测运行、指标对比、回归门禁。
6. `Strategy Center`：feature flags、策略版本、灰度、回滚。
7. `Quality Monitor`：召回质量、引用质量、拒答质量、空结果分析。
8. `Cost & Ops`：token 成本、Milvus/Collection 健康、容量与索引状态。
9. `Audit`：管理员操作审计、查询审计、导出记录。

## 4.2 页面与后端能力映射

1. 监控总览页依赖指标聚合 API，而不是直接扫日志文件。
2. 检索日志页依赖结构化 trace 存储，必须支持分页、筛选、详情。
3. 检索调试页依赖单次请求的阶段化 trace，包括 route、rerank、filter、citation。
4. 评测对比页依赖离线评测运行结果，必须保留 baseline 和 candidate。
5. 策略中心依赖后端 feature flag 和配置版本，必须支持开关、灰度、回滚记录。
6. 成本运维页依赖 token、候选数、Collection 健康和索引版本。

---

## 5. 分阶段实现路线

## Phase 0（P0）管理台基础重构与知识库闭环

**目标**：让当前单页后台变成可扩展的管理台骨架，同时保持知识库上传、任务、检索测试可用。

### 5.0.1 前端任务

1. 拆分路由结构：
   - `/dashboard`
   - `/knowledge-bases`
   - `/knowledge-bases/[kbId]`
   - `/retrieval-lab`
2. 抽离公共布局：
   - 顶部 Header
   - 左侧导航 Sider
   - 面包屑 Breadcrumb
   - 当前知识库选择器
3. 抽离业务组件：
   - `KnowledgeBaseList`
   - `DocumentTable`
   - `IngestJobTable`
   - `UploadDocumentModal`
   - `RetrieveTestPanel`
   - `CitationCard`
4. 完善现有检索结果展示：
   - 展示 `score`
   - 展示 `citation.file_name/chunk_index/chunk_id`
   - 展示 `source.route/collection/retriever_version`
   - 缺字段时明确标红，暴露契约不完整问题
5. 增加最小状态卡片：
   - 知识库数量
   - 文档数量
   - 处理中任务数
   - 失败任务数

### 5.0.2 后端需要配合

1. 当前已有知识库、文档、任务、检索 API 继续保持稳定。
2. 检索接口必须稳定返回 `request_id/items/content/score/citation/source`。
3. 文档列表建议补充：
   - `ingest_duration_ms`
   - `last_ingest_job_id`
   - `chunk_count`
   - `file_hash`
4. 任务列表建议补充：
   - `stage`
   - `progress`
   - `retry_count`
   - `error_code`
   - `error_msg`
   - `started_at`
   - `finished_at`

### 5.0.3 验收

1. 页面拆分后原有上传、删除、任务重试、检索测试功能不回退。
2. 检索结果能完整展示 `score/citation/source`。
3. 当前后台具备继续扩展监控页面的布局基础。

---

## Phase 1（P1）RAG 监控总览与结构化日志可视化

**目标**：把后端已有日志变成可筛选、可下钻、可教学展示的监控面板。

### 5.1.1 前端任务

1. 新增 `/dashboard` 监控总览页：
   - 入库成功率趋势
   - 入库任务状态分布
   - 检索请求量趋势
   - 检索 P50/P95 趋势
   - 空结果率趋势
   - 失败类型 TopN
2. 新增 `/trace-logs/retrieval` 检索日志页：
   - 按时间、知识库、用户、query、request_id、route、是否空结果筛选
   - 表格展示 `query/kb_id/topk/final_count/duration_ms/status`
   - 支持点击进入 trace 详情
3. 新增 `/trace-logs/ingest` 入库日志页：
   - 按任务状态、错误类型、文件类型、知识库筛选
   - 展示解析、切块、embedding、写入向量库耗时
4. 新增 trace 详情抽屉：
   - 基础请求信息
   - 阶段耗时瀑布图
   - route 命中数量
   - rerank 数量
   - filter 前后数量
   - final results
   - error detail
5. 支持 `request_id` 复制和跳转，方便教学演示和问题定位。

### 5.1.2 后端需要配合

1. 新增指标聚合 API：
   - `GET /api/admin/kb/metrics/overview`
   - `GET /api/admin/kb/metrics/ingest`
   - `GET /api/admin/kb/metrics/retrieval`
2. 新增结构化日志查询 API：
   - `GET /api/admin/kb/logs/retrieval`
   - `GET /api/admin/kb/logs/retrieval/{request_id}`
   - `GET /api/admin/kb/logs/ingest`
   - `GET /api/admin/kb/logs/ingest/{job_id}`
3. 检索日志最小字段：
   - `request_id`
   - `query`
   - `user_id`
   - `kb_id`
   - `expr`
   - `topk`
   - `rewrite`
   - `routes`
   - `final_count`
   - `duration_ms`
   - `stage_durations`
   - `empty_reason`
   - `status`
   - `created_at`
4. 入库日志最小字段：
   - `job_id`
   - `document_id`
   - `kb_id`
   - `file_name`
   - `stage`
   - `status`
   - `chunk_count`
   - `duration_ms`
   - `error_code`
   - `error_msg`
   - `created_at`
5. 后端需要把日志从“普通文本日志”沉淀为可查询存储，可以先用数据库表，后续再接 ELK、Loki 或 ClickHouse。

### 5.1.3 验收

1. 管理台能看到最近 1h/24h/7d 的 RAG 关键指标。
2. 任意一次检索请求可以通过 `request_id` 查到结构化 trace。
3. 失败任务和空结果能定位到明确原因分类。

---

## Phase 2（P2）检索质量与离线评测看板

**目标**：把 Recall@K、MRR、nDCG、引用准确率等 RAG 优化指标做成可对比看板。

### 5.2.1 前端任务

1. 新增 `/evaluation/datasets` 评测集页面：
   - `qa_goldens` 列表
   - query 类型分布
   - gold docs/chunks 覆盖情况
   - 导入、导出、校验状态
2. 新增 `/evaluation/runs` 评测运行页面：
   - 创建评测运行
   - 选择 baseline/candidate
   - 选择策略配置
   - 查看运行状态
3. 新增 `/evaluation/reports/[runId]` 评测报告页：
   - Recall@K
   - HitRate@K
   - MRR
   - nDCG@K
   - Citation Precision
   - Empty-After-Filter Rate
   - Score Completeness
   - P50/P95 延迟
   - token 成本
4. 新增 A/B 对比视图：
   - `phase1_baseline` vs `hybrid_retrieval`
   - `hybrid_retrieval` vs `hybrid+rewrite`
   - `hybrid+rewrite` vs `hybrid+dynamic_topk`
5. 新增问题样本列表：
   - 召回失败样本
   - 引用不支撑样本
   - filter 后为空样本
   - rewrite 负收益样本
6. 检索测试页增加“保存为评测样本”按钮，方便课堂演示从真实 query 沉淀评测集。

### 5.2.2 后端需要配合

1. 新增评测集 API：
   - `GET /api/admin/kb/eval/datasets`
   - `POST /api/admin/kb/eval/datasets`
   - `GET /api/admin/kb/eval/datasets/{id}/items`
   - `POST /api/admin/kb/eval/datasets/{id}/items`
   - `POST /api/admin/kb/eval/datasets/{id}/import`
2. 新增评测运行 API：
   - `POST /api/admin/kb/eval/runs`
   - `GET /api/admin/kb/eval/runs`
   - `GET /api/admin/kb/eval/runs/{run_id}`
   - `GET /api/admin/kb/eval/runs/{run_id}/report`
   - `GET /api/admin/kb/eval/runs/{run_id}/cases`
3. 评测结果必须持久化：
   - `run_id`
   - `dataset_id`
   - `baseline_config`
   - `candidate_config`
   - `metrics`
   - `case_results`
   - `created_at`
4. 每条 case 结果建议包含：
   - `query`
   - `query_type`
   - `gold_doc_ids`
   - `gold_chunk_ids`
   - `retrieved_doc_ids`
   - `retrieved_chunk_ids`
   - `hit`
   - `rank`
   - `recall`
   - `citation_supported`
   - `failure_reason`

### 5.2.3 验收

1. 前端能展示一次评测运行的完整指标报告。
2. 能对比 baseline 和 candidate 的 Recall@10、MRR、nDCG、Citation Precision。
3. 能下钻到失败样本，并跳转到对应 request trace 或重新执行检索。

---

## Phase 3（P3）高级检索调试视图与策略中心

**目标**：把 Phase 3 高级检索能力变成可解释、可灰度、可回滚的前端工作台。

### 5.3.1 前端任务

1. 新增 `/retrieval-lab/debug` 检索调试页：
   - original query
   - rewritten query
   - route-specific final query
   - dense/sparse/rewrite route hits
   - fusion 前后对比
   - dedupe 前后对比
   - rerank 前后排序
   - filter/truncate 原因
   - final result citations
2. 新增 parent-child 调试视图：
   - child hit 列表
   - parent 聚合结果
   - sibling/section window 回填结果
   - token 预算截断过程
   - `parent_fill_strategy` 与 `parent_fill_tokens`
3. 新增动态 TopK 决策视图：
   - score distribution
   - rerank gap
   - evidence density
   - token budget
   - final K
   - `topk_decision_reason`
4. 新增证据不足拒答视图：
   - evidence gate result
   - refusal reason
   - threshold 命中情况
   - 是否误伤
5. 新增引用一致性视图：
   - answer claims
   - citation snippets
   - `citation_support_score`
   - `unsupported_claims`
6. 新增 `/strategy-center` 策略中心：
   - feature flag 列表
   - 当前策略版本
   - 灰度比例
   - 最近指标变化
   - 一键回滚按钮
7. 支持“按策略查看收益”：
   - Parent Fill Gain
   - Rewrite Gain
   - Route Contribution
   - Evidence Refusal Rate
   - Refusal False Positive Rate

### 5.3.2 后端需要配合

1. 检索 trace 详情扩展字段：
   - `original_query`
   - `rewritten_query`
   - `route_final_queries`
   - `route_hits`
   - `fusion_results`
   - `dedupe_results`
   - `rerank_results`
   - `filter_results`
   - `final_results`
2. parent-child 字段：
   - `parent_child_enabled`
   - `parent_fill_strategy`
   - `parent_fill_count`
   - `parent_fill_tokens`
   - `child_hits`
   - `parent_contexts`
3. TopK 字段：
   - `topk_policy_version`
   - `score_distribution`
   - `rerank_gap`
   - `evidence_density`
   - `topk_decision_reason`
   - `final_topk`
4. evidence gate 字段：
   - `evidence_gate_result`
   - `refusal_reason`
   - `thresholds`
   - `evidence_gate_error`
5. citation consistency 字段：
   - `citation_supported`
   - `citation_support_score`
   - `unsupported_claims`
   - `citation_check_version`
6. 策略中心 API：
   - `GET /api/admin/kb/strategy/flags`
   - `PATCH /api/admin/kb/strategy/flags/{flag_key}`
   - `GET /api/admin/kb/strategy/versions`
   - `POST /api/admin/kb/strategy/rollback`
   - `GET /api/admin/kb/strategy/impact`
7. 后端必须保证策略开关可独立关闭：
   - `RAG_ENABLE_PARENT_CHILD_RETRIEVAL`
   - `RAG_ENABLE_STRATEGIC_TOPK`
   - `RAG_ENABLE_EVIDENCE_REFUSAL`
   - `RAG_ENABLE_CITATION_CONSISTENCY`
   - `RAG_ENABLE_DOMAIN_TERMS`
   - `RAG_ENABLE_ROUTE_SPECIFIC_REWRITE`
   - `RAG_ENABLE_MODEL_ASSISTED_REWRITE`

### 5.3.3 验收

1. 一次复杂 query 可以在前端完整还原检索链路。
2. 任一高级策略能看到开启前后指标变化。
3. 策略异常时，管理员能在前端关闭开关并留下操作记录。

---

## Phase 4（P4）企业治理、成本、告警与审计

**目标**：让管理台达到企业级可运维标准，形成质量、成本、安全、合规闭环。

### 5.4.1 前端任务

1. 新增 `/cost-ops/cost` 成本看板：
   - embedding token
   - LLM token
   - 平均候选数
   - 平均上下文 token
   - 单知识库成本趋势
   - 单策略成本变化
2. 新增 `/cost-ops/vector-db` Milvus/Collection 运维页：
   - Collection 列表
   - active Collection 标记
   - Collection 健康状态
   - 容量、实体数、索引状态
   - 重建、切换、回滚记录
3. 新增 `/alerts` 告警页：
   - Recall 下降告警
   - P95 延迟升高告警
   - Empty-After-Filter Rate 异常告警
   - Citation Precision 下降告警
   - 入库失败率异常告警
4. 新增 `/audit` 审计页：
   - 管理员操作记录
   - 策略开关变更记录
   - 文档删除记录
   - query trace 访问记录
   - 报告导出记录
5. 自动化周报页面：
   - 本周质量变化
   - 本周稳定性变化
   - 本周成本变化
   - Top 问题样本
   - 策略收益总结

### 5.4.2 后端需要配合

1. 成本 API：
   - `GET /api/admin/kb/cost/summary`
   - `GET /api/admin/kb/cost/timeseries`
   - `GET /api/admin/kb/cost/by-kb`
   - `GET /api/admin/kb/cost/by-strategy`
2. Vector DB 运维 API：
   - `GET /api/admin/kb/vector/collections`
   - `GET /api/admin/kb/vector/collections/{name}/health`
   - `POST /api/admin/kb/vector/collections/{name}/rebuild`
   - `POST /api/admin/kb/vector/collections/{name}/switch`
   - `POST /api/admin/kb/vector/collections/{name}/rollback`
3. 告警 API：
   - `GET /api/admin/kb/alerts`
   - `PATCH /api/admin/kb/alerts/{alert_id}/ack`
   - `PATCH /api/admin/kb/alerts/{alert_id}/resolve`
   - `GET /api/admin/kb/alert-rules`
   - `POST /api/admin/kb/alert-rules`
4. 审计 API：
   - `GET /api/admin/kb/audit/events`
   - `GET /api/admin/kb/audit/events/{event_id}`
   - `POST /api/admin/kb/reports/weekly`
5. 后端需要记录审计事件：
   - `actor_id`
   - `actor_name`
   - `action`
   - `resource_type`
   - `resource_id`
   - `before`
   - `after`
   - `request_id`
   - `ip`
   - `created_at`

### 5.4.3 验收

1. 管理台能从质量、稳定性、成本、安全四个维度观察 RAG 系统。
2. 策略变更、文档删除、Collection 切换都有审计记录。
3. 指标异常能产生告警，并能跳转到相关 trace 或评测报告。

---

## 6. 后端需要优先补齐的能力清单

如果要配合前端把企业级监控真正做起来，后端建议优先补以下能力。

## 6.1 结构化日志存储

不要只把 RAG 过程写进普通日志文件。前端需要可查询数据，因此后端至少要把以下内容落库：

1. `retrieval_trace`：单次检索请求 trace。
2. `retrieval_trace_stage`：embedding/search/fusion/rerank/filter/answer 阶段耗时。
3. `retrieval_trace_result`：每个候选结果的 score、route、citation、source。
4. `ingest_trace`：文档入库任务 trace。
5. `ingest_trace_stage`：parse/chunk/embed/write_milvus 阶段耗时。
6. `eval_run` 与 `eval_case_result`：离线评测运行和样本结果。
7. `strategy_flag` 与 `strategy_change_log`：策略开关和变更记录。
8. `audit_event`：管理员操作审计。

## 6.2 指标聚合层

前端不应该自己扫全量日志做聚合。后端需要提供按时间窗口聚合的 API：

1. 按 `time_range` 聚合。
2. 按 `kb_id` 聚合。
3. 按 `query_type` 聚合。
4. 按 `strategy_version` 聚合。
5. 按 `route` 聚合。
6. 按 `error_code/empty_reason/refusal_reason` 聚合。

## 6.3 统一 request_id

后端每次检索、入库、评测运行都应该生成并贯穿：

1. `request_id`：单次用户请求。
2. `trace_id`：链路追踪 ID，可与 request_id 相同或映射。
3. `run_id`：离线评测运行 ID。
4. `job_id`：入库任务 ID。

前端所有“从指标下钻到日志”的体验，都依赖这些 ID。

## 6.4 可复现的检索调试数据

检索调试页不是只看最终答案，而是要能还原过程。后端需要在 debug 模式或采样模式下保存：

1. 原 query。
2. rewrite 后 query。
3. route-specific final query。
4. dense/sparse 各自召回候选。
5. fusion 和 dedupe 后候选。
6. rerank 前后排序。
7. filter/truncate 原因。
8. evidence gate 结果。
9. citation consistency 结果。
10. final response 和 final citations。

## 6.5 Feature Flag 与回滚接口

后端需要把高级策略都做成可独立开关，前端才能安全展示和管理：

1. 查询策略开关列表。
2. 修改策略开关。
3. 查询策略版本。
4. 查询策略影响指标。
5. 一键回滚到上一稳定版本。
6. 写入策略变更审计。

## 6.6 评测任务异步化

离线评测可能耗时较长，后端需要把它做成异步任务：

1. 创建评测任务后立即返回 `run_id`。
2. 前端轮询或 SSE 获取进度。
3. 完成后生成 report。
4. 失败时保留失败样本和错误信息。
5. 支持 baseline/candidate 对比。

## 6.7 权限与脱敏

企业级后台涉及 query、文档内容、引用片段和用户信息，后端需要支持：

1. 管理员角色权限。
2. query 日志脱敏。
3. 文档片段脱敏。
4. trace 访问审计。
5. 报告导出权限。

---

## 7. 推荐 API 契约草案

## 7.1 监控总览

```http
GET /api/admin/kb/metrics/overview?kb_id=1&range=24h
```

建议返回：

```json
{
  "range": "24h",
  "cards": {
    "ingest_success_rate": 0.992,
    "retrieval_p95_ms": 2380,
    "empty_result_rate": 0.036,
    "recall_at_10": 0.86,
    "citation_precision": 0.95
  },
  "timeseries": [
    {
      "time": "2026-05-25T10:00:00+08:00",
      "retrieval_count": 120,
      "retrieval_p95_ms": 2400,
      "empty_result_rate": 0.03
    }
  ],
  "top_errors": [
    {
      "error_code": "Empty-After-Filter",
      "count": 18
    }
  ]
}
```

## 7.2 检索日志列表

```http
GET /api/admin/kb/logs/retrieval?kb_id=1&query=redis&range=24h&page=1&page_size=20
```

建议返回：

```json
{
  "items": [
    {
      "request_id": "req_abc",
      "kb_id": 1,
      "query": "redis 分布式锁怎么实现",
      "topk": 10,
      "routes": ["dense", "sparse"],
      "final_count": 5,
      "duration_ms": 1280,
      "status": "success",
      "empty_reason": null,
      "created_at": "2026-05-25T10:00:00+08:00"
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 20
}
```

## 7.3 检索 trace 详情

```http
GET /api/admin/kb/logs/retrieval/req_abc
```

建议返回：

```json
{
  "request_id": "req_abc",
  "original_query": "redis 分布式锁怎么实现",
  "rewritten_query": "redis distributed lock setnx redlock",
  "kb_id": 1,
  "stage_durations": {
    "embedding": 120,
    "dense_search": 180,
    "sparse_search": 90,
    "fusion": 20,
    "rerank": 300,
    "answer": 600
  },
  "routes": [
    {
      "route": "dense",
      "hit_count": 20,
      "contribution": 0.62
    },
    {
      "route": "sparse",
      "hit_count": 16,
      "contribution": 0.38
    }
  ],
  "topk_decision": {
    "policy_version": "strategic_topk_v1",
    "final_topk": 8,
    "reason": "score_distribution_flat"
  },
  "evidence_gate": {
    "result": "passed",
    "refusal_reason": null
  },
  "citation_check": {
    "citation_supported": true,
    "citation_support_score": 0.93,
    "unsupported_claims": []
  },
  "final_results": []
}
```

## 7.4 评测报告

```http
GET /api/admin/kb/eval/runs/run_001/report
```

建议返回：

```json
{
  "run_id": "run_001",
  "dataset_id": "qa_goldens_core",
  "baseline": "phase1_baseline",
  "candidate": "hybrid_retrieval_v1",
  "metrics": {
    "recall_at_10": 0.87,
    "mrr": 0.74,
    "ndcg_at_10": 0.79,
    "citation_precision": 0.95,
    "empty_after_filter_rate": 0.02,
    "score_completeness": 1.0,
    "retrieval_p95_ms": 2600
  },
  "deltas": {
    "recall_at_10": 0.08,
    "mrr": 0.05,
    "retrieval_p95_ms": 220
  }
}
```

---

## 8. 推荐实施节奏

1. 第一步先完成 P0：把当前单页后台拆成可扩展管理台骨架。
2. 第二步做 P1：先把结构化日志、监控总览、trace 详情打通，这一步最能体现“企业级可观测”。
3. 第三步做 P2：把 Recall@10、MRR、nDCG、Citation Precision 做成评测看板，给 RAG 优化提供证据。
4. 第四步做 P3：做检索链路调试和策略中心，让 Phase 3 高级检索能力可解释、可灰度。
5. 第五步做 P4：补成本、告警、审计、Milvus 运维，形成完整企业治理闭环。

建议前后端并行方式：

1. 后端先交 OpenAPI/JSON 示例，前端先基于 mock 实现页面骨架。
2. 每个页面先接列表和详情，再补图表。
3. 指标口径先写死在文档里，后续不要在页面和后端各解释一套。
4. 所有接口都保留 `request_id`，便于从图表跳到日志。

---

## 9. 阶段验收模板

每个阶段完成后按以下模板验收：

1. 已完成页面：
2. 已接入 API：
3. 已展示指标：
4. 可下钻链路：
5. 后端缺口：
6. 发现的问题样本：
7. 是否影响现有知识库上传/检索：
8. 是否可以进入下一阶段：

---

## 10. 第一批立即执行任务

1. 新建 `admin/docs/rag-admin-frontend-roadmap.md`，作为前端管理台主路线文档。
2. 后端补 `retrieval_trace` 和 `ingest_trace` 的数据结构设计。
3. 前端拆分 `admin/src/app/page.tsx`，先形成 Layout + 知识库管理页。
4. 新增 `/dashboard`，先接入最小指标卡片。
5. 新增 `/trace-logs/retrieval`，先打通检索日志列表和 `request_id` 详情。
6. 新增 `/evaluation/runs` 的静态骨架，等待后端评测 API。

这份路线图的核心原则是：先让日志可见，再让指标可比，最后让策略可控。
