package retrieval

import (
	"context"

	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// FloatVectorConverter 将 float64 向量转换为 FloatVector
// 这个转换器与 indexer 中使用的向量类型保持一致
func FloatVectorConverter(ctx context.Context, vectors [][]float64) ([]entity.Vector, error) {
	result := make([]entity.Vector, 0, len(vectors))
	for _, vector := range vectors {
		float32Vec := make([]float32, len(vector))
		for i, v := range vector {
			float32Vec[i] = float32(v)
		}
		result = append(result, entity.FloatVector(float32Vec))
	}
	return result, nil
}
