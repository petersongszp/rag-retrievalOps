# API Key 粒度知识库权限方案

## 1. 问题分析

### 1.1 当前问题

| 问题 | 说明 |
|------|------|
| kb_ids 不可见 | 前端知识库列表没有显示 ID，用户需要查数据库 |
| 权限粒度粗 | 当前权限在租户级别，同一租户所有 Agent 共享所有知识库 |
| 配置不灵活 | Agent 配置文件硬编码 `kb_ids: [3]`，无法动态发现 |

### 1.2 当前权限模型

```
租户 (tenant_id=8)
  ├── 用户 A (owner)
  │   ├── 知识库 1 (kb_id=1) ← 自动授权 admin
  │   └── 知识库 2 (kb_id=2) ← 自动授权 admin
  ├── 用户 B (member)
  │   └── 无权限
  └── API Key X
      └── 无独立 KB 权限（继承租户权限）
```

**问题**：API Key X 可以访问租户下所有知识库（只要租户有权限）。

---

## 2. 改进方案

### 2.1 目标权限模型

```
租户 (tenant_id=8)
  ├── Agent 1 (API Key A)
  │   └── 只能访问知识库 1
  ├── Agent 2 (API Key B)
  │   └── 只能访问知识库 2
  └── Agent 3 (API Key C)
      └── 可以访问知识库 1 + 2
```

### 2.2 实现步骤

#### 步骤 1：前端显示知识库 ID

修改 `admin/src/components/admin/knowledge-base-list.tsx`，在列表中显示 `id` 字段。

#### 步骤 2：API Key 增加 kb_ids 权限字段

修改 `rag_api_key` 表，增加 `allowed_kb_ids` 字段：

```sql
ALTER TABLE rag_api_key ADD COLUMN allowed_kb_ids VARCHAR(500) DEFAULT '';
```

格式：`"1,2,3"`（逗号分隔的知识库 ID）

#### 步骤 3：创建 API Key 时指定可访问的知识库

前端 API Key 创建表单增加「可访问知识库」多选框。

#### 步骤 4：检索时校验 API Key 的 kb_ids 权限

修改 `/v1/retrieve` 接口，检查请求的 `kb_ids` 是否在 API Key 的 `allowed_kb_ids` 范围内。

#### 步骤 5：SDK 增加「自动发现知识库」接口

新增接口：`GET /v1/api-keys/me/kb`，返回当前 API Key 可访问的知识库列表。

---

## 3. 详细设计

### 3.1 数据库变更

```sql
-- API Key 表增加字段
ALTER TABLE rag_api_key 
ADD COLUMN allowed_kb_ids VARCHAR(500) DEFAULT '' COMMENT '可访问的知识库ID，逗号分隔，为空表示可访问所有';
```

### 3.2 API Key 创建接口变更

**POST /v1/api-keys**

```json
{
  "name": "agent-1-key",
  "app_id": "interview-agent",
  "allowed_kb_ids": [1, 2],
  "permissions": ["retrieve"]
}
```

### 3.3 新增「查询可访问知识库」接口

**GET /v1/api-keys/me/kb**

认证：API Key

响应：
```json
{
  "code": 200,
  "data": {
    "kb_ids": [1, 2],
    "knowledge_bases": [
      {"id": 1, "name": "面试知识库", "status": "active"},
      {"id": 2, "name": "JVM 知识库", "status": "active"}
    ]
  }
}
```

### 3.4 检索接口权限校验变更

```go
// 在 /v1/retrieve 中增加校验
func validateAPIKeyKBAccess(apiKey *model.RAGAPIKey, requestedKBIDs []uint64) error {
    // 如果 API Key 没有限制 KB，允许访问所有
    if apiKey.AllowedKBIDs == "" {
        return nil
    }
    
    allowedKBIDs := parseKBIDs(apiKey.AllowedKBIDs)
    allowedSet := make(map[uint64]bool)
    for _, id := range allowedKBIDs {
        allowedSet[id] = true
    }
    
    for _, kbID := range requestedKBIDs {
        if !allowedSet[kbID] {
            return fmt.Errorf("access denied: kb_id=%d not in allowed list", kbID)
        }
    }
    return nil
}
```

### 3.5 SDK 新增自动发现方法

```go
// pkg/ragsdk/client.go

// ListAccessibleKBs 查询当前 API Key 可访问的知识库
func (c *Client) ListAccessibleKBs(ctx context.Context) (*ListKBsResponse, error) {
    url := c.BaseURL + "/v1/api-keys/me/kb"
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }
    req.Header.Set("Authorization", "Bearer "+c.APIKey)
    
    resp, err := c.HTTPClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var result ListKBsResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }
    return &result, nil
}
```

---

## 4. Agent 对接流程

### 4.1 方式一：手动配置（当前方式）

```yaml
# config.yaml
rag_platform:
  enabled: true
  base_url: "http://rag-server:8899"
  api_key: "rag_xxxxxxxxxxxx"
  kb_ids: [3]  # 手动指定
```

**缺点**：需要查数据库获取 kb_id。

### 4.2 方式二：自动发现（推荐）

```go
// Agent 启动时自动发现可访问的知识库
client := ragsdk.NewClient(ragsdk.ClientConfig{
    BaseURL: cfg.RAGPlatform.BaseURL,
    APIKey:  cfg.RAGPlatform.APIKey,
})

// 自动获取可访问的知识库
kbResp, err := client.ListAccessibleKBs(ctx)
if err != nil {
    log.Fatalf("获取知识库失败: %v", err)
}

// 使用知识库
kbIDs := kbResp.KBIDs
fmt.Printf("可访问的知识库: %v\n", kbIDs)

// 检索时使用
resp, err := client.Retrieve(ctx, ragsdk.RetrieveRequest{
    Query: "什么是 JVM 调优？",
    KBIDs: kbIDs,
    TopK:  5,
})
```

### 4.3 方式三：每个 Agent 独立 API Key

```
Agent 1 (面试 Agent)
  └── API Key A (allowed_kb_ids=[1])
      └── 只能访问面试知识库

Agent 2 (简历 Agent)
  └── API Key B (allowed_kb_ids=[2])
      └── 只能访问简历知识库

Agent 3 (通用 Agent)
  └── API Key C (allowed_kb_ids=[1,2])
      └── 可以访问所有知识库
```

**配置方式**：

```yaml
# Agent 1 配置
rag_platform:
  enabled: true
  base_url: "http://rag-server:8899"
  api_key: "rag_agent1_key_xxx"
  # 不需要配置 kb_ids，自动从 API Key 权限获取

# Agent 2 配置
rag_platform:
  enabled: true
  base_url: "http://rag-server:8899"
  api_key: "rag_agent2_key_xxx"
```

---

## 5. 实施计划

### 5.1 Phase 1：前端显示 ID（1 小时）

- [ ] 知识库列表显示 `id` 字段
- [ ] 知识库详情页显示 `id`

### 5.2 Phase 2：API Key KB 权限（4 小时）

- [ ] 数据库增加 `allowed_kb_ids` 字段
- [ ] API Key 创建/编辑支持 `kb_ids` 配置
- [ ] 前端 API Key 页面增加知识库多选
- [ ] 检索接口校验 API Key 的 KB 权限

### 5.3 Phase 3：自动发现接口（2 小时）

- [ ] 新增 `GET /v1/api-keys/me/kb` 接口
- [ ] SDK 增加 `ListAccessibleKBs` 方法
- [ ] 文档更新

### 5.4 Phase 4：Agent 集成（2 小时）

- [ ] 面试吧 Agent 改用自动发现
- [ ] 移除硬编码 `kb_ids` 配置
- [ ] 测试多 Agent 场景

---

## 6. 快速解决方案（临时）

如果需要立即使用，可以通过以下方式获取知识库 ID：

### 6.1 通过数据库查询

```sql
SELECT id, name, tenant_id, status 
FROM kb_knowledge_base 
WHERE tenant_id = 8 AND status = 'active';
```

### 6.2 通过 API 查询

```bash
# 列出所有知识库
curl http://localhost:8899/api/admin/kb/bases \
  -H "Authorization: Bearer <jwt_token>"

# 响应中包含 id 字段
{
  "items": [
    {"id": 3, "name": "test-kb-2", "tenant_id": 8},
    {"id": 4, "name": "test-kb-3", "tenant_id": 8}
  ]
}
```

### 6.3 配置示例

```yaml
rag_platform:
  enabled: true
  base_url: "http://rag-server:8899"
  api_key: "rag_xxxxxxxxxxxx"
  kb_ids: [3, 4]  # 从上面查询结果获取
```

---

## 7. 总结

| 方案 | 优点 | 缺点 | 推荐度 |
|------|------|------|--------|
| 手动配置 kb_ids | 简单直接 | 需要查数据库 | ⭐⭐ |
| API 查询后配置 | 不需要查数据库 | 需要手动操作 | ⭐⭐⭐ |
| API Key KB 权限 | 精细化控制 | 需要代码改造 | ⭐⭐⭐⭐ |
| 自动发现 | 最灵活 | 需要新增接口 | ⭐⭐⭐⭐⭐ |

**建议**：
1. 短期：先用 API 查询获取 kb_ids
2. 中期：实现 API Key KB 权限
3. 长期：实现自动发现接口
