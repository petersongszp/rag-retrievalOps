package retrieval

import (
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"
)

const (
	EvidenceGateResultDisabled     = "disabled"
	EvidenceGateResultPass         = "pass"
	EvidenceGateResultRefused      = "refused"
	EvidenceGateResultDegradedPass = "degraded_pass"

	RefusalReasonNoRetrievalHit            = "No-Retrieval-Hit"
	RefusalReasonLowRerankConfidence       = "Low-Rerank-Confidence"
	RefusalReasonInsufficientCitationCover = "Insufficient-Citation-Coverage"
	RefusalReasonContradictoryEvidence     = "Contradictory-Evidence"
	RefusalReasonOutOfKBScope              = "Out-Of-KB-Scope"
	EmptyReasonEvidenceRefusal             = "Evidence-Refusal"
)

type EvidenceGateConfig struct {
	Enabled             bool
	MinRerankScore      float64
	MinEvidenceDensity  float64
	MinCitationCoverage float64
}

type EvidenceGateOutcome struct {
	Result               string
	RefusalReason        string
	CitationSupportScore float64
	Error                string
}

func (o EvidenceGateOutcome) Refused() bool {
	return strings.EqualFold(strings.TrimSpace(o.Result), EvidenceGateResultRefused)
}

func EvaluateEvidenceGate(query string, docs []*schema.Document, metrics SearchMetrics, cfg EvidenceGateConfig) EvidenceGateOutcome {
	if !cfg.Enabled {
		return EvidenceGateOutcome{Result: EvidenceGateResultDisabled}
	}

	thresholds, err := resolveEvidenceThresholds(strings.TrimSpace(query), cfg)
	if err != nil {
		return EvidenceGateOutcome{
			Result: EvidenceGateResultDegradedPass,
			Error:  err.Error(),
		}
	}

	if len(docs) == 0 {
		return EvidenceGateOutcome{
			Result:        EvidenceGateResultRefused,
			RefusalReason: RefusalReasonNoRetrievalHit,
		}
	}

	citationSupportScore := computeCitationSupportScore(docs)
	maxRerankScore := computeMaxEvidenceScore(docs)
	evidenceDensity := metrics.EvidenceDensity
	if evidenceDensity <= 0 {
		evidenceDensity = computeFallbackEvidenceDensity(docs, thresholds.MinRerankScore)
	}

	outcome := EvidenceGateOutcome{
		Result:               EvidenceGateResultPass,
		CitationSupportScore: citationSupportScore,
	}

	if hasContradictoryEvidence(query, docs, thresholds.MinRerankScore) {
		outcome.Result = EvidenceGateResultRefused
		outcome.RefusalReason = RefusalReasonContradictoryEvidence
		return outcome
	}

	if isLikelyOutOfKBScope(query, docs, maxRerankScore, evidenceDensity, thresholds) {
		outcome.Result = EvidenceGateResultRefused
		outcome.RefusalReason = RefusalReasonOutOfKBScope
		return outcome
	}

	if maxRerankScore < thresholds.MinRerankScore || evidenceDensity < thresholds.MinEvidenceDensity {
		outcome.Result = EvidenceGateResultRefused
		outcome.RefusalReason = RefusalReasonLowRerankConfidence
		return outcome
	}

	if citationSupportScore < thresholds.MinCitationCoverage {
		outcome.Result = EvidenceGateResultRefused
		outcome.RefusalReason = RefusalReasonInsufficientCitationCover
		return outcome
	}

	return outcome
}

type evidenceThresholds struct {
	MinRerankScore      float64
	MinEvidenceDensity  float64
	MinCitationCoverage float64
}

func resolveEvidenceThresholds(query string, cfg EvidenceGateConfig) (evidenceThresholds, error) {
	minRerank := clampNormalizedRatio(cfg.MinRerankScore)
	minDensity := clampNormalizedRatio(cfg.MinEvidenceDensity)
	minCitation := clampNormalizedRatio(cfg.MinCitationCoverage)
	if minRerank <= 0 || minDensity <= 0 || minCitation <= 0 {
		return evidenceThresholds{}, ErrInvalidEvidenceGateConfig
	}

	multiplier := evidenceSensitivityMultiplier(query)
	return evidenceThresholds{
		MinRerankScore:      clampNormalizedRatio(minRerank * multiplier),
		MinEvidenceDensity:  clampNormalizedRatio(minDensity * multiplier),
		MinCitationCoverage: clampNormalizedRatio(minCitation * multiplier),
	}, nil
}

func evidenceSensitivityMultiplier(query string) float64 {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return 1
	}

	multiplier := 1.0
	termCount := len(strings.Fields(trimmed))
	runeCount := utf8.RuneCountInString(trimmed)

	if isBroadQuery(trimmed) {
		multiplier += 0.12
	}
	if runeCount <= 12 || termCount <= 2 {
		multiplier += 0.05
	}
	if containsAnyFold(trimmed, "how", "why", "compare", "difference", "区别", "对比", "怎么", "如何", "为什么") {
		multiplier += 0.05
	}

	return math.Min(multiplier, 1.25)
}

func computeCitationSupportScore(docs []*schema.Document) float64 {
	if len(docs) == 0 {
		return 0
	}

	limit := minInt(len(docs), 3)
	citableCount := 0
	for i := 0; i < limit; i++ {
		doc := docs[i]
		if doc == nil {
			continue
		}
		documentID := getFloatMetadataValue(doc, "document_id")
		chunkID := strings.TrimSpace(firstNonEmptyMetadata(doc, "chunk_id"))
		if documentID > 0 && (chunkID != "" || strings.TrimSpace(doc.ID) != "") {
			citableCount++
		}
	}
	return float64(citableCount) / float64(limit)
}

func computeMaxEvidenceScore(docs []*schema.Document) float64 {
	maxScore := 0.0
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		score := getFloatMetadataValue(doc, "rerank_score")
		if score <= 0 {
			score = getFloatMetadataValue(doc, "score")
		}
		if score > maxScore {
			maxScore = score
		}
	}
	return maxScore
}

func computeFallbackEvidenceDensity(docs []*schema.Document, minScore float64) float64 {
	if len(docs) == 0 {
		return 0
	}
	strongCount := 0
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		score := getFloatMetadataValue(doc, "rerank_score")
		if score <= 0 {
			score = getFloatMetadataValue(doc, "score")
		}
		if score >= minScore {
			strongCount++
		}
	}
	return float64(strongCount) / float64(len(docs))
}

func hasContradictoryEvidence(query string, docs []*schema.Document, minScore float64) bool {
	if len(docs) < 2 {
		return false
	}

	candidates := make([]*schema.Document, 0, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		score := getFloatMetadataValue(doc, "rerank_score")
		if score <= 0 {
			score = getFloatMetadataValue(doc, "score")
		}
		if score >= minScore {
			candidates = append(candidates, doc)
		}
	}
	if len(candidates) < 2 {
		return false
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return computeMaxEvidenceScore([]*schema.Document{candidates[i]}) > computeMaxEvidenceScore([]*schema.Document{candidates[j]})
	})

	queryTokens := tokenize(query)
	first := candidates[0]
	second := candidates[1]
	if !docTouchesQuery(first, queryTokens) || !docTouchesQuery(second, queryTokens) {
		return false
	}

	return containsNegationCue(first.Content) != containsNegationCue(second.Content)
}

func isLikelyOutOfKBScope(query string, docs []*schema.Document, maxScore, evidenceDensity float64, thresholds evidenceThresholds) bool {
	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return false
	}

	coverage := computeQueryCoverage(queryTokens, docs)
	if coverage >= 0.2 {
		return false
	}

	return maxScore < thresholds.MinRerankScore || evidenceDensity < thresholds.MinEvidenceDensity
}

func computeQueryCoverage(queryTokens map[string]struct{}, docs []*schema.Document) float64 {
	if len(queryTokens) == 0 {
		return 0
	}
	matched := 0
	for token := range queryTokens {
		if token == "" {
			continue
		}
		for _, doc := range docs {
			if doc == nil {
				continue
			}
			contentTokens := tokenize(doc.Content)
			if _, ok := contentTokens[token]; ok {
				matched++
				break
			}
		}
	}
	return float64(matched) / float64(len(queryTokens))
}

func docTouchesQuery(doc *schema.Document, queryTokens map[string]struct{}) bool {
	if doc == nil || len(queryTokens) == 0 {
		return false
	}
	contentTokens := tokenize(doc.Content)
	for token := range queryTokens {
		if _, ok := contentTokens[token]; ok {
			return true
		}
	}
	return false
}

func containsNegationCue(content string) bool {
	return containsAnyFold(content, " not ", " no ", " never ", " without ", " cannot ", " can't ", "禁止", "不能", "不支持", "不会", "无", "未")
}

func containsAnyFold(content string, keywords ...string) bool {
	lower := strings.ToLower(content)
	for _, keyword := range keywords {
		if strings.Contains(lower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func getFloatMetadataValue(doc *schema.Document, key string) float64 {
	if doc == nil || doc.MetaData == nil {
		return 0
	}
	if value, ok := doc.MetaData[key]; ok {
		if score, ok := castScore(value); ok {
			return score
		}
	}
	return 0
}

func firstNonEmptyMetadata(doc *schema.Document, keys ...string) string {
	if doc == nil || doc.MetaData == nil {
		return ""
	}
	for _, key := range keys {
		if value := strings.TrimSpace(getStringMetadata(doc.MetaData, key)); value != "" {
			return value
		}
	}
	return ""
}

func clampNormalizedRatio(value float64) float64 {
	switch {
	case value < 0:
		return 0
	case value > 1:
		return 1
	default:
		return value
	}
}

var ErrInvalidEvidenceGateConfig = errInvalidEvidenceGateConfig("invalid evidence gate config")

type errInvalidEvidenceGateConfig string

func (e errInvalidEvidenceGateConfig) Error() string {
	return string(e)
}
