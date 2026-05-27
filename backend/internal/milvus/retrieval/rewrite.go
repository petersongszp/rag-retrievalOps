package retrieval

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	RewriteStrategyNone                = "none"
	RewriteStrategyBlacklist           = "blacklist"
	RewriteStrategyRuleBased           = "rule_based"
	RewriteStrategyTimeout             = "timeout_fallback"
	RewriteStrategyDomainTerms         = "domain_terms"
	RewriteStrategyRouteSpecific       = "route_specific"
	RewriteStrategyModelAssistedShadow = "model_assisted_shadow"
	defaultDomainDictionaryVersion     = "term-dict-v1"
	defaultModelRewriteShadowRatio     = 0.1
	defaultDomainTermTimeout           = 120 * time.Millisecond
	defaultModelRewriteTimeout         = 150 * time.Millisecond
	modelRewriteRiskLow                = "low"
	modelRewriteRiskMedium             = "medium"
	modelRewriteRiskHigh               = "high"
)

type QueryRewriteRequest struct {
	Query         string
	KBID          uint64
	KBScope       string
	Language      DocumentLanguage
	Category      DocumentCategory
	Collection    string
	RequestID     string
	CandidateTopK int
}

type QueryRewriteResult struct {
	OriginalQuery         string
	RewriteQuery          string
	FinalQuery            string
	DenseQuery            string
	SparseQuery           string
	Strategy              string
	Applied               bool
	Skipped               bool
	ExpansionTerms        []string
	CorrectedTerms        []string
	BlockedByPolicy       bool
	RouteQueries          map[string]string
	RouteStrategies       map[string]string
	TermDictScope         string
	TermDictVersion       string
	TermHits              []string
	ModelRewriteApplied   bool
	ModelRewriteShadow    bool
	ModelRewriteRiskLevel string
	ModelRewriteTerms     []string
}

type QueryRewriter interface {
	Rewrite(ctx context.Context, request QueryRewriteRequest) QueryRewriteResult
}

type QueryRewriterConfig struct {
	MaxExpansions              int
	EnableDomainTerms          bool
	EnableRouteSpecificRewrite bool
	EnableModelAssistedRewrite bool
	DomainTermTimeout          time.Duration
	ModelRewriteTimeout        time.Duration
	ModelRewriteShadowRatio    float64
	DomainTerms                DomainTermProvider
	ModelAssistant             ModelRewriteAssistant
}

type DomainTermProvider interface {
	Resolve(ctx context.Context, request QueryRewriteRequest) DomainTermResolution
}

type DomainTermResolution struct {
	Scope    string
	Version  string
	Terms    map[string][]string
	HitTerms []string
}

type ModelRewriteAssistant interface {
	Assist(ctx context.Context, request ModelRewriteRequest) (ModelRewriteSuggestion, error)
}

type ModelRewriteRequest struct {
	Query           string
	Context         QueryRewriteRequest
	ExistingTerms   []string
	ExpansionBudget int
}

type ModelRewriteSuggestion struct {
	NormalizedTerms []string
	Aliases         []string
	Abbreviations   []string
	MustKeepTerms   []string
	RiskLevel       string
}

type ControlledQueryRewriter struct {
	config          QueryRewriterConfig
	abbreviations   map[string][]string
	aliases         map[string][]string
	typoCorrections map[string]string
	blacklist       []string
	domainTerms     DomainTermProvider
	modelAssistant  ModelRewriteAssistant
}

func NewControlledQueryRewriter(cfg *QueryRewriterConfig) *ControlledQueryRewriter {
	config := QueryRewriterConfig{
		MaxExpansions:           3,
		DomainTermTimeout:       defaultDomainTermTimeout,
		ModelRewriteTimeout:     defaultModelRewriteTimeout,
		ModelRewriteShadowRatio: defaultModelRewriteShadowRatio,
	}
	if cfg != nil {
		if cfg.MaxExpansions > 0 {
			config.MaxExpansions = cfg.MaxExpansions
		}
		config.EnableDomainTerms = cfg.EnableDomainTerms
		config.EnableRouteSpecificRewrite = cfg.EnableRouteSpecificRewrite
		config.EnableModelAssistedRewrite = cfg.EnableModelAssistedRewrite
		if cfg.DomainTermTimeout > 0 {
			config.DomainTermTimeout = cfg.DomainTermTimeout
		}
		if cfg.ModelRewriteTimeout > 0 {
			config.ModelRewriteTimeout = cfg.ModelRewriteTimeout
		}
		if cfg.ModelRewriteShadowRatio >= 0 && cfg.ModelRewriteShadowRatio <= 1 {
			config.ModelRewriteShadowRatio = cfg.ModelRewriteShadowRatio
		}
		config.DomainTerms = cfg.DomainTerms
		config.ModelAssistant = cfg.ModelAssistant
	}

	domainTerms := config.DomainTerms
	if config.EnableDomainTerms && domainTerms == nil {
		domainTerms = NewStaticDomainTermProvider(defaultDomainDictionaryVersion)
	}
	modelAssistant := config.ModelAssistant
	if config.EnableModelAssistedRewrite && modelAssistant == nil {
		modelAssistant = NewHeuristicModelRewriteAssistant()
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
			"cas":   {"compare and swap"},
			"stw":   {"stop the world"},
		},
		aliases: map[string][]string{
			"golang":       {"go"},
			"go":           {"golang"},
			"redis":        {"redis cache"},
			"es":           {"elasticsearch"},
			"spring":       {"spring framework"},
			"springboot":   {"spring boot"},
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
			"gpm":          "gmp",
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
			"ignore previous",
			"system prompt",
		},
		domainTerms:    domainTerms,
		modelAssistant: modelAssistant,
	}
}

func (r *ControlledQueryRewriter) Rewrite(ctx context.Context, request QueryRewriteRequest) QueryRewriteResult {
	trimmed := strings.TrimSpace(request.Query)
	result := QueryRewriteResult{
		OriginalQuery:   trimmed,
		FinalQuery:      trimmed,
		DenseQuery:      trimmed,
		SparseQuery:     trimmed,
		Strategy:        RewriteStrategyNone,
		RouteQueries:    map[string]string{routeDense: trimmed, routeSparse: trimmed},
		RouteStrategies: map[string]string{routeDense: RewriteStrategyNone, routeSparse: RewriteStrategyNone},
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

	strategyParts := make([]string, 0, 4)
	if len(tokens) > 0 {
		strategyParts = append(strategyParts, RewriteStrategyRuleBased)
	}

	denseTerms := make([]string, 0, len(tokens)+2)
	sparseTerms := make([]string, 0, len(tokens)+r.config.MaxExpansions)
	denseSeen := make(map[string]struct{}, len(tokens)+r.config.MaxExpansions)
	sparseSeen := make(map[string]struct{}, len(tokens)+r.config.MaxExpansions)

	addTerm := func(target *[]string, seen map[string]struct{}, term string) bool {
		normalized := normalizeRewriteTerm(term)
		if normalized == "" {
			return false
		}
		if _, exists := seen[normalized]; exists {
			return false
		}
		seen[normalized] = struct{}{}
		*target = append(*target, term)
		return true
	}

	correctedTerms := make([]string, 0, 2)
	expansions := make([]string, 0, r.config.MaxExpansions)
	expansionCount := 0
	addExpansion := func(term string, includeDense bool) bool {
		if expansionCount >= r.config.MaxExpansions {
			return false
		}
		if !addTerm(&sparseTerms, sparseSeen, term) {
			return false
		}
		expansions = append(expansions, term)
		expansionCount++
		if includeDense {
			addTerm(&denseTerms, denseSeen, term)
		}
		return true
	}

	for _, token := range tokens {
		select {
		case <-ctx.Done():
			result.Strategy = RewriteStrategyTimeout
			result.Skipped = true
			result.RewriteQuery = ""
			result.FinalQuery = trimmed
			result.DenseQuery = trimmed
			result.SparseQuery = trimmed
			result.RouteQueries = map[string]string{routeDense: trimmed, routeSparse: trimmed}
			result.RouteStrategies = map[string]string{routeDense: RewriteStrategyTimeout, routeSparse: RewriteStrategyTimeout}
			return result
		default:
		}

		normalized := normalizeRewriteTerm(token)
		if corrected, ok := r.typoCorrections[normalized]; ok {
			if addTerm(&denseTerms, denseSeen, corrected) {
				addTerm(&sparseTerms, sparseSeen, corrected)
				correctedTerms = append(correctedTerms, corrected)
			}
			normalized = normalizeRewriteTerm(corrected)
		}
		addTerm(&denseTerms, denseSeen, token)
		addTerm(&sparseTerms, sparseSeen, token)

		ruleCandidates := make([]string, 0, 4)
		if values, ok := r.abbreviations[normalized]; ok {
			ruleCandidates = append(ruleCandidates, values...)
		}
		if values, ok := r.aliases[normalized]; ok {
			ruleCandidates = append(ruleCandidates, values...)
		}
		for index, value := range ruleCandidates {
			addExpansion(value, r.config.EnableRouteSpecificRewrite && index == 0)
		}
	}

	if len(correctedTerms) == 0 && len(expansions) == 0 {
		strategyParts = strategyParts[:0]
	}

	domainResolution := DomainTermResolution{}
	if r.config.EnableDomainTerms && r.domainTerms != nil {
		domainCtx, cancel := context.WithTimeout(ctx, r.config.DomainTermTimeout)
		domainResolution = r.domainTerms.Resolve(domainCtx, request)
		cancel()
		if len(domainResolution.HitTerms) > 0 {
			strategyParts = appendIfMissing(strategyParts, RewriteStrategyDomainTerms)
			result.TermDictScope = domainResolution.Scope
			result.TermDictVersion = domainResolution.Version
			result.TermHits = append([]string(nil), domainResolution.HitTerms...)
			for _, hit := range domainResolution.HitTerms {
				candidates := domainResolution.Terms[hit]
				for index, candidate := range candidates {
					addExpansion(candidate, r.config.EnableRouteSpecificRewrite && index == 0)
				}
			}
		}
	}

	modelSuggestion := ModelRewriteSuggestion{}
	if r.shouldApplyModelAssist(request, tokens, correctedTerms, expansions, domainResolution) {
		modelCtx, cancel := context.WithTimeout(ctx, r.config.ModelRewriteTimeout)
		suggestion, err := r.modelAssistant.Assist(modelCtx, ModelRewriteRequest{
			Query:           trimmed,
			Context:         request,
			ExistingTerms:   append([]string(nil), sparseTerms...),
			ExpansionBudget: max(0, r.config.MaxExpansions-expansionCount),
		})
		cancel()
		if err == nil {
			modelSuggestion = sanitizeModelSuggestion(suggestion)
			result.ModelRewriteRiskLevel = modelSuggestion.RiskLevel
			if modelSuggestion.RiskLevel != modelRewriteRiskHigh {
				modelTerms := append([]string(nil), modelSuggestion.NormalizedTerms...)
				modelTerms = append(modelTerms, modelSuggestion.MustKeepTerms...)
				modelTerms = append(modelTerms, modelSuggestion.Aliases...)
				modelTerms = append(modelTerms, modelSuggestion.Abbreviations...)
				for index, term := range dedupeStrings(modelTerms) {
					if addExpansion(term, r.config.EnableRouteSpecificRewrite && index == 0) {
						result.ModelRewriteTerms = append(result.ModelRewriteTerms, term)
					}
				}
				if len(result.ModelRewriteTerms) > 0 {
					result.ModelRewriteApplied = true
					result.ModelRewriteShadow = true
					strategyParts = appendIfMissing(strategyParts, RewriteStrategyModelAssistedShadow)
				}
			}
		}
	}

	if !r.config.EnableRouteSpecificRewrite {
		denseTerms = append([]string(nil), sparseTerms...)
		denseSeen = cloneSet(sparseSeen)
	} else if !strings.EqualFold(strings.TrimSpace(strings.Join(denseTerms, " ")), strings.TrimSpace(strings.Join(sparseTerms, " "))) {
		strategyParts = appendIfMissing(strategyParts, RewriteStrategyRouteSpecific)
	}

	denseQuery := strings.TrimSpace(strings.Join(denseTerms, " "))
	sparseQuery := strings.TrimSpace(strings.Join(sparseTerms, " "))
	if denseQuery == "" {
		denseQuery = trimmed
	}
	if sparseQuery == "" {
		sparseQuery = trimmed
	}

	result.CorrectedTerms = correctedTerms
	result.ExpansionTerms = expansions
	result.DenseQuery = denseQuery
	result.SparseQuery = sparseQuery
	result.RouteQueries = map[string]string{
		routeDense:  denseQuery,
		routeSparse: sparseQuery,
	}
	result.RouteStrategies = map[string]string{
		routeDense:  resolveRouteRewriteStrategy(r.config.EnableRouteSpecificRewrite, denseQuery, trimmed, correctedTerms, result.TermHits, result.ModelRewriteApplied, true),
		routeSparse: resolveRouteRewriteStrategy(r.config.EnableRouteSpecificRewrite, sparseQuery, trimmed, correctedTerms, result.TermHits, result.ModelRewriteApplied, false),
	}

	result.Strategy = RewriteStrategyNone
	if len(strategyParts) > 0 {
		result.Strategy = strings.Join(dedupeStrings(strategyParts), "+")
	}

	finalQuery := sparseQuery
	if !r.config.EnableRouteSpecificRewrite {
		finalQuery = denseQuery
	}
	if finalQuery == "" {
		finalQuery = trimmed
	}
	if strings.EqualFold(finalQuery, trimmed) && strings.EqualFold(denseQuery, trimmed) && strings.EqualFold(sparseQuery, trimmed) {
		result.Skipped = true
		result.FinalQuery = trimmed
		result.RewriteQuery = ""
		result.Strategy = firstNonEmpty(result.Strategy, RewriteStrategyNone)
		return result
	}

	result.Applied = true
	result.FinalQuery = finalQuery
	result.RewriteQuery = finalQuery
	if strings.EqualFold(strings.TrimSpace(result.RewriteQuery), trimmed) && !strings.EqualFold(strings.TrimSpace(denseQuery), trimmed) {
		result.RewriteQuery = denseQuery
	}
	return result
}

func (r *ControlledQueryRewriter) shouldApplyModelAssist(
	request QueryRewriteRequest,
	tokens []string,
	correctedTerms []string,
	expansions []string,
	domainResolution DomainTermResolution,
) bool {
	if r == nil || !r.config.EnableModelAssistedRewrite || r.modelAssistant == nil {
		return false
	}
	query := strings.TrimSpace(request.Query)
	if query == "" || isHighRiskModelRewriteQuery(query) {
		return false
	}
	if !sampleModelRewriteShadow(request, r.config.ModelRewriteShadowRatio) {
		return false
	}
	if len(tokens) >= 10 {
		return false
	}
	return len(correctedTerms) == 0 || (len(expansions) == 0 && len(domainResolution.HitTerms) == 0)
}

func tokenizeRewriteTerms(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	lower := strings.ToLower(text)
	var tokens []string
	var current strings.Builder
	var prevIsCJK bool
	for _, r := range lower {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			prevIsCJK = false
			continue
		}
		isCJK := isCJKUnified(r)
		if isCJK && !prevIsCJK && current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		} else if !isCJK && prevIsCJK && current.Len() > 0 {
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

func isCJKUnified(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) ||
		(r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0x20000 && r <= 0x2A6DF) ||
		(r >= 0xF900 && r <= 0xFAFF)
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
	parts := []string{result.Strategy}
	if len(result.RouteStrategies) > 0 {
		parts = append(parts, "routes="+formatRouteStrategyPairs(result.RouteStrategies))
	}
	if result.TermDictScope != "" {
		parts = append(parts, "term_scope="+result.TermDictScope)
	}
	if result.TermDictVersion != "" {
		parts = append(parts, "term_version="+result.TermDictVersion)
	}
	if len(result.TermHits) > 0 {
		parts = append(parts, "term_hits="+strings.Join(dedupeStrings(result.TermHits), "|"))
	}
	if result.ModelRewriteApplied {
		parts = append(parts, "model_shadow=true")
	}
	if result.ModelRewriteRiskLevel != "" {
		parts = append(parts, "model_risk="+result.ModelRewriteRiskLevel)
	}
	if len(result.ModelRewriteTerms) > 0 {
		parts = append(parts, "model_terms="+strings.Join(dedupeStrings(result.ModelRewriteTerms), "|"))
	}
	if len(result.CorrectedTerms) > 0 {
		parts = append(parts, "corrected="+strings.Join(dedupeStrings(result.CorrectedTerms), "|"))
	}
	if len(result.ExpansionTerms) > 0 {
		parts = append(parts, "expanded="+strings.Join(dedupeStrings(result.ExpansionTerms), "|"))
	}
	return strings.Join(parts, ";")
}

func resolveRouteRewriteStrategy(enableRouteSpecific bool, routeQuery, originalQuery string, correctedTerms, termHits []string, modelApplied bool, dense bool) string {
	if strings.EqualFold(strings.TrimSpace(routeQuery), strings.TrimSpace(originalQuery)) {
		return RewriteStrategyNone
	}
	parts := make([]string, 0, 5)
	parts = append(parts, RewriteStrategyRuleBased)
	if enableRouteSpecific {
		if dense {
			parts = append(parts, RewriteStrategyRouteSpecific+":conservative")
		} else {
			parts = append(parts, RewriteStrategyRouteSpecific+":aggressive")
		}
	}
	if len(termHits) > 0 {
		parts = append(parts, RewriteStrategyDomainTerms)
	}
	if modelApplied {
		parts = append(parts, RewriteStrategyModelAssistedShadow)
	}
	if len(correctedTerms) > 0 {
		parts = append(parts, "typo_corrected")
	}
	return strings.Join(dedupeStrings(parts), "+")
}

func formatRouteStrategyPairs(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		value := strings.TrimSpace(values[key])
		if value == "" {
			continue
		}
		pairs = append(pairs, key+":"+value)
	}
	return strings.Join(pairs, "|")
}

func sampleModelRewriteShadow(request QueryRewriteRequest, ratio float64) bool {
	if ratio <= 0 {
		return false
	}
	if ratio >= 1 {
		return true
	}
	key := strings.ToLower(strings.TrimSpace(request.Query)) + "|" + strings.TrimSpace(request.KBScope) + "|" + request.Collection
	if request.KBID > 0 {
		key += "|" + strconv.FormatUint(request.KBID, 10)
	}
	sum := sha1.Sum([]byte(key))
	bucket := binary.BigEndian.Uint32(sum[:4]) % 10000
	return float64(bucket) < ratio*10000
}

func sanitizeModelSuggestion(suggestion ModelRewriteSuggestion) ModelRewriteSuggestion {
	suggestion.NormalizedTerms = dedupeStrings(suggestion.NormalizedTerms)
	suggestion.Aliases = dedupeStrings(suggestion.Aliases)
	suggestion.Abbreviations = dedupeStrings(suggestion.Abbreviations)
	suggestion.MustKeepTerms = dedupeStrings(suggestion.MustKeepTerms)
	switch strings.ToLower(strings.TrimSpace(suggestion.RiskLevel)) {
	case modelRewriteRiskLow:
		suggestion.RiskLevel = modelRewriteRiskLow
	case modelRewriteRiskHigh:
		suggestion.RiskLevel = modelRewriteRiskHigh
	default:
		suggestion.RiskLevel = modelRewriteRiskMedium
	}
	return suggestion
}

func isHighRiskModelRewriteQuery(query string) bool {
	lower := strings.ToLower(strings.TrimSpace(query))
	if lower == "" {
		return true
	}
	riskyMarkers := []string{
		"site:",
		"http://",
		"https://",
		"ignore previous",
		"system prompt",
		"```",
		"select ",
		"delete ",
		"drop ",
	}
	for _, marker := range riskyMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return len(tokenizeRewriteTerms(query)) > 12
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := normalizeRewriteTerm(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func uniqueStringsPreserveOrder(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := normalizeRewriteTerm(value)
		if normalized == "" {
			normalized = strings.TrimSpace(strings.ToLower(value))
		}
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, value)
	}
	return out
}

func appendIfMissing(values []string, value string) []string {
	for _, existing := range values {
		if strings.EqualFold(strings.TrimSpace(existing), strings.TrimSpace(value)) {
			return values
		}
	}
	return append(values, value)
}

func cloneSet(source map[string]struct{}) map[string]struct{} {
	if len(source) == 0 {
		return map[string]struct{}{}
	}
	cloned := make(map[string]struct{}, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
