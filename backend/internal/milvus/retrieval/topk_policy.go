package retrieval

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"
)

const (
	TruncateReasonNone        = ""
	TruncateReasonFinalTopK   = "final_topk"
	TruncateReasonTokenBudget = "token_budget"

	TopKPolicyVersionRule      = "phase2-rule-v1"
	TopKPolicyVersionStrategic = "phase3-strategic-v1"
)

type DynamicTopKConfig struct {
	Enabled              bool
	MinTopK              int
	MaxTopK              int
	TokenBudget          int
	MinAnswerChunks      int
	StrategicEnabled     bool
	StrategicMinTopK     int
	StrategicMaxTopK     int
	StrategicBudgetRatio float64
}

type TopKDecision struct {
	CandidateTopK          int
	RequestedTopK          int
	FinalTopK              int
	TokenBudget            int
	TokenBudgetRemaining   int
	EstimatedContextTokens int
	TruncateReason         string
	PolicyVersion          string
	ScoreDistribution      string
	RerankGap              float64
	EvidenceDensity        float64
	DecisionReason         string
}

func DecideDynamicTopK(query string, candidateTopK int, requestedTopK int, cfg DynamicTopKConfig) TopKDecision {
	minTopK, maxTopK := resolveTopKBounds(cfg.MinTopK, cfg.MaxTopK, candidateTopK)

	finalTopK := requestedTopK
	if finalTopK <= 0 {
		finalTopK = maxTopK
	}
	if !cfg.Enabled {
		finalTopK = clampInt(finalTopK, minTopK, maxTopK)
		return TopKDecision{
			CandidateTopK:  candidateTopK,
			RequestedTopK:  requestedTopK,
			FinalTopK:      finalTopK,
			TokenBudget:    cfg.TokenBudget,
			PolicyVersion:  TopKPolicyVersionRule,
			DecisionReason: "dynamic_topk_disabled",
		}
	}

	queryTrimmed := strings.TrimSpace(query)
	runeCount := utf8.RuneCountInString(queryTrimmed)
	termCount := len(strings.Fields(queryTrimmed))

	ruleTopK := minTopK
	reasons := make([]string, 0, 2)
	switch {
	case isBroadQuery(queryTrimmed):
		ruleTopK = maxTopK
		reasons = append(reasons, "broad_query")
	case runeCount >= 48 || termCount >= 8:
		ruleTopK = maxTopK
		reasons = append(reasons, "long_query")
	case runeCount >= 24 || termCount >= 5:
		ruleTopK = minTopK + (maxTopK-minTopK)/2 + 1
		reasons = append(reasons, "medium_query")
	case isShortPreciseQuery(queryTrimmed):
		ruleTopK = minTopK
		reasons = append(reasons, "short_precise_query")
	default:
		ruleTopK = minTopK + (maxTopK-minTopK)/2
		reasons = append(reasons, "default_mid_range")
	}

	finalTopK = clampInt(ruleTopK, minTopK, maxTopK)
	if requestedTopK > 0 && requestedTopK < finalTopK {
		finalTopK = clampInt(requestedTopK, minTopK, maxTopK)
		reasons = append(reasons, "requested_cap")
	}

	return TopKDecision{
		CandidateTopK:  candidateTopK,
		RequestedTopK:  requestedTopK,
		FinalTopK:      finalTopK,
		TokenBudget:    cfg.TokenBudget,
		PolicyVersion:  TopKPolicyVersionRule,
		DecisionReason: strings.Join(reasons, "+"),
	}
}

func DecideStrategicTopK(query string, candidateTopK int, requestedTopK int, docs []*schema.Document, cfg DynamicTopKConfig) (decision TopKDecision) {
	base := DecideDynamicTopK(query, candidateTopK, requestedTopK, cfg)
	if !cfg.StrategicEnabled || len(docs) == 0 {
		return base
	}
	decision = base

	defer func() {
		if recovered := recover(); recovered != nil {
			decision = base
			decision.PolicyVersion = TopKPolicyVersionRule
			decision.DecisionReason = strings.Trim(strings.Join([]string{
				strings.TrimSpace(base.DecisionReason),
				fmt.Sprintf("strategic_fallback:%v", recovered),
			}, "+"), "+")
		}
	}()

	minTopK, maxTopK := resolveTopKBounds(cfg.StrategicMinTopK, cfg.StrategicMaxTopK, candidateTopK)
	if maxTopK < minTopK {
		maxTopK = minTopK
	}
	signals := analyzeStrategicTopKSignals(docs)
	reasons := []string{"strategic_enabled"}
	finalTopK := clampInt(base.FinalTopK, minTopK, maxTopK)

	switch signals.Distribution {
	case "cliff":
		delta := 1
		if signals.RerankGap >= 0.18 {
			delta = 2
		}
		finalTopK -= delta
		reasons = append(reasons, "score_cliff")
	case "flat":
		finalTopK += 2
		reasons = append(reasons, "flat_distribution")
	default:
		reasons = append(reasons, "balanced_distribution")
	}

	if signals.ParentDiversity >= minInt(len(docs), 3) && signals.DominantParentShare < 0.7 {
		finalTopK++
		reasons = append(reasons, "diverse_parent_coverage")
	} else if signals.DominantParentShare >= 0.75 {
		finalTopK--
		reasons = append(reasons, "single_parent_concentration")
	}

	if signals.EvidenceDensity < 0.34 {
		finalTopK = minInt(finalTopK, base.FinalTopK)
		reasons = append(reasons, "low_evidence_density_no_expand")
	} else if signals.EvidenceDensity >= 0.7 && signals.RerankGap < 0.08 {
		finalTopK++
		reasons = append(reasons, "dense_evidence")
	}

	effectiveBudget := resolveStrategicTokenBudget(cfg.TokenBudget, cfg.StrategicBudgetRatio)
	estimatedTokens := estimateTopKTokens(docs, finalTopK)
	if effectiveBudget > 0 {
		budgetTopK, budgetTokens := estimateBudgetCappedTopK(docs, effectiveBudget, cfg.MinAnswerChunks, minTopK, maxTopK)
		if budgetTopK < finalTopK {
			finalTopK = budgetTopK
			estimatedTokens = budgetTokens
			reasons = append(reasons, "token_budget_cap")
		}
	}

	finalTopK = clampInt(finalTopK, minTopK, maxTopK)
	if requestedTopK > 0 && requestedTopK < finalTopK {
		finalTopK = clampInt(requestedTopK, minTopK, maxTopK)
		reasons = append(reasons, "requested_cap")
	}
	estimatedTokens = estimateTopKTokens(docs, finalTopK)

	tokenBudgetRemaining := 0
	if effectiveBudget > 0 && estimatedTokens < effectiveBudget {
		tokenBudgetRemaining = effectiveBudget - estimatedTokens
	}

	decision = TopKDecision{
		CandidateTopK:          candidateTopK,
		RequestedTopK:          requestedTopK,
		FinalTopK:              finalTopK,
		TokenBudget:            effectiveBudget,
		TokenBudgetRemaining:   tokenBudgetRemaining,
		EstimatedContextTokens: estimatedTokens,
		PolicyVersion:          TopKPolicyVersionStrategic,
		ScoreDistribution:      signals.Summary,
		RerankGap:              signals.RerankGap,
		EvidenceDensity:        signals.EvidenceDensity,
		DecisionReason:         strings.Join(reasons, "+"),
	}
	return decision
}

func ApplyTokenBudgetGuard(docs []*schema.Document, decision TopKDecision, cfg DynamicTopKConfig) ([]*schema.Document, TopKDecision) {
	if len(docs) == 0 {
		return docs, decision
	}

	guardTopK := decision.FinalTopK
	if guardTopK <= 0 || guardTopK > len(docs) {
		guardTopK = len(docs)
	}
	if guardTopK < 1 {
		guardTopK = 1
	}
	if cfg.MinAnswerChunks <= 0 {
		cfg.MinAnswerChunks = 1
	}
	if cfg.MinAnswerChunks > len(docs) {
		cfg.MinAnswerChunks = len(docs)
	}
	if guardTopK < cfg.MinAnswerChunks {
		guardTopK = cfg.MinAnswerChunks
	}

	truncated := docs
	if len(truncated) > guardTopK {
		truncated = truncated[:guardTopK]
		decision.TruncateReason = TruncateReasonFinalTopK
	}

	if decision.TokenBudget <= 0 {
		decision.EstimatedContextTokens = estimateTopKTokens(truncated, len(truncated))
		decision.TokenBudgetRemaining = 0
		decision.FinalTopK = len(truncated)
		return truncated, decision
	}

	totalTokens := 0
	budgeted := make([]*schema.Document, 0, len(truncated))
	for idx, doc := range truncated {
		docTokens := estimateDocumentTokens(doc)
		if idx < cfg.MinAnswerChunks {
			budgeted = append(budgeted, doc)
			totalTokens += docTokens
			continue
		}
		if totalTokens+docTokens > decision.TokenBudget {
			decision.TruncateReason = TruncateReasonTokenBudget
			break
		}
		budgeted = append(budgeted, doc)
		totalTokens += docTokens
	}

	if len(budgeted) == 0 {
		budgeted = truncated[:1]
		totalTokens = estimateDocumentTokens(budgeted[0])
	}
	if totalTokens < decision.TokenBudget {
		decision.TokenBudgetRemaining = decision.TokenBudget - totalTokens
	} else {
		decision.TokenBudgetRemaining = 0
	}
	decision.EstimatedContextTokens = totalTokens
	decision.FinalTopK = len(budgeted)
	return budgeted, decision
}

type strategicTopKSignals struct {
	Distribution        string
	Summary             string
	RerankGap           float64
	EvidenceDensity     float64
	ParentDiversity     int
	DominantParentShare float64
}

func analyzeStrategicTopKSignals(docs []*schema.Document) strategicTopKSignals {
	scores := make([]float64, 0, len(docs))
	strongHits := 0
	parentCounts := make(map[string]int, len(docs))
	maxParentHits := 0

	for _, doc := range docs {
		if doc == nil {
			continue
		}
		score := readStrategicScore(doc)
		scores = append(scores, score)

		parentKey := strings.TrimSpace(readMetadataString(doc, "parent_id"))
		if parentKey == "" {
			parentKey = buildDedupeKey(doc)
		}
		parentCounts[parentKey]++
		if parentCounts[parentKey] > maxParentHits {
			maxParentHits = parentCounts[parentKey]
		}
	}

	if len(scores) == 0 {
		return strategicTopKSignals{
			Distribution: "empty",
			Summary:      "empty",
		}
	}

	topScore := scores[0]
	for _, score := range scores {
		if topScore <= 0 {
			if score > 0 {
				strongHits++
			}
			continue
		}
		if score >= topScore*0.82 {
			strongHits++
		}
	}

	rerankGap := 0.0
	if len(scores) > 1 {
		rerankGap = maxFloat64(0, scores[0]-scores[1])
	}

	distribution := classifyScoreDistribution(scores, rerankGap)
	evidenceDensity := float64(strongHits) / float64(len(scores))
	dominantParentShare := 1.0
	if len(scores) > 0 {
		dominantParentShare = float64(maxParentHits) / float64(len(scores))
	}

	return strategicTopKSignals{
		Distribution:        distribution,
		Summary:             fmt.Sprintf("%s(top=%.3f,gap=%.3f,strong=%d/%d)", distribution, scores[0], rerankGap, strongHits, len(scores)),
		RerankGap:           rerankGap,
		EvidenceDensity:     evidenceDensity,
		ParentDiversity:     len(parentCounts),
		DominantParentShare: dominantParentShare,
	}
}

func classifyScoreDistribution(scores []float64, rerankGap float64) string {
	if len(scores) <= 1 {
		return "single"
	}
	normalized := normalizeScoreWindow(scores)
	spread := scores[0] - scores[len(scores)-1]
	avgAdjGap := 0.0
	for idx := 1; idx < len(normalized); idx++ {
		avgAdjGap += math.Abs(normalized[idx-1] - normalized[idx])
	}
	avgAdjGap = avgAdjGap / float64(len(normalized)-1)

	if spread <= 0.08 {
		return "flat"
	}
	if rerankGap >= 0.15 || (spread > 0.12 && normalized[0]-normalized[1] >= 0.22) {
		return "cliff"
	}
	if avgAdjGap <= 0.08 {
		return "flat"
	}
	return "balanced"
}

func normalizeScoreWindow(scores []float64) []float64 {
	if len(scores) == 0 {
		return nil
	}
	maxScore := scores[0]
	minScore := scores[0]
	for _, score := range scores[1:] {
		if score > maxScore {
			maxScore = score
		}
		if score < minScore {
			minScore = score
		}
	}
	if maxScore <= minScore {
		out := make([]float64, len(scores))
		for idx := range out {
			out[idx] = 1
		}
		return out
	}
	out := make([]float64, 0, len(scores))
	for _, score := range scores {
		out = append(out, (score-minScore)/(maxScore-minScore))
	}
	return out
}

func readStrategicScore(doc *schema.Document) float64 {
	if doc == nil || doc.MetaData == nil {
		return 0
	}
	for _, key := range []string{"rerank_score", "fusion_score", "score"} {
		if value, ok := doc.MetaData[key]; ok {
			if score, ok := castScore(value); ok {
				return score
			}
		}
	}
	return 0
}

func estimateTopKTokens(docs []*schema.Document, limit int) int {
	if len(docs) == 0 || limit <= 0 {
		return 0
	}
	if limit > len(docs) {
		limit = len(docs)
	}
	total := 0
	for idx := 0; idx < limit; idx++ {
		total += estimateDocumentTokens(docs[idx])
	}
	return total
}

func estimateBudgetCappedTopK(docs []*schema.Document, budget int, minAnswerChunks int, minTopK int, maxTopK int) (int, int) {
	if len(docs) == 0 {
		return minTopK, 0
	}
	if budget <= 0 {
		return clampInt(len(docs), minTopK, maxTopK), estimateTopKTokens(docs, clampInt(len(docs), minTopK, maxTopK))
	}
	if minAnswerChunks <= 0 {
		minAnswerChunks = 1
	}

	totalTokens := 0
	allowedTopK := 0
	for idx, doc := range docs {
		if idx >= maxTopK {
			break
		}
		docTokens := estimateDocumentTokens(doc)
		if idx < minAnswerChunks {
			totalTokens += docTokens
			allowedTopK++
			continue
		}
		if totalTokens+docTokens > budget {
			break
		}
		totalTokens += docTokens
		allowedTopK++
	}

	if allowedTopK == 0 {
		allowedTopK = 1
		totalTokens = estimateDocumentTokens(docs[0])
	}
	allowedTopK = clampInt(allowedTopK, minTopK, maxTopK)
	return allowedTopK, totalTokens
}

func resolveStrategicTokenBudget(baseBudget int, ratio float64) int {
	if baseBudget <= 0 {
		return 0
	}
	if ratio <= 0 || ratio > 1 {
		ratio = 1
	}
	budget := int(math.Floor(float64(baseBudget) * ratio))
	if budget < 1 {
		return 1
	}
	return budget
}

func resolveTopKBounds(minTopK int, maxTopK int, candidateTopK int) (int, int) {
	if minTopK <= 0 {
		minTopK = 1
	}
	if maxTopK < minTopK {
		maxTopK = minTopK
	}
	if candidateTopK > 0 && maxTopK > candidateTopK {
		maxTopK = candidateTopK
	}
	if candidateTopK > 0 && minTopK > candidateTopK {
		minTopK = candidateTopK
	}
	if minTopK <= 0 {
		minTopK = 1
	}
	if maxTopK < minTopK {
		maxTopK = minTopK
	}
	return minTopK, maxTopK
}

func estimateDocumentTokens(doc *schema.Document) int {
	if doc == nil {
		return 0
	}
	content := strings.TrimSpace(doc.Content)
	if content == "" {
		return 0
	}
	runes := utf8.RuneCountInString(content)
	tokens := runes / 4
	if runes%4 != 0 {
		tokens++
	}
	if tokens < 1 {
		tokens = 1
	}
	return tokens
}

func isBroadQuery(query string) bool {
	lower := strings.ToLower(query)
	keywords := []string{
		"区别", "对比", "总结", "全面", "原理", "设计", "最佳实践",
		"difference", "compare", "overview", "design", "tradeoff", "best practice",
	}
	for _, keyword := range keywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func isShortPreciseQuery(query string) bool {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return false
	}
	return utf8.RuneCountInString(trimmed) <= 12 && len(strings.Fields(trimmed)) <= 3
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
