package retrieval

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestCandidateBM25RankerRanksCandidates(t *testing.T) {
	candidates := testSparseCandidates()
	ranker := &CandidateBM25Ranker{}
	hits, stats, err := ranker.Rank(nil, "golang map", []string{"golang", "map"}, candidates, 2)
	if err != nil {
		t.Fatalf("Rank returned error: %v", err)
	}
	if stats.RankerName != "candidate_bm25" {
		t.Fatalf("RankerName = %q, want candidate_bm25", stats.RankerName)
	}
	if stats.CandidateCountBefore != 3 {
		t.Fatalf("CandidateCountBefore = %d, want 3", stats.CandidateCountBefore)
	}
	if len(hits) == 0 {
		t.Fatal("expected hits")
	}
	if hits[0].Explain.BM25Score <= 0 {
		t.Fatalf("expected bm25 explain score > 0")
	}
	if len(hits[0].Explain.MatchedTerms) == 0 {
		t.Fatalf("expected matched term explain")
	}
}

func TestSparseRetrieverFallsBackWhenRankerFails(t *testing.T) {
	retriever := &SparseRetriever{
		collection: "knowledge",
		config:     SparseRetrieverConfig{},
		candidateProvider: &stubSparseCandidateProvider{
			candidates: testSparseCandidates(),
			stats: SparseCandidateProviderStats{
				ProviderName:        "stub_provider",
				DedupCandidateCount: 3,
			},
		},
		ranker: &stubSparseRanker{err: errors.New("boom")},
	}

	docs, stats, err := retriever.Search(context.Background(), &HybridSearchRequest{
		Query:         "golang map",
		SparseQuery:   "golang map",
		CandidateTopK: 2,
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(docs) != 0 {
		t.Fatalf("expected sparse fallback to return empty docs, got %d", len(docs))
	}
	if stats.FallbackReason != "ranker_error" {
		t.Fatalf("FallbackReason = %q, want ranker_error", stats.FallbackReason)
	}
}

func testSparseCandidates() []*schema.Document {
	return []*schema.Document{
		{
			ID:      "doc-1",
			Content: "golang map internals runtime",
			MetaData: map[string]interface{}{
				"document_id": "doc-1",
				"chunk_id":    "chunk-1",
			},
		},
		{
			ID:      "doc-2",
			Content: "java hashmap internals",
			MetaData: map[string]interface{}{
				"document_id": "doc-2",
				"chunk_id":    "chunk-1",
			},
		},
		{
			ID:      "doc-3",
			Content: "golang channel select",
			MetaData: map[string]interface{}{
				"document_id": "doc-3",
				"chunk_id":    "chunk-1",
			},
		},
	}
}

type stubSparseCandidateProvider struct {
	candidates []*schema.Document
	stats      SparseCandidateProviderStats
	err        error
}

func (s *stubSparseCandidateProvider) SearchCandidates(ctx context.Context, req *HybridSearchRequest, terms []string) ([]*schema.Document, SparseCandidateProviderStats, error) {
	return s.candidates, s.stats, s.err
}

type stubSparseRanker struct {
	hits  []SparseSearchHit
	stats SparseRankStats
	err   error
}

func (s *stubSparseRanker) Rank(ctx context.Context, query string, terms []string, candidates []*schema.Document, topK int) ([]SparseSearchHit, SparseRankStats, error) {
	return s.hits, s.stats, s.err
}
