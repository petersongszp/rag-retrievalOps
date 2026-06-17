package kb

import (
	"context"
	"testing"

	authpkg "interview-agents/internal/auth"
	"interview-agents/internal/milvus/retrieval"
	"interview-agents/internal/model"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/hertz/pkg/app"
)

func TestMergeKnowledgeBaseSearchResultsRecomputesFinalRouteMetrics(t *testing.T) {
	densePrimary := &schema.Document{
		ID: "doc-dense",
		MetaData: map[string]interface{}{
			"score": 5.0,
			"route": "dense",
			"route_contrib": map[string]interface{}{
				"dense": 0.9,
				"sparse": 0.2,
			},
		},
	}
	sparsePrimary := &schema.Document{
		ID: "doc-sparse",
		MetaData: map[string]interface{}{
			"score": 4.0,
			"route": "sparse",
			"route_contrib": map[string]interface{}{
				"sparse": 0.8,
			},
		},
	}
	trimmedDense := &schema.Document{
		ID: "doc-trimmed",
		MetaData: map[string]interface{}{
			"score": 3.0,
			"route": "dense",
			"route_contrib": map[string]interface{}{
				"dense": 0.7,
			},
		},
	}

	merged := mergeKnowledgeBaseSearchResults([]*retrieval.SearchResult{
		{
			Documents: []*schema.Document{densePrimary, trimmedDense},
			Metrics: retrieval.SearchMetrics{
				DenseHits:             2,
				DenseParticipation:    2,
				PrimaryDenseCount:     2,
				DenseContribution:     2,
				CandidateTopK:         2,
				FinalTopK:             2,
				RetrieverVersion:      retrieval.HybridRetrieverVersion,
			},
		},
		{
			Documents: []*schema.Document{sparsePrimary},
			Metrics: retrieval.SearchMetrics{
				SparseHits:            3,
				SparseParticipation:   1,
				PrimarySparseCount:    1,
				SparseContribution:    1,
				CandidateTopK:         3,
				FinalTopK:             1,
				RetrieverVersion:      retrieval.HybridRetrieverVersion,
			},
		},
	}, "multi:kb-a,kb-b", 2, true)

	if got := len(merged.Documents); got != 2 {
		t.Fatalf("merged documents len = %d, want 2", got)
	}
	if merged.Metrics.DenseHits != 2 || merged.Metrics.SparseHits != 3 {
		t.Fatalf("raw hits should sum across KBs, got dense=%d sparse=%d", merged.Metrics.DenseHits, merged.Metrics.SparseHits)
	}
	if merged.Metrics.DenseParticipation != 1 || merged.Metrics.SparseParticipation != 2 {
		t.Fatalf("participation should be recomputed from final docs, got dense=%d sparse=%d", merged.Metrics.DenseParticipation, merged.Metrics.SparseParticipation)
	}
	if merged.Metrics.PrimaryDenseCount != 1 || merged.Metrics.PrimarySparseCount != 1 {
		t.Fatalf("primary counts should be recomputed from final docs, got dense=%d sparse=%d", merged.Metrics.PrimaryDenseCount, merged.Metrics.PrimarySparseCount)
	}
	if merged.Metrics.DualRouteFinalCount != 1 {
		t.Fatalf("DualRouteFinalCount = %d, want 1", merged.Metrics.DualRouteFinalCount)
	}
	if merged.Metrics.DenseContribution != 1 || merged.Metrics.SparseContribution != 1 {
		t.Fatalf("contribution should follow primary counts after recompute, got dense=%d sparse=%d", merged.Metrics.DenseContribution, merged.Metrics.SparseContribution)
	}
	if merged.Metrics.CandidateTopK != 3 || merged.Metrics.FinalTopK != 2 {
		t.Fatalf("topk metrics = candidate:%d final:%d", merged.Metrics.CandidateTopK, merged.Metrics.FinalTopK)
	}
}

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
		DenseHits:          3,
		SparseHits:         2,
		DenseParticipation: 1,
		SparseParticipation: 1,
		PrimaryDenseCount:  1,
		PrimarySparseCount: 0,
		DualRouteFinalCount: 1,
		DenseContribution:  1,
		SparseContribution: 0,
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
	if decoded.Routes[0].Participation != 1 || decoded.Routes[0].PrimaryCount != 1 {
		t.Fatalf("decoded dense route counters = %#v", decoded.Routes[0])
	}
	if decoded.DenseParticipation != 0 || decoded.PrimaryDenseCount != 0 {
		t.Fatalf("log-backed counters should stay zero without log fallback, got trace=%#v", decoded)
	}
}

func TestBuildRetrievalDebugTraceResponseUsesStructuredDebugTrace(t *testing.T) {
	logEntry := &model.KBRetrieveLog{
		RequestID:         "req-l1",
		Query:             "java lock",
		FinalQuery:        "java lock",
		KBIDs:             "1,2",
		RewriteGainBucket: "route_gain_candidate",
	}
	metrics := retrieval.SearchMetrics{
		OriginalQuery:                  "java lock",
		RewriteQuery:                   "java lock aqs",
		RewriteStrategy:                "rule_based+domain_terms",
		FinalQuery:                     "java lock aqs",
		DenseQuery:                     "java lock",
		SparseQuery:                    "java lock aqs",
		TermHits:                       []string{"aqs", "lock"},
		CandidateTopK:                  10,
		FinalTopK:                      4,
		TokenBudget:                    800,
		TokenBudgetRemain:              120,
		TopKPolicyVersion:              "phase3-strategic-v1",
		TopKDecisionReason:             "high_evidence_density",
		EvidenceGateResult:             retrieval.EvidenceGateResultPass,
		CitationSupported:              true,
		CitationSupportScore:           0.94,
		CitationCheckVersion:           "citation-v1",
		CitationCheckLatencyMs:         18,
		UnsupportedClaims:              []string{"claim-a"},
		ParentChildEnabled:             true,
		ParentFillStrategy:             "section_window",
		ParentFillCount:                2,
		ParentFillTokens:               60,
		SemanticCacheEnabled:           true,
		SemanticCacheHit:               true,
		SemanticCacheLookupMs:          7,
		SemanticCacheSimilarity:        0.98,
		SemanticCacheEntryID:           "entry-1",
		SemanticCacheReason:            "hit",
		SemanticCacheCandidateCount:    3,
		SemanticCacheThreshold:         0.95,
		SemanticCacheHighestSimilarity: 0.98,
	}
	debugTrace := &retrieval.DebugTrace{
		RouteHits: []retrieval.RouteDebugHit{
			{Route: "dense", Query: "java lock", HitsCount: 5, ParticipationCount: 3, PrimaryCount: 2, Contribution: 2},
			{Route: "sparse", Query: "java lock aqs", HitsCount: 4, ParticipationCount: 2, PrimaryCount: 2, Contribution: 2},
		},
		Fusion: retrieval.FusionDebugInfo{
			Before:                   []retrieval.DebugDocument{{ChunkID: "c1"}},
			After:                    []retrieval.DebugDocument{{ChunkID: "c2"}},
			DenseParticipationCount:  3,
			SparseParticipationCount: 2,
			DualRouteFinalCount:      1,
			PrimaryRouteDistribution: map[string]int{"dense": 2, "sparse": 2},
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
	if trace.RouteHits[0].HitsCount != 5 || trace.RouteHits[0].ParticipationCount != 3 || trace.RouteHits[0].PrimaryCount != 2 {
		t.Fatalf("dense route hit counters = %#v", trace.RouteHits[0])
	}
	if trace.RewriteStrategy != "rule_based+domain_terms" {
		t.Fatalf("RewriteStrategy = %q", trace.RewriteStrategy)
	}
	if trace.RewriteGainBucket != "route_gain_candidate" {
		t.Fatalf("RewriteGainBucket = %q", trace.RewriteGainBucket)
	}
	if len(trace.TermHits) != 2 || trace.TermHits[0] != "aqs" {
		t.Fatalf("TermHits = %#v", trace.TermHits)
	}
	if trace.TopKDecision.TopKPolicyVersion != "phase3-strategic-v1" {
		t.Fatalf("TopKPolicyVersion = %q", trace.TopKDecision.TopKPolicyVersion)
	}
	if !trace.SemanticCache.Hit || trace.SemanticCache.EntryID != "entry-1" {
		t.Fatalf("SemanticCache = %#v", trace.SemanticCache)
	}
	if len(trace.ParentChild.ParentContexts) != 1 || trace.ParentChild.ParentContexts[0].ParentID != "p1" {
		t.Fatalf("ParentContexts = %#v", trace.ParentChild.ParentContexts)
	}
	if trace.FusionResults.DenseParticipationCount != 3 || trace.FusionResults.PrimaryRouteDistribution["dense"] != 2 {
		t.Fatalf("FusionResults = %#v", trace.FusionResults)
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
		OriginalQuery:         "legacy query",
		RewriteQuery:          "legacy rewrite",
		FinalQuery:            "legacy final",
		DenseQuery:            "legacy query",
		SparseQuery:           "legacy final",
		CandidateTopK:         5,
		FinalTopK:             0,
		TokenBudget:           300,
		TokenBudgetRemain:     300,
		ParentChildEnabled:    false,
		RewriteApplied:        true,
		EvidenceGateResult:    retrieval.EvidenceGateResultDisabled,
		CitationSupported:     false,
		CitationSupportScore: 0,
		SemanticCacheEnabled:  true,
		SemanticCacheReason:   "miss",
		DenseHits:             6,
		SparseHits:            4,
		DenseParticipation:    2,
		SparseParticipation:   1,
		PrimaryDenseCount:     0,
		PrimarySparseCount:    0,
		DualRouteFinalCount:   1,
		DenseContribution:     0,
		SparseContribution:    0,
		SparseCandidateBefore: 9,
		SparseCandidateAfter:  4,
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
	if trace.SemanticCache.Reason != "miss" {
		t.Fatalf("SemanticCache.Reason = %q, want miss", trace.SemanticCache.Reason)
	}
	if trace.RouteHits[0].HitsCount != 6 || trace.RouteHits[0].ParticipationCount != 2 {
		t.Fatalf("fallback dense route = %#v", trace.RouteHits[0])
	}
	if trace.RouteHits[1].CandidateCountBefore != 9 || trace.RouteHits[1].CandidateCountAfter != 4 {
		t.Fatalf("fallback sparse route = %#v", trace.RouteHits[1])
	}
}

func TestEnrichRetrieveLogWithPlatformContextForAPIKey(t *testing.T) {
	ctx := authpkg.WithIdentity(context.Background(), &authpkg.Identity{
		AuthType: authpkg.AuthTypeAPIKey,
		TenantID: 12,
		UserID:   7,
		AppID:    "support-bot",
		APIKeyID: 34,
	})
	c := &app.RequestContext{}
	c.Request.SetRequestURI("/v1/retrieve")
	c.Set("auth_type", "api_key")
	c.Set("tenant_id", uint64(12))
	c.Set("app_id", "support-bot")
	c.Set("api_key_id", uint64(34))

	entry := &model.KBRetrieveLog{RequestID: "req-platform"}
	enrichRetrieveLogWithPlatformContext(ctx, c, entry, "allowed")

	if entry.TenantID != 12 || entry.AppID != "support-bot" || entry.APIKeyID != 34 {
		t.Fatalf("unexpected platform identifiers: %+v", entry)
	}
	if entry.AuthType != "api_key" || entry.SourceAPI != "v1" || entry.PermissionResult != "allowed" {
		t.Fatalf("unexpected auth trace fields: %+v", entry)
	}
	if entry.IsLegacy {
		t.Fatalf("IsLegacy = true, want false")
	}
}

func TestEnrichRetrieveLogWithPlatformContextForLegacy(t *testing.T) {
	ctx := authpkg.WithIdentity(context.Background(), &authpkg.Identity{
		AuthType: authpkg.AuthTypeLegacyAppID,
		TenantID: 1,
		UserID:   1,
		AppID:    "interview-agent",
		IsLegacy: true,
	})
	c := &app.RequestContext{}
	c.Request.SetRequestURI("/v1/retrieve")
	c.Set("auth_type", "legacy_app_id")
	c.Set("tenant_id", uint64(1))
	c.Set("app_id", "interview-agent")
	c.Set("is_legacy", true)

	entry := &model.KBRetrieveLog{RequestID: "req-legacy"}
	enrichRetrieveLogWithPlatformContext(ctx, c, entry, "allowed")

	if entry.AuthType != "legacy_app_id" || entry.SourceAPI != "v1" {
		t.Fatalf("unexpected auth/source for legacy request: %+v", entry)
	}
	if !entry.IsLegacy {
		t.Fatalf("IsLegacy = false, want true")
	}
}
