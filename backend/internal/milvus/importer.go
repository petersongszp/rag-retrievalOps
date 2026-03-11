package milvus

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MarkdownImporter Markdown 文档导入器
// 用于将 Markdown 文档解析、分割并存储到 Milvus
type MarkdownImporter struct {
	manager *MilvusManager
}

// NewMarkdownImporter 创建新的 Markdown 导入器
func NewMarkdownImporter(manager *MilvusManager) (*MarkdownImporter, error) {
	if manager == nil {
		return nil, fmt.Errorf("milvus manager.go is nil")
	}
	return &MarkdownImporter{
		manager: manager,
	}, nil
}

// ImportOptions 导入选项
type ImportOptions struct {
	// 文件路径
	Path string
	// 语言类型
	Language DocumentLanguage
	// 文档分类
	Category DocumentCategory
	// 来源
	Source string
	// 是否递归处理子目录
	Recursive bool
	// 文件扩展名过滤（默认：.md, .markdown）
	Extensions []string
	// 是否跳过隐藏文件
	SkipHidden bool
	// 最大文件大小（字节，0 表示无限制）
	MaxFileSize int64
}

// DefaultImportOptions 返回默认的导入选项
func DefaultImportOptions() *ImportOptions {
	return &ImportOptions{
		Recursive:   true,
		Extensions:  []string{".md", ".markdown"},
		SkipHidden:  true,
		MaxFileSize: 10 * 1024 * 1024, // 10MB
	}
}

// ImportResult 导入结果
type ImportResult struct {
	// 处理的文件总数
	TotalFiles int

	// 成功处理的文件数
	SuccessFiles int

	// 失败的文件数
	FailedFiles int

	// 总文档块数
	TotalChunks int

	// 存储的文档 ID 列表
	DocumentIDs []string

	// 错误信息
	Errors []error
}

// ImportFile 导入单个 Markdown 文件
func (mi *MarkdownImporter) ImportFile(ctx context.Context, filePath string, opts *ImportOptions) (*ImportResult, error) {
	if opts == nil {
		opts = DefaultImportOptions()
	}

	// 读取文件内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// 检查文件大小
	if opts.MaxFileSize > 0 && int64(len(content)) > opts.MaxFileSize {
		return nil, fmt.Errorf("file %s exceeds max size %d bytes", filePath, opts.MaxFileSize)
	}

	// 不再进行自动推断：如果未指定，保持为空
	language := opts.Language
	category := opts.Category

	// 创建元数据
	metadata := NewDocumentMetadata(filePath, language, category)
	if opts.Source != "" {
		metadata.Source = opts.Source
	}

	// 尝试从 Markdown 内容提取标题
	title := extractTitleFromMarkdown(string(content))
	if title != "" {
		metadata.Title = title
	}

	// 使用 SplitMarkdown 分割文档
	chunks, err := mi.manager.SplitterService.SplitMarkdown(ctx, string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to split markdown file %s: %w", filePath, err)
	}

	// 为每个块添加元数据和唯一 ID
	enrichedChunks := EnrichDocumentsWithMetadata(chunks, metadata)
	for i, chunk := range enrichedChunks {
		if chunk.ID == "" {
			// 生成唯一 ID：语言类型 + 文档分类 + 时间戳 + 块索引
			chunk.ID = generateChunkID(metadata.Language, metadata.Category, i)
		}
	}

	// 存储到 Milvus
	docIDs, err := mi.manager.IndexerService.Store(ctx, enrichedChunks)
	if err != nil {
		return nil, fmt.Errorf("failed to store documents to Milvus: %w", err)
	}

	return &ImportResult{
		TotalFiles:   1,
		SuccessFiles: 1,
		FailedFiles:  0,
		TotalChunks:  len(enrichedChunks),
		DocumentIDs:  docIDs,
		Errors:       nil,
	}, nil
}

// ImportDirectory 导入目录中的所有 Markdown 文件
func (mi *MarkdownImporter) ImportDirectory(ctx context.Context, dirPath string, opts *ImportOptions) (*ImportResult, error) {
	if opts == nil {
		opts = DefaultImportOptions()
	}

	result := &ImportResult{
		DocumentIDs: make([]string, 0),
		Errors:      make([]error, 0),
	}

	// 遍历目录
	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("error accessing path %s: %w", path, err))
			return nil // 继续处理其他文件
		}

		// 跳过目录
		if d.IsDir() {
			// 如果不递归，跳过子目录
			if !opts.Recursive && path != dirPath {
				return fs.SkipDir
			}
			// 跳过隐藏目录
			if opts.SkipHidden && strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}

		// 检查文件扩展名
		if !hasValidExtension(path, opts.Extensions) {
			return nil
		}

		// 跳过隐藏文件
		if opts.SkipHidden && strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		result.TotalFiles++

		// 导入文件
		fileResult, err := mi.ImportFile(ctx, path, opts)
		if err != nil {
			result.FailedFiles++
			result.Errors = append(result.Errors, fmt.Errorf("failed to import file %s: %w", path, err))
			return nil // 继续处理其他文件
		}

		result.SuccessFiles++
		result.TotalChunks += fileResult.TotalChunks
		result.DocumentIDs = append(result.DocumentIDs, fileResult.DocumentIDs...)

		return nil
	})

	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("error walking directory %s: %w", dirPath, err))
	}

	return result, nil
}

// Import 导入文件或目录（自动判断）
func (mi *MarkdownImporter) Import(ctx context.Context, path string, opts *ImportOptions) (*ImportResult, error) {
	if opts == nil {
		opts = DefaultImportOptions()
	}

	// 检查路径是否存在
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("path %s does not exist: %w", path, err)
	}

	if info.IsDir() {
		return mi.ImportDirectory(ctx, path, opts)
	}

	return mi.ImportFile(ctx, path, opts)
}

// BatchImport 批量导入多个文件或目录
func (mi *MarkdownImporter) BatchImport(ctx context.Context, paths []string, opts *ImportOptions) (*ImportResult, error) {
	if opts == nil {
		opts = DefaultImportOptions()
	}

	result := &ImportResult{
		DocumentIDs: make([]string, 0),
		Errors:      make([]error, 0),
	}

	for _, path := range paths {
		pathResult, err := mi.Import(ctx, path, opts)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("failed to import %s: %w", path, err))
			result.FailedFiles += pathResult.FailedFiles
			continue
		}

		result.TotalFiles += pathResult.TotalFiles
		result.SuccessFiles += pathResult.SuccessFiles
		result.FailedFiles += pathResult.FailedFiles
		result.TotalChunks += pathResult.TotalChunks
		result.DocumentIDs = append(result.DocumentIDs, pathResult.DocumentIDs...)
		result.Errors = append(result.Errors, pathResult.Errors...)
	}

	return result, nil
}

// GetManager 获取 Milvus 管理器
func (mi *MarkdownImporter) GetManager() *MilvusManager {
	return mi.manager
}

// extractTitleFromMarkdown 从 Markdown 内容中提取标题
// 优先提取第一个一级标题，如果没有则提取第一个二级标题
func extractTitleFromMarkdown(content string) string {
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 检查一级标题
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}

	// 如果没有一级标题，查找二级标题
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "## "))
		}
	}

	return ""
}

// hasValidExtension 检查文件是否有有效的扩展名
func hasValidExtension(filePath string, extensions []string) bool {
	if len(extensions) == 0 {
		return true // 如果没有指定扩展名，接受所有文件
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	for _, validExt := range extensions {
		if ext == strings.ToLower(validExt) {
			return true
		}
	}

	return false
}

// generateChunkID 生成文档块的唯一 ID：语言类型_文档分类_时间戳_块索引
func generateChunkID(language DocumentLanguage, category DocumentCategory, chunkIndex int) string {
	timestamp := time.Now().Unix()
	// 如果未提供语言和分类，则直接返回时间戳
	if string(language) == "" && string(category) == "" {
		return fmt.Sprintf("%d", timestamp)
	}
	return fmt.Sprintf("%s_%s_%d_%d", string(language), string(category), timestamp, chunkIndex)
}

// ============================================================================
// 以下是新增的纯文本导入方法，支持从内存中的字符串直接导入到 Milvus
// 适用于飞书文档等已经转换为纯文本的场景
// ============================================================================

// TextImportOptions 纯文本导入选项
// 用于从内存中的字符串导入，不需要文件路径
type TextImportOptions struct {
	// 文档标题（可选，用于元数据）
	Title string
	// 语言类型：golang|java|middleware
	Language DocumentLanguage
	// 文档分类：basic|specialized|comprehensive
	Category DocumentCategory
	// 来源描述（如：飞书文档、API等）
	Source string
	// 最大文件大小（字节，0 表示无限制）
	MaxSize int64
}

// DefaultTextImportOptions 返回默认的纯文本导入选项
func DefaultTextImportOptions() *TextImportOptions {
	return &TextImportOptions{
		Title:    "未命名文档",
		Language: "", // 空表示不指定
		Category: "", // 空表示不指定
		Source:   "text",
		MaxSize:  10 * 1024 * 1024, // 10MB
	}
}

// ImportText 导入纯文本字符串到 Milvus
// 这是核心方法，用于将已经转换好的纯文本（如飞书文档的 Markdown 输出）导入到 Milvus
//
// 参数说明：
//   - ctx: 上下文，用于控制超时和取消
//   - content: 要导入的纯文本内容（Markdown 格式效果最佳）
//   - opts: 导入选项，包含标题、语言、分类等元数据
//
// 返回值：
//   - *ImportResult: 导入结果，包含成功/失败信息和文档ID列表
//   - error: 错误信息
//
// 使用示例：
//
//	```go
//	// 1. 初始化 MilvusManager
//	manager, _ := milvus.InitMilvusManager(ctx, cfg)
//	// 2. 创建导入器
//	importer, _ := milvus.NewMarkdownImporter(manager)
//	// 3. 准备导入选项
//	opts := &milvus.TextImportOptions{
//	    Title:    "Go 并发编程",
//	    Language: milvus.LanguageGolang,
//	    Category: milvus.CategorySpecialized,
//	    Source:   "feishu",
//	}
//	// 4. 导入文本
//	result, _ := importer.ImportText(ctx, markdownContent, opts)
//	```
func (mi *MarkdownImporter) ImportText(ctx context.Context, content string, opts *TextImportOptions) (*ImportResult, error) {
	// 使用默认选项（如果未提供）
	if opts == nil {
		opts = DefaultTextImportOptions()
	}

	// 检查内容是否为空
	if content == "" {
		return nil, fmt.Errorf("content is empty, nothing to import")
	}

	// 检查内容大小
	if opts.MaxSize > 0 && int64(len(content)) > opts.MaxSize {
		return nil, fmt.Errorf("content size %d bytes exceeds max size %d bytes", len(content), opts.MaxSize)
	}

	// 创建元数据（使用 "memory" 作为虚拟文件路径，表示来自内存）
	metadata := NewDocumentMetadata("memory://"+opts.Title, opts.Language, opts.Category)
	metadata.Title = opts.Title
	metadata.Source = opts.Source
	metadata.FilePath = "" // 清空文件路径，因为是纯文本导入
	metadata.FileName = opts.Title

	// 尝试从 Markdown 内容中提取标题（如果用户没有提供标题）
	if opts.Title == "" || opts.Title == "未命名文档" {
		extractedTitle := extractTitleFromMarkdown(content)
		if extractedTitle != "" {
			metadata.Title = extractedTitle
		}
	}

	// ========================================
	// 核心步骤1：使用 SplitMarkdown 切割文档
	// ========================================
	// SplitMarkdown 会根据 Markdown 的语义结构（标题、段落、代码块等）
	// 将长文本切割成适合向量检索的小块
	chunks, err := mi.manager.SplitterService.SplitMarkdown(ctx, content)
	if err != nil {
		return nil, fmt.Errorf("failed to split text content: %w", err)
	}

	// 为每个块添加元数据和唯一 ID
	// 元数据包含：语言、分类、标题、来源等信息，方便后续检索和过滤
	enrichedChunks := EnrichDocumentsWithMetadata(chunks, metadata)
	for i, chunk := range enrichedChunks {
		if chunk.ID == "" {
			// 生成唯一 ID：语言类型_文档分类_时间戳_块索引
			chunk.ID = generateChunkID(metadata.Language, metadata.Category, i)
		}
	}

	// ========================================
	// 核心步骤2：使用 Store 上传到 Milvus
	// ========================================
	// Store 会：1) 计算每个块的向量嵌入 2) 存储到 Milvus 数据库
	docIDs, err := mi.manager.IndexerService.Store(ctx, enrichedChunks)
	if err != nil {
		return nil, fmt.Errorf("failed to store documents to Milvus: %w", err)
	}

	// 返回导入结果
	return &ImportResult{
		TotalFiles:   1, // 虽然不是文件，但计为1个文档
		SuccessFiles: 1,
		FailedFiles:  0,
		TotalChunks:  len(enrichedChunks), // 切片后的块数
		DocumentIDs:  docIDs,              // Milvus 返回的文档ID列表
		Errors:       nil,
	}, nil
}

// ImportTexts 批量导入多个纯文本字符串到 Milvus
// 适用于需要一次性导入多个文档的场景
//
// 参数说明：
//   - ctx: 上下文
//   - contents: 文本内容数组，每个元素是一个独立的文档
//   - opts: 共享的导入选项（每个文档会共享相同的 Language、Category、Source）
//
// 注意：每个文档会尝试从内容中提取标题，如果提取失败则使用 "文档_序号" 作为标题
func (mi *MarkdownImporter) ImportTexts(ctx context.Context, contents []string, opts *TextImportOptions) (*ImportResult, error) {
	if opts == nil {
		opts = DefaultTextImportOptions()
	}

	if len(contents) == 0 {
		return nil, fmt.Errorf("contents array is empty, nothing to import")
	}

	// 汇总结果
	result := &ImportResult{
		DocumentIDs: make([]string, 0),
		Errors:      make([]error, 0),
	}

	// 逐个导入
	for i, content := range contents {
		// 为每个文档创建独立的选项副本
		docOpts := &TextImportOptions{
			Title:    fmt.Sprintf("文档_%d", i+1), // 默认标题
			Language: opts.Language,
			Category: opts.Category,
			Source:   opts.Source,
			MaxSize:  opts.MaxSize,
		}

		// 尝试从内容提取标题
		extractedTitle := extractTitleFromMarkdown(content)
		if extractedTitle != "" {
			docOpts.Title = extractedTitle
		}

		result.TotalFiles++

		// 导入单个文档
		singleResult, err := mi.ImportText(ctx, content, docOpts)
		if err != nil {
			result.FailedFiles++
			result.Errors = append(result.Errors, fmt.Errorf("failed to import text %d: %w", i+1, err))
			continue
		}

		result.SuccessFiles++
		result.TotalChunks += singleResult.TotalChunks
		result.DocumentIDs = append(result.DocumentIDs, singleResult.DocumentIDs...)
	}

	return result, nil
}
