# RAG RetrievalOps 平台后续实施路线大纲

## 1. 文档目的

本文是在 `backend/docs/rag-platform-independent-product-evaluation-roadmap.md` 的基础上，结合当前项目代码完成度与混合检索现状，整理出的后续执行路线。

它回答三个问题：

1. 当前项目已经做到什么程度。
2. 后面要按什么顺序补齐平台化能力。
3. ES / OpenSearch 在本项目中的定位、影响和接入方式是什么。

本文不是市场评估文档，而是后续研发、产品、算法、SRE 可以对齐的路线大纲。

---

## 2. 当前项目完成度判断

## 2.1 已经具备的基础能力

当前项目已经具备企业级 RAG 平台的雏形，不是从 0 开始。

1. 知识库管理已经存在：`KBKnowledgeBase`、`KBDocument`、`KBIngestJob` 等模型已经建立。
2. 文档入库链路已经存在：上传、解析、切块、embedding、写入 Milvus、任务状态记录已经有基础实现。
3. 统一检索入口已经存在：`POST /api/kb/retrieve` 和 `POST /api/admin/kb/retrieve` 已经支持 `kb_id/kb_ids/query/top_k`。
4. dense retrieval 已经存在：通过 Milvus 向量检索返回候选。
5. hybrid retrieval 已经存在：`phase2-hybrid-v1` 已经跑通 `dense + sparse -> fusion -> dedupe -> rerank -> filter/truncate`。
6. sparse retrieval 已经存在：当前是轻量实现，不依赖 ES，通过 `content like` 拉候选，再用 BM25 倒排索引重排。
7. 分数融合已经存在：dense/sparse 分别归一化，再按 `hybrid_dense_weight` 和 `hybrid_sparse_weight` 加权。
8. query rewrite 已经存在：支持 controlled rewrite、domain terms、route-specific rewrite、model-assisted rewrite 的配置基础。
9. parent-child retrieval 已经存在：支持父子块元数据、上下文回填、token budget 控制的基础能力。
10. strategy / experiment / release 已经存在：有 feature flag、baseline/candidate、灰度、rollback 的雏形。
11. evaluation 已经存在：有评测集、评测任务、报告、策略 profile 的基础代码。
12. observability 已经存在：检索日志、debug trace、route hits、route contribution、empty reason、cost trace 已经有结构化字段。
13. governance 已经存在：audit event、weekly report、alert、vector ops、index lifecycle 已经有代码雏形。

## 2.2 当前关键缺口

要从“面试业务后台能力”升级成“可独立交付的 RetrievalOps 平台”，核心缺口不在单次检索，而在平台边界与企业接入能力。

1. 缺少独立公开 API：还没有稳定的 `/v1/retrieve`、`/v1/ingest`、`/v1/kb/*` 产品化契约。
2. 缺少多租户模型：还没有稳定的 `tenant_id/app_id/workspace_id` 作为一级平台对象。
3. 缺少 API Key / Service Token：外部业务 Agent 还不能以标准方式接入。
4. 缺少应用到知识库的授权模型：还需要 `app -> kb_ids` 权限关系与强制检索过滤。
5. 策略还偏全局配置：需要升级成按 `app_id/kb_id/strategy_profile/strategy_version` 生效。
6. sparse backend 还没有抽象：当前 sparse 写死为本地轻量实现，还没有 ES / OpenSearch / 外部搜索引擎适配层。
7. 外部企业存量数据源接入不足：企业已有 ES、已有文档系统、已有权限体系时，还缺少连接器。
8. SDK 与接入文档不足：Go / Node / Python / Agent Tool 的最小接入包需要补齐。
9. SLA 与限流还不完整：需要按 app 配置 QPS、timeout、topK 上限、daily quota。
10. 私有化部署边界还不清晰：需要明确平台服务、元数据 DB、对象存储、向量库、可选 ES/OpenSearch 的部署关系。

---

## 3. 统一产品定位

推荐定位继续保持：

> 企业级 RAG RetrievalOps 平台，面向多业务 Agent 和 AI 应用提供统一知识接入、混合检索、检索调试、策略治理、质量评估、成本监控、审计告警和索引运维能力。

产品边界需要固定：

1. 不做通用向量数据库。
2. 不做通用 ES / 搜索引擎替代品。
3. 不做完整 Agent 编排平台。
4. 不替业务生成最终回答，平台主要返回证据、引用、分数、链路调试信息。
5. 不把 ES 作为强制依赖，而是把 ES / OpenSearch 作为可插拔 sparse backend。

---

## 4. ES / OpenSearch 的定位与影响

## 4.1 混合检索是否必须接 ES

不必须。

混合检索的本质是 `dense + sparse`，不是 `dense + ES`。ES 只是企业里最常见的一种 sparse / keyword retrieval 引擎。

当前项目已经有混合检索：

```text
query
  -> dense route: Milvus vector search
  -> sparse route: local sparse search(content like + BM25)
  -> fusion: route score normalization + weighted score
  -> dedupe
  -> rerank
  -> filter / parent-child / evidence gate
```

所以当前阶段不需要为了证明混合检索成立而强行引入 ES。

## 4.2 为什么后续仍然要支持 ES / OpenSearch

企业客户里常见的现实情况是：

1. 很多企业已经有 ES / OpenSearch 集群。
2. 很多企业已有日志、文档、工单、FAQ、商品、客服知识等 ES 索引。
3. 这些企业可能没有向量检索能力，或者没有统一 embedding / rerank / RAG debug 能力。
4. 他们更希望“接入现有 ES + 补齐向量检索 + 获得统一 RAG 治理”，而不是重新迁移所有数据。

因此 ES 对我们的产品价值不是“替代当前检索”，而是：

1. 降低企业接入成本。
2. 复用客户已有关键词索引。
3. 提升 hybrid sparse route 的成熟度和性能。
4. 让平台覆盖“已有 ES、缺向量能力”的企业场景。

## 4.3 ES 不应成为平台硬依赖

不建议把 ES 写成平台必选中间件。

原因：

1. 中小部署场景可能不想维护 ES。
2. 当前本地 sparse 已能支撑小规模与 MVP 验证。
3. ES 引入后会带来版本兼容、索引 mapping、安全认证、权限隔离、集群容量、慢查询治理等额外复杂度。
4. 如果平台强依赖 ES，会削弱轻量私有化部署能力。

推荐做法：

```text
SparseEngine interface
  -> LocalSparseEngine: content like + BM25
  -> ElasticsearchSparseEngine: ES BM25 / match / multi_match
  -> OpenSearchSparseEngine: OpenSearch 兼容实现
```

## 4.4 企业已有 ES 时怎么接

企业已有 ES 时，平台应支持 BYO ES 模式。

BYO ES 指：

1. 客户提供 ES endpoint、认证方式、index、字段 mapping。
2. 平台保存连接配置与健康检查结果。
3. 检索时平台仍然统一接收 `/v1/retrieve`。
4. 平台内部把 query 拆成 `dense_query` 和 `sparse_query`。
5. dense route 查 Milvus / Qdrant / pgvector。
6. sparse route 调客户 ES。
7. 平台统一做归一化、融合、去重、rerank、debug trace。

目标链路：

```text
/v1/retrieve
  -> auth: tenant_id/app_id/api_key
  -> strategy: choose dense backend + sparse backend
  -> dense route: vector DB
  -> sparse route: customer ES / OpenSearch
  -> score normalization
  -> weighted fusion
  -> dedupe by document_id + chunk_id
  -> rerank / parent-child / evidence gate
  -> return content / score / citation / source / request_id
```

## 4.5 接 ES 的关键契约

接 ES 前必须先固定契约，否则会出现“ES 命中了，但无法和向量结果融合”的问题。

1. 文档主键契约：`tenant_id/app_id/kb_id/document_id/chunk_id` 必须能在 ES 和向量库两侧对齐。
2. 内容字段契约：ES 至少要有 `content/title/metadata`，字段名可以映射，但平台内部要统一。
3. 权限过滤契约：`tenant_id/app_id/kb_id/acl` 必须能下推到 ES query filter。
4. 分数契约：ES 原始 BM25 分数不能直接和向量分比较，必须进入 route score normalization。
5. 路由契约：最终结果必须保留 `route/source.route/route_contrib/route_raw_scores/retriever_version`。
6. 调试契约：debug trace 必须展示 `dense_hits/sparse_hits/dense_contribution/sparse_contribution`。
7. 降级契约：ES 超时或报错时，可以继续返回 dense route，并记录 degradation reason。

## 4.6 ES 接入的阶段安排

ES 不建议放在 P0。推荐放在 P2 后半段到 P3。

1. P0-P1：先完成平台边界、多租户、API Key、权限、日志。
2. P2：抽象 `SparseEngine`，保留当前 local sparse，实现接口化。
3. P2 后半段：实现 ES / OpenSearch 只读 sparse adapter。
4. P3：支持 BYO ES 连接器、mapping 校验、健康检查、debug trace、策略配置。
5. P4：支持托管 ES/OpenSearch、索引生命周期、容量治理、跨租户隔离、审计与成本归因。

---

## 5. 后续实施路线

## Phase 0：平台 API 边界冻结与最小外部接入

目标：

> 把当前 RAG 能力从业务项目里切出清晰边界，让一个外部业务可以只通过 API 完成检索。

任务：

1. 冻结 `/v1/retrieve` API contract。
2. 复用当前 `/api/kb/retrieve` 主链路，新增产品化入口。
3. 返回结构固定为 `request_id/items/content/score/citation/source/strategy_version`。
4. 新增最小 `app_id/api_key_id` 请求身份。
5. 检索日志扩展 `app_id/api_key_id`。
6. 当前面试 Agent 改为通过 HTTP 调 `/v1/retrieve`，不再直接调用内部 Milvus Tool。
7. Admin 增加“外部应用接入”最小页面，能看到 app、API Key、请求日志。

完成标准：

1. 面试 Agent 能通过 `/v1/retrieve` 正常检索。
2. 每次检索都能通过 `request_id` 查到 debug trace。
3. 无 API Key 请求被拒绝。
4. 返回字段对外稳定，不泄漏内部模型结构。

下一步：

进入 Phase 1，补齐多业务接入与权限隔离。

## Phase 1：多业务接入、权限隔离与生产可用

目标：

> 支持 2-3 个业务通过统一 API 接入，并具备基础权限、限流、审计、成本归因能力。

任务：

1. 新增平台模型：`rag_tenant`、`rag_app`、`rag_api_key`、`rag_app_kb_permission`、`rag_app_quota`。
2. 建立 `app -> kb_ids` 授权关系。
3. 检索前强制校验 `api_key_id/app_id/kb_ids`。
4. 所有检索 filter 强制注入授权后的 `kb_id` 范围。
5. 按 app 配置 `qps_limit/timeout_ms/max_topk/daily_quota`。
6. 检索日志补齐 `tenant_id/app_id/api_key_id/kb_ids/final_filter_expr`。
7. 成本看板支持按 `app_id/kb_id/strategy_version` 聚合。
8. 输出 Go / Node / Python 最小 SDK 示例。

完成标准：

1. 至少两个业务共用同一套 RAG 平台。
2. A 业务无法检索 B 业务未授权知识库。
3. 任意请求都能按 app 追踪质量、延迟、成本和错误。
4. API Key 泄漏或禁用后可立即阻断访问。

下一步：

进入 Phase 2，开始把策略、评测、hybrid sparse backend 做成可配置平台能力。

## Phase 2：检索质量平台化与 SparseEngine 抽象

目标：

> 把当前已有 hybrid 能力升级成可配置、可评估、可回滚的检索质量平台。

任务：

1. 策略模型从全局配置升级为 `app_id/kb_id/strategy_profile/strategy_version`。
2. 支持配置 `top_k/candidate_topk/hybrid_dense_weight/hybrid_sparse_weight/rewrite/rerank/parent_child/evidence_gate`。
3. 固定 baseline/candidate/offline evaluation/rollout/rollback 闭环。
4. 抽象 `SparseEngine` 接口，当前 local sparse 变成 `LocalSparseEngine`。
5. 保持当前 local sparse 能力：`content like + BM25`。
6. 新增 ES / OpenSearch adapter 的技术设计，不直接影响默认链路。
7. 扩展 debug trace：展示 `route_raw_scores/route_contrib/sparse_backend/sparse_query/sparse_latency_ms`。
8. 建立 hybrid 评测集：实体词、缩写词、关键词强匹配、长上下文、无证据拒答。

完成标准：

1. 某个业务能在不改代码的情况下切换 hybrid 权重和 rerank 策略。
2. 每次策略发布前都有 baseline vs candidate 报告。
3. sparse route 可以通过接口替换实现，不影响 fusion/dedupe/rerank。
4. local sparse 和 ES adapter 可以共用同一套 route contribution 与 debug trace。

下一步：

进入 Phase 3，把 ES/OpenSearch 作为企业接入能力落地，并增强外部数据源适配。

## Phase 3：ES / OpenSearch 接入与企业存量搜索系统融合

目标：

> 支持企业复用已有 ES / OpenSearch 作为 sparse route，并由平台统一完成混合检索、融合、调试和治理。

范围边界：

本阶段必须做：

1. ES / OpenSearch 只读 sparse adapter。
2. BYO ES 连接配置。
3. mapping 校验与连接健康检查。
4. ES sparse route debug trace。
5. ES 超时/失败降级到 dense 或 local sparse。
6. 按 app/kb/strategy 选择 sparse backend：`local`、`es`、`opensearch`。

本阶段不做：

1. 不做完整 ES 集群运维平台。
2. 不做复杂 ES 索引自动调优。
3. 不强制所有知识库迁移到 ES。
4. 不把 ES 作为平台必选依赖。

实现路线：

1. L0：冻结 `SparseEngine` 接口。
2. L1：新增 `LocalSparseEngine`，迁移当前 `SparseRetriever` 逻辑。
3. L2：新增 `ESSparseEngine`，支持 `match/multi_match/filter/highlight` 的第一版查询。
4. L3：新增 ES connection model，保存 endpoint、auth、index、field mapping、timeout。
5. L4：新增连接健康检查：连通性、index 存在、字段存在、样例 query。
6. L5：新增策略配置：`sparse_backend=local/es/opensearch`。
7. L6：扩展 debug trace：ES query DSL、latency、raw BM25 score、hit count、error code。
8. L7：增加降级策略：ES route failed 时继续 dense route，并记录 `degradation.reason=es_route_error`。
9. L8：构建 ES 专项评测集，对比 local sparse、ES sparse、dense only、hybrid。

完成标准：

1. 一个知识库可以选择客户 ES 作为 sparse route。
2. ES 命中结果能和 dense 结果按统一分数融合。
3. debug 页面能解释“ES 命中了什么、为什么最后 sparse/dense 赢了”。
4. ES 超时不导致整个检索失败。
5. ES adapter 可以关闭并回退到 local sparse。

下一步：

进入 Phase 4，补齐产品化、治理、私有化部署和规模化交付能力。

## Phase 4：产品化交付、治理与规模化

目标：

> 把平台从内部能力升级为可交付、可部署、可演示、可运营的企业产品。

任务：

1. 输出 OpenAPI 文档与 SDK。
2. 支持 SaaS、私有化、混合云部署拓扑。
3. 支持多向量库适配：Milvus、Qdrant、pgvector、Pinecone。
4. 支持多 sparse backend：local、ES、OpenSearch。
5. 支持多 embedding / rerank provider。
6. 完善 Vector Ops：index registry、build candidate、switch active、rollback、health check。
7. 完善 Search Ops：sparse backend health、ES query latency、route contribution trend。
8. 完善 Governance：策略变更审计、权限审计、成本审计、合规导出。
9. 完善 weekly report：质量趋势、成本趋势、空召回、高风险策略、ES 慢查询。
10. 准备 Demo Space：面试知识库、客服 FAQ、内部制度、销售资料四类场景。

完成标准：

1. 新业务可在 1 天内完成 API 接入。
2. 新知识库可在 30 分钟内完成上传、入库、检索测试。
3. 新策略可在管理台配置、评测、灰度、回滚。
4. 有 ES 的企业可以复用已有 ES，没有 ES 的企业可以用 local sparse 或托管方案。
5. 平台可以独立部署给其他项目使用。

---

## 6. 推荐优先级

短期优先级：

1. `/v1/retrieve` API contract。
2. `app_id/api_key_id` 最小接入模型。
3. 检索日志扩展 app 维度。
4. 策略配置从全局迁移到 app/kb 维度。
5. `SparseEngine` 接口抽象。

中期优先级：

1. ES / OpenSearch 只读 sparse adapter。
2. BYO ES connection 配置与健康检查。
3. ES route debug trace。
4. 策略中心支持 sparse backend 选择。
5. offline evaluation 支持 sparse backend 对比。

长期优先级：

1. 多向量库适配。
2. 托管 ES/OpenSearch 方案。
3. 私有化部署模板。
4. 企业权限系统集成。
5. 合规审计与成本计费。

---

## 7. 首批可执行任务

建议第一批任务按这个顺序做：

1. 新增 `/v1/retrieve` contract 文档。
2. 新增 `rag_app`、`rag_api_key`、`rag_app_kb_permission` 数据模型草案。
3. 扩展 `KBRetrieveLog`，加入 `app_id/api_key_id/final_filter_expr/sparse_backend`。
4. 把当前 sparse 检索抽象为 `SparseEngine` 接口。
5. 将当前 `SparseRetriever` 迁移为 `LocalSparseEngine`。
6. 给 fusion/dedupe/debug trace 增加 `sparse_backend` 字段。
7. 编写 ES adapter 技术设计文档。
8. 新增 ES connection 配置模型草案。
9. 准备 hybrid 评测集，覆盖 JVM、Kafka、Redis、MySQL、Go 等当前测试文档场景。
10. 用当前面试 Agent 做一次远程 `/v1/retrieve` 接入演示。

---

## 8. 验收与回归规则

每个阶段结束必须完成：

1. 功能演示：能通过真实接口跑通。
2. 回归测试：至少覆盖检索 API、权限过滤、日志字段、debug trace。
3. 离线评测：对比 baseline/candidate 的 Recall@K、MRR、nDCG、Citation Support Score。
4. 稳定性观察：P95、错误率、empty rate、route error、timeout。
5. 成本观察：embedding cost、rerank cost、context token、ES 查询开销。
6. 回滚演练：策略、sparse backend、ES adapter 均可关闭或切回 baseline。

---

## 9. 团队对齐口径

可以用下面这段话给团队同步：

> 我们不是要把平台做成另一个 ES，也不是必须先接 ES 才能做混合检索。当前项目已经有 `dense + local sparse` 的混合检索链路。ES / OpenSearch 是企业场景中非常重要的 sparse backend，因为很多客户已经有 ES，但缺少向量检索、RAG 调试、策略治理和质量评估。我们的正确路线是先把平台边界、多租户、策略、日志和 SparseEngine 接口打牢，再把 ES 作为可插拔 sparse route 接进来。这样没有 ES 的客户可以轻量部署，有 ES 的客户可以复用存量索引，平台统一负责融合、rerank、debug、评估、审计和回滚。

---

## 10. 最终判断

后续路线的核心不是“加不加 ES”，而是“把检索能力平台化、可插拔、可观测、可治理”。

ES / OpenSearch 应该进入路线，但它的位置是：

1. 企业存量关键词检索系统连接器。
2. hybrid sparse route 的一种实现。
3. 大规模关键词召回与权限过滤能力的增强。
4. 产品化交付时降低企业接入成本的重要能力。

它不应该成为：

1. 平台的硬依赖。
2. 当前 hybrid 是否成立的前提。
3. 替代向量检索的方案。
4. 绕过平台统一 fusion、debug、evaluation、governance 的旁路系统。

推荐下一步先做 `SparseEngine` 抽象，再做 ES adapter。这样路线更稳，工程改动也更可控。
