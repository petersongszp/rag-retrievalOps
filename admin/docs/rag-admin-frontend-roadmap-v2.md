# RAG 管理后台前端路线大纲（v2）

## 1. 文档目的与使用方式

本文档基于 `backend/docs/rag-enterprise-fusion-roadmap.md`，描述管理后台前端在每个后端阶段完成后能展示什么、依赖哪些 API、字段口径是什么。

**定位**：前端是后端能力的消费方，不驱动后端做新东西。后端路线大纲是主文档，本文档是它的前端视角摘要。

使用方式：

1. 后端每完成一个阶段，对照本文档的对应 Phase 实现前端页面。
2. 每个 Phase 先冻结 API 字段口径，再实现页面。
3. 后端没有的数据，前端不伪造，只展示空状态或契约缺口提示。
4. 每个 Phase 完成后可按本文档拆出对应的详细实现路线文档。

---

## 2. 当前前端现状

### 2.1 已有能力

1. Next.js 14 + React 18 + TypeScript + Ant Design 5。
2. 单页知识库管理：`admin/src/app/page.tsx`。
3. 知识库列表、创建知识库。
4. 文档上传、文档列表、文档删除。
5. 入库任务列表、失败任务重试、任务取消。
6. 检索测试弹窗，能展示 `content/score/citation/source`。
7. API 客户端：`admin/src/services/api/client.ts`。
8. 基础类型定义：`admin/src/types/kb.ts`。

### 2.2 关键缺口

1. 单页结构，无法扩展监控、评测、调试能力。
2. 没有 RAG 监控总览，学员看不到入库成功率、检索 P95、空结果率。
3. 没有检索链路日志，无法按 `request_id` 查看 trace。
4. 没有离线评测看板，Recall@K、MRR、nDCG 等指标无处展示。
5. 没有高级检索调试视图，rewrite/rerank/filter 链路不可见。

---

## 3. 前端建设目标

**核心目标**：让学员能直观看到 RAG 质量优化为什么有效，而不是只看后端日志。

具体来说：

1. 文档从上传到可检索的完整状态可见。
2. 检索链路里的召回、融合、rerank、过滤、引用等关键过程可见。
3. Recall@K、MRR、nDCG、Citation Precision 等质量指标可对比。
4. 高级检索策略的开关状态、收益、风险可管理。

---

## 4. 信息架构

把 `admin` 从单页升级为以下导航结构，按 Phase 逐步激活：

| 导航项 | 激活阶段 | 说明 |
|---|---|---|
| Dashboard | P0 | 最小状态卡片，P1 补趋势图 |
| Knowledge Bases | P0 | 知识库、文档、入库任务管理 |
| Retrieval Lab | P0 | 检索测试与引用展示，P3 补调试视图 |
| Trace Logs | P1 | 结构化检索日志与入库日志 |
| Evaluation | P2 | 离线评测集、评测运行、指标对比 |
| Strategy Center | P3 | feature flags、策略版本、灰度、回滚 |
| Cost & Ops | P4 | token 成本、Milvus 健康、容量 |
| Audit | P4 | 管理员操作审计、查询审计 |

---

## 5. 分阶段实现路线

### Phase 0（P0）管理台基础重构与知识库闭环

**对应后端**：`rag-enterprise-fusion-roadmap.md` Phase 0 闭环可用期。

**目标**：把单页后台拆成可扩展管理台骨架，保持知识库上传、任务、检索测试可用。

#### 5.0.1 前端任务

1. 拆分路由结构：
   - `/dashboard`
   - `/knowledge-bases`
   - `/knowledge-bases/[kbId]`
   - `/retrieval-lab`
2. 抽离公共布局：Header、Sider、Breadcrumb、当前知识库选择器。
3. 抽离业务组件：
   - `KnowledgeBaseList`
   - `DocumentTable`
   - `IngestJobTable`
   - `UploadDocumentModal`
   - `RetrieveTestPanel`
   - `CitationCard`
4. 检索结果完整展示 `score/citation/source`，缺字段时明确标红。
5. Dashboard 展示最小状态卡片：知识库数量、文档数量、处理中任务数、失败任务数。
6. 预留 P1 导航入口：Trace Logs（禁用态）。

#### 5.0.2 依赖后端 API

后端 P0 已有或需稳定的接口：

- `POST /api/admin/kb/bases`
- `GET /api/admin/kb/bases`
- `POST /api/admin/kb/documents/upload`
- `GET /api/admin/kb/documents?kb_id=`
- `GET /api/admin/kb/jobs/:job_id`
- `POST /api/admin/kb/jobs/:job_id/retry`
- `POST /api/admin/kb/jobs/:job_id/cancel`
- `DELETE /api/admin/kb/documents/:document_id`
- `POST /api/kb/retrieve`

检索接口必须稳定返回：`request_id / items / content / score / citation / source`。

文档列表建议补充：`ingest_duration_ms / last_ingest_job_id / chunk_count / file_hash`。

任务列表建议补充：`stage / progress / retry_count / error_code / error_msg / started_at / finished_at`。

#### 5.0.3 验收

1. 页面拆分后原有上传、删除、任务重试、检索测试功能不回退。
2. 检索结果能完整展示 `score/citation/source`。
3. 管理台具备继续扩展监控页面的布局基础。

---

### Phase 1（P1）RAG 监控总览与结构化日志可视化

**对应后端**：`rag-enterprise-fusion-roadmap.md` Phase 1 生产可用期（可观测性部分）。

**目标**：把后端已有的结构化日志变成可筛选、可下钻的监控面板。

#### 5.1.1 前端任务

1. 激活 `/dashboard` 监控总览：
   - 入库成功率趋势
   - 入库任务状态分布
   - 检索请求量趋势
   - 检索 P50/P95 趋势
   - 空结果率趋势
   - 失败类型 TopN
2. 新增 `/trace-logs/retrieval` 检索日志页：
   - 按时间、知识库、query、request_id、route、是否空结果筛选
   - 表格展示 `query / kb_id / topk / final_count / duration_ms / status`
   - 点击进入 trace 详情抽屉
3. 新增 `/trace-logs/ingest` 入库日志页：
   - 按任务状态、错误类型、文件类型、知识库筛选
   - 展示解析、切块、embedding、写入向量库耗时
4. trace 详情抽屉展示：
   - 基础请求信息
   - 阶段耗时（embedding / search / rerank / answer）
   - route 命中数量
   - filter 前后数量
   - final results
   - error detail
5. 支持 `request_id` 复制，方便教学演示和问题定位。

#### 5.1.2 依赖后端 API

后端 P1 需要提供的接口（对应后端路线大纲可观测性部分）：

- `GET /api/admin/kb/metrics/overview?kb_id=&range=`
- `GET /api/admin/kb/metrics/ingest`
- `GET /api/admin/kb/metrics/retrieval`
- `GET /api/admin/kb/logs/retrieval`
- `GET /api/admin/kb/logs/retrieval/{request_id}`
- `GET /api/admin/kb/logs/ingest`
- `GET /api/admin/kb/logs/ingest/{job_id}`

检索日志最小字段：`request_id / query / kb_id / topk / routes / final_count / duration_ms / stage_durations / empty_reason / status / created_at`。

#### 5.1.3 验收

1. 管理台能看到最近 1h/24h/7d 的 RAG 关键指标。
2. 任意一次检索请求可以通过 `request_id` 查到结构化 trace。
3. 失败任务和空结果能定位到明确原因分类。

---

### Phase 2（P2）检索质量与离线评测看板

**对应后端**：`rag-enterprise-fusion-roadmap.md` Phase 2 检索质量优化期（评测闭环部分）。

**目标**：把 Recall@K、MRR、nDCG、Citation Precision 等指标做成可对比看板，让学员看到优化前后的数据变化。

#### 5.2.1 前端任务

1. 新增 `/evaluation/datasets` 评测集页面：
   - `qa_goldens` 列表
   - query 类型分布
   - 导入、导出、校验状态
2. 新增 `/evaluation/runs` 评测运行页面：
   - 创建评测运行（选择 baseline/candidate、策略配置）
   - 查看运行状态（轮询或 SSE）
3. 新增 `/evaluation/reports/[runId]` 评测报告页：
   - Recall@K、HitRate@K、MRR、nDCG@K
   - Citation Precision
   - Empty-After-Filter Rate
   - Score Completeness
   - P50/P95 延迟、token 成本
4. 新增 A/B 对比视图：
   - baseline vs candidate 指标对比表
   - delta 高亮（正收益绿色，负收益红色）
5. 新增失败样本列表：
   - 召回失败样本
   - 引用不支撑样本
   - filter 后为空样本
6. 检索测试页增加"保存为评测样本"按钮。

#### 5.2.2 依赖后端 API

后端 P2 需要提供的接口（对应后端路线大纲离线评测部分）：

- `GET /api/admin/kb/eval/datasets`
- `POST /api/admin/kb/eval/datasets`
- `GET /api/admin/kb/eval/datasets/{id}/items`
- `POST /api/admin/kb/eval/datasets/{id}/items`
- `POST /api/admin/kb/eval/runs`
- `GET /api/admin/kb/eval/runs`
- `GET /api/admin/kb/eval/runs/{run_id}`
- `GET /api/admin/kb/eval/runs/{run_id}/report`
- `GET /api/admin/kb/eval/runs/{run_id}/cases`

评测报告最小字段：`run_id / dataset_id / baseline_config / candidate_config / metrics / deltas / created_at`。

#### 5.2.3 验收

1. 前端能展示一次评测运行的完整指标报告。
2. 能对比 baseline 和 candidate 的 Recall@10、MRR、nDCG、Citation Precision。
3. 能下钻到失败样本，并跳转到对应 request trace。

---

### Phase 3（P3）高级检索调试视图与策略中心

**对应后端**：`rag-enterprise-fusion-roadmap.md` Phase 3 高级检索能力期（L0~L8）。

**详细执行文档**：`admin/docs/phase3-rag-admin-p3-detailed-roadmap.md`。

**目标**：把父子块、动态 TopK、证据拒答、高级 rewrite 等高级策略变成可解释、可灰度、可回滚的前端工作台。

#### 5.3.1 前端任务

1. 升级 `/retrieval-lab` 为检索调试视图：
   - original query / rewritten query / route-specific final query
   - dense/sparse route hits 与 contribution
   - fusion 前后对比
   - rerank 前后排序
   - filter/truncate 原因
   - parent-child 回填前后上下文差异
   - TopK 决策原因（`topk_decision_reason`）
   - evidence gate 判定结果
   - citation consistency 结果
2. 新增 `/strategy-center` 策略中心：
   - feature flag 列表与当前开关状态
   - 当前策略版本
   - 灰度比例
   - 最近指标变化
   - 一键回滚按钮
3. 策略收益视图：
   - Parent Fill Gain
   - Rewrite Gain
   - Route Contribution
   - Evidence Refusal Rate
   - Refusal False Positive Rate

#### 5.3.2 依赖后端 API

后端 P3 需要提供的接口（对应后端路线大纲 L7 调试视图部分）：

检索 trace 详情扩展字段：
- `original_query / rewritten_query / route_final_queries`
- `route_hits / fusion_results / rerank_results / filter_results`
- `parent_child_enabled / parent_fill_strategy / parent_fill_count`
- `topk_policy_version / topk_decision_reason / final_topk`
- `evidence_gate_result / refusal_reason`
- `citation_support_score / unsupported_claims`

策略中心接口：
- `GET /api/admin/kb/strategy/flags`
- `PATCH /api/admin/kb/strategy/flags/{flag_key}`
- `GET /api/admin/kb/strategy/versions`
- `POST /api/admin/kb/strategy/rollback`
- `GET /api/admin/kb/strategy/impact`

后端必须保证以下策略开关可独立关闭（对应后端路线大纲 L0 Feature Flag）：
- `RAG_ENABLE_PARENT_CHILD_RETRIEVAL`
- `RAG_ENABLE_STRATEGIC_TOPK`
- `RAG_ENABLE_EVIDENCE_REFUSAL`
- `RAG_ENABLE_CITATION_CONSISTENCY`
- `RAG_ENABLE_DOMAIN_TERMS`
- `RAG_ENABLE_ROUTE_SPECIFIC_REWRITE`
- `RAG_ENABLE_MODEL_ASSISTED_REWRITE`

#### 5.3.3 验收

1. 一次复杂 query 可以在前端完整还原检索链路。
2. 任一高级策略能看到开启前后指标变化。
3. 策略异常时，管理员能在前端关闭开关并留下操作记录。

---

### Phase 4（P4）企业治理、成本与审计

**对应后端**：`rag-enterprise-fusion-roadmap.md` Phase 4 企业治理与规模化期。

**详细执行文档**：`admin/docs/phase4-rag-admin-p4-detailed-roadmap.md`。

**目标**：让管理台达到企业级可运维标准，形成质量、成本、安全、合规闭环。

#### 5.4.1 前端任务

1. 新增 `/cost-ops/cost` 成本看板：
   - embedding token、LLM token 趋势
   - 平均候选数、平均上下文 token
   - 单知识库成本趋势
2. 新增 `/cost-ops/vector-db` Milvus 运维页：
   - Collection 列表与 active 标记
   - Collection 健康状态、容量、实体数、索引状态
   - 重建、切换、回滚操作记录
3. 新增 `/audit` 审计页：
   - 管理员操作记录
   - 策略开关变更记录
   - 文档删除记录
   - 报告导出记录

#### 5.4.2 依赖后端 API

后端 P4 需要提供的接口（对应后端路线大纲治理部分）：

- `GET /api/admin/kb/cost/summary`
- `GET /api/admin/kb/cost/timeseries`
- `GET /api/admin/kb/vector/collections`
- `GET /api/admin/kb/vector/collections/{name}/health`
- `POST /api/admin/kb/vector/collections/{name}/rebuild`
- `GET /api/admin/kb/audit/events`

#### 5.4.3 验收

1. 管理台能从质量、稳定性、成本、安全四个维度观察 RAG 系统。
2. 策略变更、文档删除、Collection 切换都有审计记录。

---

## 6. 推荐实施节奏

1. P0：先把单页后台拆成可扩展管理台骨架，保持知识库闭环可用。
2. P1：后端 P1 可观测性完成后，立即接入监控总览和 trace 日志，这一步最能体现"企业级可观测"。
3. P2：后端 P2 评测闭环完成后，做评测看板，给学员展示 RAG 优化的数据证据。
4. P3：后端 P3 高级检索完成后，做检索调试视图和策略中心，让高级策略可解释。
5. P4：后端 P4 治理能力完成后，补成本、审计、Milvus 运维，形成完整企业治理闭环。

前后端协作原则：

1. 后端先交 API 示例或 OpenAPI，前端基于 mock 实现页面骨架。
2. 每个页面先接列表和详情，再补图表。
3. 指标口径先写死在文档里，不在页面和后端各解释一套。
4. 所有接口都保留 `request_id`，便于从图表跳到日志。

---

## 7. 阶段验收模板

每个 Phase 完成后按以下模板验收：

1. 已完成页面：
2. 已接入 API：
3. 已展示指标：
4. 可下钻链路：
5. 后端缺口（字段/接口）：
6. 是否影响现有知识库上传/检索：
7. 是否可以进入下一阶段：
