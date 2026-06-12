package retrieval

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"

	"interview-agents/internal/observability/metrics"
)

const (
	routeDense             = "dense"
	routeSparse            = "sparse"
	HybridRetrieverVersion = "phase2-hybrid-v1"
	hybridRetrieverVersion = HybridRetrieverVersion
)

type HybridSearchRequest struct {
	Query                 string
	OriginalQuery         string
	RewriteQuery          string
	FinalQuery            string
	DenseQuery            string
	SparseQuery           string
	RewriteStrategy       string
	RewriteApplied        bool
	RouteRewriteStrategy  map[string]string
	Expr                  string
	TopK                  int
	KBScope               string
	KBID                  uint64
	Language              DocumentLanguage
	Category              DocumentCategory
	RequestID             string
	Collection            string
	CandidateTopK         int
	TermDictScope         string
	TermDictVersion       string
	TermHits              []string
	ModelRewriteApplied   bool
	ModelRewriteShadow    bool
	ModelRewriteRiskLevel string
	ModelRewriteTerms     []string
	QueryType             string
	ForceRewriteOff       bool
	ExperimentID          string
	StrategyVersion       string
	ReleaseID             string
}

type HybridRetriever struct {
	retriever       *RetrieverService
	sparseRetriever *SparseRetriever
	reranker        Reranker
	queryRewriter   QueryRewriter
	parentChild     *parentChildPostProcessor
	citationChecker *CitationConsistencyChecker
	config          *HybridRetrieverConfig
}

type HybridRetrieverConfig struct {
	CandidateTopK   int
	FusionStrategy  string
	RRFK            int
	RRFDenseWeight  float64
	RRFSparseWeight float64
	DenseWeight     float64
	SparseWeight    float64
	SparseConfig    *SparseRetrieverConfig
	RerankerImpl    Reranker
	QueryRewriter   QueryRewriter
	DynamicTopK     DynamicTopKConfig
	ParentChild     ParentChildConfig
	EvidenceGate    EvidenceGateConfig
	CitationCheck   CitationConsistencyConfig
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
	if strings.TrimSpace(config.FusionStrategy) == "" {
		config.FusionStrategy = "minmax_v1"
	}
	if config.RRFK <= 0 {
		config.RRFK = 60
	}
	if config.RRFDenseWeight <= 0 {
		config.RRFDenseWeight = config.DenseWeight
	}
	if config.RRFSparseWeight <= 0 {
		config.RRFSparseWeight = config.SparseWeight
	}

	sparseRetriever, err := NewSparseRetriever(retriever.client, retriever.config.Collection, config.SparseConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to init sparse retriever: %w", err)
	}

	hr := &HybridRetriever{
		retriever:       retriever,
		sparseRetriever: sparseRetriever,
		queryRewriter:   config.QueryRewriter,
		parentChild:     newParentChildPostProcessor(retriever.client, retriever.config.Collection, config.ParentChild),
		citationChecker: NewCitationConsistencyChecker(config.CitationCheck),
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
	result, err := h.SearchWithMetrics(ctx, query, opts)
	if err != nil {
		return nil, err
	}
	return result.Documents, nil
}

func (h *HybridRetriever) SearchWithMetrics(ctx context.Context, query string, opts *RetrieveOptions) (*SearchResult, error) {
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
		Query:                query,
		OriginalQuery:        query,
		FinalQuery:           query,
		DenseQuery:           query,
		SparseQuery:          query,
		RouteRewriteStrategy: map[string]string{},
		TopK:                 finalTopK,
		RequestID:            requestID,
		Collection:           h.retriever.config.Collection,
		CandidateTopK:        candidateTopK,
	}
	if opts != nil {
		req.Expr = strings.TrimSpace(BuildFilterExpr(opts))
		if req.Expr == "" {
			req.Expr = strings.TrimSpace(opts.Expr)
		}
		req.KBScope = strings.TrimSpace(opts.KBScope)
		req.KBID = opts.ActiveGlobalKBID
		req.Language = opts.Language
		req.Category = opts.Category
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
		req.DenseQuery = firstNonEmptyQuery(req.DenseQuery, req.FinalQuery, req.OriginalQuery)
		req.SparseQuery = firstNonEmptyQuery(req.SparseQuery, req.FinalQuery, req.OriginalQuery)
		if strings.TrimSpace(opts.RewriteStrategy) != "" {
			req.RewriteStrategy = strings.TrimSpace(opts.RewriteStrategy)
		}
		req.RewriteApplied = opts.RewriteApplied
		req.QueryType = strings.TrimSpace(opts.QueryType)
		req.ForceRewriteOff = opts.ForceRewriteOff
		req.ExperimentID = strings.TrimSpace(opts.ExperimentID)
		req.StrategyVersion = strings.TrimSpace(opts.StrategyVersion)
		req.ReleaseID = strings.TrimSpace(opts.ReleaseID)
	}

	return h.SearchWithRequestAndMetrics(ctx, req)
}

func (h *HybridRetriever) SearchWithRequest(ctx context.Context, req *HybridSearchRequest) ([]*schema.Document, error) {
	result, err := h.SearchWithRequestAndMetrics(ctx, req)
	if err != nil {
		return nil, err
	}
	return result.Documents, nil
}

func (h *HybridRetriever) SearchWithRequestAndMetrics(ctx context.Context, req *HybridSearchRequest) (*SearchResult, error) {
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
	if strings.TrimSpace(req.DenseQuery) == "" {
		req.DenseQuery = req.FinalQuery
	}
	if strings.TrimSpace(req.SparseQuery) == "" {
		req.SparseQuery = req.FinalQuery
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
		metrics  SearchMetrics
		err      error
		duration time.Duration
	}

	resultCh := make(chan routeResult, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		routeStart := time.Now()
		denseResult, err := h.searchDenseWithMetrics(ctx, req)
		routeRes := routeResult{route: routeDense, err: err, duration: time.Since(routeStart)}
		if denseResult != nil {
			routeRes.docs = denseResult.Documents
			routeRes.metrics = denseResult.Metrics
		}
		resultCh <- routeRes
	}()

	go func() {
		defer wg.Done()
		routeStart := time.Now()
		docs, sparseStats, err := h.sparseRetriever.Search(ctx, req)
		resultCh <- routeResult{
			route:    routeSparse,
			docs:     docs,
			err:      err,
			duration: time.Since(routeStart),
			metrics: SearchMetrics{
				SparseHits:            len(docs),
				SparseTerms:           append([]string(nil), sparseStats.Terms...),
				SparseFallbackReason:  sparseStats.FallbackReason,
				TermSources:           copyStringMap(sparseStats.TermSources),
				DroppedTerms:          copyStringMap(sparseStats.DroppedTerms),
				SparseCandidateBefore: sparseStats.CandidateCountBefore,
				SparseCandidateAfter:  sparseStats.CandidateCountAfter,
			},
		}
	}()

	wg.Wait()
	close(resultCh)

	var (
		denseDocs    []*schema.Document
		sparseDocs   []*schema.Document
		denseErr     error
		sparseErr    error
		denseMS      int64
		sparseMS     int64
		denseMetric  SearchMetrics
		sparseMetric SearchMetrics
	)
	for routeRes := range resultCh {
		switch routeRes.route {
		case routeDense:
			denseDocs = routeRes.docs
			denseErr = routeRes.err
			denseMS = routeRes.duration.Milliseconds()
			denseMetric = routeRes.metrics
		case routeSparse:
			sparseDocs = routeRes.docs
			sparseErr = routeRes.err
			sparseMS = routeRes.duration.Milliseconds()
			sparseMetric = routeRes.metrics
		}
		h.observeRouteMetric(routeRes.route, routeRes.duration, routeRes.err, len(routeRes.docs))
	}

	debugTrace := &DebugTrace{
		RouteHits: []RouteDebugHit{
			{
				Route:     routeDense,
				Query:     req.DenseQuery,
				Hits:      SnapshotDocuments(denseDocs),
				HitsCount: len(denseDocs),
				LatencyMs: denseMS,
				Error:     toLogError(denseErr),
			},
			{
				Route:                routeSparse,
				Query:                req.SparseQuery,
				Hits:                 SnapshotDocuments(sparseDocs),
				HitsCount:            len(sparseDocs),
				SparseTerms:          append([]string(nil), sparseMetric.SparseTerms...),
				FallbackReason:       sparseMetric.SparseFallbackReason,
				TermSources:          copyStringMap(sparseMetric.TermSources),
				DroppedTerms:         copyStringMap(sparseMetric.DroppedTerms),
				CandidateCountBefore: sparseMetric.SparseCandidateBefore,
				CandidateCountAfter:  sparseMetric.SparseCandidateAfter,
				LatencyMs:            sparseMS,
				Error:                toLogError(sparseErr),
			},
		},
		StageDurations: map[string]int64{
			"dense_ms":  denseMS,
			"sparse_ms": sparseMS,
		},
	}
	if denseErr != nil || sparseErr != nil {
		debugTrace.Degradation = DegradationDebugInfo{
			Enabled:          true,
			Reason:           "route_error",
			FallbackStrategy: "continue_with_available_routes",
			ErrorCode:        firstNonEmptyString(errorCodeFromRouteErr(denseErr), errorCodeFromRouteErr(sparseErr)),
		}
	}

	if denseErr != nil && sparseErr != nil {
		totalMS := time.Since(start).Milliseconds()
		log.Printf(
			"[RAG:L1] request_id=%s query=%q rewrite=%q final_query=%q rewrite_strategy=%q rewrite_applied=%t expr=%q candidate_topk=%d final_topk=%d token_budget=%d truncate_reason=%q topk_policy_version=%q score_distribution=%q rerank_gap=%.4f evidence_density=%.4f topk_decision_reason=%q routes=%s route_hits={dense:0,sparse:0} final_count=0 empty_reason=empty-after-retrieve duration_ms=%d dense_error=%q sparse_error=%q",
			req.RequestID, req.OriginalQuery, req.RewriteQuery, req.FinalQuery, req.RewriteStrategy, req.RewriteApplied, req.Expr, topKDecision.CandidateTopK, topKDecision.FinalTopK, topKDecision.TokenBudget, topKDecision.TruncateReason, topKDecision.PolicyVersion, topKDecision.ScoreDistribution, topKDecision.RerankGap, topKDecision.EvidenceDensity, topKDecision.DecisionReason, "dense+sparse", totalMS, denseErr.Error(), sparseErr.Error(),
		)
		return nil, fmt.Errorf("hybrid retrieval failed: dense=%v sparse=%v", denseErr, sparseErr)
	}

	rawCandidateCount := len(denseDocs) + len(sparseDocs)
	if rawCandidateCount == 0 {
		evidenceOutcome := h.evaluateEvidenceGate(req.FinalQuery, nil, topKDecision, CitationConsistencyOutcome{})
		debugTrace.Fusion = FusionDebugInfo{
			Before: SnapshotDocuments(append(append([]*schema.Document(nil), denseDocs...), sparseDocs...)),
			After:  nil,
		}
		totalMS := time.Since(start).Milliseconds()
		log.Printf(
			"[RAG:L2] request_id=%s query=%q rewrite=%q final_query=%q rewrite_strategy=%q rewrite_applied=%t expr=%q candidate_topk=%d final_topk=%d token_budget=%d truncate_reason=%q topk_policy_version=%q score_distribution=%q rerank_gap=%.4f evidence_density=%.4f topk_decision_reason=%q evidence_gate_result=%q refusal_reason=%q citation_support_score=%.4f evidence_gate_error=%q routes=%s route_hits={dense:%d,sparse:%d} final_count=0 empty_reason=%s duration_ms=%d dense_ms=%d sparse_ms=%d dense_error=%q sparse_error=%q",
			req.RequestID, req.OriginalQuery, req.RewriteQuery, req.FinalQuery, req.RewriteStrategy, req.RewriteApplied, req.Expr, topKDecision.CandidateTopK, topKDecision.FinalTopK, topKDecision.TokenBudget, topKDecision.TruncateReason, topKDecision.PolicyVersion, topKDecision.ScoreDistribution, topKDecision.RerankGap, topKDecision.EvidenceDensity, topKDecision.DecisionReason, evidenceOutcome.Result, evidenceOutcome.RefusalReason, evidenceOutcome.CitationSupportScore, evidenceOutcome.Error, "dense+sparse", len(denseDocs), len(sparseDocs), EmptyReasonAfterRetrieve, totalMS, denseMS, sparseMS, toLogError(denseErr), toLogError(sparseErr),
		)
		return &SearchResult{
			Documents: []*schema.Document{},
			Metrics: SearchMetrics{
				EmbeddingMs:           denseMetric.EmbeddingMs,
				SearchMs:              denseMetric.SearchMs + sparseMS,
				PostprocessMs:         maxInt64(0, totalMS-denseMetric.EmbeddingMs-(denseMetric.SearchMs+sparseMS)),
				HitCount:              0,
				CandidateTopK:         topKDecision.CandidateTopK,
				FinalTopK:             0,
				TokenBudget:           topKDecision.TokenBudget,
				TruncateReason:        topKDecision.TruncateReason,
				Strategy:              "phase2",
				RetrieverVersion:      HybridRetrieverVersion,
				FusionStrategy:        h.config.FusionStrategy,
				RRFK:                  h.config.RRFK,
				RewriteStrategy:       req.RewriteStrategy,
				RewriteApplied:        req.RewriteApplied,
				OriginalQuery:         req.OriginalQuery,
				RewriteQuery:          req.RewriteQuery,
				FinalQuery:            req.FinalQuery,
				DenseQuery:            req.DenseQuery,
				SparseQuery:           req.SparseQuery,
				RouteRewriteDense:     resolveRouteRewriteStrategyFromRequest(req, routeDense),
				RouteRewriteSparse:    resolveRouteRewriteStrategyFromRequest(req, routeSparse),
				TermDictScope:         req.TermDictScope,
				TermDictVersion:       req.TermDictVersion,
				TermHits:              append([]string(nil), req.TermHits...),
				ModelRewriteApplied:   req.ModelRewriteApplied,
				ModelRewriteShadow:    req.ModelRewriteShadow,
				ModelRewriteRiskLevel: req.ModelRewriteRiskLevel,
				ModelRewriteTerms:     append([]string(nil), req.ModelRewriteTerms...),
				EmptyReason:           EmptyReasonAfterRetrieve,
				DenseHits:             len(denseDocs),
				SparseHits:            len(sparseDocs),
				SparseTerms:           append([]string(nil), sparseMetric.SparseTerms...),
				SparseFallbackReason:  sparseMetric.SparseFallbackReason,
				TermSources:           copyStringMap(sparseMetric.TermSources),
				DroppedTerms:          copyStringMap(sparseMetric.DroppedTerms),
				SparseCandidateBefore: sparseMetric.SparseCandidateBefore,
				SparseCandidateAfter:  sparseMetric.SparseCandidateAfter,
				ParentChildEnabled:    h.parentChild != nil,
				ParentFillStrategy:    h.parentChildStrategy(),
				EvidenceGateResult:    evidenceOutcome.Result,
				RefusalReason:         evidenceOutcome.RefusalReason,
				CitationSupportScore:  evidenceOutcome.CitationSupportScore,
				EvidenceGateError:     evidenceOutcome.Error,
			},
			Debug: debugTrace,
		}, nil
	}

	fusionBefore := append(append([]*schema.Document(nil), denseDocs...), sparseDocs...)
	fused := FuseRouteCandidates(denseDocs, sparseDocs, FusionConfig{
		FusionStrategy:  h.config.FusionStrategy,
		RRFK:            h.config.RRFK,
		DenseWeight:     h.config.DenseWeight,
		SparseWeight:    h.config.SparseWeight,
		RRFDenseWeight:  h.config.RRFDenseWeight,
		RRFSparseWeight: h.config.RRFSparseWeight,
	})
	debugTrace.Fusion = FusionDebugInfo{
		Before: SnapshotDocuments(fusionBefore),
		After:  SnapshotFusedDocuments(fused),
	}
	if len(fused) == 0 {
		evidenceOutcome := h.evaluateEvidenceGate(req.FinalQuery, nil, topKDecision, CitationConsistencyOutcome{})
		totalMS := time.Since(start).Milliseconds()
		log.Printf(
			"[RAG:L2] request_id=%s query=%q rewrite=%q final_query=%q rewrite_strategy=%q rewrite_applied=%t expr=%q candidate_topk=%d final_topk=%d token_budget=%d truncate_reason=%q topk_policy_version=%q score_distribution=%q rerank_gap=%.4f evidence_density=%.4f topk_decision_reason=%q evidence_gate_result=%q refusal_reason=%q citation_support_score=%.4f evidence_gate_error=%q routes=%s route_hits={dense:%d,sparse:%d} final_count=0 empty_reason=%s duration_ms=%d dense_ms=%d sparse_ms=%d dense_error=%q sparse_error=%q",
			req.RequestID, req.OriginalQuery, req.RewriteQuery, req.FinalQuery, req.RewriteStrategy, req.RewriteApplied, req.Expr, topKDecision.CandidateTopK, topKDecision.FinalTopK, topKDecision.TokenBudget, topKDecision.TruncateReason, topKDecision.PolicyVersion, topKDecision.ScoreDistribution, topKDecision.RerankGap, topKDecision.EvidenceDensity, topKDecision.DecisionReason, evidenceOutcome.Result, evidenceOutcome.RefusalReason, evidenceOutcome.CitationSupportScore, evidenceOutcome.Error, "dense+sparse", len(denseDocs), len(sparseDocs), EmptyReasonAfterFusion, totalMS, denseMS, sparseMS, toLogError(denseErr), toLogError(sparseErr),
		)
		return h.buildHybridResultMetrics(req, denseMetric, sparseMetric, len(denseDocs), len(sparseDocs), sparseMS, topKDecision, totalMS, nil, EmptyReasonAfterFusion, evidenceOutcome, debugTrace), nil
	}

	merged := DeduplicateFusedDocuments(fused)
	debugTrace.Dedupe = DedupeDebugInfo{
		BeforeCount: len(fused),
		AfterCount:  len(merged),
		Removed:     RemovedFusedSnapshots(fused, merged),
	}
	if len(merged) == 0 {
		evidenceOutcome := h.evaluateEvidenceGate(req.FinalQuery, nil, topKDecision, CitationConsistencyOutcome{})
		totalMS := time.Since(start).Milliseconds()
		log.Printf(
			"[RAG:L2] request_id=%s query=%q rewrite=%q final_query=%q rewrite_strategy=%q rewrite_applied=%t expr=%q candidate_topk=%d final_topk=%d token_budget=%d truncate_reason=%q topk_policy_version=%q score_distribution=%q rerank_gap=%.4f evidence_density=%.4f topk_decision_reason=%q evidence_gate_result=%q refusal_reason=%q citation_support_score=%.4f evidence_gate_error=%q routes=%s route_hits={dense:%d,sparse:%d} final_count=0 empty_reason=%s duration_ms=%d dense_ms=%d sparse_ms=%d dense_error=%q sparse_error=%q",
			req.RequestID, req.OriginalQuery, req.RewriteQuery, req.FinalQuery, req.RewriteStrategy, req.RewriteApplied, req.Expr, topKDecision.CandidateTopK, topKDecision.FinalTopK, topKDecision.TokenBudget, topKDecision.TruncateReason, topKDecision.PolicyVersion, topKDecision.ScoreDistribution, topKDecision.RerankGap, topKDecision.EvidenceDensity, topKDecision.DecisionReason, evidenceOutcome.Result, evidenceOutcome.RefusalReason, evidenceOutcome.CitationSupportScore, evidenceOutcome.Error, "dense+sparse", len(denseDocs), len(sparseDocs), EmptyReasonAfterFusion, totalMS, denseMS, sparseMS, toLogError(denseErr), toLogError(sparseErr),
		)
		return h.buildHybridResultMetrics(req, denseMetric, sparseMetric, len(denseDocs), len(sparseDocs), sparseMS, topKDecision, totalMS, nil, EmptyReasonAfterFusion, evidenceOutcome, debugTrace), nil
	}

	beforeRerank := append([]*schema.Document(nil), merged...)
	rerankModel := ""
	rerankVersion := ""
	rerankFallback := false
	rerankReason := ""
	var rerankMS int64
	if h.reranker != nil {
		reranked, err := h.reranker.Rerank(ctx, req.OriginalQuery, merged)
		if err != nil {
			log.Printf("[RAG:L5] request_id=%s rerank_failed=true rerank_error=%q", req.RequestID, err.Error())
		} else if reranked != nil {
			rerankModel = reranked.Model
			rerankVersion = reranked.Version
			rerankFallback = reranked.Fallback
			rerankReason = reranked.Reason
			rerankMS = reranked.Latency.Milliseconds()
			if len(reranked.Documents) > 0 {
				merged = reranked.Documents
			}
		}
	}
	debugTrace.Rerank = RerankDebugInfo{
		Before:        SnapshotDocuments(beforeRerank),
		After:         SnapshotDocuments(merged),
		RerankModel:   rerankModel,
		RerankVersion: rerankVersion,
		Fallback:      rerankFallback,
		Reason:        rerankReason,
	}
	topKDecision = DecideStrategicTopK(req.FinalQuery, req.CandidateTopK, req.TopK, merged, h.config.DynamicTopK)

	emptyReason := EmptyReasonNone
	if len(merged) == 0 {
		emptyReason = EmptyReasonAfterFilter
	}

	parentFillStats := ParentChildFillStats{}
	beforeParentFill := append([]*schema.Document(nil), merged...)
	if h.parentChild != nil {
		merged, parentFillStats = h.parentChild.Fill(ctx, merged)
	}
	debugTrace.ParentChild = ParentChildDebugInfo{
		ChildHits:      SnapshotDocuments(beforeParentFill),
		ParentContexts: ParentContextSnapshots(merged),
	}
	if parentFillStats.FallbackCount > 0 {
		debugTrace.ParentChild.FallbackReason = "partial_parent_fill_fallback"
	}

	beforeTruncateCount := len(merged)
	beforeFilter := append([]*schema.Document(nil), merged...)
	merged, topKDecision = ApplyTokenBudgetGuard(merged, topKDecision, h.config.DynamicTopK)
	truncatedCount := beforeTruncateCount - len(merged)
	if len(merged) == 0 && emptyReason == EmptyReasonNone {
		if h.parentChild != nil && beforeTruncateCount > 0 {
			emptyReason = EmptyReasonAfterParentBudget
		} else {
			emptyReason = EmptyReasonAfterFilter
		}
	}

	citationCandidateCount := len(merged)
	citationOutcome := h.evaluateCitationConsistency(req.FinalQuery, merged)
	merged, citationOutcome = h.tryRefineUnsupportedCitations(req.FinalQuery, merged, citationOutcome)
	truncatedCount += citationCandidateCount - len(merged)
	debugTrace.Filter = FilterDebugInfo{
		BeforeCount:    len(beforeFilter),
		AfterCount:     len(merged),
		Removed:        RemovedSnapshots(beforeFilter, merged),
		DropReasons:    buildFilterDropReasons(beforeFilter, merged, topKDecision.TruncateReason),
		TruncateReason: topKDecision.TruncateReason,
	}
	evidenceOutcome := h.evaluateEvidenceGate(req.FinalQuery, merged, topKDecision, citationOutcome)
	totalMS := time.Since(start).Milliseconds()
	debugTrace.FinalResults = SnapshotDocuments(merged)
	debugTrace.StageDurations["rerank_ms"] = rerankMS
	debugTrace.StageDurations["total_ms"] = totalMS
	if rerankFallback {
		debugTrace.Degradation = DegradationDebugInfo{
			Enabled:          true,
			Reason:           firstNonEmptyString(rerankReason, "rerank_fallback"),
			FallbackStrategy: "use_pre_rerank_order",
		}
	}
	if evidenceOutcome.Result == EvidenceGateResultDegradedPass {
		debugTrace.Degradation = DegradationDebugInfo{
			Enabled:          true,
			Reason:           firstNonEmptyString(evidenceOutcome.Error, "evidence_gate_degraded"),
			FallbackStrategy: "return_results_without_strict_evidence_gate",
			ErrorCode:        "evidence_gate_degraded",
		}
	}
	splitStrategySummary := summarizeMetadataValues(merged, "split_strategy")
	embeddingStrategySummary := summarizeMetadataValues(merged, "embedding_build_strategy")
	contextVersionSummary := summarizeMetadataValues(merged, "context_version")
	semanticSplitEnabled := anyMetadataBool(merged, "semantic_split_enabled")
	log.Printf(
		"[RAG:L2] request_id=%s query=%q rewrite=%q final_query=%q rewrite_strategy=%q rewrite_applied=%t expr=%q candidate_topk=%d final_topk=%d token_budget=%d token_budget_remaining=%d context_tokens=%d truncate_reason=%q topk_policy_version=%q score_distribution=%q rerank_gap=%.4f evidence_density=%.4f topk_decision_reason=%q evidence_gate_result=%q refusal_reason=%q citation_supported=%t citation_support_score=%.4f unsupported_claim_count=%d citation_check_version=%q citation_check_latency_ms=%d citation_check_error=%q evidence_gate_error=%q split_strategy=%q embedding_build_strategy=%q context_version=%q semantic_split_enabled=%t routes=%s route_hits={dense:%d,sparse:%d} final_count=%d truncated_count=%d empty_reason=%s parent_fill_strategy=%q parent_fill_count=%d parent_fill_fallback=%d parent_fill_tokens=%d parent_fill_reason=%q duration_ms=%d dense_ms=%d sparse_ms=%d rerank_ms=%d rerank_model=%q rerank_version=%q rerank_fallback=%t rerank_reason=%q dense_error=%q sparse_error=%q",
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
		topKDecision.TokenBudgetRemaining,
		topKDecision.EstimatedContextTokens,
		topKDecision.TruncateReason,
		topKDecision.PolicyVersion,
		topKDecision.ScoreDistribution,
		topKDecision.RerankGap,
		topKDecision.EvidenceDensity,
		topKDecision.DecisionReason,
		evidenceOutcome.Result,
		evidenceOutcome.RefusalReason,
		citationOutcome.Supported,
		evidenceOutcome.CitationSupportScore,
		len(citationOutcome.UnsupportedClaims),
		citationOutcome.Version,
		citationOutcome.Latency.Milliseconds(),
		citationOutcome.Error,
		evidenceOutcome.Error,
		splitStrategySummary,
		embeddingStrategySummary,
		contextVersionSummary,
		semanticSplitEnabled,
		"dense+sparse",
		len(denseDocs),
		len(sparseDocs),
		len(merged),
		truncatedCount,
		emptyReason,
		parentFillStats.Strategy,
		parentFillStats.FilledCount,
		parentFillStats.FallbackCount,
		parentFillStats.FilledTokens,
		parentFillStats.Reason,
		totalMS,
		denseMS,
		sparseMS,
		rerankMS,
		rerankModel,
		rerankVersion,
		rerankFallback,
		rerankReason,
		toLogError(denseErr),
		toLogError(sparseErr),
	)

	result := h.buildHybridResultMetrics(req, denseMetric, sparseMetric, len(denseDocs), len(sparseDocs), sparseMS, topKDecision, totalMS, merged, emptyReason, evidenceOutcome, debugTrace)
	result.Metrics.CitationSupported = citationOutcome.Supported
	result.Metrics.UnsupportedClaims = append([]string(nil), citationOutcome.UnsupportedClaims...)
	result.Metrics.UnsupportedClaimCount = len(citationOutcome.UnsupportedClaims)
	result.Metrics.CitationCheckVersion = citationOutcome.Version
	result.Metrics.CitationCheckLatencyMs = citationOutcome.Latency.Milliseconds()
	result.Metrics.CitationCheckError = citationOutcome.Error
	result.Metrics.TruncatedCount = truncatedCount
	result.Metrics.RerankMs = rerankMS
	result.Metrics.RerankModel = rerankModel
	result.Metrics.RerankVersion = rerankVersion
	result.Metrics.RerankFallback = rerankFallback
	result.Metrics.RerankReason = rerankReason
	result.Metrics.ParentChildEnabled = h.parentChild != nil
	result.Metrics.ParentFillStrategy = parentFillStats.Strategy
	result.Metrics.ParentFillCount = parentFillStats.FilledCount
	result.Metrics.ParentFillFallback = parentFillStats.FallbackCount
	result.Metrics.ParentFillTokens = parentFillStats.FilledTokens
	result.Metrics.ParentFillReason = parentFillStats.Reason
	return result, nil
}

func (h *HybridRetriever) buildHybridResultMetrics(
	req *HybridSearchRequest,
	denseMetric SearchMetrics,
	sparseMetric SearchMetrics,
	denseHits, sparseHits int,
	sparseMS int64,
	topKDecision TopKDecision,
	totalMS int64,
	docs []*schema.Document,
	emptyReason string,
	evidenceOutcome EvidenceGateOutcome,
	debugTrace *DebugTrace,
) *SearchResult {
	routeSummary := summarizeFinalRouteStats(docs)
	searchStageMS := denseMetric.SearchMs + sparseMS
	if debugTrace != nil {
		for index := range debugTrace.RouteHits {
			switch debugTrace.RouteHits[index].Route {
			case routeDense:
				debugTrace.RouteHits[index].ParticipationCount = routeSummary.DenseParticipation
				debugTrace.RouteHits[index].PrimaryCount = routeSummary.PrimaryDenseCount
				debugTrace.RouteHits[index].Contribution = routeSummary.PrimaryDenseCount
			case routeSparse:
				debugTrace.RouteHits[index].ParticipationCount = routeSummary.SparseParticipation
				debugTrace.RouteHits[index].PrimaryCount = routeSummary.PrimarySparseCount
				debugTrace.RouteHits[index].Contribution = routeSummary.PrimarySparseCount
			}
		}
		debugTrace.Fusion.DenseParticipationCount = routeSummary.DenseParticipation
		debugTrace.Fusion.SparseParticipationCount = routeSummary.SparseParticipation
		debugTrace.Fusion.DualRouteFinalCount = routeSummary.DualRouteFinalCount
		debugTrace.Fusion.PrimaryRouteDistribution = map[string]int{
			routeDense:  routeSummary.PrimaryDenseCount,
			routeSparse: routeSummary.PrimarySparseCount,
		}
	}
	return &SearchResult{
		Documents: docs,
		Metrics: SearchMetrics{
			EmbeddingMs:           denseMetric.EmbeddingMs,
			SearchMs:              searchStageMS,
			PostprocessMs:         maxInt64(0, totalMS-denseMetric.EmbeddingMs-searchStageMS),
			HitCount:              denseHits + sparseHits,
			CandidateTopK:         topKDecision.CandidateTopK,
			FinalTopK:             len(docs),
			TokenBudget:           topKDecision.TokenBudget,
			TruncateReason:        topKDecision.TruncateReason,
			Strategy:              resolveRetrieveStrategy(h.parentChild),
			RetrieverVersion:      HybridRetrieverVersion,
			FusionStrategy:        h.config.FusionStrategy,
			RRFK:                  h.config.RRFK,
			RewriteStrategy:       req.RewriteStrategy,
			RewriteApplied:        req.RewriteApplied,
			OriginalQuery:         req.OriginalQuery,
			RewriteQuery:          req.RewriteQuery,
			FinalQuery:            req.FinalQuery,
			DenseQuery:            req.DenseQuery,
			SparseQuery:           req.SparseQuery,
			RouteRewriteDense:     resolveRouteRewriteStrategyFromRequest(req, routeDense),
			RouteRewriteSparse:    resolveRouteRewriteStrategyFromRequest(req, routeSparse),
			TermDictScope:         req.TermDictScope,
			TermDictVersion:       req.TermDictVersion,
			TermHits:              append([]string(nil), req.TermHits...),
			ModelRewriteApplied:   req.ModelRewriteApplied,
			ModelRewriteShadow:    req.ModelRewriteShadow,
			ModelRewriteRiskLevel: req.ModelRewriteRiskLevel,
			ModelRewriteTerms:     append([]string(nil), req.ModelRewriteTerms...),
			EmptyReason:           emptyReason,
			DenseHits:             denseHits,
			SparseHits:            sparseHits,
			DenseParticipation:    routeSummary.DenseParticipation,
			SparseParticipation:   routeSummary.SparseParticipation,
			PrimaryDenseCount:     routeSummary.PrimaryDenseCount,
			PrimarySparseCount:    routeSummary.PrimarySparseCount,
			DualRouteFinalCount:   routeSummary.DualRouteFinalCount,
			DenseContribution:     routeSummary.PrimaryDenseCount,
			SparseContribution:    routeSummary.PrimarySparseCount,
			SparseTerms:           append([]string(nil), sparseMetric.SparseTerms...),
			TermSources:           copyStringMap(sparseMetric.TermSources),
			DroppedTerms:          copyStringMap(sparseMetric.DroppedTerms),
			SparseCandidateBefore: sparseMetric.SparseCandidateBefore,
			SparseCandidateAfter:  sparseMetric.SparseCandidateAfter,
			TopKPolicyVersion:     topKDecision.PolicyVersion,
			ScoreDistribution:     topKDecision.ScoreDistribution,
			RerankGap:             topKDecision.RerankGap,
			EvidenceDensity:       topKDecision.EvidenceDensity,
			TopKDecisionReason:    topKDecision.DecisionReason,
			TokenBudgetRemain:     topKDecision.TokenBudgetRemaining,
			ContextTokens:         topKDecision.EstimatedContextTokens,
			ParentChildEnabled:    h.parentChild != nil,
			ParentFillStrategy:    h.parentChildStrategy(),
			EvidenceGateResult:    evidenceOutcome.Result,
			RefusalReason:         evidenceOutcome.RefusalReason,
			CitationSupportScore:  evidenceOutcome.CitationSupportScore,
			EvidenceGateError:     evidenceOutcome.Error,
		},
		Debug: debugTrace,
	}
}

func (h *HybridRetriever) parentChildStrategy() string {
	if h == nil || h.parentChild == nil {
		return ""
	}
	return strings.TrimSpace(h.parentChild.config.FillStrategy)
}

func resolveRetrieveStrategy(parentChild *parentChildPostProcessor) string {
	if parentChild != nil {
		return "phase3-parent-child"
	}
	return "phase2"
}

type finalRouteStats struct {
	DenseParticipation  int
	SparseParticipation int
	PrimaryDenseCount   int
	PrimarySparseCount  int
	DualRouteFinalCount int
}

func summarizeFinalRouteStats(docs []*schema.Document) finalRouteStats {
	stats := finalRouteStats{}
	for _, doc := range docs {
		if doc == nil || doc.MetaData == nil {
			continue
		}
		routeContrib := getRouteContrib(doc.MetaData)
		_, hasDense := routeContrib[routeDense]
		_, hasSparse := routeContrib[routeSparse]
		if hasDense {
			stats.DenseParticipation++
		}
		if hasSparse {
			stats.SparseParticipation++
		}
		if hasDense && hasSparse {
			stats.DualRouteFinalCount++
		}
		switch strings.TrimSpace(strings.ToLower(getStringMetadata(doc.MetaData, "route"))) {
		case routeSparse:
			stats.PrimarySparseCount++
		default:
			stats.PrimaryDenseCount++
		}
	}
	return stats
}

func summarizeMetadataValues(docs []*schema.Document, key string) string {
	if len(docs) == 0 {
		return ""
	}
	seen := make(map[string]struct{})
	values := make([]string, 0, len(docs))
	for _, doc := range docs {
		if doc == nil || doc.MetaData == nil {
			continue
		}
		value := getStringMetadata(doc.MetaData, key)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	sort.Strings(values)
	return strings.Join(values, ",")
}

func anyMetadataBool(docs []*schema.Document, key string) bool {
	for _, doc := range docs {
		if doc == nil || doc.MetaData == nil {
			continue
		}
		if debugBoolMetadata(doc.MetaData, key) {
			return true
		}
	}
	return false
}

func getRouteContrib(metadata map[string]interface{}) map[string]float64 {
	result := make(map[string]float64)
	if metadata == nil {
		return result
	}
	raw, ok := metadata["route_contrib"].(map[string]interface{})
	if !ok {
		return result
	}
	for key, value := range raw {
		if score, ok := castScore(value); ok {
			result[strings.TrimSpace(strings.ToLower(key))] = score
		}
	}
	return result
}

func buildFilterDropReasons(before, after []*schema.Document, truncateReason string) map[string]int {
	if len(before) == 0 {
		return nil
	}
	afterKeys := make(map[string]struct{}, len(after))
	for _, doc := range after {
		if doc == nil {
			continue
		}
		afterKeys[debugDocumentKey(doc)] = struct{}{}
	}
	removed := 0
	for _, doc := range before {
		if doc == nil {
			continue
		}
		if _, ok := afterKeys[debugDocumentKey(doc)]; ok {
			continue
		}
		removed++
	}
	if removed == 0 {
		return nil
	}
	reason := strings.TrimSpace(truncateReason)
	if reason == "" {
		reason = "removed_after_filter"
	}
	return map[string]int{reason: removed}
}

func copyStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func (h *HybridRetriever) searchDenseWithMetrics(ctx context.Context, req *HybridSearchRequest) (*SearchResult, error) {
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
	denseQuery := firstNonEmptyQuery(req.DenseQuery, req.FinalQuery, req.OriginalQuery)
	result, err := h.retriever.RetrieveWithOptionsAndMetrics(ctx, denseQuery, opts)
	if err != nil {
		return nil, err
	}
	for _, doc := range result.Documents {
		if doc.MetaData == nil {
			doc.MetaData = make(map[string]interface{})
		}
		doc.MetaData["route"] = routeDense
		doc.MetaData["dense_score"] = readDocScore(doc)
		doc.MetaData["retriever_version"] = HybridRetrieverVersion
		source := ensureSourceMetadata(doc)
		source["route"] = routeDense
		source["retriever_version"] = HybridRetrieverVersion
		if collection := readCollectionFromDoc(doc); collection != "" {
			source["collection"] = collection
		}
		doc.MetaData["source"] = source
		annotateParentChildSource(doc)
		attachRewriteMetadata(doc, req, routeDense)
	}
	result.Metrics.RetrieverVersion = HybridRetrieverVersion
	result.Metrics.Strategy = "phase2"
	result.Metrics.RewriteStrategy = req.RewriteStrategy
	result.Metrics.RewriteApplied = req.RewriteApplied
	result.Metrics.DenseHits = len(result.Documents)
	return result, nil
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

func (h *HybridRetriever) evaluateEvidenceGate(query string, docs []*schema.Document, topKDecision TopKDecision, citationOutcome CitationConsistencyOutcome) EvidenceGateOutcome {
	if h == nil {
		return EvidenceGateOutcome{Result: EvidenceGateResultDisabled}
	}
	return EvaluateEvidenceGate(query, docs, SearchMetrics{
		EvidenceDensity:        topKDecision.EvidenceDensity,
		CitationSupportScore:   citationOutcome.SupportScore,
		CitationSupported:      citationOutcome.Supported,
		UnsupportedClaims:      append([]string(nil), citationOutcome.UnsupportedClaims...),
		UnsupportedClaimCount:  len(citationOutcome.UnsupportedClaims),
		CitationCheckVersion:   citationOutcome.Version,
		CitationCheckLatencyMs: citationOutcome.Latency.Milliseconds(),
		CitationCheckError:     citationOutcome.Error,
	}, h.config.EvidenceGate)
}

func (h *HybridRetriever) evaluateCitationConsistency(query string, docs []*schema.Document) CitationConsistencyOutcome {
	if h == nil || h.citationChecker == nil {
		return CitationConsistencyOutcome{}
	}
	return h.citationChecker.Check(query, docs)
}

func (h *HybridRetriever) tryRefineUnsupportedCitations(query string, docs []*schema.Document, current CitationConsistencyOutcome) ([]*schema.Document, CitationConsistencyOutcome) {
	if h == nil || h.citationChecker == nil || !h.config.CitationCheck.Enabled {
		return docs, current
	}
	if current.Supported || len(docs) <= 1 {
		return docs, current
	}

	supportedDocs := make([]*schema.Document, 0, len(docs))
	for _, doc := range docs {
		if doc == nil || doc.MetaData == nil {
			continue
		}
		if value, ok := doc.MetaData["citation_supported"]; ok && castBool(value) {
			supportedDocs = append(supportedDocs, doc)
		}
	}
	if len(supportedDocs) == 0 || len(supportedDocs) == len(docs) {
		return docs, current
	}

	refined := h.citationChecker.Check(query, supportedDocs)
	if refined.SupportScore > current.SupportScore || (refined.Supported && !current.Supported) {
		return supportedDocs, refined
	}
	return docs, current
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
		req.DenseQuery = req.OriginalQuery
		req.SparseQuery = req.OriginalQuery
		req.RewriteQuery = ""
		req.RewriteStrategy = RewriteStrategyNone
		req.RewriteApplied = false
		req.RouteRewriteStrategy = map[string]string{}
		return
	}

	result := rewriter.Rewrite(ctx, QueryRewriteRequest{
		Query:         req.OriginalQuery,
		KBID:          req.KBID,
		KBScope:       req.KBScope,
		Language:      req.Language,
		Category:      req.Category,
		Collection:    req.Collection,
		RequestID:     req.RequestID,
		CandidateTopK: req.CandidateTopK,
	})
	if req.ForceRewriteOff {
		req.Query = req.OriginalQuery
		req.RewriteQuery = ""
		req.FinalQuery = req.OriginalQuery
		req.DenseQuery = req.OriginalQuery
		req.SparseQuery = req.OriginalQuery
		req.RewriteStrategy = "experiment_force_rewrite_off"
		req.RewriteApplied = false
		req.RouteRewriteStrategy = map[string]string{}
		return
	}
	req.Query = req.OriginalQuery
	req.RewriteQuery = strings.TrimSpace(result.RewriteQuery)
	req.FinalQuery = strings.TrimSpace(result.FinalQuery)
	if req.FinalQuery == "" {
		req.FinalQuery = req.OriginalQuery
	}
	req.DenseQuery = firstNonEmptyQuery(result.DenseQuery, req.FinalQuery, req.OriginalQuery)
	req.SparseQuery = firstNonEmptyQuery(result.SparseQuery, req.FinalQuery, req.OriginalQuery)
	req.RewriteStrategy = formatRewriteStrategy(result)
	req.RewriteApplied = result.Applied
	req.RouteRewriteStrategy = cloneStringMap(result.RouteStrategies)
	req.TermDictScope = result.TermDictScope
	req.TermDictVersion = result.TermDictVersion
	req.TermHits = append([]string(nil), result.TermHits...)
	req.ModelRewriteApplied = result.ModelRewriteApplied
	req.ModelRewriteShadow = result.ModelRewriteShadow
	req.ModelRewriteRiskLevel = result.ModelRewriteRiskLevel
	req.ModelRewriteTerms = append([]string(nil), result.ModelRewriteTerms...)
	if !req.RewriteApplied {
		req.RewriteQuery = ""
		req.FinalQuery = req.OriginalQuery
		req.DenseQuery = req.OriginalQuery
		req.SparseQuery = req.OriginalQuery
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

func errorCodeFromRouteErr(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case strings.Contains(strings.ToLower(err.Error()), "timeout"):
		return "timeout"
	default:
		return "route_error"
	}
}

func attachRewriteMetadata(doc *schema.Document, req *HybridSearchRequest, route string) {
	if doc == nil || req == nil {
		return
	}
	if doc.MetaData == nil {
		doc.MetaData = make(map[string]interface{})
	}
	doc.MetaData["original_query"] = req.OriginalQuery
	doc.MetaData["rewrite_query"] = req.RewriteQuery
	doc.MetaData["final_query"] = req.FinalQuery
	doc.MetaData["dense_query"] = req.DenseQuery
	doc.MetaData["sparse_query"] = req.SparseQuery
	doc.MetaData["route_final_query"] = resolveRouteQuery(req, route)
	doc.MetaData["route_rewrite_strategy"] = resolveRouteRewriteStrategyFromRequest(req, route)
	doc.MetaData["rewrite_strategy"] = req.RewriteStrategy
	doc.MetaData["rewrite_applied"] = req.RewriteApplied
	if req.TermDictScope != "" {
		doc.MetaData["term_dict_scope"] = req.TermDictScope
	}
	if req.TermDictVersion != "" {
		doc.MetaData["term_dict_version"] = req.TermDictVersion
	}
	if len(req.TermHits) > 0 {
		doc.MetaData["term_hits"] = append([]string(nil), req.TermHits...)
	}
	if req.ModelRewriteApplied {
		doc.MetaData["model_rewrite_applied"] = true
	}
	if req.ModelRewriteShadow {
		doc.MetaData["model_rewrite_shadow"] = true
	}
	if req.ModelRewriteRiskLevel != "" {
		doc.MetaData["model_rewrite_risk_level"] = req.ModelRewriteRiskLevel
	}
	if len(req.ModelRewriteTerms) > 0 {
		doc.MetaData["model_rewrite_terms"] = append([]string(nil), req.ModelRewriteTerms...)
	}
}

func resolveRouteQuery(req *HybridSearchRequest, route string) string {
	if req == nil {
		return ""
	}
	switch strings.TrimSpace(strings.ToLower(route)) {
	case routeSparse:
		return firstNonEmptyQuery(req.SparseQuery, req.FinalQuery, req.OriginalQuery)
	default:
		return firstNonEmptyQuery(req.DenseQuery, req.FinalQuery, req.OriginalQuery)
	}
}

func resolveRouteRewriteStrategyFromRequest(req *HybridSearchRequest, route string) string {
	if req == nil || len(req.RouteRewriteStrategy) == 0 {
		return ""
	}
	return strings.TrimSpace(req.RouteRewriteStrategy[strings.TrimSpace(strings.ToLower(route))])
}

func firstNonEmptyQuery(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func cloneStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func getStringMetadata(metadata map[string]interface{}, key string) string {
	if metadata == nil {
		return ""
	}
	switch value := metadata[key].(type) {
	case string:
		return value
	case []byte:
		return string(value)
	default:
		if value != nil {
			return fmt.Sprint(value)
		}
	}
	return ""
}
