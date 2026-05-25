package benchmark

import (
	"context"
	"fmt"

	milvusRetriever "github.com/cloudwego/eino-ext/components/retriever/milvus"
	milvusClient "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"

	"interview-agents/internal/milvus/retrieval"
)

type MilvusProfileHarness struct {
	Client        milvusClient.Client
	Collection    string
	VectorField   string
	IndexName     string
	BaseRetriever *milvusRetriever.RetrieverConfig
}

func (h *MilvusProfileHarness) ApplyProfile(ctx context.Context, profile IndexProfile) error {
	if h.Client == nil {
		return fmt.Errorf("milvus client is nil")
	}
	if err := ValidateProfile(profile); err != nil {
		return err
	}
	index, err := buildMilvusIndex(profile)
	if err != nil {
		return err
	}
	_ = h.Client.ReleaseCollection(ctx, h.Collection)
	_ = h.Client.DropIndex(ctx, h.Collection, h.vectorField(), milvusClient.WithIndexName(h.indexName()))
	if err := h.Client.CreateIndex(ctx, h.Collection, h.vectorField(), index, false, milvusClient.WithIndexName(h.indexName())); err != nil {
		return fmt.Errorf("create index for %s: %w", profile.Name, err)
	}
	if err := h.Client.LoadCollection(ctx, h.Collection, false); err != nil {
		return fmt.Errorf("load collection after %s: %w", profile.Name, err)
	}
	return nil
}

func (h *MilvusProfileHarness) SearcherFactory() SearcherFactory {
	return func(profile IndexProfile) (Searcher, error) {
		if h.BaseRetriever == nil {
			return nil, fmt.Errorf("base retriever config is nil")
		}
		searchParam, err := buildSearchParam(profile)
		if err != nil {
			return nil, err
		}
		cloned := *h.BaseRetriever
		cloned.Collection = h.Collection
		cloned.Sp = searchParam
		cloned.MetricType = entity.MetricType(profile.MetricType)
		service, err := retrieval.NewRetrieverService(context.Background(), &cloned)
		if err != nil {
			return nil, err
		}
		return &denseSearcher{retriever: service, collection: h.Collection}, nil
	}
}

type denseSearcher struct {
	retriever  *retrieval.RetrieverService
	collection string
}

func (s *denseSearcher) Search(ctx context.Context, query string, topK int) ([]string, error) {
	docs, err := s.retriever.RetrieveWithOptions(ctx, query, &retrieval.RetrieveOptions{
		TopK:       topK,
		Collection: s.collection,
	})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(docs))
	for _, doc := range docs {
		ids = append(ids, doc.ID)
	}
	return ids, nil
}

func buildMilvusIndex(profile IndexProfile) (entity.Index, error) {
	metric := entity.MetricType(profile.MetricType)
	switch profile.Family {
	case IndexFamilyHNSW:
		return entity.NewIndexHNSW(metric, profile.HNSW.M, profile.HNSW.EfConstruction)
	case IndexFamilyIVF:
		return entity.NewIndexIvfFlat(metric, profile.IVF.NList)
	default:
		return nil, fmt.Errorf("unsupported profile family %s", profile.Family)
	}
}

func buildSearchParam(profile IndexProfile) (entity.SearchParam, error) {
	switch profile.Family {
	case IndexFamilyHNSW:
		return entity.NewIndexHNSWSearchParam(profile.HNSW.EfSearch)
	case IndexFamilyIVF:
		return entity.NewIndexIvfFlatSearchParam(profile.IVF.NProbe)
	default:
		return nil, fmt.Errorf("unsupported profile family %s", profile.Family)
	}
}

func (h *MilvusProfileHarness) indexName() string {
	if h.IndexName != "" {
		return h.IndexName
	}
	return DefaultIndexName
}

func (h *MilvusProfileHarness) vectorField() string {
	if h.VectorField != "" {
		return h.VectorField
	}
	return DefaultVectorField
}
