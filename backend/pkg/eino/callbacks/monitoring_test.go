package callbacks

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/eino/callbacks"
)

func TestMonitoringHandler_TraceIDInjection(t *testing.T) {
	handler := NewMonitoringHandler()
	ctx := context.Background()

	info := &callbacks.RunInfo{
		Name:      "TestComponent",
		Component: "test",
	}

	// Call OnStart - should inject TraceID
	ctx = handler.OnStart(ctx, info, nil)

	// Verify TraceID was injected
	traceID, ok := ctx.Value(ctxKeyTraceID).(string)
	if !ok || traceID == "" {
		t.Errorf("Expected TraceID to be injected, got: %v", traceID)
	}

	// Call OnStart again with same context - should preserve TraceID
	originalTraceID := traceID
	ctx = handler.OnStart(ctx, info, nil)

	newTraceID, _ := ctx.Value(ctxKeyTraceID).(string)
	if newTraceID != originalTraceID {
		t.Errorf("Expected TraceID to be preserved, got different ID: %s vs %s", originalTraceID, newTraceID)
	}
}

func TestMonitoringHandler_StartTimeInjection(t *testing.T) {
	handler := NewMonitoringHandler()
	ctx := context.Background()

	info := &callbacks.RunInfo{
		Name:      "TestComponent",
		Component: "test",
	}

	beforeStart := time.Now()
	ctx = handler.OnStart(ctx, info, nil)
	afterStart := time.Now()

	// Verify start time was injected
	startTime, ok := ctx.Value(ctxKeyStartTime).(time.Time)
	if !ok {
		t.Fatal("Expected start time to be injected")
	}

	if startTime.Before(beforeStart) || startTime.After(afterStart) {
		t.Errorf("Start time %v is outside expected range [%v, %v]", startTime, beforeStart, afterStart)
	}
}

func TestMonitoringHandler_OnEnd(t *testing.T) {
	handler := NewMonitoringHandler()
	ctx := context.Background()

	info := &callbacks.RunInfo{
		Name:      "TestComponent",
		Component: "test",
	}

	// Start and wait a bit
	ctx = handler.OnStart(ctx, info, nil)
	time.Sleep(10 * time.Millisecond)

	// End - should not panic
	ctx = handler.OnEnd(ctx, info, nil)

	// Verify context still has TraceID
	traceID, ok := ctx.Value(ctxKeyTraceID).(string)
	if !ok || traceID == "" {
		t.Error("Expected TraceID to be preserved after OnEnd")
	}
}

func TestMonitoringHandler_OnEndWithOutput(t *testing.T) {
	handler := NewMonitoringHandler()
	ctx := context.Background()

	info := &callbacks.RunInfo{
		Name:      "TestComponent",
		Component: "test",
	}

	ctx = handler.OnStart(ctx, info, nil)

	// Should not panic with nil output
	ctx = handler.OnEnd(ctx, info, nil)

	// Verify TraceID preserved
	if _, ok := ctx.Value(ctxKeyTraceID).(string); !ok {
		t.Error("Expected TraceID to be preserved")
	}
}

func TestMonitoringHandler_OnError(t *testing.T) {
	handler := NewMonitoringHandler()
	ctx := context.Background()

	info := &callbacks.RunInfo{
		Name:      "TestComponent",
		Component: "test",
	}

	ctx = handler.OnStart(ctx, info, nil)

	// Call OnError - should not panic
	testErr := &testError{msg: "test error"}
	ctx = handler.OnError(ctx, info, testErr)

	// Verify TraceID preserved
	if _, ok := ctx.Value(ctxKeyTraceID).(string); !ok {
		t.Error("Expected TraceID to be preserved after OnError")
	}
}

// testError is a simple error implementation for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
