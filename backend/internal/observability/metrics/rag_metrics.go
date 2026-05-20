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
