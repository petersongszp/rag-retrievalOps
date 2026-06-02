# RAG Platform Go SDK

## 安装

```go
import "your-module/pkg/ragsdk"
```

## 使用

```go
// 创建客户端（使用 API Key）
client := ragsdk.NewClient(ragsdk.ClientConfig{
    BaseURL: "https://your-rag-platform.com",
    APIKey:  "rag_xxxxxxxxxxxx",  // 从 Admin UI 获取
})

// 执行检索
resp, err := client.Retrieve(ctx, ragsdk.RetrieveRequest{
    Query: "什么是 JVM 调优？",
    KBIDs: []uint64{1, 2, 3},
    TopK:  5,
})

// 使用结果
for _, item := range resp.Items {
    fmt.Printf("[%.2f] %s\n", item.Score, item.Content)
}
```

## 认证方式

| 方式 | 说明 | 适用场景 |
|------|------|---------|
| API Key | `Authorization: Bearer rag_xxx` | Agent/SDK 接入（推荐） |
| JWT | `Authorization: Bearer eyJhb...` | Admin UI 管理端 |
| Legacy app_id | 请求体 `app_id` 字段 | 向后兼容（Phase 2 后弃用） |

## API 参考

### POST /v1/retrieve

**认证**：API Key 或 JWT

**请求**：
```json
{
  "query": "什么是 JVM 调优？",
  "kb_ids": [1, 2, 3],
  "top_k": 5
}
```

**响应**：
```json
{
  "request_id": "uuid",
  "items": [
    {
      "content": "检索到的文本内容",
      "score": 0.85,
      "citation": {...},
      "source": {...}
    }
  ]
}
```

## 错误码

| HTTP | 错误码 | 说明 |
|------|--------|------|
| 401 | INVALID_API_KEY | API Key 无效 |
| 401 | API_KEY_REVOKED | API Key 已吊销 |
| 401 | API_KEY_EXPIRED | API Key 已过期 |
| 403 | PERMISSION_DENIED | 权限不足 |
| 429 | QUOTA_EXCEEDED | 配额超限 |
