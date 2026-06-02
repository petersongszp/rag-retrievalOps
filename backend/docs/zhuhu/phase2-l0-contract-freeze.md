# Phase 2 L0 契约冻结记录

## 冻结日期
2026-06-03

## API Key 数据字段

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint64 | 主键 |
| tenant_id | uint64 | 所属租户 |
| user_id | uint64 | 创建者 |
| app_id | string | 应用标识 |
| key_hash | string | Key 哈希（SHA256） |
| key_prefix | string | Key 前缀（用于显示） |
| name | string | Key 名称 |
| permissions | JSON | 权限配置 |
| status | string | 状态：active/revoked |
| last_used_at | timestamp | 最后使用时间 |
| expires_at | timestamp | 过期时间 |
| created_at | timestamp | 创建时间 |

## API Key 认证格式

```
Authorization: Bearer rag_<random>
```

## 身份上下文

```json
{
  "auth_type": "api_key",
  "tenant_id": 1,
  "user_id": 1,
  "role": "owner",
  "app_id": "interview-agent",
  "api_key_id": 1,
  "permissions": ["retrieve"],
  "is_legacy": false
}
```

## API 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /v1/api-keys | 列表 |
| POST | /v1/api-keys | 创建 |
| PUT | /v1/api-keys/:id | 更新 |
| DELETE | /v1/api-keys/:id | 吊销 |
| POST | /v1/api-keys/:id/rotate | 轮换（可选） |

## 回滚方式

```bash
git revert <commit-hash>
```
