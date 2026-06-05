package retrieval

import (
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
