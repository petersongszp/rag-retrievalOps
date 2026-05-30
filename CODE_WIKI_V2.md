# 面试吧 (Interview Bar) — Code Wiki V2

> 基于 Hertz + Eino 的 AI 智能面试平台，面向美加市场出海
>
> V2 新增：企业级 RAG 知识库系统 + 管理后台 + 评估/实验/治理平台

---

## 1. 项目概览

| 维度 | 说明 |
|------|------|
| **产品定位** | AI 驱动的模拟面试系统 + 企业级 RAG 知识库管理平台 |
| **Go Module** | `interview-agents` |
| **Go 版本** | 1.25.1 |
| **后端框架** | [Hertz](https://github.com/cloudwego/hertz)（字节跳动高性能 HTTP 框架） |
| **AI 框架** | [Eino](https://github.com/cloudwego/eino)（字节跳动 LLM 应用编排框架） |
| **前端框架** | Next.js 14+ / React / TypeScript / Tailwind CSS / Ant Design |
| **管理后台** | 独立 Next.js 应用（端口 3001） |
| **数据库** | MySQL (GORM) + Redis + Milvus（向量数据库） |
| **消息队列** | Redis Stream |
| **认证** | JWT + GitHub OAuth + Google OAuth |
| **支付** | Stripe + PayPal（策略模式 + 注册中心） |
| **监控** | Prometheus + Grafana + 飞书告警 |
| **部署** | Docker Compose + Nginx 反向代理 |

---

## 2. 系统架构总览

```
┌──────────────────────────────────────────────────────────────────────────┐
│                          Nginx (反向代理)                                │
│             :81 → Frontend / :3001 → Admin / :8899 → Backend            │
└─────┬───────────────────────────┬──────────────────────────┬────────────┘
      │                           │                          │
┌─────▼──────────┐     ┌──────────▼──────────┐    ┌─────────▼──────────────┐
│ Frontend (用户端)│     │  Admin (管理后台)    │    │  Backend (Hertz+Eino)  │
│ Next.js         │     │  Next.js + Antd     │    │                        │
│ - 面试功能       │     │  - 知识库管理        │    │  ┌──────────────────┐  │
│ - 用户中心       │     │  - 检索调试          │    │  │ API Layer        │  │
│ - 简历管理       │     │  - 评估/实验平台     │    │  │ (Handler+Router) │  │
│ - 支付           │     │  - 策略中心          │    │  ├──────────────────┤  │
└────────────────┘     │  - 成本/审计/告警    │    │  │ Service Layer    │  │
                       │  - 治理/报告         │    │  │ ├─ Interview     │  │
                       └─────────────────────┘    │  │ ├─ RAG KB       │  │
                                                  │  │ ├─ Payment      │  │
                                                  │  │ ├─ Evaluation   │  │
                                                  │  │ └─ User/Resume  │  │
                                                  │  ├──────────────────┤  │
                                                  │  │ Agent Layer      │  │
                                                  │  │ (Eino Agents)    │  │
                                                  │  ├──────────────────┤  │
                                                  │  │ Retrieval Layer  │  │
                                                  │  │ ├─ Hybrid Search │  │
                                                  │  │ ├─ Query Rewrite │  │
                                                  │  │ ├─ Reranker      │  │
                                                  │  │ ├─ Parent-Child  │  │
                                                  │  │ ├─ Evidence Gate │  │
                                                  │  │ └─ Citation Check│  │
                                                  │  ├──────────────────┤  │
                                                  │  │ Governance Layer │  │
                                                  │  │ ├─ Experiment    │  │
                                                  │  │ ├─ Release Ctrl  │  │
                                                  │  │ ├─ Index Lifecycle│ │
                                                  │  │ └─ Audit/Alert   │  │
                                                  │  ├──────────────────┤  │
                                                  │  │ Repository Layer │  │
                                                  │  │ ├─ MySQL (GORM)  │  │
                                                  │  │ ├─ Redis         │  │
                                                  │  │ └─ Milvus        │  │
                                                  │  └──────────────────┘  │
                                                  └────────┬───────────────┘
                                                           │
                              ┌─────────────────────┬──────┴──────┬───────────────┐
                              │                     │             │               │
                      ┌───────▼──────┐    ┌────────▼──────┐ ┌───▼────┐  ┌───────▼──────┐
                      │    MySQL      │    │    Redis       │ │ Milvus │  │  Prometheus  │
                      │  (GORM ORM)   │    │  (Cache/MQ)   │ │(向量库) │  │  + Grafana   │
                      └──────────────┘    └───────────────┘ └────────┘  └──────────────┘
```

---

## 3. 目录结构详解

```
mianshiba-eino-open-source/
├── admin/                              # ★ V2 新增：知识库管理后台
│   ├── src/
│   │   ├── app/
│   │   │   └── (admin)/               #   Admin 路由组（嵌套布局）
│   │   │       ├── dashboard/          #     仪表盘
│   │   │       ├── knowledge-bases/    #     知识库管理
│   │   │       ├── retrieval-lab/      #     检索实验室（Debug）
│   │   │       ├── strategy-center/    #     策略中心
│   │   │       ├── evaluation/         #     评估系统
│   │   │       ├── cost-ops/           #     成本运营
│   │   │       ├── vector-ops/         #     向量运维
│   │   │       ├── trace-logs/         #     日志追踪
│   │   │       ├── audit/              #     审计中心
│   │   │       ├── alerts/             #     告警管理
│   │   │       ├── quality-monitor/    #     质量监控
│   │   │       ├── reports/            #     报告管理
│   │   │       └── layout.tsx          #     Admin 布局
│   │   ├── components/admin/           #   Admin 组件
│   │   ├── config/                     #   API 配置
│   │   ├── services/api/               #   API 客户端
│   │   └── types/kb.ts                 #   类型定义（900+ 行）
│   ├── test_data/                      #   测试数据
│   ├── package.json                    #   Next.js 14 + Ant Design 5
│   └── vitest.config.ts                #   测试配置
│
├── backend/                            # 后端 Go 服务
│   ├── cmd/
│   │   ├── server/main.go              # ★ 服务唯一入口
│   │   ├── retrieval-benchmark/        #   检索基准测试 CLI
│   │   └── retrieval-eval/             #   检索评估 CLI
│   │
│   ├── api/                            # 接口层（HTTP 边界）
│   │   ├── handler/
│   │   │   ├── interview/              #     面试 Handler
│   │   │   ├── kb/                     # ★ V2 新增：知识库 Handler（30+ 文件）
│   │   │   │   ├── handler.go          #       核心 CRUD + 检索
│   │   │   │   ├── handler_eval_*.go   #       评估系统
│   │   │   │   ├── handler_strategy*.go#       策略中心
│   │   │   │   ├── handler_experiment.go#      实验平台
│   │   │   │   ├── handler_cost.go     #       成本运营
│   │   │   │   ├── handler_audit.go    #       审计中心
│   │   │   │   ├── handler_alerts.go   #       告警管理
│   │   │   │   ├── handler_governance_gate.go # 治理门禁
│   │   │   │   ├── handler_index_lifecycle.go # 索引生命周期
│   │   │   │   ├── handler_weekly_report.go   # 周报
│   │   │   │   ├── handler_refusal.go  #       证据拒绝
│   │   │   │   └── retrieval_debug_trace_v2.go# 调试追踪
│   │   │   ├── payment/                #     支付 Handler
│   │   │   └── resume/                 #     简历 Handler
│   │   ├── model/                      #   API 层 DTO
│   │   ├── router/
│   │   │   ├── custom_kb.go            # ★ V2 新增：60+ KB 路由注册
│   │   │   └── ...
│   │   └── response/                   #   统一响应格式
│   │
│   ├── internal/                       # ★ 私有业务逻辑
│   │   ├── agents/                     #   AI 智能体层
│   │   │   ├── evaluation/             #     评估智能体
│   │   │   ├── interview/              #     面试智能体
│   │   │   ├── llm/                    #     LLM Provider 抽象
│   │   │   ├── multiagent/             #     多智能体编排
│   │   │   ├── prediction/             #     预测智能体
│   │   │   ├── resume/                 #     简历智能体
│   │   │   ├── tools/                  #     Agent 工具
│   │   │   └── usecase/                #     业务用例
│   │   │
│   │   ├── milvus/                     #   向量数据库
│   │   │   ├── retrieval/              # ★ V2 大幅增强：检索管线
│   │   │   │   ├── hybrid_search.go    #       混合检索（Dense+Sparse）
│   │   │   │   ├── sparse_search.go    #       倒排索引 BM25 检索
│   │   │   │   ├── sparse_inverted_index.go # 倒排索引实现
│   │   │   │   ├── rewrite.go          #       查询改写（规则/领域词/路由/模型）
│   │   │   │   ├── rewrite_sources.go  #       改写来源追踪
│   │   │   │   ├── reranker.go         #       重排序（Jaccard Reranker）
│   │   │   │   ├── fusion.go           #       融合去重
│   │   │   │   ├── dedupe.go           #       文档去重
│   │   │   │   ├── filter.go           #       结果过滤
│   │   │   │   ├── topk_policy.go      #       动态 TopK 策略
│   │   │   │   ├── parent_child.go     #       父子文档填充
│   │   │   │   ├── evidence_gate.go    #       证据门禁（拒绝回答）
│   │   │   │   ├── citation_consistency.go # 引用一致性校验
│   │   │   │   ├── debug_trace.go      #       调试追踪结构
│   │   │   │   ├── retriever.go        #       基础检索服务
│   │   │   │   ├── search.go           #       搜索实现
│   │   │   │   └── options.go          #       检索选项
│   │   │   ├── evaluation/             # ★ V2 新增：评估引擎
│   │   │   │   ├── runner.go           #       评估运行器
│   │   │   │   ├── gate.go             #       质量门禁
│   │   │   │   ├── metrics.go          #       评估指标
│   │   │   │   ├── profiles.go         #       策略配置
│   │   │   │   └── types.go            #       评估类型
│   │   │   ├── benchmark/              # ★ V2 新增：基准测试
│   │   │   ├── splitter/               #     文档分割
│   │   │   ├── storage/                #     存储/Embedding
│   │   │   └── feishu/                 #     飞书导入
│   │   │
│   │   ├── rag/                        # ★ V2 新增：RAG 治理层
│   │   │   ├── experiment/             #     A/B 实验平台
│   │   │   │   └── state.go            #       实验状态管理
│   │   │   ├── governance/             #     治理框架
│   │   │   │   └── governance.go       #       Feature Flag + 指标 + 补偿
│   │   │   ├── release/                #     发布控制器
│   │   │   │   └── controller.go       #       灰度发布 + 回滚
│   │   │   ├── indexlifecycle/         #     索引生命周期
│   │   │   │   └── service.go          #       索引注册/构建/切换/回滚
│   │   │   ├── phase3/                 #     Phase3 策略合约
│   │   │   └── phase3admin/            #     Phase3 管理状态
│   │   │
│   │   ├── model/                      # ★ V2 大幅扩展：数据模型
│   │   │   ├── kb_knowledge_base.go    #       知识库
│   │   │   ├── kb_document.go          #       文档
│   │   │   ├── kb_ingest_job.go        #       导入任务
│   │   │   ├── kb_job_operation_log.go #       任务操作日志
│   │   │   ├── kb_retrieve_log.go      #       检索日志（40+ 字段）
│   │   │   ├── kb_cost_trace.go        #       成本追踪
│   │   │   ├── kb_audit_event.go       #       审计事件
│   │   │   ├── kb_eval_dataset.go      #       评估数据集
│   │   │   ├── kb_eval_case.go         #       评估用例
│   │   │   ├── kb_eval_run.go          #       评估运行
│   │   │   ├── kb_eval_report.go       #       评估报告
│   │   │   ├── kb_index_registry.go    #       索引注册表
│   │   │   ├── kb_index_operation_log.go #     索引操作日志
│   │   │   └── ...                     #       原有模型
│   │   │
│   │   ├── observability/              # ★ V2 增强：可观测性
│   │   │   ├── metrics/rag_metrics.go  #       Prometheus 指标注册
│   │   │   └── looptrace/              #       链路追踪
│   │   │
│   │   ├── alert/                      #     飞书告警
│   │   ├── mq/                         #     消息队列
│   │   ├── service/                    #     领域服务
│   │   └── ...                         #     其他原有模块
│   │
│   ├── deploy/                         # ★ V2 新增：部署配置
│   │   └── monitoring/
│   │       ├── prometheus/             #       Prometheus 配置 + 告警规则
│   │       ├── grafana/                #       Grafana 仪表盘
│   │       └── alertmanager/           #       AlertManager 配置
│   │
│   └── docs/                           #   项目文档（30+ 篇实现教程）
│
├── frontend/                           # 用户端前端（原有）
├── docker-compose.yml                  # 全栈部署编排
└── nginx.conf                          # Nginx 配置
```

---

## 4. 核心模块深度解析

### 4.1 面试引擎 (Interview Engine) — 系统心脏

**位置**: `internal/service/interview/engine/`

面试引擎是整个系统最核心的模块，负责驱动完整的面试流程。它有两种运行模式：

#### 4.1.1 简单循环模式 (`RunInterviewLoop`)

```
[生成问题] → [SSE流式推送] → [等待回答] → [保存对话] → [下一题] → ... → [完成]
```

- 逐个生成问题，每次一道
- 保留前 2 道题的历史作为上下文
- 最多 10 道问题
- 30 分钟回答超时，15 秒心跳保活

#### 4.1.2 Graph 编排模式 (`RunInterviewLoopWithGraph`) — ★ 推荐模式

基于 Eino 的 `compose.Graph` 实现的状态机编排，支持**自适应难度调节**：

```
                    ┌──────────────────────────────────────────────┐
                    │                                              │
START → start_init → question → wait_answer → evaluate → branch   │
                    ▲                                  │          │
                    │                    ┌─────────────┼──────────┤
                    │                    │             │          │
                    │               deepen(≥8分)  continue(4-8) lower(<4分)  switch(话题覆盖充分)
                    │                    │             │          │          │
                    │                    └──────┬──────┴─────┬────┘    ┌────┘
                    │                           │            │         │
                    └───────────────────────────┴────────────┴─────────┘
                                                          (loop back to question)

                    当 questionIndex >= maxQuestions 或 ShouldStop → END
```

#### 4.1.3 会话管理 (`SessionManager`)

- 内存级会话管理器（全局单例 `sync.Once`）
- 每个会话通过 `AnswerChan` (buffered channel, cap=1) 实现问答同步
- 支持对话快照 (`dialoguesSnapshot`)，在 Graph 异常退出时仍可落库

#### 4.1.4 SSE 实时推送

| 事件类型 | 用途 |
|----------|------|
| `chunk` | 流式分块消息（打字机效果） |
| `structured_message` | 完整结构化消息（含角色/状态/元数据） |
| `ready_for_answer` | 通知前端等待用户回答 |
| `answer_received` | 回答已接收，含进度百分比 |
| `heartbeat` | 心跳保活 |
| `complete` | 面试结束 |
| `error` | 错误通知 |
| `model_failover_required` | 模型故障切换通知 |

---

### 4.2 ★ RAG 知识库系统 — V2 核心新增

**位置**: `backend/api/handler/kb/` + `backend/internal/milvus/` + `backend/internal/rag/`

#### 4.2.1 知识库管理

**核心功能**:

| 功能 | API | 说明 |
|------|-----|------|
| 创建知识库 | `POST /api/kb/bases` | 名称唯一性校验 |
| 列出知识库 | `GET /api/kb/bases` | 分页查询 |
| 上传文档 | `POST /api/kb/documents/upload` | PDF/TXT/MD，≤20MB，SHA256 去重 |
| 列出文档 | `GET /api/kb/documents` | 按知识库筛选 |
| 删除文档 | `DELETE /api/kb/documents/:id` | 软删除 + Milvus 向量清理 |
| 任务管理 | `GET/POST /api/kb/jobs/:id/retry\|cancel` | 任务状态机：pending→processing→completed/failed/dead |
| 检索 | `POST /api/kb/retrieve` | 混合检索 + 证据门禁 |
| 仪表盘 | `GET /api/kb/dashboard/stats` | KB数/文档数/任务数 |

**文档导入流程**:

```
用户上传 → 文件校验 → SHA256 去重检查 → OSS 存储
    → 创建 Document + IngestJob（事务）
    → 发布 Redis Stream 消息
    → Consumer 异步处理：分割 → Embedding → Milvus 写入
    → 更新 Document/Job 状态
```

**Ingest Job 状态机**:

```
pending → processing → completed
                    → failed → retrying → processing → ...
                                       → dead
          → canceled
```

#### 4.2.2 混合检索管线 (Hybrid Retrieval Pipeline)

**位置**: `internal/milvus/retrieval/hybrid_search.go`

这是 V2 最核心的检索能力增强，从单一向量检索升级为多路融合检索：

```
用户查询
    │
    ▼
┌─────────────────────────────────────┐
│         Query Rewrite (查询改写)      │
│  ┌───────────────────────────────┐  │
│  │ 1. Blacklist Filter           │  │  过滤噪声词
│  │ 2. Rule-Based Rewrite         │  │  语法规则改写
│  │ 3. Domain Term Expansion      │  │  领域词典扩展
│  │ 4. Route-Specific Rewrite     │  │  Dense/Sparse 分路改写
│  │ 5. Model-Assisted Shadow      │  │  LLM 辅助改写（Shadow 模式）
│  └───────────────────────────────┘  │
└──────────────┬──────────────────────┘
               │
    ┌──────────┴──────────┐
    ▼                     ▼
┌─────────┐         ┌──────────┐
│  Dense   │         │  Sparse  │
│  Route   │         │  Route   │
│ (向量检索)│         │ (BM25)   │
└────┬────┘         └────┬─────┘
     │                   │
     └─────────┬─────────┘
               ▼
┌─────────────────────────────────────┐
│          Fusion + Dedupe             │
│     (RRF 融合 + 去重)                │
└──────────────┬──────────────────────┘
               ▼
┌─────────────────────────────────────┐
│         Reranker (重排序)             │
│     Jaccard Reranker (默认)          │
└──────────────┬──────────────────────┘
               ▼
┌─────────────────────────────────────┐
│         Dynamic TopK (动态截断)       │
│  基于 Token Budget + 分数分布决策      │
└──────────────┬──────────────────────┘
               ▼
┌─────────────────────────────────────┐
│       Parent-Child Fill (父子填充)    │
│  child_only → parent_only → section  │
│  → sibling_window → child_first+parent│
└──────────────┬──────────────────────┘
               ▼
┌─────────────────────────────────────┐
│       Evidence Gate (证据门禁)        │
│  检查：Rerank 分数/密度/引用覆盖       │
│  不通过 → 拒绝回答 + 原因              │
└──────────────┬──────────────────────┘
               ▼
┌─────────────────────────────────────┐
│    Citation Consistency (引用校验)    │
│  L5 引用一致性验证                     │
└──────────────┬──────────────────────┘
               ▼
          返回结果 + Debug Trace
```

#### 4.2.3 查询改写 (Query Rewriting)

**位置**: `internal/milvus/retrieval/rewrite.go`

支持多级改写策略：

| 策略 | 说明 |
|------|------|
| `none` | 不改写 |
| `blacklist` | 黑名单过滤 |
| `rule_based` | 规则改写（拼写纠错、同义词） |
| `domain_terms` | 领域词典扩展（技术术语） |
| `route_specific` | Dense/Sparse 分路独立改写 |
| `model_assisted_shadow` | LLM 辅助改写（Shadow 模式，不影响主流程） |

**领域词典**: 内置技术领域术语映射，如 `redis` → `Redis 缓存 内存数据库`。

#### 4.2.4 动态 TopK 策略

**位置**: `internal/milvus/retrieval/topk_policy.go`

基于多维信号动态决定返回文档数量：

```go
type TopKDecision struct {
    CandidateTopK          int     // 候选文档数
    FinalTopK              int     // 最终返回数
    TokenBudget            int     // Token 预算
    ScoreDistribution      string  // 分数分布类型
    RerankGap              float64 // 重排分数间隔
    EvidenceDensity        float64 // 证据密度
    DecisionReason         string  // 决策原因
}
```

**策略版本**:
- `phase2-rule-v1`: 基于规则的 TopK
- `phase3-strategic-v1`: 策略化 TopK（考虑分数分布 + Token 预算）

#### 4.2.5 父子文档填充 (Parent-Child)

**位置**: `internal/milvus/retrieval/parent_child.go`

检索到子文档后，自动填充父文档上下文：

| 策略 | 说明 |
|------|------|
| `child_only` | 仅子文档 |
| `parent_only` | 替换为父文档 |
| `sibling_window` | 包含相邻兄弟文档 |
| `section_window` | 包含同章节文档 |
| `child_first_with_parent_summary` | 子文档优先 + 父文档摘要 |

#### 4.2.6 证据门禁 (Evidence Gate)

**位置**: `internal/milvus/retrieval/evidence_gate.go`

当检索结果质量不足时，拒绝回答而非给出低质量回复：

```go
type EvidenceGateConfig struct {
    Enabled             bool
    MinRerankScore      float64  // 最低重排分数
    MinEvidenceDensity  float64  // 最低证据密度
    MinCitationCoverage float64  // 最低引用覆盖率
}
```

**拒绝原因**:
- `No-Retrieval-Hit`: 未检索到任何结果
- `Low-Rerank-Confidence`: 候选证据置信度不足
- `Insufficient-Citation-Coverage`: 引用覆盖不足
- `Contradictory-Evidence`: 候选证据存在冲突
- `Out-Of-KB-Scope`: 问题超出知识库范围

#### 4.2.7 引用一致性校验 (Citation Consistency)

**位置**: `internal/milvus/retrieval/citation_consistency.go`

L5 层引用验证，检查检索结果是否真正支持查询中的声明：

```go
type CitationConsistencyOutcome struct {
    Supported         bool     // 是否支持
    SupportScore      float64  // 支持分数
    UnsupportedClaims []string // 不支持的声明
    Version           string   // 校验版本
    Latency           time.Duration
}
```

---

### 4.3 ★ 评估系统 — V2 新增

**位置**: `internal/milvus/evaluation/`

#### 4.3.1 评估数据集

管理评估用例集合，每个用例包含：
- 查询（Query）
- 相关文档 ID（RelevantIDs）
- 引用目标（CitationTargets）
- 查询类型、标签

#### 4.3.2 评估运行器

```go
type Runner struct {
    Factory SearcherFactory  // 按策略 Profile 创建 Searcher
}

func (r *Runner) Run(ctx, dataset, profiles, thresholds, baseline, candidate) (*Report, error)
```

**评估指标**:

| 指标 | 说明 |
|------|------|
| Recall@K | 前 K 个结果中命中的相关文档比例 |
| MRR (Mean Reciprocal Rank) | 第一个相关结果的排名倒数 |
| NDCG | 归一化折损累积增益 |
| Citation Accuracy | 引用准确率 |
| Citation Precision | 引用精确率 |
| Citation Recall | 引用召回率 |
| P50/P95 Latency | 延迟分位数 |
| Refusal Accuracy | 拒绝回答准确率 |

#### 4.3.3 质量门禁 (Gate)

自动对比 Baseline 与 Candidate 的指标差异，判断是否满足上线条件：

```go
type GateThresholds struct {
    MinRecallDelta               float64  // Recall 提升下限
    MinMRRDelta                  float64  // MRR 不可回退
    MaxP95LatencyRegressionRatio float64  // P95 延迟回退比例上限
    MaxRefusalFalsePositiveRate  float64  // 误拒率上限
}
```

---

### 4.4 ★ 实验平台 — V2 新增

**位置**: `internal/rag/experiment/`

支持检索策略的 A/B 测试：

```
┌─────────────────────────────────────────┐
│           Experiment Platform            │
│                                         │
│  实验配置:                                │
│  - 策略类型: rewrite / candidate_topk    │
│  - 流量分配: baseline / candidate / shadow│
│  - 目标: KB IDs / Query Types / 环境     │
│  - Shadow 模式: 不影响用户结果            │
│                                         │
│  流量路由:                                │
│  UserID + ExperimentID → FNV Hash        │
│  → Group (baseline/candidate/shadow)     │
│  → Override (查询类型/TopK/改写开关)      │
└─────────────────────────────────────────┘
```

**实验状态**: `draft → running → paused → stopped → finished`

---

### 4.5 ★ 发布控制器 (Release Controller) — V2 新增

**位置**: `internal/rag/release/`

实现检索策略的灰度发布：

```
Phase1 (向量检索) ──→ Phase2 (混合检索)

灰度阶段:
  phase1 → internal → small_flow → batch → full
```

**决策逻辑**:

```go
func Decide(ragCfg, phase2Available, userID, userRole) Decision {
    // 1. 检查 Runtime Override（手动回滚）
    // 2. 检查 Release 配置
    // 3. 按阶段 + 用户角色决定策略
}
```

**运行时回滚**: 管理员可随时触发回滚到 Phase1，无需重启。

---

### 4.6 ★ 治理框架 (Governance) — V2 新增

**位置**: `internal/rag/governance/`

#### Feature Flags

| Flag | 说明 |
|------|------|
| `RAG_ENABLE_COST_GOVERNANCE` | 成本治理 |
| `RAG_ENABLE_AUDIT_CENTER` | 审计中心 |
| `RAG_ENABLE_VECTOR_OPS` | 向量运维 |
| `RAG_ENABLE_GOVERNANCE_ALERTS` | 治理告警 |
| `RAG_ENABLE_WEEKLY_REPORT` | 周报 |
| `RAG_ENABLE_EXPERIMENT_PLATFORM` | 实验平台 |
| `RAG_ENABLE_INDEX_LIFECYCLE` | 索引生命周期 |

#### 审计事件

所有关键操作自动记录审计事件：

| Action | 触发场景 |
|--------|----------|
| `document_upload` | 文档上传 |
| `document_delete` | 文档删除 |
| `strategy_flag_update` | 策略标志更新 |
| `experiment_update` | 实验配置更新 |
| `collection_rebuild` | 集合重建 |
| `collection_switch` | 集合切换 |
| `alert_ack` / `alert_resolve` | 告警确认/解决 |

#### 补偿任务

操作失败时自动入队补偿任务，支持重试：

```go
type CompensationTask struct {
    ID        string
    Scope     string  // "retrieve_audit" / "audit_event"
    TraceID   string
    RequestID string
    Reason    string
    Payload   map[string]interface{}
}
```

---

### 4.7 ★ 索引生命周期 (Index Lifecycle) — V2 新增

**位置**: `internal/rag/indexlifecycle/`

管理 Milvus 向量索引的完整生命周期：

```
Register → Build Candidate → Health Check → Switch Active → Rollback (if needed)
```

**索引角色**: `active` / `candidate` / `standby` / `rollback` / `deprecated`

**健康检查**:

```go
type HealthReport struct {
    CollectionExists  bool
    DimensionMatch    bool
    MetricTypeMatch   bool
    LoadHealthy       bool
    QuerySmokeHealthy bool
}
```

---

### 4.8 AI 智能体层 (Agent Layer)

**位置**: `internal/agents/`

#### 智能体类型体系

```
InterviewAgentType (枚举)
├── ComprehensiveSchool    // 校招综合面试
├── ComprehensiveSocial    // 社招综合面试
├── SpecializedGo          // Go 专项面试
├── SpecializedJava        // Java 专项面试
├── SpecializedMySQL       // MySQL 专项面试
├── SpecializedRedis       // Redis 专项面试
├── SpecializedMQ          // 消息队列专项面试
└── GroupInterview         // 多人模拟面试（多智能体协作）
```

#### 多智能体编排 (Multi-Agent)

**位置**: `internal/agents/multiagent/orchestrator.go`

采用 **Agent-as-Tool** 模式：

```
┌─────────────────────────────────────────────┐
│         MainInterviewer (主面试官)            │
│  Tools:                                    │
│    ├─ CoInterviewer (技术面试官) → AgentTool  │
│    ├─ ProjectInterviewer (项目面试官) → AgentTool │
│    └─ GetResumeInfoTool (简历查询)            │
└─────────────────────────────────────────────┘
```

---

### 4.9 LLM Provider 抽象层

**位置**: `internal/agents/llm/`

```
CreatOpenAiChatModel(ctx, userId)
    │
    ├── parseLLMConfig() → 解析全局 LLM 配置
    │
    ├── resolveProtocol(providerName) → 确定协议类型
    │   ├── "gemini" → Gemini 协议
    │   ├── "ark"/"volcengine"/"doubao" → Ark 协议
    │   └── 其他 → OpenAI 兼容协议
    │
    ├── 创建 ChatModel
    │   ├── createGeminiModel()  → Google Gemini
    │   ├── createArkModel()     → 火山方舟 (豆包)
    │   └── createOpenAIModel()  → OpenAI/DeepSeek/Ollama/Qwen/Groq/Grok
    │
    └── 包装为 tracedChatModel → 注入 TraceID/UserID/Token 监控
```

**关键特性**:
- **熔断器** (`circuitbreaker`): LLM 调用自带熔断保护
- **HTTP 连接池**: 共享 `http.Transport`
- **客户端缓存**: HTTP Client 按 cacheKey 缓存 10 分钟
- **Token 配额**: 每用户每日 Token 消耗限制
- **全链路追踪**: 通过 `tracedChatModel` 包装

---

### 4.10 支付系统

**位置**: `internal/payment/`

采用**策略模式 + 注册中心**实现多渠道支付（Stripe + PayPal）。

**Webhook 处理流程**:

```
Webhook 请求 → 签名验证 → 事件标准化 → 幂等处理 → 业务逻辑
```

---

### 4.11 语音识别 (ASR) 服务

**位置**: `internal/service/interview/asr/`

支持 Google Speech-to-Text + 自定义 ASR，含防护层（静音检测/超时/重试）和后处理（标点恢复/纠错）。

---

### 4.12 消息队列 (Message Queue)

**位置**: `internal/mq/`

```
MessageQueue Interface
├── Publish(ctx, *Message)
├── Subscribe(ctx, MessageHandler)
└── Close()

实现:
├── InMemoryQueue     ← 内存队列（开发/测试）
└── RedisStreamQueue  ← Redis Stream（生产）
```

**V2 新增消息类型**:

| 类型 | 用途 |
|------|------|
| `knowledge_ingest` | 文档导入处理 |
| `evaluation_report` | 评估报告生成 |
| `topic_evaluation` | 主题评估 |
| `resume_parse` | 简历解析 |

---

## 5. 数据流全景

### 5.1 RAG 知识库检索流程 (V2 新增)

```
1. 前端发起检索 → POST /api/kb/retrieve
   → 用户认证 + 限流检查
   → 解析请求（KB IDs / Query / TopK）

2. 决策阶段:
   → Release Controller 决定 Phase1/Phase2 策略
   → Experiment Platform 决定实验分组
   → Query Rewrite 改写查询

3. 检索阶段 (Phase2 混合检索):
   → Dense Route: 向量 Embedding → Milvus ANN 检索
   → Sparse Route: 关键词提取 → 倒排索引 BM25 检索
   → Fusion: RRF 分数融合
   → Dedupe: 去重
   → Reranker: Jaccard 重排序

4. 后处理阶段:
   → Dynamic TopK: 基于 Token Budget 截断
   → Parent-Child Fill: 父子文档上下文填充
   → Evidence Gate: 证据质量检查
   → Citation Check: 引用一致性校验

5. 结果返回:
   → 构建检索结果 + Citation + Source 元数据
   → 记录 RetrieveLog（40+ 字段）
   → 记录 CostTrace（成本追踪）
   → 记录 AuditEvent（审计事件）
   → 返回 JSON 响应 + Debug Trace
```

### 5.2 面试流程数据流

```
1. 用户创建面试 → POST /api/v1/interview
2. 用户开始面试 → POST /api/v1/interview/:id/start (SSE)
3. 面试循环 (Graph 模式) → question → wait_answer → evaluate → branch
4. 面试结束 → 保存对话 → 发布 MQ → 异步生成评估报告
5. 用户获取结果 → GET /api/v1/interview/:id/evaluation
```

### 5.3 支付流程数据流

```
1. 创建支付 → POST /api/v1/payment/checkout → CheckoutURL
2. 用户支付 → Stripe/PayPal 页面
3. Webhook 回调 → 签名验证 → 幂等处理 → 更新订单
```

---

## 6. 关键设计模式

| 模式 | 应用场景 | 代码位置 |
|------|----------|----------|
| **策略模式** | 支付渠道切换 / 检索策略切换 | `internal/payment/provider.go`, `internal/milvus/retrieval/` |
| **注册中心模式** | 支付 Provider 全局注册 | `internal/payment/registry.go` |
| **Agent-as-Tool** | 多智能体协作 | `internal/agents/multiagent/orchestrator.go` |
| **Graph 状态机** | 面试流程编排 | `internal/service/interview/engine/graph_loop.go` |
| **管道模式** | RAG 检索管线（改写→检索→融合→重排→过滤） | `internal/milvus/retrieval/hybrid_search.go` |
| **单例模式** | SessionManager / ASR Service / Milvus Manager | `engine/types.go`, `asr/singleton.go` |
| **工厂模式** | LLM Model 创建 / 评估 Searcher 创建 | `internal/agents/llm/provider.go`, `evaluation/runner.go` |
| **观察者模式** | SSE 事件推送 | `internal/service/interview/engine/events.go` |
| **熔断器模式** | LLM 调用保护 | `pkg/circuitbreaker/breaker.go` |
| **消息队列** | 异步任务解耦 | `internal/mq/mq.go` |
| **DAO 模式** | 数据访问层 | `internal/model/*.go` |
| **Feature Flag** | 治理功能开关 | `internal/rag/governance/governance.go` |
| **灰度发布** | 检索策略分阶段上线 | `internal/rag/release/controller.go` |
| **A/B 测试** | 检索策略实验 | `internal/rag/experiment/state.go` |
| **门禁模式** | 质量门禁 / 证据门禁 | `evaluation/gate.go`, `retrieval/evidence_gate.go` |
| **补偿模式** | 操作失败自动补偿 | `internal/rag/governance/governance.go` |

---

## 7. 配置体系

**配置文件**: `backend/config.yaml`

支持**环境变量注入**：YAML 中使用 `${VAR_NAME}` 或 `$VAR_NAME`，加载时自动替换。

```yaml
# RAG 配置示例
rag:
  enabled: true
  release:
    enabled: true
    stage: "phase1"  # phase1/internal/small_flow/batch/full
  feature_flags:
    enable_hybrid_retrieval: true
    enable_query_rewrite: true
    enable_dynamic_topk: true
    enable_parent_child: true
    enable_evidence_refusal: true
    enable_citation_check: true
    enable_retrieve_audit: true
  phase3:
    evidence_min_rerank_score: 0.3
    evidence_min_density: 0.2
    evidence_min_citation_coverage: 0.5
  thresholds:
    retrieve_timeout_ms: 3000
    user_qps_limit: 10
```

---

## 8. 前端架构

### 8.1 用户端 (Frontend)

```
Next.js App Router
├── app/
│   ├── interview/          → 面试（campus/social/special/multi）
│   ├── user/               → 用户中心（面试记录/笔记/支付）
│   ├── resume/             → 简历管理
│   └── questions/          → 题库
├── hooks/                  → Auth/ASR/Speech Hooks
├── services/api/           → API 客户端
├── store/                  → Zustand 认证状态
└── types/                  → TypeScript 类型
```

### 8.2 ★ 管理后台 (Admin) — V2 新增

**技术栈**: Next.js 14 + Ant Design 5 + TanStack Query + Zustand + Vitest

```
admin/src/
├── app/(admin)/
│   ├── dashboard/          → 仪表盘（KB数/文档数/任务数）
│   ├── knowledge-bases/    → 知识库管理（CRUD + 文档上传）
│   │   └── [kbId]/         → 知识库详情
│   ├── retrieval-lab/      → 检索实验室
│   │   └── debug/          → 检索调试（Debug Trace 可视化）
│   ├── strategy-center/    → 策略中心（Flag管理/版本/回滚/影响分析/门禁）
│   ├── evaluation/         → 评估系统
│   │   ├── datasets/       → 评估数据集
│   │   ├── runs/           → 评估运行
│   │   └── reports/[runId]/→ 评估报告
│   ├── cost-ops/           → 成本运营
│   │   ├── cost/           → 成本概览/时序/明细
│   │   └── vector-db/      → 向量数据库成本
│   ├── vector-ops/         → 向量运维（集合健康/容量/重建/切换）
│   ├── trace-logs/         → 日志追踪
│   │   ├── ingest/         → 导入日志
│   │   └── retrieval/      → 检索日志
│   ├── audit/              → 审计中心（事件列表/详情/导出）
│   ├── alerts/             → 告警管理（确认/解决）
│   ├── quality-monitor/    → 质量监控
│   ├── reports/weekly/     → 周报
│   └── layout.tsx          → Admin Shell 布局
│
├── components/admin/       → 20+ Admin 组件
├── services/api/client.ts  → Axios API 客户端
├── types/kb.ts             → 900+ 行类型定义
└── __tests__/              → Vitest 测试
```

---

## 9. 可观测性 (Observability)

### 9.1 Prometheus 指标

**位置**: `internal/observability/metrics/rag_metrics.go`

| 指标 | 类型 | 说明 |
|------|------|------|
| `rag_retrieve_requests_total` | Counter | 检索请求总数（按 status/error_code） |
| `rag_retrieve_duration_seconds` | Histogram | 检索延迟分布 |
| `rag_retrieve_result_count` | Histogram | 检索结果数分布 |
| `rag_retrieve_route_requests_total` | Counter | 路由级请求（dense/sparse） |
| `rag_retrieve_strategy_total` | Counter | 策略级统计 |
| `rag_retrieve_empty_reason` | Counter | 空结果原因分布 |
| `rag_retrieve_rewrite_total` | Counter | 查询改写统计 |
| `rag_retrieve_rerank_latency` | Histogram | 重排延迟 |
| `rag_retrieve_route_contrib` | Counter | 路由贡献度 |
| `rag_ingest_jobs_total` | Counter | 导入任务统计 |
| `rag_ingest_duration_seconds` | Histogram | 导入延迟 |
| `rag_release_rollback_total` | Counter | 发布回滚统计 |
| `rag_consumer_backlog` | Gauge | 消费者积压 |

### 9.2 Grafana 仪表盘

- `rag-l3-overview.json`: RAG L3 总览（检索/导入/错误/延迟）
- `rag-l8-rollout-overview.json`: RAG L8 灰度发布总览

### 9.3 告警规则

| 告警 | 级别 | 条件 |
|------|------|------|
| `RAGIngestFailureRateHigh` | P1 | 导入失败率 > 5%（10m） |
| `RAGRetrieveErrorRateHigh` | P1 | 检索错误率 > 3%（5m） |
| `RAGRetrieveP95LatencyHigh` | P2 | 检索 P95 延迟 > 2s（10m） |
| `RAGConsumerBacklogRising` | P2 | 消费者积压持续上升（15m） |

### 9.4 飞书告警

**位置**: `internal/alert/feishu.go`

告警自动推送到飞书群，支持文本/富文本卡片消息。

---

## 10. 部署架构

```
docker-compose.yml
├── nginx        → 反向代理 (:81 → Frontend / :3001 → Admin / :8899 → Backend)
├── backend      → Go 服务 (Hertz)
├── frontend     → Next.js 用户端
├── admin        → Next.js 管理后台
├── mysql        → 数据库 (:3307)
├── redis        → 缓存/MQ (:6379)
├── etcd         → Milvus 依赖
├── minio        → Milvus 对象存储
├── milvus       → 向量数据库 (:19530)
├── prometheus   → 指标采集
├── grafana      → 可视化仪表盘
└── alertmanager → 告警路由
```

---

## 11. 面试高频考点速查

### Go 语言层面

| 考点 | 项目体现 |
|------|----------|
| goroutine 调度 | 面试循环在独立 goroutine 中运行，`context.CancelFunc` 控制生命周期 |
| channel 通信 | `AnswerChan` (buffered chan) 实现问答同步 |
| sync 包 | `sync.RWMutex` 保护 SessionManager/Registry/ClientCache/ReleaseOverride |
| sync.Once | 全局单例（SessionManager/Transport/MetricsCollectors） |
| context 传播 | 全链路 context 传递，支持取消/超时/TraceID |
| 限流 | `golang.org/x/time/rate` + Redis 分布式限流 + 用户级 QPS 限流 |
| 熔断器 | 自研 `circuitbreaker` 包，保护 LLM 调用 |
| 连接池 | HTTP Transport 连接池配置，GORM 数据库连接池 |

### 架构设计层面

| 考点 | 项目体现 |
|------|----------|
| 分层架构 | Handler → Service → Agent → Repository 四层分离 |
| DDD 思想 | `internal/` 强制封装，领域模型与基础设施分离 |
| 状态机 | Eino Graph 编排面试流程 / Ingest Job 状态机 |
| 策略模式 | 支付 Provider / 检索策略 / 查询改写策略 |
| 管道模式 | RAG 检索管线（7 阶段串行处理） |
| 消息队列 | Redis Stream 异步解耦评估/导入 |
| SSE 实时通信 | 面试过程流式推送，心跳保活 |
| 多智能体 | Agent-as-Tool 模式，主面试官调度副面试官 |
| RAG | Milvus 混合检索 + 查询改写 + 重排序 + 证据门禁 |
| 自适应难度 | 评分驱动分支路由（deepen/continue/lower/switch） |
| 幂等性 | Webhook 幂等键防止重复处理 |
| 可观测性 | Prometheus 指标 + Grafana 仪表盘 + 飞书告警 |
| 灰度发布 | 分阶段发布策略（Phase1→internal→small→batch→full） |
| A/B 测试 | 实验平台流量分组 + Shadow 模式 |
| 治理门禁 | 成本/审计/索引/实验/发布 五维门禁 |
| Feature Flag | 治理功能开关，支持运行时切换 |
| 补偿机制 | 操作失败自动入队补偿任务 |

### 框架层面

| 考点 | 项目体现 |
|------|----------|
| Hertz 中间件链 | Recovery → RateLimiter → CORS → JWT |
| Eino Graph | `compose.NewGraph` + `AddLambdaNode` + `AddBranch` + `Compile` |
| Eino ADK | `adk.NewChatModelAgent` + `adk.NewAgentTool` |
| GORM | 模型定义 + DAO 封装 + 连接池 + 事务 |
| Redis | 缓存 + 限流 + 消息队列 (Stream) + 幂等键 |
| Milvus | 向量检索 + 混合检索 + 元数据过滤 |
| Prometheus | Counter/Histogram/Gauge 指标注册 + 自定义 Collector |

---

## 12. 错误处理体系

**位置**: `internal/errors/`

```
errors.NewOpenAIError()           → LLM 调用通用错误
errors.NewInsufficientTokensError() → Token 额度不足
errors.NewRateLimitExceededError()  → 限流
errors.NewContextLengthExceededError() → 上下文超长
errors.NewModelUnavailableError()    → 模型不可用（触发 failover）
errors.NewInvalidParamError()        → 参数校验失败
errors.NewDBError()                  → 数据库错误
errors.NewMilvusError()              → Milvus 错误
errors.NewNotFoundError()            → 资源未找到
errors.NewValidationError()          → 业务校验失败
errors.NewInternalError()            → 内部错误
```

---

## 13. 测试策略

| 测试类型 | 位置 | 说明 |
|----------|------|------|
| 单元测试 | `*_test.go` 各模块 | 评分器/ASR/熔断器/限流器/中间件/检索管线 |
| 集成测试 | `milvus/integration_test.go` | Milvus 向量检索集成测试 |
| Handler 测试 | `handler/kb/*_test.go` | 知识库 API 测试（30+ 测试文件） |
| 前端测试 | `admin/src/__tests__/` | Vitest + Testing Library |
| 基准测试 | `cmd/retrieval-benchmark/` | 检索性能基准 |
| 评估测试 | `internal/milvus/evaluation/` | 检索质量评估（Recall/MRR/NDCG） |
| 评估脚本 | `scripts/evaluation/` | Python 评估脚本 + TruLens 报告 |

---

> 本 Wiki 基于项目代码自动生成，最后更新时间: 2026-05-29
