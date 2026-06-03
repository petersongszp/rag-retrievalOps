# Phase 0 — L0 认证与路由基线盘点报告

> 生成时间：2026-06-02  
> 项目路径：`D:\Bear\rag-retrievalOps`  
> 分支：`LittleBear`  
> 状态：基线锁定，仅可追加不可回退

---

## 1. 路由清单

### 1.1 公共路由（无认证组）

| # | 路由 | 方法 | 认证方式 | 身份来源 | 风险等级 | 标签 |
|---|------|------|---------|---------|---------|------|
| 1 | `/healthz` | GET | 无 | 无 | 🟢 低 | — |
| 2 | `/readyz` | GET | 无 | 无 | 🟢 低 | — |

### 1.2 v1 公开检索路由

| # | 路由 | 方法 | 认证方式 | 身份来源 | 风险等级 | 标签 |
|---|------|------|---------|---------|---------|------|
| 3 | `/v1/retrieve` | POST | app_id 静态白名单 | 请求体 `app_id` 字段 | 🟡 中 | public_api |

### 1.3 `/api/kb/*` 路由（JWT 认证）

所有 `/api/kb/*` 路由通过 `main.go` 中间件走 `ParseAndSetUserFromToken`，即从 JWT token 中解析身份。token 无效时 `user_id=0`（匿名）。

| # | 路由 | 方法 | 认证方式 | 身份来源 | 风险等级 | 标签 |
|---|------|------|---------|---------|---------|------|
| 4 | `/api/kb/dashboard/stats` | GET | JWT | ParseAndSetUserFromToken | 🟡 中 | kb_common |
| 5 | `/api/kb/bases` | POST | JWT | ParseAndSetUserFromToken | 🟡 中 | kb_common |
| 6 | `/api/kb/bases` | GET | JWT | ParseAndSetUserFromToken | 🟡 中 | kb_common |
| 7 | `/api/kb/bases/:kb_id` | DELETE | JWT | ParseAndSetUserFromToken | 🟡 中 | kb_common |
| 8 | `/api/kb/documents/upload` | POST | JWT | ParseAndSetUserFromToken | 🟡 中 | kb_common |
| 9 | `/api/kb/documents` | GET | JWT | ParseAndSetUserFromToken | 🟡 中 | kb_common |
| 10 | `/api/kb/documents/:document_id` | DELETE | JWT | ParseAndSetUserFromToken | 🟡 中 | kb_common |
| 11 | `/api/kb/jobs` | GET | JWT | ParseAndSetUserFromToken | 🟢 低 | kb_common |
| 12 | `/api/kb/jobs/:job_id` | GET | JWT | ParseAndSetUserFromToken | 🟢 低 | kb_common |
| 13 | `/api/kb/jobs/:job_id/retry` | POST | JWT | ParseAndSetUserFromToken | 🟡 中 | kb_common |
| 14 | `/api/kb/jobs/:job_id/cancel` | POST | JWT | ParseAndSetUserFromToken | 🟡 中 | kb_common |
| 15 | `/api/kb/retrieve` | POST | JWT | ParseAndSetUserFromToken | 🟡 中 | kb_common |
| 16 | `/api/kb/retrieve/audit/:request_id` | GET | JWT | ParseAndSetUserFromToken | 🟢 低 | kb_common |
| 17 | `/api/kb/retrieve/audit/:request_id/debug` | GET | JWT | ParseAndSetUserFromToken | 🟢 低 | kb_common (phase3 debug) |
| 18 | `/api/kb/retrieve/debug/:request_id` | GET | JWT | ParseAndSetUserFromToken | 🟢 低 | kb_common (legacy debug) |
| 19 | `/api/kb/retrieve/audit` | GET | JWT | ParseAndSetUserFromToken | 🟢 低 | kb_common |
| 20 | `/api/kb/metrics/overview` | GET | JWT | ParseAndSetUserFromToken | 🟢 低 | kb_common |
| 21 | `/api/kb/logs/ingest` | GET | JWT | ParseAndSetUserFromToken | 🟢 低 | kb_common |
| 22 | `/api/kb/logs/ingest/:job_id` | GET | JWT | ParseAndSetUserFromToken | 🟢 低 | kb_common |
| 23 | `/api/kb/ingest/pause` | POST | JWT | ParseAndSetUserFromToken | 🟡 中 | kb_common |
| 24 | `/api/kb/ingest/resume` | POST | JWT | ParseAndSetUserFromToken | 🟡 中 | kb_common |
| 25 | `/api/kb/ingest/status` | GET | JWT | ParseAndSetUserFromToken | 🟢 低 | kb_common |

### 1.4 `/api/admin/kb/*` 路由（Admin 注入）

所有 `/api/admin/kb/*` 路由通过 `main.go` 中间件**硬编码注入**身份：`user_id=1, role=admin, username=admin`。无环境门禁，dev 和 prod 行为一致。

admin 路由包含上述 `/api/kb/*` 的全部 22 条公共路由，加上以下 admin-only 路由：

| # | 路由 | 方法 | 认证方式 | 身份来源 | 风险等级 | 标签 |
|---|------|------|---------|---------|---------|------|
| 26 | `/api/admin/kb/eval/datasets` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | eval |
| 27 | `/api/admin/kb/eval/datasets` | POST | Admin 注入 | user_id=1/role=admin | 🔴 高 | eval |
| 28 | `/api/admin/kb/eval/datasets/:id/items` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | eval |
| 29 | `/api/admin/kb/eval/datasets/:id/items` | POST | Admin 注入 | user_id=1/role=admin | 🔴 高 | eval |
| 30 | `/api/admin/kb/eval/datasets/:id/items/import` | POST | Admin 注入 | user_id=1/role=admin | 🔴 高 | eval |
| 31 | `/api/admin/kb/eval/datasets/:id/items/export` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | eval |
| 32 | `/api/admin/kb/eval/datasets/:id/validate` | POST | Admin 注入 | user_id=1/role=admin | 🔴 高 | eval |
| 33 | `/api/admin/kb/eval/runs` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | eval |
| 34 | `/api/admin/kb/eval/runs` | POST | Admin 注入 | user_id=1/role=admin | 🔴 高 | eval |
| 35 | `/api/admin/kb/eval/runs/:run_id` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | eval |
| 36 | `/api/admin/kb/eval/runs/:run_id/report` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | eval |
| 37 | `/api/admin/kb/eval/runs/:run_id/cases` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | eval |
| 38 | `/api/admin/kb/eval/runs/:run_id/report/export` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | eval |
| 39 | `/api/admin/kb/strategy/flags` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | strategy |
| 40 | `/api/admin/kb/strategy/flags/:flag_key` | PATCH | Admin 注入 | user_id=1/role=admin | 🔴 高 | strategy |
| 41 | `/api/admin/kb/strategy/versions` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | strategy |
| 42 | `/api/admin/kb/strategy/versions/:version_id` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | strategy |
| 43 | `/api/admin/kb/strategy/rollback` | POST | Admin 注入 | user_id=1/role=admin | 🔴 高 | strategy |
| 44 | `/api/admin/kb/strategy/impact` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | strategy |
| 45 | `/api/admin/kb/strategy/gates` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | strategy |
| 46 | `/api/admin/kb/strategy/operations` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | strategy |
| 47 | `/api/admin/kb/experiments` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | experiment |
| 48 | `/api/admin/kb/experiments` | POST | Admin 注入 | user_id=1/role=admin | 🔴 高 | experiment |
| 49 | `/api/admin/kb/experiments/:id/rollback` | POST | Admin 注入 | user_id=1/role=admin | 🔴 高 | experiment |
| 50 | `/api/admin/kb/experiments/summary` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | experiment |
| 51 | `/api/admin/kb/index-lifecycle/registry` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | index_lifecycle |
| 52 | `/api/admin/kb/index-lifecycle/register` | POST | Admin 注入 | user_id=1/role=admin | 🔴 高 | index_lifecycle |
| 53 | `/api/admin/kb/index-lifecycle/build` | POST | Admin 注入 | user_id=1/role=admin | 🔴 高 | index_lifecycle |
| 54 | `/api/admin/kb/index-lifecycle/health/:index_version` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | index_lifecycle |
| 55 | `/api/admin/kb/index-lifecycle/switch/:index_version` | POST | Admin 注入 | user_id=1/role=admin | 🔴 高 | index_lifecycle |
| 56 | `/api/admin/kb/index-lifecycle/rollback` | POST | Admin 注入 | user_id=1/role=admin | 🔴 高 | index_lifecycle |
| 57 | `/api/admin/kb/index-lifecycle/operations` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | index_lifecycle |
| 58 | `/api/admin/kb/cost/summary` | GET | Admin 注入 | user_id=1/role=admin | 🟡 中 | cost |
| 59 | `/api/admin/kb/cost/timeseries` | GET | Admin 注入 | user_id=1/role=admin | 🟡 中 | cost |
| 60 | `/api/admin/kb/cost/by-kb` | GET | Admin 注入 | user_id=1/role=admin | 🟡 中 | cost |
| 61 | `/api/admin/kb/cost/by-strategy` | GET | Admin 注入 | user_id=1/role=admin | 🟡 中 | cost |
| 62 | `/api/admin/kb/cost/by-model` | GET | Admin 注入 | user_id=1/role=admin | 🟡 中 | cost |
| 63 | `/api/admin/kb/cost/high-cost-queries` | GET | Admin 注入 | user_id=1/role=admin | 🟡 中 | cost |
| 64 | `/api/admin/kb/vector/collections` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | vector_ops |
| 65 | `/api/admin/kb/vector/collections/:name/health` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | vector_ops |
| 66 | `/api/admin/kb/vector/collections/:name/capacity` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | vector_ops |
| 67 | `/api/admin/kb/vector/collections/:name/rebuild` | POST | Admin 注入 | user_id=1/role=admin | 🔴 高 | vector_ops |
| 68 | `/api/admin/kb/vector/collections/:name/switch` | POST | Admin 注入 | user_id=1/role=admin | 🔴 高 | vector_ops |
| 69 | `/api/admin/kb/vector/collections/:name/rollback` | POST | Admin 注入 | user_id=1/role=admin | 🔴 高 | vector_ops |
| 70 | `/api/admin/kb/vector/operations` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | vector_ops |
| 71 | `/api/admin/kb/audit/events` | GET | Admin 注入 | user_id=1/role=admin | 🟡 中 | audit |
| 72 | `/api/admin/kb/audit/events/:event_id` | GET | Admin 注入 | user_id=1/role=admin | 🟡 中 | audit |
| 73 | `/api/admin/kb/audit/events/export` | POST | Admin 注入 | user_id=1/role=admin | 🟡 中 | audit |
| 74 | `/api/admin/kb/alerts` | GET | Admin 注入 | user_id=1/role=admin | 🟡 中 | alerts |
| 75 | `/api/admin/kb/alerts/:alert_id/ack` | PATCH | Admin 注入 | user_id=1/role=admin | 🟡 中 | alerts |
| 76 | `/api/admin/kb/alerts/:alert_id/resolve` | PATCH | Admin 注入 | user_id=1/role=admin | 🟡 中 | alerts |
| 77 | `/api/admin/kb/reports/weekly` | POST | Admin 注入 | user_id=1/role=admin | 🟡 中 | reports |
| 78 | `/api/admin/kb/reports/weekly` | GET | Admin 注入 | user_id=1/role=admin | 🟡 中 | reports |
| 79 | `/api/admin/kb/reports/weekly/:report_id` | GET | Admin 注入 | user_id=1/role=admin | 🟡 中 | reports |
| 80 | `/api/admin/kb/reports/weekly/:report_id/export` | POST | Admin 注入 | user_id=1/role=admin | 🟡 中 | reports |
| 81 | `/api/admin/kb/weekly-report` | GET | Admin 注入 | user_id=1/role=admin | 🟡 中 | reports |
| 82 | `/api/admin/kb/governance/gates` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | governance |
| 83 | `/api/admin/kb/governance/gates/check` | POST | Admin 注入 | user_id=1/role=admin | 🔴 高 | governance |
| 84 | `/api/admin/kb/governance/gate` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | governance |
| 85 | `/api/admin/kb/phase4/acceptance` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | governance |
| 86 | `/api/admin/kb/release/status` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | release |
| 87 | `/api/admin/kb/release/summary` | GET | Admin 注入 | user_id=1/role=admin | 🔴 高 | release |
| 88 | `/api/admin/kb/release/rollback` | POST | Admin 注入 | user_id=1/role=admin | 🔴 高 | release |
| 89 | `/api/admin/kb/release/activate` | POST | Admin 注入 | user_id=1/role=admin | 🔴 高 | release |

> **统计**：共计 89 条路由。其中 `/api/kb/*` 公共路由 22 条，`/api/admin/kb/*` admin-only 路由 64 条（含 22 条与 `/api/kb` 同构路由 + 42 条 admin 专属路由），`/v1/retrieve` 1 条，健康检查 2 条。

---

## 2. 认证方式分析

### 2.1 Admin 注入（`/api/admin/*`）

**位置**：`cmd/rag-server/main.go` — 全局中间件

**工作原理**：
```go
if strings.HasPrefix(path, "/api/admin/") {
    c.Set("user_id", uint(1))
    c.Set("role", "admin")
    c.Set("username", "admin")
    c.Next(ctx)
    return
}
```

- 无条件注入，不检查 token、不检查环境变量
- 所有 `/api/admin/*` 请求自动获得 `user_id=1, role=admin`
- 无环境门禁：dev/staging/prod 行为完全一致

**标记**：`legacy` — 应在 Phase 1 中替换为真实认证

### 2.2 JWT 认证（`/api/kb/*`）

**位置**：`internal/middleware/jwt.go` — `ParseAndSetUserFromToken`

**工作原理**：
1. 从 `Authorization: Bearer <token>` / `X-Auth-Token` / `?token=` / Cookie 中提取 token
2. 使用 `config.Global.Security.JWTSecret` 进行 HS256 签名验证
3. 解析出 `JWTClaims`，写入 Hertz context

**JWT Claims 结构**（`internal/middleware/jwt.go`）：
```go
type JWTClaims struct {
    UserID   uint   `json:"user_id"`
    Username string `json:"username"`
    Role     string `json:"role"`
    jwt.RegisteredClaims
}
```

**当前字段**：
| 字段 | 存在 | 用途 |
|------|------|------|
| `user_id` | ✅ | 用户标识 |
| `username` | ✅ | 用户名 |
| `role` | ✅ | 角色（admin/user） |
| `exp/iat/nbf/iss` | ✅ | 标准 JWT 注册字段 |

**当前缺失**：
| 字段 | 状态 | 用途（Phase 1+ 需要） |
|------|------|----------------------|
| `tenant_id` | ❌ 缺失 | 多租户隔离标识 |
| `auth_type` | ❌ 缺失 | 认证来源类型（jwt/api_key/sso） |
| `app_id` | ❌ 缺失 | 应用标识 |
| `api_key_id` | ❌ 缺失 | API Key 关联 ID |
| `permissions` | ❌ 缺失 | 细粒度权限列表 |

**降级行为**：token 无效或缺失时，`ParseAndSetUserFromToken` 返回 `user_id=0`，不 abort 请求。下游 handler 依赖 `user_id` 做数据隔离，`user_id=0` 可能导致数据泄露或访问异常。

### 2.3 app_id 白名单（`/v1/retrieve`）

**位置**：`api/handler/rag/retrieve.go`

**工作原理**：
```go
var allowedAppIDs = map[string]string{
    "interview-agent":   "interview-agent",
    "mianshiba-web":     "mianshiba-web",
    "mianshiba-admin":   "mianshiba-admin",
}
```

1. 从请求体解析 `app_id` 字段
2. 在硬编码 map 中查找匹配
3. 未匹配则返回 403 Forbidden
4. 匹配后委托给 `kb.Retrieve` 处理

**标记**：`dev_only` — 硬编码白名单，无动态管理能力

---

## 3. JWT 中间件字段盘点

### 3.1 当前 JWT Claims 结构

```go
// internal/middleware/jwt.go
type JWTClaims struct {
    UserID   uint   `json:"user_id"`   // 用户 ID
    Username string `json:"username"`  // 用户名
    Role     string `json:"role"`      // 角色
    jwt.RegisteredClaims               // exp, iat, nbf, iss
}
```

### 3.2 字段缺口矩阵

| 字段 | 当前状态 | 多租户 Phase 3 需要 | API Key Phase 2 需要 | 说明 |
|------|---------|-------------------|--------------------|-----|
| `user_id` | ✅ 有 | ✅ | ✅ | 用户标识 |
| `username` | ✅ 有 | ⚪ 可选 | ⚪ 可选 | 展示用 |
| `role` | ✅ 有 | ⚠️ 需扩展 | ⚠️ 需扩展 | 当前仅 admin/user，缺少细粒度权限 |
| `tenant_id` | ❌ 缺 | ✅ 必须 | ⚠️ 建议 | 多租户隔离的唯一标识 |
| `auth_type` | ❌ 缺 | ✅ 必须 | ✅ 必须 | 区分 jwt/api_key/sso 认证来源 |
| `app_id` | ❌ 缺 | ⚠️ 建议 | ✅ 必须 | 应用级标识 |
| `api_key_id` | ❌ 缺 | ⚪ 不需要 | ✅ 必须 | API Key 关联，用于审计和吊销 |
| `permissions` | ❌ 缺 | ✅ 必须 | ✅ 必须 | 细粒度权限列表 |

### 3.3 Token 提取优先级

当前 `extractToken` 按以下顺序查找 token：
1. `Authorization: Bearer <token>` header
2. `X-Auth-Token` header
3. `?token=<token>` query parameter
4. `token` cookie

---

## 4. 公开检索契约（`/v1/retrieve`）

### 4.1 请求结构

```go
// api/handler/rag/retrieve.go
type RAGRetrieveRequest struct {
    AppID           string                 `json:"app_id"`            // 应用标识（必填，白名单校验）
    KBID            uint64                 `json:"kb_id"`             // 单个知识库 ID
    KBIDs           []uint64               `json:"kb_ids"`            // 多知识库 ID 列表
    Query           string                 `json:"query"`             // 检索查询（必填）
    TopK            int                    `json:"top_k"`             // 返回结果数量
    StrategyProfile string                 `json:"strategy_profile"`  // 策略配置名
    MetadataFilter  map[string]interface{} `json:"metadata_filter"`   // 元数据过滤条件
}
```

### 4.2 响应结构

```go
type RAGRetrieveResponse struct {
    RequestID       string            `json:"request_id"`       // 请求追踪 ID
    Items           []RAGRetrieveItem `json:"items"`            // 检索结果列表
    StrategyVersion string            `json:"strategy_version"` // 使用的策略版本
    RequestCost     RAGRequestCost    `json:"request_cost"`     // 请求成本信息
}

type RAGRetrieveItem struct {
    Content  string      `json:"content"`  // 文本内容
    Score    float64     `json:"score"`    // 相关性得分
    Citation RAGCitation `json:"citation"` // 引用信息
    Source   RAGSource   `json:"source"`   // 来源信息
}

type RAGCitation struct {
    KBID       uint64 `json:"kb_id"`       // 知识库 ID
    DocumentID uint64 `json:"document_id"` // 文档 ID
    ChunkID    string `json:"chunk_id"`    // 分块 ID
    FileName   string `json:"file_name"`   // 文件名
    ChunkIndex int    `json:"chunk_index"` // 分块序号
}

type RAGSource struct {
    Route            string `json:"route"`              // 检索路由
    Collection       string `json:"collection"`         // Milvus 集合名
    RetrieverVersion string `json:"retriever_version"`  // 检索器版本
}

type RAGRequestCost struct {
    EstimatedCost float64 `json:"estimated_cost"` // 预估成本
}
```

### 4.3 行为特征

- `app_id` 为硬编码白名单校验，非租户级隔离
- `kb_id` / `kb_ids` 可选，未指定时使用默认知识库
- 实际检索逻辑委托给 `kb.Retrieve` handler
- 响应中包含 `request_id` 用于审计追踪

---

## 5. 配置入口盘点

### 5.1 配置文件结构

| 文件 | 用途 | 关键字段 |
|------|------|---------|
| `backend/config.yaml` | 主配置文件 | host, port, database, redis, security, rag |
| `backend/config.rag.example.yaml` | RAG 配置示例 | rag, Embedding, Milvus, DocumentSplitter |
| `backend/internal/config/config.go` | 配置结构定义 | Config, RAGConfig, SecurityConfig 等 |

### 5.2 认证相关配置

```yaml
# config.yaml
security:
  jwt_secret: "your-rag-jwt-secret-key"    # JWT 签名密钥
  jwt_expiration: "24h"                      # Token 过期时间

rag:
  environment: dev                           # 环境标识 (dev/staging/prod)
  enabled: true                              # RAG 功能总开关
```

### 5.3 配置加载流程

1. 读取 `config.yaml`
2. 环境变量替换（`${VAR}` / `$VAR`）
3. 叠加环境特定配置（`config.{env}.yaml`、`config.{env}.local.yaml`）
4. 应用环境变量覆盖（`RAG_*` 系列）
5. 标准化 Phase 4 别名
6. 应用默认值
7. 生成配置版本号
8. 写入 Phase 1/2/3 基线快照（首次启动时）

### 5.4 app_id 白名单管理

当前 `allowedAppIDs` 硬编码在 `api/handler/rag/retrieve.go` 中，无配置化管理：
```go
var allowedAppIDs = map[string]string{
    "interview-agent":   "interview-agent",
    "mianshiba-web":     "mianshiba-web",
    "mianshiba-admin":   "mianshiba-admin",
}
```

**标记**：`dev_only` — 需要在 Phase 2 中迁移到配置文件或数据库

---

## 6. 风险清单

### 6.1 高风险

| # | 风险 | 影响范围 | 当前状态 | 建议缓解 |
|---|------|---------|---------|---------|
| R1 | Admin 注入无环境门禁 | 所有 `/api/admin/*` 路由（64 条） | 生产环境可直接访问 admin API，无需认证 | Phase 1：增加环境变量门禁或移除注入中间件 |
| R2 | JWT 缺少 `tenant_id` | 所有 `/api/kb/*` 路由（22 条） | 无法区分租户，数据隔离依赖 `user_id` | Phase 1：扩展 JWT Claims |
| R3 | token 无效时不 abort | 所有 `/api/kb/*` 路由 | `ParseAndSetUserFromToken` 返回 `user_id=0` 后继续执行 | 立即修复：增加 abort 逻辑 |
| R4 | `app_id` 白名单硬编码 | `/v1/retrieve` | 无动态管理，新增应用需改代码 | Phase 2：迁移到配置或数据库 |
| R5 | JWT Secret 硬编码默认值 | 所有 JWT 认证路由 | `config.yaml` 中 `jwt_secret: "your-rag-jwt-secret-key"` | 生产部署时必须覆盖 |

### 6.2 中风险

| # | 风险 | 影响范围 | 当前状态 | 建议缓解 |
|---|------|---------|---------|---------|
| R6 | 无租户级数据隔离 | 全局 | 所有数据查询无 `WHERE tenant_id = ?` | Phase 3：数据层增加租户过滤 |
| R7 | 无 API Key 认证 | `/v1/retrieve` | 仅 app_id 白名单，无法吊销/轮换 | Phase 2：新增 API Key 中间件 |
| R8 | Token 提取渠道过多 | JWT 认证 | 支持 header/query/cookie，攻击面大 | Phase 1：收窄为仅 header |
| R9 | 无请求速率限制中间件 | 全局 | `ratelimit.go` 存在但未在 main.go 中启用 | 立即修复：启用限流 |

### 6.3 低风险

| # | 风险 | 影响范围 | 当前状态 | 建议缓解 |
|---|------|---------|---------|---------|
| R10 | 健康检查无认证 | `/healthz`, `/readyz` | 可被外部探测服务状态 | 可接受，但生产环境建议 IP 白名单 |
| R11 | CORS 配置未在 rag-server 中显式设置 | 全局 | 依赖 Hertz 默认 CORS 行为 | Phase 1：显式配置 CORS |

---

## 7. Phase 1/2/3 依赖关系

### 7.1 Phase 1（JWT 扩展）依赖

| 依赖项 | 来源 | 状态 |
|--------|------|------|
| JWT Claims 结构扩展 | `internal/middleware/jwt.go` | 需新增 `tenant_id`, `auth_type` 字段 |
| Token 生成函数更新 | `GenerateToken()` | 需支持新字段 |
| Admin 注入中间件重构 | `cmd/rag-server/main.go` | 需替换为环境门禁 + 真实认证 |
| Handler 身份读取统一 | 所有 handler | 需统一使用 `GetUserID()` / 新增 `GetTenantID()` |

### 7.2 Phase 2（API Key）依赖

| 依赖项 | 来源 | 状态 |
|--------|------|------|
| API Key 数据模型 | 数据库 | 需新建 `api_keys` 表 |
| API Key 认证中间件 | `internal/middleware/` | 需新增 `apikey.go` |
| app_id 白名单迁移 | `api/handler/rag/retrieve.go` | 从硬编码迁移到数据库 |
| Key 轮换/吊销机制 | 管理 API | 需新增 admin 路由 |

### 7.3 Phase 3（多租户隔离）依赖

| 依赖项 | 来源 | 状态 |
|--------|------|------|
| `tenant_id` 字段全链路注入 | JWT + 数据库 + Milvus | 需 Phase 1 完成后才能开始 |
| 数据库查询租户过滤 | 所有 repository 层 | 需新增 `WHERE tenant_id = ?` |
| Milvus 集合租户隔离 | `internal/milvus/` | 需决定集合级或分区级隔离 |
| 知识库租户绑定 | `knowledge_bases` 表 | 需新增 `tenant_id` 外键 |

---

## 8. 基线快照元数据

```
snapshot_type:   phase0_l0_baseline
generated_at:    2026-06-02T22:35:00+08:00
project:         rag-retrievalOps
branch:          LittleBear
total_routes:    89
auth_methods:    3 (admin_inject, jwt, appid_whitelist)
risk_summary:    high=5, medium=4, low=2
```

---

## 附录 A：文件依赖清单

| 文件 | 角色 | 盘点状态 |
|------|------|---------|
| `api/ragrouter/register.go` | 路由注册入口 | ✅ 已扫描 |
| `cmd/rag-server/main.go` | 服务启动 + 中间件注入 | ✅ 已扫描 |
| `internal/middleware/jwt.go` | JWT 认证实现 | ✅ 已扫描 |
| `api/handler/rag/retrieve.go` | v1 公开检索 handler | ✅ 已扫描 |
| `api/handler/kb/handler.go` | KB handler 集合 | ✅ 已扫描（头部） |
| `internal/config/config.go` | 配置结构定义 | ✅ 已扫描 |
| `config.yaml` | 主配置文件 | ✅ 已扫描 |
| `internal/rag/phase3/contract.go` | Phase 3 路由常量 | ✅ 已扫描 |
| `internal/middleware/ratelimit.go` | IP 限流器 | ✅ 已扫描（未启用） |

---

*报告结束。此文件为只读基线，后续阶段不得修改已有内容，仅可追加。*
