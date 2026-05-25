package retrieval

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"

	"interview-agents/internal/observability/metrics"
)

const (
	routeDense  = "dense"
	routeSparse = "sparse"
)

// HybridSearchRequest is the unified recall input contract for L1.
// query/expr/topk/kb_scope/kb_id/request_id
type HybridSearchRequest struct {
	Query      string
	Expr       string
	TopK       int
	KBScope    string
	KBID       uint64
	RequestID  string
	Collection string
}

// HybridRetriever orchestrates dense + sparse routes and keeps backward compatibility with Search(ctx, query, opts).
type HybridRetriever struct {
	retriever       *RetrieverService
	sparseRetriever *SparseRetriever
	reranker        Reranker
	config          *HybridRetrieverConfig
}

// HybridRetrieverConfig controls mixed retrieval behavior in L1.
type HybridRetrieverConfig struct {
	CandidateTopK int
	DenseWeight   float64
	SparseWeight  float64
	SparseConfig  *SparseRetrieverConfig
	RerankerImpl  Reranker
}

func NewHybridRetriever(retriever *RetrieverService, config *HybridRetrieverConfig) (*HybridRetriever, error) {
	if retriever == nil {
		return nil, fmt.Errorf("retriever service is nil")
	}
	if config == nil {
		config = &HybridRetrieverConfig{CandidateTopK: 10}
	}
	if config.CandidateTopK <= 0 {
		config.CandidateTopK = 10
	}
	if config.DenseWeight <= 0 {
		config.DenseWeight = 0.7
	}
	if config.SparseWeight <= 0 {
		config.SparseWeight = 0.3
	}

	sparseRetriever, err := NewSparseRetriever(retriever.client, retriever.config.Collection, config.SparseConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to init sparse retriever: %w", err)
	}

	hr := &HybridRetriever{
		retriever:       retriever,
		sparseRetriever: sparseRetriever,
		config:          config,
	}
	if config.RerankerImpl != nil {
		hr.reranker = config.RerankerImpl
	} else {
		hr.reranker = NewJaccardReranker(nil)
	}
	return hr, nil
}

// Search 混合检索器的对外搜索入口方法
// 入参：上下文、用户查询词、检索配置
// 返参：匹配的文档列表、错误信息
func (h *HybridRetriever) Search(ctx context.Context, query string, opts *RetrieveOptions) ([]*schema.Document, error) {
	// 1. 校验查询词是否为空
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is empty")
	}

	// 2. 处理请求ID（RequestID）：优先使用配置中的，无则自动生成
	requestID := ""
	if opts != nil {
		requestID = strings.TrimSpace(opts.RequestID)
	}
	if requestID == "" {
		// 自动生成唯一RequestID：hyb-时间戳(纳秒)
		requestID = fmt.Sprintf("hyb-%d", time.Now().UnixNano())
	}

	// 3. 处理返回结果数量（TopK）：优先使用配置中的，无则使用默认配置
	topK := h.config.CandidateTopK
	if opts != nil && opts.TopK > 0 {
		topK = opts.TopK
	}

	// 4. 构建底层检索需要的请求对象
	req := &HybridSearchRequest{
		Query:      query,
		TopK:       topK,
		RequestID:  requestID,
		Collection: h.retriever.config.Collection, // 默认使用检索器配置的集合
	}

	// 5. 如果传入了自定义配置opts，覆盖/填充请求对象的扩展字段
	if opts != nil {
		// 构建过滤表达式：优先使用自动生成的，空则兜底使用用户自定义的Expr
		req.Expr = strings.TrimSpace(BuildFilterExpr(opts))
		if req.Expr == "" {
			req.Expr = strings.TrimSpace(opts.Expr)
		}
		// 填充知识库范围、全局知识库ID
		req.KBScope = strings.TrimSpace(opts.KBScope)
		req.KBID = opts.ActiveGlobalKBID
		// 如果配置了自定义集合，覆盖默认集合
		if strings.TrimSpace(opts.Collection) != "" {
			req.Collection = strings.TrimSpace(opts.Collection)
		}
	}

	// 6. 调用底层真正执行混合检索的方法，返回结果
	return h.SearchWithRequest(ctx, req)
}

// SearchWithRequest is the L1 hybrid recall entry.
func (h *HybridRetriever) SearchWithRequest(ctx context.Context, req *HybridSearchRequest) ([]*schema.Document, error) {
	if req == nil {
		return nil, fmt.Errorf("hybrid search request is nil")
	}
	if strings.TrimSpace(req.Query) == "" {
		return nil, fmt.Errorf("query is empty")
	}
	if strings.TrimSpace(req.RequestID) == "" {
		req.RequestID = fmt.Sprintf("hyb-%d", time.Now().UnixNano())
	}
	if req.TopK <= 0 {
		req.TopK = h.config.CandidateTopK
	}
	if strings.TrimSpace(req.Collection) == "" {
		req.Collection = h.retriever.config.Collection
	}

	start := time.Now()
	type routeResult struct {
		route    string
		docs     []*schema.Document
		err      error
		duration time.Duration
	}

	resultCh := make(chan routeResult, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		routeStart := time.Now()
		docs, err := h.searchDense(ctx, req)
		resultCh <- routeResult{
			route:    routeDense,
			docs:     docs,
			err:      err,
			duration: time.Since(routeStart),
		}
	}()

	go func() {
		defer wg.Done()
		routeStart := time.Now()
		docs, err := h.sparseRetriever.Search(ctx, req)
		resultCh <- routeResult{
			route:    routeSparse,
			docs:     docs,
			err:      err,
			duration: time.Since(routeStart),
		}
	}()

	wg.Wait()
	close(resultCh)

	var (
		denseDocs  []*schema.Document
		sparseDocs []*schema.Document
		denseErr   error
		sparseErr  error
		denseMS    int64
		sparseMS   int64
	)
	for routeRes := range resultCh {
		switch routeRes.route {
		case routeDense:
			denseDocs = routeRes.docs
			denseErr = routeRes.err
			denseMS = routeRes.duration.Milliseconds()
		case routeSparse:
			sparseDocs = routeRes.docs
			sparseErr = routeRes.err
			sparseMS = routeRes.duration.Milliseconds()
		}
		h.observeRouteMetric(routeRes.route, routeRes.duration, routeRes.err, len(routeRes.docs))
	}

	if denseErr != nil && sparseErr != nil {
		totalMS := time.Since(start).Milliseconds()
		log.Printf(
			"[RAG:L1] request_id=%s query=%q final_query=%q expr=%q topk=%d routes=%s route_hits={dense:0,sparse:0} final_count=0 empty_reason=empty-after-retrieve duration_ms=%d dense_error=%q sparse_error=%q",
			req.RequestID, req.Query, req.Query, req.Expr, req.TopK, "dense+sparse", totalMS, denseErr.Error(), sparseErr.Error(),
		)
		return nil, fmt.Errorf("hybrid retrieval failed: dense=%v sparse=%v", denseErr, sparseErr)
	}

	rawCandidateCount := len(denseDocs) + len(sparseDocs)
	if rawCandidateCount == 0 {
		totalMS := time.Since(start).Milliseconds()
		log.Printf(
			"[RAG:L2] request_id=%s query=%q final_query=%q expr=%q topk=%d routes=%s route_hits={dense:%d,sparse:%d} final_count=0 empty_reason=%s duration_ms=%d dense_ms=%d sparse_ms=%d dense_error=%q sparse_error=%q",
			req.RequestID, req.Query, req.Query, req.Expr, req.TopK, "dense+sparse", len(denseDocs), len(sparseDocs), EmptyReasonAfterRetrieve, totalMS, denseMS, sparseMS, toLogError(denseErr), toLogError(sparseErr),
		)
		return []*schema.Document{}, nil
	}

	fused := FuseRouteCandidates(denseDocs, sparseDocs, FusionConfig{
		DenseWeight:  h.config.DenseWeight,
		SparseWeight: h.config.SparseWeight,
	})
	if len(fused) == 0 {
		totalMS := time.Since(start).Milliseconds()
		log.Printf(
			"[RAG:L2] request_id=%s query=%q final_query=%q expr=%q topk=%d routes=%s route_hits={dense:%d,sparse:%d} final_count=0 empty_reason=%s duration_ms=%d dense_ms=%d sparse_ms=%d dense_error=%q sparse_error=%q",
			req.RequestID, req.Query, req.Query, req.Expr, req.TopK, "dense+sparse", len(denseDocs), len(sparseDocs), EmptyReasonAfterFusion, totalMS, denseMS, sparseMS, toLogError(denseErr), toLogError(sparseErr),
		)
		return []*schema.Document{}, nil
	}

	merged := DeduplicateFusedDocuments(fused)
	if len(merged) == 0 {
		totalMS := time.Since(start).Milliseconds()
		log.Printf(
			"[RAG:L2] request_id=%s query=%q final_query=%q expr=%q topk=%d routes=%s route_hits={dense:%d,sparse:%d} final_count=0 empty_reason=%s duration_ms=%d dense_ms=%d sparse_ms=%d dense_error=%q sparse_error=%q",
			req.RequestID, req.Query, req.Query, req.Expr, req.TopK, "dense+sparse", len(denseDocs), len(sparseDocs), EmptyReasonAfterFusion, totalMS, denseMS, sparseMS, toLogError(denseErr), toLogError(sparseErr),
		)
		return []*schema.Document{}, nil
	}

	if h.reranker != nil {
		reranked, err := h.reranker.Rerank(ctx, req.Query, merged)
		if err == nil && len(reranked) > 0 {
			merged = reranked
		}
	}

	emptyReason := EmptyReasonNone
	if len(merged) == 0 {
		emptyReason = EmptyReasonAfterFilter
	}

	if len(merged) > req.TopK {
		merged = merged[:req.TopK]
	}

	totalMS := time.Since(start).Milliseconds()
	log.Printf(
		"[RAG:L2] request_id=%s query=%q final_query=%q expr=%q topk=%d routes=%s route_hits={dense:%d,sparse:%d} final_count=%d empty_reason=%s duration_ms=%d dense_ms=%d sparse_ms=%d dense_error=%q sparse_error=%q",
		req.RequestID,
		req.Query,
		req.Query,
		req.Expr,
		req.TopK,
		"dense+sparse",
		len(denseDocs),
		len(sparseDocs),
		len(merged),
		emptyReason,
		totalMS,
		denseMS,
		sparseMS,
		toLogError(denseErr),
		toLogError(sparseErr),
	)
	return merged, nil
}

func (h *HybridRetriever) searchDense(ctx context.Context, req *HybridSearchRequest) ([]*schema.Document, error) {
	opts := &RetrieveOptions{
		Expr:             req.Expr,
		TopK:             req.TopK,
		Collection:       req.Collection,
		KBScope:          req.KBScope,
		ActiveGlobalKBID: req.KBID,
		RequestID:        req.RequestID,
	}
	docs, err := h.retriever.RetrieveWithOptions(ctx, req.Query, opts)
	if err != nil {
		return nil, err
	}
	for _, doc := range docs {
		if doc.MetaData == nil {
			doc.MetaData = make(map[string]interface{})
		}
		doc.MetaData["route"] = routeDense
		doc.MetaData["dense_score"] = readDocScore(doc)
	}
	return docs, nil
}

func (h *HybridRetriever) observeRouteMetric(route string, duration time.Duration, routeErr error, hitCount int) {
	status := "ok"
	errCode := "none"
	if routeErr != nil {
		status = "error"
		errCode = "route_failed"
	}
	metrics.ObserveRetrieveRoute(route, duration, status, errCode, hitCount)
}

func readDocScore(doc *schema.Document) float64 {
	if doc == nil || doc.MetaData == nil {
		return 0
	}
	if value, ok := doc.MetaData["score"]; ok {
		if score, ok := castScore(value); ok {
			return score
		}
	}
	if value, ok := doc.MetaData["sparse_score"]; ok {
		if score, ok := castScore(value); ok {
			return score
		}
	}
	return 0
}

func toLogError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
