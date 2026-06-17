package kb

import (
	"context"
	"testing"

	"interview-agents/internal/config"
	"interview-agents/internal/milvus/retrieval"
	"interview-agents/internal/model"
	"interview-agents/internal/rag/experiment"
)

func TestShouldSemanticCacheBackfillRejectsUnsafeResults(t *testing.T) {
	original := config.Global
	t.Cleanup(func() {
		config.Global = original
	})

	config.Global.RAG.FeatureFlags.EnableSemanticCache = true

	base := semanticCacheBackfillInput{
		RequestID:          "req-1",
		TenantID:           9,
		Query:              "what is semantic cache",
		TopK:               5,
		KBIDs:              []uint64{1},
		QueryType:          "general",
		StrategyVersion:    "phase1",
		ExperimentDecision: experiment.Decision{Group: experiment.GroupBaseline},
		ResultStatus:       model.RetrieveResultStatusSuccess,
		Response: retrieveResponse{
			Items: []retrieveItem{
				{Content: "doc"},
			},
		},
	}

	refused := base
	refused.Response.Refusal = &refusalPayload{Reason: "No-Retrieval-Hit"}
	if reason := shouldSemanticCacheBackfill(refused); reason != "evidence_refusal" {
		t.Fatalf("refusal reason = %q, want evidence_refusal", reason)
	}

	empty := base
	empty.Response.Items = nil
	if reason := shouldSemanticCacheBackfill(empty); reason != "empty_result" {
		t.Fatalf("empty reason = %q, want empty_result", reason)
	}

	filtered := base
	filtered.ResultStatus = model.RetrieveResultStatusFilteredOut
	filtered.Response.EvidenceGateResult = retrieval.EvidenceGateResultRefused
	if reason := shouldSemanticCacheBackfill(filtered); reason != "evidence_refusal" {
		t.Fatalf("filtered reason = %q, want evidence_refusal", reason)
	}
}

func TestBackfillSemanticCacheWritesEntry(t *testing.T) {
	original := config.Global
	t.Cleanup(func() {
		config.Global = original
	})

	config.Global.RAG.FeatureFlags.EnableSemanticCache = true
	config.Global.RAG.SemanticCache.TTLSeconds = 60
	config.Global.RAG.SemanticCache.MaxEntriesPerScope = 10

	store := &fakeSemanticCacheStore{}
	embedder := &fakeSemanticCacheEmbedder{vectors: [][]float64{{0.2, 0.8}}}
	input := semanticCacheBackfillInput{
		RequestID:          "req-2",
		TenantID:           9,
		Query:              "what is semantic cache",
		TopK:               5,
		KBIDs:              []uint64{3, 1},
		QueryType:          "general",
		StrategyVersion:    "phase1",
		ExperimentDecision: experiment.Decision{Group: experiment.GroupBaseline},
		RetrieverVersion:   "phase1-dense-v1",
		ResultStatus:       model.RetrieveResultStatusSuccess,
		Response: retrieveResponse{
			RequestID: "req-2",
			Items: []retrieveItem{
				{
					Content: "semantic cache intro",
					Citation: citation{
						KBID:       1,
						DocumentID: 101,
					},
				},
			},
		},
	}

	if err := backfillSemanticCache(context.Background(), input, store, embedder); err != nil {
		t.Fatalf("backfillSemanticCache err: %v", err)
	}
	if store.putN != 1 {
		t.Fatalf("put count = %d, want 1", store.putN)
	}
	if store.putEntry == nil {
		t.Fatal("expected semantic cache entry to be written")
	}
	if store.putEntry.Query != input.Query {
		t.Fatalf("entry query = %q, want %q", store.putEntry.Query, input.Query)
	}
	if store.putEntry.TopK != input.TopK {
		t.Fatalf("entry topk = %d, want %d", store.putEntry.TopK, input.TopK)
	}
	if len(store.putScope.KBIDs) != 2 || store.putScope.KBIDs[0] != 3 || store.putScope.KBIDs[1] != 1 {
		t.Fatalf("scope kb_ids = %v, want [3 1]", store.putScope.KBIDs)
	}
}

func TestInvalidateSemanticCacheByKnowledgeBase(t *testing.T) {
	store := &fakeSemanticCacheStore{}
	if err := invalidateSemanticCacheByKnowledgeBase(context.Background(), 12, 7, store); err != nil {
		t.Fatalf("invalidateSemanticCacheByKnowledgeBase err: %v", err)
	}
	if store.delN != 1 {
		t.Fatalf("delete count = %d, want 1", store.delN)
	}
	if store.delTenant != 12 || store.delKB != 7 {
		t.Fatalf("delete target = tenant %d kb %d, want tenant 12 kb 7", store.delTenant, store.delKB)
	}
}
