package retrieval

import (
	"context"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	milvusRetriever "github.com/cloudwego/eino-ext/components/retriever/milvus"
	"github.com/cloudwego/eino/schema"
	milvusClient "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// SearchMetrics 检索各阶段耗时统计
type SearchMetrics struct {
	EmbeddingMs   int64
	SearchMs      int64
	PostprocessMs int64
	HitCount      int
	TruncatedCount int
}

// SearchResult 带指标的检索结果
type SearchResult struct {
	Documents []*schema.Document
	Metrics   SearchMetrics
}

// SearchWithExpr 使用表达式过滤进行检索
func SearchWithExpr(
	ctx context.Context,
	client milvusClient.Client,
	config *milvusRetriever.RetrieverConfig,
	query string,
	expr string,
	opts *RetrieveOptions,
) ([]*schema.Document, error) {
	result, err := SearchWithExprAndMetrics(ctx, client, config, query, expr, opts)
	if err != nil {
		return nil, err
	}
	return result.Documents, nil
}

// SearchWithExprAndMetrics 使用表达式过滤进行检索，返回带耗时指标的结果
func SearchWithExprAndMetrics(
	ctx context.Context,
	client milvusClient.Client,
	config *milvusRetriever.RetrieverConfig,
	query string,
	expr string,
	opts *RetrieveOptions,
) (*SearchResult, error) {
	// 获取 embedding
	embedder := config.Embedding
	if embedder == nil {
		return nil, fmt.Errorf("embedding is nil")
	}

	// 将查询文本转换为向量
	embedStart := time.Now()
	vectors, err := embedder.EmbedStrings(ctx, []string{query})
	embeddingMs := time.Since(embedStart).Milliseconds()
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("empty embedding result")
	}

	// 转换为 float32 向量
	queryVector := make([]float32, len(vectors[0]))
	for i, v := range vectors[0] {
		queryVector[i] = float32(v)
	}

	// 确定 TopK
	topK := opts.TopK
	if topK <= 0 {
		topK = config.TopK
		if topK <= 0 {
			topK = 10
		}
	}

	// 确定使用的集合名称（优先使用 opts 中指定的集合）
	collectionName := config.Collection
	if opts.Collection != "" {
		collectionName = opts.Collection
	}

	// 构建搜索参数
	searchParam, err := entity.NewIndexAUTOINDEXSearchParam(1)
	if err != nil {
		return nil, fmt.Errorf("failed to create search param: %w", err)
	}

	// 执行搜索
	searchStart := time.Now()
	searchResults, err := client.Search(
		ctx,
		collectionName,
		[]string{},                            // partitions
		expr,                                  // 过滤表达式
		[]string{"id", "content", "metadata"}, // output fields
		[]entity.Vector{entity.FloatVector(queryVector)},
		config.VectorField,
		config.MetricType,
		topK,
		searchParam,
	)
	searchMs := time.Since(searchStart).Milliseconds()
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	// 转换搜索结果
	postprocessStart := time.Now()
	documents := make([]*schema.Document, 0)
	rawHitCount := 0
	if len(searchResults) > 0 {
		result := searchResults[0]
		// 解析字段数据
		contentField := result.Fields.GetColumn("content")
		metadataField := result.Fields.GetColumn("metadata")
		idField := result.IDs

		// 获取结果数量
		rawHitCount = idField.Len()
		for i := 0; i < rawHitCount; i++ {
			// 获取 ID
			idValue, err := idField.GetAsString(i)
			if err != nil {
				continue
			}

			doc := &schema.Document{
				ID:       idValue,
				Content:  "",
				MetaData: make(map[string]interface{}),
			}

			// 提取 content
			if contentField != nil {
				if content, err := contentField.GetAsString(i); err == nil {
					doc.Content = content
				}
			}

			// 提取 metadata (JSON 字段)
			if metadataField != nil {
				// JSON 字段需要特殊处理
				// 尝试作为字符串获取（Milvus JSON 字段通常以字符串形式返回）
				if jsonStr, err := metadataField.GetAsString(i); err == nil && jsonStr != "" {
					// 解析 JSON metadata
					var metadata map[string]interface{}
					if err := sonic.Unmarshal([]byte(jsonStr), &metadata); err == nil {
						doc.MetaData = metadata
					}
				}
			}

			// 添加相似度分数
			if i < len(result.Scores) {
				doc.MetaData["score"] = result.Scores[i]
			}

			documents = append(documents, doc)
		}
	}
	postprocessMs := time.Since(postprocessStart).Milliseconds()

	return &SearchResult{
		Documents: documents,
		Metrics: SearchMetrics{
			EmbeddingMs:    embeddingMs,
			SearchMs:       searchMs,
			PostprocessMs:  postprocessMs,
			HitCount:       rawHitCount,
			TruncatedCount: rawHitCount - len(documents),
		},
	}, nil
}
