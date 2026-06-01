# RAG 后台代码剥离执行方案

## 1. 文档目的

本文用于指导后续把当前项目中的 RAG 后台能力从“面试吧业务后端”中剥离出来，形成一个独立的 RAG Platform 服务。

执行目标不是重写一套 RAG，而是把现有已经可用的知识库、入库、检索、策略、评估、成本、审计、告警能力拆出清晰边界，让其他业务后续只通过 API / SDK 接入，不再依赖面试吧 Agent 内部包、Milvus 全局对象或面试业务数据库模型。

推荐使用方式：

1. Phase 0-1 先在当前仓库内完成代码边界冻结和 Agent 直连剥离。
2. Phase 2-3 再把 RAG 服务抽成独立启动入口、独立路由、独立迁移和独立配置。
3. Phase 4-5 再做物理拆仓、SDK、部署和多业务接入。

---

## 2. 当前代码边界盘点

## 2.1 当前可直接复用的 RAG 能力

现有 RAG 后台能力主要分布在以下路径：

1. `api/router/custom_kb.go`：注册 `/api/kb` 和 `/api/admin/kb` 路由。
2. `api/handler/kb/*`：知识库管理、文档上传、检索、调试、策略、评估、成本、审计、告警、周报、Vector Ops。
3. `internal/milvus/*`：Milvus 初始化、collection 解析、导入、向量写入、检索、hybrid retrieval、rerank、rewrite、parent-child、evidence gate、citation consistency。
4. `internal/rag/*`：release、experiment、governance、index lifecycle、phase3 策略状态。
5. `internal/model/kb_*`：知识库、文档、入库任务、检索日志、索引、评估、成本、审计模型。
6. `internal/service/kb/*`：评估执行等 RAG service 能力。
7. `internal/mq` 中的 `MessageTypeKnowledgeIngest`、`KnowledgeIngestPayload` 和 consumer 中的知识入库链路。
8. `internal/observability/metrics/rag_metrics.go`：RAG 指标采集。
9. `cmd/retrieval-eval`、`cmd/retrieval-benchmark`：离线评估和 benchmark 工具。
10. `deploy/monitoring`：RAG Prometheus / Grafana / Alertmanager 配置。

这些属于未来 RAG Platform 的核心资产，原则上保留并迁移。

## 2.2 当前必须剥离的面试吧业务能力

以下路径属于面试吧业务，不应进入独立 RAG Platform：

1. `internal/agents/interview/*`
2. `internal/agents/evaluation/*`
3. `internal/agents/resume/*`
4. `internal/agents/prediction/*`
5. `internal/agents/usecase/*`
6. `internal/service/interview/*`
7. `internal/service/resume/*`
8. `internal/service/prediction/*`
9. `api/handler/interview/*`
10. `api/handler/resume/*`
11. `api/handler/payment/*`
12. `api/router/interview/*`
13. `api/router/payment/*`
14. `internal/payment/*`
15. `internal/model/interview_*`、`internal/model/resume.go`、`internal/model/prediction.go`、`internal/model/payment_*`、`internal/model/subscription.go`
16. `idl/interview/*`、`idl/resume/*`、`idl/prediction/*`、支付相关 IDL 或生成代码。

这些模块后续只作为外部业务方存在，通过 RAG Platform 的公开 API 调用检索能力。

## 2.3 当前最关键耦合点

当前不建议直接搬目录，因为存在四类耦合：

1. 启动耦合：`cmd/server/main.go` 同时初始化 DB、Redis、MQ、Milvus、支付、LLM TokenQuota、面试路由、RAG 路由。
2. 模型耦合：`internal/repository/database.go` 在同一个 `AutoMigrate` 中迁移用户、面试、简历、支付、KB 全部表。
3. MQ 耦合：`internal/mq/consumer.go` 同时消费简历解析、评估报告、topic evaluation、knowledge ingest。
4. Agent 直连耦合：`internal/agents/tools/milvus_retriever_tool.go` 直接调用 `milvus.GetMilvusManager()`，面试 Agent 通过工具绕过 RAG API。

拆分时优先处理第 4 类耦合，再处理启动、迁移、MQ，最后物理拆仓。

---

## 3. 独立后目标边界

## 3.1 RAG Platform 对外只暴露接口

第一版公开 API 建议固定为：

1. `POST /v1/retrieve`
2. `POST /v1/ingest/documents`
3. `GET /v1/ingest/jobs/:job_id`
4. `GET /v1/kbs`
5. `POST /v1/kbs`
6. `POST /v1/apps`
7. `POST /v1/api-keys`
8. `GET /v1/retrieve/audit/:request_id`

兼容路由保留：

1. `/api/kb/*`
2. `/api/admin/kb/*`

但兼容路由只能作为过渡期入口，所有新业务必须使用 `/v1/*`。

## 3.2 面试吧 Agent 的目标接入方式

面试吧 Agent 不再导入 `internal/milvus`，也不再使用 `get_milvus_retriever` 直连工具。

目标调用链：

```text
面试吧 Agent
  -> RAG SDK / HTTP Client
  -> POST /v1/retrieve
  -> RAG Platform
  -> 返回 request_id/items/citation/source/strategy_version
  -> Agent 把 evidence 注入自己的 prompt / workflow
```

Agent 侧只保留一个业务工具，例如：

```text
get_rag_retrieve
```

它的职责是调用 HTTP API，不拥有 Milvus、embedding、hybrid、rerank、strategy 的内部知识。

## 3.3 独立服务内部分层

建议拆成以下工程层：

```text
cmd/rag-server
  -> api/router/rag
  -> api/handler/rag
  -> internal/ragplatform/application
  -> internal/ragplatform/domain
  -> internal/ragplatform/repository
  -> internal/retrieval
  -> internal/vectorstore
  -> internal/ingest
  -> internal/observability
```

第一阶段不强制一次改成以上目录，但新代码必须朝这个方向收敛，禁止继续把新逻辑堆进 `api/handler/kb/handler.go`。

---

## 4. 代码拆分总路线

推荐按 L0-L8 执行：

1. L0：冻结边界和建立防回流规则。
2. L1：新增 `/v1/retrieve` API Contract。
3. L2：切断 Agent 直连 Milvus，改为 HTTP 调用 RAG API。
4. L3：抽出 RAG application service，瘦身 `api/handler/kb`。
5. L4：拆分启动入口，新增 `cmd/rag-server`。
6. L5：拆分数据模型和迁移，只迁移 RAG 表。
7. L6：拆分 MQ consumer，只保留 RAG ingest 消费链路。
8. L7：拆分配置、部署、监控和健康检查。
9. L8：物理拆仓 / 模块改名 / SDK 输出。

执行原则：

1. 每一层都必须可编译、可回滚。
2. 先做适配层，再替换调用方，最后删除旧入口。
3. 不允许 Agent 包反向导入 RAG 内部实现。
4. 不允许 RAG Platform 导入 `internal/agents/*`、`internal/service/interview/*`、`internal/payment/*`。
5. 每个阶段都要保留一条兼容路径，直到新路径完成验证。

---

## 5. L0：边界冻结与依赖检查

## 5.1 目标

先建立“哪些包属于 RAG，哪些包属于业务”的硬边界，避免后续边拆边长出新耦合。

## 5.2 任务

1. 新增文档维护 RAG 归属目录和业务归属目录。
2. 新增依赖检查脚本或 CI 检查规则。
3. 检查 RAG 包不能导入：
   - `internal/agents/*`
   - `internal/service/interview/*`
   - `internal/service/resume/*`
   - `internal/service/prediction/*`
   - `internal/payment/*`
4. 检查业务 Agent 不能导入：
   - `internal/milvus/*`
   - `internal/rag/*`
   - `api/handler/kb/*`
5. 标记旧直连工具 `get_milvus_retriever` 为 deprecated。

## 5.3 建议新增文件

1. `backend/docs/rag-platform-code-extraction-execution-plan.md`
2. `backend/scripts/check_rag_boundaries.ps1`
3. `backend/scripts/check_rag_boundaries.sh`

## 5.4 验收 Gate

1. 能列出所有 Agent -> Milvus 的直接导入点。
2. 能列出所有 RAG -> 业务模块的反向导入点。
3. 新增检查脚本在当前代码上可以输出已知问题清单。
4. 后续新增代码若违反边界，CI 能失败。

---

## 6. L1：新增 `/v1/retrieve` API Contract

## 6.1 目标

在不移动现有检索核心逻辑的前提下，新增平台化公开入口，为后续 Agent 改造提供稳定接口。

## 6.2 请求结构

```json
{
  "app_id": "mianshiba-agent",
  "kb_id": 1,
  "kb_ids": [1, 2],
  "query": "Go map 底层结构是什么？",
  "top_k": 5,
  "strategy_profile": "default",
  "metadata_filter": {
    "scene": "interview"
  }
}
```

## 6.3 响应结构

```json
{
  "request_id": "uuid",
  "items": [
    {
      "content": "chunk text",
      "score": 0.82,
      "citation": {
        "kb_id": 1,
        "document_id": 10,
        "chunk_id": "10-3",
        "file_name": "go-map.md",
        "chunk_index": 3
      },
      "source": {
        "route": "hybrid",
        "collection": "kb_1",
        "retriever_version": "hybrid-v1"
      }
    }
  ],
  "strategy_version": "baseline",
  "request_cost": {
    "estimated_cost": 0
  }
}
```

## 6.4 任务

1. 新增 `api/router/rag_public.go` 注册 `/v1/retrieve`。
2. 新增 `api/handler/rag/retrieve.go`，不要继续追加到 `api/handler/kb/handler.go`。
3. 第一版内部复用现有 `kb.Retrieve` 的主链路，但抽出可调用 service，避免 handler 调 handler。
4. 新增 `app_id` 字段，第一版可先通过配置白名单或静态映射校验。
5. 扩展 `KBRetrieveLog`，预留 `tenant_id`、`app_id`、`api_key_id` 字段。
6. 兼容 `/api/kb/retrieve`、`/api/admin/kb/retrieve`，但日志中标记 `source_api=legacy_kb`。

## 6.5 验收 Gate

1. `POST /v1/retrieve` 可返回与 `/api/kb/retrieve` 等价的检索结果。
2. 响应中固定包含 `request_id`、`items`、`citation`、`source`。
3. 检索日志能记录 `app_id` 和 `source_api`。
4. 旧路由不受影响。

---

## 7. L2：剥离面试吧 Agent 直连 Milvus

## 7.1 目标

让面试吧 Agent 成为 RAG Platform 的第一个外部调用方，不再使用内部 Milvus Tool。

## 7.2 当前要替换的直连点

重点替换：

1. `internal/agents/tools/milvus_retriever_tool.go`
2. `internal/agents/interview/specialized/*_agent.go`
3. `internal/agents/interview/comprehensive/*_agent.go`
4. `internal/agents/resume/resume.go`
5. `internal/agents/evaluation/*_agent.go`
6. `internal/agents/multiagent/orchestrator.go`

这些代码当前通过 `tool2.GetMilvusRetrieverTool()` 获取内部工具，后续应改成 `GetRAGRetrieveTool()`。

## 7.3 任务

1. 新增 `internal/agents/tools/rag_retrieve_tool.go`。
2. 新工具只依赖 HTTP client 和配置：
   - `rag_platform.base_url`
   - `rag_platform.api_key`
   - `rag_platform.app_id`
   - `rag_platform.default_kb_ids`
3. 新工具请求 `/v1/retrieve`。
4. 工具返回给 Agent 的格式保持接近旧工具：
   - `documents`
   - `count`
   - `request_id`
   - `error`
5. 所有 Agent 构造函数从 `GetMilvusRetrieverTool()` 切到 `GetRAGRetrieveTool()`。
6. 保留旧工具一个版本，但默认不开启，只作为回滚开关。

## 7.4 回滚策略

新增配置：

```yaml
rag_platform:
  enabled: true
  use_remote_retrieve: true
  fallback_to_local_milvus_tool: false
```

如果 `/v1/retrieve` 出现生产故障，可以临时将 `fallback_to_local_milvus_tool=true`，但该开关必须有下线日期，不能长期保留。

## 7.5 验收 Gate

1. `rg "GetMilvusRetrieverTool|internal/milvus" backend/internal/agents` 不再出现生产代码依赖。
2. 面试 Agent 检索时日志能看到 `/v1/retrieve` 的 `request_id`。
3. RAG 后台关闭时，Agent 得到明确错误，不静默降级为空结果。
4. 旧工具关闭后，面试 Agent 单测和主要流程仍可运行。

---

## 8. L3：抽出 RAG Application Service

## 8.1 目标

把 handler 中的检索编排、日志写入、策略决策、权限校验抽到 service 层，让 API 层只负责参数绑定和响应转换。

## 8.2 建议新增目录

```text
internal/ragplatform/application/retrieve_service.go
internal/ragplatform/application/ingest_service.go
internal/ragplatform/application/kb_service.go
internal/ragplatform/application/audit_service.go
internal/ragplatform/domain/retrieve_contract.go
internal/ragplatform/domain/kb_contract.go
internal/ragplatform/repository/kb_repository.go
internal/ragplatform/repository/retrieve_log_repository.go
```

## 8.3 任务

1. 抽出 `RetrieveService.Retrieve(ctx, req)`。
2. 抽出 `KnowledgeBaseService`，承接 create/list/delete/upload/job 操作。
3. 抽出 `RetrieveAuditService`，承接 request debug 和 audit log 查询。
4. 抽出 `StrategyService`，承接 release / experiment / strategy 状态。
5. `api/handler/kb` 和 `api/handler/rag` 共用同一组 service。
6. service 层不得依赖 `github.com/cloudwego/hertz/pkg/app`。
7. domain contract 不直接暴露 GORM model，避免 API Contract 被表结构绑死。

## 8.4 验收 Gate

1. `Retrieve` 主链路可以被 handler 和单测直接调用。
2. handler 文件只保留参数解析、错误映射、响应输出。
3. `api/handler/kb/handler.go` 不再继续增长，后续逐步拆小。
4. 新 service 不导入面试 Agent、支付、简历、预测模块。

---

## 9. L4：新增独立启动入口 `cmd/rag-server`

## 9.1 目标

当前仓库内先跑出一个只服务 RAG 的独立进程，证明 RAG 后台不依赖面试吧业务启动。

## 9.2 任务

1. 新增 `cmd/rag-server/main.go`。
2. 只初始化：
   - config
   - DB
   - Redis
   - RAG MQ
   - Milvus / vector store
   - RAG metrics
   - RAG routes
3. 不初始化：
   - payment provider
   - interview router
   - prediction router
   - resume router
   - ASR router
   - Agent runtime
4. 新增 `api/router/rag_register.go`，只注册 `/v1/*`、`/api/kb/*`、`/api/admin/kb/*`。
5. 新增 `/healthz`、`/readyz`：
   - `/healthz` 检查进程存活。
   - `/readyz` 检查 DB、Redis、Milvus、active collection。
6. 保留旧 `cmd/server`，但把 RAG 注册改为可配置：
   - `rag.run_mode=embedded`
   - `rag.run_mode=external`

## 9.3 验收 Gate

1. `go run ./cmd/rag-server` 可以单独启动。
2. 关闭面试相关配置后，RAG 服务仍可完成知识库检索。
3. `cmd/rag-server` 不导入 `internal/agents/*`、`internal/payment/*`。
4. 面试吧原 `cmd/server` 可通过配置切换到远程 RAG。

---

## 10. L5：拆分数据模型与迁移

## 10.1 目标

RAG Platform 只迁移和管理 RAG 自己的表，避免独立部署时还创建面试、简历、支付表。

## 10.2 第一批保留表

1. `kb_knowledge_base`
2. `kb_document`
3. `kb_ingest_job`
4. `kb_job_operation_log`
5. `kb_index_registry`
6. `kb_index_operation_log`
7. `kb_retrieve_log`
8. `kb_cost_trace`
9. `kb_audit_event`
10. `kb_eval_dataset`
11. `kb_eval_case`
12. `kb_eval_run`

## 10.3 平台化新增表

1. `rag_tenant`
2. `rag_app`
3. `rag_api_key`
4. `rag_app_kb_permission`
5. `rag_app_quota`
6. `rag_strategy_profile`
7. `rag_strategy_version`
8. `rag_strategy_release`
9. `rag_strategy_operation_log`

## 10.4 任务

1. 将 `repository.migrateDatabase()` 拆成：
   - `migrateBusinessDatabase()`
   - `migrateRAGDatabase()`
2. `cmd/rag-server` 只调用 `migrateRAGDatabase()`。
3. `cmd/server` 过渡期仍可调用全量迁移。
4. `KBKnowledgeBase.UserID` 第一阶段保留，新增 `TenantID`、`AppID`，不要马上删除。
5. 检索权限从 `user_id` 逐步切到 `app_id + kb_id` 授权。
6. `KBRetrieveLog` 新增 `tenant_id`、`app_id`、`api_key_id`、`source_api`。

## 10.5 验收 Gate

1. 空库启动 `cmd/rag-server` 只产生 RAG 相关表。
2. 旧业务服务启动后仍能访问原业务表。
3. `/v1/retrieve` 权限校验不依赖面试用户 ID。
4. 未授权 app 访问 kb 时直接 403，不做 fallback。

---

## 11. L6：拆分 MQ Consumer 与入库链路

## 11.1 目标

RAG 服务只消费知识入库任务，不再承载简历解析、评估报告、topic evaluation 等业务任务。

## 11.2 任务

1. 新增 `internal/ingest/consumer.go`，只处理 `knowledge_ingest`。
2. 新增 `internal/ingest/publisher.go`，承接文档入库任务发布。
3. `internal/mq` 保留通用接口，但业务消息类型下沉到业务服务。
4. `cmd/rag-server` 只启动 RAG ingest consumer。
5. `cmd/server` 过渡期可继续启动混合 consumer。
6. 后续拆仓时，把 `MessageTypeResumeParse`、`MessageTypeEvaluationReport`、`MessageTypeTopicEvaluation` 留在面试吧服务。

## 11.3 验收 Gate

1. `cmd/rag-server` 不导入 `internal/agents/usecase/*`。
2. 文档上传后 `knowledge_ingest` 能被 RAG consumer 消费。
3. 简历解析消息不会被 RAG 服务消费。
4. 入库暂停、恢复、重试、取消能力保持可用。

---

## 12. L7：拆分配置、部署和监控

## 12.1 目标

让 RAG Platform 拥有独立配置文件、容器、健康检查、监控面板和告警规则。

## 12.2 建议新增配置

```yaml
server:
  service_name: rag-platform
  host: 0.0.0.0
  port: 8081

rag_platform:
  public_api_enabled: true
  legacy_api_enabled: true
  api_key_required: true

database:
  dsn: "${RAG_DB_DSN}"

redis:
  address: "${RAG_REDIS_ADDR}"

vector_store:
  provider: milvus
  milvus:
    address: "${MILVUS_ADDR}"
    database: default

auth:
  api_key_header: X-RAG-API-Key
```

## 12.3 任务

1. 新增 `config.rag.example.yaml`。
2. 新增 `Dockerfile.rag` 或 Dockerfile target。
3. 新增 `deploy/rag-platform/*`。
4. Prometheus 指标加上 `app_id`、`tenant_id`、`source_api` label，注意控制 label 基数。
5. Grafana 面板按 app / kb / strategy 过滤。
6. 告警规则按 app 维度支持 P95、错误率、空召回率、成本异常。

## 12.4 验收 Gate

1. RAG 服务可以独立容器启动。
2. `/metrics` 不依赖面试服务。
3. Dashboard 可以区分面试吧 Agent 和其他 app。
4. API Key 错误、未授权 KB、Milvus 不可用都有明确告警或日志。

---

## 13. L8：物理拆仓与 SDK 输出

## 13.1 目标

在当前仓库内完成边界稳定后，再把 RAG 迁移成独立仓库或独立 Go module，避免过早拆仓导致双边改动成本爆炸。

## 13.2 建议新仓结构

```text
rag-platform/
  cmd/rag-server
  api
  internal/ragplatform
  internal/retrieval
  internal/vectorstore
  internal/ingest
  internal/model
  internal/repository
  internal/config
  internal/observability
  pkg/ragsdk-go
  docs
  deploy
  scripts
```

## 13.3 任务

1. 修改 Go module 名称，例如 `module rag-platform`。
2. 将 `internal/milvus` 逐步改名为 `internal/vectorstore/milvus`。
3. 将 `api/handler/kb` 逐步改名为 `api/handler/ragadmin` 和 `api/handler/ragpublic`。
4. 输出 Go SDK：
   - `Retrieve(ctx, req)`
   - `UploadDocument(ctx, req)`
   - `GetJob(ctx, jobID)`
5. 输出 HTTP Quickstart。
6. 面试吧仓库删除对 RAG 内部包的所有导入，只保留 SDK 或 HTTP client。

## 13.4 验收 Gate

1. 新仓不包含 `internal/agents`、`internal/payment`、`internal/service/interview`。
2. 面试吧服务仅通过 SDK / HTTP 调用 RAG。
3. 新业务可在 1 天内完成接入：申请 app、创建 api key、授权 kb、调用 retrieve。
4. 旧 `/api/kb/*` 兼容路由有明确下线计划。

---

## 14. 推荐执行顺序

## 14.1 第一批任务

优先做这 8 件事：

1. 新增 `/v1/retrieve`，复用现有检索主链路。
2. 给 `KBRetrieveLog` 增加 `app_id`、`api_key_id`、`source_api`。
3. 新增静态 `app_id -> kb_ids` 授权配置。
4. 新增 `rag_retrieve_tool.go`，让面试 Agent 通过 HTTP 调用 `/v1/retrieve`。
5. 把所有 Agent 的 `GetMilvusRetrieverTool()` 替换为 `GetRAGRetrieveTool()`。
6. 把 `RetrieveService` 从 handler 中抽出。
7. 新增 `cmd/rag-server`，只注册 RAG 路由。
8. 拆 `migrateRAGDatabase()`，让 RAG 服务不迁移业务表。

## 14.2 可以并行的任务

1. API Contract 文档和 OpenAPI 草案。
2. API Key / app / tenant 数据模型。
3. Agent HTTP Tool 改造。
4. RAG Service 抽层。
5. Docker / deploy / monitoring 草案。

## 14.3 不建议提前做的任务

1. 一开始就物理拆仓。
2. 一开始就删除 `/api/kb/*` 旧路由。
3. 一开始就删除 `user_id` 字段。
4. 一开始就替换 Milvus 为多向量库适配。
5. 一开始就做复杂租户计费。

原因是这些动作都会扩大回归面。先让“面试 Agent 通过 API 使用 RAG”跑通，独立化才有可验证闭环。

---

## 15. 阶段验收模板

每完成一个阶段，按以下模板验收：

```text
阶段：
目标：
涉及目录：
新增接口：
废弃接口：
新增配置：
新增表/字段：
兼容策略：
回滚方式：
单测：
集成测试：
人工验收：
风险项：
下一阶段入口条件：
```

最低验收要求：

1. `go test ./...` 或阶段相关 package 测试通过。
2. `/v1/retrieve` 返回结构稳定。
3. RAG 服务日志能用 `request_id` 串联请求。
4. Agent 侧没有直接 Milvus 依赖。
5. 未授权 KB 访问被拒绝。
6. 旧业务路径可回滚。

---

## 16. 风险与应对

## 16.1 一次性拆仓风险

风险：Go internal 包、配置、迁移、MQ、路由、部署同时变化，容易无法定位问题。

应对：先在当前仓库内跑出 `cmd/rag-server`，再拆仓。

## 16.2 Agent 检索质量回退

风险：从本地工具改为 HTTP API 后，Agent prompt 接收的字段变化，可能影响回答质量。

应对：`GetRAGRetrieveTool()` 的输出第一版尽量兼容旧 `MilvusRetrieverOutput`，新增 `request_id` 而不是重写所有 prompt。

## 16.3 权限遗漏导致知识库串库

风险：平台化后多个 app 共用 RAG，若 `kb_ids` 未强制授权，可能出现 A 业务访问 B 业务知识库。

应对：所有检索必须执行 `app_id -> kb_ids` 校验；无授权直接 403；检索日志记录最终 filter expression。

## 16.4 旧路由长期存在

风险：旧 `/api/kb/*` 一直被新业务使用，平台边界无法稳定。

应对：旧路由只给面试吧兼容期使用，新业务只允许 `/v1/*`；日志记录 `source_api`，每周检查 legacy 调用量。

## 16.5 handler 继续膨胀

风险：继续在 `api/handler/kb/handler.go` 追加逻辑，后续拆分成本继续升高。

应对：新增功能必须落到 service / domain / repository，handler 只做 transport 层。

---

## 17. 最终完成标准

当以下条件全部满足，可以认为 RAG 后台已经完成第一轮独立化：

1. RAG Platform 可以通过 `cmd/rag-server` 独立启动和部署。
2. 面试吧 Agent 只通过 `/v1/retrieve` 或 SDK 调用 RAG。
3. RAG 服务不导入 `internal/agents/*`、`internal/payment/*`、`internal/service/interview/*`。
4. RAG 数据库迁移只包含 RAG 表。
5. RAG MQ consumer 只处理知识入库任务。
6. `/v1/retrieve` 支持 `app_id`、API Key、KB 授权、标准响应、request audit。
7. Admin 能按 `app_id` 查看请求日志、成本、策略、调试链路。
8. 新业务不需要理解 Milvus / embedding / hybrid / rerank，只需申请 API Key 并调用接口。

最终代码关系应变成：

```text
mianshiba backend
  -> rag sdk / http client
  -> rag-platform /v1/retrieve

rag-platform
  -> ingest
  -> retrieval core
  -> vector store
  -> strategy / eval / cost / audit / ops
```

这就是后续执行代码剥离时的主线：先把调用边界变成 API，再把服务边界变成进程，最后再把仓库边界变成独立产品。
