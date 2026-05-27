package retrieval

import (
	"context"
	"sort"
	"strings"
	"sync"
)

type StaticDomainTermProvider struct {
	mu      sync.RWMutex
	version string
	scopes  map[string]map[string][]string
}

func NewStaticDomainTermProvider(version string) *StaticDomainTermProvider {
	if strings.TrimSpace(version) == "" {
		version = defaultDomainDictionaryVersion
	}
	provider := &StaticDomainTermProvider{
		version: version,
		scopes:  make(map[string]map[string][]string),
	}
	provider.RegisterScope("global", map[string][]string{
		"iac":   {"infrastructure as code"},
		"sso":   {"single sign on"},
		"ci":    {"continuous integration"},
		"cd":    {"continuous delivery"},
		"oauth": {"open authorization"},
	})
	provider.RegisterScope("language:java", map[string][]string{
		"juc":  {"java.util.concurrent", "concurrent utilities", "aqs"},
		"jmm":  {"java memory model"},
		"aqs":  {"abstract queued synchronizer"},
		"jfr":  {"java flight recorder"},
		"tlab": {"thread local allocation buffer"},
	})
	provider.RegisterScope("language:golang", map[string][]string{
		"gmp":     {"goroutine machine processor scheduler", "go scheduler"},
		"pprof":   {"performance profiling"},
		"gctrace": {"garbage collector trace"},
		"csp":     {"communicating sequential processes"},
	})
	provider.RegisterScope("language:middleware", map[string][]string{
		"dlq": {"dead letter queue"},
		"isr": {"in sync replica"},
		"osr": {"out of sync replica"},
		"eos": {"exactly once semantics"},
	})
	return provider
}

func (p *StaticDomainTermProvider) RegisterScope(scope string, terms map[string][]string) {
	scope = strings.TrimSpace(strings.ToLower(scope))
	if scope == "" || len(terms) == 0 {
		return
	}
	normalized := make(map[string][]string, len(terms))
	for key, values := range terms {
		termKey := normalizeRewriteTerm(key)
		if termKey == "" {
			continue
		}
		expansions := make([]string, 0, len(values))
		for _, value := range values {
			if normalizeRewriteTerm(value) == "" {
				continue
			}
			expansions = append(expansions, strings.TrimSpace(value))
		}
		if len(expansions) == 0 {
			continue
		}
		normalized[termKey] = expansions
	}
	if len(normalized) == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.scopes[scope] = normalized
}

func (p *StaticDomainTermProvider) Resolve(ctx context.Context, request QueryRewriteRequest) DomainTermResolution {
	select {
	case <-ctx.Done():
		return DomainTermResolution{Version: p.version}
	default:
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	scopeKeys := resolveDomainScopeKeys(request)
	lowerQuery := strings.ToLower(strings.TrimSpace(request.Query))
	tokenSet := make(map[string]struct{}, 8)
	for _, token := range tokenizeRewriteTerms(request.Query) {
		tokenSet[token] = struct{}{}
	}

	resolution := DomainTermResolution{
		Version: p.version,
		Terms:   make(map[string][]string),
	}
	appliedScopes := make([]string, 0, len(scopeKeys))
	hitTerms := make([]string, 0, 4)
	for _, scope := range scopeKeys {
		terms, ok := p.scopes[scope]
		if !ok {
			continue
		}
		appliedScopes = append(appliedScopes, scope)
		for key, values := range terms {
			select {
			case <-ctx.Done():
				resolution.Scope = strings.Join(appliedScopes, ">")
				resolution.HitTerms = dedupeStrings(hitTerms)
				return resolution
			default:
			}

			if !matchesRewriteKey(lowerQuery, tokenSet, key) {
				continue
			}
			resolution.Terms[key] = append([]string(nil), values...)
			hitTerms = append(hitTerms, key)
		}
	}
	resolution.Scope = strings.Join(appliedScopes, ">")
	resolution.HitTerms = dedupeStrings(hitTerms)
	return resolution
}

type HeuristicModelRewriteAssistant struct {
	knowledge map[string]ModelRewriteSuggestion
}

func NewHeuristicModelRewriteAssistant() *HeuristicModelRewriteAssistant {
	return &HeuristicModelRewriteAssistant{
		knowledge: map[string]ModelRewriteSuggestion{
			"aqs": {
				NormalizedTerms: []string{"abstract queued synchronizer"},
				Aliases:         []string{"queue synchronizer"},
				MustKeepTerms:   []string{"aqs"},
				RiskLevel:       modelRewriteRiskLow,
			},
			"cas": {
				NormalizedTerms: []string{"compare and swap"},
				Aliases:         []string{"atomic compare swap"},
				MustKeepTerms:   []string{"cas"},
				RiskLevel:       modelRewriteRiskLow,
			},
			"gmp": {
				NormalizedTerms: []string{"go scheduler"},
				Aliases:         []string{"goroutine scheduler"},
				MustKeepTerms:   []string{"gmp"},
				RiskLevel:       modelRewriteRiskLow,
			},
			"stw": {
				NormalizedTerms: []string{"stop the world"},
				Aliases:         []string{"gc pause"},
				MustKeepTerms:   []string{"stw"},
				RiskLevel:       modelRewriteRiskLow,
			},
			"tlab": {
				NormalizedTerms: []string{"thread local allocation buffer"},
				MustKeepTerms:   []string{"tlab"},
				RiskLevel:       modelRewriteRiskLow,
			},
		},
	}
}

func (a *HeuristicModelRewriteAssistant) Assist(ctx context.Context, request ModelRewriteRequest) (ModelRewriteSuggestion, error) {
	select {
	case <-ctx.Done():
		return ModelRewriteSuggestion{RiskLevel: modelRewriteRiskMedium}, ctx.Err()
	default:
	}

	tokens := tokenizeRewriteTerms(request.Query)
	if len(tokens) == 0 {
		return ModelRewriteSuggestion{RiskLevel: modelRewriteRiskMedium}, nil
	}

	collected := ModelRewriteSuggestion{RiskLevel: modelRewriteRiskMedium}
	for _, token := range tokens {
		select {
		case <-ctx.Done():
			return ModelRewriteSuggestion{RiskLevel: modelRewriteRiskMedium}, ctx.Err()
		default:
		}

		suggestion, ok := a.knowledge[token]
		if !ok {
			continue
		}
		collected.NormalizedTerms = append(collected.NormalizedTerms, suggestion.NormalizedTerms...)
		collected.Aliases = append(collected.Aliases, suggestion.Aliases...)
		collected.Abbreviations = append(collected.Abbreviations, suggestion.Abbreviations...)
		collected.MustKeepTerms = append(collected.MustKeepTerms, suggestion.MustKeepTerms...)
		collected.RiskLevel = suggestion.RiskLevel
	}
	return sanitizeModelSuggestion(collected), nil
}

func resolveDomainScopeKeys(request QueryRewriteRequest) []string {
	keys := make([]string, 0, 5)
	if request.KBID > 0 {
		keys = append(keys, "kb:"+uint64ToString(request.KBID))
	}
	if value := strings.TrimSpace(strings.ToLower(request.KBScope)); value != "" {
		keys = append(keys, "kb_scope:"+value)
	}
	if value := strings.TrimSpace(strings.ToLower(string(request.Language))); value != "" {
		keys = append(keys, "language:"+value)
	}
	if value := strings.TrimSpace(strings.ToLower(string(request.Category))); value != "" {
		keys = append(keys, "category:"+value)
	}
	keys = append(keys, "global")
	return uniqueStringsPreserveOrder(keys)
}

func matchesRewriteKey(lowerQuery string, tokens map[string]struct{}, key string) bool {
	key = normalizeRewriteTerm(key)
	if key == "" {
		return false
	}
	if _, ok := tokens[key]; ok {
		return true
	}
	return strings.Contains(lowerQuery, key)
}

func uint64ToString(value uint64) string {
	if value == 0 {
		return ""
	}
	buf := make([]byte, 0, 20)
	for value > 0 {
		buf = append(buf, byte('0'+value%10))
		value /= 10
	}
	for left, right := 0, len(buf)-1; left < right; left, right = left+1, right-1 {
		buf[left], buf[right] = buf[right], buf[left]
	}
	return string(buf)
}

func sortedKeys(values map[string][]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
