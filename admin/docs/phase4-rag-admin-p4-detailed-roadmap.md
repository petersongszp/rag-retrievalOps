# Phase 4 详细功能实现路线（企业治理、成本与审计，前后端并行推进）

## 1. 文档定位

本文档是 `admin/docs/rag-admin-frontend-roadmap-v2.md` 中 Phase 4（P4）"企业治理、成本与审计"的详细执行手册，同时覆盖前后端，可直接按 L 编号逐步实现。

三个用途：

1. 作为前后端 P4 联合推进的统一执行文档，以 P3 高级检索调试视图、策略中心和最小操作日志为起点。
2. 作为冻结成本口径、Milvus/Collection 运维口径、审计事件、告警规则、周报字段和治理门禁的协作基线。
3. 作为 RAG 管理台进入长期企业运营后的运行手册雏形，支撑后续周期复盘、预算治理、合规排查和容量治理。

**统一口径说明：**

1. `企业治理` 固定指：围绕质量、稳定性、成本、安全、合规、容量和回滚形成可观测、可操作、可审计的闭环，不等同于单纯增加几个大屏。
2. `成本治理` 固定指：按请求、知识库、策略版本、实验组和模型维度归因 embedding、retrieval、rerank、LLM、向量存储与索引重建成本。
3. `Milvus/Collection 运维` 固定指：Collection 列表、active 标记、健康检查、容量巡检、索引状态、重建计划、切换计划、回滚记录和操作审计。
4. `完整审计中心` 固定指：统一查询上传、检索、生成、删除、策略变更、报告导出、Collection 切换、权限操作和 trace 访问等事件。
5. `自动化周报` 固定指：按固定时间窗口输出质量、稳定性、成本、策略、告警、索引健康和风险清单，不是一次性的导出截图。
6. `治理门禁` 固定指：策略发布、实验转正、Collection 切换、索引重建、成本优化和审计策略变更前必须通过的质量、延迟、成本、错误率、审计覆盖率检查。
7. `可降级` 固定指：P4 的成本统计、审计查询、告警、周报、运维页面异常时不能影响 P0-P3 主链路；最多降级为记录告警、页面提示和补偿任务。
8. `契约缺口` 固定指：关键字段或接口缺失时页面明确标识，不静默隐藏、不用假数据补齐、不把未知值展示为 0。

---

## 2. 当前现状（基于文档和代码扫描）

### 2.1 P3 交给 P4 的稳定底座

基于 `admin/docs/phase3-rag-admin-p3-detailed-roadmap.md`：

1. 高级检索链路已经具备可解释基础：rewrite、route hits、fusion、rerank、filter、parent-child、TopK、evidence、citation 可在调试视图中展示。
2. 策略中心已经定义 feature flags、策略版本、灰度比例、回滚和策略影响分析。
3. 策略影响分析已经能展示质量、延迟、token 变化和拒答风险，可作为 P4 成本治理的前置指标。
4. 策略操作已有最小日志，P4 需要并入统一 `/audit`。
5. 回滚操作已有 `rollback_id` 和 changed flags，P4 需要接入审计、告警和周报。
6. P4 需要补齐的能力包括：
   - `GET /api/admin/kb/audit/events`
   - `GET /api/admin/kb/cost/summary`
   - `GET /api/admin/kb/cost/timeseries`
   - `GET /api/admin/kb/vector/collections`
   - 告警规则、自动化周报或报告导出

### 2.2 后端已有或应复用能力

基于后端 Phase 4 路线文档和 P1-P3 文档：

1. P1 已有结构化检索日志、入库日志和基础 trace 下钻能力。
2. P2 已有离线评测、baseline/candidate 报告和质量门禁概念。
3. P3 已有策略影响分析、策略版本、灰度、回滚和高级 trace 字段。
4. 后端 Phase 4 路线已明确覆盖：
   - AB 实验平台和灰度治理
   - 索引生命周期和 Collection 治理
   - 成本采集、归因与成本看板
   - 合规审计与数据保留策略
   - 自动化周报
   - Milvus/向量库运维工具化
   - 规模化监控告警、容量巡检和治理门禁
5. Milvus 管理层已有健康检查和 Collection 操作基础，P4 需要把它工具化、权限化、审计化。

### 2.3 前端已有或应复用能力

基于 `admin/src/` 和 P0-P3 路线文档：

1. `AdminShell` 已有管理台骨架、导航、面包屑和知识库选择器。
2. `/dashboard` 已有 P0/P1 监控总览底座，可承接 P4 治理门禁和告警摘要。
3. `/trace-logs/retrieval` 和 `/trace-logs/ingest` 已有日志查询与下钻入口，可承接审计跳转。
4. `/evaluation` 已有评测集、评测运行和报告页，可为实验和治理门禁提供基线。
5. `/strategy-center` 已有 P3 策略管理入口，可扩展成本、告警、审计联动。
6. `admin/src/config/api.ts` 和 `admin/src/types/kb.ts` 已有 P0-P3 类型与 API 常量，P4 需要补成本、向量库、审计、告警、周报和门禁类型。

### 2.4 当前真实缺口

**后端缺口：**

1. 缺少请求级、策略级、知识库级、模型级成本采集和归因接口。
2. 缺少统一审计事件模型，P1/P3 的日志还未并入完整 Audit。
3. 缺少审计脱敏、保留期、导出、补偿队列和覆盖率统计。
4. 缺少 Milvus/Collection 列表、健康、容量、索引、重建、切换和回滚 API。
5. 缺少告警规则、告警事件、确认、解决和治理门禁 API。
6. 缺少自动化周报生成、查询、导出和异常项下钻接口。
7. 缺少 P4 治理能力的独立开关与降级策略。

**前端缺口：**

1. `/cost-ops/cost` 成本看板不存在。
2. `/cost-ops/vector-db` Milvus/Collection 运维页不存在。
3. `/audit` 完整审计页不存在。
4. `/alerts` 或 Dashboard 告警中心入口不存在。
5. `/reports/weekly` 自动化周报页面不存在。
6. P4 API 常量和类型定义不存在。
7. 管理台还没有成本异常、审计缺口、容量风险和 Collection 切换的可视化门禁。

---

## 3. 范围边界与通过标准（Gate）

### 3.1 P4 必须完成

1. 成本看板可按时间范围、知识库、策略版本、实验组和模型维度展示成本。
2. 成本看板至少展示每千次问答成本、embedding token、LLM token、rerank 成本、平均上下文 token、平均候选数、高成本 query TopN。
3. Milvus/Collection 运维页可展示 Collection 列表、active 标记、健康状态、容量、实体数、索引状态、版本和最近操作。
4. Collection 重建、切换、回滚必须有二次确认、变更原因、执行结果和审计记录。
5. 审计中心可按时间、事件类型、资源类型、操作者、`kb_id`、`request_id`、`document_id`、`trace_id` 查询事件。
6. 审计详情必须展示 before/after 摘要、操作原因、请求来源、关联资源和脱敏后的上下文。
7. 告警中心可展示质量、稳定性、成本、容量、审计五类告警，并支持确认、解决、下钻。
8. 自动化周报可展示质量趋势、稳定性趋势、成本趋势、策略变更、Collection 健康、告警复盘和下周动作。
9. Dashboard 可展示 P4 治理总览：成本、审计覆盖率、活动告警、Collection 健康、周报状态。
10. 所有 P4 页面在字段缺失或接口失败时展示契约缺口或局部降级，不影响 P0-P3 页面。

### 3.2 P4 明确不做

1. 不在前端计算真实成本账单；前端只展示后端返回的成本归因结果。
2. 不把成本优化做成单纯降配；任何成本下降都必须同时观察质量、延迟、错误率和拒答误伤。
3. 不在管理台直接暴露未脱敏 query、文档片段、用户隐私字段和敏感 before/after 原文。
4. 不让前端直接执行底层 Milvus 命令；所有运维操作必须通过后端受控 API。
5. 不做绕过 P3 回滚体系的策略变更；策略相关操作仍通过 Strategy Center 和后端策略 API。
6. 不把告警做成静态列表；告警必须能关联指标、资源、trace、策略版本或 Collection 版本。
7. 不要求 P4 首版接入外部 Grafana、Loki、ELK 或云账单系统；可以先以数据库聚合和后端采集为准。

### 3.3 Phase 4 通过标准（全满足）

1. 管理台能从质量、稳定性、成本、安全、容量五个维度观察 RAG 系统。
2. 成本看板能展示每千次问答成本，并能定位到 `kb_id / strategy_version / experiment_id / query_type / model`。
3. 审计中心覆盖上传、检索、生成、删除、策略变更、报告导出、Collection 切换、权限操作和 trace 访问。
4. 审计查询具备脱敏、权限控制、保留期展示和导出记录。
5. Milvus/Collection 运维页能完成健康巡检、容量巡检、重建计划、切换计划、回滚演练记录展示。
6. 告警中心能展示质量、稳定性、成本、容量、审计五类告警，并可下钻到 trace、评测报告、策略版本或 Collection 版本。
7. 自动化周报能稳定生成，且异常项可以继续下钻。
8. P4 任一治理模块异常时不拖垮主查询链路，页面有明确降级提示。
9. P0/P1/P2/P3 回归通过，尤其是知识库管理、Trace Logs、Evaluation、Retrieval Debug、Strategy Center 不回退。

---

## 4. 实现路线总览（L0 -> L8）

Phase 4 按 9 条路线推进，按门禁顺序合流：

1. L0：P4 治理边界、指标口径、审计事件与降级策略冻结
2. L1：后端 - 成本采集、归因与成本 API
3. L2：后端 - Milvus/Collection 运维与索引生命周期 API
4. L3：后端 - 统一审计事件、脱敏、保留期与补偿机制
5. L4：后端 - 告警、治理门禁与自动化周报 API
6. L5：前端 - P4 类型契约、API 路径、导航与 Dashboard 治理总览
7. L6：前端 - 成本看板与成本异常下钻
8. L7：前端 - Milvus/Collection 运维页、审计中心、告警中心、周报页
9. L8：联调验收、降级演练、审计抽样与长期运营交接

建议顺序：`L0 -> L1 + L2 + L3 + L4（后端并行） -> L5 -> L6 + L7（前端并行） -> L8`

---

## 5. 详细路线拆解

### 5.1 L0 P4 治理边界、指标口径、审计事件与降级策略冻结

#### 目标

在开发前冻结 P4 的指标口径、事件模型、API 路径、操作边界、权限边界和降级策略，避免成本、审计、运维、告警各自解释一套。

#### 功能任务

1. 统一 P4 管理台 API 前缀：
   - 成本：`/api/admin/kb/cost/*`
   - 向量库运维：`/api/admin/kb/vector/*`
   - 审计：`/api/admin/kb/audit/*`
   - 告警：`/api/admin/kb/alerts/*`
   - 周报：`/api/admin/kb/reports/*`
   - 治理门禁：`/api/admin/kb/governance/*`
2. 冻结成本基础字段：
   - `request_id`
   - `kb_id`
   - `query_type`
   - `strategy_version`
   - `experiment_id`
   - `model_name`
   - `embedding_tokens`
   - `llm_input_tokens`
   - `llm_output_tokens`
   - `rerank_tokens`
   - `context_tokens`
   - `candidate_count`
   - `final_count`
   - `estimated_cost`
   - `currency`
   - `created_at`
3. 冻结成本聚合指标：
   - `cost_per_1k_queries`
   - `total_estimated_cost`
   - `embedding_cost`
   - `llm_cost`
   - `rerank_cost`
   - `vector_storage_cost`
   - `index_rebuild_cost`
   - `avg_context_tokens`
   - `avg_candidate_count`
   - `high_cost_query_count`
4. 冻结 Collection 字段：
   - `collection_name`
   - `kb_id`
   - `active`
   - `status`
   - `health_status`
   - `entity_count`
   - `capacity_bytes`
   - `index_status`
   - `index_version`
   - `schema_version`
   - `last_rebuild_at`
   - `last_switch_at`
   - `rollback_collection`
5. 冻结审计事件分类：
   - `kb_create`
   - `document_upload`
   - `document_delete`
   - `ingest_retry`
   - `ingest_cancel`
   - `retrieve_query`
   - `trace_view`
   - `eval_run_create`
   - `report_export`
   - `strategy_flag_update`
   - `strategy_rollback`
   - `collection_rebuild`
   - `collection_switch`
   - `collection_rollback`
   - `alert_ack`
   - `alert_resolve`
   - `permission_change`
6. 冻结审计字段：
   - `event_id`
   - `audit_trace_id`
   - `actor_id`
   - `actor_name`
   - `action`
   - `resource_type`
   - `resource_id`
   - `kb_id`
   - `request_id`
   - `document_id`
   - `before`
   - `after`
   - `reason`
   - `ip`
   - `user_agent`
   - `sensitive_fields_masked`
   - `created_at`
7. 冻结告警分类：
   - `quality`
   - `stability`
   - `cost`
   - `capacity`
   - `audit`
8. 冻结 P4 治理开关：
   - `RAG_ENABLE_COST_GOVERNANCE`
   - `RAG_ENABLE_AUDIT_CENTER`
   - `RAG_ENABLE_VECTOR_OPS`
   - `RAG_ENABLE_GOVERNANCE_ALERTS`
   - `RAG_ENABLE_WEEKLY_REPORT`
9. 冻结降级策略：
   - 成本采集失败：记录告警，不影响检索回答
   - 审计写入失败：进入补偿队列，主链路继续
   - 审计查询失败：只影响 `/audit` 页面
   - 告警计算失败：Dashboard 显示告警模块异常
   - 周报生成失败：保留最近成功版本，并展示失败原因
   - Collection 操作失败：不修改 active 状态，记录失败审计
10. 冻结 P4 不可回退功能清单：
    - P3 策略中心可用
    - P3 调试视图可用
    - P2 评测报告可用
    - P1 Trace Logs 可用
    - P0 知识库管理闭环可用

#### 验收

1. 前后端对 P4 必做与不做边界达成一致。
2. 成本字段、审计字段、Collection 字段、告警分类和治理开关以本文档为准。
3. 所有 P4 模块都有明确降级策略，不影响主检索链路。

---

### 5.2 L1 后端 - 成本采集、归因与成本 API

#### 目标

把成本从模型账单后的粗粒度统计升级为请求级归因、策略级对比、预算级告警的数据基础。

#### 功能任务

1. 在检索、rerank、生成和评测链路中采集成本字段：
   - `embedding_tokens`
   - `llm_input_tokens`
   - `llm_output_tokens`
   - `rerank_tokens`
   - `context_tokens`
   - `candidate_count`
   - `final_count`
   - `model_name`
   - `strategy_version`
   - `experiment_id`
2. 建立成本记录表或聚合存储：
   - `request_id`
   - `kb_id`
   - `query_type`
   - `cost_breakdown`
   - `estimated_cost`
   - `currency`
   - `created_at`
3. 成本估算规则必须版本化：
   - `pricing_version`
   - `embedding_unit_price`
   - `llm_input_unit_price`
   - `llm_output_unit_price`
   - `rerank_unit_price`
   - `storage_unit_price`
4. 新增成本汇总接口：
   ```http
   GET /api/admin/kb/cost/summary?kb_id=&range=24h&strategy_version=&experiment_id=
   ```
   最小返回：
   ```json
   {
     "range": "24h",
     "total_estimated_cost": 12.34,
     "currency": "USD",
     "cost_per_1k_queries": 0.83,
     "embedding_cost": 1.2,
     "llm_cost": 8.9,
     "rerank_cost": 0.7,
     "vector_storage_cost": 1.1,
     "index_rebuild_cost": 0.44,
     "avg_context_tokens": 1800,
     "avg_candidate_count": 42
   }
   ```
5. 新增成本时序接口：
   ```http
   GET /api/admin/kb/cost/timeseries?range=7d&bucket=1h&kb_id=
   ```
6. 新增成本维度接口：
   ```http
   GET /api/admin/kb/cost/by-kb?range=7d
   GET /api/admin/kb/cost/by-strategy?range=7d
   GET /api/admin/kb/cost/by-model?range=7d
   ```
7. 新增高成本 query 接口：
   ```http
   GET /api/admin/kb/cost/high-cost-queries?range=24h&page=1&page_size=20
   ```
8. 成本归因必须关联下钻：
   - `request_id` 可跳 trace
   - `strategy_version` 可跳策略中心
   - `experiment_id` 可跳评测或实验报告
   - `kb_id` 可跳知识库详情
9. 成本统计缺失时返回 `contract_gaps`，不返回假 0。

#### 验收

1. 成本 API 可按 `range / kb_id / strategy_version / experiment_id / model_name` 过滤。
2. 成本看板能展示总成本、成本拆分、每千次问答成本和高成本 query。
3. 成本下降必须能同时展示质量、延迟和错误率参考指标。
4. 成本采集异常不影响检索回答，并能产生告警或契约缺口。

---

### 5.3 L2 后端 - Milvus/Collection 运维与索引生命周期 API

#### 目标

把 Milvus/Collection 运维从临时命令升级为可查询、可计划、可审计、可回滚的管理台能力。

#### 功能任务

1. 新增 Collection 列表接口：
   ```http
   GET /api/admin/kb/vector/collections?kb_id=&status=
   ```
2. Collection 列表最小字段：
   - `collection_name`
   - `kb_id`
   - `active`
   - `status`
   - `health_status`
   - `entity_count`
   - `capacity_bytes`
   - `index_status`
   - `index_version`
   - `schema_version`
   - `last_rebuild_at`
   - `last_switch_at`
   - `rollback_collection`
3. 新增 Collection 健康详情接口：
   ```http
   GET /api/admin/kb/vector/collections/{name}/health
   ```
   返回：
   - `load_state`
   - `index_build_progress`
   - `query_latency_p95_ms`
   - `insert_latency_p95_ms`
   - `segment_count`
   - `sealed_segment_count`
   - `growing_segment_count`
   - `last_error`
4. 新增容量接口：
   ```http
   GET /api/admin/kb/vector/collections/{name}/capacity
   ```
5. 新增重建计划接口：
   ```http
   POST /api/admin/kb/vector/collections/{name}/rebuild
   ```
   请求体：
   ```json
   {
     "target_index_version": "idx_v2",
     "reason": "parent-child metadata changed",
     "dry_run": false
   }
   ```
6. 新增切换接口：
   ```http
   POST /api/admin/kb/vector/collections/{name}/switch
   ```
7. 新增回滚接口：
   ```http
   POST /api/admin/kb/vector/collections/{name}/rollback
   ```
8. 所有高风险操作必须校验：
   - `reason` 必填
   - 目标 Collection 健康检查通过
   - candidate 与 active 的 schema 兼容
   - 切换前有可用 rollback collection
   - 操作写入审计事件
9. 新增 Collection 操作记录接口：
   ```http
   GET /api/admin/kb/vector/operations?collection_name=&page=1&page_size=20
   ```

#### 验收

1. 管理台能展示 Collection 列表、active 标记、健康状态、容量和索引状态。
2. 重建、切换、回滚操作都有受控 API、二次确认所需字段和审计记录。
3. Collection 切换失败不产生半切换状态。
4. Milvus 运维 API 异常不影响已有检索服务。

---

### 5.4 L3 后端 - 统一审计事件、脱敏、保留期与补偿机制

#### 目标

建立覆盖 RAG 管理台关键行为的统一审计中心，让操作、查询、导出、策略、索引和权限变更可追溯、可脱敏、可导出、可留存。

#### 功能任务

1. 建立统一审计事件模型：
   - `event_id`
   - `audit_trace_id`
   - `actor_id`
   - `actor_name`
   - `action`
   - `resource_type`
   - `resource_id`
   - `kb_id`
   - `request_id`
   - `document_id`
   - `before`
   - `after`
   - `reason`
   - `ip`
   - `user_agent`
   - `sensitive_fields_masked`
   - `created_at`
2. 写入审计事件：
   - 知识库创建
   - 文档上传、删除
   - 入库重试、取消
   - 检索请求
   - trace 查看
   - 评测运行创建
   - 报告导出
   - 策略开关变更
   - 策略回滚
   - Collection 重建、切换、回滚
   - 告警确认、解决
   - 权限变更
3. 新增审计列表接口：
   ```http
   GET /api/admin/kb/audit/events?action=&resource_type=&actor_id=&kb_id=&request_id=&start_time=&end_time=&page=1&page_size=20
   ```
4. 新增审计详情接口：
   ```http
   GET /api/admin/kb/audit/events/{event_id}
   ```
5. 新增审计导出接口：
   ```http
   POST /api/admin/kb/audit/events/export
   ```
6. 审计脱敏规则：
   - query 原文按权限展示，默认展示脱敏摘要
   - 文档片段默认只展示 snippet 摘要和 `document_id/chunk_id`
   - before/after 中敏感字段替换为 `***`
   - IP 和 user agent 可按权限展示
7. 数据保留策略：
   - 展示 `retention_days`
   - 展示过期清理任务状态
   - 清理动作自身写审计
8. 审计补偿机制：
   - 审计写入失败进入补偿队列
   - 补偿失败触发告警
   - 审计缺口按 `audit_coverage_rate` 统计

#### 验收

1. 上传、检索、生成、删除、策略变更、Collection 切换均有审计事件。
2. 审计查询可按 `audit_trace_id / request_id / kb_id / document_id / operator_id` 定位。
3. 审计展示具备脱敏与权限控制，不泄露非必要原文。
4. 审计写入失败不会影响主链路，但必须可补偿、可告警、可统计缺口。

---

### 5.5 L4 后端 - 告警、治理门禁与自动化周报 API

#### 目标

把质量、稳定性、成本、容量和审计风险变成可追踪、可确认、可解决、可复盘的企业治理流程。

#### 功能任务

1. 新增告警列表接口：
   ```http
   GET /api/admin/kb/alerts?status=&severity=&category=&kb_id=&page=1&page_size=20
   ```
2. 告警字段：
   - `alert_id`
   - `category`
   - `severity`
   - `status`
   - `title`
   - `message`
   - `resource_type`
   - `resource_id`
   - `kb_id`
   - `metric_name`
   - `threshold`
   - `current_value`
   - `related_request_id`
   - `related_report_id`
   - `related_strategy_version`
   - `related_collection`
   - `created_at`
   - `acknowledged_at`
   - `resolved_at`
3. 新增告警操作接口：
   ```http
   PATCH /api/admin/kb/alerts/{alert_id}/ack
   PATCH /api/admin/kb/alerts/{alert_id}/resolve
   ```
4. 新增告警规则接口：
   ```http
   GET /api/admin/kb/alert-rules
   POST /api/admin/kb/alert-rules
   PATCH /api/admin/kb/alert-rules/{rule_id}
   ```
5. 首批告警规则：
   - `recall_at_10` 低于基线
   - `retrieval_p95_ms` 持续超阈值
   - `empty_after_filter_rate` 异常升高
   - `citation_support_score` 下降
   - `cost_per_1k_queries` 超预算
   - 高成本 query 激增
   - Collection 容量超阈值
   - Collection 健康检查失败
   - 审计补偿队列堆积
   - `audit_coverage_rate` 低于阈值
6. 新增治理门禁接口：
   ```http
   GET /api/admin/kb/governance/gates?target_type=&target_id=
   POST /api/admin/kb/governance/gates/check
   ```
7. 门禁结果字段：
   - `gate_status`
   - `passed`
   - `failed_rules`
   - `quality_summary`
   - `latency_summary`
   - `cost_summary`
   - `audit_summary`
   - `rollback_ready`
8. 新增周报接口：
   ```http
   POST /api/admin/kb/reports/weekly
   GET /api/admin/kb/reports/weekly
   GET /api/admin/kb/reports/weekly/{report_id}
   POST /api/admin/kb/reports/weekly/{report_id}/export
   ```
9. 周报内容：
   - 质量趋势
   - 稳定性趋势
   - 成本趋势
   - 策略变更
   - Collection 健康
   - 告警复盘
   - 审计覆盖率
   - Top 问题样本
   - 下周动作

#### 验收

1. 质量、稳定性、成本、容量、审计五类告警均有明确阈值与处理建议。
2. 告警可确认、解决，并写入审计。
3. 治理门禁可用于策略发布、Collection 切换和成本优化前检查。
4. 周报能生成、查看、导出，并能下钻到异常来源。

---

### 5.6 L5 前端 - P4 类型契约、API 路径、导航与 Dashboard 治理总览

#### 目标

补齐前端消费 P4 API 所需的类型、路径、路由、导航和治理总览，先建立稳定契约再实现页面。

#### 功能任务

1. 在 `admin/src/types/kb.ts` 新增成本类型：
   - `CostSummary`
   - `CostTimeseriesPoint`
   - `CostBreakdown`
   - `CostByKbItem`
   - `CostByStrategyItem`
   - `HighCostQuery`
2. 新增向量库运维类型：
   - `VectorCollection`
   - `VectorCollectionHealth`
   - `VectorCollectionCapacity`
   - `VectorOperation`
   - `VectorOperationRequest`
   - `VectorOperationResult`
3. 新增审计类型：
   - `AuditEvent`
   - `AuditEventDetail`
   - `AuditAction`
   - `AuditExportRequest`
   - `AuditExportResult`
4. 新增告警和周报类型：
   - `GovernanceAlert`
   - `AlertRule`
   - `GovernanceGateSummary`
   - `WeeklyReport`
   - `WeeklyReportDetail`
5. 在 `admin/src/config/api.ts` 增加 P4 API 常量：
   ```ts
   COST_SUMMARY: `${API_BASE_URL}/admin/kb/cost/summary`,
   COST_TIMESERIES: `${API_BASE_URL}/admin/kb/cost/timeseries`,
   COST_BY_KB: `${API_BASE_URL}/admin/kb/cost/by-kb`,
   COST_BY_STRATEGY: `${API_BASE_URL}/admin/kb/cost/by-strategy`,
   HIGH_COST_QUERIES: `${API_BASE_URL}/admin/kb/cost/high-cost-queries`,
   VECTOR_COLLECTIONS: `${API_BASE_URL}/admin/kb/vector/collections`,
   VECTOR_COLLECTION_HEALTH: (name: string) =>
     `${API_BASE_URL}/admin/kb/vector/collections/${name}/health`,
   VECTOR_COLLECTION_REBUILD: (name: string) =>
     `${API_BASE_URL}/admin/kb/vector/collections/${name}/rebuild`,
   VECTOR_COLLECTION_SWITCH: (name: string) =>
     `${API_BASE_URL}/admin/kb/vector/collections/${name}/switch`,
   VECTOR_COLLECTION_ROLLBACK: (name: string) =>
     `${API_BASE_URL}/admin/kb/vector/collections/${name}/rollback`,
   AUDIT_EVENTS: `${API_BASE_URL}/admin/kb/audit/events`,
   AUDIT_EVENT_DETAIL: (eventId: string) =>
     `${API_BASE_URL}/admin/kb/audit/events/${eventId}`,
   ALERTS: `${API_BASE_URL}/admin/kb/alerts`,
   ALERT_ACK: (alertId: string) =>
     `${API_BASE_URL}/admin/kb/alerts/${alertId}/ack`,
   ALERT_RESOLVE: (alertId: string) =>
     `${API_BASE_URL}/admin/kb/alerts/${alertId}/resolve`,
   WEEKLY_REPORTS: `${API_BASE_URL}/admin/kb/reports/weekly`,
   WEEKLY_REPORT_DETAIL: (reportId: string) =>
     `${API_BASE_URL}/admin/kb/reports/weekly/${reportId}`,
   GOVERNANCE_GATES: `${API_BASE_URL}/admin/kb/governance/gates`,
   ```
6. 新增路由：
   - `admin/src/app/(admin)/cost-ops/cost/page.tsx`
   - `admin/src/app/(admin)/cost-ops/vector-db/page.tsx`
   - `admin/src/app/(admin)/audit/page.tsx`
   - `admin/src/app/(admin)/alerts/page.tsx`
   - `admin/src/app/(admin)/reports/weekly/page.tsx`
7. 修改 `admin-shell.tsx`：
   - 激活 `Cost & Ops`
   - 激活 `Audit`
   - 增加 `Alerts` 或在 Dashboard 增加告警入口
   - 增加周报入口
8. 升级 `/dashboard` 治理总览：
   - 本周成本
   - 每千次问答成本
   - 活动告警数
   - 审计覆盖率
   - Collection 健康状态
   - 最近周报状态

#### 验收

1. TypeScript 编译无类型错误。
2. P4 导航可进入页面。
3. Dashboard 可展示 P4 治理摘要或契约缺口。
4. API 路径和类型字段与 L0-L4 冻结口径一致。

---

### 5.7 L6 前端 - 成本看板与成本异常下钻

#### 目标

让管理员能看清 RAG 成本来自哪里、为什么变化、是否值得、异常时该下钻到哪条请求或哪个策略。

#### 功能任务

1. 新建 `/cost-ops/cost` 页面：
   - 时间范围：`1h / 24h / 7d / 30d`
   - 过滤：`kb_id / strategy_version / experiment_id / model_name`
   - 顶部卡片：总成本、每千次问答成本、LLM 成本、embedding 成本、rerank 成本、向量存储成本
2. 成本趋势图：
   - `total_estimated_cost`
   - `cost_per_1k_queries`
   - `llm_cost`
   - `embedding_cost`
   - `rerank_cost`
3. 成本拆分表：
   - 按知识库
   - 按策略版本
   - 按模型
   - 按 query 类型
4. 成本与质量联动：
   - 同屏展示 `recall_at_10`
   - `citation_support_score`
   - `retrieval_p95_ms`
   - `empty_after_filter_rate`
   - `error_rate`
5. 高成本 query TopN：
   - `request_id`
   - `query`
   - `kb_id`
   - `strategy_version`
   - `model_name`
   - `estimated_cost`
   - `context_tokens`
   - `candidate_count`
   - 操作：跳转 Trace、跳转调试视图
6. 成本异常区：
   - 展示成本告警
   - 展示触发阈值和当前值
   - 可跳转 `/alerts?category=cost`
7. 降级策略：
   - summary 失败时整页 `Alert`，保留筛选器
   - timeseries 失败时只降级图表
   - 高成本 query 失败时只降级列表
   - 指标缺失时展示契约缺口，不展示 0

#### 验收

1. `/cost-ops/cost` 能展示成本汇总、趋势、拆分和高成本 query。
2. 任一高成本 query 可跳转到 trace 或调试视图。
3. 成本变化能同时看到质量和延迟参考指标。
4. 接口失败时页面局部降级，不白屏。

---

### 5.8 L7 前端 - Milvus/Collection 运维页、审计中心、告警中心、周报页

#### 目标

把 P4 的运维、审计、告警和周报能力做成可执行的管理台工作区，形成企业治理闭环。

#### 功能任务

**Milvus/Collection 运维页（`/cost-ops/vector-db`）：**

1. Collection 列表：
   - `collection_name`
   - `kb_id`
   - `active`
   - `status`
   - `health_status`
   - `entity_count`
   - `capacity_bytes`
   - `index_status`
   - `index_version`
   - `last_rebuild_at`
   - `last_switch_at`
2. 健康详情抽屉：
   - load state
   - query latency
   - insert latency
   - segment count
   - index build progress
   - last error
3. 高风险操作：
   - 重建
   - 切换
   - 回滚
   - 必填 `reason`
   - 二次确认
   - 操作后刷新列表和操作记录
4. 操作记录：
   - operator
   - operation
   - from/to collection
   - reason
   - status
   - created_at

**审计中心（`/audit`）：**

5. 审计筛选：
   - 时间范围
   - action
   - resource_type
   - actor
   - `kb_id`
   - `request_id`
   - `document_id`
6. 审计表格：
   - `event_id`
   - `action`
   - `resource_type`
   - `resource_id`
   - `actor_name`
   - `kb_id`
   - `request_id`
   - `created_at`
7. 审计详情抽屉：
   - before/after 摘要
   - reason
   - IP/user agent（按权限展示）
   - 脱敏标记
   - 关联跳转：trace、知识库、文档、策略、Collection
8. 审计导出：
   - 二次确认
   - 必填 reason
   - 导出操作本身写审计

**告警中心（`/alerts`）：**

9. 告警列表：
   - status
   - severity
   - category
   - title
   - resource
   - threshold
   - current_value
   - created_at
10. 告警操作：
   - ack
   - resolve
   - 填写处理备注
   - 写入审计
11. 告警下钻：
   - trace
   - 评测报告
   - 策略中心
   - Collection 健康详情
   - 成本看板

**自动化周报（`/reports/weekly`）：**

12. 周报列表：
   - report_id
   - range
   - status
   - generated_at
   - total_alerts
   - cost_delta
   - quality_delta
13. 周报详情：
   - 质量趋势
   - 稳定性趋势
   - 成本趋势
   - 策略变更
   - Collection 健康
   - 告警复盘
   - 审计覆盖率
   - Top 问题样本
   - 下周动作
14. 周报导出：
   - 导出 PDF/JSON 由后端决定
   - 前端只触发导出并展示结果

#### 验收

1. `/cost-ops/vector-db` 可展示 Collection 状态并执行受控操作。
2. `/audit` 可查询、下钻、导出审计事件。
3. `/alerts` 可确认、解决告警，并能下钻到相关资源。
4. `/reports/weekly` 可查看周报详情和异常项。
5. 所有高风险操作都有二次确认、reason 和审计记录。

---

### 5.9 L8 联调验收、降级演练、审计抽样与长期运营交接

#### 目标

证明 P4 企业治理能力安全上线，并能进入长期运营状态：可控成本、可查审计、可管容量、可处理告警、可周期复盘。

#### 冒烟测试清单

1. 访问 `/dashboard` 成功，展示 P4 治理摘要或契约缺口。
2. 访问 `/cost-ops/cost` 成功，展示成本汇总。
3. 切换时间范围后，成本趋势刷新。
4. 按 `kb_id` 过滤成本，数据正确变化。
5. 点击高成本 query，跳转到 trace 或调试视图。
6. 访问 `/cost-ops/vector-db` 成功，展示 Collection 列表。
7. 打开 Collection 健康详情，展示实体数、容量、索引状态和延迟。
8. 发起 Collection rebuild dry run，要求填写 reason。
9. 发起 Collection switch 非法请求时后端拒绝，前端展示错误。
10. 访问 `/audit` 成功，按 action 筛选事件。
11. 打开审计详情，before/after 已脱敏。
12. 审计事件可跳转到关联 trace、策略或 Collection。
13. 访问 `/alerts` 成功，展示活动告警。
14. ack 一个告警成功，并写入审计事件。
15. resolve 一个告警成功，并展示处理备注。
16. 访问 `/reports/weekly` 成功，查看最近周报。
17. 周报异常项可下钻到成本、告警、策略或 Collection。

#### 回归测试清单

1. P0 知识库管理、文档上传、任务重试/取消不回退。
2. P1 Dashboard 趋势图和 Trace Logs 不回退。
3. P2 Evaluation 数据集、运行、报告页面不回退。
4. P3 Retrieval Debug 和 Strategy Center 不回退。
5. P4 API 失败时，只影响对应模块，不影响其他页面。
6. 高风险操作取消后不产生后端状态变更。
7. 审计脱敏规则生效，页面不展示非必要原文。
8. 告警 ack/resolve 操作失败时不乐观更新本地状态。

#### 降级与回滚预案

1. **成本采集异常**：关闭 `RAG_ENABLE_COST_GOVERNANCE` 或暂停聚合任务，成本看板展示最近成功数据和告警。
2. **审计写入异常**：审计事件进入补偿队列，主链路继续，Dashboard 展示审计覆盖率下降。
3. **审计查询异常**：`/audit` 局部不可用，不影响日志、评测、策略中心。
4. **Collection 运维 API 异常**：禁用重建/切换/回滚按钮，保留只读健康状态。
5. **Collection 切换异常**：不修改 active 标记，提示失败原因，保留 rollback collection。
6. **告警计算异常**：告警中心展示计算失败，已有告警列表继续可查。
7. **周报生成异常**：展示最近成功周报，当前周报标记 failed，并记录失败原因。

#### 审计抽样清单

1. 抽样一条文档上传审计，确认 actor、resource、kb_id、document_id、created_at 完整。
2. 抽样一条文档删除审计，确认 before/after 脱敏且 reason 可见。
3. 抽样一条策略变更审计，确认 from/to 状态、rollout 变化和 reason 完整。
4. 抽样一条 Collection 切换审计，确认 from/to collection、健康门禁和 rollback collection 可见。
5. 抽样一条 trace 查看审计，确认 request_id、actor、ip 脱敏规则符合权限。
6. 抽样一条报告导出审计，确认导出 reason 和导出结果记录完整。

#### 长期运营交接清单

1. 成本异常处理 SOP 已明确。
2. 审计查询和导出 SOP 已明确。
3. Collection 重建、切换、回滚 SOP 已明确。
4. 告警确认、解决、复盘 SOP 已明确。
5. 周报生成、复核、归档 SOP 已明确。
6. 治理门禁失败后的处理路径已明确。

#### 验收

1. 冒烟测试清单全通过。
2. 回归测试清单无阻塞问题。
3. 降级与回滚预案演练通过。
4. 审计抽样无字段缺失、无脱敏违规。
5. 长期运营交接清单已确认。

---

## 6. 推荐协作节奏

1. 先完成 `L0`，冻结 P4 字段口径、审计事件、治理开关和降级策略。
2. `L1 + L2 + L3 + L4` 后端并行推进：
   - `L1` 管成本采集和成本 API
   - `L2` 管 Milvus/Collection 运维 API
   - `L3` 管统一审计和脱敏保留
   - `L4` 管告警、门禁和周报
3. 后端提供最小 OpenAPI 或 JSON 示例后，前端进入 `L5`。
4. `L6 + L7` 前端并行推进：
   - `L6` 依赖 `L1`
   - `L7` 依赖 `L2 + L3 + L4`
5. `L8` 统一收口，重点验证主链路不回退、高风险操作可审计、治理模块可降级。

---

## 7. 角色分工建议

1. 后端A：负责 `L1`，成本采集、成本归因、成本聚合和高成本 query API。
2. 后端B：负责 `L2`，Collection 列表、健康、容量、重建、切换、回滚和操作记录。
3. 后端C：负责 `L3`，统一审计事件、脱敏、保留期、导出和补偿队列。
4. 后端D：负责 `L4`，告警规则、告警事件、治理门禁和自动化周报。
5. 前端A：负责 `L5 + L6`，P4 类型/API、Dashboard 治理总览和成本看板。
6. 前端B：负责 `L7`，Milvus 运维页、审计中心、告警中心和周报页。
7. QA/SRE：负责 `L8`，冒烟、回归、降级演练、审计抽样和 SOP 交接。
8. 安全/合规：负责确认审计脱敏、权限控制、保留期和导出规则。

---

## 8. 阶段验收模板（执行后填写）

1. 功能完成情况（按 L0～L8）：
2. 已完成接口：
3. 已冻结字段口径：
4. 成本治理验收：
   - 成本汇总：
   - 成本趋势：
   - 成本拆分：
   - 高成本 query：
   - 成本与质量联动：
5. Milvus/Collection 运维验收：
   - Collection 列表：
   - 健康详情：
   - 容量巡检：
   - 重建：
   - 切换：
   - 回滚：
6. 审计中心验收：
   - 事件覆盖：
   - 筛选查询：
   - 详情脱敏：
   - 导出：
   - 补偿队列：
   - 审计覆盖率：
7. 告警与门禁验收：
   - 告警列表：
   - ack/resolve：
   - 告警下钻：
   - 门禁检查：
8. 周报验收：
   - 生成：
   - 查看：
   - 导出：
   - 异常项下钻：
9. 契约缺口记录：
   - 接口：
   - 字段：
   - 影响页面：
   - 是否阻塞长期运营：
10. 冒烟测试结果：
11. 回归测试结果：
12. 降级演练结果：
13. 审计抽样结果：
14. 已知遗留问题：
15. 是否完成 Phase 4 Gate：是/否

---

## 9. Phase 4 完成后下一步

**P4 完成后系统进入长期运营模式：**

1. 将自动化周报转为固定运营节奏。
2. 将成本异常、质量退化、容量风险和审计缺口纳入每周复盘。
3. 基于成本与质量趋势调整 TopK、rewrite、parent fill、rerank 和模型策略。
4. 基于 Collection 健康与容量趋势安排索引重建和容量扩展。
5. 基于审计与风险清单完善权限、安全、数据保留和导出策略。
6. 后续新增策略、实验、索引或审计事件时，先更新本文档再实现。

完成 Phase 4 Gate 后，RAG 管理台不再按一次性项目推进，而进入“指标驱动 + 门禁发布 + 周期复盘”的长期治理模式。

---

## 10. 已知遗留问题（P4 首版不修复）

| 问题 | 原因 | 影响 | 计划阶段 |
|---|---|---|---|
| 不接入真实云厂商账单 | 首版以请求级估算成本为主 | 账单与估算可能存在差异 | 后续运营增强 |
| 不做跨区域容灾控制台 | 当前管理台聚焦单区域 RAG 治理 | 无法在前端完成跨区域切换演练 | 后续基础设施阶段 |
| 不做完整权限系统重构 | P4 只消费已有管理员身份和权限 | 审计查询权限粒度受现有体系限制 | 安全专项 |
| 不做外部告警渠道集成 | 首版先在管理台内闭环 | 不能直接推送飞书、Slack、PagerDuty | 后续告警集成 |
| 不在前端计算成本与门禁 | 口径必须由后端统一 | 前端强依赖后端聚合 API 完整性 | 持续约束 |
| 不直接暴露原始敏感内容 | 合规要求最小必要展示 | 排查时需按权限申请更多上下文 | 持续约束 |

