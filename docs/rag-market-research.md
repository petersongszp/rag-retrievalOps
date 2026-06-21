# 当前市场先进 RAG 技术调研与项目对比分析

> 适用对象：产品评审、技术规划会、架构分享  
> 调研日期：2026-06-21  
> 项目：`rag-retrievalOps`

---

## 1. 报告结论先看

### 1.1 一句话判断

你们当前系统已经不是“基础 RAG”，而是已经具备企业级 RAG 平台雏形：**混合检索、评测门禁、灰度治理、缓存、审计、监控**这几层做得明显比很多只强调“检索效果”的开源方案更完整。

当前最大短板不在“平台治理”，而在**高精度召回与复杂问题处理能力**，具体体现为：

1. **重排序能力偏弱**
   当前以 `Jaccard Reranker` 为主，和市场主流的 `Cross-Encoder / BGE-Reranker / Cohere Rerank` 还有明显差距。
2. **检索控制器不够智能**
   已有动态 `TopK`、查询改写、Evidence Gate，但还没有形成完整的 `Self-RAG / CRAG / Adaptive RAG` 控制闭环。
3. **复杂知识组织能力不足**
   还没有 `Graph RAG`、`Propositions`、`Multi-Vector Retrieval`，对多跳推理、事实级检索、图文混合检索的支持不足。
4. **文档增强还停留在“轻上下文”阶段**
   已有 `embedding_content = [Document] + [Section] + [Chunk]`，这是很好的基础，但距离 Anthropic 那类完整 `Contextual Retrieval` 还有一步。
5. **评测体系强，但标准生态接入不够**
   你们自研 L0-L7 很强，治理门禁也很强，但和 `RAGAS / DeepEval / Phoenix / Langfuse` 这类外部标准生态的接合度还不够高。

### 1.2 当前优先级建议

最值得优先补强的 5 个方向：

1. **先进重排序级联**
   最高 ROI，改造成本可控，最容易直接提升 nDCG、MRR、引用质量。
2. **Adaptive / Self / CRAG 轻量控制器**
   在你们现有动态 `TopK`、Evidence Gate、Query Rewrite 基础上最容易升级。
3. **轻量 Contextual Retrieval + Proposition 实验索引**
   直接提升孤立 chunk 可检索性和事实级召回能力。
4. **标准化评测与可观测性**
   把现有自研评测优势升级为“可持续优化飞轮”。
5. **Multi-Vector / GraphRAG 实验轨**
   用实验车道先验证复杂问题收益，不建议直接主链路替换。

### 1.3 你们当前最强的优势

和市场上很多“会检索、不会治理”的 RAG 系统相比，你们的优势非常清楚：

1. **平台化能力强**
   已有策略中心、灰度发布、回滚、门禁、告警、审计，这些是企业落地最难补的部分。
2. **检索链路已经模块化**
   已有 Dense + Sparse、RRF / MinMax、动态 `TopK`、Query Rewrite、Evidence Gate，后续升级空间很好。
3. **文档切分基础扎实**
   已有结构感知切片、语义二次切分、Parent-Child 检索补全、上下文化嵌入。
4. **评测意识成熟**
   很多团队先上功能、后补评测；你们已经有 baseline/candidate、Gate、延迟/成本/一致性监控，这非常难得。

---

## 2. 项目当前能力定位

### 2.1 当前能力总览

| 层 | 当前能力 | 市场定位判断 |
| --- | --- | --- |
| 文档处理 | 结构感知切片、语义二次切分、上下文化嵌入、Parent-Child | 高于多数基础 RAG |
| 检索 | Dense + Sparse、自建 BM25、RRF / MinMax、动态 TopK、Query Rewrite | 已达成熟工程化水平 |
| 生成安全 | Evidence Gate、Citation Consistency | 强于多数开源默认方案 |
| 缓存 | 语义缓存 L0-L6 | 有明显工程优势 |
| 评测 | L0-L7、baseline/candidate、Gate | 企业级能力 |
| 治理 | 策略中心、灰度、回滚、审计、监控、告警 | 企业级能力 |
| 扩展 | MCP、多模态调研、Loop Engineering 文档 | 有扩展接口，但未形成新主能力 |

### 2.2 与市场先进方案相比的总体判断

如果把当前先进 RAG 能力拆成四层：

1. **召回层**
   你们已经达到主流水平，但还没进入 `Cross-Encoder / Multi-Vector / GraphRAG` 这类前沿区。
2. **控制层**
   你们已经有“轻控制器”，但还没形成真正的自适应检索决策系统。
3. **知识表示层**
   你们有结构化 chunk，但还没有命题层、图谱层、多向量层。
4. **优化飞轮层**
   你们治理和门禁很强，但缺少与行业标准评测、Tracing、在线反馈闭环的深度耦合。

---

## 3. 检索技术调研

## 3.1 Advanced Reranking

### 原理简述

先进重排序的核心思路是：**先用高召回的检索器找候选，再用更贵但更准的模型做二次排序**。

主流路线有三类：

1. **Cross-Encoder**
   把 `query + chunk` 一起送进模型，直接输出相关性分数，准确率高，但成本和延迟更高。
2. **BGE-Reranker**
   BAAI 提供的通用重排序模型，覆盖中英、多语和不同尺寸，部署灵活，开源友好。
3. **Cohere Rerank**
   商业 API 方案，效果稳定，接入快，适合先验证收益再决定是否自建。

### 代表产品 / 开源项目

1. `Cross-Encoder`：Hugging Face `cross-encoder/ms-marco-*`
2. `BGE-Reranker`：`BAAI/bge-reranker-v2-m3`
3. `Cohere Rerank`：Cohere 官方 `Rerank`

### 与我们项目的对比

| 项 | 现状 |
| --- | --- |
| 候选召回 | **已实现** |
| 融合召回 | **已实现** |
| 轻量重排（Jaccard） | **已实现** |
| Cross-Encoder / BGE / Cohere 级精排 | **部分实现**，架构可扩展，但未形成线上主能力 |

### 差距分析

1. 现在的 `Jaccard Reranker` 更像启发式排序，不是语义级精排。
2. 缺少 `TopN -> Cross-Encoder` 的标准级联链路。
3. 没有“重排效果收益 vs 延迟/成本”的在线灰度评估闭环。
4. 对复杂查询、长 chunk、跨句推断的精度仍受限。

### 落地建议

1. 第一阶段直接上 **两段式重排**
   `Hybrid Recall Top50 -> Cross-Encoder/BGE Rerank Top10`。
2. 优先顺序建议：
   `BGE-Reranker 本地化` > `Cohere Rerank Shadow` > `全量自建 Cross-Encoder 服务`
3. 先走影子流量：
   不改线上答案，只记录 `MRR/nDCG/引用一致性/延迟/成本`。
4. 在策略中心新增：
   `rerank_provider`、`rerank_topn`、`rerank_timeout_ms`、`rerank_fallback_mode`。
5. 失败自动降级到现有 `Jaccard`，不要把精排变成单点风险。

### 判断

**优先级：P0**  
**投入产出比：最高**

---

## 3.2 Multi-Vector Retrieval

### 原理简述

传统向量检索通常是“一个 chunk 一个向量”。Multi-Vector Retrieval 则允许一个文档、段落或页面拥有**多个向量表示**。

两条最典型路线：

1. **ColBERT**
   保留 token 级向量，用 `late interaction / MaxSim` 做匹配，兼顾召回与精度。
2. **ColPali / ColQwen 系**
   面向文档页面图像的视觉检索，不依赖纯 OCR 文本，可直接检索复杂版式、表格、图文页面。

### 代表产品 / 开源项目

1. `ColBERT`
2. `ColPali`
3. `Byaldi / Vidore / ColQwen2` 生态

### 与我们项目的对比

| 项 | 现状 |
| --- | --- |
| 单向量 chunk 检索 | **已实现** |
| Parent-Child 上下文补全 | **已实现** |
| Multi-Vector Token Interaction | **未实现** |
| 页面图像级检索 | **未实现** |

### 差距分析

1. 当前索引单元仍是“chunk 级单向量”，对细粒度短语匹配、多词对齐能力有限。
2. 多页 PDF、图文混排、表格、扫描件等场景，纯文本向量效果会快速衰减。
3. 现有 Milvus 方案适合存 chunk 向量，但还没有 `token-vector` 或 `page-image-vector` 的索引组织方案。

### 落地建议

1. 不建议一上来主链路替换。
   先在 `retrieval-lab` 做 **实验车道**。
2. 文本多向量优先做：
   `ColBERTv2` 实验索引，仅用于高价值知识库或复杂问答集。
3. 多模态多向量优先做：
   `ColPali / ColQwen2` 页面检索微服务，作为 PDF 高难场景专用策略。
4. 指标重点看：
   `Recall@20`、`nDCG@10`、表格问答成功率、复杂页面问答命中率。
5. 基础设施上建议单独做 Python 服务，不要把 token 级推理塞进 Go 主进程。

### 判断

**优先级：P1**  
**更适合实验轨，不适合立刻主链路替换**

---

## 3.3 Graph RAG

### 原理简述

Graph RAG 的核心不是“再加一个数据库”，而是把知识从“chunk 集合”升级为**实体-关系-社区摘要**。

典型流程：

1. 从文档中抽取实体、关系、事件
2. 构建知识图或属性图
3. 为社区、子图、路径生成摘要
4. 查询时结合向量召回 + 图遍历 + 社区摘要回答复杂问题

它特别适合：

1. 多跳推理
2. 跨文档关系追踪
3. 原因链、人物关系、制度依赖、系统依赖分析

### 代表产品 / 开源项目

1. `Microsoft GraphRAG`
2. `Neo4j + Vector` 混合方案
3. `LlamaIndex KnowledgeGraph / Property Graph` 生态

### 与我们项目的对比

| 项 | 现状 |
| --- | --- |
| 向量检索 | **已实现** |
| 文档结构层级 | **已实现** |
| 实体关系抽取 | **未实现** |
| 图索引 / 图遍历 / 社区摘要 | **未实现** |

### 差距分析

1. 当前系统更擅长“局部证据召回”，不擅长“跨段、跨文档关系推理”。
2. 面对“某制度如何影响 A、B、C 三个流程”这类问题，仅靠 chunk 检索容易漏掉中间关系。
3. GraphRAG 引入成本高，且实体抽取质量直接决定收益。

### 落地建议

1. 不建议直接上重型图数据库主链路。
2. 第一阶段可做 **轻量图侧车**
   在 MySQL 中先存 `entity`、`relation`、`doc_entity_map` 三类表。
3. 第二阶段再上：
   `子图扩展检索 -> 社区摘要 -> 和向量召回融合`
4. GraphRAG 更适合的首批场景：
   组织架构、制度流程、产品依赖、项目关系、代码知识图谱。
5. 要求严格门禁：
   图谱质量不过关时，必须能回退到现有向量主链路。

### 判断

**优先级：P2**  
**是战略能力，不是短期最高 ROI 功能**

---

## 3.4 Hybrid Search 最新实践

### 原理简述

当前市场上最有效的 Hybrid Search 已经不是简单的“BM25 + Dense 拼一下”，而是以下四点组合：

1. **多路召回**
   Dense、BM25、Sparse Expansion、Metadata Filter、Multi-Query 并行。
2. **分数归一与融合**
   `RRF`、`Relative Score Fusion`、`MinMax`、加权融合。
3. **查询自适应**
   根据查询类型动态调整 BM25、Dense、Sparse 权重。
4. **二阶段 / 三阶段排序**
   Recall -> Fusion -> Rerank -> Evidence Gate

### 代表产品 / 开源项目

1. `Weaviate Hybrid Search`
2. `Vespa Hybrid Retrieval`
3. `Milvus Hybrid Search`
4. `Pinecone Sparse + Dense`

### 与我们项目的对比

| 项 | 现状 |
| --- | --- |
| Dense + Sparse 混合检索 | **已实现** |
| 自建 BM25 倒排 | **已实现** |
| RRF / MinMax 融合 | **已实现** |
| 动态 TopK | **已实现** |
| Query Rewrite | **已实现** |
| 查询自适应权重学习 | **部分实现** |
| Multi-Query Fusion | **部分实现** |
| Learned Fusion / LTR | **未实现** |

### 差距分析

1. 当前已经具备很强的 Hybrid 基线。
2. 主要差距不是“有没有 Hybrid”，而是**Hybrid 是否会根据问题自动变策略**。
3. 融合仍偏规则驱动，离“数据驱动权重优化”还有一步。

### 落地建议

1. 在现有动态 `TopK` 基础上，升级为 **查询类型控制器**
   事实问答、模糊概念问答、长尾缩写问答、多跳问答使用不同融合配方。
2. 增加 **Multi-Query + RRF**
   一次问题改写生成 2-4 个候选查询，分别召回后再融合。
3. 对已有离线评测集做 **融合参数自动寻优**
   先做网格搜索，再考虑轻量学习排序。
4. 在元数据层补强：
   `section_type`、`doc_type`、`time_range`、`authority_level`，提升 filter-aware retrieval。

### 判断

**优先级：P0-P1**  
**你们这块基础很好，建议做“精修”而不是推倒重来**

---

## 4. 生成技术调研

## 4.1 Self-RAG

### 原理简述

Self-RAG 的核心是：**模型在生成过程中自己判断是否需要检索、是否需要更多证据、当前答案是否可信**，而不是固定“一问一检索”。

它通常包含三个动作：

1. 先判断是否需要检索
2. 检索后判断证据是否足够
3. 生成后反思答案是否需要修正

### 代表产品 / 开源项目

1. `Self-RAG` 论文与官方实现
2. 各类基于反思 token / reflection prompt 的工程实现

### 与我们项目的对比

| 项 | 现状 |
| --- | --- |
| Evidence Gate | **已实现** |
| 引用一致性检查 | **已实现** |
| 动态 TopK | **已实现** |
| 生成过程中的自反思检索决策 | **部分实现** |

### 差距分析

1. 你们已经有 Self-RAG 的两个重要前置件：
   `Evidence Gate` 和 `Citation Consistency`。
2. 但当前反思主要发生在链路外侧，不是在生成中间态驱动检索策略。
3. 缺少“答案不够 -> 自动补检索 -> 再生成”的闭环。

### 落地建议

1. 先做 **Self-RAG Lite**
   不改基础模型，只加控制器。
2. 控制器判断条件建议包括：
   命中分数分布、证据覆盖率、引用冲突率、答案不确定性。
3. 输出动作建议只有三类：
   `直接回答`、`补检索一次`、`拒答/澄清`。
4. 不建议第一阶段做复杂反思 token 训练，先用规则 + 小模型分类器落地。

### 判断

**优先级：P0**  
**你们已经具备良好基础，升级成本不高**

---

## 4.2 CRAG

### 原理简述

CRAG 的核心是：**先评估检索结果质量，再决定是否纠偏**。

典型动作包括：

1. 给检索结果打置信分
2. 如果低置信，则补充检索、重新查询、外部搜索或局部知识修复
3. 最终只把高质量证据送入生成

### 代表产品 / 开源项目

1. `CRAG` 论文
2. 工程上常见的 “retrieval evaluator + fallback retriever” 实现

### 与我们项目的对比

| 项 | 现状 |
| --- | --- |
| Evidence Gate | **已实现** |
| baseline/candidate 对比 | **已实现** |
| 查询改写 | **已实现** |
| 检索质量评估后自动纠偏 | **部分实现** |

### 差距分析

1. 你们已经有“评估检索质量”的意识，但更多体现在离线 Gate 和最终拒答。
2. 还缺少“在线纠偏动作编排”：
   例如低置信时自动切策略、扩查询、补 parent、换 reranker、补外部检索。

### 落地建议

1. 增加一个 **Retrieval Quality Scorer**
   输入包括召回重叠率、分数陡峭度、引用覆盖率、chunk 冲突率。
2. 低质量时执行固定纠偏序列：
   `扩查询 -> 提高 TopK -> 开启 parent 扩展 -> 启用精排 -> 再判定`
3. 每次纠偏都要记录成本和收益，不然 CRAG 容易把链路越做越慢。

### 判断

**优先级：P0-P1**  
**非常适合你们现有门禁和策略中心体系**

---

## 4.3 Adaptive RAG

### 原理简述

Adaptive RAG 关注的是：**不同复杂度的问题，走不同的检索和生成链路**。

例如：

1. 简单事实问题：单次检索 + 快速回答
2. 模糊问题：改写 + 多路召回 + 精排
3. 多跳问题：分解问题 + 多轮检索 + 聚合

### 代表产品 / 开源项目

1. `Adaptive-RAG` 官方实现
2. `LlamaIndex Router / Workflow`
3. `Haystack Conditional Router`

### 与我们项目的对比

| 项 | 现状 |
| --- | --- |
| 动态 TopK | **已实现** |
| Query Rewrite | **已实现** |
| Feature Flags | **已实现** |
| 按复杂度自动切换检索链路 | **部分实现** |

### 差距分析

1. 当前已经有“自适应雏形”，但粒度主要是参数级，而不是工作流级。
2. 缺少明确的 query complexity classifier。
3. 缺少“简单问题走快链路，复杂问题走深链路”的标准编排。

### 落地建议

1. 新增查询复杂度分类：
   `simple / ambiguous / multi-hop / analytical`
2. 每一类绑定一套策略模板。
3. 把策略模板放进你们现有策略中心，而不是写死在代码里。
4. 用现有 L0-L7 评测框架为每类问题分别设 Gate，不要只看总体平均值。

### 判断

**优先级：P0**  
**是你们现有能力最自然的升级方向**

---

## 4.4 Agentic RAG

### 原理简述

Agentic RAG 的核心是：**把检索从单步操作升级为可规划、可回溯、可调用工具的多步流程**。

典型能力包括：

1. 问题分解
2. 多轮检索
3. 工具调用
4. 子结论合并
5. 预算控制和失败回退

### 代表产品 / 开源项目

1. `LangGraph`
2. `LlamaIndex Agent / Workflow`
3. `Haystack Agent`
4. `OpenAI / Anthropic` 工具调用工作流

### 与我们项目的对比

| 项 | 现状 |
| --- | --- |
| MCP 模块 | **已实现** |
| Loop Engineering 规划 | **已出文档** |
| 单轮 RAG 主链路 | **已实现** |
| 多步规划式 Agentic RAG | **未实现** |

### 差距分析

1. 你们已经有 MCP，这意味着工具扩展面不差。
2. 但还没有真正的 planner / executor / verifier 链路。
3. Agentic RAG 很容易变慢、变贵、变不可控，如果没有预算和门禁，会吞掉平台稳定性。

### 落地建议

1. 第一阶段只做 **受限 Agentic RAG**
   仅允许固定 2-3 步，不做开放式长循环。
2. 先选高价值场景：
   多文档综述、制度对比、故障排查、复杂客户问答。
3. 必加三道保险：
   `最大步数`、`最大 token 预算`、`每步证据校验`。
4. Agent 流程状态写入 Trace，必须能在后台完整回放。

### 判断

**优先级：P2**  
**适合作为高价值场景增强，不适合作为全量默认链路**

---

## 5. 文档处理技术调研

## 5.1 Late Chunking

### 原理简述

Late Chunking 的核心是：**先对长文本做整体编码，再按 chunk 边界从 token 表示里池化得到 chunk 向量**。  
这样每个 chunk 向量会携带更多全局上下文。

### 代表产品 / 开源项目

1. `Jina Late Chunking`
2. 长上下文 embedding 模型生态

### 与我们项目的对比

| 项 | 现状 |
| --- | --- |
| 结构感知切片 | **已实现** |
| 语义二次切分 | **已实现** |
| 上下文化嵌入 | **已实现** |
| Token-level Late Chunking | **未实现** |

### 差距分析

1. 你们的上下文化嵌入已经缓解了一部分“孤立 chunk”问题。
2. 但 Late Chunking 依赖长上下文 embedding 和 token 级表示访问能力，当前基础设施并不具备。
3. 这类方案对工程和推理成本要求较高，短期不如重排序和 Contextual Retrieval 划算。

### 落地建议

1. 暂不建议主链路优先。
2. 仅在长文档、强指代、跨节依赖重的场景做小规模实验。
3. 先完成轻量 Contextual Retrieval，再评估是否还需要 Late Chunking。

### 判断

**优先级：P2-P3**

---

## 5.2 Contextual Retrieval

### 原理简述

Anthropic 的 Contextual Retrieval 核心是：**在索引前为 chunk 补充“该 chunk 在整篇文档中的上下文说明”**，然后同时用于检索和排序。

它解决的是：chunk 自身文本太短、太孤立、太依赖上下文时，检索器很难命中的问题。

### 代表产品 / 开源项目

1. `Anthropic Contextual Retrieval`
2. 各类 `chunk summary / context prefix` 实现

### 与我们项目的对比

| 项 | 现状 |
| --- | --- |
| embedding_content = [Document] + [Section] + [Chunk] | **部分实现** |
| Parent-Child 命中后补父上下文 | **已实现** |
| 基于 LLM 的 chunk 级上下文说明生成 | **未实现** |
| Context-aware BM25 / Dense 一体化索引 | **部分实现** |

### 差距分析

1. 你们已经做了轻量上下文化，这一点非常正确。
2. 但当前更像“规则式上下文拼接”，还不是“chunk 专属上下文解释”。
3. Parent-Child 是命中后补上下文，Contextual Retrieval 是索引前增强，这两者互补，不冲突。

### 落地建议

1. 这是非常适合你们的方向，建议直接做。
2. 第一阶段先生成轻量上下文说明：
   `本节主题 + 上级标题 + chunk 角色`
3. 展示和引用仍使用原文，检索和 embedding 使用增强文本。
4. 在元数据里显式区分：
   `raw_content`、`embedding_content`、`context_summary`
5. 对监管、制度、财报、手册型文档优先启用。

### 判断

**优先级：P0-P1**  
**这是文档处理层最值得补强的方向**

---

## 5.3 Propositions

### 原理简述

Propositions 可以理解为：**把文档从“段落索引”升级为“事实 / 命题索引”**。  
一个 chunk 中可能包含多个事实，而命题索引会把它们拆成更细粒度的原子单元，再保留回溯原文的映射。

它的价值在于：

1. 更适合事实问答
2. 更适合多跳拼接
3. 更适合引用核验和证据覆盖判断

### 代表产品 / 开源项目

1. `Dense X Retrieval` 相关研究
2. `LlamaIndex` 命题 / 原子事实类实践
3. 各类 claim extraction / atomic facts pipeline

### 与我们项目的对比

| 项 | 现状 |
| --- | --- |
| chunk 级索引 | **已实现** |
| Parent-Child 回溯 | **已实现** |
| 命题级抽取与索引 | **未实现** |

### 差距分析

1. 当前检索粒度还是 chunk。
2. 面对“一个段落里有多个条件、多个规则、多个数字”的内容时，chunk 级召回会混杂噪声。
3. 命题索引会带来额外抽取成本、映射维护成本和索引膨胀问题。

### 落地建议

1. 不建议全库默认启用。
2. 建议做 **双层索引**
   `chunk 主索引 + proposition 实验索引`。
3. 首批只在强事实型知识库落地：
   FAQ、规章制度、产品参数、技术规范。
4. 命题必须保留：
   `source_chunk_id`、`char_range`、`confidence`。

### 判断

**优先级：P1**  
**适合作为高精度事实问答增强层**

---

## 5.4 多模态 RAG

### 原理简述

当前市场上的多模态 RAG 已经从“图片 OCR 一下再检索”发展到三条路线：

1. **图文统一 embedding**
   图像、页面、文本进入同一向量空间。
2. **页面级视觉检索**
   直接检索 PDF 页面截图、表格、图文混排内容。
3. **视频理解**
   结合 `ASR + keyframe + timeline chunking + multimodal embedding`。

### 代表产品 / 开源项目

1. `ColPali / ColQwen2`
2. `Qwen2-VL / Gemini / GPT-4o` 视觉理解链路
3. `Milvus / Qdrant / Weaviate` 多模态检索实践

### 与我们项目的对比

| 项 | 现状 |
| --- | --- |
| 多模态调研 | **已完成** |
| 多模态主链路实现 | **未实现** |
| 图文统一索引 | **未实现** |
| 视频理解与时间轴检索 | **未实现** |

### 差距分析

1. 当前系统仍是文本型 RAG。
2. 面对图片说明书、扫描 PDF、图表报告、视频知识库时能力空白明显。
3. 多模态一旦落地，会对存储、索引、评测、前端展示都提出新要求。

### 落地建议

1. 先做 **PDF 页面图像检索**，不要一上来做视频。
2. 技术路线建议：
   `文档页面截图 -> ColPali/ColQwen2 embedding -> 页面召回 -> OCR/原文辅助解释`
3. 第二阶段再做：
   `图片问答`、`表格问答`、`图表证据引用`
4. 视频只建议做专题试点，不建议近期平台化铺开。

### 判断

**优先级：P1-P2**  
**有战略价值，但首期要聚焦 PDF 页面场景**

---

## 6. 评测与监控调研

## 6.1 RAGAS、DeepEval 等评测框架

### 原理简述

市场主流 RAG 评测框架关注三类指标：

1. **检索质量**
   Recall、MRR、nDCG、Context Precision、Context Recall
2. **答案质量**
   Faithfulness、Answer Relevancy、Correctness
3. **系统质量**
   延迟、成本、稳定性、失败率

### 代表产品 / 开源项目

1. `RAGAS`
2. `DeepEval`
3. `TruLens`
4. `Arize Phoenix`

### 与我们项目的对比

| 项 | 现状 |
| --- | --- |
| 自研 L0-L7 评测框架 | **已实现** |
| baseline/candidate 对比 | **已实现** |
| Gate 检查 | **已实现** |
| RAGAS 标准指标体系对齐 | **部分实现** |
| DeepEval 生态接入 | **未实现** |

### 差距分析

1. 自研框架是优势，但外部标准接入不足会影响行业可比性。
2. 现有 Gate 已经很强，但还可以补充 `Faithfulness / Answer Relevancy / Hallucination` 等通用指标。
3. 少了一层“对外可沟通”的标准话语体系。

### 落地建议

1. 保留 L0-L7，不要替换。
2. 在此基础上新增一个 **标准指标兼容层**
   输出 RAGAS / DeepEval 风格结果。
3. 评测报告同时展示：
   `你们内部 Gate` + `行业标准指标`。
4. 这样既保留工程实战口径，也便于对外汇报和招聘传播。

### 判断

**优先级：P1**

---

## 6.2 RAG 可观测性最新实践

### 原理简述

当前先进实践不是只看接口成功率，而是看**一次问答全链路的每个检索与生成决策**。

重点包括：

1. Query Rewrite 前后对比
2. 每路召回的候选、分数、去重情况
3. Fusion 前后排序变化
4. Reranker 输入输出
5. Evidence Gate、引用检查结果
6. 最终答案与证据绑定关系

### 代表产品 / 开源项目

1. `Langfuse`
2. `Arize Phoenix`
3. `LangSmith`
4. `OpenTelemetry` + 自建看板

### 与我们项目的对比

| 项 | 现状 |
| --- | --- |
| Prometheus + Grafana | **已实现** |
| 飞书告警 | **已实现** |
| 审计日志 | **已实现** |
| Span 级 RAG Trace 语义 | **部分实现** |
| 检索候选可回放与对比分析 | **部分实现** |

### 差距分析

1. 你们有基础监控，但还缺少行业正在普及的 `trace-first RAG observability`。
2. 平台侧要回答的不只是“慢不慢”，还要回答“为什么这次没召回到”“为什么 rerank 把它压下去了”。

### 落地建议

1. 统一 Trace Schema：
   `query -> rewrite -> recall -> fusion -> rerank -> evidence -> generate -> cite-check`
2. 每步至少记录：
   输入、输出、候选 id、分数、耗时、策略版本。
3. 在后台提供单次问答链路回放页。
4. 将异常模式做成告警：
   `高置信低正确`、`高召回低引用一致性`、`重排序收益为负`。

### 判断

**优先级：P1**  
**这会放大你们已有评测与治理优势**

---

## 6.3 自动化评测与 A/B 测试

### 原理简述

当前市场最佳实践是把 RAG 优化做成持续实验系统：

1. 新策略先离线评测
2. 通过 Gate 后进入小流量
3. 在线比对成本、延迟、点击、人工反馈
4. 再决定是否扩大灰度

### 代表产品 / 开源项目

1. `LangSmith Online Eval`
2. `Phoenix Evals`
3. `DeepEval CI/CD`
4. 自建 A/B 实验平台

### 与我们项目的对比

| 项 | 现状 |
| --- | --- |
| baseline/candidate | **已实现** |
| Gate 检查 | **已实现** |
| 灰度发布 | **已实现** |
| 回滚 | **已实现** |
| 自动化实验闭环 | **部分实现** |

### 差距分析

1. 你们已经比很多团队领先。
2. 下一步重点不是“有没有 A/B”，而是把线上反馈真正接回评测集和策略优化。

### 落地建议

1. 增加线上反馈标签：
   `有帮助 / 无帮助 / 证据不足 / 引用错误 / 回答过慢`
2. 把失败样本自动回流到评测集。
3. 在发布门禁里加入：
   `线上影子收益阈值`。

### 判断

**优先级：P1**

---

## 7. 架构模式调研

## 7.1 Modular RAG

### 原理简述

Modular RAG 强调把 RAG 拆成可替换模块：

1. 解析
2. 切片
3. 索引
4. 召回
5. 融合
6. 重排
7. 生成
8. 评测
9. 治理

### 代表产品 / 开源项目

1. `LlamaIndex`
2. `Haystack`
3. `DSPy / workflow` 风格系统

### 与我们项目的对比

| 项 | 现状 |
| --- | --- |
| 模块化检索链路 | **已实现** |
| Feature Flags / 策略中心 | **已实现** |
| 灰度 / 回滚 / 审计 | **已实现** |
| 统一模块契约 | **部分实现** |

### 差距分析

1. 你们其实已经很接近 Modular RAG 平台。
2. 主要问题不是“模块少”，而是“模块接口规范化和实验编排标准化”还可以更进一步。

### 落地建议

1. 给解析、切片、检索、重排、生成定义统一实验契约。
2. 每个模块都要可替换、可灰度、可回滚、可观测。

### 判断

**状态：已具备明显优势**

---

## 7.2 RAG Fusion

### 原理简述

RAG Fusion 一般指：

1. 对同一问题生成多种查询表达
2. 多路检索
3. 用 `RRF` 等方法融合结果

### 代表产品 / 开源项目

1. `RAG-Fusion` 论文 / 社区实现
2. `LlamaIndex Fusion Retriever`

### 与我们项目的对比

| 项 | 现状 |
| --- | --- |
| RRF / MinMax 融合 | **已实现** |
| Query Rewrite | **已实现** |
| 多查询并行融合 | **部分实现** |

### 差距分析

你们已经做到了 Fusion 的一半，差的是：

1. 多查询扩展还不够系统
2. 融合权重还不够数据驱动

### 落地建议

1. 在 Query Rewrite 之外增加 `query set generation`
2. 让每个子查询走独立召回，再统一融合

### 判断

**优先级：P1**  
**可以直接叠加在现有链路上**

---

## 7.3 Speculative RAG

### 原理简述

Speculative RAG 的目标是：**用更便宜、更快的路径提前猜测答案或证据，再让更贵的路径只验证关键部分**。  
本质上是把“生成”和“检索”做并行化或分层化，以降低端到端延迟。

### 代表产品 / 开源项目

1. `Speculative RAG` 研究工作
2. 草稿模型 + 主模型验证类架构

### 与我们项目的对比

| 项 | 现状 |
| --- | --- |
| 快慢链路拆分 | **部分实现**，有缓存和治理基础 |
| Speculative Retrieval / Draft Answer | **未实现** |

### 差距分析

1. 当前系统更偏稳健，不偏极致低延迟。
2. 在没有强流量压力和毫秒级目标前，Speculative RAG 的优先级不高。

### 落地建议

1. 暂不建议近期优先投入。
2. 只有当低延迟成为核心业务指标时，再考虑引入。

### 判断

**优先级：P3**

---

## 7.4 Cache-Augmented Generation（CAG）

### 原理简述

CAG 的核心思路是：**把高频知识和生成上下文尽可能缓存起来，减少重复检索和重复推理**。  
它比普通语义缓存更进一步，目标是把“答案缓存”升级为“知识与上下文缓存”。

### 代表产品 / 开源项目

1. `Cache-Augmented Generation` 相关研究
2. 各类 `semantic cache + context cache + prompt cache` 实践

### 与我们项目的对比

| 项 | 现状 |
| --- | --- |
| 语义缓存 L0-L6 | **已实现** |
| 基于相似问题的命中 | **已实现** |
| 上下文包级缓存 / 证据包缓存 | **部分实现** |
| 生成态 cache orchestration | **未实现** |

### 差距分析

1. 你们已经有很好的缓存基础。
2. 但当前更多是“问答级缓存”，还不是“检索包 / 证据包 / 生成上下文包”的统一缓存编排。

### 落地建议

1. 在现有 L0-L6 上继续演进，而不是另起炉灶。
2. 新增两类缓存：
   `retrieval bundle cache`、`evidence bundle cache`。
3. 对高频知识库可探索：
   周期性预热热点问题上下文包。

### 判断

**优先级：P1**  
**属于你们已有优势上的顺势增强**

---

## 8. 全量对比结论

### 8.1 已实现 / 部分实现 / 未实现总表

| 方向 | 结论 |
| --- | --- |
| Advanced Reranking | **部分实现** |
| Multi-Vector Retrieval | **未实现** |
| Graph RAG | **未实现** |
| Hybrid Search 最新实践 | **已实现，仍可优化** |
| Self-RAG | **部分实现** |
| CRAG | **部分实现** |
| Adaptive RAG | **部分实现** |
| Agentic RAG | **未实现** |
| Late Chunking | **未实现** |
| Contextual Retrieval | **部分实现** |
| Propositions | **未实现** |
| 多模态 RAG | **未实现，已有调研** |
| RAGAS / DeepEval 接入 | **部分实现 / 未实现** |
| 可观测性最新实践 | **部分实现** |
| 自动化评测与 A/B | **已实现，仍可增强** |
| Modular RAG | **已实现较强基础** |
| RAG Fusion | **部分实现到已实现之间** |
| Speculative RAG | **未实现** |
| CAG | **部分实现** |

### 8.2 我们项目的技术优势总结

1. **企业级治理能力明显领先**
   灰度、回滚、门禁、审计、告警、监控、策略中心，这一层是很多 RAG 系统最薄弱的地方。
2. **检索主链路扎实**
   Dense + Sparse、自建 BM25、RRF / MinMax、动态 `TopK`、Query Rewrite，已经超过“拼装式 RAG”。
3. **文档处理基础好**
   结构感知切片、语义二次切分、Parent-Child、上下文化嵌入，为后续升级留足了接口。
4. **评测文化成熟**
   L0-L7、baseline/candidate、Gate，是持续优化的好底座。

### 8.3 最大差距在哪里

最大的差距集中在三件事：

1. **精排能力还不够强**
   这会直接限制最终相关性与引用质量上限。
2. **检索控制器还不够智能**
   还没形成完整的 Self-RAG / CRAG / Adaptive RAG 闭环。
3. **知识表示还不够先进**
   没有命题层、图谱层、多向量层，复杂推理和复杂文档场景会吃亏。

---

## 9. 最值得优先补强的 5 个方向

## 9.1 方向一：先进重排序级联

### 为什么优先

1. 收益最直接
2. 改造风险最低
3. 和现有 Hybrid 检索、Gate、灰度体系天然兼容

### 具体落地建议

1. 新增 Python `reranker-service`
2. 首期接 `BGE-Reranker`
3. 第二期接 `Cohere Rerank` 做外部效果对照
4. 线上策略：
   `Top50 Recall -> Top15 Rerank -> Top8 Answer`
5. 发布规则：
   只有在 `nDCG@10`、引用一致性提升且延迟可控时才灰度放量

## 9.2 方向二：Adaptive / Self / CRAG 轻量控制器

### 为什么优先

1. 你们已有动态 `TopK`、Query Rewrite、Evidence Gate
2. 基础已经具备，差的是统一编排

### 具体落地建议

1. 新增 `query_complexity_classifier`
2. 新增 `retrieval_quality_scorer`
3. 策略分三档：
   `Fast`、`Balanced`、`Deep`
4. 低质量时自动纠偏：
   `改写 -> 扩召回 -> parent 扩展 -> 精排 -> 再判断`

## 9.3 方向三：轻量 Contextual Retrieval + Proposition 实验索引

### 为什么优先

1. 你们已经有上下文化嵌入基础
2. 这是文档处理层最具确定性的升级路径

### 具体落地建议

1. 为每个 chunk 生成 `context_summary`
2. embedding 和 BM25 同时使用增强文本
3. 对强事实型知识库增加 proposition 子索引
4. 评测重点看：
   长尾问答召回率、孤立 chunk 命中率、事实问答正确率

## 9.4 方向四：标准化评测与 Trace-First 可观测性

### 为什么优先

1. 你们已有评测与治理优势
2. 这能把优势放大成长期优化飞轮

### 具体落地建议

1. 输出兼容 `RAGAS / DeepEval` 的评测报告
2. 用统一 Trace Schema 记录完整检索链路
3. 让后台支持单次问答全链路回放
4. 失败样本自动回流评测集

## 9.5 方向五：Multi-Vector / GraphRAG 实验轨

### 为什么优先

1. 这是复杂问题场景的中长期壁垒
2. 但工程成本高，不宜直接主链路化

### 具体落地建议

1. `ColBERT` 先做文本复杂问答实验
2. `ColPali / ColQwen2` 先做 PDF 页面级实验
3. `GraphRAG` 先做轻量图侧车，不急着引入重型图数据库

---

## 10. 建议的实施顺序

### Phase 1：1-2 个迭代内完成

1. Advanced Reranking
2. Adaptive / Self / CRAG 轻量控制器
3. Contextual Retrieval 增强版

### Phase 2：2-4 个迭代内完成

1. 标准化评测兼容层
2. Trace-First 可观测性
3. Proposition 实验索引

### Phase 3：中期实验轨

1. Multi-Vector Retrieval
2. PDF 页面级多模态检索
3. GraphRAG
4. Agentic RAG

### 不建议近期优先的方向

1. Late Chunking
2. Speculative RAG
3. 全量视频 RAG 平台化

原因很简单：这些方向不是没有价值，而是**在你们当前阶段，投入产出比明显不如精排、轻控制器和上下文增强**。

---

## 11. 参考资料

以下资料用于本报告的市场方向判断与技术对比：

1. Self-RAG（ICLR 2024）
   <https://openreview.net/forum?id=hSyW5go0v8>
2. CRAG（Corrective Retrieval Augmented Generation）
   <https://arxiv.org/abs/2401.15884>
3. Adaptive-RAG 官方实现
   <https://github.com/starsuzi/Adaptive-RAG>
4. Microsoft GraphRAG
   <https://github.com/microsoft/graphrag>
5. ColBERT
   <https://github.com/stanford-futuredata/ColBERT>
6. ColPali
   <https://github.com/illuin-tech/colpali>
7. BGE-Reranker v2 m3
   <https://huggingface.co/BAAI/bge-reranker-v2-m3>
8. Cohere Rerank
   <https://docs.cohere.com/docs/rerank-overview>
9. Anthropic Contextual Retrieval
   <https://www.anthropic.com/engineering/contextual-retrieval>
10. Jina Late Chunking
   <https://jina.ai/news/late-chunking-in-long-context-embedding-models/>
11. Dense X Retrieval / Proposition 相关研究
   <https://arxiv.org/abs/2312.06648>
12. Weaviate Hybrid Search
   <https://docs.weaviate.io/weaviate/concepts/search/hybrid-search>
13. RAGAS
   <https://docs.ragas.io/>
14. DeepEval
   <https://github.com/confident-ai/deepeval>
15. Arize Phoenix
   <https://docs.arize.com/phoenix>
16. LangGraph
   <https://langchain-ai.github.io/langgraph/>
17. Speculative RAG
   <https://arxiv.org/abs/2407.08223>
18. Cache-Augmented Generation
   <https://arxiv.org/abs/2505.08261>

