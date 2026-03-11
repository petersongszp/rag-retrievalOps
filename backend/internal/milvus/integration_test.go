package milvus

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestFullWorkflowWithMarkdown 覆盖每个数据源类别的单个与批量存储，以及检索
func TestFullWorkflowWithMarkdown(t *testing.T) {
	ctx := context.Background()
	cfg := getTestConfig()

	// 1. 初始化 Milvus 管理器
	t.Log("=== Step 1: 初始化 Milvus 管理器 ===")
	manager, err := InitMilvusManager(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to initialize MilvusManager: %v", err)
	}
	defer manager.Close()

	// 健康检查
	if err := manager.HealthCheck(ctx); err != nil {
		t.Logf("Health check warning: %v", err)
	} else {
		t.Log("✓ Milvus 连接正常")
	}

	// 2. 创建 Markdown 导入器
	t.Log("\n=== Step 2: 创建 Markdown 导入器 ===")
	importer, err := NewMarkdownImporter(manager)
	if err != nil {
		t.Fatalf("Failed to create MarkdownImporter: %v", err)
	}
	t.Log("✓ Markdown 导入器创建成功")

	// 3. 针对每个数据源类别：单个存储 + 批量存储
	t.Log("\n=== Step 3: 针对每个数据源类别进行单个与批量存储 ===")
	dataRoot := filepath.Join(".", "data")
	if _, statErr := os.Stat(dataRoot); statErr != nil {
		t.Fatalf("数据目录不存在: %s, err=%v", dataRoot, statErr)
	}

	// 3.1 Golang
	t.Run("Store_Golang_SingleAndBatch", func(t *testing.T) {
		// 单个存储：go基础/intro.md
		goSingle := filepath.Join(dataRoot, "go基础", "intro.md")
		t.Logf("Golang 单个存储: %s", goSingle)
		_, err := importer.ImportFile(ctx, goSingle, &ImportOptions{
			Path:     goSingle,
			Language: LanguageGolang,
			Category: CategoryBasic,
			Source:   "单测",
		})
		if err != nil {
			t.Fatalf("Golang 单个存储失败: %v", err)
		}
		// 批量存储：go专项 目录
		goBatchDir := filepath.Join(dataRoot, "go专项")
		t.Logf("Golang 批量存储目录: %s", goBatchDir)
		_, err = importer.ImportDirectory(ctx, goBatchDir, &ImportOptions{
			Recursive:  true,
			Extensions: []string{".md", ".markdown"},
			SkipHidden: true,
			Language:   LanguageGolang,
			Category:   CategorySpecialized,
			Source:     "单测",
		})
		if err != nil {
			t.Fatalf("Golang 批量存储失败: %v", err)
		}
	})

	// 3.2 Java
	t.Run("Store_Java_SingleAndBatch", func(t *testing.T) {
		// 单个存储：java专项/jvm-tuning.md
		javaSingle := filepath.Join(dataRoot, "java专项", "jvm-tuning.md")
		t.Logf("Java 单个存储: %s", javaSingle)
		_, err := importer.ImportFile(ctx, javaSingle, &ImportOptions{
			Path:     javaSingle,
			Language: LanguageJava,
			Category: CategorySpecialized,
			Source:   "单测",
		})
		if err != nil {
			t.Fatalf("Java 单个存储失败: %v", err)
		}
		// 批量存储：java基础 目录（若存在）
		javaBatchDir := filepath.Join(dataRoot, "java基础")
		if _, errDir := os.Stat(javaBatchDir); errDir == nil {
			t.Logf("Java 批量存储目录: %s", javaBatchDir)
			_, err = importer.ImportDirectory(ctx, javaBatchDir, &ImportOptions{
				Recursive:  true,
				Extensions: []string{".md", ".markdown"},
				SkipHidden: true,
				Language:   LanguageJava,
				Category:   CategoryBasic,
				Source:     "单测",
			})
			if err != nil {
				t.Fatalf("Java 批量存储失败: %v", err)
			}
		} else {
			t.Logf("跳过 Java 基础批量导入，目录不存在: %s", javaBatchDir)
		}
	})

	// 3.3 中间件
	t.Run("Store_Middleware_SingleAndBatch", func(t *testing.T) {
		// 单个存储：中间件基础/intro.md
		mwSingle := filepath.Join(dataRoot, "中间件基础", "intro.md")
		t.Logf("中间件 单个存储: %s", mwSingle)
		_, err := importer.ImportFile(ctx, mwSingle, &ImportOptions{
			Path:     mwSingle,
			Language: LanguageMiddleware,
			Category: CategoryBasic,
			Source:   "单测",
		})
		if err != nil {
			t.Fatalf("中间件 单个存储失败: %v", err)
		}
		// 批量存储：中间件专项 目录
		mwBatchDir := filepath.Join(dataRoot, "中间件专项")
		t.Logf("中间件 批量存储目录: %s", mwBatchDir)
		_, err = importer.ImportDirectory(ctx, mwBatchDir, &ImportOptions{
			Recursive:  true,
			Extensions: []string{".md", ".markdown"},
			SkipHidden: true,
			Language:   LanguageMiddleware,
			Category:   CategorySpecialized,
			Source:     "单测",
		})
		if err != nil {
			t.Fatalf("中间件 批量存储失败: %v", err)
		}
	})

	// 3.4 综合（作为类别维度的补充）
	t.Run("Store_Comprehensive_SingleAndBatch", func(t *testing.T) {
		// 单个存储：综合/architectures.md
		compSingle := filepath.Join(dataRoot, "综合", "architectures.md")
		t.Logf("综合 单个存储: %s", compSingle)
		_, err := importer.ImportFile(ctx, compSingle, &ImportOptions{
			Path:   compSingle,
			Source: "单测",
			// 语言/分类依路径推断
		})
		if err != nil {
			t.Fatalf("综合 单个存储失败: %v", err)
		}
		// 批量存储：综合 目录（若后续新增多个文件）
		compBatchDir := filepath.Join(dataRoot, "综合")
		t.Logf("综合 批量存储目录: %s", compBatchDir)
		_, err = importer.ImportDirectory(ctx, compBatchDir, &ImportOptions{
			Recursive:  true,
			Extensions: []string{".md", ".markdown"},
			SkipHidden: true,
			Source:     "单测",
		})
		if err != nil {
			t.Fatalf("综合 批量存储失败: %v", err)
		}
	})

	// 等待索引完成
	t.Log("\n等待索引完成...")
	time.Sleep(3 * time.Second)

	// 4. 测试检索功能
	t.Log("\n=== Step 4: 测试检索功能 ===")

	// 4.1 基本检索（不带过滤）
	t.Log("\n--- 测试 1: 基本检索（不带过滤） ---")
	query1 := "Go 并发 编程"
	results1, err := manager.RetrieverService.Retrieve(ctx, query1)
	if err != nil {
		t.Fatalf("Failed to retrieve: %v", err)
	}
	t.Logf("查询: %s", query1)
	t.Logf("找到 %d 个结果:", len(results1))
	for i, doc := range results1 {
		score := "N/A"
		if s, ok := doc.MetaData["score"].(float32); ok {
			score = fmt.Sprintf("%.4f", s)
		}
		language := "N/A"
		if l, ok := doc.MetaData["language"].(string); ok {
			language = l
		}
		t.Logf("  %d. (score: %s, language: %s) %s", i+1, score, language, truncate(doc.Content, 100))
	}

	// 4.2 按语言类型过滤 - Golang
	t.Log("\n--- 测试 2: 按语言类型过滤 - Golang ---")
	query2 := "Goroutine Channel Worker Pool"
	opts2 := &RetrieveOptions{
		Language: LanguageGolang,
		TopK:     5,
	}
	results2, err := manager.RetrieverService.RetrieveWithOptions(ctx, query2, opts2)
	if err != nil {
		t.Fatalf("Failed to retrieve with options: %v", err)
	}
	t.Logf("查询: %s (过滤: Golang)", query2)
	t.Logf("找到 %d 个结果:", len(results2))
	for i, doc := range results2 {
		score := "N/A"
		if s, ok := doc.MetaData["score"].(float32); ok {
			score = fmt.Sprintf("%.4f", s)
		}
		language := "N/A"
		if l, ok := doc.MetaData["language"].(string); ok {
			language = l
		}
		t.Logf("  %d. (score: %s, language: %s) %s", i+1, score, language, truncate(doc.Content, 100))
	}

	// 4.3 按语言类型过滤 - Java
	t.Log("\n--- 测试 3: 按语言类型过滤 - Java ---")
	query3 := "JVM 垃圾回收 G1 ZGC"
	opts3 := &RetrieveOptions{
		Language: LanguageJava,
		TopK:     5,
	}
	results3, err := manager.RetrieverService.RetrieveWithOptions(ctx, query3, opts3)
	if err != nil {
		t.Fatalf("Failed to retrieve with options: %v", err)
	}
	t.Logf("查询: %s (过滤: Java)", query3)
	t.Logf("找到 %d 个结果:", len(results3))
	for i, doc := range results3 {
		score := "N/A"
		if s, ok := doc.MetaData["score"].(float32); ok {
			score = fmt.Sprintf("%.4f", s)
		}
		language := "N/A"
		if l, ok := doc.MetaData["language"].(string); ok {
			language = l
		}
		t.Logf("  %d. (score: %s, language: %s) %s", i+1, score, language, truncate(doc.Content, 100))
	}

	// 4.4 按语言类型过滤 - 中间件
	t.Log("\n--- 测试 4: 按语言类型过滤 - 中间件 ---")
	query4 := "Kafka 副本 ISR 生产者 幂等 事务"
	opts4 := &RetrieveOptions{
		Language: LanguageMiddleware,
		TopK:     5,
	}
	results4, err := manager.RetrieverService.RetrieveWithOptions(ctx, query4, opts4)
	if err != nil {
		t.Fatalf("Failed to retrieve with options: %v", err)
	}
	t.Logf("查询: %s (过滤: 中间件)", query4)
	t.Logf("找到 %d 个结果:", len(results4))
	for i, doc := range results4 {
		score := "N/A"
		if s, ok := doc.MetaData["score"].(float32); ok {
			score = fmt.Sprintf("%.4f", s)
		}
		language := "N/A"
		if l, ok := doc.MetaData["language"].(string); ok {
			language = l
		}
		t.Logf("  %d. (score: %s, language: %s) %s", i+1, score, language, truncate(doc.Content, 100))
	}

	// 4.5 按分类过滤 - 专项
	t.Log("\n--- 测试 5: 按分类过滤 - 专项 ---")
	query5 := "并发 优化 调优"
	opts5 := &RetrieveOptions{
		Category: CategorySpecialized,
		TopK:     5,
	}
	results5, err := manager.RetrieverService.RetrieveWithOptions(ctx, query5, opts5)
	if err != nil {
		t.Fatalf("Failed to retrieve with options: %v", err)
	}
	t.Logf("查询: %s (过滤: 专项)", query5)
	t.Logf("找到 %d 个结果:", len(results5))
	for i, doc := range results5 {
		score := "N/A"
		if s, ok := doc.MetaData["score"].(float32); ok {
			score = fmt.Sprintf("%.4f", s)
		}
		category := "N/A"
		if c, ok := doc.MetaData["category"].(string); ok {
			category = c
		}
		t.Logf("  %d. (score: %s, category: %s) %s", i+1, score, category, truncate(doc.Content, 100))
	}

	// 4.6 组合过滤 - Golang + 专项
	t.Log("\n--- 测试 6: 组合过滤 - Golang + 专项 ---")
	query6 := "GMP 调度 Channel 模式"
	opts6 := &RetrieveOptions{
		Language: LanguageGolang,
		Category: CategorySpecialized,
		TopK:     5,
	}
	results6, err := manager.RetrieverService.RetrieveWithOptions(ctx, query6, opts6)
	if err != nil {
		t.Fatalf("Failed to retrieve with options: %v", err)
	}
	t.Logf("查询: %s (过滤: Golang + 专项)", query6)
	t.Logf("找到 %d 个结果:", len(results6))
	for i, doc := range results6 {
		score := "N/A"
		if s, ok := doc.MetaData["score"].(float32); ok {
			score = fmt.Sprintf("%.4f", s)
		}
		language := "N/A"
		if l, ok := doc.MetaData["language"].(string); ok {
			language = l
		}
		category := "N/A"
		if c, ok := doc.MetaData["category"].(string); ok {
			category = c
		}
		t.Logf("  %d. (score: %s, language: %s, category: %s) %s", i+1, score, language, category, truncate(doc.Content, 100))
	}

	// 4.7 组合过滤 - Java + 综合
	t.Log("\n--- 测试 7: 组合过滤 - Java + 综合 ---")
	query7 := "架构 可观测性 稳定性 发布策略"
	opts7 := &RetrieveOptions{
		Language: LanguageJava,
		Category: CategoryComprehensive,
		TopK:     5,
	}
	results7, err := manager.RetrieverService.RetrieveWithOptions(ctx, query7, opts7)
	if err != nil {
		t.Fatalf("Failed to retrieve with options: %v", err)
	}
	t.Logf("查询: %s (过滤: Java + 综合)", query7)
	t.Logf("找到 %d 个结果:", len(results7))
	for i, doc := range results7 {
		score := "N/A"
		if s, ok := doc.MetaData["score"].(float32); ok {
			score = fmt.Sprintf("%.4f", s)
		}
		language := "N/A"
		if l, ok := doc.MetaData["language"].(string); ok {
			language = l
		}
		category := "N/A"
		if c, ok := doc.MetaData["category"].(string); ok {
			category = c
		}
		t.Logf("  %d. (score: %s, language: %s, category: %s) %s", i+1, score, language, category, truncate(doc.Content, 100))
	}

	t.Log("\n=== ✓ 完整流程测试完成 ===")
}

// TestImportDirectory 测试导入整个目录
func TestImportDirectory(t *testing.T) {
	ctx := context.Background()
	cfg := getTestConfig()

	// 初始化 Milvus 管理器
	manager, err := InitMilvusManager(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to initialize MilvusManager: %v", err)
	}
	defer manager.Close()

	// 创建 Markdown 导入器
	importer, err := NewMarkdownImporter(manager)
	if err != nil {
		t.Fatalf("Failed to create MarkdownImporter: %v", err)
	}

	// 导入整个目录
	testDataDir := filepath.Join(".", "data")
	t.Logf("导入目录: %s", testDataDir)

	opts := &ImportOptions{
		Recursive:  true,
		Extensions: []string{".md", ".markdown"},
		SkipHidden: true,
	}

	result, err := importer.ImportDirectory(ctx, testDataDir, opts)
	if err != nil {
		t.Fatalf("Failed to import directory: %v", err)
	}

	t.Logf("导入结果:")
	t.Logf("  总文件数: %d", result.TotalFiles)
	t.Logf("  成功文件数: %d", result.SuccessFiles)
	t.Logf("  失败文件数: %d", result.FailedFiles)
	t.Logf("  总块数: %d", result.TotalChunks)
	t.Logf("  文档 ID 数: %d", len(result.DocumentIDs))

	if len(result.Errors) > 0 {
		t.Logf("  错误数: %d", len(result.Errors))
		for i, err := range result.Errors {
			t.Logf("    错误 %d: %v", i+1, err)
		}
	}
}
