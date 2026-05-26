package retrieval

import (
	"context"
	"sort"
	"strings"
	"unicode"
)

const (
	RewriteStrategyNone      = "none"
	RewriteStrategyBlacklist = "blacklist"
	RewriteStrategyRuleBased = "rule_based"
	RewriteStrategyTimeout   = "timeout_fallback"
)

type QueryRewriteResult struct {
	OriginalQuery   string
	RewriteQuery    string
	FinalQuery      string
	Strategy        string
	Applied         bool
	Skipped         bool
	ExpansionTerms  []string
	CorrectedTerms  []string
	BlockedByPolicy bool
}

type QueryRewriter interface {
	Rewrite(ctx context.Context, query string) QueryRewriteResult
}

type QueryRewriterConfig struct {
	MaxExpansions int
}

type ControlledQueryRewriter struct {
	config          QueryRewriterConfig
	abbreviations   map[string][]string
	aliases         map[string][]string
	typoCorrections map[string]string
	blacklist       []string
}

func NewControlledQueryRewriter(cfg *QueryRewriterConfig) *ControlledQueryRewriter {
	config := QueryRewriterConfig{
		MaxExpansions: 3,
	}
	if cfg != nil && cfg.MaxExpansions > 0 {
		config.MaxExpansions = cfg.MaxExpansions
	}

	return &ControlledQueryRewriter{
		config: config,
		abbreviations: map[string][]string{
			"jvm":   {"java virtual machine"},
			"gc":    {"garbage collection"},
			"rpc":   {"remote procedure call"},
			"mq":    {"message queue", "message broker"},
			"orm":   {"object relational mapping"},
			"ioc":   {"inversion of control"},
			"aop":   {"aspect oriented programming"},
			"ddl":   {"data definition language"},
			"dml":   {"data manipulation language"},
			"mvcc":  {"multi version concurrency control"},
			"mysql": {"my sql"},
			"k8s":   {"kubernetes"},
		},
		aliases: map[string][]string{
			"golang":       {"go"},
			"go":           {"golang"},
			"redis":        {"redis cache"},
			"es":           {"elasticsearch"},
			"spring":       {"spring framework"},
			"springboot":   {"spring boot"},
			"spring boot":  {"springboot"},
			"microservice": {"microservices", "distributed service"},
			"middleware":   {"middle ware"},
			"rabbitmq":     {"rabbit mq"},
			"rocketmq":     {"rocket mq"},
			"kubernetes":   {"k8s"},
		},
		typoCorrections: map[string]string{
			"sprinboot":    "springboot",
			"spingboot":    "springboot",
			"javva":        "java",
			"golnag":       "golang",
			"redsi":        "redis",
			"kafak":        "kafka",
			"elaticsearch": "elasticsearch",
			"kubenetes":    "kubernetes",
		},
		blacklist: []string{
			"\"",
			"'",
			"`",
			"site:",
			"http://",
			"https://",
			"select ",
			"update ",
			"delete ",
			"insert ",
			"drop ",
			"truncate ",
		},
	}
}

func (r *ControlledQueryRewriter) Rewrite(ctx context.Context, query string) QueryRewriteResult {
	trimmed := strings.TrimSpace(query)
	result := QueryRewriteResult{
		OriginalQuery: trimmed,
		FinalQuery:    trimmed,
		Strategy:      RewriteStrategyNone,
	}
	if trimmed == "" {
		result.Skipped = true
		return result
	}

	select {
	case <-ctx.Done():
		result.Strategy = RewriteStrategyTimeout
		result.Skipped = true
		return result
	default:
	}

	lowerQuery := strings.ToLower(trimmed)
	for _, token := range r.blacklist {
		if strings.Contains(lowerQuery, token) {
			result.Strategy = RewriteStrategyBlacklist
			result.Skipped = true
			result.BlockedByPolicy = true
			return result
		}
	}

	tokens := tokenizeRewriteTerms(trimmed)
	if len(tokens) == 0 {
		result.Skipped = true
		return result
	}

	expansionLimit := r.config.MaxExpansions
	rewriteTerms := make([]string, 0, len(tokens)+expansionLimit)
	seen := make(map[string]struct{}, len(tokens)+expansionLimit)
	expansions := make([]string, 0, expansionLimit)
	correctedTerms := make([]string, 0, 2)

	addTerm := func(term string) bool {
		normalized := normalizeRewriteTerm(term)
		if normalized == "" {
			return false
		}
		if _, exists := seen[normalized]; exists {
			return false
		}
		seen[normalized] = struct{}{}
		rewriteTerms = append(rewriteTerms, term)
		return true
	}
	addExpansion := func(term string) {
		if len(expansions) >= expansionLimit {
			return
		}
		if addTerm(term) {
			expansions = append(expansions, term)
		}
	}

	for _, token := range tokens {
		select {
		case <-ctx.Done():
			result.Strategy = RewriteStrategyTimeout
			result.Skipped = true
			result.RewriteQuery = ""
			result.FinalQuery = trimmed
			result.Applied = false
			return result
		default:
		}

		normalized := normalizeRewriteTerm(token)
		if corrected, ok := r.typoCorrections[normalized]; ok {
			if addTerm(corrected) {
				correctedTerms = append(correctedTerms, corrected)
			}
		}
		addTerm(token)
		if values, ok := r.abbreviations[normalized]; ok {
			for _, value := range values {
				addExpansion(value)
			}
		}
		if values, ok := r.aliases[normalized]; ok {
			for _, value := range values {
				addExpansion(value)
			}
		}
	}

	finalQuery := strings.TrimSpace(strings.Join(rewriteTerms, " "))
	if finalQuery == "" || strings.EqualFold(finalQuery, trimmed) {
		result.Skipped = true
		result.FinalQuery = trimmed
		result.RewriteQuery = ""
		result.CorrectedTerms = correctedTerms
		result.ExpansionTerms = expansions
		return result
	}

	result.Strategy = RewriteStrategyRuleBased
	result.Applied = true
	result.RewriteQuery = finalQuery
	result.FinalQuery = finalQuery
	result.ExpansionTerms = expansions
	result.CorrectedTerms = correctedTerms
	return result
}

func tokenizeRewriteTerms(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	lower := strings.ToLower(text)
	// 先按非字母数字字符拆分，再在中英文边界处拆分
	var tokens []string
	var current strings.Builder
	var prevIsCJK bool
	for _, r := range lower {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) {
			// 非字母数字字符：拆分
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			prevIsCJK = false
			continue
		}
		isCJK := isCJKUnified(r)
		if isCJK && !prevIsCJK && current.Len() > 0 {
			// 英文 → 中文边界：拆分
			tokens = append(tokens, current.String())
			current.Reset()
		} else if !isCJK && prevIsCJK && current.Len() > 0 {
			// 中文 → 英文边界：拆分
			tokens = append(tokens, current.String())
			current.Reset()
		}
		current.WriteRune(r)
		prevIsCJK = isCJK
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	seen := make(map[string]struct{}, len(tokens))
	out := make([]string, 0, len(tokens))
	for _, token := range tokens {
		normalized := normalizeRewriteTerm(token)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

// isCJKUnified 检测是否为 CJK 统一汉字（包括扩展）
func isCJKUnified(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified Ideographs
		(r >= 0x3400 && r <= 0x4DBF) || // CJK Unified Ideographs Extension A
		(r >= 0x20000 && r <= 0x2A6DF) || // CJK Unified Ideographs Extension B
		(r >= 0xF900 && r <= 0xFAFF) // CJK Compatibility Ideographs
}

func normalizeRewriteTerm(term string) string {
	trimmed := strings.TrimSpace(strings.ToLower(term))
	if len([]rune(trimmed)) < 2 {
		return ""
	}
	return trimmed
}

func formatRewriteStrategy(result QueryRewriteResult) string {
	if result.Strategy == "" {
		return RewriteStrategyNone
	}
	if len(result.ExpansionTerms) == 0 && len(result.CorrectedTerms) == 0 {
		return result.Strategy
	}
	parts := []string{result.Strategy}
	if len(result.CorrectedTerms) > 0 {
		sorted := append([]string(nil), result.CorrectedTerms...)
		sort.Strings(sorted)
		parts = append(parts, "corrected="+strings.Join(sorted, "|"))
	}
	if len(result.ExpansionTerms) > 0 {
		sorted := append([]string(nil), result.ExpansionTerms...)
		sort.Strings(sorted)
		parts = append(parts, "expanded="+strings.Join(sorted, "|"))
	}
	return strings.Join(parts, ";")
}
