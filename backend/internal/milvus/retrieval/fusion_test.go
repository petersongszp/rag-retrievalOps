package retrieval

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestFuseRouteCandidatesAndDedupe(t *testing.T) {
	denseDocs := []*schema.Document{
		{
			ID:      "dense-1",
			Content: "golang map runtime internals",
			MetaData: map[string]interface{}{
				"document_id": "doc-1",
				"chunk_id":    "chunk-1",
				"dense_score": 0.91,
				"score":       0.91,
			},
		},
		{
			ID:      "dense-2",
			Content: "java hashmap internals",
			MetaData: map[string]interface{}{
				"document_id": "doc-2",
				"chunk_id":    "chunk-1",
				"dense_score": 0.42,
				"score":       0.42,
			},
		},
	}
	sparseDocs := []*schema.Document{
		{
			ID:      "sparse-1",
			Content: "golang map runtime internals",
			MetaData: map[string]interface{}{
				"document_id":  "doc-1",
				"chunk_id":     "chunk-1",
				"sparse_score": 11.2,
				"score":        11.2,
			},
		},
		{
			ID:      "sparse-3",
			Content: "go channel concurrency",
			MetaData: map[string]interface{}{
				"document_id":  "doc-3",
				"chunk_id":     "chunk-1",
				"sparse_score": 3.6,
				"score":        3.6,
			},
		},
	}

	fused := FuseRouteCandidates(denseDocs, sparseDocs, FusionConfig{
		DenseWeight:  0.7,
		SparseWeight: 0.3,
	})
	if len(fused) != 4 {
		t.Fatalf("expected 4 fused candidates, got %d", len(fused))
	}

	merged := DeduplicateFusedDocuments(fused)
	if len(merged) != 3 {
		t.Fatalf("expected 3 deduped results, got %d", len(merged))
	}

	first := merged[0]
	if got := readMetadataString(first, "route"); got != routeDense {
		t.Fatalf("expected primary route %q, got %q", routeDense, got)
	}

	source, ok := first.MetaData["source"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected source metadata map, got %T", first.MetaData["source"])
	}
	if source["route"] != routeDense {
		t.Fatalf("expected source.route=%q, got %v", routeDense, source["route"])
	}
	if source["retriever_version"] != hybridRetrieverVersion {
		t.Fatalf("expected source.retriever_version=%q, got %v", hybridRetrieverVersion, source["retriever_version"])
	}

	routeContrib, ok := source["route_contrib"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected source.route_contrib map, got %T", source["route_contrib"])
	}
	if _, exists := routeContrib[routeDense]; !exists {
		t.Fatalf("expected dense contribution to exist")
	}
	if _, exists := routeContrib[routeSparse]; !exists {
		t.Fatalf("expected sparse contribution to exist")
	}
}

func TestBuildDedupeKeyPrefersDocumentAndChunkID(t *testing.T) {
	doc := &schema.Document{
		ID:      "fallback-id",
		Content: "same chunk",
		MetaData: map[string]interface{}{
			"document_id": "doc-9",
			"chunk_id":    "chunk-3",
		},
	}

	if got := buildDedupeKey(doc); got != "doc-9:chunk-3" {
		t.Fatalf("unexpected dedupe key: %s", got)
	}
}

func TestSummarizeFinalRouteStats(t *testing.T) {
	docs := []*schema.Document{
		{
			ID: "doc-1",
			MetaData: map[string]interface{}{
				"route": "dense",
				"route_contrib": map[string]interface{}{
					"dense":  0.7,
					"sparse": 0.4,
				},
			},
		},
		{
			ID: "doc-2",
			MetaData: map[string]interface{}{
				"route": "sparse",
				"route_contrib": map[string]interface{}{
					"sparse": 0.6,
				},
			},
		},
	}

	stats := summarizeFinalRouteStats(docs)
	if stats.DenseParticipation != 1 {
		t.Fatalf("DenseParticipation = %d, want 1", stats.DenseParticipation)
	}
	if stats.SparseParticipation != 2 {
		t.Fatalf("SparseParticipation = %d, want 2", stats.SparseParticipation)
	}
	if stats.PrimaryDenseCount != 1 || stats.PrimarySparseCount != 1 {
		t.Fatalf("unexpected primary counts: %+v", stats)
	}
	if stats.DualRouteFinalCount != 1 {
		t.Fatalf("DualRouteFinalCount = %d, want 1", stats.DualRouteFinalCount)
	}
}

func TestFuseRouteCandidatesRRFAnnotatesRanksAndContrib(t *testing.T) {
	denseDocs := []*schema.Document{
		{
			ID: "dense-1",
			MetaData: map[string]interface{}{
				"document_id": "doc-1",
				"chunk_id":    "chunk-1",
				"dense_score": 0.9,
			},
		},
	}
	sparseDocs := []*schema.Document{
		{
			ID: "sparse-1",
			MetaData: map[string]interface{}{
				"document_id":  "doc-1",
				"chunk_id":     "chunk-1",
				"sparse_score": 8.0,
			},
		},
	}

	fused := FuseRouteCandidates(denseDocs, sparseDocs, FusionConfig{
		FusionStrategy:  "rrf_v1",
		RRFK:            60,
		RRFDenseWeight:  0.7,
		RRFSparseWeight: 0.3,
		DenseWeight:     0.7,
		SparseWeight:    0.3,
	})
	if len(fused) != 2 {
		t.Fatalf("expected 2 fused docs, got %d", len(fused))
	}
	if fused[0].FusionStrategy != "rrf_v1" {
		t.Fatalf("FusionStrategy = %q, want rrf_v1", fused[0].FusionStrategy)
	}

	merged := DeduplicateFusedDocuments(fused)
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged doc, got %d", len(merged))
	}
	doc := merged[0]
	if got := readMetadataString(doc, "primary_route"); got == "" {
		t.Fatalf("expected primary_route metadata")
	}
	if got := readMetadataString(doc, "fusion_strategy"); got != "rrf_v1" {
		t.Fatalf("fusion_strategy = %q, want rrf_v1", got)
	}
	if _, ok := doc.MetaData["route_rank"].(map[string]interface{}); !ok {
		t.Fatalf("expected route_rank map, got %T", doc.MetaData["route_rank"])
	}
	if _, ok := doc.MetaData["route_rrf_contrib"].(map[string]interface{}); !ok {
		t.Fatalf("expected route_rrf_contrib map, got %T", doc.MetaData["route_rrf_contrib"])
	}
}
