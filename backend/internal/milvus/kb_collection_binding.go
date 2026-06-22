package milvus

import (
	"context"
	"fmt"
	"strings"

	milvusIndexer "github.com/cloudwego/eino-ext/components/indexer/milvus"

	"interview-agents/internal/milvus/storage"
)

func DefaultKnowledgeBaseCollectionName(kbID uint64) string {
	if kbID == 0 {
		return "kb_unknown_docs"
	}
	return fmt.Sprintf("kb_%d_docs", kbID)
}

func (m *MilvusManager) NewIndexerServiceForCollection(ctx context.Context, collection string) (*storage.IndexerService, error) {
	if m == nil {
		return nil, fmt.Errorf("milvus manager is nil")
	}
	if strings.TrimSpace(collection) == "" {
		return nil, fmt.Errorf("collection name is empty")
	}
	if m.Client == nil {
		return nil, fmt.Errorf("milvus client is nil")
	}
	if m.EmbeddingService == nil {
		return nil, fmt.Errorf("embedding service is nil")
	}
	if m.Config == nil {
		return nil, fmt.Errorf("milvus config is nil")
	}

	indexerConfig := &milvusIndexer.IndexerConfig{
		Client:     m.Client,
		Collection: strings.TrimSpace(collection),
		Embedding:  m.EmbeddingService.GetEmbedder(),
	}
	return storage.NewIndexerServiceWithDimension(ctx, indexerConfig, m.Config.Embedding.Dimensions, &m.Config.DocumentSplitter)
}
