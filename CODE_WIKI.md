# 面试吧 (Interview Bar) — Code Wiki

> 基于 Hertz + Eino 的 AI 智能面试平台，面向美加市场出海

---

## 1. 项目概览

| 维度 | 说明 |
|------|------|
| **产品定位** | AI 驱动的模拟面试系统，帮助求职者通过智能面试练习提升面试技能 |
| **Go Module** | `interview-agents` |
| **Go 版本** | 1.25.1 |
| **后端框架** | [Hertz](https://github.com/cloudwego/hertz)（字节跳动高性能 HTTP 框架） |
| **AI 框架** | [Eino](https://github.com/cloudwego/eino)（字节跳动 LLM 应用编排框架） |
| **前端框架** | Next.js 14+ / React / TypeScript / Tailwind CSS |
| **数据库** | MySQL (GORM) + Redis + Milvus（向量数据库） |
| **消息队列** | Redis Stream |
| **认证** | JWT + GitHub OAuth + Google OAuth |
| **支付** | Stripe + PayPal（策略模式 + 注册中心） |
| **部署** | Docker Compose + Nginx 反向代理 |

---

## 2. 系统架构总览

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Nginx (反向代理)                              │
│                    :81 → Frontend / :8899 → Backend                  │
└──────────┬──────────────────────────────────────┬───────────────────┘
           │                                      │
┌──────────▼──────────┐              ┌─────────────▼─────────────────┐
│   Frontend (Next.js) │              │    Backend (Hertz + Eino)     │
│   - React + TS       │              │                               │
│   - Tailwind CSS     │   REST API   │  ┌─────────────────────────┐ │
│   - Zustand 状态管理  │◄────────────►│  │  API Layer (Handler)    │ │
│   - i18n (中/英)     │              │  ├─────────────────────────┤ │
└──────────────────────┘              │  │  Service Layer          │ │
                                      │  │  ├─ Interview Engine    │ │
                                      │  │  ├─ Payment Service     │ │
                                      │  │  ├─ User Service        │ │
                                      │  │  └─ Resume Service      │ │
                                      │  ├─────────────────────────┤ │
                                      │  │  Agent Layer (Eino)     │ │
                                      │  │  ├─ Interview Agents    │ │
                                      │  │  ├─ Evaluation Agent    │ │
                                      │  │  ├─ Prediction Agent    │ │
                                      │  │  └─ Resume Agent        │ │
                                      │  ├─────────────────────────┤ │
                                      │  │  Repository Layer       │ │
                                      │  │  ├─ MySQL (GORM)        │ │
                                      │  │  └─ Redis               │ │
                                      │  └─────────────────────────┘ │
                                      └───────────────┬───────────────┘
                                                      │
                              ┌───────────────────────┼───────────────────┐
                              │                       │                   │
                      ┌───────▼──────┐      ┌────────▼──────┐   ┌───────▼──────┐
                      │    MySQL      │      │    Redis       │   │   Milvus     │
                      │  (GORM ORM)   │      │  (Cache/MQ)   │   │ (向量检索)    │
                      └──────────────┘      └───────────────┘   └──────────────┘
```

---

## 3. 目录结构详解

```
mianshiba-eino-overseas/
├── backend/                          # 后端 Go 服务
│   ├── cmd/
│   │   └── server/main.go            # ★ 服务唯一入口
│   ├── api/                          # 接口层（HTTP 边界）
│   │   ├── handler/                  #   请求处理器（参数绑定、校验、响应格式化）
│   │   │   ├── interview/            #     面试相关 Handler
│   │   │   ├── payment/              #     支付相关 Handler
│   │   │   └── resume/               #     简历相关 Handler
│   │   ├── model/                    #   API 层 DTO 模型
│   │   ├── router/                   #   路由注册 + 中间件
│   │   │   ├── interview/            #     面试路由 + AuthSkipper
│   │   │   ├── middleware/           #     Recovery 中间件
│   │   │   └── payment/              #     支付路由
│   │   ├── response/                 #   统一响应格式
│   │   └── idl/                      #   Thrift IDL 定义
│   ├── internal/                     # ★ 私有业务逻辑（不可被外部引用）
│   │   ├── agents/                   #   AI 智能体层（UseCase）
│   │   │   ├── evaluation/           #     答案评估智能体
│   │   │   ├── interview/            #     面试智能体
│   │   │   │   ├── comprehensive/    #       综合面试（校招/社招）
│   │   │   │   └── specialized/      #       专项面试（Go/Java/MySQL/Redis/MQ）
│   │   │   ├── llm/                  #     LLM Provider 抽象层
│   │   │   ├── multiagent/           #     多智能体编排（主面试官+副面试官）
│   │   │   ├── prediction/           #     预测推荐智能体
│   │   │   ├── resume/               #     简历分析智能体
│   │   │   ├── tools/                #     Agent 工具（Milvus检索/简历查询/PDF解析）
│   │   │   ├── usecase/              #     业务用例封装
│   │   │   │   ├── evaluation/       #       评估用例（含分布式锁）
│   │   │   │   ├── interview/        #       面试用例
│   │   │   │   └── resume/           #       简历用例
│   │   │   └── pkg/                  #     Agent 内部工具函数
│   │   ├── config/                   #   配置管理（YAML + 环境变量注入）
│   │   ├── errors/                   #   统一错误定义
│   │   ├── middleware/               #   JWT / 限流中间件
│   │   ├── milvus/                   #   向量数据库（Embedding/检索/导入）
│   │   ├── model/                    #   数据模型（GORM 实体 + DAO）
│   │   ├── mq/                       #   消息队列（Redis Stream 实现）
│   │   ├── observability/            #   可观测性（CozeLoop Trace）
│   │   ├── payment/                  #   支付系统（Provider 接口 + Webhook）
│   │   ├── repository/               #   数据库/Redis 初始化
│   │   ├── service/                  #   领域服务层
│   │   │   ├── common/               #     通用服务（邮件/密码）
│   │   │   ├── interview/            #     面试服务
│   │   │   │   ├── asr/              #       语音识别（ASR）服务
│   │   │   │   └── engine/           #       ★ 面试引擎核心
│   │   │   ├── payment/              #     支付服务
│   │   │   ├── prediction/           #     预测服务
│   │   │   ├── resume/               #     简历服务
│   │   │   └── user/                 #     用户服务
│   │   ├── storage/                  #   OSS 文件存储
│   │   └── alert/                    #   飞书告警
│   ├── pkg/                          # 公共工具库（可被外部引用）
│   │   ├── circuitbreaker/           #   熔断器
│   │   ├── eino/callbacks/           #   Eino 回调（Token 监控）
│   │   └── ratelimiter/              #   限流器
│   ├── scripts/                      # 评估脚本
│   ├── config.yaml                   # 配置文件
│   └── go.mod                        # 依赖管理
├── frontend/                         # 前端 Next.js 应用
│   └── src/
│       ├── app/                      #   页面路由（App Router）
│       │   ├── interview/            #     面试页面（campus/social/special/multi）
│       │   ├── user/                 #     用户中心（面试记录/笔记/支付）
│       │   ├── resume/               #     简历管理
│       │   └── questions/            #     题库
│       ├── components/               #   组件
│       ├── hooks/                    #   自定义 Hooks（Auth/ASR/Speech）
│       ├── services/api/             #   API 客户端
│       ├── store/                    #   Zustand 状态管理
│       └── types/                    #   TypeScript 类型定义
├── litellm/                          # LiteLLM 模型网关配置
├── doc/                              # 项目文档
├── docker-compose.yml                # 全栈部署编排
└── nginx.conf                        # Nginx 配置
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

**关键节点说明**：

| 节点 | 功能 | 关键逻辑 |
|------|------|----------|
| `start_init` | 初始化 | 设置 QuestionIndex=1 |
| `question` | 生成问题 | 调用 Agent 流式生成，SSE 推送 chunk |
| `wait_answer` | 等待回答 | 阻塞等待用户输入，带心跳保活 |
| `evaluate` | 评分 | 4 维度评分（正确性40%/深度25%/完整性20%/实践性15%） |
| `deepen` | 深入追问 | 高分(≥8)分支，追问底层实现/边界情况 |
| `continue` | 继续话题 | 中等分数(4-8)分支，换角度考察同话题 |
| `lower` | 降低难度 | 低分(<4)分支，切换到基础概念 |
| `switch` | 切换话题 | 当前话题覆盖充分，TopicTracker 推荐新话题 |
| `end_loop` | 结束 | 保存对话、发布 MQ 消息触发评估报告 |

**核心数据结构** — `InterviewState`：

```go
type InterviewState struct {
    Session           *InterviewSession       // 会话信息
    AgentSvc          *InterviewAgentService  // 智能体服务
    Evaluator         *Evaluator              // 评分器
    TopicTracker      *TopicTracker           // 话题追踪器
    QuestionIndex     int                     // 当前题号
    QuestionText      string                  // 当前问题
    Answer            string                  // 用户回答
    EvalResult        *EvaluationResult       // 评分结果
    NextActionHint    string                  // 下一题 Prompt 提示
    AllDialogues      []*InterviewDialogueData // 全部对话
    RecentHistory     []historyItem           // 最近2轮历史
    ShouldStop        bool                    // 是否终止
}
```

#### 4.1.3 会话管理 (`SessionManager`)

- 内存级会话管理器（全局单例 `sync.Once`）
- 每个会话通过 `AnswerChan` (buffered channel, cap=1) 实现问答同步
- 支持对话快照 (`dialoguesSnapshot`)，在 Graph 异常退出时仍可落库
- SessionID 格式: `session_20060102150405_XXXXXXXX`

#### 4.1.4 SSE 实时推送

面试过程通过 **Server-Sent Events** 实现实时流式通信：

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

### 4.2 AI 智能体层 (Agent Layer)

**位置**: `internal/agents/`

#### 4.2.1 智能体类型体系

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

#### 4.2.2 多智能体编排 (Multi-Agent)

**位置**: `internal/agents/multiagent/orchestrator.go`

多人模拟面试采用 **Agent-as-Tool** 模式：

```
┌─────────────────────────────────────────────┐
│         MainInterviewer (主面试官)            │
│  - 统筹面试流程                               │
│  - 调度副面试官                               │
│  - Tools:                                    │
│    ├─ CoInterviewer (技术面试官) → AgentTool  │
│    ├─ ProjectInterviewer (项目面试官) → AgentTool │
│    └─ GetResumeInfoTool (简历查询)            │
└─────────────────────────────────────────────┘
```

- **主面试官**: 负责流程引导、节奏控制、最终评价
- **技术面试官**: 深度技术追问（Redis/MySQL/Go/分布式等）
- **项目面试官**: 项目真实性考察、落地能力评估

#### 4.2.3 专项面试智能体

每个专项面试智能体（如 `GoSpecializedAgent`）都配备：
- **Milvus 检索工具**: 从知识库中检索专业题目
- **简历查询工具**: 根据候选人简历定制问题
- **领域专属 Prompt**: 针对特定技术栈的面试指令

#### 4.2.4 评估智能体 (Evaluation Agent)

**位置**: `internal/agents/evaluation/`

评分维度与权重：

| 维度 | 权重 | 说明 |
|------|------|------|
| Correctness (正确性) | 40% | 答案的技术准确性 |
| Depth (深度) | 25% | 是否触及底层原理 |
| Completeness (完整性) | 20% | 覆盖面是否全面 |
| Practicality (实践性) | 15% | 是否有实际工程经验 |

评分结果驱动自适应策略：
- **≥8 分**: `ActionDeepen` — 深入追问
- **4-8 分**: `ActionContinue` — 继续当前话题
- **<4 分**: `ActionLower` — 降低难度
- 连续 N 题（默认2）满足条件才触发策略切换

#### 4.2.5 话题追踪器 (TopicTracker)

**位置**: `internal/agents/evaluation/topic/`

- 维护面试话题树 (`TopicTree`)
- 追踪已覆盖话题和评分
- 当话题覆盖充分时推荐切换到新话题 (`SuggestNextTopic`)

---

### 4.3 LLM Provider 抽象层

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

**关键特性**：
- **熔断器** (`circuitbreaker`): LLM 调用自带熔断保护，5xx/429 触发熔断
- **HTTP 连接池**: 共享 `http.Transport`，MaxIdleConns=100, MaxIdleConnsPerHost=20
- **客户端缓存**: HTTP Client 按 cacheKey 缓存 10 分钟，避免重复创建
- **Token 配额**: 每用户每日 Token 消耗限制，超限拒绝请求
- **全链路追踪**: 通过 `tracedChatModel` 包装，自动注入 CozeLoop Trace

---

### 4.4 支付系统

**位置**: `internal/payment/`

采用**策略模式 + 注册中心**实现多渠道支付：

```
┌─────────────────────────────────────────────┐
│              Provider Interface              │
│  - CreateCheckout()                          │
│  - CreateSubscription()                      │
│  - CancelSubscription()                      │
│  - Refund()                                  │
│  - VerifyWebhook()                           │
└──────────┬──────────────────┬───────────────┘
           │                  │
    ┌──────▼──────┐   ┌──────▼──────┐
    │   Stripe     │   │   PayPal    │
    │  Adapter     │   │  Adapter    │
    └─────────────┘   └─────────────┘
           │                  │
           └──────┬───────────┘
                  │
         ┌────────▼────────┐
         │    Registry      │  ← 全局注册中心 (sync.RWMutex)
         │  providers map   │
         └─────────────────┘
```

**Webhook 处理流程**：

```
Webhook 请求 → 签名验证 (VerifyWebhook) → 事件标准化 (EventMapping) → 幂等处理 (Idempotency) → 业务逻辑
```

- **幂等性**: 基于 Redis 的幂等键，防止重复处理
- **事件映射**: 各渠道原始事件 → 标准化事件类型
- **超时窗口**: 可配置 webhook 事件时间窗口

---

### 4.5 消息队列 (Message Queue)

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

**消息类型**：

| 类型 | 用途 | 触发时机 |
|------|------|----------|
| `evaluation_report` | 生成评估报告 | 面试结束后异步触发 |
| `topic_evaluation` | 主题评估 | 面试结束后异步触发 |
| `resume_parse` | 简历解析 | 简历上传后异步触发 |

---

### 4.6 语音识别 (ASR) 服务

**位置**: `internal/service/interview/asr/`

支持语音输入面试回答：

```
ASR Provider Interface
├── Google Speech-to-Text
└── 自定义 ASR 服务

核心组件:
├── Config     → ASR 配置（语言/采样率/模型）
├── Guard      → 防护层（静音检测/超时/重试）
├── Modifier   → 后处理（标点恢复/纠错/格式化）
├── Provider   → ASR 引擎抽象
├── Service    → 业务编排
└── Singleton  → 全局单例管理
```

---

### 4.7 向量检索 (Milvus RAG)

**位置**: `internal/milvus/`

```
Milvus Manager
├── storage/          → Embedding + Indexer + Converter
├── retrieval/        → Retriever + HybridSearch + Reranker + Filter
├── splitter/         → Markdown 分割器 + 递归分割器
├── feishu/           → 飞书文档导入（Markdown 转换）
├── cmd/milvusctl/    → CLI 管理工具
└── data/             → 知识库数据
    ├── go专项/       → Go 高级并发
    ├── java专项/     → JVM 调优
    ├── 中间件专项/   → Kafka 深度
    └── 综合/         → 架构设计
```

**检索流程**：

```
用户问题 → Embedding → Milvus 向量检索 → Reranker 重排序 → Filter 过滤 → 返回 TopK 结果
```

支持混合检索（向量 + 关键词），距离度量支持 L2/IP/COSINE。

---

### 4.8 中间件链

**请求处理中间件顺序**（在 `main.go` 中注册）：

```
Request → Recovery (Panic 捕获)
       → RateLimiter (IP/Redis 限流)
       → CORS (跨域处理)
       → JWT (认证，可配置跳过路径)
       → Handler (业务处理)
```

| 中间件 | 位置 | 说明 |
|--------|------|------|
| Recovery | `api/router/middleware/` | 捕获 panic，返回 500 |
| IPRateLimiter | `internal/middleware/` | 基于 `golang.org/x/time/rate` 的单机限流 |
| RedisRateLimiter | `internal/middleware/` | 基于 Redis 的分布式限流 |
| CORS | `main.go` 内联 | 全局 CORS，支持 OPTIONS 预检 |
| JWT | `internal/middleware/` | JWT 认证，支持 Skipper 跳过指定路由 |

---

## 5. 数据流全景

### 5.1 面试流程数据流

```
1. 用户创建面试 → POST /api/v1/interview
   → Handler 创建 InterviewRecord → 返回 recordID

2. 用户开始面试 → POST /api/v1/interview/:id/start (SSE)
   → Handler 创建 Session → 启动 goroutine 运行 InterviewEngine
   → SSE 连接建立

3. 面试循环 (Graph 模式):
   ┌─ question 节点:
   │  → Agent 生成问题 → SSE chunk 推送 → structured_message
   │
   ├─ wait_answer 节点:
   │  → SSE ready_for_answer → 用户提交答案 → AnswerChan
   │
   ├─ evaluate 节点:
   │  → Evaluator 评分 → TopicTracker 更新 → 保存 Dialogue
   │
   └─ branch 节点:
      → 根据评分路由到 deepen/continue/lower/switch

4. 面试结束 → END 节点
   → 保存所有对话 → 更新 InterviewRecord 状态
   → 发布 MQ 消息 → 异步生成评估报告

5. 用户获取结果 → GET /api/v1/interview/:id/evaluation
```

### 5.2 支付流程数据流

```
1. 创建支付 → POST /api/v1/payment/checkout
   → PaymentService.CreateCheckout() → Provider.CreateCheckout()
   → 返回 CheckoutURL

2. 用户支付 → Stripe/PayPal 页面完成支付

3. Webhook 回调 → POST /api/v1/payment/webhook/:provider
   → Provider.VerifyWebhook() → EventMapping 标准化
   → WebhookProcessor 幂等处理 → 更新订单状态

4. 订阅管理 → POST /api/v1/payment/subscription
   → Provider.CreateSubscription() → 返回订阅链接
```

---

## 6. 关键设计模式

| 模式 | 应用场景 | 代码位置 |
|------|----------|----------|
| **策略模式** | 支付渠道切换 (Stripe/PayPal) | `internal/payment/provider.go` |
| **注册中心模式** | 支付 Provider 全局注册 | `internal/payment/registry.go` |
| **Agent-as-Tool** | 多智能体协作 | `internal/agents/multiagent/orchestrator.go` |
| **Graph 状态机** | 面试流程编排 | `internal/service/interview/engine/graph_loop.go` |
| **单例模式** | SessionManager / ASR Service | `engine/types.go`, `asr/singleton.go` |
| **工厂模式** | LLM Model 创建 | `internal/agents/llm/provider.go` |
| **观察者模式** | SSE 事件推送 | `internal/service/interview/engine/events.go` |
| **熔断器模式** | LLM 调用保护 | `pkg/circuitbreaker/breaker.go` |
| **消息队列** | 异步任务解耦 | `internal/mq/mq.go` |
| **DAO 模式** | 数据访问层 | `internal/model/*.go` |

---

## 7. 配置体系

**配置文件**: `backend/config.yaml`

支持**环境变量注入**：YAML 中使用 `${VAR_NAME}` 或 `$VAR_NAME`，加载时自动替换。

```yaml
# 示例
database:
  dsn: "${DB_USER}:${DB_PASSWORD}@tcp(${DB_HOST}:${DB_PORT})/${DB_NAME}"

llm:
  api_key: "${LLM_API_KEY}"
  base_url: "${LLM_BASE_URL}"
  model_name: "${LLM_MODEL_NAME}"
  provider_name: "${LLM_PROVIDER_NAME}"
```

**配置加载流程**：
1. `godotenv.Load()` → 加载 `.env` 文件到环境变量
2. `config.LoadConfig()` → 读取 YAML → 正则替换环境变量 → 反序列化到 `Config` 结构体
3. `config.Global` → 全局配置实例

---

## 8. 前端架构

```
Next.js App Router
├── app/
│   ├── interview/
│   │   ├── campus/     → 校招面试
│   │   ├── social/     → 社招面试
│   │   ├── special/    → 专项面试
│   │   └── multi/      → 多人面试
│   ├── user/
│   │   ├── center/     → 个人中心
│   │   ├── interviews/ → 面试记录 + 结果详情
│   │   ├── notes/      → 笔记
│   │   └── pay/        → 支付（success/cancel）
│   ├── resume/         → 简历管理
│   └── questions/      → 题库
├── hooks/
│   ├── useAuth.ts              → 认证状态
│   ├── useASRCapability.ts     → ASR 能力检测
│   └── useSpeechAnswerInput.ts → 语音输入
├── services/api/
│   ├── client.ts       → Axios 实例
│   └── prediction.ts   → 预测 API
├── store/
│   └── authStore.ts    → Zustand 认证状态
└── types/
    ├── message-schema.ts  → SSE 消息类型定义
    └── prediction.ts      → 预测类型定义
```

**国际化**: `messages/en.json` + `messages/zh.json`

---

## 9. 部署架构

```
docker-compose.yml
├── nginx        → 反向代理 (:81 → Frontend / :8899 → Backend)
├── backend      → Go 服务 (Hertz)
├── frontend     → Next.js 服务
├── mysql        → 数据库 (:3307)
├── redis        → 缓存/MQ (:6379)
├── etcd         → Milvus 依赖
├── minio        → Milvus 对象存储
└── milvus       → 向量数据库 (:19530)
```

**LiteLLM 模型网关**: `litellm/config.yaml` — 统一管理多 LLM Provider 的路由和限流

---

## 10. 面试高频考点速查

### Go 语言层面

| 考点 | 项目体现 |
|------|----------|
| goroutine 调度 | 面试循环在独立 goroutine 中运行，`context.CancelFunc` 控制生命周期 |
| channel 通信 | `AnswerChan` (buffered chan) 实现问答同步 |
| sync 包 | `sync.RWMutex` 保护 SessionManager/Registry/ClientCache |
| sync.Once | 全局单例（SessionManager/Transport） |
| context 传播 | 全链路 context 传递，支持取消/超时/TraceID |
| 限流 | `golang.org/x/time/rate` + Redis 分布式限流 |
| 熔断器 | 自研 `circuitbreaker` 包，保护 LLM 调用 |
| 连接池 | HTTP Transport 连接池配置，GORM 数据库连接池 |

### 架构设计层面

| 考点 | 项目体现 |
|------|----------|
| 分层架构 | Handler → Service → Agent → Repository 四层分离 |
| DDD 思想 | `internal/` 强制封装，领域模型与基础设施分离 |
| 状态机 | Eino Graph 编排面试流程，节点+分支+循环 |
| 策略模式 | 支付 Provider 接口 + 多实现 + 注册中心 |
| 消息队列 | Redis Stream 异步解耦评估报告生成 |
| SSE 实时通信 | 面试过程流式推送，心跳保活 |
| 多智能体 | Agent-as-Tool 模式，主面试官调度副面试官 |
| RAG | Milvus 向量检索 + Embedding + Reranker |
| 自适应难度 | 评分驱动分支路由（deepen/continue/lower/switch） |
| 幂等性 | Webhook 幂等键防止重复处理 |
| 可观测性 | CozeLoop Trace + Token 监控 + 飞书告警 |

### 框架层面

| 考点 | 项目体现 |
|------|----------|
| Hertz 中间件链 | Recovery → RateLimiter → CORS → JWT |
| Eino Graph | `compose.NewGraph` + `AddLambdaNode` + `AddBranch` + `Compile` |
| Eino ADK | `adk.NewChatModelAgent` + `adk.NewAgentTool` |
| GORM | 模型定义 + DAO 封装 + 连接池 |
| Redis | 缓存 + 限流 + 消息队列 (Stream) + 幂等键 |

---

## 11. 错误处理体系

**位置**: `internal/errors/`

```
errors.NewOpenAIError()           → LLM 调用通用错误
errors.NewInsufficientTokensError() → Token 额度不足
errors.NewRateLimitExceededError()  → 限流
errors.NewContextLengthExceededError() → 上下文超长
errors.NewModelUnavailableError()    → 模型不可用（触发 failover）
errors.NewInvalidParamError()        → 参数校验失败
```

**模型故障切换 (Failover)**：
当 LLM 调用失败时，引擎发送 `model_failover_required` SSE 事件，携带可用备用模型列表，前端可让用户选择切换。

---

## 12. 测试策略

| 测试类型 | 位置 | 说明 |
|----------|------|------|
| 单元测试 | `*_test.go` 各模块 | 评分器/ASR/熔断器/限流器/中间件 |
| 集成测试 | `milvus/integration_test.go` | Milvus 向量检索集成测试 |
| 评估脚本 | `scripts/evaluation/` | Python 评估脚本 + TruLens 报告 |

---

> 本 Wiki 基于项目代码自动生成，最后更新时间: 2026-05-13
