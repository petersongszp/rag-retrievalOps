# Eino 监控回调 (Monitoring Callbacks) 实现指南

## 1. 概述
本项目利用 Eino 框架的 `callbacks` 机制，实现了对 AI 组件的全链路监控。监控模块位于 `backend/pkg/eino/callbacks/monitoring.go`。

## 2. 核心功能
`MonitoringHandler` 实现了 `callbacks.Handler` 接口，提供以下能力：
- **OnStart**: 捕获组件（Chain, Graph, Node）的开始事件，记录开始时间。
- **OnEnd**: 捕获组件正常结束事件，计算并记录执行耗时 (Latency)。
- **OnEndWithStreamOutput**: 捕获流式输出 (Streaming) 的首包时间，用于监控 Time-to-First-Token (TTFT)。
- **OnError**: 捕获组件执行过程中的错误，便于快速定位问题。

## 3. 集成方式

### 3.1 已接入的调用路径（12.1.1 全链路监控）

- **面试主流程**：`backend/internal/agents/usecase/interview/interview_agent_service.go` 的 `runAgentWithIterator` 中已集成 MonitoringHandler：
  - 每次 Agent Run 前调用 `OnStart`，结束后 `defer` 调用 `OnEnd`（并传入最后一次 `event.Output` 以尝试提取 Token）；
  - 任意错误路径调用 `OnError`。
- **会话级 TraceID**：`backend/internal/service/interview/engine/graph_loop.go` 的 `RunInterviewLoopWithGraph` 开头调用 `mycallbacks.WithTraceID(ctx, session.SessionID)`，整场面试多轮 Run 共享同一 TraceID，便于日志聚合与回溯。
- **外部注入 TraceID**：若需从 HTTP 或上游传入 TraceID，可在入口处调用 `mycallbacks.WithTraceID(ctx, traceID)` 再传入后续逻辑。

## 4. 监控指标
通过日志输出，我们可以收集以下指标：
- `[Eino Monitor] Start Component`: 组件调用次数 (QPS)
- `[Eino Monitor] End Component ... Latency`: 组件响应延迟 (P99/P95)
- `[Eino Monitor] Error Component`: 错误率与错误堆栈

## 5. 未来演进
- **接入 OpenTelemetry**: 修改 `MonitoringHandler`，将日志输出替换为 `otel.Tracer` 和 `meter.Record`，实现与 Jaeger/Prometheus 的对接。
- **Token 统计**: 在 `OnEnd` 中解析 `CallbackOutput`，提取 Token Usage 信息，用于成本核算。
