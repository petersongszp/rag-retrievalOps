# RAG + 后台管理平台学习指南

## 一、先搞清楚这个项目在做什么

### 1. 用一句话概括项目目标

这个项目要做的事情可以概括成一句话：

**把知识库的入库、检索、评测、策略灰度、链路排障、质量和成本治理，做成一套可运行、可观测、可回滚的 RAG 平台。**

它不是一个“只会搜文档”的简单 RAG Demo，也不是一个“只会上传文件”的后台。它更像一套完整的 RAG 控制台：

- 文档怎么进来
- 向量怎么写入 Milvus
- 检索链路怎么跑
- 检索效果怎么人工调试
- 检索策略怎么评测、灰度、回滚
- 出问题以后去哪里查日志、查审计、查成本

当前仓库已经把面试项目拆成两部分：

1. Agent 面试能力
2. RAG 能力 + RAG 后台管理平台

这篇文档只关注第 2 部分。

### 2. 技术栈全景

这一部分最重要的技术栈是：

- `Go + Hertz`
  负责后端 API、入库任务、检索链路、评测、策略治理、日志聚合。
- `Next.js + React + Ant Design`
  负责后台管理平台页面，页面逻辑主要都在 `admin/src/components/admin/`。
- `Milvus`
  负责向量存储和检索，是 RAG 的核心数据面。
- `MySQL`
  负责知识库、文档、入库任务、检索日志、评测运行、审计事件等结构化数据。
- `Redis`
  负责缓存，也承担消息队列实现的底座。
- `MinIO`
  是 Milvus 的依赖对象存储，不是你上传知识库文件的主要入口，但它是 Milvus standalone 运行所需。
- `MQ/异步消费`
  文档上传后不是同步入库，而是先落库、再发消息、由消费者异步处理。
- `Prometheus + Alertmanager + Grafana`
  提供监控、告警、看板，属于平台治理侧能力。

### 3. Docker 服务拆分说明

根目录 `docker-compose.yml` 反映了整个平台的运行边界。可以把它分成 5 组来看。

#### 第一组：基础依赖

- `mysql`
- `redis`

这两个服务支撑业务数据和队列能力。

#### 第二组：Milvus 依赖栈

- `etcd`
- `minio`
- `milvus`
- `attu`

其中：

- `milvus` 是真正的向量库
- `etcd` 和 `minio` 是 Milvus standalone 的依赖
- `attu` 是 Milvus 的 GUI，可用于人工看 collection

#### 第三组：业务服务

- `backend`
- `admin`
- `frontend`
- `nginx`

这里要注意：

- `backend` 是 Go 后端，端口映射到 `8899`
- `admin` 是当前这篇文档关注的后台管理平台，容器内跑在 `3001`，宿主映射到 `3002`
- `frontend` 是面向业务侧的前台，不是这篇文档的重点
- `nginx` 统一做反向代理

#### 第四组：监控治理

- `prometheus`
- `alertmanager`
- `grafana`

这组服务说明项目目标不只是“功能跑通”，还包括“线上治理”。

#### 第五组：你学习时最常用的访问入口

- 后端 API：`http://localhost:8899`
- 管理后台：`http://localhost:3002`
- Milvus GUI：`http://localhost:8000`
- Grafana：`http://localhost:3003`

### 4. 目录结构应该怎么理解

#### `backend/`

这是 Go 后端主项目。学习时重点看这几层：

- `backend/api/`
  路由和 HTTP handler 入口。
- `backend/internal/milvus/`
  RAG 数据面核心实现，包含切分、向量化、入库、检索、fusion、rerank、Parent-Child。
- `backend/internal/rag/`
  RAG 控制面能力，包含策略状态、实验、发布、治理、索引生命周期。
- `backend/internal/mq/`
  文档入库异步消费和重试补偿。
- `backend/internal/model/`
  MySQL 模型层，能帮助你理解页面数据到底存在哪里。

#### `admin/`

这是 Next.js 管理后台。学习时重点看：

- `admin/src/app/(admin)/`
  路由层，通常只是把页面组件挂上去。
- `admin/src/components/admin/`
  真正的页面逻辑层。这个目录是你看后台功能最应该先看的地方。
- `admin/src/config/api.ts`
  前端调用的后端接口总表。
- `admin/src/types/kb.ts`
  前后端契约类型，能快速看懂页面到底依赖哪些字段。

#### `docs/`

这里已经有 5 篇后台使用文档：

1. `admin-dashboard-overview-page-guide.md`
2. `admin-knowledge-bases-page-guide.md`
3. `admin-retrieval-lab-guide.md`
4. `admin-strategy-center-guide.md`
5. `admin-trace-logs-guide.md`

这 5 篇文档更像“单模块深挖”。本文则是“先学什么、后学什么”的总学习路线。

## 二、学习路线建议（按阶段）

### 第一阶段：先跑起来，先看到效果

这一阶段不要急着读算法实现，先建立直觉。

你要做的事情：

1. 用 `docker-compose.yml` 把依赖和服务跑起来。
2. 打开后台管理平台，先看左侧菜单都有哪些模块。
3. 新建一个知识库。
4. 上传一个 `md`、`txt` 或 `pdf` 文档。
5. 在知识库详情页观察文档状态和入库任务状态。
6. 到检索实验室跑一次查询。
7. 拿 `request_id` 去调试视图和检索日志页面看看。

这一阶段的目标不是“理解代码”，而是回答下面几个最基本的问题：

- 知识库是什么
- 文档上传以后发生了什么
- 检索结果长什么样
- 后台到底能帮你看到哪些运行信息

建议先读的文件：

- `docker-compose.yml`
- `admin/src/components/admin/admin-shell.tsx`
- `admin/src/components/admin/dashboard-page.tsx`
- `admin/src/components/admin/knowledge-bases-page.tsx`
- `admin/src/components/admin/knowledge-base-detail-page.tsx`
- `admin/src/components/admin/retrieval-lab-page.tsx`

### 第二阶段：理解 RAG 核心链路

这一阶段开始真正看后端。

你要重点建立一条完整链路：

**上传文档 -> 创建入库任务 -> MQ 异步消费 -> 切分 -> 向量化 -> 写 Milvus -> 检索 -> dense/sparse 融合 -> rerank -> Parent-Child 补全 -> 返回结果**

建议按这个顺序读：

1. `backend/api/handler/kb/handler.go`
   先看 `UploadDocument`、`Retrieve`，知道请求是从哪进来的。
2. `backend/internal/mq/mq.go`
   看上传后发了什么消息。
3. `backend/internal/mq/consumer.go`
   看真正的入库执行在哪里发生。
4. `backend/internal/milvus/init.go`
   看 MilvusManager 怎么把 split / embed / index / retrieve 串起来。
5. `backend/internal/milvus/splitter/parent_child.go`
   看 Parent-Child 元数据是怎么在切分阶段打上的。
6. `backend/internal/milvus/storage/indexer.go`
   看 collection schema 和向量字段怎么建。
7. `backend/internal/milvus/retrieval/hybrid_search.go`
   看 hybrid 检索主流程。
8. `backend/internal/milvus/retrieval/fusion.go`
   看 dense / sparse 分数怎么融合。
9. `backend/internal/milvus/retrieval/reranker.go`
   看 rerank 逻辑和 fallback。
10. `backend/internal/milvus/retrieval/topk_policy.go`
    看 Dynamic TopK 和 Strategic TopK。
11. `backend/internal/milvus/retrieval/parent_child.go`
    看检索后 Parent-Child 补全如何做。

这一阶段最重要的不是记住每个函数名，而是搞清楚两件事：

1. 这个项目把“入库”和“检索”拆成了哪些阶段。
2. 每个阶段的输入、输出和风险点是什么。

### 第三阶段：理解后台管理平台

当你已经知道后端链路以后，再回来看后台页面，就不会只停留在“会点按钮”。

建议把后台理解成 3 层：

1. `操作层`
   知识库、文档、入库任务。
2. `观测层`
   检索实验室、调试视图、检索日志、入库日志、质量监控、成本看板。
3. `治理层`
   评测、策略中心、审计、告警、周报、发布回滚。

建议阅读顺序：

1. 知识库
2. 检索实验室
3. 检索日志 / 入库日志
4. 评测
5. 策略中心
6. 质量监控
7. 成本运营

这时候再去看 `docs/` 里已有的 5 篇页面文档，会非常顺。

### 第四阶段：理解策略与评测体系

这一阶段是很多新同学最容易“看功能懂了，但看设计没懂”的地方。

你要理解的是：

- 为什么不是改完检索策略就直接上线
- 为什么需要评测集、评测运行、Gate Check
- 为什么策略中心既有开关，又有版本，又有回滚
- 为什么还要区分 `phase1 / phase2 / phase3 / phase4`

建议重点读：

- `backend/internal/milvus/evaluation/profiles.go`
- `backend/internal/milvus/evaluation/gate.go`
- `backend/internal/milvus/evaluation/runner.go`
- `backend/api/handler/kb/handler_eval_run.go`
- `backend/api/handler/kb/handler_eval_report.go`
- `backend/internal/rag/phase3/contract.go`
- `backend/internal/rag/phase3admin/state.go`
- `backend/api/handler/kb/handler_strategy.go`
- `backend/api/handler/kb/handler_strategy_insights.go`
- `backend/internal/rag/release/controller.go`

这一阶段的关键认知是：

**`milvus` 目录解决“检索能力怎么做”，`rag` 目录解决“检索能力怎么安全上线、怎么观测、怎么回滚”。**

### 第五阶段：做到能改代码、能加功能

当你完成前四个阶段以后，再开始真正动代码。

推荐的练手顺序：

1. 先改一个前后端字段透传
   例如把某个检索调试字段补到前端页面。
2. 再改一个页面筛选条件
   例如给已有日志页加一个 `kb_id` 或 `status` 筛选。
3. 再改一个后端聚合口径
   例如给 metrics overview 增加一个新的趋势指标。
4. 最后再碰检索策略本身
   例如 TopK、rewrite、citation、evidence gate。

原因很简单：

- 先改字段，你会熟悉 handler、types、page 这条契约链路。
- 再改聚合，你会熟悉日志表、统计口径、页面展示。
- 最后再碰检索策略，风险最低，理解也更稳。

## 三、模块导读

下面这部分按“这个模块解决什么问题、前端入口在哪、后端入口在哪、难点是什么”来写。

### 1. 知识库管理

**解决什么问题**

把原始资料变成可检索的数据资产，包括：

- 新建知识库
- 绑定 `vector_collection`
- 上传文档
- 创建入库任务
- 追踪任务状态
- 失败重试 / 取消

**前端入口**

- `admin/src/app/(admin)/knowledge-bases/page.tsx`
- `admin/src/app/(admin)/knowledge-bases/[kbId]/page.tsx`
- `admin/src/components/admin/knowledge-bases-page.tsx`
- `admin/src/components/admin/knowledge-base-detail-page.tsx`

**后端入口**

- `backend/api/handler/kb/handler.go`
  重点看 `CreateKnowledgeBase`、`UploadDocument`、`ListDocuments`、`ListJobs`、`RetryJob`、`CancelJob`
- `backend/api/handler/kb/knowledge_base_binding.go`
  重点看 collection 绑定
- `backend/internal/mq/consumer.go`
  重点看异步入库执行

**学习重点和难点**

- 难点 1：知识库不是创建完就自动有数据，真正的数据进入 Milvus 是在异步消费者里完成的。
- 难点 2：`vector_collection` 不是纯前端概念，它会影响后续入库写到哪个 collection，也影响检索读哪个 collection。
- 难点 3：任务状态不是简单的 `pending -> completed`，还会有 `retrying`、`dead`、`canceled`。

### 2. 检索引擎

**解决什么问题**

根据 query 在一个或多个知识库里找出最相关的文档片段，并尽量让结果既相关，又可解释，还能控制成本和风险。

**前端入口**

没有独立“检索引擎源码页”，前台入口主要通过这些页面观察它：

- `admin/src/components/admin/retrieval-lab-page.tsx`
- `admin/src/components/admin/retrieval-debug-page.tsx`
- `admin/src/components/admin/retrieval-logs-page.tsx`

**后端入口**

- `backend/api/handler/kb/handler.go`
  重点看 `Retrieve`
- `backend/api/handler/kb/knowledge_base_binding.go`
  重点看多知识库、多 collection 检索如何 fan-out 再 merge
- `backend/internal/milvus/retrieval/hybrid_search.go`
  hybrid 检索总控
- `backend/internal/milvus/retrieval/fusion.go`
  dense / sparse 融合
- `backend/internal/milvus/retrieval/topk_policy.go`
  Dynamic TopK / Strategic TopK
- `backend/internal/milvus/retrieval/reranker.go`
  rerank 与 fallback
- `backend/internal/milvus/retrieval/parent_child.go`
  Parent-Child 补全
- `backend/internal/milvus/retrieval/citation_consistency.go`
- `backend/internal/milvus/retrieval/evidence_gate.go`

**学习重点和难点**

- 难点 1：这是一个多阶段链路，不是一次 Milvus 搜索就结束。
- 难点 2：dense 和 sparse 是并行召回，再做归一化和加权融合。
- 难点 3：TopK 分成 candidateTopK 和 finalTopK，两者不是一回事。
- 难点 4：Parent-Child 有两处逻辑，一处在切分时打元数据，一处在检索后补父上下文。
- 难点 5：最终结果还会经过 citation consistency 和 evidence gate，这决定是否拒答、是否降级。

### 3. 检索实验室

**解决什么问题**

给研发和运营一个“不写代码也能手动调检索”的地方，并把一次检索结果沉淀成评测样本。

**前端入口**

- `admin/src/app/(admin)/retrieval-lab/page.tsx`
- `admin/src/app/(admin)/retrieval-lab/debug/page.tsx`
- `admin/src/components/admin/retrieval-lab-page.tsx`
- `admin/src/components/admin/retrieval-debug-page.tsx`

**后端入口**

- `backend/api/handler/kb/handler.go`
  重点看 `Retrieve`、`GetRetrieveDebugView`
- `backend/api/handler/kb/retrieval_debug_trace_v2.go`
  重点看调试视图返回结构

**学习重点和难点**

- 难点 1：`request_id` 是贯穿实验室、调试视图、检索日志的主键。
- 难点 2：Contract Gap 不是业务值，而是前后端契约缺口提示。
- 难点 3：调试视图不是重新执行检索，而是读取落到日志里的 debug trace。

### 4. 评测体系

**解决什么问题**

把“我感觉这个策略更好”变成“我可以拿数据集、指标、Gate 来判断它是否能上线”。

**前端入口**

- `admin/src/app/(admin)/evaluation/datasets/page.tsx`
- `admin/src/app/(admin)/evaluation/runs/page.tsx`
- `admin/src/app/(admin)/evaluation/reports/[runId]/page.tsx`
- `admin/src/components/admin/evaluation-datasets-page.tsx`
- `admin/src/components/admin/evaluation-runs-page.tsx`
- `admin/src/components/admin/evaluation-report-page.tsx`

**后端入口**

- `backend/api/handler/kb/handler_eval_dataset.go`
- `backend/api/handler/kb/handler_eval_run.go`
- `backend/api/handler/kb/handler_eval_report.go`
- `backend/internal/milvus/evaluation/runner.go`
- `backend/internal/milvus/evaluation/gate.go`
- `backend/internal/milvus/evaluation/profiles.go`

**学习重点和难点**

- 难点 1：评测集、评测运行、评测报告是三层对象，不要混成一个概念。
- 难点 2：策略 profile 不是 UI 概念，它会真正决定检索时启用哪些能力。
- 难点 3：Gate Check 决定的是“是否允许上线”，不是“页面上显示绿色就完事了”。
- 难点 4：报告会反向依赖检索日志，因为失败样本要看 trace 是否存在。

### 5. 策略中心

**解决什么问题**

把已经实现的 Phase3 检索策略做成可查看、可变更、可回滚的治理入口。

**前端入口**

- `admin/src/app/(admin)/strategy-center/page.tsx`
- `admin/src/components/admin/strategy-center-page.tsx`

**后端入口**

- `backend/api/handler/kb/handler_strategy.go`
- `backend/api/handler/kb/handler_strategy_insights.go`
- `backend/internal/rag/phase3/contract.go`
- `backend/internal/rag/phase3admin/state.go`

**学习重点和难点**

- 难点 1：策略中心管理的是“开关和状态”，真正的检索逻辑仍然在 `backend/internal/milvus/retrieval/`。
- 难点 2：策略版本和操作日志目前主要是进程内状态，不是完整持久化治理平台。
- 难点 3：高风险策略 `RAG_ENABLE_MODEL_ASSISTED_REWRITE` 不能直接一把开到 enabled，必须先 shadow/canary。
- 难点 4：Impact 和 Gate 很依赖评测和检索日志数据，不是只靠静态配置就能得出。

### 6. 链路日志

**解决什么问题**

当检索慢、空结果、入库失败、任务被人工改状态时，你需要一个结构化排障入口，而不是去翻原始日志文件。

**前端入口**

- `admin/src/app/(admin)/trace-logs/retrieval/page.tsx`
- `admin/src/app/(admin)/trace-logs/ingest/page.tsx`
- `admin/src/components/admin/retrieval-logs-page.tsx`
- `admin/src/components/admin/ingest-logs-page.tsx`

**后端入口**

- `backend/api/handler/kb/handler.go`
  重点看 `ListRetrieveAuditLogs`、`GetRetrieveAuditLog`、`ListIngestLogs`、`GetIngestLogDetail`
- `backend/internal/model/kb_retrieve_log.go`
- `backend/internal/model/kb_ingest_job.go`
- `backend/internal/model/kb_job_operation_log.go`

**学习重点和难点**

- 难点 1：这里看到的是结构化日志表，不是 stdout 文本日志。
- 难点 2：检索日志承载了大量派生指标，例如 `empty_reason`、`dense_contribution`、`citation_support_score`。
- 难点 3：入库日志需要结合任务状态和操作审计一起看，不能只看最终失败。

### 7. 质量监控

**解决什么问题**

把“最近一次成功评测”浓缩成一个摘要页，让你快速知道当前质量是升了还是降了。

**前端入口**

- `admin/src/app/(admin)/quality-monitor/page.tsx`
- `admin/src/components/admin/quality-monitor-page.tsx`

**后端入口**

- 前端当前主要复用评测接口：
  - `GET /api/admin/kb/eval/runs`
  - `GET /api/admin/kb/eval/runs/:run_id/report`

**学习重点和难点**

- 难点 1：质量监控不是独立评测引擎，它是评测体系的摘要视图。
- 难点 2：如果没有成功评测报告，这个页面本身就会变空。
- 难点 3：这页适合运营和管理看趋势，不适合替代研发看细节。

### 8. 成本运营

**解决什么问题**

把“RAG 结果好不好”之外的另一个核心问题补上：**值不值、贵不贵、贵在哪里。**

**前端入口**

- `admin/src/app/(admin)/cost-ops/cost/page.tsx`
- `admin/src/components/admin/cost-ops-cost-page.tsx`

**后端入口**

- `backend/api/handler/kb/handler_cost.go`
- `backend/internal/model/kb_cost_trace.go`
- `backend/api/handler/kb/handler.go`
  重点看 `persistCostTrace` 和 `buildRetrieveCostTrace`

**学习重点和难点**

- 难点 1：成本页不是直接从检索结果推出来，而是从 `KBCostTrace` 聚合出来。
- 难点 2：成本和质量是联动的，TopK、上下文长度、rerank 都会影响成本。
- 难点 3：这部分已经开始进入 Phase4 治理能力，不再只是 RAG 核心算法问题。

## 四、关键代码路径速查表

| 如果你想看 | 建议先看 | 再看 |
| --- | --- | --- |
| 后台整体导航和知识库上下文 | `admin/src/components/admin/admin-shell.tsx` | `admin/src/components/admin/knowledge-base-provider.tsx` |
| Docker 服务关系 | `docker-compose.yml` | `nginx.conf` |
| 知识库列表和详情页 | `admin/src/components/admin/knowledge-bases-page.tsx` | `admin/src/components/admin/knowledge-base-detail-page.tsx` |
| 创建知识库 / 绑定 collection | `backend/api/handler/kb/handler.go` | `backend/api/handler/kb/knowledge_base_binding.go` |
| 文档上传怎么进入系统 | `backend/api/handler/kb/handler.go` 的 `UploadDocument` | `backend/internal/mq/mq.go` |
| 入库任务真正怎么执行 | `backend/internal/mq/consumer.go` 的 `handleKnowledgeIngest` | `backend/internal/milvus/init.go` |
| 文档如何切分并生成 Parent-Child 元数据 | `backend/internal/milvus/splitter/parent_child.go` | `backend/internal/milvus/splitter/splitter.go` |
| Milvus collection / schema 怎么建 | `backend/internal/milvus/storage/indexer.go` | `backend/internal/milvus/collection_resolver.go` |
| 检索主入口 | `backend/api/handler/kb/handler.go` 的 `Retrieve` | `backend/api/handler/kb/knowledge_base_binding.go` |
| dense + sparse hybrid 主流程 | `backend/internal/milvus/retrieval/hybrid_search.go` | `backend/internal/milvus/retrieval/fusion.go` |
| TopK 决策 | `backend/internal/milvus/retrieval/topk_policy.go` | `backend/internal/milvus/retrieval/hybrid_search.go` |
| rerank 和 fallback | `backend/internal/milvus/retrieval/reranker.go` | `backend/internal/milvus/retrieval/hybrid_search.go` |
| 检索后 Parent-Child 补父上下文 | `backend/internal/milvus/retrieval/parent_child.go` | `backend/internal/milvus/splitter/parent_child.go` |
| 调试视图数据怎么来的 | `backend/api/handler/kb/retrieval_debug_trace_v2.go` | `admin/src/components/admin/retrieval-debug-page.tsx` |
| 检索实验室如何做 Contract Gap 检查 | `admin/src/components/admin/retrieval-lab-page.tsx` | `backend/api/handler/kb/retrieval_debug_trace_v2.go` |
| 检索日志和监控指标 | `backend/api/handler/kb/handler.go` 的 `ListRetrieveAuditLogs` / `GetMetricsOverview` | `admin/src/components/admin/retrieval-logs-page.tsx` |
| 评测集、评测运行、评测报告 | `backend/api/handler/kb/handler_eval_dataset.go` | `backend/api/handler/kb/handler_eval_run.go`、`handler_eval_report.go` |
| Gate 阈值 | `backend/internal/milvus/evaluation/gate.go` | `backend/api/handler/kb/handler_eval_run.go` |
| 策略开关和回滚 | `backend/internal/rag/phase3/contract.go` | `backend/internal/rag/phase3admin/state.go` |
| 策略 Impact / Gate 页面 | `admin/src/components/admin/strategy-center-page.tsx` | `backend/api/handler/kb/handler_strategy_insights.go` |
| 质量监控摘要 | `admin/src/components/admin/quality-monitor-page.tsx` | `backend/api/handler/kb/handler_eval_report.go` |
| 成本看板 | `admin/src/components/admin/cost-ops-cost-page.tsx` | `backend/api/handler/kb/handler_cost.go` |
| 发布灰度和总回滚 | `backend/internal/rag/release/controller.go` | `backend/api/handler/kb/handler.go` 里的 release 相关接口 |

## 五、常见学习问题 FAQ

### 1. Milvus collection 是怎么创建的？

要分成两层看。

第一层是“知识库绑定哪个 collection”：

- 在 `backend/api/handler/kb/knowledge_base_binding.go`
- 如果知识库还没有 `vector_collection`，会分配默认名：`kb_{kbID}_docs`

第二层是“这个 collection 的 schema 什么时候真正创建”：

- 真正写入发生在 `backend/internal/mq/consumer.go` 的 `ingestKnowledgeDocument`
- 它会调用 `manager.NewIndexerServiceForCollection(...)`
- 再走到 `backend/internal/milvus/storage/indexer.go`
- `Store` 时会用定义好的字段创建 collection schema

也就是说：

**知识库创建时先拿到 collection 名，第一次真正写向量时才把 schema 落到 Milvus。**

### 2. 检索时 dense 和 sparse 是怎么融合的？

在 `backend/internal/milvus/retrieval/hybrid_search.go` 里，dense 和 sparse 是并行跑的。

跑完以后会进入 `backend/internal/milvus/retrieval/fusion.go`：

1. 先分别收集 dense 和 sparse 的原始分数
2. 对每一路做归一化
3. 按权重加权，默认大致是 dense 0.7、sparse 0.3
4. 生成统一的 fusion score
5. 再做去重、rerank、Parent-Child、TopK 截断、citation/evidence 检查

所以不要把 hybrid 理解成“Milvus 一次查询返回混合结果”。这个项目里的 hybrid 更像一个编排流程。

### 3. 评测 Gate Check 的阈值在哪里定义？

默认阈值定义在：

- `backend/internal/milvus/evaluation/gate.go`

默认值来自 `DefaultGateThresholds()`，例如：

- `MinRecallDelta = 0.08`
- `MaxP95LatencyRegressionRatio = 0.20`
- `MaxRefusalFalsePositiveRate = 0.05`

而运行时是否直接用默认值，要看：

- `backend/api/handler/kb/handler_eval_run.go`

如果创建评测运行时没有传有效阈值，就会退回默认值。

### 4. 策略中心里的“回滚”到底回滚到哪里？

这里其实有两种回滚，不要混淆。

第一种是 **Phase3 策略回滚**：

- 代码在 `backend/internal/rag/phase3admin/state.go`
- 如果目标是 `phase2_baseline`
- 就会按 `backend/internal/rag/phase3/contract.go` 里的 rollback order 依次关闭受管的 Phase3 flag

第二种是 **发布路由回滚**：

- 代码在 `backend/internal/rag/release/controller.go`
- 它控制的是请求流量回到 `phase1` 还是继续走 `phase2`

所以更准确地说：

- 策略中心回滚的是“检索策略开关状态”
- release 回滚的是“流量走哪条大链路”

### 5. Contract Gap 是怎么检测的？

也是两层机制。

第一层是前端主动检查：

- 例如 `admin/src/components/admin/retrieval-lab-page.tsx`
- 页面会明确检查 `score`、`citation.file_name`、`source.route` 等字段是否缺失
- 缺了就显示 `Contract gap`

第二层是后端直接返回 `contract_gaps`：

- 例如 `backend/api/handler/kb/retrieval_debug_trace_v2.go`
- 当 debug trace 不完整时，会直接在响应里标出缺失字段
- 成本、审计、策略 Impact 等接口也有类似设计

所以 Contract Gap 的本质不是“系统报错”，而是：

**前端预期这里应该有字段，但当前后端契约没有完整给出。**

### 6. Parent-Child 到底是哪一步做的？

也是两步。

第一步在切分阶段：

- `backend/internal/milvus/splitter/parent_child.go`

这里会给 chunk 补上：

- `parent_id`
- `chunk_id`
- `hierarchy_path`
- `section_title`
- `parent_start_offset`
- `parent_end_offset`

第二步在检索后处理阶段：

- `backend/internal/milvus/retrieval/parent_child.go`

这里会根据 `parent_id` 或 `hierarchy_path` 把父上下文或兄弟窗口补回来。

### 7. 为什么知识库上传是异步的，不直接同步入库？

因为入库过程至少包含下面几步：

1. 读文件
2. 解析文本
3. 切分
4. 向量化
5. 写 Milvus

这条链路可能慢，也可能失败，还需要重试和暂停恢复。

所以项目把它拆成：

- 用户请求只负责“创建文档记录 + 创建任务 + 发 MQ 消息”
- 真正重活放到 `backend/internal/mq/consumer.go`

这是一个典型的平台化做法。

### 8. 为什么 `backend/internal/milvus/` 和 `backend/internal/rag/` 要分开？

可以用一句话记：

- `milvus` 管“怎么检索”
- `rag` 管“怎么治理这套检索能力”

更具体一点：

- `milvus/` 偏数据面，解决切分、索引、召回、融合、rerank
- `rag/` 偏控制面，解决策略、实验、发布、回滚、治理门禁

## 六、建议的学习顺序

下面给一个比较稳的 2 周学习计划。

### 第 1 周：把一条链路看通

#### 第 1 天

- 跑起 Docker
- 打开后台
- 看左侧菜单
- 读 `admin-shell.tsx`

#### 第 2 天

- 新建知识库
- 上传一个文档
- 看知识库详情页
- 对照 `UploadDocument` 和 `handleKnowledgeIngest`

#### 第 3 天

- 在检索实验室跑查询
- 记下 `request_id`
- 去调试视图和检索日志页

#### 第 4 天

- 读 `backend/internal/milvus/init.go`
- 读 `backend/internal/milvus/storage/indexer.go`
- 读 `backend/internal/milvus/splitter/parent_child.go`

#### 第 5 天

- 读 `backend/internal/milvus/retrieval/hybrid_search.go`
- 读 `fusion.go`
- 读 `topk_policy.go`
- 读 `reranker.go`

这一周结束时，你至少应该能自己讲清楚：

**一个文档如何从上传走到 Milvus，一个 query 如何从接口走到最终结果。**

### 第 2 周：把后台和治理看通

#### 第 1 天

- 读 `evaluation-datasets-page.tsx`
- 读 `handler_eval_dataset.go`
- 理解评测集结构

#### 第 2 天

- 读 `evaluation-runs-page.tsx`
- 读 `handler_eval_run.go`
- 读 `evaluation/runner.go`

#### 第 3 天

- 读 `evaluation-report-page.tsx`
- 读 `handler_eval_report.go`
- 读 `evaluation/gate.go`

#### 第 4 天

- 读 `strategy-center-page.tsx`
- 读 `phase3/contract.go`
- 读 `phase3admin/state.go`
- 读 `handler_strategy_insights.go`

#### 第 5 天

- 读 `retrieval-logs-page.tsx`
- 读 `ingest-logs-page.tsx`
- 读 `quality-monitor-page.tsx`
- 读 `cost-ops-cost-page.tsx`

这一周结束时，你应该能回答：

- 这个项目怎么判断一个新策略能不能上线
- 上线以后怎么观察它
- 出事以后怎么回滚
- 成本和质量怎么一起看

### 进入可开发状态后的第一个练手任务

建议你自己做一个很小的改动，例如：

1. 给调试视图补一个缺失字段
2. 给检索日志页加一个筛选条件
3. 给 metrics overview 新增一个简单聚合指标

如果你能独立完成这样一个小改动，并且知道前后端、日志、类型、页面都要改哪里，基本就说明已经入门了。

## 建议配套阅读

当你看完本文以后，建议按下面顺序继续读已有文档：

1. `docs/admin-knowledge-bases-page-guide.md`
2. `docs/admin-retrieval-lab-guide.md`
3. `docs/admin-trace-logs-guide.md`
4. `docs/admin-strategy-center-guide.md`
5. `docs/admin-dashboard-overview-page-guide.md`

这个顺序的理由很简单：

- 先从“数据怎么进来”开始
- 再看“数据怎么被检索”
- 再看“出了问题怎么查”
- 再看“策略怎么治理”
- 最后回到“总览页怎么读”
