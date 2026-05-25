package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"

	"interview-agents/internal/errors"
	"interview-agents/internal/milvus"
	"interview-agents/internal/milvus/retrieval"
)

type MilvusRetrieverInput struct {
	Query string `json:"query" description:"要检索的查询文本"`
}

type MilvusRetrieverOutput struct {
	Documents []DocumentInfo `json:"documents" description:"检索到的相关文档列表"`
	Count     int            `json:"count" description:"检索到的文档数量"`
	Error     string         `json:"error,omitempty" description:"错误信息"`
}

type DocumentInfo struct {
	ID       string                 `json:"id"`
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Score    float32                `json:"score,omitempty"`
}

var TestMilvusCallCallback func(query string)

func GetMilvusRetrieverWithInput(ctx context.Context, input MilvusRetrieverInput) (string, error) {
	if TestMilvusCallCallback != nil {
		TestMilvusCallCallback(input.Query)
	}
	return GetMilvusRetriever(ctx, input.Query)
}

func GetMilvusRetriever(ctx context.Context, query string) (string, error) {
	if query == "" {
		return "", fmt.Errorf("query cannot be empty")
	}

	mgr, err := milvus.GetMilvusManager()
	if err != nil {
		log.Printf("milvus manager unavailable: %v", err)
		return formatErrorOutput(fmt.Sprintf("milvus manager unavailable: %v", err), 0)
	}

	timeout := 3 * time.Second
	if mgr.Config != nil && mgr.Config.RAG.Thresholds.RetrieveTimeoutMS > 0 {
		timeout = time.Duration(mgr.Config.RAG.Thresholds.RetrieveTimeoutMS) * time.Millisecond
	}
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	useHybrid := mgr.Config != nil &&
		mgr.Config.RAG.FeatureFlags.EnableHybridRetrieval &&
		mgr.HybridRetriever != nil

	documents, err := retrieveDocuments(ctxWithTimeout, mgr, query, useHybrid)
	if err != nil {
		log.Printf("retrieval failed: %v", err)
		return formatErrorOutput(fmt.Sprintf("retrieval failed: %v", err), 0)
	}

	if len(documents) == 0 {
		log.Printf("no documents found, query=%s", query)
		return formatOutput([]DocumentInfo{}, 0)
	}

	docInfos := make([]DocumentInfo, 0, len(documents))
	for _, doc := range documents {
		score := readScore32(doc)
		docInfos = append(docInfos, DocumentInfo{
			ID:       doc.ID,
			Content:  doc.Content,
			Metadata: doc.MetaData,
			Score:    score,
		})
	}

	log.Printf("retrieval success, found %d documents", len(docInfos))
	return formatOutput(docInfos, len(docInfos))
}

func retrieveDocuments(ctx context.Context, mgr *milvus.MilvusManager, query string, useHybrid bool) ([]*schema.Document, error) {
	if useHybrid {
		topK := 10
		if mgr.Config != nil {
			if mgr.Config.RAG.Phase2.CandidateTopK > 0 {
				topK = mgr.Config.RAG.Phase2.CandidateTopK
			} else if mgr.Config.Milvus.TopK > 0 {
				topK = mgr.Config.Milvus.TopK
			}
		}
		return mgr.HybridRetriever.Search(ctx, query, &retrieval.RetrieveOptions{
			TopK:      topK,
			RequestID: fmt.Sprintf("tool-%d", time.Now().UnixNano()),
		})
	}

	retriever := mgr.RetrieverService
	if retriever == nil {
		return nil, fmt.Errorf("retriever service is not initialized")
	}
	return retriever.Retrieve(ctx, query)
}

func readScore32(doc *schema.Document) float32 {
	if doc == nil || doc.MetaData == nil {
		return 0
	}
	if v, ok := doc.MetaData["score"]; ok {
		switch s := v.(type) {
		case float32:
			return s
		case float64:
			return float32(s)
		case int:
			return float32(s)
		}
	}
	return 0
}

func formatOutput(documents []DocumentInfo, count int) (string, error) {
	output := MilvusRetrieverOutput{
		Documents: documents,
		Count:     count,
	}
	jsonBytes, err := json.Marshal(output)
	if err != nil {
		log.Printf("failed to marshal output: %v", err)
		return "", err
	}
	return string(jsonBytes), nil
}

func formatErrorOutput(errMsg string, count int) (string, error) {
	output := MilvusRetrieverOutput{
		Documents: []DocumentInfo{},
		Count:     count,
		Error:     errMsg,
	}
	jsonBytes, err := json.Marshal(output)
	if err != nil {
		log.Printf("failed to marshal error output: %v", err)
		return "", err
	}
	return string(jsonBytes), nil
}

func GetMilvusRetrieverTool() (tool.InvokableTool, error) {
	t, err := utils.InferTool(
		"get_milvus_retriever",
		"从向量数据库中检索相关内容。输入查询文本，返回最相关文档。",
		GetMilvusRetrieverWithInput,
	)
	if err != nil {
		return nil, errors.NewMilvusError("创建检索工具失败", err)
	}
	return t, nil
}
