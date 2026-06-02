# Phase 1 详细功能实现路线（账号 + 租户最小闭环）

## 1. 文档定位

本文档是多租户平台改造 Phase 1 的执行手册，目标是把“账号 + 租户最小闭环”拆成可直接实施、可验收、可回滚的细颗粒任务路线。

它有三个用途：

1. 作为团队推进 Phase 1 注册、登录、JWT、租户与用户模型落地的统一执行文档。
2. 作为 Phase 2 API Key + Agent 接入 MVP 的真实用户、真实租户、统一身份上下文基线。
3. 作为 Phase 3 多租户强隔离前的身份可信入口，确保后续所有 `tenant_id` 均来自认证链路，而不是临时注入或猜测。

本文档风格与现有路线文档保持一致：
目标 -> 路线 -> 模块任务 -> 验收门禁 -> 下一步入口。

统一口径说明：

1. `账号 + 租户最小闭环` 固定指：用户可以注册、登录、刷新 Token、获取当前用户信息；第一个注册用户自动拥有一个租户并成为 `owner`。
2. `租户用户模型` 固定指：`rag_tenant` 与 `rag_user` 两张表，以及后续可被 Phase 2/3 复用的 `tenant_id/user_id/role/status` 字段。
3. `Phase 1 JWT` 固定指：JWT claims 必须包含 `tenant_id/user_id/role/auth_type=jwt/token_type/exp/iat/iss`，并写入 Phase 0 冻结的 `统一身份上下文`。
4. `统一身份上下文` 沿用 Phase 0 口径：`auth_type/user_id/tenant_id/role/app_id/api_key_id/permissions/is_legacy`。
5. `Admin API 接入 JWT` 固定指：管理端路由优先读取真实 JWT 身份；`dev_admin_bypass` 只保留为本地兜底，生产环境不得启用。
6. `基础 RBAC` 固定指：先实现 `owner/admin/member/viewer` 的最小权限判断，不做复杂资源级授权和邀请审批。
7. `Bootstrap 落库` 固定指：Phase 0 已完成配置校验；Phase 1 需要真正创建或校验 `rag_tenant/rag_user` 中的测试 Owner。
8. `Phase 1 回归` 固定指：注册、登录、刷新、登出、密码修改、JWT 中间件、Admin API 身份切换、基础 RBAC、生产配置门禁的自动化验证。

---

## 2. Phase 1 范围边界

## 2.1 本阶段必须完成

1. 创建 `rag_tenant` 与 `rag_user` 数据模型，包含状态、角色、密码哈希、租户 slug、套餐占位和基础时间字段。
2. 实现 `POST /v1/auth/register`，注册时创建租户和 `owner` 用户，邮箱全局唯一。
3. 实现 `POST /v1/auth/login`，完成邮箱密码校验并返回 `access_token`、`refresh_token` 与用户身份摘要。
4. 实现 `POST /v1/auth/refresh`、`POST /v1/auth/logout`、`GET /v1/auth/me`、`PUT /v1/auth/password` 的最小可用版本。
5. 升级 `backend/internal/middleware/jwt.go`，让 JWT claims 与请求上下文包含 `tenant_id/auth_type/token_type`。
6. 将 Phase 0 的 `auth.Identity` 接入 Hertz 请求上下文，业务 handler 可以统一读取身份字段。
7. 将 `/api/admin/*` 管理端路由逐步切到真实 JWT 身份，保留 `dev_admin_bypass` 作为本地兜底。
8. 实现基础 RBAC 中间件或 helper，覆盖租户读取、租户修改、成员读取、知识库管理、日志读取等最小权限。
9. 将 `auth.BootstrapAdmin` 从“配置校验”升级为“幂等创建测试 Owner”。
10. 增加 Phase 1 自动化测试、curl smoke 与验收记录，证明 Phase 2 可以直接接 API Key。

## 2.2 本阶段明确不做

1. 不实现真实 API Key CRUD、Key hash 存储、吊销、轮换和 Agent 鉴权，该能力属于 Phase 2。
2. 不做所有知识库、文档、任务、日志表的强制 `tenant_id` 过滤和 Milvus metadata 隔离，该能力属于 Phase 3。
3. 不实现复杂成员邀请流程，如邮件邀请、邀请链接、组织审批和域名限制。
4. 不实现 OAuth 2.0、企业 SSO、IP 白名单和 Webhook。
5. 不实现套餐计费、账单、支付和商业化运营。
6. 不移除 legacy `app_id` 白名单；Phase 1 只保证它不会伪装成真实 JWT 用户。
7. 不完成完整 Admin UI 产品闭环；Phase 1 只保证管理 API 和 UI 可以用真实 JWT 对接。

---

## 3. 目标与通过标准（Gate）

Phase 1 通过标准（全满足）：

1. 新用户通过 `POST /v1/auth/register` 注册后，系统自动创建一个 `active` 租户，用户角色为 `owner`，响应能返回 `tenant_id/user_id/role`。
2. 登录接口使用 bcrypt 校验密码，错误密码、停用用户、停用租户都不能获得 Token。
3. `access_token` 与 `refresh_token` 可区分 `token_type`，过期时间符合配置；refresh token 不能直接访问普通业务接口。
4. JWT claims 与业务上下文都能读取 `tenant_id/user_id/role/auth_type=jwt`，后续查询不需要再猜用户属于哪个租户。
5. `/api/admin/*` 在真实 JWT 可用时不依赖固定 `user_id=1`；生产环境不存在默认 Admin 放行。
6. Bootstrap 管理员可以幂等写入 `rag_tenant/rag_user`，重复启动不会重复创建，弱密码与生产默认 bootstrap 仍被拒绝。
7. 基础 RBAC 能区分 `owner/admin/member/viewer`，无权限操作返回 `403 forbidden` 或项目冻结的等价错误码。
8. Phase 1 回归测试覆盖注册、登录、刷新、登出、密码修改、Token 过期、权限不足、dev bypass 降级和生产配置门禁。
9. Phase 2 可以直接基于 `tenant_id/user_id/role/auth_type` 创建 API Key，不需要重做账号或租户模型。

---

## 4. 实现路线总览（L0 -> L8）

Phase 1 按 9 条路线推进，按门禁顺序合流：

1. L0：Phase 0 基线确认、契约差异冻结与迁移入口准备
2. L1：`rag_tenant/rag_user` 数据模型、迁移与仓储层
3. L2：密码安全、Token 签发与 JWT claims 升级
4. L3：注册接口与租户 owner 创建闭环
5. L4：登录、刷新、登出、当前用户与修改密码接口
6. L5：JWT 中间件、统一身份上下文与 Admin API 接入
7. L6：基础 RBAC、租户状态门禁与错误码收敛
8. L7：Bootstrap 落库、测试脚本、日志与回归门禁
9. L8：灰度发布、回滚预案与 Phase 1 验收收口

建议顺序：`L0 -> L1 + L2 -> L3 + L4 -> L5 + L6 -> L7 -> L8`

---

## 5. 详细路线拆解

## 5.1 L0 Phase 0 基线确认、契约差异冻结与迁移入口准备

### 目标

在开始写注册登录前，冻结 Phase 0 交接物和 Phase 1 最终契约，避免代码中的临时结构体、主 roadmap 字段和实际接口响应出现三套口径。

### 功能任务

1. 确认 Phase 0 交接物可用：
   - `backend/docs/zhuhu/phase0-acceptance-record.md`
   - `backend/internal/auth/context.go`
   - `backend/internal/auth/contract.go`
   - `backend/internal/auth/bootstrap.go`
   - `backend/internal/config/config.go`
   - `backend/cmd/rag-server/main.go`
2. 冻结 Phase 1 API 响应口径：
   - 注册响应是否使用 `user.id` 嵌套结构，还是当前 `RegisterResponse.user_id`
   - 登录响应是否返回 `user` 嵌套结构，还是当前 `LoginResponse.user_id/role/tenant_id`
   - 错误码使用 lowercase 对外口径，或映射当前 `ErrCodeInvalidCredentials` 等内部常量
3. 冻结 JWT 配置来源：
   - `rag.auth.jwt_secret`
   - `security.jwt_secret`
   - `JWT_SECRET`
   - access token TTL
   - refresh token TTL
4. 冻结迁移入口：
   - 如果当前没有迁移目录，先建立 `backend/migrations`
   - 迁移文件编号建议：`001_create_rag_tenant.up.sql`、`002_create_rag_user.up.sql`
   - down 文件必须可回滚 Phase 1 新表
5. 冻结首批索引与唯一约束：
   - `rag_tenant.slug` 全局唯一
   - `rag_user.email` 全局唯一
   - `rag_user.tenant_id/status/role` 常用查询索引
6. 输出 Phase 1 契约冻结记录：
   - API 字段
   - JWT claims
   - 错误码映射
   - 迁移目录
   - 回滚方式

### 验收

1. Phase 1 开发前只存在一份 API 字段口径。
2. `auth/contract.go` 与 `multi-tenant-roadmap.md` 的字段差异已经明确取舍。
3. JWT secret、过期时间与 token type 的配置来源清楚。
4. 数据库迁移目录和文件命名确定，后续实现不再用临时 SQL 片段绕过。

---

## 5.2 L1 `rag_tenant/rag_user` 数据模型、迁移与仓储层

### 目标

落地账号与租户的最小数据底座，让注册、登录、bootstrap 和后续 API Key 都能读写同一套真实模型。

### 功能任务

1. 创建 `rag_tenant` 表：
   - `id`
   - `name`
   - `slug`
   - `plan`
   - `status`
   - `max_kb_count`
   - `max_doc_count`
   - `max_storage_mb`
   - `max_api_calls_per_day`
   - `created_at`
   - `updated_at`
2. 创建 `rag_user` 表：
   - `id`
   - `tenant_id`
   - `email`
   - `password_hash`
   - `name`
   - `role`
   - `status`
   - `last_login_at`
   - `created_at`
   - `updated_at`
3. 建议新增模块目录：
   - `backend/internal/tenant/model.go`
   - `backend/internal/tenant/repository.go`
   - `backend/internal/tenant/service.go`
   - `backend/internal/user/model.go`
   - `backend/internal/user/repository.go`
   - `backend/internal/user/service.go`
4. 实现租户仓储方法：
   - `CreateTenant`
   - `GetTenantByID`
   - `GetTenantBySlug`
   - `UpdateTenantStatus`
   - `GenerateUniqueSlug`
5. 实现用户仓储方法：
   - `CreateUser`
   - `GetUserByEmail`
   - `GetUserByID`
   - `GetUserWithTenant`
   - `UpdateLastLoginAt`
   - `UpdatePasswordHash`
6. 事务边界：
   - 注册时租户与 owner 用户必须在同一事务提交
   - 邮箱冲突时不得创建孤儿租户
   - slug 冲突时可重试生成，不得覆盖已有租户
7. 默认值策略：
   - `tenant.plan=free`
   - `tenant.status=active`
   - `user.role=owner/admin/member/viewer`
   - `user.status=active`

### 验收

1. 迁移可在空库创建 `rag_tenant/rag_user`，重复执行不会破坏已有表。
2. down 迁移可在测试库回滚 Phase 1 新表。
3. 注册事务失败不会留下只有租户没有用户的脏数据。
4. 用户查询能一次拿到租户状态，供登录和 JWT 签发判断。

---

## 5.3 L2 密码安全、Token 签发与 JWT claims 升级

### 目标

把当前只包含 `user_id/username/role` 的 JWT 升级为多租户身份 Token，并保证密码存储、Token 类型和过期行为可测试。

### 功能任务

1. 新增或完善密码工具：
   - `HashPassword`
   - `VerifyPassword`
   - bcrypt cost 建议不低于 12
   - 注册与修改密码共用同一套强度校验
2. 密码强度规则：
   - 最少 12 位，或与 Phase 0 bootstrap 规则保持一致
   - 禁止常见弱密码
   - 建议包含大小写字母、数字或特殊字符
3. 升级 `backend/internal/middleware/jwt.go` 的 `JWTClaims`：
   - `tenant_id`
   - `user_id`
   - `email` 或 `username`
   - `role`
   - `auth_type=jwt`
   - `token_type=access/refresh`
   - `iss`
   - `iat`
   - `exp`
4. 拆分 Token 生成方法：
   - `GenerateAccessToken`
   - `GenerateRefreshToken`
   - `GenerateTokenPair`
   - refresh token 不能作为普通业务接口凭证
5. 配置接入：
   - `access_token_ttl`
   - `refresh_token_ttl`
   - `jwt_issuer`
   - `jwt_secret`
6. Token 校验：
   - 校验签名算法
   - 校验过期时间
   - 校验 `token_type`
   - 校验用户与租户仍为 `active`
7. 登出策略第一版：
   - 无 Redis 黑名单时可先做客户端丢弃 Token
   - 如已有 Redis，可记录 access token jti 黑名单
   - 文档必须说明第一版登出能力边界

### 验收

1. 明文密码不会写入数据库、日志或错误响应。
2. access token 与 refresh token claims 可区分。
3. refresh token 调用普通 Admin API 会被拒绝。
4. 停用用户或停用租户即使 Token 未过期，也不能继续登录或刷新。
5. Token 过期、签名错误、格式错误都有明确 `401` 错误。

---

## 5.4 L3 注册接口与租户 owner 创建闭环

### 目标

实现第一个用户注册即拥有租户的闭环，为 Admin UI 和后续 API Key 创建提供真实 owner 身份。

### 功能任务

1. 注册路由：
   - `POST /v1/auth/register`
   - 挂载到现有路由注册入口，建议集中在 `backend/api/ragrouter/register.go` 或同类 auth router
2. 请求字段：
   - `email`
   - `password`
   - `name`
   - `tenant_name`
3. 输入校验：
   - 邮箱格式
   - 密码强度
   - 用户名非空
   - 租户名非空；缺失时可从邮箱域名或用户名派生，但必须记录规则
4. 创建租户：
   - `name=tenant_name`
   - `slug` 从租户名派生
   - 冲突时追加短后缀
   - 默认 `plan=free/status=active`
5. 创建 owner 用户：
   - `tenant_id=<new tenant id>`
   - `email`
   - `password_hash`
   - `name`
   - `role=owner`
   - `status=active`
6. 注册响应：
   - 返回用户摘要
   - 返回租户摘要
   - 返回 `access_token/refresh_token/expires_in`
   - 不返回 `password_hash`
7. 审计与日志：
   - `auth.register`
   - `tenant_id`
   - `user_id`
   - `email`
   - 不记录明文密码

### 验收

1. 注册成功后数据库中同时存在一个租户和一个 owner 用户。
2. 相同邮箱重复注册返回冲突错误，不创建新租户。
3. 注册响应中的 JWT 可以直接访问需要登录的 `/v1/auth/me`。
4. 弱密码、非法邮箱、空租户名都有明确错误。
5. 注册失败不会留下孤儿租户或孤儿用户。

---

## 5.5 L4 登录、刷新、登出、当前用户与修改密码接口

### 目标

补齐邮箱密码登录后的基础账号操作，让 Admin UI 可以使用真实 JWT 完成日常会话管理。

### 功能任务

1. 登录接口：
   - `POST /v1/auth/login`
   - 通过 `email` 查询 `rag_user`
   - 校验 `password_hash`
   - 校验 `user.status=active`
   - 校验 `tenant.status=active`
   - 更新 `last_login_at`
   - 返回 Token pair 与用户摘要
2. 刷新接口：
   - `POST /v1/auth/refresh`
   - 仅接受 `token_type=refresh`
   - 重新查询用户和租户状态
   - 返回新的 Token pair
3. 登出接口：
   - `POST /v1/auth/logout`
   - 如果没有黑名单能力，返回成功并记录 `stateless_logout=true`
   - 如果启用黑名单，记录 token jti 与过期时间
4. 当前用户接口：
   - `GET /v1/auth/me`
   - 返回 `id/email/name/role/tenant_id/tenant/last_login_at/created_at`
   - 不返回敏感字段
5. 修改密码接口：
   - `PUT /v1/auth/password`
   - 校验旧密码
   - 校验新密码强度
   - 更新 `password_hash`
   - 建议使旧 refresh token 失效；若暂未实现，写入验收风险
6. 错误处理：
   - 邮箱不存在与密码错误建议统一为 `invalid_credentials`
   - 停用用户返回 `403 user_suspended`
   - 停用租户返回 `403 tenant_suspended`
7. 审计日志：
   - `auth.login`
   - `auth.login_failed`
   - `auth.refresh`
   - `auth.logout`
   - `auth.password_change`

### 验收

1. 正确邮箱密码可以登录并返回可用 Token pair。
2. 错误密码不能泄露“邮箱是否存在”的细节。
3. refresh token 可以换取新 Token，access token 不能调用 refresh 接口。
4. `/v1/auth/me` 返回的租户信息与 JWT claims 一致。
5. 修改密码后旧密码不能再登录，新密码可以登录。

---

## 5.6 L5 JWT 中间件、统一身份上下文与 Admin API 接入

### 目标

让管理端业务 handler 从真实 JWT 获取身份，逐步摆脱固定 `user_id=1` 与临时 Admin 注入。

### 功能任务

1. 升级 JWT 中间件写入字段：
   - `jwt_claims`
   - `user_id`
   - `tenant_id`
   - `role`
   - `auth_type=jwt`
   - `token_type`
2. 接入 Phase 0 `auth.Identity`：
   - 在 Hertz request context 写入统一身份
   - 保留旧 `GetUserID` 兼容
   - 增加或接入 `GetTenantID/GetRole/GetAuthType`
3. 收窄 Token 提取渠道：
   - Phase 1 正式入口优先只支持 `Authorization: Bearer <jwt>`
   - `X-Auth-Token/query/cookie` 如需保留，必须标记为兼容路径并限制环境
4. Admin 路由接入：
   - `/api/admin/*` 优先走 JWT
   - 无 JWT 时只在 `dev` 且 `dev_admin_bypass_enabled=true` 时允许 bypass
   - `prod/staging` 不允许默认 Admin bypass
5. Handler 改造原则：
   - 管理端读取 `tenant_id/user_id/role`
   - 不在 handler 内重新解析 JWT
   - 不把 legacy `app_id` 当作登录用户
6. 日志字段：
   - `request_id`
   - `auth_type`
   - `tenant_id`
   - `user_id`
   - `role`
   - `route`
7. 降级策略：
   - JWT 中间件异常时不回退生产默认 Admin
   - 本地 UI 调试可临时开启 dev bypass
   - 日志必须明显标记 `auth_type=dev_admin_bypass`

### 验收

1. 使用注册或登录得到的 JWT 可以访问需要登录的 Admin API。
2. 缺失 JWT 的 Admin 请求在生产配置下返回未认证。
3. `tenant_id` 能从中间件传到业务 handler。
4. dev bypass 与真实 JWT 在日志中可区分。
5. handler 不再依赖固定 `user_id=1` 才能工作。

---

## 5.7 L6 基础 RBAC、租户状态门禁与错误码收敛

### 目标

把“已登录”升级为“有对应角色权限”，防止 Phase 1 刚上线就出现 viewer 可以修改租户或 member 可以删除成员的权限漏洞。

### 功能任务

1. 建立权限定义：
   - `tenant:read`
   - `tenant:write`
   - `member:read`
   - `member:invite`
   - `member:remove`
   - `member:role`
   - `kb:read`
   - `kb:write`
   - `kb:delete`
   - `log:read`
   - `audit:read`
2. 建立角色映射：
   - `owner` 拥有租户、成员、知识库、日志、审计的完整权限
   - `admin` 可管理成员、知识库和日志，但不能转移 owner 或修改租户核心设置
   - `member` 可读租户、读成员、管理自己可访问的知识库
   - `viewer` 只读租户、成员、知识库和检索能力
3. 实现权限 helper 或中间件：
   - `RequireAuth`
   - `RequireRole`
   - `RequirePermission`
   - `RequireActiveTenant`
4. 租户状态门禁：
   - `tenant.status=active` 正常访问
   - `tenant.status=suspended` 禁止管理和检索
   - `tenant.status=deleted` 视为不可访问
5. 用户状态门禁：
   - `user.status=active` 正常访问
   - `user.status=suspended/deleted` 拒绝登录、刷新和业务请求
6. 错误码收敛：
   - 未认证：`401 unauthorized`
   - Token 无效：`401 invalid_token`
   - 权限不足：`403 forbidden`
   - 租户停用：`403 tenant_suspended`
   - 邮箱冲突：`409 email_exists`
   - 弱密码：`422 weak_password`
7. 测试覆盖：
   - owner/admin/member/viewer 权限矩阵
   - 停用租户
   - 停用用户
   - 缺失身份上下文

### 验收

1. `viewer` 不能执行写操作。
2. 非 `owner` 不能修改租户核心设置或转移 owner 角色。
3. 停用租户下的用户不能继续使用旧 Token 访问管理接口。
4. 权限错误与认证错误可区分。
5. Phase 2 创建 API Key 时可以复用同一套 RBAC 判断。

---

## 5.8 L7 Bootstrap 落库、测试脚本、日志与回归门禁

### 目标

把 Phase 1 的账号租户能力固化为可重复验证的测试入口，确保新环境、CI 和本地开发都不依赖手工改库。

### 功能任务

1. 升级 `backend/internal/auth/bootstrap.go`：
   - 检查 `rag_tenant/rag_user` 是否存在
   - 根据 `BOOTSTRAP_TENANT_NAME` 创建或查找租户
   - 根据 `BOOTSTRAP_ADMIN_EMAIL` 创建或查找用户
   - 已存在时校验角色为 `owner` 或输出修复建议
   - 不覆盖已有用户密码，除非显式配置允许
2. Bootstrap 幂等规则：
   - 重复启动不重复创建
   - 同邮箱不同租户时拒绝并提示
   - 同 slug 租户存在时复用或生成后缀，规则必须固定
3. 更新测试脚本：
   - 注册
   - 登录
   - refresh
   - me
   - 修改密码
   - Admin API JWT 访问
   - dev bypass 本地兜底
4. 自动化测试：
   - `internal/auth`
   - `internal/middleware`
   - `internal/tenant`
   - `internal/user`
   - `api` auth handler
5. 日志与审计字段：
   - `auth_type`
   - `tenant_id`
   - `user_id`
   - `role`
   - `action`
   - `request_id`
6. 回归报告：
   - 注册登录结果
   - Token claims 快照
   - Admin API 接入情况
   - RBAC 权限矩阵结果
   - 遗留风险

### 验收

1. 新环境首次启动可以获得一个真实落库的测试 Owner。
2. 本地脚本能从注册跑到 Admin API JWT 访问。
3. 自动化测试能稳定覆盖 Phase 1 Gate。
4. 日志中能区分 bootstrap、jwt、dev bypass、legacy app_id。
5. Phase 1 回归报告可作为进入 Phase 2 的 Gate 材料。

---

## 5.9 L8 灰度发布、回滚预案与 Phase 1 验收收口

### 目标

确保账号租户闭环上线后不阻断现有开发测试，并且出现认证异常时能快速回退到 Phase 0 安全基线。

### 功能任务

1. 灰度顺序：
   - 本地 `dev` 环境启用注册登录
   - 本地 Admin UI 改用 JWT
   - 测试环境启用 bootstrap 落库
   - staging 关闭 dev bypass，只验证真实 JWT
   - prod 只允许真实注册登录或受控 bootstrap 一次性初始化
2. 回滚顺序：
   - 关闭新注册入口或隐藏 UI 注册入口
   - 回退 Admin UI 到 dev bypass，仅限本地 `dev`
   - 保留数据库表，不执行生产 destructive down
   - 回滚 JWT 中间件时不得恢复生产默认 Admin
   - 必要时恢复 Phase 0 契约与 legacy `app_id` 兼容路径
3. 告警与观察：
   - 登录失败率异常
   - 注册失败率异常
   - Token 解析失败率异常
   - `tenant_suspended` 异常升高
   - `forbidden` 异常升高
4. 验收材料：
   - 迁移执行记录
   - API smoke 结果
   - RBAC 矩阵测试结果
   - Bootstrap 幂等测试结果
   - 回滚演练记录
5. Phase 2 交接：
   - `tenant_id/user_id/role` 已可信
   - `owner/admin/member` 可创建 API Key
   - API Key 表可引用 `tenant_id/user_id`
   - `/v1/retrieve` 可从 API Key 推导 `tenant_id/app_id`

### 验收

1. 灰度过程中注册、登录和 Admin API 访问稳定。
2. 认证异常时可以回到 Phase 0 的安全测试入口，但不恢复生产默认 Admin。
3. 所有 Phase 1 验收材料齐全。
4. Phase 2 可以直接开工，不需要重做账号、租户或 JWT 上下文。

---

## 6. 推荐实施节奏（1 周）

## 6.1 阶段推进建议

1. 第 0.5 天完成 `L0`，冻结契约差异、JWT 配置来源和迁移入口。
2. 第 1-2 天完成 `L1 + L2`，落地表结构、仓储层、密码工具和 Token 签发。
3. 第 3 天完成 `L3`，跑通注册创建租户和 owner 的事务闭环。
4. 第 4 天完成 `L4 + L5`，补齐登录、刷新、me、密码修改，并让 Admin API 读取真实 JWT。
5. 第 5 天完成 `L6 + L7`，补齐基础 RBAC、bootstrap 落库、自动化测试和脚本。
6. 第 5-6 天完成 `L8`，做灰度、回滚演练和 Phase 1 验收记录。

## 6.2 并行与合流规则

1. 可并行：`L1` 迁移与 `L2` Token 工具设计，`L3` 注册 handler 与 `L4` 登录 handler 的接口草稿，`L6` 权限矩阵与 `L7` 测试用例设计。
2. 必须串行：`L3` 依赖 `L1/L2`，`L5` 依赖 JWT claims 稳定，`L8` 依赖 `L1~L7` 全部通过。
3. 合流条件：注册、登录、JWT 中间件、Admin API JWT、基础 RBAC 和 bootstrap 落库全部通过后，才允许进入 Phase 2。
4. Code review 重点：密码存储、Token claims、生产 dev bypass 禁用、注册事务、权限矩阵和错误码映射必须统一审查。

---

## 7. 角色分工（建议）

1. 后端 A：`L1 + L3`，负责租户/用户模型、迁移、仓储和注册事务。
2. 后端 B：`L2 + L4`，负责密码工具、JWT Token pair、登录、刷新、登出、me 和修改密码。
3. 后端 C：`L5 + L6`，负责 JWT 中间件、统一身份上下文、Admin API 接入和基础 RBAC。
4. QA/SRE：`L0 + L7 + L8`，负责契约冻结、配置回归、bootstrap 幂等、灰度和回滚演练。
5. 前端/文档：配合 `L4 + L5`，更新 Admin UI 登录态、Token 存储、curl smoke 和接入说明。

补充协作约束：

1. 后端与前端先冻结注册、登录、me 的响应字段，再改 UI。
2. 后端与 QA 先冻结 access token 与 refresh token 的过期配置和测试方法。
3. 任何恢复默认 Admin 的需求只能限制在 `dev`，并必须保留 Phase 0 生产门禁。
4. Phase 1 不允许为了临时跑通 UI 把 `tenant_id=1` 写死到业务 handler。

---

## 8. 阶段验收模板（执行后填写）

1. 功能完成情况（按 L0-L8）：
2. 数据库迁移结果：
   - `rag_tenant`
   - `rag_user`
   - 索引与唯一约束
   - down 迁移演练
3. API 验证：
   - `POST /v1/auth/register`
   - `POST /v1/auth/login`
   - `POST /v1/auth/refresh`
   - `POST /v1/auth/logout`
   - `GET /v1/auth/me`
   - `PUT /v1/auth/password`
4. JWT claims 快照：
   - `tenant_id`
   - `user_id`
   - `role`
   - `auth_type`
   - `token_type`
   - `exp`
   - `iss`
5. Admin API 接入验证：
   - JWT
   - dev bypass
   - 无认证
   - 生产配置
6. RBAC 权限矩阵验证：
   - owner
   - admin
   - member
   - viewer
7. Bootstrap 验证：
   - 首次创建
   - 重复启动
   - 弱密码拒绝
   - 生产默认关闭
8. 自动化测试结果：
   - auth
   - middleware
   - tenant/user repository
   - handler smoke
   - config guard
9. 回滚演练结果：
   - 关闭注册入口
   - 关闭 bootstrap
   - dev bypass 本地兜底
   - JWT 中间件回退
10. 遗留风险与负责人：
11. 是否允许进入 Phase 2（是/否）：

---

## 9. Phase 1 完成后下一步（明确路线衔接）

下一阶段固定进入 Phase 2：API Key + Agent 接入 MVP，按以下顺序衔接：

1. 创建 `rag_api_key` 表，引用 Phase 1 已稳定的 `tenant_id/user_id`。
2. 实现 `GET/POST/PUT/DELETE /v1/api-keys`，创建时只返回一次明文 Key。
3. 实现 API Key 鉴权中间件，解析 `Authorization: Bearer rag_<key>`。
4. 统一 JWT 与 API Key 的身份输出，继续写入 `auth.Identity`。
5. `/v1/retrieve` 优先使用 API Key 推导 `tenant_id/app_id/api_key_id/permissions`。
6. legacy `app_id` 白名单继续作为兼容路径，但日志必须标记 `auth_type=legacy_app_id`。
7. SDK 与 Agent 接入文档切换到正式 API Key。

Phase 2 完成后，再进入 Phase 3 多租户强隔离 + 基础配额，补齐知识库、文档、日志、Milvus metadata 与配额层面的 `tenant_id` 强制过滤。

---

## 10. 文档维护规则（作为后续实现参考基线）

1. 任何 Phase 1 范围变更，先更新本文档，再改代码。
2. 修改注册、登录、刷新、me、密码接口字段时，必须同步更新 `L0/L3/L4/阶段验收模板`。
3. 修改 JWT claims、Token TTL、签名算法或 token type 时，必须同步更新 `L2/L5/阶段验收模板`。
4. 修改角色权限矩阵时，必须同步更新 `L6`，并补充 RBAC 测试。
5. 修改 bootstrap 行为时，必须同步更新 `L7`，并补充幂等和生产门禁测试。
6. Phase 2/3 实现时，以本文档的 `tenant_id/user_id/role/auth_type=jwt` 作为账号租户可信基线。
