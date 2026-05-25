# Phase 1 详细功能实现路线（RAG 监控总览与结构化日志可视化，前后端并行推进）

## 1. 文档定位

本文档是 `admin/docs/rag-admin-frontend-roadmap-v2.md` 中 Phase 1（P1）"RAG 监控总览与结构化日志可视化"的详细执行手册，同时覆盖前后端，可直接按 L 编号逐步实现。

三个用途：

1. 作为前后端 P1 联合推进的统一执行文档，以 P0 稳定底座为起点。
2. 作为冻结监控 API 字段口径、日志结构、指标定义的协作基线。
3. 作为 Phase 2 接入离线评测看板前的可观测性底座。

**统一口径说明：**

1. `监控总览` 固定指：Dashboard 页面展示入库成功率、检索请求量、检索 P95、空结果率、失败类型 TopN 五类指标，时间范围支持 1h/24h/7d。
2. `结构化检索日志` 固定指：`request_id / query / kb_id / topk / routes / final_count / duration_ms / stage_durations / result_status / error_code / created_at`，字段缺失时页面明确标识，不静默隐藏。
3. `stage_durations` 固定指：`embedding_ms / search_ms / postprocess_ms`（对应后端 `KBRetrieveLog` 已有字段）。
4. `trace 详情` 固定指：单次检索请求的完整结构化信息，包含基础请求信息、阶段耗时、route 命中数量、filter 前后数量、final results、error detail。
5. `入库日志` 固定指：`KBIngestJob` 的完整状态机记录，含操作审计（`KBJobOperationLog`）。
6. `metrics/overview API` 固定指：后端基于 `KBRetrieveLog` 和 `KBIngestJob` 聚合计算，不依赖 Prometheus 外部查询，P1 不接入 Grafana。
7. `契约缺口` 固定指：关键字段缺失时页面明确标识，而不是静默隐藏或用默认值填充。

---

## 2. 当前现状（基于代码扫描）

### 2.1 后端已有能力

经扫描 `backend/api/handler/kb/handler.go`、`backend/api/router/custom_kb.go`、`backend/internal/model/`、`backend/internal/observability/`：

1. **检索日志模型完整**：`KBRetrieveLog`（`backend/internal/model/kb_retrieve_log.go`）已有 30+ 字段，含 `request_id / query / final_query / rewrite / routes / final_count / result_status / error_code / embedding_ms / search_ms / postprocess_ms / duration_ms / created_at` 等全部 P1 所需字段。
2. **检索日志 DAO 完整**：`GetByRequestID() / ListByUserID() / ListByStatus()` 已实现，支持分页和状态过滤。
3. **检索日志写入已接入**：`Retrieve()` 函数通过 `persistRetrieveLog()` 异步写入，`EnableRetrieveAudit` 开关控制。
4. **检索日志 API 已注册**：
   - `GET /api/admin/kb/retrieve/audit` → `ListRetrieveAuditLogs`（支持 `result_status` 过滤和分页）
   - `GET /api/admin/kb/retrieve/audit/:request_id` → `GetRetrieveAuditLog`
5. **入库任务模型完整**：`KBIngestJob` 已有完整状态机（7 态）和操作审计字段（`operator_id / operation / operation_reason / operated_at`）。
6. **操作审计日志模型完整**：`KBJobOperationLog`（`backend/internal/model/kb_job_operation_log.go`）已有 `job_id / operator_id / operation / from_status / to_status / operation_reason / created_at`。
7. **Prometheus 指标已实现**：`backend/internal/observability/metrics/rag_metrics.go` 已有 11 类指标（retrieve/ingest/error/consumer），`ObserveRetrieve()` 在每次检索后调用。
8. **分布式追踪已接入**：CozeLoop 客户端（`backend/internal/observability/looptrace/client.go`）已实现，P1 不需要新建追踪基础设施。

### 2.2 前端已有能力

经扫描 `admin/src/`：

1. **布局骨架完整**：`AdminShell`（`admin/src/components/admin/admin-shell.tsx`）已有 Sider 导航，`/trace-logs`、`/quality-monitor`、`/audit` 三个 P1 导航项已声明但处于禁用态。
2. **Dashboard 骨架已就绪**：`dashboard-page.tsx` 已有 P1 预留区域（`Empty` 占位），明确标注"此处不展示虚假图表，预留给 Phase 1"。
3. **API 客户端可复用**：`admin/src/services/api/client.ts` 已有通用 Axios 实例，含响应拦截和错误处理。
4. **知识库 Context 可复用**：`KnowledgeBaseProvider` 提供 `selectedBase / bases / setSelectedBaseId`。
5. **检索实验室已有 `request_id` 复制入口**：`retrieval-lab-page.tsx` 已有 P1 trace 链接预留位（灰色禁用态）。
6. **P0 类型定义已就绪**：`admin/src/types/kb.ts` 已有 `KBIngestJob / KBDocument / RetrieveResponse` 等基础类型。

### 2.3 当前真实缺口

**后端缺口：**

1. `GET /api/admin/kb/retrieve/audit` 不支持 `kb_id` 过滤，前端无法按知识库筛选检索日志。
2. `GET /api/admin/kb/retrieve/audit` 不支持 `query` 模糊搜索和 `request_id` 精确查找。
3. `GET /api/admin/kb/retrieve/audit` 不支持时间范围过滤（`start_time / end_time`）。
4. 没有 `GET /api/admin/kb/metrics/overview` 聚合指标接口，Dashboard 无法展示趋势数据。
5. 没有 `GET /api/admin/kb/logs/ingest` 入库日志列表接口（当前只有 `GET /api/admin/kb/jobs`，字段不完全对齐 P1 需求）。
6. 没有 `GET /api/admin/kb/logs/ingest/:job_id` 入库日志详情接口（含操作审计记录）。
7. `KBRetrieveLog` 的 `ListByUserID` 和 `ListByStatus` DAO 方法不支持多条件组合过滤（kb_id + status + 时间范围）。

**前端缺口：**

1. `admin/src/types/kb.ts` 缺少 `KBRetrieveLog` 类型定义（后端模型已有，前端类型未建）。
2. `admin/src/types/kb.ts` 缺少 `KBJobOperationLog` 类型定义。
3. `admin/src/types/kb.ts` 缺少 `MetricsOverview` 等监控聚合类型。
4. `admin/src/config/api.ts` 缺少所有 P1 监控相关 API 路径常量。
5. `/trace-logs/retrieval` 路由和页面组件不存在（导航项禁用，无对应 `page.tsx`）。
6. `/trace-logs/ingest` 路由和页面组件不存在。
7. Dashboard 监控趋势图区域为 `Empty` 占位，未接入任何真实数据。
8. 检索实验室 P1 trace 链接预留位为禁用态，P1 需激活为真实跳转。

---

## 3. 目标与通过标准（Gate）

Phase 1 通过标准（全满足）：

1. `/dashboard` 展示真实的入库成功率、检索请求量、检索 P95、空结果率趋势图，支持 1h/24h/7d 切换。
2. `/trace-logs/retrieval` 页面可按时间范围、知识库、result_status、request_id 筛选检索日志，表格展示 `query / kb_id / topk / final_count / duration_ms / result_status`。
3. 点击任意检索日志行，右侧抽屉展示完整 trace 详情（阶段耗时、route 命中、filter 前后数量、error detail）。
4. `/trace-logs/ingest` 页面可按知识库、任务状态、错误类型筛选入库日志，展示任务操作审计记录。
5. 检索实验室 `request_id` 旁的 trace 链接激活，点击可跳转到 `/trace-logs/retrieval?request_id=xxx`。
6. 所有监控页面接口失败时有可读错误提示，页面不白屏。
7. 前端类型定义与后端返回字段完全对齐，不存在 TypeScript 类型错误。
8. `/trace-logs` 和 `/quality-monitor` 导航项从禁用态激活（`/quality-monitor` 激活为 P2 预留占位页）。

---

## 4. 实现路线总览（L0 → L8）

Phase 1 按 9 条路线推进，按门禁顺序合流：

1. L0：现状盘点与 P1 口径冻结
2. L1：后端 — 检索日志查询接口增强（多条件过滤）
3. L2：后端 — 监控聚合指标接口（metrics/overview）
4. L3：后端 — 入库日志详情接口（含操作审计）
5. L4：前端 — P1 类型契约与 API 路径补齐
6. L5：前端 — 检索日志页（/trace-logs/retrieval）
7. L6：前端 — Dashboard 监控趋势图接入
8. L7：前端 — 入库日志页（/trace-logs/ingest）+ 检索实验室 trace 链接激活
9. L8：回归验收、回滚预案与 Phase 2 交接

建议顺序：`L0 → L1 + L2 + L3（并行）→ L4 → L5 + L6 + L7（并行）→ L8`

---

## 5. 详细路线拆解

### 5.1 L0 现状盘点与 P1 口径冻结

#### 目标

在动手之前，冻结 P1 所有 API 字段口径、监控指标定义、日志结构，避免开发过程中来回改口径。

#### 功能任务

1. 确认 `KBRetrieveLog` 当前 JSON 字段列表（`request_id / user_id / kb_ids / query / final_query / expr / top_k / candidate_topk / final_topk / token_budget / truncate_reason / rewrite / rewrite_strategy / rewrite_applied / routes / collection / retriever_version / final_count / truncated_count / result_status / error_code / error_msg / embedding_ms / search_ms / postprocess_ms / duration_ms / timeout_ms / created_at`），作为前端类型定义的依据。
2. 确认 `KBJobOperationLog` 当前 JSON 字段列表（`id / job_id / operator_id / operation / from_status / to_status / operation_reason / created_at`），作为入库日志详情的依据。
3. 冻结 `metrics/overview` 接口返回结构（见 L2 定义），前后端对齐后不再变更。
4. 冻结 P1 不可回退功能清单：
   - 检索日志可按 `request_id` 精确查找
   - 检索日志可按时间范围、知识库、状态筛选
   - 单次检索 trace 详情可展示阶段耗时
   - 入库日志可展示操作审计记录
5. 记录已知缺口（不在 P1 修复）：
   - P1 不接入 Prometheus/Grafana，监控数据来自数据库聚合
   - P1 不做实时推送（WebSocket/SSE），监控数据手动刷新或定时轮询
   - P1 不做 `/quality-monitor` 真实内容，只激活为占位页

#### 验收

1. 前后端对 P1 必做与不做边界达成一致。
2. 所有 API 字段口径以本文档为准，不靠口头同步。

---

### 5.2 L1 后端 — 检索日志查询接口增强

#### 目标

增强 `GET /api/admin/kb/retrieve/audit` 接口，支持多条件组合过滤，让前端检索日志页可以按知识库、状态、时间范围、query 关键词、request_id 筛选。

#### 功能任务

1. 修改 `backend/internal/model/kb_retrieve_log.go` 中的 `List` 方法（或新增 `ListWithFilter` 方法）：
   - 增加过滤参数：`kbID *uint64 / resultStatus *string / startTime *time.Time / endTime *time.Time / queryKeyword *string / requestID *string`
   - `kbID` 非空时：`WHERE JSON_CONTAINS(kb_ids, JSON_QUOTE(?))` 或按实际存储格式过滤
   - `resultStatus` 非空时：`WHERE result_status = ?`
   - `startTime / endTime` 非空时：`WHERE created_at BETWEEN ? AND ?`
   - `queryKeyword` 非空时：`WHERE query LIKE ?`（前后加 `%`，不做全文索引，P1 数据量小可接受）
   - `requestID` 非空时：`WHERE request_id = ?`（精确匹配）
   - 保持原有分页逻辑不变
2. 修改 `backend/api/handler/kb/handler.go` 中的 `ListRetrieveAuditLogs` 函数：
   - 读取新增 Query 参数：`kb_id / start_time / end_time / query_keyword / request_id`
   - 参数校验：`start_time / end_time` 格式为 RFC3339，非法时返回 400
   - 把参数传入 DAO 查询

#### 接口变更

```
GET /api/admin/kb/retrieve/audit?kb_id=1&result_status=error&start_time=2026-05-01T00:00:00Z&end_time=2026-05-26T23:59:59Z&query_keyword=推荐&request_id=xxx&page=1&page_size=20
```

返回结构不变，仍是：

```json
{
  "items": [...],
  "total": 42,
  "page": 1,
  "page_size": 20
}
```

#### 验收

1. `?kb_id=1` 只返回该知识库的检索日志。
2. `?result_status=error` 只返回失败日志。
3. `?start_time=&end_time=` 时间范围过滤正确。
4. `?query_keyword=推荐` 返回 query 包含"推荐"的日志。
5. `?request_id=xxx` 精确返回该请求的日志（0 或 1 条）。
6. 不传任何过滤参数时行为与原来一致（返回全量分页）。
7. 非法时间格式返回 400。

---

### 5.3 L2 后端 — 监控聚合指标接口

#### 目标

提供 `GET /api/admin/kb/metrics/overview` 接口，让 Dashboard 页面一次获取 P1 所需的 5 类监控指标，不依赖 Prometheus 外部查询。

#### 功能任务

1. 在 `backend/api/handler/kb/handler.go` 新增 `GetMetricsOverview` 函数：
   - 读取 Query 参数：`kb_id`（可选）、`range`（枚举：`1h / 24h / 7d`，默认 `24h`）
   - 根据 `range` 计算 `startTime = now - range`
   - 并发（或串行）查询以下聚合数据：
     - **入库成功率**：`COUNT(*) WHERE status='completed' / COUNT(*) WHERE status IN ('completed','failed','dead')` 按时间分桶
     - **检索请求量**：`COUNT(*) FROM kb_retrieve_log WHERE created_at >= startTime` 按时间分桶
     - **检索 P95 耗时**：`duration_ms` 的 P95 分位数（按时间分桶，或全量计算）
     - **空结果率**：`COUNT(*) WHERE result_status='no_result' / COUNT(*)` 按时间分桶
     - **失败类型 TopN**：`GROUP BY error_code ORDER BY COUNT(*) DESC LIMIT 5`（`result_status='error'` 的日志）
   - 时间分桶规则：`1h` → 5 分钟桶，`24h` → 1 小时桶，`7d` → 6 小时桶
   - 返回结构：
     ```json
     {
       "range": "24h",
       "ingest_success_rate": [
         {"bucket": "2026-05-25T10:00:00Z", "rate": 0.95, "total": 20, "success": 19}
       ],
       "retrieve_request_count": [
         {"bucket": "2026-05-25T10:00:00Z", "count": 42}
       ],
       "retrieve_p95_ms": [
         {"bucket": "2026-05-25T10:00:00Z", "p95_ms": 320}
       ],
       "retrieve_empty_rate": [
         {"bucket": "2026-05-25T10:00:00Z", "rate": 0.08, "total": 42, "empty": 3}
       ],
       "error_type_topn": [
         {"error_code": "embedding_failed", "count": 5},
         {"error_code": "timeout", "count": 2}
       ]
     }
     ```
2. P95 计算方式：对时间窗口内所有 `duration_ms` 排序后取第 95 百分位（数据量小时可接受，P2 可优化为预聚合）。
3. 在 `backend/api/router/custom_kb.go` 注册新路由：
   ```go
   group.GET("/metrics/overview", kb.GetMetricsOverview)
   ```

#### 接口

```
GET /api/admin/kb/metrics/overview?kb_id=1&range=24h
```

#### 验收

1. 接口可独立打通，返回 5 类指标数据。
2. `range=1h` 返回 12 个 5 分钟桶，`range=24h` 返回 24 个 1 小时桶，`range=7d` 返回 28 个 6 小时桶。
3. `kb_id` 过滤时，检索指标只统计该知识库的日志，入库指标只统计该知识库的任务。
4. 无数据时返回空数组，不返回 null。
5. 非法 `range` 值返回 400。

---

### 5.4 L3 后端 — 入库日志详情接口

#### 目标

提供 `GET /api/admin/kb/logs/ingest` 列表接口和 `GET /api/admin/kb/logs/ingest/:job_id` 详情接口，让前端入库日志页可以展示任务操作审计记录，而不只是任务状态。

#### 功能任务

1. 在 `backend/api/handler/kb/handler.go` 新增 `ListIngestLogs` 函数：
   - 复用 `ListJobs` 的查询逻辑，但返回字段更完整（含 `last_error_code / last_error_detail / operation / operation_reason / operated_at`）
   - 支持过滤参数：`kb_id / status / error_code / start_time / end_time / page / page_size`
   - 返回结构与 `ListJobs` 一致（`items / total / page / page_size`）
   - 注意：`ListIngestLogs` 与 `ListJobs` 的区别在于字段完整性和过滤能力，不是两套数据
2. 在 `backend/api/handler/kb/handler.go` 新增 `GetIngestLogDetail` 函数：
   - 根据 `job_id` 查询 `KBIngestJob` 完整记录
   - 同时查询 `KBJobOperationLog` 中该 `job_id` 的所有操作记录（按 `created_at` 升序）
   - 返回结构：
     ```json
     {
       "job": { ...KBIngestJob 完整字段... },
       "operation_logs": [
         {
           "id": 1,
           "operator_id": 42,
           "operation": "retry",
           "from_status": "failed",
           "to_status": "retrying",
           "operation_reason": "手动重试",
           "created_at": "2026-05-25T10:00:00Z"
         }
       ]
     }
     ```
3. 在 `backend/api/router/custom_kb.go` 注册新路由：
   ```go
   group.GET("/logs/ingest", kb.ListIngestLogs)
   group.GET("/logs/ingest/:job_id", kb.GetIngestLogDetail)
   ```

#### 接口

```
GET /api/admin/kb/logs/ingest?kb_id=1&status=failed&error_code=embedding_failed&start_time=...&page=1&page_size=20
GET /api/admin/kb/logs/ingest/:job_id
```

#### 验收

1. `ListIngestLogs` 返回包含 `last_error_code / operation / operation_reason` 的完整任务记录。
2. `GetIngestLogDetail` 返回任务详情和该任务的所有操作审计记录（按时间升序）。
3. 没有操作记录时 `operation_logs` 返回空数组，不返回 null。
4. `job_id` 不存在时返回 404。

---

### 5.5 L4 前端 — P1 类型契约与 API 路径补齐

#### 目标

在 `admin/src/types/kb.ts` 和 `admin/src/config/api.ts` 中补齐 P1 所需的全部类型定义和 API 路径常量，为后续三条前端路线提供类型安全基础。

#### 功能任务

1. 在 `admin/src/types/kb.ts` 新增以下类型：
   ```ts
   export interface KBRetrieveLog {
     id: number;
     request_id: string;
     user_id: number;
     kb_ids: number[];
     query: string;
     final_query?: string;
     expr?: string;
     top_k: number;
     candidate_topk?: number;
     final_topk?: number;
     token_budget?: number;
     truncate_reason?: string;
     rewrite?: boolean;
     rewrite_strategy?: string;
     rewrite_applied?: boolean;
     routes?: string;
     collection?: string;
     retriever_version?: string;
     final_count: number;
     truncated_count?: number;
     result_status: 'success' | 'no_result' | 'filtered_out' | 'error' | 'timeout';
     error_code?: string;
     error_msg?: string;
     embedding_ms?: number;
     search_ms?: number;
     postprocess_ms?: number;
     duration_ms: number;
     timeout_ms?: number;
     created_at: string;
   }

   export interface KBJobOperationLog {
     id: number;
     job_id: number;
     operator_id: number;
     operation: string;
     from_status: string;
     to_status: string;
     operation_reason?: string;
     created_at: string;
   }

   export interface IngestLogDetail {
     job: KBIngestJob;
     operation_logs: KBJobOperationLog[];
   }

   export type MetricsRange = '1h' | '24h' | '7d';

   export interface MetricsBucket {
     bucket: string;
   }

   export interface IngestSuccessRateBucket extends MetricsBucket {
     rate: number;
     total: number;
     success: number;
   }

   export interface RetrieveCountBucket extends MetricsBucket {
     count: number;
   }

   export interface RetrieveP95Bucket extends MetricsBucket {
     p95_ms: number;
   }

   export interface RetrieveEmptyRateBucket extends MetricsBucket {
     rate: number;
     total: number;
     empty: number;
   }

   export interface ErrorTypeTopN {
     error_code: string;
     count: number;
   }

   export interface MetricsOverview {
     range: MetricsRange;
     ingest_success_rate: IngestSuccessRateBucket[];
     retrieve_request_count: RetrieveCountBucket[];
     retrieve_p95_ms: RetrieveP95Bucket[];
     retrieve_empty_rate: RetrieveEmptyRateBucket[];
     error_type_topn: ErrorTypeTopN[];
   }
   ```

2. 在 `admin/src/config/api.ts` 的 `KB_ADMIN_API` 中补充 P1 路径：
   ```ts
   // P1 监控相关
   METRICS_OVERVIEW: `${API_BASE_URL}/admin/kb/metrics/overview`,
   LIST_RETRIEVE_LOGS: `${API_BASE_URL}/admin/kb/retrieve/audit`,
   GET_RETRIEVE_LOG: (requestId: string) =>
     `${API_BASE_URL}/admin/kb/retrieve/audit/${requestId}`,
   LIST_INGEST_LOGS: `${API_BASE_URL}/admin/kb/logs/ingest`,
   GET_INGEST_LOG_DETAIL: (jobId: number | string) =>
     `${API_BASE_URL}/admin/kb/logs/ingest/${jobId}`,
   ```

#### 验收

1. TypeScript 编译无类型错误。
2. `KBRetrieveLog` 字段与后端 `KBRetrieveLog` JSON 完全对齐。
3. `MetricsOverview` 字段与 L2 后端接口返回结构完全对齐。
4. 所有 P1 API 路径常量已定义，后续路线直接引用，不硬编码 URL。

---

### 5.6 L5 前端 — 检索日志页（/trace-logs/retrieval）

#### 目标

新建 `/trace-logs/retrieval` 页面，展示可筛选的检索日志列表，点击行展示 trace 详情抽屉。同时激活 `/trace-logs` 导航项。

#### 功能任务

1. 新建路由文件 `admin/src/app/(admin)/trace-logs/retrieval/page.tsx`，渲染 `RetrievalLogsPage` 组件。
2. 新建 `admin/src/components/admin/retrieval-logs-page.tsx`：
   - **筛选栏**（顶部）：
     - 时间范围选择器（`DatePicker.RangePicker`，默认最近 24h）
     - 知识库选择器（复用 `KnowledgeBaseContext` 的 `bases` 列表，可选"全部"）
     - 状态筛选（`Select`：全部 / success / no_result / filtered_out / error / timeout）
     - `request_id` 精确搜索输入框（`Input.Search`）
     - query 关键词搜索输入框
   - **日志表格**（`Table`）：
     - 列：`request_id`（可复制）、`query`（截断 50 字）、`kb_id`、`topk`、`final_count`、`duration_ms`（格式化为 `Xms`）、`result_status`（`Tag` 着色）、`created_at`
     - 分页：默认 20 条/页，支持切换
     - 行点击：打开 trace 详情抽屉
   - **trace 详情抽屉**（`Drawer`，宽 600px）：
     - 基础信息：`request_id`（可复制）、`query`、`final_query`（有 rewrite 时展示）、`kb_ids`、`routes`、`result_status`
     - 阶段耗时（`Descriptions`）：`embedding_ms / search_ms / postprocess_ms / duration_ms`，缺失字段展示 `-`
     - 检索参数：`top_k / candidate_topk / final_topk / token_budget / truncate_reason`
     - 结果统计：`final_count / truncated_count`
     - 错误信息（有 `error_code` 时展示）：`error_code`（红色 `Tag`）、`error_msg`
     - rewrite 信息（有 `rewrite=true` 时展示）：`rewrite_strategy / rewrite_applied`
   - **加载态**：表格 `loading` 状态
   - **空态**：无数据时展示 `Empty`
   - **错误态**：接口失败时展示 `Alert`，不白屏
3. 修改 `admin/src/components/admin/admin-shell.tsx`：
   - 将 `/trace-logs` 导航项从禁用态改为启用态
   - 将 `/trace-logs/retrieval` 作为 `/trace-logs` 的默认子路由（点击跳转）
4. 新建 `admin/src/app/(admin)/trace-logs/page.tsx`，重定向到 `/trace-logs/retrieval`（或直接渲染检索日志页）。

#### 接口依赖

- `GET /api/admin/kb/retrieve/audit`（L1 后端增强后对接）
- `GET /api/admin/kb/retrieve/audit/:request_id`（已有，用于抽屉详情）

#### 验收

1. `/trace-logs/retrieval` 页面可访问，展示检索日志列表。
2. 筛选条件变更后，表格数据自动刷新（不需要手动点击搜索按钮，或有明确搜索按钮）。
3. 点击任意行，右侧抽屉展示完整 trace 详情，阶段耗时字段缺失时展示 `-`。
4. `request_id` 可复制。
5. 接口失败时展示 `Alert`，不白屏。
6. 导航栏 `/trace-logs` 从禁用态变为可点击。

---

### 5.7 L6 前端 — Dashboard 监控趋势图接入

#### 目标

把 Dashboard 页面的 P1 预留区域从 `Empty` 占位升级为展示真实监控趋势图。

#### 功能任务

1. 修改 `admin/src/components/admin/dashboard-page.tsx`：
   - 在组件 mount 时调用 `GET /api/admin/kb/metrics/overview?range=24h`（默认 24h）
   - 顶部新增时间范围切换器（`Radio.Group`：1h / 24h / 7d），切换后重新请求
   - 展示 5 个监控区域：
     - **入库成功率趋势**：折线图，X 轴为时间桶，Y 轴为成功率（0-100%）
     - **检索请求量趋势**：柱状图，X 轴为时间桶，Y 轴为请求数
     - **检索 P95 耗时趋势**：折线图，X 轴为时间桶，Y 轴为毫秒
     - **空结果率趋势**：折线图，X 轴为时间桶，Y 轴为空结果率（0-100%）
     - **失败类型 TopN**：横向柱状图，X 轴为数量，Y 轴为 `error_code`
   - 图表库：使用 Ant Design Charts（`@ant-design/charts`）或 Recharts，优先选择项目已有依赖
   - 加载中展示 `Skeleton` 占位
   - 接口失败时展示 `Alert`，图表区域展示空态，不白屏
   - 无数据时（空数组）展示 `Empty` 占位，不展示空坐标轴
2. 保留 P0 的 4 个数量卡片（知识库数量、文档数量、处理中任务数、失败任务数），趋势图在卡片下方。
3. 如果项目没有图表库依赖，使用 Ant Design `Statistic` + 简单的数字展示替代趋势图，并在代码注释中标注"待引入图表库后升级为折线图"。

#### 接口依赖

`GET /api/admin/kb/metrics/overview`（L2 后端完成后对接）。

#### 验收

1. `/dashboard` 展示 5 类监控指标，数据来自真实 API。
2. 时间范围切换后，图表数据更新。
3. 接口失败时图表区域展示 `Alert`，P0 数量卡片不受影响。
4. 无数据时展示 `Empty`，不展示空坐标轴或假数据。

---

### 5.8 L7 前端 — 入库日志页 + 检索实验室 trace 链接激活

#### 目标

新建 `/trace-logs/ingest` 入库日志页，展示任务操作审计记录；同时激活检索实验室的 trace 链接，使其跳转到 `/trace-logs/retrieval?request_id=xxx`。

#### 功能任务

**入库日志页（`/trace-logs/ingest`）：**

1. 新建路由文件 `admin/src/app/(admin)/trace-logs/ingest/page.tsx`，渲染 `IngestLogsPage` 组件。
2. 新建 `admin/src/components/admin/ingest-logs-page.tsx`：
   - **筛选栏**：知识库选择器、任务状态筛选（`Select`）、错误类型输入框（`Input`）、时间范围选择器
   - **日志表格**（`Table`）：
     - 列：`id`、`document_id`、`kb_id`、`status`（`Tag` 着色）、`last_error_code`（红色 `Tag`，无值时 `-`）、`retry_count`、`started_at`、`finished_at`、`duration`（`finished_at - started_at`，格式化）
     - 行点击：打开操作审计抽屉
   - **操作审计抽屉**（`Drawer`，宽 500px）：
     - 任务基础信息：`id / kb_id / document_id / status / retry_count / last_error_code / last_error_detail`
     - 操作记录时间线（`Timeline`）：按 `created_at` 升序展示每条 `KBJobOperationLog`，每条显示 `operation / from_status → to_status / operation_reason / created_at`
     - 无操作记录时展示"暂无操作记录"
3. 在 `admin/src/components/admin/admin-shell.tsx` 中，`/trace-logs` 下新增子导航项：
   - 检索日志（`/trace-logs/retrieval`）
   - 入库日志（`/trace-logs/ingest`）

**检索实验室 trace 链接激活（`retrieval-lab-page.tsx`）：**

4. 修改 `admin/src/components/admin/retrieval-lab-page.tsx`：
   - 将 P0 预留的灰色禁用链接"查看 Trace（P1 上线后启用）"改为真实可点击链接
   - 点击后跳转到 `/trace-logs/retrieval?request_id={request_id}`（使用 Next.js `router.push`）
   - 移除 P0 的 `Tooltip` 说明文字（功能已上线，不再需要说明）

#### 接口依赖

- `GET /api/admin/kb/logs/ingest`（L3 后端完成后对接）
- `GET /api/admin/kb/logs/ingest/:job_id`（L3 后端完成后对接）

#### 验收

1. `/trace-logs/ingest` 页面可访问，展示入库日志列表。
2. 点击任意行，抽屉展示任务详情和操作审计时间线。
3. 无操作记录时抽屉展示"暂无操作记录"，不崩溃。
4. 检索实验室 trace 链接可点击，跳转到 `/trace-logs/retrieval?request_id=xxx`，且检索日志页自动填充该 `request_id` 并展示对应日志。
5. `/trace-logs` 导航下有"检索日志"和"入库日志"两个子项。

---

### 5.9 L8 回归验收、回滚预案与 Phase 2 交接

#### 目标

证明 P1 所有功能闭环，并把 P2 所需的基础交接清楚。

#### 冒烟测试清单

1. 访问 `/dashboard` 成功，展示 P0 数量卡片 + P1 监控趋势图。
2. 时间范围切换（1h/24h/7d）后，趋势图数据更新。
3. 访问 `/trace-logs/retrieval` 成功，展示检索日志列表。
4. 按知识库、状态、时间范围筛选后，列表数据正确过滤。
5. 输入 `request_id` 精确搜索，返回对应日志（0 或 1 条）。
6. 点击日志行，抽屉展示完整 trace 详情，阶段耗时字段缺失时展示 `-`。
7. 访问 `/trace-logs/ingest` 成功，展示入库日志列表。
8. 点击入库日志行，抽屉展示任务详情和操作审计时间线。
9. 检索实验室执行检索后，点击 trace 链接跳转到 `/trace-logs/retrieval?request_id=xxx`，自动展示对应日志。
10. 所有监控页面接口失败时有 `Alert` 提示，不白屏。

#### 回归测试清单

1. P0 所有功能不回退：知识库管理、文档上传、任务重试/取消、检索测试。
2. Dashboard P0 数量卡片不受 P1 趋势图影响。
3. 检索实验室 `request_id` 复制功能不受 trace 链接激活影响。
4. 切换知识库后，检索日志筛选器的知识库选项自动更新。
5. 接口 500 或网络失败时，页面有可读提示，不白屏。

#### 回滚预案

1. **后端 `metrics/overview` 接口**：如果出问题，前端降级为展示 P0 数量卡片，趋势图区域展示 `Alert`，不影响其他页面。
2. **后端检索日志过滤增强**：如果出问题，前端可临时移除新增筛选参数，退回到只支持 `result_status` 过滤的原始接口。
3. **后端入库日志详情接口**：如果出问题，前端降级为不展示操作审计时间线，只展示任务基础信息。
4. **前端图表库**：如果图表渲染出问题，降级为 `Statistic` 数字展示，不影响数据正确性。

#### Phase 2 交接清单

P1 完成后，Phase 2 可直接基于以下底座推进：

1. 可复用监控布局：Dashboard 趋势图区域（P2 可在此基础上增加评测指标趋势）。
2. 可复用日志筛选组件：检索日志页的筛选栏和表格（P2 评测报告可复用相同模式）。
3. 可复用 trace 详情抽屉：P2 可在此基础上增加 rerank 分数、citation precision 等字段。
4. 可复用 API 路径常量：`KB_ADMIN_API` 中的 P1 路径（P2 新增评测相关路径时遵循相同命名规范）。
5. 可复用类型定义：`KBRetrieveLog / MetricsOverview`（P2 评测类型可参考相同结构）。
6. `/evaluation` 导航项已在 `admin-shell.tsx` 中声明（禁用态），P2 直接激活。

后端 P2 需补充的接口（P1 不做，供 P2 参考）：

- `POST /api/admin/kb/evaluation/datasets`：创建评测集
- `GET /api/admin/kb/evaluation/datasets`：评测集列表
- `POST /api/admin/kb/evaluation/runs`：创建评测运行
- `GET /api/admin/kb/evaluation/runs/:run_id`：评测运行状态
- `GET /api/admin/kb/evaluation/reports/:run_id`：评测报告（Recall@K、MRR、nDCG 等）

#### 验收

1. 冒烟测试清单全通过。
2. 回归测试清单无阻塞问题。
3. P2 交接清单已确认，后端和前端各自知道下一步做什么。

---

## 6. 推荐协作节奏

1. 先完成 `L0`，对齐 P1 口径，冻结 API 字段和监控指标定义。
2. `L1 + L2 + L3` 后端并行推进，三个改动相互独立。
3. `L1 + L2 + L3` 完成后，前端进入 `L4` 类型契约补齐。
4. `L5 + L6 + L7` 前端并行推进：`L5` 依赖 `L1`，`L6` 依赖 `L2`，`L7` 依赖 `L3` 和 `L4`。
5. 所有路线在 `L8` 合流验收。

---

## 7. 角色分工建议

1. 后端：负责 `L1 + L2 + L3`，按顺序或并行推进，完成后通知前端对接。
2. 前端：负责 `L4 + L5 + L6 + L7`，`L4` 先行，后三条并行。
3. 联调/验收：执行 `L8` 冒烟、回归、回滚预案演练。

---

## 8. 阶段验收模板（执行后填写）

1. 功能完成情况（按 L0～L8）：
2. 已完成接口：
3. 已展示字段：
4. 监控趋势图状态：
   - 图表库：
   - 时间范围切换：
   - 无数据降级：
5. 检索日志筛选能力：
   - 支持的过滤参数：
   - 分页行为：
6. trace 详情抽屉：
   - 展示字段：
   - 缺失字段处理：
7. 契约缺口记录：
   - 接口：
   - 字段：
   - 影响页面：
   - 是否阻塞 P2：
8. 冒烟测试结果：
9. 回归测试结果：
10. 已知遗留问题：
11. 是否可以进入 Phase 2：是/否

---

## 9. Phase 1 完成后下一步

**P1 完成后交给 P2 的稳定底座：**

1. 可观测性基础设施完整：检索日志可查、入库日志可查、监控趋势可见。
2. trace 链路打通：从检索实验室 → 检索日志 → trace 详情的完整下钻路径可用。
3. Dashboard 监控框架就绪：P2 可在此基础上增加评测指标趋势图。
4. 导航结构稳定：`/trace-logs` 已激活，`/evaluation` 预留位就绪。

**P2 需要的 API 和能力：**

- 评测集 CRUD API
- 评测运行触发和状态查询 API
- 评测报告聚合 API（Recall@K、MRR、nDCG、Citation Precision）
- 检索日志中增加 rerank 分数字段（P1 不做，P2 补充）

---

## 10. 已知遗留问题（P1 不修复）

| 问题 | 原因 | 影响 | 计划阶段 |
|---|---|---|---|
| P1 不接入 Prometheus/Grafana | 监控数据来自数据库聚合，避免引入外部依赖 | 监控数据有延迟，不是实时流式 | P4 |
| 监控趋势图无实时推送 | P1 不做 WebSocket/SSE | 需手动刷新或定时轮询才能看到最新数据 | P3/P4 |
| `query` 关键词搜索无全文索引 | P1 数据量小，LIKE 查询可接受 | 数据量 >10 万条时查询变慢 | P2 优化 |
| P95 耗时无预聚合 | P1 实时计算，数据量小时可接受 | 数据量 >10 万条时聚合变慢 | P2 优化 |
| `/quality-monitor` 只是占位页 | P1 不做质量监控内容 | 导航项可点击但无实质内容 | P2 |
| 检索 trace 无 rerank 分数字段 | 后端 `KBRetrieveLog` 当前无 rerank 字段 | trace 详情看不到 rerank 过程 | P2 |
| 入库日志无阶段耗时（parsing/chunking/embedding/writing） | `KBIngestJob` 无阶段耗时字段 | 入库日志只能看总耗时，看不到哪个阶段慢 | P2 |
