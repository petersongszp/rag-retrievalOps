package retrieval

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/retriever/milvus"
	"github.com/cloudwego/eino/schema"
	milvusClient "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// RetrieverService wraps the Milvus retriever and direct SDK search helpers.
type RetrieverService struct {
	retriever *milvus.Retriever
	config    *milvus.RetrieverConfig
	client    milvusClient.Client
}

func NewRetrieverService(ctx context.Context, config *milvus.RetrieverConfig) (*RetrieverService, error) {
	if config == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if config.Client == nil {
		return nil, fmt.Errorf("milvus client is nil")
	}
	if config.Collection == "" {
		return nil, fmt.Errorf("collection name is empty")
	}
	if config.Embedding == nil {
		return nil, fmt.Errorf("embedding is nil")
	}
	if config.VectorField == "" {
		config.VectorField = "vector"
	}
	if len(config.OutputFields) == 0 {
		config.OutputFields = []string{"id", "content", "metadata"}
	}
	if config.TopK <= 0 {
		config.TopK = 10
	}

	retrieverConfig := &milvus.RetrieverConfig{
		Client:            config.Client,
		Collection:        config.Collection,
		Partition:         config.Partition,
		VectorField:       config.VectorField,
		OutputFields:      config.OutputFields,
		DocumentConverter: config.DocumentConverter,
		VectorConverter:   FloatVectorConverter,
		MetricType:        config.MetricType,
		TopK:              config.TopK,
		Embedding:         config.Embedding,
	}
	if config.Sp == nil {
		searchParam, err := entity.NewIndexAUTOINDEXSearchParam(1)
		if err != nil {
			return nil, fmt.Errorf("failed to create search param: %w", err)
		}
		retrieverConfig.Sp = searchParam
	} else {
		retrieverConfig.Sp = config.Sp
	}
	if config.ScoreThreshold > 0 {
		retrieverConfig.ScoreThreshold = config.ScoreThreshold
	}

	retriever, err := milvus.NewRetriever(ctx, retrieverConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create milvus retriever: %w", err)
	}

	return &RetrieverService{
		retriever: retriever,
		config:    config,
		client:    config.Client,
	}, nil
}

func (s *RetrieverService) Retrieve(ctx context.Context, query string) ([]*schema.Document, error) {
	result, err := s.RetrieveWithOptionsAndMetrics(ctx, query, nil)
	if err != nil {
		return nil, err
	}
	return result.Documents, nil
}

func (s *RetrieverService) RetrieveWithOptions(ctx context.Context, query string, opts *RetrieveOptions) ([]*schema.Document, error) {
	result, err := s.RetrieveWithOptionsAndMetrics(ctx, query, opts)
	if err != nil {
		return nil, err
	}
	return result.Documents, nil
}

func (s *RetrieverService) RetrieveWithOptionsAndMetrics(ctx context.Context, query string, opts *RetrieveOptions) (*SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("query is empty")
	}

	if opts == nil {
		opts = &RetrieveOptions{
			TopK:       s.config.TopK,
			Collection: s.config.Collection,
		}
	}
	if opts.TopK <= 0 {
		opts.TopK = s.config.TopK
	}
	if opts.Collection == "" {
		opts.Collection = s.config.Collection
	}

	expr := BuildFilterExpr(opts)
	return SearchWithExprAndMetrics(ctx, s.client, s.config, query, expr, opts)
}

func (s *RetrieverService) RetrieveWithDatabaseAndCollection(
	ctx context.Context,
	query string,
	database string,
	collection string,
	opts *RetrieveOptions,
) ([]*schema.Document, error) {
	if query == "" {
		return nil, fmt.Errorf("query is empty")
	}
	if collection == "" {
		return nil, fmt.Errorf("collection name is empty")
	}

	retrieveOpts := &RetrieveOptions{
		Collection: collection,
	}
	if opts != nil {
		retrieveOpts.Language = opts.Language
		retrieveOpts.Category = opts.Category
		retrieveOpts.Expr = opts.Expr
		retrieveOpts.TenantID = opts.TenantID
		retrieveOpts.AllowedKBIDs = append([]uint64(nil), opts.AllowedKBIDs...)
		retrieveOpts.TopK = opts.TopK
		retrieveOpts.KBScope = opts.KBScope
		retrieveOpts.ActiveGlobalKBID = opts.ActiveGlobalKBID
		if opts.Collection != "" {
			retrieveOpts.Collection = opts.Collection
		}
	}

	result, err := SearchWithExprAndMetrics(ctx, s.client, s.config, query, BuildFilterExpr(retrieveOpts), retrieveOpts)
	if err != nil {
		return nil, err
	}
	return result.Documents, nil
}

func (s *RetrieverService) GetConfig() *milvus.RetrieverConfig {
	return s.config
}

func (s *RetrieverService) RetrieveKnowledge(ctx context.Context, query string, activeGlobalKBID uint64, topK int, collection string) ([]*schema.Document, error) {
	result, err := s.RetrieveKnowledgeWithMetrics(ctx, query, activeGlobalKBID, topK, collection)
	if err != nil {
		return nil, err
	}
	return result.Documents, nil
}

func (s *RetrieverService) RetrieveKnowledgeWithMetrics(ctx context.Context, query string, activeGlobalKBID uint64, topK int, collection string) (*SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("query is empty")
	}
	if activeGlobalKBID == 0 {
		return nil, fmt.Errorf("active_global_kb_id is required for knowledge retrieval")
	}
	if topK <= 0 {
		topK = 5
	}
	if topK > 20 {
		topK = 20
	}
	if collection == "" {
		collection = s.config.Collection
	}
	opts := &RetrieveOptions{
		KBScope:          "global",
		ActiveGlobalKBID: activeGlobalKBID,
		TopK:             topK,
		Collection:       collection,
	}
	return SearchWithExprAndMetrics(ctx, s.client, s.config, query, BuildFilterExpr(opts), opts)
}
