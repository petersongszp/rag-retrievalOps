package retrieval

import (
	"context"
	"sort"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// Reranker 重排序器接口
type Reranker interface {
	Rerank(ctx context.Context, query string, docs []*schema.Document) ([]*schema.Document, error)
}

// JaccardRerankerConfig 配置 Jaccard 重排序器
type JaccardRerankerConfig struct {
	// OriginalScoreWeight 原始向量分数的权重 (0.0-1.0)
	// Jaccard 分数权重为 1.0 - OriginalScoreWeight
	OriginalScoreWeight float64
	// TopK 重排序后返回的文档数量
	TopK int
}

// JaccardReranker 基于 Jaccard 相似度的重排序器
type JaccardReranker struct {
	config *JaccardRerankerConfig
}

// NewJaccardReranker 创建新的 Jaccard 重排序器
func NewJaccardReranker(config *JaccardRerankerConfig) *JaccardReranker {
	if config == nil {
		config = &JaccardRerankerConfig{
			OriginalScoreWeight: 0.7, // 默认 70% 依赖向量分数
			TopK:                5,
		}
	}
	if config.OriginalScoreWeight < 0 || config.OriginalScoreWeight > 1 {
		config.OriginalScoreWeight = 0.7
	}
	if config.TopK <= 0 {
		config.TopK = 5
	}
	return &JaccardReranker{config: config}
}

// Rerank 对文档进行重排序
func (r *JaccardReranker) Rerank(ctx context.Context, query string, docs []*schema.Document) ([]*schema.Document, error) {
	if len(docs) == 0 {
		return []*schema.Document{}, nil
	}

	// 简单的分词 (按空格分割)
	queryTokens := tokenize(query)

	type scoredDoc struct {
		doc   *schema.Document
		score float64
	}

	scoredDocs := make([]scoredDoc, 0, len(docs))

	for _, doc := range docs {
		// 计算 Jaccard 相似度
		contentTokens := tokenize(doc.Content)
		jaccardScore := calculateJaccardSimilarity(queryTokens, contentTokens)

		// 获取原始分数 (假设存储在 metadata "score" 中，如果没有则默认为 0.5)
		originalScore := 0.5
		if scoreVal, ok := doc.MetaData["score"]; ok {
			switch v := scoreVal.(type) {
			case float64:
				originalScore = v
			case float32:
				originalScore = float64(v)
			}
		}

		// 融合分数
		finalScore := (originalScore * r.config.OriginalScoreWeight) + (jaccardScore * (1 - r.config.OriginalScoreWeight))

		// 更新文档分数
		doc.MetaData["rerank_score"] = finalScore

		scoredDocs = append(scoredDocs, scoredDoc{
			doc:   doc,
			score: finalScore,
		})
	}

	// 排序
	sort.Slice(scoredDocs, func(i, j int) bool {
		return scoredDocs[i].score > scoredDocs[j].score
	})

	// 截取 TopK
	resultTopK := r.config.TopK
	// 如果配置的 TopK 大于实际文档数，则返回所有文档
	if resultTopK > len(scoredDocs) {
		resultTopK = len(scoredDocs)
	}

	resultDocs := make([]*schema.Document, 0, resultTopK)
	for i := 0; i < resultTopK; i++ {
		resultDocs = append(resultDocs, scoredDocs[i].doc)
	}

	return resultDocs, nil
}

// tokenize 简单的分词函数
func tokenize(text string) map[string]struct{} {
	tokens := make(map[string]struct{})
	// 转小写并按空格分割
	parts := strings.Fields(strings.ToLower(text))
	for _, p := range parts {
		// 简单的清理
		p = strings.Trim(p, ".,!?-;\"'()[]{}")
		if len(p) > 0 {
			tokens[p] = struct{}{}
		}
	}
	return tokens
}

// calculateJaccardSimilarity 计算 Jaccard 相似度
func calculateJaccardSimilarity(set1, set2 map[string]struct{}) float64 {
	intersection := 0
	union := len(set1) + len(set2)

	for k := range set1 {
		if _, exists := set2[k]; exists {
			intersection++
		}
	}

	// Union = |A| + |B| - |A ∩ B|
	union -= intersection

	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}
