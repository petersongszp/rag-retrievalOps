# 项目优化建议 - AI 功能篇

**优先级**: ⭐⭐ 中优先级  
**预计工作量**: 2-3 周  
**影响范围**: AI 功能、用户体验

---

## 目录
1. [Agent 模块优化](#1-agent-模块优化)
2. [Prompt 工程](#2-prompt-工程)
3. [流式处理优化](#3-流式处理优化)
4. [AI 可观测性](#4-ai-可观测性)
5. [模型管理优化](#5-模型管理优化)

---

## 1. Agent 模块优化 🟠 P1

### 当前 Agent 架构分析

**优点**:
- ✅ 模块化设计良好
- ✅ 使用 Eino ADK，符合最佳实践
- ✅ 职责分离清晰

**待优化**:
- ⚠️ Agent 之间通信效率
- ⚠️ 缺少 Agent 性能监控
- ⚠️ 错误处理和重试机制
- ⚠️ 缺少 Agent 测试

### 优化建议

#### 1.1 统一 Agent 接口

```go
// pkg/agent/base/agent.go
package base

import (
    "context"
    "time"
)

// Agent 基础接口
type Agent interface {
    Name() string
    Description() string
    Process(ctx context.Context, input *Input) (*Output, error)
}

// StreamingAgent 流式 Agent
type StreamingAgent interface {
    Agent
    ProcessStream(ctx context.Context, input *Input) (<-chan *Event, error)
}

// Input 统一输入
type Input struct {
    Query     string
    Context   map[string]interface{}
    Metadata  *Metadata
}

// Output 统一输出
type Output struct {
    Content   string
    Data      interface{}
    Metadata  *Metadata
}

// Event 流式事件
type Event struct {
    Type      EventType
    AgentName string
    Content   string
    Data      interface{}
    Timestamp time.Time
    Error     error
}

type EventType string

const (
    EventTypeQuestion EventType = "question"
    EventTypeAnswer   EventType = "answer"
    EventTypeReport   EventType = "report"
    EventTypeError    EventType = "error"
)

// Metadata 元数据
type Metadata struct {
    RequestID  string
    UserID     string
    StartTime  time.Time
    EndTime    time.Time
    Duration   time.Duration
    TokenUsage *TokenUsage
}

type TokenUsage struct {
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
}
```

#### 1.2 Agent 包装器（添加功能）

```go
// pkg/agent/middleware/logger.go
package middleware

import (
    "ai-eino-interview-agent/pkg/agent/base"
    "ai-eino-interview-agent/pkg/logger"
    "context"
    "go.uber.org/zap"
)

// LoggingAgent Agent 日志包装器
type LoggingAgent struct {
    base.Agent
}

func WithLogging(agent base.Agent) base.Agent {
    return &LoggingAgent{Agent: agent}
}

func (a *LoggingAgent) Process(ctx context.Context, input *base.Input) (*base.Output, error) {
    logger.Info("agent processing started",
        zap.String("agent", a.Name()),
        zap.String("query", input.Query),
    )
    
    output, err := a.Agent.Process(ctx, input)
    
    if err != nil {
        logger.Error("agent processing failed",
            zap.String("agent", a.Name()),
            zap.Error(err),
        )
    } else {
        logger.Info("agent processing completed",
            zap.String("agent", a.Name()),
            zap.Duration("duration", output.Metadata.Duration),
        )
    }
    
    return output, err
}
```

```go
// pkg/agent/middleware/metrics.go
package middleware

import (
    "ai-eino-interview-agent/pkg/agent/base"
    "ai-eino-interview-agent/pkg/metrics"
    "context"
    "time"
)

// MetricsAgent Agent 指标包装器
type MetricsAgent struct {
    base.Agent
}

func WithMetrics(agent base.Agent) base.Agent {
    return &MetricsAgent{Agent: agent}
}

func (a *MetricsAgent) Process(ctx context.Context, input *base.Input) (*base.Output, error) {
    start := time.Now()
    
    output, err := a.Agent.Process(ctx, input)
    
    duration := time.Since(start)
    status := "success"
    if err != nil {
        status = "error"
    }
    
    // 记录指标
    metrics.AgentCallsTotal.WithLabelValues(a.Name(), status).Inc()
    metrics.AgentDuration.WithLabelValues(a.Name()).Observe(duration.Seconds())
    
    if output != nil && output.Metadata != nil && output.Metadata.TokenUsage != nil {
        metrics.AITokensUsed.WithLabelValues(a.Name()).
            Add(float64(output.Metadata.TokenUsage.TotalTokens))
    }
    
    return output, err
}
```

#### 1.3 Agent 组合模式

```go
// pkg/agent/supervisor/supervisor.go
package supervisor

import (
    "ai-eino-interview-agent/pkg/agent/base"
    "context"
)

// SupervisorAgent 协调器 Agent
type SupervisorAgent struct {
    name        string
    agents      map[string]base.Agent
    router      Router
}

type Router interface {
    Route(ctx context.Context, input *base.Input) (string, error)
}

func NewSupervisorAgent(agents map[string]base.Agent, router Router) *SupervisorAgent {
    return &SupervisorAgent{
        name:   "supervisor",
        agents: agents,
        router: router,
    }
}

func (s *SupervisorAgent) ProcessStream(ctx context.Context, input *base.Input) (<-chan *base.Event, error) {
    eventChan := make(chan *base.Event, 10)
    
    go func() {
        defer close(eventChan)
        
        // 路由到合适的 Agent
        agentName, err := s.router.Route(ctx, input)
        if err != nil {
            eventChan <- &base.Event{
                Type:  base.EventTypeError,
                Error: err,
            }
            return
        }
        
        agent, exists := s.agents[agentName]
        if !exists {
            eventChan <- &base.Event{
                Type:  base.EventTypeError,
                Error: fmt.Errorf("agent not found: %s", agentName),
            }
            return
        }
        
        // 执行 Agent
        if streamingAgent, ok := agent.(base.StreamingAgent); ok {
            agentChan, err := streamingAgent.ProcessStream(ctx, input)
            if err != nil {
                eventChan <- &base.Event{
                    Type:  base.EventTypeError,
                    Error: err,
                }
                return
            }
            
            // 转发事件
            for event := range agentChan {
                select {
                case eventChan <- event:
                case <-ctx.Done():
                    return
                }
            }
        }
    }()
    
    return eventChan, nil
}
```

---

## 2. Prompt 工程 🟠 P1

### 当前问题

- Prompt 硬编码在 Agent 代码中
- 缺少 Prompt 版本管理
- 难以测试和优化 Prompt

### 解决方案

#### 2.1 Prompt 模板管理

```go
// pkg/agent/prompt/template.go
package prompt

import (
    "bytes"
    "text/template"
)

type TemplateManager struct {
    templates map[string]*template.Template
}

func NewTemplateManager() *TemplateManager {
    return &TemplateManager{
        templates: make(map[string]*template.Template),
    }
}

func (tm *TemplateManager) LoadFromFile(name, path string) error {
    tmpl, err := template.ParseFiles(path)
    if err != nil {
        return err
    }
    tm.templates[name] = tmpl
    return nil
}

func (tm *TemplateManager) Render(name string, data interface{}) (string, error) {
    tmpl, exists := tm.templates[name]
    if !exists {
        return "", fmt.Errorf("template not found: %s", name)
    }
    
    var buf bytes.Buffer
    if err := tmpl.Execute(&buf, data); err != nil {
        return "", err
    }
    
    return buf.String(), nil
}
```

#### 2.2 Prompt 模板文件

```
configs/prompts/
├── resume_analysis.tmpl
├── question_generation.tmpl
├── answer_evaluation.tmpl
└── report_generation.tmpl
```

```text
# configs/prompts/question_generation.tmpl
你是一名资深的{{.Position}}技术面试官。

候选人背景：
- 姓名：{{.CandidateName}}
- 工作年限：{{.YearsOfExperience}}年
- 核心技能：{{join .Skills ", "}}

已问问题：
{{range .AskedQuestions}}
- {{.}}
{{end}}

请基于以上信息，生成一个{{.Difficulty}}难度的技术问题。

要求：
1. 问题应该深入考察候选人的实际经验
2. 避免重复已问过的问题
3. 问题应该具有启发性
4. 保持专业且友好的语气

请直接输出问题，不需要其他解释。
```

#### 2.3 在 Agent 中使用

```go
// pkg/agent/interview/question_generator.go
func (qg *QuestionGeneratorAgent) generateQuestion(ctx context.Context, data *QuestionData) (string, error) {
    // 渲染 Prompt
    prompt, err := qg.promptMgr.Render("question_generation", data)
    if err != nil {
        return "", err
    }
    
    // 调用模型
    response, err := qg.model.Generate(ctx, prompt)
    if err != nil {
        return "", err
    }
    
    return response, nil
}
```

#### 2.4 Prompt 版本管理

```yaml
# configs/prompts/versions.yaml
prompts:
  question_generation:
    current_version: "v2"
    versions:
      v1:
        file: "question_generation_v1.tmpl"
        created_at: "2024-10-01"
        deprecated: true
      v2:
        file: "question_generation_v2.tmpl"
        created_at: "2024-11-01"
        active: true
```

---

## 3. 流式处理优化 🟡 P2

### 当前实现

流式处理已实现，但可以优化：
- 添加背压控制
- 优化缓冲区大小
- 错误恢复机制

### 优化建议

```go
// pkg/agent/stream/stream.go
package stream

import (
    "context"
    "time"
)

type StreamProcessor struct {
    bufferSize    int
    flushInterval time.Duration
    errorHandler  ErrorHandler
}

type ErrorHandler func(error) error

func NewStreamProcessor(opts ...Option) *StreamProcessor {
    sp := &StreamProcessor{
        bufferSize:    10,
        flushInterval: 100 * time.Millisecond,
        errorHandler:  defaultErrorHandler,
    }
    
    for _, opt := range opts {
        opt(sp)
    }
    
    return sp
}

func (sp *StreamProcessor) Process(ctx context.Context, input <-chan *Event) <-chan *Event {
    output := make(chan *Event, sp.bufferSize)
    
    go func() {
        defer close(output)
        
        buffer := make([]*Event, 0, sp.bufferSize)
        ticker := time.NewTicker(sp.flushInterval)
        defer ticker.Stop()
        
        for {
            select {
            case event, ok := <-input:
                if !ok {
                    // Input closed, flush buffer
                    sp.flushBuffer(output, buffer)
                    return
                }
                
                // Handle error
                if event.Error != nil {
                    if err := sp.errorHandler(event.Error); err != nil {
                        output <- &Event{
                            Type:  EventTypeError,
                            Error: err,
                        }
                        continue
                    }
                }
                
                buffer = append(buffer, event)
                
                // Flush if buffer full
                if len(buffer) >= sp.bufferSize {
                    sp.flushBuffer(output, buffer)
                    buffer = buffer[:0]
                }
                
            case <-ticker.C:
                // Periodic flush
                if len(buffer) > 0 {
                    sp.flushBuffer(output, buffer)
                    buffer = buffer[:0]
                }
                
            case <-ctx.Done():
                return
            }
        }
    }()
    
    return output
}

func (sp *StreamProcessor) flushBuffer(output chan<- *Event, buffer []*Event) {
    for _, event := range buffer {
        select {
        case output <- event:
        default:
            // 背压：丢弃或等待
        }
    }
}
```

---

## 4. AI 可观测性 🟡 P2

### 指标收集

```go
// pkg/metrics/ai_metrics.go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    AgentCallsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "agent_calls_total",
            Help: "Total agent calls",
        },
        []string{"agent", "status"},
    )
    
    AgentDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "agent_duration_seconds",
            Help:    "Agent processing duration",
            Buckets: []float64{0.1, 0.5, 1, 2, 5, 10},
        },
        []string{"agent"},
    )
    
    AITokensUsed = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "ai_tokens_used_total",
            Help: "Total AI tokens used",
        },
        []string{"agent"},
    )
    
    PromptLength = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "prompt_length_chars",
            Help:    "Prompt length in characters",
            Buckets: []float64{100, 500, 1000, 2000, 5000},
        },
        []string{"agent"},
    )
)
```

### 追踪

```go
// pkg/agent/tracing/tracing.go
package tracing

import (
    "context"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
)

func TraceAgent(ctx context.Context, agentName string, fn func(context.Context) error) error {
    tracer := otel.Tracer("agent")
    ctx, span := tracer.Start(ctx, "agent.process",
        trace.WithAttributes(
            attribute.String("agent.name", agentName),
        ),
    )
    defer span.End()
    
    err := fn(ctx)
    if err != nil {
        span.RecordError(err)
    }
    
    return err
}
```

---

## 5. 模型管理优化 🟢 P3

### 当前 modelmgr 分析

**优点**:
- ✅ 设计优秀，支持多协议
- ✅ 配置灵活
- ✅ 代码质量高

**可优化点**:
- 添加模型健康检查
- 实现模型故障转移
- 添加模型性能监控

### 优化建议

```go
// modelmgr/health.go
package modelmgr

import (
    "context"
    "time"
)

type HealthChecker struct {
    mgr      ModelManager
    interval time.Duration
}

func (hc *HealthChecker) Start(ctx context.Context) {
    ticker := time.NewTicker(hc.interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            models, _ := hc.mgr.ListModels(ctx)
            for _, model := range models {
                go hc.checkModel(ctx, model.ID)
            }
        case <-ctx.Done():
            return
        }
    }
}

func (hc *HealthChecker) checkModel(ctx context.Context, modelID int64) {
    // 简单的健康检查：尝试生成一个响应
    // 记录延迟和成功率
}
```

---

## 实施计划

### Week 1: Agent 优化
- Day 1-2: 统一 Agent 接口
- Day 3-4: Agent 包装器（日志、指标）
- Day 5: Agent 测试

### Week 2: Prompt 工程
- Day 1-2: Prompt 模板管理
- Day 3: 迁移现有 Prompt
- Day 4-5: Prompt 优化和测试

### Week 3: 可观测性
- Day 1-2: AI 指标收集
- Day 3: 流式处理优化
- Day 4-5: 文档和培训

---

**下一步**: 阅读 [待确认问题](./项目优化建议_待确认问题.md)
