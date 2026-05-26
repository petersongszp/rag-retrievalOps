package retrieval

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"
)

const defaultCitationCheckThreshold = 0.7

// CitationConsistencyConfig controls L5 citation verification.
type CitationConsistencyConfig struct {
	Enabled   bool
	Threshold float64
	Version   string
}

// CitationConsistencyOutcome captures query-to-citation support signals.
type CitationConsistencyOutcome struct {
	Supported         bool
	SupportScore      float64
	UnsupportedClaims []string
	Version           string
	Latency           time.Duration
	Error             string
}

// CitationConsistencyChecker validates whether retrieved chunks support query claims.
type CitationConsistencyChecker struct {
	config CitationConsistencyConfig
}

func NewCitationConsistencyChecker(config CitationConsistencyConfig) *CitationConsistencyChecker {
	if config.Threshold <= 0 || config.Threshold > 1 {
		config.Threshold = defaultCitationCheckThreshold
	}
	return &CitationConsistencyChecker{config: config}
}

func (c *CitationConsistencyChecker) Check(query string, docs []*schema.Document) CitationConsistencyOutcome {
	start := time.Now()
	if c == nil || !c.config.Enabled {
		return CitationConsistencyOutcome{}
	}
	if strings.TrimSpace(c.config.Version) == "" {
		return CitationConsistencyOutcome{
			Supported: true,
			Latency:   time.Since(start),
			Error:     ErrInvalidCitationConsistencyConfig.Error(),
		}
	}

	claims := extractCitationClaims(query)
	if len(claims) == 0 {
		trimmed := strings.TrimSpace(query)
		if trimmed != "" {
			claims = []string{trimmed}
		}
	}
	if len(claims) == 0 {
		return CitationConsistencyOutcome{
			Supported: true,
			Version:   c.config.Version,
			Latency:   time.Since(start),
		}
	}

	docScores := make([]float64, len(docs))
	unsupportedClaims := make([]string, 0, len(claims))
	totalScore := 0.0
	for _, claim := range claims {
		bestScore := 0.0
		for idx, doc := range docs {
			score := scoreCitationClaimAgainstDocument(claim, doc)
			if score > docScores[idx] {
				docScores[idx] = score
			}
			if score > bestScore {
				bestScore = score
			}
		}
		totalScore += bestScore
		if bestScore < c.config.Threshold {
			unsupportedClaims = append(unsupportedClaims, claim)
		}
	}

	supportScore := 0.0
	if len(claims) > 0 {
		supportScore = totalScore / float64(len(claims))
	}
	c.annotateDocuments(docs, docScores)

	return CitationConsistencyOutcome{
		Supported:         len(unsupportedClaims) == 0,
		SupportScore:      supportScore,
		UnsupportedClaims: unsupportedClaims,
		Version:           c.config.Version,
		Latency:           time.Since(start),
	}
}

func (c *CitationConsistencyChecker) annotateDocuments(docs []*schema.Document, docScores []float64) {
	for idx, doc := range docs {
		if doc == nil {
			continue
		}
		score := 0.0
		if idx < len(docScores) {
			score = docScores[idx]
		}
		supported := score >= c.config.Threshold

		if doc.MetaData == nil {
			doc.MetaData = make(map[string]interface{})
		}
		doc.MetaData["citation_supported"] = supported
		doc.MetaData["citation_support_score"] = score
		doc.MetaData["citation_check_version"] = c.config.Version
		doc.MetaData["low_support_citation"] = !supported

		source := ensureSourceMetadata(doc)
		source["citation_supported"] = supported
		source["citation_support_score"] = score
		source["citation_check_version"] = c.config.Version
		source["low_support_citation"] = !supported
		doc.MetaData["source"] = source
	}
}

func extractCitationClaims(query string) []string {
	normalized := strings.TrimSpace(query)
	if normalized == "" {
		return nil
	}

	parts := strings.FieldsFunc(normalized, func(r rune) bool {
		return r == '\n' || r == '\r' || unicode.IsPunct(r)
	})
	claims := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		for _, candidate := range splitCompositeClaim(part) {
			candidate = normalizeClaim(candidate)
			if !isMeaningfulClaim(candidate) {
				continue
			}
			key := strings.ToLower(candidate)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			claims = append(claims, candidate)
			if len(claims) >= 4 {
				return claims
			}
		}
	}
	if len(claims) == 0 {
		return []string{normalizeClaim(normalized)}
	}
	return claims
}

func splitCompositeClaim(claim string) []string {
	candidates := []string{claim}
	connectors := []string{" and ", " vs ", " versus ", " compare "}
	for _, connector := range connectors {
		next := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			if utf8.RuneCountInString(candidate) < 12 {
				next = append(next, candidate)
				continue
			}
			parts := strings.Split(candidate, connector)
			if len(parts) == 1 {
				next = append(next, candidate)
				continue
			}
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part != "" {
					next = append(next, part)
				}
			}
		}
		candidates = next
	}
	return candidates
}

func normalizeClaim(claim string) string {
	claim = strings.TrimSpace(claim)
	claim = strings.Trim(claim, `"'[](){}<>`)
	claim = strings.Join(strings.Fields(claim), " ")
	lower := strings.ToLower(claim)
	for _, prefix := range []string{
		"how does ", "how do ", "what is ", "what are ", "why does ", "why do ",
		"explain ", "describe ", "tell me ", "show me ",
	} {
		if strings.HasPrefix(lower, prefix) {
			claim = strings.TrimSpace(claim[len(prefix):])
			break
		}
	}
	return strings.TrimSpace(claim)
}

func isMeaningfulClaim(claim string) bool {
	if claim == "" {
		return false
	}
	tokens := meaningfulTokens(claim)
	if len(tokens) >= 2 {
		return true
	}
	return utf8.RuneCountInString(claim) >= 4
}

func scoreCitationClaimAgainstDocument(claim string, doc *schema.Document) float64 {
	if strings.TrimSpace(claim) == "" || doc == nil {
		return 0
	}
	content := strings.TrimSpace(doc.Content)
	if content == "" {
		return 0
	}

	normalizedClaim := normalizeForComparison(claim)
	normalizedContent := normalizeForComparison(content)
	if normalizedClaim == "" || normalizedContent == "" {
		return 0
	}
	if strings.Contains(normalizedContent, normalizedClaim) {
		return 0.98
	}

	claimTokens := meaningfulTokens(claim)
	contentTokens := meaningfulTokens(content)
	lexicalCoverage := tokenCoverage(claimTokens, contentTokens)

	claimEntities := extractEntityTerms(claim)
	contentEntities := extractEntityTerms(content)
	entityCoverage := tokenCoverage(claimEntities, contentEntities)

	charCoverage := characterNGramCoverage(normalizedClaim, normalizedContent, 3)
	score := (lexicalCoverage * 0.5) + (entityCoverage * 0.2) + (charCoverage * 0.3)

	switch {
	case lexicalCoverage >= 1 && len(claimTokens) > 0:
		score = maxCitationFloat64(score, 0.9)
	case lexicalCoverage >= 0.8 && entityCoverage >= 0.8:
		score = maxCitationFloat64(score, 0.85)
	case entityCoverage >= 1 && len(claimEntities) > 0:
		score = maxCitationFloat64(score, 0.82)
	}

	if score > 1 {
		return 1
	}
	return score
}

func meaningfulTokens(text string) []string {
	rawTokens := splitIntoTerms(text)
	if len(rawTokens) == 0 {
		return nil
	}
	result := make([]string, 0, len(rawTokens))
	seen := make(map[string]struct{}, len(rawTokens))
	for _, token := range rawTokens {
		token = strings.ToLower(strings.TrimSpace(token))
		if token == "" || isStopToken(token) {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		result = append(result, token)
	}
	return result
}

func extractEntityTerms(text string) []string {
	terms := splitIntoTerms(text)
	result := make([]string, 0, len(terms))
	seen := make(map[string]struct{}, len(terms))
	for _, term := range terms {
		normalized := strings.ToLower(strings.TrimSpace(term))
		if normalized == "" {
			continue
		}
		if !looksLikeEntityTerm(term) {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	sort.Strings(result)
	return result
}

func splitIntoTerms(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return strings.FieldsFunc(text, func(r rune) bool {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
		switch r {
		case '_', '-', '.', '/', '#', '+':
			return false
		default:
			return true
		}
	})
}

func looksLikeEntityTerm(term string) bool {
	runeCount := utf8.RuneCountInString(term)
	if runeCount >= 2 && containsDigit(term) {
		return true
	}
	if runeCount >= 3 && strings.ContainsAny(term, "_-/#+.") {
		return true
	}
	if runeCount >= 3 && hasUppercase(term) {
		return true
	}
	if runeCount >= 4 {
		return true
	}
	return false
}

func tokenCoverage(claimTokens, contentTokens []string) float64 {
	if len(claimTokens) == 0 {
		return 0
	}
	contentSet := make(map[string]struct{}, len(contentTokens))
	for _, token := range contentTokens {
		contentSet[token] = struct{}{}
	}
	matched := 0
	for _, token := range claimTokens {
		if _, ok := contentSet[token]; ok {
			matched++
		}
	}
	return float64(matched) / float64(len(claimTokens))
}

func characterNGramCoverage(claim, content string, n int) float64 {
	claimGrams := buildCharacterNGrams(claim, n)
	if len(claimGrams) == 0 {
		return 0
	}
	contentGrams := buildCharacterNGrams(content, n)
	if len(contentGrams) == 0 {
		return 0
	}
	matched := 0
	for gram := range claimGrams {
		if _, ok := contentGrams[gram]; ok {
			matched++
		}
	}
	return float64(matched) / float64(len(claimGrams))
}

func buildCharacterNGrams(text string, n int) map[string]struct{} {
	if n <= 0 {
		return nil
	}
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}
	if len(runes) < n {
		return map[string]struct{}{string(runes): struct{}{}}
	}
	grams := make(map[string]struct{}, len(runes)-n+1)
	for i := 0; i <= len(runes)-n; i++ {
		grams[string(runes[i:i+n])] = struct{}{}
	}
	return grams
}

func normalizeForComparison(text string) string {
	var builder strings.Builder
	builder.Grow(len(text))
	for _, r := range strings.ToLower(text) {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			continue
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

func containsDigit(text string) bool {
	for _, r := range text {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func hasUppercase(text string) bool {
	for _, r := range text {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

func isStopToken(token string) bool {
	switch token {
	case "", "the", "a", "an", "is", "are", "was", "were", "what", "how", "why", "when", "where", "which", "who",
		"to", "of", "for", "in", "on", "at", "by", "with", "about", "and", "or", "vs", "versus":
		return true
	default:
		return false
	}
}

func maxCitationFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

var ErrInvalidCitationConsistencyConfig = fmt.Errorf("invalid citation consistency config")
