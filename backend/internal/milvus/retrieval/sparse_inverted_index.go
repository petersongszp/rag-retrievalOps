package retrieval

import (
	"math"
	"sort"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// SparseIndexConfig controls the explicit BM25 inverted index.
type SparseIndexConfig struct {
	K1 float64
	B  float64
}

type sparsePosting struct {
	docID string
	tf    int
}

// SparseSearchHit is a ranked hit returned by the sparse BM25 index.
type SparseSearchHit struct {
	DocID    string
	Document *schema.Document
	Score    float64
}

// SparseInvertedIndex stores postings and document statistics for BM25 ranking.
type SparseInvertedIndex struct {
	postings     map[string][]sparsePosting
	documents    map[string]*schema.Document
	docLengths   map[string]int
	avgDocLength float64
	totalDocs    float64
	config       SparseIndexConfig
}

// BuildSparseInvertedIndex builds an explicit inverted index from sparse candidates.
func BuildSparseInvertedIndex(docs []*schema.Document, cfg *SparseIndexConfig) *SparseInvertedIndex {
	indexCfg := SparseIndexConfig{
		K1: 1.2,
		B:  0.75,
	}
	if cfg != nil {
		if cfg.K1 > 0 {
			indexCfg.K1 = cfg.K1
		}
		if cfg.B >= 0 && cfg.B <= 1 {
			indexCfg.B = cfg.B
		}
	}

	index := &SparseInvertedIndex{
		postings:   make(map[string][]sparsePosting),
		documents:  make(map[string]*schema.Document),
		docLengths: make(map[string]int),
		config:     indexCfg,
	}
	if len(docs) == 0 {
		return index
	}

	totalLength := 0
	for _, doc := range docs {
		if doc == nil {
			continue
		}

		docID := strings.TrimSpace(doc.ID)
		if docID == "" {
			docID = buildPseudoDocID(doc)
			doc.ID = docID
		}
		if docID == "" {
			continue
		}

		tfByTerm := tokenizeWithFreq(doc.Content)
		docLength := 0
		for _, tf := range tfByTerm {
			docLength += tf
		}
		if docLength == 0 {
			docLength = 1
		}

		index.documents[docID] = doc
		index.docLengths[docID] = docLength
		totalLength += docLength

		for term, tf := range tfByTerm {
			index.postings[term] = append(index.postings[term], sparsePosting{
				docID: docID,
				tf:    tf,
			})
		}
	}

	index.totalDocs = float64(len(index.documents))
	if index.totalDocs == 0 {
		return index
	}

	index.avgDocLength = float64(totalLength) / index.totalDocs
	if index.avgDocLength <= 0 {
		index.avgDocLength = 1
	}

	return index
}

// Search ranks query terms against the inverted index with BM25.
func (idx *SparseInvertedIndex) Search(queryTerms []string, topK int) []SparseSearchHit {
	if idx == nil || idx.totalDocs == 0 || len(queryTerms) == 0 {
		return []SparseSearchHit{}
	}

	scores := make(map[string]float64, len(idx.documents))
	seenTerms := make(map[string]struct{}, len(queryTerms))
	for _, rawTerm := range queryTerms {
		// 1. 清洗查询词：去空格 + 转小写（大小写不敏感检索）
		term := strings.TrimSpace(strings.ToLower(rawTerm))
		if term == "" {
			continue
		}
		// 2. 跳过重复查询词（去重）
		if _, exists := seenTerms[term]; exists {
			continue
		}
		seenTerms[term] = struct{}{}

		// 3. 从倒排索引中，获取该词对应的所有文档（倒排表）
		postings := idx.postings[term]
		if len(postings) == 0 {
			continue
		}

		// 4. 计算BM25核心参数：IDF 逆文档频率
		df := float64(len(postings))
		idf := math.Log(1 + (idx.totalDocs-df+0.5)/(df+0.5))

		// 5. 遍历所有匹配该词的文档，计算并累加BM25分数
		for _, posting := range postings {
			// 文档长度兜底
			docLength := float64(idx.docLengths[posting.docID])
			if docLength <= 0 {
				docLength = 1
			}

			tf := float64(posting.tf) // 词频：该词在文档中出现的次数
			// 长度归一化系数（惩罚过长文档，避免长文本刷分）
			norm := idx.config.K1 * (1 - idx.config.B + idx.config.B*(docLength/idx.avgDocLength))
			// BM25核心公式：分数 = IDF * 归一化词频，累加到文档总分
			scores[posting.docID] += idf * (tf * (idx.config.K1 + 1) / (tf + norm))
		}
	}

	hits := make([]SparseSearchHit, 0, len(scores))
	for docID, score := range scores {
		// 过滤无效分数（分数≤0的文档，无相关性）
		if score <= 0 {
			continue
		}
		// 组装结果对象
		hits = append(hits, SparseSearchHit{
			DocID:    docID,
			Document: idx.documents[docID],
			Score:    score,
		})
	}

	sort.SliceStable(hits, func(i, j int) bool {
		// 分数相同：按文档ID升序排序（保证结果稳定）
		if hits[i].Score == hits[j].Score {
			return hits[i].DocID < hits[j].DocID
		}
		// 分数不同：按分数降序（高分在前，最相关优先）
		return hits[i].Score > hits[j].Score
	})

	if topK > 0 && len(hits) > topK {
		hits = hits[:topK]
	}
	return hits
}
