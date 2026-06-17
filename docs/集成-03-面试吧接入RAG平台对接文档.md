# 面试吧接入 RAG 平台对接文档

更新时间：2026-06-03

## 1. 文档目的

本文基于以下两个项目的当前代码状态整理：

- `D:\Bear\rag-retrievalOps`：RAG 平台
- `D:\Bear\mianshiba-eino-overseas`：面试吧项目

目标是说明：

- 如何获取 RAG 平台 API Key
- 如何让面试吧项目改为使用 RAG 平台
- 如何调用 `/v1/retrieve`
- 如何上传文档到知识库
- 当前实现和计划文档之间有哪些差异

## 2. 扫描结论

### 2.1 RAG 平台结论

- 当前 Go SDK 入口在 `backend/pkg/ragsdk/client.go`，核心能力只有 `Retrieve`。
- SDK 当前真实用法是：`BaseURL + APIKey + Timeout`，直接 `POST /v1/retrieve`。
- `/v1/retrieve` 当前认证优先级是：`API Key > JWT > legacy app_id`。
- API Key 模式下，服务端会使用 API Key 记录上的 `app_id`，不依赖请求体里的 `app_id`。
- `/v1/retrieve` 实际最终返回是复用 `kb.Retrieve` 的结果，不是“计划文档里定义但未真正返回”的完整契约。
- 知识库创建后，会自动给当前租户写入一条 `rag_tenant_kb_permission` 的 `admin` 权限记录。
- 文档上传接口当前支持 `pdf/txt/md/markdown`，单文件最大 `20MB`。

### 2.2 面试吧项目结论

- `backend/internal/agents/` 目录当前不存在。
- `backend/internal/milvus/` 仍保留完整本地检索链路：Milvus 连接、embedding、splitter、indexer、retriever、hybrid retriever。
- `backend/internal/config/config.go` 里已经有 `rag_platform` 配置结构。
- `backend/config.rag.example.yaml` 里已经有 `rag_platform` 示例配置。
- 但我没有在面试吧项目里找到实际消费 `rag_platform` 配置并发起远程 `/v1/retrieve` 调用的代码。

结论：面试吧项目目前“有配置位、无接线”，仍以本地 Milvus 检索为主。

## 3. 当前推荐接入方式

推荐路线：

1. 面试吧不要继续直连本地 `internal/milvus` 做业务检索。
2. 统一改为通过 RAG 平台的 `/v1/retrieve` 或其 Go SDK 调用。
3. 管理面操作仍走 RAG 平台自己的后台接口完成知识库创建、文档上传、任务查看。

不推荐继续依赖：

- 面试吧本地 `MilvusManager`
- 面试吧本地 `RetrieverService`
- legacy `app_id` 白名单调用

## 4. 如何获取 API Key

当前最稳妥的方式是：先用 JWT 登录后台账号，再创建 API Key。

### 4.1 注册租户和账号

如果是第一次使用：

```bash
curl -X POST "$RAG_BASE_URL/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "owner@example.com",
    "password": "YourStrongPassword123!",
    "name": "RAG Owner",
    "tenant_name": "Interview Bar"
  }'
```

### 4.2 登录获取 JWT

```bash
curl -X POST "$RAG_BASE_URL/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "owner@example.com",
    "password": "YourStrongPassword123!"
  }'
```

返回示例：

```json
{
  "code": 200,
  "message": "Success",
  "data": {
    "access_token": "xxx",
    "refresh_token": "yyy",
    "expires_in": 7200,
    "user_id": 1,
    "role": "owner",
    "tenant_id": 1
  }
}
```

### 4.3 创建 API Key

```bash
curl -X POST "$RAG_BASE_URL/v1/api-keys" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $RAG_JWT" \
  -d '{
    "name": "mianshiba-prod",
    "app_id": "interview-agent",
    "permissions": ["retrieve"],
    "expires_in": 0
  }'
```

返回示例：

```json
{
  "code": 200,
  "message": "Success",
  "data": {
    "id": 12,
    "name": "mianshiba-prod",
    "app_id": "interview-agent",
    "key": "rag_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
    "key_prefix": "rag_xxxxxxxx...",
    "permissions": ["retrieve"],
    "created_at": "2026-06-03T10:00:00Z"
  }
}
```

注意：

- 明文 `key` 只会在创建或轮换时返回一次。
- 生产环境应把它保存到面试吧服务端环境变量，例如 `RAG_API_KEY`。
- 当前实现里 `permissions` 会被存储，但 `/v1/retrieve` 还没有对 API Key 的 `permissions` 做强制校验，真正生效的访问边界主要还是“租户 + KB 权限”。

## 5. 如何配置面试吧项目使用 RAG 平台

### 5.1 配置项

面试吧项目已经有如下配置结构：

```yaml
rag_platform:
  enabled: true
  base_url: "http://your-rag-host:8899"
  api_key: "${RAG_API_KEY}"
  app_id: "interview-agent"
  default_kb_ids: [1]
```

建议把这段加入面试吧实际运行配置，例如 `backend/config.yaml`。

说明：

- `enabled`：是否启用远程 RAG 平台
- `base_url`：RAG 平台地址
- `api_key`：通过 `/v1/api-keys` 申请到的 `rag_` 开头密钥
- `app_id`：建议与创建 API Key 时的 `app_id` 保持一致
- `default_kb_ids`：默认检索知识库 ID

### 5.2 接入建议

面试吧当前没有 `backend/internal/agents/` 目录，所以这里给出“通用业务层接法”：

1. 新增一个远程 RAG 客户端封装。
2. 读取 `config.Global.RAGPlatform`。
3. 当 `rag_platform.enabled=true` 时，业务侧统一走远程 `/v1/retrieve`。
4. 仅在迁移过渡期保留本地 Milvus fallback，最终要删除。

### 5.3 Go 封装示例

```go
package ragplatform

import (
	"context"
	"fmt"
	"strings"
	"time"

	"interview-agents/internal/config"
	"interview-agents/pkg/ragsdk"
)

type Service struct {
	client       *ragsdk.Client
	defaultKBIDs []uint64
	enabled      bool
}

func NewService(cfg *config.Config) (*Service, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if !cfg.RAGPlatform.Enabled {
		return &Service{enabled: false}, nil
	}
	if strings.TrimSpace(cfg.RAGPlatform.BaseURL) == "" {
		return nil, fmt.Errorf("rag_platform.base_url is required")
	}
	if strings.TrimSpace(cfg.RAGPlatform.APIKey) == "" {
		return nil, fmt.Errorf("rag_platform.api_key is required")
	}

	return &Service{
		enabled: true,
		client: ragsdk.NewClient(ragsdk.ClientConfig{
			BaseURL: cfg.RAGPlatform.BaseURL,
			APIKey:  cfg.RAGPlatform.APIKey,
			Timeout: 10 * time.Second,
		}),
		defaultKBIDs: cfg.RAGPlatform.DefaultKBIDs,
	}, nil
}

func (s *Service) Retrieve(ctx context.Context, query string, kbIDs []uint64, topK int) (*ragsdk.RetrieveResponse, error) {
	if s == nil || !s.enabled {
		return nil, fmt.Errorf("rag platform is disabled")
	}
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is required")
	}
	if len(kbIDs) == 0 {
		kbIDs = s.defaultKBIDs
	}
	return s.client.Retrieve(ctx, ragsdk.RetrieveRequest{
		Query: query,
		KBIDs: kbIDs,
		TopK:  topK,
	})
}
```

## 6. 如何调用 `/v1/retrieve`

### 6.1 推荐认证方式

推荐只使用 API Key：

```http
Authorization: Bearer rag_xxx...
```

不推荐面试吧业务调用使用：

- JWT
- legacy `app_id`

### 6.2 请求参数

当前可安全依赖的请求参数：

```json
{
  "query": "什么是 JVM 调优？",
  "kb_ids": [1, 2],
  "top_k": 5
}
```

补充说明：

- `kb_id` 和 `kb_ids` 都支持，推荐统一使用 `kb_ids`
- `top_k <= 0` 时默认按 `5`
- `top_k > 20` 时会被截断到 `20`

### 6.3 cURL 示例

```bash
curl -X POST "$RAG_BASE_URL/v1/retrieve" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $RAG_API_KEY" \
  -d '{
    "query": "什么是 JVM 调优？",
    "kb_ids": [1],
    "top_k": 5
  }'
```

### 6.4 Go SDK 示例

```go
client := ragsdk.NewClient(ragsdk.ClientConfig{
	BaseURL: "http://localhost:8899",
	APIKey:  os.Getenv("RAG_API_KEY"),
	Timeout: 10 * time.Second,
})

resp, err := client.Retrieve(ctx, ragsdk.RetrieveRequest{
	Query: "什么是 JVM 调优？",
	KBIDs: []uint64{1},
	TopK:  5,
})
if err != nil {
	return err
}

for _, item := range resp.Items {
	fmt.Printf("[%.4f] %s\n", item.Score, item.Content)
}
```

### 6.5 Python 示例

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
        "query": "什么是 JVM 调优？",
        "kb_ids": [1],
        "top_k": 5,
    },
    timeout=30,
)

print(resp.status_code)
print(resp.json())
```

### 6.6 当前真实返回结构

注意：HTTP 200 之外，平台还会用统一响应包一层。

成功返回示例：

```json
{
  "code": 200,
  "message": "Success",
  "data": {
    "request_id": "8c26d3a0-7b40-4fcf-a4aa-0f3c3e2f8b1e",
    "items": [
      {
        "content": "JVM 调优通常从堆大小、GC 策略和对象分配行为入手。",
        "score": 0.8731,
        "citation": {
          "kb_id": 1,
          "document_id": 10,
          "chunk_id": "doc-10-child-003",
          "file_name": "jvm-tuning.md",
          "chunk_index": 3,
          "snippet_offset": 0
        },
        "source": {
          "route": "hybrid",
          "collection": "kb_1_docs",
          "retriever_version": "hybrid-v1",
          "parent_id": "",
          "child_id": "doc-10-child-003",
          "section_title": "GC 调优",
          "hierarchy_path": "JVM > GC 调优",
          "parent_fill_strategy": "",
          "parent_fill_tokens": 0,
          "citation_supported": true,
          "citation_support_score": 0.92,
          "citation_check_version": "phase3-citation-v1",
          "low_support_citation": false
        }
      }
    ],
    "evidence_gate_result": "allowed",
    "citation_check": {
      "supported": true,
      "support_score": 0.92,
      "unsupported_claim_count": 0,
      "version": "phase3-citation-v1",
      "latency_ms": 8
    }
  }
}
```

失败返回示例：

```json
{
  "code": 401,
  "message": "API key is expired",
  "data": null
}
```

## 7. 如何上传文档到知识库

当前推荐使用后台管理接口，而不是让面试吧业务服务自己上传。

### 7.1 创建知识库

```bash
curl -X POST "$RAG_BASE_URL/api/admin/kb/bases" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $RAG_JWT" \
  -d '{
    "name": "Interview Java KB",
    "description": "Java interview documents"
  }'
```

返回中的 `data.id` 就是 `kb_id`。

### 7.2 上传文档

```bash
curl -X POST "$RAG_BASE_URL/api/admin/kb/documents/upload" \
  -H "Authorization: Bearer $RAG_JWT" \
  -F "kb_id=1" \
  -F "file=@D:/docs/jvm-tuning.md"
```

返回示例：

```json
{
  "code": 200,
  "message": "Success",
  "data": {
    "document_id": 101,
    "job_id": 202,
    "status": "pending"
  }
}
```

### 7.3 查看入库任务状态

```bash
curl -X GET "$RAG_BASE_URL/api/admin/kb/jobs/202" \
  -H "Authorization: Bearer $RAG_JWT"
```

### 7.4 上传限制

- 支持格式：`pdf`、`txt`、`md`、`markdown`
- 最大大小：`20MB`
- 重复文件会按内容 hash 复用已有文档记录

## 8. 知识库授权说明

### 8.1 当前真实生效的授权方式

当前代码里，`/v1/retrieve` 的授权重点不是“app_id -> kb_ids”静态映射，而是：

1. API Key 绑定一个 `tenant_id`
2. 检索请求里的 `kb_id/kb_ids` 必须属于这个租户可访问的知识库
3. 租户对对应 KB 必须至少有 `read` 权限

### 8.2 当前没有发现可直接使用的公开“app 绑定 KB”接口

我在当前代码中没有找到稳定可用的公开接口用于：

- 给某个 `app_id` 单独授权一组 `kb_ids`

所以当前可落地做法是：

- 用同一租户下创建的 API Key 调同一租户下的 KB
- 依赖 `rag_tenant_kb_permission` 控制租户对 KB 的读写权限

### 8.3 已创建 KB 的权限处理

新建知识库时，系统会自动给当前租户写入 `admin` 权限。

如果是历史数据或跨环境迁移，发现 API Key 调用时返回 `403 Permission denied`，需要检查表：

- `rag_tenant_kb_permission`

MySQL 示例：

```sql
INSERT INTO rag_tenant_kb_permission (tenant_id, kb_id, permission, created_at)
VALUES (1, 12, 'read', NOW())
ON DUPLICATE KEY UPDATE permission = 'read';
```

## 9. 面试吧项目的改造建议

由于当前扫描到的面试吧项目已经没有 `backend/internal/agents/`，这里给出“业务接入层”的改造建议：

### 9.1 第一步：新增远程 RAG service

- 读取 `config.Global.RAGPlatform`
- 封装 `ragsdk.Client`
- 提供统一 `Retrieve(ctx, query, kbIDs, topK)` 方法

### 9.2 第二步：替换本地检索调用

需要重点替换的方向：

- 任何直接引用 `internal/milvus`
- 任何直接使用 `MilvusManager`
- 任何直接调用 `RetrieverService` / `HybridRetriever`

### 9.3 第三步：保留短期 fallback，长期删除

过渡期可以保留：

- `rag_platform.enabled=true` 走远程 RAG
- `rag_platform.enabled=false` 走本地 Milvus

但长期目标应是：

- 面试吧业务只负责发起检索请求
- 检索、分块、embedding、向量库、审计全部由 RAG 平台负责

## 10. 已知差异和注意事项

以下内容要特别注意，不要按旧计划文档直接接：

### 10.1 SDK 与计划文档不一致

`docs/集成-01-Agent接入RAG平台方案.md` 里描述了 SDK 传 `AppID` 的方案，但当前 `rag-retrievalOps/backend/pkg/ragsdk/client.go` 的真实 SDK 配置只有：

- `BaseURL`
- `APIKey`
- `Timeout`

建议以代码为准，不以旧方案文档为准。

### 10.2 `/v1/retrieve` 请求字段与实际执行链路有差异

当前公开请求结构里有：

- `strategy_profile`
- `metadata_filter`

但真正执行时是直接复用 `kb.Retrieve`，而 `kb.Retrieve` 当前只实际消费：

- `kb_id`
- `kb_ids`
- `query`
- `top_k`

因此：

- `strategy_profile` 目前不要作为强依赖
- `metadata_filter` 目前不要作为强依赖

### 10.3 `/v1/retrieve` 响应字段与计划契约有差异

公开 handler 里定义了：

- `strategy_version`
- `request_cost`

但当前运行时最终返回并不稳定提供这两个字段，因为实际响应来自 `kb.Retrieve`。

集成时请只依赖当前稳定字段：

- `code`
- `message`
- `data.request_id`
- `data.items`
- `data.evidence_gate_result`
- `data.citation_check`
- `data.refusal`

## 11. 常见问题

### Q1：为什么我拿到 JWT 后直接调 `/v1/retrieve` 还是不建议上线？

因为面试吧业务接 RAG 平台时，更适合服务到服务认证。当前推荐上线方式是 API Key，不是用户 JWT。

### Q2：为什么返回 `401`？

常见原因：

- `Authorization` 没带
- 传的不是 `rag_` 开头的 API Key
- API Key 已过期
- API Key 已被吊销

### Q3：为什么返回 `403 Permission denied`？

说明你的 API Key 所属租户对目标 KB 没有 `read` 权限，或 KB 不属于该租户访问范围。

### Q4：为什么返回 200，但 `items` 为空？

常见原因：

- `kb_ids` 不对
- 知识库里没有命中文档
- 命中了，但被后处理过滤掉
- 命中了，但被证据门控拦掉，此时要看 `data.refusal`

### Q5：`app_id` 还要不要传？

API Key 模式下，不应该把它当作真正的鉴权来源。当前服务端会以 API Key 记录上的 `app_id` 为准。

### Q6：面试吧项目里已经有 `rag_platform` 配置，为什么还不能直接用？

因为当前只看到了配置结构，没有看到真正消费这套配置并发起远程 `/v1/retrieve` 的业务代码。

## 12. 最小接入清单

如果只追求尽快打通，最小步骤如下：

1. 在 RAG 平台注册租户账号并登录。
2. 创建 API Key，`app_id` 用 `interview-agent`。
3. 创建知识库并上传文档。
4. 在面试吧项目配置 `rag_platform.base_url`、`rag_platform.api_key`、`rag_platform.default_kb_ids`。
5. 新增一个远程 RAG service 封装 SDK。
6. 用远程 `Retrieve` 替换业务侧本地 Milvus 检索调用。
7. 只依赖当前稳定响应字段，不依赖 `strategy_profile`、`metadata_filter`、`strategy_version`、`request_cost`。
