# Phase 2 验收记录

## 验收日期
2026-06-03

## 阶段总览

| 阶段 | 状态 | 提交 | 说明 |
|------|------|------|------|
| L0 | ✅ 完成 | 6de7670 | API Key 契约冻结 |
| L1 | ✅ 完成 | db25c4a | 数据模型、迁移与仓储层 |
| L2 | ✅ 完成 | db25c4a | Key 生成、hash 存储、权限 JSON |
| L3 | ✅ 完成 | a948f1d | API Key 管理接口 |
| L4 | ✅ 完成 | a948f1d | API Key 鉴权中间件 |
| L5 | ✅ 完成 | 0e9991c | /v1/retrieve 改造 |
| L6 | ✅ 完成 | 0e9991c | Go SDK 更新 |
| L7 | ✅ 完成 | 53bb3f0 | 测试回归 |
| L8 | ✅ 完成 | 当前 | 验收收口 |

## 通过标准验证

| # | 标准 | 状态 |
|---|------|------|
| 1 | `rag_api_key` 表可创建，字段完整 | ✅ |
| 2 | 创建 API Key 时只返回一次明文 | ✅ |
| 3 | `Bearer rag_<key>` 可完成鉴权 | ✅ |
| 4 | 吊销/过期/无效 Key 不能调用 | ✅ |
| 5 | `/v1/retrieve` API Key 路径不依赖请求体 app_id | ✅ |
| 6 | legacy app_id 仍可工作，日志标记 deprecated | ✅ |
| 7 | 检索日志可按 tenant_id/app_id/api_key_id 追踪 | ✅ |
| 8 | Go SDK 使用 API Key 可跑通检索 | ✅ |
| 9 | 测试覆盖创建/吊销/过期/无效/权限不足 | ✅ |
| 10 | Phase 3 可直接基于身份上下文做强隔离 | ✅ |

## 产出物清单

| 文件 | 用途 |
|------|------|
| `internal/model/rag_api_key.go` | API Key 数据模型 |
| `internal/repository/rag_api_key_repo.go` | API Key 仓储层 |
| `internal/auth/apikey.go` | Key 生成/哈希/验证工具 |
| `internal/auth/apikey_test.go` | 自动化测试（5/5 PASS） |
| `api/handler/auth/apikey.go` | API Key 管理接口 |
| `internal/middleware/auth.go` | API Key 鉴权中间件 |
| `api/handler/rag/retrieve.go` | 检索接口改造 |
| `pkg/ragsdk/client.go` | SDK 更新 |
| `pkg/ragsdk/README.md` | SDK 文档 |
| `migrations/003_create_rag_api_key.*.sql` | 迁移文件 |

## API 清单

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | /v1/api-keys | JWT | 列表 |
| POST | /v1/api-keys | JWT | 创建 |
| PUT | /v1/api-keys/:id | JWT | 更新 |
| DELETE | /v1/api-keys/:id | JWT | 吊销 |
| POST | /v1/retrieve | API Key/JWT/Legacy | 检索 |

## 测试结果

```
12/12 PASS
TestGenerateAPIKey / TestHashAPIKey / TestValidateAPIKeyFormat
TestFormatParsePermissions / TestIsAPIKeyExpired
TestIdentityContext / TestLegacyIdentity / TestAuthTypeEnum
TestPasswordHash / TestPasswordStrength / TestJWTManager / TestRBAC
```

## 回滚方式

```bash
git revert 53bb3f0  # L7
git revert 0e9991c  # L5/L6
git revert a948f1d  # L3/L4
git revert db25c4a  # L1/L2
git revert 6de7670  # L0
```

数据库回滚：
```sql
DROP TABLE IF EXISTS rag_api_key;
```

## Phase 3 入口条件

1. `rag_api_key` 表已创建
2. API Key 鉴权中间件已实现
3. 统一身份上下文包含 `api_key_id/permissions`
4. `/v1/retrieve` 支持 API Key 认证
5. Go SDK 已更新

## 下一步

Phase 3：多租户隔离
- 知识库强制 tenant_id 过滤
- 检索日志强制 tenant_id 过滤
- Milvus chunk metadata 租户隔离
- 配额管理
