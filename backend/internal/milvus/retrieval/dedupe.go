package retrieval

import (
	"sort"

	"github.com/cloudwego/eino/schema"
)

// DeduplicateFusedDocuments 按 document_id + chunk_id 去重，并保留最高融合分与路由贡献。
func DeduplicateFusedDocuments(candidates []*FusedDocument) []*schema.Document {
	if len(candidates) == 0 {
		return []*schema.Document{}
	}

	type dedupedItem struct {
		key          string
		doc          *schema.Document
		score        float64
		primaryRoute string
		fusionStrategy string
		contrib      map[string]float64
		rawScores    map[string]float64
		routeRanks   map[string]int
		rrfContrib   map[string]float64
	}

	bestByKey := make(map[string]*dedupedItem, len(candidates))
	for _, candidate := range candidates {
		if candidate == nil || candidate.Doc == nil || candidate.Key == "" {
			continue
		}

		existing, ok := bestByKey[candidate.Key]
		if !ok {
			bestByKey[candidate.Key] = &dedupedItem{
				key:          candidate.Key,
				doc:          cloneDocumentWithMetadata(candidate.Doc),
				score:        candidate.Score,
				primaryRoute: candidate.PrimaryRoute,
				fusionStrategy: candidate.FusionStrategy,
				contrib:      copyFloatMap(candidate.RouteContrib),
				rawScores:    copyFloatMap(candidate.RouteRawScores),
				routeRanks:   copyIntMap(candidate.RouteRanks),
				rrfContrib:   copyFloatMap(candidate.RouteRRFContrib),
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
