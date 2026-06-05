package retrieval

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

type DebugDocument struct {
	DocumentID         uint64                 `json:"document_id,omitempty"`
	ChunkID            string                 `json:"chunk_id,omitempty"`
	ParentID           string                 `json:"parent_id,omitempty"`
	FileName           string                 `json:"file_name,omitempty"`
	Route              string                 `json:"route,omitempty"`
	Score              float64                `json:"score,omitempty"`
	RerankScore        float64                `json:"rerank_score,omitempty"`
	Content            string                 `json:"content,omitempty"`
	Collection         string                 `json:"collection,omitempty"`
	SectionTitle       string                 `json:"section_title,omitempty"`
	HierarchyPath      string                 `json:"hierarchy_path,omitempty"`
	ParentFillApplied  bool                   `json:"parent_fill_applied,omitempty"`
	ParentFillStrategy string                 `json:"parent_fill_strategy,omitempty"`
	ParentFillReason   string                 `json:"parent_fill_reason,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
}

type RouteDebugHit struct {
	Route                  string         `json:"route"`
	Query                  string         `json:"query,omitempty"`
	Hits                   []DebugDocument `json:"hits,omitempty"`
	HitsCount              int            `json:"hits_count,omitempty"`
	ParticipationCount     int            `json:"participation_count,omitempty"`
	PrimaryCount           int            `json:"primary_count,omitempty"`
	Contribution           int            `json:"contribution"`
	SparseTerms            []string       `json:"sparse_terms,omitempty"`
	TermSources            map[string]string `json:"term_sources,omitempty"`
	DroppedTerms           map[string]string `json:"dropped_terms,omitempty"`
	PerTermCandidateCounts map[string]int `json:"per_term_candidate_counts,omitempty"`
	CandidateCountBefore   int            `json:"candidate_count_before_bm25,omitempty"`
	CandidateCountAfter    int            `json:"candidate_count_after_bm25,omitempty"`
	LatencyMs              int64          `json:"latency_ms,omitempty"`
	Error                  string         `json:"error,omitempty"`
}

type FusionDebugInfo struct {
	Before                   []DebugDocument `json:"before,omitempty"`
	After                    []DebugDocument `json:"after,omitempty"`
	DenseParticipationCount  int             `json:"dense_participation_count,omitempty"`
	SparseParticipationCount int             `json:"sparse_participation_count,omitempty"`
	DualRouteFinalCount      int             `json:"dual_route_final_count,omitempty"`
	PrimaryRouteDistribution map[string]int  `json:"primary_route_distribution,omitempty"`
}

type DedupeDebugInfo struct {
	BeforeCount int             `json:"before_count"`
	AfterCount  int             `json:"after_count"`
	Removed     []DebugDocument `json:"removed,omitempty"`
}

type RerankDebugInfo struct {
	Before        []DebugDocument `json:"before,omitempty"`
	After         []DebugDocument `json:"after,omitempty"`
	RerankModel   string          `json:"rerank_model,omitempty"`
	RerankVersion string          `json:"rerank_version,omitempty"`
	Fallback      bool            `json:"fallback,omitempty"`
	Reason        string          `json:"reason,omitempty"`
}

type FilterDebugInfo struct {
	BeforeCount    int               `json:"before_count"`
	AfterCount     int               `json:"after_count"`
	Removed        []DebugDocument   `json:"removed,omitempty"`
	DropReasons    map[string]int    `json:"drop_reasons,omitempty"`
	TruncateReason string            `json:"truncate_reason,omitempty"`
}

type ParentChildDebugInfo struct {
	ChildHits      []DebugDocument `json:"child_hits,omitempty"`
	ParentContexts []DebugDocument `json:"parent_contexts,omitempty"`
	FallbackReason string          `json:"fallback_reason,omitempty"`
}

type DegradationDebugInfo struct {
	Enabled          bool   `json:"enabled"`
	Reason           string `json:"reason,omitempty"`
	FallbackStrategy string `json:"fallback_strategy,omitempty"`
	ErrorCode        string `json:"error_code,omitempty"`
}

type DebugTrace struct {
	RouteHits      []RouteDebugHit      `json:"route_hits,omitempty"`
	Fusion         FusionDebugInfo      `json:"fusion,omitempty"`
	Dedupe         DedupeDebugInfo      `json:"dedupe,omitempty"`
	Rerank         RerankDebugInfo      `json:"rerank,omitempty"`
	Filter         FilterDebugInfo      `json:"filter,omitempty"`
	ParentChild    ParentChildDebugInfo `json:"parent_child,omitempty"`
	FinalResults   []DebugDocument      `json:"final_results,omitempty"`
	StageDurations map[string]int64     `json:"stage_durations,omitempty"`
	Degradation    DegradationDebugInfo `json:"degradation,omitempty"`
}

func SnapshotDocuments(docs []*schema.Document) []DebugDocument {
	if len(docs) == 0 {
		return nil
	}
	items := make([]DebugDocument, 0, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		metadata := doc.MetaData
		items = append(items, DebugDocument{
			DocumentID:         debugUint64Metadata(metadata, "document_id"),
			ChunkID:            firstNonEmptyString(doc.ID, getStringMetadata(metadata, "chunk_id"), getStringMetadata(metadata, "child_id")),
			ParentID:           getStringMetadata(metadata, "parent_id"),
			FileName:           getStringMetadata(metadata, "file_name"),
			Route:              getStringMetadata(metadata, "route"),
			Score:              debugFloat64Metadata(metadata, "score"),
			RerankScore:        debugFloat64Metadata(metadata, "rerank_score"),
			Content:            doc.Content,
			Collection:         firstNonEmptyString(getStringMetadata(metadata, "collection"), getStringMetadata(metadata, "source.collection")),
			SectionTitle:       getStringMetadata(metadata, "section_title"),
			HierarchyPath:      getStringMetadata(metadata, "hierarchy_path"),
			ParentFillApplied:  debugBoolMetadata(metadata, "parent_fill_applied"),
			ParentFillStrategy: getStringMetadata(metadata, "parent_fill_strategy"),
			ParentFillReason:   getStringMetadata(metadata, "parent_fill_reason"),
			Metadata:           cloneDebugMetadata(metadata),
		})
	}
	return items
}

func RemovedSnapshots(before, after []*schema.Document) []DebugDocument {
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

	removed := make([]*schema.Document, 0)
	for _, doc := range before {
		if doc == nil {
			continue
		}
		if _, ok := afterKeys[debugDocumentKey(doc)]; ok {
			continue
		}
		removed = append(removed, doc)
	}
	return SnapshotDocuments(removed)
}

func SnapshotFusedDocuments(docs []*FusedDocument) []DebugDocument {
	if len(docs) == 0 {
		return nil
	}
	items := make([]DebugDocument, 0, len(docs))
	for _, item := range docs {
		if item == nil || item.Doc == nil {
			continue
		}
		snapshot := SnapshotDocuments([]*schema.Document{item.Doc})
		if len(snapshot) == 0 {
			continue
		}
		snapshot[0].Score = item.Score
		snapshot[0].Route = firstNonEmptyString(snapshot[0].Route, item.PrimaryRoute)
		items = append(items, snapshot[0])
	}
	return items
}

func RemovedFusedSnapshots(before []*FusedDocument, after []*schema.Document) []DebugDocument {
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
	removed := make([]*FusedDocument, 0)
	for _, item := range before {
		if item == nil || item.Doc == nil {
			continue
		}
		if _, ok := afterKeys[debugDocumentKey(item.Doc)]; ok {
			continue
		}
		removed = append(removed, item)
	}
	return SnapshotFusedDocuments(removed)
}

func ParentContextSnapshots(docs []*schema.Document) []DebugDocument {
	if len(docs) == 0 {
		return nil
	}
	contexts := make([]*schema.Document, 0, len(docs))
	for _, doc := range docs {
		if doc == nil || doc.MetaData == nil {
			continue
		}
		if debugBoolMetadata(doc.MetaData, "parent_fill_applied") || getStringMetadata(doc.MetaData, "parent_id") != "" {
			contexts = append(contexts, doc)
		}
	}
	return SnapshotDocuments(contexts)
}

func debugDocumentKey(doc *schema.Document) string {
	if doc == nil {
		return ""
	}
	if strings.TrimSpace(doc.ID) != "" {
		return strings.TrimSpace(doc.ID)
	}
	if doc.MetaData == nil {
		return ""
	}
	documentID := debugUint64Metadata(doc.MetaData, "document_id")
	chunkID := firstNonEmptyString(getStringMetadata(doc.MetaData, "chunk_id"), getStringMetadata(doc.MetaData, "child_id"))
	if documentID > 0 || chunkID != "" {
		return fmt.Sprintf("%d:%s", documentID, chunkID)
	}
	return strings.TrimSpace(doc.Content)
}

func cloneDebugMetadata(source map[string]interface{}) map[string]interface{} {
	if len(source) == 0 {
		return nil
	}
	cloned := make(map[string]interface{}, len(source))
	for key, value := range source {
		if key == "source" {
			continue
		}
		cloned[key] = value
	}
	return cloned
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func debugFloat64Metadata(metadata map[string]interface{}, key string) float64 {
	if metadata == nil {
		return 0
	}
	if score, ok := castScore(metadata[key]); ok {
		return score
	}
	return 0
}

func debugUint64Metadata(metadata map[string]interface{}, key string) uint64 {
	if metadata == nil {
		return 0
	}
	switch value := metadata[key].(type) {
	case uint64:
		return value
	case uint32:
		return uint64(value)
	case uint:
		return uint64(value)
	case int:
		if value >= 0 {
			return uint64(value)
		}
	case int64:
		if value >= 0 {
			return uint64(value)
		}
	case float64:
		if value >= 0 {
			return uint64(value)
		}
	}
	return 0
}

func debugBoolMetadata(metadata map[string]interface{}, key string) bool {
	if metadata == nil {
		return false
	}
	return castBool(metadata[key])
}
