package splitter

import (
	"context"
	"fmt"

	"interview-agents/internal/milvus/chunkmeta"

	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
)

//参考文档 https://www.cloudwego.io/zh/docs/eino/ecosystem_integration/document/splitter_recursive/

// DocumentSplitterService 封装文档分割器服务
type DocumentSplitterService struct {
	config         *recursive.Config
	splitter       document.Transformer
	semanticConfig SemanticSplitConfig
}

type SemanticSplitConfig struct {
	Enabled              bool
	MinBlockSize         int
	TargetChunkSize      int
	MaxChunkSize         int
	BreakpointPercentile int
	MinSentencesPerChunk int
	Embedder             embedding.Embedder
}

// NewDocumentSplitterService 创建新的文档分割器服务
func NewDocumentSplitterService(ctx context.Context, config *recursive.Config) (*DocumentSplitterService, error) {
	if config == nil {
		return nil, fmt.Errorf("config is nil")
	}
	// 设置默认值
	if config.ChunkSize <= 0 {
		config.ChunkSize = 1000
	}
	if config.OverlapSize < 0 {
		config.OverlapSize = 200
	}
	if len(config.Separators) == 0 {
		// 中英文混合的默认分隔符
		config.Separators = []string{"\n\n", "\n", "。", "！", "？", ". ", "! ", "? "}
	}
	// 创建分割器
	splitter, err := recursive.NewSplitter(ctx, &recursive.Config{
		ChunkSize:   config.ChunkSize,
		OverlapSize: config.OverlapSize,
		Separators:  config.Separators,
		KeepType:    config.KeepType,
		LenFunc:     nil, // 使用默认的长度计算函数
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create document splitter: %w", err)
	}
	return &DocumentSplitterService{
		config:   config,
		splitter: splitter,
	}, nil
}

func (s *DocumentSplitterService) ConfigureSemanticSplit(cfg SemanticSplitConfig) {
	if s == nil {
		return
	}
	if cfg.MinBlockSize <= 0 {
		cfg.MinBlockSize = 1200
	}
	if cfg.TargetChunkSize <= 0 {
		cfg.TargetChunkSize = 1000
	}
	if cfg.MaxChunkSize <= 0 {
		cfg.MaxChunkSize = 1400
	}
	if cfg.BreakpointPercentile <= 0 {
		cfg.BreakpointPercentile = 20
	}
	if cfg.MinSentencesPerChunk <= 0 {
		cfg.MinSentencesPerChunk = 2
	}
	s.semanticConfig = cfg
}

// Split 分割文档
func (s *DocumentSplitterService) Split(ctx context.Context, docs []*schema.Document) ([]*schema.Document, error) {
	if len(docs) == 0 {
		return nil, fmt.Errorf("docs is empty")
	}

	results := make([]*schema.Document, 0)
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		splitResults, err := s.splitter.Transform(ctx, []*schema.Document{doc})
		if err != nil {
			return nil, fmt.Errorf("failed to split documents: %w", err)
		}
		splitResults = s.applySemanticSecondarySplit(ctx, splitResults)
		results = append(results, s.annotateSplitChunks(doc, splitResults, chunkmeta.SplitStrategyRecursiveV1)...)
	}

	return results, nil
}

// SplitText 分割单个文本内容
func (s *DocumentSplitterService) SplitText(ctx context.Context, text string) ([]*schema.Document, error) {
	if text == "" {
		return nil, fmt.Errorf("text is empty")
	}

	doc := &schema.Document{
		Content: text,
	}

	return s.Split(ctx, []*schema.Document{doc})
}

// GetConfig 获取配置信息
func (s *DocumentSplitterService) GetConfig() *recursive.Config {
	return s.config
}
