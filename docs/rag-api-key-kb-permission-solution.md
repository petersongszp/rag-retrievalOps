# RAG 平台 API Key 与知识库权限改造方案

## 1. 文档定位

本文用于梳理当前 RAG 平台的 API Key、租户知识库权限、前端展示与 SDK 检索链路，并给出一套可实施的改造方案，目标是解决以下问题：

1. `kb_ids` 目前是绝对 ID，用户在前端看不到也难以维护。
2. 用户希望 API Key 能自动发现自己可访问的知识库，而不是每次手工传 `kb_ids`。
3. 一个租户下有多个 Agent，每个 Agent 需要独立的 API Key 和独立的知识库访问范围。

本文范围覆盖：

1. `backend/internal/model/rag_api_key.go`
2. `backend/internal/model/rag_tenant_kb_permission.go`
3. `backend/api/handler/auth/`
4. `backend/api/handler/rag/retrieve.go`
5. `backend/pkg/ragsdk/client.go`
6. `admin/src/app/(admin)/api-keys/`
7. `admin/src/app/(admin)/knowledge-bases/`

---

## 2. 当前权限模型分析

### 2.1 API Key 模型现状

`backend/internal/model/rag_api_key.go` 当前字段包含：

1. `tenant_id`、`user_id`：归属租户和创建人。
2. `app_id`：接入方标识。
3. `key_hash`、`key_prefix`：密钥存储与展示。
4. `permissions`：`text` 类型，实际存的是 JSON 字符串。
5. `status`、`expires_at`、`last_used_at`：生命周期信息。

当前模型特点：

1. API Key 自身没有结构化的知识库授权关系。
2. `permissions` 只是一个 `[]string`，并不是资源级权限模型。
3. 现有后端合同 `CreateAPIKeyRequest` / `UpdateAPIKeyRequest` 也只接受 `[]string`，没有 `kb_ids`、`kb_permissions` 这类结构化字段。

结论：

1. 当前 `rag_api_key` 更像“认证凭证 + 粗粒度能力标签”。
2. 它还不是“Agent 级知识库访问边界”。

### 2.2 租户知识库权限模型现状

`backend/internal/model/rag_tenant_kb_permission.go` 当前模型是：

1. 唯一键：`tenant_id + kb_id`
2. 权限级别：`read` / `write` / `admin`

`backend/internal/service/kb_permission_service.go` 里也明确把这张表当作“租户对知识库”的权限表来使用。

同时，`backend/api/handler/kb/handler.go` 在创建知识库时，会自动为当前租户写入一条 `admin` 权限。

结论：

1. 当前系统已经有“租户 -> KB”的授权边界。
2. 但没有“API Key -> KB”的授权边界。
3. 因此，一个租户下的多个 API Key 目前默认共享同一批 KB 可见范围。

### 2.3 API Key CRUD 接口现状

`backend/api/handler/auth/apikey.go` 的行为很直接：

1. 创建时把 `req.Permissions` 直接写入 `rag_api_key.permissions`。
2. 列表时把 `permissions` JSON 解析成 `[]string` 返回。
3. 更新时也只是更新 `name` 和 `permissions`。
4. 不校验 KB 范围，也不关联知识库名称。

这意味着：

1. API Key 创建流程没有知识库选择步骤。
2. API Key 列表页拿不到“这个 Key 对应哪些 KB”。
3. 现有接口无法支撑“一个 Agent 一把 Key 一组 KB 权限”的运营方式。

### 2.4 `/v1/retrieve` 实际鉴权现状

`backend/api/handler/rag/retrieve.go` 当前流程是：

1. 优先解析 `Authorization: Bearer rag_xxx`。
2. 命中 API Key 后，把 `permissions` 解析到 `auth.Identity.Permissions`。
3. 再调用 `authorizeRetrieveKBIDs(...)` 对请求里的 `kb_id` / `kb_ids` 做校验。

但 `authorizeRetrieveKBIDs(...)` 当前只做两件事：

1. 校验 `kb_id` 是否属于当前 `tenant_id`。
2. 校验 `rag_tenant_kb_permission` 上租户是否有该 KB 的 `read` 权限。

它没有做的事情：

1. 没有读取 API Key 自身的 KB 范围。
2. 没有根据 API Key 过滤 `requestedKBIDs`。
3. 没有“省略 `kb_ids` 时自动用该 Key 可访问 KB”的逻辑。

结论：

1. 当前 `/v1/retrieve` 已经完成了“API Key 认证”。
2. 但没有完成“API Key 级资源授权”。
3. 所以现在的真实边界是“租户级 KB 权限”，不是“API Key 级 KB 权限”。

### 2.5 SDK 检索逻辑现状

`backend/pkg/ragsdk/client.go` 当前 `RetrieveRequest` 结构为：

1. `query`
2. `kb_ids`
3. `top_k`
4. `strategy_profile`
5. `metadata_filter`

SDK 当前行为：

1. 只负责把调用方传入的 `kb_ids` 原样发给 `/v1/retrieve`。
2. 没有知识库发现接口。
3. 没有本地缓存的 KB 元数据。
4. `kb_ids` 为空时也不会自动补全。

而后端当前又要求：

1. `kb_id` 或 `kb_ids` 必填。

所以现在 Agent 侧必须显式维护绝对 ID。

### 2.6 前端现状

#### API Key 页面

`admin/src/components/admin/api-keys-page.tsx` 当前特点：

1. 创建表单只有 `name`、`app_id`、`permissions`、`expires_in`。
2. `permissions` 只支持字符串标签，如 `retrieve`、`kb:read`、`log:read`。
3. cURL 示例里直接写死 `"kb_ids": [1]`。
4. 列表页只展示 `permissions` 标签，不展示关联 KB。

这直接导致：

1. 前端无法配置 Key 对应的知识库。
2. 用户看到的是抽象能力词，不是实际资源范围。
3. 文档示例把 `kb_ids` 暴露成了接入方必须知道的内部 ID。

#### 知识库页面

`admin/src/components/admin/knowledge-bases-page.tsx` 与 `knowledge-base-detail-page.tsx` 已能展示：

1. `kb.id`
2. `name`
3. `description`
4. `vector_collection`
5. `status`

也就是说，前端已经具备“展示租户可见 KB 列表”的能力，但这套能力还没有被 API Key 页面复用。

---

## 3. 当前问题总结

### 3.1 核心问题

1. API Key 有认证能力，没有 KB 级授权能力。
2. KB 权限停留在租户级，不能支撑多 Agent 隔离。
3. SDK 依赖调用方维护绝对 `kb_ids`，接入成本高，配置也不可视。
4. 前端 API Key 页面和知识库页面是割裂的，用户无法把“Key 对应哪几个 KB”直观看出来。

### 3.2 风险

1. 同租户下任意 API Key 都可能访问租户已授权的所有 KB。
2. Agent 数量增多后，`app_id + 手工 kb_ids` 的维护会迅速失控。
3. 知识库删除、重建、迁移后，外部系统保存的绝对 ID 容易失效。
4. 运维和排障时只能看到 `api_key_id/app_id`，难以快速判断该 Key 理论上应该访问哪些 KB。

---

## 4. 改进方案：API Key 粒度的 KB 权限

## 4.1 目标模型

改造后权限分三层：

1. `Tenant -> KB`
   作用：租户基础授权边界，决定该租户整体能否拥有或使用某 KB。
2. `API Key -> KB`
   作用：Agent 级访问边界，决定某把 Key 能否检索该 KB。
3. `API Key -> Capability`
   作用：能力开关，如 `retrieve`、`log:read`，保留在 API Key 自身。

这样可以把“谁是这个 Agent”与“这个 Agent 能访问哪些 KB”分离开。

## 4.2 推荐数据模型

推荐新增一张表，而不是继续把 `kb_ids` 塞进 `permissions` 字符串数组。

### 新增表：`rag_api_key_kb_permission`

建议字段：

1. `id`
2. `api_key_id`
3. `tenant_id`
4. `kb_id`
5. `permission`
6. `created_at`
7. `updated_at`

建议约束：

1. 唯一键：`api_key_id + kb_id`
2. 冗余 `tenant_id` 用于查询与审计加速
3. `permission` 第一阶段只需要 `read`

建议保留：

1. `rag_tenant_kb_permission` 继续存在，作为上层边界
2. `rag_api_key.permissions` 继续存能力类权限，不再承载 KB 列表

### 为什么不建议继续用 `permissions: ["retrieve", "kb:1", "kb:2"]`

1. 无法做结构化查询。
2. 无法方便关联 KB 名称、状态、删除标记。
3. 更新和审计都很脆弱。
4. 与现有租户 KB 权限模型不一致，后续代码会越来越难维护。

## 4.3 Agent 与 API Key 的关系

当前代码里没有单独的 `Agent` 实体，因此建议分两阶段处理：

### 阶段 A：最小改造

1. 先把“一个 API Key 代表一个 Agent 接入实例”作为约定。
2. `name` 用于展示 Agent 名称。
3. `app_id` 继续表示接入应用或业务域。

这样可以快速满足“一个租户多个 Agent，每个 Agent 独立 Key 和 KB 权限”。

### 阶段 B：需要更强治理时

后续如果一个 Agent 需要多把 Key，或者需要轮换历史、环境区分、灰度区分，再新增：

1. `rag_agent`
2. `rag_agent_api_key`

第一期不建议直接引入，避免把改造面做大。

## 4.4 检索鉴权改造

### 改造原则

`/v1/retrieve` 上的 KB 权限最终应取两层交集：

1. 租户允许的 KB
2. API Key 允许的 KB

最终规则：

1. 如果租户无权访问某 KB，直接拒绝。
2. 如果 API Key 无权访问某 KB，直接拒绝。
3. 只有同时满足两层权限，才允许检索。

### 建议流程

当请求使用 API Key 调用 `/v1/retrieve` 时：

1. 认证 API Key，得到 `tenant_id`、`api_key_id`、`app_id`。
2. 校验 Key 具备 `retrieve` capability。
3. 查询该 Key 的 `allowed_kb_ids`。
4. 若请求显式传了 `kb_ids`：
   只允许请求集合是 `allowed_kb_ids` 的子集。
5. 若请求未传 `kb_ids`：
   自动使用该 Key 的 `allowed_kb_ids`。
6. 将最终 `effective_kb_ids` 写入检索上下文和审计日志。

### 建议返回错误

1. Key 无 `retrieve` 能力：`403 PERMISSION_DENIED`
2. Key 未绑定任何 KB：`403 NO_KB_ACCESS`
3. 请求的 KB 超出 Key 范围：`403 KB_SCOPE_DENIED`

## 4.5 推荐接口合同

### API Key 管理接口

建议扩展 `POST /v1/api-keys` 与 `PUT /v1/api-keys/:id`：

```json
{
  "name": "support-agent-prod",
  "app_id": "support-bot",
  "capabilities": ["retrieve", "log:read"],
  "kb_ids": [12, 18, 25],
  "expires_in": 0
}
```

建议响应：

```json
{
  "id": 101,
  "name": "support-agent-prod",
  "app_id": "support-bot",
  "key_prefix": "rag_abcd1234...",
  "capabilities": ["retrieve", "log:read"],
  "knowledge_bases": [
    { "id": 12, "name": "售前 FAQ", "permission": "read" },
    { "id": 18, "name": "退款政策", "permission": "read" }
  ],
  "status": "active"
}
```

### 新增 API Key 可访问 KB 查询接口

建议新增：

1. `GET /v1/api-keys/:id/knowledge-bases`
   用于 Admin UI 管理某把 Key 的资源范围
2. `GET /v1/knowledge-bases/accessible`
   使用 API Key 调用时，返回该 Key 当前可访问 KB

`GET /v1/knowledge-bases/accessible` 建议响应：

```json
{
  "items": [
    {
      "id": 12,
      "name": "售前 FAQ",
      "description": "对外咨询知识库",
      "permission": "read",
      "status": "active"
    }
  ],
  "default_kb_ids": [12]
}
```

### 检索接口兼容策略

建议 `/v1/retrieve` 兼容两种模式：

1. 显式模式：传 `kb_ids`
2. 自动模式：不传 `kb_ids`，由后端按 API Key 自动推导

推荐第一期保留 `kb_ids` 字段，但把它从“必填”降为“可选”。

---

## 5. 前端展示改进

## 5.1 API Key 页面改造目标

目标不是让用户配置一串绝对 ID，而是让用户在 UI 里直接看到：

1. 这是哪个 Agent 的 Key
2. 这把 Key 具备哪些能力
3. 这把 Key 可以访问哪些知识库

## 5.2 创建/编辑表单改造

当前表单字段：

1. `name`
2. `app_id`
3. `permissions`
4. `expires_in`

建议调整为：

1. `name`
2. `app_id`
3. `capabilities`
4. `knowledge_bases`
5. `expires_in`

其中 `knowledge_bases` 应来自当前租户可读 KB 列表，展示内容建议包含：

1. `KB 名称`
2. `KB 描述`
3. `状态`
4. `ID` 作为辅信息，不作为主展示

交互建议：

1. 使用多选下拉或穿梭框。
2. 支持按 KB 名称搜索。
3. 支持“全选当前租户可访问 KB”。
4. 当某 KB 已停用或已删除时，编辑页要给出风险提示。

## 5.3 API Key 列表页改造

建议新增“可访问知识库”列：

1. 直接展示 KB 名称标签
2. 数量多时显示前 2~3 个并附 `+N`
3. 悬浮或展开时展示完整列表

建议保留：

1. `app_id`
2. `status`
3. `last_used_at`
4. `expires_at`

建议弱化：

1. 单纯的 `permissions` 标签展示

原因是对用户真正有价值的是“这把 Key 能访问什么资源”，不是只看抽象 capability 名词。

## 5.4 检索示例展示改造

当前 cURL 示例里写死：

```json
"kb_ids": [1]
```

建议改成两种示例：

### 自动发现模式

```bash
curl -X POST "$RAG_BASE_URL/v1/retrieve" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $RAG_API_KEY" \
  -d '{
    "query": "退款政策是什么？",
    "top_k": 5
  }'
```

### 显式指定模式

```bash
curl -X POST "$RAG_BASE_URL/v1/retrieve" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $RAG_API_KEY" \
  -d '{
    "query": "退款政策是什么？",
    "kb_ids": [12],
    "top_k": 5
  }'
```

这样前端不再向用户灌输“必须知道一个内部绝对 KB ID”。

---

## 6. SDK 自动发现 KB 方案

## 6.1 目标

让 SDK 在大多数场景下做到：

1. 持有 API Key 即可工作
2. 不需要业务方手工维护 `kb_ids`
3. 仍然保留显式指定 KB 的能力

## 6.2 推荐 SDK 能力

建议给 `backend/pkg/ragsdk/client.go` 增加以下方法：

1. `ListAccessibleKnowledgeBases(ctx)`
2. `RetrieveAuto(ctx, query, opts)`
3. `ResolveKnowledgeBaseIDs(ctx, selectors)`

建议新增结构：

```go
type KnowledgeBaseInfo struct {
    ID          uint64 `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
    Permission  string `json:"permission"`
    Status      string `json:"status"`
}
```

## 6.3 自动发现逻辑

推荐行为：

1. 如果 `RetrieveRequest.KBIDs` 非空，沿用调用方指定值。
2. 如果 `RetrieveRequest.KBIDs` 为空，SDK 先调用 `GET /v1/knowledge-bases/accessible`。
3. 把返回的 `default_kb_ids` 或全部 `items[].id` 作为本次 `effective_kb_ids`。
4. 结果可做短 TTL 缓存，例如 30 秒到 5 分钟。

## 6.4 推荐缓存策略

建议 SDK 缓存：

1. key 维度：`base_url + api_key_hash`
2. value：`accessible_kbs` + `default_kb_ids`
3. TTL：默认 60 秒

缓存失效场景：

1. 检索返回 `403 KB_SCOPE_DENIED`
2. 检索返回 `403 NO_KB_ACCESS`
3. 调用方主动刷新

## 6.5 推荐调用方式

### 业务最简模式

```go
resp, err := client.Retrieve(ctx, ragsdk.RetrieveRequest{
    Query: "退款政策是什么？",
    TopK:  5,
})
```

SDK 内部自动发现并补全 `kb_ids`。

### 高级模式

```go
resp, err := client.Retrieve(ctx, ragsdk.RetrieveRequest{
    Query: "退款政策是什么？",
    KBIDs: []uint64{12},
    TopK:  5,
})
```

调用方仍然可以覆盖默认行为。

---

## 7. 实施步骤

## 7.1 Phase 1：后端模型与合同落地

目标：

1. 引入 API Key 级 KB 权限模型
2. 不破坏现有 API Key 基本能力

任务：

1. 新增 `rag_api_key_kb_permission` 模型、repository、service
2. 扩展 `CreateAPIKeyRequest` / `UpdateAPIKeyRequest`
3. 扩展 `APIKeyItem` / `CreateAPIKeyResponse`
4. 为 API Key 新增查询可访问 KB 的接口
5. 新增迁移脚本

验收：

1. 后端可创建带 KB 范围的 API Key
2. 列表接口能返回 KB 名称和 ID
3. 未绑定 KB 的 Key 在管理端可被识别

## 7.2 Phase 2：检索鉴权切换到 API Key 粒度

目标：

1. `/v1/retrieve` 真正按 API Key 控制 KB 范围

任务：

1. 改造 `authorizeRetrieveKBIDs(...)`
2. 支持 `kb_ids` 省略时自动推导
3. 记录 `effective_kb_ids` 与 `permission_result`
4. 补齐 API Key capability 校验

验收：

1. 同租户两个 API Key 对不同 KB 的越权访问会被拒绝
2. 不传 `kb_ids` 时可自动命中 Key 已授权 KB
3. 审计日志能看到 `api_key_id` 和最终 KB 范围

## 7.3 Phase 3：前端 API Key 页面改造

目标：

1. 用户能在 UI 中配置并查看 Key 对应的 KB

任务：

1. 复用知识库列表接口
2. API Key 创建/编辑弹窗增加 KB 多选
3. 列表页增加 KB 展示列
4. cURL 示例改为自动发现优先

验收：

1. 用户不需要记忆 KB 绝对 ID
2. 用户能直接从 UI 看出某个 Agent Key 能访问哪些 KB

## 7.4 Phase 4：SDK 自动发现

目标：

1. 降低接入复杂度

任务：

1. 新增 `ListAccessibleKnowledgeBases`
2. 新增自动发现与缓存逻辑
3. 更新 Go SDK 使用示例

验收：

1. 只提供 `BaseURL + APIKey` 即可完成默认检索
2. 业务方不再必须手填 `kb_ids`

## 7.5 Phase 5：迁移与回滚策略

目标：

1. 平滑迁移，不中断现有接入

任务：

1. 兼容旧 `permissions` 字段
2. 对历史 API Key 设置默认 KB 范围
3. 灰度开启“`kb_ids` 可选 + API Key 自动推导”
4. 监控 401/403 与空结果率

建议迁移策略：

1. 历史 Key 默认继承当前租户已授权 KB 全量范围
2. 新建 Key 必须显式绑定 KB
3. 待所有 Agent 完成迁移后，再收紧历史兼容逻辑

---

## 8. 推荐落地顺序

1. 先做后端表结构与接口扩展。
2. 再做 `/v1/retrieve` 的 API Key 级鉴权。
3. 然后改造前端 API Key 页面。
4. 最后补 SDK 自动发现与接入文档。

原因：

1. 如果先改 SDK 或前端，没有后端资源级授权，实际安全边界仍然不存在。
2. 如果先改鉴权但前端还不能配置 KB，会导致 Key 创建后不可用或难以运维。

---

## 9. 最终建议

推荐采用以下原则作为正式方案：

1. `rag_tenant_kb_permission` 继续作为租户级授权边界。
2. 新增 `rag_api_key_kb_permission` 作为 Agent 级授权边界。
3. `rag_api_key.permissions` 只保留 capability，不再承载 KB 列表。
4. `/v1/retrieve` 支持不传 `kb_ids`，由 API Key 自动发现可访问 KB。
5. Admin UI 直接展示 KB 名称与范围，避免用户维护绝对 ID。

这套方案能同时满足：

1. 多 Agent 隔离
2. 前端可视化配置
3. SDK 低接入成本
4. 后端真实资源级鉴权

