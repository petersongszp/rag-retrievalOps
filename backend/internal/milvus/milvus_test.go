package milvus

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"interview-agents/internal/config"

	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

// init 在所有测试前自动加载 .env 文件
func init() {
	// 获取项目根目录的 .env 文件路径
	// 从 internal/eino/milvus/ 到根目录需要向上3级
	envPath := filepath.Join("..", "..", "..", ".env")
	if err := godotenv.Load(envPath); err != nil {
		fmt.Printf("Warning: Could not load .env file from %s: %v\n", envPath, err)
		fmt.Println("Tests will use system environment variables or default values")
	} else {
		fmt.Printf("Successfully loaded .env file from %s\n", envPath)
	}
}

// 创建测试配置
func getTestConfig() *config.Config {
	return &config.Config{
		Embedding: config.EmbeddingConfig{
			APIKey:     os.Getenv("EMBEDDING_API_KEY"),
			Model:      getEnvOrDefault("EMBEDDING_MODEL", "doubao-embedding-text-240715"), // 默认使用 embedding 模型
			BaseURL:    "https://ark.cn-beijing.volces.com/api/v3/",
			Region:     getEnvOrDefault("EMBEDDING_REGION", "cn-beijing"),
			Timeout:    30 * time.Second,
			RetryTimes: 3,
			Dimensions: 2560, // 向量维度（doubao-embedding-text-240715 实际输出维度）
		},
		DocumentSplitter: config.SplitterConfig{
			ChunkSize:   500,
			OverlapSize: 50,
			Separators:  []string{"\n\n", "\n", " "},
			KeepType:    0, // 0=不保留, 1=保留在开头, 2=保留在结尾
		},
		Milvus: config.MilvusConfig{
			Address:        getEnvOrDefault("MILVUS_ADDRESS", "localhost:19530"),
			CollectionName: "knowledge",
			DatabaseName:   getEnvOrDefault("MILVUS_DATABASE", "default"),
			MetricType:     "COSINE",
			Username:       getEnvOrDefault("MILVUS_USERNAME", "minioadmin"),
			Password:       getEnvOrDefault("MILVUS_PASSWORD", "minioadmin"),
			TopK:           5,
			ConnectTimeout: 10 * time.Second,
			SearchTimeout:  30 * time.Second,
		},
	}
}

// getEnvOrDefault 获取环境变量，如果不存在则返回默认值
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// TestMilvusManagerInit 测试 MilvusManager 初始化
func TestMilvusManagerInit(t *testing.T) {

	ctx := context.Background()
	cfg := getTestConfig()

	manager, err := InitMilvusManager(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to initialize MilvusManager: %v", err)
	}
	defer manager.Close()

	// 测试健康检查
	if err := manager.HealthCheck(ctx); err != nil {
		t.Logf("Health check warning: %v", err)
	}

	t.Log("MilvusManager initialized successfully")
}

// TestEmbeddingService 测试嵌入服务
func TestEmbeddingService(t *testing.T) {

	ctx := context.Background()
	cfg := getTestConfig()

	manager, err := InitMilvusManager(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to initialize MilvusManager: %v", err)
	}
	defer manager.Close()

	// 测试单个文本嵌入
	testText := "这是一个测试文本，用于验证嵌入服务是否正常工作。"

	embedder := manager.EmbeddingService.GetEmbedder()
	result, err := embedder.EmbedStrings(ctx, []string{testText})
	if err != nil {
		t.Fatalf("Failed to embed text: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("Expected 1 embedding, got %d", len(result))
	}

	if len(result[0]) != cfg.Embedding.Dimensions {
		t.Fatalf("Expected embedding dimension %d, got %d", cfg.Embedding.Dimensions, len(result[0]))
	}

	t.Logf("Successfully embedded text, dimension: %d", len(result[0]))
	t.Logf("First 5 values: %v", result[0][:5])
}

// TestDocumentSplitter 测试文档分割服务
func TestDocumentSplitter(t *testing.T) {

	ctx := context.Background()
	cfg := getTestConfig()

	manager, err := InitMilvusManager(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to initialize MilvusManager: %v", err)
	}
	defer manager.Close()

	// 测试文档分割
	longText := `
这是一个长文档的第一段。它包含了一些基本信息。

这是第二段，讨论了一些其他的主题。这里有更多的内容需要处理。

第三段继续深入探讨主题。我们需要确保文档分割器能够正确地处理这些内容。

第四段添加了更多的细节。文档分割器应该能够根据配置的参数将这个长文档分割成多个小块。

最后一段总结了全文的内容。这样我们就有了一个完整的测试文档。
	`

	doc := &schema.Document{
		Content: longText,
		MetaData: map[string]any{
			"source": "test",
			"author": "tester",
		},
	}

	chunks, err := manager.SplitterService.Split(ctx, []*schema.Document{doc})
	if err != nil {
		t.Fatalf("Failed to split document: %v", err)
	}

	t.Logf("Document split into %d chunks", len(chunks))
	for i, chunk := range chunks {
		t.Logf("Chunk %d (length: %d): %s...", i+1, len(chunk.Content), truncate(chunk.Content, 50))
	}
}

// TestIndexerService 测试索引服务（存储文档）
func TestIndexerService(t *testing.T) {

	ctx := context.Background()
	cfg := getTestConfig()

	manager, err := InitMilvusManager(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to initialize MilvusManager: %v", err)
	}
	defer manager.Close()

	// 准备测试文档
	docs := []*schema.Document{
		{
			ID:      "doc1",
			Content: "Go 是一种开源编程语言，由 Google 开发。它具有静态类型、编译型、并发性等特点。",
			MetaData: map[string]any{
				"topic":    "programming",
				"language": "go",
			},
		},
		{
			ID:      "doc2",
			Content: "Python 是一种高级编程语言，广泛用于数据科学、机器学习和 Web 开发。",
			MetaData: map[string]any{
				"topic":    "programming",
				"language": "python",
			},
		},
		{
			ID:      "doc3",
			Content: "向量数据库是一种专门用于存储和检索高维向量的数据库系统，常用于相似度搜索。",
			MetaData: map[string]any{
				"topic": "database",
			},
		},
	}

	// 存储文档
	docIDs, err := manager.IndexerService.Store(ctx, docs)
	if err != nil {
		t.Fatalf("Failed to store documents: %v", err)
	}

	t.Logf("Successfully stored %d documents", len(docIDs))
	for i, id := range docIDs {
		t.Logf("Document %d ID: %s", i+1, id)
	}

	// 等待一下，确保数据被索引
	time.Sleep(2 * time.Second)
}

// TestRetrieverService 测试检索服务
func TestRetrieverService(t *testing.T) {

	ctx := context.Background()
	cfg := getTestConfig()

	manager, err := InitMilvusManager(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to initialize MilvusManager: %v", err)
	}
	defer manager.Close()

	// 先存储一些文档
	docs := []*schema.Document{
		{
			ID:      "doc1",
			Content: "Go 语言是一种现代编程语言，特别适合构建微服务和云原生应用。",
			MetaData: map[string]any{
				"topic": "go",
			},
		},
		{
			ID:      "doc2",
			Content: "Milvus 是一个开源向量数据库，专为 AI 应用设计，支持大规模向量检索。",
			MetaData: map[string]any{
				"topic": "database",
			},
		},
		{
			ID:      "doc3",
			Content: "机器学习模型可以将文本转换为向量表示，这个过程称为文本嵌入。",
			MetaData: map[string]any{
				"topic": "ai",
			},
		},
	}

	_, err = manager.IndexerService.Store(ctx, docs)
	if err != nil {
		t.Fatalf("Failed to store documents: %v", err)
	}

	// 等待索引完成
	time.Sleep(2 * time.Second)

	// 测试检索
	query := "Go 编程语言的特点"
	results, err := manager.RetrieverService.Retrieve(ctx, query)
	if err != nil {
		t.Fatalf("Failed to retrieve documents: %v", err)
	}

	t.Logf("Query: %s", query)
	t.Logf("Found %d results:", len(results))
	for i, doc := range results {
		score := "N/A"
		if s, ok := doc.MetaData["score"].(float32); ok {
			score = fmt.Sprintf("%.4f", s)
		}
		t.Logf("Result %d (score: %s): %s", i+1, score, truncate(doc.Content, 80))
	}
}

// TestFullWorkflow 测试完整的工作流程
func TestFullWorkflow(t *testing.T) {

	ctx := context.Background()
	cfg := getTestConfig()

	manager, err := InitMilvusManager(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to initialize MilvusManager: %v", err)
	}
	defer manager.Close()

	// 1. 准备长文档
	longDoc := `
Go 语言简介

Go（又称 Golang）是 Google 开发的一种静态强类型、编译型、并发型编程语言。
Go 语言于 2009 年 11 月正式宣布推出，成为开放源代码项目，并在 Linux 及 Mac OS X 平台上进行了实现。

主要特点

1. 简洁性：Go 语言的语法简洁明了，易于学习和使用。
2. 并发性：Go 原生支持并发，通过 goroutine 和 channel 实现。
3. 性能：作为编译型语言，Go 的执行效率接近 C/C++。
4. 垃圾回收：自动内存管理，减少内存泄漏风险。

应用场景

Go 语言特别适合以下场景：
- 云原生应用开发
- 微服务架构
- 网络编程
- 分布式系统
- DevOps 工具
	`

	t.Log("Step 1: Splitting document...")
	doc := &schema.Document{
		Content: longDoc,
		MetaData: map[string]any{
			"title":  "Go 语言介绍",
			"source": "test",
		},
	}

	chunks, err := manager.SplitterService.Split(ctx, []*schema.Document{doc})
	if err != nil {
		t.Fatalf("Failed to split document: %v", err)
	}
	t.Logf("Document split into %d chunks", len(chunks))

	// 2. 存储文档块
	t.Log("\nStep 2: Storing document chunks...")
	docIDs, err := manager.IndexerService.Store(ctx, chunks)
	if err != nil {
		t.Fatalf("Failed to store chunks: %v", err)
	}
	t.Logf("Stored %d chunks", len(docIDs))

	// 等待索引完成
	time.Sleep(2 * time.Second)

	// 3. 执行检索
	t.Log("\nStep 3: Retrieving relevant documents...")
	queries := []string{
		"Go 语言的并发特性是什么？",
		"Go 语言适合哪些应用场景？",
		"Go 语言是什么时候发布的？",
	}

	for _, query := range queries {
		t.Logf("\n--- Query: %s ---", query)
		results, err := manager.RetrieverService.Retrieve(ctx, query)
		if err != nil {
			t.Errorf("Failed to retrieve for query '%s': %v", query, err)
			continue
		}

		t.Logf("Found %d results:", len(results))
		for i, result := range results {
			score := "N/A"
			if s, ok := result.MetaData["score"].(float32); ok {
				score = fmt.Sprintf("%.4f", s)
			}
			t.Logf("  %d. (score: %s) %s", i+1, score, truncate(result.Content, 100))
		}
	}

	t.Log("\n✓ Full workflow completed successfully!")
}

// TestIndexerMultipleDocuments 测试存储多个文档
func TestIndexerMultipleDocuments(t *testing.T) {

	ctx := context.Background()
	cfg := getTestConfig()

	manager, err := InitMilvusManager(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to initialize MilvusManager: %v", err)
	}
	defer manager.Close()

	// 准备多个测试文档
	docs := []*schema.Document{
		{
			ID:      "test_doc_1",
			Content: "这是第一个测试文档，包含一些元数据。",
			MetaData: map[string]any{
				"category": "test",
				"priority": 1,
			},
		},
		{
			ID:      "test_doc_2",
			Content: "这是第二个测试文档，用于批量存储测试。",
			MetaData: map[string]any{
				"category": "test",
				"priority": 2,
			},
		},
		{
			ID:      "test_doc_3",
			Content: "这是第三个测试文档，验证批量插入功能。",
			MetaData: map[string]any{
				"category": "test",
				"priority": 3,
			},
		},
	}

	// 批量存储文档
	docIDs, err := manager.IndexerService.Store(ctx, docs)
	if err != nil {
		t.Fatalf("Failed to store documents: %v", err)
	}

	if len(docIDs) != len(docs) {
		t.Fatalf("Expected %d document IDs, got %d", len(docs), len(docIDs))
	}

	t.Logf("Successfully stored %d documents", len(docIDs))
	for i, id := range docIDs {
		t.Logf("  Document %d ID: %s", i+1, id)
	}
}

// TestRetrieverMultipleQueries 测试多个查询
func TestRetrieverMultipleQueries(t *testing.T) {

	ctx := context.Background()
	cfg := getTestConfig()

	manager, err := InitMilvusManager(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to initialize MilvusManager: %v", err)
	}
	defer manager.Close()

	// 先存储文档
	docs := []*schema.Document{
		{
			ID:      "multi_doc_1",
			Content: "向量数据库使用向量嵌入来表示数据，这是机器学习中的关键技术。",
			MetaData: map[string]any{
				"topic": "vector_db",
			},
		},
		{
			ID:      "multi_doc_2",
			Content: "Go 语言是一种高效的编程语言，特别适合构建云原生应用。",
			MetaData: map[string]any{
				"topic": "programming",
			},
		},
		{
			ID:      "multi_doc_3",
			Content: "相似度搜索通过计算向量之间的距离来找到最相关的内容。",
			MetaData: map[string]any{
				"topic": "vector_db",
			},
		},
	}

	_, err = manager.IndexerService.Store(ctx, docs)
	if err != nil {
		t.Fatalf("Failed to store documents: %v", err)
	}

	time.Sleep(2 * time.Second)

	// 测试多个查询
	queries := []string{
		"向量嵌入是什么？",
		"Go 语言的特点",
		"如何进行相似度搜索？",
	}

	for i, query := range queries {
		t.Logf("\n--- Query %d: %s ---", i+1, query)
		results, err := manager.RetrieverService.Retrieve(ctx, query)
		if err != nil {
			t.Errorf("Failed to retrieve for query '%s': %v", query, err)
			continue
		}

		t.Logf("Found %d results:", len(results))
		for j, doc := range results {
			score := "N/A"
			if s, ok := doc.MetaData["score"].(float32); ok {
				score = fmt.Sprintf("%.4f", s)
			}
			t.Logf("  %d. (score: %s) %s", j+1, score, truncate(doc.Content, 60))
		}
	}
}

// TestMetricTypes 测试不同的距离度量类型
func TestMetricTypes(t *testing.T) {

	ctx := context.Background()

	metricTypes := []string{"COSINE", "L2", "IP"}

	for _, metricType := range metricTypes {
		t.Run(fmt.Sprintf("MetricType_%s", metricType), func(t *testing.T) {
			cfg := getTestConfig()
			cfg.Milvus.MetricType = metricType
			cfg.Milvus.CollectionName = fmt.Sprintf("test_collection_%s", metricType)

			manager, err := InitMilvusManager(ctx, cfg)
			if err != nil {
				t.Fatalf("Failed to initialize with metric type %s: %v", metricType, err)
			}
			defer manager.Close()

			t.Logf("Successfully initialized with metric type: %s", metricType)
		})
	}
}

// 辅助函数：截断字符串
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// TestHybridRetrieverIntegration 集成测试：混合检索服务
func TestHybridRetrieverIntegration(t *testing.T) {
	ctx := context.Background()
	cfg := getTestConfig()

	manager, err := InitMilvusManager(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to initialize MilvusManager: %v", err)
	}
	defer manager.Close()

	// 1. 准备测试文档
	docs := []*schema.Document{
		{
			ID:       "hybrid_doc_1",
			Content:  "Go 语言的并发模型基于 CSP，使用 goroutine 和 channel。",
			MetaData: map[string]any{"topic": "concurrency"},
		},
		{
			ID:       "hybrid_doc_2",
			Content:  "Go 内存分配器使用了类似 TCMalloc 的分分级分配策略。",
			MetaData: map[string]any{"topic": "memory"},
		},
	}

	_, err = manager.IndexerService.Store(ctx, docs)
	if err != nil {
		t.Fatalf("Failed to store documents: %v", err)
	}

	// 等待索引
	time.Sleep(2 * time.Second)

	// 2. 执行混合检索
	query := "Go 并发 goroutine"
	hybridRetriever := manager.GetHybridRetriever()

	results, err := hybridRetriever.Search(ctx, query, nil)
	if err != nil {
		t.Fatalf("Hybrid search failed: %v", err)
	}

	t.Logf("Hybrid Search Results (Query: %s):", query)
	for i, doc := range results {
		rerankScore := doc.MetaData["rerank_score"]
		t.Logf("  %d. [Score: %v] %s", i+1, rerankScore, truncate(doc.Content, 100))
	}

	if len(results) == 0 {
		t.Error("Expected at least one result")
	} else if _, ok := results[0].MetaData["rerank_score"]; !ok {
		t.Error("Expected rerank_score in document metadata")
	}
}
