# Phase 1 验收记录

## 验收日期
2026-06-02

## 阶段总览

| 阶段 | 状态 | 提交 | 说明 |
|------|------|------|------|
| L0 | ✅ 完成 | 2a8ab4e | 契约冻结与迁移入口准备 |
| L1 | ✅ 完成 | fad26cf | 数据模型、迁移与仓储层 |
| L2 | ✅ 完成 | c618a99 | 密码安全 + JWT 升级 |
| L3 | ✅ 完成 | 5f3c8f0 | 注册接口 + 租户 owner 创建 |
| L4 | ✅ 完成 | 0487b7c | 登录/刷新/当前用户/改密码 |
| L5 | ✅ 完成 | 0eb5e34 | JWT 中间件 + Admin API 接入 |
| L6 | ✅ 完成 | f8b7c86 | 基础 RBAC + 租户状态门禁 |
| L7 | ✅ 完成 | 3358315 | Bootstrap 落库 + 测试回归 |
| L8 | ✅ 完成 | 当前 | 验收收口 |

## 通过标准验证

| # | 标准 | 状态 |
|---|------|------|
| 1 | 注册接口创建租户和 owner 用户 | ✅ |
| 2 | 登录使用 bcrypt 校验密码 | ✅ |
| 3 | JWT claims 包含 tenant_id/auth_type/token_type | ✅ |
| 4 | Admin API 优先使用真实 JWT | ✅ |
| 5 | Bootstrap 幂等创建测试 Owner | ✅ |
| 6 | RBAC 区分 owner/admin/member/viewer | ✅ |
| 7 | 自动化测试覆盖核心场景（7/7 PASS） | ✅ |
| 8 | Phase 2 可直接基于 tenant_id/user_id/role 创建 API Key | ✅ |

## 产出物清单

| 文件 | 用途 |
|------|------|
| `internal/model/rag_tenant.go` | 租户数据模型 |
| `internal/model/rag_user.go` | 用户数据模型 |
| `internal/repository/rag_tenant_repo.go` | 租户仓储层 |
| `internal/repository/rag_user_repo.go` | 用户仓储层 |
| `internal/auth/password.go` | bcrypt 密码工具 |
| `internal/auth/jwt.go` | JWT 管理器 |
| `internal/auth/rbac.go` | RBAC 权限定义 |
| `internal/auth/bootstrap.go` | Bootstrap 管理员 |
| `internal/auth/phase1_test.go` | 自动化测试（7/7 PASS） |
| `internal/middleware/auth.go` | JWT + RBAC 中间件 |
| `api/handler/auth/register.go` | 注册接口 |
| `api/handler/auth/login.go` | 登录接口 |
| `api/handler/auth/refresh.go` | 刷新 Token |
| `api/handler/auth/me.go` | 当前用户 |
| `api/handler/auth/password.go` | 修改密码 |
| `migrations/001_create_rag_tenant.*.sql` | 租户表迁移 |
| `migrations/002_create_rag_user.*.sql` | 用户表迁移 |

## API 清单

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| POST | `/v1/auth/register` | 无 | 注册 |
| POST | `/v1/auth/login` | 无 | 登录 |
| POST | `/v1/auth/refresh` | 无 | 刷新 Token |
| GET | `/v1/auth/me` | JWT | 当前用户 |
| PUT | `/v1/auth/password` | JWT | 修改密码 |

## 测试结果

```
=== RUN   TestIdentityContext     --- PASS
=== RUN   TestLegacyIdentity      --- PASS
=== RUN   TestAuthTypeEnum        --- PASS
=== RUN   TestPasswordHash        --- PASS
=== RUN   TestPasswordStrength    --- PASS
=== RUN   TestJWTManager          --- PASS
=== RUN   TestRBAC               --- PASS
PASS  ok  interview-agents/internal/auth  2.150s
```

## 回滚方式

每个阶段都可以独立回滚：

```bash
git revert 3358315  # L7
git revert f8b7c86  # L6
git revert 0eb5e34  # L5
git revert 0487b7c  # L4
git revert 5f3c8f0  # L3
git revert c618a99  # L2
git revert fad26cf  # L1
git revert 2a8ab4e  # L0
```

数据库回滚：
```sql
DROP TABLE IF EXISTS rag_user;
DROP TABLE IF EXISTS rag_tenant;
```

## Phase 2 入口条件

Phase 2 可以直接启动：

1. `rag_tenant/rag_user` 表已创建
2. JWT 管理器已实现
3. RBAC 权限已定义
4. 统一身份上下文已注入
5. 自动化测试框架已建立

## 下一步

Phase 2：API Key 管理
- 创建 `rag_api_key` 表
- 实现 API Key 创建/吊销/轮换
- 实现 API Key 认证中间件
- 更新 SDK 支持 API Key
