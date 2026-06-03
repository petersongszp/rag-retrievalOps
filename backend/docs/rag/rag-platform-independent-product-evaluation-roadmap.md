# RAG 管控平台独立化评估与产品实施路线图

## 1. 文档目的与使用方式

本文用于团队内部评审和对齐，回答三个核心问题：

1. 当前 RAG 后台管理能力能不能从“面试吧”业务中独立出去，变成一个通用平台。
2. 其他业务的 Agent 是否可以只调用本平台接口，就获得向量检索、知识库检索、引用返回、调试监控和策略调优能力。
3. 这个平台应该被打造成什么产品，如何分阶段落地，如何形成市场竞争力。

本文建议按以下方式使用：

1. 对管理层或产品团队：重点看第 2、3、4、5 节，判断方向和产品定位。
2. 对研发团队：重点看第 6、7、8、9、10 节，判断架构改造和实施路线。
3. 对业务接入团队：重点看第 7、8 节，理解 Agent 如何接入、接入后能获得什么能力。

一句话结论：

> 这个平台技术上可以独立出去，而且当前代码已经具备雏形。更推荐的产品定位不是“通用向量数据库”或“普通知识库后台”，而是“企业级 RAG 管控平台 / RetrievalOps 平台”：让多业务通过统一 API 接入检索能力，并在平台上完成知识入库、向量化、检索策略、质量评估、成本治理、审计告警和可视化调试。

---

## 2. 核心结论

## 2.1 能不能独立出去

可以。

当前项目里 RAG 能力已经有比较清晰的服务边界：

1. 后端已经注册了非 Admin 的业务入口 `/api/kb`，也注册了管理入口 `/api/admin/kb`。
2. 后端已有统一检索接口 `POST /api/kb/retrieve` / `POST /api/admin/kb/retrieve`。
3. 检索请求已经支持 `kb_id`、`kb_ids`、`query`、`top_k`。
4. 检索链路已经接入 release / experiment / strategy 决策。
5. 管理台已经具备知识库管理、文档上传、任务监控、检索测试、调试视图、策略中心、成本看板、审计中心、告警中心、Vector DB 运维等方向的功能雏形。

所以独立化不是从 0 重新做，而是把现有 RAG 模块从业务项目里抽成一个明确的独立平台边界。

## 2.2 其他业务能不能只调接口实现向量检索

可以。

目标调用方式应该是：

```text
业务 Agent
  -> 调用 RAG 平台 /v1/retrieve
  -> 平台按 app_id / tenant_id / kb_id / strategy_version 选择检索策略
  -> 平台完成 embedding、vector search、hybrid search、rerank、filter、evidence gate
  -> 返回 chunks、score、citation、source、request_id
  -> 业务 Agent 把检索结果注入自己的 prompt 或工作流
  -> 管理台通过 request_id 看调试、成本、审计、质量
```

接入方不需要自己接 Milvus、Qdrant、Pinecone，不需要自己管理 embedding 模型，不需要自己处理 chunk、index、rerank、策略灰度、日志审计和可视化监控。

## 2.3 平台能不能通过调策略提升检索精准度

可以，但前提是策略必须平台化、版本化、可评估、可回滚。

现在代码里已经有策略中心和实验机制雏形，后续要从全局配置升级为按业务、按知识库、按版本生效：

```text
app_id + tenant_id + kb_id + strategy_profile + strategy_version + rollout_status
```

这样不同业务可以使用不同的：

1. `top_k`
2. `candidate_topk`
3. hybrid dense/sparse 权重
4. query rewrite 策略
5. rerank 模型和阈值
6. parent-child 上下文补全策略
7. evidence gate 阈值
8. citation consistency 检查版本
9. fallback / refusal 策略

平台化之后，业务方只需要在管理台选择策略、灰度发布、观察效果、必要时回滚，不需要每个业务团队重复修改代码。

---

## 3. 市场与方向评估

## 3.1 市场空间

RAG 不是短期概念，已经成为企业 AI 应用落地的基础设施方向。

公开市场报告显示：

1. Grand View Research 估算全球 RAG 市场 2024 年约 12 亿美元，到 2030 年约 110 亿美元，2025-2030 年 CAGR 约 49.1%。
2. Grand View Research 估算全球向量数据库市场 2023 年约 16.6 亿美元，到 2030 年约 73.4 亿美元，2024-2030 年 CAGR 约 23.7%。
3. AWS Bedrock Knowledge Bases、Azure AI Search、Google Vertex AI Search 等云厂商都在把 RAG、向量检索、知识库、Agent 检索能力作为核心云 AI 基础设施。

这说明企业需求是真实存在的：

1. 企业需要把内部文档、知识库、工单、产品资料、培训资料、客服资料接入 AI。
2. 企业不希望每个业务团队都从 0 搭建向量库、embedding、chunk、rerank、评估和运维。
3. RAG 上线后，最大问题往往不是“能不能搜”，而是“搜得准不准、为什么搜错、成本为什么涨、策略改了能不能回滚、证据是否可信”。

## 3.2 竞争格局

当前市场可以分成五类竞争者。

### 3.2.1 向量数据库

代表产品：

1. Pinecone
2. Weaviate
3. Qdrant
4. Milvus
5. pgvector

它们解决的是底层向量存储、相似度搜索、过滤、索引和扩展能力。

优点：

1. 检索底层能力强。
2. 性能和可扩展性成熟。
3. 社区和生态丰富。

局限：

1. 不直接解决企业业务知识库管理。
2. 不直接解决 RAG 策略治理。
3. 不直接解决 request 级调试、成本归因、审计告警。
4. 业务团队仍然要自己接 embedding、chunk、rerank、prompt、监控、评估。

### 3.2.2 云厂商 RAG 服务

代表产品：

1. AWS Bedrock Knowledge Bases
2. Azure AI Search
3. Google Vertex AI Search
4. IBM watsonx Discovery

它们解决的是云上托管 RAG、知识库、向量检索、数据源集成。

优点：

1. 托管能力强。
2. 与云生态集成深。
3. 对企业 IT 采购友好。

局限：

1. 绑定云厂商生态。
2. 私有化、混合云、本地部署、国内模型适配不一定灵活。
3. 策略调试、业务级治理、RAG 质量分析通常不够贴近具体业务。
4. 对中小业务团队来说，配置、权限、账单、数据源链路仍然复杂。

### 3.2.3 开源 LLMOps / RAG 应用平台

代表产品：

1. Dify
2. RAGFlow
3. LlamaIndex
4. LangChain / LangGraph 生态
5. Haystack

它们解决的是应用编排、知识库、Agent、RAG pipeline 组装。

优点：

1. 上手快。
2. 生态活跃。
3. 适合快速做 Demo 和中小应用。

局限：

1. 企业级多业务治理、权限隔离、策略灰度、成本归因和审计能力通常需要二次开发。
2. 对“检索为什么错”的细粒度 debug 不一定足够深入。
3. 复杂 RAG 策略上线后的评估和回滚机制不一定完整。

### 3.2.4 LLM 可观测平台

代表产品：

1. LangSmith
2. Langfuse
3. Arize Phoenix
4. Galileo

它们解决的是 trace、prompt、eval、LLM 调用可观测。

优点：

1. Trace 和 Eval 能力强。
2. 适合排查 LLM 应用链路。
3. 对 prompt、dataset、evaluation 支持较好。

局限：

1. 更偏 LLM 应用可观测，不一定直接管理知识库、向量库、索引生命周期。
2. 不一定提供企业业务侧的 RAG 策略中心。
3. 不一定替业务完成 embedding、chunk、retrieval、rerank、citation gate 的完整托管。

### 3.2.5 企业内部自研 RAG 中台

很多企业最终会走向自研或半自研，因为：

1. 数据安全要求高。
2. 多业务都要接入。
3. 外部产品难以完全匹配内部权限、审计和模型生态。
4. RAG 质量需要结合业务长期调参。

这正是我们平台最适合切入的位置。

## 3.3 我们的竞争力判断

如果只做“上传文档 + 向量检索 + 返回 TopK”，竞争力一般。

因为这件事 Dify、RAGFlow、Bedrock Knowledge Bases、Azure AI Search、Pinecone 等都能做。

但如果定位为“企业级 RAG 管控平台”，竞争力会明显提高。原因是我们已经不只是做检索，而是在做完整的 RAG 运营闭环：

1. 检索链路可解释：能看到 query rewrite、dense/sparse route、fusion、dedupe、rerank、filter、parent-child、TopK、evidence gate、citation consistency。
2. 策略可治理：feature flag、灰度、版本、impact 分析、gate、rollback。
3. 成本可归因：embedding、rerank、LLM、context token、按 request_id / strategy_version / kb_id 下钻。
4. 运维可视化：collection、index version、向量库健康、容量、告警。
5. 审计闭环：谁改了策略、为什么改、改前改后、是否有风险。

这类能力更接近企业真正上线后的痛点。

我们的差异化不应该是“我也有一个向量库”，而应该是：

> 让每个业务不用懂向量数据库和 RAG 工程细节，也能接入一个可调试、可评估、可灰度、可回滚、可审计、可控成本的检索服务。

---

## 4. 产品定位建议

## 4.1 不建议的定位

不建议把平台定位成：

1. 通用向量数据库。
2. 普通知识库问答系统。
3. 又一个 Dify / RAGFlow。
4. 只服务面试业务的内部后台。

这些定位要么竞争过强，要么边界太窄，要么无法体现我们现有治理能力的价值。

## 4.2 推荐定位

推荐定位：

> 企业级 RAG 管控平台，面向多业务 Agent 和 AI 应用提供统一知识接入、向量检索、检索调试、策略治理、质量评估、成本监控和审计告警能力。

也可以称为：

1. RAG Control Plane
2. RetrievalOps Platform
3. 企业知识检索中台
4. 多业务 RAG 管控中台
5. AI Agent 知识检索基础设施

## 4.3 产品一句话

> 业务只需要把文档交给平台，把 Agent 接到统一检索 API，就能获得可调优、可监控、可审计的企业级向量检索能力。

## 4.4 核心用户

### 4.4.1 业务研发团队

诉求：

1. 不想自己搭向量库。
2. 不想自己接 embedding 模型。
3. 不想自己写 chunk、rerank、citation、监控、审计。
4. 希望一个接口就能拿到可靠检索结果。

平台价值：

1. 提供统一 `/v1/retrieve`。
2. 提供 SDK 和 Agent Tool。
3. 提供标准返回结构。
4. 提供 request_id 便于排查。

### 4.4.2 业务运营 / 知识库管理员

诉求：

1. 能上传和管理业务知识。
2. 能看到文档是否入库成功。
3. 能测试检索效果。
4. 能知道哪些问题搜不到、哪些知识需要补。

平台价值：

1. 知识库管理。
2. 文档管理。
3. 入库任务监控。
4. Retrieval Lab。
5. 空召回分析和高频 query 分析。

### 4.4.3 算法 / AI 平台团队

诉求：

1. 能调 retrieval 策略。
2. 能比较 baseline / candidate。
3. 能做灰度发布和回滚。
4. 能量化 Recall@K、MRR、nDCG、Citation Support Score。

平台价值：

1. 策略中心。
2. Evaluation Dataset。
3. Experiment Platform。
4. Strategy Impact。
5. Governance Gate。

### 4.4.4 SRE / 平台运维团队

诉求：

1. 能监控检索延迟、错误率、队列积压、向量库健康。
2. 能做 collection 切换和回滚。
3. 能处理告警。
4. 能控制成本。

平台价值：

1. Vector DB 运维页。
2. Prometheus / Grafana 指标。
3. 告警中心。
4. 成本看板。
5. 周报和治理报告。

## 4.5 适合优先切入的业务场景

第一批适合接入的平台业务：

1. 面试 Agent：技术题库、简历评估、追问建议、候选人回答点评。
2. 客服 Agent：产品 FAQ、售后流程、政策条款、工单知识。
3. 销售支持 Agent：产品资料、案例库、竞品话术、报价规则。
4. 内部知识助手：制度文档、流程文档、研发规范、运维手册。
5. 培训/考试 Agent：课程资料、题库、讲义、学习路径。

优先选择标准：

1. 文档数量中等。
2. 问答场景明确。
3. 知识更新频繁。
4. 对引用和准确性有要求。
5. 当前业务团队没有成熟 RAG 基础设施。

---

## 5. 当前能力盘点

## 5.1 已有可复用能力

基于当前仓库能力，已经具备以下基础：

1. Admin 前端独立服务：`admin/`，Next.js + Ant Design。
2. 后端知识库路由：`/api/kb` 和 `/api/admin/kb`。
3. 知识库管理：创建、列表、选择知识库。
4. 文档管理：上传、列表、删除。
5. 入库任务：job 状态、重试、取消、日志。
6. 检索接口：按 `kb_id` / `kb_ids` / `query` / `top_k` 检索。
7. Milvus 检索服务：dense retrieval。
8. Hybrid retrieval：dense + sparse、fusion、dedupe、rerank、filter/truncate。
9. Debug Trace：检索链路结构化追踪。
10. Strategy Center：feature flags、版本、impact、gate、rollback。
11. Experiment：baseline / candidate、灰度、实验维度。
12. Evaluation：评测集、评测任务、评测报告。
13. Cost Ops：成本汇总、时序、按知识库、按策略、按模型、高成本 query。
14. Vector Ops：collection 列表、健康检查、容量、重建、切换、回滚。
15. Governance：审计事件、告警、周报、gate。
16. Metrics：`rag_retrieve_requests_total`、`rag_retrieve_duration_seconds`、`rag_retrieve_result_count`、`rag_retrieve_route_hits`、`rag_ingest_jobs_total` 等。

## 5.2 当前关键缺口

要独立成平台，还必须补齐：

1. 多租户模型：`tenant_id`、`app_id`、`workspace_id`。
2. API Key / Service Token：给外部业务 Agent 调用。
3. 权限隔离：业务只能访问自己授权的知识库。
4. 策略隔离：不同业务、不同知识库使用不同策略配置。
5. 公开 API Contract：`/v1/retrieve`、`/v1/ingest`、`/v1/kb/*`。
6. Agent SDK：Go / Node / Python 的最小接入包。
7. 接入审计：记录哪个业务、哪个 Agent、哪个 API Key 发起检索。
8. SLA 与限流：按业务设置 QPS、超时、重试、熔断。
9. 计量计费或成本归因：按 app / tenant / kb 统计调用和成本。
10. 独立部署边界：从业务后端中拆出独立服务和配置。

---

## 6. 独立化后的目标架构

## 6.1 总体架构

```text
                +-------------------------+
                |      Admin Console       |
                | 知识库 / 策略 / 监控 / 审计 |
                +------------+------------+
                             |
                             v
+-----------+      +---------+----------+      +----------------+
| 业务 Agent | ---> |  RAG Platform API  | ---> | Retrieval Core |
+-----------+      +---------+----------+      +--------+-------+
                             |                          |
                             v                          v
                    +--------+---------+       +--------+--------+
                    | Governance Layer |       | Vector / Index  |
                    | 审计/成本/告警/评估 |       | Milvus/Qdrant/... |
                    +--------+---------+       +--------+--------+
                             |                          |
                             v                          v
                    +--------+---------+       +--------+--------+
                    | Metadata DB      |       | Object Storage  |
                    | MySQL/Postgres   |       | 文档原文/解析结果 |
                    +------------------+       +-----------------+
```

## 6.2 能力分层

### 6.2.1 接入层

职责：

1. API Key 鉴权。
2. app / tenant 识别。
3. 请求限流。
4. 请求参数校验。
5. 接入审计。

核心对象：

1. `tenant_id`
2. `app_id`
3. `api_key_id`
4. `workspace_id`
5. `request_id`

### 6.2.2 知识入库层

职责：

1. 文档上传。
2. 文件解析。
3. chunk 切分。
4. metadata 注入。
5. embedding 生成。
6. 向量写入。
7. 任务状态机。

核心对象：

1. `knowledge_base`
2. `document`
3. `chunk`
4. `ingest_job`
5. `embedding_model`
6. `collection_version`

### 6.2.3 检索执行层

职责：

1. query preprocess。
2. query rewrite。
3. dense retrieval。
4. sparse retrieval。
5. fusion。
6. dedupe。
7. rerank。
8. filter / truncate。
9. parent-child context fill。
10. evidence gate。
11. citation consistency。

核心输出：

1. `content`
2. `score`
3. `citation`
4. `source`
5. `request_id`
6. `strategy_version`

### 6.2.4 策略治理层

职责：

1. 策略配置。
2. 策略版本。
3. 灰度发布。
4. AB 实验。
5. Gate 判断。
6. 一键回滚。

核心对象：

1. `strategy_profile`
2. `strategy_version`
3. `experiment_id`
4. `release_stage`
5. `rollback_reason`

### 6.2.5 可观测与治理层

职责：

1. 检索日志。
2. debug trace。
3. 成本归因。
4. 质量评估。
5. 审计事件。
6. 告警。
7. 周报。

核心指标：

1. `retrieve_request_count`
2. `retrieve_p95_ms`
3. `retrieve_empty_rate`
4. `Recall@K`
5. `MRR`
6. `nDCG`
7. `Citation Support Score`
8. `Evidence Refusal Rate`
9. `cost_per_1k_queries`
10. `audit_coverage_rate`

---

## 7. 外部业务 Agent 接入方案

## 7.1 推荐公开 API

第一版公开 API 建议固定为：

```http
POST /v1/retrieve
Authorization: Bearer <api_key>
X-App-ID: interview-bar
Content-Type: application/json

{
  "kb_ids": [1001, 1002],
  "query": "Go map 底层结构是什么？",
  "top_k": 8,
  "filters": {
    "language": "zh",
    "category": "golang"
  },
  "debug": false
}
```

返回：

```json
{
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "app_id": "interview-bar",
  "strategy_version": "rag-v3-canary-202605",
  "items": [
    {
      "content": "Go map 底层由 hmap、bucket、overflow bucket 等结构组成...",
      "score": 0.86,
      "citation": {
        "kb_id": 1001,
        "document_id": 88,
        "file_name": "go-map.md",
        "chunk_id": "chunk-001",
        "chunk_index": 12
      },
      "source": {
        "route": "hybrid",
        "collection": "knowledge_active_v3",
        "retriever_version": "hybrid-v1",
        "parent_fill_strategy": "section_window",
        "citation_supported": true,
        "citation_support_score": 0.91
      }
    }
  ],
  "usage": {
    "embedding_tokens": 32,
    "context_tokens": 1200,
    "estimated_cost": 0.0012
  }
}
```

## 7.2 Agent Tool 形态

给业务 Agent 提供工具定义：

```json
{
  "name": "retrieve_knowledge",
  "description": "从企业 RAG 平台检索与问题相关的知识片段，返回可引用证据。",
  "parameters": {
    "type": "object",
    "properties": {
      "query": {
        "type": "string",
        "description": "需要检索的问题或上下文"
      },
      "kb_ids": {
        "type": "array",
        "items": { "type": "integer" },
        "description": "可选知识库 ID 列表"
      },
      "top_k": {
        "type": "integer",
        "description": "返回结果数量"
      }
    },
    "required": ["query"]
  }
}
```

业务 Agent 使用方式：

```text
用户问题
  -> Agent 判断需要知识检索
  -> 调用 retrieve_knowledge
  -> 平台返回证据 chunks
  -> Agent 基于 chunks 生成回答
  -> 回答中带 citation
```

## 7.3 接入方省掉的工作

接入平台后，业务团队不用重复做：

1. 选择和部署向量数据库。
2. 选择和接入 embedding 模型。
3. 文档解析和 chunk 切分。
4. metadata 设计和过滤表达式。
5. dense / sparse / hybrid 检索。
6. rerank 和 TopK 策略。
7. citation 返回。
8. request 级 debug。
9. 质量评估。
10. 成本归因。
11. 审计日志。
12. 向量库运维。

## 7.4 接入方仍然需要负责的事

平台不能替业务完全解决：

1. 业务知识是否完整。
2. 文档是否真实有效。
3. Agent 何时应该调用检索工具。
4. Agent 如何把检索结果组织成最终回答。
5. 业务侧的权限和用户身份映射。

所以平台边界要明确：

> 平台负责“知识入库、检索、证据、策略、监控、治理”；业务 Agent 负责“业务意图、对话流程、最终回答和用户体验”。

---

## 8. 策略中心如何帮助提高检索精准度

## 8.1 为什么只做向量检索不够

单纯向量检索常见问题：

1. 专有名词、缩写、代码名、产品名召回不稳。
2. query 很短时语义不足。
3. query 很长时噪声过多。
4. 只用 dense retrieval 容易漏掉关键词强匹配内容。
5. 返回 chunk 太短导致上下文断裂。
6. 返回 chunk 太多导致 token 成本高、LLM 混淆。
7. rerank 后结果可能被过滤空。
8. 引用存在但不一定真正支持回答。

所以企业级 RAG 的竞争力不在“能搜”，而在“可控地搜准”。

## 8.2 平台可调策略

平台应支持以下策略：

1. `top_k`：最终返回多少条结果。
2. `candidate_topk`：候选召回数量。
3. `hybrid_dense_weight`：向量召回权重。
4. `hybrid_sparse_weight`：关键词召回权重。
5. `rewrite_strategy`：是否启用术语扩展、别名改写、模型辅助改写。
6. `rerank_model`：使用哪个 rerank 模型。
7. `rerank_threshold`：低于多少分过滤。
8. `parent_child_fill_strategy`：是否补父块、相邻块、章节上下文。
9. `token_budget`：上下文最大 token 数。
10. `evidence_min_density`：证据密度阈值。
11. `evidence_min_citation_coverage`：引用覆盖阈值。
12. `citation_check_threshold`：引用一致性阈值。

## 8.3 策略调优闭环

每次策略变更都应该走闭环：

```text
提出策略候选
  -> 离线评测 baseline vs candidate
  -> 通过 Gate
  -> 小流量灰度
  -> 观察 Recall / Citation / P95 / Cost / Empty Rate
  -> 扩大流量或回滚
  -> 生成策略变更审计记录
```

## 8.4 策略调优效果指标

建议第一版关注：

1. `Recall@5 / Recall@10`
2. `MRR`
3. `nDCG`
4. `retrieve_empty_rate`
5. `Citation Support Score`
6. `Evidence Refusal Rate`
7. `P95 latency`
8. `cost_per_1k_queries`
9. `high_cost_query_count`
10. `rollback_count`

---

## 9. 产品能力全景

## 9.1 MVP 必须有

MVP 不要做成“大而全平台”，先证明多业务接入闭环。

必须有：

1. 多业务管理：`tenant_id`、`app_id`、API Key。
2. 知识库管理：创建、列表、授权。
3. 文档入库：上传、解析、切块、embedding、写入向量库。
4. 统一检索接口：`POST /v1/retrieve`。
5. 标准返回：`content / score / citation / source / request_id`。
6. Retrieval Lab：业务可以测试问题。
7. Retrieval Debug：通过 `request_id` 看一次检索链路。
8. 基础策略配置：topK、hybrid 开关、rerank 开关。
9. 基础监控：QPS、P95、错误率、空召回率。
10. 基础审计：上传、删除、检索、策略变更。

## 9.2 第一版增强能力

1. 策略中心：Feature Flag、版本、灰度、回滚。
2. Evaluation：评测集、离线评测、baseline/candidate 对比。
3. 成本看板：按业务、知识库、策略、模型归因。
4. Vector Ops：collection 健康、容量、重建、切换、回滚。
5. 告警中心：质量、稳定性、成本、容量、审计。
6. 周报：质量趋势、成本趋势、风险项、下周动作。

## 9.3 中长期高级能力

1. 多向量库适配：Milvus、Qdrant、Pinecone、pgvector。
2. 多 embedding 模型适配：OpenAI、豆包、BGE、本地模型。
3. 多 rerank 模型适配。
4. 自动策略建议：根据失败 query 推荐 rewrite、metadata、chunk 调整。
5. 自动知识缺口分析：高频空召回 query、低支持 citation、低质量回答。
6. 租户级成本预算。
7. 合规导出和脱敏审计。
8. SDK 和 MCP Server。

---

## 10. 分阶段实施路线

## Phase 0：平台边界冻结与最小独立闭环

目标：

> 把当前 RAG 能力从业务项目中切出清晰边界，让一个外部业务可以通过 API 完成检索。

任务：

1. 冻结当前 RAG API Contract。
2. 新增 `/v1/retrieve`，先复用现有 `Retrieve` 主链路。
3. 新增 `app_id` 和 API Key 验证。
4. 建立 `app -> kb_ids` 授权关系。
5. 当前面试 Agent 改成通过 HTTP 调 `/v1/retrieve`，不再直接调用项目内部 Milvus Tool。
6. 返回结构固定为 `request_id/items/citation/source/strategy_version`。
7. 管理台能按 `app_id` 查看请求日志。

完成标准：

1. 面试业务 Agent 可以通过 `/v1/retrieve` 正常检索。
2. 管理台能看到该请求的 `request_id`、query、kb_id、items、耗时。
3. 无授权业务不能访问知识库。
4. 关闭平台服务时，业务 Agent 能得到明确错误，而不是静默失败。

## Phase 1：多业务接入可用

目标：

> 支持 2-3 个业务通过 API 接入，平台具备基础隔离、限流、监控和审计。

任务：

1. 新增 `tenant_id`、`app_id`、`api_key_id` 数据模型。
2. 新增 API Key 管理页面。
3. 新增知识库授权页面。
4. 支持按 `app_id` 配置 QPS、超时、最大 topK。
5. 检索日志记录 `tenant_id/app_id/api_key_id/kb_ids/request_id`。
6. 成本看板支持按 `app_id` 聚合。
7. 告警支持按业务分组。
8. 输出 Go / Node / Python 的最小 SDK 示例。

完成标准：

1. 至少两个业务共用同一个 RAG 平台。
2. A 业务无法访问 B 业务未授权知识库。
3. 任意请求都能按 `app_id` 追踪。
4. 业务方接入成本降到“申请 API Key + 配置 kb_id + 调 SDK”。

## Phase 2：检索质量提升与策略平台化

目标：

> 平台不仅能检索，还能系统性提升检索精准度。

任务：

1. 策略模型从全局配置升级为 `app_id/kb_id/strategy_profile`。
2. 支持策略版本：baseline、candidate、active、rollback。
3. 支持 topK、candidateTopK、hybrid 权重、rewrite、rerank、parent-child、evidence gate 等配置。
4. 建立评测集管理能力。
5. 支持 baseline vs candidate 离线评测。
6. 策略发布必须通过 Gate。
7. 支持灰度比例：internal -> 5% -> 20% -> 50% -> 100%。
8. 支持策略一键回滚。

完成标准：

1. 某个业务可以在不改代码的情况下切换检索策略。
2. 策略发布前能看到质量、延迟、成本对比。
3. 策略异常时能在 10 分钟内回滚。
4. 管理台能解释一次请求为什么走了某个策略。

## Phase 3：企业级治理与运维能力

目标：

> 平台可用于生产业务，支持运维、成本、审计、告警和周报。

任务：

1. 成本归因完善到 request / app / kb / strategy / model。
2. Vector Ops 支持 collection 版本、重建、切换、回滚。
3. 审计覆盖上传、删除、检索、策略变更、索引切换。
4. 告警覆盖质量、稳定性、成本、容量、审计。
5. 周报自动生成。
6. 治理 Gate 支持上线前检查。
7. Prometheus/Grafana Dashboard 按业务拆分。

完成标准：

1. 任意业务可以看到自己的检索质量和成本。
2. 任意策略或索引变更都有审计记录。
3. collection 切换可回滚。
4. 周报能输出风险项和下周动作。

## Phase 4：平台产品化与规模化

目标：

> 从内部平台升级为可复制、可部署、可演示、可交付的产品。

任务：

1. 梳理部署形态：SaaS、私有化、混合云。
2. 抽象多向量库适配层。
3. 抽象多 embedding / rerank 模型适配层。
4. 提供 OpenAPI 文档和 SDK。
5. 提供 Demo Space：客服、面试、内部知识助手。
6. 提供租户级配额、预算和账单报表。
7. 提供数据脱敏、保留期和合规导出。
8. 提供插件化接入：Webhook、MCP Server、Agent Tool。

完成标准：

1. 新业务可在 1 天内完成接入。
2. 新知识库可在 30 分钟内完成上传、入库、检索测试。
3. 新策略可在管理台配置、评估、灰度、回滚。
4. 平台可以独立部署给其他项目使用。

---

## 11. 数据模型建议

## 11.1 平台租户与业务

建议新增：

1. `rag_tenant`
2. `rag_app`
3. `rag_api_key`
4. `rag_app_kb_permission`
5. `rag_app_quota`

核心字段：

```text
tenant_id
app_id
api_key_id
app_name
owner
status
qps_limit
daily_request_limit
allowed_kb_ids
created_at
updated_at
```

## 11.2 策略模型

建议新增：

1. `rag_strategy_profile`
2. `rag_strategy_version`
3. `rag_strategy_release`
4. `rag_strategy_operation_log`

核心字段：

```text
strategy_profile_id
app_id
kb_id
strategy_version
release_stage
top_k
candidate_topk
hybrid_dense_weight
hybrid_sparse_weight
rewrite_strategy
rerank_model
parent_fill_strategy
evidence_gate_config
citation_check_config
rollback_target_version
```

## 11.3 检索日志

现有 `KBRetrieveLog` 可以扩展：

```text
tenant_id
app_id
api_key_id
request_id
kb_ids
query
final_query
strategy_version
experiment_id
collection_version
retriever_version
duration_ms
embedding_ms
search_ms
rerank_ms
result_status
empty_reason
estimated_cost
```

---

## 12. 技术风险与应对

## 12.1 多租户数据泄露风险

风险：

1. A 业务检索到 B 业务文档。
2. API Key 权限配置错误。
3. metadata filter 缺失导致跨知识库召回。

应对：

1. 所有检索强制注入 `tenant_id/app_id/kb_id` 过滤。
2. `kb_ids` 必须经过授权校验。
3. 未授权请求直接拒绝，不做 fallback。
4. 每次检索日志记录最终 filter expr。
5. 增加权限回归测试。

## 12.2 策略调优导致质量回退

风险：

1. rewrite 改坏 query。
2. rerank 过滤掉正确答案。
3. topK 过小导致召回不足。
4. evidence gate 误拒答。

应对：

1. 所有策略必须有 baseline/candidate 对比。
2. 所有策略必须支持灰度。
3. 所有策略必须支持回滚。
4. Gate 同时看质量、延迟、成本和错误率。
5. 高风险策略先 shadow，不直接影响用户。

## 12.3 成本失控风险

风险：

1. context token 过长。
2. rerank 模型调用过多。
3. embedding 重复生成。
4. 高流量业务无配额。

应对：

1. 设置 `token_budget`。
2. 设置 `candidate_topk` 上限。
3. 文档 hash 去重。
4. 按 app 设置 QPS 和日调用上限。
5. 成本异常告警。

## 12.4 平台性能成为业务瓶颈

风险：

1. 所有业务集中调用导致平台高负载。
2. 向量库慢查询影响 Agent 响应。
3. 入库任务影响检索任务。

应对：

1. 查询链路和入库链路隔离。
2. 检索服务横向扩展。
3. 按业务限流。
4. 热门 query 缓存。
5. 向量库读写分离或 collection 分层。

## 12.5 产品边界膨胀风险

风险：

平台如果同时想做 Agent 编排、模型网关、向量数据库、知识库、BI、审计，会失焦。

应对：

1. 第一阶段只做 RAG 检索管控，不做完整 Agent 平台。
2. 不替代底层向量数据库，而是适配底层向量库。
3. 不替业务生成最终回答，只提供证据和检索上下文。
4. 不做无门槛自动策略学习，先做可控策略和可回滚灰度。

---

## 13. 团队分工建议

## 13.1 后端 / 平台

负责：

1. `/v1/retrieve` API。
2. API Key 鉴权。
3. tenant/app/kb 权限。
4. 策略配置模型。
5. 检索链路改造。
6. 日志、审计、成本采集。

## 13.2 前端 / Admin

负责：

1. 多业务管理页面。
2. API Key 管理页面。
3. 知识库授权页面。
4. 策略中心按业务改造。
5. 成本、审计、告警按业务筛选。
6. 接入文档和 Demo 页面。

## 13.3 算法 / RAG 质量

负责：

1. 评测集建设。
2. baseline/candidate 指标体系。
3. rewrite、rerank、TopK、evidence gate 策略。
4. 质量 Gate。
5. 检索失败归因。

## 13.4 SRE / 运维

负责：

1. 部署拓扑。
2. Prometheus/Grafana。
3. 告警规则。
4. 容量评估。
5. 备份和回滚。
6. SLA 定义。

## 13.5 业务接入方

负责：

1. 提供业务知识文档。
2. 配置知识库。
3. 在 Agent 中调用平台 SDK。
4. 标注评测问题。
5. 反馈 bad case。

---

## 14. 第一批建议任务

建议从以下任务开始：

1. 冻结 `/v1/retrieve` API Contract。
2. 新增 `app_id/api_key` 最小模型。
3. 给当前面试 Agent 做一次“远程检索 API 改造”。
4. 管理台新增“业务应用”列表。
5. 管理台新增“API Key”页面。
6. 检索日志扩展 `app_id/api_key_id`。
7. 成本看板支持按 `app_id` 聚合。
8. 策略中心支持按 `app_id` 过滤和配置。
9. 写一份外部业务接入 Quickstart。
10. 准备一个团队演示 Demo：同一个 RAG 平台服务两个业务知识库。

---

## 15. 团队展示讲法

可以按这个顺序给团队讲：

1. 我们现在做的不是一个“面试业务后台”，而是已经具备 RAG 管控平台雏形。
2. 市场上不缺向量数据库，也不缺简单知识库应用，真正缺的是企业上线后能调试、能治理、能控成本、能审计的 RAG 平台。
3. 我们当前已有知识库、文档入库、检索接口、debug trace、策略中心、成本看板、向量库运维、审计告警等能力。
4. 独立化之后，其他业务 Agent 不需要自己接向量库和 embedding，只要调用 `/v1/retrieve`。
5. 平台返回的不只是文本片段，还包括 score、citation、source、request_id、strategy_version。
6. 以后检索不准，不需要每个业务改代码，而是在平台上调策略、做评测、灰度、回滚。
7. 第一阶段目标不是做大而全产品，而是让 2-3 个业务真实接入，证明平台化闭环。

---

## 16. 最终判断

这个平台值得独立，而且独立方向成立。

推荐判断：

1. 市场可行性：高。RAG 和向量检索仍在高速增长，企业有真实需求。
2. 技术可行性：高。当前代码已经有 `/api/kb`、`/api/admin/kb`、检索接口、策略、调试、成本和治理雏形。
3. 产品竞争力：中高。前提是不要做普通知识库，而要做 RAG 管控平台。
4. 改造难度：中等偏高。难点在多租户、权限、策略隔离、公开 API、SLA 和产品化，不在单次向量检索。
5. 推荐投入方式：先内部平台化，再多业务接入验证，最后产品化输出。

最终产品建议：

> 打造成一个面向企业多业务 Agent 的 RAG RetrievalOps 平台。它提供统一知识入库、向量检索、检索调试、策略灰度、质量评估、成本治理、审计告警和向量库运维能力，让业务团队无需重复建设 RAG 基础设施，只需通过 API 或 SDK 接入即可获得高质量、可治理的知识检索能力。

---

## 17. 参考资料

1. Grand View Research: Retrieval Augmented Generation Market Size Report, 2030  
   https://www.grandviewresearch.com/industry-analysis/retrieval-augmented-generation-rag-market-report
2. Grand View Research: Vector Database Market Size, Share & Trends Report, 2030  
   https://www.grandviewresearch.com/industry-analysis/vector-database-market-report
3. AWS: Retrieve data and generate AI responses with Amazon Bedrock Knowledge Bases  
   https://docs.aws.amazon.com/bedrock/latest/userguide/knowledge-base.html
4. AWS: Foundation Models for RAG - Amazon Bedrock Knowledge Bases  
   https://aws.amazon.com/bedrock/knowledge-bases/
5. Microsoft Learn: Retrieval-augmented generation in Azure AI Search  
   https://learn.microsoft.com/en-us/azure/search/retrieval-augmented-generation-overview
6. Microsoft Learn: Azure AI Search documentation  
   https://learn.microsoft.com/en-us/azure/search/
7. LangSmith documentation  
   https://docs.smith.langchain.com/
8. Langfuse documentation  
   https://langfuse.com/docs
9. Arize Phoenix documentation  
   https://docs.arize.com/phoenix
10. RAGFlow project  
   https://github.com/infiniflow/ragflow
