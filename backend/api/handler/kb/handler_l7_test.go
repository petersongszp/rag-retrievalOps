package kb

import (
	"testing"

	"interview-agents/internal/milvus/retrieval"
	"interview-agents/internal/model"

	"github.com/cloudwego/eino/schema"
)

func TestClassifyRewriteGainBucket(t *testing.T) {
	tests := []struct {
		name       string
		metrics    retrieval.SearchMetrics
		finalCount int
		status     model.RetrieveResultStatus
		want       string
	}{
		{
			name:   "not applied",
			status: model.RetrieveResultStatusSuccess,
			want:   "not_applied",
		},
		{
			name: "model rewrite success",
			metrics: retrieval.SearchMetrics{
				RewriteApplied:      true,
				ModelRewriteApplied: true,
			},
			finalCount: 2,
			status:     model.RetrieveResultStatusSuccess,
			want:       "model_gain_candidate",
		},
		{
			name: "refusal risk",
			metrics: retrieval.SearchMetrics{
				RewriteApplied:     true,
				EvidenceGateResult: retrieval.EvidenceGateResultRefused,
			},
			status: model.RetrieveResultStatusFilteredOut,
			want:   "risk_refusal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyRewriteGainBucket(tt.metrics, tt.finalCount, tt.status); got != tt.want {
				t.Fatalf("classifyRewriteGainBucket() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildRetrieveDebugTraceIncludesParentFillDiff(t *testing.T) {
	logEntry := &model.KBRetrieveLog{
		RequestID:         "req-1",
		Query:             "what is mvcc",
		FinalQuery:        "what is mvcc",
		RewriteGainBucket: "gain_candidate",
		ResultStatus:      model.RetrieveResultStatusSuccess,
	}
	metrics := retrieval.SearchMetrics{
		OriginalQuery:      "what is mvcc",
		RewriteQuery:       "mvcc multi version concurrency control",
		FinalQuery:         "mvcc multi version concurrency control",
		DenseQuery:         "mvcc",
		SparseQuery:        "mvcc multi version concurrency control",
		RouteRewriteDense:  "rule_based",
		RouteRewriteSparse: "rule_based+route_specific:aggressive",
		RewriteApplied:     true,
		ParentChildEnabled: true,
		ParentFillStrategy: "section_window",
		ParentFillCount:    1,
	}
	docs := []*schema.Document{
		{
			ID:      "chunk-1",
			Content: "expanded content with parent context",
			MetaData: map[string]interface{}{
				"chunk_id":               "chunk-1",
				"parent_id":              "parent-1",
				"parent_fill_strategy":   "section_window",
				"parent_fill_reason":     "applied",
				"parent_fill_applied":    true,
				"parent_fill_count":      1,
				"parent_fill_tokens":     42,
				"original_child_content": "child content",
			},
		},
	}

	trace := buildRetrieveDebugTrace(logEntry, metrics, docs, nil)
	if trace.ParentChild.FillCount != 1 {
		t.Fatalf("ParentChild.FillCount = %d, want 1", trace.ParentChild.FillCount)
	}
	if len(trace.ParentChild.Items) != 1 {
		t.Fatalf("ParentChild.Items len = %d, want 1", len(trace.ParentChild.Items))
	}
	if trace.ParentChild.Items[0].BeforeContent != "child content" {
		t.Fatalf("BeforeContent = %q, want child content", trace.ParentChild.Items[0].BeforeContent)
	}

	raw := encodeRetrieveDebugTrace(trace)
	decoded, err := decodeRetrieveDebugTrace(raw)
	if err != nil {
		t.Fatalf("decodeRetrieveDebugTrace error = %v", err)
	}
	if decoded == nil || decoded.Rewrite.GainBucket != "gain_candidate" {
		t.Fatalf("decoded rewrite gain bucket = %#v", decoded)
	}
}
