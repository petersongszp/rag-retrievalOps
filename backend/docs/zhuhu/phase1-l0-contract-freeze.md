# Phase 1 L0 契约冻结记录

## 冻结日期
2026-06-02

## API 响应口径

### 注册响应
```json
{
  "user_id": 1,
  "email": "user@example.com",
  "tenant_id": 1
}
```

### 登录响应
```json
{
  "access_token": "eyJhb...",
  "refresh_token": "eyJhb...",
  "expires_in": 7200,
  "user_id": 1,
  "role": "owner",
  "tenant_id": 1
}
```

## JWT 配置来源

| 配置项 | 字段 | 默认值 |
|--------|------|--------|
| JWT Secret | `rag.auth.jwt_secret` | 无（必填） |
| Access Token TTL | `rag.auth.access_token_ttl` | 2h |
| Refresh Token TTL | `rag.auth.refresh_token_ttl` | 168h |

## JWT Claims 结构

```json
{
  "user_id": 1,
  "tenant_id": 1,
  "role": "owner",
  "auth_type": "jwt",
  "token_type": "access",
  "exp": 1234567890,
  "iat": 1234567890,
  "iss": "rag-platform"
}
```

## 错误码映射

| 内部常量 | HTTP 状态码 | 说明 |
|----------|------------|------|
| `ErrCodeInvalidCredentials` | 401 | 邮箱或密码错误 |
| `ErrCodeEmailExists` | 409 | 邮箱已注册 |
| `ErrCodeWeakPassword` | 400 | 密码太弱 |
| `ErrCodePermissionDenied` | 403 | 权限不足 |
| `ErrCodeTenantNotFound` | 404 | 租户不存在 |

## 迁移目录

```
backend/migrations/
├── 001_create_rag_tenant.up.sql
├── 001_create_rag_tenant.down.sql
├── 002_create_rag_user.up.sql
└── 002_create_rag_user.down.sql
```

## 回滚方式

Phase 1 各阶段可独立回滚：
```bash
git revert <commit-hash>
```

迁移回滚：
```bash
# 回滚用户表
DROP TABLE IF EXISTS rag_user;
# 回滚租户表
DROP TABLE IF EXISTS rag_tenant;
```
