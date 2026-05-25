package retrieval

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
	milvusClient "github.com/milvus-io/milvus-sdk-go/v2/client"
)

// SparseRetrieverConfig controls sparse route behavior.
type SparseRetrieverConfig struct {
	DefaultTopK   int
	MaxTerms      int
	PerTermFactor int
	MinPerTermK   int
}

// SparseRetriever provides keyword recall and ranks candidates with an explicit inverted BM25 index.
type SparseRetriever struct {
	client     milvusClient.Client
	collection string
	config     SparseRetrieverConfig
}

func NewSparseRetriever(client milvusClient.Client, collection string, cfg *SparseRetrieverConfig) (*SparseRetriever, error) {
	if client == nil {
		return nil, fmt.Errorf("milvus client is nil")
	}
	if strings.TrimSpace(collection) == "" {
		return nil, fmt.Errorf("collection is empty")
	}

	out := SparseRetrieverConfig{
		DefaultTopK:   10,
		MaxTerms:      6,
		PerTermFactor: 4,
		MinPerTermK:   20,
	}
	if cfg != nil {
		if cfg.DefaultTopK > 0 {
			out.DefaultTopK = cfg.DefaultTopK
		}
		if cfg.MaxTerms > 0 {
			out.MaxTerms = cfg.MaxTerms
		}
		if cfg.PerTermFactor > 0 {
			out.PerTermFactor = cfg.PerTermFactor
		}
		if cfg.MinPerTermK > 0 {
			out.MinPerTermK = cfg.MinPerTermK
		}
	}

	return &SparseRetriever{
		client:     client,
		collection: collection,
		config:     out,
	}, nil
}

func (s *SparseRetriever) Search(ctx context.Context, req *HybridSearchRequest) ([]*schema.Document, error) {
	if req == nil {
		return nil, fmt.Errorf("hybrid search request is nil")
	}
	query := strings.TrimSpace(req.FinalQuery)
	if query == "" {
		query = strings.TrimSpace(req.Query)
	}
	if query == "" {
		return nil, fmt.Errorf("query is empty")
	}

	topK := req.CandidateTopK
	if topK <= 0 {
		topK = s.config.DefaultTopK
	}
	terms := extractSparseTerms(query, s.config.MaxTerms)
	if len(terms) == 0 {
		return []*schema.Document{}, nil
	}

	baseExpr := strings.TrimSpace(req.Expr)
	if baseExpr == "" && (strings.TrimSpace(req.KBScope) != "" || req.KBID > 0) {
		baseExpr = BuildFilterExpr(&RetrieveOptions{
			KBScope:          req.KBScope,
			ActiveGlobalKBID: req.KBID,
		})
	}

	collection := strings.TrimSpace(req.Collection)
	if collection == "" {
		collection = s.collection
	}

	perTermLimit := topK * s.config.PerTermFactor
	if perTermLimit < s.config.MinPerTermK {
		perTermLimit = s.config.MinPerTermK
	}

	merged := make(map[string]*schema.Document, perTermLimit)
	for _, term := range terms {
		likeExpr := fmt.Sprintf("content like \"%%%s%%\"", escapeLikeValue(term))
		expr := likeExpr
		if baseExpr != "" {
			expr = fmt.Sprintf("(%s) && (%s)", baseExpr, likeExpr)
		}

		resultSet, err := s.client.Query(
			ctx,
			collection,
			nil,
			expr,
			[]string{"id", "content", "metadata"},
			milvusClient.WithLimit(int64(perTermLimit)),
		)
		if err != nil {
			return nil, fmt.Errorf("sparse query failed, term=%q expr=%q: %w", term, expr, err)
		}

		for _, doc := range parseQueryResultSet(resultSet) {
			docID := strings.TrimSpace(doc.ID)
			if docID == "" {
				docID = buildPseudoDocID(doc)
			}
			if docID == "" {
				continue
			}

			if _, exists := merged[docID]; exists {
				continue
			}
			if doc.MetaData == nil {
				doc.MetaData = make(map[string]interface{})
			}
			doc.MetaData["route"] = "sparse"
			merged[docID] = doc
		}
	}

	if len(merged) == 0 {
		return []*schema.Document{}, nil
	}

	candidates := make([]*schema.Document, 0, len(merged))
	for _, doc := range merged {
		candidates = append(candidates, doc)
	}
	index := BuildSparseInvertedIndex(candidates, nil)
	hits := index.Search(terms, topK)
	if len(hits) == 0 {
		return []*schema.Document{}, nil
	}

	results := make([]*schema.Document, 0, len(hits))
	for _, hit := range hits {
		doc := hit.Document
		if doc == nil {
			continue
		}
		if doc.MetaData == nil {
			doc.MetaData = make(map[string]interface{})
		}
		doc.MetaData["route"] = routeSparse
		doc.MetaData["sparse_score"] = hit.Score
		doc.MetaData["score"] = hit.Score
		attachRewriteMetadata(doc, req)
		results = append(results, doc)
	}

	return results, nil
}

func parseQueryResultSet(rs milvusClient.ResultSet) []*schema.Document {
	out := make([]*schema.Document, 0)
	if rs.Len() == 0 {
		return out
	}

	idCol := rs.GetColumn("id")
	contentCol := rs.GetColumn("content")
	metaCol := rs.GetColumn("metadata")
	if idCol == nil || contentCol == nil {
		return out
	}

	count := idCol.Len()
	for i := 0; i < count; i++ {
		id, err := idCol.GetAsString(i)
		if err != nil {
			continue
		}
		content, err := contentCol.GetAsString(i)
		if err != nil {
			continue
		}

		doc := &schema.Document{
			ID:       id,
			Content:  content,
			MetaData: map[string]interface{}{},
		}

		if metaCol != nil {
			if metaRaw, err := metaCol.Get(i); err == nil {
				switch v := metaRaw.(type) {
				case string:
					if strings.TrimSpace(v) != "" {
						var metaMap map[string]interface{}
						if err := sonic.Unmarshal([]byte(v), &metaMap); err == nil {
							doc.MetaData = metaMap
						}
					}
				case map[string]interface{}:
					doc.MetaData = v
				}
			}
		}

		out = append(out, doc)
	}

	return out
}

func buildPseudoDocID(doc *schema.Document) string {
	if doc == nil {
		return ""
	}
	if doc.MetaData != nil {
		documentID := strings.TrimSpace(fmt.Sprint(doc.MetaData["document_id"]))
		chunkID := strings.TrimSpace(fmt.Sprint(doc.MetaData["chunk_id"]))
		if documentID != "" && chunkID != "" {
			return documentID + ":" + chunkID
		}
		if documentID != "" {
			return documentID
		}
	}
	return strings.TrimSpace(doc.Content)
}

func extractSparseTerms(query string, maxTerms int) []string {
	if maxTerms <= 0 {
		maxTerms = 6
	}
	parts := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return false
		}
		return true
	})

	stopWords := map[string]struct{}{
		"the": {}, "a": {}, "an": {}, "to": {}, "of": {}, "in": {}, "on": {}, "for": {}, "is": {}, "are": {},
		"and": {}, "or": {}, "with": {}, "what": {}, "how": {}, "why": {}, "when": {}, "where": {},
	}
	terms := make([]string, 0, maxTerms)
	seen := make(map[string]struct{}, maxTerms)
	for _, part := range parts {
		term := strings.TrimSpace(part)
		if term == "" || len([]rune(term)) < 2 {
			continue
		}
		if _, blocked := stopWords[term]; blocked {
			continue
		}
		if _, ok := seen[term]; ok {
			continue
		}
		seen[term] = struct{}{}
		terms = append(terms, term)
		if len(terms) >= maxTerms {
			break
		}
	}
	return terms
}

func escapeLikeValue(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"%", "\\%",
		"_", "\\_",
	)
	return replacer.Replace(value)
}

func tokenizeWithFreq(text string) map[string]int {
	out := make(map[string]int)
	if strings.TrimSpace(text) == "" {
		return out
	}
	parts := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return false
		}
		return true
	})
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		out[p]++
	}
	return out
}
