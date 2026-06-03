# RAG 平台多租户 + 权限 + 接入方案

> **版本**: v1.1  
> **日期**: 2026-06-02  
> **作者**: 架构设计  
> **状态**: Optimized Draft

---

## 目录

- [一、多租户架构设计](#一多租户架构设计)
- [二、认证授权设计](#二认证授权设计)
- [三、API 设计](#三api-设计)
- [四、接入流程设计](#四接入流程设计)
- [五、实现路线](#五实现路线)
- [六、安全设计](#六安全设计)
- [七、兼容性设计](#七兼容性设计)

---

## 一、多租户架构设计

### 1.1 数据模型设计

#### ER 关系概览

```
rag_tenant (1) ──< rag_user (N)
rag_tenant (1) ──< rag_api_key (N)
rag_user   (1) ──< rag_api_key (N)
rag_tenant (1) ──< rag_tenant_kb_permission (N)
```

#### 建表 SQL

```sql
-- ============================================================
-- 1. 租户表
-- ============================================================
CREATE TABLE rag_tenant (
    id                     BIGINT PRIMARY KEY AUTO_INCREMENT,
    name                   VARCHAR(128)  NOT NULL COMMENT '租户名称',
    slug                   VARCHAR(64)   NOT NULL COMMENT '租户标识（URL 友好，全局唯一）',
    plan                   VARCHAR(32)   NOT NULL DEFAULT 'free' COMMENT '套餐：free / pro / enterprise',
    status                 VARCHAR(16)   NOT NULL DEFAULT 'active' COMMENT '状态：active / suspended / deleted',
    max_kb_count           INT           NOT NULL DEFAULT 5      COMMENT '最大知识库数量',
    max_doc_count          INT           NOT NULL DEFAULT 100    COMMENT '最大文档数量',
    max_storage_mb         INT           NOT NULL DEFAULT 1024   COMMENT '最大存储（MB）',
    max_api_calls_per_day  INT           NOT NULL DEFAULT 10000  COMMENT '每日 API 调用上限',
    created_at             TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at             TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_slug (slug),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='租户表';

-- ============================================================
-- 2. 用户表
-- ============================================================
CREATE TABLE rag_user (
    id             BIGINT        PRIMARY KEY AUTO_INCREMENT,
    tenant_id      BIGINT        NOT NULL COMMENT '所属租户',
    email          VARCHAR(255)  NOT NULL COMMENT '邮箱（登录账号，全局唯一）',
    password_hash  VARCHAR(255)  NOT NULL COMMENT '密码哈希（bcrypt）',
    name           VARCHAR(128)  NOT NULL COMMENT '用户名',
    role           VARCHAR(32)   NOT NULL DEFAULT 'member' COMMENT '角色：owner / admin / member / viewer',
    status         VARCHAR(16)   NOT NULL DEFAULT 'active' COMMENT '状态：active / suspended / deleted',
    last_login_at  TIMESTAMP     NULL     COMMENT '最后登录时间',
    created_at     TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_email (email),
    INDEX idx_tenant_id (tenant_id),
    INDEX idx_status (status),
    FOREIGN KEY (tenant_id) REFERENCES rag_tenant(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户表';

-- ============================================================
-- 3. API Key 表
-- ============================================================
CREATE TABLE rag_api_key (
    id           BIGINT        PRIMARY KEY AUTO_INCREMENT,
    tenant_id    BIGINT        NOT NULL COMMENT '所属租户',
    user_id      BIGINT        NOT NULL COMMENT '创建者',
    app_id       VARCHAR(64)   NOT NULL COMMENT '应用标识（用于日志和配额追踪）',
    key_hash     VARCHAR(255)  NOT NULL COMMENT 'API Key 的 SHA256 哈希',
    key_prefix   VARCHAR(16)   NOT NULL COMMENT 'Key 前缀（用于 UI 显示，如 rag_a1b2****）',
    name         VARCHAR(128)  NULL     COMMENT 'Key 名称（用户自定义）',
    permissions  JSON          NOT NULL DEFAULT ('{}') COMMENT '权限配置 JSON',
    status       VARCHAR(16)   NOT NULL DEFAULT 'active' COMMENT '状态：active / revoked',
    last_used_at TIMESTAMP     NULL     COMMENT '最后使用时间',
    expires_at   TIMESTAMP     NULL     COMMENT '过期时间（NULL 表示永不过期）',
    created_at   TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_key_hash (key_hash),
    INDEX idx_tenant_id (tenant_id),
    INDEX idx_user_id (user_id),
    INDEX idx_app_id (app_id),
    INDEX idx_status (status),
    FOREIGN KEY (tenant_id) REFERENCES rag_tenant(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id)   REFERENCES rag_user(id)   ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='API Key 表';

-- ============================================================
-- 4. 知识库权限表（租户 ↔ 知识库映射）
-- ============================================================
CREATE TABLE rag_tenant_kb_permission (
    id         BIGINT       PRIMARY KEY AUTO_INCREMENT,
    tenant_id  BIGINT       NOT NULL COMMENT '租户 ID',
    kb_id      BIGINT       NOT NULL COMMENT '知识库 ID',
    permission VARCHAR(32)  NOT NULL DEFAULT 'read' COMMENT '权限：read / write / admin',
    created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_tenant_kb (tenant_id, kb_id),
    INDEX idx_kb_id (kb_id),
    FOREIGN KEY (tenant_id) REFERENCES rag_tenant(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='租户-知识库权限映射表';
```

#### 字段说明

| 表 | 关键字段 | 说明 |
|---|---|---|
| `rag_tenant` | `slug` | URL 友好标识，如 `acme-corp`，用于 API 路由和 UI 显示 |
| `rag_tenant` | `plan` | 套餐等级，决定配额上限 |
| `rag_user` | `email` | 全局唯一，一个邮箱只能属于一个租户 |
| `rag_user` | `role` | RBAC 角色，决定操作权限 |
| `rag_api_key` | `key_hash` | 存储哈希而非明文，Key 只在创建时返回一次 |
| `rag_api_key` | `key_prefix` | 前缀用于 UI 列表展示，如 `rag_a1b2****` |
| `rag_api_key` | `app_id` | 应用标识，用于日志追踪和配额统计 |
| `rag_api_key` | `permissions` | JSON 字段，预留精细权限控制 |

### 1.2 隔离策略

#### 隔离维度总览

| 维度 | 隔离方式 | 实现层 |
|------|---------|--------|
| **数据隔离** | 按 `tenant_id` 过滤，查询层强制注入 | 中间件 + Repository 层 |
| **资源隔离** | 按套餐配额限制（知识库数量、文档数量、存储、API 调用） | 中间件 + 配额检查 |
| **日志隔离** | 审计日志和检索日志按 `tenant_id` 过滤 | 查询层 WHERE 条件 |
| **计费隔离** | 每个租户独立统计用量和费用 | 计费服务 |

#### 数据隔离实现

```go
// 中间件：从认证信息中提取 tenant_id，注入 context
func TenantIsolationMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        tenantID := getTenantIDFromAuth(c) // 从 JWT 或 API Key 获取
        if tenantID == 0 {
            c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
            return
        }
        // 注入到 context，所有下游查询必须使用
        c.Set("tenant_id", tenantID)
        c.Next()
    }
}

// Repository 层：所有查询自动附加 tenant_id 条件
func (r *KBRepository) ListByTenant(ctx context.Context) ([]*KnowledgeBase, error) {
    tenantID := ctx.Value("tenant_id").(int64)
    return r.db.WithContext(ctx).
        Where("tenant_id = ?", tenantID).
        Find(&[]KnowledgeBase{}).Error
}
```

#### 资源配额检查

```go
// 配额检查中间件
func QuotaCheckMiddleware(resource string) gin.HandlerFunc {
    return func(c *gin.Context) {
        tenantID := c.GetInt64("tenant_id")
        tenant := getTenant(tenantID)
        
        switch resource {
        case "kb":
            current := countKBs(tenantID)
            if current >= tenant.MaxKBCount {
                c.AbortWithStatusJSON(429, gin.H{
                    "error": "quota_exceeded",
                    "message": fmt.Sprintf("知识库数量已达上限（%d）", tenant.MaxKBCount),
                })
                return
            }
        case "api_call":
            today := countAPICallsToday(tenantID)
            if today >= tenant.MaxAPICallsPerDay {
                c.AbortWithStatusJSON(429, gin.H{
                    "error": "quota_exceeded",
                    "message": "今日 API 调用次数已达上限",
                })
                return
            }
        }
        c.Next()
    }
}
```

---

## 二、认证授权设计

### 2.1 认证方式总览

| 方式 | 适用场景 | 实现状态 |
|------|---------|---------|
| **邮箱密码登录** | 用户通过 Admin UI 操作 | Phase 1 |
| **API Key 认证** | 程序化接入（SDK / HTTP） | Phase 2 |
| **OAuth 2.0** | 第三方登录（飞书、GitHub 等） | Phase 5（预留） |

### 2.2 邮箱密码登录流程

```
用户输入 email + password
        │
        ▼
   查询 rag_user（WHERE email = ? AND status = 'active'）
        │
        ▼
   bcrypt.CompareHashAndPassword(password_hash, password)
        │
        ├─ 失败 → 返回 401
        ▼
   生成 JWT Token（含 tenant_id, user_id, role）
        │
        ├─ access_token  （有效期 2 小时）
        └─ refresh_token （有效期 7 天）
```

### 2.3 API Key 认证流程

```
客户端请求
    │
    ▼
检查 Authorization header
    │
    ├─ 格式：Authorization: Bearer rag_<key>
    ▼
提取 key 明文 → 计算 SHA256(key) → 查询 rag_api_key WHERE key_hash = ?
    │
    ├─ 未找到 → 401 Invalid API Key
    ├─ status != 'active' → 401 API Key revoked
    ├─ expires_at < now → 401 API Key expired
    ▼
查询关联的 rag_tenant 和 rag_user
    │
    ├─ tenant.status != 'active' → 403 Tenant suspended
    ▼
注入 context：
    - tenant_id
    - user_id
    - app_id
    - permissions
    │
    ▼
更新 last_used_at → 继续处理请求
```

#### API Key 格式

```
格式：rag_<32位随机字符>
示例：rag_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6

存储：
  - key_hash  = SHA256(完整 key)  → 数据库存储
  - key_prefix = key[:12] + "****" → UI 显示
```

### 2.4 RBAC 权限模型

#### 角色权限矩阵

| 操作 | Owner | Admin | Member | Viewer |
|------|-------|-------|--------|--------|
| **租户管理** | | | | |
| 查看租户信息 | ✅ | ✅ | ✅ | ✅ |
| 修改租户设置 | ✅ | ❌ | ❌ | ❌ |
| 查看用量统计 | ✅ | ✅ | ✅ | ✅ |
| **成员管理** | | | | |
| 查看成员列表 | ✅ | ✅ | ✅ | ✅ |
| 邀请成员 | ✅ | ✅ | ❌ | ❌ |
| 修改成员角色 | ✅ | ❌ | ❌ | ❌ |
| 移除成员 | ✅ | ✅ | ❌ | ❌ |
| **知识库** | | | | |
| 查看知识库列表 | ✅ | ✅ | ✅ | ✅ |
| 创建知识库 | ✅ | ✅ | ✅ | ❌ |
| 上传文档 | ✅ | ✅ | ✅ | ❌ |
| 删除知识库 | ✅ | ✅ | ❌ | ❌ |
| **API Key** | | | | |
| 查看 API Key | ✅ | ✅ | ✅ | ❌ |
| 创建 API Key | ✅ | ✅ | ✅ | ❌ |
| 吊销 API Key | ✅ | ✅ | ❌ | ❌ |
| **检索** | | | | |
| 执行检索 | ✅ | ✅ | ✅ | ✅ |
| **日志** | | | | |
| 查看检索日志 | ✅ | ✅ | ✅ | ✅ |
| 查看审计日志 | ✅ | ✅ | ❌ | ❌ |

#### 权限检查中间件

```go
// 角色权限映射
var rolePermissions = map[string]map[string]bool{
    "owner": {
        "tenant:read": true, "tenant:write": true,
        "member:read": true, "member:invite": true, "member:remove": true, "member:role": true,
        "kb:read": true, "kb:write": true, "kb:delete": true,
        "apikey:read": true, "apikey:write": true, "apikey:revoke": true,
        "retrieve": true,
        "log:read": true, "audit:read": true,
    },
    "admin": {
        "tenant:read": true,
        "member:read": true, "member:invite": true, "member:remove": true,
        "kb:read": true, "kb:write": true, "kb:delete": true,
        "apikey:read": true, "apikey:write": true,
        "retrieve": true,
        "log:read": true, "audit:read": true,
    },
    "member": {
        "tenant:read": true,
        "member:read": true,
        "kb:read": true, "kb:write": true,
        "apikey:read": true, "apikey:write": true,
        "retrieve": true,
        "log:read": true,
    },
    "viewer": {
        "tenant:read": true,
        "member:read": true,
        "kb:read": true,
        "retrieve": true,
        "log:read": true,
    },
}

func RequirePermission(perm string) gin.HandlerFunc {
    return func(c *gin.Context) {
        role := c.GetString("user_role")
        if !rolePermissions[role][perm] {
            c.AbortWithStatusJSON(403, gin.H{
                "error": "forbidden",
                "message": fmt.Sprintf("需要权限：%s", perm),
            })
            return
        }
        c.Next()
    }
}
```

---

## 三、API 设计

### 3.1 认证相关 API

#### POST /v1/auth/register — 注册

注册新用户并自动创建租户。

**请求：**

```json
{
    "email": "user@example.com",
    "password": "SecurePass123!",
    "name": "张三",
    "tenant_name": "示例公司"
}
```

**响应（201）：**

```json
{
    "user": {
        "id": 1,
        "email": "user@example.com",
        "name": "张三",
        "role": "owner",
        "tenant_id": 1
    },
    "tenant": {
        "id": 1,
        "name": "示例公司",
        "slug": "example-corp",
        "plan": "free"
    },
    "access_token": "eyJhbGciOi...",
    "refresh_token": "eyJhbGciOi...",
    "expires_in": 7200
}
```

**错误响应：**

```json
// 邮箱已存在
{ "error": "email_exists", "message": "该邮箱已注册" }

// 密码不符合要求
{ "error": "weak_password", "message": "密码至少 8 位，需包含大小写字母和数字" }
```

#### POST /v1/auth/login — 登录

**请求：**

```json
{
    "email": "user@example.com",
    "password": "SecurePass123!"
}
```

**响应（200）：**

```json
{
    "user": {
        "id": 1,
        "email": "user@example.com",
        "name": "张三",
        "role": "owner",
        "tenant_id": 1
    },
    "access_token": "eyJhbGciOi...",
    "refresh_token": "eyJhbGciOi...",
    "expires_in": 7200
}
```

#### POST /v1/auth/logout — 登出

**请求头：**

```
Authorization: Bearer <access_token>
```

**响应（200）：**

```json
{ "message": "已登出" }
```

#### POST /v1/auth/refresh — 刷新 Token

**请求：**

```json
{
    "refresh_token": "eyJhbGciOi..."
}
```

**响应（200）：**

```json
{
    "access_token": "eyJhbGciOi...",
    "refresh_token": "eyJhbGciOi...",
    "expires_in": 7200
}
```

#### GET /v1/auth/me — 获取当前用户信息

**请求头：**

```
Authorization: Bearer <access_token>
```

**响应（200）：**

```json
{
    "id": 1,
    "email": "user@example.com",
    "name": "张三",
    "role": "owner",
    "tenant_id": 1,
    "tenant": {
        "id": 1,
        "name": "示例公司",
        "slug": "example-corp",
        "plan": "free"
    },
    "last_login_at": "2026-06-02T12:00:00Z",
    "created_at": "2026-06-01T00:00:00Z"
}
```

#### PUT /v1/auth/password — 修改密码

**请求：**

```json
{
    "old_password": "OldPass123!",
    "new_password": "NewPass456!"
}
```

**响应（200）：**

```json
{ "message": "密码已更新" }
```

### 3.2 租户管理 API

#### GET /v1/tenant — 获取当前租户信息

**响应（200）：**

```json
{
    "id": 1,
    "name": "示例公司",
    "slug": "example-corp",
    "plan": "free",
    "status": "active",
    "max_kb_count": 5,
    "max_doc_count": 100,
    "max_storage_mb": 1024,
    "max_api_calls_per_day": 10000,
    "created_at": "2026-06-01T00:00:00Z"
}
```

#### PUT /v1/tenant — 更新租户信息

**权限：** Owner

**请求：**

```json
{
    "name": "新公司名称"
}
```

#### GET /v1/tenant/usage — 获取用量统计

**响应（200）：**

```json
{
    "kb_count": 3,
    "doc_count": 42,
    "storage_mb": 256,
    "api_calls_today": 1234,
    "api_calls_this_month": 45678,
    "limits": {
        "max_kb_count": 5,
        "max_doc_count": 100,
        "max_storage_mb": 1024,
        "max_api_calls_per_day": 10000
    }
}
```

#### GET /v1/tenant/members — 获取成员列表

**响应（200）：**

```json
{
    "members": [
        {
            "id": 1,
            "email": "owner@example.com",
            "name": "张三",
            "role": "owner",
            "status": "active",
            "last_login_at": "2026-06-02T12:00:00Z"
        },
        {
            "id": 2,
            "email": "member@example.com",
            "name": "李四",
            "role": "member",
            "status": "active",
            "last_login_at": "2026-06-01T08:00:00Z"
        }
    ],
    "total": 2
}
```

#### POST /v1/tenant/members — 邀请成员

**权限：** Owner / Admin

**请求：**

```json
{
    "email": "newuser@example.com",
    "name": "王五",
    "role": "member"
}
```

**响应（201）：**

```json
{
    "id": 3,
    "email": "newuser@example.com",
    "name": "王五",
    "role": "member",
    "status": "active"
}
```

#### PUT /v1/tenant/members/:id — 更新成员角色

**权限：** Owner

**请求：**

```json
{
    "role": "admin"
}
```

#### DELETE /v1/tenant/members/:id — 移除成员

**权限：** Owner / Admin

**响应（200）：**

```json
{ "message": "成员已移除" }
```

### 3.3 API Key 管理 API

#### GET /v1/api-keys — 获取 API Key 列表

**响应（200）：**

```json
{
    "keys": [
        {
            "id": 1,
            "name": "生产环境",
            "app_id": "app_mianshiba",
            "key_prefix": "rag_a1b2c3****",
            "permissions": { "retrieve": true, "kb_ids": [1, 2] },
            "status": "active",
            "last_used_at": "2026-06-02T12:00:00Z",
            "expires_at": null,
            "created_at": "2026-06-01T00:00:00Z"
        }
    ],
    "total": 1
}
```

#### POST /v1/api-keys — 创建 API Key

**请求：**

```json
{
    "name": "测试环境",
    "app_id": "app_test",
    "permissions": {
        "retrieve": true,
        "kb_ids": [1]
    },
    "expires_at": "2027-06-01T00:00:00Z"
}
```

**响应（201）：**

```json
{
    "id": 2,
    "name": "测试环境",
    "app_id": "app_test",
    "key": "rag_x9y8z7w6v5u4t3s2r1q0p9o8n7m6l5k4",  // ⚠️ 只在创建时返回一次
    "key_prefix": "rag_x9y8z7****",
    "permissions": { "retrieve": true, "kb_ids": [1] },
    "status": "active",
    "expires_at": "2027-06-01T00:00:00Z",
    "created_at": "2026-06-02T21:00:00Z"
}
```

> ⚠️ **重要**：`key` 字段只在创建响应中返回一次，后续无法再获取明文。请务必保存。

#### GET /v1/api-keys/:id — 获取 API Key 详情

**响应（200）：** 同列表项格式（不含 key 明文）

#### PUT /v1/api-keys/:id — 更新 API Key

**请求：**

```json
{
    "name": "新名称",
    "permissions": { "retrieve": true, "kb_ids": [1, 2, 3] }
}
```

#### DELETE /v1/api-keys/:id — 吊销 API Key

**响应（200）：**

```json
{ "message": "API Key 已吊销" }
```

#### POST /v1/api-keys/:id/rotate — 轮换 API Key

吊销旧 Key 并生成新 Key。

**响应（200）：**

```json
{
    "id": 2,
    "name": "测试环境",
    "app_id": "app_test",
    "key": "rag_new_key_here_32chars________",  // ⚠️ 新 Key，只返回一次
    "key_prefix": "rag_new_ke****",
    "status": "active",
    "expires_at": "2027-06-01T00:00:00Z"
}
```

### 3.4 知识库权限 API

#### GET /v1/kb-permissions — 获取可访问的知识库列表

**响应（200）：**

```json
{
    "permissions": [
        {
            "id": 1,
            "kb_id": 1,
            "kb_name": "Java 面试题库",
            "permission": "read",
            "created_at": "2026-06-01T00:00:00Z"
        },
        {
            "id": 2,
            "kb_id": 2,
            "kb_name": "Go 面试题库",
            "permission": "write",
            "created_at": "2026-06-01T00:00:00Z"
        }
    ],
    "total": 2
}
```

#### POST /v1/kb-permissions — 授权知识库访问

**权限：** Admin 以上

**请求：**

```json
{
    "kb_id": 3,
    "permission": "read"
}
```

**响应（201）：**

```json
{
    "id": 3,
    "kb_id": 3,
    "kb_name": "Python 面试题库",
    "permission": "read",
    "created_at": "2026-06-02T21:00:00Z"
}
```

#### DELETE /v1/kb-permissions/:id — 取消知识库访问

**响应（200）：**

```json
{ "message": "权限已取消" }
```

### 3.5 检索 API（现有，增加认证）

#### POST /v1/retrieve

**认证：** JWT Token 或 API Key（任选其一）

**请求头：**

```
Authorization: Bearer <access_token 或 api_key>
Content-Type: application/json
```

**请求：**

```json
{
    "query": "什么是 JVM 调优？",
    "kb_ids": [1, 2, 3],
    "top_k": 5,
    "score_threshold": 0.5
}
```

**响应（200）：**

```json
{
    "items": [
        {
            "content": "JVM 调优是指...",
            "score": 0.92,
            "kb_id": 1,
            "doc_id": 42,
            "chunk_id": 128,
            "metadata": { "source": "java-guide.pdf", "page": 15 }
        }
    ],
    "query_id": "q_abc123",
    "latency_ms": 45
}
```

**错误响应：**

```json
// 无权限访问指定知识库
{
    "error": "kb_forbidden",
    "message": "无权访问知识库 ID=5",
    "forbidden_kb_ids": [5]
}

// 配额超限
{
    "error": "quota_exceeded",
    "message": "今日 API 调用次数已达上限",
    "retry_after": 3600
}
```

---

## 四、接入流程设计

### 4.1 用户接入流程（最简化）

```
┌─────────────────────────────────────────────────────────────┐
│  Step 1: 注册                                                │
│  → 访问 Admin UI → 输入邮箱/密码/公司名 → 自动创建租户       │
├─────────────────────────────────────────────────────────────┤
│  Step 2: 登录                                                │
│  → 使用邮箱密码登录 Admin UI                                 │
├─────────────────────────────────────────────────────────────┤
│  Step 3: 创建 API Key                                        │
│  → 进入「API Key 管理」→ 创建 Key → 复制保存                 │
├─────────────────────────────────────────────────────────────┤
│  Step 4: 接入使用                                            │
│  → 方式 A: HTTP 调用 /v1/retrieve（带 API Key）              │
│  → 方式 B: Go SDK（ragsdk）                                  │
│  → 方式 C: Admin UI 检索实验室                               │
├─────────────────────────────────────────────────────────────┤
│  Step 5: 监控（可选）                                        │
│  → 在 Admin UI 查看日志、成本、用量统计                      │
└─────────────────────────────────────────────────────────────┘
```

### 4.2 HTTP 接入示例

#### cURL

```bash
curl -X POST https://rag-platform.com/v1/retrieve \
  -H "Authorization: Bearer rag_x9y8z7w6v5u4t3s2r1q0p9o8n7m6l5k4" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "什么是 JVM 调优？",
    "kb_ids": [1, 2, 3],
    "top_k": 5
  }'
```

#### Python

```python
import requests

resp = requests.post(
    "https://rag-platform.com/v1/retrieve",
    headers={"Authorization": "Bearer rag_x9y8z7w6v5u4t3s2r1q0p9o8n7m6l5k4"},
    json={
        "query": "什么是 JVM 调优？",
        "kb_ids": [1, 2, 3],
        "top_k": 5,
    },
)
data = resp.json()
for item in data["items"]:
    print(f"[{item['score']:.2f}] {item['content']}")
```

### 4.3 Go SDK 接入示例

```go
package main

import (
    "context"
    "fmt"
    
    "your-project/pkg/ragsdk"
)

func main() {
    // 1. 创建客户端
    client := ragsdk.NewClient(ragsdk.ClientConfig{
        BaseURL: "https://rag-platform.com",
        APIKey:  "rag_x9y8z7w6v5u4t3s2r1q0p9o8n7m6l5k4", // 从 Admin UI 获取
    })

    ctx := context.Background()

    // 2. 检索
    resp, err := client.Retrieve(ctx, ragsdk.RetrieveRequest{
        Query:   "什么是 JVM 调优？",
        KBIDs:   []uint64{1, 2, 3},
        TopK:    5,
        ScoreThreshold: 0.5,
    })
    if err != nil {
        panic(err)
    }

    // 3. 使用结果
    for _, item := range resp.Items {
        fmt.Printf("[%.2f] %s\n", item.Score, item.Content)
    }
}
```

### 4.4 SDK 适配要点

需要在现有 `ragsdk` 中增加以下功能：

```go
// pkg/ragsdk/client.go

type ClientConfig struct {
    BaseURL    string        // 平台地址
    APIKey     string        // API Key（Bearer token）
    AppID      string        // 应用标识（可选，从 API Key 自动获取）
    Timeout    time.Duration // 请求超时
    MaxRetries int           // 最大重试次数
}

func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
    req, _ := http.NewRequestWithContext(ctx, method, c.config.BaseURL+path, bodyReader(body))
    
    // 认证头
    req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
    req.Header.Set("Content-Type", "application/json")
    
    // 可选：自定义 User-Agent
    req.Header.Set("User-Agent", "rag-sdk-go/1.0")
    
    return c.httpClient.Do(req)
}
```

---

## 五、实现路线

### 5.1 路线定位

本路线按“先安全可测、再开放接入、再强隔离、最后商业化”的顺序推进。第一版不追求完整 SaaS，而是先让真实 Agent 能安全调用平台，并确保不同租户的数据不会串。

**必须先做：**
1. 认证闭环：注册、登录、JWT、测试管理员创建方式。
2. Agent 接入闭环：API Key 创建、鉴权、吊销、SDK 调用。
3. 租户隔离闭环：数据库、向量库、日志都能按 `tenant_id` 隔离。
4. Admin 可测闭环：Owner 能在 UI 里创建知识库、创建 Key、查看调用日志。

**明确后置：**
1. 套餐计费、账单、支付集成。
2. OAuth 2.0 第三方登录。
3. 复杂成员邀请流程，如邮件邀请、邀请链接、组织审批。
4. Webhook、企业 SSO、细粒度套餐运营。

### 5.2 总览

```
Phase 0 ──→ Phase 1 ──→ Phase 2 ──→ Phase 3 ──→ Phase 4 ──→ Phase 5
基线修正      账号租户      API Key      强隔离        Admin闭环     商业化后置
(2-3天)      (1周)        (1周)        (1-2周)      (1周)        (按验证后启动)
```

**第一版上线 Gate：**
1. 测试管理员可以稳定创建，且生产环境不存在默认放行的 Admin 后门。
2. Agent 可以只靠 `Authorization: Bearer rag_<key>` 调用 `/v1/retrieve`。
3. API Key 能自动识别 `tenant_id`、`app_id`、`permissions`。
4. 任意租户不能读取、检索、查看其他租户的知识库、文档、日志。
5. Admin UI 能完成“注册登录 -> 创建知识库 -> 上传文档 -> 创建 API Key -> Agent 检索 -> 查看日志”闭环。

### Phase 0：基线修正与测试入口（2-3 天）

**目标：** 先把当前项目从“开发期可跑”调整成“可安全测试”，避免后续在错误地基上扩展。

| 任务 | 说明 | 预估 |
|------|------|------|
| 0.1 当前认证基线盘点 | 梳理 `/api/admin/*`、`/api/kb/*`、`/v1/retrieve` 的实际鉴权方式 | 0.5 天 |
| 0.2 开发期 Admin 后门隔离 | `/api/admin/*` 自动注入 `user_id=1, role=admin` 只能在 `dev/test` 环境启用 | 0.5 天 |
| 0.3 Bootstrap 管理员方案 | 增加 `BOOTSTRAP_ADMIN_EMAIL`、`BOOTSTRAP_ADMIN_PASSWORD`，首次启动创建测试 Owner | 0.5 天 |
| 0.4 接口契约冻结 | 明确 `/v1/auth/*`、`/v1/api-keys`、`/v1/retrieve` 第一版字段 | 0.5 天 |
| 0.5 测试脚本更新 | 更新本地 curl / SDK 示例，区分 JWT 和 API Key 场景 | 0.5 天 |

**验收：**
1. 本地测试仍可快速进入 Admin，但生产配置不能自动获得 admin 权限。
2. 有一个确定的测试管理员创建方式，不依赖手工改数据库。
3. 文档明确当前旧 `app_id` 白名单和新 API Key 的迁移边界。

### Phase 1：账号 + 租户最小闭环（1 周）

**目标：** 用户能注册登录；第一个注册用户自动成为租户 `owner`；Admin UI 和管理 API 使用 JWT。

| 任务 | 说明 | 预估 |
|------|------|------|
| 1.1 数据库迁移 | 创建 `rag_tenant`、`rag_user`，并预留 `tenant_id` 回填能力 | 0.5 天 |
| 1.2 注册接口 | `POST /v1/auth/register`，注册时创建租户和 owner 用户 | 1 天 |
| 1.3 登录接口 | `POST /v1/auth/login`，返回 `access_token` 和 `refresh_token` | 1 天 |
| 1.4 JWT 中间件 | JWT 注入 `tenant_id`、`user_id`、`role`、`auth_type=jwt` | 0.5 天 |
| 1.5 当前 Admin API 接入 JWT | 管理端路由从默认 admin 逐步切到真实 JWT 身份 | 1 天 |
| 1.6 基础 RBAC | 先实现 `owner/admin/member/viewer` 的最小权限判断 | 0.5 天 |
| 1.7 测试 | 注册、登录、Token 过期、无权限访问测试 | 1 天 |

**验收：**
1. 新用户注册后自动拥有一个租户，角色为 `owner`。
2. Admin UI 不再依赖固定 `user_id=1` 才能工作。
3. JWT 中能拿到 `tenant_id`，后续查询不需要再猜用户属于哪个租户。

### Phase 2：API Key + Agent 接入 MVP（1 周）

**目标：** 真实 Agent 可以用 API Key 调用平台；API Key 不再只是“Bearer 字符串”，而是可追踪、可吊销、可授权的门禁卡。

| 任务 | 说明 | 预估 |
|------|------|------|
| 2.1 数据库迁移 | 创建 `rag_api_key`，保存 `key_hash`、`key_prefix`、`tenant_id`、`user_id`、`app_id` | 0.5 天 |
| 2.2 API Key CRUD | `GET/POST/PUT/DELETE /v1/api-keys`，创建时只返回一次明文 Key | 1 天 |
| 2.3 API Key 鉴权中间件 | 解析 `Authorization: Bearer rag_<key>`，注入 `tenant_id/app_id/api_key_id/permissions` | 1 天 |
| 2.4 统一认证入口 | JWT 用于 Admin/UI；API Key 用于 Agent/SDK；同一个中间件输出统一身份上下文 | 0.5 天 |
| 2.5 `/v1/retrieve` 改造 | 新认证优先，`app_id` 从 API Key 推导；旧 `app_id` 白名单作为兼容回退 | 1 天 |
| 2.6 SDK 修正 | Go SDK 自动带 API Key，并补齐 `AppID` 兼容字段或移除强依赖 | 0.5 天 |
| 2.7 调用日志 | 检索日志记录 `tenant_id`、`app_id`、`api_key_id`、`source_api` | 0.5 天 |
| 2.8 测试 | API Key 创建、吊销、过期、无效 Key、SDK 检索测试 | 1 天 |

**验收：**
1. Agent 每次请求只需要带 API Key，不需要终端用户手动传 Key。
2. 吊销 API Key 后，旧 Key 立即不能调用。
3. 检索日志能按 `app_id/api_key_id` 追踪到具体应用。
4. 老的 `app_id` 白名单调用仍可临时工作，但日志标记为 `auth_type=legacy`。

### Phase 3：多租户强隔离 + 基础配额（1-2 周）

**目标：** 不同租户之间的数据、向量检索结果、日志和配额完全隔离。这一阶段是平台能否正式对外的核心 Gate。

| 任务 | 说明 | 预估 |
|------|------|------|
| 3.1 现有表补 `tenant_id` | 给知识库、文档、任务、审计、检索日志补齐 `tenant_id` | 1 天 |
| 3.2 数据迁移 | 将现有 `user_id` 知识库迁移到默认系统租户或测试租户 | 0.5 天 |
| 3.3 Repository 强制过滤 | 列表、详情、删除、上传、日志查询统一追加 `tenant_id` 条件 | 1 天 |
| 3.4 知识库授权表 | 创建 `rag_tenant_kb_permission`，校验租户可访问的 `kb_id` | 0.5 天 |
| 3.5 检索权限校验 | `/v1/retrieve` 在检索前校验 `kb_ids` 是否属于当前租户或 Key 权限 | 1 天 |
| 3.6 向量元数据隔离 | 入库 chunk metadata 写入 `tenant_id/kb_id`，检索 expr 强制包含租户过滤 | 1 天 |
| 3.7 基础配额 | 先实现每日 API 调用、最大知识库数、最大文档数，不做计费 | 1 天 |
| 3.8 隔离测试 | 构造租户 A/B，验证互相无法读取、检索、删除、查看日志 | 1 天 |

**验收：**
1. 租户 A 即使知道租户 B 的 `kb_id`，也无法检索或读取。
2. Milvus 检索表达式中能看到最终的 `tenant_id` 过滤条件。
3. 配额超限返回 `429 quota_exceeded`，不影响其他租户。
4. 审计日志和检索日志都只能看到当前租户自己的数据。

### Phase 4：Admin UI 闭环 + 接入文档（1 周）

**目标：** 让非开发人员也能完成管理闭环，让 Agent 接入不再依赖口头说明。

| 任务 | 说明 | 预估 |
|------|------|------|
| 4.1 登录注册页面 | Admin UI 支持注册、登录、Token 刷新、登出 | 1 天 |
| 4.2 API Key 页面 | 创建、列表、复制一次性 Key、吊销、轮换 | 1 天 |
| 4.3 租户设置页面 | 展示租户名、套餐占位、状态、基础配额 | 0.5 天 |
| 4.4 用量页面 | 展示 `api_calls_today`、知识库数、文档数、存储占用 | 0.5 天 |
| 4.5 权限可见性 | 前端按 `role` 隐藏不可用操作，后端仍做最终权限校验 | 0.5 天 |
| 4.6 接入文档 | 输出 cURL、Go SDK、Python requests、Agent 后端接入示例 | 1 天 |
| 4.7 E2E 验收 | 完成注册到 Agent 检索的端到端测试 | 1 天 |

**验收：**
1. Owner 可以在 UI 创建知识库、上传文档、创建 API Key。
2. 使用 UI 生成的 Key 可以直接跑通 `/v1/retrieve`。
3. 文档明确“用户不直接持有 Key，Agent 后端持有 Key”。
4. 测试指南中的 curl 示例全部带正确认证方式。

### Phase 5：商业化与高级企业能力（验证后启动）

**目标：** 在前四个阶段稳定后，再引入商业化和复杂组织能力，避免过早增加系统复杂度。

| 任务 | 说明 | 启动条件 |
|------|------|------|
| 5.1 套餐管理 | free/pro/enterprise 的额度配置和升级流程 | 已有真实租户用量数据 |
| 5.2 计费系统 | 账单、支付、欠费停用、发票等 | API 调用量和成本统计稳定 |
| 5.3 OAuth 2.0 | 飞书、GitHub、Google 登录 | 邮箱密码登录稳定且有企业客户需求 |
| 5.4 复杂成员邀请 | 邮件邀请、邀请链接、过期、审批、域名限制 | 基础成员管理已稳定 |
| 5.5 Webhook 通知 | 配额预警、Key 泄露风险、异常调用通知 | 审计和告警基础稳定 |
| 5.6 企业安全增强 | SSO、IP 白名单、Key 作用域模板、审计导出 | 企业客户明确要求 |

**验收：**
1. 商业化能力不能破坏 Phase 1-4 的认证、隔离、Agent 接入闭环。
2. 套餐和计费只读取已稳定的用量统计，不重新定义检索主流程。
3. OAuth 和邀请流程只影响登录/成员加入，不影响 API Key 的服务端调用方式。

### 5.3 推荐执行节奏

1. Phase 0 和 Phase 1 串行执行，先保证管理员测试入口和 JWT 身份可信。
2. Phase 2 可以和 Admin UI 的 API Key 页面设计并行，但后端鉴权必须先合入。
3. Phase 3 不建议并行拆太散，所有涉及 `tenant_id` 的查询要统一 code review。
4. Phase 4 只补产品闭环，不新增复杂业务规则。
5. Phase 5 必须等至少一个真实 Agent 项目跑通并产生稳定调用日志后再启动。

### 5.4 回滚与降级

| 风险 | 降级方案 |
|------|---------|
| 新 API Key 鉴权异常 | 临时回退旧 `app_id` 白名单，但必须记录 `auth_type=legacy` |
| 租户过滤误伤检索结果 | 保留灰度开关，只对测试租户启用强隔离验证 |
| Admin 登录影响本地测试 | `dev/test` 环境保留 bootstrap 管理员，不保留生产默认 admin |
| 配额统计异常 | 只关闭配额拦截，保留调用日志记录 |
| SDK 兼容问题 | `/v1/retrieve` 暂时兼容 `app_id` 请求体字段，文档标记 deprecated |

---

## 六、安全设计

### 6.1 认证安全

| 项目 | 方案 | 说明 |
|------|------|------|
| **密码存储** | bcrypt（cost=12） | 不可逆哈希，抗彩虹表 |
| **API Key 存储** | SHA256 哈希 | Key 明文只在创建时返回一次 |
| **JWT Token** | HS256 / RS256 | access_token 2 小时，refresh_token 7 天 |
| **Token 黑名单** | Redis Set | 登出时加入黑名单，中间件检查 |
| **密码策略** | 最少 8 位，含大小写和数字 | 注册和修改密码时校验 |

### 6.2 传输安全

- **HTTPS 强制**：所有 API 必须通过 HTTPS 访问
- **HSTS**：启用 HTTP Strict Transport Security
- **CORS**：限制允许的 Origin

### 6.3 访问控制

- **Rate Limiting**：按租户限流（基于 `tenant_id`）
  - 默认：100 次/分钟（API Key）
  - 默认：30 次/分钟（JWT，UI 操作）
- **IP 白名单**：API Key 可配置允许的 IP（可选）
- **操作频率限制**：登录失败 5 次后锁定 15 分钟

### 6.4 审计日志

所有敏感操作记录审计日志：

```sql
-- 审计日志表（已有，增加 tenant_id）
CREATE TABLE rag_audit_log (
    id          BIGINT PRIMARY KEY AUTO_INCREMENT,
    tenant_id   BIGINT       NOT NULL COMMENT '租户 ID',
    user_id     BIGINT       NULL     COMMENT '操作用户',
    action      VARCHAR(64)  NOT NULL COMMENT '操作类型',
    resource    VARCHAR(64)  NOT NULL COMMENT '资源类型',
    resource_id VARCHAR(128) NULL     COMMENT '资源 ID',
    detail      JSON         NULL     COMMENT '操作详情',
    ip          VARCHAR(45)  NULL     COMMENT '客户端 IP',
    user_agent  VARCHAR(255) NULL     COMMENT 'User-Agent',
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_tenant_id (tenant_id),
    INDEX idx_user_id (user_id),
    INDEX idx_action (action),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='审计日志表';
```

**审计的操作类型：**

| 操作 | action | 说明 |
|------|--------|------|
| 用户注册 | `auth.register` | 新用户注册 |
| 用户登录 | `auth.login` | 登录成功 |
| 登录失败 | `auth.login_failed` | 密码错误 |
| 修改密码 | `auth.password_change` | 密码变更 |
| 创建 API Key | `apikey.create` | 创建新 Key |
| 吊销 API Key | `apikey.revoke` | 吊销 Key |
| 轮换 API Key | `apikey.rotate` | 轮换 Key |
| 邀请成员 | `member.invite` | 邀请新成员 |
| 移除成员 | `member.remove` | 移除成员 |
| 角色变更 | `member.role_change` | 修改成员角色 |
| 授权知识库 | `kb.grant` | 授权知识库访问 |
| 取消授权 | `kb.revoke` | 取消知识库访问 |
| 检索请求 | `retrieve` | 每次检索（可选，量大） |

### 6.5 数据安全

- **敏感数据加密**：数据库中的密钥、Token 等字段加密存储
- **日志脱敏**：日志中不记录密码、完整 API Key
- **备份策略**：每日全量备份，binlog 实时同步
- **数据删除**：租户删除时，数据保留 30 天后彻底清除

---

## 七、兼容性设计

### 7.1 向后兼容策略

现有系统使用 `app_id` 白名单机制进行访问控制。迁移策略：

```
现有系统                          迁移后
─────────                        ────────
app_id 白名单                     rag_api_key 表
（配置文件）                      （数据库）

迁移路径：
1. 现有 app_id 自动映射为「系统租户」的 API Key
2. 系统租户拥有所有知识库的访问权限
3. 老接口继续工作，新功能可选启用
```

#### 兼容中间件

```go
func AuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // 优先尝试新认证（JWT / API Key）
        if tryNewAuth(c) {
            c.Next()
            return
        }
        
        // 回退到老的 app_id 白名单
        if tryLegacyAuth(c) {
            c.Next()
            return
        }
        
        c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
    }
}

func tryLegacyAuth(c *gin.Context) bool {
    appID := c.GetHeader("X-App-ID")
    if appID == "" {
        return false
    }
    
    // 检查白名单（配置文件或数据库）
    if !isAllowedAppID(appID) {
        return false
    }
    
    // 映射到系统租户
    c.Set("tenant_id", SYSTEM_TENANT_ID)
    c.Set("app_id", appID)
    c.Set("auth_type", "legacy")
    return true
}
```

### 7.2 渐进迁移方案

| 阶段 | 说明 | 时间 |
|------|------|------|
| **阶段 A：共存** | 新旧认证并存，老用户无感知 | Phase 1-2 |
| **阶段 B：通知** | 通知老用户迁移，提供迁移工具 | Phase 3 |
| **阶段 C：强制** | 老接口标记 deprecated，设定下线日期 | Phase 5 |
| **阶段 D：下线** | 移除老的 app_id 白名单机制 | +3 个月 |

### 7.3 API 版本管理

```
/v1/...  — 现有 API，保持不变
/v2/...  — 新版本（如需要 breaking change）
```

**策略：**
- v1 API 持续维护，不引入破坏性变更
- 如需 breaking change，在 v2 中实现
- v1 和 v2 可共存，通过路径前缀区分

### 7.4 数据迁移脚本

```sql
-- 迁移步骤 1：创建系统租户
INSERT INTO rag_tenant (id, name, slug, plan, status, max_kb_count, max_doc_count, max_storage_mb, max_api_calls_per_day)
VALUES (1, '系统租户', 'system', 'enterprise', 'active', 9999, 99999, 102400, 10000000);

-- 迁移步骤 2：将现有 app_id 映射为系统租户的 API Key
-- （需要根据实际的 app_id 列表生成）
INSERT INTO rag_api_key (tenant_id, user_id, app_id, key_hash, key_prefix, name, status)
VALUES 
    (1, 1, 'app_mianshiba', SHA2('legacy_mianshiba', 256), 'legacy_****', '面试吧（迁移）', 'active'),
    (1, 1, 'app_test', SHA2('legacy_test', 256), 'legacy_****', '测试应用（迁移）', 'active');

-- 迁移步骤 3：现有知识库授权给系统租户
-- INSERT INTO rag_tenant_kb_permission (tenant_id, kb_id, permission)
-- SELECT 1, id, 'admin' FROM knowledge_base;
```

---

## 附录

### A. JWT Token 结构

```json
// Header
{
    "alg": "HS256",
    "typ": "JWT"
}

// Payload
{
    "sub": "1",                    // user_id
    "tenant_id": 1,                // 租户 ID
    "email": "user@example.com",   // 邮箱
    "role": "owner",               // 角色
    "type": "access",              // token 类型：access / refresh
    "iat": 1717344000,             // 签发时间
    "exp": 1717351200,             // 过期时间（+2h）
    "iss": "rag-platform"          // 签发者
}
```

### B. 错误码规范

| HTTP Status | Error Code | 说明 |
|-------------|-----------|------|
| 400 | `bad_request` | 请求参数错误 |
| 401 | `unauthorized` | 未认证 |
| 401 | `invalid_token` | Token 无效或过期 |
| 401 | `invalid_api_key` | API Key 无效 |
| 401 | `api_key_revoked` | API Key 已吊销 |
| 401 | `api_key_expired` | API Key 已过期 |
| 403 | `forbidden` | 无权限 |
| 403 | `tenant_suspended` | 租户已停用 |
| 404 | `not_found` | 资源不存在 |
| 409 | `email_exists` | 邮箱已注册 |
| 422 | `weak_password` | 密码不符合要求 |
| 429 | `quota_exceeded` | 配额超限 |
| 429 | `rate_limited` | 请求过于频繁 |
| 500 | `internal_error` | 服务器内部错误 |

### C. 套餐配额对照表

| 配额项 | Free | Pro | Enterprise |
|--------|------|-----|------------|
| 最大知识库数 | 5 | 20 | 无限 |
| 最大文档数 | 100 | 1,000 | 无限 |
| 最大存储 | 1 GB | 10 GB | 100 GB |
| 每日 API 调用 | 10,000 | 100,000 | 无限 |
| 成员数 | 3 | 20 | 无限 |
| API Key 数 | 5 | 20 | 无限 |
| 技术支持 | 社区 | 邮件 | 专属 |

### D. 目录结构建议

```
rag-retrievalOps/
├── internal/
│   ├── auth/                    # 认证模块
│   │   ├── jwt.go               # JWT 生成和验证
│   │   ├── password.go          # 密码哈希和验证
│   │   └── middleware.go        # 认证中间件
│   ├── tenant/                  # 租户模块
│   │   ├── model.go             # 租户模型
│   │   ├── repository.go        # 租户数据访问
│   │   └── service.go           # 租户业务逻辑
│   ├── user/                    # 用户模块
│   │   ├── model.go
│   │   ├── repository.go
│   │   └── service.go
│   ├── apikey/                  # API Key 模块
│   │   ├── model.go
│   │   ├── repository.go
│   │   └── service.go
│   ├── rbac/                    # RBAC 权限模块
│   │   ├── permissions.go       # 权限定义
│   │   └── middleware.go        # 权限检查中间件
│   └── quota/                   # 配额模块
│       ├── checker.go           # 配额检查
│       └── counter.go           # 用量计数
├── migrations/                  # 数据库迁移
│   ├── 001_create_tenant.up.sql
│   ├── 001_create_tenant.down.sql
│   ├── 002_create_user.up.sql
│   └── ...
├── pkg/ragsdk/                  # Go SDK（对外）
│   ├── client.go
│   ├── retrieve.go
│   └── types.go
└── docs/
    └── multi-tenant-roadmap.md  # 本文档
```
