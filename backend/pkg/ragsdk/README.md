# RAG Platform Go SDK

## 接入方式

### 方式一：API Key（推荐，程序化接入）

```go
client := ragsdk.NewClient(ragsdk.ClientConfig{
    BaseURL: "https://your-rag-platform.com",
    APIKey:  "rag_xxxxxxxxxxxx",  // 从 Admin UI 获取
})

resp, err := client.Retrieve(ctx, ragsdk.RetrieveRequest{
    Query: "什么是 JVM 调优？",
    KBIDs: []uint64{1, 2, 3},
})
```

### 方式二：JWT Token（管理端场景）

```go
client := ragsdk.NewClient(ragsdk.ClientConfig{
    BaseURL: "https://your-rag-platform.com",
    APIKey:  "eyJhbGciOiJIUzI1NiIs...",  // JWT Token
})
```

### 方式三：Legacy app_id（向后兼容，Phase 2 后弃用）

```go
resp, err := client.Retrieve(ctx, ragsdk.RetrieveRequest{
    Query:   "什么是 JVM 调优？",
    KBIDs:   []uint64{1, 2, 3},
    // AppID 通过请求体传递，平台内部使用白名单校验
})
```

## API 参考

### POST /v1/retrieve

**认证方式**：
- `Authorization: Bearer rag_xxx` — API Key（推荐）
- `Authorization: Bearer eyJhb...` — JWT Token
- 请求体 `app_id` 字段 — Legacy 兼容（Phase 2 后弃用）

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
  "items": [...],
  "strategy_version": "baseline"
}
```

## 错误码

| 错误码 | 说明 |
|--------|------|
| `INVALID_CREDENTIALS` | 认证失败 |
| `INVALID_API_KEY` | API Key 无效 |
| `API_KEY_REVOKED` | API Key 已吊销 |
| `API_KEY_EXPIRED` | API Key 已过期 |
| `PERMISSION_DENIED` | 权限不足 |
| `QUOTA_EXCEEDED` | 配额超限 |
