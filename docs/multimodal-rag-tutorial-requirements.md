# 多模态 RAG（图像/视频检索）开发教程 — 需求文档

> 版本：v1.0 | 日期：2026-06-04 | 作者：AutoClaw

---

## 1. 文档目的

本文档定义一份**面向开发者的多模态 RAG 教程**的需求，覆盖从"什么是多模态 RAG"到"我能跑通一个完整的图像/视频检索 Demo"的完整学习路径。教程需基于本仓库现有的 RAG 平台架构（Hertz + Eino + Milvus），延伸至多模态场景。

---

## 2. 目标读者

| 画像 | 描述 |
|------|------|
| **后端工程师** | 有 Go 基础，了解 REST API 和数据库，但不熟悉向量检索 |
| **AI 应用开发者** | 用过 LLM API，搭过文本 RAG，想扩展到图像/视频 |
| **全栈开发者** | 想从零搭建一个端到端的多模态知识库系统 |

**前置知识**：Go 基础、HTTP API 开发、基本 SQL、了解向量嵌入概念（不要求精通）

---

## 3. 教程目标

学完本教程后，读者能：

1. **理解**多模态 RAG 的核心概念与架构差异（相比纯文本 RAG）
2. **搭建**图像/视频向量化管线（Embedding + 存储 + 索引）
3. **实现**跨模态检索（文本查图、图查图、视频关键帧检索）
4. **集成**到现有 RAG 平台，复用已有的混合检索、重排序、证据门控等能力
5. **调优**检索质量（多模态融合权重、跨模态对齐、检索评估）

---

## 4. 教程结构设计

### 4.1 章节大纲

| 章节 | 标题 | 核心内容 | 产出物 |
|------|------|---------|--------|
| **Ch.1** | 多模态 RAG 概述 | 什么是多模态 RAG；与文本 RAG 的区别；典型应用场景（电商图搜、视频知识库、医学影像检索） | 架构对比图 |
| **Ch.2** | 多模态嵌入模型选型 | CLIP / SigLIP / Jina CLIP / EVA-CLIP 对比；API 服务 vs 本地部署；模型评估指标（Recall@K、MRR） | 模型选型决策树 |
| **Ch.3** | 图像处理管线 | 图像预处理（缩放/裁剪/归一化）；批量 Embedding 调用；向量存储到 Milvus（Collection 设计、Schema 定义） | 可运行的图像入库脚本 |
| **Ch.4** | 视频处理管线 | 视频关键帧提取（FFmpeg/场景切换）；关键帧去重；时间戳元数据管理；帧级 vs 片段级索引策略 | 可运行的视频入库脚本 |
| **Ch.5** | 跨模态检索实现 | 文本→图像检索；图像→图像检索；混合检索（文本+视觉特征融合）；Milvus 多向量字段查询 | 完整的检索 API |
| **Ch.6** | 集成到现有 RAG 平台 | 扩展 `EmbeddingService` 支持多模态模型；Milvus 多模态 Collection 设计；检索路由（自动判断模态）；与已有 Hybrid Search、Reranker、Evidence Gate 的衔接 | 可合并的代码改动 |
| **Ch.7** | 前端交互 | 图片上传与预览；检索结果画廊展示；视频时间线定位与播放；管理后台多模态知识库管理 | 前端组件代码 |
| **Ch.8** | 评估与调优 | 多模态检索评估数据集构建；跨模态 Recall@K / nDCG 评测；融合权重搜索；常见 Bad Case 分析 | 评测脚本 + 基线报告 |
| **Ch.9** | 生产部署 | 大规模图像/视频入库的吞吐优化；GPU 推理服务部署；冷热数据分层；成本估算 | 部署清单 + 配置模板 |

### 4.2 教程风格要求

- **代码先行**：每节先给可运行代码，再解释原理，避免大段理论堆砌
- **渐进式**：从最小可跑的 Demo → 工程化改造 → 生产级优化，层层递进
- **贴合本项目**：基于仓库现有的 Hertz + Eino + Milvus 架构，直接复用已有模块（`EmbeddingService`、`HybridSearch`、`Reranker`），不做从零造轮子
- **中文撰写**：注释和正文用中文，代码变量名和 API 用英文
- **Bad Case 驱动**：每章至少 1 个"常见踩坑 + 修复"的实战案例

---

## 5. 功能性需求

### 5.1 图像检索

| ID | 需求 | 优先级 | 验收标准 |
|----|------|--------|---------|
| IMG-01 | 支持图像文件上传（JPG/PNG/WebP），生成向量并写入 Milvus | P0 | 上传 1 张图，能在 Milvus 中查到对应向量 |
| IMG-02 | 支持批量图像入库（100+ 张/次），含进度回调 | P1 | 批量上传无报错，进度事件正确推送 |
| IMG-03 | 文本→图像检索（输入文字描述，返回相似图像 Top-K） | P0 | "红色连衣裙"能召回含红裙的图片，Recall@5 ≥ 0.6 |
| IMG-04 | 图像→图像检索（以图搜图） | P0 | 上传参考图，返回视觉相似图片，Top-1 准确率 ≥ 0.7 |
| IMG-05 | 图像元数据管理（标签、描述、来源、时间） | P1 | 元数据随向量同步存储，支持过滤查询 |
| IMG-06 | 图像预处理（缩放、格式转换、EXIF 清理） | P1 | 预处理不丢失关键视觉信息，处理耗时 < 500ms/张 |

### 5.2 视频检索

| ID | 需求 | 优先级 | 验收标准 |
|----|------|--------|---------|
| VID-01 | 视频上传后自动提取关键帧（基于场景切换） | P0 | 1 分钟视频提取 5-15 帧，无重复帧 |
| VID-02 | 关键帧向量化 + 时间戳元数据写入 Milvus | P0 | 帧向量含 `video_id`、`timestamp` 元数据，可按视频/时间范围过滤 |
| VID-03 | 文本→视频片段检索 | P0 | "海边日落"能定位到对应视频片段（±3s 内） |
| VID-04 | 视频时间线展示与跳转 | P1 | 检索结果可点击跳转到视频对应时间点播放 |
| VID-05 | 长视频分段索引（>10min 视频自动分 Chapter） | P2 | 分段信息入库，检索结果标注所属 Chapter |

### 5.3 跨模态融合

| ID | 需求 | 优先级 | 验收标准 |
|----|------|--------|---------|
| FUS-01 | 混合检索：文本特征 + 视觉特征加权融合 | P0 | 融合检索 mAP 比单模态提升 ≥ 10% |
| FUS-02 | 自动模态路由（根据 query 自动选择检索模态） | P1 | 含图像 URL 的 query 自动触发视觉检索，纯文本走文本链路 |
| FUS-03 | 跨模态 Reranker（对多模态候选集统一重排序） | P1 | 重排序后 Recall@10 提升 ≥ 5% |
| FUS-04 | 支持多模态 Evidence Gate（过滤低质量多模态结果） | P2 | 无关视觉内容过滤率 ≥ 80%，误滤率 < 10% |

### 5.4 平台集成

| ID | 需求 | 优先级 | 验收标准 |
|----|------|--------|---------|
| INT-01 | 扩展现有 `EmbeddingService`，新增 `multimodal` Provider | P0 | 配置切换 `provider: "clip"` 即可启用多模态嵌入 |
| INT-02 | Milvus 新增多模态 Collection Schema（含图像/视频字段） | P0 | Schema 定义文档 + 自动 Migration 脚本 |
| INT-03 | 复用已有 `HybridSearch` 实现多模态混合检索 | P1 | 无需重写检索逻辑，仅扩展查询构造 |
| INT-04 | 管理后台支持多模态知识库管理 | P1 | 可上传图片/视频、查看检索结果、管理 Collection |
| INT-05 | 与现有 API Key / 租户体系兼容 | P1 | 多模态 API 使用同一套认证鉴权 |

---

## 6. 非功能性需求

| 维度 | 要求 |
|------|------|
| **性能** | 单张图像 Embedding 延迟 < 200ms（API）/ < 100ms（本地 GPU）；检索 P99 < 500ms（10 万向量规模） |
| **可扩展性** | 支持 100 万+ 图像向量、10 万+ 视频帧向量；Milvus Collection 可水平扩展 |
| **成本** | 提供按需 vs 批量 Embedding 的成本对比；API 调用费用估算模板 |
| **兼容性** | 向后兼容现有纯文本 RAG 流程；多模态为可选增强，不影响已有功能 |
| **可观测性** | 关键指标（Embedding 耗时、检索延迟、缓存命中率）接入已有 Prometheus + Grafana 体系 |
| **安全** | 图像/视频上传需校验文件类型和大小；防止通过恶意图片触发 SSRF |

---

## 7. 技术选型建议

| 组件 | 推荐方案 | 备选方案 | 理由 |
|------|---------|---------|------|
| **多模态 Embedding** | CLIP ViT-L/14（OpenAI API 或自部署） | SigLIP、Jina CLIP v2 | 生态最成熟，文本-图像对齐质量高 |
| **图像预处理** | Go `imaging` + `bimg`（libvips 绑定） | FFmpeg（图像模式） | 纯 Go，无 CGO 依赖，轻量 |
| **视频关键帧提取** | FFmpeg（场景切换检测） | PySceneDetect（gRPC 调用） | 通用性最好，Go 可直接 exec 调用 |
| **向量数据库** | Milvus 2.4+（已有） | — | 复用现有基础设施 |
| **GPU 推理服务** | Triton Inference Server | TorchServe、vLLM（多模态） | 生产级，支持多模型、批处理 |
| **对象存储** | MinIO（自部署）/ S3 | OSS / COS | 存原始图像/视频文件，Milvus 只存向量 |

---

## 8. Milvus Schema 设计（草案）

```python
# 图像 Collection
image_collection = {
    "name": "multimodal_images",
    "fields": [
        {"name": "id", "dtype": "INT64", "is_primary": True, "auto_id": True},
        {"name": "image_id", "dtype": "VARCHAR", "max_length": 64},
        {"name": "vector", "dtype": "FLOAT_VECTOR", "dim": 768},        # CLIP ViT-L/14
        {"name": "description", "dtype": "VARCHAR", "max_length": 1024}, # 图像描述（可选）
        {"name": "tags", "dtype": "VARCHAR", "max_length": 512},        # 逗号分隔标签
        {"name": "source", "dtype": "VARCHAR", "max_length": 256},      # 来源标识
        {"name": "kb_id", "dtype": "INT64"},                             # 知识库 ID
        {"name": "tenant_id", "dtype": "INT64"},                         # 租户 ID
        {"name": "created_at", "dtype": "INT64"},                        # Unix 时间戳
    ],
    "index": {
        "field": "vector",
        "type": "IVF_FLAT",
        "metric_type": "COSINE",
        "params": {"nlist": 1024}
    }
}

# 视频关键帧 Collection
video_frame_collection = {
    "name": "multimodal_video_frames",
    "fields": [
        {"name": "id", "dtype": "INT64", "is_primary": True, "auto_id": True},
        {"name": "frame_id", "dtype": "VARCHAR", "max_length": 64},
        {"name": "video_id", "dtype": "VARCHAR", "max_length": 64},
        {"name": "vector", "dtype": "FLOAT_VECTOR", "dim": 768},
        {"name": "timestamp_ms", "dtype": "INT64"},                      # 帧在视频中的时间位置
        {"name": "chapter", "dtype": "VARCHAR", "max_length": 128},      # 所属章节（可选）
        {"name": "kb_id", "dtype": "INT64"},
        {"name": "tenant_id", "dtype": "INT64"},
        {"name": "created_at", "dtype": "INT64"},
    ],
    "index": {
        "field": "vector",
        "type": "IVF_FLAT",
        "metric_type": "COSINE",
        "params": {"nlist": 1024}
    }
}
```

---

## 9. API 设计（草案）

### 9.1 图像入库

```
POST /v1/multimodal/images/upload
Content-Type: multipart/form-data

Body:
  file: <image binary>
  kb_id: 1
  description: "产品主图 - 红色连衣裙"
  tags: "服装,连衣裙,红色"

Response:
{
  "code": 200,
  "data": {
    "image_id": "img_a1b2c3",
    "vector_dim": 768,
    "status": "indexed"
  }
}
```

### 9.2 视频入库

```
POST /v1/multimodal/videos/upload
Content-Type: multipart/form-data

Body:
  file: <video binary>
  kb_id: 1
  title: "产品演示视频"
  extract_mode: "scene_change"  # scene_change | fixed_interval
  interval_seconds: 5           # fixed_interval 模式参数

Response:
{
  "code": 200,
  "data": {
    "video_id": "vid_x1y2z3",
    "frames_extracted": 12,
    "status": "indexed"
  }
}
```

### 9.3 跨模态检索

```
POST /v1/multimodal/retrieve
Content-Type: application/json

Body:
{
  "query": "海边的日落风景",
  "modalities": ["image", "video"],
  "top_k": 10,
  "kb_id": 1,
  "fusion_weights": {
    "text": 0.4,
    "visual": 0.6
  },
  "image_url": "https://example.com/ref.jpg"  // 可选，触发以图搜图
}

Response:
{
  "code": 200,
  "data": {
    "results": [
      {
        "type": "image",
        "image_id": "img_a1b2c3",
        "score": 0.92,
        "url": "/storage/img_a1b2c3.jpg",
        "description": "海边日落",
        "metadata": {"tags": "风景,日落,海边"}
      },
      {
        "type": "video_frame",
        "video_id": "vid_x1y2z3",
        "frame_id": "frame_007",
        "timestamp_ms": 14500,
        "score": 0.88,
        "url": "/storage/vid_x1y2z3/frame_007.jpg",
        "video_url": "/storage/vid_x1y2z3/video.mp4#t=14.5"
      }
    ],
    "request_id": "req_abc123",
    "latency_ms": 342
  }
}
```

---

## 10. 代码组织建议

在现有仓库结构中，多模态模块建议如下扩展：

```
backend/
  api/handler/
    multimodal/               # 新增：多模态 API Handler
      image.go                #   图像上传/检索
      video.go                #   视频上传/检索
      retrieve.go             #   跨模态检索
  internal/
    milvus/
      storage/
        multimodal_embedding.go  # 新增：多模态 Embedding 服务
      retrieval/
        multimodal_search.go     # 新增：跨模态检索逻辑
        multimodal_fusion.go     # 新增：多模态融合策略
    multimodal/                  # 新增：多模态处理模块
      image_preprocess.go        #   图像预处理
      frame_extractor.go         #   视频关键帧提取
      modality_router.go         #   模态自动路由
  api/ragrouter/
    register.go                  # 修改：注册多模态路由

admin/
  src/components/admin/
    multimodal/                  # 新增：前端多模态组件
      ImageUploader.tsx
      VideoUploader.tsx
      ResultGallery.tsx
      VideoTimeline.tsx
```

---

## 11. 交付物清单

| # | 交付物 | 格式 | 说明 |
|---|--------|------|------|
| 1 | 教程正文（Ch.1 - Ch.9） | Markdown | 每章含原理讲解 + 可运行代码 + 练习题 |
| 2 | 示例代码仓库 | Go + TypeScript | 可直接 `go run` / `npm run dev` 跑通 |
| 3 | 示例数据集 | 图片 + 短视频 | 50 张图片 + 5 段短视频（含标注） |
| 4 | Milvus Migration 脚本 | Go | 自动创建多模态 Collection |
| 5 | 评测脚本 + 基线报告 | Go + Markdown | Recall@K、MRR、mAP 指标 |
| 6 | Docker Compose 扩展 | YAML | 新增 GPU 推理服务、MinIO |
| 7 | 架构图 | PNG/SVG | 多模态 RAG 全链路架构图 |

---

## 12. 验收标准

### 必须达成（P0）

- [ ] 教程全部章节可运行，无断链
- [ ] 文本→图像检索 Demo 可跑通，Recall@5 ≥ 0.6
- [ ] 以图搜图 Demo 可跑通，Top-1 准确率 ≥ 0.7
- [ ] 视频关键帧提取 + 文本→视频检索可跑通
- [ ] 代码可集成到现有 RAG 平台，不影响已有文本检索功能

### 应该达成（P1）

- [ ] 混合检索融合权重可配置，融合效果优于单模态
- [ ] 管理后台可上传图片/视频并查看检索结果
- [ ] 评测脚本产出可读报告，含 Bad Case 分析

### 可选达成（P2）

- [ ] 自动模态路由（根据 query 自动选择检索策略）
- [ ] 长视频 Chapter 级索引
- [ ] GPU 推理服务部署方案

---

## 13. 风险与依赖

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| 多模态 Embedding 模型 API 不稳定/贵 | 阻塞开发与教程体验 | 提供本地部署方案（ONNX Runtime）作为 fallback |
| 视频处理依赖 FFmpeg，Windows 兼容性差 | 读者环境不一致 | 教程默认使用 Docker 环境，保证一致性 |
| Milvus 多向量字段查询性能 | 大规模数据下检索慢 | 提供 IVF_PQ 量化索引方案；冷热分层 |
| CLIP 模型对中文理解较弱 | 中文检索效果差 | 提供中文 CLIP（Chinese-CLIP）替代方案章节 |

---

## 14. 时间线建议

| 阶段 | 内容 | 工期 |
|------|------|------|
| **Phase 1** | Ch.1-3（概述 + 模型选型 + 图像管线） | 3 天 |
| **Phase 2** | Ch.4-5（视频管线 + 跨模态检索） | 3 天 |
| **Phase 3** | Ch.6-7（平台集成 + 前端） | 4 天 |
| **Phase 4** | Ch.8-9（评估调优 + 生产部署） | 3 天 |
| **Review** | 全文审校 + 示例代码验证 | 2 天 |

**总计**：约 15 个工作日

---

_本文档为需求定义，后续可根据实际情况调整优先级和范围。_
