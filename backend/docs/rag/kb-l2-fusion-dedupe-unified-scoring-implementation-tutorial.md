# 知识库检索二级流程的融合、去重与统一打分实现教程

## 背景

这篇教程讲的是知识库检索链路里的一个非常具体、但非常关键的能力：

**把 dense 和 sparse 两路召回结果拉到同一条流水线上，先做统一打分，再做去重，最后把结果交给 rerank 和最终返回。**

如果没有这一层，系统很容易出现三个问题：

1. `dense_score` 和 `sparse_score` 量纲不同，不能直接放在一起比较。
2. 同一个文档块可能被两条 route 同时召回，结果列表里会出现重复。
3. 后续 rerank、日志、tool 输出都需要稳定读取 `score`，但不同 route 如果各用各的字段，公共层会越来越乱。

这次实现的核心不是“多写几个 helper”，而是把混合检索从“两个结果数组拼起来”升级成一条真正可维护的 L2 后处理流水线。

---

## 这篇教程会做什么

跟着这篇文档做完后，你会得到这样一条检索后处理链路：

`dense/sparse 召回 -> 写入统一 score 来源 -> 路由内归一化 -> 加权融合 -> 按 chunk 去重 -> 保留路由贡献 -> rerank -> 截断返回`

这次会涉及这些文件：

1. `backend/internal/milvus/retrieval/search.go`
2. `backend/internal/milvus/retrieval/sparse_search.go`
3. `backend/internal/milvus/retrieval/hybrid_search.go`
4. `backend/internal/milvus/retrieval/fusion.go`
5. `backend/internal/milvus/retrieval/dedupe.go`
6. `backend/internal/milvus/retrieval/fusion_test.go`
7. `backend/internal/milvus/init.go`

最终控制流可以先理解成这样：

1. dense route 先从向量检索里拿到候选，并把原始向量分写到 `score` 和 `dense_score`。
2. sparse route 先做关键词候选召回，再用 BM25 排序，并把结果写到 `score` 和 `sparse_score`。
3. `FuseRouteCandidates` 分别对 dense 与 sparse 的原始分做归一化，再按权重算出新的统一主分。
4. `DeduplicateFusedDocuments` 按 `document_id + chunk_id` 去重，并把多路贡献沉淀到 metadata。
5. `HybridRetriever.SearchWithRequest` 把这条 L2 链路串起来，并负责空结果原因、日志、rerank 和 TopK 截断。

---

## 需要先理解的术语

### 什么是检索通道

这里的 `route` 不是 HTTP 路由，而是“检索通道”。

在这次实现里只有两条 route：

1. `dense`：向量检索通道。
2. `sparse`：关键词/BM25 检索通道。

同一个 query 会同时跑这两条 route，然后在 L2 层统一处理结果。

### 什么是统一打分

统一打分可以先理解成：

**不再直接拿 route 原始分做跨路比较，而是先把每条 route 自己的分数压到同一范围，再乘权重，得到可比较的新分数。**

比如：

1. dense 命中的原始分可能是 `0.91`
2. sparse 命中的原始分可能是 `11.2`

这两个数字看起来都是“分数”，但它们根本不是同一种量纲，直接比大小没有意义。所以我们先对每个 route 单独归一化到 `0 ~ 1`，再按权重做融合。

### 什么是归一化

归一化（normalization）可以先理解成“把一个 route 内部的原始分拉到统一范围”。

这次用的是最容易理解的 `min-max` 归一化：

`(score - min) / (max - min)`

如果某条 route 只有一个候选，或者所有候选分都一样，那就没法正常算分母。这时实现里采用了一个很稳妥的兜底：

1. 如果分数小于等于 0，归一化成 0
2. 否则归一化成 1

这样做的目的不是追求最复杂的统计效果，而是先保证工程行为稳定。

### 什么是去重键

去重键（dedupe key）就是“系统用什么规则判断两条结果其实是同一个文档块”。

这次优先级是：

1. `document_id + chunk_id`
2. 只有 `document_id` 时，用 `document_id`
3. 再不行就退回 `doc.ID`
4. 还不行就退回 pseudo ID

这里最重要的不是 key 长什么样，而是它必须稳定。因为只要 key 不稳定，融合之后的重复结果就永远消不干净。

### 什么是主检索通道

同一个 chunk 可能同时来自 dense 和 sparse。去重后最终只能保留一份文档对象，所以需要一个“主 route”。

这次的规则很直接：

**谁的融合分更高，谁就是最终保留下来的 primary route。**

但同时我们不会丢掉另一条 route 的贡献，而是把它写进：

1. `route_contrib`
2. `route_raw_scores`
3. `source.route_contrib`

所以你可以把它理解成：

1. `route` 表示当前展示给上层看的主来源
2. `route_contrib` 表示这条结果其实受过哪些 route 的贡献

---

## 整体流程

先看全局，不急着看代码。

这次 L2 流水线的执行顺序是：

1. 请求进入 `HybridRetriever.SearchWithRequest`
2. dense route 和 sparse route 并发召回
3. dense route 结果写入 `dense_score`
4. sparse route 结果写入 `sparse_score`
5. 两路结果进入 `FuseRouteCandidates`
6. 在每条 route 内做归一化，然后按 `DenseWeight / SparseWeight` 算融合分
7. 融合后的候选进入 `DeduplicateFusedDocuments`
8. 按 `document_id + chunk_id` 去重，并保留所有 route 的贡献信息
9. 去重结果进入 reranker
10. 最后按 `TopK` 截断并返回

可以把这一层的职责理解成一句话：

**L1 负责“把候选找回来”，L2 负责“把候选整理成可比较、可消费、可观测的最终候选池”。**

---

## 分步实现

## 第一步：先确保两条检索通道都会写统一分数字段

### 目标

先把 dense 和 sparse 的结果结构对齐。否则后面的融合层就没有稳定输入。

### 文件

`backend/internal/milvus/retrieval/search.go`

### 完整代码

```go
package retrieval

import (
	"context"
	"fmt"

	"github.com/bytedance/sonic"
	milvusRetriever "github.com/cloudwego/eino-ext/components/retriever/milvus"
	"github.com/cloudwego/eino/schema"
	milvusClient "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// SearchWithExpr 使用表达式过滤进行检索
func SearchWithExpr(
	ctx context.Context,
	client milvusClient.Client,
	config *milvusRetriever.RetrieverConfig,
	query string,
	expr string,
	opts *RetrieveOptions,
) ([]*schema.Document, error) {
	embedder := config.Embedding
	if embedder == nil {
		return nil, fmt.Errorf("embedding is nil")
	}

	vectors, err := embedder.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("empty embedding result")
	}

	queryVector := make([]float32, len(vectors[0]))
	for i, v := range vectors[0] {
		queryVector[i] = float32(v)
	}

	topK := opts.TopK
	if topK <= 0 {
		topK = config.TopK
		if topK <= 0 {
			topK = 10
		}
	}

	collectionName := config.Collection
	if opts.Collection != "" {
		collectionName = opts.Collection
	}

	searchParam, err := entity.NewIndexAUTOINDEXSearchParam(1)
	if err != nil {
		return nil, fmt.Errorf("failed to create search param: %w", err)
	}

	searchResults, err := client.Search(
		ctx,
		collectionName,
		[]string{},
		expr,
		[]string{"id", "content", "metadata"},
		[]entity.Vector{entity.FloatVector(queryVector)},
		config.VectorField,
		config.MetricType,
		topK,
		searchParam,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	documents := make([]*schema.Document, 0)
	if len(searchResults) > 0 {
		result := searchResults[0]
		contentField := result.Fields.GetColumn("content")
		metadataField := result.Fields.GetColumn("metadata")
		idField := result.IDs

		resultCount := idField.Len()
		for i := 0; i < resultCount; i++ {
			idValue, err := idField.GetAsString(i)
			if err != nil {
				continue
			}

			doc := &schema.Document{
				ID:       idValue,
				Content:  "",
				MetaData: make(map[string]interface{}),
			}

			if contentField != nil {
				if content, err := contentField.GetAsString(i); err == nil {
					doc.Content = content
				}
			}

			if metadataField != nil {
				if jsonStr, err := metadataField.GetAsString(i); err == nil && jsonStr != "" {
					var metadata map[string]interface{}
					if err := sonic.Unmarshal([]byte(jsonStr), &metadata); err == nil {
						doc.MetaData = metadata
					}
				}
			}

			if i < len(result.Scores) {
				doc.MetaData["score"] = result.Scores[i]
			}

			documents = append(documents, doc)
		}
	}

	return documents, nil
}
```

### 这段代码在做什么

dense 检索的底层入口已经在这里把 Milvus 返回的向量分数写到了 `doc.MetaData["score"]`。这件事非常重要，因为它给上层提供了一个最基础的统一读取口。

换句话说，哪怕你后面还会补 `dense_score`、还会做融合分，dense route 至少已经先保证了一个公共分数字段存在。

### 为什么要这样写

更简单的做法是“dense 只保留 Milvus 原始结果，上层谁要分数谁自己去找”。短期看省事，长期会出问题：

1. 上层要知道底层返回结构细节。
2. 每一层都可能各自写一套读分逻辑。
3. 一旦后面加 sparse、fusion、rerank，分数读取会迅速分叉。

所以这里先把 dense 结果统一投影成 `schema.Document + metadata["score"]`，后面所有公共层都能围绕这个契约继续扩展。

### 它如何衔接下一步

接下来 sparse route 也要做同样的事情，但它除了写 `score`，还要保留 `sparse_score` 这个 route 专属字段。

---

## 第二步：让关键词通道既保留原始分，又写统一分数字段

### 目标

把 sparse route 的输出也对齐成公共契约，并且保留足够的 route 信息给后面的融合层使用。

### 文件

`backend/internal/milvus/retrieval/sparse_search.go`

### 完整代码

```go
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
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return nil, fmt.Errorf("query is empty")
	}

	topK := req.TopK
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
```

### 这段代码在做什么

这段代码完成了 sparse route 的两层工作：

1. 先用 `like` 把每个 term 对应的候选文档捞回来。
2. 再在应用层用倒排索引和 BM25 做重排。

关键点在最后这三行：

```go
doc.MetaData["route"] = routeSparse
doc.MetaData["sparse_score"] = hit.Score
doc.MetaData["score"] = hit.Score
```

它们建立了一个很重要的分层：

1. `sparse_score` 是 route 专属原始分，给融合层读取。
2. `score` 是公共分数字段，给公共流程兜底读取。

### 为什么要这样写

如果 sparse 只写 `sparse_score`，那公共层就必须知道“这条结果来自 sparse，所以去读 `sparse_score`；另一条来自 dense，所以去读 `score` 或 `dense_score`”。这样逻辑很快会散到很多地方。

而如果 sparse 只写 `score`，又会丢掉 route 专属分，后面就没法做精确融合。

所以这里的设计重点不是“多写一个字段”，而是同时满足两件事：

1. 公共层永远有稳定入口去读分。
2. route 层原始信息不丢失。

### 它如何衔接下一步

现在 dense 和 sparse 都能把分数写进公共结构了。下一步要做的是让 `HybridRetriever` 真正进入 L2 逻辑，不再只是简单合并数组。

---

## 第三步：在混合检索器里接入二级处理流水线

### 目标

把“并发召回结果”升级成“可融合、可去重、可观测的 L2 处理链路”。

### 文件

`backend/internal/milvus/retrieval/hybrid_search.go`

### 完整代码

```go
package retrieval

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"

	"interview-agents/internal/observability/metrics"
)

const (
	routeDense  = "dense"
	routeSparse = "sparse"
)

// HybridSearchRequest is the unified recall input contract for L1.
// query/expr/topk/kb_scope/kb_id/request_id
type HybridSearchRequest struct {
	Query      string
	Expr       string
	TopK       int
	KBScope    string
	KBID       uint64
	RequestID  string
	Collection string
}

// HybridRetriever orchestrates dense + sparse routes and keeps backward compatibility with Search(ctx, query, opts).
type HybridRetriever struct {
	retriever       *RetrieverService
	sparseRetriever *SparseRetriever
	reranker        Reranker
	config          *HybridRetrieverConfig
}

// HybridRetrieverConfig controls mixed retrieval behavior in L1.
type HybridRetrieverConfig struct {
	CandidateTopK int
	DenseWeight   float64
	SparseWeight  float64
	SparseConfig  *SparseRetrieverConfig
	RerankerImpl  Reranker
}

func NewHybridRetriever(retriever *RetrieverService, config *HybridRetrieverConfig) (*HybridRetriever, error) {
	if retriever == nil {
		return nil, fmt.Errorf("retriever service is nil")
	}
	if config == nil {
		config = &HybridRetrieverConfig{CandidateTopK: 10}
	}
	if config.CandidateTopK <= 0 {
		config.CandidateTopK = 10
	}
	if config.DenseWeight <= 0 {
		config.DenseWeight = 0.7
	}
	if config.SparseWeight <= 0 {
		config.SparseWeight = 0.3
	}

	sparseRetriever, err := NewSparseRetriever(retriever.client, retriever.config.Collection, config.SparseConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to init sparse retriever: %w", err)
	}

	hr := &HybridRetriever{
		retriever:       retriever,
		sparseRetriever: sparseRetriever,
		config:          config,
	}
	if config.RerankerImpl != nil {
		hr.reranker = config.RerankerImpl
	} else {
		hr.reranker = NewJaccardReranker(nil)
	}
	return hr, nil
}

// Search 混合检索器的对外搜索入口方法
// 入参：上下文、用户查询词、检索配置
// 返回：匹配的文档列表、错误信息
func (h *HybridRetriever) Search(ctx context.Context, query string, opts *RetrieveOptions) ([]*schema.Document, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is empty")
	}

	requestID := ""
	if opts != nil {
		requestID = strings.TrimSpace(opts.RequestID)
	}
	if requestID == "" {
		requestID = fmt.Sprintf("hyb-%d", time.Now().UnixNano())
	}

	topK := h.config.CandidateTopK
	if opts != nil && opts.TopK > 0 {
		topK = opts.TopK
	}

	req := &HybridSearchRequest{
		Query:      query,
		TopK:       topK,
		RequestID:  requestID,
		Collection: h.retriever.config.Collection,
	}

	if opts != nil {
		req.Expr = strings.TrimSpace(BuildFilterExpr(opts))
		if req.Expr == "" {
			req.Expr = strings.TrimSpace(opts.Expr)
		}
		req.KBScope = strings.TrimSpace(opts.KBScope)
		req.KBID = opts.ActiveGlobalKBID
		if strings.TrimSpace(opts.Collection) != "" {
			req.Collection = strings.TrimSpace(opts.Collection)
		}
	}

	return h.SearchWithRequest(ctx, req)
}

// SearchWithRequest is the L1 hybrid recall entry.
func (h *HybridRetriever) SearchWithRequest(ctx context.Context, req *HybridSearchRequest) ([]*schema.Document, error) {
	if req == nil {
		return nil, fmt.Errorf("hybrid search request is nil")
	}
	if strings.TrimSpace(req.Query) == "" {
		return nil, fmt.Errorf("query is empty")
	}
	if strings.TrimSpace(req.RequestID) == "" {
		req.RequestID = fmt.Sprintf("hyb-%d", time.Now().UnixNano())
	}
	if req.TopK <= 0 {
		req.TopK = h.config.CandidateTopK
	}
	if strings.TrimSpace(req.Collection) == "" {
		req.Collection = h.retriever.config.Collection
	}

	start := time.Now()
	type routeResult struct {
		route    string
		docs     []*schema.Document
		err      error
		duration time.Duration
	}

	resultCh := make(chan routeResult, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		routeStart := time.Now()
		docs, err := h.searchDense(ctx, req)
		resultCh <- routeResult{
			route:    routeDense,
			docs:     docs,
			err:      err,
			duration: time.Since(routeStart),
		}
	}()

	go func() {
		defer wg.Done()
		routeStart := time.Now()
		docs, err := h.sparseRetriever.Search(ctx, req)
		resultCh <- routeResult{
			route:    routeSparse,
			docs:     docs,
			err:      err,
			duration: time.Since(routeStart),
		}
	}()

	wg.Wait()
	close(resultCh)

	var (
		denseDocs  []*schema.Document
		sparseDocs []*schema.Document
		denseErr   error
		sparseErr  error
		denseMS    int64
		sparseMS   int64
	)
	for routeRes := range resultCh {
		switch routeRes.route {
		case routeDense:
			denseDocs = routeRes.docs
			denseErr = routeRes.err
			denseMS = routeRes.duration.Milliseconds()
		case routeSparse:
			sparseDocs = routeRes.docs
			sparseErr = routeRes.err
			sparseMS = routeRes.duration.Milliseconds()
		}
		h.observeRouteMetric(routeRes.route, routeRes.duration, routeRes.err, len(routeRes.docs))
	}

	if denseErr != nil && sparseErr != nil {
		totalMS := time.Since(start).Milliseconds()
		log.Printf(
			"[RAG:L1] request_id=%s query=%q final_query=%q expr=%q topk=%d routes=%s route_hits={dense:0,sparse:0} final_count=0 empty_reason=empty-after-retrieve duration_ms=%d dense_error=%q sparse_error=%q",
			req.RequestID, req.Query, req.Query, req.Expr, req.TopK, "dense+sparse", totalMS, denseErr.Error(), sparseErr.Error(),
		)
		return nil, fmt.Errorf("hybrid retrieval failed: dense=%v sparse=%v", denseErr, sparseErr)
	}

	rawCandidateCount := len(denseDocs) + len(sparseDocs)
	if rawCandidateCount == 0 {
		totalMS := time.Since(start).Milliseconds()
		log.Printf(
			"[RAG:L2] request_id=%s query=%q final_query=%q expr=%q topk=%d routes=%s route_hits={dense:%d,sparse:%d} final_count=0 empty_reason=%s duration_ms=%d dense_ms=%d sparse_ms=%d dense_error=%q sparse_error=%q",
			req.RequestID, req.Query, req.Query, req.Expr, req.TopK, "dense+sparse", len(denseDocs), len(sparseDocs), EmptyReasonAfterRetrieve, totalMS, denseMS, sparseMS, toLogError(denseErr), toLogError(sparseErr),
		)
		return []*schema.Document{}, nil
	}

	fused := FuseRouteCandidates(denseDocs, sparseDocs, FusionConfig{
		DenseWeight:  h.config.DenseWeight,
		SparseWeight: h.config.SparseWeight,
	})
	if len(fused) == 0 {
		totalMS := time.Since(start).Milliseconds()
		log.Printf(
			"[RAG:L2] request_id=%s query=%q final_query=%q expr=%q topk=%d routes=%s route_hits={dense:%d,sparse:%d} final_count=0 empty_reason=%s duration_ms=%d dense_ms=%d sparse_ms=%d dense_error=%q sparse_error=%q",
			req.RequestID, req.Query, req.Query, req.Expr, req.TopK, "dense+sparse", len(denseDocs), len(sparseDocs), EmptyReasonAfterFusion, totalMS, denseMS, sparseMS, toLogError(denseErr), toLogError(sparseErr),
		)
		return []*schema.Document{}, nil
	}

	merged := DeduplicateFusedDocuments(fused)
	if len(merged) == 0 {
		totalMS := time.Since(start).Milliseconds()
		log.Printf(
			"[RAG:L2] request_id=%s query=%q final_query=%q expr=%q topk=%d routes=%s route_hits={dense:%d,sparse:%d} final_count=0 empty_reason=%s duration_ms=%d dense_ms=%d sparse_ms=%d dense_error=%q sparse_error=%q",
			req.RequestID, req.Query, req.Query, req.Expr, req.TopK, "dense+sparse", len(denseDocs), len(sparseDocs), EmptyReasonAfterFusion, totalMS, denseMS, sparseMS, toLogError(denseErr), toLogError(sparseErr),
		)
		return []*schema.Document{}, nil
	}

	if h.reranker != nil {
		reranked, err := h.reranker.Rerank(ctx, req.Query, merged)
		if err == nil && len(reranked) > 0 {
			merged = reranked
		}
	}

	emptyReason := EmptyReasonNone
	if len(merged) == 0 {
		emptyReason = EmptyReasonAfterFilter
	}

	if len(merged) > req.TopK {
		merged = merged[:req.TopK]
	}

	totalMS := time.Since(start).Milliseconds()
	log.Printf(
		"[RAG:L2] request_id=%s query=%q final_query=%q expr=%q topk=%d routes=%s route_hits={dense:%d,sparse:%d} final_count=%d empty_reason=%s duration_ms=%d dense_ms=%d sparse_ms=%d dense_error=%q sparse_error=%q",
		req.RequestID,
		req.Query,
		req.Query,
		req.Expr,
		req.TopK,
		"dense+sparse",
		len(denseDocs),
		len(sparseDocs),
		len(merged),
		emptyReason,
		totalMS,
		denseMS,
		sparseMS,
		toLogError(denseErr),
		toLogError(sparseErr),
	)
	return merged, nil
}

func (h *HybridRetriever) searchDense(ctx context.Context, req *HybridSearchRequest) ([]*schema.Document, error) {
	opts := &RetrieveOptions{
		Expr:             req.Expr,
		TopK:             req.TopK,
		Collection:       req.Collection,
		KBScope:          req.KBScope,
		ActiveGlobalKBID: req.KBID,
		RequestID:        req.RequestID,
	}
	docs, err := h.retriever.RetrieveWithOptions(ctx, req.Query, opts)
	if err != nil {
		return nil, err
	}
	for _, doc := range docs {
		if doc.MetaData == nil {
			doc.MetaData = make(map[string]interface{})
		}
		doc.MetaData["route"] = routeDense
		doc.MetaData["dense_score"] = readDocScore(doc)
	}
	return docs, nil
}

func (h *HybridRetriever) observeRouteMetric(route string, duration time.Duration, routeErr error, hitCount int) {
	status := "ok"
	errCode := "none"
	if routeErr != nil {
		status = "error"
		errCode = "route_failed"
	}
	metrics.ObserveRetrieveRoute(route, duration, status, errCode, hitCount)
}

func readDocScore(doc *schema.Document) float64 {
	if doc == nil || doc.MetaData == nil {
		return 0
	}
	if value, ok := doc.MetaData["score"]; ok {
		if score, ok := castScore(value); ok {
			return score
		}
	}
	if value, ok := doc.MetaData["sparse_score"]; ok {
		if score, ok := castScore(value); ok {
			return score
		}
	}
	return 0
}

func toLogError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
```

### 这段代码在做什么

这一步做了四件关键事情：

1. 在 `HybridRetrieverConfig` 里加入 `DenseWeight` 和 `SparseWeight`。
2. 在 `searchDense` 里补写 `dense_score`。
3. 在 `SearchWithRequest` 里用 `FuseRouteCandidates` 替代旧的“简单拼接 + 高分保留”。
4. 把日志阶段从旧的 L1 合并，升级成更清晰的 L2 空结果原因追踪。

这里最值得注意的是这一段：

```go
fused := FuseRouteCandidates(...)
merged := DeduplicateFusedDocuments(fused)
if h.reranker != nil {
    reranked, err := h.reranker.Rerank(ctx, req.Query, merged)
    ...
}
```

它把处理顺序固定成：

`先融合 -> 再去重 -> 再 rerank`

### 为什么要这样写

最容易想到的简单方案是“dense 和 sparse 拼一起后，按原始 score 排序，再直接去重”。这个方案表面简单，但会马上出问题：

1. 原始分不可比，dense 的 `0.91` 不代表一定比 sparse 的 `11.2` 差。
2. 如果先去重再记录路由贡献，会把另一条 route 的信息丢掉。
3. 如果 route 内部先各自 rerank，再拼起来，就不是对同一个统一候选池做比较了。

所以正确的工程顺序是：

1. 先让两条 route 的分数进入同一比较空间。
2. 再按稳定 key 折叠重复项。
3. 再把去重后的统一候选池交给 reranker。

### 它如何衔接下一步

现在主干链路已经接通了。下一步要看的就是 `FuseRouteCandidates` 本身，它是统一打分的核心。

---

## 第四步：实现路由内归一化和加权融合

### 目标

把 dense 和 sparse 的候选变成“可跨 route 比较”的统一候选。

### 文件

`backend/internal/milvus/retrieval/fusion.go`

### 完整代码

```go
package retrieval

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/cloudwego/eino/schema"
)

const (
	// EmptyReasonNone 表示本次请求最终有结果。
	EmptyReasonNone = "None"
	// EmptyReasonAfterRetrieve 表示多路召回结束后候选池为空。
	EmptyReasonAfterRetrieve = "Empty-After-Retrieve"
	// EmptyReasonAfterFusion 表示召回有结果，但融合/去重后被全部淘空。
	EmptyReasonAfterFusion = "Empty-After-Fusion"
	// EmptyReasonAfterFilter 表示后续重排/过滤/截断后结果为空。
	EmptyReasonAfterFilter = "Empty-After-Filter"
)

// FusionConfig 控制多路候选的归一化与加权融合行为。
type FusionConfig struct {
	DenseWeight  float64
	SparseWeight float64
}

// RouteContribution 记录单条文档在某一路由上的归一化贡献。
type RouteContribution struct {
	Route           string
	RawScore        float64
	NormalizedScore float64
	WeightedScore   float64
}

// FusedDocument 表示融合但尚未去重的候选项。
type FusedDocument struct {
	Doc              *schema.Document
	Key              string
	Score            float64
	PrimaryRoute     string
	RouteContrib     map[string]float64
	RouteRawScores   map[string]float64
	Contributions    []RouteContribution
	SourceCollection string
}

// FuseRouteCandidates 对 dense/sparse 候选做归一化并生成统一主分。
func FuseRouteCandidates(denseDocs, sparseDocs []*schema.Document, cfg FusionConfig) []*FusedDocument {
	cfg = normalizeFusionConfig(cfg)
	inputs := []struct {
		route  string
		weight float64
		docs   []*schema.Document
	}{
		{route: routeDense, weight: cfg.DenseWeight, docs: denseDocs},
		{route: routeSparse, weight: cfg.SparseWeight, docs: sparseDocs},
	}

	results := make([]*FusedDocument, 0, len(denseDocs)+len(sparseDocs))
	for _, input := range inputs {
		rawScores := collectRouteScores(input.docs, input.route)
		stats := buildScoreStats(rawScores)
		for _, doc := range input.docs {
			if doc == nil {
				continue
			}
			key := buildDedupeKey(doc)
			if key == "" {
				continue
			}

			rawScore := readRouteScore(doc, input.route)
			normalizedScore := normalizeRouteScore(rawScore, stats)
			weightedScore := normalizedScore * input.weight

			annotatedDoc := cloneDocumentWithMetadata(doc)
			if annotatedDoc.MetaData == nil {
				annotatedDoc.MetaData = make(map[string]interface{})
			}
			annotatedDoc.MetaData["fusion_score"] = weightedScore
			annotatedDoc.MetaData["score"] = weightedScore
			annotatedDoc.MetaData["route"] = input.route
			annotatedDoc.MetaData["route_score"] = rawScore

			results = append(results, &FusedDocument{
				Doc:          annotatedDoc,
				Key:          key,
				Score:        weightedScore,
				PrimaryRoute: input.route,
				RouteContrib: map[string]float64{
					input.route: weightedScore,
				},
				RouteRawScores: map[string]float64{
					input.route: rawScore,
				},
				Contributions: []RouteContribution{
					{
						Route:           input.route,
						RawScore:        rawScore,
						NormalizedScore: normalizedScore,
						WeightedScore:   weightedScore,
					},
				},
				SourceCollection: readCollectionFromDoc(doc),
			})
		}
	}

	sort.SliceStable(results, func(i, j int) bool {
		if nearlyEqual(results[i].Score, results[j].Score) {
			return results[i].Key < results[j].Key
		}
		return results[i].Score > results[j].Score
	})
	return results
}

func normalizeFusionConfig(cfg FusionConfig) FusionConfig {
	if cfg.DenseWeight <= 0 {
		cfg.DenseWeight = 0.7
	}
	if cfg.SparseWeight <= 0 {
		cfg.SparseWeight = 0.3
	}
	total := cfg.DenseWeight + cfg.SparseWeight
	if total <= 0 {
		return FusionConfig{DenseWeight: 0.7, SparseWeight: 0.3}
	}
	cfg.DenseWeight = cfg.DenseWeight / total
	cfg.SparseWeight = cfg.SparseWeight / total
	return cfg
}

type scoreStats struct {
	min float64
	max float64
}

func buildScoreStats(scores []float64) scoreStats {
	if len(scores) == 0 {
		return scoreStats{}
	}
	stats := scoreStats{min: scores[0], max: scores[0]}
	for _, score := range scores[1:] {
		if score < stats.min {
			stats.min = score
		}
		if score > stats.max {
			stats.max = score
		}
	}
	return stats
}

func collectRouteScores(docs []*schema.Document, route string) []float64 {
	scores := make([]float64, 0, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		scores = append(scores, readRouteScore(doc, route))
	}
	return scores
}

func normalizeRouteScore(score float64, stats scoreStats) float64 {
	if stats.max <= stats.min {
		if score <= 0 {
			return 0
		}
		return 1
	}
	normalized := (score - stats.min) / (stats.max - stats.min)
	if normalized < 0 {
		return 0
	}
	if normalized > 1 {
		return 1
	}
	return normalized
}

func readRouteScore(doc *schema.Document, route string) float64 {
	if doc == nil || doc.MetaData == nil {
		return 0
	}

	switch route {
	case routeDense:
		if value, ok := doc.MetaData["dense_score"]; ok {
			if score, ok := castScore(value); ok {
				return score
			}
		}
	case routeSparse:
		if value, ok := doc.MetaData["sparse_score"]; ok {
			if score, ok := castScore(value); ok {
				return score
			}
		}
	}

	return readDocScore(doc)
}

func castScore(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

func buildDedupeKey(doc *schema.Document) string {
	if doc == nil {
		return ""
	}
	documentID := strings.TrimSpace(readMetadataString(doc, "document_id"))
	chunkID := strings.TrimSpace(readMetadataString(doc, "chunk_id"))
	if documentID != "" && chunkID != "" {
		return fmt.Sprintf("%s:%s", documentID, chunkID)
	}
	if documentID != "" {
		return documentID
	}
	if docID := strings.TrimSpace(doc.ID); docID != "" {
		return docID
	}
	return buildPseudoDocID(doc)
}

func cloneDocumentWithMetadata(doc *schema.Document) *schema.Document {
	if doc == nil {
		return nil
	}
	cloned := &schema.Document{
		ID:      doc.ID,
		Content: doc.Content,
	}
	if doc.MetaData != nil {
		cloned.MetaData = make(map[string]interface{}, len(doc.MetaData))
		for key, value := range doc.MetaData {
			cloned.MetaData[key] = value
		}
	}
	return cloned
}

func readMetadataString(doc *schema.Document, key string) string {
	if doc == nil || doc.MetaData == nil {
		return ""
	}
	value, ok := doc.MetaData[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func readCollectionFromDoc(doc *schema.Document) string {
	if doc == nil {
		return ""
	}
	for _, key := range []string{"collection", "collection_name"} {
		if value := readMetadataString(doc, key); value != "" {
			return value
		}
	}
	return ""
}

func nearlyEqual(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9
}
```

### 这段代码在做什么

这个文件是整个 L2 的核心。它做的不是“简单合并”，而是这几件更精细的事情：

1. 把 route 权重正规化，避免配置传进来不是 `0.7 + 0.3` 时行为混乱。
2. 在每条 route 内收集原始分，算出 `min/max`。
3. 按 route 读取对应的原始分字段。
4. 把原始分归一化，再乘 route 权重，得到新的 `weightedScore`。
5. 把这个新分数写回 `fusion_score` 和公共 `score`。
6. 同时构造 `FusedDocument`，把 key、主 route、原始分、贡献分都带出去。

最关键的工程动作是这两行：

```go
annotatedDoc.MetaData["fusion_score"] = weightedScore
annotatedDoc.MetaData["score"] = weightedScore
```

它意味着从融合层往后，公共流程看到的 `score` 已经不再是 route 原始分，而是“统一比较空间里的主分”。

### 为什么要这样写

如果我们只是把 `dense_score` 和 `sparse_score` 原样带到后面，让 rerank 或调用方自己理解，会有三个直接后果：

1. 公共层没人知道应该比较哪个分。
2. 不同调用方可能会自己发明不同的比较规则。
3. 日志和调试信息会变得不一致。

所以这里的原则很明确：

1. route 原始分保留在 route 专属字段里。
2. 融合之后的公共主分统一写回 `score`。
3. 后面的公共层只围绕新的主分工作。

你可以把这一层理解成“翻译层”。它把两种不同语言的分数，翻译成一套共同语言。

### 它如何衔接下一步

现在每个候选都已经有统一主分了，但同一个 chunk 仍然可能出现两份。下一步就要按稳定 key 去重，并保留多路贡献信息。

---

## 第五步：按文档块去重，并保留多路贡献

### 目标

把同一个文档块的多路候选折叠成一条最终结果，同时不丢失 route 贡献信息。

### 文件

`backend/internal/milvus/retrieval/dedupe.go`

### 完整代码

```go
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
		contrib      map[string]float64
		rawScores    map[string]float64
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
				contrib:      copyFloatMap(candidate.RouteContrib),
				rawScores:    copyFloatMap(candidate.RouteRawScores),
			}
			continue
		}

		mergeFloatMaps(existing.contrib, candidate.RouteContrib)
		mergeFloatMaps(existing.rawScores, candidate.RouteRawScores)

		if candidate.Score > existing.score || (nearlyEqual(candidate.Score, existing.score) && candidate.Key < existing.key) {
			existing.doc = cloneDocumentWithMetadata(candidate.Doc)
			existing.score = candidate.Score
			existing.primaryRoute = candidate.PrimaryRoute
		}
	}

	out := make([]*schema.Document, 0, len(bestByKey))
	for _, item := range bestByKey {
		annotateDedupedDocument(item.doc, item.score, item.primaryRoute, item.contrib, item.rawScores)
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

func annotateDedupedDocument(doc *schema.Document, score float64, primaryRoute string, contrib, rawScores map[string]float64) {
	if doc == nil {
		return
	}
	if doc.MetaData == nil {
		doc.MetaData = make(map[string]interface{})
	}

	doc.MetaData["score"] = score
	doc.MetaData["fusion_score"] = score
	doc.MetaData["route"] = primaryRoute
	doc.MetaData["route_contrib"] = floatMapToInterfaceMap(contrib)
	doc.MetaData["route_raw_scores"] = floatMapToInterfaceMap(rawScores)

	source := make(map[string]interface{})
	if existing, ok := doc.MetaData["source"].(map[string]interface{}); ok && existing != nil {
		for key, value := range existing {
			source[key] = value
		}
	}
	source["route"] = primaryRoute
	source["route_contrib"] = floatMapToInterfaceMap(contrib)
	if collection := readCollectionFromDoc(doc); collection != "" {
		source["collection"] = collection
	}
	doc.MetaData["source"] = source
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
```

### 这段代码在做什么

这一步做的事情可以概括成一句话：

**同一个 key 只保留一条主文档，但把所有 route 的贡献都留下来。**

实现细节是：

1. `bestByKey` 负责按 dedupe key 折叠重复项。
2. `mergeFloatMaps` 负责把各 route 的贡献和原始分汇总起来。
3. 如果新的候选融合分更高，就让它成为新的主文档。
4. 最后用 `annotateDedupedDocument` 把最终结果写成稳定的 metadata 结构。

这里有几个很值得注意的输出字段：

1. `score` / `fusion_score`
2. `route`
3. `route_contrib`
4. `route_raw_scores`
5. `source.route`
6. `source.route_contrib`

这意味着上层看到的不是“一个被简单去重后的黑盒结果”，而是“一个仍然带诊断信息的最终结果”。

### 为什么要这样写

更简单的去重方法是“发现重复 key 时，保留第一条或最高分那条，然后把其他的直接丢掉”。这对列表展示也许够用，但对工程演进不够：

1. 你会丢失另一条 route 的贡献信息。
2. 调试时看不出这条结果是单路命中还是双路共同支持。
3. 后面如果想做更复杂的 fusion 或 A/B 分析，已经没有原始信息了。

所以这里的关键设计不是“只去重”，而是“去重但不抹平信息”。

### 它如何衔接下一步

到这里，L2 结果已经是最终可消费的候选池了。下一步只剩两个收尾动作：把权重配置接进初始化，再加回归测试。

---

## 第六步：把融合权重从配置接入初始化

### 目标

让 dense/sparse 融合权重不是写死在代码里，而是由配置驱动。

### 文件

`backend/internal/milvus/init.go`

### 完整代码

```go
candidateTopK := cfg.Milvus.TopK * 2
if cfg.RAG.Phase2.CandidateTopK > 0 {
	candidateTopK = cfg.RAG.Phase2.CandidateTopK
}
hybridConfig := &retrieval.HybridRetrieverConfig{
	CandidateTopK: candidateTopK,
	DenseWeight:   cfg.RAG.Phase2.HybridDenseWeight,
	SparseWeight:  cfg.RAG.Phase2.HybridSparseWeight,
	SparseConfig: &retrieval.SparseRetrieverConfig{
		DefaultTopK: candidateTopK,
	},
}
hybridRetriever, err := retrieval.NewHybridRetriever(retrieverService, hybridConfig)
if err != nil {
	return nil, fmt.Errorf("failed to initialize hybrid retriever: %w", err)
}
manager.HybridRetriever = hybridRetriever
```

### 这段代码在做什么

这段代码把 `HybridDenseWeight` 和 `HybridSparseWeight` 从配置层传进 `HybridRetrieverConfig`。

这样一来：

1. 默认值仍然可以在代码里兜底。
2. 生产环境可以通过配置调整权重。
3. 后面做实验或按场景微调时，不需要改源码。

### 为什么要这样写

如果把权重硬编码在 `fusion.go` 里，短期当然能跑，但后面只要想做一次线上调优，就必须重新发版。对检索系统来说，这个反馈回路太慢了。

权重本质上是“策略参数”，不是“算法常量”。所以它应该从初始化和配置层进入。

### 它如何衔接下一步

配置接好以后，最后一步就是补测试，确保这条 L2 链路以后改动时不被悄悄破坏。

---

## 第七步：用测试固定融合和去重行为

### 目标

用一组小而明确的回归测试，把“融合后去重”和“dedupe key 选择规则”固定下来。

### 文件

`backend/internal/milvus/retrieval/fusion_test.go`

### 完整代码

```go
package retrieval

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestFuseRouteCandidatesAndDedupe(t *testing.T) {
	denseDocs := []*schema.Document{
		{
			ID:      "dense-1",
			Content: "golang map runtime internals",
			MetaData: map[string]interface{}{
				"document_id": "doc-1",
				"chunk_id":    "chunk-1",
				"dense_score": 0.91,
				"score":       0.91,
			},
		},
		{
			ID:      "dense-2",
			Content: "java hashmap internals",
			MetaData: map[string]interface{}{
				"document_id": "doc-2",
				"chunk_id":    "chunk-1",
				"dense_score": 0.42,
				"score":       0.42,
			},
		},
	}
	sparseDocs := []*schema.Document{
		{
			ID:      "sparse-1",
			Content: "golang map runtime internals",
			MetaData: map[string]interface{}{
				"document_id":  "doc-1",
				"chunk_id":     "chunk-1",
				"sparse_score": 11.2,
				"score":        11.2,
			},
		},
		{
			ID:      "sparse-3",
			Content: "go channel concurrency",
			MetaData: map[string]interface{}{
				"document_id":  "doc-3",
				"chunk_id":     "chunk-1",
				"sparse_score": 3.6,
				"score":        3.6,
			},
		},
	}

	fused := FuseRouteCandidates(denseDocs, sparseDocs, FusionConfig{
		DenseWeight:  0.7,
		SparseWeight: 0.3,
	})
	if len(fused) != 4 {
		t.Fatalf("expected 4 fused candidates, got %d", len(fused))
	}

	merged := DeduplicateFusedDocuments(fused)
	if len(merged) != 3 {
		t.Fatalf("expected 3 deduped results, got %d", len(merged))
	}

	first := merged[0]
	if got := readMetadataString(first, "route"); got != routeDense {
		t.Fatalf("expected primary route %q, got %q", routeDense, got)
	}

	source, ok := first.MetaData["source"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected source metadata map, got %T", first.MetaData["source"])
	}
	if source["route"] != routeDense {
		t.Fatalf("expected source.route=%q, got %v", routeDense, source["route"])
	}

	routeContrib, ok := source["route_contrib"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected source.route_contrib map, got %T", source["route_contrib"])
	}
	if _, exists := routeContrib[routeDense]; !exists {
		t.Fatalf("expected dense contribution to exist")
	}
	if _, exists := routeContrib[routeSparse]; !exists {
		t.Fatalf("expected sparse contribution to exist")
	}
}

func TestBuildDedupeKeyPrefersDocumentAndChunkID(t *testing.T) {
	doc := &schema.Document{
		ID:      "fallback-id",
		Content: "same chunk",
		MetaData: map[string]interface{}{
			"document_id": "doc-9",
			"chunk_id":    "chunk-3",
		},
	}

	if got := buildDedupeKey(doc); got != "doc-9:chunk-3" {
		t.Fatalf("unexpected dedupe key: %s", got)
	}
}
```

### 这段代码在做什么

这组测试验证了两件最容易在重构时被破坏的行为：

1. 同一个 chunk 被 dense 和 sparse 同时召回时，最终只保留一份结果，但 route 贡献必须都在。
2. 去重键必须优先使用 `document_id + chunk_id`。

这里用了一个很直观的小样本：

1. `doc-1:chunk-1` 同时出现在 dense 和 sparse
2. `doc-2:chunk-1` 只在 dense 出现
3. `doc-3:chunk-1` 只在 sparse 出现

所以最终去重后应该只剩 3 条。

### 为什么要这样写

这类测试的价值不在于覆盖率数字，而在于它把“系统行为契约”钉住了。

以后无论谁改融合策略、改去重逻辑、改 metadata 结构，只要破坏了这些基础行为，测试都会第一时间告诉你。

### 它如何衔接下一步

到这里，整个 L2 功能实现已经闭环。下面就进入验收和演进建议。

---

## 如何验证

建议至少做下面 4 组验证。

### 1. 跑单元测试

在 `backend` 目录执行：

```bash
go test ./internal/milvus/retrieval -run "TestFuseRouteCandidatesAndDedupe|TestBuildDedupeKeyPrefersDocumentAndChunkID"
```

成功信号：

1. 两个测试都通过。
2. 没有因为 metadata 结构变化导致断言失败。

### 2. 看二级流程日志是否出现新的空结果原因

触发几种场景后，观察日志里的这些值：

1. `Empty-After-Retrieve`
2. `Empty-After-Fusion`
3. `Empty-After-Filter`
4. `None`

如果日志里只能看到“有结果/没结果”，但看不到空结果阶段，说明这条 L2 观察链路还没有真正接通。

### 3. 打印一条最终结果的元数据

重点确认这些字段存在且含义正确：

1. `score`
2. `fusion_score`
3. `route`
4. `route_contrib`
5. `route_raw_scores`
6. `source.route`
7. `source.route_contrib`

成功结果通常像这样：

```json
{
  "score": 0.7,
  "fusion_score": 0.7,
  "route": "dense",
  "route_contrib": {
    "dense": 0.7,
    "sparse": 0.3
  },
  "route_raw_scores": {
    "dense": 0.91,
    "sparse": 11.2
  }
}
```

### 4. 验证重复文档块是否真的折叠

找一个同时会被 dense 和 sparse 命中的 query，比如包含强关键词、同时又有明确语义的问题。确认：

1. 去重前两路候选数大于最终结果数。
2. 最终结果里没有相同 `document_id + chunk_id` 的重复项。
3. 但该结果的 `route_contrib` 里能看到两条 route 都出现过。

---

## 取舍与后续优化

这版实现优先解决的是“工程上先稳定可用”，不是“一步到位做到最强融合算法”。

当前它主要优化了这些事情：

1. 让 dense/sparse 有稳定公共分数契约。
2. 让跨 route 比较变得合理，而不是直接比较原始分。
3. 让去重后仍然保留 route 贡献，便于调试和继续演进。

它暂时没有解决的点也很明确：

1. 现在使用的是简单 `min-max` 归一化，不是更复杂的学习排序或 RRF。
2. `mergeFloatMaps` 当前保留的是各 route 的最高贡献，不是求和或更复杂聚合。
3. reranker 仍然是后置阶段，还没有显式利用 `route_contrib` 等更多特征。
4. 现在只支持 dense/sparse 两条 route，未来如果加第三条 route，还需要重新审视 metadata 结构和权重策略。

自然的下一步演进方向通常是：

1. 把 `FusionConfig` 扩展成支持更多融合算法，例如 RRF 或按 query 类型分权重。
2. 让 reranker 显式读取 `fusion_score`、`route_contrib`、`route_raw_scores`。
3. 增加更多测试，覆盖单路失败、全零分、单候选归一化等边界场景。
4. 把 route 贡献和 empty reason 接进更完整的指标面板。

这次 L2 的本质成果，可以用一句最朴素的话来记：

**不是把两堆结果拼起来，而是把两堆结果整理成一份真正可比较、可去重、可调试、可继续优化的统一候选池。**
