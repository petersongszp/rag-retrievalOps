package retrieval

import (
	"context"
	"fmt"

	"github.com/bytedance/sonic"
	milvusRetriever "github.com/cloudwego/eino-ext/components/retriever/milvus"
	"github.com/cloudwego/eino/schema"
	milvusClient "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// SearchWithExpr 使用表达式过滤进行检索
func SearchWithExpr(
	ctx context.Context,
	client milvusClient.Client,
	config *milvusRetriever.RetrieverConfig,
	query string,
	expr string,
	opts *RetrieveOptions,
) ([]*schema.Document, error) {
	// 获取 embedding
	embedder := config.Embedding
	if embedder == nil {
		return nil, fmt.Errorf("embedding is nil")
	}

	// 将查询文本转换为向量
	vectors, err := embedder.EmbedStrings(ctx, []string{query})
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
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	// 转换搜索结果
	documents := make([]*schema.Document, 0)
	if len(searchResults) > 0 {
		result := searchResults[0]
		// 解析字段数据
		contentField := result.Fields.GetColumn("content")
		metadataField := result.Fields.GetColumn("metadata")
		idField := result.IDs

		// 获取结果数量
		resultCount := idField.Len()
		for i := 0; i < resultCount; i++ {
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

	return documents, nil
}
