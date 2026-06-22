# TASK-002: Cross-Encoder 重排序详细实现教程

> 🎯 **任务 ID**: TASK-002
>
> **功能名称**: Cross-Encoder 重排序
>
> **预估工时**: 12 小时
>
> **难度**: ⭐⭐⭐ (进阶级)
>
> **技术栈**: Go、Python、HuggingFace、gRPC
>
> **推荐人数**: 2 人

---

## 📋 目录

- [一、需求是什么？](#一需求是什么)
- [二、为什么要做这个？](#二为什么要做这个)
- [三、技术原理](#三技术原理)
- [四、实现步骤](#四实现步骤)
- [五、如何验证？](#五如何验证)
- [六、代码提交流程](#六代码提交流程)

---

## 一、需求是什么？

### 1.1 问题背景

目前的重排序是 **Jaccard 相似度**（词频匹配），对于语义理解不够好：

| 查询 | 候选文本 | Jaccard 相似度 | 真实相关性 |
|------|---------|---------------|-----------|
| 什么是 RAG？ | 检索增强生成技术 | 0.2 | ⭐⭐⭐⭐⭐ |
| 什么是 RAG？ | RAG 是一种方法 | 0.8 | ⭐⭐⭐ |

**问题**：Jaccard 只看词重叠，不看语义！

### 1.2 解决方案

引入 **Cross-Encoder**（交叉编码器）：
1. 先召回 Top 100（快）
2. 用 Cross-Encoder 重排序 Top 20（准）
3. 返回 Top 10

### 1.3 功能需求

| 功能点 | 说明 |
|--------|------|
| Cross-Encoder 服务 | Python + HuggingFace，提供 gRPC 接口 |
| Go 客户端 | 调用 Cross-Encoder 服务 |
| 可配置 | 支持开关、TopN、超时配置 |
| 监控指标 | rerank_latency、rerank_count |
| 降级策略 | Cross-Encoder 挂了自动回退到 Jaccard |

---

## 二、为什么要做这个？

### 2.1 业务价值

| 指标 | 预期提升 |
|------|---------|
| 检索相关性 | Recall@10 提升 15-20% |
| NDCG@10 | 从 0.65 提升到 0.80 |

### 2.2 技术价值

- 学习 **ML 模型服务化**
- 掌握 **gRPC 通信**
- 理解 **Bi-Encoder vs Cross-Encoder**
- 实践 **降级策略**

---

## 三、技术原理

### 3.1 Bi-Encoder vs Cross-Encoder

```
Bi-Encoder（当前用的，快）：
  Query → [Encoder] → Vector
  Chunk → [Encoder] → Vector
  相似度 = cos(Vector1, Vector2)
  ✅ 快    ❌ 不太准

Cross-Encoder（我们要加的，准）：
  [Query + Chunk] → [Encoder] → Score
  ✅ 很准  ❌ 慢（所以只对 TopN 重排序）
```

### 3.2 系统架构

```
┌─────────────┐
│   Go 后端    │
└──────┬──────┘
       │
       ▼
┌─────────────────┐
│  召回 Top 100   │ ← (1) 快速召回
└──────┬──────────┘
       │
       ▼
┌─────────────────┐
│ Cross-Encoder   │ ← (2) 只对 Top 20 重排序
│  (Python gRPC)  │
└──────┬──────────┘
       │
       ▼
┌─────────────────┐
│  返回 Top 10    │
└─────────────────┘
```

### 3.3 推荐模型

| 模型 | 参数量 | 速度 | 效果 |
|------|--------|------|------|
| BAAI/bge-reranker-base | 110M | 快 | 好 |
| BAAI/bge-reranker-large | 330M | 中 | 很好 |
| cross-encoder/ms-marco-MiniLM-L-6-v2 | 22M | 很快 | 一般 |

**建议**: 先用 `bge-reranker-base`，平衡速度和效果

---

## 四、实现步骤

### Step 0: 现有代码位置

```
backend/
├── internal/
│   └── milvus/
│       └── retrieval/
│           ├── rerank.go          ← 现有 Jaccard 重排序
│           └── reranker.go        ← 我们在这里扩展
└── services/
    └── cross-encoder/             ← (新建) Python 服务
        ├── server.py
        └── proto/
            └── rerank.proto
```

### Step 1: 定义 gRPC 协议

**文件**: `backend/services/cross-encoder/proto/rerank.proto`

```protobuf
syntax = "proto3";

package rerank;

option go_package = "./proto";

service Reranker {
  rpc Rerank(RerankRequest) returns (RerankResponse) {}
}

message RerankRequest {
  string query = 1;
  repeated Document documents = 2;
}

message Document {
  string id = 1;
  string text = 2;
}

message RerankResponse {
  repeated ScoredDocument results = 1;
}

message ScoredDocument {
  string id = 1;
  string text = 2;
  float score = 3;
}
```

### Step 2: 实现 Python Cross-Encoder 服务

**文件**: `backend/services/cross-encoder/server.py`

```python
import grpc
from concurrent import futures
import time
import os
from sentence_transformers import CrossEncoder

import rerank_pb2
import rerank_pb2_grpc

class RerankerService(rerank_pb2_grpc.RerankerServicer):
    def __init__(self):
        model_name = os.getenv("MODEL_NAME", "BAAI/bge-reranker-base")
        print(f"Loading model: {model_name}")
        self.model = CrossEncoder(model_name)
        print("Model loaded!")

    def Rerank(self, request, context):
        query = request.query
        documents = [doc.text for doc in request.documents]

        if not documents:
            return rerank_pb2.RerankResponse(results=[])

        # 构建 (query, document) pair
        pairs = [[query, doc] for doc in documents]

        # 预测分数
        scores = self.model.predict(pairs)

        # 构建响应
        results = []
        for i, doc in enumerate(request.documents):
            results.append(rerank_pb2.ScoredDocument(
                id=doc.id,
                text=doc.text,
                score=float(scores[i])
            ))

        return rerank_pb2.RerankResponse(results=results)

def serve():
    port = os.getenv("PORT", "50051")
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    rerank_pb2_grpc.add_RerankerServicer_to_server(RerankerService(), server)
    server.add_insecure_port(f"[::]:{port}")
    server.start()
    print(f"Server started on port {port}")
    server.wait_for_termination()

if __name__ == "__main__":
    serve()
```

**文件**: `backend/services/cross-encoder/requirements.txt`

```
sentence-transformers>=2.2.0
grpcio>=1.50.0
grpcio-tools>=1.50.0
protobuf>=4.20.0
torch>=2.0.0
```

**文件**: `backend/services/cross-encoder/Dockerfile`

```dockerfile
FROM python:3.10-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

# 预下载模型（避免启动时下载）
RUN python -c "from sentence_transformers import CrossEncoder; CrossEncoder('BAAI/bge-reranker-base')"

EXPOSE 50051

CMD ["python", "server.py"]
```

### Step 3: 生成 Go gRPC 代码

```bash
cd backend/services/cross-encoder

# 安装 protoc 编译器
# macOS: brew install protobuf
# Linux: apt install protobuf-compiler

# 安装 Go 插件
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 生成代码
protoc --go_out=. --go-grpc_out=. proto/rerank.proto
```

### Step 4: 实现 Go 客户端

**文件**: `backend/internal/milvus/retrieval/cross_encoder_reranker.go`

```go
package retrieval

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "your-project/backend/services/cross-encoder/proto"
)

// CrossEncoderReranker Cross-Encoder 重排器
type CrossEncoderReranker struct {
	client pb.RerankerClient
	topN   int
	timeout time.Duration
	enabled bool
	// 降级用的 Jaccard 重排器
	fallback *JaccardReranker
}

// NewCrossEncoderReranker 创建 Cross-Encoder 重排器
func NewCrossEncoderReranker(addr string, topN int, timeout time.Duration, enabled bool) (*CrossEncoderReranker, error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &CrossEncoderReranker{
		client:   pb.NewRerankerClient(conn),
		topN:     topN,
		timeout:  timeout,
		enabled:  enabled,
		fallback: NewJaccardReranker(),
	}, nil
}

// Rerank 重排序
func (r *CrossEncoderReranker) Rerank(ctx context.Context, query string, chunks []*Chunk) ([]*Chunk, error) {
	if !r.enabled {
		return r.fallback.Rerank(ctx, query, chunks)
	}

	if len(chunks) == 0 {
		return chunks, nil
	}

	// 只重排前 TopN
	candidates := chunks
	if len(candidates) > r.topN {
		candidates = candidates[:r.topN]
	}

	// 构建请求
	docs := make([]*pb.Document, len(candidates))
	for i, chunk := range candidates {
		docs[i] = &pb.Document{
			Id:   chunk.ID,
			Text: chunk.Text,
		}
	}

	reqCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// 调用 Cross-Encoder 服务
	resp, err := r.client.Rerank(reqCtx, &pb.RerankRequest{
		Query:     query,
		Documents: docs,
	})

	if err != nil {
		// 降级到 Jaccard
		fmt.Printf("Cross-Encoder error: %v, fallback to Jaccard\n", err)
		metrics.RerankFallback.Inc()
		return r.fallback.Rerank(ctx, query, chunks)
	}

	// 重新排序
	scoreMap := make(map[string]float64)
	for _, res := range resp.Results {
		scoreMap[res.Id] = float64(res.Score)
	}

	// 按照 Cross-Encoder 分数排序
	sorted := make([]*Chunk, len(candidates))
	copy(sorted, candidates)

	for i := range sorted {
		for j := i + 1; j < len(sorted); j++ {
			if scoreMap[sorted[i].ID] < scoreMap[sorted[j].ID] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	metrics.RerankSuccess.Inc()
	return sorted, nil
}
```

### Step 5: 集成到检索流程

**修改文件**: `backend/internal/milvus/retrieval/retrieval.go`

在 `doRetrieve` 函数中，替换重排序逻辑：

```go
func (s *Service) doRetrieve(ctx context.Context, kbID string, query string, queryEmbedding []float32) ([]*Chunk, error) {
	// ... 前面的召回逻辑 ...

	// 重排序（现在用 Cross-Encoder）
	if s.crossEncoderReranker != nil {
		chunks, err = s.crossEncoderReranker.Rerank(ctx, query, chunks)
	} else {
		// 降级到 Jaccard
		chunks = s.jaccardReranker.Rerank(ctx, query, chunks)
	}

	// ... 后面的逻辑 ...
}
```

### Step 6: 添加监控指标

**修改文件**: `backend/internal/observability/metrics/rag_metrics.go`

```go
var (
	// RerankSuccess Cross-Encoder 成功次数
	RerankSuccess = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "rag_rerank_success_total",
			Help: "Total number of successful cross-encoder reranks",
		},
	)

	// RerankFallback Cross-Encoder 降级次数
	RerankFallback = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "rag_rerank_fallback_total",
			Help: "Total number of cross-encoder fallbacks",
		},
	)

	// RerankLatency Cross-Encoder 延迟
	RerankLatency = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "rag_rerank_latency_seconds",
			Help:    "Cross-encoder latency distribution",
			Buckets: prometheus.DefBuckets,
		},
	)
)
```

---

## 五、如何验证？

### 5.1 启动 Cross-Encoder 服务

```bash
cd backend/services/cross-encoder

# 方式一：Docker（推荐）
docker build -t cross-encoder .
docker run -p 50051:50051 cross-encoder

# 方式二：本地运行
pip install -r requirements.txt
python server.py
```

### 5.2 单元测试

**文件**: `backend/internal/milvus/retrieval/cross_encoder_reranker_test.go`

```go
package retrieval

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCrossEncoderReranker(t *testing.T) {
	// 注意：这个测试需要 Cross-Encoder 服务运行
	// 如果只是测试代码逻辑，可以 mock client

	// 创建重排器（测试环境可以 disable）
	reranker, err := NewCrossEncoderReranker(
		"localhost:50051",
		20,
		5*time.Second,
		false, // disable，测试降级逻辑
	)
	assert.NoError(t, err)

	// 测试数据
	query := "什么是 RAG？"
	chunks := []*Chunk{
		{ID: "1", Text: "这是一段无关的文字"},
		{ID: "2", Text: "RAG 是检索增强生成技术"},
	}

	// 测试（因为 disabled，会走 Jaccard）
	result, err := reranker.Rerank(context.Background(), query, chunks)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
}
```

### 5.3 手动测试

#### 步骤 1: 启动所有服务

```bash
# 终端 1: Cross-Encoder
cd backend/services/cross-encoder
docker run -p 50051:50051 cross-encoder

# 终端 2: Go 后端
cd backend
make run
```

#### 步骤 2: 测试检索

```bash
curl -X POST http://localhost:8080/api/v1/retrieve \
  -H "Content-Type: application/json" \
  -d '{
    "kb_id": "test-kb",
    "query": "什么是 RAG？"
  }'
```

**预期**: 返回的结果应该比之前更相关！

#### 步骤 3: 测试降级

```bash
# 停止 Cross-Encoder 服务
docker stop <container-id>

# 再次查询
curl -X POST http://localhost:8080/api/v1/retrieve ...
```

**预期**: 应该自动降级到 Jaccard，`rag_rerank_fallback_total` 指标 +1

### 5.4 评估验证

用项目自带的评估框架测试：

```bash
cd backend
go run cmd/evaluate/main.go --kb-id test-kb --dataset testdata.json
```

**验收标准**:

| 指标 | 要求 |
|------|------|
| NDCG@10 | 提升 >= 10% |
| Recall@5 | 提升 >= 8% |
| 降级成功率 | 100% |

---

## 六、代码提交流程

### 6.1 提交代码

```bash
git checkout -b feature/TASK-002-cross-encoder

git add backend/services/cross-encoder/
git add backend/internal/milvus/retrieval/cross_encoder_reranker.go
git add backend/internal/observability/metrics/rag_metrics.go

git commit -m "feat: TASK-002 实现 Cross-Encoder 重排序

- 实现 Python Cross-Encoder gRPC 服务
- 实现 Go 客户端集成
- 添加降级策略
- 添加监控指标"

git push origin feature/TASK-002-cross-encoder
```

### 6.2 创建 PR

**标题**: `feat: TASK-002 实现 Cross-Encoder 重排序`

**内容**:

```markdown
## 任务说明
- 任务 ID：TASK-002
- 功能：Cross-Encoder 重排序
- 实现人：[你的名字]

## 实现方案
- Python + HuggingFace + gRPC 服务
- 支持 bge-reranker-base 模型
- 自动降级到 Jaccard
- Prometheus 监控

## 验证结果
- [x] 单元测试通过
- [x] 手动测试通过
- [x] 降级测试通过
- [x] 评估：NDCG@10 从 0.65 提升到 0.78

## 相关文档
- 教程：./docs/TASK-002-cross-encoder-rerank.md
```

---

## 🎉 恭喜！

完成这个任务后，你将：
- ✅ 理解 Cross-Encoder 原理
- ✅ 掌握 gRPC 服务开发
- ✅ 学会 ML 模型服务化
- ✅ 实践降级策略

**下一步**: 去做 TASK-003（OpenTelemetry 追踪）！
