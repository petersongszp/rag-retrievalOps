# Phase 4 L0 契约冻结记录

## 冻结日期
2026-06-03

## Phase 3 交接物确认

以下交接物已存在，可作为 Phase 4 开发基线：

- `backend/docs/zhuhu/phase3-acceptance-report.md`
- `backend/docs/zhuhu/phase3-multi-tenant-isolation-quota-detailed-roadmap.md`
- `backend/internal/auth/context.go`
- `backend/internal/auth/contract.go`
- `backend/internal/quota/quota.go`
- `backend/internal/repository/kb_tenant_repo.go`
- `backend/internal/repository/rag_retrieve_log_repo.go`
- `backend/api/handler/rag/retrieve.go`

## 认证接口契约

### 已落地接口

| 方法 | 路径 | 当前状态 | 响应要点 |
|------|------|----------|----------|
| POST | `/v1/auth/register` | 已实现 | 返回 `user_id/email/tenant_id` |
| POST | `/v1/auth/login` | 已实现 | 返回 `access_token/refresh_token/expires_in/user_id/role/tenant_id` |
| POST | `/v1/auth/refresh` | 已实现 | 返回与登录等价的 token 响应 |
| GET | `/v1/auth/me` | 已实现 | 返回 `user_id/email/name/role/tenant_id/tenant_name/created_at` |
| PUT | `/v1/auth/password` | 已实现 | 成功返回 message |

### 注册后行为冻结

Phase 4 首版按方案 B 实施：

- 注册成功后后端不直接返回 token
- 前端展示注册成功提示并跳转 `/login`
- 不假设注册后自动进入 Admin

### 已知限制

- 当前没有 `POST /v1/auth/logout`
- `login/refresh` 中的 `expires_in` 当前固定为 `7200` 秒，前端先按接口返回值消费，不自行推导 TTL

## API Key 接口契约

### 已落地接口

| 方法 | 路径 | 当前状态 | 说明 |
|------|------|----------|------|
| GET | `/v1/api-keys` | 已实现 | 列表仅返回 `key_prefix`，不返回明文 |
| POST | `/v1/api-keys` | 已实现 | 创建成功返回一次性明文 `key` |
| PUT | `/v1/api-keys/:id` | 已实现 | 支持更新 `name/permissions` |
| DELETE | `/v1/api-keys/:id` | 已实现 | 吊销 Key |

### 暂缺接口

| 方法 | 路径 | 当前状态 | Phase 4 处理策略 |
|------|------|----------|------------------|
| POST | `/v1/api-keys/:id/rotate` | 未实现 | 后端补齐后再开放前端轮换按钮；在此之前前端隐藏或置灰入口 |

### 一次性明文展示冻结

- 后端仅在 `POST /v1/api-keys` 响应中返回完整 `key`
- 列表与后续详情只展示 `key_prefix`
- 前端不得把完整 `key` 写入 `localStorage/sessionStorage/URL/全局 store`

## 租户与用量契约

### 当前可用契约

| 方法 | 路径 | 当前状态 | 说明 |
|------|------|----------|------|
| GET | `/v1/auth/me` | 已实现 | 可兜底提供 `tenant_id/tenant_name` |

### 当前缺口

| 方法 | 路径 | 当前状态 | Phase 4 处理策略 |
|------|------|----------|------------------|
| GET | `/v1/tenant` | 未实现 | 后端补齐；L4 完成前前端可用 `/v1/auth/me` 最小兜底 |
| GET | `/v1/tenant/usage` | 未实现 | 后端补齐；未补齐前页面明确显示“后端契约缺口”，不展示假数据 |
| PUT | `/v1/tenant` | 未实现 | 仅在后端实现后开放租户名修改 |

### 用量口径冻结

L4 页面与 L7 验收统一按以下字段展示或校验：

- `api_calls_today`
- `api_calls_this_month`（如后端返回）
- `kb_count`
- `doc_count`
- `storage_mb`
- `limits.max_kb_count`
- `limits.max_doc_count`
- `limits.max_storage_mb`
- `limits.max_api_calls_per_day`

## `/v1/retrieve` Phase 4 验收字段冻结

### 当前已具备

- 请求链路可解析并注入 `tenant_id/app_id/api_key_id/auth_type`
- 多租户 KB 权限校验已接入租户隔离基线

### 当前缺口

- 检索日志未稳定持久化 `tenant_id/app_id/api_key_id/source_api`
- `auth_type` 仅存在于请求上下文，未落到检索日志模型
- `permission_result` 尚未建模与落库
- `quota` 和 API Key `permissions.kb_ids` 还未形成完整验收闭环

### L7 前必须满足

- UI 创建出的 Key 调 `/v1/retrieve` 后，日志中可验证：
  - `tenant_id`
  - `app_id`
  - `api_key_id`
  - `auth_type`
  - `source_api`
  - `permission_result`

## 前端路由冻结

### 已存在管理路由

- `/dashboard`
- `/knowledge-bases`
- `/knowledge-bases/[kbId]`
- `/retrieval-lab`
- `/trace-logs/retrieval`
- `/trace-logs/ingest`
- `/evaluation/datasets`
- `/evaluation/runs`
- `/audit`
- `/alerts`

### Phase 4 新增路由

- `admin/src/app/(auth)/login/page.tsx`
- `admin/src/app/(auth)/register/page.tsx`
- `admin/src/app/(admin)/api-keys/page.tsx`
- `admin/src/app/(admin)/tenant/settings/page.tsx`
- `admin/src/app/(admin)/tenant/usage/page.tsx`
- `admin/src/app/(admin)/docs/integration/page.tsx`

### 根路由策略冻结

- 未登录访问 `/` 时最终应进入 `/login`
- 已登录访问 `/` 时进入 `/dashboard`
- `(admin)` 路由组统一做会话保护，不在单页散落硬编码保护

## 前端基础模块冻结

以下模块作为 Phase 4 核心改造入口：

- `admin/src/services/api/client.ts`
- `admin/src/config/api.ts`
- `admin/src/components/admin/admin-shell.tsx`
- `admin/src/components/admin/knowledge-base-provider.tsx`
- 新增 `admin/src/services/auth/session.ts`
- 新增 `admin/src/services/auth/store.tsx`
- 新增 `admin/src/types/auth.ts`
- 新增 `admin/src/types/tenant.ts`
- 新增 `admin/src/types/api-key.ts`
- 新增 `admin/src/components/auth/*`

## E2E 数据准备冻结

Phase 4 验收默认准备以下数据：

- 一个 bootstrap owner 或新注册 owner
- 一个测试租户
- 一个测试知识库
- 一份小文档
- 一个通过 UI 创建的 API Key
- 一个 `/v1/retrieve` 检索 query

## 失败用例冻结

- 未登录访问 Admin 页面
- access token 失效后访问受保护页面
- refresh token 失效导致安全登出
- viewer 创建 API Key
- revoked API Key 调 `/v1/retrieve`
- API Key `kb_ids` 越权检索
- 配额超限上传文档
- 配额超限调用 `/v1/retrieve`

## 风险与缺口清单

| 模块 | 缺口 | 是否阻塞后续 |
|------|------|--------------|
| Auth | 无 logout 远端失效接口 | 不阻塞 L1，记录限制 |
| API Key | rotate 接口缺失 | 阻塞 L3 的轮换闭环 |
| Tenant | `/v1/tenant` 缺失 | 阻塞 L4 正式契约 |
| Usage | `/v1/tenant/usage` 缺失 | 阻塞 L4 正式契约 |
| Retrieve Log | 审计字段未完整落库 | 阻塞 L7 最终验收 |
| Quota | 检索链路 quota 闭环未接齐 | 阻塞 L4/L7 完整验收 |

## L0 验收结论

- Phase 4 按方案 B 处理注册后行为
- 前端认证体系需要新建，不复用现有“未鉴权 Admin 壳层”假设
- 后端 `auth + api-keys` 可直接作为 L1-L3 基础
- L4 与 L7 前必须优先补齐 `tenant/usage/retrieve log` 契约缺口
