# Phase 4 详细功能实现路线（Admin UI 闭环 + 接入文档）

## 1. 文档定位

本文档是多租户平台改造 Phase 4 的执行手册，目标是把“Admin UI 闭环 + 接入文档”拆成可直接实施、可验收、可回滚的细颗粒任务路线。

它有三个用途：

1. 作为团队推进登录注册、会话管理、API Key 管理、租户设置、用量展示、权限可见性和接入文档的统一执行文档。
2. 作为 Phase 3 “多租户强隔离 + 基础配额”之后的产品闭环 Gate，确保非开发人员可以完成“注册登录 -> 创建知识库 -> 上传文档 -> 创建 API Key -> Agent 检索 -> 查看日志”。
3. 作为 Phase 5 商业化和高级企业能力的交接基线，确保套餐、成员、计费、OAuth、Webhook、企业安全增强不会重新定义 Phase 1-4 的认证、隔离和 Agent 接入主链路。

本文档风格与现有路线文档保持一致：
目标 -> 路线 -> 模块任务 -> 验收门禁 -> 下一步入口。

统一口径说明：

1. `Admin UI 闭环` 固定指：用户不依赖手工改库或口头说明，可以在管理台完成登录、知识库管理、文档上传、API Key 管理、用量查看和日志查看。
2. `JWT 会话` 固定指：Admin UI 只使用 `/v1/auth/login`、`/v1/auth/refresh`、`/v1/auth/me` 返回或校验的 `access_token/refresh_token`，不继续依赖开发期默认 admin。
3. `API Key 明文一次性展示` 固定指：`POST /v1/api-keys` 创建成功后只展示一次完整 `key`，后续列表和详情只展示 `key_prefix`。
4. `Agent 接入` 固定指：Agent 后端或 SDK 持有 `Authorization: Bearer rag_<key>`，终端用户不直接持有 API Key。
5. `租户用量` 固定指：基于 Phase 3 配额口径展示 `api_calls_today/kb_count/doc_count/storage_mb/limits`，前端不自行计算后端未返回的配额。
6. `权限可见性` 固定指：前端按 `role/permissions` 隐藏或禁用不可操作入口，但后端仍是最终权限校验来源。
7. `接入文档` 固定指：输出可复制运行的 cURL、Go SDK、Python requests、Agent 后端接入示例，并明确 JWT 与 API Key 的使用边界。
8. `Phase 4 E2E` 固定指：从新租户注册或 bootstrap owner 登录开始，到 UI 创建 Key，再用该 Key 调用 `/v1/retrieve`，最后在日志中看到 `tenant_id/app_id/api_key_id/auth_type`。

---

## 2. Phase 4 范围边界

## 2.1 本阶段必须完成

1. Admin UI 支持注册、登录、Token 刷新、登出和当前用户信息展示。
2. `admin/src/services/api/client.ts` 支持自动附带 JWT、处理 `401`、执行 refresh 或跳转登录。
3. Admin Shell 展示当前租户、当前用户、角色和基础导航状态。
4. API Key 页面支持创建、列表、复制一次性 Key、更新基础信息、吊销和轮换入口。
5. API Key 创建表单支持 `name/app_id/permissions/kb_ids/expires_in` 或与后端契约等价字段。
6. 租户设置页面展示租户名、slug、套餐占位、状态和基础配额限制。
7. 用量页面展示 `api_calls_today/kb_count/doc_count/storage_mb/limits`，并能解释配额超限错误。
8. 知识库、文档、API Key、日志页面按 `role` 和权限隐藏或禁用不可用操作。
9. 接入文档输出 cURL、Go SDK、Python requests、Agent 后端接入示例和错误码说明。
10. 完成 Phase 4 E2E 验收：注册登录 -> 创建知识库 -> 上传文档 -> 创建 API Key -> Agent 检索 -> 查看日志。

## 2.2 本阶段明确不做

1. 不做套餐购买、支付、账单、欠费停用和发票能力，该能力属于 Phase 5。
2. 不做 OAuth、SSO、IP 白名单、Webhook 和复杂成员邀请流程。
3. 不做完整成员管理和企业组织审批；Phase 4 只消费当前 `owner/admin/member/viewer` 角色。
4. 不重写 Phase 3 多租户强隔离、知识库授权、Milvus tenant expr 和基础配额逻辑。
5. 不让前端保存或再次展示 API Key 明文；明文只在创建或轮换响应中出现一次。
6. 不把前端隐藏按钮当作安全边界；所有敏感操作必须继续由后端校验。
7. 不下线 legacy `app_id` 兼容路径；Phase 4 只在文档中标记 deprecated，并引导新接入使用 API Key。
8. 不在接入文档中要求终端用户把自己的 JWT 交给 Agent；Agent 后端必须使用服务端 API Key。

---

## 3. 目标与通过标准（Gate）

Phase 4 通过标准（全满足）：

1. Admin UI 可以在无开发期默认 admin 的情况下，通过真实 JWT 完成登录和访问受保护页面。
2. 注册流程能创建租户和 owner 用户；如果后端注册接口不直接返回 token，前端必须明确引导用户继续登录。
3. `GET /v1/auth/me` 能驱动 Admin Shell 展示 `user_id/email/name/role/tenant_id/tenant_name` 或等价字段。
4. API client 能自动附带 `Authorization: Bearer <access_token>`，并在 access token 过期时执行 refresh 或安全登出。
5. API Key 创建后只展示一次完整 `rag_<key>`，刷新页面或重新进入列表后不可再看到明文。
6. 吊销 API Key 后，旧 Key 调用 `/v1/retrieve` 返回 `401/403`，UI 状态和接入文档都能解释。
7. 使用 UI 生成的 Key 可以直接通过 cURL、Go SDK、Python requests 跑通 `/v1/retrieve`。
8. 租户设置和用量页面展示的数据均来自当前租户，不接受前端传入或覆盖 `tenant_id`。
9. `viewer` 角色看不到或无法触发创建知识库、上传文档、创建/吊销 API Key 等写操作，后端仍拒绝越权请求。
10. E2E 验收记录包含请求样例、响应样例、日志截图或日志字段摘录、失败用例和回滚结果。

---

## 4. 实现路线总览（L0 -> L8）

Phase 4 按 9 条路线推进，按门禁顺序合流：

1. L0：Phase 3 交接基线、前后端契约与 E2E 场景冻结
2. L1：Admin UI 认证页面、JWT 会话与路由保护
3. L2：API client、Auth Store、错误处理与安全登出
4. L3：API Key 管理页面、一次性明文展示与轮换/吊销闭环
5. L4：租户设置、用量页面与配额提示
6. L5：权限可见性、角色体验与跨页面联动
7. L6：接入文档、SDK 示例、curl smoke 与错误码手册
8. L7：E2E 自动化验收、日志验证与兼容路径确认
9. L8：灰度发布、回滚预案与 Phase 4 验收收口

建议顺序：`L0 -> L1 + L2 -> L3 + L4 -> L5 + L6 -> L7 -> L8`

---

## 5. 详细路线拆解

## 5.1 L0 Phase 3 交接基线、前后端契约与 E2E 场景冻结

### 目标

在开发 Admin UI 闭环前冻结后端接口、前端路由、会话字段、权限字段和 E2E 验收路径，避免前端用临时 mock 或绕过多租户安全基线。

### 功能任务

1. 确认 Phase 3 交接物可用：
   - `backend/docs/zhuhu/phase3-acceptance-report.md`
   - `backend/docs/zhuhu/phase3-multi-tenant-isolation-quota-detailed-roadmap.md`
   - `backend/internal/auth/context.go`
   - `backend/internal/auth/contract.go`
   - `backend/internal/quota/quota.go`
   - `backend/internal/repository/kb_tenant_repo.go`
   - `backend/internal/repository/rag_retrieve_log_repo.go`
   - `backend/api/handler/rag/retrieve.go`
2. 冻结认证接口契约：
   - `POST /v1/auth/register`
   - `POST /v1/auth/login`
   - `POST /v1/auth/refresh`
   - `GET /v1/auth/me`
   - `PUT /v1/auth/password`
   - 登出如果后端暂未提供黑名单接口，前端先执行本地 token 清理并记录限制。
3. 冻结注册后行为：
   - 方案 A：注册成功后后端直接返回 token，前端进入 Admin。
   - 方案 B：注册成功后只返回 `user_id/email/tenant_id`，前端提示注册成功并跳转登录。
   - 当前代码更接近方案 B，Phase 4 首版按方案 B 实施，除非后端同步升级注册响应。
4. 冻结 API Key 接口契约：
   - `GET /v1/api-keys`
   - `POST /v1/api-keys`
   - `PUT /v1/api-keys/:id`
   - `DELETE /v1/api-keys/:id`
   - `POST /v1/api-keys/:id/rotate` 如果后端暂缺，页面先保留禁用入口或隐藏入口，不用前端伪造轮换。
5. 冻结租户和用量接口契约：
   - `GET /v1/auth/me` 用于用户和租户基础信息。
   - `GET /v1/tenant` 用于租户详情；如果暂缺，可由 `/v1/auth/me` 最小兜底。
   - `GET /v1/tenant/usage` 用于用量；如果暂缺，需要后端补齐或页面展示契约缺口。
6. 冻结前端新增路由：
   - `admin/src/app/(auth)/login/page.tsx`
   - `admin/src/app/(auth)/register/page.tsx`
   - `admin/src/app/(admin)/api-keys/page.tsx`
   - `admin/src/app/(admin)/tenant/settings/page.tsx`
   - `admin/src/app/(admin)/tenant/usage/page.tsx`
   - `admin/src/app/(admin)/docs/integration/page.tsx` 或文档外链入口
7. 冻结前端基础模块：
   - `admin/src/services/api/client.ts`
   - `admin/src/config/api.ts`
   - `admin/src/components/admin/admin-shell.tsx`
   - 新增 `admin/src/services/auth/session.ts`
   - 新增 `admin/src/types/auth.ts`
   - 新增 `admin/src/types/tenant.ts`
   - 新增 `admin/src/types/api-key.ts`
8. 冻结 E2E 测试账号和测试数据：
   - bootstrap owner 或新注册 owner
   - 一个测试租户
   - 一个测试知识库
   - 一份小文档
   - 一个 API Key
   - 一个 `/v1/retrieve` query
9. 冻结失败用例：
   - 未登录访问 Admin 页面
   - token 过期访问 Admin 页面
   - viewer 创建 API Key
   - revoked API Key 调用 `/v1/retrieve`
   - API Key `kb_ids` 越权检索
   - 配额超限上传或检索

### 验收

1. Phase 4 开始前已有一份可执行的接口和路由清单。
2. 注册后是否自动登录的行为已冻结，前端不再猜后端响应。
3. 租户设置和用量接口缺口已明确标记，不用假数据填充。
4. E2E 成功路径和失败路径都有测试数据准备方案。

---

## 5.2 L1 Admin UI 认证页面、JWT 会话与路由保护

### 目标

让管理端从开发期入口切换到真实账号体系，用户可以注册、登录、刷新会话、登出，并且未登录用户不能进入受保护页面。

### 功能任务

1. 新增登录页面：
   - 路径：`/login`
   - 字段：`email/password`
   - 调用：`POST /v1/auth/login`
   - 成功后保存 `access_token/refresh_token/expires_in/user_id/role/tenant_id`
   - 跳转到 `/dashboard` 或用户原始访问地址
2. 新增注册页面：
   - 路径：`/register`
   - 字段：`email/password/name/tenant_name`
   - 调用：`POST /v1/auth/register`
   - 注册成功后按 L0 冻结策略跳转登录或进入 Admin
   - 密码强度错误、邮箱已存在、租户创建失败需要给出可读提示
3. 新增会话读取：
   - 页面初始化调用 `GET /v1/auth/me`
   - 获取 `user_id/email/name/role/tenant_id/tenant_name`
   - Admin Shell 展示用户和租户摘要
4. 新增路由保护：
   - `(admin)` layout 检查 token
   - 无 token 跳转 `/login`
   - token 无效或 refresh 失败时清理本地会话
   - 登录页已登录时跳转 `/dashboard`
5. 登出：
   - 清理本地 `access_token/refresh_token`
   - 清理用户缓存
   - 跳转 `/login`
   - 如果后端后续提供 `POST /v1/auth/logout`，再补远端 token 失效
6. 密码修改入口：
   - 可放入租户设置或用户菜单
   - 调用 `PUT /v1/auth/password`
   - 修改成功后建议要求重新登录
7. 页面体验：
   - 登录失败展示统一错误区
   - 正在登录展示 loading
   - token 校验中避免页面闪烁泄露受保护内容
   - 不在页面或 console 输出 token

### 验收

1. 未登录访问 `/dashboard`、`/knowledge-bases`、`/api-keys` 会跳转 `/login`。
2. owner 可以登录后进入 Admin Shell，并看到当前租户和角色。
3. 注册成功后的行为与 L0 契约一致，不出现空白状态。
4. access token 无效时不会继续调用受保护 API。
5. 登出后浏览器刷新仍保持未登录状态。

---

## 5.3 L2 API client、Auth Store、错误处理与安全登出

### 目标

把认证能力下沉到统一 API client 和会话模块，让所有页面一致地附带 JWT、处理过期、识别权限错误和配额错误。

### 功能任务

1. 改造 `admin/src/services/api/client.ts`：
   - 请求拦截器读取 `access_token`
   - 自动附带 `Authorization: Bearer <access_token>`
   - 保留 FormData 自动删除 `Content-Type` 的现有逻辑
   - 保留 `/api` 前缀兼容逻辑
2. 增加 refresh 流程：
   - `401 invalid_token/token_expired` 触发 `POST /v1/auth/refresh`
   - refresh 成功后重放原请求
   - refresh 失败后清理会话并跳转 `/login`
   - 同一时刻多个 401 只触发一次 refresh，其他请求等待结果
3. 新增 Auth Store：
   - 可用轻量 `localStorage + React context`
   - 保存 token、用户摘要、租户摘要
   - 提供 `login/logout/refresh/loadMe`
   - 不保存 API Key 明文
4. 新增 API 常量：
   - `AUTH_REGISTER`
   - `AUTH_LOGIN`
   - `AUTH_REFRESH`
   - `AUTH_ME`
   - `AUTH_PASSWORD`
   - `API_KEYS`
   - `API_KEY_DETAIL`
   - `API_KEY_ROTATE`
   - `TENANT_DETAIL`
   - `TENANT_USAGE`
5. 统一错误类型：
   - `401 unauthorized/invalid_token`
   - `403 forbidden/tenant_suspended`
   - `404 not_found`
   - `409 email_exists`
   - `422 weak_password`
   - `429 quota_exceeded/rate_limited`
6. 配额错误展示：
   - 读取 `quota_type/current/limit/reset_at`
   - 创建知识库、上传文档、检索页面都能展示对应限制
7. 权限错误展示：
   - `403 forbidden` 显示“当前角色无权限”
   - `404 not_found` 在详情页显示资源不存在或无权访问
   - 不提示其他租户资源是否存在
8. 测试：
   - token 附带测试
   - refresh 重放测试
   - refresh 失败登出测试
   - 429 配额错误格式化测试

### 验收

1. 所有 Admin API 请求都自动附带 JWT。
2. access token 过期后可以 refresh 并重放原请求。
3. refresh token 失效后进入安全登出，不出现无限重试。
4. 配额错误和权限错误在页面上有明确、可读、不中断整体页面的提示。
5. API client 不记录或泄露 token/API Key 明文。

---

## 5.4 L3 API Key 管理页面、一次性明文展示与轮换/吊销闭环

### 目标

让 owner/admin/member 在权限允许范围内通过 UI 创建和管理 Agent 接入 Key，并确保 API Key 明文只在创建或轮换后展示一次。

### 功能任务

1. 新增 `/api-keys` 页面：
   - 列表展示 `name/app_id/key_prefix/permissions/status/last_used_at/expires_at/created_at`
   - 支持空态、加载态、错误态
   - 支持按 `status/app_id` 过滤或搜索
2. 创建 API Key：
   - 表单字段：`name`
   - 表单字段：`app_id`
   - 表单字段：`permissions.retrieve`
   - 表单字段：`permissions.kb_ids`
   - 表单字段：`expires_in` 或 `expires_at`，以当前后端契约为准
   - 创建成功后展示完整 `key`
   - 提供复制按钮
   - 弹窗关闭前提示“关闭后无法再次查看完整 Key”
3. API Key 明文处理：
   - 不写入 localStorage/sessionStorage
   - 不写入 URL
   - 不进入全局 store
   - 只存在当前弹窗组件状态
   - 弹窗关闭后清空内存状态
4. 更新 API Key：
   - 支持修改 `name`
   - 支持修改 `permissions`
   - 修改后刷新列表
   - 不允许修改 `key_prefix/key_hash/tenant_id`
5. 吊销 API Key：
   - 操作前二次确认
   - 调用 `DELETE /v1/api-keys/:id`
   - 成功后状态变更为 `revoked` 或从 active 列表移除
   - 提示旧 Key 会立即失效
6. 轮换 API Key：
   - 如果后端已实现 `POST /v1/api-keys/:id/rotate`，页面展示“轮换”按钮
   - 轮换成功后展示新 Key 明文一次
   - 如果后端暂未实现，页面隐藏或置灰，并在文档中标记暂不支持
7. 权限与知识库联动：
   - `permissions.kb_ids` 只能选择当前租户有 `read` 权限的知识库
   - 无知识库时提示先创建知识库
   - viewer 角色不可创建、更新、吊销 Key
8. 接入提示：
   - 创建成功弹窗内展示 cURL 示例
   - 展示“Agent 后端持有 Key，终端用户不直接持有 Key”
   - 展示 `Authorization: Bearer rag_<key>` 的格式
9. 测试：
   - 创建成功只展示一次明文
   - 刷新页面后只显示 `key_prefix`
   - 吊销后旧 Key 无法检索
   - API Key `kb_ids` 越权请求失败

### 验收

1. 用户可以在 UI 创建 API Key，并立即复制一次完整 Key。
2. 关闭创建结果弹窗后，UI 无法再次查看完整 Key。
3. API Key 列表不展示明文，只展示 `key_prefix`。
4. 吊销后的 Key 调用 `/v1/retrieve` 失败。
5. 页面明确区分 API Key 用于 Agent/SDK，JWT 用于 Admin UI。

---

## 5.5 L4 租户设置、用量页面与配额提示

### 目标

让租户 owner 能在管理端看到自己的租户基础信息、套餐占位、状态和资源使用情况，为 Phase 5 商业化打好展示口径。

### 功能任务

1. 新增 `/tenant/settings` 页面：
   - 展示 `tenant_id`
   - 展示 `name`
   - 展示 `slug`
   - 展示 `plan`
   - 展示 `status`
   - 展示创建时间或更新时间（如后端返回）
2. 租户名称更新：
   - 如果后端已实现 `PUT /v1/tenant`，owner 可修改 `name`
   - 如果暂未实现，只读展示并标记“暂不支持修改”
   - 不允许前端修改 `tenant_id/slug/plan/status`
3. 新增 `/tenant/usage` 页面：
   - 展示 `api_calls_today`
   - 展示 `api_calls_this_month`（如后端返回）
   - 展示 `kb_count`
   - 展示 `doc_count`
   - 展示 `storage_mb`
   - 展示 `limits.max_kb_count`
   - 展示 `limits.max_doc_count`
   - 展示 `limits.max_storage_mb`
   - 展示 `limits.max_api_calls_per_day`
4. 用量展示方式：
   - 进度条展示当前值和上限
   - 80% 以上展示预警样式
   - 超限展示 `quota_exceeded` 说明
   - `Enterprise` 或无限额度用“无限制”展示，不用巨大数字误导用户
5. 配额错误联动：
   - 创建知识库超限时跳转或提示查看 `/tenant/usage`
   - 上传文档超限时展示文档数或存储限制
   - API 调用超限时展示 `reset_at`
6. Admin Shell 租户摘要：
   - 显示当前租户名
   - 显示当前用户角色
   - 显示套餐占位
7. 契约缺口处理：
   - `/v1/tenant` 缺失时可用 `/v1/auth/me` 的租户名兜底
   - `/v1/tenant/usage` 缺失时页面显示“后端用量接口待补齐”，不展示假 0
8. 测试：
   - 当前租户信息展示正确
   - 用量进度条边界值正确
   - 429 错误能关联到对应配额项

### 验收

1. owner 登录后可以看到当前租户基础信息。
2. 用量页面显示当前值、上限和配额风险。
3. 前端不允许用户输入或覆盖 `tenant_id`。
4. 后端接口缺失时页面明确显示契约缺口，不使用 mock 数据伪装上线。
5. 配额超限错误能被用户理解，并知道应减少资源或等待重置。

---

## 5.6 L5 权限可见性、角色体验与跨页面联动

### 目标

让不同角色在 UI 中看到符合权限的操作入口，同时保持后端最终校验，降低误操作和越权尝试。

### 功能任务

1. 冻结角色可见性矩阵：
   - `owner`：租户设置、API Key 管理、知识库创建、上传、删除、日志、审计或未来成员入口
   - `admin`：API Key 管理、知识库创建、上传、删除、日志
   - `member`：知识库查看、上传、检索、日志查看、按后端权限创建 API Key
   - `viewer`：知识库查看、检索、日志查看，不展示写操作
2. 前端权限工具：
   - 新增 `canCreateKB(role)`
   - 新增 `canUploadDocument(role)`
   - 新增 `canManageAPIKey(role)`
   - 新增 `canViewTenantSettings(role)`
   - 新增 `canViewUsage(role)`
3. Admin Shell 导航过滤：
   - viewer 隐藏 API Key 创建入口或整页写操作
   - 无权限页面访问时展示 `403` 友好页
   - 不把隐藏导航当作安全边界
4. 知识库页面联动：
   - 创建知识库按钮按角色显示
   - 上传文档按钮按角色显示
   - 删除知识库按钮按角色显示
   - 操作失败时展示后端返回的 `forbidden/quota_exceeded`
5. API Key 页面联动：
   - 无管理权限时只读或不可访问
   - `permissions.kb_ids` 只展示当前租户授权知识库
   - Key 状态 `revoked` 时禁用更新和再次吊销
6. 日志页面联动：
   - 检索日志只展示当前租户数据
   - `request_id` 详情访问失败时不泄露资源存在性
7. 审计提示：
   - Phase 4 首版不做完整审计中心，但敏感操作页面应提示“操作会被审计记录”
   - 对应后端若已有 `kb_audit_event`，保留后续接入位置
8. 测试：
   - viewer 看不到创建知识库按钮
   - viewer 直接访问创建接口仍被后端拒绝
   - owner/admin 可完成写操作
   - 跨租户 `request_id/kb_id` 不可查看

### 验收

1. 不同角色看到的导航和操作按钮符合权限矩阵。
2. 直接构造请求绕过前端时，后端仍能拒绝越权。
3. 权限不足和资源不存在的错误提示不会泄露其他租户信息。
4. 页面权限逻辑集中维护，不在各组件散落硬编码。

---

## 5.7 L6 接入文档、SDK 示例、curl smoke 与错误码手册

### 目标

让 Agent 接入不再依赖口头说明，开发者能复制文档中的命令和代码完成服务端接入，并理解认证、权限、配额和错误码。

### 功能任务

1. 新增或更新接入文档：
   - `backend/docs/zhuhu/agent-integration-guide.md`
   - 更新 `backend/pkg/ragsdk/README.md`
   - 可在 Admin UI 增加 `/docs/integration` 页面或外链入口
2. 文档必须包含认证边界：
   - Admin UI 使用 JWT
   - Agent/SDK 使用 API Key
   - 终端用户不直接持有 API Key
   - legacy `app_id` 仅兼容旧系统，标记 deprecated
3. cURL 示例：
   - 登录获取 JWT
   - 创建 API Key
   - 使用 `Authorization: Bearer rag_<key>` 调用 `/v1/retrieve`
   - 吊销 API Key
   - revoked Key 调用失败示例
4. Go SDK 示例：
   - `ragsdk.NewClient`
   - `Retrieve`
   - 设置 `KBIDs`
   - 处理 `401/403/429`
5. Python requests 示例：
   - 设置 `headers.Authorization`
   - 构造 `query/kb_ids/top_k`
   - 打印 `request_id/items/citation/source`
   - 捕获错误码
6. Agent 后端接入示例：
   - 服务端读取环境变量 `RAG_API_KEY`
   - 不把 Key 返回给浏览器
   - 为每个环境创建不同 API Key
   - 使用 `app_id` 追踪应用
   - 根据业务选择 `kb_ids`
7. 权限说明：
   - `permissions.retrieve`
   - `permissions.kb_ids`
   - API Key 权限只能收窄租户授权，不能扩大授权
   - 知识库无权限时返回 `403/404`
8. 配额说明：
   - `quota_exceeded`
   - `quota_type`
   - `current`
   - `limit`
   - `reset_at`
9. 错误码手册：
   - `401 invalid_api_key`
   - `401 api_key_revoked`
   - `401 api_key_expired`
   - `403 forbidden`
   - `403 tenant_suspended`
   - `404 not_found`
   - `429 quota_exceeded`
   - `429 rate_limited`
10. smoke 脚本：
    - 更新 `backend/scripts/test-retrieve.sh`
    - 可新增 `backend/scripts/smoke/phase4-agent-retrieve.ps1`
    - 支持从环境变量读取 `RAG_BASE_URL/RAG_API_KEY/KB_ID`

### 验收

1. 新接入者只看文档即可创建 Key 并调用 `/v1/retrieve`。
2. cURL、Go SDK、Python requests 示例至少各跑通一次。
3. 文档明确 API Key 不进入浏览器、不交给终端用户。
4. 错误码说明能覆盖无效 Key、吊销 Key、权限不足和配额超限。
5. smoke 脚本可以作为 Phase 4 验收材料复跑。

---

## 5.8 L7 E2E 自动化验收、日志验证与兼容路径确认

### 目标

把 Phase 4 产品闭环固化成可复跑的验收流程，证明 UI、API Key、检索、日志和配额都能在真实路径中协作。

### 功能任务

1. E2E 成功路径：
   - 打开 `/register`
   - 注册租户 owner 或使用 bootstrap owner 登录
   - 进入 `/dashboard`
   - 创建知识库
   - 上传测试文档
   - 等待入库完成
   - 创建 API Key
   - 复制 Key
   - 使用 cURL 或 SDK 调用 `/v1/retrieve`
   - 在检索日志页面查看 `request_id`
2. E2E 失败路径：
   - 错误密码登录失败
   - 未登录访问 Admin 页面跳转登录
   - viewer 访问写操作失败
   - revoked Key 检索失败
   - API Key `kb_ids` 越权失败
   - 配额超限返回 `429`
3. 日志验证字段：
   - `tenant_id`
   - `app_id`
   - `api_key_id`
   - `auth_type`
   - `source_api`
   - `is_legacy`
   - `permission_result`
4. UI 验证字段：
   - 当前用户
   - 当前租户
   - 当前角色
   - API Key prefix
   - 用量 current/limit
5. 自动化建议：
   - 前端使用 `vitest` 覆盖 Auth Store 和 API client
   - 可用 Playwright 或脚本化 smoke 覆盖核心路径
   - 后端继续跑 auth、apikey、retrieve、tenant isolation、quota 测试
6. 回归命令建议：
   - `npm --prefix admin test`
   - `npm --prefix admin run lint`
   - `npm --prefix admin run build`
   - `go test ./internal/auth ./internal/repository ./internal/quota ./internal/ragplatform/...`
   - `go test ./api/...`
7. 兼容路径确认：
   - legacy `app_id` 仍能按 Phase 3 规则映射明确租户
   - 新文档不推荐 legacy
   - 错误 API Key 不会降级到 legacy
8. 验收记录：
   - 输出 `backend/docs/zhuhu/phase4-acceptance-record.md`
   - 记录通过项、失败项、截图或日志摘录、遗留问题和负责人

### 验收

1. Phase 4 E2E 成功路径可以稳定复跑。
2. 失败路径不会泄露跨租户资源。
3. 检索日志能证明 UI 创建的 Key 被真实 Agent 调用。
4. 自动化测试和 smoke 脚本覆盖核心闭环。
5. 验收记录可支撑进入 Phase 5 决策。

---

## 5.9 L8 灰度发布、回滚预案与 Phase 4 验收收口

### 目标

确保 Admin UI 闭环和接入文档安全上线，出现认证、Key 管理或用量展示异常时可局部回滚，不破坏 Phase 3 强隔离底线。

### 功能任务

1. 灰度顺序：
   - 本地 owner 完整跑通
   - 测试环境开启登录注册页面
   - 测试环境开启 API Key 页面
   - 测试环境跑通 Agent 检索 smoke
   - staging 对内部租户开放
   - 小范围真实 Agent 项目试用
   - 全量替换旧口头接入说明
2. 回滚顺序：
   - 隐藏 `/docs/integration` 新入口，保留静态文档
   - 隐藏 API Key 轮换按钮
   - 禁用 API Key 创建表单，保留列表只读
   - 回退登录页新样式，但不回退真实 JWT 鉴权
   - 用量页面接口异常时展示契约缺口，不阻断其他页面
3. 不能回滚的安全底线：
   - 不允许恢复生产默认 admin 后门
   - 不允许前端保存 API Key 明文
   - 不允许绕过 JWT 访问 Admin API
   - 不允许 `/v1/retrieve` 接受前端传入 `tenant_id`
   - 不允许错误 API Key 降级到 legacy
   - 不允许跨租户日志详情读取
4. 告警与观察：
   - 登录失败率异常升高
   - refresh 失败率异常升高
   - API Key 创建失败率异常升高
   - revoked Key 仍可检索
   - `/v1/retrieve` 401/403/429 异常升高
   - 前端页面白屏或 build 异常
5. 验收材料：
   - E2E 录屏或截图
   - cURL/Go/Python smoke 输出
   - API Key 一次性展示验证
   - revoked Key 失败验证
   - 用量页面截图
   - 权限可见性验证
   - 回滚演练记录
6. Phase 5 交接：
   - 套餐页面只展示占位，不接支付
   - 用量统计可作为计费输入之一
   - 角色和 API Key 权限可作为企业权限增强起点
   - 接入文档可作为商业客户 onboarding 材料

### 验收

1. 灰度过程中登录、API Key 创建、Agent 检索和日志查看稳定。
2. API Key 明文一次性展示和吊销失效验证通过。
3. 回滚演练没有破坏 Phase 3 强隔离底线。
4. Phase 4 验收材料齐全。
5. Phase 5 可以直接围绕商业化和企业能力继续推进。

---

## 6. 推荐实施节奏（1 周）

## 6.1 阶段推进建议

1. 第 0.5 天完成 `L0`，冻结接口契约、路由、会话策略、E2E 数据和失败用例。
2. 第 1-2 天完成 `L1 + L2`，打通登录注册、JWT 会话、API client refresh 和路由保护。
3. 第 2-4 天完成 `L3`，实现 API Key 列表、创建、一次性明文展示、吊销和接入提示。
4. 第 3-5 天完成 `L4 + L5`，实现租户设置、用量页面、配额提示和角色可见性。
5. 第 5-6 天完成 `L6`，补齐接入文档、SDK 示例、curl smoke 和错误码手册。
6. 第 6-7 天完成 `L7 + L8`，执行 E2E、失败路径、灰度、回滚演练和验收记录。

## 6.2 并行与合流规则

1. 可并行：`L1` 认证页面和 `L2` API client，`L3` API Key 页面和 `L6` 接入文档，`L4` 用量页面和 `L5` 权限矩阵。
2. 必须串行：`L3` 依赖 `L1/L2` 会话稳定，`L4` 依赖 Phase 3 配额口径，`L7` 依赖 `L1-L6` 全部通过。
3. 合流条件：登录注册、API client、API Key 页面、租户用量、权限可见性、接入文档和 smoke 脚本全部通过后，才允许进入 Phase 4 验收。
4. Code review 重点：token 存储、API Key 明文生命周期、权限按钮、`tenant_id` 可信来源、错误码处理和文档示例可运行性。

---

## 7. 角色分工（建议）

1. 前端 A：`L1 + L2`，负责登录注册、Auth Store、API client refresh、路由保护和登出。
2. 前端 B：`L3 + L5`，负责 API Key 页面、一次性 Key 展示、权限可见性和知识库联动。
3. 前端 C：`L4`，负责租户设置、用量页面、配额提示和 Admin Shell 租户摘要。
4. 后端 A：补齐或确认 `GET /v1/tenant`、`GET /v1/tenant/usage`、API Key rotate、logout 等契约缺口。
5. 后端 B：协助验证 `/v1/retrieve`、API Key permissions、配额错误和日志字段。
6. 文档/SDK：`L6`，负责接入文档、Go SDK README、Python 示例和 smoke 脚本。
7. QA/SRE：`L7 + L8`，负责 E2E、失败路径、灰度观察、回滚演练和验收记录。

补充协作约束：

1. 前后端先冻结注册后行为和用量接口契约，再实现页面。
2. API Key 页面必须和文档一起验收，确保 UI 创建出来的 Key 能直接复制到文档示例中运行。
3. 任何为了演示而引入的 mock 数据必须在合流前删除或标记为契约缺口。
4. 认证、API Key、用量、权限相关变更必须补充至少一条失败路径验证。

---

## 8. 阶段验收模板（执行后填写）

1. 功能完成情况（按 L0-L8）：
2. 认证闭环验证：
   - 注册：
   - 登录：
   - refresh：
   - `/v1/auth/me`：
   - 登出：
   - 未登录路由保护：
3. API client 验证：
   - 自动附带 JWT：
   - 401 refresh：
   - refresh 失败登出：
   - 403 提示：
   - 429 配额提示：
4. API Key 页面验证：
   - 列表：
   - 创建：
   - 一次性明文展示：
   - 复制：
   - 更新：
   - 吊销：
   - 轮换：
5. 租户和用量验证：
   - 租户详情：
   - plan/status：
   - API 调用用量：
   - 知识库数：
   - 文档数：
   - 存储：
   - limits：
6. 权限可见性验证：
   - owner：
   - admin：
   - member：
   - viewer：
   - 直接构造越权请求：
7. 接入文档验证：
   - cURL：
   - Go SDK：
   - Python requests：
   - Agent 后端示例：
   - 错误码：
8. `/v1/retrieve` 验证：
   - UI 创建 Key 检索成功：
   - revoked Key 失败：
   - `kb_ids` 越权失败：
   - 配额超限失败：
9. 日志验证：
   - `tenant_id`：
   - `app_id`：
   - `api_key_id`：
   - `auth_type`：
   - `source_api`：
   - `permission_result`：
10. 自动化测试结果：
    - admin unit：
    - admin build：
    - backend auth：
    - backend apikey：
    - backend retrieve：
    - smoke 脚本：
11. 灰度与回滚演练结果：
12. 契约缺口记录：
    - 接口：
    - 字段：
    - 影响页面：
    - 是否阻塞 Phase 4 Gate：
13. 遗留风险与负责人：
14. 是否允许进入 Phase 5（是/否）：

---

## 9. Phase 4 完成后下一步（明确路线衔接）

下一阶段固定进入 Phase 5：商业化与高级企业能力，按以下顺序衔接：

1. 套餐管理：基于 Phase 4 的 `plan/limits/usage` 展示，补齐套餐配置、升级流程和运营规则。
2. 计费系统：基于可信用量统计建立账单、支付、欠费策略和发票能力。
3. OAuth 2.0：在邮箱密码登录稳定后，引入飞书、GitHub、Google 等第三方登录。
4. 复杂成员邀请：在角色和租户页面稳定后，引入邮件邀请、邀请链接、审批和域名限制。
5. Webhook 通知：在 API Key、配额和告警口径稳定后，引入配额预警、Key 风险和异常调用通知。
6. 企业安全增强：在审计和权限模型稳定后，引入 SSO、IP 白名单、Key 作用域模板和审计导出。

Phase 5 不应破坏 Phase 1-4 的认证、隔离、API Key Agent 接入和 Admin UI 基础闭环。

---

## 10. 文档维护规则（作为后续实现参考基线）

1. 任何 Phase 4 范围变更，先更新本文档，再改代码。
2. 修改登录、注册、refresh、me 响应字段时，必须同步更新 `L0/L1/L2/阶段验收模板`。
3. 修改 API Key 字段、权限结构、一次性展示策略或轮换行为时，必须同步更新 `L3/L6/L7`。
4. 修改租户详情、用量字段或配额错误结构时，必须同步更新 `L4/阶段验收模板`。
5. 修改角色权限矩阵时，必须同步更新 `L5`，并补充 owner/admin/member/viewer 验证。
6. 修改接入示例、SDK 参数或 `/v1/retrieve` 请求响应时，必须同步更新 `L6` 和 smoke 脚本。
7. Phase 5 实现商业化或企业能力时，以本文档的 Admin UI 闭环和 Agent API Key 接入方式作为唯一产品基线。
