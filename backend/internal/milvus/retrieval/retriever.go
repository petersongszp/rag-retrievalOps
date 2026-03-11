package retrieval

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/retriever/milvus"
	"github.com/cloudwego/eino/schema"
	milvusClient "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

//参考文档 https://www.cloudwego.io/zh/docs/eino/ecosystem_integration/retriever/retriever_milvus/

// RetrieverService 封装 Milvus 检索器服务
type RetrieverService struct {
	retriever *milvus.Retriever
	config    *milvus.RetrieverConfig
	client    milvusClient.Client
}

// NewRetrieverService 创建新的检索器服务
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
	// 设置默认值
	if config.VectorField == "" {
		config.VectorField = "vector"
	}
	if len(config.OutputFields) == 0 {
		config.OutputFields = []string{"id", "content", "metadata"}
	}
	if config.TopK <= 0 {
		config.TopK = 10
	}
	// 构建检索器配置
	// 注意：需要显式设置 Sp（搜索参数）和 VectorConverter
	// 1. eino-service 的 defaultSearchParam 函数错误地将维度值作为 radius 参数传递
	//    参考：https://github.com/cloudwego/eino-ext/blob/main/components/retriever/milvus/utils.go#L40
	// 2. eino-service 的 defaultVectorConverter 返回 BinaryVector，但我们使用 FloatVector
	//    参考：https://github.com/cloudwego/eino-ext/blob/main/components/retriever/milvus/utils.go#L97
	retrieverConfig := &milvus.RetrieverConfig{
		Client:            config.Client,
		Collection:        config.Collection,
		Partition:         config.Partition,
		VectorField:       config.VectorField,
		OutputFields:      config.OutputFields,
		DocumentConverter: config.DocumentConverter,
		// 设置 VectorConverter 为 FloatVector 转换器（与 indexer 保持一致）
		VectorConverter: FloatVectorConverter,
		MetricType:      config.MetricType,
		TopK:            config.TopK,
		Embedding:       config.Embedding,
	}
	if config.Sp == nil {
		// 创建一个简单的 AUTOINDEX 搜索参数，不使用 range search
		searchParam, err := entity.NewIndexAUTOINDEXSearchParam(1)
		if err != nil {
			return nil, fmt.Errorf("failed to create search param: %w", err)
		}
		retrieverConfig.Sp = searchParam
	} else {
		retrieverConfig.Sp = config.Sp
	}
	// 只有当 ScoreThreshold > 0 时才设置
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

// Retrieve 检索相关文档（不带过滤条件）
func (s *RetrieverService) Retrieve(ctx context.Context, query string) ([]*schema.Document, error) {
	return s.RetrieveWithOptions(ctx, query, nil)
}

// RetrieveWithOptions 检索相关文档（支持过滤选项）
func (s *RetrieverService) RetrieveWithOptions(ctx context.Context, query string, opts *RetrieveOptions) ([]*schema.Document, error) {
	if query == "" {
		return nil, fmt.Errorf("query is empty")
	}

	// 因为默认的 retriever 绑定到特定的集合
	if opts != nil && (opts.Collection != "") {
		expr := BuildFilterExpr(opts)
		return SearchWithExpr(ctx, s.client, s.config, query, expr, opts)
	}

	// 如果没有过滤选项，使用默认检索
	if opts == nil {
		documents, err := s.retriever.Retrieve(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve documents: %w", err)
		}
		return documents, nil
	}

	// 构建过滤表达式
	expr := BuildFilterExpr(opts)
	if expr != "" {
		// 如果有过滤表达式，使用 Milvus SDK 直接搜索
		return SearchWithExpr(ctx, s.client, s.config, query, expr, opts)
	}

	// 没有过滤条件，使用默认检索
	documents, err := s.retriever.Retrieve(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve documents: %w", err)
	}
	return documents, nil
}

// RetrieveWithDatabaseAndCollection 检索相关文档（指定数据库和集合）
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

	// 创建检索选项，包含集合信息（忽略数据库）
	retrieveOpts := &RetrieveOptions{
		Collection: collection,
	}

	// 如果提供了其他选项，合并它们
	if opts != nil {
		retrieveOpts.Language = opts.Language
		retrieveOpts.Category = opts.Category
		retrieveOpts.Expr = opts.Expr
		retrieveOpts.TopK = opts.TopK
		// 如果 opts 中也指定了集合，使用 opts 中的值（优先级更高）
		if opts.Collection != "" {
			retrieveOpts.Collection = opts.Collection
		}
	}

	// 构建过滤表达式
	expr := BuildFilterExpr(retrieveOpts)
	return SearchWithExpr(ctx, s.client, s.config, query, expr, retrieveOpts)
}

// GetConfig 获取配置信息
func (s *RetrieverService) GetConfig() *milvus.RetrieverConfig {
	return s.config
}
