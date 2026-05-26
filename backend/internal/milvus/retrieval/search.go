package retrieval

import (
	"context"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	milvusRetriever "github.com/cloudwego/eino-ext/components/retriever/milvus"
	"github.com/cloudwego/eino/schema"
	milvusClient "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

const DenseRetrieverVersion = "phase1-dense-v1"

// SearchMetrics carries retrieve-stage observability fields used across L4-L8.
type SearchMetrics struct {
	EmbeddingMs        int64
	SearchMs           int64
	PostprocessMs      int64
	HitCount           int
	TruncatedCount     int
	CandidateTopK      int
	FinalTopK          int
	TokenBudget        int
	TruncateReason     string
	Strategy           string
	ReleaseStage       string
	ReleaseReason      string
	RetrieverVersion   string
	RewriteApplied     bool
	EmptyReason        string
	RerankMs           int64
	RerankModel        string
	RerankVersion      string
	RerankFallback     bool
	RerankReason       string
	DenseHits          int
	SparseHits         int
	DenseContribution  int
	SparseContribution int
	TopKPolicyVersion  string
	ScoreDistribution  string
	RerankGap          float64
	EvidenceDensity    float64
	TopKDecisionReason string
	TokenBudgetRemain  int
	ContextTokens      int
}

// SearchResult bundles documents with observable metrics.
type SearchResult struct {
	Documents []*schema.Document
	Metrics   SearchMetrics
}

func SearchWithExpr(
	ctx context.Context,
	client milvusClient.Client,
	config *milvusRetriever.RetrieverConfig,
	query string,
	expr string,
	opts *RetrieveOptions,
) ([]*schema.Document, error) {
	result, err := SearchWithExprAndMetrics(ctx, client, config, query, expr, opts)
	if err != nil {
		return nil, err
	}
	return result.Documents, nil
}

func SearchWithExprAndMetrics(
	ctx context.Context,
	client milvusClient.Client,
	config *milvusRetriever.RetrieverConfig,
	query string,
	expr string,
	opts *RetrieveOptions,
) (*SearchResult, error) {
	embedder := config.Embedding
	if embedder == nil {
		return nil, fmt.Errorf("embedding is nil")
	}

	embedStart := time.Now()
	vectors, err := embedder.EmbedStrings(ctx, []string{query})
	embeddingMs := time.Since(embedStart).Milliseconds()
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("empty embedding result")
	}

	queryVector := make([]float32, len(vectors[0]))
	for i, value := range vectors[0] {
		queryVector[i] = float32(value)
	}

	topK := 0
	if opts != nil && opts.TopK > 0 {
		topK = opts.TopK
	}
	if topK <= 0 {
		topK = config.TopK
		if topK <= 0 {
			topK = 10
		}
	}

	collectionName := config.Collection
	if opts != nil && opts.Collection != "" {
		collectionName = opts.Collection
	}

	searchParam := config.Sp
	if searchParam == nil {
		searchParam, err = entity.NewIndexAUTOINDEXSearchParam(1)
		if err != nil {
			return nil, fmt.Errorf("failed to create search param: %w", err)
		}
	}

	searchStart := time.Now()
	searchResults, err := client.Search(
		ctx,
		collectionName,
		[]string{},
		expr,
		[]string{"id", "content", "metadata"},
		[]entity.Vector{entity.FloatVector(queryVector)},
		config.VectorField,
		config.MetricType,
		topK,
		searchParam,
	)
	searchMs := time.Since(searchStart).Milliseconds()
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	postprocessStart := time.Now()
	documents := make([]*schema.Document, 0)
	rawHitCount := 0
	if len(searchResults) > 0 {
		result := searchResults[0]
		contentField := result.Fields.GetColumn("content")
		metadataField := result.Fields.GetColumn("metadata")
		idField := result.IDs

		rawHitCount = idField.Len()
		for i := 0; i < rawHitCount; i++ {
			idValue, idErr := idField.GetAsString(i)
			if idErr != nil {
				continue
			}

			doc := &schema.Document{
				ID:       idValue,
				MetaData: make(map[string]interface{}),
			}

			if contentField != nil {
				if content, contentErr := contentField.GetAsString(i); contentErr == nil {
					doc.Content = content
				}
			}
			if metadataField != nil {
				if jsonStr, metadataErr := metadataField.GetAsString(i); metadataErr == nil && jsonStr != "" {
					var metadata map[string]interface{}
					if unmarshalErr := sonic.Unmarshal([]byte(jsonStr), &metadata); unmarshalErr == nil {
						doc.MetaData = metadata
					}
				}
			}
			if i < len(result.Scores) {
				doc.MetaData["score"] = result.Scores[i]
			}
			doc.MetaData["retriever_version"] = DenseRetrieverVersion
			source := ensureSourceMetadata(doc)
			source["route"] = routeDense
			source["retriever_version"] = DenseRetrieverVersion
			if collectionName != "" {
				source["collection"] = collectionName
			}
			doc.MetaData["source"] = source
			annotateParentChildSource(doc)

			documents = append(documents, doc)
		}
	}
	postprocessMs := time.Since(postprocessStart).Milliseconds()

	return &SearchResult{
		Documents: documents,
		Metrics: SearchMetrics{
			EmbeddingMs:       embeddingMs,
			SearchMs:          searchMs,
			PostprocessMs:     postprocessMs,
			HitCount:          rawHitCount,
			TruncatedCount:    rawHitCount - len(documents),
			CandidateTopK:     topK,
			FinalTopK:         len(documents),
			Strategy:          "phase1",
			RetrieverVersion:  DenseRetrieverVersion,
			DenseHits:         len(documents),
			DenseContribution: len(documents),
		},
	}, nil
}
