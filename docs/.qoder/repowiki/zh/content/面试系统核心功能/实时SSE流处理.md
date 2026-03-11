# 实时SSE流处理

<cite>
**本文档引用的文件**
- [backend/api/handler/interview/mianshi/engine.go](file://backend/api/handler/interview/mianshi/engine.go)
- [backend/api/handler/interview/mianshi/events.go](file://backend/api/handler/interview/mianshi/events.go)
- [backend/api/handler/interview/mianshi/types.go](file://backend/api/handler/interview/mianshi/types.go)
- [backend/api/handler/interview/mianshi/utils.go](file://backend/api/handler/interview/mianshi/utils.go)
- [backend/api/handler/interview/mianshi_service.go](file://backend/api/handler/interview/mianshi_service.go)
- [backend/api/router/interview/api.go](file://backend/api/router/interview/api.go)
- [mcpserver/internal/protocol/sse.go](file://mcpserver/internal/protocol/sse.go)
- [mcpserver/internal/server/sse.go](file://mcpserver/internal/server/sse.go)
- [frontend/src/app/interview/campus/start/page.tsx](file://frontend/src/app/interview/campus/start/page.tsx)
- [frontend/src/app/interview/social/start/page.tsx](file://frontend/src/app/interview/social/start/page.tsx)
- [frontend/TROUBLESHOOTING.md](file://frontend/TROUBLESHOOTING.md)
</cite>

## 目录
1. [简介](#简介)
2. [项目结构](#项目结构)
3. [核心组件](#核心组件)
4. [架构总览](#架构总览)
5. [详细组件分析](#详细组件分析)
6. [依赖关系分析](#依赖关系分析)
7. [性能考虑](#性能考虑)
8. [故障排除指南](#故障排除指南)
9. [结论](#结论)
10. [附录](#附录)

## 简介
本文件系统性地解析实时SSE（Server-Sent Events）流处理在面试系统中的实现与架构设计。重点覆盖：
- SSE连接建立与维护
- 事件类型与数据模型
- 流式数据传输与背压处理
- 客户端连接管理与重连机制
- 错误恢复与调试监控

## 项目结构
该系统采用前后端分离架构，面试SSE流由后端Hertz框架提供，前端使用浏览器原生SSE或fetch+ReadableStream消费事件流。

```mermaid
graph TB
subgraph "前端"
FE_Campus["校园面试页面<br/>page.tsx"]
FE_Social["社会面试页面<br/>page.tsx"]
end
subgraph "后端"
API_Router["路由注册<br/>api.go"]
Handler_Stream["面试流处理器<br/>mianshi_service.go"]
Engine["面试引擎<br/>engine.go"]
SessionMgr["会话管理器<br/>types.go"]
Utils["SSE工具与心跳<br/>utils.go"]
Events["SSE事件封装<br/>events.go"]
end
FE_Campus --> API_Router
FE_Social --> API_Router
API_Router --> Handler_Stream
Handler_Stream --> SessionMgr
Handler_Stream --> Engine
Engine --> Events
Engine --> Utils
Utils --> Events
```

图表来源
- [backend/api/router/interview/api.go](file://backend/api/router/interview/api.go#L44-L46)
- [backend/api/handler/interview/mianshi_service.go](file://backend/api/handler/interview/mianshi_service.go#L26-L117)
- [backend/api/handler/interview/mianshi/engine.go](file://backend/api/handler/interview/mianshi/engine.go#L39-L230)
- [backend/api/handler/interview/mianshi/types.go](file://backend/api/handler/interview/mianshi/types.go#L9-L50)
- [backend/api/handler/interview/mianshi/utils.go](file://backend/api/handler/interview/mianshi/utils.go#L11-L56)
- [backend/api/handler/interview/mianshi/events.go](file://backend/api/handler/interview/mianshi/events.go#L11-L71)

章节来源
- [backend/api/router/interview/api.go](file://backend/api/router/interview/api.go#L44-L46)
- [backend/api/handler/interview/mianshi_service.go](file://backend/api/handler/interview/mianshi_service.go#L26-L117)

## 核心组件
- SSE连接建立与响应头设置：在启动面试流时，后端设置SSE专用响应头并建立流式输出。
- 会话管理器：负责会话生命周期、并发安全、答案通道与活动时间维护。
- 面试引擎：驱动面试循环，按序生成问题、等待答案、发送进度与完成事件。
- SSE事件封装：统一SSE事件格式，支持错误、完成、就绪、心跳等事件类型。
- 心跳与超时：在等待答案阶段定时发送心跳事件，避免连接空闲断开。

章节来源
- [backend/api/handler/interview/mianshi_service.go](file://backend/api/handler/interview/mianshi_service.go#L77-L117)
- [backend/api/handler/interview/mianshi/types.go](file://backend/api/handler/interview/mianshi/types.go#L9-L50)
- [backend/api/handler/interview/mianshi/engine.go](file://backend/api/handler/interview/mianshi/engine.go#L39-L230)
- [backend/api/handler/interview/mianshi/events.go](file://backend/api/handler/interview/mianshi/events.go#L11-L71)
- [backend/api/handler/interview/mianshi/utils.go](file://backend/api/handler/interview/mianshi/utils.go#L11-L56)

## 架构总览
SSE流式架构分为三层：
- 路由层：注册面试流启动与答案提交接口。
- 处理层：建立SSE连接、写入初始事件、启动面试引擎。
- 引擎层：生成问题、等待答案、发送事件、持久化记录。

```mermaid
sequenceDiagram
participant Browser as "浏览器"
participant Router as "路由(api.go)"
participant Handler as "StartMianshiStream"
participant Pipe as "io.Pipe"
participant Engine as "InterviewEngine"
participant SM as "SessionManager"
participant Writer as "SSE Writer"
Browser->>Router : POST /api/mianshi/stream/start
Router->>Handler : 调用处理器
Handler->>Handler : 设置SSE响应头
Handler->>Pipe : 创建管道
Handler->>Browser : 返回流式响应
Handler->>Writer : 发送session_id事件
Handler->>Writer : 发送start事件
Handler->>Engine : 启动面试循环
Engine->>SM : 等待答案(带心跳)
Engine->>Writer : 发送question事件
Engine->>Writer : 发送ready_for_answer事件
Engine->>SM : 清空答案通道
Engine->>Writer : 发送answer_received事件
Engine->>Writer : 发送complete事件
Browser-->>Handler : 断开连接
```

图表来源
- [backend/api/router/interview/api.go](file://backend/api/router/interview/api.go#L44-L46)
- [backend/api/handler/interview/mianshi_service.go](file://backend/api/handler/interview/mianshi_service.go#L26-L117)
- [backend/api/handler/interview/mianshi/engine.go](file://backend/api/handler/interview/mianshi/engine.go#L39-L230)
- [backend/api/handler/interview/mianshi/events.go](file://backend/api/handler/interview/mianshi/events.go#L11-L71)

## 详细组件分析

### 会话管理器（SessionManager）
- 负责会话的创建、查询、答案提交与清理。
- 使用读写锁保证并发安全。
- 维护每个会话的活动时间、问题计数与取消函数。

```mermaid
classDiagram
class SessionManager {
+sessions map[string]*InterviewSession
+mu RWMutex
+CreateSession(...)
+GetSession(id) *InterviewSession
+SubmitAnswer(id, answer) error
+GetAnswer(id, timeout) (string, bool)
+ClearAnswer(id)
+DeleteSession(id)
}
class InterviewSession {
+SessionID string
+UserID uint
+RecordID uint64
+ResumeId int64
+HasResume bool
+AnswerChan chan string
+CancelFunc CancelFunc
+StartTime time.Time
+LastActivity time.Time
+QuestionCount int32
}
SessionManager --> InterviewSession : "管理"
```

图表来源
- [backend/api/handler/interview/mianshi/types.go](file://backend/api/handler/interview/mianshi/types.go#L9-L50)
- [backend/api/handler/interview/mianshi/types.go](file://backend/api/handler/interview/mianshi/types.go#L52-L149)

章节来源
- [backend/api/handler/interview/mianshi/types.go](file://backend/api/handler/interview/mianshi/types.go#L9-L149)

### 面试引擎（InterviewEngine）
- 驱动面试循环，按序生成问题并等待答案。
- 维护历史上下文（最近N题），动态调整后续问题。
- 发送多种SSE事件：问题、就绪、进度、完成、错误。

```mermaid
flowchart TD
Start(["开始面试"]) --> CreateAgent["创建智能体服务"]
CreateAgent --> SelectAgent["选择智能体类型"]
SelectAgent --> Loop{"问题计数 < 最大数量?"}
Loop --> |是| BuildPrompt["构建提示词(含历史上下文)"]
BuildPrompt --> GenQuestion["生成问题"]
GenQuestion --> SendQuestion["发送question事件"]
SendQuestion --> SendReady["发送ready_for_answer事件"]
SendReady --> WaitAnswer["WaitForAnswerWithHeartbeat"]
WaitAnswer --> AnswerTimeout{"收到答案?"}
AnswerTimeout --> |否| SendError["发送error事件并退出"]
AnswerTimeout --> |是| SaveDialog["保存问答记录"]
SaveDialog --> UpdateHistory["更新历史上下文"]
UpdateHistory --> SendProgress["发送answer_received事件"]
SendProgress --> Loop
Loop --> |否| SaveAll["保存全部对话"]
SaveAll --> SendComplete["发送complete事件"]
SendComplete --> End(["结束"])
```

图表来源
- [backend/api/handler/interview/mianshi/engine.go](file://backend/api/handler/interview/mianshi/engine.go#L39-L230)
- [backend/api/handler/interview/mianshi/utils.go](file://backend/api/handler/interview/mianshi/utils.go#L11-L56)
- [backend/api/handler/interview/mianshi/events.go](file://backend/api/handler/interview/mianshi/events.go#L11-L71)

章节来源
- [backend/api/handler/interview/mianshi/engine.go](file://backend/api/handler/interview/mianshi/engine.go#L39-L230)
- [backend/api/handler/interview/mianshi/utils.go](file://backend/api/handler/interview/mianshi/utils.go#L11-L56)

### SSE事件封装与心跳
- 统一SSE事件格式：event字段对应type，data为JSON。
- 提供错误、完成、就绪、心跳等事件发送函数。
- 心跳机制：在等待答案期间周期性发送心跳事件，防止连接空闲断开。

```mermaid
sequenceDiagram
participant Engine as "InterviewEngine"
participant Utils as "WaitForAnswerWithHeartbeat"
participant Writer as "SSE Writer"
Engine->>Utils : 等待答案(带心跳)
loop 每heartbeatInterval
Utils->>Writer : 发送heartbeat事件
end
Utils-->>Engine : 返回答案或超时
```

图表来源
- [backend/api/handler/interview/mianshi/events.go](file://backend/api/handler/interview/mianshi/events.go#L11-L71)
- [backend/api/handler/interview/mianshi/utils.go](file://backend/api/handler/interview/mianshi/utils.go#L11-L56)

章节来源
- [backend/api/handler/interview/mianshi/events.go](file://backend/api/handler/interview/mianshi/events.go#L11-L71)
- [backend/api/handler/interview/mianshi/utils.go](file://backend/api/handler/interview/mianshi/utils.go#L11-L56)

### 前端SSE消费与重连
- 前端使用fetch+ReadableStream读取SSE流，解析data块为JSON事件。
- 识别session_id与start事件以初始化会话。
- 页面中包含自动重试与错误日志输出，便于调试。

```mermaid
sequenceDiagram
participant Page as "面试页面"
participant Fetch as "fetch + ReadableStream"
participant SSE as "SSE事件流"
Page->>Fetch : 发起SSE请求
Fetch->>SSE : 建立连接
SSE-->>Fetch : data : {"type" : "session_id",...}
Fetch-->>Page : 解析并设置会话ID
SSE-->>Fetch : data : {"type" : "start",...}
Fetch-->>Page : 显示面试开始
loop 读取事件
SSE-->>Fetch : data : {"type" : "question",...}
Fetch-->>Page : 展示问题
Page->>SSE : 提交答案
end
```

图表来源
- [frontend/src/app/interview/campus/start/page.tsx](file://frontend/src/app/interview/campus/start/page.tsx#L137-L177)
- [frontend/src/app/interview/social/start/page.tsx](file://frontend/src/app/interview/social/start/page.tsx#L185-L214)
- [frontend/TROUBLESHOOTING.md](file://frontend/TROUBLESHOOTING.md#L105-L117)

章节来源
- [frontend/src/app/interview/campus/start/page.tsx](file://frontend/src/app/interview/campus/start/page.tsx#L137-L177)
- [frontend/src/app/interview/social/start/page.tsx](file://frontend/src/app/interview/social/start/page.tsx#L185-L214)
- [frontend/TROUBLESHOOTING.md](file://frontend/TROUBLESHOOTING.md#L75-L117)

## 依赖关系分析
- 路由层依赖处理器层，处理器层依赖会话管理器与面试引擎。
- 面试引擎依赖事件封装与心跳工具。
- 前端依赖后端SSE事件格式约定。

```mermaid
graph LR
Router["路由(api.go)"] --> Handler["StartMianshiStream"]
Handler --> SessionMgr["SessionManager"]
Handler --> Engine["InterviewEngine"]
Engine --> Events["SSE事件封装"]
Engine --> Utils["WaitForAnswerWithHeartbeat"]
Frontend["前端页面"] --> SSE["SSE事件流"]
SSE --> Events
```

图表来源
- [backend/api/router/interview/api.go](file://backend/api/router/interview/api.go#L44-L46)
- [backend/api/handler/interview/mianshi_service.go](file://backend/api/handler/interview/mianshi_service.go#L26-L117)
- [backend/api/handler/interview/mianshi/engine.go](file://backend/api/handler/interview/mianshi/engine.go#L39-L230)
- [backend/api/handler/interview/mianshi/events.go](file://backend/api/handler/interview/mianshi/events.go#L11-L71)
- [backend/api/handler/interview/mianshi/utils.go](file://backend/api/handler/interview/mianshi/utils.go#L11-L56)

章节来源
- [backend/api/router/interview/api.go](file://backend/api/router/interview/api.go#L44-L46)
- [backend/api/handler/interview/mianshi_service.go](file://backend/api/handler/interview/mianshi_service.go#L26-L117)

## 性能考虑
- 缓冲区与背压
  - 使用io.Pipe建立无缓冲管道，避免额外内存拷贝。
  - SSE事件发送后立即Flush，确保客户端及时接收。
  - 会话答案通道容量为1，避免堆积过多答案导致内存膨胀。
- 心跳保活
  - 定时发送心跳事件，维持连接活跃，减少空闲断开。
- 超时控制
  - 等待答案阶段设置超时，超时后发送错误事件并终止流程。
- 并发与锁
  - 会话管理器使用读写锁，降低高并发下的竞争开销。
- 数据持久化
  - 逐条保存问答记录，避免一次性写入大量数据造成阻塞。

章节来源
- [backend/api/handler/interview/mianshi_service.go](file://backend/api/handler/interview/mianshi_service.go#L80-L117)
- [backend/api/handler/interview/mianshi/events.go](file://backend/api/handler/interview/mianshi/events.go#L29-L33)
- [backend/api/handler/interview/mianshi/types.go](file://backend/api/handler/interview/mianshi/types.go#L76-L107)
- [backend/api/handler/interview/mianshi/utils.go](file://backend/api/handler/interview/mianshi/utils.go#L15-L19)

## 故障排除指南
- 常见问题
  - 404错误：检查路由是否正确注册，确认URL路径与权限。
  - CORS错误：确认后端已设置允许跨域响应头。
  - 登录过期：重新登录获取新token。
  - 无法连接后端：确认后端服务运行地址与端口。
- 前端调试
  - 使用浏览器控制台查看SSE事件日志，定位会话ID与事件类型。
  - 检查事件格式是否符合data: JSON的SSE标准。
- 后端调试
  - 关注日志输出，特别是会话ID、超时与错误事件。
  - 检查Flush调用是否成功，避免事件丢失。

章节来源
- [frontend/TROUBLESHOOTING.md](file://frontend/TROUBLESHOOTING.md#L75-L117)
- [backend/api/handler/interview/mianshi/events.go](file://backend/api/handler/interview/mianshi/events.go#L25-L27)

## 结论
该SSE流处理实现通过清晰的分层架构与严格的事件模型，实现了面试过程的实时流式交互。会话管理器与面试引擎配合SSE事件封装，提供了稳定的心跳保活与超时控制。前端通过SSE或fetch+ReadableStream消费事件，具备良好的可调试性与扩展性。建议在生产环境中结合监控指标（如事件延迟、连接断开率、超时比例）持续优化性能与稳定性。

## 附录

### 事件类型定义
- 会话ID事件：标识会话建立，包含会话ID、记录ID与开始时间。
- 开始事件：通知前端面试已开始。
- 问题事件：包含问题索引、总数与问题文本。
- 就绪事件：提示前端可提交答案。
- 进度事件：包含问题索引、总数与百分比进度。
- 完成事件：面试结束通知。
- 错误事件：异常或超时错误描述。
- 心跳事件：保持连接活跃。

章节来源
- [backend/api/handler/interview/mianshi/events.go](file://backend/api/handler/interview/mianshi/events.go#L11-L71)
- [backend/api/handler/interview/mianshi/engine.go](file://backend/api/handler/interview/mianshi/engine.go#L148-L200)