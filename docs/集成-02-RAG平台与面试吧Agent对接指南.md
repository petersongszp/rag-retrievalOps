# RAG 平台与面试吧 Agent 对接指南

## 1. 架构概览

```
面试吧 Agent (mianshiba-eino-overseas)
    │
    │ HTTP POST /v1/retrieve
    │ Authorization: Bearer rag_xxx
    ▼
RAG Platform (rag-retrievalOps)
    │
    ├── 知识库管理
    ├── 文档入库
    ├── 向量检索 (Milvus)
    └── 租户隔离
```

---

## 2. 前置条件

### 2.1 RAG Platform 已部署

- RAG Server 运行在 `http://localhost:8899`（或你的服务器地址）
- Admin 前端运行在 `http://localhost:3003`

### 2.2 创建租户和 API Key

1. 打开 `http://localhost:3003/register` 注册账户
2. 登录后进入 `/api-keys` 页面创建 API Key
3. 记录 API Key（格式：`rag_xxxxxxxxxxxx`，只显示一次）

### 2.3 创建知识库并上传文档

1. 进入 `/knowledge-bases` 创建知识库
2. 进入知识库详情，上传文档（支持 `.txt`、`.md`、`.pdf`）
3. 等待文档处理完成（状态变为 `completed`）

---

## 3. 对接方式

### 方式一：使用 Go SDK（推荐）

RAG Platform 已提供 Go SDK，位于 `backend/pkg/ragsdk/client.go`。

**步骤 1：复制 SDK 到面试吧项目**

将 `rag-retrievalOps/backend/pkg/ragsdk/` 目录复制到面试吧项目的 `backend/pkg/ragsdk/`。

**步骤 2：配置 RAG Platform 地址**

在面试吧项目的 `config.yaml` 中添加：

```yaml
rag_platform:
  enabled: true
  base_url: "http://localhost:8899"  # RAG Platform 地址
  api_key: "rag_xxxxxxxxxxxx"        # 你的 API Key
```

**步骤 3：在 Agent 中调用 SDK**

```go
package main

import (
    "context"
    "fmt"
    "log"

    "interview-agents/pkg/ragsdk"
)

func main() {
    // 创建客户端
    client := ragsdk.NewClient(ragsdk.ClientConfig{
        BaseURL: "http://localhost:8899",
        APIKey:  "rag_xxxxxxxxxxxx",
    })

    // 执行检索
    resp, err := client.Retrieve(context.Background(), ragsdk.RetrieveRequest{
        Query: "什么是 JVM 调优？",
        KBIDs: []uint64{1, 2},  // 指定知识库 ID
        TopK:  5,
    })
    if err != nil {
        log.Fatalf("检索失败: %v", err)
    }

    // 处理结果
    fmt.Printf("请求 ID: %s\n", resp.RequestID)
    for i, item := range resp.Items {
        fmt.Printf("[%d] 分数: %.2f, 内容: %s\n", i+1, item.Score, item.Content)
    }
}
```

### 方式二：直接 HTTP 调用

如果不使用 Go SDK，可以直接调用 HTTP API。

**检索接口：**

```
POST http://localhost:8899/v1/retrieve
Authorization: Bearer rag_xxxxxxxxxxxx
Content-Type: application/json
```

**请求体：**

```json
{
    "query": "什么是 JVM 调优？",
    "kb_ids": [1, 2],
    "top_k": 5,
    "strategy_profile": "default",
    "metadata_filter": {}
}
```

**响应体：**

```json
{
    "code": 200,
    "message": "Success",
    "data": {
        "request_id": "uuid-xxxx",
        "items": [
            {
                "content": "JVM 调优是指...",
                "score": 0.85,
                "citation": {
                    "kb_id": 1,
                    "document_id": 10,
                    "chunk_id": "chunk_001",
                    "file_name": "jvm-guide.md"
                },
                "source": {
                    "route": "dense",
                    "collection": "kb_1_docs",
                    "retriever_version": "phase2-hybrid-v1"
                }
            }
        ]
    }
}
```

### 方式三：curl 命令测试

```bash
# 检索
curl -X POST http://localhost:8899/v1/retrieve \
  -H "Authorization: Bearer rag_xxxxxxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{"query":"什么是 JVM 调优？","kb_ids":[1],"top_k":5}'

# 上传文档
curl -X POST http://localhost:8899/api/admin/kb/documents/upload \
  -H "Authorization: Bearer <JWT_TOKEN>" \
  -F "kb_id=1" \
  -F "file=@/path/to/document.txt"
```

---

## 4. API 接口文档

### 4.1 检索接口

**POST /v1/retrieve**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| query | string | 是 | 检索查询内容 |
| kb_ids | uint64[] | 否 | 知识库 ID 列表，不传则检索所有已授权知识库 |
| top_k | int | 否 | 返回结果数量，默认 5 |
| strategy_profile | string | 否 | 策略配置，默认 "default" |
| metadata_filter | object | 否 | 元数据过滤条件 |

**认证方式：**
- API Key：`Authorization: Bearer rag_xxxxxxxxxxxx`
- JWT：`Authorization: Bearer <jwt_token>`

**响应字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| request_id | string | 请求 ID，用于日志追踪 |
| items | array | 检索结果列表 |
| items[].content | string | 文本内容 |
| items[].score | float64 | 相关性分数 |
| items[].citation.kb_id | uint64 | 知识库 ID |
| items[].citation.document_id | uint64 | 文档 ID |
| items[].citation.chunk_id | string | 分块 ID |
| items[].citation.file_name | string | 文件名 |
| items[].source.route | string | 检索路由（dense/sparse/hybrid） |
| items[].source.collection | string | Milvus 集合名 |

### 4.2 知识库管理接口

| 接口 | 方法 | 说明 |
|------|------|------|
| /api/admin/kb/bases | GET | 列出知识库 |
| /api/admin/kb/bases | POST | 创建知识库 |
| /api/admin/kb/bases/:kb_id | DELETE | 删除知识库 |
| /api/admin/kb/documents/upload | POST | 上传文档 |
| /api/admin/kb/documents | GET | 列出文档 |
| /api/admin/kb/jobs | GET | 列出入库任务 |

### 4.3 API Key 管理接口

| 接口 | 方法 | 说明 |
|------|------|------|
| /v1/api-keys | GET | 列出 API Key |
| /v1/api-keys | POST | 创建 API Key |
| /v1/api-keys/:id | DELETE | 删除 API Key |

---

## 5. 面试吧 Agent 改造示例

### 5.1 替换现有 Milvus Tool

**改造前（直接调用 Milvus）：**

```go
// 旧方式：直接调用内部 Milvus
results, err := manager.RetrieverService.Retrieve(ctx, query)
```

**改造后（调用 RAG Platform）：**

```go
// 新方式：通过 RAG Platform SDK
client := ragsdk.NewClient(ragsdk.ClientConfig{
    BaseURL: cfg.RAGPlatform.BaseURL,
    APIKey:  cfg.RAGPlatform.APIKey,
})

resp, err := client.Retrieve(ctx, ragsdk.RetrieveRequest{
    Query: query,
    KBIDs: []uint64{kbID},
    TopK:  5,
})
```

### 5.2 在 Agent Tool 中集成

```go
package tools

import (
    "context"
    "fmt"

    "interview-agents/pkg/ragsdk"
)

type RAGRetrieveTool struct {
    client *ragsdk.Client
}

func NewRAGRetrieveTool(baseURL, apiKey string) *RAGRetrieveTool {
    return &RAGRetrieveTool{
        client: ragsdk.NewClient(ragsdk.ClientConfig{
            BaseURL: baseURL,
            APIKey:  apiKey,
        }),
    }
}

func (t *RAGRetrieveTool) Execute(ctx context.Context, query string, kbIDs []uint64) (string, error) {
    resp, err := t.client.Retrieve(ctx, ragsdk.RetrieveRequest{
        Query: query,
        KBIDs: kbIDs,
        TopK:  5,
    })
    if err != nil {
        return "", fmt.Errorf("RAG retrieve failed: %w", err)
    }

    // 拼接检索结果为上下文
    context := ""
    for i, item := range resp.Items {
        context += fmt.Sprintf("[%d] %s\n", i+1, item.Content)
    }
    return context, nil
}
```

---

## 6. 多租户隔离说明

### 6.1 API Key 绑定

- 每个 API Key 绑定到创建它的租户
- 使用 API Key 检索时，只能访问该租户的知识库
- 跨租户访问返回 404（不泄露资源存在性）

### 6.2 知识库权限

- 创建知识库时自动授予创建者 `admin` 权限
- 权限层级：`read` < `write` < `admin`
- 检索需要 `read` 权限

### 6.3 配额限制

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| max_kb_count | 5 | 最大知识库数量 |
| max_doc_count | 100 | 最大文档数量 |
| max_storage_mb | 1024 | 最大存储空间（MB） |
| max_api_calls_per_day | 10000 | 每日 API 调用次数 |

---

## 7. 常见问题

### Q: 检索返回 401

A: API Key 无效或已过期，请检查 API Key 是否正确。

### Q: 检索返回 404

A: 知识库不属于当前租户，或知识库 ID 不存在。

### Q: 检索返回 500

A: 知识库可能还没有上传文档，Milvus 集合不存在。请先上传文档。

### Q: 如何获取知识库 ID？

A: 调用 `GET /api/admin/kb/bases` 接口列出所有知识库。

### Q: 可以同时检索多个知识库吗？

A: 可以，在 `kb_ids` 数组中传入多个知识库 ID。

### Q: 检索结果为空怎么办？

A: 检查：
1. 知识库是否有文档
2. 文档是否处理完成
3. 查询内容是否与文档内容相关

---

## 8. 测试流程

### 8.1 完整测试流程

1. **注册 RAG Platform 账户**
   ```
   POST http://localhost:8899/v1/auth/register
   {"username":"agent","password":"TestPass12345","email":"agent@test.com"}
   ```

2. **登录获取 JWT**
   ```
   POST http://localhost:8899/v1/auth/login
   {"email":"agent@test.com","password":"TestPass12345"}
   ```

3. **创建 API Key**
   ```
   POST http://localhost:8899/v1/api-keys
   Authorization: Bearer <jwt_token>
   {"name":"agent-key"}
   ```

4. **创建知识库**
   ```
   POST http://localhost:8899/api/admin/kb/bases
   Authorization: Bearer <jwt_token>
   {"name":"面试知识库","description":"面试相关文档"}
   ```

5. **上传文档**
   ```
   POST http://localhost:8899/api/admin/kb/documents/upload
   Authorization: Bearer <jwt_token>
   kb_id=<kb_id>
   file=@interview-guide.txt
   ```

6. **等待文档处理完成**
   ```
   GET http://localhost:8899/api/admin/kb/jobs?kb_id=<kb_id>
   Authorization: Bearer <jwt_token>
   ```

7. **使用 API Key 检索**
   ```
   POST http://localhost:8899/v1/retrieve
   Authorization: Bearer rag_xxxxxxxxxxxx
   {"query":"什么是 JVM 调优？","kb_ids":[<kb_id>],"top_k":5}
   ```

### 8.2 集成测试

在面试吧项目中运行：

```go
func TestRAGIntegration(t *testing.T) {
    client := ragsdk.NewClient(ragsdk.ClientConfig{
        BaseURL: "http://localhost:8899",
        APIKey:  "rag_xxxxxxxxxxxx",
    })

    resp, err := client.Retrieve(context.Background(), ragsdk.RetrieveRequest{
        Query: "测试查询",
        KBIDs: []uint64{1},
        TopK:  3,
    })
    if err != nil {
        t.Fatalf("检索失败: %v", err)
    }

    if len(resp.Items) == 0 {
        t.Log("警告: 检索结果为空，请检查知识库是否有文档")
    }

    t.Logf("检索成功，返回 %d 条结果", len(resp.Items))
}
```

---

## 9. 部署配置

### 9.1 本地开发

```yaml
# 面试吧 config.yaml
rag_platform:
  enabled: true
  base_url: "http://localhost:8899"
  api_key: "rag_xxxxxxxxxxxx"
```

### 9.2 Docker 网络

如果面试吧和 RAG Platform 都在 Docker 中运行：

```yaml
# 面试吧 config.yaml
rag_platform:
  enabled: true
  base_url: "http://rag-server:8899"  # 使用 Docker 服务名
  api_key: "rag_xxxxxxxxxxxx"
```

### 9.3 生产环境

```yaml
# 面试吧 config.yaml
rag_platform:
  enabled: true
  base_url: "https://rag.your-domain.com"
  api_key: "rag_xxxxxxxxxxxx"
```
