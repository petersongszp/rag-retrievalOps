package kb

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"interview-agents/internal/config"
	"interview-agents/internal/milvus/retrieval"
	"interview-agents/internal/model"
)

const standardRefusalTemplateVersion = "phase3-standard-refusal-v1"

type retrievalDebugTraceResponse struct {
	RequestID         string                              `json:"request_id"`
	DebugAvailable    bool                                `json:"debug_available"`
	KBIDs             []uint64                            `json:"kb_ids,omitempty"`
	OriginalQuery     string                              `json:"original_query,omitempty"`
	RewrittenQuery    string                              `json:"rewritten_query,omitempty"`
	RewriteStrategy   string                              `json:"rewrite_strategy,omitempty"`
	RewriteGainBucket string                              `json:"rewrite_gain_bucket,omitempty"`
	TermHits          []string                            `json:"term_hits,omitempty"`
	RouteFinalQueries map[string]string                   `json:"route_final_queries,omitempty"`
	RouteHits         []retrievalDebugRouteHitResponse    `json:"route_hits,omitempty"`
	FusionResults     retrievalDebugFusionResponse        `json:"fusion_results,omitempty"`
	DedupeResults     retrievalDebugDedupeResponse        `json:"dedupe_results,omitempty"`
	RerankResults     retrievalDebugRerankResponse        `json:"rerank_results,omitempty"`
	FilterResults     retrievalDebugFilterResponse        `json:"filter_results,omitempty"`
	ParentChild       retrievalDebugParentChildResponse   `json:"parent_child"`
	TopKDecision      retrievalDebugTopKDecisionResponse  `json:"topk_decision"`
	SemanticCache     retrievalDebugSemanticCacheResponse `json:"semantic_cache"`
	EvidenceGate      retrievalDebugEvidenceGateResponse  `json:"evidence_gate"`
	CitationCheck     retrievalDebugCitationCheckResponse `json:"citation_check"`
	FinalResults      []retrieveItem                      `json:"final_results,omitempty"`
	StageDurations    map[string]int64                    `json:"stage_durations,omitempty"`
	Degradation       retrievalDebugDegradationResponse   `json:"degradation"`
	ContractGaps      []string                            `json:"contract_gaps,omitempty"`
	CreatedAt         time.Time                           `json:"created_at"`
}

type retrievalDebugRouteHitResponse struct {
	Route        string                    `json:"route"`
	Query        string                    `json:"query,omitempty"`
	Hits         []retrieval.DebugDocument `json:"hits,omitempty"`
	Contribution int                       `json:"contribution"`
	LatencyMs    int64                     `json:"latency_ms,omitempty"`
	Error        string                    `json:"error,omitempty"`
}

type retrievalDebugFusionResponse struct {
	Before []retrieval.DebugDocument `json:"before,omitempty"`
	After  []retrieval.DebugDocument `json:"after,omitempty"`
}

type retrievalDebugDedupeResponse struct {
	BeforeCount int                       `json:"before_count"`
	AfterCount  int                       `json:"after_count"`
	Removed     []retrieval.DebugDocument `json:"removed,omitempty"`
}

type retrievalDebugRerankResponse struct {
	Before        []retrieval.DebugDocument `json:"before,omitempty"`
	After         []retrieval.DebugDocument `json:"after,omitempty"`
	RerankModel   string                    `json:"rerank_model,omitempty"`
	RerankVersion string                    `json:"rerank_version,omitempty"`
	Fallback      bool                      `json:"fallback,omitempty"`
	Reason        string                    `json:"reason,omitempty"`
}

type retrievalDebugFilterResponse struct {
	BeforeCount    int                       `json:"before_count"`
	AfterCount     int                       `json:"after_count"`
	Removed        []retrieval.DebugDocument `json:"removed,omitempty"`
	TruncateReason string                    `json:"truncate_reason,omitempty"`
}

type retrievalDebugParentChildResponse struct {
	ParentChildEnabled   bool                      `json:"parent_child_enabled"`
	ParentFillStrategy   string                    `json:"parent_fill_strategy,omitempty"`
	ParentFillCount      int                       `json:"parent_fill_count"`
	ParentFillTokens     int                       `json:"parent_fill_tokens"`
	ChildHits            []retrieval.DebugDocument `json:"child_hits,omitempty"`
	ParentContexts       []retrieval.DebugDocument `json:"parent_contexts,omitempty"`
	ParentChildAvailable bool                      `json:"parent_child_available"`
	FallbackReason       string                    `json:"fallback_reason,omitempty"`
}

type retrievalDebugTopKDecisionResponse struct {
	TopKPolicyVersion    string  `json:"topk_policy_version,omitempty"`
	CandidateTopK        int     `json:"candidate_topk"`
	FinalTopK            int     `json:"final_topk"`
	ScoreDistribution    string  `json:"score_distribution,omitempty"`
	RerankGap            float64 `json:"rerank_gap,omitempty"`
	EvidenceDensity      float64 `json:"evidence_density,omitempty"`
	TokenBudget          int     `json:"token_budget"`
	TokenBudgetRemaining int     `json:"token_budget_remaining"`
	TopKDecisionReason   string  `json:"topk_decision_reason,omitempty"`
}

type retrievalDebugSemanticCacheResponse struct {
	Enabled           bool    `json:"enabled"`
	Hit               bool    `json:"hit"`
	Reason            string  `json:"reason,omitempty"`
	EntryID           string  `json:"entry_id,omitempty"`
	Similarity        float64 `json:"similarity,omitempty"`
	Threshold         float64 `json:"threshold,omitempty"`
	CandidateCount    int     `json:"candidate_count"`
	HighestSimilarity float64 `json:"highest_similarity,omitempty"`
	LookupMs          int64   `json:"lookup_ms,omitempty"`
}

type retrievalDebugEvidenceGateResponse struct {
	EvidenceGateResult     string                           `json:"evidence_gate_result,omitempty"`
	RefusalReason          string                           `json:"refusal_reason,omitempty"`
	Thresholds             retrievalDebugEvidenceThresholds `json:"thresholds"`
	EvidenceGateError      string                           `json:"evidence_gate_error,omitempty"`
	RefusalTemplateVersion string                           `json:"refusal_template_version,omitempty"`
}

type retrievalDebugEvidenceThresholds struct {
	MinRerankScore      float64 `json:"min_rerank_score"`
	MinDensity          float64 `json:"min_density"`
	MinCitationCoverage float64 `json:"min_citation_coverage"`
}

type retrievalDebugCitationCheckResponse struct {
	CitationSupported      bool     `json:"citation_supported"`
	CitationSupportScore   float64  `json:"citation_support_score,omitempty"`
	UnsupportedClaims      []string `json:"unsupported_claims,omitempty"`
	CitationCheckVersion   string   `json:"citation_check_version,omitempty"`
	CitationCheckLatencyMs int64    `json:"citation_check_latency_ms,omitempty"`
}

type retrievalDebugDegradationResponse struct {
	Enabled          bool   `json:"enabled"`
	Reason           string `json:"reason,omitempty"`
	FallbackStrategy string `json:"fallback_strategy,omitempty"`
	ErrorCode        string `json:"error_code,omitempty"`
}

func buildRetrievalDebugTraceResponse(
	logEntry *model.KBRetrieveLog,
	searchMetrics retrieval.SearchMetrics,
	items []retrieveItem,
	debugTrace *retrieval.DebugTrace,
) retrievalDebugTraceResponse {
	response := retrievalDebugTraceResponse{
		RequestID:      fallbackRequestID(logEntry),
		DebugAvailable: debugTrace != nil,
		KBIDs:          parseKBIDs(logEntry),
		OriginalQuery:  firstNonEmptyString(searchMetrics.OriginalQuery, logQuery(logEntry)),
		RewrittenQuery: firstNonEmptyString(searchMetrics.RewriteQuery, logRewrite(logEntry)),
		RewriteStrategy: firstNonEmptyString(
			searchMetrics.RewriteStrategy,
			func() string {
				if logEntry != nil {
					return strings.TrimSpace(logEntry.RewriteStrategy)
				}
				return ""
			}(),
		),
		RewriteGainBucket: firstNonEmptyString(
			func() string {
				if logEntry != nil {
					return strings.TrimSpace(logEntry.RewriteGainBucket)
				}
				return ""
			}(),
		),
		TermHits: append([]string(nil), searchMetrics.TermHits...),
		RouteFinalQueries: map[string]string{
			"dense":  firstNonEmptyString(searchMetrics.DenseQuery, searchMetrics.FinalQuery, logFinalQuery(logEntry)),
			"sparse": firstNonEmptyString(searchMetrics.SparseQuery, searchMetrics.FinalQuery, logFinalQuery(logEntry)),
		},
		TopKDecision: retrievalDebugTopKDecisionResponse{
			TopKPolicyVersion:    searchMetrics.TopKPolicyVersion,
			CandidateTopK:        searchMetrics.CandidateTopK,
			FinalTopK:            searchMetrics.FinalTopK,
			ScoreDistribution:    searchMetrics.ScoreDistribution,
			RerankGap:            searchMetrics.RerankGap,
			EvidenceDensity:      searchMetrics.EvidenceDensity,
			TokenBudget:          searchMetrics.TokenBudget,
			TokenBudgetRemaining: searchMetrics.TokenBudgetRemain,
			TopKDecisionReason:   firstNonEmptyString(searchMetrics.TopKDecisionReason, logTopKReason(logEntry)),
		},
		SemanticCache: retrievalDebugSemanticCacheResponse{
			Enabled: boolValue(
				searchMetrics.SemanticCacheEnabled,
				func() bool {
					if logEntry != nil {
						return logEntry.SemanticCacheEnabled
					}
					return false
				}(),
			),
			Hit: boolValue(
				searchMetrics.SemanticCacheHit,
				func() bool {
					if logEntry != nil {
						return logEntry.SemanticCacheHit
					}
					return false
				}(),
			),
			Reason: firstNonEmptyString(
				searchMetrics.SemanticCacheReason,
				func() string {
					if logEntry != nil {
						return strings.TrimSpace(logEntry.SemanticCacheReason)
					}
					return ""
				}(),
			),
			EntryID: firstNonEmptyString(
				searchMetrics.SemanticCacheEntryID,
				func() string {
					if logEntry != nil {
						return strings.TrimSpace(logEntry.SemanticCacheEntryID)
					}
					return ""
				}(),
			),
			Similarity:        firstNonEmptyFloat(searchMetrics.SemanticCacheSimilarity, logSemanticCacheSimilarity(logEntry)),
			Threshold:         firstNonEmptyFloat(searchMetrics.SemanticCacheThreshold, config.Global.RAG.SemanticCache.SimilarityThreshold),
			CandidateCount:    searchMetrics.SemanticCacheCandidateCount,
			HighestSimilarity: maxFloat64(searchMetrics.SemanticCacheHighestSimilarity, searchMetrics.SemanticCacheSimilarity),
			LookupMs:          maxInt64(searchMetrics.SemanticCacheLookupMs, logSemanticCacheLookupMs(logEntry)),
		},
		EvidenceGate: retrievalDebugEvidenceGateResponse{
			EvidenceGateResult: searchMetrics.EvidenceGateResult,
			RefusalReason:      firstNonEmptyString(searchMetrics.RefusalReason, logRefusalReason(logEntry)),
			Thresholds: retrievalDebugEvidenceThresholds{
				MinRerankScore:      config.Global.RAG.Phase3.EvidenceMinRerankScore,
				MinDensity:          config.Global.RAG.Phase3.EvidenceMinDensity,
				MinCitationCoverage: config.Global.RAG.Phase3.EvidenceMinCitationCoverage,
			},
			EvidenceGateError:      searchMetrics.EvidenceGateError,
			RefusalTemplateVersion: standardRefusalTemplateVersion,
		},
		CitationCheck: retrievalDebugCitationCheckResponse{
			CitationSupported:      searchMetrics.CitationSupported,
			CitationSupportScore:   searchMetrics.CitationSupportScore,
			UnsupportedClaims:      append([]string(nil), searchMetrics.UnsupportedClaims...),
			CitationCheckVersion:   searchMetrics.CitationCheckVersion,
			CitationCheckLatencyMs: searchMetrics.CitationCheckLatencyMs,
		},
		ParentChild: retrievalDebugParentChildResponse{
			ParentChildEnabled:   searchMetrics.ParentChildEnabled,
			ParentFillStrategy:   firstNonEmptyString(searchMetrics.ParentFillStrategy, logParentFillStrategy(logEntry)),
			ParentFillCount:      maxInt(searchMetrics.ParentFillCount, logParentFillCount(logEntry)),
			ParentFillTokens:     maxInt(searchMetrics.ParentFillTokens, logParentFillTokens(logEntry)),
			ParentChildAvailable: searchMetrics.ParentChildEnabled,
		},
		FinalResults:   items,
		StageDurations: buildDebugStageDurations(searchMetrics, debugTrace),
		CreatedAt:      logCreatedAt(logEntry),
	}

	if debugTrace != nil {
		response.RouteHits = make([]retrievalDebugRouteHitResponse, 0, len(debugTrace.RouteHits))
		for _, route := range debugTrace.RouteHits {
			response.RouteHits = append(response.RouteHits, retrievalDebugRouteHitResponse{
				Route:        route.Route,
				Query:        route.Query,
				Hits:         append([]retrieval.DebugDocument(nil), route.Hits...),
				Contribution: route.Contribution,
				LatencyMs:    route.LatencyMs,
				Error:        route.Error,
			})
		}
		response.FusionResults = retrievalDebugFusionResponse{
			Before: append([]retrieval.DebugDocument(nil), debugTrace.Fusion.Before...),
			After:  append([]retrieval.DebugDocument(nil), debugTrace.Fusion.After...),
		}
		response.DedupeResults = retrievalDebugDedupeResponse{
			BeforeCount: debugTrace.Dedupe.BeforeCount,
			AfterCount:  debugTrace.Dedupe.AfterCount,
			Removed:     append([]retrieval.DebugDocument(nil), debugTrace.Dedupe.Removed...),
		}
		response.RerankResults = retrievalDebugRerankResponse{
			Before:        append([]retrieval.DebugDocument(nil), debugTrace.Rerank.Before...),
			After:         append([]retrieval.DebugDocument(nil), debugTrace.Rerank.After...),
			RerankModel:   firstNonEmptyString(debugTrace.Rerank.RerankModel, searchMetrics.RerankModel),
			RerankVersion: firstNonEmptyString(debugTrace.Rerank.RerankVersion, searchMetrics.RerankVersion),
			Fallback:      debugTrace.Rerank.Fallback || searchMetrics.RerankFallback,
			Reason:        firstNonEmptyString(debugTrace.Rerank.Reason, searchMetrics.RerankReason),
		}
		response.FilterResults = retrievalDebugFilterResponse{
			BeforeCount:    debugTrace.Filter.BeforeCount,
			AfterCount:     debugTrace.Filter.AfterCount,
			Removed:        append([]retrieval.DebugDocument(nil), debugTrace.Filter.Removed...),
			TruncateReason: firstNonEmptyString(debugTrace.Filter.TruncateReason, searchMetrics.TruncateReason),
		}
		response.ParentChild.ChildHits = append([]retrieval.DebugDocument(nil), debugTrace.ParentChild.ChildHits...)
		response.ParentChild.ParentContexts = append([]retrieval.DebugDocument(nil), debugTrace.ParentChild.ParentContexts...)
		response.ParentChild.FallbackReason = debugTrace.ParentChild.FallbackReason
		response.Degradation = retrievalDebugDegradationResponse{
			Enabled:          debugTrace.Degradation.Enabled,
			Reason:           debugTrace.Degradation.Reason,
			FallbackStrategy: debugTrace.Degradation.FallbackStrategy,
			ErrorCode:        debugTrace.Degradation.ErrorCode,
		}
	} else {
		response.RouteHits = buildFallbackRouteHits(searchMetrics)
		response.ContractGaps = []string{
			"route_hits",
			"fusion_results",
			"dedupe_results",
			"rerank_results",
			"filter_results",
			"parent_child.child_hits",
			"parent_child.parent_contexts",
		}
	}

	if len(response.CitationCheck.UnsupportedClaims) == 0 && logEntry != nil && logEntry.UnsupportedClaimCount > 0 {
		response.ContractGaps = append(response.ContractGaps, "citation_check.unsupported_claims")
	}
	if !response.DebugAvailable {
		response.Degradation = retrievalDebugDegradationResponse{
			Enabled:          true,
			Reason:           "legacy_debug_trace_missing",
			FallbackStrategy: "basic_trace_fallback",
			ErrorCode:        "debug_trace_unavailable",
		}
	}
	return response
}

func encodeRetrievalDebugTraceResponse(trace retrievalDebugTraceResponse) string {
	data, err := json.Marshal(trace)
	if err != nil {
		return ""
	}
	return string(data)
}

func decodeRetrievalDebugTraceResponse(raw string) (*retrievalDebugTraceResponse, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var trace retrievalDebugTraceResponse
	if err := json.Unmarshal([]byte(raw), &trace); err != nil {
		return nil, err
	}
	return &trace, nil
}

func buildFallbackRouteHits(metrics retrieval.SearchMetrics) []retrievalDebugRouteHitResponse {
	return []retrievalDebugRouteHitResponse{
		{
			Route:        "dense",
			Query:        firstNonEmptyString(metrics.DenseQuery, metrics.FinalQuery, metrics.OriginalQuery),
			Contribution: metrics.DenseContribution,
			LatencyMs:    metrics.SearchMs,
		},
		{
			Route:        "sparse",
			Query:        firstNonEmptyString(metrics.SparseQuery, metrics.FinalQuery, metrics.OriginalQuery),
			Contribution: metrics.SparseContribution,
		},
	}
}

func buildDebugStageDurations(metrics retrieval.SearchMetrics, debugTrace *retrieval.DebugTrace) map[string]int64 {
	result := map[string]int64{
		"embedding_ms":   metrics.EmbeddingMs,
		"search_ms":      metrics.SearchMs,
		"postprocess_ms": metrics.PostprocessMs,
		"rerank_ms":      metrics.RerankMs,
	}
	if debugTrace == nil || len(debugTrace.StageDurations) == 0 {
		return result
	}
	for key, value := range debugTrace.StageDurations {
		result[key] = value
	}
	return result
}

func parseKBIDs(logEntry *model.KBRetrieveLog) []uint64 {
	if logEntry == nil {
		return nil
	}
	raw := strings.TrimSpace(logEntry.KBIDs)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]uint64, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			continue
		}
		result = append(result, parsed)
	}
	return result
}

func fallbackRequestID(logEntry *model.KBRetrieveLog) string {
	if logEntry == nil {
		return ""
	}
	return logEntry.RequestID
}

func logQuery(logEntry *model.KBRetrieveLog) string {
	if logEntry == nil {
		return ""
	}
	return logEntry.Query
}

func logRewrite(logEntry *model.KBRetrieveLog) string {
	if logEntry == nil {
		return ""
	}
	return logEntry.Rewrite
}

func logFinalQuery(logEntry *model.KBRetrieveLog) string {
	if logEntry == nil {
		return ""
	}
	return logEntry.FinalQuery
}

func logTopKReason(logEntry *model.KBRetrieveLog) string {
	if logEntry == nil {
		return ""
	}
	return logEntry.TopKDecisionReason
}

func logRefusalReason(logEntry *model.KBRetrieveLog) string {
	if logEntry == nil {
		return ""
	}
	return logEntry.RefusalReason
}

func logSemanticCacheSimilarity(logEntry *model.KBRetrieveLog) float64 {
	if logEntry == nil {
		return 0
	}
	return logEntry.SemanticCacheSimilarity
}

func logSemanticCacheLookupMs(logEntry *model.KBRetrieveLog) int64 {
	if logEntry == nil {
		return 0
	}
	return logEntry.SemanticCacheLookupMs
}

func boolValue(primary bool, fallback bool) bool {
	if primary {
		return true
	}
	return fallback
}

func logParentFillStrategy(logEntry *model.KBRetrieveLog) string {
	if logEntry == nil {
		return ""
	}
	return logEntry.ParentFillStrategy
}

func logParentFillCount(logEntry *model.KBRetrieveLog) int {
	if logEntry == nil {
		return 0
	}
	return logEntry.ParentFillCount
}

func logParentFillTokens(logEntry *model.KBRetrieveLog) int {
	if logEntry == nil {
		return 0
	}
	return logEntry.ParentFillTokens
}

func logCreatedAt(logEntry *model.KBRetrieveLog) time.Time {
	if logEntry == nil {
		return time.Time{}
	}
	return logEntry.CreatedAt
}
