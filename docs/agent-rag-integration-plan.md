# Agent 接入 RAG 平台方案

## 一、当前状态

### 面试吧项目 (mianshiba-eino-overseas)
- Agent 代码已被删除（`internal/agents/` 不存在）
- 只保留了 RAG 相关代码（`internal/rag`、`internal/milvus`、`api/handler/kb` 等）
- 本质上已经变成了一个纯 RAG 服务

### RAG 平台 (rag-retrievalOps)
- 独立部署的 RAG 服务
- 对外暴露 API：`POST /v1/retrieve`
- 提供 Go SDK：`pkg/ragsdk/`
- 已注册的应用白名单：`interview-agent`、`mianshiba-web`、`mianshiba-admin`

---

## 二、Agent 接入方式

### 方式一：使用 Go SDK（推荐）

```go
import "your-rag-platform/pkg/ragsdk"

// 创建客户端
client := ragsdk.NewClient(ragsdk.ClientConfig{
    BaseURL: "http://your-rag-server:8081",  // RAG 平台地址
    APIKey:  "your-api-key",                  // API Key（可选）
    AppID:   "interview-agent",               // 应用 ID
    Timeout: 10 * time.Second,
})

// 执行检索
resp, err := client.Retrieve(ctx, ragsdk.RetrieveRequest{
    Query: "什么是 JVM 调优？",
    KBIDs: []uint64{1, 2, 3},  // 知识库 ID
    TopK:  5,
})

// 使用结果
for _, item := range resp.Items {
    fmt.Printf("Score: %.2f, Content: %s\n", item.Score, item.Content)
}
```

### 方式二：直接 HTTP 调用

```bash
curl -X POST http://your-rag-server:8081/v1/retrieve \
  -H "Content-Type: application/json" \
  -d '{
    "app_id": "interview-agent",
    "query": "什么是 JVM 调优？",
    "kb_ids": [1, 2, 3],
    "top_k": 5
  }'
```

### 方式三：封装为 Eino Tool

```go
package tools

import (
    "context"
    "encoding/json"
    "fmt"
    "your-rag-platform/pkg/ragsdk"
    
    "github.com/cloudwego/eino/schema"
    "github.com/cloudwego/eino/components/tool"
)

// RAGRetrieveTool 封装 RAG 检索为 Eino Tool
type RAGRetrieveTool struct {
    client     *ragsdk.Client
    defaultKBs []uint64
}

func NewRAGRetrieveTool(baseURL string, appID string, defaultKBs []uint64) *RAGRetrieveTool {
    return &RAGRetrieveTool{
        client: ragsdk.NewClient(ragsdk.ClientConfig{
            BaseURL: baseURL,
            AppID:   appID,
        }),
        defaultKBs: defaultKBs,
    }
}

func (t *RAGRetrieveTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
    return &schema.ToolInfo{
        Name: "rag_retrieve",
        Desc: "从知识库中检索相关文档",
        ParamsOneOf: &schema.ParamsInfo{
            OneOfParams: []*schema.ParameterInfo{
                {Name: "query", Type: "string", Desc: "检索查询", Required: true},
                {Name: "kb_ids", Type: "array", Desc: "知识库 ID（可选）", Required: false},
            },
        },
    }, nil
}

func (t *RAGRetrieveTool) InvokableRun(ctx context.Context, arguments string, opts ...tool.Option) (string, error) {
    var params struct {
        Query string   `json:"query"`
        KBIDs []uint64 `json:"kb_ids"`
    }
    if err := json.Unmarshal([]byte(arguments), &params); err != nil {
        return "", fmt.Errorf("parse arguments: %w", err)
    }
    
    kbIDs := params.KBIDs
    if len(kbIDs) == 0 {
        kbIDs = t.defaultKBs
    }
    
    resp, err := t.client.Retrieve(ctx, ragsdk.RetrieveRequest{
        Query: params.Query,
        KBIDs: kbIDs,
    })
    if err != nil {
        return "", err
    }
    
    result, _ := json.Marshal(resp)
    return string(result), nil
}
```

---

## 三、接入步骤

### 步骤 1：部署 RAG 平台

```bash
cd D:\Bear\rag-retrievalOps
docker compose -f docker-compose.rag.yml up -d
```

服务地址：
- RAG Server: `http://localhost:8081`
- Admin UI: `http://localhost:3003`
- Attu: `http://localhost:8001`

### 步骤 2：配置应用白名单

在 RAG 平台的配置中注册你的 Agent：

```yaml
# config.yaml
rag_platform:
  allowed_apps:
    - interview-agent
    - mianshiba-web
    - your-new-agent  # 添加新应用
```

或者在代码中硬编码（当前实现）：

```go
var allowedAppIDs = map[string]string{
    "interview-agent": "interview-agent",
    "mianshiba-web":   "mianshiba-web",
    "your-new-agent":  "your-new-agent",
}
```

### 步骤 3：Agent 侧集成

1. 复制 `pkg/ragsdk/` 到你的 Agent 项目
2. 创建 RAG 客户端
3. 在 Agent 的 Tool 中调用 `client.Retrieve()`

### 步骤 4：配置知识库权限

在 RAG 平台中为你的应用授权可访问的知识库：

```bash
# 通过 Admin API 授权
curl -X POST http://localhost:8081/api/admin/kb/apps \
  -H "Content-Type: application/json" \
  -d '{
    "app_id": "interview-agent",
    "kb_ids": [1, 2, 3, 4]
  }'
```

---

## 四、API 参考

### POST /v1/retrieve

**请求：**
```json
{
  "app_id": "interview-agent",      // 必填：应用 ID
  "query": "什么是 JVM 调优？",     // 必填：检索查询
  "kb_ids": [1, 2, 3],              // 可选：知识库 ID
  "top_k": 5,                       // 可选：返回数量
  "strategy_profile": "default",    // 可选：策略配置
  "metadata_filter": {}             // 可选：元数据过滤
}
```

**响应：**
```json
{
  "request_id": "uuid",
  "items": [
    {
      "content": "检索到的文本内容",
      "score": 0.85,
      "citation": {
        "kb_id": 1,
        "document_id": 10,
        "chunk_id": "doc-10-child-003",
        "file_name": "jvm-tuning.md",
        "chunk_index": 3
      },
      "source": {
        "route": "hybrid",
        "collection": "kb_1_docs",
        "retriever_version": "hybrid-v1"
      }
    }
  ],
  "strategy_version": "baseline",
  "request_cost": {
    "estimated_cost": 0.001
  }
}
```

---

## 五、安全建议

1. **API Key 认证**：生产环境应启用 API Key 验证
2. **应用隔离**：每个应用只能访问授权的知识库
3. **网络隔离**：RAG 平台应部署在内网，不暴露公网
4. **日志审计**：所有检索请求都会记录 `app_id` 和 `request_id`

---

## 六、后续演进

1. **L2 阶段**：Agent 从 Milvus 直连改为 HTTP 调用（已完成）
2. **API Key**：从白名单模式升级为 API Key 认证
3. **多租户**：支持 `tenant_id` 隔离
4. **SDK 扩展**：增加 `UploadDocument`、`GetJob` 等方法
