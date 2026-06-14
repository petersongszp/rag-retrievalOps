package retrieval

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
)

const (
	DefaultRerankModelJaccardV1 = "jaccard-v1"
	DefaultRerankVersion        = "jaccard-v1"
)

// RerankResult carries reranked documents and observable metadata for L5.
type RerankResult struct {
	Documents  []*schema.Document
	Model      string
	Version    string
	Latency    time.Duration
	Fallback   bool
	Reason     string
	CandidateN int
	PreRanks   map[string]int
	PostRanks  map[string]int
}

// Reranker is the configurable rerank interface for hybrid retrieval.
type Reranker interface {
	Rerank(ctx context.Context, query string, docs []*schema.Document) (*RerankResult, error)
}

// JaccardRerankerConfig configures the fallback Jaccard reranker.
type JaccardRerankerConfig struct {
	OriginalScoreWeight float64
	TopK                int
	ModelName           string
	Version             string
}

// JaccardReranker reranks by mixing original score with Jaccard similarity.
type JaccardReranker struct {
	config *JaccardRerankerConfig
}

// NewJaccardReranker creates a new Jaccard reranker.
func NewJaccardReranker(config *JaccardRerankerConfig) *JaccardReranker {
	if config == nil {
		config = &JaccardRerankerConfig{
			OriginalScoreWeight: 0.7,
			TopK:                5,
			ModelName:           DefaultRerankModelJaccardV1,
			Version:             DefaultRerankVersion,
		}
	}
	if config.OriginalScoreWeight <= 0 || config.OriginalScoreWeight > 1 {
		config.OriginalScoreWeight = 0.7
	}
	if config.TopK <= 0 {
		config.TopK = 5
	}
	if strings.TrimSpace(config.ModelName) == "" {
		config.ModelName = DefaultRerankModelJaccardV1
	}
	if strings.TrimSpace(config.Version) == "" {
		config.Version = config.ModelName
	}
	return &JaccardReranker{config: config}
}

// ConfigurableReranker wraps a primary reranker with timeout and fallback behavior.
type ConfigurableReranker struct {
	modelName string
	timeout   time.Duration
	primary   Reranker
	fallback  Reranker
}

// NewConfigurableReranker creates a reranker that can fall back on timeout or error.
func NewConfigurableReranker(modelName string, timeout time.Duration, primary, fallback Reranker) *ConfigurableReranker {
	name := strings.TrimSpace(modelName)
	if name == "" {
		name = DefaultRerankModelJaccardV1
	}
	return &ConfigurableReranker{
		modelName: name,
		timeout:   timeout,
		primary:   primary,
		fallback:  fallback,
	}
}

// Rerank runs the primary reranker first and falls back when needed.
func (r *ConfigurableReranker) Rerank(ctx context.Context, query string, docs []*schema.Document) (*RerankResult, error) {
	if len(docs) == 0 {
		return &RerankResult{
			Documents:  []*schema.Document{},
			Model:      r.modelName,
			Version:    r.modelName,
			CandidateN: 0,
		}, nil
	}

	if r.primary == nil {
		if r.fallback == nil {
			return nil, fmt.Errorf("reranker is not configured")
		}
		result, err := r.fallback.Rerank(ctx, query, docs)
		if err != nil {
			return nil, err
		}
		result.Fallback = true
		if strings.TrimSpace(result.Reason) == "" {
			result.Reason = "primary_unavailable"
		}
		return result, nil
	}

	runCtx := ctx
	var cancel context.CancelFunc
	if r.timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, r.timeout)
		defer cancel()
	}

	result, err := r.primary.Rerank(runCtx, query, docs)
	if err == nil {
		return result, nil
	}

	if r.fallback == nil {
		return nil, err
	}

	fallbackResult, fallbackErr := r.fallback.Rerank(ctx, query, docs)
	if fallbackErr != nil {
		return nil, fmt.Errorf("primary rerank failed: %w; fallback rerank failed: %v", err, fallbackErr)
	}
	fallbackResult.Fallback = true
	fallbackResult.Reason = classifyRerankFallbackReason(runCtx, err)
	return fallbackResult, nil
}

func classifyRerankFallbackReason(ctx context.Context, err error) string {
	if err == nil {
		return ""
	}
	if ctx != nil && ctx.Err() == context.DeadlineExceeded {
		return "timeout"
	}
	if ctx != nil && ctx.Err() == context.Canceled {
		return "canceled"
	}
	return "error"
}

// Rerank reranks the documents and annotates them with L5 metadata.
func (r *JaccardReranker) Rerank(ctx context.Context, query string, docs []*schema.Document) (*RerankResult, error) {
	start := time.Now()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return &RerankResult{
			Documents:  []*schema.Document{},
			Model:      r.config.ModelName,
			Version:    r.config.Version,
			Latency:    time.Since(start),
			CandidateN: 0,
		}, nil
	}

	queryTokens := tokenize(query)
	type scoredDoc struct {
		doc   *schema.Document
		score float64
	}

	scoredDocs := make([]scoredDoc, 0, len(docs))
	preRanks := make(map[string]int, len(docs))
	for idx, doc := range docs {
		if doc != nil {
			preRanks[doc.ID] = idx + 1
		}
	}
	for _, doc := range docs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		contentTokens := tokenize(doc.Content)
		jaccardScore := calculateJaccardSimilarity(queryTokens, contentTokens)

		originalScore := readDocScore(doc)
		if originalScore <= 0 {
			originalScore = 0.5
		}

		finalScore := (originalScore * r.config.OriginalScoreWeight) + (jaccardScore * (1 - r.config.OriginalScoreWeight))
		annotateRerankMetadata(doc, finalScore, r.config.Version, 0, preRanks[doc.ID], 0)

		scoredDocs = append(scoredDocs, scoredDoc{
			doc:   doc,
			score: finalScore,
		})
	}

	sort.Slice(scoredDocs, func(i, j int) bool {
		return scoredDocs[i].score > scoredDocs[j].score
	})

	resultTopK := r.config.TopK
	if resultTopK > len(scoredDocs) {
		resultTopK = len(scoredDocs)
	}

	latency := time.Since(start)
	resultDocs := make([]*schema.Document, 0, resultTopK)
	postRanks := make(map[string]int, resultTopK)
	for i := 0; i < resultTopK; i++ {
		postRanks[scoredDocs[i].doc.ID] = i + 1
		annotateRerankMetadata(scoredDocs[i].doc, scoredDocs[i].score, r.config.Version, latency.Milliseconds(), preRanks[scoredDocs[i].doc.ID], i+1)
		resultDocs = append(resultDocs, scoredDocs[i].doc)
	}

	return &RerankResult{
		Documents:  resultDocs,
		Model:      r.config.ModelName,
		Version:    r.config.Version,
		Latency:    latency,
		CandidateN: len(docs),
		PreRanks:   preRanks,
		PostRanks:  postRanks,
	}, nil
}

func annotateRerankMetadata(doc *schema.Document, score float64, version string, latencyMS int64, preRank, postRank int) {
	if doc == nil {
		return
	}
	if doc.MetaData == nil {
		doc.MetaData = make(map[string]interface{})
	}

	originalScore := readDocScore(doc)
	doc.MetaData["rerank_score"] = score
	doc.MetaData["rerank_version"] = version
	doc.MetaData["rerank_latency_ms"] = latencyMS
	doc.MetaData["pre_rerank_rank"] = preRank
	doc.MetaData["post_rerank_rank"] = postRank
	doc.MetaData["original_score"] = originalScore
	doc.MetaData["score_delta"] = score - originalScore

	source := ensureSourceMetadata(doc)
	source["rerank_score"] = score
	if _, exists := source["retriever_version"]; !exists {
		source["retriever_version"] = hybridRetrieverVersion
	}
	source["rerank_version"] = version
	source["rerank_latency_ms"] = latencyMS
	source["pre_rerank_rank"] = preRank
	source["post_rerank_rank"] = postRank
	source["original_score"] = originalScore
	source["score_delta"] = score - originalScore
	doc.MetaData["source"] = source
	annotateParentChildSource(doc)
}

func ensureSourceMetadata(doc *schema.Document) map[string]interface{} {
	source := make(map[string]interface{})
	if doc != nil && doc.MetaData != nil {
		switch existing := doc.MetaData["source"].(type) {
		case map[string]interface{}:
			if existing != nil {
				for key, value := range existing {
					source[key] = value
				}
			}
		case string:
			if strings.TrimSpace(existing) != "" {
				source["origin"] = strings.TrimSpace(existing)
			}
		default:
			if existing != nil {
				if value := strings.TrimSpace(fmt.Sprint(existing)); value != "" {
					source["origin"] = value
				}
			}
		}
	}
	return source
}

func annotateParentChildSource(doc *schema.Document) {
	if doc == nil {
		return
	}
	if doc.MetaData == nil {
		doc.MetaData = make(map[string]interface{})
	}

	source := ensureSourceMetadata(doc)
	copyMetadataFieldsToMap(source, doc.MetaData,
		"document_id",
		"chunk_id",
		"parent_id",
		"child_id",
		"chunk_index",
		"section_title",
		"hierarchy_path",
		"parent_start_offset",
		"parent_end_offset",
		"child_start_offset",
		"child_end_offset",
		"parent_build_strategy",
		"parent_build_version",
		"parent_token_count",
	)

	parentChildAvailable := false
	if value, ok := doc.MetaData["parent_child_available"]; ok {
		parentChildAvailable = castBool(value)
	} else {
		parentChildAvailable = strings.TrimSpace(readMetadataString(doc, "parent_id")) != "" &&
			strings.TrimSpace(readMetadataString(doc, "child_id")) != ""
		doc.MetaData["parent_child_available"] = parentChildAvailable
	}
	source["parent_child_available"] = parentChildAvailable
	doc.MetaData["source"] = source
}

func copyMetadataFieldsToMap(target map[string]interface{}, metadata map[string]interface{}, keys ...string) {
	if target == nil || metadata == nil {
		return
	}
	for _, key := range keys {
		if value, exists := metadata[key]; exists && value != nil {
			target[key] = value
		}
	}
}

func castBool(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return strings.EqualFold(strings.TrimSpace(fmt.Sprint(v)), "true")
	}
}

func tokenize(text string) map[string]struct{} {
	tokens := make(map[string]struct{})
	parts := strings.Fields(strings.ToLower(text))
	for _, p := range parts {
		p = strings.Trim(p, ".,!?-;\"'()[]{}")
		if len(p) > 0 {
			tokens[p] = struct{}{}
		}
	}
	return tokens
}

func calculateJaccardSimilarity(set1, set2 map[string]struct{}) float64 {
	intersection := 0
	union := len(set1) + len(set2)

	for k := range set1 {
		if _, exists := set2[k]; exists {
			intersection++
		}
	}

	union -= intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
