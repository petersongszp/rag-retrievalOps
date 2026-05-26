# Phase 0 详细功能实现路线（前后端并行推进）

## 1. 文档定位

本文档是 `admin/docs/rag-admin-frontend-roadmap-v2.md` 中 Phase 0（P0）"管理台基础重构与知识库闭环"的详细执行手册，同时覆盖前后端，可直接按 L 编号逐步实现。

三个用途：

1. 作为前后端 P0 联合推进的统一执行文档。
2. 作为冻结 API 字段口径、状态机、错误分类的协作基线。
3. 作为 Phase 1 接入监控总览和结构化日志前的稳定底座。

**统一口径说明：**

1. `知识库闭环` 固定指：知识库创建、文档上传、文档列表、文档删除、异步入库、任务状态追踪、失败任务重试、任务取消、检索测试。
2. `统一检索结果契约` 固定指：`request_id / items / content / score / citation / source`。
3. `citation` 在 P0 最小包含：`kb_id / document_id / chunk_id / file_name / chunk_index`。
4. `source` 在 P0 最小包含：`route / collection / retriever_version`。
5. `任务状态机` 固定指：`pending → processing → completed/failed/retrying/dead/canceled`（已在后端实现）。
6. `契约缺口` 固定指：关键字段缺失时页面明确标识，而不是静默隐藏或用默认值填充。

---

## 2. 当前现状（基于代码扫描）

### 2.1 后端已有能力

经扫描 `backend/api/handler/kb/handler.go` 和 `backend/internal/model/`：

1. 全部 P0 API 已实现并注册：
   - `POST /api/admin/kb/bases`、`GET /api/admin/kb/bases`
   - `POST /api/admin/kb/documents/upload`、`GET /api/admin/kb/documents`、`DELETE /api/admin/kb/documents/:document_id`
   - `GET /api/admin/kb/jobs`、`GET /api/admin/kb/jobs/:job_id`、`POST /api/admin/kb/jobs/:job_id/retry`、`POST /api/admin/kb/jobs/:job_id/cancel`
   - `POST /api/admin/kb/retrieve`
   - `GET /api/admin/kb/retrieve/audit`、`GET /api/admin/kb/retrieve/audit/:request_id`
   - `POST /api/admin/kb/ingest/pause`、`POST /api/admin/kb/ingest/resume`、`GET /api/admin/kb/ingest/status`
2. `KBIngestJob` 模型已有完整状态机和字段：`status / retry_count / last_error_code / last_error_detail / started_at / finished_at / operation / operation_reason`。
3. `KBRetrieveLog` 模型已有完整检索日志字段：`request_id / query / final_query / rewrite / routes / final_count / result_status / embedding_ms / search_ms / duration_ms` 等。
4. 文件重复上传保护（`file_hash` 去重）已实现。
5. 入库队列暂停/恢复已实现。

### 2.2 前端已有能力

经扫描 `admin/src/`：

1. 路由骨架已就绪：`/dashboard`、`/knowledge-bases`、`/knowledge-bases/[kbId]`、`/retrieval-lab`。
2. 公共布局已就绪：`AdminShell`（`admin-shell.tsx`）含 Sider、Header、Breadcrumb、知识库选择器。
3. 知识库 Context 已就绪：`KnowledgeBaseProvider`（`knowledge-base-provider.tsx`）含 `bases / selectedBase / createBase / refreshBases`。
4. 知识库列表页已就绪：`KnowledgeBasesPage`（`knowledge-bases-page.tsx`）。
5. 知识库详情页已就绪：`KnowledgeBaseDetailPage`（`knowledge-base-detail-page.tsx`），含上传、文档表、任务表、重试、取消。
6. 检索测试页已就绪：`RetrievalLabPage`（`retrieval-lab-page.tsx`），含 `request_id` 复制和契约缺口检测。
7. API 配置已就绪：`admin/src/config/api.ts`（`KB_ADMIN_API`）。
8. 类型定义已就绪：`admin/src/types/kb.ts`（`KnowledgeBase / KBDocument / KBIngestJob / RetrieveItem / RetrieveResponse`）。

### 2.3 当前真实缺口

**后端缺口：**

1. `GET /api/admin/kb/jobs` 不支持 `?kb_id=` 过滤，前端只能拉全量再客户端过滤，分页下会漏数据。
2. 没有 Dashboard 聚合统计 API，无法一次获取知识库数量、文档数量、处理中任务数、失败任务数。
3. 文档列表接口不返回 `ingest_duration_ms` 和 `last_ingest_job_id`（需关联查询或冗余存储）。
4. `KBIngestJob` 模型没有 `stage` 和 `progress` 字段（当前无阶段进度，P0 先不做，标记为已知缺口）。

**前端缺口：**

1. `admin/src/types/kb.ts` 中 `KBIngestJob` 类型缺少 `kb_id / last_error_code / last_error_detail / operation / operation_reason / operated_at` 字段。
2. `KnowledgeBaseDetailPage` 中 `ListJobs` 调用不传 `kb_id`，靠 `job.kb_id === kbId` 客户端过滤（类型也未定义 `kb_id`，TypeScript 过滤无效）。
3. 任务表未展示 `last_error_code` 和 `operation` 字段，失败原因不可读。
4. 任务列表没有自动轮询，需手动刷新才能看到状态变化。
5. `Dashboard` 页面只展示静态占位卡片，未接入真实 API 数据。
6. 文档列表未展示 `last_ingest_job_id` 和 `ingest_duration_ms`（后端先补，前端再接）。
7. 检索结果未预留 P1 的 trace 下钻入口（当前只有 `request_id` 复制按钮）。

---

## 3. 目标与通过标准（Gate）

Phase 0 通过标准（全满足）：

1. `/dashboard` 展示真实的知识库数量、文档数量、处理中任务数、失败任务数。
2. `/knowledge-bases/[kbId]` 任务列表通过 `?kb_id=` 过滤，分页正确。
3. 任务失败后页面能看到明确错误分类（`last_error_code`），而不是只看 `error_msg` 堆栈。
4. 任务状态变化自动轮询刷新，无需手动点击刷新。
5. 检索测试结果稳定展示 `score / citation / source`，契约缺口明确提示。
6. `request_id` 可复制，并预留 P1 的 trace 下钻链接入口。
7. 前端类型定义与后端返回字段完全对齐，不存在 TypeScript 类型过滤失效问题。
8. 所有接口失败都有可读错误提示，页面不白屏。

---

## 4. 实现路线总览（L0 → L8）

Phase 0 按 9 条路线推进，按门禁顺序合流：

1. L0：现状盘点与缺口冻结
2. L1：后端 — 任务列表 kb_id 过滤
3. L2：后端 — Dashboard 聚合统计 API
4. L3：后端 — 文档列表补充 last_ingest_job_id 与 ingest_duration_ms
5. L4：前端 — 类型契约补齐
6. L5：前端 — 任务表格修复与自动轮询
7. L6：前端 — Dashboard 真实数据接入
8. L7：前端 — 文档列表字段展示 + 检索实验室 P1 入口预留
9. L8：回归验收、回滚预案与 Phase 1 交接

建议顺序：`L0 → L1 + L2 + L3（并行）→ L4 → L5 + L6 + L7（并行）→ L8`

---

## 5. 详细路线拆解

## 5.1 L0 现状盘点与缺口冻结

### 目标

在动手之前，先冻结当前代码现状、已有能力清单、P0 缺口清单和不可回退行为，避免开发过程中来回改口径。

### 任务

1. 确认后端路由文件 `backend/api/router/custom_kb.go` 中所有 P0 API 已注册（已确认）。
2. 确认 `KBIngestJob` 当前 JSON 字段列表（`id / kb_id / document_id / user_id / status / retry_count / error_msg / last_error_code / last_error_detail / started_at / finished_at / created_at / updated_at`），作为前端类型补齐的依据。
3. 确认 `KBRetrieveLog` 当前 JSON 字段列表作为 P1 对接基础（P0 只用 `request_id`，不展示 trace 详情）。
4. 冻结 P0 不可回退功能清单：
   - 上传文档后任务列表能看到对应任务
   - 失败任务可重试
   - 任务可取消
   - 删除文档后检索不再命中
   - 检索结果能展示 `request_id / score / citation / source`
5. 记录已知缺口（不在 P0 修复）：
   - `KBIngestJob` 无 `stage / progress` 字段，任务进度无法分阶段展示
   - P0 不做监控趋势图，Dashboard 只展示 4 个数量卡片

### 验收

1. 前后端对 P0 必做与不做边界达成一致。
2. 后续任何字段变更都有本文档为依据，不靠口头同步。

---

## 5.2 L1 后端 — 任务列表 kb_id 过滤

### 目标

让 `GET /api/admin/kb/jobs` 支持 `?kb_id=` 参数，前端不再需要拉全量再客户端过滤。

### 功能任务

1. 修改 `backend/api/handler/kb/handler.go` 中的 `ListJobs` 函数：
   - 读取 Query 参数 `kb_id`（可选）
   - 非空时解析为 `uint64`，参数无效时返回 400
   - 把 `kb_id` 传入查询逻辑
2. 修改 `backend/internal/model/kb_ingest_job.go` 中的 `_KBIngestJob.List` 方法：
   - 增加 `kbID *uint64` 参数
   - 非空时在查询中加 `WHERE kb_id = ?`
   - 保持原有 `status` 过滤逻辑不变
3. 同样更新 `ListJobs` 中对 `model.KBIngestJobDao.List` 的调用，传入解析后的 `kb_id`。

### 接口变更

```
GET /api/admin/kb/jobs?kb_id=1&status=failed&page=1&page_size=10
```

返回结构不变，仍是：

```json
{
  "items": [...],
  "total": 5,
  "page": 1,
  "page_size": 10
}
```

### 验收

1. `GET /api/admin/kb/jobs?kb_id=1` 只返回该知识库的任务。
2. 不传 `kb_id` 时行为与原来一致（返回全量）。
3. 传入非法 `kb_id` 时返回 400。

---

## 5.3 L2 后端 — Dashboard 聚合统计 API

### 目标

提供一个轻量聚合接口，让 Dashboard 页面一次获取 4 个数量指标，而不是前端并发 3 个列表请求再手动计数。

### 功能任务

1. 在 `backend/api/handler/kb/handler.go` 新增 `GetDashboardStats` 函数：
   - 并发查询以下数据（用 goroutine 或串行均可，数量小）：
     - 知识库总数：`model.KBKnowledgeBaseDao.Count()`
     - 文档总数：`model.KBDocumentDao.CountNonDeleted()`
     - 处理中任务数：`model.KBIngestJobDao.CountByStatus(pending, processing, retrying)`
     - 失败任务数：`model.KBIngestJobDao.CountByStatus(failed, dead)`
   - 返回结构：
     ```json
     {
       "kb_count": 3,
       "document_count": 42,
       "processing_job_count": 2,
       "failed_job_count": 1
     }
     ```
2. 在 `backend/internal/model/kb_knowledge_base.go` 新增 `Count()` 方法。
3. 在 `backend/internal/model/kb_document.go` 新增 `CountNonDeleted()` 方法（WHERE `deleted = 0`）。
4. 在 `backend/internal/model/kb_ingest_job.go` 新增 `CountByStatuses(statuses []KBIngestJobStatus)` 方法。
5. 在 `backend/api/router/custom_kb.go` 的 `registerKBGroup` 中注册新路由：
   ```go
   group.GET("/dashboard/stats", kb.GetDashboardStats)
   ```

### 接口

```
GET /api/admin/kb/dashboard/stats
```

### 验收

1. 接口可独立打通，返回 4 个正确数字。
2. 知识库、文档、任务各自的数字与数据库实际数据一致。

---

## 5.4 L3 后端 — 文档列表补充 last_ingest_job_id 与 ingest_duration_ms

### 目标

让文档列表返回两个附加字段，供前端展示入库结果摘要，而不需要前端再额外请求任务详情。

### 功能任务

1. 在 `backend/internal/model/kb_document.go` 的 `KBDocument` 结构体中新增两个**非数据库列的计算字段**（`gorm:"-"` 标签）：
   ```go
   LastIngestJobID  *uint64 `json:"last_ingest_job_id" gorm:"-"`
   IngestDurationMs *int64  `json:"ingest_duration_ms" gorm:"-"`
   ```
2. 在 `backend/api/handler/kb/handler.go` 的 `ListDocuments` 函数中，查完文档列表后，批量查一次各文档最新任务，回填这两个字段：
   - 对每个 `document_id`，用 `model.KBIngestJobDao.GetLatestByDocumentID` 获取最新任务
   - `LastIngestJobID` = 最新任务的 `ID`
   - `IngestDurationMs`：若任务 `StartedAt` 和 `FinishedAt` 都不为空，则 = `FinishedAt - StartedAt`（毫秒）；否则为 nil
   - 文档数量通常较少（分页 10 条），N+1 查询在 P0 可接受；P1 可优化为批量 IN 查询

### 验收

1. 文档列表返回中包含 `last_ingest_job_id` 和 `ingest_duration_ms`。
2. 完成入库的文档两个字段都有值，未入库或入库失败的文档 `ingest_duration_ms` 为 null。
3. 原有文档列表字段（`id / file_name / status / chunk_count` 等）不变。

---

## 5.5 L4 前端 — 类型契约补齐

### 目标

让 `admin/src/types/kb.ts` 中的类型与后端实际 JSON 完全对齐，消除当前 `kb_id` 过滤失效、字段读取为 `undefined` 等隐性 bug。

### 功能任务

1. 补齐 `KBIngestJob` 类型（在 `admin/src/types/kb.ts`）：
   ```ts
   export interface KBIngestJob {
     id: number;
     kb_id: number;                    // 补充（当前缺失）
     document_id: number;
     user_id: number;
     status: 'pending' | 'processing' | 'completed' | 'failed' | 'retrying' | 'dead' | 'canceled';
     retry_count: number;
     error_msg?: string;
     last_error_code?: string;         // 补充
     last_error_detail?: string;       // 补充
     operation?: string;               // 补充
     operation_reason?: string;        // 补充
     operated_at?: string;             // 补充
     started_at?: string;
     finished_at?: string;
     created_at: string;
     updated_at: string;
   }
   ```
2. 补齐 `KBDocument` 类型中的附加字段：
   ```ts
   export interface KBDocument {
     // ...原有字段...
     last_ingest_job_id?: number;      // 补充（L3 后端完成后生效）
     ingest_duration_ms?: number;      // 补充（L3 后端完成后生效）
   }
   ```
3. 在 `admin/src/config/api.ts` 中补充新接口路径：
   ```ts
   export const KB_ADMIN_API = {
     // ...原有...
     DASHBOARD_STATS: `${API_BASE_URL}/admin/kb/dashboard/stats`,
     LIST_JOBS_BY_KB: (kbId: number | string) =>
       `${API_BASE_URL}/admin/kb/jobs?kb_id=${kbId}`,
   };
   ```

### 验收

1. TypeScript 编译无类型错误。
2. `KBIngestJob.kb_id` 类型正确，客户端过滤逻辑有效。
3. 补充字段在后端返回时前端能正常读取，不返回时不报错。

---

## 5.6 L5 前端 — 任务表格修复与自动轮询

### 目标

修复任务列表两个核心问题：按 `kb_id` 过滤（从后端过滤替换客户端过滤）和自动轮询状态变化。

### 功能任务

1. 修改 `admin/src/components/admin/knowledge-base-detail-page.tsx` 中的任务加载逻辑：
   - 将 `apiClient.get(KB_ADMIN_API.LIST_JOBS)` 改为 `apiClient.get(KB_ADMIN_API.LIST_JOBS_BY_KB(kbId))`
   - 删除客户端 `.filter((job) => job.kb_id === kbId)` 的过滤逻辑
2. 补充任务表格字段展示：
   - 新增 `last_error_code` 列：有值时展示为红色 `<Tag>`，无值时展示空
   - 新增 `operation` 列：展示最近一次操作（`retry` / `cancel`），有值时展示，无值时空
3. 增加自动轮询逻辑（在 `knowledge-base-detail-page.tsx`）：
   - 检查 `jobs` 中是否存在 `pending / processing / retrying` 状态的任务
   - 有则每 3 秒刷新一次任务列表（只刷新任务，不刷新文档）
   - 所有任务到达终态（`completed / failed / dead / canceled`）后停止轮询
   - 页面 unmount 时清除定时器
   - 轮询失败时静默，不打断用户操作
4. 上传文档成功后，同时刷新文档列表和任务列表。

### 验收

1. 知识库详情页任务列表只展示当前知识库的任务，分页正确。
2. 有进行中任务时，状态变化自动刷新，无需手动点击刷新。
3. 任务失败后能看到 `last_error_code`（如 `parse_failed` / `embedding_failed`）。
4. 页面切换后轮询自动停止，不产生内存泄漏。

---

## 5.7 L6 前端 — Dashboard 真实数据接入

### 目标

把 Dashboard 页面从静态占位升级为展示真实聚合数据的状态卡片。

### 功能任务

1. 修改 `admin/src/components/admin/dashboard-page.tsx`：
   - 在组件 mount 时调用 `GET /api/admin/kb/dashboard/stats`
   - 展示 4 个真实数字卡片：
     - 知识库数量（点击跳转 `/knowledge-bases`）
     - 文档数量（点击跳转 `/knowledge-bases`）
     - 处理中任务数（点击跳转当前知识库详情的任务 Tab）
     - 失败任务数（点击跳转当前知识库详情的任务 Tab）
   - 加载中展示 `Skeleton` 占位
   - 接口失败时展示 `Alert` 提示，卡片展示 `-`，不白屏
2. P1 预留区域保持 `Empty` 占位，不展示静态假图表。
3. 使用 `KnowledgeBaseContext` 的 `selectedBase` 决定失败/处理中任务的跳转目标（有选中知识库时跳详情，无时跳列表）。

### 接口依赖

`GET /api/admin/kb/dashboard/stats`（L2 后端完成后对接）。

### 验收

1. `/dashboard` 展示 4 个来自真实 API 的数量卡片。
2. 点击卡片能进入对应管理页面。
3. 接口失败时卡片显示 `-`，页面不白屏，有错误提示。
4. 不展示任何硬编码的趋势图或假数据。

---

## 5.8 L7 前端 — 文档列表字段展示 + 检索实验室 P1 入口预留

### 目标

展示 L3 后端新增的附加字段，同时在检索实验室预留 P1 trace 下钻入口。

### 功能任务

**文档列表（`knowledge-base-detail-page.tsx`）：**

1. 在 `documentColumns` 中补充两列：
   - `last_ingest_job_id`：有值时展示为可点击链接（未来可跳转至任务详情），无值时展示 `-`
   - `ingest_duration_ms`：有值时格式化为 `Xms` 或 `X.Xs`，无值时展示 `-`
2. 字段缺失时不报错，展示 `-`（后端未返回时的降级）。

**检索实验室（`retrieval-lab-page.tsx`）：**

3. 在展示 `request_id` 的区域，`复制 request_id` 按钮旁边新增一个灰色禁用链接：
   - 文字：`查看 Trace（P1 上线后启用）`
   - 使用 `<Tooltip>` 说明：`Phase 1 完成后，此处可直接跳转到 /trace-logs/retrieval/{request_id}`
   - P0 不实际跳转，只预留 UI 入口位置
4. 不修改现有检索流程和结果展示逻辑。

### 验收

1. 文档列表展示 `last_ingest_job_id` 和 `ingest_duration_ms`（后端 L3 完成前显示 `-`）。
2. 检索实验室有 P1 trace 入口预留位，且有 Tooltip 说明当前状态。
3. 新增字段在后端未返回时不导致页面崩溃。

---

## 5.9 L8 回归验收、回滚预案与 Phase 1 交接

### 目标

证明 P0 所有功能闭环，并把 P1 所需的基础交接清楚。

### 冒烟测试清单

1. 访问 `/dashboard` 成功，展示真实数量卡片。
2. 访问 `/knowledge-bases` 成功，知识库列表正常加载。
3. 创建知识库成功，自动跳转详情页。
4. 进入 `/knowledge-bases/[kbId]` 成功，展示知识库基本信息。
5. 上传合法文件（pdf/md/txt）成功，文档列表出现记录。
6. 任务列表只展示当前知识库的任务（不混入其他知识库）。
7. 入库任务状态自动刷新，无需手动点击。
8. 失败任务显示 `last_error_code`，点击重试后状态变更。
9. 任务取消成功，状态变为 `canceled`。
10. 删除文档二次确认后成功，列表刷新。
11. 访问 `/retrieval-lab` 成功，执行检索测试返回结果。
12. 检索结果展示 `score / citation / source`，有缺失字段时标红提示。
13. `request_id` 可复制，P1 入口预留位可见。

### 回归测试清单

1. P0 各页面拆分后，上传/删除/重试/取消/检索流程不回退。
2. 刷新 `/knowledge-bases/[kbId]` 后，知识库信息和当前选择状态正确恢复。
3. 切换知识库后，任务列表和检索测试默认使用新 `kb_id`。
4. 后端字段缺失时（如 `last_error_code` 为空），页面不崩溃，展示降级 UI。
5. 接口 500 或网络失败时，页面有可读提示，不白屏。

### 回滚预案

1. 后端 `ListJobs` 新增 `kb_id` 参数：如果出问题，可在前端临时改回 `LIST_JOBS` 并恢复客户端过滤，不影响展示正确性（只影响大知识库分页场景）。
2. 后端 Dashboard Stats API：如果出问题，前端降级为展示 `-`，不影响其他页面。
3. 后端文档列表附加字段：字段缺失时前端展示 `-`，不影响现有功能。
4. 前端轮询：如果轮询导致接口压力，可将间隔从 3 秒改为 10 秒，或增加 `paused` 状态判断跳过轮询。

### Phase 1 交接清单

P0 完成后，Phase 1 可直接基于以下底座推进：

1. 可复用 Layout：`AdminShell`（Header/Sider/Breadcrumb/知识库选择器）。
2. 可复用知识库 Context：`selectedBase / bases / setSelectedBaseId`。
3. 可复用 API client：`admin/src/services/api/client.ts`。
4. 可复用检索结果展示：`RetrievalLabPage`、契约缺口检测。
5. `/dashboard` 页面骨架（P1 指标趋势图挂载位已预留）。
6. `request_id` 复制入口和 P1 trace 链接预留位。
7. `/trace-logs/retrieval` 和 `/trace-logs/ingest` 导航承接位（Sider 已有禁用态入口）。

后端 P1 需补充的接口（P0 不做，供 P1 参考）：

- `GET /api/admin/kb/metrics/overview`：监控总览聚合指标
- `GET /api/admin/kb/logs/retrieval`：检索日志列表（P0 已有 `retrieve/audit` 基础）
- `GET /api/admin/kb/logs/retrieval/{request_id}`：单次检索 trace 详情（P0 已有 `retrieve/audit/:request_id`）

### 验收

1. 冒烟测试清单全通过。
2. 回归测试清单无阻塞问题。
3. P1 交接清单已确认，后端和前端各自知道下一步做什么。

---

## 6. 推荐协作节奏

1. 先完成 `L0`，对齐缺口清单，不要边开发边发现字段。
2. `L1 + L2 + L3` 后端并行推进，三个改动相互独立。
3. `L1 + L2 + L3` 完成后，前端进入 `L4` 类型契约补齐。
4. `L5 + L6 + L7` 前端并行推进，`L5` 依赖 `L1`，`L6` 依赖 `L2`，`L7` 依赖 `L3`（文档字段部分）和 `L4`（类型）。
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
4. 任务轮询行为：
   - 轮询间隔：
   - 终止条件：
   - 失败处理：
5. 契约缺口记录：
   - 接口：
   - 字段：
   - 影响页面：
   - 是否阻塞 P1：
6. 冒烟测试结果：
7. 回归测试结果：
8. 已知遗留问题（如 stage/progress 缺失）：
9. 是否可以进入 Phase 1：是/否

---

## 9. 已知遗留问题（P0 不修复）

| 问题 | 原因 | 影响 | 计划阶段 |
|---|---|---|---|
| `KBIngestJob` 无 `stage / progress` 字段 | 后端当前无阶段进度上报 | 任务表只能看最终态，看不到"正在 embedding"等中间阶段 | P1 |
| Dashboard 无入库成功率趋势图 | 依赖聚合时序数据，P0 不做 | Dashboard 只有数量卡片，无趋势可见 | P1 |
| 检索 trace 详情页 | 依赖 `/trace-logs` 路由和 P1 后端接口 | P0 只有 `request_id` 复制，无法下钻 | P1 |
| 文档 ingest_duration_ms 批量查询 N+1 | P0 先单条查，文档数少时可接受 | 文档数 >50 时查询变慢 | P1 优化 |
