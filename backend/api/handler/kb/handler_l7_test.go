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

func TestBuildRetrievalDebugTraceResponseUsesStructuredDebugTrace(t *testing.T) {
	logEntry := &model.KBRetrieveLog{
		RequestID:  "req-l1",
		Query:      "java lock",
		FinalQuery: "java lock",
		KBIDs:      "1,2",
	}
	metrics := retrieval.SearchMetrics{
		OriginalQuery:          "java lock",
		RewriteQuery:           "java lock aqs",
		FinalQuery:             "java lock aqs",
		DenseQuery:             "java lock",
		SparseQuery:            "java lock aqs",
		CandidateTopK:          10,
		FinalTopK:              4,
		TokenBudget:            800,
		TokenBudgetRemain:      120,
		TopKPolicyVersion:      "phase3-strategic-v1",
		TopKDecisionReason:     "high_evidence_density",
		EvidenceGateResult:     retrieval.EvidenceGateResultPass,
		CitationSupported:      true,
		CitationSupportScore:   0.94,
		CitationCheckVersion:   "citation-v1",
		CitationCheckLatencyMs: 18,
		UnsupportedClaims:      []string{"claim-a"},
		ParentChildEnabled:     true,
		ParentFillStrategy:     "section_window",
		ParentFillCount:        2,
		ParentFillTokens:       60,
	}
	debugTrace := &retrieval.DebugTrace{
		RouteHits: []retrieval.RouteDebugHit{
			{Route: "dense", Query: "java lock", Contribution: 2},
			{Route: "sparse", Query: "java lock aqs", Contribution: 2},
		},
		Fusion: retrieval.FusionDebugInfo{
			Before: []retrieval.DebugDocument{{ChunkID: "c1"}},
			After:  []retrieval.DebugDocument{{ChunkID: "c2"}},
		},
		ParentChild: retrieval.ParentChildDebugInfo{
			ChildHits:      []retrieval.DebugDocument{{ChunkID: "c3"}},
			ParentContexts: []retrieval.DebugDocument{{ChunkID: "c4", ParentID: "p1"}},
		},
	}

	trace := buildRetrievalDebugTraceResponse(logEntry, metrics, nil, debugTrace)
	if !trace.DebugAvailable {
		t.Fatal("DebugAvailable = false, want true")
	}
	if len(trace.KBIDs) != 2 || trace.KBIDs[0] != 1 || trace.KBIDs[1] != 2 {
		t.Fatalf("KBIDs = %#v, want [1 2]", trace.KBIDs)
	}
	if len(trace.RouteHits) != 2 {
		t.Fatalf("RouteHits len = %d, want 2", len(trace.RouteHits))
	}
	if trace.TopKDecision.TopKPolicyVersion != "phase3-strategic-v1" {
		t.Fatalf("TopKPolicyVersion = %q", trace.TopKDecision.TopKPolicyVersion)
	}
	if len(trace.ParentChild.ParentContexts) != 1 || trace.ParentChild.ParentContexts[0].ParentID != "p1" {
		t.Fatalf("ParentContexts = %#v", trace.ParentChild.ParentContexts)
	}
	if len(trace.ContractGaps) != 0 {
		t.Fatalf("ContractGaps = %#v, want empty", trace.ContractGaps)
	}
}

func TestBuildRetrievalDebugTraceResponseFallbackMarksContractGaps(t *testing.T) {
	logEntry := &model.KBRetrieveLog{
		RequestID:          "req-legacy",
		Query:              "legacy query",
		Rewrite:            "legacy rewrite",
		FinalQuery:         "legacy final",
		TopKDecisionReason: "legacy_reason",
		KBIDs:              "9",
	}
	metrics := retrieval.SearchMetrics{
		OriginalQuery:        "legacy query",
		RewriteQuery:         "legacy rewrite",
		FinalQuery:           "legacy final",
		DenseQuery:           "legacy query",
		SparseQuery:          "legacy final",
		CandidateTopK:        5,
		FinalTopK:            0,
		TokenBudget:          300,
		TokenBudgetRemain:    300,
		ParentChildEnabled:   false,
		RewriteApplied:       true,
		EvidenceGateResult:   retrieval.EvidenceGateResultDisabled,
		CitationSupported:    false,
		CitationSupportScore: 0,
	}

	trace := buildRetrievalDebugTraceResponse(logEntry, metrics, nil, nil)
	if trace.DebugAvailable {
		t.Fatal("DebugAvailable = true, want false")
	}
	if len(trace.ContractGaps) == 0 {
		t.Fatal("ContractGaps = empty, want fallback gaps")
	}
	if !trace.Degradation.Enabled || trace.Degradation.ErrorCode != "debug_trace_unavailable" {
		t.Fatalf("Degradation = %#v", trace.Degradation)
	}
	if len(trace.RouteHits) != 2 {
		t.Fatalf("fallback RouteHits len = %d, want 2", len(trace.RouteHits))
	}
}
