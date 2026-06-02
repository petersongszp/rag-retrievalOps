# Phase 2 详细功能实现路线（API Key + Agent 接入 MVP）

## 1. 文档定位

本文档是多租户平台改造 Phase 2 的执行手册，目标是把“API Key + Agent 接入 MVP”拆成可直接实施、可验收、可回滚的细颗粒任务路线。

它有三个用途：

1. 作为团队推进 API Key 数据模型、管理 API、鉴权中间件、SDK 与 Agent 接入的统一执行文档。
2. 作为 Phase 3 多租户强隔离前的服务端调用身份基线，确保 `/v1/retrieve` 能从 API Key 推导 `tenant_id/app_id/api_key_id/permissions`。
3. 作为 legacy `app_id` 白名单迁移到正式 API Key 的过渡方案，保证旧调用可观测、可标记、可逐步替换。

本文档风格与现有路线文档保持一致：
目标 -> 路线 -> 模块任务 -> 验收门禁 -> 下一步入口。

统一口径说明：

1. `API Key + Agent 接入 MVP` 固定指：租户内用户可以创建、查看、更新、吊销 API Key；Agent 后端可以通过 `Authorization: Bearer rag_<key>` 调用 `/v1/retrieve`。
2. `API Key 明文` 固定指只在创建或轮换响应中返回一次的完整 Key，服务端只保存 `key_hash/key_prefix`，日志和列表接口不得输出完整 Key。
3. `API Key 身份上下文` 固定指：`auth_type=api_key/tenant_id/user_id/app_id/api_key_id/permissions/role`，并写入 Phase 0/1 冻结的 `auth.Identity`。
4. `统一认证入口` 固定指：JWT 用于 Admin/UI，API Key 用于 Agent/SDK，legacy `app_id` 仅作为兼容路径；业务 handler 优先读取统一身份上下文。
5. `legacy app_id 兼容` 固定指：当前请求体 `app_id` 或旧白名单仍可临时调用，但必须标记 `auth_type=legacy_app_id/is_legacy=true/deprecated=true`。
6. `API Key permissions` 在 Phase 2 固定指最小 JSON 权限：`retrieve`、`kb_ids`、可选 `metadata_filter` 与 `rate_limit_hint`；不做完整企业级策略模板。
7. `Agent 持 Key` 固定指：终端用户不直接持有 API Key，API Key 由 Agent 后端、服务端 SDK 或可信集成服务保存和发送。
8. `Phase 2 回归` 固定指：API Key 创建、明文一次性返回、hash 校验、吊销、过期、权限不足、SDK 检索、legacy 回退和调用日志追踪的自动化验证。

---

## 2. Phase 2 范围边界

## 2.1 本阶段必须完成

1. 创建 `rag_api_key` 数据模型与迁移，关联 Phase 1 已稳定的 `tenant_id/user_id`。
2. 实现 API Key 生成、hash 存储、prefix 展示、状态管理、过期时间与明文一次性返回。
3. 实现 `GET/POST/PUT/DELETE /v1/api-keys`，支持 Key 列表、创建、更新、吊销。
4. 实现 API Key 鉴权中间件，解析 `Authorization: Bearer rag_<key>`，校验 hash、状态、过期时间、租户状态和用户状态。
5. 将 API Key 身份写入 `auth.Identity`，与 JWT、legacy `app_id`、dev bypass 共用统一身份上下文。
6. 改造 `/v1/retrieve`：新 API Key 鉴权优先，`app_id` 从 Key 推导；旧 `app_id` 白名单作为兼容回退。
7. 补齐 Go SDK，使 Agent 调用自动带 `Authorization: Bearer <api_key>`，并明确 `AppID` 只作为兼容或日志字段。
8. 调用日志记录 `tenant_id/app_id/api_key_id/auth_type/source_api/is_legacy`，支持按具体应用追踪调用。
9. 增加 Phase 2 自动化测试、curl smoke、SDK 示例和验收记录，证明真实 Agent 可只靠 API Key 调用平台。

## 2.2 本阶段明确不做

1. 不做知识库、文档、任务、审计、检索日志所有查询的强制 `tenant_id` 过滤，该能力属于 Phase 3。
2. 不做 Milvus chunk metadata 的 `tenant_id/kb_id` 强制写入与检索 expr 租户过滤，该能力属于 Phase 3。
3. 不做基础配额拦截、计费、套餐升级和欠费停用，该能力属于 Phase 3/5。
4. 不实现复杂 API Key 权限模板、IP 白名单、Webhook、企业 SSO 和审计导出。
5. 不要求 Admin UI 完整产品闭环；Phase 2 只保证后端 API、SDK 和脚本可对接。
6. 不删除 legacy `app_id` 白名单；Phase 2 只把它降级为可追踪的兼容路径。
7. 不允许 API Key 冒充终端用户登录 Admin UI；API Key 只用于 Agent/SDK 调用。
8. 不把 Key 明文写入数据库、日志、错误响应、调用日志或 SDK debug 输出。

---

## 3. 目标与通过标准（Gate）

Phase 2 通过标准（全满足）：

1. `rag_api_key` 表可通过迁移创建，字段包含 `tenant_id/user_id/app_id/key_hash/key_prefix/name/permissions/status/last_used_at/expires_at/created_at`。
2. `POST /v1/api-keys` 创建成功时只返回一次完整 `key`，后续列表、详情、日志和数据库均无法读取完整明文。
3. `Authorization: Bearer rag_<key>` 可以完成 API Key 鉴权，并向请求上下文注入 `auth_type=api_key/tenant_id/user_id/app_id/api_key_id/permissions`。
4. 吊销、过期、无效格式、hash 不存在、租户停用、用户停用的 Key 均不能继续调用，错误码符合契约。
5. `/v1/retrieve` 在 API Key 路径下不再依赖请求体 `app_id`；如请求体仍带 `app_id`，必须以 Key 绑定的 `app_id` 为准或返回冲突错误。
6. legacy `app_id` 白名单仍可临时工作，但日志必须标记 `auth_type=legacy_app_id/is_legacy=true/deprecated=true`。
7. 检索日志能按 `tenant_id/app_id/api_key_id/source_api` 追踪一次 Agent 调用，不泄露完整 API Key。
8. Go SDK 使用 `APIKey` 可以直接跑通 `/v1/retrieve`；`AppID` 不再是正式鉴权字段。
9. Phase 2 回归测试覆盖创建、列表、更新、吊销、过期、无效 Key、权限不足、SDK 检索、legacy 兼容和日志字段。
10. Phase 3 可以直接基于 Phase 2 身份上下文做强隔离，不需要重写 API Key 鉴权链路。

---

## 4. 实现路线总览（L0 -> L8）

Phase 2 按 9 条路线推进，按门禁顺序合流：

1. L0：Phase 1 基线确认、API Key 契约冻结与 legacy 边界复核
2. L1：`rag_api_key` 数据模型、迁移与仓储层
3. L2：Key 生成、hash 存储、权限 JSON 与安全策略
4. L3：API Key 管理接口（列表、创建、更新、吊销）
5. L4：API Key 鉴权中间件与统一身份上下文接入
6. L5：`/v1/retrieve` 新认证优先、legacy 回退与权限门禁
7. L6：Go SDK、Agent 接入示例与本地脚本更新
8. L7：调用日志、审计、自动化测试与回归门禁
9. L8：灰度发布、回滚预案与 Phase 2 验收收口

建议顺序：`L0 -> L1 + L2 -> L3 + L4 -> L5 + L6 -> L7 -> L8`

---

## 5. 详细路线拆解

## 5.1 L0 Phase 1 基线确认、API Key 契约冻结与 legacy 边界复核

### 目标

在开始 API Key 实现前确认 Phase 1 的账号、租户、JWT、RBAC 和统一身份上下文已经稳定，避免 API Key 重复定义身份字段或绕过 Phase 1 权限边界。

### 功能任务

1. 确认 Phase 1 交接物可用：
   - `backend/docs/zhuhu/phase1-acceptance-record.md`
   - `backend/internal/auth/context.go`
   - `backend/internal/auth/contract.go`
   - `backend/internal/auth/rbac.go`
   - `backend/internal/auth/jwt.go`
   - `backend/internal/model/rag_tenant.go`
   - `backend/internal/model/rag_user.go`
   - `backend/internal/repository/rag_tenant_repo.go`
   - `backend/internal/repository/rag_user_repo.go`
2. 冻结 API Key API 字段：
   - `GET /v1/api-keys`
   - `POST /v1/api-keys`
   - `PUT /v1/api-keys/:id`
   - `DELETE /v1/api-keys/:id`
   - 可选预留：`POST /v1/api-keys/:id/rotate`
3. 冻结 API Key 数据字段：
   - `tenant_id`
   - `user_id`
   - `app_id`
   - `key_hash`
   - `key_prefix`
   - `name`
   - `permissions`
   - `status`
   - `last_used_at`
   - `expires_at`
4. 冻结身份上下文字段：
   - `auth_type=api_key`
   - `tenant_id`
   - `user_id`
   - `role`
   - `app_id`
   - `api_key_id`
   - `permissions`
   - `is_legacy=false`
5. 冻结 legacy 迁移边界：
   - 当前 `backend/api/handler/rag/retrieve.go` 的 `allowedAppIDs`
   - `rag.auth.legacy_app_ids` 或同类配置来源
   - legacy 请求体 `app_id` 标记为 `deprecated after Phase 2`
   - legacy 调用映射到 `SYSTEM_TENANT_ID` 或测试租户占位
6. 冻结错误码：
   - `401 invalid_api_key`
   - `401 api_key_revoked`
   - `401 api_key_expired`
   - `403 forbidden`
   - `403 tenant_suspended`
   - `409 app_id_conflict`
   - `422 invalid_permissions`
7. 输出 Phase 2 契约冻结记录：
   - API 字段
   - Key 格式
   - permissions JSON
   - 统一身份上下文
   - legacy 回退条件

### 验收

1. API Key 实现前只存在一份 API 字段口径。
2. Phase 1 的 `tenant_id/user_id/role` 可被 API Key 表引用，不需要重复建用户模型。
3. legacy `app_id` 与正式 API Key 的身份边界清楚。
4. `/v1/retrieve` 不再新增临时字段绕过 API Key 鉴权。

---

## 5.2 L1 `rag_api_key` 数据模型、迁移与仓储层

### 目标

落地可追踪、可吊销、可授权的 API Key 数据底座，为管理 API、鉴权中间件和调用日志提供统一存储。

### 功能任务

1. 创建迁移文件：
   - `backend/migrations/003_create_rag_api_key.up.sql`
   - `backend/migrations/003_create_rag_api_key.down.sql`
2. 创建 `rag_api_key` 表：
   - `id`
   - `tenant_id`
   - `user_id`
   - `app_id`
   - `key_hash`
   - `key_prefix`
   - `name`
   - `permissions`
   - `status`
   - `last_used_at`
   - `expires_at`
   - `created_at`
   - `updated_at`
3. 建议索引与约束：
   - `UNIQUE KEY uk_key_hash (key_hash)`
   - `INDEX idx_tenant_id (tenant_id)`
   - `INDEX idx_user_id (user_id)`
   - `INDEX idx_app_id (app_id)`
   - `INDEX idx_status (status)`
   - 可选：`INDEX idx_tenant_app (tenant_id, app_id)`
4. 新增模型：
   - `backend/internal/model/rag_api_key.go`
   - status 枚举：`active/revoked`
   - permissions 使用 JSON 字段或序列化结构体
5. 新增仓储：
   - `backend/internal/repository/rag_api_key_repo.go`
   - `CreateAPIKey`
   - `ListAPIKeysByTenant`
   - `GetAPIKeyByIDForTenant`
   - `GetAPIKeyByHash`
   - `UpdateAPIKey`
   - `RevokeAPIKey`
   - `UpdateLastUsedAt`
6. 仓储查询边界：
   - 管理接口按 `tenant_id` 查询
   - 鉴权接口按 `key_hash` 查询
   - 更新和吊销必须同时匹配 `tenant_id/id`
   - 不返回 Key 明文
7. 事务边界：
   - 创建 Key 与审计日志尽量同事务提交
   - 吊销 Key 与审计日志尽量同事务提交
   - `last_used_at` 更新失败不应阻断成功鉴权，但必须记录降级日志

### 验收

1. 迁移可在 Phase 1 数据库基础上创建 `rag_api_key`。
2. down 迁移可在测试库回滚 Phase 2 新表。
3. 数据库中只保存 `key_hash/key_prefix`，没有完整 API Key 明文。
4. 管理接口的仓储查询无法跨租户读取其他租户 Key。
5. 鉴权路径可通过 `key_hash` 快速定位 Key 记录。

---

## 5.3 L2 Key 生成、hash 存储、权限 JSON 与安全策略

### 目标

把 API Key 做成真正的服务端凭证：足够随机、不可逆存储、可展示前缀、可限定权限，并且不会在日志和响应中泄露。

### 功能任务

1. Key 格式：
   - 前缀固定为 `rag_`
   - 随机主体建议不少于 32 位高熵字符
   - 示例：`rag_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6`
2. Key 生成：
   - 使用加密安全随机数
   - 禁止使用时间戳、递增 ID、邮箱、租户 slug 拼接
   - 生成后立即计算 hash
3. hash 策略：
   - MVP 使用 SHA256 或 HMAC-SHA256
   - 如使用 HMAC，密钥来源必须配置化且生产不可缺失
   - 数据库保存 `key_hash`
4. prefix 策略：
   - `key_prefix` 建议为 `rag_` 后若干可识别字符加 `****`
   - 列表和详情只展示 `key_prefix`
   - 日志只允许记录 `key_prefix` 或 `api_key_id`
5. permissions JSON 最小结构：
   - `retrieve: true`
   - `kb_ids: [1, 2]`
   - 可选 `metadata_filter`
   - 可选 `rate_limit_hint`
6. permissions 校验：
   - `retrieve` 必须是布尔值
   - `kb_ids` 为空表示 Phase 2 暂按租户默认权限处理，Phase 3 再强制收敛
   - `kb_ids` 非空时必须是正整数数组
   - 非法 JSON 返回 `422 invalid_permissions`
7. app_id 规则：
   - `app_id` 用于日志、追踪和配额预留
   - 同租户下建议唯一或至少可读
   - 不允许从请求体覆盖 API Key 绑定的 `app_id`
8. 安全输出规则：
   - 创建响应返回完整 `key` 一次
   - 更新、列表、详情、吊销响应不返回完整 `key`
   - panic、错误日志、审计日志、SDK debug 不输出完整 Key

### 验收

1. 新建 Key 的随机性来源符合安全要求。
2. 数据库和日志搜索不到完整 `rag_<key>` 明文。
3. 列表接口只展示 `key_prefix`。
4. 非法 permissions 会被拒绝，不会写入脏配置。
5. 请求体 `app_id` 无法覆盖 API Key 绑定的 `app_id`。

---

## 5.4 L3 API Key 管理接口（列表、创建、更新、吊销）

### 目标

让已登录的租户用户可以管理自己租户内的 API Key，并且 API Key 创建、更新、吊销都受 Phase 1 JWT 与 RBAC 约束。

### 功能任务

1. 路由挂载：
   - `GET /v1/api-keys`
   - `POST /v1/api-keys`
   - `PUT /v1/api-keys/:id`
   - `DELETE /v1/api-keys/:id`
   - 可选预留：`POST /v1/api-keys/:id/rotate`
2. 建议新增 handler：
   - `backend/api/handler/apikey/list.go`
   - `backend/api/handler/apikey/create.go`
   - `backend/api/handler/apikey/update.go`
   - `backend/api/handler/apikey/revoke.go`
3. 权限要求：
   - 列表：`apikey:read`
   - 创建：`apikey:write`
   - 更新：`apikey:write`
   - 吊销：`apikey:revoke`
   - MVP 可复用 Phase 1 `owner/admin/member` 中的权限矩阵
4. 创建请求字段：
   - `name`
   - `app_id`
   - `permissions`
   - `expires_at`
5. 创建响应字段：
   - `id`
   - `name`
   - `app_id`
   - `key`
   - `key_prefix`
   - `permissions`
   - `status`
   - `expires_at`
   - `created_at`
6. 列表响应字段：
   - `id`
   - `name`
   - `app_id`
   - `key_prefix`
   - `permissions`
   - `status`
   - `last_used_at`
   - `expires_at`
   - `created_at`
7. 更新能力：
   - 更新 `name`
   - 更新 `permissions`
   - 更新 `expires_at`
   - 不允许更新 `key_hash`
   - 如需改变 Key 明文，走 rotate 或重新创建
8. 吊销能力：
   - `DELETE /v1/api-keys/:id` 将 `status` 改为 `revoked`
   - 不物理删除，保留审计与历史日志可追踪
   - 重复吊销可幂等返回成功或明确状态
9. 审计事件：
   - `apikey.create`
   - `apikey.update`
   - `apikey.revoke`
   - 可选 `apikey.rotate`

### 验收

1. JWT 登录用户只能管理自己租户内的 API Key。
2. 创建响应只在第一次返回完整 `key`。
3. 更新接口不会生成新明文 Key，也不会改变 `key_hash`。
4. 吊销后 Key 状态为 `revoked`，历史日志仍可关联 `api_key_id`。
5. 权限不足用户访问管理接口返回 `403 forbidden`。

---

## 5.5 L4 API Key 鉴权中间件与统一身份上下文接入

### 目标

让 Agent 请求只凭 `Authorization: Bearer rag_<key>` 获得可信身份，并与 JWT 管理端共享同一套 `auth.Identity` 输出。

### 功能任务

1. 新增或扩展认证模块：
   - `backend/internal/auth/api_key.go`
   - `backend/internal/middleware/auth.go`
   - 保留 `backend/internal/middleware/jwt.go`
2. Header 解析：
   - 只接受 `Authorization: Bearer rag_<key>`
   - Key 缺失、格式错误返回 `401 invalid_api_key`
   - 禁止从 query、body、cookie 读取 API Key 作为正式路径
3. 鉴权流程：
   - 提取完整 Key
   - 计算 `key_hash`
   - 查询 `rag_api_key`
   - 校验 `status=active`
   - 校验 `expires_at` 未过期
   - 查询并校验 `rag_tenant.status=active`
   - 查询并校验创建用户或绑定用户状态仍可用
4. 上下文写入：
   - `auth_type=api_key`
   - `tenant_id`
   - `user_id`
   - `role`
   - `app_id`
   - `api_key_id`
   - `permissions`
   - `is_legacy=false`
5. 统一认证顺序：
   - 优先尝试 JWT，服务 Admin/UI
   - 再尝试 API Key，服务 Agent/SDK
   - 最后按明确开关尝试 legacy `app_id`
   - dev bypass 仅限 Phase 0/1 规定的本地环境
6. `last_used_at` 更新：
   - 成功鉴权后异步或同步更新
   - 更新失败不影响请求继续
   - 日志记录 `last_used_update_failed=true`
7. 错误码：
   - hash 不存在：`401 invalid_api_key`
   - revoked：`401 api_key_revoked`
   - expired：`401 api_key_expired`
   - tenant suspended：`403 tenant_suspended`
   - user suspended：`403 user_suspended`
8. 安全日志：
   - 不记录完整 Key
   - 记录 `api_key_id/key_prefix/app_id/tenant_id/auth_type`
   - 对无效 Key 只记录 prefix 或 hash 前缀，避免泄露完整输入

### 验收

1. 有效 API Key 可生成完整 `auth.Identity`。
2. revoked、expired、格式错误、hash 不存在的 Key 均不能通过。
3. API Key 请求不会被错误识别为 JWT 或 legacy `app_id`。
4. 业务 handler 可以通过统一 helper 读取 `tenant_id/app_id/api_key_id/permissions`。
5. 日志不泄露完整 API Key。

---

## 5.6 L5 `/v1/retrieve` 新认证优先、legacy 回退与权限门禁

### 目标

把 `/v1/retrieve` 从“请求体 app_id 白名单”迁移为“API Key 身份驱动”，同时保留 legacy 回退，保证旧 Agent 不被立即中断。

### 功能任务

1. 改造入口：
   - 当前重点文件：`backend/api/handler/rag/retrieve.go`
   - 路由保持 `POST /v1/retrieve`
   - 中间件先注入 `auth.Identity`
2. API Key 路径：
   - 读取 `identity.auth_type=api_key`
   - `app_id` 使用 `identity.app_id`
   - `tenant_id` 使用 `identity.tenant_id`
   - `api_key_id` 使用 `identity.api_key_id`
   - permissions 使用 `identity.permissions`
3. 请求体 `app_id` 兼容规则：
   - API Key 路径下请求体不需要传 `app_id`
   - 如果传了且与 Key 绑定 `app_id` 一致，可忽略或记录兼容使用
   - 如果传了且不一致，建议返回 `409 app_id_conflict`
4. permissions 门禁：
   - `permissions.retrieve=true` 才允许检索
   - `permissions.kb_ids` 非空时，请求 `kb_ids` 必须是其子集
   - `permissions.kb_ids` 为空时，Phase 2 可放行到 Phase 3 强隔离前的租户默认权限
   - 不允许 API Key 请求绕过 `top_k`、`metadata_filter` 的基础校验
5. legacy 回退：
   - 仅当没有 JWT/API Key 身份，且请求体或旧 header 满足 legacy `app_id` 白名单时启用
   - 设置 `auth_type=legacy_app_id`
   - 设置 `is_legacy=true`
   - 设置 `deprecated=true`
   - 映射到 `SYSTEM_TENANT_ID` 或测试租户占位
6. 调用日志字段：
   - `tenant_id`
   - `app_id`
   - `api_key_id`
   - `auth_type`
   - `source_api=v1`
   - `is_legacy`
   - `request_id`
7. 响应提示：
   - legacy 响应可增加 header：`X-RAG-Auth-Mode: legacy_app_id`
   - API Key 响应可增加 header：`X-RAG-Auth-Mode: api_key`
8. 降级策略：
   - API Key 鉴权失败不得自动回退 legacy，除非请求明确满足 legacy 条件且没有提交 API Key
   - legacy 配置异常时不影响 API Key 路径
   - permissions 解析异常返回明确错误，不静默放行

### 验收

1. Agent 使用 API Key 调用 `/v1/retrieve` 不需要传 `app_id`。
2. API Key 绑定的 `app_id` 能进入检索日志。
3. Key permissions 禁止的 `kb_ids` 请求会被拒绝。
4. 带错误 API Key 的请求不会降级到 legacy 白名单。
5. legacy 调用仍可临时工作，但日志和响应 header 能明确识别。

---

## 5.7 L6 Go SDK、Agent 接入示例与本地脚本更新

### 目标

让 Agent 接入方使用的是正式 API Key 方式，而不是继续复制请求体 `app_id` 或默认 Admin 的旧示例。

### 功能任务

1. 更新 Go SDK：
   - `backend/pkg/ragsdk/client.go`
   - `backend/pkg/ragsdk/README.md`
   - `backend/pkg/ragsdk/example_test.go`
2. SDK 配置：
   - `BaseURL`
   - `APIKey`
   - `Timeout`
   - 可选 `AppID`，仅用于兼容或日志说明，不作为正式鉴权
3. SDK 请求头：
   - `Authorization: Bearer <api_key>`
   - `Content-Type: application/json`
   - `User-Agent: rag-sdk-go/<version>`
4. SDK 请求体：
   - `query`
   - `kb_ids`
   - `top_k`
   - `strategy_profile`
   - `metadata_filter`
   - 不再要求 `app_id`
5. SDK 错误处理：
   - `invalid_api_key`
   - `api_key_revoked`
   - `api_key_expired`
   - `forbidden`
   - `app_id_conflict`
6. 更新 curl 示例：
   - 创建 API Key 的 JWT 管理端示例
   - API Key 调用 `/v1/retrieve` 示例
   - 吊销 Key 后再次调用失败示例
   - legacy `app_id` 兼容示例
7. Agent 接入说明：
   - API Key 保存在 Agent 后端环境变量或密钥管理系统
   - 终端用户不直接持有 Key
   - 不在浏览器前端暴露 Key
   - Key 泄露时通过吊销或 rotate 处理
8. 本地脚本：
   - 登录获取 JWT
   - 创建 API Key
   - 用 API Key 检索
   - 吊销 API Key
   - 验证吊销后失败

### 验收

1. SDK 使用 `APIKey` 可以跑通 `/v1/retrieve`。
2. SDK 文档不再把请求体 `app_id` 写成正式认证方式。
3. curl smoke 可以从 JWT 登录跑到创建 Key，再跑到 Agent 检索。
4. Key 吊销后 SDK 请求返回认证失败。
5. 接入说明明确 Key 由 Agent 后端持有，不暴露给终端用户或浏览器。

---

## 5.8 L7 调用日志、审计、自动化测试与回归门禁

### 目标

把 Phase 2 的 API Key 接入固化为可追踪、可回放、可验证的回归门禁，确保后续 Phase 3 强隔离有可信调用数据。

### 功能任务

1. 检索日志扩展：
   - 当前重点文件：`backend/internal/model/kb_retrieve_log.go`
   - `tenant_id`
   - `app_id`
   - `api_key_id`
   - `auth_type`
   - `source_api`
   - `is_legacy`
   - 可选 `key_prefix`
2. 审计日志：
   - `apikey.create`
   - `apikey.update`
   - `apikey.revoke`
   - `apikey.auth_failed`
   - `retrieve`
3. 管理 API 测试：
   - 创建 Key
   - 列表只返回 prefix
   - 更新 name/permissions/expires_at
   - 吊销 Key
   - 权限不足拒绝
4. 鉴权测试：
   - 有效 Key
   - 格式错误
   - hash 不存在
   - revoked
   - expired
   - tenant suspended
   - user suspended
5. `/v1/retrieve` 测试：
   - API Key 成功
   - API Key 缺少 `retrieve` 权限
   - `kb_ids` 超出 permissions
   - 请求体 `app_id` 冲突
   - legacy 回退
   - 错误 Key 不回退 legacy
6. SDK 测试：
   - 自动加 Authorization header
   - 不要求请求体 `app_id`
   - 错误码映射
7. 日志断言：
   - API Key 请求包含 `auth_type=api_key`
   - legacy 请求包含 `auth_type=legacy_app_id`
   - 所有路径不包含完整 Key 明文
8. 回归命令建议：
   - `go test ./internal/auth ./internal/middleware ./internal/repository ./api/...`
   - `go test ./pkg/ragsdk`
   - 本地 curl smoke

### 验收

1. 自动化测试能稳定覆盖 Phase 2 Gate。
2. 检索日志能按 `api_key_id/app_id/tenant_id` 追踪调用。
3. 审计日志能追踪 Key 创建、更新、吊销和失败鉴权。
4. 日志与测试输出不泄露完整 Key。
5. Phase 2 回归报告可作为进入 Phase 3 的 Gate 材料。

---

## 5.9 L8 灰度发布、回滚预案与 Phase 2 验收收口

### 目标

确保 API Key 接入上线后真实 Agent 可以安全调用，同时 legacy 路径可控保留，出现异常时能快速回退到 Phase 1/legacy 稳定状态。

### 功能任务

1. 灰度顺序：
   - 本地 `dev` 完整跑通 JWT 创建 Key 与 API Key 检索
   - 测试环境启用 API Key 鉴权
   - staging 用测试 Agent 小流量调用
   - 真实 Agent 先只切一个 `app_id`
   - 逐步将 legacy `app_id` 调用迁移到 API Key
2. 回滚顺序：
   - 回退 SDK 到 legacy `app_id` 兼容调用
   - 暂停新 API Key 创建入口
   - 保留已创建 Key 记录，不物理删除
   - `/v1/retrieve` 临时只保留 legacy 回退路径
   - 修复后重新启用 API Key 中间件
3. 不能回滚的安全底线：
   - 不恢复生产默认 Admin
   - 不把 Key 明文落库
   - 不允许错误 API Key 自动降级 legacy
   - 不在浏览器暴露 Key
4. 告警与观察：
   - `invalid_api_key` 异常升高
   - `api_key_revoked` 异常升高
   - API Key 鉴权 P95 延迟异常
   - `/v1/retrieve` `app_id_conflict` 异常
   - legacy 调用占比未下降
5. 验收材料：
   - 迁移执行记录
   - API smoke 结果
   - SDK smoke 结果
   - API Key 安全检查结果
   - legacy 兼容调用统计
   - 回滚演练记录
6. Phase 3 交接：
   - `tenant_id` 已能从 API Key 推导
   - `api_key_id/app_id` 已进入检索日志
   - `permissions.kb_ids` 已有 MVP 门禁
   - legacy 调用可被识别和统计

### 验收

1. 灰度过程中 API Key 创建、吊销和检索调用稳定。
2. API Key 异常时可回退 legacy 兼容调用，但不会泄露或放行错误 Key。
3. 所有 Phase 2 验收材料齐全。
4. Phase 3 可以直接开始租户强隔离、向量 metadata 隔离和基础配额。

---

## 6. 推荐实施节奏（1 周）

## 6.1 阶段推进建议

1. 第 0.5 天完成 `L0`，冻结 API Key 契约、permissions JSON、legacy 回退边界和错误码。
2. 第 1-2 天完成 `L1 + L2`，落地 `rag_api_key` 迁移、模型、仓储、Key 生成和安全存储。
3. 第 3 天完成 `L3`，跑通 API Key 管理接口和 RBAC 权限。
4. 第 4 天完成 `L4 + L5`，打通 API Key 鉴权中间件、统一身份上下文和 `/v1/retrieve` 新认证优先。
5. 第 5 天完成 `L6 + L7`，补齐 SDK、curl、本地脚本、日志和自动化回归。
6. 第 5-6 天完成 `L8`，做灰度、回滚演练和 Phase 2 验收记录。

## 6.2 并行与合流规则

1. 可并行：`L1` 迁移与 `L2` Key 生成工具设计，`L3` handler 草稿与 `L4` 中间件接口设计，`L6` SDK 示例与 `L7` 测试用例设计。
2. 必须串行：`L3` 依赖 `L1/L2`，`L5` 依赖 `L4` 身份上下文稳定，`L8` 依赖 `L1~L7` 全部通过。
3. 合流条件：API Key 创建、鉴权、`/v1/retrieve`、SDK、日志和 legacy 回退全部通过后，才允许进入 Phase 3。
4. Code review 重点：Key 明文处理、hash 策略、permissions 校验、错误 Key 不降级、日志脱敏和 legacy 标记必须统一审查。

---

## 7. 角色分工（建议）

1. 后端 A：`L1 + L2`，负责 `rag_api_key` 迁移、模型、仓储、Key 生成、hash 与 permissions 校验。
2. 后端 B：`L3 + L4`，负责 API Key 管理接口、RBAC 接入、API Key 鉴权中间件和统一身份上下文。
3. 后端 C：`L5 + L7`，负责 `/v1/retrieve` 改造、调用日志、审计日志和回归测试。
4. SDK/文档：`L6`，负责 Go SDK、curl smoke、Agent 接入说明和 legacy 迁移说明。
5. QA/SRE：`L0 + L8`，负责契约冻结、灰度策略、回滚演练、日志脱敏检查和验收记录。

补充协作约束：

1. 后端与 SDK 先冻结 `Authorization: Bearer rag_<key>` 和请求体去 `app_id` 的正式口径，再改示例。
2. 后端与 QA 先冻结 Key 明文不落库、不进日志、不进列表响应的检查方法。
3. `/v1/retrieve` 改造时必须明确 API Key、JWT、legacy、dev bypass 的认证优先级。
4. Phase 2 不允许为了临时兼容把请求体 `app_id` 当作正式身份来源。

---

## 8. 阶段验收模板（执行后填写）

1. 功能完成情况（按 L0-L8）：
2. 数据库迁移结果：
   - `rag_api_key`
   - 索引与唯一约束
   - down 迁移演练
3. API Key 管理接口验证：
   - `GET /v1/api-keys`
   - `POST /v1/api-keys`
   - `PUT /v1/api-keys/:id`
   - `DELETE /v1/api-keys/:id`
   - 可选 `POST /v1/api-keys/:id/rotate`
4. Key 安全检查：
   - 创建响应明文一次性返回
   - 数据库无完整 Key
   - 日志无完整 Key
   - 列表和详情只显示 `key_prefix`
5. API Key 鉴权验证：
   - valid
   - invalid
   - revoked
   - expired
   - tenant suspended
   - user suspended
6. 统一身份上下文快照：
   - `auth_type`
   - `tenant_id`
   - `user_id`
   - `role`
   - `app_id`
   - `api_key_id`
   - `permissions`
7. `/v1/retrieve` 验证：
   - API Key 成功
   - 无请求体 `app_id` 成功
   - `app_id` 冲突失败
   - permissions 拒绝
   - legacy 回退
   - 错误 Key 不回退
8. SDK 与脚本验证：
   - Go SDK
   - curl smoke
   - 吊销后失败
   - legacy 示例
9. 日志与审计验证：
   - `api_key_id`
   - `app_id`
   - `tenant_id`
   - `auth_type`
   - `is_legacy`
   - `apikey.create/update/revoke`
10. 自动化测试结果：
   - auth
   - middleware
   - repository
   - handler
   - SDK
11. 回滚演练结果：
   - 暂停 API Key 创建
   - 回退 legacy 调用
   - 保留 Key 记录
   - 重新启用 API Key
12. 遗留风险与负责人：
13. 是否允许进入 Phase 3（是/否）：

---

## 9. Phase 2 完成后下一步（明确路线衔接）

下一阶段固定进入 Phase 3：多租户强隔离 + 基础配额，按以下顺序衔接：

1. 给知识库、文档、任务、审计日志、检索日志补齐 `tenant_id`。
2. 将现有 `user_id` 知识库迁移到默认系统租户或测试租户。
3. Repository 层对列表、详情、删除、上传、日志查询统一追加 `tenant_id` 条件。
4. 创建 `rag_tenant_kb_permission`，校验租户可访问的 `kb_id`。
5. `/v1/retrieve` 在检索前校验 `kb_ids` 是否属于当前租户或 Key 权限。
6. 入库 chunk metadata 写入 `tenant_id/kb_id`，Milvus 检索 expr 强制包含租户过滤。
7. 实现每日 API 调用、最大知识库数、最大文档数等基础配额。
8. 构造租户 A/B，验证互相无法读取、检索、删除、查看日志。

Phase 3 完成后，再进入 Phase 4 Admin UI 闭环 + 接入文档，把注册登录、知识库、API Key、Agent 检索和日志查看串成可由非开发人员操作的完整产品链路。

---

## 10. 文档维护规则（作为后续实现参考基线）

1. 任何 Phase 2 范围变更，先更新本文档，再改代码。
2. 修改 API Key 表字段、索引、状态枚举或 permissions JSON 时，必须同步更新 `L1/L2/阶段验收模板`。
3. 修改 Key 格式、hash 策略、prefix 策略或明文返回规则时，必须同步更新 `L2`，并补充日志脱敏测试。
4. 修改 `/v1/api-keys` 字段、错误码或权限要求时，必须同步更新 `L3` 和阶段验收模板。
5. 修改 API Key 鉴权顺序、统一身份上下文字段或 legacy 回退条件时，必须同步更新 `L4/L5`。
6. 修改 SDK 请求体、header 或示例时，必须同步更新 `L6`，避免 Agent 接入继续依赖请求体 `app_id`。
7. Phase 3 实现强隔离时，以本文档的 `auth_type=api_key/tenant_id/app_id/api_key_id/permissions` 作为服务端调用身份基线。
