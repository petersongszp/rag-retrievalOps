# TASK-003: OpenTelemetry 全链路追踪详细实现教程

> 🎯 **任务 ID**: TASK-003
>
> **功能名称**: OpenTelemetry 全链路追踪
>
> **预估工时**: 10 小时
>
> **难度**: ⭐⭐⭐ (进阶级)
>
> **技术栈**: Go、OpenTelemetry、Jaeger、gRPC
>
> **推荐人数**: 1-2 人

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

现在系统出问题时，很难定位：
- "这个查询怎么这么慢？"
- "是 Redis 慢还是 Milvus 慢？"
- "这个错误是从哪里抛出来的？"

### 1.2 解决方案

接入 **OpenTelemetry**，实现全链路追踪：
1. 每个请求有唯一 TraceID
2. 记录每个步骤的耗时
3. 可视化调用链
4. 快速定位瓶颈

### 1.3 功能需求

| 功能点 | 说明 |
|--------|------|
| Trace 透传 | HTTP → gRPC → 各服务透传 TraceID |
| Span 埋点 | 关键步骤（检索、重排、缓存等）创建 Span |
| Jaeger 导出 | 导出到 Jaeger 可视化 |
| 采样率配置 | 支持配置采样率（生产 10%，测试 100%） |

---

## 二、为什么要做这个？

### 2.1 业务价值

| 指标 | 预期提升 |
|------|---------|
| 问题定位时间 | 从 30 分钟降到 5 分钟 |
| 性能瓶颈发现 | 可直观看到各阶段耗时 |

### 2.2 技术价值

- 学习 **可观测性三支柱**（Trace、Metrics、Log）
- 掌握 **OpenTelemetry 标准**
- 理解 **分布式追踪**
- 实践 **Span 埋点**

---

## 三、技术原理

### 3.1 可观测性三支柱

```
┌─────────────────────────────────────┐
│         Observability               │
├─────────────┬─────────────┬─────────┤
│   Metrics   │   Traces    │  Logs   │
│  (指标)     │  (追踪)     │ (日志)  │
│  Prometheus │   Jaeger    │  ELK    │
└─────────────┴─────────────┴─────────┘
```

### 3.2 Trace & Span 概念

```
─ TraceID: abc123 (整个请求)
  ├─ Span 1: HTTP /api/v1/retrieve [100ms]
  │  ├─ Span 2: Semantic Cache Check [5ms]
  │  ├─ Span 3: Dense Retrieval [40ms]
  │  ├─ Span 4: Sparse Retrieval [30ms]
  │  ├─ Span 5: Fusion [5ms]
  │  └─ Span 6: Rerank [20ms]
```

### 3.3 系统架构

```
┌─────────────┐
│   前端/Agent │
└──────┬──────┘
       │ HTTP (带 TraceID)
       ▼
┌─────────────────┐
│   Go 后端 API   │ ← (1) 创建 Root Span
└──────┬──────────┘
       │
       ├───────────────┐
       ▼               ▼
┌─────────────┐  ┌─────────────┐
│   Milvus    │  │   Redis     │
│  (带 Span)  │  │  (带 Span)  │
└─────────────┘  └─────────────┘
       │               │
       └───────┬───────┘
               ▼
        ┌─────────────┐
        │   Jaeger    │ ← (2) 收集展示
        └─────────────┘
```

---

## 四、实现步骤

### Step 0: 现有代码结构

```
backend/
├── internal/
│   └── observability/
│       ├── metrics/
│       └── tracing/              ← (新建) 在这里实现
└── pkg/
    └── middleware/               ← HTTP 中间件
```

### Step 1: 添加依赖

**文件**: `backend/go.mod`

```go
require (
	go.opentelemetry.io/otel v1.19.0
	go.opentelemetry.io/otel/exporters/jaeger v1.17.0
	go.opentelemetry.io/otel/sdk v1.19.0
	go.opentelemetry.io/otel/trace v1.19.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.45.0
)
```

```bash
cd backend
go mod tidy
```

### Step 2: 初始化 Tracer Provider

**文件**: `backend/internal/observability/tracing/tracer.go`

```go
package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	// Tracer 全局 Tracer
	Tracer trace.Tracer
)

// Config 追踪配置
type Config struct {
	ServiceName string
	Environment string
	JaegerAddr  string
	Sampler     float64 // 0.0 - 1.0
}

// Init 初始化 OpenTelemetry
func Init(cfg Config) (*tracesdk.TracerProvider, error) {
	// 1. 创建 Jaeger exporter
	exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(cfg.JaegerAddr)))
	if err != nil {
		return nil, fmt.Errorf("failed to create jaeger exporter: %w", err)
	}

	// 2. 创建 TracerProvider
	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exporter),
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.DeploymentEnvironmentKey.String(cfg.Environment),
		)),
		tracesdk.WithSampler(tracesdk.TraceIDRatioBased(cfg.Sampler)),
	)

	// 3. 设置全局 TracerProvider
	otel.SetTracerProvider(tp)
	Tracer = tp.Tracer(cfg.ServiceName)

	return tp, nil
}

// StartSpan 开始一个 Span（便捷函数）
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return Tracer.Start(ctx, name, opts...)
}

// AddEvent 给当前 Span 添加事件
func AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	if span := trace.SpanFromContext(ctx); span != nil {
		span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

// RecordError 记录错误
func RecordError(ctx context.Context, err error) {
	if span := trace.SpanFromContext(ctx); span != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error", err.Error()))
	}
}
```

### Step 3: 添加 HTTP 中间件

**文件**: `backend/pkg/middleware/otel.go`

```go
package middleware

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// OTelMiddleware OpenTelemetry HTTP 中间件
func OTelMiddleware(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(
			next,
			serviceName,
			otelhttp.WithPublicEndpoint(),
		)
	}
}
```

### Step 4: 在检索流程中埋点

**修改文件**: `backend/internal/milvus/retrieval/retrieval.go`

```go
import (
	// ...
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"your-project/backend/internal/observability/tracing"
)

func (s *Service) Retrieve(ctx context.Context, kbID string, query string) ([]*Chunk, error) {
	// 创建 Root Span
	ctx, span := tracing.StartSpan(ctx, "retrieve")
	defer span.End()

	// 添加标签
	span.SetAttributes(
		attribute.String("kb_id", kbID),
		attribute.String("query", query),
	)

	// 1. 缓存检查
	ctx, cacheSpan := tracing.StartSpan(ctx, "semantic_cache_check")
	defer cacheSpan.End()

	cached, hit, err := s.semanticCache.Get(ctx, kbID, queryEmbedding)
	if hit {
		cacheSpan.SetAttributes(attribute.Bool("hit", true))
		tracing.AddEvent(ctx, "cache_hit")
		return cached, nil
	}
	cacheSpan.SetAttributes(attribute.Bool("hit", false))

	// 2. Dense 检索
	ctx, denseSpan := tracing.StartSpan(ctx, "dense_retrieval")
	denseResults, err := s.denseRetrieve(ctx, kbID, queryEmbedding)
	if err != nil {
		tracing.RecordError(ctx, err)
		return nil, err
	}
	denseSpan.SetAttributes(attribute.Int("count", len(denseResults)))
	denseSpan.End()

	// 3. Sparse 检索
	ctx, sparseSpan := tracing.StartSpan(ctx, "sparse_retrieval")
	sparseResults, err := s.sparseRetrieve(ctx, kbID, query)
	if err != nil {
		tracing.RecordError(ctx, err)
		return nil, err
	}
	sparseSpan.SetAttributes(attribute.Int("count", len(sparseResults)))
	sparseSpan.End()

	// 4. Fusion 融合
	ctx, fusionSpan := tracing.StartSpan(ctx, "fusion")
	fusedResults, err := s.fusion.Fuse(ctx, denseResults, sparseResults)
	if err != nil {
		tracing.RecordError(ctx, err)
		return nil, err
	}
	fusionSpan.End()

	// 5. 重排序
	ctx, rerankSpan := tracing.StartSpan(ctx, "rerank")
	rerankedResults, err := s.reranker.Rerank(ctx, query, fusedResults)
	if err != nil {
		tracing.RecordError(ctx, err)
		return nil, err
	}
	rerankSpan.End()

	span.SetAttributes(attribute.Int("result_count", len(rerankedResults)))
	return rerankedResults, nil
}
```

### Step 5: 在 main.go 初始化

**修改文件**: `backend/cmd/server/main.go`

```go
import (
	// ...
	"your-project/backend/internal/observability/tracing"
)

func main() {
	// ...

	// 初始化 OpenTelemetry
	tp, err := tracing.Init(tracing.Config{
		ServiceName: "rag-backend",
		Environment: "development",
		JaegerAddr:  "http://localhost:14268/api/traces",
		Sampler:     1.0, // 测试环境 100% 采样
	})
	if err != nil {
		log.Fatalf("Failed to init tracing: %v", err)
	}
	defer tp.Shutdown(context.Background())

	// 使用 HTTP 中间件
	router.Use(middleware.OTelMiddleware("rag-backend"))

	// ...
}
```

### Step 6: 添加 docker-compose 配置

**文件**: `docker-compose.jaeger.yml`

```yaml
version: '3'
services:
  jaeger:
    image: jaegertracing/all-in-one:1.51
    ports:
      - "16686:16686"  # UI
      - "14268:14268"  # Collector
      - "4317:4317"    # OTLP gRPC
    environment:
      - COLLECTOR_OTLP_ENABLED=true
```

---

## 五、如何验证？

### 5.1 启动 Jaeger

```bash
docker-compose -f docker-compose.jaeger.yml up -d
```

访问 Jaeger UI: http://localhost:16686

### 5.2 启动后端服务

```bash
cd backend
make run
```

### 5.3 发送测试请求

```bash
curl -X POST http://localhost:8080/api/v1/retrieve \
  -H "Content-Type: application/json" \
  -d '{
    "kb_id": "test-kb",
    "query": "什么是 RAG？"
  }'
```

### 5.4 在 Jaeger 查看 Trace

1. 打开 http://localhost:16686
2. Service 选择 `rag-backend`
3. 点击 Find Traces
4. 点击最新的 Trace，应该看到：
   - `/api/v1/retrieve` (Root Span)
   - `semantic_cache_check`
   - `dense_retrieval`
   - `sparse_retrieval`
   - `fusion`
   - `rerank`

### 5.5 验收标准

| 验收项 | 标准 |
|--------|------|
| Jaeger 收到 Trace | ✅ 能在 Jaeger 看到 |
| Span 完整 | 各关键步骤都有 Span |
| 属性正确 | kb_id、query 等标签正确 |
| 性能 | 不增加超过 5% 的开销 |

---

## 六、代码提交流程

### 6.1 提交代码

```bash
git checkout -b feature/TASK-003-opentelemetry

git add backend/internal/observability/tracing/
git add backend/pkg/middleware/otel.go
git add backend/internal/milvus/retrieval/retrieval.go
git add backend/cmd/server/main.go
git add docker-compose.jaeger.yml

git commit -m "feat: TASK-003 实现 OpenTelemetry 全链路追踪

- 集成 OpenTelemetry + Jaeger
- 实现 HTTP 中间件
- 检索流程关键步骤埋点
- 添加 docker-compose 配置"

git push origin feature/TASK-003-opentelemetry
```

### 6.2 创建 PR

**标题**: `feat: TASK-003 实现 OpenTelemetry 全链路追踪`

**内容**:

```markdown
## 任务说明
- 任务 ID：TASK-003
- 功能：OpenTelemetry 全链路追踪
- 实现人：[你的名字]

## 实现方案
- OpenTelemetry + Jaeger
- HTTP 中间件自动透传 Trace
- 检索流程关键步骤埋点
- 支持采样率配置

## 验证结果
- [x] Jaeger 能正常接收 Trace
- [x] 各 Span 完整
- [x] 性能开销 < 5%

## 截图
[Jaeger UI 截图]

## 相关文档
- 教程：./docs/TASK-003-opentelemetry-tracing.md
```

---

## 🎉 恭喜！

完成这个任务后，你将：
- ✅ 理解 OpenTelemetry 原理
- ✅ 掌握分布式追踪
- ✅ 学会 Span 埋点
- ✅ 实践可观测性

**下一步**: 去做 TASK-004（RRF 融合）！
