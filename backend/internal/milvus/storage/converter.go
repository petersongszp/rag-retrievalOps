package storage

import (
	"context"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
)

// FloatVectorSchema 定义浮点向量的数据结构（相当于表的结构）
type FloatVectorSchema struct {
	ID       string    `json:"id" milvus:"name:id"`
	Content  string    `json:"content" milvus:"name:content"`
	Vector   []float32 `json:"vector" milvus:"name:vector"`
	Metadata []byte    `json:"metadata" milvus:"name:metadata"`
}

// FloatVectorDocumentConverter 将 schema.Document 转换为浮点向量格式
func FloatVectorDocumentConverter(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]interface{}, error) {
	if len(docs) != len(vectors) {
		return nil, fmt.Errorf("docs and vectors length mismatch: %d != %d", len(docs), len(vectors))
	}
	rows := make([]interface{}, 0, len(docs))
	for idx, doc := range docs {
		// 序列化 metadata
		metadata, err := sonic.Marshal(doc.MetaData)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
		vector := make([]float32, len(vectors[idx]))
		for i, v := range vectors[idx] {
			vector[i] = float32(v)
		}
		row := &FloatVectorSchema{
			ID:       doc.ID,
			Content:  doc.Content,
			Vector:   vector,
			Metadata: metadata,
		}
		rows = append(rows, row)
	}

	return rows, nil
}
