# RAG 边界规则 (Boundary Rules)

> 本文档定义了 RAG 模块与业务模块之间的导入边界，用于保障 RAG Platform 独立拆分的可行性。
> 自动化检查脚本：`scripts/check_rag_boundaries.sh` / `scripts/check_rag_boundaries.ps1`

---

## 1. RAG 归属目录（允许导入）

以下目录属于 RAG Platform，允许被 RAG 相关包导入：

| 路径 | 说明 |
|------|------|
| `api/router/custom_kb.go` | 知识库 API 路由定义 |
| `api/handler/kb/*` | 知识库 HTTP Handler |
| `internal/milvus/*` | Milvus 向量数据库客户端 |
| `internal/rag/*` | RAG 核心逻辑（检索、召回、融合等） |
| `internal/model/kb_*` | 知识库相关数据模型 |
| `internal/service/kb/*` | 知识库业务服务层 |
| `internal/mq` 中的 `knowledge_ingest` 相关 | 知识库文档摄入消息队列 |
| `internal/observability/metrics/rag_metrics.go` | RAG 指标监控 |
| `cmd/retrieval-eval` | 检索评估 CLI 工具 |
| `cmd/retrieval-benchmark` | 检索基准测试 CLI 工具 |

---

## 2. 业务归属目录（RAG 包禁止导入）

以下目录属于业务层，RAG 包**禁止**导入这些模块：

| 路径 | 说明 |
|------|------|
| `internal/agents/*` | Agent 编排层 |
| `internal/service/interview/*` | 面试业务服务 |
| `internal/service/resume/*` | 简历业务服务 |
| `internal/service/prediction/*` | 预测业务服务 |
| `internal/payment/*` | 支付模块 |
| `api/handler/interview/*` | 面试 HTTP Handler |
| `api/handler/resume/*` | 简历 HTTP Handler |
| `api/handler/payment/*` | 支付 HTTP Handler |
| `api/router/interview/*` | 面试路由 |
| `api/router/payment/*` | 支付路由 |
| `internal/model/interview_*` | 面试数据模型 |
| `internal/model/resume.go` | 简历数据模型 |
| `internal/model/prediction.go` | 预测数据模型 |
| `internal/model/payment_*` | 支付数据模型 |
| `internal/model/subscription.go` | 订阅数据模型 |

---

## 3. 业务 Agent 禁止导入的 RAG 包

业务层（特别是 Agent 层）**禁止**直接导入以下 RAG 内部实现包：

| 路径 | 说明 |
|------|------|
| `internal/milvus/*` | Milvus 客户端（应通过 RAG 服务层间接调用） |
| `internal/rag/*` | RAG 核心逻辑（应通过 RAG 服务层间接调用） |
| `api/handler/kb/*` | 知识库 Handler（不应在业务层直接调用） |

---

## 4. 依赖方向规则

```
┌─────────────────────────────────┐
│         业务层 (Business)        │
│  agents / interview / resume /  │
│  prediction / payment           │
└──────────────┬──────────────────┘
               │ 通过 service/kb 接口调用
               ▼
┌─────────────────────────────────┐
│       RAG 服务层 (RAG Service)   │
│  service/kb / handler/kb /      │
│  router/custom_kb.go            │
└──────────────┬──────────────────┘
               │ 内部实现
               ▼
┌─────────────────────────────────┐
│       RAG 实现层 (RAG Core)      │
│  internal/rag / internal/milvus │
│  model/kb_* / mq/knowledge_*    │
└─────────────────────────────────┘
```

**核心原则：**
- 业务层 → RAG 服务层：✅ 允许（通过接口）
- 业务层 → RAG 实现层：❌ 禁止
- RAG 实现层 → 业务层：❌ 禁止
- RAG 服务层 → RAG 实现层：✅ 允许

---

## 5. 变更记录

| 日期 | 操作 | 说明 |
|------|------|------|
| 2026-06-02 | 创建 | L0 阶段：边界冻结与依赖检查 |
