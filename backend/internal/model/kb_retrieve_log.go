package model

import (
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

type RetrieveResultStatus string

const (
	RetrieveResultStatusSuccess     RetrieveResultStatus = "success"
	RetrieveResultStatusNoResult    RetrieveResultStatus = "no_result"
	RetrieveResultStatusFilteredOut RetrieveResultStatus = "filtered_out"
	RetrieveResultStatusError       RetrieveResultStatus = "error"
	RetrieveResultStatusTimeout     RetrieveResultStatus = "timeout"
)

var KBRetrieveLogDao _KBRetrieveLog

type KBRetrieveLogListFilter struct {
	UserID       *uint
	KBID         *uint64
	ResultStatus *RetrieveResultStatus
	StartTime    *time.Time
	EndTime      *time.Time
	QueryKeyword *string
	RequestID    *string
	Page         int
	PageSize     int
}

type (
	_KBRetrieveLog struct{}
	KBRetrieveLog  struct {
		ID                      uint64               `json:"id" gorm:"primaryKey;autoIncrement"`
		RequestID               string               `json:"request_id" gorm:"uniqueIndex;size:64;not null"`
		ExperimentID            string               `json:"experiment_id" gorm:"size:128;index"`
		ExperimentGroup         string               `json:"experiment_group" gorm:"size:32;index"`
		StrategyVersion         string               `json:"strategy_version" gorm:"size:128;index"`
		FusionStrategy          string               `json:"fusion_strategy" gorm:"size:32;index"`
		RRFK                    int                  `json:"rrf_k"`
		IndexVersion            string               `json:"index_version" gorm:"size:128"`
		CollectionVersion       string               `json:"collection_version" gorm:"size:128"`
		CostTraceID             string               `json:"cost_trace_id" gorm:"size:128;index"`
		AuditTraceID            string               `json:"audit_trace_id" gorm:"size:128;index"`
		ReleaseID               string               `json:"release_id" gorm:"size:128;index"`
		UserID                  uint                 `json:"user_id" gorm:"index;not null"`
		KBIDs                   string               `json:"kb_ids" gorm:"size:500"`
		Query                   string               `json:"query" gorm:"size:2000;not null"`
		FinalQuery              string               `json:"final_query" gorm:"size:2000"`
		Expr                    string               `json:"expr" gorm:"size:2000"`
		TopK                    int                  `json:"top_k"`
		CandidateTopK           int                  `json:"candidate_topk"`
		FinalTopK               int                  `json:"final_topk"`
		TokenBudget             int                  `json:"token_budget"`
		ContextTokens           int                  `json:"context_tokens"`
		QueryType               string               `json:"query_type" gorm:"size:64;index"`
		TruncateReason          string               `json:"truncate_reason" gorm:"size:64"`
		Rewrite                 string               `json:"rewrite" gorm:"size:1000"`
		RewriteStrategy         string               `json:"rewrite_strategy" gorm:"size:255"`
		RewriteApplied          bool                 `json:"rewrite_applied"`
		Strategy                string               `json:"strategy" gorm:"size:20;index"`
		ReleaseStage            string               `json:"release_stage" gorm:"size:32;index"`
		ReleaseReason           string               `json:"release_reason" gorm:"size:255"`
		Routes                  string               `json:"routes" gorm:"size:200"`
		Collection              string               `json:"collection" gorm:"size:200"`
		RetrieverVersion        string               `json:"retriever_version" gorm:"size:50"`
		EmptyReason             string               `json:"empty_reason" gorm:"size:64;index"`
		ParentChildEnabled      bool                 `json:"parent_child_enabled"`
		ParentFillStrategy      string               `json:"parent_fill_strategy" gorm:"size:64"`
		ParentFillCount         int                  `json:"parent_fill_count"`
		ParentFillFallback      int                  `json:"parent_fill_fallback"`
		ParentFillTokens        int                  `json:"parent_fill_tokens"`
		TopKDecisionReason      string               `json:"topk_decision_reason" gorm:"size:255"`
		EvidenceGateResult      string               `json:"evidence_gate_result" gorm:"size:32;index"`
		RefusalReason           string               `json:"refusal_reason" gorm:"size:64;index"`
		CitationSupported       bool                 `json:"citation_supported"`
		CitationSupportScore    float64              `json:"citation_support_score"`
		RewriteGainBucket       string               `json:"rewrite_gain_bucket" gorm:"size:64;index"`
		UnsupportedClaimCount   int                  `json:"unsupported_claim_count"`
		CitationCheckVersion    string               `json:"citation_check_version" gorm:"size:64"`
		CitationCheckLatencyMs  int64                `json:"citation_check_latency_ms"`
		EvidenceGateError       string               `json:"evidence_gate_error" gorm:"size:500"`
		CitationCheckError      string               `json:"citation_check_error" gorm:"size:500"`
		SemanticCacheEnabled    bool                 `json:"semantic_cache_enabled"`
		SemanticCacheHit        bool                 `json:"semantic_cache_hit"`
		SemanticCacheLookupMs   int64                `json:"semantic_cache_lookup_ms"`
		SemanticCacheSimilarity float64              `json:"semantic_cache_similarity"`
		SemanticCacheEntryID    string               `json:"semantic_cache_entry_id" gorm:"size:128"`
		SemanticCacheReason     string               `json:"semantic_cache_reason" gorm:"size:64;index"`
		EmbeddingCacheEnabled   bool                 `json:"embedding_cache_enabled"`
		EmbeddingCacheHit       bool                 `json:"embedding_cache_hit"`
		EmbeddingCacheLookupMs  int64                `json:"embedding_cache_lookup_ms"`
		EmbeddingCacheReason    string               `json:"embedding_cache_reason" gorm:"size:64;index"`
		FinalCount              int                  `json:"final_count"`
		TruncatedCount          int                  `json:"truncated_count"`
		DenseHits               int                  `json:"dense_hits"`
		SparseHits              int                  `json:"sparse_hits"`
		DenseParticipation      int                  `json:"dense_participation"`
		SparseParticipation     int                  `json:"sparse_participation"`
		PrimaryDenseCount       int                  `json:"primary_dense_count"`
		PrimarySparseCount      int                  `json:"primary_sparse_count"`
		DualRouteFinalCount     int                  `json:"dual_route_final_count"`
		DenseContribution       int                  `json:"dense_contribution"`
		SparseContribution      int                  `json:"sparse_contribution"`
		SparseCandidateBefore   int                  `json:"sparse_candidate_before_bm25"`
		SparseCandidateAfter    int                  `json:"sparse_candidate_after_bm25"`
		ResultStatus            RetrieveResultStatus `json:"result_status" gorm:"size:20;not null;default:'success';index"`
		ErrorCode               string               `json:"error_code" gorm:"size:64"`
		ErrorMsg                string               `json:"error_msg" gorm:"size:1000"`
		EmbeddingMs             int64                `json:"embedding_ms"`
		SearchMs                int64                `json:"search_ms"`
		PostprocessMs           int64                `json:"postprocess_ms"`
		RerankMs                int64                `json:"rerank_ms"`
		RerankModel             string               `json:"rerank_model" gorm:"size:128"`
		DurationMs              int64                `json:"duration_ms"`
		TimeoutMs               int64                `json:"timeout_ms"`
		DebugTrace              string               `json:"-" gorm:"type:longtext"`
		TenantID                uint64               `json:"tenant_id" gorm:"index"`
		AppID                   string               `json:"app_id" gorm:"index;size:64"`
		APIKeyID                uint64               `json:"api_key_id" gorm:"index"`
		AuthType                string               `json:"auth_type" gorm:"index;size:32"`
		SourceAPI               string               `json:"source_api" gorm:"size:32"`
		PermissionResult        string               `json:"permission_result" gorm:"index;size:32"`
		IsLegacy                bool                 `json:"is_legacy"`
		CreatedAt               time.Time            `json:"created_at" gorm:"autoCreateTime:milli;index"`
	}
)

func (KBRetrieveLog) TableName() string {
	return "kb_retrieve_log"
}

func (d *_KBRetrieveLog) Create(log *KBRetrieveLog) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().Create(log).Error
}

func (d *_KBRetrieveLog) GetByRequestID(requestID string) (*KBRetrieveLog, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var logEntry KBRetrieveLog
	err := getDB().Where("request_id = ?", requestID).First(&logEntry).Error
	if err != nil {
		return nil, err
	}
	return &logEntry, nil
}

func (d *_KBRetrieveLog) ListByUserID(userID uint, page, pageSize int) ([]*KBRetrieveLog, int64, error) {
	return d.ListWithFilter(KBRetrieveLogListFilter{
		UserID:   &userID,
		Page:     page,
		PageSize: pageSize,
	})
}

func (d *_KBRetrieveLog) ListByStatus(resultStatus RetrieveResultStatus, page, pageSize int) ([]*KBRetrieveLog, int64, error) {
	return d.ListWithFilter(KBRetrieveLogListFilter{
		ResultStatus: &resultStatus,
		Page:         page,
		PageSize:     pageSize,
	})
}

func (d *_KBRetrieveLog) ListWithFilter(filter KBRetrieveLogListFilter) ([]*KBRetrieveLog, int64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}

	var list []*KBRetrieveLog
	var total int64

	query := applyKBRetrieveLogFilters(getDB().Model(&KBRetrieveLog{}), filter)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Offset((filter.Page - 1) * filter.PageSize).
		Limit(filter.PageSize).
		Order("created_at DESC").
		Find(&list).Error
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (d *_KBRetrieveLog) ListByCreatedAt(startTime, endTime time.Time, kbID *uint64) ([]*KBRetrieveLog, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}

	filter := KBRetrieveLogListFilter{
		KBID:      kbID,
		StartTime: &startTime,
		EndTime:   &endTime,
	}

	var list []*KBRetrieveLog
	err := applyKBRetrieveLogFilters(getDB().Model(&KBRetrieveLog{}), filter).
		Order("created_at ASC").
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

func applyKBRetrieveLogFilters(query *gorm.DB, filter KBRetrieveLogListFilter) *gorm.DB {
	if filter.UserID != nil {
		query = query.Where("user_id = ?", *filter.UserID)
	}
	if filter.KBID != nil {
		query = applyCommaSeparatedUint64Filter(query, "kb_ids", *filter.KBID)
	}
	if filter.ResultStatus != nil {
		query = query.Where("result_status = ?", *filter.ResultStatus)
	}
	if filter.StartTime != nil {
		query = query.Where("created_at >= ?", *filter.StartTime)
	}
	if filter.EndTime != nil {
		query = query.Where("created_at <= ?", *filter.EndTime)
	}
	if filter.QueryKeyword != nil {
		keyword := strings.TrimSpace(*filter.QueryKeyword)
		if keyword != "" {
			query = query.Where("query LIKE ?", "%"+keyword+"%")
		}
	}
	if filter.RequestID != nil {
		requestID := strings.TrimSpace(*filter.RequestID)
		if requestID != "" {
			query = query.Where("request_id = ?", requestID)
		}
	}
	return query
}

func applyCommaSeparatedUint64Filter(query *gorm.DB, column string, value uint64) *gorm.DB {
	target := strconv.FormatUint(value, 10)
	return query.Where(
		column+" = ? OR "+column+" LIKE ? OR "+column+" LIKE ? OR "+column+" LIKE ?",
		target,
		target+",%",
		"%,"+target+",%",
		"%,"+target,
	)
}

func ParseRetrieveResultStatus(raw string) (RetrieveResultStatus, bool) {
	status := RetrieveResultStatus(raw)
	switch status {
	case RetrieveResultStatusSuccess,
		RetrieveResultStatusNoResult,
		RetrieveResultStatusFilteredOut,
		RetrieveResultStatusError,
		RetrieveResultStatusTimeout:
		return status, true
	default:
		return "", false
	}
}
