package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	registerOnce sync.Once

	toolCallTotal *prometheus.CounterVec
	toolCallDuration *prometheus.HistogramVec
	toolCallItems prometheus.Histogram

	upstreamErrorTotal *prometheus.CounterVec
	authMissingTotal   prometheus.Counter
	forbiddenTotal     prometheus.Counter
	backendTimeoutTotal prometheus.Counter
)

func register() {
	registerOnce.Do(func() {
		toolCallTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcp_tool_call_total",
				Help: "Total number of MCP tool calls by tool, status, and error code.",
			},
			[]string{"tool", "status", "error_code"},
		)
		toolCallDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mcp_tool_call_duration_ms",
				Help:    "Latency distribution of MCP tool calls in milliseconds.",
				Buckets: []float64{5, 10, 25, 50, 100, 200, 350, 500, 1000, 2000, 5000, 10000},
			},
			[]string{"tool", "status"},
		)
		toolCallItems = prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "mcp_tool_call_result_count",
				Help:    "Result item count returned by MCP tool calls.",
				Buckets: []float64{0, 1, 2, 3, 5, 8, 10, 20, 50},
			},
		)
		upstreamErrorTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcp_upstream_error_total",
				Help: "Total number of upstream RAG errors observed by the MCP server.",
			},
			[]string{"error_code"},
		)
		authMissingTotal = prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "mcp_auth_missing_total",
				Help: "Total number of MCP requests rejected for missing authentication.",
			},
		)
		forbiddenTotal = prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "mcp_forbidden_total",
				Help: "Total number of forbidden responses returned via MCP.",
			},
		)
		backendTimeoutTotal = prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "mcp_backend_timeout_total",
				Help: "Total number of upstream backend timeouts returned via MCP.",
			},
		)
		prometheus.MustRegister(
			toolCallTotal,
			toolCallDuration,
			toolCallItems,
			upstreamErrorTotal,
			authMissingTotal,
			forbiddenTotal,
			backendTimeoutTotal,
		)
	})
}

func ObserveToolCall(tool, status, errorCode string, durationMs float64, resultCount int) {
	register()
	toolLabel := sanitize(tool, "unknown")
	statusLabel := sanitize(status, "unknown")
	toolCallTotal.WithLabelValues(
		toolLabel,
		statusLabel,
		sanitize(errorCode, "none"),
	).Inc()
	if durationMs >= 0 {
		toolCallDuration.WithLabelValues(toolLabel, statusLabel).Observe(durationMs)
	}
	if resultCount >= 0 {
		toolCallItems.Observe(float64(resultCount))
	}
}

func IncUpstreamError(errorCode string) {
	register()
	upstreamErrorTotal.WithLabelValues(sanitize(errorCode, "unknown")).Inc()
}

func IncAuthMissing() {
	register()
	authMissingTotal.Inc()
}

func IncForbidden() {
	register()
	forbiddenTotal.Inc()
}

func IncBackendTimeout() {
	register()
	backendTimeoutTotal.Inc()
}

func sanitize(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
