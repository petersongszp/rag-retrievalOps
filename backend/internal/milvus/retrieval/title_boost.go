package retrieval

import (
	"sort"
	"strings"
	"unicode"

	"github.com/cloudwego/eino/schema"
)

const titleMatchBoostVersion = "title-match-boost-v1"

func applyTitleMatchBoost(query string, docs []*schema.Document) []*schema.Document {
	if len(docs) <= 1 || !isShortPreciseQuery(query) {
		return docs
	}
	normalizedQuery := normalizeTitleMatchText(query)
	if len([]rune(normalizedQuery)) < 3 {
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
