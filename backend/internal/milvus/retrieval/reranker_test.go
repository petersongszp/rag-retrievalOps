package retrieval

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestJaccardRerankerAnnotatesSourceContract(t *testing.T) {
	reranker := NewJaccardReranker(&JaccardRerankerConfig{
		OriginalScoreWeight: 0.7,
		TopK:                2,
		ModelName:           DefaultRerankModelJaccardV1,
		Version:             DefaultRerankVersion,
	})

	docs := []*schema.Document{
		{
			ID:      "doc-1",
			Content: "go map runtime implementation details",
			MetaData: map[string]interface{}{
				"score":             0.8,
				"retriever_version": hybridRetrieverVersion,
				"source": map[string]interface{}{
					"route":      routeDense,
					"collection": "knowledge",
				},
			},
		},
	}

	result, err := reranker.Rerank(context.Background(), "go map implementation", docs)
	if err != nil {
		t.Fatalf("rerank returned error: %v", err)
	}
	if len(result.Documents) != 1 {
		t.Fatalf("expected 1 reranked document, got %d", len(result.Documents))
	}

	doc := result.Documents[0]
	if _, ok := doc.MetaData["rerank_score"]; !ok {
		t.Fatalf("expected rerank_score metadata")
	}
	if doc.MetaData["rerank_version"] != DefaultRerankVersion {
		t.Fatalf("expected rerank_version=%q, got %v", DefaultRerankVersion, doc.MetaData["rerank_version"])
	}

	source, ok := doc.MetaData["source"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected source map, got %T", doc.MetaData["source"])
	}
	if source["collection"] != "knowledge" {
		t.Fatalf("expected source.collection to be preserved, got %v", source["collection"])
	}
	if source["retriever_version"] != hybridRetrieverVersion {
		t.Fatalf("expected source.retriever_version=%q, got %v", hybridRetrieverVersion, source["retriever_version"])
	}
	if source["rerank_version"] != DefaultRerankVersion {
		t.Fatalf("expected source.rerank_version=%q, got %v", DefaultRerankVersion, source["rerank_version"])
	}
	if _, ok := source["rerank_score"]; !ok {
		t.Fatalf("expected source.rerank_score")
	}
	if source["pre_rerank_rank"] != 1 {
		t.Fatalf("expected source.pre_rerank_rank=1, got %v", source["pre_rerank_rank"])
	}
	if source["post_rerank_rank"] != 1 {
		t.Fatalf("expected source.post_rerank_rank=1, got %v", source["post_rerank_rank"])
	}
	if _, ok := source["score_delta"]; !ok {
		t.Fatalf("expected source.score_delta")
	}
}

func TestJaccardRerankerDefaultsOriginalScoreWeightForPartialConfig(t *testing.T) {
	reranker := NewJaccardReranker(&JaccardRerankerConfig{
		TopK:      2,
		ModelName: DefaultRerankModelJaccardV1,
		Version:   DefaultRerankVersion,
	})

	docs := []*schema.Document{
		{
			ID:      "title-boosted",
			Content: "### 1.2 超过规格限制后行为",
			MetaData: map[string]interface{}{
				"score": 1.0,
			},
		},
		{
			ID:      "lexical-overlap",
			Content: "超过 tps 规格 上限 后 会 怎样",
			MetaData: map[string]interface{}{
				"score": 0.2,
			},
		},
	}

	result, err := reranker.Rerank(context.Background(), "超过 tps 规格上限后会怎样", docs)
	if err != nil {
		t.Fatalf("rerank returned error: %v", err)
	}
	if len(result.Documents) != 2 {
		t.Fatalf("expected 2 reranked documents, got %d", len(result.Documents))
	}
	if result.Documents[0].ID != "title-boosted" {
		t.Fatalf("expected original score to keep boosted document first, got %s", result.Documents[0].ID)
	}
}

func TestConfigurableRerankerFallsBackOnPrimaryError(t *testing.T) {
	primary := &stubReranker{err: errors.New("boom")}
	fallback := &stubReranker{
		result: &RerankResult{
			Documents: []*schema.Document{
				{ID: "fallback-doc", Content: "fallback"},
			},
			Model:   DefaultRerankModelJaccardV1,
			Version: DefaultRerankVersion,
		},
	}

	reranker := NewConfigurableReranker("test-model", 0, primary, fallback)
	result, err := reranker.Rerank(context.Background(), "query", []*schema.Document{
		{ID: "doc-1", Content: "query"},
	})
	if err != nil {
		t.Fatalf("expected fallback success, got error: %v", err)
	}
	if !result.Fallback {
		t.Fatalf("expected fallback to be marked")
	}
	if result.Reason != "error" {
		t.Fatalf("expected fallback reason error, got %q", result.Reason)
	}
	if len(result.Documents) != 1 || result.Documents[0].ID != "fallback-doc" {
		t.Fatalf("unexpected fallback docs: %+v", result.Documents)
	}
}

type stubReranker struct {
	result *RerankResult
	err    error
}

func (s *stubReranker) Rerank(ctx context.Context, query string, docs []*schema.Document) (*RerankResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}
