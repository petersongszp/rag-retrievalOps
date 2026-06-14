package retrieval

import (
	"sort"
	"strings"
	"unicode"

	"github.com/cloudwego/eino/schema"
)

const titleMatchBoostVersion = "title-match-boost-v1"

func applyTitleMatchBoost(query string, docs []*schema.Document) []*schema.Document {
	if len(docs) <= 1 {
		return docs
	}
	normalizedQuery := normalizeTitleMatchText(query)
	if len([]rune(normalizedQuery)) < 3 {
		return docs
	}
	if !isShortPreciseQuery(query) && !isLimitBehaviorIntent(normalizedQuery) && !isBillingFormulaIntent(normalizedQuery) {
		return docs
	}

	for _, doc := range docs {
		boost, reason := computeTitleMatchBoost(normalizedQuery, doc)
		if boost <= 0 {
			continue
		}
		applyTitleBoostMetadata(doc, boost, reason)
	}

	sort.SliceStable(docs, func(i, j int) bool {
		left := readDocScore(docs[i])
		right := readDocScore(docs[j])
		if nearlyEqual(left, right) {
			return buildDedupeKey(docs[i]) < buildDedupeKey(docs[j])
		}
		return left > right
	})
	return docs
}

func computeTitleMatchBoost(normalizedQuery string, doc *schema.Document) (float64, string) {
	if doc == nil || doc.MetaData == nil {
		return 0, ""
	}

	sectionTitle := normalizeTitleMatchText(readMetadataString(doc, "section_title"))
	hierarchyPath := normalizeTitleMatchText(readMetadataString(doc, "hierarchy_path"))

	switch {
	case isLimitBehaviorIntent(normalizedQuery) && matchesLimitBehaviorSection(sectionTitle, hierarchyPath):
		return 0.35, "limit_behavior_intent"
	case isBillingFormulaIntent(normalizedQuery) && matchesBillingFormulaEvidence(sectionTitle, hierarchyPath, doc.Content):
		return 0.35, "billing_formula_intent"
	case sectionTitle != "" && sectionTitle == normalizedQuery:
		return 0.40, "section_title_exact"
	case hierarchyPath != "" && hierarchyPath == normalizedQuery:
		return 0.40, "hierarchy_path_exact"
	case hierarchyPath != "" && strings.Contains(hierarchyPath, normalizedQuery):
		return 0.35, "hierarchy_path_contains"
	case sectionTitle != "" && strings.Contains(sectionTitle, normalizedQuery):
		return 0.30, "section_title_contains"
	default:
		return 0, ""
	}
}

func isLimitBehaviorIntent(normalizedQuery string) bool {
	if normalizedQuery == "" {
		return false
	}
	if !strings.Contains(normalizedQuery, "超过") {
		return false
	}
	if !containsAny(normalizedQuery, "tps", "规格", "上限", "限制") {
		return false
	}
	return containsAny(normalizedQuery, "怎样", "怎么", "如何", "后", "行为", "处理", "限流")
}

func isBillingFormulaIntent(normalizedQuery string) bool {
	if normalizedQuery == "" {
		return false
	}
	if !containsAny(normalizedQuery, "费用", "计费", "价格", "收费") {
		return false
	}
	if !containsAny(normalizedQuery, "计算", "怎么算", "怎么计算", "如何计算", "公式", "规则") {
		return false
	}
	return containsAny(normalizedQuery, "规格", "实例", "rocketmq", "mq", "消息")
}

func matchesBillingFormulaEvidence(sectionTitle, hierarchyPath, content string) bool {
	return isBillingFormulaEvidenceText(sectionTitle) ||
		isBillingFormulaEvidenceText(hierarchyPath) ||
		isBillingFormulaEvidenceText(normalizeTitleMatchText(content))
}

func isBillingFormulaEvidenceText(text string) bool {
	if text == "" {
		return false
	}
	if containsAny(text, "计费公式", "费用公式") {
		return true
	}
	if strings.Contains(text, "计费规则") {
		return true
	}
	return containsAny(text, "费用", "计费") && strings.Contains(text, "公式")
}

func matchesLimitBehaviorSection(sectionTitle, hierarchyPath string) bool {
	return isLimitBehaviorSectionText(sectionTitle) || isLimitBehaviorSectionText(hierarchyPath)
}

func isLimitBehaviorSectionText(text string) bool {
	if text == "" {
		return false
	}
	return strings.Contains(text, "超过") &&
		containsAny(text, "规格", "上限", "限制") &&
		containsAny(text, "行为", "后", "限流", "处理")
}

func containsAny(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}

func applyTitleBoostMetadata(doc *schema.Document, boost float64, reason string) {
	if doc == nil {
		return
	}
	if doc.MetaData == nil {
		doc.MetaData = make(map[string]interface{})
	}

	originalScore := readDocScore(doc)
	boostedScore := originalScore + boost
	if boostedScore > 1 {
		boostedScore = 1
	}
	doc.MetaData["pre_title_match_score"] = originalScore
	doc.MetaData["title_match_boost"] = boost
	doc.MetaData["title_match_boost_applied"] = true
	doc.MetaData["title_match_boost_reason"] = reason
	doc.MetaData["title_match_boost_version"] = titleMatchBoostVersion
	doc.MetaData["score"] = boostedScore

	source := ensureSourceMetadata(doc)
	source["pre_title_match_score"] = originalScore
	source["title_match_boost"] = boost
	source["title_match_boost_applied"] = true
	source["title_match_boost_reason"] = reason
	source["title_match_boost_version"] = titleMatchBoostVersion
	doc.MetaData["source"] = source
}

func normalizeTitleMatchText(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" {
		return ""
	}
	var builder strings.Builder
	for _, r := range input {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}
		builder.WriteRune(r)
	}
	return builder.String()
}
