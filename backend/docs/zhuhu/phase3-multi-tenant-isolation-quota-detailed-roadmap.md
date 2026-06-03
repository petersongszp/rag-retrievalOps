# Phase 3 详细功能实现路线（多租户强隔离 + 基础配额）

## 1. 文档定位

本文档是多租户平台改造 Phase 3 的执行手册，目标是把“多租户强隔离 + 基础配额”拆成可直接实施、可验收、可回滚的细颗粒任务路线。

它有三个用途：

1. 作为团队推进数据库隔离、知识库授权、检索权限、向量元数据隔离、日志隔离与基础配额的统一执行文档。
2. 作为 Phase 4 Admin UI 闭环前的安全 Gate，确保不同租户之间不能读取、检索、删除或查看彼此的数据。
3. 作为 Phase 5 商业化和企业安全能力的资源用量基线，确保配额、审计、计费预留字段来自可信的 `tenant_id` 维度。

本文档风格与现有路线文档保持一致：
目标 -> 路线 -> 模块任务 -> 验收门禁 -> 下一步入口。

统一口径说明：

1. `多租户强隔离` 固定指：数据库查询、向量检索、检索日志、审计日志、任务数据和配额统计都以 `tenant_id` 为第一隔离维度。
2. `租户身份基线` 固定指 Phase 2 已稳定的 `auth.Identity`：`auth_type/tenant_id/user_id/role/app_id/api_key_id/permissions/is_legacy`。
3. `Repository 强制过滤` 固定指：列表、详情、更新、删除、上传、日志查询等所有数据访问都必须追加 `tenant_id` 条件，不能只依赖 handler 传参。
4. `知识库授权` 固定指：通过 `rag_tenant_kb_permission` 维护 `tenant_id -> kb_id -> permission`，并在管理 API 与 `/v1/retrieve` 前置校验。
5. `向量元数据隔离` 固定指：入库 chunk metadata 写入 `tenant_id/kb_id/document_id`，Milvus 检索 expr 强制包含当前租户过滤条件。
6. `基础配额` 固定指：先实现每日 API 调用数、最大知识库数、最大文档数、存储占用的拦截和统计，不做支付、账单、套餐升级。
7. `legacy app_id` 在 Phase 3 固定指可观测的兼容路径；必须映射到系统租户或测试租户，不能绕过租户隔离。
8. `Phase 3 隔离回归` 固定指：构造租户 A/B，验证跨租户读取、检索、删除、日志查询、配额影响全部失败或互不影响。

---

## 2. Phase 3 范围边界

## 2.1 本阶段必须完成

1. 给知识库、文档、入库任务、操作日志、审计日志、检索日志补齐或确认 `tenant_id` 字段。
2. 将历史 `user_id` 维度知识库迁移到默认系统租户、测试租户或对应真实租户，并输出可复跑迁移脚本。
3. Repository、Service、Handler 全链路按 `tenant_id` 强制过滤，避免只在 UI 或 handler 层做软限制。
4. 创建并接入 `rag_tenant_kb_permission`，支持租户对知识库的 `read/write/admin` 最小授权模型。
5. 改造 `/v1/retrieve`：检索前校验请求 `kb_ids` 是否属于当前租户或 API Key 权限范围。
6. 入库 chunk metadata 写入 `tenant_id/kb_id/document_id`，检索 expr 强制拼接 `metadata["tenant_id"] == <tenant_id>` 与 `metadata["kb_id"]` 条件。
7. 实现基础配额：每日 API 调用数、最大知识库数、最大文档数、存储占用统计与超限拦截。
8. 审计日志、检索日志、调试视图、用量统计都只能查询当前租户数据。
9. 构造租户 A/B 隔离测试，覆盖 DB、Milvus、日志、配额和 legacy 回退路径。
10. 完成灰度、回滚演练与 Phase 3 验收记录，证明可进入 Phase 4 Admin UI 闭环。

## 2.2 本阶段明确不做

1. 不做套餐购买、支付、账单、欠费停用和发票能力，该能力属于 Phase 5。
2. 不做 OAuth、SSO、IP 白名单、Webhook、企业审计导出和复杂成员邀请流程。
3. 不做完整 AB 实验平台；只保留隔离策略灰度和回滚开关。
4. 不下线 legacy `app_id` 白名单；Phase 3 只要求 legacy 路径必须映射租户并可观测。
5. 不做跨区域多活、向量库灾备和索引生命周期自动治理。
6. 不允许用前端隐藏按钮代替后端 `tenant_id` 和权限校验。
7. 不允许为了兼容旧数据在生产环境放开“无 `tenant_id` 查询所有数据”的路径。
8. 不重写 Phase 2 API Key 鉴权链路；Phase 3 只消费已冻结的 `auth.Identity`。

---

## 3. 目标与通过标准（Gate）

Phase 3 通过标准（全满足）：

1. 现有核心表具备可信 `tenant_id`：`kb_knowledge_base`、`kb_document`、`kb_ingest_job`、`kb_job_operation_log`、`kb_audit_event`、`kb_retrieve_log`。
2. 租户 A 即使知道租户 B 的 `kb_id/document_id/request_id`，也无法读取、更新、删除、检索或查看日志。
3. `rag_tenant_kb_permission` 可表达租户对知识库的 `read/write/admin` 权限，且管理端和检索端均使用同一套校验逻辑。
4. `/v1/retrieve` 在 API Key 路径下必须从 `auth.Identity.tenant_id` 推导租户，不接受请求体覆盖 `tenant_id`。
5. Milvus 最终检索 expr 可在日志或调试输出中看到 `metadata["tenant_id"] == <tenant_id>`，且不被 `metadata_filter` 覆盖。
6. API Key `permissions.kb_ids` 与 `rag_tenant_kb_permission` 同时生效，请求的 `kb_ids` 必须是两者允许集合的交集。
7. 配额超限返回 `429 quota_exceeded`，错误响应包含 `quota_type/current/limit/reset_at` 或等价字段，且不影响其他租户。
8. 检索日志、审计日志、用量统计按 `tenant_id` 过滤，无法跨租户枚举。
9. legacy `app_id` 调用被标记为 `auth_type=legacy_app_id/is_legacy=true`，并映射到明确租户，不得进入无租户检索。
10. Phase 3 隔离回归、配额回归、Milvus expr 回归和回滚演练全部通过。

---

## 4. 实现路线总览（L0 -> L8）

Phase 3 按 9 条路线推进，按门禁顺序合流：

1. L0：Phase 2 身份基线冻结、隔离契约与风险清单复核
2. L1：`tenant_id` schema 补齐、历史数据迁移与权限表迁移
3. L2：Repository/Service 强制租户过滤与管理 API 改造
4. L3：知识库授权服务与 `rag_tenant_kb_permission` 接入
5. L4：`/v1/retrieve` 租户权限门禁与 API Key permissions 收敛
6. L5：Milvus chunk metadata 隔离、检索 expr 强制过滤与旧索引兼容
7. L6：基础配额计数、拦截、错误码与降级策略
8. L7：日志隔离、审计、可观测性与自动化隔离回归
9. L8：灰度发布、回滚预案与 Phase 3 验收收口

建议顺序：`L0 -> L1 -> L2 + L3 -> L4 + L5 -> L6 + L7 -> L8`

---

## 5. 详细路线拆解

## 5.1 L0 Phase 2 身份基线冻结、隔离契约与风险清单复核

### 目标

在开始强隔离前确认 Phase 2 的 API Key、JWT、legacy 回退和统一身份上下文已经稳定，避免后续用临时字段绕过 `tenant_id`。

### 功能任务

1. 确认 Phase 2 交接物可用：
   - `backend/docs/zhuhu/phase2-acceptance-record.md`
   - `backend/internal/auth/context.go`
   - `backend/internal/auth/contract.go`
   - `backend/internal/auth/apikey.go`
   - `backend/internal/repository/rag_api_key_repo.go`
   - `backend/api/handler/auth/apikey.go`
   - `backend/api/handler/rag/retrieve.go`
2. 冻结身份来源优先级：
   - JWT 用于 Admin/UI
   - API Key 用于 Agent/SDK
   - legacy `app_id` 只作为兼容路径
   - dev bypass 只允许 `dev/test` 环境
3. 冻结 Phase 3 隔离字段：
   - `tenant_id`
   - `kb_id`
   - `document_id`
   - `api_key_id`
   - `app_id`
   - `auth_type`
   - `source_api`
   - `is_legacy`
4. 冻结需要改造的数据表清单：
   - `kb_knowledge_base`
   - `kb_document`
   - `kb_ingest_job`
   - `kb_job_operation_log`
   - `kb_audit_event`
   - `kb_retrieve_log`
   - `rag_tenant_kb_permission`
5. 冻结需要强过滤的接口类型：
   - 知识库列表、详情、创建、删除
   - 文档列表、上传、删除
   - 入库任务查询与操作日志
   - 检索日志和审计日志
   - `/v1/retrieve`
   - 调试视图和评测入口
6. 输出风险清单：
   - 只按 `user_id` 过滤导致同租户成员不可协作或跨租户误读
   - `kb_id` 被猜中后可读取详情
   - Milvus expr 只有 `kb_id` 没有 `tenant_id`
   - legacy `app_id` 无租户身份
   - 配额统计失败导致误封或放飞

### 验收

1. Phase 3 开始前只有一份 `tenant_id` 来源口径。
2. 所有待补 `tenant_id` 表和接口有明确清单。
3. legacy 路径的租户映射策略已冻结。
4. 后续实现不得新增“请求体传 `tenant_id` 即可信”的临时方案。

---

## 5.2 L1 `tenant_id` schema 补齐、历史数据迁移与权限表迁移

### 目标

把多租户隔离所需的数据结构落到数据库，完成历史数据回填，确保新旧数据都能进入统一租户模型。

### 功能任务

1. 创建 Phase 3 迁移文件：
   - `backend/migrations/004_add_tenant_id_to_kb_tables.up.sql`
   - `backend/migrations/004_add_tenant_id_to_kb_tables.down.sql`
   - `backend/migrations/005_create_rag_tenant_kb_permission.up.sql`
   - `backend/migrations/005_create_rag_tenant_kb_permission.down.sql`
2. 给核心表补 `tenant_id`：
   - `kb_knowledge_base.tenant_id`
   - `kb_document.tenant_id`
   - `kb_ingest_job.tenant_id`
   - `kb_job_operation_log.tenant_id`
   - `kb_audit_event.tenant_id`
   - `kb_retrieve_log.tenant_id` 已存在时确认索引和回填策略
3. 增加建议索引：
   - `idx_kb_tenant_status (tenant_id, status)`
   - `idx_doc_tenant_kb_deleted (tenant_id, kb_id, deleted)`
   - `idx_job_tenant_kb_status (tenant_id, kb_id, status)`
   - `idx_retrieve_tenant_created (tenant_id, created_at)`
   - `idx_audit_tenant_created (tenant_id, created_at)`
4. 创建 `rag_tenant_kb_permission`：
   - `id`
   - `tenant_id`
   - `kb_id`
   - `permission`
   - `created_at`
   - `updated_at`
   - `UNIQUE KEY uk_tenant_kb (tenant_id, kb_id)`
5. 历史数据迁移：
   - 创建或确认 `SYSTEM_TENANT_ID`
   - 将无法归属的历史知识库迁入系统租户或测试租户
   - 能通过 `user_id -> rag_user.tenant_id` 归属的数据按真实租户回填
   - 同步给 owner 租户写入 `rag_tenant_kb_permission(permission=admin)`
6. 模型更新：
   - `backend/internal/model/kb_knowledge_base.go`
   - `backend/internal/model/kb_document.go`
   - `backend/internal/model/kb_ingest_job.go`
   - `backend/internal/model/kb_job_operation_log.go`
   - `backend/internal/model/kb_audit_event.go`
   - `backend/internal/model/kb_retrieve_log.go`
   - 新增 `backend/internal/model/rag_tenant_kb_permission.go`
7. 迁移安全：
   - up 迁移先 nullable 回填，再视情况改为 not null
   - down 迁移只在测试库演练，生产回滚以关闭强隔离开关为主
   - 迁移输出回填数量、失败数量和未归属样本

### 验收

1. 新表和新字段可在 Phase 2 数据库基础上迁移成功。
2. 历史知识库、文档、任务、日志均有明确 `tenant_id`。
3. `rag_tenant_kb_permission` 能查询到每个已迁移知识库的 owner 租户授权。
4. 不存在生产查询需要靠 `tenant_id IS NULL` 才能返回数据。
5. 迁移报告记录回填规则、异常样本和人工处理清单。

---

## 5.3 L2 Repository/Service 强制租户过滤与管理 API 改造

### 目标

把隔离规则下沉到 Repository/Service 层，让任何管理 API 和内部调用都不能绕过 `tenant_id` 读取其他租户数据。

### 功能任务

1. 改造知识库仓储：
   - `ListByTenant`
   - `GetByIDForTenant`
   - `CreateForTenant`
   - `UpdateByIDForTenant`
   - `DeleteByIDForTenant`
   - `CountByTenant`
2. 改造文档仓储：
   - `ListByKBIDForTenant`
   - `GetByIDForTenant`
   - `GetByFileHashForTenant`
   - `SoftDeleteForTenant`
   - `CountByTenant`
   - `SumStorageByTenant`
3. 改造任务和日志仓储：
   - 入库任务按 `tenant_id/kb_id` 查询
   - 操作日志按 `tenant_id` 查询
   - 审计日志按 `tenant_id` 查询
   - 检索日志按 `tenant_id` 查询
4. 改造重点文件：
   - `backend/internal/ragplatform/repository/kb_repository.go`
   - `backend/internal/ragplatform/repository/retrieve_log_repository.go`
   - `backend/internal/ragplatform/application/kb_service.go`
   - `backend/internal/ragplatform/application/retrieve_service.go`
   - `backend/api/handler/kb/handler.go`
   - `backend/api/handler/kb/knowledge_base_binding.go`
5. Handler 读取统一身份：
   - 只从 `auth.GetIdentity(ctx)` 或 middleware 注入值读取 `tenant_id`
   - 请求体、query string、path 中的 `tenant_id` 不作为可信来源
   - `tenant_id=0` 直接返回 `401 unauthorized` 或 `403 forbidden`
6. 错误码策略：
   - 无身份：`401 unauthorized`
   - 无租户：`403 tenant_required`
   - 跨租户访问：优先返回 `404 not_found`，避免泄露资源存在性
   - 明确权限不足：`403 forbidden`
7. 单元测试：
   - 租户 A 只能列出自己的知识库
   - 租户 A 查询租户 B 的 `kb_id` 返回 404
   - 租户 A 删除租户 B 的文档失败
   - 管理日志不能跨租户分页枚举

### 验收

1. 所有管理端数据查询至少在 Repository 层包含 `tenant_id`。
2. 直接调用 Service 也不能绕过租户过滤。
3. handler 不信任请求体或 URL 中的 `tenant_id`。
4. 跨租户详情、删除、日志查询返回 404 或 403，不泄露敏感信息。
5. 代码审查清单覆盖所有 `Where("id = ?")`、`Where("kb_id = ?")` 和 `ListByIDs` 风险点。

---

## 5.4 L3 知识库授权服务与 `rag_tenant_kb_permission` 接入

### 目标

建立统一知识库授权入口，让租户对知识库的读写权限可查询、可审计、可复用到管理 API 和 Agent 检索。

### 功能任务

1. 新增模型与仓储：
   - `backend/internal/model/rag_tenant_kb_permission.go`
   - `backend/internal/repository/rag_tenant_kb_permission_repo.go`
2. 仓储方法：
   - `GrantTenantKB`
   - `RevokeTenantKB`
   - `GetPermission`
   - `ListKBIDsByTenant`
   - `CheckTenantKBPermission`
   - `ListTenantsByKB`
3. 权限枚举：
   - `read`：可查看、检索
   - `write`：可上传文档、触发入库
   - `admin`：可删除知识库、授权、修改设置
4. 授权规则：
   - 租户创建知识库时自动写入 `admin`
   - owner/admin 可管理本租户授权
   - member 默认只能使用已授权知识库
   - API Key 还要叠加 `permissions.kb_ids`
5. API 与审计：
   - 可先不开放复杂授权页面
   - 内部服务必须支持授权写入和撤销
   - 审计事件：`kb.grant`、`kb.revoke`、`kb.permission_denied`
6. 管理 API 接入：
   - 知识库详情需要 `read`
   - 上传文档需要 `write/admin`
   - 删除知识库需要 `admin`
   - 日志查看需要 `read`
7. 降级策略：
   - 权限表缺失授权时，不能默认放行
   - 迁移期仅允许对 `SYSTEM_TENANT_ID` 或明确测试租户按配置兜底
   - 兜底必须记录 `permission_fallback=true`

### 验收

1. 新创建知识库自动拥有创建租户的 `admin` 授权。
2. 租户只能看到已授权知识库。
3. 权限不足时上传、删除、授权操作被拒绝。
4. API Key 的 `kb_ids` 不能扩大租户授权，只能收窄可访问范围。
5. 授权变更有审计记录，可追踪操作者和资源。

---

## 5.5 L4 `/v1/retrieve` 租户权限门禁与 API Key permissions 收敛

### 目标

把公共检索入口从“知道 `kb_id` 即可尝试检索”升级为“身份租户 + 知识库授权 + API Key 权限”共同决定可检索范围。

### 功能任务

1. 改造入口文件：
   - `backend/api/handler/rag/retrieve.go`
   - `backend/api/handler/kb/handler.go`
   - `backend/internal/ragplatform/application/retrieve_service.go`
2. 统一解析请求知识库：
   - `kb_id`
   - `kb_ids`
   - 默认知识库配置
   - metadata filter 中不得传入可覆盖租户的 `tenant_id`
3. 权限校验顺序：
   - 读取 `auth.Identity`
   - 校验 `tenant_id > 0`
   - 解析目标 `kb_ids`
   - 校验 `rag_tenant_kb_permission` 至少具备 `read`
   - 校验 API Key `permissions` 包含 `retrieve`
   - 如 API Key `permissions.kb_ids` 非空，请求 `kb_ids` 必须是其子集
4. legacy 路径处理：
   - legacy `app_id` 必须映射到 `SYSTEM_TENANT_ID` 或明确租户
   - legacy 请求不能检索未授权 `kb_ids`
   - 日志记录 `auth_type=legacy_app_id/is_legacy=true/deprecated=true`
5. 响应与错误：
   - 未认证：`401 unauthorized`
   - 租户停用：`403 tenant_suspended`
   - 无知识库权限：`403 forbidden` 或 `404 not_found`
   - API Key 无 retrieve 权限：`403 forbidden`
   - `kb_ids` 超出 permissions：`403 forbidden`
6. 调用日志补齐：
   - `tenant_id`
   - `app_id`
   - `api_key_id`
   - `auth_type`
   - `source_api=v1`
   - `is_legacy`
   - `permission_result`
7. 回归测试：
   - 租户 A 检索自己的知识库成功
   - 租户 A 检索租户 B 的 `kb_id` 失败
   - API Key `kb_ids=[1]` 请求 `kb_ids=[1,2]` 失败
   - legacy 未映射租户失败
   - metadata filter 试图传 `tenant_id` 被拒绝或忽略

### 验收

1. `/v1/retrieve` 不接受请求体覆盖租户身份。
2. `kb_ids` 必须同时满足租户授权与 API Key permissions。
3. 错误 API Key 不会降级到 legacy。
4. legacy 路径也必须具备明确租户和授权范围。
5. 检索日志能还原一次权限判定结果。

---

## 5.6 L5 Milvus chunk metadata 隔离、检索 expr 强制过滤与旧索引兼容

### 目标

确保向量库层面的召回结果也按租户隔离，避免数据库层已经隔离但 Milvus 返回其他租户 chunk。

### 功能任务

1. 扩展文档元数据：
   - 当前重点文件：`backend/internal/milvus/document_metadata.go`
   - `tenant_id`
   - `kb_id`
   - `document_id`
   - `chunk_id`
   - `operator_admin_id`
2. 入库链路写入 `tenant_id`：
   - 当前重点文件：`backend/internal/ragqueue/consumer.go`
   - 从 `kb_id` 查询知识库租户
   - 生成 `DocumentMetadata` 时写入 `tenant_id/kb_id/document_id`
   - 入库日志记录 `tenant_id`
3. 检索配置扩展：
   - `backend/internal/milvus/retrieval/options.go`
   - `RetrieveOptions.TenantID`
   - `RetrieveOptions.AllowedKBIDs`
   - `RetrieveOptions.ForbidTenantOverride`
4. expr 构建改造：
   - 当前重点文件：`backend/internal/milvus/retrieval/filter.go`
   - 强制拼接 `metadata["tenant_id"] == <tenant_id>`
   - 对请求 `kb_ids` 拼接 `metadata["kb_id"] in [...]` 或等价表达式
   - `metadata_filter` 不允许覆盖 `tenant_id/kb_id`
   - 自定义 `Expr` 必须通过安全合并，不允许直接替代租户过滤
5. 旧索引兼容：
   - 无 `tenant_id` metadata 的 chunk 默认不可被生产租户检索
   - 只允许迁移任务或测试开关读取旧索引
   - 提供重建或回填计划，让旧 chunk 写入 `tenant_id`
6. 调试与日志：
   - 输出最终 expr
   - 输出 `tenant_filter_applied=true`
   - 输出 `kb_filter_applied=true`
   - 输出 `legacy_metadata_fallback=false/true`
7. Milvus 回归：
   - 同一 query 下租户 A 不返回租户 B chunk
   - 只传 `metadata_filter` 不能绕过租户过滤
   - 旧 chunk 不会在生产强隔离路径泄露
   - `kb_id` 过滤和 `tenant_id` 过滤同时存在

### 验收

1. 新入库 chunk metadata 包含 `tenant_id/kb_id/document_id`。
2. Milvus 最终 expr 可观测且包含当前租户过滤。
3. 自定义 metadata filter 无法覆盖或删除租户过滤。
4. 无 `tenant_id` 的旧 chunk 不会被正式租户检索到。
5. 租户 A/B 向量检索隔离测试稳定通过。

---

## 5.7 L6 基础配额计数、拦截、错误码与降级策略

### 目标

实现第一版资源保护，让租户按套餐字段受到基础用量限制，同时为后续计费和套餐升级保留可信统计口径。

### 功能任务

1. 配额来源：
   - `rag_tenant.max_kb_count`
   - `rag_tenant.max_doc_count`
   - `rag_tenant.max_storage_mb`
   - `rag_tenant.max_api_calls_per_day`
2. 新增配额模块：
   - `backend/internal/quota/checker.go`
   - `backend/internal/quota/counter.go`
   - `backend/internal/quota/contract.go`
   - 或按现有目录放入 `backend/internal/ragplatform/application`
3. 知识库数量配额：
   - 创建知识库前统计 `CountKBByTenant`
   - 超限返回 `429 quota_exceeded`
   - 删除或禁用知识库后释放占用
4. 文档数量与存储配额：
   - 上传文档前统计当前非删除文档数
   - 统计 `file_size` 汇总为 `storage_mb`
   - 入库失败不应重复占用文档数
   - 软删除后从有效占用中排除
5. API 调用配额：
   - `/v1/retrieve` 成功进入鉴权后计数
   - 建议按 `tenant_id + date` 计数
   - 可以先使用 Redis 计数，必要时落库汇总
   - 计数失败时可配置为 fail-open 或 fail-closed，默认测试环境 fail-open、生产环境按配置
6. 错误响应：
   - `error=quota_exceeded`
   - `quota_type=kb_count/doc_count/storage_mb/api_calls_per_day`
   - `current`
   - `limit`
   - `reset_at`
7. 配额日志与指标：
   - `quota_check_result`
   - `quota_type`
   - `tenant_id`
   - `app_id`
   - `api_key_id`
   - `quota_counter_error`
8. 降级策略：
   - 只关闭配额拦截，不关闭用量记录
   - 配额计数异常不得影响租户隔离
   - 配额错误不会回退到 legacy 或无租户路径

### 验收

1. 创建知识库超限返回 `429 quota_exceeded`。
2. 上传文档超出最大文档数或存储限制返回 `429 quota_exceeded`。
3. 每日 API 调用超限后 `/v1/retrieve` 被拦截。
4. 租户 A 超限不影响租户 B。
5. 关闭配额拦截后仍记录用量，且隔离逻辑不受影响。

---

## 5.8 L7 日志隔离、审计、可观测性与自动化隔离回归

### 目标

把 Phase 3 强隔离固化成可观测、可回放、可持续回归的门禁，确保后续 UI 和商业化不会建立在松散隔离上。

### 功能任务

1. 检索日志隔离：
   - `KBRetrieveLogListFilter` 增加 `TenantID`
   - `ListWithFilter` 强制按 `tenant_id` 查询
   - `GetByRequestID` 提供 `GetByRequestIDForTenant`
2. 审计日志隔离：
   - `kb_audit_event` 补 `tenant_id`
   - 查询 API 必须按 `tenant_id` 过滤
   - 审计详情不能跨租户读取
3. 调试视图隔离：
   - `backend/api/handler/kb/retrieval_debug_trace_v2.go`
   - request_id 查询前校验当前租户
   - debug trace 不泄露其他租户 chunk 或 expr
4. 指标补齐：
   - `tenant_isolation_denied_total`
   - `tenant_permission_denied_total`
   - `quota_exceeded_total`
   - `legacy_tenant_mapping_total`
   - `milvus_tenant_filter_missing_total`
5. 自动化测试分组：
   - DB 隔离测试
   - KB 授权测试
   - `/v1/retrieve` 权限测试
   - Milvus expr 构造测试
   - 日志隔离测试
   - 配额隔离测试
   - legacy 映射测试
6. 测试夹具：
   - 租户 A：owner/admin/member/viewer
   - 租户 B：owner/admin/member/viewer
   - A 专属知识库
   - B 专属知识库
   - 系统租户 legacy 知识库
   - API Key A1/A2/B1，分别配置不同 `kb_ids`
7. 回归命令建议：
   - `go test ./internal/auth ./internal/repository ./internal/ragplatform/...`
   - `go test ./api/...`
   - `go test ./internal/milvus/retrieval`
   - 本地 curl smoke：A/B 互访、配额超限、legacy 映射

### 验收

1. 日志列表和详情都不能跨租户查询。
2. 调试视图不能通过 `request_id` 读取其他租户链路。
3. 自动化测试能稳定覆盖 Phase 3 Gate。
4. 监控指标能区分隔离拒绝、权限拒绝、配额拒绝和 legacy 映射。
5. Phase 3 回归报告可作为进入 Phase 4 的 Gate 材料。

---

## 5.9 L8 灰度发布、回滚预案与 Phase 3 验收收口

### 目标

确保强隔离与基础配额在真实流量中安全上线，出现异常时能回滚拦截策略，但不能回滚到跨租户可读状态。

### 功能任务

1. 灰度顺序：
   - 本地构造租户 A/B 完整跑通
   - 测试环境开启 DB 强过滤
   - 测试环境开启 Milvus tenant expr
   - staging 对测试租户开启配额拦截
   - 小流量真实 Agent 开启强隔离
   - 全量启用日志隔离和审计隔离
2. 回滚顺序：
   - 暂停配额拦截，保留用量记录
   - 回退新建知识库入口，保留已有授权表
   - 暂停 legacy 迁移扩大，保留已映射租户
   - 对 Milvus 过滤异常只允许回滚到“无结果或 child-only 安全路径”，不允许无租户检索
   - DB 强过滤不得整体关闭，除非仅在测试环境排障
3. 不能回滚的安全底线：
   - 不允许生产环境 `tenant_id=0` 读取业务数据
   - 不允许 `/v1/retrieve` 不带租户身份检索
   - 不允许 metadata filter 覆盖 `tenant_id`
   - 不允许错误 API Key 降级 legacy
   - 不允许日志详情跨租户读取
4. 告警与观察：
   - `tenant_required` 异常升高
   - `forbidden` 异常升高
   - `quota_exceeded` 异常升高
   - Milvus 空结果率异常升高
   - legacy 调用未映射租户
   - 检索 P95 延迟异常
5. 验收材料：
   - 迁移执行记录
   - A/B 租户隔离测试结果
   - Milvus expr 截图或日志样本
   - 配额 smoke 结果
   - 日志/审计隔离验证
   - 回滚演练记录
6. Phase 4 交接：
   - UI 可直接读取租户内知识库、API Key、用量和日志
   - Owner 可以安全展示配额
   - 接入文档可以明确 API Key 与 `kb_ids` 权限关系

### 验收

1. 灰度过程中未出现跨租户数据读取或检索泄露。
2. 配额异常可关闭拦截但保留用量记录。
3. 回滚演练没有破坏 `tenant_id` 强隔离底线。
4. 所有 Phase 3 验收材料齐全。
5. Phase 4 可以直接开始 Admin UI 闭环和接入文档。

---

## 6. 推荐实施节奏（1-2 周）

## 6.1 阶段推进建议

1. 第 0.5-1 天完成 `L0`，冻结身份基线、待隔离表、接口清单和 legacy 租户映射。
2. 第 1-3 天完成 `L1`，落地 `tenant_id` 迁移、历史回填和 `rag_tenant_kb_permission`。
3. 第 3-5 天完成 `L2 + L3`，把 DB 层和管理 API 的强过滤、知识库授权打通。
4. 第 5-7 天完成 `L4 + L5`，打通 `/v1/retrieve` 权限门禁与 Milvus tenant expr。
5. 第 7-9 天完成 `L6 + L7`，补齐配额、日志隔离、审计、监控和自动化回归。
6. 第 9-10 天完成 `L8`，执行灰度、回滚演练和 Phase 3 验收记录。

## 6.2 并行与合流规则

1. 可并行：`L1` 迁移脚本与 `L3` 授权服务接口设计，`L2` Repository 改造与 `L7` 测试夹具设计，`L6` 配额 contract 与 `L4` 检索权限用例设计。
2. 必须串行：`L2` 依赖 `L1` 字段稳定，`L4` 依赖 `L3` 授权服务，`L5` 依赖 `L4` 传入可信 `tenant_id/kb_ids`，`L8` 依赖 `L1~L7` 全部通过。
3. 合流条件：DB 强过滤、KB 授权、检索权限、Milvus expr、日志隔离和配额回归全部通过后，才允许进入 Phase 4。
4. Code review 重点：任何 `GetByID`、`ListByIDs`、`Where("kb_id = ?")`、自定义 `Expr`、日志详情查询都必须检查租户过滤。

---

## 7. 角色分工（建议）

1. 后端 A：`L1 + L2`，负责迁移、模型、Repository 强过滤、管理 API 租户隔离。
2. 后端 B：`L3 + L4`，负责知识库授权服务、`/v1/retrieve` 权限门禁、API Key permissions 收敛。
3. 后端 C：`L5 + L7`，负责 Milvus metadata/expr、日志隔离、调试视图隔离和自动化回归。
4. 平台/SRE：`L6 + L8`，负责配额计数、限额拦截、灰度、告警和回滚演练。
5. QA/文档：`L0 + 阶段验收`，负责隔离用例、curl smoke、验收记录和 Phase 4 交接材料。

补充协作约束：

1. 后端与 QA 先冻结 A/B 租户测试夹具，再开始大规模改 Repository。
2. 后端与 SRE 先冻结配额 fail-open/fail-closed 规则，再启用生产拦截。
3. Milvus expr 改造必须和 `/v1/retrieve` 权限门禁一起合流，不能先上线只有 DB 隔离的半套方案。
4. legacy 路径的任何放行都必须记录 `auth_type=legacy_app_id`、`tenant_id` 和 `is_legacy=true`。

---

## 8. 阶段验收模板（执行后填写）

1. 功能完成情况（按 L0-L8）：
2. 数据库迁移结果：
   - `kb_knowledge_base.tenant_id`
   - `kb_document.tenant_id`
   - `kb_ingest_job.tenant_id`
   - `kb_job_operation_log.tenant_id`
   - `kb_audit_event.tenant_id`
   - `kb_retrieve_log.tenant_id`
   - `rag_tenant_kb_permission`
3. 历史数据回填结果：
   - 回填总数
   - 系统租户数量
   - 真实租户数量
   - 未归属样本
   - 人工处理清单
4. Repository 强过滤验证：
   - 知识库列表/详情/删除
   - 文档列表/上传/删除
   - 入库任务
   - 审计日志
   - 检索日志
5. 知识库授权验证：
   - `read`
   - `write`
   - `admin`
   - grant/revoke 审计
   - API Key `kb_ids` 收窄
6. `/v1/retrieve` 验证：
   - 租户内检索成功
   - 跨租户 `kb_id` 失败
   - API Key permissions 拒绝
   - metadata filter 覆盖租户失败
   - legacy 租户映射成功
7. Milvus 隔离验证：
   - 入库 metadata 包含 `tenant_id`
   - 最终 expr 包含 `tenant_id`
   - A/B 租户互不返回 chunk
   - 旧 chunk 兼容策略
8. 配额验证：
   - 最大知识库数
   - 最大文档数
   - 最大存储
   - 每日 API 调用
   - 租户间互不影响
9. 日志与审计验证：
   - 检索日志按租户过滤
   - 审计日志按租户过滤
   - 调试视图按租户过滤
   - request_id 跨租户不可读
10. 自动化测试结果：
   - repository
   - handler
   - retrieve
   - milvus/retrieval
   - quota
   - legacy
11. 灰度与回滚演练结果：
   - 配额拦截关闭
   - legacy 映射回退
   - Milvus 安全降级
   - DB 强过滤底线检查
12. 遗留风险与负责人：
13. 是否允许进入 Phase 4（是/否）：

---

## 9. Phase 3 完成后下一步（明确路线衔接）

下一阶段固定进入 Phase 4：Admin UI 闭环 + 接入文档，按以下顺序衔接：

1. 登录注册页面接入真实 JWT，展示当前租户信息。
2. 知识库页面只展示当前租户授权知识库。
3. 文档上传页面接入写权限和文档/存储配额提示。
4. API Key 页面展示 `app_id/key_prefix/permissions/kb_ids/status`，支持创建、吊销、轮换。
5. 用量页面展示 `api_calls_today/kb_count/doc_count/storage_mb/limits`。
6. 检索日志页面按 `tenant_id/app_id/api_key_id` 查询，不允许跨租户 request_id。
7. 接入文档输出 cURL、Go SDK、Python requests、Agent 后端接入示例。
8. E2E 验收“注册登录 -> 创建知识库 -> 上传文档 -> 创建 API Key -> Agent 检索 -> 查看日志”闭环。

Phase 4 完成后，再进入 Phase 5 商业化与高级企业能力，围绕套餐、账单、OAuth、复杂邀请、Webhook 和企业安全增强展开。

---

## 10. 文档维护规则（作为后续实现参考基线）

1. 任何 Phase 3 范围变更，先更新本文档，再改代码。
2. 修改 `tenant_id` 迁移表、索引或历史回填规则时，必须同步更新 `L1/阶段验收模板`。
3. 修改 Repository 强过滤策略时，必须同步更新 `L2`，并补充跨租户回归用例。
4. 修改 `rag_tenant_kb_permission` 字段、权限枚举或授权规则时，必须同步更新 `L3`。
5. 修改 `/v1/retrieve` 权限顺序、API Key permissions 或 legacy 映射时，必须同步更新 `L4`。
6. 修改 Milvus metadata、filter expr 或旧索引兼容策略时，必须同步更新 `L5`。
7. 修改配额类型、错误响应、fail-open/fail-closed 策略时，必须同步更新 `L6`。
8. Phase 4 实现 Admin UI 时，以本文档的 `tenant_id` 强隔离和知识库授权规则作为唯一安全基线。
