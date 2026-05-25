package retrieval

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestBuildSparseInvertedIndexSearchRanksByBM25(t *testing.T) {
	index := BuildSparseInvertedIndex([]*schema.Document{
		{ID: "doc-1", Content: "go goroutine scheduler channel"},
		{ID: "doc-2", Content: "go runtime scheduler"},
		{ID: "doc-3", Content: "database index btree"},
	}, nil)

	hits := index.Search([]string{"goroutine", "scheduler"}, 3)
	if len(hits) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(hits))
	}
	if hits[0].DocID != "doc-1" {
		t.Fatalf("expected doc-1 ranked first, got %s", hits[0].DocID)
	}
	if hits[0].Score <= hits[1].Score {
		t.Fatalf("expected first hit score > second hit score, got %f <= %f", hits[0].Score, hits[1].Score)
	}
}

func TestBuildSparseInvertedIndexAssignsPseudoDocID(t *testing.T) {
	doc := &schema.Document{
		Content: "global kb retrieval fallback",
		MetaData: map[string]interface{}{
			"document_id": 42,
			"chunk_id":    7,
		},
	}

	index := BuildSparseInvertedIndex([]*schema.Document{doc}, nil)
	hits := index.Search([]string{"fallback"}, 1)
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
	if hits[0].DocID != "42:7" {
		t.Fatalf("expected pseudo doc id 42:7, got %s", hits[0].DocID)
	}
	if doc.ID != "42:7" {
		t.Fatalf("expected document ID to be populated, got %s", doc.ID)
	}
}

func TestBuildSparseInvertedIndexSearchHandlesEmptyInput(t *testing.T) {
	index := BuildSparseInvertedIndex(nil, nil)
	hits := index.Search([]string{"hybrid"}, 5)
	if len(hits) != 0 {
		t.Fatalf("expected no hits from empty index, got %d", len(hits))
	}
}
