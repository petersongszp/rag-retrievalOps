package retrieval

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/cloudwego/eino/schema"
	milvusClient "github.com/milvus-io/milvus-sdk-go/v2/client"
)

// SparseRetrieverConfig controls sparse route behavior.
type SparseRetrieverConfig struct {
	DefaultTopK   int
	MaxTerms      int
	PerTermFactor int
	MinPerTermK   int
	ProviderName  string
}

// SparseRetriever provides keyword recall and ranks candidates with an explicit inverted BM25 index.
type SparseRetriever struct {
	client            milvusClient.Client
	collection        string
	config            SparseRetrieverConfig
	candidateProvider SparseCandidateProvider
	ranker            SparseRanker
}

type SparseSearchStats struct {
	ProviderName         string
	TermSources          map[string]string
	DroppedTerms         map[string]string
	Terms                 []string
	PerTermCandidateCount map[string]int
	CandidateCountBefore  int
	CandidateCountAfter   int
}

type SparseTerm struct {
	Value  string
	Kind   string
	Source string
}

type SparseCandidateProviderStats struct {
	ProviderName        string
	ProviderVersion     string
	TermCount           int
	PerTermLimit        int
	RawCandidateCount   int
	DedupCandidateCount int
	LatencyMS           int64
	FallbackReason      string
	PerTermCandidateCount map[string]int
}

type SparseCandidateProvider interface {
	SearchCandidates(ctx context.Context, req *HybridSearchRequest, terms []string) ([]*schema.Document, SparseCandidateProviderStats, error)
}

type SparseRankStats struct {
	RankerName          string
	CandidateCountBefore int
	CandidateCountAfter  int
}

type SparseRanker interface {
	Rank(ctx context.Context, query string, terms []string, candidates []*schema.Document, topK int) ([]SparseSearchHit, SparseRankStats, error)
}

type MilvusLikeCandidateProvider struct {
	client     milvusClient.Client
	collection string
	config     SparseRetrieverConfig
}

type CandidateBM25Ranker struct{}

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
		client:            client,
		collection:        collection,
		config:            out,
		candidateProvider: &MilvusLikeCandidateProvider{client: client, collection: collection, config: out},
		ranker:            &CandidateBM25Ranker{},
	}, nil
}

func (s *SparseRetriever) Search(ctx context.Context, req *HybridSearchRequest) ([]*schema.Document, SparseSearchStats, error) {
	if req == nil {
		return nil, SparseSearchStats{}, fmt.Errorf("hybrid search request is nil")
	}
	query := strings.TrimSpace(req.SparseQuery)
	if query == "" {
		query = strings.TrimSpace(req.FinalQuery)
	}
	if query == "" {
		query = strings.TrimSpace(req.Query)
	}
	if query == "" {
		return nil, SparseSearchStats{}, fmt.Errorf("query is empty")
	}

	topK := req.CandidateTopK
	if topK <= 0 {
		topK = s.config.DefaultTopK
	}
	termLimit := s.config.MaxTerms
	if strings.TrimSpace(req.SparseQuery) != "" && !strings.EqualFold(strings.TrimSpace(req.SparseQuery), strings.TrimSpace(req.OriginalQuery)) && termLimit < 10 {
		termLimit = 10
	}
	extractedTerms, droppedTerms := extractSparseTermsDetailed(query, termLimit)
	terms := flattenSparseTerms(extractedTerms)
	if len(terms) == 0 {
		return []*schema.Document{}, SparseSearchStats{}, nil
	}
	stats := SparseSearchStats{
		Terms:                 append([]string(nil), terms...),
		TermSources:           make(map[string]string, len(extractedTerms)),
		DroppedTerms:          droppedTerms,
		PerTermCandidateCount: make(map[string]int, len(terms)),
	}
	for _, term := range extractedTerms {
		stats.TermSources[term.Value] = term.Source
	}

	candidates, providerStats, err := s.candidateProvider.SearchCandidates(ctx, req, terms)
	if err != nil {
		return nil, stats, err
	}
	stats.ProviderName = providerStats.ProviderName
	for key, value := range providerStats.PerTermCandidateCount {
		stats.PerTermCandidateCount[key] = value
	}
	if len(candidates) == 0 {
		return []*schema.Document{}, stats, nil
	}
	stats.CandidateCountBefore = providerStats.DedupCandidateCount

	hits, rankStats, err := s.ranker.Rank(ctx, query, terms, candidates, topK)
	if err != nil {
		return nil, stats, err
	}
	if len(hits) == 0 {
		return []*schema.Document{}, stats, nil
	}
	stats.CandidateCountAfter = rankStats.CandidateCountAfter

	results := make([]*schema.Document, 0, len(hits))
	collection := strings.TrimSpace(req.Collection)
	if collection == "" {
		collection = s.collection
	}
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
		doc.MetaData["retriever_version"] = hybridRetrieverVersion
		if collection != "" {
			doc.MetaData["collection"] = collection
		}
		source := ensureSourceMetadata(doc)
		source["route"] = routeSparse
		source["retriever_version"] = hybridRetrieverVersion
		if collection != "" {
			source["collection"] = collection
		}
		doc.MetaData["source"] = source
		annotateParentChildSource(doc)
		attachRewriteMetadata(doc, req, routeSparse)
		results = append(results, doc)
	}

	return results, stats, nil
}

func (p *MilvusLikeCandidateProvider) SearchCandidates(ctx context.Context, req *HybridSearchRequest, terms []string) ([]*schema.Document, SparseCandidateProviderStats, error) {
	stats := SparseCandidateProviderStats{
		ProviderName:         "milvus_like",
		ProviderVersion:      "v1",
		TermCount:            len(terms),
		PerTermCandidateCount: make(map[string]int, len(terms)),
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
		collection = p.collection
	}
	perTermLimit := req.CandidateTopK * p.config.PerTermFactor
	if perTermLimit < p.config.MinPerTermK {
		perTermLimit = p.config.MinPerTermK
	}
	stats.PerTermLimit = perTermLimit
	start := time.Now()
	merged := make(map[string]*schema.Document, perTermLimit)
	for _, term := range terms {
		likeExpr := fmt.Sprintf("content like \"%%%s%%\"", escapeLikeValue(term))
		expr := likeExpr
		if baseExpr != "" {
			expr = fmt.Sprintf("(%s) && (%s)", baseExpr, likeExpr)
		}
		resultSet, err := p.client.Query(
			ctx,
			collection,
			nil,
			expr,
			[]string{"id", "content", "metadata"},
			milvusClient.WithLimit(int64(perTermLimit)),
		)
		if err != nil {
			return nil, stats, fmt.Errorf("sparse query failed, term=%q expr=%q: %w", term, expr, err)
		}
		stats.PerTermCandidateCount[term] = resultSet.Len()
		stats.RawCandidateCount += resultSet.Len()
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
			doc.MetaData["route"] = routeSparse
			doc.MetaData["retriever_version"] = hybridRetrieverVersion
			if collection != "" {
				doc.MetaData["collection"] = collection
			}
			source := ensureSourceMetadata(doc)
			source["route"] = routeSparse
			source["retriever_version"] = hybridRetrieverVersion
			if collection != "" {
				source["collection"] = collection
			}
			doc.MetaData["source"] = source
			annotateParentChildSource(doc)
			merged[docID] = doc
		}
	}
	stats.LatencyMS = time.Since(start).Milliseconds()
	stats.DedupCandidateCount = len(merged)
	out := make([]*schema.Document, 0, len(merged))
	for _, doc := range merged {
		out = append(out, doc)
	}
	return out, stats, nil
}

func (r *CandidateBM25Ranker) Rank(ctx context.Context, query string, terms []string, candidates []*schema.Document, topK int) ([]SparseSearchHit, SparseRankStats, error) {
	index := BuildSparseInvertedIndex(candidates, nil)
	hits := index.Search(terms, topK)
	return hits, SparseRankStats{
		RankerName:           "candidate_bm25",
		CandidateCountBefore: len(candidates),
		CandidateCountAfter:  len(hits),
	}, nil
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
				doc.MetaData = parseMilvusMetadata(metaRaw)
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
	terms, _ := extractSparseTermsDetailed(query, maxTerms)
	return flattenSparseTerms(terms)
}

func extractSparseTermsDetailed(query string, maxTerms int) ([]SparseTerm, map[string]string) {
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
	terms := make([]SparseTerm, 0, maxTerms)
	dropped := make(map[string]string)
	seen := make(map[string]struct{}, maxTerms)
	for _, part := range parts {
		term := strings.TrimSpace(part)
		if term == "" || len([]rune(term)) < 2 {
			if term != "" {
				dropped[term] = "too_short"
			}
			continue
		}
		if _, blocked := stopWords[term]; blocked {
			dropped[term] = "stopword"
			continue
		}
		if _, ok := seen[term]; ok {
			dropped[term] = "duplicate"
			continue
		}
		seen[term] = struct{}{}
		kind := "word"
		if isLikelyAcronym(term) {
			kind = "acronym"
		}
		terms = append(terms, SparseTerm{
			Value:  term,
			Kind:   kind,
			Source: "original",
		})
		if len(terms) >= maxTerms {
			break
		}
	}
	return terms, dropped
}

func flattenSparseTerms(terms []SparseTerm) []string {
	out := make([]string, 0, len(terms))
	for _, term := range terms {
		if strings.TrimSpace(term.Value) == "" {
			continue
		}
		out = append(out, term.Value)
	}
	return out
}

func isLikelyAcronym(term string) bool {
	if len(term) < 2 || len(term) > 8 {
		return false
	}
	return strings.ToUpper(term) == term || strings.ToLower(term) == term
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
