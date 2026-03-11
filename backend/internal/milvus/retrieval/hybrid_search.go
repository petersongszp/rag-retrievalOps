package retrieval

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/schema"
)

// HybridRetriever 混合检索器服务
// 结合了向量检索和重排序功能
type HybridRetriever struct {
	retriever *RetrieverService
	reranker  Reranker
	config    *HybridRetrieverConfig
}

// HybridRetrieverConfig 混合检索配置
type HybridRetrieverConfig struct {
	// CandidateTopK 向量检索阶段召回的候选文档数量
	// 通常该值应大于最终需要的 TopK，以便重排序器有发挥空间
	CandidateTopK int
	// RerankerImpl 重排序器实现，如果不提供则默认使用 JaccardReranker
	RerankerImpl Reranker
}

// NewHybridRetriever 创建新的混合检索器
func NewHybridRetriever(retriever *RetrieverService, config *HybridRetrieverConfig) (*HybridRetriever, error) {
	if retriever == nil {
		return nil, fmt.Errorf("retriever service is nil")
	}

	if config == nil {
		config = &HybridRetrieverConfig{
			CandidateTopK: 10,
		}
	}
	if config.CandidateTopK <= 0 {
		config.CandidateTopK = 10
	}

	hr := &HybridRetriever{
		retriever: retriever,
		config:    config,
	}

	if config.RerankerImpl != nil {
		hr.reranker = config.RerankerImpl
	} else {
		// 默认使用 JaccardReranker
		hr.reranker = NewJaccardReranker(nil)
	}

	return hr, nil
}

// Search 执行混合检索
func (s *HybridRetriever) Search(ctx context.Context, query string, opts *RetrieveOptions) ([]*schema.Document, error) {
	if query == "" {
		return nil, fmt.Errorf("query is empty")
	}

	// 1. 向量检索阶段 (Vector Retrievel)
	// 使用配置的 CandidateTopK 覆盖 opts 中的 TopK (或者如果 opts 未指定)
	retrieveOpts := &RetrieveOptions{}
	if opts != nil {
		*retrieveOpts = *opts
	}
	// 强制使用 CandidateTopK 进行召回
	retrieveOpts.TopK = s.config.CandidateTopK

	candidates, err := s.retriever.RetrieveWithOptions(ctx, query, retrieveOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve candidates: %w", err)
	}

	if len(candidates) == 0 {
		return []*schema.Document{}, nil
	}

	// 2. 重排序阶段 (Reranking)
	rerankedDocs, err := s.reranker.Rerank(ctx, query, candidates)
	if err != nil {
		return nil, fmt.Errorf("failed to rerank documents: %w", err)
	}

	// Reranker 已经处理了 TopK 截断 (基于 RerankerConfig 的 TopK)
	// 如果需要动态调整最终返回数量，可以在 Reranker 接口中传递更多参数，目前简化处理

	// 注意：如果 opts 指定了较小的 TopK，我们可以在这里再次截断
	finalTopK := 5 // 默认
	if opts != nil && opts.TopK > 0 {
		finalTopK = opts.TopK
	}

	if len(rerankedDocs) > finalTopK {
		rerankedDocs = rerankedDocs[:finalTopK]
	}

	return rerankedDocs, nil
}
