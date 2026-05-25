package retrieval

import (
	"strings"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"
)

const (
	TruncateReasonNone        = ""
	TruncateReasonFinalTopK   = "final_topk"
	TruncateReasonTokenBudget = "token_budget"
)

type DynamicTopKConfig struct {
	Enabled         bool
	MinTopK         int
	MaxTopK         int
	TokenBudget     int
	MinAnswerChunks int
}

type TopKDecision struct {
	CandidateTopK  int
	RequestedTopK  int
	FinalTopK      int
	TokenBudget    int
	TruncateReason string
}

func DecideDynamicTopK(query string, candidateTopK int, requestedTopK int, cfg DynamicTopKConfig) TopKDecision {
	minTopK := cfg.MinTopK
	if minTopK <= 0 {
		minTopK = 1
	}
	maxTopK := cfg.MaxTopK
	if maxTopK < minTopK {
		maxTopK = minTopK
	}
	if candidateTopK > 0 && maxTopK > candidateTopK {
		maxTopK = candidateTopK
	}

	finalTopK := requestedTopK
	if finalTopK <= 0 {
		finalTopK = maxTopK
	}
	if !cfg.Enabled {
		finalTopK = clampInt(finalTopK, minTopK, maxTopK)
		return TopKDecision{
			CandidateTopK: candidateTopK,
			RequestedTopK: requestedTopK,
			FinalTopK:     finalTopK,
			TokenBudget:   cfg.TokenBudget,
		}
	}

	queryTrimmed := strings.TrimSpace(query)
	runeCount := utf8.RuneCountInString(queryTrimmed)
	termCount := len(strings.Fields(queryTrimmed))

	ruleTopK := minTopK
	switch {
	case isBroadQuery(queryTrimmed):
		ruleTopK = maxTopK
	case runeCount >= 48 || termCount >= 8:
		ruleTopK = maxTopK
	case runeCount >= 24 || termCount >= 5:
		ruleTopK = minTopK + (maxTopK-minTopK)/2 + 1
	case isShortPreciseQuery(queryTrimmed):
		ruleTopK = minTopK
	default:
		ruleTopK = minTopK + (maxTopK-minTopK)/2
	}

	finalTopK = clampInt(ruleTopK, minTopK, maxTopK)
	if requestedTopK > 0 && requestedTopK < finalTopK {
		finalTopK = clampInt(requestedTopK, minTopK, maxTopK)
	}

	return TopKDecision{
		CandidateTopK: candidateTopK,
		RequestedTopK: requestedTopK,
		FinalTopK:     finalTopK,
		TokenBudget:   cfg.TokenBudget,
	}
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
	decision.FinalTopK = len(budgeted)
	return budgeted, decision
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
