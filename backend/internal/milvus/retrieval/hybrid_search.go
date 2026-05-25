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

type HybridSearchRequest struct {
	Query           string
	OriginalQuery   string
	RewriteQuery    string
	FinalQuery      string
	RewriteStrategy string
	RewriteApplied  bool
	Expr            string
	TopK            int
	KBScope         string
	KBID            uint64
	RequestID       string
	Collection      string
	CandidateTopK   int
}

type HybridRetriever struct {
	retriever       *RetrieverService
	sparseRetriever *SparseRetriever
	reranker        Reranker
	queryRewriter   QueryRewriter
	config          *HybridRetrieverConfig
}

type HybridRetrieverConfig struct {
	CandidateTopK int
	DenseWeight   float64
	SparseWeight  float64
	SparseConfig  *SparseRetrieverConfig
	RerankerImpl  Reranker
	QueryRewriter QueryRewriter
	DynamicTopK   DynamicTopKConfig
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
		queryRewriter:   config.QueryRewriter,
		config:          config,
	}
	if config.RerankerImpl != nil {
		hr.reranker = config.RerankerImpl
	} else {
		hr.reranker = NewJaccardReranker(nil)
	}
	return hr, nil
}

func (h *HybridRetriever) Search(ctx context.Context, query string, opts *RetrieveOptions) ([]*schema.Document, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is empty")
	}

	requestID := ""
	if opts != nil {
		requestID = strings.TrimSpace(opts.RequestID)
	}
	if requestID == "" {
		requestID = fmt.Sprintf("hyb-%d", time.Now().UnixNano())
	}

	candidateTopK := h.config.CandidateTopK
	if opts != nil && opts.CandidateTopK > 0 {
		candidateTopK = opts.CandidateTopK
	}
	finalTopK := 0
	if opts != nil && opts.TopK > 0 {
		finalTopK = opts.TopK
	}

	req := &HybridSearchRequest{
		Query:         query,
		OriginalQuery: query,
		FinalQuery:    query,
		TopK:          finalTopK,
		RequestID:     requestID,
		Collection:    h.retriever.config.Collection,
		CandidateTopK: candidateTopK,
	}

	if opts != nil {
		req.Expr = strings.TrimSpace(BuildFilterExpr(opts))
		if req.Expr == "" {
			req.Expr = strings.TrimSpace(opts.Expr)
		}
		req.KBScope = strings.TrimSpace(opts.KBScope)
		req.KBID = opts.ActiveGlobalKBID
		if strings.TrimSpace(opts.Collection) != "" {
			req.Collection = strings.TrimSpace(opts.Collection)
		}
		if strings.TrimSpace(opts.OriginalQuery) != "" {
			req.OriginalQuery = strings.TrimSpace(opts.OriginalQuery)
		}
		if strings.TrimSpace(opts.RewriteQuery) != "" {
			req.RewriteQuery = strings.TrimSpace(opts.RewriteQuery)
		}
		if strings.TrimSpace(opts.FinalQuery) != "" {
			req.FinalQuery = strings.TrimSpace(opts.FinalQuery)
		}
		if strings.TrimSpace(opts.RewriteStrategy) != "" {
			req.RewriteStrategy = strings.TrimSpace(opts.RewriteStrategy)
		}
		req.RewriteApplied = opts.RewriteApplied
	}

	return h.SearchWithRequest(ctx, req)
}

func (h *HybridRetriever) SearchWithRequest(ctx context.Context, req *HybridSearchRequest) ([]*schema.Document, error) {
	if req == nil {
		return nil, fmt.Errorf("hybrid search request is nil")
	}
	if strings.TrimSpace(req.Query) == "" {
		return nil, fmt.Errorf("query is empty")
	}
	if strings.TrimSpace(req.OriginalQuery) == "" {
		req.OriginalQuery = strings.TrimSpace(req.Query)
	}
	if strings.TrimSpace(req.FinalQuery) == "" {
		req.FinalQuery = req.OriginalQuery
	}
	if strings.TrimSpace(req.RequestID) == "" {
		req.RequestID = fmt.Sprintf("hyb-%d", time.Now().UnixNano())
	}
	if req.CandidateTopK <= 0 {
		req.CandidateTopK = h.config.CandidateTopK
	}
	if req.TopK <= 0 {
		req.TopK = req.CandidateTopK
	}
	if strings.TrimSpace(req.Collection) == "" {
		req.Collection = h.retriever.config.Collection
	}

	req.applyControlledRewrite(ctx, h.queryRewriter)
	topKDecision := DecideDynamicTopK(req.FinalQuery, req.CandidateTopK, req.TopK, h.config.DynamicTopK)

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
		resultCh <- routeResult{route: routeDense, docs: docs, err: err, duration: time.Since(routeStart)}
	}()

	go func() {
		defer wg.Done()
		routeStart := time.Now()
		docs, err := h.sparseRetriever.Search(ctx, req)
		resultCh <- routeResult{route: routeSparse, docs: docs, err: err, duration: time.Since(routeStart)}
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
			"[RAG:L1] request_id=%s query=%q rewrite=%q final_query=%q rewrite_strategy=%q rewrite_applied=%t expr=%q candidate_topk=%d final_topk=%d token_budget=%d truncate_reason=%q routes=%s route_hits={dense:0,sparse:0} final_count=0 empty_reason=empty-after-retrieve duration_ms=%d dense_error=%q sparse_error=%q",
			req.RequestID, req.OriginalQuery, req.RewriteQuery, req.FinalQuery, req.RewriteStrategy, req.RewriteApplied, req.Expr, topKDecision.CandidateTopK, topKDecision.FinalTopK, topKDecision.TokenBudget, topKDecision.TruncateReason, "dense+sparse", totalMS, denseErr.Error(), sparseErr.Error(),
		)
		return nil, fmt.Errorf("hybrid retrieval failed: dense=%v sparse=%v", denseErr, sparseErr)
	}

	rawCandidateCount := len(denseDocs) + len(sparseDocs)
	if rawCandidateCount == 0 {
		totalMS := time.Since(start).Milliseconds()
		log.Printf(
			"[RAG:L2] request_id=%s query=%q rewrite=%q final_query=%q rewrite_strategy=%q rewrite_applied=%t expr=%q candidate_topk=%d final_topk=%d token_budget=%d truncate_reason=%q routes=%s route_hits={dense:%d,sparse:%d} final_count=0 empty_reason=%s duration_ms=%d dense_ms=%d sparse_ms=%d dense_error=%q sparse_error=%q",
			req.RequestID, req.OriginalQuery, req.RewriteQuery, req.FinalQuery, req.RewriteStrategy, req.RewriteApplied, req.Expr, topKDecision.CandidateTopK, topKDecision.FinalTopK, topKDecision.TokenBudget, topKDecision.TruncateReason, "dense+sparse", len(denseDocs), len(sparseDocs), EmptyReasonAfterRetrieve, totalMS, denseMS, sparseMS, toLogError(denseErr), toLogError(sparseErr),
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
			"[RAG:L2] request_id=%s query=%q rewrite=%q final_query=%q rewrite_strategy=%q rewrite_applied=%t expr=%q candidate_topk=%d final_topk=%d token_budget=%d truncate_reason=%q routes=%s route_hits={dense:%d,sparse:%d} final_count=0 empty_reason=%s duration_ms=%d dense_ms=%d sparse_ms=%d dense_error=%q sparse_error=%q",
			req.RequestID, req.OriginalQuery, req.RewriteQuery, req.FinalQuery, req.RewriteStrategy, req.RewriteApplied, req.Expr, topKDecision.CandidateTopK, topKDecision.FinalTopK, topKDecision.TokenBudget, topKDecision.TruncateReason, "dense+sparse", len(denseDocs), len(sparseDocs), EmptyReasonAfterFusion, totalMS, denseMS, sparseMS, toLogError(denseErr), toLogError(sparseErr),
		)
		return []*schema.Document{}, nil
	}

	merged := DeduplicateFusedDocuments(fused)
	if len(merged) == 0 {
		totalMS := time.Since(start).Milliseconds()
		log.Printf(
			"[RAG:L2] request_id=%s query=%q rewrite=%q final_query=%q rewrite_strategy=%q rewrite_applied=%t expr=%q candidate_topk=%d final_topk=%d token_budget=%d truncate_reason=%q routes=%s route_hits={dense:%d,sparse:%d} final_count=0 empty_reason=%s duration_ms=%d dense_ms=%d sparse_ms=%d dense_error=%q sparse_error=%q",
			req.RequestID, req.OriginalQuery, req.RewriteQuery, req.FinalQuery, req.RewriteStrategy, req.RewriteApplied, req.Expr, topKDecision.CandidateTopK, topKDecision.FinalTopK, topKDecision.TokenBudget, topKDecision.TruncateReason, "dense+sparse", len(denseDocs), len(sparseDocs), EmptyReasonAfterFusion, totalMS, denseMS, sparseMS, toLogError(denseErr), toLogError(sparseErr),
		)
		return []*schema.Document{}, nil
	}

	if h.reranker != nil {
		reranked, err := h.reranker.Rerank(ctx, req.OriginalQuery, merged)
		if err == nil && len(reranked) > 0 {
			merged = reranked
		}
	}

	emptyReason := EmptyReasonNone
	if len(merged) == 0 {
		emptyReason = EmptyReasonAfterFilter
	}

	beforeTruncateCount := len(merged)
	merged, topKDecision = ApplyTokenBudgetGuard(merged, topKDecision, h.config.DynamicTopK)
	truncatedCount := beforeTruncateCount - len(merged)

	totalMS := time.Since(start).Milliseconds()
	log.Printf(
		"[RAG:L2] request_id=%s query=%q rewrite=%q final_query=%q rewrite_strategy=%q rewrite_applied=%t expr=%q candidate_topk=%d final_topk=%d token_budget=%d truncate_reason=%q routes=%s route_hits={dense:%d,sparse:%d} final_count=%d truncated_count=%d empty_reason=%s duration_ms=%d dense_ms=%d sparse_ms=%d dense_error=%q sparse_error=%q",
		req.RequestID,
		req.OriginalQuery,
		req.RewriteQuery,
		req.FinalQuery,
		req.RewriteStrategy,
		req.RewriteApplied,
		req.Expr,
		topKDecision.CandidateTopK,
		topKDecision.FinalTopK,
		topKDecision.TokenBudget,
		topKDecision.TruncateReason,
		"dense+sparse",
		len(denseDocs),
		len(sparseDocs),
		len(merged),
		truncatedCount,
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
		TopK:             req.CandidateTopK,
		Collection:       req.Collection,
		KBScope:          req.KBScope,
		ActiveGlobalKBID: req.KBID,
		RequestID:        req.RequestID,
		OriginalQuery:    req.OriginalQuery,
		RewriteQuery:     req.RewriteQuery,
		FinalQuery:       req.FinalQuery,
		RewriteStrategy:  req.RewriteStrategy,
		RewriteApplied:   req.RewriteApplied,
	}
	docs, err := h.retriever.RetrieveWithOptions(ctx, req.FinalQuery, opts)
	if err != nil {
		return nil, err
	}
	for _, doc := range docs {
		if doc.MetaData == nil {
			doc.MetaData = make(map[string]interface{})
		}
		doc.MetaData["route"] = routeDense
		doc.MetaData["dense_score"] = readDocScore(doc)
		attachRewriteMetadata(doc, req)
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

func (req *HybridSearchRequest) applyControlledRewrite(ctx context.Context, rewriter QueryRewriter) {
	if req == nil {
		return
	}
	if strings.TrimSpace(req.OriginalQuery) == "" {
		req.OriginalQuery = strings.TrimSpace(req.Query)
	}
	if rewriter == nil {
		req.Query = req.OriginalQuery
		req.FinalQuery = req.OriginalQuery
		req.RewriteQuery = ""
		req.RewriteStrategy = RewriteStrategyNone
		req.RewriteApplied = false
		return
	}

	result := rewriter.Rewrite(ctx, req.OriginalQuery)
	req.Query = req.OriginalQuery
	req.RewriteQuery = strings.TrimSpace(result.RewriteQuery)
	req.FinalQuery = strings.TrimSpace(result.FinalQuery)
	if req.FinalQuery == "" {
		req.FinalQuery = req.OriginalQuery
	}
	req.RewriteStrategy = formatRewriteStrategy(result)
	req.RewriteApplied = result.Applied && !strings.EqualFold(req.FinalQuery, req.OriginalQuery)
	if !req.RewriteApplied {
		req.RewriteQuery = ""
		req.FinalQuery = req.OriginalQuery
	}
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

func attachRewriteMetadata(doc *schema.Document, req *HybridSearchRequest) {
	if doc == nil || req == nil {
		return
	}
	if doc.MetaData == nil {
		doc.MetaData = make(map[string]interface{})
	}
	doc.MetaData["original_query"] = req.OriginalQuery
	doc.MetaData["rewrite_query"] = req.RewriteQuery
	doc.MetaData["final_query"] = req.FinalQuery
	doc.MetaData["rewrite_strategy"] = req.RewriteStrategy
	doc.MetaData["rewrite_applied"] = req.RewriteApplied
}
