package model

import "time"

var KBCostTraceDao _KBCostTrace

type (
	_KBCostTrace struct{}
	KBCostTrace  struct {
		ID                     uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
		RequestID              string    `json:"request_id" gorm:"size:64;index;not null"`
		CostTraceID            string    `json:"cost_trace_id" gorm:"size:128;index;not null"`
		KBID                   uint64    `json:"kb_id" gorm:"index"`
		UserID                 uint      `json:"user_id" gorm:"index"`
		ExperimentID           string    `json:"experiment_id" gorm:"size:128;index"`
		StrategyVersion        string    `json:"strategy_version" gorm:"size:128;index"`
		QueryType              string    `json:"query_type" gorm:"size:64;index"`
		EmbeddingTokens        int       `json:"embedding_tokens"`
		ContextTokens          int       `json:"context_tokens"`
		CompletionTokens       int       `json:"completion_tokens"`
		RetrievalCandidateCount int      `json:"retrieval_candidate_count"`
		RerankCandidateCount   int       `json:"rerank_candidate_count"`
		LLMModel               string    `json:"llm_model" gorm:"size:128"`
		EmbeddingCost          float64   `json:"embedding_cost"`
		RetrievalCost          float64   `json:"retrieval_cost"`
		RerankCost             float64   `json:"rerank_cost"`
		LLMCost                float64   `json:"llm_cost"`
		VectorStorageCost      float64   `json:"vector_storage_cost"`
		TotalCost              float64   `json:"total_cost"`
		CreatedAt              time.Time `json:"created_at" gorm:"autoCreateTime:milli;index"`
	}
)

func (KBCostTrace) TableName() string {
	return "kb_cost_trace"
}

func (d *_KBCostTrace) Create(record *KBCostTrace) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().Create(record).Error
}

func (d *_KBCostTrace) ListByCreatedAt(startTime, endTime time.Time, kbID *uint64) ([]*KBCostTrace, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var list []*KBCostTrace
	query := getDB().Model(&KBCostTrace{}).Where("created_at >= ? AND created_at <= ?", startTime, endTime)
	if kbID != nil {
		query = query.Where("kb_id = ?", *kbID)
	}
	if err := query.Order("created_at ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}
