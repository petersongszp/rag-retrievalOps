package splitter

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
	"github.com/cloudwego/eino/schema"
)

// SplitMarkdown 专门用于切割 Markdown 文档的方法
// 使用适合 Markdown 结构的分隔符，保持文档的语义完整性
func (s *DocumentSplitterService) SplitMarkdown(ctx context.Context, markdownContent string) ([]*schema.Document, error) {
	if markdownContent == "" {
		return nil, fmt.Errorf("markdown content is empty")
	}

	// 创建专门用于 Markdown 的配置
	// 使用适合 Markdown 的分隔符，优先级从高到低
	markdownSeparators := []string{
		"\n\n\n",    // 多个空行（章节分隔）
		"\n## ",     // 二级标题
		"\n### ",    // 三级标题
		"\n#### ",   // 四级标题
		"\n##### ",  // 五级标题
		"\n###### ", // 六级标题
		"\n# ",      // 一级标题（放在后面，避免误匹配）
		"\n\n",      // 段落分隔
		"\n```",     // 代码块开始
		"\n---",     // 水平分割线
		"\n***",     // 水平分割线
		"\n- ",      // 无序列表
		"\n* ",      // 无序列表
		"\n1. ",     // 有序列表
		"\n2. ",     // 有序列表
		"\n3. ",     // 有序列表
		"\n",        // 单行分隔
		"。",         // 中文句号
		"！",         // 中文感叹号
		"？",         // 中文问号
		". ",        // 英文句号
		"! ",        // 英文感叹号
		"? ",        // 英文问号
	}

	// 创建临时的 Markdown 分割器配置
	markdownConfig := &recursive.Config{
		ChunkSize:   s.config.ChunkSize,   // 使用原有配置的块大小
		OverlapSize: s.config.OverlapSize, // 使用原有配置的重叠大小
		Separators:  markdownSeparators,   // 使用 Markdown 专用分隔符
		KeepType:    s.config.KeepType,    // 保持原有类型设置
	}

	// 创建临时的 Markdown 分割器
	markdownSplitter, err := recursive.NewSplitter(ctx, markdownConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create markdown splitter: %w", err)
	}

	// 创建文档对象
	doc := &schema.Document{
		Content: markdownContent,
	}

	// 执行分割
	results, err := markdownSplitter.Transform(ctx, []*schema.Document{doc})
	if err != nil {
		return nil, fmt.Errorf("failed to split markdown document: %w", err)
	}

	return results, nil
}

// SplitMarkdownDocuments 批量切割多个 Markdown 文档
func (s *DocumentSplitterService) SplitMarkdownDocuments(ctx context.Context, docs []*schema.Document) ([]*schema.Document, error) {
	if len(docs) == 0 {
		return nil, fmt.Errorf("docs is empty")
	}

	// Markdown 专用分隔符
	markdownSeparators := []string{
		"\n\n\n",
		"\n## ",
		"\n### ",
		"\n#### ",
		"\n##### ",
		"\n###### ",
		"\n# ",
		"\n\n",
		"\n```",
		"\n---",
		"\n***",
		"\n- ",
		"\n* ",
		"\n1. ",
		"\n2. ",
		"\n3. ",
		"\n",
		"。",
		"！",
		"？",
		". ",
		"! ",
		"? ",
	}

	// 创建临时的 Markdown 分割器配置
	markdownConfig := &recursive.Config{
		ChunkSize:   s.config.ChunkSize,
		OverlapSize: s.config.OverlapSize,
		Separators:  markdownSeparators,
		KeepType:    s.config.KeepType,
	}

	// 创建临时的 Markdown 分割器
	markdownSplitter, err := recursive.NewSplitter(ctx, markdownConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create markdown splitter: %w", err)
	}

	// 执行分割
	results, err := markdownSplitter.Transform(ctx, docs)
	if err != nil {
		return nil, fmt.Errorf("failed to split markdown documents: %w", err)
	}

	return results, nil
}
