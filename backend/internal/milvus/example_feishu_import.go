package milvus

// ============================================================================
// 飞书文档导入示例
// 本文件展示如何将飞书文档（或任何纯文本）切片后上传到 Milvus 数据库
//
// 核心流程：
//   1. 获取纯文本（如从飞书 API 获取并转换为 Markdown）
//   2. 使用 SplitMarkdown 切割文档
//   3. 使用 Store 上传到 Milvus
// ============================================================================

import (
	"context"
	"fmt"

	"interview-agents/internal/config"
)

// ExampleFeishuImport 飞书文档导入示例函数
// 这是一个完整的示例，展示从纯文本到 Milvus 存储的完整流程
//
// 使用场景：
//   - 你已经通过 transmarkdown.go 获取了飞书文档的 Markdown 内容
//   - 现在想要将这些内容切片并存储到 Milvus 以便后续检索
//
// 参数说明：
//   - ctx: 上下文，用于控制超时和取消
//   - cfg: 配置信息，包含 Milvus 连接信息、Embedding 配置等
//   - markdownContent: 已经转换好的 Markdown 文本内容
//
// 返回值：
//   - error: 如果过程中出现错误则返回错误信息
func ExampleFeishuImport(ctx context.Context, cfg *config.Config, markdownContent string) error {
	// ========================================
	// 步骤1：初始化 MilvusManager
	// ========================================
	// MilvusManager 是核心管理器，负责初始化所有服务：
	//   - EmbeddingService: 文本向量化服务
	//   - SplitterService: 文档切割服务
	//   - IndexerService: Milvus 索引服务（用于存储）
	//   - RetrieverService: Milvus 检索服务（用于查询）
	manager, err := InitMilvusManager(ctx, cfg)
	if err != nil {
		return fmt.Errorf("初始化 MilvusManager 失败: %w", err)
	}
	// 重要：使用 defer 确保资源被正确释放
	defer manager.Close()

	// ========================================
	// 步骤2：创建 MarkdownImporter
	// ========================================
	// MarkdownImporter 封装了文档导入的完整流程
	importer, err := NewMarkdownImporter(manager)
	if err != nil {
		return fmt.Errorf("创建 MarkdownImporter 失败: %w", err)
	}

	// ========================================
	// 步骤3：配置导入选项
	// ========================================
	// TextImportOptions 定义了导入的元数据信息
	opts := &TextImportOptions{
		Title:    "Go 并发编程指南",         // 文档标题（可选，留空则自动从内容提取）
		Language: LanguageGolang,      // 语言类型：golang|java|middleware
		Category: CategorySpecialized, // 文档分类：basic|specialized|comprehensive
		Source:   "feishu",            // 来源标识
	}

	// ========================================
	// 步骤4：执行导入
	// ========================================
	// ImportText 方法会：
	//   1. 使用 SplitMarkdown 将长文本切割成小块
	//   2. 为每个块添加元数据和唯一 ID
	//   3. 使用 IndexerService.Store 上传到 Milvus（包含向量化）
	result, err := importer.ImportText(ctx, markdownContent, opts)
	if err != nil {
		return fmt.Errorf("导入文档失败: %w", err)
	}

	// ========================================
	// 步骤5：处理导入结果
	// ========================================
	fmt.Printf("导入成功！\n")
	fmt.Printf("  - 切片数量: %d\n", result.TotalChunks)
	fmt.Printf("  - 存储的文档ID数量: %d\n", len(result.DocumentIDs))

	return nil
}

// ExampleStepByStep 分步骤示例，展示底层 API 的使用方式
// 如果你想更精细地控制每个步骤，可以参考这个示例
func ExampleStepByStep(ctx context.Context, cfg *config.Config, markdownContent string) error {
	// 初始化 MilvusManager
	manager, err := InitMilvusManager(ctx, cfg)
	if err != nil {
		return err
	}
	defer manager.Close()

	// ========================================
	// 分步骤1：使用 SplitterService 切割文档
	// ========================================
	// SplitMarkdown 会根据 Markdown 语法结构智能切割文档
	// 切割后的每个块都是一个 schema.Document 对象
	chunks, err := manager.SplitterService.SplitMarkdown(ctx, markdownContent)
	if err != nil {
		return fmt.Errorf("文档切割失败: %w", err)
	}
	fmt.Printf("文档被切割成 %d 个块\n", len(chunks))

	// 查看切割结果（前3个块）
	for i, chunk := range chunks {
		if i >= 3 {
			break
		}
		// 截取前100个字符显示
		content := chunk.Content
		if len(content) > 100 {
			content = content[:100] + "..."
		}
		fmt.Printf("块 %d: %s\n", i+1, content)
	}

	// ========================================
	// 分步骤2：为每个块添加元数据
	// ========================================
	// 元数据用于后续检索时的过滤和排序
	metadata := NewDocumentMetadata("", LanguageGolang, CategorySpecialized)
	metadata.Title = "示例文档"
	metadata.Source = "example"

	// EnrichDocumentsWithMetadata 会将元数据添加到每个块
	enrichedChunks := EnrichDocumentsWithMetadata(chunks, metadata)

	// 为每个块生成唯一 ID
	for i, chunk := range enrichedChunks {
		if chunk.ID == "" {
			// ID 格式：语言_分类_时间戳_块索引
			chunk.ID = fmt.Sprintf("golang_specialized_%d_%d", 1234567890, i)
		}
	}

	// ========================================
	// 分步骤3：使用 IndexerService 存储到 Milvus
	// ========================================
	// Store 方法会：
	//   1. 调用 EmbeddingService 计算每个块的向量
	//   2. 将向量和文本内容一起存储到 Milvus
	docIDs, err := manager.IndexerService.Store(ctx, enrichedChunks)
	if err != nil {
		return fmt.Errorf("存储到 Milvus 失败: %w", err)
	}
	fmt.Printf("成功存储 %d 个文档块到 Milvus\n", len(docIDs))

	return nil
}

// ExampleRetrieve 检索示例，展示如何从 Milvus 检索相似文档
func ExampleRetrieve(ctx context.Context, cfg *config.Config, query string) error {
	manager, err := InitMilvusManager(ctx, cfg)
	if err != nil {
		return err
	}
	defer manager.Close()

	// 使用 RetrieverService 进行相似度检索
	// 它会：
	//   1. 将查询文本转换为向量
	//   2. 在 Milvus 中搜索最相似的文档块
	//   3. 返回 TopK 个结果
	results, err := manager.RetrieverService.Retrieve(ctx, query)
	if err != nil {
		return fmt.Errorf("检索失败: %w", err)
	}

	fmt.Printf("查询: %s\n", query)
	fmt.Printf("找到 %d 个相关结果:\n", len(results))

	for i, doc := range results {
		score := doc.MetaData["score"]
		content := doc.Content
		if len(content) > 150 {
			content = content[:150] + "..."
		}
		fmt.Printf("%d) 相似度: %v\n   内容: %s\n", i+1, score, content)
	}

	return nil
}
