package kb

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	semanticcache "interview-agents/internal/cache/semantic"
	"interview-agents/internal/config"
	"interview-agents/internal/milvus/retrieval"
	"interview-agents/internal/model"
	"interview-agents/internal/rag/experiment"
)

const semanticCacheAsyncTimeout = 3 * time.Second

type semanticCacheBackfillInput struct {
	RequestID          string
	TenantID           uint64
	Query              string
	TopK               int
	KBIDs              []uint64
	QueryType          string
	StrategyVersion    string
	ExperimentDecision experiment.Decision
	DebugRequested     bool
	RetrieverVersion   string
	Response           retrieveResponse
	ResultStatus       model.RetrieveResultStatus
}

func shouldSemanticCacheBackfill(input semanticCacheBackfillInput) string {
	// 先复用 L2 的绕过规则，保证“不能查缓存”的请求也不会被回填进缓存。
	reason := semanticCacheBypassReason(semanticCacheLookupInput{
		RequestID:          input.RequestID,
		TenantID:           input.TenantID,
		Query:              input.Query,
		TopK:               input.TopK,
		KBIDs:              input.KBIDs,
		QueryType:          input.QueryType,
		StrategyVersion:    input.StrategyVersion,
		ExperimentDecision: input.ExperimentDecision,
		DebugRequested:     input.DebugRequested,
	})
	if reason != "" {
		return reason
	}
	if input.Response.Refusal != nil || strings.EqualFold(strings.TrimSpace(input.Response.EvidenceGateResult), retrieval.EvidenceGateResultRefused) {
		// 被 Evidence Gate 拒绝的结果不应该进入缓存，
		// 否则以后相似 query 会一直复用这次拒答。
		return "evidence_refusal"
	}
	if len(input.Response.Items) == 0 {
		return "empty_result"
	}
	if input.ResultStatus != model.RetrieveResultStatusSuccess {
		return "result_not_success"
	}
	return ""
}

func scheduleSemanticCacheBackfill(input semanticCacheBackfillInput, store semanticCacheStore, embedder semanticCacheEmbedding) {
	if reason := shouldSemanticCacheBackfill(input); reason != "" {
		return
	}
	if store == nil || embedder == nil {
		return
	}

	// 回填放到异步 goroutine 里，是因为它属于“锦上添花”的动作，
	// 不应该阻塞这次用户请求的主响应时间。
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), semanticCacheAsyncTimeout)
		defer cancel()

		if err := backfillSemanticCache(ctx, input, store, embedder); err != nil {
			log.Printf("[Semantic Cache] backfill failed request_id=%s tenant_id=%d kb_ids=%v err=%v", input.RequestID, input.TenantID, input.KBIDs, err)
		}
	}()
}

func backfillSemanticCache(ctx context.Context, input semanticCacheBackfillInput, store semanticCacheStore, embedder semanticCacheEmbedding) error {
	if reason := shouldSemanticCacheBackfill(input); reason != "" {
		return fmt.Errorf("semantic cache backfill bypassed: %s", reason)
	}
	if store == nil || embedder == nil {
		return fmt.Errorf("semantic cache dependency unavailable")
	}

	vectors, err := embedder.EmbedBatch(ctx, []string{strings.TrimSpace(input.Query)})
	if err != nil {
		return err
	}
	if len(vectors) == 0 || len(vectors[0]) == 0 {
		return fmt.Errorf("semantic cache embedding is empty")
	}

	// 回填和命中共用同一份 scope 规则，保证“怎么写进去”与“怎么读出来”严格一致。
	scope := semanticcache.Scope{
		TenantID:        input.TenantID,
		KBIDs:           append([]uint64(nil), input.KBIDs...),
		StrategyVersion: strings.TrimSpace(input.StrategyVersion),
		QueryType:       strings.TrimSpace(input.QueryType),
	}
	if err := scope.Validate(); err != nil {
		return err
	}

	payload, err := json.Marshal(input.Response)
	if err != nil {
		return err
	}

	entry := &semanticcache.Entry{
		TenantID:         input.TenantID,
		KBIDs:            append([]uint64(nil), input.KBIDs...),
		StrategyVersion:  strings.TrimSpace(input.StrategyVersion),
		RetrieverVersion: firstNonEmptyString(strings.TrimSpace(input.RetrieverVersion), inferRetrieverVersionFromItems(input.Response.Items)),
		QueryType:        strings.TrimSpace(input.QueryType),
		Query:            strings.TrimSpace(input.Query),
		QueryEmbedding:   float64VectorTo32(vectors[0]),
		ResponsePayload:  payload,
		ResultPayload:    semanticcache.ResultPayloadTag,
		TopK:             input.TopK,
	}

	// 真正写入 Redis 的动作在 store.Put 里，
	// handler 这一层只负责把业务数据整理成标准缓存 entry。
	ttl := time.Duration(config.Global.RAG.SemanticCache.TTLSeconds) * time.Second
	return store.Put(ctx, scope, entry, ttl, config.Global.RAG.SemanticCache.MaxEntriesPerScope)
}

func scheduleSemanticCacheInvalidation(tenantID uint64, kbID uint64, store semanticCacheStore) {
	if !config.Global.RAG.FeatureFlags.EnableSemanticCache {
		return
	}
	if tenantID == 0 || kbID == 0 || store == nil {
		return
	}

	// 失效也用异步，是因为知识库变更的主流程不应该被 Redis 清理动作拖慢。
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), semanticCacheAsyncTimeout)
		defer cancel()

		if err := invalidateSemanticCacheByKnowledgeBase(ctx, tenantID, kbID, store); err != nil {
			log.Printf("[Semantic Cache] invalidation failed tenant_id=%d kb_id=%d err=%v", tenantID, kbID, err)
		}
	}()
}

func invalidateSemanticCacheByKnowledgeBase(ctx context.Context, tenantID uint64, kbID uint64, store semanticCacheStore) error {
	// 这里按 knowledge base 粒度删除，是 L3 最关键的防污染动作之一。
	// 一旦知识库内容变了，旧缓存就不应该继续服务新请求。
	if store == nil {
		return fmt.Errorf("semantic cache store is not initialized")
	}
	if tenantID == 0 {
		return fmt.Errorf("tenant_id must be > 0")
	}
	if kbID == 0 {
		return fmt.Errorf("kb_id must be > 0")
	}
	return store.DeleteByKnowledgeBase(ctx, tenantID, kbID)
}
