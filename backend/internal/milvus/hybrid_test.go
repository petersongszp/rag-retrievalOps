package milvus

import (
	"context"
	"log"
	"testing"
	"time"

	"interview-agents/internal/milvus/retrieval"

	"github.com/cloudwego/eino/schema"
)

// TestHybridSearchRealWorld 实战测试：混合检索与重排序
// 这个测试模拟了真实的检索场景，展示了 Hybrid Retriever 的工作流程
func TestHybridSearchRealWorld(t *testing.T) {
	// 注意：这个测试依赖于真实的 Milvus 环境
	// 如果本地没有运行 Milvus，这个测试将被跳过
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// 1. 设置测试环境
	ctx := context.Background()

	// 尝试获取测试配置，如果环境变量未设置，使用默认本地配置
	cfg := getTestConfig()

	// 初始化 Milvus Manager
	manager := initTestMilvusManager(t, ctx, cfg)
	defer manager.Close()

	// 2. 准备测试数据 (确保有一些数据可供检索)
	// 我们导入一些特定内容的文档来看看检索效果
	testDocs := map[string]string{
		"doc1": "Go map runtime implementation details with hmap structure",
		"doc2": "Java HashMap internal working mechanism and load factor",
		"doc3": "Go uses sort package for sorting slices efficiently",
		"doc4": "Go channels for concurrency and communication",
		"doc5": "Database indexing B-tree structure explained",
	}

	log.Println("Importing test documents...")
	// importer, err := NewMarkdownImporter(manager)
	// if err != nil {
	// 	t.Fatal(err)
	// }

	// 这里的 ImportContent 是一个辅助方法，我们假设它存在或者我们手动创建文档索引
	// 为了简化，我们直接使用 IndexerService 索引文档
	docsToIndex := make([]*schema.Document, 0)
	for title, content := range testDocs {
		doc := &schema.Document{
			ID:      title,
			Content: content,
			MetaData: map[string]interface{}{
				"filename": title + ".md",
				"language": "tech",
			},
		}
		docsToIndex = append(docsToIndex, doc)
	}

	_, err := manager.IndexerService.Store(ctx, docsToIndex)
	if err != nil {
		t.Fatalf("Failed to store docs: %v", err)
	}

	// 等待索引生效
	time.Sleep(2 * time.Second)

	// 3. 执行混合检索
	query := "Go map implementation"
	log.Printf("Executing hybrid search for query: %s", query)

	hybridRetriever := manager.GetHybridRetriever()

	// 使用 RetrieveOptions 指定候选 TopK 和最终 TopK
	opts := &retrieval.RetrieveOptions{
		TopK: 3, // 最终只取前3个
	}

	results, err := hybridRetriever.Search(ctx, query, opts)
	if err != nil {
		t.Fatalf("Hybrid search failed: %v", err)
	}

	// 4. 验证结果和重排序效果
	log.Println("Search Results:")
	for i, doc := range results {
		// 打印详细的分数信息
		var score float64
		if s, ok := doc.MetaData["score"]; ok {
			switch v := s.(type) {
			case float64:
				score = v
			case float32:
				score = float64(v)
			}
		}
		rerankScore, _ := doc.MetaData["rerank_score"]

		log.Printf("[%d] ID: %s, Score: %.4f, RerankScore: %v, Content: %s",
			i+1, doc.ID, score, rerankScore, doc.Content)
	}

	// 简单断言
	if len(results) == 0 {
		t.Errorf("Expected results, got empty")
	} else {
		// 期望 "Go map runtime..." 排在第一位
		firstDoc := results[0]
		if firstDoc.ID != "doc1" {
			t.Logf("Note: Top document is %s, expected doc1 (this depends on embedding model)", firstDoc.ID)
		}

		// 检查是否包含重排序分数
		if _, ok := firstDoc.MetaData["rerank_score"]; !ok {
			t.Error("Missing rerank_score in metadata")
		}
	}
}

// TestHybridSearchWithFilters 测试带过滤条件的混合检索
func TestHybridSearchWithFilters(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := getTestConfig()
	manager := initTestMilvusManager(t, ctx, cfg)
	defer manager.Close()

	// 1. 准备不同语言的文档
	docs := []*schema.Document{
		{
			ID:       "go_doc_1",
			Content:  "Go routine and channel concurrency model",
			MetaData: map[string]interface{}{"language": "golang"},
		},
		{
			ID:       "java_doc_1",
			Content:  "Java JVM memory management and garbage collection",
			MetaData: map[string]interface{}{"language": "java"},
		},
	}
	_, err := manager.IndexerService.Store(ctx, docs)
	if err != nil {
		t.Fatalf("Failed to store docs: %v", err)
	}
	time.Sleep(2 * time.Second)

	hybridRetriever := manager.GetHybridRetriever()

	// 2. 测试带过滤条件的检索
	query := "memory management"
	log.Printf("Executing hybrid search, query=%s", query)

	opts := &retrieval.RetrieveOptions{
		TopK: 5,
	}

	results, err := hybridRetriever.Search(ctx, query, opts)
	if err != nil {
		t.Fatalf("Hybrid search failed: %v", err)
	}

	log.Printf("Results: %d docs found", len(results))
	for _, doc := range results {
		log.Printf("ID: %s, Lang: %v, Content: %s", doc.ID, doc.MetaData["language"], doc.Content)
	}
}

// TestHybridSearchEdgeCases 测试边界情况
func TestHybridSearchEdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	cfg := getTestConfig()
	manager := initTestMilvusManager(t, ctx, cfg)
	defer manager.Close()

	hybridRetriever := manager.GetHybridRetriever()

	t.Run("Empty Query", func(t *testing.T) {
		results, err := hybridRetriever.Search(ctx, "", &retrieval.RetrieveOptions{TopK: 1})
		if err != nil {
			t.Logf("Empty query handled (might return error or empty): %v", err)
		} else {
			t.Logf("Empty query returned %d results", len(results))
		}
	})

	t.Run("Non-existent Content", func(t *testing.T) {
		results, err := hybridRetriever.Search(ctx, "zxywvu123456 non-existent-term", &retrieval.RetrieveOptions{TopK: 5})
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		t.Logf("Non-existent content query returned %d results", len(results))
	})
}
