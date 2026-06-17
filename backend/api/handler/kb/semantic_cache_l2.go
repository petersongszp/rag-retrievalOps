package kb

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	semanticcache "interview-agents/internal/cache/semantic"
	"interview-agents/internal/config"
	"interview-agents/internal/milvus"
	"interview-agents/internal/milvus/storage"
	"interview-agents/internal/rag/experiment"
	"interview-agents/internal/rag/release"
	"interview-agents/internal/repository"

	"github.com/cloudwego/hertz/pkg/app"
)

const semanticCacheTopKPolicyVersion = "semantic-cache-exact-topk-v1"

type semanticCacheEmbedding interface {
	EmbedBatch(ctx context.Context, texts []string) ([][]float64, error)
}

// semanticCacheStore 把 handler 和底层 Redis 实现隔开。
// 这样上层只关心“查候选 / 续期 / 写入 / 删除”四件事，
// 不需要直接知道 Redis 的 key 结构细节。
type semanticCacheStore interface {
	GetCandidates(ctx context.Context, scope semanticcache.Scope, maxCandidates int) (*semanticcache.LookupResult, error)
	Touch(ctx context.Context, entry *semanticcache.Entry, ttl time.Duration) error
	Put(ctx context.Context, scope semanticcache.Scope, entry *semanticcache.Entry, ttl time.Duration, maxEntries int) error
	DeleteByKnowledgeBase(ctx context.Context, tenantID uint64, kbID uint64) error
}

type semanticCacheLookupInput struct {
	RequestID          string
	TenantID           uint64
	UserID             uint
	Query              string
	TopK               int
	KBIDs              []uint64
	QueryType          string
	StrategyVersion    string
	ExperimentDecision experiment.Decision
	DebugRequested     bool
}

type semanticCacheHit struct {
	Response         retrieveResponse
	EntryID          string
	RetrieverVersion string
	Similarity       float64
	CandidateCount   int
	EmbeddingMs      int64
	LookupMs         int64
}

type semanticCacheLookupTrace struct {
	Enabled           bool
	Hit               bool
	Reason            string
	EntryID           string
	Similarity        float64
	HighestSimilarity float64
	CandidateCount    int
	EmbeddingMs       int64
	LookupMs          int64
	Threshold         float64
	EmbeddingCache    storage.EmbeddingCacheTrace
}

func trySemanticCacheHit(
	ctx context.Context,
	input semanticCacheLookupInput,
	store semanticCacheStore,
	embedder semanticCacheEmbedding,
) (*semanticCacheHit, semanticCacheLookupTrace, error) {
	// trace 是这次缓存尝试的“过程记录本”。
	// 不管最后命中还是未命中，L4/L5 都要靠它解释发生了什么。
	trace := semanticCacheLookupTrace{
		Enabled:   config.Global.RAG.FeatureFlags.EnableSemanticCache,
		Threshold: config.Global.RAG.SemanticCache.SimilarityThreshold,
	}
	if reason := semanticCacheBypassReason(input); reason != "" {
		trace.Reason = reason
		return nil, trace, nil
	}
	if store == nil || embedder == nil {
		trace.Reason = "dependency_unavailable"
		return nil, trace, nil
	}

	// scope 就是缓存隔离边界。
	// 只有 tenant、kb、strategy、query_type 都一致的请求，才允许互相复用结果。
	scope := semanticcache.Scope{
		TenantID:        input.TenantID,
		KBIDs:           append([]uint64(nil), input.KBIDs...),
		StrategyVersion: strings.TrimSpace(input.StrategyVersion),
		QueryType:       strings.TrimSpace(input.QueryType),
	}
	if err := scope.Validate(); err != nil {
		trace.Reason = "invalid_scope"
		return nil, trace, err
	}

	embedCtx := storage.WithEmbeddingCacheTrace(ctx)
	embeddingStart := time.Now()
	vectors, err := embedder.EmbedBatch(embedCtx, []string{strings.TrimSpace(input.Query)})
	embeddingMs := time.Since(embeddingStart).Milliseconds()
	trace.EmbeddingMs = embeddingMs
	trace.EmbeddingCache = storage.ConsumeEmbeddingCacheTrace(embedCtx)
	if err != nil {
		trace.Reason = "embed_failed"
		return nil, trace, err
	}
	if len(vectors) == 0 || len(vectors[0]) == 0 {
		trace.Reason = "embed_empty"
		return nil, trace, fmt.Errorf("semantic cache embedding is empty")
	}
	// Redis 里缓存的向量是 float32，这里统一转成同一精度再算相似度。
	queryEmbedding := float64VectorTo32(vectors[0])

	lookupStart := time.Now()
	lookup, err := store.GetCandidates(ctx, scope, config.Global.RAG.SemanticCache.MaxCandidates)
	lookupMs := time.Since(lookupStart).Milliseconds()
	trace.LookupMs = embeddingMs + lookupMs
	if err != nil {
		trace.Reason = "lookup_failed"
		return nil, trace, err
	}
	if lookup == nil || len(lookup.Candidates) == 0 {
		trace.Reason = "miss"
		return nil, trace, nil
	}
	trace.CandidateCount = len(lookup.Candidates)

	var (
		bestEntry      *semanticcache.Entry
		bestSimilarity float64
		seenSimilarity bool
	)
	threshold := config.Global.RAG.SemanticCache.SimilarityThreshold
	for _, candidate := range lookup.Candidates {
		// 这里采用 exact top_k 约束：
		// top_k 不同，结果规模就可能不同，所以先不允许互相复用。
		if candidate == nil || candidate.TopK != input.TopK {
			continue
		}
		similarity, ok := cosineSimilarity32(queryEmbedding, candidate.QueryEmbedding)
		if !ok {
			continue
		}
		if !seenSimilarity || similarity > trace.HighestSimilarity {
			trace.HighestSimilarity = similarity
			seenSimilarity = true
		}
		if similarity < threshold {
			continue
		}
		if bestEntry == nil || similarity > bestSimilarity {
			bestEntry = candidate
			bestSimilarity = similarity
		}
	}
	if bestEntry == nil {
		trace.Reason = "miss"
		return nil, trace, nil
	}

	// 找到最优候选后，真正返回前还要再做一次 payload 校验。
	// 这是为了防止 Redis 中的旧数据或异常数据把错误结果带回主链路。
	var resp retrieveResponse
	if err := json.Unmarshal(bestEntry.ResponsePayload, &resp); err != nil {
		trace.Reason = "decode_failed"
		return nil, trace, err
	}
	resp.RequestID = input.RequestID
	resp.Items = sanitizeSemanticCacheItems(resp.Items, input.KBIDs)
	if len(resp.Items) == 0 && !hasSemanticCacheTerminalPayload(resp) {
		trace.Reason = "payload_scope_mismatch"
		return nil, trace, fmt.Errorf("semantic cache payload lost all items after scope filtering")
	}

	// 命中后做 Touch，不是为了改业务结果，而是为了两件事：
	// 1. 更新 hit_count / last_hit_at
	// 2. 把热点缓存的 TTL 向后续一段时间
	ttl := time.Duration(config.Global.RAG.SemanticCache.TTLSeconds) * time.Second
	if err := store.Touch(ctx, bestEntry, ttl); err != nil {
		log.Printf("[Semantic Cache] touch failed entry_id=%s err=%v", bestEntry.EntryID, err)
	}

	trace.Hit = true
	trace.Reason = "hit"
	trace.EntryID = bestEntry.EntryID
	trace.Similarity = bestSimilarity
	trace.HighestSimilarity = bestSimilarity

	return &semanticCacheHit{
		Response:         resp,
		EntryID:          bestEntry.EntryID,
		RetrieverVersion: firstNonEmptyString(bestEntry.RetrieverVersion, inferRetrieverVersionFromItems(resp.Items)),
		Similarity:       bestSimilarity,
		CandidateCount:   len(lookup.Candidates),
		EmbeddingMs:      embeddingMs,
		LookupMs:         embeddingMs + lookupMs,
	}, trace, nil
}

func semanticCacheBypassReason(input semanticCacheLookupInput) string {
	// 这里集中定义“哪些请求不允许查缓存”。
	// 这样 L0 的边界规则不会散落在 handler 各处。
	if !config.Global.RAG.FeatureFlags.EnableSemanticCache {
		return "disabled"
	}
	if strings.TrimSpace(input.Query) == "" {
		return "empty_query"
	}
	if input.DebugRequested {
		return "debug_request"
	}
	if input.TenantID == 0 {
		return "authorization_abnormal"
	}
	if input.TopK <= 0 || len(input.KBIDs) == 0 {
		return "contract_incomplete"
	}
	if strings.TrimSpace(input.QueryType) == "" || strings.TrimSpace(input.StrategyVersion) == "" {
		return "contract_incomplete"
	}
	if strings.EqualFold(strings.TrimSpace(input.ExperimentDecision.Group), experiment.GroupShadow) {
		return "high_risk_experiment"
	}
	return ""
}

func resolveSemanticCacheStrategyVersion(experimentDecision experiment.Decision, releaseDecision release.Decision) string {
	switch experimentDecision.Group {
	case experiment.GroupCandidate, experiment.GroupShadow:
		return firstNonEmptyString(experimentDecision.CandidateVersion, releaseDecision.Strategy)
	case experiment.GroupBaseline:
		return firstNonEmptyString(experimentDecision.BaselineVersion, releaseDecision.Strategy)
	default:
		return firstNonEmptyString(experimentDecision.BaselineVersion, experimentDecision.CandidateVersion, releaseDecision.Strategy)
	}
}

func isSemanticCacheDebugRequest(c *app.RequestContext) bool {
	if c == nil {
		return false
	}
	for _, key := range []string{"debug", "trace", "include_debug"} {
		if isTruthySemanticCacheFlag(string(c.Query(key))) {
			return true
		}
	}
	return false
}

func isTruthySemanticCacheFlag(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func sanitizeSemanticCacheItems(items []retrieveItem, allowedKBIDs []uint64) []retrieveItem {
	if len(items) == 0 {
		return []retrieveItem{}
	}
	// 即使 scope 已经过滤过，真正出结果前再按 kb_ids 过滤一遍，
	// 相当于做最后一道保险。
	allowed := make(map[uint64]struct{}, len(allowedKBIDs))
	for _, kbID := range allowedKBIDs {
		if kbID > 0 {
			allowed[kbID] = struct{}{}
		}
	}
	filtered := make([]retrieveItem, 0, len(items))
	for _, item := range items {
		if len(allowed) > 0 {
			if _, ok := allowed[item.Citation.KBID]; !ok {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func hasSemanticCacheTerminalPayload(resp retrieveResponse) bool {
	if resp.Refusal != nil {
		return true
	}
	if strings.TrimSpace(resp.EvidenceGateResult) != "" {
		return true
	}
	return resp.CitationCheck != nil
}

func inferRetrieverVersionFromItems(items []retrieveItem) string {
	for _, item := range items {
		if strings.TrimSpace(item.Source.RetrieverVersion) != "" {
			return strings.TrimSpace(item.Source.RetrieverVersion)
		}
	}
	return ""
}

func resolveSemanticCacheStore() semanticCacheStore {
	// 启动时 repository.InitRedis 会把全局 Redis client 初始化好。
	// 这里再把通用 client 包装成 semantic cache 专用 store。
	client := repository.GetRedis()
	if client == nil {
		return nil
	}
	return semanticcache.NewStore(client)
}

func resolveSemanticCacheEmbedder(manager *milvus.MilvusManager) semanticCacheEmbedding {
	// 语义缓存查找并不是全文检索，但它仍然需要 embedding 服务，
	// 因为“语义相近”是通过向量相似度来判断的。
	if manager == nil || manager.GetEmbeddingService() == nil {
		return nil
	}
	// 这里额外包一层进程内存缓存：
	// 同一个 query 的 embedding 向量在短时间内只算一次，
	// 后续命中同 query 时直接复用，减少 lookup P95。
	return manager.GetEmbeddingService()
}

func float64VectorTo32(values []float64) []float32 {
	out := make([]float32, 0, len(values))
	for _, value := range values {
		out = append(out, float32(value))
	}
	return out
}

func cosineSimilarity32(a, b []float32) (float64, bool) {
	if len(a) == 0 || len(a) != len(b) {
		return 0, false
	}
	var (
		dot   float64
		normA float64
		normB float64
	)
	for i := range a {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		normA += av * av
		normB += bv * bv
	}
	if normA == 0 || normB == 0 {
		return 0, false
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB)), true
}
