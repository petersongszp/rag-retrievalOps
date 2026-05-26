# Phase 0 详细功能实现路线（RAG 管理后台前端基础重构与知识库闭环）

## 1. 文档定位

本文档是 `admin/docs/rag-admin-frontend-roadmap.md` 中 Phase 0（P0）的详细执行手册，目标是把“管理台基础重构与知识库闭环”拆成可直接实施、可联调、可验收的任务路线。

它有三个用途：

1. 作为前端推进 P0 的统一执行文档。
2. 作为前后端冻结接口契约和字段缺口的协作基线。
3. 作为 Phase 1 接入监控总览、结构化日志和 trace 详情的页面基础。

本文档风格与 `backend/docs/phase0-rag-baseline-detailed-roadmap.md` 保持一致：
目标 -> 路线 -> 模块任务 -> 验收门禁 -> 下一步入口。

统一口径说明：

1. `知识库闭环` 固定指：知识库列表、创建知识库、文档上传、文档列表、文档删除、入库任务列表、失败任务重试、任务取消、检索测试。
2. `统一检索结果契约` 固定指：`request_id/items/content/score/citation/source`。
3. `source` 在 P0 最小包含：`route/collection`，如果后端已返回 `retriever_version`，前端也要展示。
4. `citation` 在 P0 最小包含：`file_name/chunk_index/chunk_id`，如果后端已返回 `kb_id/document_id`，前端也要展示。
5. `最小状态卡片` 固定指：知识库数量、文档数量、处理中任务数、失败任务数。
6. `契约缺口提示` 固定指：关键字段缺失时页面明确标识，而不是静默隐藏或用假值填充。

---

## 2. Phase 0 范围边界

## 2.1 本阶段必须完成

1. 将当前 `admin/src/app/page.tsx` 的单页后台拆成可扩展管理台骨架。
2. 建立 Phase 0 必需路由：
   - `/dashboard`
   - `/knowledge-bases`
   - `/knowledge-bases/[kbId]`
   - `/retrieval-lab`
3. 抽离公共布局：
   - `Header`
   - `Sider`
   - `Breadcrumb`
   - 当前知识库选择器
   - 页面内容区
4. 抽离知识库业务组件：
   - `KnowledgeBaseList`
   - `DocumentTable`
   - `IngestJobTable`
   - `UploadDocumentModal`
   - `RetrieveTestPanel`
   - `CitationCard`
5. 保持原有知识库上传、删除、任务重试、任务取消、检索测试功能不回退。
6. 检索测试结果完整展示 `score/citation/source`，并对缺字段做契约缺口提示。
7. `/dashboard` 接入最小状态卡片，数据来自真实 API 或真实列表聚合。
8. 整理 `admin/src/services/api/client.ts` 和 `admin/src/types/kb.ts`，让后续 P1 页面复用。
9. 给 Phase 1 预留 `Trace Logs` 导航入口和 `request_id` 下钻入口。

## 2.2 本阶段明确不做

1. 不做完整监控趋势图，入库成功率、检索 P95、空结果率趋势进入 Phase 1。
2. 不做结构化日志列表和 trace 详情页，`/trace-logs/retrieval` 与 `/trace-logs/ingest` 进入 Phase 1。
3. 不做离线评测集、评测运行、A/B 对比和评测报告，这些进入 Phase 2。
4. 不做高级检索链路调试，包括 rewrite、fusion、dedupe、rerank、filter/truncate 过程视图，这些进入 Phase 3。
5. 不做策略中心、feature flag、灰度、一键回滚和审计闭环，这些进入 Phase 3/4。
6. 不用前端硬编码假数据模拟监控能力；后端没有数据时只展示空状态或契约缺口。
7. 不引入新的 UI 框架，继续使用 Next.js 14、React 18、TypeScript、Ant Design 5。

---

## 3. 目标与通过标准（Gate）

Phase 0 通过标准（全满足）：

1. `/dashboard` 可访问，并展示知识库数量、文档数量、处理中任务数、失败任务数。
2. `/knowledge-bases` 可完成知识库列表查看、创建知识库、进入知识库详情。
3. `/knowledge-bases/[kbId]` 可完成文档上传、文档列表、文档删除、入库任务列表、失败任务重试、任务取消。
4. `/retrieval-lab` 可对当前知识库执行检索测试，并展示 `request_id/items/content/score/citation/source`。
5. 页面拆分后，原有上传、删除、任务重试、任务取消、检索测试功能不回退。
6. 检索结果中 `score/citation/source` 任一字段缺失时，页面能明确标识缺失字段。
7. 导航、面包屑、当前知识库选择器在刷新和切换页面后状态稳定。
8. 所有接口失败都有可读错误提示，页面不白屏。
9. Phase 1 所需的 Layout、知识库上下文、API client、`request_id` 复制入口和导航承接位已经准备好。

---

## 4. 实现路线总览（L0 -> L8）

Phase 0 按 9 条路线推进，按门禁顺序合流：

1. L0：现状盘点与契约冻结
2. L1：路由结构与管理台 Layout
3. L2：API client 与类型契约整理
4. L3：知识库列表与当前知识库上下文
5. L4：知识库详情、文档与入库任务闭环
6. L5：检索测试与引用结果展示
7. L6：Dashboard 最小状态卡片
8. L7：状态体验、错误处理与权限预留
9. L8：回归验收、回滚预案与 Phase 1 交接

建议顺序：`L0 -> L1 -> L2 -> L3 -> L4 + L5 -> L6 -> L7 -> L8`

---

## 5. 详细路线拆解

## 5.1 L0 现状盘点与契约冻结

### 目标

在拆分页面前，先冻结当前能力、API 契约、字段缺口和不可回退功能，避免重构过程中丢功能。

### 功能任务

1. 盘点 `admin/src/app/page.tsx` 当前承载的功能：
   - 知识库列表
   - 创建知识库
   - 文档上传
   - 文档列表
   - 文档删除
   - 入库任务列表
   - 失败任务重试
   - 任务取消
   - 检索测试弹窗
2. 盘点 `admin/src/services/api/client.ts` 已有 API 方法，形成 P0 API 清单：
   - 知识库列表
   - 创建知识库
   - 文档上传
   - 文档列表
   - 文档删除
   - 入库任务列表
   - 失败任务重试
   - 任务取消
   - 检索测试
3. 盘点 `admin/src/types/kb.ts` 已有类型，标出字段缺口：
   - `request_id`
   - `score`
   - `citation.file_name`
   - `citation.chunk_index`
   - `citation.chunk_id`
   - `source.route`
   - `source.collection`
   - `source.retriever_version`
4. 冻结 P0 不可回退功能清单：
   - 上传文档后能刷新文档与任务列表
   - 删除文档后列表更新
   - 失败任务能重试
   - 任务能取消
   - 检索测试能显示结果
5. 建立拆分前冒烟用例，作为 L8 回归基线。

### 验收

1. 已有功能清单、API 清单、类型缺口清单明确。
2. P0 页面都能映射到明确的数据来源。
3. 重构前后需要对比的冒烟用例已经记录。

---

## 5.2 L1 路由结构与管理台 Layout

### 目标

把单页后台拆成可扩展管理台结构，为 P1-P4 页面提供统一导航、面包屑和内容容器。

### 功能任务

1. 建立或调整路由：
   - `/dashboard`
   - `/knowledge-bases`
   - `/knowledge-bases/[kbId]`
   - `/retrieval-lab`
2. 根路径策略二选一：
   - 根路径跳转到 `/dashboard`
   - 根路径跳转到 `/knowledge-bases`
3. 抽离公共 Layout：
   - 顶部 `Header`
   - 左侧 `Sider`
   - `Breadcrumb`
   - 内容区域
   - 当前知识库选择器挂载位置
4. 左侧导航按业务域分组：
   - `Dashboard`
   - `Knowledge Bases`
   - `Retrieval Lab`
   - `Trace Logs`（P1 预留）
   - `Evaluation`（P2 预留）
   - `Strategy Center`（P3 预留）
   - `Quality Monitor`（P3/P4 预留）
   - `Cost & Ops`（P4 预留）
   - `Audit`（P4 预留）
5. 导航高亮与当前路径联动，刷新页面后保持正确高亮。
6. 面包屑按路由生成：
   - `Dashboard`
   - `Knowledge Bases`
   - `Knowledge Bases / {kb.name}`
   - `Retrieval Lab`
7. P0 暂不实现的导航项要使用禁用态或建设中空状态，避免跳到不存在页面。

### 验收

1. 四个 P0 路由可直接访问，刷新后 Layout 正常。
2. 导航高亮和面包屑正确。
3. 旧单页功能迁移后不再挤在 `admin/src/app/page.tsx` 一个文件中。
4. P1-P4 的导航位置已经预留，但不会误导用户以为功能已完成。

---

## 5.3 L2 API client 与类型契约整理

### 目标

让拆分后的页面复用统一 API client 和类型定义，避免每个页面重复拼接请求和字段判断。

### 功能任务

1. 在 `admin/src/services/api/client.ts` 中按业务域整理方法：
   - `knowledgeBase`
   - `document`
   - `ingestJob`
   - `retrieval`
2. 在 `admin/src/types/kb.ts` 中补齐或统一类型：
   - `KnowledgeBase`
   - `KnowledgeDocument`
   - `IngestJob`
   - `RetrieveRequest`
   - `RetrieveResponse`
   - `RetrieveItem`
   - `Citation`
   - `ResultSource`
3. 检索响应类型至少兼容：
   - `request_id`
   - `items`
   - `content`
   - `score`
   - `citation`
   - `source`
4. 文档列表类型建议兼容后端补充字段：
   - `ingest_duration_ms`
   - `last_ingest_job_id`
   - `chunk_count`
   - `file_hash`
5. 任务列表类型建议兼容后端补充字段：
   - `stage`
   - `progress`
   - `retry_count`
   - `error_code`
   - `error_msg`
   - `started_at`
   - `finished_at`
6. 增加统一契约缺口判断：
   - `score` 缺失
   - `citation` 缺失
   - `source` 缺失
   - `request_id` 缺失
7. API 错误处理保持统一：
   - 网络失败
   - 后端业务错误
   - 权限失败
   - 响应结构异常

### 验收

1. 页面层不直接散落重复 fetch 逻辑。
2. TypeScript 类型能表达字段可选和契约缺口。
3. 检索结果缺字段时能定位到具体字段。
4. 后端补字段后，前端无需大改页面结构即可展示。

---

## 5.4 L3 知识库列表与当前知识库上下文

### 目标

让知识库成为管理台全局上下文，后续 Dashboard、Retrieval Lab、Trace Logs 都能复用当前 `kb_id`。

### 功能任务

1. 抽离 `KnowledgeBaseList`：
   - 知识库名称
   - 描述
   - 状态
   - 创建时间
   - 文档数量（有字段则展示，无字段则空态）
2. 保留创建知识库能力：
   - 表单校验
   - 创建成功后刷新列表
   - 创建失败展示后端错误
3. 实现当前知识库选择器：
   - 支持从 Header 或页面局部区域切换
   - 切换后同步影响详情页和检索测试页
   - 无知识库时展示引导空状态
4. 点击知识库进入 `/knowledge-bases/[kbId]`。
5. 访问不存在的 `kbId` 时，展示错误状态并允许返回列表。
6. 当前知识库状态在刷新页面后可恢复：
   - 优先从 URL 中的 `kbId` 恢复
   - 其次从最近选择记录恢复
   - 无可用知识库时进入空状态

### 验收

1. 能从 `/knowledge-bases` 创建知识库并进入详情页。
2. 刷新 `/knowledge-bases/[kbId]` 后能恢复知识库基础信息。
3. 切换知识库后，文档列表、任务列表、检索测试默认使用新的 `kb_id`。
4. 无知识库、接口失败、加载中三种状态都有明确表现。

---

## 5.5 L4 知识库详情、文档与入库任务闭环

### 目标

将“上传 -> 入库任务 -> 文档状态 -> 删除/重试/取消”整理为知识库详情页的核心闭环。

### 功能任务

1. 在 `/knowledge-bases/[kbId]` 展示知识库摘要：
   - 名称
   - 描述
   - 状态
   - 创建时间
   - 文档数量
   - 处理中任务数
   - 失败任务数
2. 抽离 `UploadDocumentModal`：
   - 上传时绑定当前 `kb_id`
   - 上传中按钮进入 loading
   - 上传成功后刷新文档列表和任务列表
   - 上传失败展示后端错误
3. 抽离 `DocumentTable`：
   - 文件名
   - 文件类型
   - 文件大小
   - 状态
   - `chunk_count`
   - `file_hash`
   - `last_ingest_job_id`
   - 创建时间
   - 删除操作
4. 抽离 `IngestJobTable`：
   - `job_id`
   - `document_id`
   - `stage`
   - `progress`
   - `status`
   - `retry_count`
   - `error_code`
   - `error_msg`
   - `started_at`
   - `finished_at`
   - 重试操作
   - 取消操作
5. 删除文档前必须二次确认，删除成功后刷新文档和任务数据。
6. 失败任务重试后刷新任务状态，旧错误不能继续停留为当前状态。
7. 任务轮询策略：
   - 仅在存在 `pending/processing` 任务时轮询
   - 页面离开时停止轮询
   - 轮询失败时提示，但不阻塞用户继续操作
8. 字段缺失处理：
   - 后端未返回 `chunk_count` 时显示缺口提示
   - 后端未返回 `stage/progress` 时不伪造进度
   - 后端未返回 `error_code/error_msg` 时展示通用失败提示并记录为契约缺口

### 验收

1. 上传文档后能在文档列表看到记录，在任务列表看到对应任务。
2. 入库任务状态变化能反映到页面。
3. 失败任务可以重试，任务可以取消。
4. 删除文档后列表刷新，且不会继续把已删除文档作为当前操作对象。
5. 任务扩展字段缺失时页面不崩溃，并明确暴露契约缺口。

---

## 5.6 L5 检索测试与引用结果展示

### 目标

将现有检索测试能力整理为 `/retrieval-lab` 页面，并让检索结果的 `score/citation/source` 可见、可检查、可作为 P1 trace 下钻入口。

### 功能任务

1. 抽离 `RetrieveTestPanel`：
   - 当前知识库选择
   - query 输入
   - topK 输入或选择
   - 检索按钮
   - 请求中状态
   - 错误提示
2. 检索请求必须带上当前 `kb_id` 或后端要求的知识库范围参数。
3. 展示响应基础信息：
   - `request_id`
   - 命中数量
   - 检索耗时（如后端返回）
4. 抽离 `CitationCard` 或统一结果卡片：
   - `content`
   - `score`
   - `citation.file_name`
   - `citation.chunk_index`
   - `citation.chunk_id`
   - `source.route`
   - `source.collection`
   - `source.retriever_version`
5. 对契约缺口做醒目标识：
   - `score` 缺失
   - `citation` 缺失
   - `source` 缺失
   - `request_id` 缺失
6. 支持复制 `request_id`。
7. 保留从知识库详情页进入检索测试的入口。
8. P0 暂不做链路调试，只预留后续进入 `/trace-logs/retrieval/{request_id}` 或 trace 详情的入口。

### 验收

1. `/retrieval-lab` 能执行检索并展示结果。
2. 每条结果能看出内容、分数、引用和来源。
3. 缺少 `score/citation/source` 时页面明确提示。
4. `request_id` 可复制，为 Phase 1 下钻 trace 做准备。

---

## 5.7 L6 Dashboard 最小状态卡片

### 目标

先建立 `/dashboard` 的页面骨架和最小状态入口，不提前实现 Phase 1 的完整监控看板。

### 功能任务

1. 新增 `/dashboard` 页面。
2. 展示最小状态卡片：
   - 知识库数量
   - 文档数量
   - 处理中任务数
   - 失败任务数
3. 数据来源优先使用现有知识库、文档、任务 API 聚合。
4. 如果后端暂无聚合接口，前端只做轻量列表聚合，不扫日志、不伪造趋势。
5. 卡片点击跳转：
   - 知识库数量 -> `/knowledge-bases`
   - 文档数量 -> `/knowledge-bases` 或当前知识库详情
   - 处理中任务数 -> 当前知识库详情的任务区域
   - 失败任务数 -> 当前知识库详情的任务区域
6. 预留 Phase 1 指标区域：
   - 入库成功率趋势
   - 检索 P50/P95 趋势
   - 空结果率趋势
   - 失败类型 TopN
7. Phase 1 指标区域在 P0 只展示空状态和所需 API，不展示静态假图。

### 验收

1. `/dashboard` 可访问并展示四个最小状态卡片。
2. 卡片数字来自真实 API 或真实列表聚合。
3. 点击卡片能进入对应管理页面。
4. 页面没有硬编码监控趋势假数据。

---

## 5.8 L7 状态体验、错误处理与权限预留

### 目标

让 P0 管理台在空数据、接口失败、权限不足、字段缺失时仍然可演示、可联调、可定位问题。

### 功能任务

1. 统一加载态：
   - 页面级 loading
   - 表格级 loading
   - 按钮提交 loading
2. 统一空状态：
   - 无知识库
   - 无文档
   - 无入库任务
   - 无检索结果
3. 统一错误态：
   - 网络错误
   - 后端业务错误
   - 权限错误
   - 响应结构异常
4. 统一危险操作确认：
   - 删除文档
   - 取消任务
   - 重试失败任务
5. 权限预留：
   - 操作按钮支持禁用态
   - `403` 与普通失败分开提示
   - 不在 P0 实现完整角色管理
6. 契约缺口提示要能进入联调记录：
   - 缺字段名称
   - 所属接口
   - 影响页面
   - 是否阻塞验收

### 验收

1. 接口失败时页面不白屏。
2. 空知识库、空文档、空任务、空检索结果都有可理解的界面状态。
3. 危险操作不会误触。
4. 权限不足和字段缺失能清楚区分。

---

## 5.9 L8 回归验收、回滚预案与 Phase 1 交接

### 目标

证明 P0 拆分没有破坏原有知识库闭环，并把 P1 所需的页面基础、接口缺口、字段缺口交接清楚。

### 冒烟测试清单

1. 访问 `/dashboard` 成功。
2. 访问 `/knowledge-bases` 成功。
3. 创建知识库成功。
4. 进入 `/knowledge-bases/[kbId]` 成功。
5. 上传合法文件成功。
6. 文档列表出现上传文件。
7. 入库任务列表出现对应任务。
8. 失败任务重试入口可用。
9. 任务取消入口可用。
10. 删除文档成功。
11. 访问 `/retrieval-lab` 成功。
12. 检索测试能返回结果。
13. 检索结果展示 `score/citation/source`。
14. `request_id` 可复制。

### 回归测试清单

1. 页面拆分后原有上传流程不回退。
2. 页面拆分后原有删除流程不回退。
3. 页面拆分后原有任务重试流程不回退。
4. 页面拆分后原有任务取消流程不回退。
5. 页面拆分后原有检索测试流程不回退。
6. 刷新页面后导航和面包屑正常。
7. 切换知识库后详情页、任务列表、检索测试使用正确 `kb_id`。
8. 后端返回字段缺失时页面不崩溃。

### 回滚预案

1. 路由层面：如果 `/dashboard` 未稳定，可临时把根路径指回 `/knowledge-bases`。
2. 页面层面：如果新 Layout 影响演示，可保留知识库管理页作为主入口。
3. 组件层面：如果检索结果扩展展示异常，可降级为基础 `content/score` 展示，同时保留字段缺口提示。
4. 数据层面：前端不主动改写后端数据结构，回滚不涉及数据迁移。
5. 联调层面：如果后端暂未补齐扩展字段，P0 允许以契约缺口提示通过，但 `score/citation/source` 不应全部缺失。

### 验收

1. 冒烟测试清单全通过。
2. 回归测试清单无阻塞问题。
3. 契约缺口清单已经记录并分配负责人。
4. P1 所需页面基础和导航入口已准备好。

---

## 6. 推荐实施节奏（无固定时长）

## 6.1 阶段推进建议

1. 先完成 `L0`，冻结功能清单、API 清单和字段缺口。
2. 再完成 `L1`，拆出路由与公共 Layout。
3. 然后完成 `L2`，整理 API client 和 TypeScript 类型。
4. 接着完成 `L3 + L4`，恢复知识库、文档、任务主闭环。
5. 同步完成 `L5`，把检索测试和引用结果展示迁移到 `/retrieval-lab`。
6. 再完成 `L6`，补 `/dashboard` 最小状态卡片。
7. 最后完成 `L7 + L8`，统一状态体验、回归验收、回滚预案和 P1 交接。

## 6.2 并行与合流规则

1. `L1` 和 `L2` 可以并行，但合流前必须统一 Layout 使用方式和 API 调用方式。
2. `L3` 与 `L4` 可以并行，但都依赖 `L2` 的类型契约稳定。
3. `L5` 可以和 `L4` 并行，但检索响应字段必须先冻结。
4. `L6` 不依赖完整监控 API，但不能使用静态假数据伪造趋势。
5. 所有路线最终在 `L8` 合流验收，通过后再进入 Phase 1。

---

## 7. 角色分工（建议）

1. 前端A：`L1` 路由结构、公共 Layout、导航、面包屑。
2. 前端B：`L2 + L3` API client、类型契约、知识库列表、当前知识库上下文。
3. 前端C：`L4` 知识库详情、上传弹窗、文档表格、任务表格、任务操作。
4. 前端D：`L5 + L6` 检索测试、引用结果展示、Dashboard 最小状态卡片。
5. 后端：保持知识库、文档、任务、检索 API 稳定，并补齐 `request_id/items/content/score/citation/source`。
6. QA/联调：维护冒烟清单、回归清单、契约缺口记录和 P1 交接检查。

补充协作约束：

1. 前后端先冻结 `RetrieveResponse` 字段，再细化 `CitationCard`。
2. 前端各路线合流前必须统一当前 `kb_id` 的来源和切换行为。
3. Dashboard P0 只允许展示真实状态卡片，不允许用假趋势图提前包装 Phase 1 能力。
4. 所有契约缺口都要记录接口名、字段名、影响页面和是否阻塞。

---

## 8. 阶段验收模板（执行后填写）

1. 功能完成情况（按 L0-L8）：
2. 已完成页面：
3. 已抽离组件：
4. 已接入 API：
5. 已展示字段：
6. 契约缺口：
   - 接口：
   - 字段：
   - 影响页面：
   - 是否阻塞：
   - 负责人：
7. 冒烟测试结果：
8. 回归测试结果：
9. 页面状态体验检查：
   - loading：
   - empty：
   - error：
   - permission：
10. 回滚预案是否演练：
11. 是否影响现有知识库上传/检索：
12. 是否可以进入 Phase 1：

---

## 9. Phase 0 完成后下一步（明确路线衔接）

Phase 0 完成后固定进入 Phase 1：RAG 监控总览与结构化日志可视化。

交给 Phase 1 的稳定基础必须包含：

1. 可复用 Layout：`Header/Sider/Breadcrumb/内容区`。
2. 可复用知识库上下文：当前 `kb_id`、知识库选择器、知识库基础信息。
3. 可复用 API client：知识库、文档、任务、检索业务域方法。
4. 可复用结果展示：`RetrieveTestPanel`、`CitationCard`、契约缺口提示。
5. `/dashboard` 页面骨架和最小状态卡片。
6. `request_id` 复制入口。
7. `/trace-logs/retrieval` 与 `/trace-logs/ingest` 的导航承接位。
8. 后端待补 API 与字段清单：
   - `GET /api/admin/kb/metrics/overview`
   - `GET /api/admin/kb/metrics/ingest`
   - `GET /api/admin/kb/metrics/retrieval`
   - `GET /api/admin/kb/logs/retrieval`
   - `GET /api/admin/kb/logs/retrieval/{request_id}`
   - `GET /api/admin/kb/logs/ingest`
   - `GET /api/admin/kb/logs/ingest/{job_id}`

---

## 10. 文档维护规则（作为后续实现参考基线）

1. Phase 0 范围变更，先更新本文档，再改代码。
2. 新增页面、组件、接口或字段，必须同步更新对应 L 路线。
3. 检索结果字段命名必须保持 `request_id/items/content/score/citation/source`，不要在页面中另起一套术语。
4. 每次联调后补充“阶段验收模板”中的契约缺口和测试结果。
5. P0 不承诺 Phase 1 的监控指标，不用假数据提前包装趋势图。
6. 后续实现时，以本文档作为 Phase 0 前端拆分和验收基线。
