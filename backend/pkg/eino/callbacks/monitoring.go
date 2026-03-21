package callbacks

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/google/uuid"

	"interview-agents/internal/alert"
	"interview-agents/internal/observability/looptrace"
)

// Context keys for tracing
type contextKey string

const (
	ctxKeyTraceID   contextKey = "traceID"
	ctxKeyStartTime contextKey = "componentStartTime"
	ctxKeyUserID    contextKey = "userID" // 11.2.3 用于 Token 监控与配额（可选注入）
	ctxKeyLoopState contextKey = "cozeloopState"
)

type loopSpanState struct {
	span cozeloopSpan
	err  error
	once sync.Once
}

type cozeloopSpan interface {
	SetError(ctx context.Context, err error)
	SetInputTokens(ctx context.Context, inputTokens int)
	SetOutputTokens(ctx context.Context, outputTokens int)
	SetOutput(ctx context.Context, output interface{})
	Finish(ctx context.Context)
}

// 11.2.2 实例池：sync.Pool 在 Agent 监控路径中复用 bytes.Buffer，减少分配
var bufferPool = sync.Pool{
	New: func() interface{} { return new(bytes.Buffer) },
}

// WithTraceID 从外部注入 TraceID 到 context，用于整条调用链共享同一 Trace（如一场面试一个 SessionID）
func WithTraceID(ctx context.Context, traceID string) context.Context {
	if traceID == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyTraceID, traceID)
}

func TraceIDFromContext(ctx context.Context) string {
	traceID, _ := ctx.Value(ctxKeyTraceID).(string)
	return traceID
}

// WithUserID 11.2.3 注入 UserID 到 context，供 Token 监控/配额使用（如面试引擎在 RunInterviewLoopWithGraph 前调用）
func WithUserID(ctx context.Context, userID uint) context.Context {
	return context.WithValue(ctx, ctxKeyUserID, userID)
}

func UserIDFromContext(ctx context.Context) uint {
	userID, _ := ctx.Value(ctxKeyUserID).(uint)
	return userID
}

// TokenRecorder 11.2.3 推理成本控制：Token 消耗记录与配额检查（可由 internal/agents/llm/tokenquota 实现）
type TokenRecorder interface {
	Record(ctx context.Context, userID uint, promptTokens, completionTokens, totalTokens int64)
	CheckQuota(ctx context.Context, userID uint) error
}

// DefaultTokenRecorder 应用启动时注入（如 main 或面试引擎初始化时），未设置则不记录/不检查配额
var DefaultTokenRecorder TokenRecorder

// MonitoringHandler 实现了 Eino 的 callbacks.Handler 接口
// 用于监控组件运行状态、耗时和错误
type MonitoringHandler struct {
}

func NewMonitoringHandler() *MonitoringHandler {
	return &MonitoringHandler{}
}

// OnStart 组件开始运行时调用
func (h *MonitoringHandler) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	// Generate or retrieve TraceID
	traceID, ok := ctx.Value(ctxKeyTraceID).(string)
	if !ok || traceID == "" {
		traceID = uuid.New().String()
		ctx = context.WithValue(ctx, ctxKeyTraceID, traceID)
	}

	log.Printf("[Eino Monitor] [TraceID: %s] Start Component: %s (%s)", traceID, info.Name, info.Component)
	ctx = context.WithValue(ctx, ctxKeyStartTime, time.Now())

	spanName := info.Name
	if info.Component != "" {
		spanName = fmt.Sprintf("%s.%s", info.Name, info.Component)
	}
	if nextCtx, span, ok := looptrace.StartSpan(ctx, spanName, "custom"); ok && span != nil {
		ctx = nextCtx
		looptrace.ApplyCommonFields(ctx, span, strconv.FormatUint(uint64(UserIDFromContext(ctx)), 10), traceID, map[string]interface{}{
			"component_name": info.Name,
			"component_type": info.Component,
			"source":         "eino_callback",
		})
		ctx = context.WithValue(ctx, ctxKeyLoopState, &loopSpanState{span: span})
	}

	return ctx
}

// OnEnd 组件运行结束时调用
func (h *MonitoringHandler) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	traceID, _ := ctx.Value(ctxKeyTraceID).(string)
	startTime, ok := ctx.Value(ctxKeyStartTime).(time.Time)

	var latencyMs int64
	if ok {
		latencyMs = time.Since(startTime).Milliseconds()
	}

	// Try to extract token usage from output
	tokenInfo := extractTokenUsage(output)
	// 11.2.3 Token 监控：若有注入 Recorder 且 context 带 UserID，则记录本次消耗
	if DefaultTokenRecorder != nil {
		if uid, _ := ctx.Value(ctxKeyUserID).(uint); uid != 0 {
			if p, c, t, ok := extractTokenUsageValues(output); ok && t > 0 {
				DefaultTokenRecorder.Record(ctx, uid, p, c, t)
			}
		}
	}

	log.Printf("[Eino Monitor] [TraceID: %s] End Component: %s (%s), Latency: %dms%s",
		traceID, info.Name, info.Component, latencyMs, tokenInfo)

	if state, _ := ctx.Value(ctxKeyLoopState).(*loopSpanState); state != nil && state.span != nil {
		state.once.Do(func() {
			if p, c, _, ok := extractTokenUsageValues(output); ok {
				state.span.SetInputTokens(ctx, int(p))
				state.span.SetOutputTokens(ctx, int(c))
			}
			state.span.SetOutput(ctx, map[string]interface{}{
				"component_name": info.Name,
				"component_type": info.Component,
				"latency_ms":     latencyMs,
				"has_error":      state.err != nil,
			})
			state.span.Finish(ctx)
		})
	}

	return ctx
}

// OnEndWithStreamOutput 组件流式运行结束时调用
func (h *MonitoringHandler) OnEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo, output interface{}) context.Context {
	traceID, _ := ctx.Value(ctxKeyTraceID).(string)
	startTime, ok := ctx.Value(ctxKeyStartTime).(time.Time)

	var latencyMs int64
	if ok {
		latencyMs = time.Since(startTime).Milliseconds()
	}

	log.Printf("[Eino Monitor] [TraceID: %s] Stream Start: %s (%s), Latency: %dms",
		traceID, info.Name, info.Component, latencyMs)

	return ctx
}

// OnError 组件发生错误时调用（12.2.2 系统异常告警：同步打日志，异步发飞书）
func (h *MonitoringHandler) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	traceID, _ := ctx.Value(ctxKeyTraceID).(string)
	component := fmt.Sprintf("%s (%v)", info.Name, info.Component)
	log.Printf("[Eino Monitor] [TraceID: %s] Error Component: %s, Error: %v", traceID, component, err)
	if err != nil {
		alert.SystemException(traceID, component, err.Error())
	}
	if state, _ := ctx.Value(ctxKeyLoopState).(*loopSpanState); state != nil && state.span != nil && err != nil {
		state.err = err
		state.span.SetError(ctx, err)
	}
	return ctx
}

// extractTokenUsageValues 从 CallbackOutput 反射取出 prompt/completion/total token 数，供 11.2.3 记录与监控
func extractTokenUsageValues(output callbacks.CallbackOutput) (promptTokens, completionTokens, totalTokens int64, ok bool) {
	if output == nil {
		return 0, 0, 0, false
	}
	val := reflect.ValueOf(output)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return 0, 0, 0, false
	}
	messageField := val.FieldByName("Message")
	if !messageField.IsValid() || messageField.IsNil() {
		return 0, 0, 0, false
	}
	msgVal := messageField.Elem()
	metaField := msgVal.FieldByName("ResponseMeta")
	if !metaField.IsValid() || metaField.IsNil() {
		return 0, 0, 0, false
	}
	metaVal := metaField.Elem()
	usageField := metaVal.FieldByName("Usage")
	if !usageField.IsValid() || usageField.IsNil() {
		return 0, 0, 0, false
	}
	usageVal := usageField.Elem()
	p := usageVal.FieldByName("PromptTokens")
	c := usageVal.FieldByName("CompletionTokens")
	t := usageVal.FieldByName("TotalTokens")
	if !p.IsValid() || !c.IsValid() || !t.IsValid() {
		return 0, 0, 0, false
	}
	return p.Int(), c.Int(), t.Int(), true
}

// extractTokenUsage 从 CallbackOutput 反射取出 token 并格式化为日志字符串；11.2.2 使用 sync.Pool 的 Buffer
func extractTokenUsage(output callbacks.CallbackOutput) string {
	p, c, t, ok := extractTokenUsageValues(output)
	if !ok {
		return ""
	}
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)
	_, _ = fmt.Fprintf(buf, ", Tokens: input=%d, output=%d, total=%d", p, c, t)
	return buf.String()
}
