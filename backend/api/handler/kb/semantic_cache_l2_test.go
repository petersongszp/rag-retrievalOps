package kb

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	semanticcache "interview-agents/internal/cache/semantic"
	"interview-agents/internal/config"
	"interview-agents/internal/rag/experiment"
	"interview-agents/internal/rag/release"
)

type fakeSemanticCacheStore struct {
	lookup    *semanticcache.LookupResult
	err       error
	touchN    int
	putN      int
	delN      int
	putScope  semanticcache.Scope
	putEntry  *semanticcache.Entry
	delTenant uint64
	delKB     uint64
}

func (f *fakeSemanticCacheStore) GetCandidates(_ context.Context, _ semanticcache.Scope, _ int) (*semanticcache.LookupResult, error) {
	return f.lookup, f.err
}

func (f *fakeSemanticCacheStore) Touch(_ context.Context, _ *semanticcache.Entry, _ time.Duration) error {
	f.touchN++
	return nil
}

func (f *fakeSemanticCacheStore) Put(_ context.Context, scope semanticcache.Scope, entry *semanticcache.Entry, _ time.Duration, _ int) error {
	f.putN++
	f.putScope = scope
	f.putEntry = entry
	return nil
}

func (f *fakeSemanticCacheStore) DeleteByKnowledgeBase(_ context.Context, tenantID uint64, kbID uint64) error {
	f.delN++
	f.delTenant = tenantID
	f.delKB = kbID
	return nil
}

type fakeSemanticCacheEmbedder struct {
	vectors [][]float64
	err     error
}

func (f *fakeSemanticCacheEmbedder) EmbedBatch(_ context.Context, _ []string) ([][]float64, error) {
	return f.vectors, f.err
}

func TestSemanticCacheBypassReason(t *testing.T) {
	original := config.Global
	t.Cleanup(func() {
		config.Global = original
	})

	config.Global.RAG.FeatureFlags.EnableSemanticCache = true

	reason := semanticCacheBypassReason(semanticCacheLookupInput{
		Query:           "what is cache",
		TopK:            5,
		KBIDs:           []uint64{1},
		QueryType:       "general",
		StrategyVersion: "phase1",
		DebugRequested:  true,
		TenantID:        9,
	})
	if reason != "debug_request" {
		t.Fatalf("reason = %q, want debug_request", reason)
	}

	reason = semanticCacheBypassReason(semanticCacheLookupInput{
		Query:              "what is cache",
		TopK:               5,
		KBIDs:              []uint64{1},
		QueryType:          "general",
		StrategyVersion:    "phase1",
		TenantID:           9,
		ExperimentDecision: experiment.Decision{Group: experiment.GroupShadow},
	})
	if reason != "high_risk_experiment" {
		t.Fatalf("reason = %q, want high_risk_experiment", reason)
	}
}

func TestTrySemanticCacheHitRequiresExactTopK(t *testing.T) {
	original := config.Global
	t.Cleanup(func() {
		config.Global = original
	})

	config.Global.RAG.FeatureFlags.EnableSemanticCache = true
	config.Global.RAG.SemanticCache.SimilarityThreshold = 0.90
	config.Global.RAG.SemanticCache.MaxCandidates = 5
	config.Global.RAG.SemanticCache.TTLSeconds = 60

	payload, err := json.Marshal(retrieveResponse{
		RequestID: "old",
		Items: []retrieveItem{
			{
				Content: "semantic cache doc",
				Citation: citation{
					KBID:       1,
					DocumentID: 2,
				},
				Source: source{
					Route:            "dense",
					RetrieverVersion: "phase1-dense-v1",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	store := &fakeSemanticCacheStore{
		lookup: &semanticcache.LookupResult{
			Scope: semanticcache.Scope{
				TenantID:        9,
				KBIDs:           []uint64{1},
				StrategyVersion: "phase1",
				QueryType:       "general",
			},
			Candidates: []*semanticcache.Entry{
				{
					EntryID:          "entry-topk-3",
					TenantID:         9,
					KBIDs:            []uint64{1},
					StrategyVersion:  "phase1",
					QueryType:        "general",
					Query:            "what is cache",
					QueryEmbedding:   []float32{1, 0},
					ResponsePayload:  payload,
					ResultPayload:    semanticcache.ResultPayloadTag,
					TopK:             3,
					RetrieverVersion: "phase1-dense-v1",
				},
			},
			CandidateCount: 1,
		},
	}
	embedder := &fakeSemanticCacheEmbedder{vectors: [][]float64{{1, 0}}}

	hit, trace, err := trySemanticCacheHit(context.Background(), semanticCacheLookupInput{
		RequestID:          "req-1",
		TenantID:           9,
		Query:              "what is cache",
		TopK:               5,
		KBIDs:              []uint64{1},
		QueryType:          "general",
		StrategyVersion:    "phase1",
		ExperimentDecision: experiment.Decision{Group: experiment.GroupBaseline},
	}, store, embedder)
	if err != nil {
		t.Fatalf("trySemanticCacheHit err: %v", err)
	}
	if hit != nil {
		t.Fatalf("expected no hit when topk mismatches, got %+v", hit)
	}
	if trace.Reason != "miss" {
		t.Fatalf("reason = %q, want miss", trace.Reason)
	}
	if trace.CandidateCount != 1 {
		t.Fatalf("candidate count = %d, want 1", trace.CandidateCount)
	}
}

func TestTrySemanticCacheHitReturnsScopedResponse(t *testing.T) {
	original := config.Global
	t.Cleanup(func() {
		config.Global = original
	})

	config.Global.RAG.FeatureFlags.EnableSemanticCache = true
	config.Global.RAG.SemanticCache.SimilarityThreshold = 0.90
	config.Global.RAG.SemanticCache.MaxCandidates = 5
	config.Global.RAG.SemanticCache.TTLSeconds = 60

	payload, err := json.Marshal(retrieveResponse{
		RequestID: "old-request",
		Items: []retrieveItem{
			{
				Content: "kept",
				Citation: citation{
					KBID:       1,
					DocumentID: 101,
				},
				Source: source{
					Route:            "dense",
					RetrieverVersion: "phase1-dense-v1",
				},
			},
			{
				Content: "filtered",
				Citation: citation{
					KBID:       999,
					DocumentID: 202,
				},
				Source: source{
					Route:            "dense",
					RetrieverVersion: "phase1-dense-v1",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	store := &fakeSemanticCacheStore{
		lookup: &semanticcache.LookupResult{
			Scope: semanticcache.Scope{
				TenantID:        9,
				KBIDs:           []uint64{1},
				StrategyVersion: "phase1",
				QueryType:       "general",
			},
			Candidates: []*semanticcache.Entry{
				{
					EntryID:          "entry-hit",
					TenantID:         9,
					KBIDs:            []uint64{1},
					StrategyVersion:  "phase1",
					QueryType:        "general",
					Query:            "what is cache",
					QueryEmbedding:   []float32{1, 0},
					ResponsePayload:  payload,
					ResultPayload:    semanticcache.ResultPayloadTag,
					TopK:             5,
					RetrieverVersion: "phase1-dense-v1",
				},
			},
			CandidateCount: 1,
		},
	}
	embedder := &fakeSemanticCacheEmbedder{vectors: [][]float64{{1, 0}}}

	hit, trace, err := trySemanticCacheHit(context.Background(), semanticCacheLookupInput{
		RequestID:          "req-new",
		TenantID:           9,
		Query:              "what is cache",
		TopK:               5,
		KBIDs:              []uint64{1},
		QueryType:          "general",
		StrategyVersion:    "phase1",
		ExperimentDecision: experiment.Decision{Group: experiment.GroupBaseline},
	}, store, embedder)
	if err != nil {
		t.Fatalf("trySemanticCacheHit err: %v", err)
	}
	if trace.Reason != "hit" {
		t.Fatalf("reason = %q, want hit", trace.Reason)
	}
	if hit == nil {
		t.Fatal("expected semantic cache hit")
	}
	if !trace.Hit || trace.EntryID != "entry-hit" {
		t.Fatalf("trace = %+v, want hit on entry-hit", trace)
	}
	if hit.Response.RequestID != "req-new" {
		t.Fatalf("request_id = %q, want req-new", hit.Response.RequestID)
	}
	if len(hit.Response.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(hit.Response.Items))
	}
	if hit.Response.Items[0].Citation.KBID != 1 {
		t.Fatalf("filtered item kb_id = %d, want 1", hit.Response.Items[0].Citation.KBID)
	}
	if store.touchN != 1 {
		t.Fatalf("touch count = %d, want 1", store.touchN)
	}
}

func TestResolveSemanticCacheStrategyVersion(t *testing.T) {
	version := resolveSemanticCacheStrategyVersion(
		experiment.Decision{
			Group:            experiment.GroupCandidate,
			CandidateVersion: "cand-v2",
			BaselineVersion:  "base-v1",
		},
		release.Decision{Strategy: "phase2"},
	)
	if version != "cand-v2" {
		t.Fatalf("version = %q, want cand-v2", version)
	}
}
