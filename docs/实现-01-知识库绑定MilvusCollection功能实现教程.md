# 知识库绑定 Milvus Collection 功能实现教程

## 背景

这次功能的目标很明确：把“业务上的知识库”和“Milvus 里的向量 collection”真正绑定起来。

改造前，系统虽然已经有知识库、文档上传、检索这些能力，但上传、删除、检索默认还是走全局配置里的同一个 collection。不同知识库之间更多是靠文档 metadata 里的 `kb_id` 做过滤，而不是在向量层真正隔离。这样做短期能跑起来，但会带来三个问题：

1. 后台里建了多个知识库，看起来像隔离了，实际向量数据还可能混在一起。
2. 删除一个知识库时，很难把它对应的向量数据一起清理干净。
3. 多知识库检索如果未来真的要跨多个 collection，就必须先把路由能力补起来。

这次实现采用的是“一个知识库绑定一个 Milvus collection”的方案，但没有要求前端手动输入 collection 名，而是由后端自动分配。这样做的好处是改动更小，对已有知识库也更友好。

这类功能最容易让初学者困惑的地方有三个：

1. `knowledge base` 和 `Milvus collection` 不是一个概念。前者是业务实体，后者是向量存储实体。
2. 绑定关系不是只在创建时写一次，而是做成了“懒分配”。也就是说，旧知识库在第一次被读取、上传、检索时，也会自动补齐自己的 `vector_collection`。
3. 一旦不同知识库真的落到不同 collection，多知识库检索就不能再只调一次全局检索器，而是要先按 collection 分组，再合并结果。

## 这篇教程会做什么

看完这篇教程，你应该能从头复现下面这条链路：

1. 在知识库表里增加 `vector_collection` 字段。
2. 为每个知识库生成默认 collection 名，例如 `kb_12_docs`。
3. 在上传、重试、删除文档、删除知识库、检索这些入口里，都优先使用知识库绑定的 collection。
4. 在异步入库消费者里，按 collection 创建真正的 Milvus `IndexerService`。
5. 在多知识库检索时，按 collection 分组检索，再把结果重新合并排序。
6. 在管理后台把绑定关系展示出来，并补一个删除知识库入口，方便验证功能是否真的闭环。

这次实现主要涉及这些文件：

1. `backend/internal/model/kb_knowledge_base.go`
2. `backend/internal/repository/database.go`
3. `backend/internal/milvus/kb_collection_binding.go`
4. `backend/api/handler/kb/knowledge_base_binding.go`
5. `backend/api/handler/kb/handler.go`
6. `backend/api/router/custom_kb.go`
7. `backend/internal/mq/mq.go`
8. `backend/internal/mq/consumer.go`
9. `backend/internal/milvus/retrieval/search.go`
10. `backend/internal/milvus/retrieval/sparse_search.go`
11. `admin/src/types/kb.ts`
12. `admin/src/config/api.ts`
13. `admin/src/components/admin/knowledge-base-provider.tsx`
14. `admin/src/components/admin/knowledge-bases-page.tsx`
15. `admin/src/components/admin/knowledge-base-detail-page.tsx`

## 需要先理解的术语

### 知识库

这里的知识库是业务数据库里的记录，表名是 `kb_knowledge_base`。它描述的是“这一组文档在业务上叫什么、归谁、状态是什么”。

你可以先把它理解成后台里的一个文件夹，只不过它不直接存向量，只存业务信息。

### 向量 collection

collection 是 Milvus 里真正放向量和 metadata 的地方。

比如一个知识库叫“Java 面试题库”，它在业务库里是一条 `kb_knowledge_base` 记录；而它对应的向量数据，可能存放在 `kb_12_docs` 这个 Milvus collection 里。

### 懒分配

懒分配的意思是：不是要求所有旧数据先批量迁移完，系统才能上线，而是在第一次真正用到这个知识库时，再自动补齐 `vector_collection`。

这个设计很重要，因为它让改造可以渐进落地。旧知识库不需要先跑一轮复杂迁移，就能先开始兼容新逻辑。

### IndexerService

`IndexerService` 可以理解成“把切分后的文档块写入 Milvus 的执行器”。

之前系统只有一个默认的 `IndexerService`，它在启动时就绑定了全局 collection。现在如果每个知识库都可能写入不同 collection，就不能一直复用那一个默认实例，而是要按目标 collection 动态创建。

### 跨 collection 聚合

以前检索可以理解成“在一个大仓库里搜，再按 `kb_id` 过滤”。现在改成真正一库一 collection 后，多知识库检索就变成“分别去多个仓库搜，再把结果拼回来重新排”。

这就是跨 collection 聚合。

## 整体流程

先看全链路，再看代码会轻松很多。

1. 管理员创建知识库。
2. 后端创建 `kb_knowledge_base` 记录后，立刻给它分配一个默认 `vector_collection`，例如 `kb_5_docs`。
3. 管理员上传文档时，`UploadDocument` 会先解析出这个知识库对应的 collection，再把它塞进 MQ payload。
4. MQ 消费者拿到 payload 后，按这个 collection 创建 `IndexerService`，把切分后的 chunk 写进正确的 Milvus collection。
5. 检索时，如果只查一个知识库，就直接查它自己的 collection；如果查多个知识库，就先按 collection 分组，再分别检索，最后合并排序。
6. 删除文档时，后端按文档所属知识库找到对应 collection，只删那个 collection 里的向量。
7. 删除知识库时，后端先确认没有活跃入库任务，再删除业务记录、本地文件，最后尝试 `DropCollection`。
8. 前端列表页和详情页展示 `vector_collection`，这样我们能直接看到绑定有没有生效。

## 分步实现

## 第 1 步：给知识库模型增加 `vector_collection`

### 目标

先把绑定关系持久化下来。没有这个字段，后面所有“按知识库路由到 collection”的逻辑都没有落脚点。

### 文件

1. `backend/internal/model/kb_knowledge_base.go`
2. `backend/internal/repository/database.go`

### 完整代码

文件：`backend/internal/model/kb_knowledge_base.go`

```go
package model

import (
	"time"
)

type KBKnowledgeBaseStatus string

const (
	KBKnowledgeBaseStatusActive   KBKnowledgeBaseStatus = "active"
	KBKnowledgeBaseStatusDisabled KBKnowledgeBaseStatus = "disabled"
)

var KBKnowledgeBaseDao _KBKnowledgeBase

type (
	_KBKnowledgeBase struct{}
	KBKnowledgeBase  struct {
		ID               uint64                `json:"id" gorm:"primaryKey;autoIncrement"`
		UserID           uint                  `json:"user_id" gorm:"index;not null"`
		Name             string                `json:"name" gorm:"size:255;not null"`
		Description      string                `json:"description" gorm:"size:1000"`
		VectorCollection string                `json:"vector_collection" gorm:"size:255;index"`
		Status           KBKnowledgeBaseStatus `json:"status" gorm:"size:20;not null;default:'active';index"`
		CreatedAt        time.Time             `json:"created_at" gorm:"autoCreateTime:milli"`
		UpdatedAt        time.Time             `json:"updated_at" gorm:"autoUpdateTime:milli"`
	}
)

func (KBKnowledgeBase) TableName() string {
	return "kb_knowledge_base"
}

func (d *_KBKnowledgeBase) Create(kb *KBKnowledgeBase) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().Create(kb).Error
}

func (d *_KBKnowledgeBase) GetByID(id uint64) (*KBKnowledgeBase, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var kb KBKnowledgeBase
	err := getDB().Where("id = ?", id).First(&kb).Error
	if err != nil {
		return nil, err
	}
	return &kb, nil
}

func (d *_KBKnowledgeBase) GetByName(name string) (*KBKnowledgeBase, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var kb KBKnowledgeBase
	err := getDB().Where("name = ?", name).First(&kb).Error
	if err != nil {
		return nil, err
	}
	return &kb, nil
}

func (d *_KBKnowledgeBase) GetByVectorCollection(collection string) (*KBKnowledgeBase, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var kb KBKnowledgeBase
	err := getDB().Where("vector_collection = ?", collection).First(&kb).Error
	if err != nil {
		return nil, err
	}
	return &kb, nil
}

func (d *_KBKnowledgeBase) List(page, pageSize int) ([]*KBKnowledgeBase, int64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var list []*KBKnowledgeBase
	var total int64

	query := getDB().Model(&KBKnowledgeBase{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Offset((page - 1) * pageSize).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&list).Error
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (d *_KBKnowledgeBase) ListByIDs(ids []uint64) ([]*KBKnowledgeBase, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	if len(ids) == 0 {
		return []*KBKnowledgeBase{}, nil
	}

	var list []*KBKnowledgeBase
	if err := getDB().Where("id IN ?", ids).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (d *_KBKnowledgeBase) ListIDsByStatus(status KBKnowledgeBaseStatus) ([]uint64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}

	ids := make([]uint64, 0)
	query := getDB().Model(&KBKnowledgeBase{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if err := query.Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (d *_KBKnowledgeBase) UpdateByID(id uint64, updates map[string]interface{}) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	if len(updates) == 0 {
		return nil
	}
	return getDB().Model(&KBKnowledgeBase{}).Where("id = ?", id).Updates(updates).Error
}
```

文件：`backend/internal/repository/database.go`

```go
func migrateDatabase() error {
	return DB.AutoMigrate(
		&model.User{},
		&model.UserModel{},
		&model.InterviewRecord{},
		&model.InterviewDialogue{},
		&model.InterviewEvaluation{},
		&model.AnswerReport{},
		&model.Resume{},
		&model.PredictionRecord{},
		&model.PredictionQuestion{},
		&model.PaymentOrder{},
		&model.PaymentAttempt{},
		&model.Subscription{},
		&model.PaymentEvent{},
		&model.PaymentCallback{},
		&model.KBKnowledgeBase{},
		&model.KBDocument{},
		&model.KBIngestJob{},
		&model.KBJobOperationLog{},
		&model.KBIndexRegistry{},
		&model.KBIndexOperationLog{},
		&model.KBRetrieveLog{},
		&model.KBCostTrace{},
		&model.KBAuditEvent{},
		&model.KBEvalDataset{},
		&model.KBEvalCase{},
		&model.KBEvalRun{},
	)
}
```

### 这段代码在做什么

最重要的是这行：

`VectorCollection string                \`json:"vector_collection" gorm:"size:255;index"\``

它告诉我们，每个知识库现在都可以保存自己的目标 collection 名，而且这个字段会被建索引，后面按 collection 反查知识库时也更顺手。

`ListByIDs` 和 `UpdateByID` 这两个 DAO 方法也很关键：

1. `ListByIDs` 用来支持多知识库检索时批量读取知识库记录。
2. `UpdateByID` 用来在“懒分配”时把自动生成的 collection 名写回数据库。

### 为什么要这样做

一个常见但不够稳的做法是：不在业务表里存 `vector_collection`，而是每次用 `kb_id` 临时拼一个名字，比如 `kb_<id>_docs`。这样看起来更简单，但会有两个问题：

1. 以后如果某个知识库需要手动改绑到别的 collection，你没有持久化入口。
2. 你永远不知道数据库里的知识库和向量层到底是不是一一对应。

把绑定关系显式存下来，后面做审计、重建、迁移、删除都会更清楚。

### 它如何衔接下一步

现在我们已经有了“存在哪里”的字段。下一步就要解决“如何生成默认值”和“如何按这个值创建 Milvus 写入器”。

## 第 2 步：给 Milvus 层补上 collection 级别的写入能力

### 目标

让系统既能为知识库生成默认 collection 名，也能按目标 collection 动态创建写入器。

### 文件

`backend/internal/milvus/kb_collection_binding.go`

### 完整代码

文件：`backend/internal/milvus/kb_collection_binding.go`

```go
package milvus

import (
	"context"
	"fmt"
	"strings"

	milvusIndexer "github.com/cloudwego/eino-ext/components/indexer/milvus"

	"interview-agents/internal/milvus/storage"
)

func DefaultKnowledgeBaseCollectionName(kbID uint64) string {
	if kbID == 0 {
		return "kb_unknown_docs"
	}
	return fmt.Sprintf("kb_%d_docs", kbID)
}

func (m *MilvusManager) NewIndexerServiceForCollection(ctx context.Context, collection string) (*storage.IndexerService, error) {
	if m == nil {
		return nil, fmt.Errorf("milvus manager is nil")
	}
	if strings.TrimSpace(collection) == "" {
		return nil, fmt.Errorf("collection name is empty")
	}
	if m.Client == nil {
		return nil, fmt.Errorf("milvus client is nil")
	}
	if m.EmbeddingService == nil {
		return nil, fmt.Errorf("embedding service is nil")
	}
	if m.Config == nil {
		return nil, fmt.Errorf("milvus config is nil")
	}

	indexerConfig := &milvusIndexer.IndexerConfig{
		Client:     m.Client,
		Collection: strings.TrimSpace(collection),
		Embedding:  m.EmbeddingService.GetEmbedder(),
	}
	return storage.NewIndexerServiceWithDimension(ctx, indexerConfig, m.Config.Embedding.Dimensions)
}
```

### 这段代码在做什么

这个文件只有两个函数，但它们是这次改造的基础设施核心。

`DefaultKnowledgeBaseCollectionName` 负责约定默认命名规则。比如知识库 ID 是 `12`，默认 collection 就是 `kb_12_docs`。

`NewIndexerServiceForCollection` 负责按传入的 collection 动态创建一个新的 `IndexerService`。也就是说，真正写入 Milvus 时，不再总是写向默认 collection，而是写向当前知识库绑定的 collection。

### 为什么要这样做

如果我们继续复用 `manager.GetIndexerService()` 返回的默认写入器，那么即使前面已经算出了 `vector_collection`，最后落库时也还是会写回全局 collection，这就前功尽弃了。

你可以把这一步理解成：前面几步只是算出了“应该把货送到哪个仓库”，而这一步才是真正换了一辆能把货送到那个仓库的车。

### 它如何衔接下一步

现在 Milvus 层已经具备“按 collection 写入”的能力。下一步要补的是业务编排层，也就是：谁来给知识库分配 collection，谁来处理多 collection 检索，谁来做删除知识库。

## 第 3 步：在 handler 层建立知识库与 collection 的绑定中枢

### 目标

把这次改造最关键的规则收口到一个独立文件里，不要把“如何给知识库分配 collection”“如何按 collection 分组检索”“如何删除整个知识库”散落到各个 handler 中。

### 文件

`backend/api/handler/kb/knowledge_base_binding.go`

### 完整代码

文件：`backend/api/handler/kb/knowledge_base_binding.go`

```go
package kb

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	"interview-agents/api/response"
	"interview-agents/internal/config"
	myerrors "interview-agents/internal/errors"
	"interview-agents/internal/middleware"
	"interview-agents/internal/milvus"
	"interview-agents/internal/milvus/retrieval"
	"interview-agents/internal/model"
	"interview-agents/internal/rag/experiment"
	"interview-agents/internal/repository"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/hertz/pkg/app"
	"gorm.io/gorm"
)

type kbRetrieveTarget struct {
	Collection string
	KBIDs      []uint64
}

func ensureKnowledgeBaseCollectionAssigned(kb *model.KBKnowledgeBase) (string, error) {
	if kb == nil {
		return "", fmt.Errorf("knowledge base is nil")
	}

	collection := strings.TrimSpace(kb.VectorCollection)
	if collection != "" {
		return collection, nil
	}

	collection = milvus.DefaultKnowledgeBaseCollectionName(kb.ID)
	if err := model.KBKnowledgeBaseDao.UpdateByID(kb.ID, map[string]interface{}{
		"vector_collection": collection,
	}); err != nil {
		return "", err
	}
	kb.VectorCollection = collection
	return collection, nil
}

func resolveKnowledgeBaseCollectionByID(kbID uint64) string {
	if kbID == 0 {
		return strings.TrimSpace(config.Global.Milvus.GetCollection("knowledge"))
	}

	kb, err := model.KBKnowledgeBaseDao.GetByID(kbID)
	if err == nil && kb != nil {
		collection, assignErr := ensureKnowledgeBaseCollectionAssigned(kb)
		if assignErr == nil {
			return collection
		}
	}

	fallback := strings.TrimSpace(config.Global.Milvus.GetCollection("knowledge"))
	if fallback == "" {
		fallback = strings.TrimSpace(config.Global.Milvus.CollectionName)
	}
	return fallback
}

func buildKnowledgeBaseRetrieveTargets(kbIDs []uint64) ([]kbRetrieveTarget, string, error) {
	if len(kbIDs) == 0 {
		return nil, "", nil
	}

	kbs, err := model.KBKnowledgeBaseDao.ListByIDs(kbIDs)
	if err != nil {
		return nil, "", err
	}

	byID := make(map[uint64]*model.KBKnowledgeBase, len(kbs))
	for _, kb := range kbs {
		if kb != nil {
			byID[kb.ID] = kb
		}
	}

	targets := make([]kbRetrieveTarget, 0)
	byCollection := make(map[string]int)
	collections := make([]string, 0)
	for _, kbID := range kbIDs {
		kb := byID[kbID]
		if kb == nil {
			continue
		}
		collection, err := ensureKnowledgeBaseCollectionAssigned(kb)
		if err != nil {
			return nil, "", err
		}
		if index, ok := byCollection[collection]; ok {
			targets[index].KBIDs = append(targets[index].KBIDs, kbID)
			continue
		}
		byCollection[collection] = len(targets)
		targets = append(targets, kbRetrieveTarget{
			Collection: collection,
			KBIDs:      []uint64{kbID},
		})
		collections = append(collections, collection)
	}

	if len(targets) == 0 {
		return nil, "", nil
	}

	label := targets[0].Collection
	if len(collections) > 1 {
		label = "multi:" + strings.Join(collections, ",")
	}
	return targets, label, nil
}

func searchKnowledgeBaseTarget(
	ctx context.Context,
	manager *milvus.MilvusManager,
	retrieverService *retrieval.RetrieverService,
	useHybrid bool,
	query string,
	topK int,
	target kbRetrieveTarget,
	requestID string,
	queryType string,
	experimentDecision experiment.Decision,
) (*retrieval.SearchResult, error) {
	expr := buildKBFilterExpr(target.KBIDs)
	activeKBID := uint64(0)
	if len(target.KBIDs) == 1 {
		activeKBID = target.KBIDs[0]
	}

	if useHybrid {
		searchOpts := &retrieval.RetrieveOptions{
			TopK:             topK,
			Collection:       target.Collection,
			Expr:             expr,
			KBScope:          "global",
			ActiveGlobalKBID: activeKBID,
			RequestID:        requestID,
			OriginalQuery:    query,
			QueryType:        queryType,
			ExperimentID:     experimentDecision.ExperimentID,
			StrategyVersion:  experimentDecision.CandidateVersion,
			ReleaseID:        experimentDecision.ExperimentID,
		}
		if experimentDecision.Matched {
			searchOpts.ForceRewriteOff = experimentDecision.Override.ForceRewriteOff
			if experimentDecision.Override.CandidateTopK > 0 {
				searchOpts.CandidateTopK = experimentDecision.Override.CandidateTopK
			}
		}
		return manager.GetHybridRetriever().SearchWithMetrics(ctx, query, searchOpts)
	}

	searchOpts := &retrieval.RetrieveOptions{
		TopK:             topK,
		Collection:       target.Collection,
		Expr:             expr,
		KBScope:          "global",
		ActiveGlobalKBID: activeKBID,
	}
	return retrieverService.RetrieveWithOptionsAndMetrics(ctx, query, searchOpts)
}

func mergeKnowledgeBaseSearchResults(results []*retrieval.SearchResult, collectionLabel string, topK int, useHybrid bool) *retrieval.SearchResult {
	merged := &retrieval.SearchResult{
		Documents: []*schema.Document{},
	}
	if len(results) == 0 {
		return merged
	}

	merged.Metrics = results[0].Metrics
	merged.Metrics.CollectionVersion = collectionLabel
	merged.Metrics.EmbeddingMs = 0
	merged.Metrics.SearchMs = 0
	merged.Metrics.PostprocessMs = 0
	merged.Metrics.RerankMs = 0
	merged.Metrics.HitCount = 0
	merged.Metrics.TruncatedCount = 0
	merged.Metrics.DenseHits = 0
	merged.Metrics.SparseHits = 0
	merged.Metrics.DenseContribution = 0
	merged.Metrics.SparseContribution = 0

	candidateTopK := 0
	docs := make([]*schema.Document, 0)
	for _, result := range results {
		if result == nil {
			continue
		}
		docs = append(docs, result.Documents...)
		merged.Metrics.EmbeddingMs += result.Metrics.EmbeddingMs
		merged.Metrics.SearchMs += result.Metrics.SearchMs
		merged.Metrics.PostprocessMs += result.Metrics.PostprocessMs
		merged.Metrics.RerankMs += result.Metrics.RerankMs
		merged.Metrics.HitCount += result.Metrics.HitCount
		merged.Metrics.TruncatedCount += result.Metrics.TruncatedCount
		merged.Metrics.DenseHits += result.Metrics.DenseHits
		merged.Metrics.SparseHits += result.Metrics.SparseHits
		merged.Metrics.DenseContribution += result.Metrics.DenseContribution
		merged.Metrics.SparseContribution += result.Metrics.SparseContribution
		if result.Metrics.CandidateTopK > candidateTopK {
			candidateTopK = result.Metrics.CandidateTopK
		}
	}

	sort.SliceStable(docs, func(i, j int) bool {
		return readRetrieveDocScore(docs[i]) > readRetrieveDocScore(docs[j])
	})
	if topK > 0 && len(docs) > topK {
		docs = docs[:topK]
	}

	merged.Documents = docs
	merged.Metrics.CandidateTopK = candidateTopK
	merged.Metrics.FinalTopK = len(docs)
	if merged.Metrics.RetrieverVersion == "" {
		if useHybrid {
			merged.Metrics.RetrieverVersion = retrieval.HybridRetrieverVersion
		} else {
			merged.Metrics.RetrieverVersion = retrieval.DenseRetrieverVersion
		}
	}
	return merged
}

func readRetrieveDocScore(doc *schema.Document) float64 {
	if doc == nil {
		return 0
	}
	if score := getFloat64Metadata(doc.MetaData, "rerank_score"); score > 0 {
		return score
	}
	return getFloat64Metadata(doc.MetaData, "score")
}

func DeleteKnowledgeBase(ctx context.Context, c *app.RequestContext) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(ctx, c, "Authorization token is required")
		return
	}

	kbID, err := parseUint64(c.Param("kb_id"), "kb_id")
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}

	kb, err := mustKnowledgeBaseExist(kbID)
	if err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}

	collection, err := ensureKnowledgeBaseCollectionAssigned(kb)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to resolve knowledge base collection", err))
		return
	}

	activeStatuses := []model.KBIngestJobStatus{
		model.KBIngestJobStatusPending,
		model.KBIngestJobStatusProcessing,
		model.KBIngestJobStatusRetrying,
	}
	var activeJobs int64
	if err := repository.GetDB().
		Model(&model.KBIngestJob{}).
		Where("kb_id = ? AND status IN ?", kbID, activeStatuses).
		Count(&activeJobs).Error; err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to inspect knowledge base jobs", err))
		return
	}
	if activeJobs > 0 {
		response.BadRequest(ctx, c, "knowledge base has active ingest jobs, cancel or wait for them first")
		return
	}

	docs := make([]*model.KBDocument, 0)
	if err := repository.GetDB().Where("kb_id = ?", kbID).Find(&docs).Error; err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to load knowledge base documents", err))
		return
	}

	jobIDs := make([]uint64, 0)
	if err := repository.GetDB().Model(&model.KBIngestJob{}).Where("kb_id = ?", kbID).Pluck("id", &jobIDs).Error; err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to load knowledge base jobs", err))
		return
	}

	if err := repository.GetDB().Transaction(func(tx *gorm.DB) error {
		if len(jobIDs) > 0 {
			if err := tx.Where("job_id IN ?", jobIDs).Delete(&model.KBJobOperationLog{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("kb_id = ?", kbID).Delete(&model.KBIngestJob{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.KBDocument{}).
			Where("kb_id = ?", kbID).
			Updates(map[string]interface{}{
				"deleted":   1,
				"status":    model.KBDocumentStatusFailed,
				"error_msg": "knowledge base deleted",
			}).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ?", kbID).Delete(&model.KBKnowledgeBase{}).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to delete knowledge base", err))
		return
	}

	for _, doc := range docs {
		if doc == nil || strings.TrimSpace(doc.StoragePath) == "" {
			continue
		}
		if err := os.Remove(doc.StoragePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("[KB Delete] failed to remove source file: kb_id=%d document_id=%d path=%s err=%v", kbID, doc.ID, doc.StoragePath, err)
		}
	}

	if config.Global.RAG.Enabled {
		if manager, getErr := milvus.GetMilvusManager(); getErr == nil && manager.GetClient() != nil && collection != "" {
			if exists, existsErr := manager.GetClient().HasCollection(ctx, collection); existsErr != nil {
				log.Printf("[KB Delete] failed to inspect collection: kb_id=%d collection=%s err=%v", kbID, collection, existsErr)
			} else if exists {
				if err := manager.GetClient().DropCollection(ctx, collection); err != nil {
					log.Printf("[KB Delete] failed to drop collection: kb_id=%d collection=%s err=%v", kbID, collection, err)
				}
			}
		}
	}

	persistAuditEvent(&model.KBAuditEvent{
		AuditTraceID: fmt.Sprintf("audit-kb-delete-%d", kbID),
		OperatorID:   userID,
		UserID:       userID,
		KBID:         kbID,
		Action:       "knowledge_base_delete",
		ResourceType: "knowledge_base",
		ResourceID:   strconv.FormatUint(kbID, 10),
		Result:       "deleted",
		Reason:       getOperationReason(c),
		AfterData:    fmt.Sprintf(`{"vector_collection":"%s"}`, collection),
	})

	response.Success(ctx, c, map[string]interface{}{
		"kb_id":             kbID,
		"deleted":           true,
		"vector_collection": collection,
	})
}
```

### 这段代码在做什么

这一整层代码解决了三个核心问题。

第一，`ensureKnowledgeBaseCollectionAssigned` 实现了懒分配。如果一个知识库的 `vector_collection` 还是空，就按 `kb_<id>_docs` 自动生成，并立刻回写数据库。

第二，`buildKnowledgeBaseRetrieveTargets` 把多个知识库按 collection 分组。比如：

1. `kb_id=1` 和 `kb_id=2` 都绑定到 `kb_java_docs`
2. `kb_id=3` 绑定到 `kb_go_docs`

那么检索目标就会被整理成两组，而不是三次。

第三，`DeleteKnowledgeBase` 实现了真正的闭环删除。它不只是删业务表记录，还会：

1. 检查有没有活跃入库任务。
2. 删任务日志、任务记录、文档记录。
3. 删除本地源文件。
4. 最后去 Milvus 里 `DropCollection`。

### 为什么要这样做

如果把这些逻辑散到 `CreateKnowledgeBase`、`UploadDocument`、`Retrieve`、`DeleteDocument` 里，后面一定会出现两个问题：

1. 同样的“取 collection”逻辑在不同入口写出不同版本。
2. 多知识库检索和删除知识库这种跨层场景会越来越难改。

单独抽一个绑定文件，本质上是在给这个新能力建立“唯一真相来源”。

### 它如何衔接下一步

现在我们已经有了统一的绑定规则。下一步就要把这些规则真正接到已有入口上，也就是创建、上传、重试、删除、检索这些具体 API。

## 第 4 步：把创建、上传、重试、删除、检索都接到新绑定逻辑

### 目标

让所有真正会读写向量库的入口，都不再依赖全局默认 collection。

### 文件

1. `backend/api/handler/kb/handler.go`
2. `backend/api/router/custom_kb.go`
3. `backend/internal/mq/mq.go`
4. `backend/internal/mq/consumer.go`
5. `backend/internal/milvus/retrieval/search.go`
6. `backend/internal/milvus/retrieval/sparse_search.go`

### 完整代码

文件：`backend/api/router/custom_kb.go`

```go
func registerKBGroup(group *route.RouterGroup, adminOnly bool) {
	group.GET("/dashboard/stats", kb.GetDashboardStats)
	group.POST("/bases", kb.CreateKnowledgeBase)
	group.GET("/bases", kb.ListKnowledgeBases)
	group.DELETE("/bases/:kb_id", kb.DeleteKnowledgeBase)
	group.POST("/documents/upload", kb.UploadDocument)
	group.GET("/documents", kb.ListDocuments)
	group.GET("/jobs", kb.ListJobs)
	group.GET("/jobs/:job_id", kb.GetJob)
	group.POST("/jobs/:job_id/retry", kb.RetryJob)
	group.POST("/jobs/:job_id/cancel", kb.CancelJob)
	group.DELETE("/documents/:document_id", kb.DeleteDocument)
	group.POST("/retrieve", kb.Retrieve)
	group.GET("/retrieve/audit/:request_id", kb.GetRetrieveAuditLog)
	group.GET(phase3.LegacyRetrievalDebugRoute, kb.GetRetrieveDebugView)
	group.GET(phase3.RetrievalDebugRoute, kb.GetRetrieveDebugView)
	group.GET("/retrieve/audit", kb.ListRetrieveAuditLogs)
	group.GET("/metrics/overview", kb.GetMetricsOverview)
	group.GET("/logs/ingest", kb.ListIngestLogs)
	group.GET("/logs/ingest/:job_id", kb.GetIngestLogDetail)
	group.POST("/ingest/pause", kb.PauseIngest)
	group.POST("/ingest/resume", kb.ResumeIngest)
	group.GET("/ingest/status", kb.GetIngestStatus)
}
```

文件：`backend/api/handler/kb/handler.go`

```go
func CreateKnowledgeBase(ctx context.Context, c *app.RequestContext) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(ctx, c, "Authorization token is required")
		return
	}

	var req createKnowledgeBaseRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.BadRequest(ctx, c, "Invalid request: "+err.Error())
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if req.Name == "" {
		response.BadRequest(ctx, c, "name is required")
		return
	}

	existing, err := model.KBKnowledgeBaseDao.GetByName(req.Name)
	if err == nil && existing != nil {
		response.BadRequest(ctx, c, "knowledge base name already exists")
		return
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to check knowledge base", err))
		return
	}

	kb := &model.KBKnowledgeBase{
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		Status:      model.KBKnowledgeBaseStatusActive,
	}
	if err := model.KBKnowledgeBaseDao.Create(kb); err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to create knowledge base", err))
		return
	}
	if _, err := ensureKnowledgeBaseCollectionAssigned(kb); err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to assign knowledge base collection", err))
		return
	}

	response.Success(ctx, c, kb)
}
```

```go
func ListKnowledgeBases(ctx context.Context, c *app.RequestContext) {
	if middleware.GetUserID(c) == 0 {
		response.Unauthorized(ctx, c, "Authorization token is required")
		return
	}

	page, pageSize := getPagination(c)
	items, total, err := model.KBKnowledgeBaseDao.List(page, pageSize)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to list knowledge bases", err))
		return
	}
	for _, item := range items {
		if _, err := ensureKnowledgeBaseCollectionAssigned(item); err != nil {
			response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to resolve knowledge base collection", err))
			return
		}
	}

	response.Success(ctx, c, knowledgeBaseListResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}
```

```go
func UploadDocument(ctx context.Context, c *app.RequestContext) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(ctx, c, "Authorization token is required")
		return
	}

	kbID, err := parseUint64(c.PostForm("kb_id"), "kb_id")
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}

	kb, err := mustKnowledgeBaseExist(kbID)
	if err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}
	collection, err := ensureKnowledgeBaseCollectionAssigned(kb)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to resolve knowledge base collection", err))
		return
	}

	fileHeader, err := c.FormFile(knowledgeUploadFormKey)
	if err != nil || fileHeader == nil {
		response.BadRequest(ctx, c, "file is required")
		return
	}

	fileName := filepath.Base(fileHeader.Filename)
	fileType, err := validateKnowledgeFile(fileHeader)
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}

	content, fileHash, err := readKnowledgeFile(fileHeader)
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}

	existingDoc, err := model.KBDocumentDao.GetByFileHash(kbID, fileHash)
	if err == nil && existingDoc != nil {
		jobID := uint64(0)
		if job, jobErr := model.KBIngestJobDao.GetLatestByDocumentID(existingDoc.ID); jobErr == nil && job != nil {
			jobID = job.ID
		}
		response.Success(ctx, c, uploadDocumentResponse{
			DocumentID: existingDoc.ID,
			JobID:      jobID,
			Status:     string(existingDoc.Status),
			Reused:     true,
		})
		return
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to check duplicate document", err))
		return
	}

	ossClient, err := getKnowledgeOSS()
	if err != nil {
		response.InternalServerError(ctx, c, "failed to initialize knowledge storage")
		return
	}

	objectKey := buildKnowledgeObjectKey(kbID, fileName)
	storagePath, err := ossClient.PutObject(ctx, objectKey, bytes.NewReader(content), int64(len(content)), fileHeader.Header.Get("Content-Type"))
	if err != nil {
		response.InternalServerError(ctx, c, "failed to save document")
		return
	}

	doc := &model.KBDocument{
		KbID:        kbID,
		UserID:      userID,
		FileName:    fileName,
		FileType:    fileType,
		FileSize:    int64(len(content)),
		FileHash:    fileHash,
		StoragePath: storagePath,
		Status:      model.KBDocumentStatusPending,
	}
	job := &model.KBIngestJob{
		KbID:       kbID,
		DocumentID: 0,
		UserID:     userID,
		Status:     model.KBIngestJobStatusPending,
	}

	if err := repository.GetDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(doc).Error; err != nil {
			return err
		}
		job.DocumentID = doc.ID
		if err := tx.Create(job).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		_ = os.Remove(storagePath)
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to create ingest job", err))
		return
	}

	publishErr := mq.PublishKnowledgeIngest(ctx, mq.KnowledgeIngestPayload{
		UserID:          userID,
		OperatorAdminID: userID,
		KBID:            kbID,
		DocumentID:      doc.ID,
		JobID:           job.ID,
		FilePath:        storagePath,
		FileType:        fileType,
		Collection:      collection,
	})
	if publishErr != nil {
		log.Printf("[KB Upload] failed to publish ingest message: job_id=%d document_id=%d kb_id=%d user_id=%d err=%v",
			job.ID, doc.ID, kbID, userID, publishErr)
		errMsg := "failed to enqueue ingest task: " + publishErr.Error()
		_ = model.KBIngestJobDao.UpdateFailureState(
			job.ID,
			model.KBIngestJobStatusFailed,
			errMsg,
			"enqueue_error",
			errMsg,
			nil,
			false,
		)
		_ = model.KBDocumentDao.UpdateStatus(doc.ID, model.KBDocumentStatusFailed, errMsg)
		response.InternalServerError(ctx, c, "failed to enqueue ingest task")
		return
	}

	response.Success(ctx, c, uploadDocumentResponse{
		DocumentID: doc.ID,
		JobID:      job.ID,
		Status:     string(job.Status),
	})
}
```

```go
func RetryJob(ctx context.Context, c *app.RequestContext) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(ctx, c, "Authorization token is required")
		return
	}

	jobID, err := parseUint64(c.Param("job_id"), "job_id")
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}

	job, err := model.KBIngestJobDao.GetByID(jobID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(ctx, c, "job not found")
			return
		}
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to get job", err))
		return
	}
	if job.Status != model.KBIngestJobStatusFailed && job.Status != model.KBIngestJobStatusDead {
		response.BadRequest(ctx, c, "manual retry only allowed for failed/dead jobs")
		return
	}

	doc, err := model.KBDocumentDao.GetByID(job.DocumentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(ctx, c, "document not found")
			return
		}
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to get document", err))
		return
	}
	reason := getOperationReason(c)
	transitioned, err := model.KBIngestJobDao.MarkRetrying(jobID, userID, reason)
	if err != nil {
		if errors.Is(err, model.ErrInvalidKBIngestJobTransition) {
			response.BadRequest(ctx, c, "job status transition is invalid")
			return
		}
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to retry job", err))
		return
	}
	if !transitioned {
		response.BadRequest(ctx, c, "job status transition is invalid")
		return
	}

	if err := mq.PublishKnowledgeIngest(ctx, mq.KnowledgeIngestPayload{
		UserID:          userID,
		OperatorAdminID: userID,
		KBID:            job.KbID,
		DocumentID:      job.DocumentID,
		JobID:           job.ID,
		FilePath:        doc.StoragePath,
		FileType:        doc.FileType,
		Collection:      firstNonEmptyString(resolveKnowledgeBaseCollectionByID(job.KbID), ""),
	}); err != nil {
		errMsg := "failed to enqueue retry task: " + err.Error()
		_, _ = model.KBIngestJobDao.UpdateStatusFrom(jobID, model.KBIngestJobStatusFailed, errMsg, model.KBIngestJobStatusRetrying)
		_ = model.KBDocumentDao.UpdateStatus(doc.ID, model.KBDocumentStatusFailed, errMsg)
		response.InternalServerError(ctx, c, "failed to enqueue retry task")
		return
	}

	_ = model.KBDocumentDao.UpdateStatus(doc.ID, model.KBDocumentStatusPending, "")
	updatedJob, getErr := model.KBIngestJobDao.GetByID(jobID)
	if getErr != nil {
		response.Success(ctx, c, map[string]interface{}{
			"job_id":  jobID,
			"status":  string(model.KBIngestJobStatusRetrying),
			"message": "retry accepted",
		})
		return
	}
	response.Success(ctx, c, updatedJob)
}
```

```go
func DeleteDocument(ctx context.Context, c *app.RequestContext) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(ctx, c, "Authorization token is required")
		return
	}

	documentID, err := parseUint64(c.Param("document_id"), "document_id")
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}

	doc, err := model.KBDocumentDao.GetByID(documentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(ctx, c, "document not found")
			return
		}
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to get document", err))
		return
	}
	if err := model.KBDocumentDao.SoftDelete(documentID); err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to delete document", err))
		return
	}

	if config.Global.RAG.Enabled {
		if manager, err := milvus.GetMilvusManager(); err == nil {
			collection := resolveKnowledgeBaseCollectionByID(doc.KbID)
			if err := manager.DeleteDocumentVectors(ctx, collection, documentID); err != nil {
				log.Printf("[KB Delete] failed to delete vectors from Milvus: document_id=%d collection=%s err=%v", documentID, collection, err)
			}
		}
	}

	response.Success(ctx, c, map[string]interface{}{
		"document_id": documentID,
		"deleted":     true,
	})
}
```

```go
topK := clampTopK(req.TopK)
activeKBIDs, err := model.KBKnowledgeBaseDao.ListIDsByStatus(model.KBKnowledgeBaseStatusActive)
if err != nil {
	metricsStatus = "error"
	metricsErrorCode = "db_error"
	response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to list active knowledge bases", err))
	return
}

kbIDs := resolveRetrieveKBIDs(req, activeKBIDs)
if len(kbIDs) == 0 {
	response.Success(ctx, c, retrieveResponse{RequestID: requestID, Items: []retrieveItem{}})
	return
}
experimentDecision := experiment.Decide(&config.Global, userID, middleware.GetUserRole(c), kbIDs, req.Query, requestID, topK)
queryType := firstNonEmptyString(experimentDecision.Override.QueryType, "general")

targets, collection, err := buildKnowledgeBaseRetrieveTargets(kbIDs)
if err != nil {
	metricsStatus = "error"
	metricsErrorCode = "db_error"
	response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to resolve knowledge base collections", err))
	return
}
retrieveTimeout := resolveRetrieveTimeout()
retrieveCtx, cancel := context.WithTimeout(ctx, retrieveTimeout)
defer cancel()

start := time.Now()
var (
	searchResult *retrieval.SearchResult
	searchErr    error
)
if len(targets) == 1 {
	searchResult, searchErr = searchKnowledgeBaseTarget(
		retrieveCtx,
		manager,
		retriever,
		useHybrid,
		req.Query,
		topK,
		targets[0],
		requestID,
		queryType,
		experimentDecision,
	)
} else {
	targetResults := make([]*retrieval.SearchResult, 0, len(targets))
	for _, target := range targets {
		result, err := searchKnowledgeBaseTarget(
			retrieveCtx,
			manager,
			retriever,
			useHybrid,
			req.Query,
			topK,
			target,
			requestID,
			queryType,
			experimentDecision,
		)
		if err != nil {
			searchErr = err
			break
		}
		targetResults = append(targetResults, result)
	}
	if searchErr == nil {
		searchResult = mergeKnowledgeBaseSearchResults(targetResults, collection, topK, useHybrid)
	}
}
durationMs := time.Since(start).Milliseconds()
if searchResult == nil {
	searchResult = &retrieval.SearchResult{}
}
searchResult.Metrics.Strategy = releaseDecision.Strategy
searchResult.Metrics.ReleaseStage = releaseDecision.Stage
searchResult.Metrics.ReleaseReason = releaseDecision.Reason
searchResult.Metrics.QueryType = queryType
searchResult.Metrics.ExperimentID = experimentDecision.ExperimentID
searchResult.Metrics.StrategyVersion = searchResult.Metrics.StrategyVersion
switch experimentDecision.Group {
case experiment.GroupCandidate, experiment.GroupShadow:
	searchResult.Metrics.StrategyVersion = firstNonEmptyString(experimentDecision.CandidateVersion, searchResult.Metrics.StrategyVersion)
case experiment.GroupBaseline:
	searchResult.Metrics.StrategyVersion = firstNonEmptyString(experimentDecision.BaselineVersion, searchResult.Metrics.StrategyVersion)
}
searchResult.Metrics.ReleaseID = firstNonEmptyString(experimentDecision.ExperimentID, searchResult.Metrics.ReleaseID)
searchResult.Metrics.ExperimentGroup = experimentDecision.Group
searchResult.Metrics.CollectionVersion = firstNonEmptyString(searchResult.Metrics.CollectionVersion, collection)
if searchResult.Metrics.RetrieverVersion == "" {
	if useHybrid {
		searchResult.Metrics.RetrieverVersion = retrieval.HybridRetrieverVersion
	} else {
		searchResult.Metrics.RetrieverVersion = retrieval.DenseRetrieverVersion
	}
}
```

文件：`backend/internal/mq/mq.go`

```go
type KnowledgeIngestPayload struct {
	UserID          uint   `json:"user_id"`
	OperatorAdminID uint   `json:"operator_admin_id,omitempty"`
	KBID            uint64 `json:"kb_id"`
	DocumentID      uint64 `json:"document_id"`
	JobID           uint64 `json:"job_id"`
	FilePath        string `json:"file_path"`
	FileType        string `json:"file_type"`
	Collection      string `json:"collection,omitempty"`
}
```

文件：`backend/internal/mq/consumer.go`

```go
func ingestKnowledgeDocument(ctx context.Context, payload KnowledgeIngestPayload) (int, error) {
	rawText, err := extractKnowledgeRawText(ctx, payload.FilePath, payload.FileType)
	if err != nil {
		return 0, buildKnowledgeIngestError(knowledgeIngestErrorTypeParse, "failed to extract source text", err)
	}

	manager, err := milvus.GetMilvusManager()
	if err != nil {
		return 0, buildKnowledgeIngestError(knowledgeIngestErrorTypeMilvus, "failed to get milvus manager", err)
	}
	if manager.GetSplitterService() == nil || manager.GetIndexerService() == nil {
		return 0, buildKnowledgeIngestError(knowledgeIngestErrorTypeMilvus, "milvus services are not initialized", nil)
	}

	docRecord, err := model.KBDocumentDao.GetByID(payload.DocumentID)
	if err != nil {
		return 0, buildKnowledgeIngestError(knowledgeIngestErrorTypeUnknown, "failed to load source document", err)
	}

	collection, err := resolveKnowledgeBaseCollectionForIngest(payload.KBID, payload.Collection)
	if err != nil {
		return 0, buildKnowledgeIngestError(knowledgeIngestErrorTypeMilvus, "failed to resolve knowledge base collection", err)
	}

	baseMeta := milvus.NewKBDocumentMetadata(payload.OperatorAdminID, payload.KBID, payload.DocumentID, docRecord.FileName)
	baseMeta.Extra["collection"] = collection
	doc := &schema.Document{
		Content:  rawText,
		MetaData: baseMeta.ToMap(),
	}
	chunks, err := manager.GetSplitterService().Split(ctx, []*schema.Document{doc})
	if err != nil {
		errorCode := classifyKnowledgeIngestError(err)
		return 0, buildKnowledgeIngestError(errorCode, "failed to split knowledge document", err)
	}
	if len(chunks) == 0 {
		return 0, buildKnowledgeIngestError(knowledgeIngestErrorTypeParse, "empty chunks after split", nil)
	}

	totalChunks := len(chunks)
	for i, chunk := range chunks {
		if chunk == nil {
			continue
		}
		if chunk.ID == "" {
			chunk.ID = fmt.Sprintf("kb_%d_doc_%d_chunk_%d_%d", payload.KBID, payload.DocumentID, i, time.Now().UnixNano())
		}
	}

	indexerService := manager.GetIndexerService()
	if strings.TrimSpace(collection) != "" {
		indexerService, err = manager.NewIndexerServiceForCollection(ctx, collection)
		if err != nil {
			return 0, buildKnowledgeIngestError(knowledgeIngestErrorTypeMilvus, "failed to create collection-specific indexer", err)
		}
	}

	if _, err := indexerService.Store(ctx, chunks); err != nil {
		errorCode := classifyKnowledgeIngestError(err)
		return 0, buildKnowledgeIngestError(errorCode, "failed to store chunks to milvus", err)
	}

	return totalChunks, nil
}
```

```go
func resolveKnowledgeBaseCollectionForIngest(kbID uint64, preferred string) (string, error) {
	collection := strings.TrimSpace(preferred)
	if collection != "" {
		return collection, nil
	}

	kb, err := model.KBKnowledgeBaseDao.GetByID(kbID)
	if err != nil {
		return "", err
	}

	collection = strings.TrimSpace(kb.VectorCollection)
	if collection != "" {
		return collection, nil
	}

	collection = milvus.DefaultKnowledgeBaseCollectionName(kbID)
	if err := model.KBKnowledgeBaseDao.UpdateByID(kbID, map[string]interface{}{
		"vector_collection": collection,
	}); err != nil {
		return "", err
	}
	return collection, nil
}

func resolveKnowledgeBaseCollectionNameForRetry(kbID uint64) string {
	collection, err := resolveKnowledgeBaseCollectionForIngest(kbID, "")
	if err != nil {
		return ""
	}
	return collection
}
```

文件：`backend/internal/milvus/retrieval/search.go`

```go
if i < len(result.Scores) {
	doc.MetaData["score"] = result.Scores[i]
}
doc.MetaData["retriever_version"] = DenseRetrieverVersion
if collectionName != "" {
	doc.MetaData["collection"] = collectionName
}
source := ensureSourceMetadata(doc)
source["route"] = routeDense
source["retriever_version"] = DenseRetrieverVersion
if collectionName != "" {
	source["collection"] = collectionName
}
doc.MetaData["source"] = source
annotateParentChildSource(doc)
```

文件：`backend/internal/milvus/retrieval/sparse_search.go`

```go
if doc.MetaData == nil {
	doc.MetaData = make(map[string]interface{})
}
doc.MetaData["route"] = "sparse"
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
```

```go
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
```

### 这段代码在做什么

这一大步其实是在把“绑定关系”从概念变成真正的运行时行为。

`CreateKnowledgeBase` 和 `ListKnowledgeBases` 负责把新知识库和老知识库都补齐 `vector_collection`。这样即使数据库里存在历史记录，也不会因为字段为空而走不通后面的流程。

`UploadDocument` 现在不只是创建文档和任务，还会先算出 `collection`，然后把它写入 `KnowledgeIngestPayload`。

`RetryJob` 做了同样的事情。这里很容易漏掉，因为很多人只改首次上传，忘了重试链路。结果就是第一次上传写到了正确 collection，但重试时又掉回全局 collection。这次实现专门把这个坑补上了。

`DeleteDocument` 不再使用全局默认 collection，而是根据文档所属的 `kb_id` 反查知识库自己的 collection，然后只删除那个 collection 里的向量。

检索部分最关键。以前系统默认只要拿一个全局 collection，再拼过滤条件就够了。现在变成：

1. 先把请求中的多个知识库 ID 解析出来。
2. 用 `buildKnowledgeBaseRetrieveTargets` 按 collection 分组。
3. 如果只有一个目标 collection，就走一次搜索。
4. 如果有多个目标 collection，就分别搜索，再用 `mergeKnowledgeBaseSearchResults` 合并。

最后，在 `search.go` 和 `sparse_search.go` 里把 `collection` 写进返回文档的 metadata 和 `source`，这样我们后面调试时能知道每条结果到底来自哪个 collection。

### 为什么要这样做

这里有两个很重要的设计点。

第一，MQ payload 里显式带上 `Collection`。看起来我们在消费者里也能根据 `kb_id` 再查一次，为什么还要多带一个字段？原因是：

1. 它让生产者和消费者对“本次任务打算写到哪里”达成同一个上下文。
2. 出问题时查消息和日志会更直观。
3. 重试、补偿、异步链路里不容易因为状态变化而产生歧义。

第二，多知识库检索不是把所有 KB 一股脑塞给一个检索器，而是先按 collection 分组。更简单的做法当然是“还是搜全局 collection，再加过滤”，但那样就没有真正利用这次一库一 collection 的隔离成果。

### 它如何衔接下一步

到这里，后端已经真正具备“一个知识库绑定一个 collection”的执行能力。下一步只是把这个状态展示到前端，让管理员能看见、能删、能验证。

## 第 5 步：把绑定信息和删除入口暴露到前端

### 目标

让后台用户能直接看到每个知识库绑定了哪个 collection，并能触发删除知识库。

如果你只关心后端，这一步可以晚一点做；但从交付角度看，没有可见性就很难验证功能是否真的生效。

### 文件

1. `admin/src/types/kb.ts`
2. `admin/src/config/api.ts`
3. `admin/src/components/admin/knowledge-base-provider.tsx`
4. `admin/src/components/admin/knowledge-bases-page.tsx`
5. `admin/src/components/admin/knowledge-base-detail-page.tsx`

### 完整代码

文件：`admin/src/types/kb.ts`

```ts
export interface KnowledgeBase {
  id: number;
  name: string;
  description?: string;
  vector_collection?: string;
  status: string;
  created_at: string;
  updated_at: string;
}
```

文件：`admin/src/config/api.ts`

```ts
export const KB_ADMIN_API = {
  DASHBOARD_STATS: `${API_BASE_URL}/admin/kb/dashboard/stats`,
  METRICS_OVERVIEW: `${API_BASE_URL}/admin/kb/metrics/overview`,

  CREATE_BASE: `${API_BASE_URL}/admin/kb/bases`,
  LIST_BASES: `${API_BASE_URL}/admin/kb/bases`,
  DELETE_BASE: (id: number | string) => `${API_BASE_URL}/admin/kb/bases/${id}`,

  UPLOAD_DOCUMENT: `${API_BASE_URL}/admin/kb/documents/upload`,
  LIST_DOCUMENTS: `${API_BASE_URL}/admin/kb/documents`,
  DELETE_DOCUMENT: (id: number | string) => `${API_BASE_URL}/admin/kb/documents/${id}`,
}
```

文件：`admin/src/components/admin/knowledge-base-provider.tsx`

```ts
type KnowledgeBaseContextValue = {
  bases: KnowledgeBase[];
  selectedBase: KnowledgeBase | null;
  isLoading: boolean;
  error: string | null;
  isPermissionDenied: boolean;
  refreshBases: () => Promise<KnowledgeBase[]>;
  createBase: (payload: CreateKnowledgeBasePayload) => Promise<KnowledgeBase>;
  deleteBase: (id: number) => Promise<void>;
  setSelectedBaseId: (id?: number | null) => void;
};
```

```ts
const deleteBase = useCallback(
  async (id: number): Promise<void> => {
    await apiClient.delete(KB_ADMIN_API.DELETE_BASE(id));
    message.success('知识库已删除');
    const items = await refreshBases();
    setSelectedBaseIdState((previous) => {
      if (previous !== id) {
        return previous;
      }
      const next = items[0]?.id ?? null;
      if (typeof window !== 'undefined') {
        if (next === null) {
          window.localStorage.removeItem(STORAGE_KEY);
        } else {
          window.localStorage.setItem(STORAGE_KEY, String(next));
        }
      }
      return next;
    });
  },
  [refreshBases]
);
```

文件：`admin/src/components/admin/knowledge-bases-page.tsx`

```tsx
<Button
  key="delete"
  danger
  type="link"
  icon={<DeleteOutlined />}
  loading={deletingBaseId === base.id}
  disabled={isPermissionDenied}
  title={isPermissionDenied ? '权限不足，无法删除知识库' : undefined}
  onClick={() => {
    Modal.confirm({
      title: '删除知识库',
      content: `确认删除 "${base.name}"？这会同时清理它绑定的向量 collection 和已上传文档。`,
      okText: '确认删除',
      cancelText: '取消',
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          setDeletingBaseId(base.id);
          await deleteBase(base.id);
        } finally {
          setDeletingBaseId(null);
        }
      },
    });
  }}
>
  删除
</Button>
```

```tsx
<Text type="secondary">
  Collection: {base.vector_collection || 'Contract gap'}
</Text>
```

文件：`admin/src/components/admin/knowledge-base-detail-page.tsx`

```tsx
<Button
  danger
  icon={<DeleteOutlined />}
  loading={deleteLoading}
  disabled={isPermissionDenied}
  title={isPermissionDenied ? 'Delete knowledge base is unavailable without permission' : undefined}
  onClick={() => {
    Modal.confirm({
      title: 'Delete Knowledge Base',
      content: `Delete "${base?.name || `#${kbId}`}" and its bound vector collection?`,
      okText: 'Delete',
      cancelText: 'Cancel',
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          setDeleteLoading(true);
          await deleteBase(kbId);
          router.push('/knowledge-bases');
        } finally {
          setDeleteLoading(false);
        }
      },
    });
  }}
>
  Delete KB
</Button>
```

```tsx
<Text>Collection: {base?.vector_collection || 'Contract gap'}</Text>
```

### 这段代码在做什么

前端这部分很直白，作用就是把后端新加的能力变成一个可观察、可操作的界面。

`KnowledgeBase` 类型新增 `vector_collection`，这样接口返回的绑定信息可以被前端类型系统接住。

`KB_ADMIN_API.DELETE_BASE` 和 `deleteBase` 则把删除知识库接口接到页面逻辑里。

列表页和详情页都显示 `Collection: ...`，这样管理员一眼就能判断某个知识库是不是已经完成绑定。

### 为什么要这样做

很多后端功能在联调时失败，不是因为逻辑没写对，而是因为页面上根本看不出来当前状态。这里把 `vector_collection` 直接展示出来，是为了降低验证成本。

你可以把这一层理解成“后端功能的仪表盘”。它不是功能本身，但它能帮我们确认功能真的活着。

### 它如何衔接下一步

现在整条链路已经闭环了。最后一步就是验证。

## 如何验证

这次改造没有新增自动化测试文件，所以最重要的是把几个关键场景按顺序走一遍。

### 1. 启动依赖

至少需要这些依赖是可用的：

1. MySQL
2. Redis
3. Milvus
4. 后端服务
5. Admin 前端

因为 `KBKnowledgeBase` 已经在 `AutoMigrate` 里注册，服务启动时会自动尝试补齐表结构。

### 2. 创建知识库，确认自动绑定生效

可以走后台，也可以直接调用接口：

```bash
curl -X POST http://localhost:8888/api/admin/kb/bases \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"name":"Milvus 绑定演示库","description":"用于验证一库一 collection"}'
```

成功后要重点看返回值里有没有 `vector_collection`，例如：

1. `kb_id = 12`
2. `vector_collection = "kb_12_docs"`

如果后台列表页刷新后也能看到 `Collection: kb_12_docs`，说明创建和展示都通了。

### 3. 上传文档，确认写入目标 collection 正确

```bash
curl -X POST http://localhost:8888/api/admin/kb/documents/upload \
  -H "Authorization: Bearer <token>" \
  -F "kb_id=12" \
  -F "file=@D:/tmp/test.md"
```

验证点有三个：

1. 文档和任务在后台详情页能看到。
2. 后端日志里没有 `failed to create collection-specific indexer` 或 `failed to store chunks to milvus`。
3. 到 Milvus 管理界面里能看到数据落在 `kb_12_docs`，而不是全局 `documents`。

### 4. 重试失败任务，确认不会掉回全局 collection

如果你故意制造一次可重试失败，然后点击“重试”，需要确认：

1. `RetryJob` 仍然会把 `Collection` 带进 MQ。
2. 消费者里 `resolveKnowledgeBaseCollectionForIngest` 解析出的还是同一个 collection。
3. 重试成功后，向量仍然写回原来的 `kb_12_docs`。

这一步很重要，因为很多系统只修了首次上传，没有修重试链路。

### 5. 单知识库检索，确认结果来源正确

```bash
curl -X POST http://localhost:8888/api/admin/kb/retrieve \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"kb_id":12,"query":"请总结这份文档的主题","top_k":5}'
```

成功时可以重点看返回结果里的 `source.collection`。它应该是当前知识库对应的 collection，而不是空值，也不是全局默认 collection。

### 6. 多知识库检索，确认跨 collection 聚合生效

```bash
curl -X POST http://localhost:8888/api/admin/kb/retrieve \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"kb_ids":[12,13],"query":"比较这两个知识库中的共同点","top_k":5}'
```

这一步的关键观察点是：

1. 请求没有报错。
2. 结果可以同时来自多个 collection。
3. 检索日志里的 `collection` 可能会表现成 `multi:kb_12_docs,kb_13_docs` 这种标签。

### 7. 删除文档和删除知识库

删除文档时，要确认对应向量只从当前知识库绑定的 collection 中删除。

删除知识库时，要确认：

1. 没有活跃任务时才能删除。
2. 业务表记录被清掉。
3. 源文件被删掉。
4. Milvus 里的 collection 被 `DropCollection`。

如果业务记录删除成功，但 Milvus 删失败，日志里会出现：

1. `failed to inspect collection`
2. `failed to drop collection`

这说明数据库侧已经删完，但向量层留下了孤儿 collection，需要人工处理。

## 取舍与后续优化

这版实现是一个很实用的工程版本，但它也有明确取舍。

### 这版优先优化了什么

1. 优先保证最小改动上线。通过懒分配兼容旧知识库，而不是先做一次重迁移。
2. 优先保证链路完整。创建、上传、重试、删除文档、删除知识库、检索都接入了绑定逻辑。
3. 优先保证可观察性。前端直接展示 `vector_collection`，检索结果里也保留 `source.collection`。

### 它暂时没有解决什么

1. 历史已经写进全局 `documents` 的向量，不会自动迁移到新的 `kb_<id>_docs`。
2. 多 collection 检索现在是串行循环执行，再统一合并，没有做并发 fan-out。
3. 合并结果时只是按 `score` 或 `rerank_score` 排序，没有做更复杂的跨 collection 分数归一化。
4. 创建知识库时还不支持手动指定 collection 名，当前是后端自动生成。
5. 删除知识库时，如果数据库删成功但 `DropCollection` 失败，目前是记日志，不会自动补偿。

### 下一步最自然的演进方向

1. 增加历史向量迁移脚本，把旧的全局 collection 数据按 `kb_id` 重写入各自 collection。
2. 给多 collection 检索加并发执行，降低跨库查询延迟。
3. 给删除知识库补一个异步清理或重试任务，避免留下孤儿 collection。
4. 视业务需要决定是否开放“创建知识库时手动指定 collection 名”的能力。

## 一句话总结

你可以把这次改造理解成一件事：系统不再把“知识库”只当成一个业务标签，而是把它升级成“业务记录 + Milvus collection 绑定”的完整实体。这样上传、检索、删除这些动作，才真正知道自己应该去哪个向量空间工作。
