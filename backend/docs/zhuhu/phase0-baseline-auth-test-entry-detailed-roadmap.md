# Phase 0 详细功能实现路线（基线修正与测试入口）

## 1. 文档定位

本文档是多租户平台改造 Phase 0 的执行手册，目标是把“基线修正与测试入口”拆成可直接实施、可验收、可回滚的细颗粒任务路线。

它有三个用途：

1. 作为团队启动 Phase 1 账号租户闭环前的统一安全基线文档。
2. 作为后续 JWT、API Key、多租户隔离接入时的认证上下文与接口契约基线。
3. 作为本地测试、开发期 Admin 入口、旧 `app_id` 白名单兼容策略的边界说明。

本文档风格与现有路线文档保持一致：
目标 -> 路线 -> 模块任务 -> 验收门禁 -> 下一步入口。

统一口径说明：

1. `开发期 Admin 后门` 固定指当前 `backend/cmd/rag-server/main.go` 中对 `/api/admin/*` 自动注入 `user_id=1/role=admin/username=admin` 的临时逻辑。
2. `测试管理员` 固定指通过 `BOOTSTRAP_ADMIN_EMAIL`、`BOOTSTRAP_ADMIN_PASSWORD`、`BOOTSTRAP_ADMIN_NAME` 和 `BOOTSTRAP_TENANT_NAME` 创建或校验的第一位测试 Owner。
3. `环境门禁` 固定指基于 `rag.environment` 与显式开关控制某个开发便利能力是否可用，生产环境必须 fail-fast；当前项目配置枚举为 `dev/staging/prod`，如需独立 `test` 环境必须先扩展配置校验。
4. `统一身份上下文` 固定指后续所有鉴权方式最终写入请求上下文的字段：`auth_type/user_id/tenant_id/role/app_id/api_key_id/permissions`。
5. `旧 app_id 白名单` 固定指当前 `backend/api/handler/rag/retrieve.go` 内的 `allowedAppIDs` 静态白名单与请求体 `app_id` 校验。
6. `Phase 0 接口契约冻结` 固定指先冻结字段、错误码、鉴权入口和兼容边界，不要求本阶段完成完整业务实现。
7. `Phase 0 回归` 固定指覆盖 `/api/admin/*`、`/api/kb/*`、`/v1/retrieve` 的本地、测试、生产配置差异验证。

---

## 2. Phase 0 范围边界

## 2.1 本阶段必须完成

1. 盘点当前 `/api/admin/*`、`/api/kb/*`、`/v1/retrieve` 的实际鉴权方式，并输出可追踪的基线清单。
2. 将开发期 Admin 自动注入能力限制在 `dev` 或测试进程显式覆盖的等价本地环境，并增加显式开关与生产 fail-fast。
3. 定义 bootstrap 管理员创建方案，确保测试环境不依赖手工改数据库或固定 `user_id=1`。
4. 冻结 `/v1/auth/*`、`/v1/api-keys`、`/v1/retrieve` 第一版请求、响应、错误码和鉴权边界。
5. 明确旧 `app_id` 白名单与新 API Key 的迁移边界，避免 Phase 2 引入双重身份歧义。
6. 更新本地 curl、Go SDK 示例和测试脚本，区分 JWT 管理端场景与 API Key Agent 场景。
7. 增加 Phase 0 回归测试与配置校验，证明生产环境不存在默认 Admin 后门。
8. 输出 Phase 0 验收记录，作为进入 Phase 1 的 Gate。

## 2.2 本阶段明确不做

1. 不实现完整注册、登录、成员管理、租户管理闭环，该能力属于 Phase 1。
2. 不实现真实 API Key CRUD、Key hash 存储、吊销和轮换，该能力属于 Phase 2。
3. 不做强制 `tenant_id` 数据过滤、Milvus metadata 租户隔离和租户配额，该能力属于 Phase 3。
4. 不改造完整 Admin UI 闭环，只保证测试入口与接口契约可被 UI 对接。
5. 不删除旧 `app_id` 白名单，只把它标记为兼容路径并记录迁移边界。
6. 不引入 OAuth、企业 SSO、计费、套餐和复杂邀请流程。

---

## 3. 目标与通过标准（Gate）

Phase 0 通过标准（全满足）：

1. `backend/cmd/rag-server/main.go` 中的 `/api/admin/*` 自动 Admin 注入只能在非生产环境且显式开关开启时生效。
2. 当 `rag.environment=prod` 时，任何默认 Admin 注入、弱 bootstrap 密码、缺失 JWT secret、明文默认密钥都必须启动失败或拒绝请求。
3. 测试管理员有确定创建入口，能通过配置或启动任务创建，不依赖手工 SQL 修改。
4. `/v1/auth/*`、`/v1/api-keys`、`/v1/retrieve` 的第一版字段、错误码和认证方式已经在文档中冻结。
5. `/v1/retrieve` 的旧 `app_id` 白名单兼容路径与未来 `Authorization: Bearer rag_<key>` 路径边界清楚，日志能区分 `auth_type=legacy`。
6. 本地 curl 和 `backend/pkg/ragsdk` 示例能分别说明 JWT 管理端、legacy `app_id`、API Key 预期调用方式。
7. 自动化测试至少覆盖：dev Admin 注入可用、prod Admin 注入禁用、bootstrap 配置校验、`app_id` 白名单校验、无认证请求拒绝。
8. Phase 1 可以直接基于本文档定义的 `统一身份上下文` 接入 JWT，不需要重新讨论字段口径。

---

## 4. 实现路线总览（L0 -> L8）

Phase 0 按 9 条路线推进，按门禁顺序合流：

1. L0：当前认证与路由基线盘点
2. L1：开发期 Admin 后门环境门禁
3. L2：Bootstrap 测试管理员方案
4. L3：统一身份上下文契约冻结
5. L4：第一版 API 契约冻结
6. L5：旧 `app_id` 白名单迁移边界
7. L6：本地脚本、SDK 示例与接入说明更新
8. L7：测试、日志与生产配置回归门禁
9. L8：灰度、回滚预案与 Phase 0 验收收口

建议顺序：`L0 -> L1 + L2 -> L3 + L4 + L5 -> L6 + L7 -> L8`

---

## 5. 详细路线拆解

## 5.1 L0 当前认证与路由基线盘点

### 目标
把当前可访问入口、认证方式、隐式默认身份和日志字段全部盘清，避免后续在不可信身份基础上继续扩展多租户能力。

### 功能任务

1. 输出路由清单：
   - `backend/api/ragrouter/register.go`
   - `/api/kb/*`
   - `/api/admin/kb/*`
   - `/v1/retrieve`
   - `/healthz`
   - `/readyz`
2. 标记每类路由当前身份来源：
   - `/api/admin/*` 来自临时注入 `user_id=1/role=admin`
   - `/api/kb/*` 读取 `ParseAndSetUserFromToken` 或已有上下文
   - `/v1/retrieve` 读取请求体 `app_id` 并使用静态白名单
3. 盘点当前 JWT 中间件字段：
   - `user_id`
   - `username`
   - `role`
   - 当前缺失 `tenant_id/auth_type/app_id/api_key_id/permissions`
4. 盘点当前公开检索契约：
   - 请求：`app_id/kb_id/kb_ids/query/top_k/strategy_profile/metadata_filter`
   - 响应：`request_id/items/strategy_version/request_cost`
   - item：`content/score/citation/source`
5. 盘点当前配置入口：
   - `backend/config.yaml`
   - `backend/config.example.yaml`
   - `backend/config.rag.example.yaml`
   - `backend/internal/config/config.go`
6. 输出 Phase 0 基线报告：
   - 路由
   - 鉴权方式
   - 默认身份
   - 生产风险
   - Phase 1/2/3 依赖关系

### 验收

1. 每个受保护路由都有明确的当前鉴权来源说明。
2. 所有默认身份与兼容身份均被标记为 `dev_only`、`legacy` 或 `future_replaced`。
3. 基线报告能回答“生产环境中哪个入口可能绕过真实登录”。
4. Phase 1 接入 JWT 前不需要重新盘点路由。

---

## 5.2 L1 开发期 Admin 后门环境门禁

### 目标
保留本地快速测试能力，但禁止生产环境通过默认 `user_id=1` 获得 Admin 权限。

### 功能任务

1. 增加显式配置开关：
   - `rag.auth.dev_admin_bypass_enabled`
   - 环境变量建议：`RAG_DEV_ADMIN_BYPASS_ENABLED`
2. 增加环境约束：
   - 仅允许 `rag.environment=dev` 或测试进程显式覆盖时生效
   - 如需正式支持 `rag.environment=test`，必须先更新 `backend/internal/config/config.go` 的环境枚举
   - `staging/prod` 中设置该开关必须 fail-fast
   - 未设置开关时默认关闭
3. 改造 `backend/cmd/rag-server/main.go` 中的 Admin 注入逻辑：
   - 先判断 `path` 是否为 `/api/admin/*`
   - 再判断环境与开关
   - 通过后才设置 `user_id/role/username/auth_type`
   - 不通过时交给真实 JWT 或后续认证中间件处理
4. 为默认 Admin 注入增加审计日志：
   - `auth_type=dev_admin_bypass`
   - `route=/api/admin/*`
   - `environment`
   - `request_id`
5. 增加生产保护：
   - `rag.environment=prod` 且 `RAG_DEV_ADMIN_BYPASS_ENABLED=true` 时启动失败
   - `JWTSecret` 为默认占位值时启动失败
   - 缺失 `JWTSecret` 时启动失败
6. 保留回退策略：
   - 如果本地 UI 暂未接 JWT，可临时开启 dev bypass
   - 开启时日志必须明显提示该能力不可用于生产

### 验收

1. `dev` 环境显式开启后，本地 `/api/admin/kb/bases` 等接口仍可快速测试。
2. `prod` 环境即使设置开关，也不能获得默认 Admin 权限。
3. 自动化测试覆盖 dev 可用、prod 禁用、开关缺省关闭三个场景。
4. 日志能清楚区分 `auth_type=dev_admin_bypass` 与真实 JWT。

---

## 5.3 L2 Bootstrap 测试管理员方案

### 目标
提供确定的测试管理员创建方式，为 Phase 1 注册登录闭环前的测试、验收和 UI 对接提供稳定入口。

### 功能任务

1. 增加 bootstrap 配置字段：
   - `BOOTSTRAP_ADMIN_EMAIL`
   - `BOOTSTRAP_ADMIN_PASSWORD`
   - `BOOTSTRAP_ADMIN_NAME`
   - `BOOTSTRAP_TENANT_NAME`
   - `BOOTSTRAP_ENABLED`
2. 定义 bootstrap 生效边界：
   - 仅 `dev/staging` 或测试进程显式覆盖可开启
   - `prod` 默认关闭
   - `prod` 如需创建首位 Owner，必须走显式一次性命令或人工审批流程
3. 定义密码与密钥校验：
   - 密码不少于 12 位
   - 禁止使用 `admin/admin123/password/123456` 等弱密码
   - 邮箱格式必须合法
4. 定义数据写入策略：
   - Phase 0 可以先写入测试身份表或后续 Phase 1 的 `rag_user/rag_tenant` 预留模型
   - 若表不存在，则只输出待创建计划，不静默跳过
   - 不覆盖已有同邮箱用户
5. 定义角色口径：
   - bootstrap 用户角色固定为 `owner`
   - 默认租户 slug 建议从 `BOOTSTRAP_TENANT_NAME` 派生
   - 写入上下文时包含 `tenant_id/user_id/role/auth_type=jwt`
6. 输出 bootstrap 操作日志：
   - 创建成功
   - 已存在跳过
   - 配置缺失
   - 弱密码拒绝
   - 生产环境拒绝

### 验收

1. 新环境首次启动可以获得一个确定的测试 Owner。
2. 重复启动不会重复创建用户或租户。
3. 弱密码和生产环境默认 bootstrap 均会被拒绝。
4. Phase 1 可以直接接管该用户模型，不需要迁移测试管理员身份口径。

---

## 5.4 L3 统一身份上下文契约冻结

### 目标
冻结后续 JWT、API Key、legacy `app_id` 三种认证来源的统一输出字段，让业务处理层不再关心认证来源差异。

### 功能任务

1. 定义统一上下文字段：
   - `auth_type`
   - `tenant_id`
   - `user_id`
   - `role`
   - `app_id`
   - `api_key_id`
   - `permissions`
   - `is_legacy`
2. 定义 `auth_type` 枚举：
   - `jwt`
   - `api_key`
   - `legacy_app_id`
   - `dev_admin_bypass`
   - `bootstrap`
3. 定义缺省值规则：
   - 未认证时不得设置 `user_id=1`
   - legacy `app_id` 可映射到系统租户，但必须设置 `is_legacy=true`
   - API Key 未上线前不得伪造 `api_key_id`
4. 定义上下文读取 helper：
   - 保留现有 `GetUserID`
   - 增加 `GetTenantID`
   - 增加 `GetAuthType`
   - 增加 `GetAppID`
   - 增加 `GetPermissions`
5. 定义错误处理：
   - 缺少认证返回 `401 unauthorized`
   - 身份合法但权限不足返回 `403 forbidden`
   - legacy 路径无授权 `app_id` 返回 `403 invalid_app_id`
6. 定义日志字段：
   - `request_id`
   - `auth_type`
   - `tenant_id`
   - `user_id`
   - `app_id`
   - `source_api`

### 验收

1. 业务 handler 可以只读取统一身份上下文，不直接解析 JWT 或 API Key。
2. legacy `app_id` 和 dev bypass 在日志中不可被误认为真实登录用户。
3. Phase 1 JWT 接入时只需要补齐 `tenant_id` 和真实用户查询。
4. Phase 2 API Key 接入时只需要补齐 `api_key_id/permissions`，不改变业务 handler 字段名。

---

## 5.5 L4 第一版 API 契约冻结

### 目标
冻结多租户平台第一版 API 字段，让 Admin UI、Agent、SDK 和测试脚本可以并行开发。

### 功能任务

1. 冻结认证 API：
   - `POST /v1/auth/register`
   - `POST /v1/auth/login`
   - `POST /v1/auth/logout`
   - `POST /v1/auth/refresh`
   - `GET /v1/auth/me`
   - `PUT /v1/auth/password`
2. 冻结 API Key API：
   - `GET /v1/api-keys`
   - `POST /v1/api-keys`
   - `PUT /v1/api-keys/:id`
   - `DELETE /v1/api-keys/:id`
3. 冻结公开检索 API：
   - `POST /v1/retrieve`
   - 请求字段：`query/kb_ids/top_k/strategy_profile/metadata_filter`
   - Phase 0 兼容字段：`app_id`
   - Phase 2 正式认证：`Authorization: Bearer rag_<key>`
4. 冻结响应契约：
   - `request_id`
   - `items`
   - `strategy_version`
   - `request_cost`
   - `items[].content`
   - `items[].score`
   - `items[].citation`
   - `items[].source`
5. 冻结错误码：
   - `400 bad_request`
   - `401 unauthorized`
   - `401 invalid_token`
   - `401 invalid_api_key`
   - `403 forbidden`
   - `403 invalid_app_id`
   - `403 tenant_suspended`
   - `404 not_found`
   - `409 conflict`
   - `422 weak_password`
   - `429 quota_exceeded`
   - `500 internal_error`
6. 输出 API contract 文档位置：
   - 建议在 `backend/docs/zhuhu` 下补充 `api-contract-v1.md`
   - Phase 0 本文档先作为字段冻结依据

### 验收

1. Admin UI 和 SDK 可以按冻结字段开始对接。
2. `/v1/retrieve` 不再新增临时字段绕过认证边界。
3. 错误码可以区分未认证、权限不足、legacy `app_id` 无效和 API Key 无效。
4. Phase 1/2 的实现不得擅自改字段名，确需变更必须先更新契约文档。

---

## 5.6 L5 旧 `app_id` 白名单迁移边界

### 目标
让当前 `app_id` 白名单继续支持短期测试，同时为 Phase 2 API Key 上线留出明确替换路径。

### 功能任务

1. 将 `allowedAppIDs` 从硬编码迁移到配置或集中模块：
   - 当前代码位置：`backend/api/handler/rag/retrieve.go`
   - 建议配置路径：`rag.auth.legacy_app_ids`
   - 环境变量建议：`RAG_LEGACY_APP_IDS`
2. 定义 legacy 身份上下文：
   - `auth_type=legacy_app_id`
   - `app_id=<request.app_id>`
   - `is_legacy=true`
   - `tenant_id=SYSTEM_TENANT_ID` 或测试租户占位
3. 定义 legacy 日志字段：
   - `source_api=v1`
   - `auth_type=legacy_app_id`
   - `app_id`
   - `request_id`
4. 定义兼容期限：
   - Phase 1/2 可共存
   - Phase 3 强隔离前必须完成 API Key 替换或为 legacy app 映射租户权限
   - Phase 5 再考虑强制下线
5. 定义禁止事项：
   - 不允许 legacy `app_id` 获得真实用户角色
   - 不允许 legacy 路径绕过 `kb_ids` 授权边界
   - 不允许把请求体 `app_id` 当作 API Key
6. 增加迁移提示：
   - 响应 header 可增加 `X-RAG-Auth-Mode: legacy_app_id`
   - 日志中提示 `deprecated=true`

### 验收

1. 当前 `interview-agent/mianshiba-web/mianshiba-admin` 仍能按配置通过白名单。
2. 无效 `app_id` 返回 `403 invalid_app_id`。
3. legacy 请求日志一定包含 `auth_type=legacy_app_id`。
4. Phase 2 API Key 上线后可以按 `app_id` 一对一迁移，不需要再追溯历史调用来源。

---

## 5.7 L6 本地脚本、SDK 示例与接入说明更新

### 目标
让开发、测试、Agent 接入方看到的是同一套认证边界，避免继续使用过期的默认 Admin 或请求体 `app_id` 当作正式认证方式。

### 功能任务

1. 更新本地 curl 示例：
   - Admin dev bypass 示例
   - JWT Admin 示例
   - legacy `app_id` 检索示例
   - API Key 预期检索示例
2. 更新 `backend/pkg/ragsdk/README.md`：
   - `APIKey` 是正式 Agent 认证方式
   - `AppID` 仅作为兼容或日志字段
   - Phase 2 后 SDK 自动带 `Authorization: Bearer rag_<key>`
3. 更新 `backend/pkg/ragsdk/client.go` 的契约说明：
   - 当前 `RetrieveRequest` 不应长期携带 `app_id`
   - `Client.AppID` 如保留，后续用于兼容 header 或日志，不作为主认证
4. 更新本地测试脚本：
   - 无认证请求
   - legacy app_id 请求
   - dev bypass admin 请求
   - prod 配置拒绝 dev bypass
5. 增加接入说明：
   - 用户不直接持有 API Key
   - Agent 后端持有 API Key
   - Admin UI 使用 JWT
   - legacy `app_id` 仅短期兼容
6. 标记废弃字段：
   - 请求体 `app_id` 标记为 `deprecated after Phase 2`
   - 文档中写明下线条件，而不是立即删除

### 验收

1. 开发人员能用脚本复现 dev/test/prod 三种认证表现差异。
2. SDK 文档不会误导 Agent 继续依赖请求体 `app_id`。
3. curl 示例全部带正确认证方式。
4. 文档明确“终端用户不直接持有 Key，Agent 后端持有 Key”。

---

## 5.8 L7 测试、日志与生产配置回归门禁

### 目标
把 Phase 0 的安全边界固化成自动化回归，防止后续 Phase 1/2 改造时重新引入默认 Admin 风险。

### 功能任务

1. 增加配置测试：
   - `dev` 环境允许显式 dev bypass
   - 测试进程显式覆盖时允许 dev bypass
   - `staging/prod` 禁止 dev bypass，除非 staging 有单独审批的真实 JWT 测试入口
   - `prod` 禁止默认 JWT secret
   - 弱 bootstrap 密码拒绝
2. 增加路由测试：
   - `/api/admin/*` 在 dev bypass 开启时可用
   - `/api/admin/*` 在 dev bypass 关闭时需要真实认证
   - `/v1/retrieve` 无 `app_id` 或无认证返回错误
   - `/v1/retrieve` legacy `app_id` 合法时可走兼容路径
3. 增加日志断言：
   - dev bypass 请求包含 `auth_type=dev_admin_bypass`
   - legacy 请求包含 `auth_type=legacy_app_id`
   - 真实 JWT 请求包含 `auth_type=jwt`
4. 增加回归清单：
   - `go test ./internal/config ./internal/middleware ./api/...`
   - SDK 示例测试
   - 本地 curl smoke
5. 增加生产配置检查：
   - `rag.environment=prod`
   - `rag.feature_flags.enable_prod_guard=true`
   - dev bypass 关闭
   - bootstrap 默认关闭
   - legacy 白名单有明确迁移计划
6. 输出 Phase 0 回归报告：
   - 通过项
   - 失败项
   - 风险项
   - 是否允许进入 Phase 1

### 验收

1. 回归测试可以稳定证明生产没有默认 Admin 后门。
2. 每种认证来源都有可观察日志字段。
3. 配置错误时不是静默降级，而是 fail-fast 或返回明确错误。
4. Phase 0 回归报告可以作为 Phase 1 开工前的 Gate 材料。

---

## 5.9 L8 灰度、回滚预案与 Phase 0 验收收口

### 目标
确保 Phase 0 改造不会阻断本地开发，同时能快速恢复旧测试入口，并且把风险控制在非生产环境。

### 功能任务

1. 灰度顺序：
   - 本地 `dev` 开启 dev bypass 验证
   - 测试进程显式覆盖或隔离测试配置开启 bootstrap 验证
   - `staging` 环境关闭 dev bypass，只验证真实 JWT 或待接认证
   - `prod` 环境只运行配置校验，不开启 bootstrap 和 dev bypass
2. 回滚策略：
   - dev bypass 改造异常时，仅在本地回退临时注入逻辑
   - 生产环境不得回退默认 Admin 后门
   - bootstrap 异常时关闭 `BOOTSTRAP_ENABLED`
   - legacy `app_id` 配置异常时回退到上一版白名单配置
3. 验收材料：
   - 基线盘点报告
   - 配置门禁测试结果
   - API 契约冻结记录
   - curl/SDK 示例
   - 回滚演练记录
4. Phase 1 交接：
   - JWT claims 需要补齐 `tenant_id`
   - Admin API 需要从 dev bypass 切到真实 JWT
   - bootstrap 用户需要进入 `rag_tenant/rag_user`
   - `/v1/auth/*` 开始真实实现
5. 风险留存：
   - legacy `app_id` 白名单仍在
   - API Key 未上线
   - 租户强隔离未上线
   - Admin UI 可能仍依赖本地 bypass

### 验收

1. 本地开发入口可用，但生产默认 Admin 后门不存在。
2. Phase 0 变更可以在非生产环境快速回退。
3. 验收材料齐全，Phase 1 可以直接开工。
4. 所有遗留风险都有明确 Phase 归属。

---

## 6. 推荐实施节奏（2-3 天）

## 6.1 阶段推进建议

1. 第 0.5 天完成 `L0`，输出路由与认证基线清单。
2. 第 1 天完成 `L1 + L2`，把开发期 Admin 后门和 bootstrap 管理员边界先守住。
3. 第 1.5 天完成 `L3 + L4 + L5`，冻结身份上下文、API 契约和 legacy 迁移边界。
4. 第 2 天完成 `L6 + L7`，补齐脚本、SDK 说明、配置测试和路由回归。
5. 第 2-3 天完成 `L8`，跑灰度、回滚演练和 Phase 0 验收记录。

## 6.2 并行与合流规则

1. 可并行：`L0` 基线盘点与 `L4` API 契约草稿，`L6` 文档示例与 `L7` 测试用例设计。
2. 必须串行：`L1` 必须在任何生产配置验证前完成，`L3` 必须在 Phase 1 JWT 实现前冻结。
3. 合流条件：`L1/L2/L3/L4/L5` 全部通过后，才允许把 Phase 1 的注册登录实现接入 Admin 路由。
4. Code review 重点：所有默认身份注入、认证绕过、环境开关、弱密钥检测必须统一审查。

---

## 7. 角色分工（建议）

1. 后端 A：`L0 + L1`，负责路由盘点、Admin bypass 门禁和生产 fail-fast。
2. 后端 B：`L2 + L3`，负责 bootstrap 方案和统一身份上下文。
3. 后端 C：`L4 + L5`，负责 API 契约、legacy `app_id` 配置化和日志字段。
4. SDK/文档：`L6`，负责 curl、Go SDK、Agent 接入说明。
5. QA/SRE：`L7 + L8`，负责配置测试、路由回归、灰度和回滚演练。

补充协作约束：

1. 后端和 QA 先冻结环境命名；当前实现使用 `dev/staging/prod`，需要 `test` 时先扩展配置枚举。
2. SDK 文档不得把请求体 `app_id` 写成正式鉴权方式。
3. 任何生产环境绕过认证的需求必须回到 Phase 0 Gate 重新评审。

---

## 8. 阶段验收模板（执行后填写）

1. 功能完成情况（按 L0-L8）：
2. 路由基线清单：
   - `/api/admin/*`
   - `/api/kb/*`
   - `/v1/retrieve`
   - `/healthz`
   - `/readyz`
3. 配置快照：
   - `rag.environment`
   - `rag.feature_flags.enable_prod_guard`
   - `rag.auth.dev_admin_bypass_enabled`
   - `BOOTSTRAP_ENABLED`
   - `RAG_LEGACY_APP_IDS`
4. 认证路径验证：
   - dev bypass
   - bootstrap
   - JWT
   - legacy app_id
   - API Key 预留
5. API 契约冻结记录：
   - `/v1/auth/*`
   - `/v1/api-keys`
   - `/v1/retrieve`
6. 自动化测试结果：
   - 配置测试
   - 路由测试
   - SDK 示例测试
   - curl smoke
7. 回滚演练结果：
   - dev bypass 关闭
   - bootstrap 关闭
   - legacy app_id 配置回退
8. 遗留风险与负责人：
9. 是否允许进入 Phase 1（是/否）：

---

## 9. Phase 0 完成后下一步（明确路线衔接）

下一阶段固定进入 Phase 1：账号 + 租户最小闭环，按以下顺序衔接：

1. 创建 `rag_tenant` 与 `rag_user` 数据模型。
2. 实现 `POST /v1/auth/register`，第一个注册用户自动成为租户 `owner`。
3. 实现 `POST /v1/auth/login` 与 `POST /v1/auth/refresh`。
4. JWT claims 补齐 `tenant_id/user_id/role/auth_type=jwt`。
5. Admin API 从 dev bypass 逐步切到真实 JWT。
6. bootstrap 管理员写入真实 `rag_tenant/rag_user`，并只作为测试入口。
7. 基础 RBAC 接入 `owner/admin/member/viewer`。

Phase 1 完成后，再进入 Phase 2 API Key + Agent 接入 MVP，替换 legacy `app_id` 白名单作为正式 Agent 认证方式。

---

## 10. 文档维护规则（作为后续实现参考基线）

1. 任何 Phase 0 范围变更，先更新本文档，再改代码。
2. 新增认证来源时，必须同步更新 `统一身份上下文` 字段和 `auth_type` 枚举。
3. 新增或删除 legacy 兼容路径时，必须同步更新 `L5` 和阶段验收模板。
4. 修改 `/v1/auth/*`、`/v1/api-keys`、`/v1/retrieve` 字段时，必须同步更新 `L4`。
5. 每次灰度或回滚后，必须补充阶段验收模板记录。
6. 后续实现 Phase 1/2/3 时，以本文档的 Phase 0 Gate 作为认证安全底线。
