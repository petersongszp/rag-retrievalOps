package model

import (
	"reflect"
	"strings"
	"testing"
)

func TestKBRetrieveLogP1JSONContract(t *testing.T) {
	t.Parallel()

	expected := map[string]string{
		"RequestID":               "request_id",
		"ExperimentID":            "experiment_id",
		"ExperimentGroup":         "experiment_group",
		"StrategyVersion":         "strategy_version",
		"FusionStrategy":          "fusion_strategy",
		"RRFK":                    "rrf_k",
		"IndexVersion":            "index_version",
		"CollectionVersion":       "collection_version",
		"CostTraceID":             "cost_trace_id",
		"AuditTraceID":            "audit_trace_id",
		"ReleaseID":               "release_id",
		"UserID":                  "user_id",
		"KBIDs":                   "kb_ids",
		"Query":                   "query",
		"FinalQuery":              "final_query",
		"Expr":                    "expr",
		"TopK":                    "top_k",
		"CandidateTopK":           "candidate_topk",
		"FinalTopK":               "final_topk",
		"TokenBudget":             "token_budget",
		"ContextTokens":           "context_tokens",
		"QueryType":               "query_type",
		"TruncateReason":          "truncate_reason",
		"Rewrite":                 "rewrite",
		"RewriteStrategy":         "rewrite_strategy",
		"RewriteApplied":          "rewrite_applied",
		"Strategy":                "strategy",
		"ReleaseStage":            "release_stage",
		"ReleaseReason":           "release_reason",
		"Routes":                  "routes",
		"Collection":              "collection",
		"RetrieverVersion":        "retriever_version",
		"EmptyReason":             "empty_reason",
		"ParentChildEnabled":      "parent_child_enabled",
		"ParentFillStrategy":      "parent_fill_strategy",
		"ParentFillCount":         "parent_fill_count",
		"ParentFillFallback":      "parent_fill_fallback",
		"ParentFillTokens":        "parent_fill_tokens",
		"TopKDecisionReason":      "topk_decision_reason",
		"EvidenceGateResult":      "evidence_gate_result",
		"RefusalReason":           "refusal_reason",
		"CitationSupported":       "citation_supported",
		"CitationSupportScore":    "citation_support_score",
		"RewriteGainBucket":       "rewrite_gain_bucket",
		"UnsupportedClaimCount":   "unsupported_claim_count",
		"CitationCheckVersion":    "citation_check_version",
		"CitationCheckLatencyMs":  "citation_check_latency_ms",
		"EvidenceGateError":       "evidence_gate_error",
		"CitationCheckError":      "citation_check_error",
		"SemanticCacheEnabled":    "semantic_cache_enabled",
		"SemanticCacheHit":        "semantic_cache_hit",
		"SemanticCacheLookupMs":   "semantic_cache_lookup_ms",
		"SemanticCacheSimilarity": "semantic_cache_similarity",
		"SemanticCacheEntryID":    "semantic_cache_entry_id",
		"SemanticCacheReason":     "semantic_cache_reason",
		"EmbeddingCacheEnabled":   "embedding_cache_enabled",
		"EmbeddingCacheHit":       "embedding_cache_hit",
		"EmbeddingCacheLookupMs":  "embedding_cache_lookup_ms",
		"EmbeddingCacheReason":    "embedding_cache_reason",
		"FinalCount":              "final_count",
		"TruncatedCount":          "truncated_count",
		"DenseHits":               "dense_hits",
		"SparseHits":              "sparse_hits",
		"DenseParticipation":      "dense_participation",
		"SparseParticipation":     "sparse_participation",
		"PrimaryDenseCount":       "primary_dense_count",
		"PrimarySparseCount":      "primary_sparse_count",
		"DualRouteFinalCount":     "dual_route_final_count",
		"DenseContribution":       "dense_contribution",
		"SparseContribution":      "sparse_contribution",
		"SparseCandidateBefore":   "sparse_candidate_before_bm25",
		"SparseCandidateAfter":    "sparse_candidate_after_bm25",
		"ResultStatus":            "result_status",
		"ErrorCode":               "error_code",
		"ErrorMsg":                "error_msg",
		"EmbeddingMs":             "embedding_ms",
		"SearchMs":                "search_ms",
		"PostprocessMs":           "postprocess_ms",
		"RerankMs":                "rerank_ms",
		"RerankModel":             "rerank_model",
		"DurationMs":              "duration_ms",
		"TimeoutMs":               "timeout_ms",
		"CreatedAt":               "created_at",
	}

	assertJSONTags(t, reflect.TypeOf(KBRetrieveLog{}), expected)
}

func TestKBJobOperationLogP1JSONContract(t *testing.T) {
	t.Parallel()

	expected := map[string]string{
		"ID":              "id",
		"JobID":           "job_id",
		"OperatorID":      "operator_id",
		"Operation":       "operation",
		"OperationReason": "operation_reason",
		"FromStatus":      "from_status",
		"ToStatus":        "to_status",
		"CreatedAt":       "created_at",
	}

	assertJSONTags(t, reflect.TypeOf(KBJobOperationLog{}), expected)
}

func TestKBIngestJobP1JSONContract(t *testing.T) {
	t.Parallel()

	expected := map[string]string{
		"ID":              "id",
		"KbID":            "kb_id",
		"DocumentID":      "document_id",
		"UserID":          "user_id",
		"Status":          "status",
		"RetryCount":      "retry_count",
		"ErrorMsg":        "error_msg",
		"LastErrorCode":   "last_error_code",
		"LastErrorDetail": "last_error_detail",
		"OperatorID":      "operator_id",
		"Operation":       "operation",
		"OperationReason": "operation_reason",
		"OperatedAt":      "operated_at",
		"StartedAt":       "started_at",
		"FinishedAt":      "finished_at",
		"CreatedAt":       "created_at",
		"UpdatedAt":       "updated_at",
	}

	assertJSONTags(t, reflect.TypeOf(KBIngestJob{}), expected)
}

func assertJSONTags(t *testing.T, typ reflect.Type, expected map[string]string) {
	t.Helper()

	for fieldName, expectedTag := range expected {
		field, ok := typ.FieldByName(fieldName)
		if !ok {
			t.Fatalf("field %s not found on %s", fieldName, typ.Name())
		}

		jsonTag := strings.Split(field.Tag.Get("json"), ",")[0]
		if jsonTag != expectedTag {
			t.Fatalf("%s.%s json tag = %q, want %q", typ.Name(), fieldName, jsonTag, expectedTag)
		}
	}
}
