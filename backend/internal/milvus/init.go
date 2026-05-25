package milvus

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
	"github.com/cloudwego/eino-ext/components/embedding/ark"
	milvusIndexer "github.com/cloudwego/eino-ext/components/indexer/milvus"
	milvusRetriever "github.com/cloudwego/eino-ext/components/retriever/milvus"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"

	"interview-agents/internal/config"
	"interview-agents/internal/milvus/retrieval"
	"interview-agents/internal/milvus/splitter"
	"interview-agents/internal/milvus/storage"
)

// MilvusManager Milvus服务管理器，负责初始化和管理所有Milvus相关服务
type MilvusManager struct {
	// Milvus客户端
	Client client.Client

	// 各个服务实例
	EmbeddingService *storage.EmbeddingService
	SplitterService  *splitter.DocumentSplitterService
	IndexerService   *storage.IndexerService
	RetrieverService *retrieval.RetrieverService

	// 混合检索服务 (Vector + Reranker)
	HybridRetriever *retrieval.HybridRetriever

	// 配置信息
	Config *config.Config
}

// 全局单例管理器
var (
	globalManager *MilvusManager
)

// InitMilvusManager 初始化Milvus管理器（从配置文件）
func InitMilvusManager(ctx context.Context, cfg *config.Config) (*MilvusManager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	// 展开环境变量
	cfg.ExpandEnv()
	log.Println("Initializing Milvus Manager...")
	manager := &MilvusManager{
		Config: cfg,
	}
	// 1. 初始化 Milvus 客户端
	log.Println("Connecting to Milvus...")
	// 设置连接超时 Context
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	milvusClient, err := client.NewClient(connectCtx, client.Config{
		Address:  cfg.Milvus.Address,
		Username: cfg.Milvus.Username,
		Password: cfg.Milvus.Password,
		DBName:   cfg.Milvus.DatabaseName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Milvus: %w", err)
	}
	manager.Client = milvusClient
	log.Printf("Milvus connected successfully: %s", cfg.Milvus.Address)

	// 2. 初始化 Embedding 服务
	log.Println("Initializing Embedding Service...")
	timeout := cfg.Embedding.Timeout
	retryTimes := cfg.Embedding.RetryTimes
	embeddingConfig := &ark.EmbeddingConfig{
		APIKey:     cfg.Embedding.APIKey,
		AccessKey:  cfg.Embedding.AccessKey,
		SecretKey:  cfg.Embedding.SecretKey,
		Model:      cfg.Embedding.Model,
		BaseURL:    cfg.Embedding.BaseURL,
		Region:     cfg.Embedding.Region,
		Timeout:    &timeout,
		RetryTimes: &retryTimes,
	}
	embeddingService, err := storage.NewArkEmbeddingService(ctx, embeddingConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize embedding service: %w", err)
	}
	manager.EmbeddingService = embeddingService
	log.Printf("Embedding Service initialized: Model=%s", cfg.Embedding.Model)

	// 3. 初始化文档分割器
	log.Println("Initializing Document Splitter...")
	splitterConfig := &recursive.Config{
		ChunkSize:   cfg.DocumentSplitter.ChunkSize,
		OverlapSize: cfg.DocumentSplitter.OverlapSize,
		Separators:  cfg.DocumentSplitter.Separators,
		KeepType:    recursive.KeepType(cfg.DocumentSplitter.KeepType),
	}
	splitterService, err := splitter.NewDocumentSplitterService(ctx, splitterConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize document splitter: %w", err)
	}
	manager.SplitterService = splitterService
	log.Printf("Document Splitter initialized: ChunkSize=%d, OverlapSize=%d",
		cfg.DocumentSplitter.ChunkSize, cfg.DocumentSplitter.OverlapSize)

	// 4. 初始化索引器服务
	log.Println("Initializing Indexer Service...")
	indexerConfig := &milvusIndexer.IndexerConfig{
		Client:     milvusClient,
		Collection: cfg.Milvus.CollectionName,
		Embedding:  embeddingService.GetEmbedder(),
	}
	indexerService, err := storage.NewIndexerServiceWithDimension(ctx, indexerConfig, cfg.Embedding.Dimensions)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize indexer service: %w", err)
	}
	manager.IndexerService = indexerService
	log.Printf("Indexer Service initialized: Collection=%s, Dimension=%d",
		cfg.Milvus.CollectionName, cfg.Embedding.Dimensions)

	// 5. 初始化检索器服务
	log.Println("Initializing Retriever Service...")
	retrieverConfig := &milvusRetriever.RetrieverConfig{
		Client:       milvusClient,
		Collection:   cfg.Milvus.CollectionName,
		VectorField:  "vector",
		OutputFields: []string{"id", "content", "metadata"},
		MetricType:   entity.MetricType(cfg.Milvus.MetricType),
		TopK:         cfg.Milvus.TopK,
		Embedding:    embeddingService.GetEmbedder(),
	}
	retrieverService, err := retrieval.NewRetrieverService(ctx, retrieverConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize retriever service: %w", err)
	}
	manager.RetrieverService = retrieverService
	log.Printf("Retriever Service initialized: TopK=%d, MetricType=%s",
		cfg.Milvus.TopK, cfg.Milvus.MetricType)

	// 6. 初始化混合检索服务
	log.Println("Initializing Hybrid Retriever Service...")
	// 如果配置文件中没有专门的 Rerank 配置，使用默认值
	// 后续可以在 config.Config 中添加 Rerank 配置字段
	candidateTopK := cfg.Milvus.TopK * 2
	if cfg.RAG.Phase2.CandidateTopK > 0 {
		candidateTopK = cfg.RAG.Phase2.CandidateTopK
	}
	hybridConfig := &retrieval.HybridRetrieverConfig{
		CandidateTopK: candidateTopK,
		DenseWeight:   cfg.RAG.Phase2.HybridDenseWeight,
		SparseWeight:  cfg.RAG.Phase2.HybridSparseWeight,
		SparseConfig: &retrieval.SparseRetrieverConfig{
			DefaultTopK: candidateTopK,
		},
		DynamicTopK: retrieval.DynamicTopKConfig{
			Enabled:         cfg.RAG.FeatureFlags.EnableDynamicTopK,
			MinTopK:         cfg.RAG.Phase2.MinTopK,
			MaxTopK:         cfg.RAG.Phase2.MaxTopK,
			TokenBudget:     cfg.RAG.Phase2.TokenBudget,
			MinAnswerChunks: cfg.RAG.Phase2.MinAnswerChunks,
		},
	}
	if cfg.RAG.FeatureFlags.EnableQueryRewrite {
		hybridConfig.QueryRewriter = retrieval.NewControlledQueryRewriter(&retrieval.QueryRewriterConfig{
			MaxExpansions: cfg.RAG.Phase2.RewriteMaxExpansions,
		})
	}
	hybridRetriever, err := retrieval.NewHybridRetriever(retrieverService, hybridConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize hybrid retriever: %w", err)
	}
	manager.HybridRetriever = hybridRetriever
	log.Println("Hybrid Retriever Service initialized")

	// 保存到全局变量
	globalManager = manager

	log.Println("Milvus Manager initialized successfully!")
	return manager, nil
}

// GetMilvusManager 获取全局Milvus管理器实例
func GetMilvusManager() (*MilvusManager, error) {
	if globalManager == nil {
		return nil, fmt.Errorf("milvus manager.go not initialized, call InitMilvusManager first")
	}
	return globalManager, nil
}

// Close 关闭所有服务和连接
func (m *MilvusManager) Close() error {
	log.Println("Closing Milvus Manager...")
	var errs []error
	// 关闭 Embedding 服务
	if m.EmbeddingService != nil {
		if err := m.EmbeddingService.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close embedding service: %w", err))
		}
	}
	// 关闭 Milvus 客户端
	if m.Client != nil {
		if err := m.Client.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close milvus client: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during closing: %v", errs)
	}

	log.Println("Milvus Manager closed successfully")
	return nil
}

// GetEmbeddingService 获取Embedding服务
func (m *MilvusManager) GetEmbeddingService() *storage.EmbeddingService {
	return m.EmbeddingService
}

// GetSplitterService 获取文档分割器服务
func (m *MilvusManager) GetSplitterService() *splitter.DocumentSplitterService {
	return m.SplitterService
}

// GetIndexerService 获取索引器服务
func (m *MilvusManager) GetIndexerService() *storage.IndexerService {
	return m.IndexerService
}

// GetRetrieverService 获取检索器服务
func (m *MilvusManager) GetRetrieverService() *retrieval.RetrieverService {
	return m.RetrieverService
}

// GetHybridRetriever 获取混合检索器服务
func (m *MilvusManager) GetHybridRetriever() *retrieval.HybridRetriever {
	return m.HybridRetriever
}

// GetClient 获取Milvus客户端
func (m *MilvusManager) GetClient() client.Client {
	return m.Client
}

// HealthCheck 健康检查
func (m *MilvusManager) HealthCheck(ctx context.Context) error {
	if m.Client == nil {
		return fmt.Errorf("milvus client is nil")
	}
	// 创建一个带超时的上下文
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// 检查连接是否正常（可以通过列出集合来验证）
	collections, err := m.Client.ListCollections(checkCtx)
	if err != nil {
		return fmt.Errorf("milvus health check failed: %w", err)
	}
	log.Printf("Milvus health check passed, collections count: %d", len(collections))
	return nil
}

func (m *MilvusManager) DeleteDocumentVectors(ctx context.Context, collection string, documentID uint64) error {
	if m.IndexerService == nil {
		return fmt.Errorf("indexer service is nil")
	}
	expr := fmt.Sprintf("metadata[\"document_id\"] == %d", documentID)
	return m.IndexerService.DeleteByExpr(ctx, collection, expr)
}
