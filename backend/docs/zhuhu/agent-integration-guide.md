# Agent 接入指南

## 1. 认证边界

- Admin UI 使用 JWT：
  - `POST /v1/auth/login`
  - `POST /v1/auth/refresh`
  - `GET /v1/auth/me`
- Agent / 服务端 SDK 使用 API Key：
  - `Authorization: Bearer rag_<key>`
- 终端用户不直接持有 API Key。
- `legacy app_id` 只保留兼容旧链路，不推荐新接入继续使用。

## 2. 推荐接入流程

1. 在 Admin UI 登录。
2. 进入 `API Keys` 页面创建 Key。
3. 复制一次性展示的完整 `rag_<key>`。
4. 把 Key 存到服务端环境变量 `RAG_API_KEY`。
5. 用该 Key 调用 `/v1/retrieve`。
6. 如需停用旧 Key，在 Admin UI 吊销或轮换。

## 3. cURL 示例

### 3.1 登录获取 JWT

```bash
curl -X POST "$RAG_BASE_URL/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "owner@example.com",
    "password": "your-password"
  }'
```

### 3.2 创建 API Key

```bash
curl -X POST "$RAG_BASE_URL/v1/api-keys" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $RAG_JWT" \
  -d '{
    "name": "production-agent",
    "app_id": "support-bot",
    "permissions": ["retrieve"],
    "expires_in": 0
  }'
```

### 3.3 使用 API Key 调 `/v1/retrieve`

```bash
curl -X POST "$RAG_BASE_URL/v1/retrieve" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $RAG_API_KEY" \
  -d '{
    "query": "知识库里关于 Go 并发的内容是什么？",
    "kb_ids": [1],
    "top_k": 5
  }'
```

### 3.4 吊销 API Key

```bash
curl -X DELETE "$RAG_BASE_URL/v1/api-keys/123" \
  -H "Authorization: Bearer $RAG_JWT"
```

## 4. Go SDK 示例

```go
client := ragsdk.NewClient(ragsdk.ClientConfig{
  BaseURL: "http://localhost:8081",
  APIKey:  "rag_xxxxxxxxxxxx",
})

resp, err := client.Retrieve(ctx, ragsdk.RetrieveRequest{
  Query: "知识库里关于 Go 并发的内容是什么？",
  KBIDs: []uint64{1},
  TopK:  5,
})
```

## 5. Python requests 示例

```python
import os
import requests

resp = requests.post(
    f"{os.environ['RAG_BASE_URL']}/v1/retrieve",
    headers={
        "Content-Type": "application/json",
        "Authorization": f"Bearer {os.environ['RAG_API_KEY']}",
    },
    json={
        "query": "知识库里关于 Go 并发的内容是什么？",
        "kb_ids": [1],
        "top_k": 5,
    },
    timeout=30,
)

print(resp.status_code)
print(resp.text)
```

## 6. 常见错误码

- `401 invalid_api_key`
- `401 api_key_revoked`
- `401 api_key_expired`
- `403 forbidden`
- `403 tenant_suspended`
- `404 not_found`
- `429 quota_exceeded`
- `429 rate_limited`
