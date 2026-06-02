# RAG Platform Go SDK

## 安装

```go
import "interview-agents/pkg/ragsdk"
```

## 快速开始

```go
package main

import (
  "context"
  "errors"
  "fmt"
  "log"

  "interview-agents/pkg/ragsdk"
)

func main() {
  client := ragsdk.NewClient(ragsdk.ClientConfig{
    BaseURL: "http://localhost:8081",
    APIKey:  "rag_xxxxxxxxxxxx",
  })

  resp, err := client.Retrieve(context.Background(), ragsdk.RetrieveRequest{
    Query: "知识库里关于 Go 并发的内容是什么？",
    KBIDs: []uint64{1},
    TopK:  5,
  })
  if err != nil {
    var apiErr *ragsdk.APIError
    if errors.As(err, &apiErr) {
      log.Fatalf("retrieve failed: status=%d body=%s", apiErr.StatusCode, apiErr.Body)
    }
    log.Fatal(err)
  }

  fmt.Println("request_id:", resp.RequestID)
  for _, item := range resp.Items {
    fmt.Printf("[%.2f] %s\n", item.Score, item.Content)
  }
}
```

## 认证方式

- SDK / Agent 使用 API Key：
  - `Authorization: Bearer rag_<key>`
- Admin UI 使用 JWT，不推荐混用。
- 终端用户不直接持有 API Key。

## 请求参数

```json
{
  "query": "知识库里关于 Go 并发的内容是什么？",
  "kb_ids": [1],
  "top_k": 5
}
```

## 错误处理

当服务端返回非 `200` 时，SDK 会返回 `*ragsdk.APIError`：

```go
var apiErr *ragsdk.APIError
if errors.As(err, &apiErr) {
  switch apiErr.StatusCode {
  case 401:
    // invalid_api_key / api_key_revoked / api_key_expired
  case 403:
    // forbidden / tenant_suspended
  case 429:
    // quota_exceeded / rate_limited
  }
}
```

## 常见状态码

- `401 invalid_api_key`
- `401 api_key_revoked`
- `401 api_key_expired`
- `403 forbidden`
- `403 tenant_suspended`
- `404 not_found`
- `429 quota_exceeded`
- `429 rate_limited`
