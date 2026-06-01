# RAG Platform Go SDK

## 安装

```go
import "interview-agents/pkg/ragsdk"
```

## 使用

```go
client := ragsdk.NewClient(ragsdk.ClientConfig{
    BaseURL: "http://localhost:8081",
    APIKey:  "your-api-key",
    AppID:   "your-app-id",
})

resp, err := client.Retrieve(ctx, ragsdk.RetrieveRequest{
    Query: "什么是 JVM 调优？",
    KBIDs: []uint64{1, 2, 3},
    TopK:  5,
})

for _, item := range resp.Items {
    fmt.Printf("Score: %.2f, Content: %s\n", item.Score, item.Content)
}
```

## API

### Retrieve

执行知识库检索。

**参数：**
- `Query` (required): 检索查询内容
- `KBIDs`: 知识库 ID 列表
- `TopK`: 返回结果数量
- `StrategyProfile`: 策略配置
- `MetadataFilter`: 元数据过滤

**返回：**
- `RequestID`: 请求 ID（用于审计和调试）
- `Items`: 检索结果列表
  - `Content`: 文本内容
  - `Score`: 相关性分数
  - `Citation`: 引用信息
  - `Source`: 来源信息
