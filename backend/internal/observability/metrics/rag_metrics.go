package metrics

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
)

var (
	registerOnce sync.Once

	retrieveRequestsTotal *prometheus.CounterVec
	retrieveDuration      *prometheus.HistogramVec
	retrieveResultCount   prometheus.Histogram
	retrieveRouteRequests *prometheus.CounterVec
	retrieveRouteDuration *prometheus.HistogramVec
	retrieveRouteHits     *prometheus.HistogramVec
	retrieveStrategyTotal *prometheus.CounterVec
	retrieveEmptyReason   *prometheus.CounterVec
	retrieveRewriteTotal  *prometheus.CounterVec
	retrieveRerankLatency *prometheus.HistogramVec
	retrieveRouteContrib  *prometheus.CounterVec
	releaseRollbackTotal  *prometheus.CounterVec

	ingestJobsTotal *prometheus.CounterVec
	ingestDuration  *prometheus.HistogramVec

	errorTotal *prometheus.CounterVec

	consumerBacklog *prometheus.GaugeVec
	consumerPending *prometheus.GaugeVec
	consumerLag     *prometheus.GaugeVec
)

func init() {
	registerCollectors()
}

func registerCollectors() {
	registerOnce.Do(func() {
		retrieveRequestsTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rag_retrieve_requests_total",
				Help: "Total number of RAG retrieve requests by status and error code.",
			},
			[]string{"status", "error_code"},
		)
		retrieveDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "rag_retrieve_duration_seconds",
				Help:    "RAG retrieve request latency in seconds.",
				Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.2, 0.35, 0.5, 0.75, 1, 1.5, 2, 3, 5, 8},
			},
			[]string{"status"},
		)
		retrieveResultCount = prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "rag_retrieve_result_count",
				Help:    "RAG retrieve result count distribution per request.",
				Buckets: []float64{0, 1, 2, 3, 5, 8, 10, 15, 20, 30, 50},
			},
		)
		retrieveRouteRequests = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rag_retrieve_route_requests_total",
				Help: "Total number of route-level retrieve requests by route/status/error code.",
			},
			[]string{"route", "status", "error_code"},
		)
		retrieveRouteDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "rag_retrieve_route_duration_seconds",
				Help:    "Route-level retrieve latency in seconds.",
				Buckets: []float64{0.001, 0.005, 0.01, 0.02, 0.05, 0.1, 0.2, 0.35, 0.5, 0.75, 1, 1.5, 2, 3, 5},
			},
			[]string{"route", "status"},
		)
		retrieveRouteHits = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "rag_retrieve_route_hits",
				Help:    "Route-level hit count distribution per request.",
				Buckets: []float64{0, 1, 2, 3, 5, 8, 10, 15, 20, 30, 50},
			},
			[]string{"route"},
		)
		retrieveStrategyTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rag_retrieve_strategy_total",
				Help: "Total retrieve requests by effective strategy, release stage and decision reason.",
			},
			[]string{"strategy", "release_stage", "reason", "status"},
		)
		retrieveEmptyReason = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rag_retrieve_empty_reason_total",
				Help: "Empty-result reasons by strategy and release stage.",
			},
			[]string{"strategy", "release_stage", "empty_reason"},
		)
		retrieveRewriteTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rag_retrieve_rewrite_total",
				Help: "Rewrite applied counters by release stage and strategy.",
			},
			[]string{"strategy", "release_stage", "applied"},
		)
		retrieveRerankLatency = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "rag_retrieve_rerank_duration_seconds",
				Help:    "Rerank latency in seconds by release stage, model and status.",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.2, 0.35, 0.5, 0.75, 1, 1.5, 2},
			},
			[]string{"release_stage", "strategy", "model", "status"},
		)
		retrieveRouteContrib = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rag_retrieve_route_contribution_total",
				Help: "Final result contribution by route, strategy and release stage.",
			},
			[]string{"route", "strategy", "release_stage"},
		)
		releaseRollbackTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rag_release_rollback_total",
				Help: "Runtime release rollback and recovery actions.",
			},
			[]string{"action", "target_stage"},
		)
		ingestJobsTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rag_ingest_jobs_total",
				Help: "Total number of RAG ingest jobs by status and error code.",
			},
			[]string{"status", "error_code"},
		)
		ingestDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "rag_ingest_duration_seconds",
				Help:    "RAG ingest job latency in seconds.",
				Buckets: []float64{0.1, 0.25, 0.5, 1, 2, 3, 5, 8, 10, 15, 20, 30, 45, 60, 90, 120, 180, 300},
			},
			[]string{"status"},
		)
		errorTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rag_errors_total",
				Help: "Aggregated RAG error counters by scope and error code.",
			},
			[]string{"scope", "error_code"},
		)
		consumerBacklog = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rag_consumer_backlog",
				Help: "Approximate consumer backlog (pending + lag) per stream and group.",
			},
			[]string{"queue", "stream", "group", "message_type"},
		)
		consumerPending = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rag_consumer_pending",
				Help: "Pending (unacked) message count per stream and group.",
			},
			[]string{"queue", "stream", "group", "message_type"},
		)
		consumerLag = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rag_consumer_lag",
				Help: "Redis Stream lag reported by XINFO GROUPS.",
			},
			[]string{"queue", "stream", "group", "message_type"},
		)

		prometheus.MustRegister(
			retrieveRequestsTotal,
			retrieveDuration,
			retrieveResultCount,
			retrieveRouteRequests,
			retrieveRouteDuration,
			retrieveRouteHits,
			retrieveStrategyTotal,
			retrieveEmptyReason,
			retrieveRewriteTotal,
			retrieveRerankLatency,
			retrieveRouteContrib,
			releaseRollbackTotal,
			ingestJobsTotal,
			ingestDuration,
			errorTotal,
			consumerBacklog,
			consumerPending,
			consumerLag,
		)
	})
}

func ObserveRetrieve(duration time.Duration, status, errorCode string, resultCount int) {
	registerCollectors()
	s := sanitizeLabel(status, "unknown")
	e := sanitizeLabel(errorCode, "none")
	retrieveRequestsTotal.WithLabelValues(s, e).Inc()
	retrieveDuration.WithLabelValues(s).Observe(duration.Seconds())
	if resultCount >= 0 {
		retrieveResultCount.Observe(float64(resultCount))
	}
	if e != "none" {
		errorTotal.WithLabelValues("retrieve", e).Inc()
	}
}

func ObserveRetrieveRoute(route string, duration time.Duration, status, errorCode string, hitCount int) {
	registerCollectors()
	r := sanitizeLabel(route, "unknown")
	s := sanitizeLabel(status, "unknown")
	e := sanitizeLabel(errorCode, "none")
	retrieveRouteRequests.WithLabelValues(r, s, e).Inc()
	retrieveRouteDuration.WithLabelValues(r, s).Observe(duration.Seconds())
	if hitCount >= 0 {
		retrieveRouteHits.WithLabelValues(r).Observe(float64(hitCount))
	}
	if e != "none" {
		errorTotal.WithLabelValues("retrieve_route", e).Inc()
	}
}

func ObserveRetrieveStrategy(strategy, releaseStage, reason, status string) {
	registerCollectors()
	retrieveStrategyTotal.WithLabelValues(
		sanitizeLabel(strategy, "unknown"),
		sanitizeLabel(releaseStage, "unknown"),
		sanitizeLabel(reason, "none"),
		sanitizeLabel(status, "unknown"),
	).Inc()
}

func ObserveRetrieveEmptyReason(strategy, releaseStage, emptyReason string) {
	registerCollectors()
	retrieveEmptyReason.WithLabelValues(
		sanitizeLabel(strategy, "unknown"),
		sanitizeLabel(releaseStage, "unknown"),
		sanitizeLabel(emptyReason, "none"),
	).Inc()
}

func ObserveRetrieveRewrite(strategy, releaseStage string, applied bool) {
	registerCollectors()
	appliedLabel := "false"
	if applied {
		appliedLabel = "true"
	}
	retrieveRewriteTotal.WithLabelValues(
		sanitizeLabel(strategy, "unknown"),
		sanitizeLabel(releaseStage, "unknown"),
		appliedLabel,
	).Inc()
}

func ObserveRetrieveRerank(strategy, releaseStage, model, status string, duration time.Duration) {
	registerCollectors()
	retrieveRerankLatency.WithLabelValues(
		sanitizeLabel(releaseStage, "unknown"),
		sanitizeLabel(strategy, "unknown"),
		sanitizeLabel(model, "none"),
		sanitizeLabel(status, "none"),
	).Observe(duration.Seconds())
}

func ObserveRetrieveRouteContribution(route, strategy, releaseStage string, count int) {
	registerCollectors()
	if count <= 0 {
		return
	}
	retrieveRouteContrib.WithLabelValues(
		sanitizeLabel(route, "unknown"),
		sanitizeLabel(strategy, "unknown"),
		sanitizeLabel(releaseStage, "unknown"),
	).Add(float64(count))
}

func ObserveReleaseRollback(action, targetStage string) {
	registerCollectors()
	releaseRollbackTotal.WithLabelValues(
		sanitizeLabel(action, "unknown"),
		sanitizeLabel(targetStage, "unknown"),
	).Inc()
}

func ObserveIngest(duration time.Duration, status, errorCode string) {
	registerCollectors()
	s := sanitizeLabel(status, "unknown")
	e := sanitizeLabel(errorCode, "none")
	ingestJobsTotal.WithLabelValues(s, e).Inc()
	ingestDuration.WithLabelValues(s).Observe(duration.Seconds())
	if e != "none" {
		errorTotal.WithLabelValues("ingest", e).Inc()
	}
}

func IncError(scope, errorCode string) {
	registerCollectors()
	sc := sanitizeLabel(scope, "unknown")
	e := sanitizeLabel(errorCode, "unknown")
	errorTotal.WithLabelValues(sc, e).Inc()
}

func SetConsumerBacklog(queue, stream, group, messageType string, backlog, pending, lag int64) {
	registerCollectors()
	q := sanitizeLabel(queue, "unknown")
	s := sanitizeLabel(stream, "unknown")
	g := sanitizeLabel(group, "unknown")
	m := sanitizeLabel(messageType, "unknown")
	consumerBacklog.WithLabelValues(q, s, g, m).Set(float64(backlog))
	consumerPending.WithLabelValues(q, s, g, m).Set(float64(pending))
	consumerLag.WithLabelValues(q, s, g, m).Set(float64(lag))
}

func PrometheusHandler(ctx context.Context, c *app.RequestContext) {
	registerCollectors()
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		c.SetStatusCode(500)
		c.Header("Content-Type", "text/plain; charset=utf-8")
		_, _ = c.Write([]byte("failed to gather prometheus metrics"))
		return
	}

	var buf bytes.Buffer
	encoder := expfmt.NewEncoder(&buf, expfmt.FmtText)
	for _, mf := range mfs {
		if encodeErr := encoder.Encode(mf); encodeErr != nil {
			c.SetStatusCode(500)
			c.Header("Content-Type", "text/plain; charset=utf-8")
			_, _ = c.Write([]byte("failed to encode prometheus metrics"))
			return
		}
	}

	c.SetStatusCode(200)
	c.Header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = c.Write(buf.Bytes())
}

func sanitizeLabel(value, fallback string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return fallback
	}
	normalized := strings.Builder{}
	normalized.Grow(len(value))
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			normalized.WriteRune(r)
		case r >= '0' && r <= '9':
			normalized.WriteRune(r)
		default:
			normalized.WriteRune('_')
		}
	}
	out := strings.Trim(normalized.String(), "_")
	if out == "" {
		return fallback
	}
	return out
}
