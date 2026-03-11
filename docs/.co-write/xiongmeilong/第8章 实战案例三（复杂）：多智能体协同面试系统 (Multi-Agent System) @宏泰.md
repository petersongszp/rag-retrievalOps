# 第8章 实战案例三（复杂）：多智能体协同面试系统 (Multi-Agent System)

如果说前面的单体 Agent 是“独行侠”，那么本章我们要构建的就是一支训练有素的“特种部队”。在真实的面试场景中，往往不是一个面试官在战斗，而是由 HR（主导流程）、技术专家（考察深度）和部门主管（考察项目与软素质）组成的面试团。本章我们将挑战 AI 应用开发的高阶形态——多智能体协同（Multi-Agent Collaboration），打造一个全真模拟的群组面试系统。我们将深入探讨如何利用 Eino 的编排能力解决多角色话语权调度（Turn-Taking）难题，并引入 Redis Stream 实现异步事件驱动，让系统的复杂度与响应速度达到完美的平衡。

## 8.1 任务定义与架构设计

在着手写代码之前，我们需要先厘清这个复杂系统的“作战地图”。多智能体系统（MAS）的难点不在于单个 Agent 有多强，而在于它们之间如何配合。

### 8.1.1 场景：全真模拟“主面+技术面+项目面”的群组面试环境

我们的目标是还原一个真实的“三对一”面试现场：

*   **主面试官 (Host)**：负责控场。他像是一个主持人，负责开场介绍、引导流程、安抚候选人情绪，并在技术面试官和项目面试官之间穿针引线。
*   **技术面试官 (Co-Interviewer)**：负责“刁难”。当候选人提到某个具体技术栈（如 Redis、Go 语言）时，他会接过话茬，进行深度的技术追问，直到探到底层原理。
*   **项目面试官 (Project-Interviewer)**：负责“务实”。他关注候选人的项目经验，考察架构设计能力、解决问题的思路以及落地的真实性。

这三个角色不仅关注点不同，性格设定也不同。主面试官温和专业，技术面试官严谨犀利，项目面试官务实宏观。

### 8.1.2 难点：多角色话语权调度（Turn-Taking）与上下文共享

在多智能体系统中，最棘手的问题有两个：

1.  **谁该说话（Turn-Taking）？**：当候选人回答完一个问题后，是主面试官继续问，还是技术面试官插话？如果两个 Agent 同时抢着说话，体验就会非常糟糕。
2.  **上下文共享（Context Sharing）**：技术面试官需要知道主面试官刚才问了什么，否则可能会重复提问。如何让三个独立的 LLM 共享同一个对话历史，是技术实现的关键。

### 8.1.3 架构决策：为什么选择 Eino Graph 编排而非 MQ 异步通信？

在设计多 Agent 协作架构时，业界通常有两种流派：

*   **基于消息队列（MQ）的异步通信**：各个 Agent 监听同一个频道，谁抢到谁回答。这种方式解耦性好，但难以控制流程，容易出现“冷场”或“抢麦”。
*   **基于图（Graph）的集中编排**：有一个中心节点（Orchestrator）负责调度。这种方式流程可控，确定性强。

在面试这种对流程严谨性要求极高的场景下，我们选择了**集中编排**模式。我们利用 Eino 框架的 **Agent-as-a-Tool** 能力，将“技术面试官”和“项目面试官”封装为“主面试官”手中的工具。

*   **决策逻辑**：主面试官是唯一的“决策大脑”。他根据当前的对话上下文，决定是自己回答，还是调用“技术面试官工具”让专家发言。
*   **优势**：完美解决了话语权问题——只有主面试官允许，其他人才说话。同时也解决了上下文问题——主面试官在调用工具时，会自动将必要的上下文传递给子 Agent。

## 8.2 核心模式：Agent-as-Tool 协作架构

“Agent-as-Tool”（将智能体视为工具）是 Eino 框架的一个杀手级特性。它允许我们将一个具备独立思考能力的 Agent，封装成一个标准的 Function Tool，供另一个 Agent 调用。

### 8.2.1 主面试官（Host）：基于 Eino Graph 的状态机编排

在 [orchestrator.go](backend/internal/agents/multiagent/orchestrator.go) 中，我们定义了 `NewInterviewHostAgent`。这个函数不仅创建了主面试官，还负责组装整个团队。

代码清单8-1所示是主面试官的编排逻辑，展示了如何像搭积木一样组建 Agent 团队：

代码清单8-1 多智能体编排核心实现
```go
// NewInterviewHostAgent 创建面试主控智能体 (主面试官)
// 负责统筹整个面试流程，调度副面试官 and 项目面试官
func NewInterviewHostAgent(ctx context.Context, userId uint, needResumeTool bool) (adk.Agent, error) {
	// ... (省略模型初始化代码)

	// 1. 初始化协作 Agent (子智能体)
	coAgent, err := NewCoInterviewerAgent(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to create co-interviewer: %w", err)
	}

	projectAgent, err := NewProjectInterviewerAgent(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to create project-interviewer: %w", err)
	}

	// 2. 将协作 Agent 包装成工具 (AgentAsTool)
	// 这一步是点睛之笔：将“人”变成了“工具”
	coTool := adk.NewAgentTool(ctx, coAgent)
	projectTool := adk.NewAgentTool(ctx, projectAgent)

	// 3. 构建工具列表
	tools := []componenttool.BaseTool{
		coTool,
		projectTool,
	}

	// 4. 创建主面试官 (Host)
	hostAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "MainInterviewer",
		Description: "主面试官，负责面试全流程的引导、节奏控制 and 最终评价。",
		Instruction: MainInterviewerInstruction, // 包含调度指令的 Prompt
		Model:       model,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: tools, // 赋予主面试官调度专家的能力
			},
		},
		MaxIterations: 30,
	})

	return hostAgent, nil
}
```

上述代码解释：

- **NewCoInterviewerAgent / NewProjectInterviewerAgent**：分别创建两个垂直领域的专家 Agent。它们拥有独立的 Prompt 和知识库，专注于自己的领域。
- **adk.NewAgentTool**：这是 Eino 的核心 API。它自动为 Agent 生成 Tool Definition（工具描述），使得主面试官的 LLM 能够理解：“哦，我有一个叫 `co_interviewer` 的工具，当需要问技术细节时可以用它。”
- **ToolsConfig**：我们将这两个“人形工具”挂载到主面试官身上。从此，主面试官就拥有了“分身术”。

### 8.2.2 专家分身：将子 Agent 封装为 Function Tool

子 Agent 的定义相对简单，专注于特定领域的 Prompt 设计。以技术面试官为例：

```go
func NewCoInterviewerAgent(ctx context.Context, userId uint) (adk.Agent, error) {
    // ...
	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "co_interviewer",
		Description: "技术面试官工具。当候选人提到技术栈（如Redis、MySQL、Go...）或需要深入技术细节时，必须调用此工具。",
		Instruction: CoInterviewerInstruction, // 专注于深度追问
		Model:       model,
	})
}
```

这里的 `Description` 至关重要。它是写给主面试官看的“使用说明书”。我们必须清晰地告诉主面试官：**在什么情况下**（When）应该调用这个工具，以及这个工具**能做什么**（What）。

### 8.2.3 通信协议：定义结构化的多角色对话消息体 (Message Schema)

为了让前端能够正确渲染不同面试官的头像和语气，我们需要一套标准化的消息协议。在 [message_schema.go](backend/internal/service/interview/engine/message_schema.go) 中，我们定义了系统内部流转的“通用语言”。

代码清单8-2所示是多角色消息协议定义：

代码清单8-2 消息与角色类型定义
```go
// RoleType 角色类型
type RoleType string

const (
	RoleMainInterviewer    RoleType = "main_interviewer"    // 主面试官
	RoleTechInterviewer    RoleType = "tech_interviewer"    // 技术面试官
	RoleProjectInterviewer RoleType = "project_interviewer" // 项目面试官
	RoleCandidate          RoleType = "candidate"           // 候选人
)

// ActionType 动作类型
type ActionType string

const (
	ActionThinking ActionType = "thinking" // 思考中（前端显示加载动画）
	ActionSpeaking ActionType = "speaking" // 说话中（前端显示打字机效果）
)
```

通过这套协议，当后端返回消息时，会带上 `RoleType`。前端收到 `tech_interviewer` 的消息，就会自动切换到“技术专家”的头像和气泡样式，从而给用户营造出一种“真的有三个人在面试我”的沉浸感。

## 8.3 异步协同与事件驱动（引入 MQ）

虽然面试的对话流程是同步的，但系统中还有大量耗时的“后台任务”，如实时生成评估报告、保存对话记录、分析候选人情绪等。如果这些操作都阻塞在主线程里，用户的直观感受就是“卡顿”。

### 8.3.1 场景分析：为何将“评估生成”与“数据归档”剥离为异步任务

在一次面试交互中，LLM 的推理本身就需要几秒钟。如果我们还要等待数据库写入完成、等待分析报告生成，整个响应延迟可能会高达 10 秒以上。这是互联网产品无法接受的。

因此，我们将系统拆分为两条泳道：
1.  **快车道（同步）**：用户提问 -> Agent 思考 -> 流式返回答案。这是用户感知的核心路径，必须毫秒级响应。
2.  **慢车道（异步）**：答案生成后 -> 丢入 MQ -> 消费者慢慢处理入库、分析、归档。

### 8.3.2 技术选型：Redis Stream vs Kafka vs RabbitMQ

在选择消息队列时，我们做了一番权衡：

*   **Kafka**：吞吐量极高，但运维重，适合日志聚合等大数据场景。对于面试系统这种并发量级（单日百万级消息），有点“杀鸡用牛刀”。
*   **RabbitMQ**：功能丰富，路由灵活，但引入了新的基础设施依赖。
*   **Redis Stream**：Redis 5.0 引入的轻量级流数据结构。
    *   **优势**：我们的系统已经依赖 Redis 做缓存和 Session 存储，直接复用 Redis 做 MQ **无需引入新组件**，运维成本极低。且其支持 Consumer Group 模式，完全满足消息持久化和分组消费的需求。

最终，我们选择了 **Redis Stream** 作为异步任务的载体。

### 8.3.3 实战：基于 Redis Stream 实现面试状态的旁路监听与实时分析

在 [redis_queue.go](backend/internal/mq/redis_queue.go) 中，我们封装了一个通用的 `RedisQueue`。

代码清单8-3 Redis Stream 队列封装
```go
type RedisQueue struct {
	client   *redis.Client
	mu       sync.RWMutex
	handlers []MessageHandler
	pool     *ants.Pool // 使用 ants 协程池控制并发度
}

// 生产者：发布面试事件
func (q *RedisQueue) Publish(ctx context.Context, topic string, event interface{}) error {
    // 序列化事件
    data, _ := json.Marshal(event)
    // XAdd 命令写入 Stream
    return q.client.XAdd(ctx, &redis.XAddArgs{
        Stream: topic,
        Values: map[string]interface{}{"data": data},
    }).Err()
}
```

在面试过程中，每当产生一条新的对话记录，我们就会通过 `Publish` 方法将其发送到 `interview_events` Stream 中。后台的消费者服务会异步地拉取这些事件，进行持久化存储和实时分析，确保主流程的丝滑流畅。

## 8.4 前端专家大厅实战

后端的精彩架构，最终需要通过前端呈现给用户。我们需要在浏览器中构建一个“虚拟面试间”。

### 8.4.1 SSE 流式多路复用：在单链接中推送多角色状态

传统的 HTTP 请求是“一问一答”，无法满足 AI 生成的流式效果。我们采用了 **Server-Sent Events (SSE)** 技术，通过一条长连接，持续不断地将 Agent 的思考过程和回答推送给前端。

在 [interview_service.go](backend/api/handler/interview/interview_service.go) 的 `StartInterviewStream` 接口中，我们建立了一个 SSE 通道。

代码清单8-4 SSE 流式处理入口
```go
// StartInterviewStream 启动面试流程（SSE 模式）
func StartInterviewStream(ctx context.Context, c *app.RequestContext) {
    // ... (参数校验与 Session 创建)

    c.SetStatusCode(http.StatusOK)
    c.Response.Header.Set("Content-Type", "text/event-stream")
    c.Response.Header.Set("Cache-Control", "no-cache")
    c.Response.Header.Set("Connection", "keep-alive")

    // 开启流式写入器
    c.Stream(func(w io.Writer) bool {
        // 在这里调用 Agent 并实时 flush 数据到 w
        // 每一条数据都包含 event type 和 data
        return false // 返回 false 结束流
    })
}
```

我们定义了多种事件类型（Event Type）：
*   `heartbeat`：保持连接活跃。
*   `start`：面试开始。
*   `message`：对话内容（包含 `RoleType`）。
*   `error`：异常通知。

### 8.4.2 动态 UI：基于角色标识的实时渲染

前端接收到 SSE 推送的 JSON 数据后，解析其中的 `role` 字段：

*   如果 `role == "main_interviewer"`：在界面中央显示主面试官的气泡。
*   如果 `role == "tech_interviewer"`：在界面左侧弹出技术专家的气泡，并高亮显示技术专家的头像。
*   如果 `role == "project_interviewer"`：在界面右侧显示项目经理的点评。

这种基于事件驱动的动态渲染，让用户感觉自己真的置身于一个多人会议室中，极大地增强了产品的临场感。

## 8.5 最终产出：自动生成包含雷达图的面试报告

当面试结束后，系统会触发 `complete` 事件。此时，后台的“评估 Agent”会根据所有历史对话，从“技术深度”、“项目经验”、“沟通能力”、“逻辑思维”等维度对候选人进行打分，并生成一份详细的面试报告。

这份报告不仅包含文字评语，还会生成一组雷达图（Radar Chart）的数据。前端利用 ECharts 或 Chart.js 将这些数据可视化，让候选人一眼就能看到自己的能力六维图，从而明确自己的长板与短板。

至此，我们不仅完成了一个复杂的后端架构，更交付了一个完整的、端到端的企业级产品功能。从单体到多智能体，从同步到异步，从后端到前端，这一章的实战涵盖了 AI 应用开发的方方面面。
