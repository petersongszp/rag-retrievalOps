package retrieval

import (
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"
)

// DeduplicateFusedDocuments 按 document_id + chunk_id 去重，并保留最高融合分与路由贡献。
func DeduplicateFusedDocuments(candidates []*FusedDocument) []*schema.Document {
	if len(candidates) == 0 {
		return []*schema.Document{}
	}

	type dedupedItem struct {
		key            string
		doc            *schema.Document
		score          float64
		primaryRoute   string
		fusionStrategy string
		contrib        map[string]float64
		rawScores      map[string]float64
		routeRanks     map[string]int
		rrfContrib     map[string]float64
	}

	bestByKey := make(map[string]*dedupedItem, len(candidates))
	for _, candidate := range candidates {
		if candidate == nil || candidate.Doc == nil || candidate.Key == "" {
			continue
		}

		existing, ok := bestByKey[candidate.Key]
		if !ok {
			bestByKey[candidate.Key] = &dedupedItem{
				key:            candidate.Key,
				doc:            cloneDocumentWithMetadata(candidate.Doc),
				score:          candidate.Score,
				primaryRoute:   candidate.PrimaryRoute,
				fusionStrategy: candidate.FusionStrategy,
				contrib:        copyFloatMap(candidate.RouteContrib),
				rawScores:      copyFloatMap(candidate.RouteRawScores),
				routeRanks:     copyIntMap(candidate.RouteRanks),
				rrfContrib:     copyFloatMap(candidate.RouteRRFContrib),
			}
			continue
		}

		mergeFloatMaps(existing.contrib, candidate.RouteContrib)
		mergeFloatMaps(existing.rawScores, candidate.RouteRawScores)
		mergeIntMaps(existing.routeRanks, candidate.RouteRanks)
		mergeFloatMaps(existing.rrfContrib, candidate.RouteRRFContrib)

		if candidate.Score > existing.score || (nearlyEqual(candidate.Score, existing.score) && candidate.Key < existing.key) {
			existing.doc = cloneDocumentWithMetadata(candidate.Doc)
			existing.score = candidate.Score
			existing.primaryRoute = candidate.PrimaryRoute
			existing.fusionStrategy = candidate.FusionStrategy
		}
	}

	out := make([]*schema.Document, 0, len(bestByKey))
	for _, item := range bestByKey {
		annotateDedupedDocument(item.doc, item.score, item.primaryRoute, item.fusionStrategy, item.contrib, item.rawScores, item.routeRanks, item.rrfContrib)
		out = append(out, item.doc)
	}
	out = removeContainedSectionDuplicates(out)

	sort.SliceStable(out, func(i, j int) bool {
		left := readDocScore(out[i])
		right := readDocScore(out[j])
		if nearlyEqual(left, right) {
			return buildDedupeKey(out[i]) < buildDedupeKey(out[j])
		}
		return left > right
	})
	return out
}

func removeContainedSectionDuplicates(docs []*schema.Document) []*schema.Document {
	if len(docs) <= 1 {
		return docs
	}

	dropKeys := make(map[string]struct{})
	for _, tableDoc := range docs {
		if !isTableRetrievalChunk(tableDoc) {
			continue
		}
		tableDocumentID := readMetadataString(tableDoc, "document_id")
		if tableDocumentID == "" {
			continue
		}
		tableStart, tableEnd, hasTableSpan := readChildSpan(tableDoc)

		for _, candidate := range docs {
			if candidate == nil || candidate == tableDoc || isTableRetrievalChunk(candidate) {
				continue
			}
			if readMetadataString(candidate, "document_id") != tableDocumentID {
				continue
			}
			if shouldDropSectionDuplicateForTable(tableDoc, candidate, tableStart, tableEnd, hasTableSpan) {
				dropKeys[buildDedupeKey(candidate)] = struct{}{}
			}
		}
	}
	if len(dropKeys) == 0 {
		return docs
	}

	out := make([]*schema.Document, 0, len(docs)-len(dropKeys))
	for _, doc := range docs {
		if _, drop := dropKeys[buildDedupeKey(doc)]; drop {
			continue
		}
		out = append(out, doc)
	}
	return out
}

func shouldDropSectionDuplicateForTable(tableDoc, candidate *schema.Document, tableStart, tableEnd int, hasTableSpan bool) bool {
	if hasTableSpan {
		candidateStart, candidateEnd, ok := readChildSpan(candidate)
		if ok && candidateStart <= tableStart && tableEnd <= candidateEnd && candidateEnd-candidateStart > tableEnd-tableStart {
			return true
		}
	}
	return hasTableContentOverlap(tableDoc.Content, candidate.Content)
}

func isTableRetrievalChunk(doc *schema.Document) bool {
	if doc == nil {
		return false
	}
	unit := strings.TrimSpace(strings.ToLower(readMetadataString(doc, "chunking_unit")))
	if unit == "table" {
		return true
	}
	strategy := strings.TrimSpace(strings.ToLower(readMetadataString(doc, "chunking_strategy")))
	if strategy == "table-aware" && unit == "table" {
		return true
	}
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(doc.Content)), "table table-")
}

func hasTableContentOverlap(tableContent, candidateContent string) bool {
	tokens := extractTableDedupeTokens(tableContent)
	if len(tokens) < 4 {
		return false
	}

	candidateNormalized := normalizeTableDedupeText(candidateContent)
	if candidateNormalized == "" {
		return false
	}

	matched := 0
	longMatched := 0
	for _, token := range tokens {
		normalizedToken := normalizeTableDedupeText(token)
		if normalizedToken == "" {
			continue
		}
		if strings.Contains(candidateNormalized, normalizedToken) {
			matched++
			if utf8.RuneCountInString(normalizedToken) >= 8 {
				longMatched++
			}
		}
	}
	return matched >= 4 && longMatched >= 1
}

func extractTableDedupeTokens(content string) []string {
	parts := strings.FieldsFunc(content, func(r rune) bool {
		return r == '|' || r == '\n' || r == '\r' || r == '\t'
	})
	if len(parts) < 4 {
		return nil
	}

	tokens := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		token := strings.TrimSpace(strings.Trim(part, "-"))
		normalized := normalizeTableDedupeText(token)
		if utf8.RuneCountInString(normalized) < 2 {
			continue
		}
		if strings.HasPrefix(normalized, "tabletable") {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		tokens = append(tokens, token)
	}
	return tokens
}

func normalizeTableDedupeText(text string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(text) {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

func readChildSpan(doc *schema.Document) (int, int, bool) {
	start, okStart := readIntLikeMetadata(doc, "child_start_offset")
	end, okEnd := readIntLikeMetadata(doc, "child_end_offset")
	return start, end, okStart && okEnd && end > start
}

func readIntLikeMetadata(doc *schema.Document, key string) (int, bool) {
	if doc == nil || doc.MetaData == nil {
		return 0, false
	}
	value, ok := doc.MetaData[key]
	if !ok || value == nil {
		return 0, false
	}
	switch v := value.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(v))
		return parsed, err == nil
	default:
		parsed, err := strconv.Atoi(strings.TrimSpace(readMetadataString(doc, key)))
		return parsed, err == nil
	}
}

func annotateDedupedDocument(doc *schema.Document, score float64, primaryRoute, fusionStrategy string, contrib, rawScores map[string]float64, routeRanks map[string]int, rrfContrib map[string]float64) {
	if doc == nil {
		return
	}
	if doc.MetaData == nil {
		doc.MetaData = make(map[string]interface{})
	}

	doc.MetaData["score"] = score
	doc.MetaData["fusion_score"] = score
	doc.MetaData["route"] = primaryRoute
	doc.MetaData["primary_route"] = primaryRoute
	doc.MetaData["fusion_strategy"] = fusionStrategy
	doc.MetaData["route_contrib"] = floatMapToInterfaceMap(contrib)
	doc.MetaData["route_raw_scores"] = floatMapToInterfaceMap(rawScores)
	doc.MetaData["route_rank"] = intMapToInterfaceMap(routeRanks)
	doc.MetaData["route_rrf_contrib"] = floatMapToInterfaceMap(rrfContrib)

	source := make(map[string]interface{})
	if existing, ok := doc.MetaData["source"].(map[string]interface{}); ok && existing != nil {
		for key, value := range existing {
			source[key] = value
		}
	}
	source["route"] = primaryRoute
	source["primary_route"] = primaryRoute
	source["fusion_strategy"] = fusionStrategy
	source["route_contrib"] = floatMapToInterfaceMap(contrib)
	source["route_rank"] = intMapToInterfaceMap(routeRanks)
	source["route_rrf_contrib"] = floatMapToInterfaceMap(rrfContrib)
	if collection := readCollectionFromDoc(doc); collection != "" {
		source["collection"] = collection
	}
	if version := readMetadataString(doc, "retriever_version"); version != "" {
		source["retriever_version"] = version
	} else if _, exists := source["retriever_version"]; !exists {
		source["retriever_version"] = hybridRetrieverVersion
	}
	doc.MetaData["source"] = source
	annotateParentChildSource(doc)
}

func copyFloatMap(input map[string]float64) map[string]float64 {
	if len(input) == 0 {
		return map[string]float64{}
	}
	out := make(map[string]float64, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func mergeFloatMaps(target, input map[string]float64) {
	if target == nil || len(input) == 0 {
		return
	}
	for key, value := range input {
		if value > target[key] {
			target[key] = value
		}
	}
}

func copyIntMap(input map[string]int) map[string]int {
	if len(input) == 0 {
		return map[string]int{}
	}
	out := make(map[string]int, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func mergeIntMaps(target, input map[string]int) {
	if target == nil || len(input) == 0 {
		return
	}
	for key, value := range input {
		if current, ok := target[key]; !ok || value < current {
			target[key] = value
		}
	}
}

func floatMapToInterfaceMap(input map[string]float64) map[string]interface{} {
	if len(input) == 0 {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func intMapToInterfaceMap(input map[string]int) map[string]interface{} {
	if len(input) == 0 {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
