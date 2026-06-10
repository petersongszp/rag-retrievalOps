# RAG Platform 多模态能力调研报告

> **项目**：rag-retrievalOps — RAG 后台管理平台  
> **版本**：v1.0  
> **日期**：2026-06-09  
> **作者**：技术调研员（AutoClaw 自动生成）

---

## 目录

1. [项目概述](#1-项目概述)
2. [当前架构分析](#2-当前架构分析)
   - 2.1 [系统架构总览](#21-系统架构总览)
   - 2.2 [后端架构](#22-后端架构)
   - 2.3 [Milvus 向量存储层](#23-milvus-向量存储层)
   - 2.4 [Embedding 服务](#24-embedding-服务)
   - 2.5 [检索能力](#25-检索能力)
   - 2.6 [前端管理界面](#26-前端管理界面)
   - 2.7 [存储与队列](#27-存储与队列)
   - 2.8 [当前架构的多模态空白点](#28-当前架构的多模态空白点)
3. [多模态技术方案调研](#3-多模态技术方案调研)
   - 3.1 [图片能力方案](#31-图片能力方案)
   - 3.2 [视频能力方案](#32-视频能力方案)
   - 3.3 [多模态 Embedding 方案对比](#33-多模态-embedding-方案对比)
   - 3.4 [多模态检索方案对比](#34-多模态检索方案对比)
   - 3.5 [综合方案对比表](#35-综合方案对比表)
4. [推荐落地方案](#4-推荐落地方案)
   - 4.1 [总体架构设计](#41-总体架构设计)
   - 4.2 [Python 微服务设计](#42-python-微服务设计)
   - 4.3 [Milvus Schema 设计](#43-milvus-schema-设计)
   - 4.4 [Go 后端改造方案](#44-go-后端改造方案)
   - 4.5 [前端改造方案](#45-前端改造方案)
   - 4.6 [配置与部署方案](#46-配置与部署方案)
5. [分阶段实施计划](#5-分阶段实施计划)
   - 5.1 [Phase 1：图片知识库 MVP（6 周）](#51-phase-1图片知识库-mvp6-周)
   - 5.2 [Phase 2：视频能力（6 周）](#52-phase-2视频能力6-周)
   - 5.3 [Phase 3：高级多模态检索（6 周）](#53-phase-3高级多模态检索6-周)
6. [风险与成本评估](#6-风险与成本评估)
   - 6.1 [性能影响分析](#61-性能影响分析)
   - 6.2 [月度成本估算](#62-月度成本估算)
   - 6.3 [已知限制与风险](#63-已知限制与风险)
   - 6.4 [缓解措施](#64-缓解措施)
7. [附录](#7-附录)
   - 7.1 [检查过的关键文件列表](#71-检查过的关键文件列表)
   - 7.2 [参考资源](#72-参考资源)
   - 7.3 [术语表](#73-术语表)

---

## 1. 项目概述

### 1.1 背景

rag-retrievalOps 是一个独立的 RAG（Retrieval-Augmented Generation）后台管理平台，为知识库管理、向量检索、文档入库、检索评测等场景提供完整的管理能力。

**当前技术栈：**

| 层级 | 技术 | 说明 |
|------|------|------|
| 后端 | Go (Hertz 框架) | 入口 `backend/cmd/rag-server`，端口 8899 |
| 前端 | Next.js | 入口 `admin/`，端口 3003 |
| 向量数据库 | Milvus v2.4.23 | 端口 19531，standalone 模式 |
| 关系数据库 | MySQL 8.0 | 端口 3308 |
| 缓存 | Redis 7 | 端口 6380 |
| 对象存储 | Local OSS（可扩展 MinIO/S3） | `backend/internal/storage/` |
| Embedding | ark / openai 两种 provider | 通过 config.yaml 配置 |
| 消息队列 | Redis Stream | `backend/internal/ragqueue/` |

**当前能力：**
- Dense + Sparse 混合检索（RRF / MinMax 融合策略）
- Rerank（Jaccard / 可配置 Reranker）
- 查询改写（规则 + 模型辅助）
- 动态 TopK
- 证据拒答（Evidence Gate）
- 引用一致性检查
- 父子检索（Parent-Child Retrieval）
- 评测系统（离线评测 + A/B 实验平台）
- 治理与审计（成本监控、合规审计、告警）

**核心问题：当前系统仅支持纯文本 RAG，没有任何多模态（图片、视频）处理能力。**

### 1.2 调研目标

1. 分析当前架构，识别多模态扩展的切入点
2. 调研图片/视频处理的技术方案
3. 设计可落地的多模态架构方案
4. 制定分阶段实施计划
5. 评估成本与风险

---

## 2. 当前架构分析

### 2.1 系统架构总览

```
┌─────────────────────────────────────────────────────────────┐
│                      前端 (Next.js)                         │
│  admin/src/app/(admin)/                                     │
│  ├─ knowledge-bases/    知识库管理                           │
│  ├─ retrieval-lab/      检索实验室                           │
│  ├─ strategy-center/    策略中心                             │
│  ├─ evaluation/         评测系统                             │
│  ├─ vector-ops/         向量运维                             │
│  ├─ cost-ops/           成本运维                             │
│  ├─ audit/              审计中心                             │
│  └─ dashboard/          仪表盘                               │
└─────────────────────┬───────────────────────────────────────┘
                      │ HTTP API
┌─────────────────────▼───────────────────────────────────────┐
│                  Go 后端 (rag-server)                        │
│  api/handler/kb/       知识库管理 API                        │
│  api/handler/rag/      公共检索 API                          │
│  internal/ragqueue/    Redis Stream 入队消费                  │
│  internal/milvus/      向量检索 + 切分 + 评测                 │
│  ├─ storage/           Embedding + Indexer                   │
│  ├─ retrieval/         HybridRetriever + Reranker + Rewrite │
│  ├─ splitter/          文档切分器                             │
│  ├─ evaluation/        离线评测                               │
│  └─ benchmark/         性能基准测试                           │
│  internal/storage/     OSS 接口 (Local OSS)                  │
│  internal/config/      配置结构体                             │
│  internal/model/       数据模型                               │
└──────┬──────────┬──────────┬────────────────────────────────┘
       │          │          │
  ┌────▼───┐ ┌───▼────┐ ┌──▼──────┐
  │ MySQL  │ │ Redis  │ │ Milvus  │
  │ 3308   │ │ 6380   │ │ 19531   │
  └────────┘ └────────┘ └─────────┘
```

### 2.2 后端架构

后端采用 Go + Hertz 框架，模块结构清晰：

**入口：** `backend/cmd/rag-server`

**核心模块：**

| 模块 | 路径 | 职责 |
|------|------|------|
| 知识库 API | `api/handler/kb/` | 知识库 CRUD、文档管理、入库触发 |
| 检索 API | `api/handler/rag/` | 公共检索接口、查询改写、混合检索 |
| 消息队列 | `internal/ragqueue/` | Redis Stream 生产/消费，文档异步入库 |
| Milvus 管理器 | `internal/milvus/init.go` | 全局单例，统一管理 Milvus 客户端 + 各服务 |
| Embedding 服务 | `internal/milvus/storage/embedding.go` | 支持 ark / openai provider |
| 索引器 | `internal/milvus/storage/indexer.go` | Milvus 文档写入（FloatVector + COSINE） |
| 检索器 | `internal/milvus/retrieval/` | Dense/Sparse 混合检索、Rerank、改写 |
| 文档切分 | `internal/milvus/splitter/` | 递归切分 + 父子切分 |
| OSS 存储 | `internal/storage/` | 统一存储接口（当前仅 LocalOSS） |

**MilvusManager 初始化流程（init.go）：**

```go
// 1. 连接 Milvus
// 2. 初始化 EmbeddingService (ark/openai)
// 3. 解析 knowledge collection + 自动探测维度
// 4. 初始化 DocumentSplitterService
// 5. 初始化 IndexerService (FloatVector, COSINE)
// 6. 初始化 RetrieverService
// 7. 初始化 HybridRetriever (Dense+Sparse+Rerank+Rewrite+DynamicTopK)
```

### 2.3 Milvus 向量存储层

**当前 Collection Schema（documents）：**

```go
Fields: [
    {Name: "id",        Type: VarChar(255), PrimaryKey: true},
    {Name: "vector",    Type: FloatVector,  Dim: 动态探测},
    {Name: "content",   Type: VarChar(4096)},
    {Name: "metadata",  Type: JSON},
]
MetricType: COSINE
```

**关键特征：**
- 单 collection 设计，通过 `metadata` JSON 字段存储 document_id、knowledge_base_id 等
- 支持多 collection 映射（`Collections` 配置）
- `collection_resolver.go` 负责解析和自动创建 collection
- 向量维度通过 Embedding 服务自动探测
- 仅有一个 `vector` 字段（FloatVector），没有 sparse 向量字段
- Sparse 检索通过内存倒排索引实现（`sparse_inverted_index.go`），非 Milvus 原生

**当前无多模态相关字段：** 没有图片 URL、视频时间戳、模态类型等字段。

### 2.4 Embedding 服务

**支持的 Provider：**

| Provider | 模型示例 | 认证方式 | 说明 |
|----------|----------|----------|------|
| `ark` | 火山引擎方舟 | AK/SK | 字节跳动向量模型 |
| `openai` | text-embedding-3-large 等 | API Key | OpenAI 兼容接口，兼容国内多家 |

**配置结构（EmbeddingConfig）：**

```yaml
Embedding:
  APIKey: "${EMBEDDING_API_KEY}"
  Provider: "${EMBEDDING_PROVIDER}"    # ark | openai
  Model: "${EMBEDDING_MODEL}"
  BaseURL: "${EMBEDDING_BASE_URL}"
  Region: "${EMBEDDING_REGION}"        # ark 专用
  Timeout: 30s
  RetryTimes: 3
  Dimensions: 2048
  BatchSize: 10
```

**关键限制：** 当前 Embedding 服务仅接受文本输入，没有任何图像 Embedding 能力。

### 2.5 检索能力

**检索流水线：**

```
用户查询
  │
  ├─ Query Rewriter（查询改写）
  │   ├─ 规则改写
  │   ├─ 领域术语
  │   ├─ 路由特定改写
  │   └─ 模型辅助改写
  │
  ├─ Hybrid Retriever（混合检索）
  │   ├─ Dense Search（向量检索）
  │   ├─ Sparse Search（稀疏检索，倒排索引）
  │   └─ Fusion（RRF / MinMax 融合）
  │
  ├─ Reranker（重排序）
  │   ├─ Jaccard Reranker（默认）
  │   └─ Configurable Reranker（可扩展）
  │
  ├─ Dynamic TopK（动态截断）
  │
  ├─ Parent-Child（父子检索扩展）
  │
  ├─ Evidence Gate（证据拒答）
  │
  └─ Citation Consistency（引用一致性）
```

**关键限制：** 所有检索都基于文本向量，没有图片向量检索、图文联合检索能力。

### 2.6 前端管理界面

**当前 25+ 管理页面：**

```
admin/src/app/(admin)/
├─ dashboard/              # 仪表盘
├─ knowledge-bases/        # 知识库管理
│   └─ [kbId]/             # 知识库详情
├─ retrieval-lab/          # 检索实验室
│   └─ debug/              # 检索调试
├─ strategy-center/        # 策略中心
├─ evaluation/             # 评测系统
│   ├─ datasets/           # 评测数据集
│   ├─ runs/               # 评测运行
│   └─ reports/            # 评测报告
│       └─ [runId]/
├─ vector-ops/             # 向量运维
├─ cost-ops/               # 成本运维
│   ├─ cost/               # 成本详情
│   └─ vector-db/          # 向量数据库运维
├─ audit/                  # 审计中心
├─ alerts/                 # 告警管理
├─ api-keys/               # API Key 管理
├─ tenant/                 # 租户管理
│   ├─ settings/           # 租户设置
│   └─ usage/              # 用量统计
├─ trace-logs/             # 日志追踪
│   ├─ ingest/             # 入库日志
│   └─ retrieval/          # 检索日志
├─ quality-monitor/        # 质量监控
├─ reports/                # 报告
│   └─ weekly/             # 周报
└─ docs/                   # 文档
    └─ integration/        # 集成指南
```

**关键限制：** 没有任何多模态相关页面（图片预览、视频播放、多模态检索实验室等）。

### 2.7 存储与队列

**OSS 接口（`internal/storage/oss.go`）：**

```go
type OSS interface {
    PutObject(ctx, key, reader, size, contentType) (string, error)
    GetObject(ctx, key) (io.ReadCloser, error)
    DeleteObject(ctx, key) error
    GetURL(ctx, key) (string, error)
}
```

当前实现：`LocalOSS`（本地文件系统）。接口设计良好，天然支持扩展到 MinIO/S3。

**Redis Stream 队列（`internal/ragqueue/`）：**

- `queue.go`：队列接口定义
- `redis_stream.go`：Redis Stream 实现
- `consumer.go`：消费逻辑，负责文档入库

当前仅处理文本文档入库，没有多模态任务队列。

### 2.8 当前架构的多模态空白点

| 维度 | 当前状态 | 缺失能力 |
|------|----------|----------|
| **Embedding** | 纯文本 Embedding (ark/openai) | 图像 Embedding、多模态 Embedding |
| **Milvus Schema** | 单 vector 字段 | 多向量字段（image_vector + text_vector） |
| **检索** | 文本向量检索 | 以图搜图、图文联合检索 |
| **存储** | 仅文本文档 | 图片/视频文件存储与管理 |
| **处理** | 文本切分 + 入库 | OCR、图片理解、视频帧提取、语音转录 |
| **前端** | 文本知识库管理 | 图片预览、视频播放、多模态检索界面 |
| **队列** | 文本文档入库 | 多模态任务异步处理 |
| **评测** | 文本检索评测 | 多模态检索评测 |

---

## 3. 多模态技术方案调研

### 3.1 图片能力方案

#### 3.1.1 图片理解（OCR + 视觉描述）

| 方案 | 类型 | 优势 | 劣势 | 部署方式 | 成本 |
|------|------|------|------|----------|------|
| **PaddleOCR** | 开源 OCR | 中文识别优秀、免费、支持 80+ 语言 | 需要 GPU 加速、表格识别一般 | 自部署 Docker | GPU 服务器成本 |
| **Tesseract** | 开源 OCR | 轻量、社区活跃 | 中文识别差、布局分析弱 | 自部署 | 免费 |
| **Qwen-VL** | 多模态 LLM | 理解能力强、中文优秀、支持图片描述+OCR | API 调用成本、延迟较高 | API 调用 | ¥0.003-0.02/张 |
| **GPT-4o** | 多模态 LLM | 理解能力最强、支持复杂图表 | 成本高、数据出境风险 | API 调用 | $0.005-0.01/张 |
| **InternVL2** | 开源多模态 | 免费、中文优秀、可私有化部署 | 需要 GPU、部署复杂 | 自部署 Docker | GPU 服务器成本 |
| **MinerU** | 开档解析工具 | 专为文档设计、表格/公式识别好 | 仅限文档场景 | 自部署 | 免费 |

**推荐组合：**
- **基础 OCR：** PaddleOCR（免费、中文好、可自部署）
- **图片理解：** Qwen-VL API（高质量、中文优秀、成本可控）
- **文档解析：** MinerU（处理 PDF/文档中的图片和表格）

#### 3.1.2 图片 Embedding 方案

| 方案 | 维度 | 特点 | 中文支持 | 部署方式 | 许可证 |
|------|------|------|----------|----------|--------|
| **CLIP (ViT-L/14)** | 768 | 图文对比学习、经典方案 | 一般 | 自部署/API | MIT |
| **SigLIP** | 768 | Google 改进版、Sigmoid 损失 | 一般 | 自部署 | Apache 2.0 |
| **E5-V** | 1024 | 微软、统一图文 Embedding | 好 | 自部署 | MIT |
| **BGE-VL** | 1024 | 智源、中文优化、SOTA | 优秀 | 自部署/API | MIT |
| **Jina CLIP v2** | 768 | Jina AI、多语言、商用友好 | 好 | API/自部署 | Apache 2.0 |
| **OpenAI CLIP** | 1536 | OpenAI 官方、稳定 | 一般 | API | 商用 |
| **Cohere Embed v3** | 1024 | 多模态、商用 | 好 | API | 商用 |

**推荐方案：**
- **首选：** BGE-VL（中文最优、开源免费、维度与现有系统兼容）
- **备选：** Jina CLIP v2（API 调用简单、多语言好）
- **快速集成：** OpenAI CLIP（API 调用、无需 GPU）

#### 3.1.3 图片检索模式

| 检索模式 | 说明 | 实现方式 | 适用场景 |
|----------|------|----------|----------|
| **以图搜图** | 输入图片，返回相似图片 | CLIP/BGE-VL 图片向量 + Milvus ANN | 图片库搜索、版权检测 |
| **文搜图** | 输入文本，返回相关图片 | CLIP/BGE-VL 图文联合空间 | 知识库图文检索 |
| **图搜文** | 输入图片，返回相关文本 | 图片向量检索文本向量 | 视觉问答 |
| **多模态混合检索** | 同时检索文本和图片 | 多 collection + 结果融合 | 完整 RAG 场景 |

### 3.2 视频能力方案

#### 3.2.1 关键帧提取

| 方案 | 特点 | 优势 | 劣势 |
|------|------|------|------|
| **FFmpeg** | 视频处理工具链 | 成熟稳定、功能丰富 | 需要命令行调用 |
| **PySceneDetect** | Python 场景检测 | 内容感知、自动检测场景切换 | 处理速度较慢 |
| **OpenCV** | 计算机视觉库 | 灵活、可自定义 | 需要编程实现 |
| **FFmpeg + PySceneDetect 组合** | 混合方案 | 兼顾速度和质量 | 部署稍复杂 |

**推荐：** FFmpeg（基础帧提取）+ PySceneDetect（智能场景检测）

#### 3.2.2 语音转录

| 方案 | 语言支持 | 准确率 | 部署方式 | 成本 |
|------|----------|--------|----------|------|
| **Whisper large-v3** | 100+ 语言 | 优秀 | 自部署/API | 免费自部署 / $0.006/min API |
| **FunASR** | 中文优化 | 优秀（中文） | 自部署 | 免费 |
| **AssemblyAI** | 多语言 | 优秀 | API | $0.015/min |
| **Azure Speech** | 多语言 | 优秀 | API | $1/hour |

**推荐：** FunASR（中文场景，免费自部署）+ Whisper large-v3（多语言场景）

#### 3.2.3 视频 Embedding

| 方案 | 特点 | 适用场景 |
|------|------|----------|
| **关键帧 CLIP Embedding** | 对提取的关键帧做图片 Embedding | 视频内容检索 |
| **文本 Embedding（转录文本）** | 对 Whisper 转录文本做 Embedding | 视频语音内容检索 |
| **VideoCLIP** | 视频专用 Embedding 模型 | 端到端视频理解 |
| **多模态融合** | 关键帧向量 + 转录文本向量 | 综合视频检索 |

**推荐：** 关键帧 CLIP Embedding + 转录文本 Embedding 双通道

### 3.3 多模态 Embedding 方案对比

| 方案 | 输入类型 | 输出维度 | 中文 | 开源 | 部署难度 | 推荐度 |
|------|----------|----------|------|------|----------|--------|
| CLIP ViT-L/14 | 图/文 | 768 | ⭐⭐ | ✅ | 低 | ⭐⭐⭐ |
| SigLIP | 图/文 | 768 | ⭐⭐ | ✅ | 低 | ⭐⭐⭐ |
| E5-V | 图/文 | 1024 | ⭐⭐⭐ | ✅ | 中 | ⭐⭐⭐⭐ |
| BGE-VL | 图/文 | 1024 | ⭐⭐⭐⭐⭐ | ✅ | 中 | ⭐⭐⭐⭐⭐ |
| Jina CLIP v2 | 图/文 | 768 | ⭐⭐⭐ | ✅ | 低 | ⭐⭐⭐⭐ |
| OpenAI CLIP | 图/文 | 1536 | ⭐⭐ | ❌ | 极低 | ⭐⭐⭐ |
| Cohere Embed v3 | 图/文/表 | 1024 | ⭐⭐⭐ | ❌ | 极低 | ⭐⭐⭐ |

### 3.4 多模态检索方案对比

| 方案 | 检索方式 | Milvus 支持 | 复杂度 | 效果 |
|------|----------|-------------|--------|------|
| **独立 Collection** | 图片/文本各一个 collection，结果合并 | 原生支持 | 低 | 好 |
| **多向量字段** | 同一 collection 多个 vector 字段 | hybrid_search 原生支持 | 中 | 优秀 |
| **统一向量空间** | CLIP 图文同一空间，一个 vector 字段 | 原生支持 | 低 | 良好 |
| **跨模态 Rerank** | 初筛多路 + 跨模态 Rerank 精排 | 需要外部 Reranker | 高 | 最优 |

### 3.5 综合方案对比表

| 维度 | 方案 A：CLIP + 独立 Collection | 方案 B：BGE-VL + 多向量字段 | 方案 C：统一 Embedding + 混合 Collection |
|------|-------------------------------|---------------------------|----------------------------------------|
| **图片理解** | Qwen-VL API | Qwen-VL API + PaddleOCR | Qwen-VL API + PaddleOCR |
| **图片 Embedding** | CLIP ViT-L/14 | BGE-VL | E5-V / BGE-VL |
| **Milvus Schema** | 独立 `images` collection | 同一 collection，多 vector 字段 | 统一 collection，text_vector + image_vector |
| **检索方式** | 各模态独立检索 + 结果合并 | Milvus hybrid_search | 跨模态检索 + RRF 融合 |
| **优点** | 简单、风险低、易扩展 | 原生多向量、检索高效 | 统一管理、检索效果最优 |
| **缺点** | 需要跨 collection 合并逻辑 | Schema 迁移复杂 | 实现最复杂 |
| **部署难度** | ⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| **推荐度** | ⭐⭐⭐⭐（MVP） | ⭐⭐⭐⭐⭐（生产） | ⭐⭐⭐（远期目标） |

**最终推荐：**
- **Phase 1（MVP）：** 方案 A（CLIP + 独立 Collection），快速上线
- **Phase 2-3（生产）：** 渐进迁移到方案 B（BGE-VL + 多向量字段）

---

## 4. 推荐落地方案

### 4.1 总体架构设计

```
┌──────────────────────────────────────────────────────────────────┐
│                        前端 (Next.js)                            │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────────┐    │
│  │ 知识库管理   │  │ 多模态检索    │  │ 图片/视频预览管理     │    │
│  │ (现有页面扩展)│  │ 实验室 (新增) │  │ (新增)               │    │
│  └──────┬──────┘  └──────┬───────┘  └──────────┬───────────┘    │
└─────────┼────────────────┼─────────────────────┼────────────────┘
          │                │                     │
┌─────────▼────────────────▼─────────────────────▼────────────────┐
│                     Go 后端 (rag-server)                         │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  新增模块                                                  │   │
│  │  ├─ api/handler/multimodal/    多模态 API                 │   │
│  │  ├─ internal/multimodal/       多模态服务层                │   │
│  │  │   ├─ image/                 图片处理服务                │   │
│  │  │   ├─ video/                 视频处理服务                │   │
│  │  │   └─ retrieval/             多模态检索服务              │   │
│  │  └─ internal/milvus/retrieval/ 扩展 hybrid_search         │   │
│  └──────────────────────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  现有模块（保持不变）                                       │   │
│  │  ├─ internal/ragqueue/         扩展多模态任务类型           │   │
│  │  ├─ internal/storage/          扩展 MinIO OSS              │   │
│  │  └─ internal/milvus/           新增 images collection      │   │
│  └──────────────────────────────────────────────────────────┘   │
└──────────┬───────────┬───────────┬──────────────────────────────┘
           │           │           │
    ┌──────▼───┐ ┌─────▼────┐ ┌───▼────────────┐
    │  MySQL   │ │  Redis   │ │    Milvus       │
    │          │ │  Stream  │ │ ┌─────────────┐ │
    │          │ │          │ │ │ documents   │ │  (现有)
    │          │ │          │ │ ├─────────────┤ │
    │          │ │          │ │ │ images      │ │  (新增)
    │          │ │          │ │ ├─────────────┤ │
    │          │ │          │ │ │ videos      │ │  (Phase 2)
    └──────────┘ └──────────┘ │ └─────────────┘ │
                               └─────────────────┘
           │
    ┌──────▼──────────────────────────────────────┐
    │         Python 微服务 (multimodal-worker)     │
    │  ┌─────────────┐  ┌──────────────────────┐  │
    │  │ PaddleOCR   │  │ Qwen-VL / GPT-4o    │  │
    │  │ (图片 OCR)   │  │ (图片理解)            │  │
    │  └─────────────┘  └──────────────────────┘  │
    │  ┌─────────────┐  ┌──────────────────────┐  │
    │  │ BGE-VL      │  │ FFmpeg / Whisper     │  │
    │  │ (图片 Embed) │  │ (视频处理)            │  │
    │  └─────────────┘  └──────────────────────┘  │
    └──────────────────────────────────────────────┘
           │
    ┌──────▼───┐
    │  MinIO   │  (新增：原始图片/视频存储)
    └──────────┘
```

### 4.2 Python 微服务设计

#### 4.2.1 为什么用 Python 微服务

| 因素 | 说明 |
|------|------|
| **生态优势** | PaddleOCR、CLIP、Whisper、PySceneDetect 等均以 Python 为主 |
| **模型兼容** | PyTorch/TensorFlow 模型加载天然支持 Python |
| **Go 互补** | Go 负责 API 网关 + 业务逻辑，Python 负责 AI 推理 |
| **独立部署** | 可独立扩缩容，不影响现有 Go 服务稳定性 |
| **异步解耦** | 通过 Redis Stream 消息队列解耦，Go 不直接调用 Python |

#### 4.2.2 服务接口设计

```python
# multimodal-worker 主要接口

# 1. 图片处理服务
POST /api/v1/image/ocr          # 图片 OCR
POST /api/v1/image/describe     # 图片描述（VLM）
POST /api/v1/image/embed        # 图片 Embedding
POST /api/v1/image/process      # 完整处理流水线（OCR + 描述 + Embedding）

# 2. 视频处理服务 (Phase 2)
POST /api/v1/video/extract-frames   # 关键帧提取
POST /api/v1/video/transcribe       # 语音转录
POST /api/v1/video/embed            # 视频 Embedding

# 3. 健康检查
GET  /healthz
GET  /readyz
```

#### 4.2.3 图片处理流水线

```
图片上传
  │
  ├─ 1. 存储到 MinIO → 获取 URL
  │
  ├─ 2. OCR 提取文本 (PaddleOCR)
  │     └─ 输出：结构化文本 + 置信度
  │
  ├─ 3. 图片理解/描述 (Qwen-VL API)
  │     └─ 输出：图片描述、关键实体、场景标签
  │
  ├─ 4. 图片 Embedding (BGE-VL / CLIP)
  │     └─ 输出：768/1024 维向量
  │
  └─ 5. 结果组装 → 写入 Milvus images collection
```

### 4.3 Milvus Schema 设计

#### 4.3.1 Phase 1：独立 images collection

```go
// images collection schema
Fields: [
    {
        Name: "id",
        Type: VarChar(255),
        PrimaryKey: true,
        Description: "唯一标识符 (kb_id:doc_id:page_idx)",
    },
    {
        Name: "image_vector",
        Type: FloatVector,
        Dim: 1024,  // BGE-VL 或 CLIP 维度
        Description: "图片向量 (CLIP/BGE-VL)",
    },
    {
        Name: "text_vector",
        Type: FloatVector,
        Dim: 2048,  // 与现有文本 Embedding 维度一致
        Description: "OCR/描述文本向量",
    },
    {
        Name: "content",
        Type: VarChar(8192),
        Description: "OCR 提取文本 + 图片描述",
    },
    {
        Name: "metadata",
        Type: JSON,
        Description: "结构化元数据",
    },
]

// metadata JSON 结构
{
    "knowledge_base_id": 123,
    "document_id": 456,
    "page_index": 3,
    "image_url": "minio://bucket/kb123/doc456/page3.png",
    "image_hash": "sha256:...",
    "image_width": 1920,
    "image_height": 1080,
    "ocr_confidence": 0.95,
    "description": "这是一张产品架构图...",
    "tags": ["架构图", "系统设计"],
    "source_type": "pdf_page",  // pdf_page | standalone_image | screenshot
    "created_at": "2026-06-09T01:00:00Z",
    "modality": "image"
}

// 索引配置
Index: HNSW (image_vector, COSINE)
Index: HNSW (text_vector, COSINE)
```

#### 4.3.2 Phase 2：扩展 videos collection

```go
// videos collection schema
Fields: [
    {
        Name: "id",
        Type: VarChar(255),
        PrimaryKey: true,
    },
    {
        Name: "frame_vector",
        Type: FloatVector,
        Dim: 1024,
        Description: "关键帧图片向量",
    },
    {
        Name: "transcript_vector",
        Type: FloatVector,
        Dim: 2048,
        Description: "转录文本向量",
    },
    {
        Name: "content",
        Type: VarChar(8192),
        Description: "转录文本内容",
    },
    {
        Name: "metadata",
        Type: JSON,
        Description: "视频元数据",
    },
]

// metadata JSON 结构
{
    "knowledge_base_id": 123,
    "video_id": "vid_001",
    "video_url": "minio://bucket/videos/vid_001.mp4",
    "segment_start_sec": 30.5,
    "segment_end_sec": 45.2,
    "frame_url": "minio://bucket/frames/vid_001_30.5.jpg",
    "transcript": "接下来我们看一下系统的架构设计...",
    "speaker": "speaker_1",
    "scene_type": "presentation",
    "modality": "video"
}
```

#### 4.3.3 Phase 3：统一多模态 collection（远期）

```go
// multimodal collection (统一向量空间)
Fields: [
    {Name: "id",            Type: VarChar(255), PrimaryKey: true},
    {Name: "dense_vector",  Type: FloatVector, Dim: 1024},   // 统一多模态向量
    {Name: "content",       Type: VarChar(8192)},
    {Name: "metadata",      Type: JSON},
]
```

### 4.4 Go 后端改造方案

#### 4.4.1 新增模块清单

```
backend/
├─ api/handler/
│   └─ multimodal/              # 新增：多模态 API handler
│       ├─ image_handler.go     # 图片上传、处理、检索 API
│       ├─ video_handler.go     # 视频上传、处理、检索 API (Phase 2)
│       └─ types.go             # 请求/响应结构体
│
├─ api/ragrouter/
│   └─ multimodal_router.go     # 新增：多模态路由注册
│
├─ internal/
│   ├─ multimodal/              # 新增：多模态服务层
│   │   ├─ service.go           # 多模态服务管理器
│   │   ├─ image/
│   │   │   ├─ processor.go     # 图片处理服务（调用 Python 微服务）
│   │   │   ├─ ocr.go           # OCR 结果处理
│   │   │   └─ embedding.go     # 图片 Embedding 封装
│   │   ├─ video/               # Phase 2
│   │   │   ├─ processor.go
│   │   │   ├─ frame_extractor.go
│   │   │   └─ transcriber.go
│   │   └─ retrieval/
│   │       ├─ image_search.go  # 图片检索
│   │       └─ video_search.go  # 视频检索 (Phase 2)
│   │
│   ├─ milvus/
│   │   └─ init.go              # 修改：初始化 images/videos collection
│   │
│   ├─ ragqueue/
│   │   ├─ queue.go             # 修改：新增多模态任务类型
│   │   └─ consumer.go          # 修改：消费多模态任务
│   │
│   ├─ storage/
│   │   ├─ oss.go               # 现有接口（无需修改）
│   │   ├─ local_oss.go         # 现有实现
│   │   └─ minio_oss.go         # 新增：MinIO 实现
│   │
│   └─ config/
│       └─ config.go            # 修改：新增多模态配置结构
│
└─ config.yaml                  # 修改：新增多模态配置段
```

#### 4.4.2 配置扩展（config.yaml）

```yaml
# 新增多模态配置段
multimodal:
  enabled: true
  
  # Python 微服务配置
  worker:
    base_url: "http://multimodal-worker:8900"
    timeout: 60s
    retry_times: 3
  
  # 图片处理配置
  image:
    enabled: true
    max_size_mb: 20
    allowed_formats: ["jpg", "jpeg", "png", "webp", "bmp", "tiff"]
    ocr:
      enabled: true
      engine: "paddleocr"      # paddleocr | tesseract
      language: "ch"           # ch | en | multi
    description:
      enabled: true
      provider: "qwen-vl"     # qwen-vl | gpt-4o
      model: "qwen-vl-max"
      api_key: "${MULTIMODAL_VLM_API_KEY}"
      base_url: "${MULTIMODAL_VLM_BASE_URL}"
    embedding:
      provider: "bge-vl"      # bge-vl | clip | jina-clip
      model: "BAAI/bge-vl-base"
      dimensions: 1024
      base_url: "${MULTIMODAL_EMBEDDING_BASE_URL}"
      api_key: "${MULTIMODAL_EMBEDDING_API_KEY}"
  
  # 视频处理配置 (Phase 2)
  video:
    enabled: false
    max_size_mb: 500
    allowed_formats: ["mp4", "avi", "mov", "mkv", "webm"]
    frame_extraction:
      method: "scene_detect"   # scene_detect | interval
      interval_sec: 5          # 当 method=interval 时使用
      min_scene_duration_sec: 2
    transcription:
      enabled: true
      engine: "funasr"         # funasr | whisper
      model: "paraformer-zh"
      language: "zh"
    embedding:
      provider: "bge-vl"
      dimensions: 1024
  
  # Milvus 配置
  milvus:
    images_collection: "images"
    videos_collection: "videos"   # Phase 2
    metric_type: "COSINE"
  
  # MinIO 配置
  storage:
    provider: "minio"            # local | minio
    minio:
      endpoint: "${MINIO_ENDPOINT}"
      access_key: "${MINIO_ACCESS_KEY}"
      secret_key: "${MINIO_SECRET_KEY}"
      bucket: "rag-multimodal"
      use_ssl: false
```

#### 4.4.3 关键接口设计

```go
// api/handler/multimodal/image_handler.go

// POST /api/v1/kb/{kbId}/images/upload
// 上传图片到知识库
type ImageUploadRequest struct {
    File        *multipart.FileHeader `form:"file"`
    DocumentID  uint64                `form:"document_id"`   // 可选，关联文档
    Description string                `form:"description"`   // 可选，用户描述
    Tags        []string              `form:"tags"`          // 可选，标签
}

type ImageUploadResponse struct {
    ImageID     string   `json:"image_id"`
    URL         string   `json:"url"`
    OCRText     string   `json:"ocr_text"`
    Description string   `json:"description"`
    Tags        []string `json:"tags"`
    Status      string   `json:"status"` // processing | completed | failed
}

// POST /api/v1/kb/{kbId}/images/search
// 多模态图片检索
type ImageSearchRequest struct {
    Query       string  `json:"query"`        // 文本查询
    ImageURL    string  `json:"image_url"`    // 图片查询（可选）
    TopK        int     `json:"top_k"`
    SearchMode  string  `json:"search_mode"`  // text_to_image | image_to_image | hybrid
}

type ImageSearchResponse struct {
    Results []ImageSearchResult `json:"results"`
    Total   int                 `json:"total"`
}

type ImageSearchResult struct {
    ImageID     string  `json:"image_id"`
    URL         string  `json:"url"`
    Content     string  `json:"content"`
    Score       float64 `json:"score"`
    Metadata    map[string]interface{} `json:"metadata"`
}

// GET /api/v1/kb/{kbId}/images
// 列出知识库中的图片
// GET /api/v1/kb/{kbId}/images/{imageId}
// 获取图片详情
// DELETE /api/v1/kb/{kbId}/images/{imageId}
// 删除图片
```

### 4.5 前端改造方案

#### 4.5.1 新增页面

```
admin/src/app/(admin)/
├─ knowledge-bases/
│   └─ [kbId]/
│       ├─ images/               # 新增：图片管理 Tab
│       │   ├─ page.tsx          # 图片列表（网格视图）
│       │   └─ [imageId]/
│       │       └─ page.tsx      # 图片详情（预览 + OCR + 描述）
│       └─ videos/               # 新增：视频管理 Tab (Phase 2)
│           ├─ page.tsx          # 视频列表
│           └─ [videoId]/
│               └─ page.tsx      # 视频详情（播放 + 转录 + 关键帧）
│
├─ retrieval-lab/
│   ├─ image-search/             # 新增：图片检索实验室
│   │   └─ page.tsx             # 以图搜图 / 文搜图 测试界面
│   └─ multimodal/               # 新增：多模态检索实验室 (Phase 3)
│       └─ page.tsx             # 图文混合检索测试
│
├─ evaluation/
│   └─ datasets/
│       └─ multimodal/           # 新增：多模态评测数据集 (Phase 3)
│
└─ dashboard/
    └─ page.tsx                  # 修改：增加多模态统计卡片
```

#### 4.5.2 核心组件设计

```typescript
// components/admin/multimodal/
// ├─ ImageUploader.tsx         # 图片上传组件（拖拽 + 批量）
// ├─ ImagePreview.tsx          # 图片预览组件（OCR 高亮覆盖）
// ├─ ImageGrid.tsx             # 图片网格展示组件
// ├─ ImageSearchPanel.tsx      # 图片检索面板
// ├─ OCRResultViewer.tsx       # OCR 结果查看器
// ├─ VideoPlayer.tsx           # 视频播放器 (Phase 2)
// ├─ VideoTimeline.tsx         # 视频时间轴（关键帧标注）(Phase 2)
// ├─ TranscriptViewer.tsx      # 转录文本查看器 (Phase 2)
// └─ MultimodalSearchLab.tsx   # 多模态检索实验室 (Phase 3)
```

### 4.6 配置与部署方案

#### 4.6.1 Docker Compose 扩展

```yaml
# 新增服务
services:
  # 现有服务保持不变...

  # 新增：多模态处理微服务
  multimodal-worker:
    build:
      context: ./multimodal-worker
      dockerfile: Dockerfile
    restart: unless-stopped
    ports:
      - "8900:8900"
    environment:
      REDIS_URL: "redis://redis:6379"
      MILVUS_ADDRESS: "milvus:19530"
      MINIO_ENDPOINT: "minio:9000"
      PADDLEOCR_LANG: "ch"
      BGE_VL_MODEL: "BAAI/bge-vl-base"
    depends_on:
      redis:
        condition: service_healthy
      milvus:
        condition: service_healthy
    volumes:
      - multimodal-models:/root/.cache  # 模型缓存
    networks:
      - rag-network
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]  # GPU 加速（可选）

  # 新增：MinIO 对象存储
  minio:
    image: minio/minio:latest
    restart: unless-stopped
    command: server /data --console-address ":9001"
    ports:
      - "9000:9000"
      - "9001:9001"
    environment:
      MINIO_ROOT_USER: ${MINIO_ROOT_USER:-minioadmin}
      MINIO_ROOT_PASSWORD: ${MINIO_ROOT_PASSWORD:-minioadmin}
    volumes:
      - minio-data:/data
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9000/minio/health/live"]
      interval: 30s
      timeout: 10s
      retries: 5
    networks:
      - rag-network

volumes:
  # 现有 volumes 保持不变...
  multimodal-models:
  minio-data:
```

#### 4.6.2 多模态 Worker Dockerfile

```dockerfile
# multimodal-worker/Dockerfile
FROM python:3.11-slim

WORKDIR /app

# 安装系统依赖
RUN apt-get update && apt-get install -y \
    ffmpeg \
    libgl1-mesa-glx \
    && rm -rf /var/lib/apt/lists/*

# 安装 Python 依赖
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# 复制代码
COPY . .

# 预下载模型（可选，或在启动时下载）
# RUN python -c "from paddleocr import PaddleOCR; PaddleOCR()"

EXPOSE 8900

CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8900", "--workers", "2"]
```

```txt
# multimodal-worker/requirements.txt
fastapi==0.115.0
uvicorn==0.32.0
paddleocr==2.9.0
paddlepaddle==3.0.0
torch==2.5.0
transformers==4.46.0
open_clip_torch==2.31.0
pymilvus==2.5.0
minio==7.2.0
redis==5.2.0
python-multipart==0.0.17
pillow==11.0.0
numpy==1.26.0
```

---

## 5. 分阶段实施计划

### 5.1 Phase 1：图片知识库 MVP（6 周）

**目标：** 支持图片上传、OCR、图片描述、图片 Embedding、基本图片检索

#### Week 1-2：基础设施搭建

| 任务 | 负责 | 产出 |
|------|------|------|
| 搭建 Python 微服务框架 (FastAPI) | 后端 | multimimal-worker 骨架代码 |
| 集成 PaddleOCR | 后端 | OCR API 可用 |
| 集成 CLIP/BGE-VL Embedding | 后端 | 图片 Embedding API 可用 |
| 新增 MinIO OSS 实现 | 后端 | `minio_oss.go` |
| Milvus 新增 images collection | 后端 | collection 自动创建 |
| Docker Compose 扩展 | DevOps | minio + worker 服务可启动 |

#### Week 3-4：Go 后端改造

| 任务 | 负责 | 产出 |
|------|------|------|
| config.yaml 新增多模态配置段 | 后端 | 配置结构体扩展 |
| 新增 `internal/multimodal/` 模块 | 后端 | 图片处理服务层 |
| 新增 `api/handler/multimodal/` | 后端 | 图片 CRUD + 检索 API |
| ragqueue 扩展多模态任务类型 | 后端 | 异步图片入库 |
| MilvusManager 初始化 images collection | 后端 | 自动建 collection |

#### Week 5-6：前端开发 + 联调

| 任务 | 负责 | 产出 |
|------|------|------|
| 图片上传组件 (ImageUploader) | 前端 | 拖拽 + 批量上传 |
| 知识库图片管理 Tab | 前端 | 图片列表 + 详情页 |
| OCR 结果查看器 | 前端 | OCR 文本 + 图片预览 |
| 图片检索测试页面 | 前端 | 文搜图基本功能 |
| 端到端联调 + 测试 | 全栈 | MVP 可用 |

**Phase 1 交付物：**
- ✅ 图片上传 + 自动 OCR + 图片描述
- ✅ 图片 Embedding + 写入 Milvus
- ✅ 基本文搜图检索
- ✅ 知识库图片管理界面
- ✅ Docker Compose 一键部署

### 5.2 Phase 2：视频能力（6 周）

**目标：** 支持视频上传、关键帧提取、语音转录、视频片段检索

#### Week 7-8：视频处理基础

| 任务 | 负责 | 产出 |
|------|------|------|
| 集成 FFmpeg 帧提取 | 后端 | 关键帧提取 API |
| 集成 PySceneDetect | 后端 | 智能场景检测 |
| 集成 FunASR/Whisper | 后端 | 语音转录 API |
| Milvus 新增 videos collection | 后端 | collection 自动创建 |

#### Week 9-10：Go 后端视频模块

| 任务 | 负责 | 产出 |
|------|------|------|
| 新增 `internal/multimodal/video/` | 后端 | 视频处理服务层 |
| 新增视频 CRUD + 检索 API | 后端 | 视频管理接口 |
| ragqueue 扩展视频任务 | 后端 | 异步视频入库 |
| 视频 Embedding（帧 + 转录双通道）| 后端 | 视频向量化 |

#### Week 11-12：前端 + 联调

| 任务 | 负责 | 产出 |
|------|------|------|
| 视频上传组件 | 前端 | 视频上传 + 进度展示 |
| 视频播放器组件 | 前端 | 带时间轴的播放器 |
| 关键帧缩略图展示 | 前端 | 关键帧网格 |
| 转录文本查看器 | 前端 | 带时间戳的转录文本 |
| 视频检索页面 | 前端 | 文搜视频片段 |

**Phase 2 交付物：**
- ✅ 视频上传 + 关键帧提取 + 语音转录
- ✅ 视频 Embedding + 写入 Milvus
- ✅ 视频片段语义检索
- ✅ 知识库视频管理界面
- ✅ 视频播放 + 转录回溯

### 5.3 Phase 3：高级多模态检索（6 周）

**目标：** 多模态混合检索、跨模态 Rerank、多模态评测

#### Week 13-14：多模态混合检索

| 任务 | 负责 | 产出 |
|------|------|------|
| 多模态 RRF 融合算法 | 后端 | 文本+图片+视频结果融合 |
| 跨模态 Reranker | 后端 | 跨模态相关性重排序 |
| 统一检索 API | 后端 | 一个接口检索所有模态 |
| 检索结果聚合展示 | 前端 | 多模态结果统一展示 |

#### Week 15-16：多模态检索实验室

| 任务 | 负责 | 产出 |
|------|------|------|
| 多模态检索实验室页面 | 前端 | 支持文本/图片/视频混合查询 |
| 检索结果可视化 | 前端 | 向量空间可视化 |
| 检索策略对比 | 前端 | A/B 策略对比 |
| BGE-VL 升级（替换 CLIP） | 后端 | 更好的中文多模态效果 |

#### Week 17-18：评测 + 优化

| 任务 | 负责 | 产出 |
|------|------|------|
| 多模态评测数据集 | 全栈 | 图文/视频 QA 数据集 |
| 多模态检索评测 pipeline | 后端 | Recall@K, MRR, nDCG |
| 性能优化 | 后端 | 批量 Embedding、缓存优化 |
| 文档编写 | 全栈 | API 文档、部署指南 |

**Phase 3 交付物：**
- ✅ 多模态混合检索（文本+图片+视频）
- ✅ 跨模态 Rerank
- ✅ 多模态检索实验室
- ✅ 多模态评测系统
- ✅ 性能优化 + 完整文档

---

## 6. 风险与成本评估

### 6.1 性能影响分析

| 操作 | 当前耗时 | 多模态后耗时 | 影响 |
|------|----------|--------------|------|
| 文档入库（文本） | ~500ms/chunk | 不变 | 无影响 |
| 图片入库（OCR+Embed） | N/A | ~2-5s/张 | 异步处理，不阻塞 |
| 视频入库（帧提取+转录） | N/A | ~30-120s/分钟 | 异步处理，不阻塞 |
| 文本检索 | ~100-300ms | 不变 | 无影响 |
| 图片检索 | N/A | ~150-400ms | 独立 collection |
| 多模态混合检索 | N/A | ~300-800ms | 多路召回 + 融合 |
| Milvus 内存占用 | 基线 | +30-50% | 图片向量额外内存 |

**关键结论：** 多模态处理通过异步队列化，不影响现有文本检索性能。检索延迟增加在可接受范围内。

### 6.2 月度成本估算

#### 6.2.1 基础设施成本（自部署方案）

| 资源 | 规格 | 月成本（人民币） | 说明 |
|------|------|-----------------|------|
| GPU 服务器（推理） | 1x RTX 4090 / A10 | ¥2,000-5,000 | OCR + Embedding 推理 |
| MinIO 存储 | 500GB 起步 | ¥100-300 | 图片/视频存储 |
| Milvus 额外内存 | +8GB RAM | ¥200-500 | 多 collection 向量 |
| Python Worker 计算 | 4C8G | ¥500-1,000 | 微服务运行 |
| **合计** | | **¥2,800-6,800** | |

#### 6.2.2 API 调用方案成本（按 10,000 张图片/月）

| 服务 | 单价 | 月成本 | 说明 |
|------|------|--------|------|
| Qwen-VL 图片描述 | ¥0.01/张 | ¥100 | 图片理解 |
| BGE-VL Embedding (API) | ¥0.001/张 | ¥10 | 图片向量化 |
| Whisper API 转录 | ¥0.04/分钟 | ¥400 | 10,000 分钟视频 |
| **合计** | | **¥510** | 纯 API 方案 |

#### 6.2.3 混合方案成本（推荐）

| 组件 | 方案 | 月成本 |
|------|------|--------|
| OCR | PaddleOCR 自部署 | ¥0（含在 GPU 服务器） |
| 图片描述 | Qwen-VL API | ¥100 |
| 图片 Embedding | BGE-VL 自部署 | ¥0（含在 GPU 服务器） |
| 视频转录 | FunASR 自部署 | ¥0（含在 GPU 服务器） |
| GPU 服务器 | 1x RTX 4090 | ¥3,000 |
| MinIO + 附加存储 | 500GB | ¥200 |
| **合计** | | **¥3,300** |

### 6.3 已知限制与风险

| 风险 | 等级 | 说明 | 缓解措施 |
|------|------|------|----------|
| **GPU 资源瓶颈** | 🔴 高 | PaddleOCR/BGE-VL 需要 GPU，高并发下可能排队 | 批量处理 + 水平扩展 + CPU fallback |
| **图片质量依赖** | 🟡 中 | OCR 准确率依赖图片清晰度 | 预处理增强 + 置信度阈值 |
| **Milvus 内存增长** | 🟡 中 | 多 collection 增加内存消耗 | 分 collection 策略 + 资源监控 |
| **模型下载体积** | 🟡 中 | BGE-VL ~1.5GB, PaddleOCR ~500MB | Docker 预构建镜像 + 模型缓存卷 |
| **视频处理耗时** | 🟡 中 | 长视频处理可能需要数分钟 | 分段处理 + 进度通知 + 优先级队列 |
| **中文 Embedding 效果** | 🟡 中 | CLIP 中文效果不如 BGE-VL | Phase 1 用 CLIP 快速上线，Phase 3 升级 BGE-VL |
| **MinIO 运维复杂度** | 🟢 低 | 新增组件需要运维 | Docker Compose 部署 + 定期备份 |
| **Python-Go 跨语言调用** | 🟢 低 | HTTP 调用增加延迟 | 内网通信 + 连接池 + 超时控制 |
| **数据安全** | 🟡 中 | 图片/视频可能含敏感信息 | MinIO 加密 + 访问控制 + OCR 脱敏 |

### 6.4 缓解措施

#### 性能优化策略

1. **批量处理：** 图片 Embedding 支持批量请求，减少 API 调用次数
2. **异步队列：** 所有多模态处理通过 Redis Stream 异步执行
3. **缓存策略：** OCR 结果和 Embedding 结果缓存到 Redis，避免重复计算
4. **模型量化：** BGE-VL 使用 INT8 量化，减少 GPU 内存占用 50%
5. **分 collection：** 图片/视频独立 collection，不影响文本检索性能

#### 可靠性保障

1. **重试机制：** 多模态任务支持自动重试（复用现有 `enable_ingest_retry` 机制）
2. **降级策略：** GPU 不可用时降级为 CPU 推理（速度慢但可用）
3. **监控告警：** 多模态处理队列积压、Embedding 失败率等指标告警
4. **回滚方案：** 多模态功能通过 feature flag 控制，可随时关闭

---

## 7. 附录

### 7.1 检查过的关键文件列表

| 文件 | 说明 | 关键发现 |
|------|------|----------|
| `README.md` | 项目说明 | 确认为独立 RAG 平台，Go + Next.js 技术栈 |
| `backend/config.yaml` | 主配置文件 | Embedding 支持 ark/openai，Milvus 单 collection "documents" |
| `docker-compose.yml` | 容器编排 | 5 个服务：rag-server、mysql、redis、milvus、attu |
| `backend/internal/config/config.go` | 配置结构体 | 完整的 RAG 特性 flag 体系，无多模态配置 |
| `backend/internal/milvus/init.go` | Milvus 管理器 | 全局单例，7 步初始化流程 |
| `backend/internal/milvus/storage/indexer.go` | 索引器服务 | FloatVector + COSINE，4 个字段（id/vector/content/metadata） |
| `backend/internal/milvus/storage/embedding.go` | Embedding 服务 | 支持 ark/openai，仅文本输入 |
| `backend/internal/milvus/retrieval/hybrid_search.go` | 混合检索 | Dense+Sparse+Rerank+Rewrite 完整流水线 |
| `backend/internal/storage/oss.go` | OSS 接口 | 通用接口，PutObject/GetObject/DeleteObject/GetURL |
| `backend/internal/storage/local_oss.go` | 本地 OSS | 文件系统实现 |
| `backend/internal/ragqueue/queue.go` | 队列接口 | Redis Stream 实现 |
| `backend/internal/ragqueue/consumer.go` | 队列消费者 | 文档入库消费逻辑 |
| `admin/src/app/(admin)/` | 前端页面 | 25+ 管理页面，无多模态相关页面 |

### 7.2 参考资源

**多模态 Embedding：**
- BGE-VL: https://github.com/FlagOpen/FlagEmbedding
- Jina CLIP v2: https://jina.ai/embeddings/
- OpenCLIP: https://github.com/mlfoundations/open_clip

**OCR 方案：**
- PaddleOCR: https://github.com/PaddlePaddle/PaddleOCR
- MinerU: https://github.com/opendatalab/MinerU

**视频处理：**
- PySceneDetect: https://github.com/Breakthrough/PySceneDetect
- FunASR: https://github.com/modelscope/FunASR
- Whisper: https://github.com/openai/whisper

**Milvus 多向量：**
- Milvus hybrid_search: https://milvus.io/docs/multi-vector-search.md
- Milvus 多向量字段: https://milvus.io/docs/multivector.md

**对象存储：**
- MinIO: https://min.io/
- MinIO Go SDK: https://github.com/minio/minio-go

### 7.3 术语表

| 术语 | 说明 |
|------|------|
| **RAG** | Retrieval-Augmented Generation，检索增强生成 |
| **CLIP** | Contrastive Language-Image Pre-training，OpenAI 的图文对比学习模型 |
| **BGE-VL** | 智源研究院的多模态 Embedding 模型 |
| **OCR** | Optical Character Recognition，光学字符识别 |
| **VLM** | Vision-Language Model，视觉语言模型 |
| **ANN** | Approximate Nearest Neighbor，近似最近邻搜索 |
| **RRF** | Reciprocal Rank Fusion，倒数排名融合 |
| **HNSW** | Hierarchical Navigable Small World，Milvus 向量索引类型 |
| **FloatVector** | 浮点向量，Milvus 向量字段类型 |
| **Hybrid Search** | 混合检索，结合 Dense + Sparse 向量的检索方式 |
| **Rerank** | 重排序，对初步检索结果进行精细化排序 |
| **Parent-Child** | 父子检索，将文档分为父子层级进行检索 |
| **Evidence Gate** | 证据拒答，当检索证据不足时拒答而非胡说 |
| **FunASR** | 阿里达摩院开源的语音识别工具包 |
| **PySceneDetect** | Python 视频场景检测库 |

---

> **文档版本：** v1.0  
> **最后更新：** 2026-06-09  
> **状态：** 待评审
