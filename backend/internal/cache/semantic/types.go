package semantic

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	KeyPrefix        = "rag:semantic_cache"
	ResultPayloadTag = "retrieve_result_only"
)

// Scope 决定“哪些请求可以共享同一片缓存池”。
// 这里不是 query 本身，而是缓存的隔离边界。
type Scope struct {
	TenantID        uint64
	KBIDs           []uint64
	StrategyVersion string
	QueryType       string
}

// Entry 是 Redis 中真正存储的一条语义缓存记录。
// 你可以把它理解成“一次历史检索结果的快照”。
type Entry struct {
	EntryID          string          `json:"entry_id"`
	TenantID         uint64          `json:"tenant_id"`
	KBScope          string          `json:"kb_scope"`
	KBIDs            []uint64        `json:"kb_ids"`
	StrategyVersion  string          `json:"strategy_version"`
	RetrieverVersion string          `json:"retriever_version"`
	QueryType        string          `json:"query_type"`
	Query            string          `json:"query"`
	QueryEmbedding   []float32       `json:"query_embedding"`
	ResponsePayload  json.RawMessage `json:"response_payload"`
	ResultPayload    string          `json:"result_payload"`
	TopK             int             `json:"top_k"`
	CreatedAt        time.Time       `json:"created_at"`
	ExpiresAt        time.Time       `json:"expires_at"`
	HitCount         int             `json:"hit_count"`
	LastHitAt        time.Time       `json:"last_hit_at"`
}

type LookupResult struct {
	Scope          Scope
	Candidates     []*Entry
	CandidateCount int
}

func (s Scope) Normalize() Scope {
	// 先排序再去重，是为了让 [1,2] 和 [2,1] 这种等价 scope 生成同一组 key。
	normalized := Scope{
		TenantID:        s.TenantID,
		KBIDs:           append([]uint64(nil), s.KBIDs...),
		StrategyVersion: strings.TrimSpace(s.StrategyVersion),
		QueryType:       strings.TrimSpace(s.QueryType),
	}
	sort.Slice(normalized.KBIDs, func(i, j int) bool {
		return normalized.KBIDs[i] < normalized.KBIDs[j]
	})
	deduped := normalized.KBIDs[:0]
	var last uint64
	for i, kbID := range normalized.KBIDs {
		if i == 0 || kbID != last {
			deduped = append(deduped, kbID)
			last = kbID
		}
	}
	normalized.KBIDs = deduped
	return normalized
}

func (s Scope) Validate() error {
	normalized := s.Normalize()
	if normalized.TenantID == 0 {
		return fmt.Errorf("tenant_id must be > 0")
	}
	if len(normalized.KBIDs) == 0 {
		return fmt.Errorf("kb_ids must not be empty")
	}
	if normalized.StrategyVersion == "" {
		return fmt.Errorf("strategy_version must not be empty")
	}
	if normalized.QueryType == "" {
		return fmt.Errorf("query_type must not be empty")
	}
	return nil
}

func (e *Entry) Validate() error {
	if e == nil {
		return fmt.Errorf("entry is nil")
	}
	scope := e.Scope()
	if err := scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(e.Query) == "" {
		return fmt.Errorf("query must not be empty")
	}
	if len(e.QueryEmbedding) == 0 {
		return fmt.Errorf("query_embedding must not be empty")
	}
	if len(e.ResponsePayload) == 0 {
		return fmt.Errorf("response_payload must not be empty")
	}
	if e.TopK <= 0 {
		return fmt.Errorf("top_k must be > 0")
	}
	if e.ResultPayload == "" {
		e.ResultPayload = ResultPayloadTag
	}
	// 当前阶段只允许缓存 retrieve 结果，不缓存最终 LLM answer。
	if e.ResultPayload != ResultPayloadTag {
		return fmt.Errorf("unsupported result_payload: %s", e.ResultPayload)
	}
	return nil
}

func (e *Entry) Scope() Scope {
	return Scope{
		TenantID:        e.TenantID,
		KBIDs:           append([]uint64(nil), e.KBIDs...),
		StrategyVersion: e.StrategyVersion,
		QueryType:       e.QueryType,
	}
}
