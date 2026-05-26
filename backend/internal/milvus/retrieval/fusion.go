package retrieval

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/cloudwego/eino/schema"
)

const (
	// EmptyReasonNone 表示本次请求最终有结果。
	EmptyReasonNone = "None"
	// EmptyReasonAfterRetrieve 表示多路召回结束后候选池为空。
	EmptyReasonAfterRetrieve = "Empty-After-Retrieve"
	// EmptyReasonAfterFusion 表示召回有结果，但融合/去重后被全部淘空。
	EmptyReasonAfterFusion = "Empty-After-Fusion"
	// EmptyReasonAfterFilter 表示后续重排/过滤/截断后结果为空。
	EmptyReasonAfterFilter        = "Empty-After-Filter"
	EmptyReasonAfterChildRetrieve = "Empty-After-Child-Retrieve"
	EmptyReasonAfterParentFill    = "Empty-After-Parent-Fill"
	EmptyReasonAfterParentBudget  = "Empty-After-Parent-Budget"
)

// FusionConfig 控制多路候选的归一化与加权融合行为。
type FusionConfig struct {
	DenseWeight  float64
	SparseWeight float64
}

// RouteContribution 记录单条文档在某一路由上的归一化贡献。
type RouteContribution struct {
	Route           string
	RawScore        float64
	NormalizedScore float64
	WeightedScore   float64
}

// FusedDocument 表示融合但尚未去重的候选项。
type FusedDocument struct {
	Doc              *schema.Document
	Key              string
	Score            float64
	PrimaryRoute     string
	RouteContrib     map[string]float64
	RouteRawScores   map[string]float64
	Contributions    []RouteContribution
	SourceCollection string
}

// FuseRouteCandidates 对 dense/sparse 候选做归一化并生成统一主分。
func FuseRouteCandidates(denseDocs, sparseDocs []*schema.Document, cfg FusionConfig) []*FusedDocument {
	cfg = normalizeFusionConfig(cfg)
	inputs := []struct {
		route  string
		weight float64
		docs   []*schema.Document
	}{
		{route: routeDense, weight: cfg.DenseWeight, docs: denseDocs},
		{route: routeSparse, weight: cfg.SparseWeight, docs: sparseDocs},
	}

	results := make([]*FusedDocument, 0, len(denseDocs)+len(sparseDocs))
	for _, input := range inputs {
		rawScores := collectRouteScores(input.docs, input.route)
		stats := buildScoreStats(rawScores)
		for _, doc := range input.docs {
			if doc == nil {
				continue
			}
			key := buildDedupeKey(doc)
			if key == "" {
				continue
			}

			rawScore := readRouteScore(doc, input.route)
			normalizedScore := normalizeRouteScore(rawScore, stats)
			weightedScore := normalizedScore * input.weight

			annotatedDoc := cloneDocumentWithMetadata(doc)
			if annotatedDoc.MetaData == nil {
				annotatedDoc.MetaData = make(map[string]interface{})
			}
			annotatedDoc.MetaData["fusion_score"] = weightedScore
			annotatedDoc.MetaData["score"] = weightedScore
			annotatedDoc.MetaData["route"] = input.route
			annotatedDoc.MetaData["route_score"] = rawScore

			results = append(results, &FusedDocument{
				Doc:          annotatedDoc,
				Key:          key,
				Score:        weightedScore,
				PrimaryRoute: input.route,
				RouteContrib: map[string]float64{
					input.route: weightedScore,
				},
				RouteRawScores: map[string]float64{
					input.route: rawScore,
				},
				Contributions: []RouteContribution{
					{
						Route:           input.route,
						RawScore:        rawScore,
						NormalizedScore: normalizedScore,
						WeightedScore:   weightedScore,
					},
				},
				SourceCollection: readCollectionFromDoc(doc),
			})
		}
	}

	sort.SliceStable(results, func(i, j int) bool {
		if nearlyEqual(results[i].Score, results[j].Score) {
			return results[i].Key < results[j].Key
		}
		return results[i].Score > results[j].Score
	})
	return results
}

func normalizeFusionConfig(cfg FusionConfig) FusionConfig {
	if cfg.DenseWeight <= 0 {
		cfg.DenseWeight = 0.7
	}
	if cfg.SparseWeight <= 0 {
		cfg.SparseWeight = 0.3
	}
	total := cfg.DenseWeight + cfg.SparseWeight
	if total <= 0 {
		return FusionConfig{DenseWeight: 0.7, SparseWeight: 0.3}
	}
	cfg.DenseWeight = cfg.DenseWeight / total
	cfg.SparseWeight = cfg.SparseWeight / total
	return cfg
}

type scoreStats struct {
	min float64
	max float64
}

func buildScoreStats(scores []float64) scoreStats {
	if len(scores) == 0 {
		return scoreStats{}
	}
	stats := scoreStats{min: scores[0], max: scores[0]}
	for _, score := range scores[1:] {
		if score < stats.min {
			stats.min = score
		}
		if score > stats.max {
			stats.max = score
		}
	}
	return stats
}

func collectRouteScores(docs []*schema.Document, route string) []float64 {
	scores := make([]float64, 0, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		scores = append(scores, readRouteScore(doc, route))
	}
	return scores
}

func normalizeRouteScore(score float64, stats scoreStats) float64 {
	if stats.max <= stats.min {
		if score <= 0 {
			return 0
		}
		return 1
	}
	normalized := (score - stats.min) / (stats.max - stats.min)
	if normalized < 0 {
		return 0
	}
	if normalized > 1 {
		return 1
	}
	return normalized
}

func readRouteScore(doc *schema.Document, route string) float64 {
	if doc == nil || doc.MetaData == nil {
		return 0
	}

	switch route {
	case routeDense:
		if value, ok := doc.MetaData["dense_score"]; ok {
			if score, ok := castScore(value); ok {
				return score
			}
		}
	case routeSparse:
		if value, ok := doc.MetaData["sparse_score"]; ok {
			if score, ok := castScore(value); ok {
				return score
			}
		}
	}

	return readDocScore(doc)
}

func castScore(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

func buildDedupeKey(doc *schema.Document) string {
	if doc == nil {
		return ""
	}
	documentID := strings.TrimSpace(readMetadataString(doc, "document_id"))
	chunkID := strings.TrimSpace(readMetadataString(doc, "chunk_id"))
	if documentID != "" && chunkID != "" {
		return fmt.Sprintf("%s:%s", documentID, chunkID)
	}
	if documentID != "" {
		return documentID
	}
	if docID := strings.TrimSpace(doc.ID); docID != "" {
		return docID
	}
	return buildPseudoDocID(doc)
}

func cloneDocumentWithMetadata(doc *schema.Document) *schema.Document {
	if doc == nil {
		return nil
	}
	cloned := &schema.Document{
		ID:      doc.ID,
		Content: doc.Content,
	}
	if doc.MetaData != nil {
		cloned.MetaData = make(map[string]interface{}, len(doc.MetaData))
		for key, value := range doc.MetaData {
			cloned.MetaData[key] = value
		}
	}
	return cloned
}

func readMetadataString(doc *schema.Document, key string) string {
	if doc == nil || doc.MetaData == nil {
		return ""
	}
	value, ok := doc.MetaData[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func readCollectionFromDoc(doc *schema.Document) string {
	if doc == nil {
		return ""
	}
	for _, key := range []string{"collection", "collection_name"} {
		if value := readMetadataString(doc, key); value != "" {
			return value
		}
	}
	return ""
}

func nearlyEqual(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9
}
