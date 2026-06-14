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
	merged.Metrics.DenseParticipation = 0
	merged.Metrics.SparseParticipation = 0
	merged.Metrics.PrimaryDenseCount = 0
	merged.Metrics.PrimarySparseCount = 0
	merged.Metrics.DualRouteFinalCount = 0
	merged.Metrics.DenseContribution = 0
	merged.Metrics.SparseContribution = 0
	merged.Metrics.SparseCandidateBefore = 0
	merged.Metrics.SparseCandidateAfter = 0

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
		merged.Metrics.DenseParticipation += result.Metrics.DenseParticipation
		merged.Metrics.SparseParticipation += result.Metrics.SparseParticipation
		merged.Metrics.PrimaryDenseCount += result.Metrics.PrimaryDenseCount
		merged.Metrics.PrimarySparseCount += result.Metrics.PrimarySparseCount
		merged.Metrics.DualRouteFinalCount += result.Metrics.DualRouteFinalCount
		merged.Metrics.DenseContribution += result.Metrics.DenseContribution
		merged.Metrics.SparseContribution += result.Metrics.SparseContribution
		merged.Metrics.SparseCandidateBefore += result.Metrics.SparseCandidateBefore
		merged.Metrics.SparseCandidateAfter += result.Metrics.SparseCandidateAfter
		if result.Metrics.CandidateTopK > candidateTopK {
			candidateTopK = result.Metrics.CandidateTopK
		}
	}

	sortRetrieveDocsByDisplayScore(docs)
	if topK > 0 && len(docs) > topK {
		docs = docs[:topK]
	}

	merged.Documents = docs
	merged.Metrics.CandidateTopK = candidateTopK
	merged.Metrics.FinalTopK = len(docs)
	finalSummary := summarizeMergedKnowledgeBaseRoutes(docs)
	merged.Metrics.DenseParticipation = finalSummary.DenseParticipation
	merged.Metrics.SparseParticipation = finalSummary.SparseParticipation
	merged.Metrics.PrimaryDenseCount = finalSummary.PrimaryDenseCount
	merged.Metrics.PrimarySparseCount = finalSummary.PrimarySparseCount
	merged.Metrics.DualRouteFinalCount = finalSummary.DualRouteFinalCount
	merged.Metrics.DenseContribution = finalSummary.PrimaryDenseCount
	merged.Metrics.SparseContribution = finalSummary.PrimarySparseCount
	if merged.Metrics.RetrieverVersion == "" {
		if useHybrid {
			merged.Metrics.RetrieverVersion = retrieval.HybridRetrieverVersion
		} else {
			merged.Metrics.RetrieverVersion = retrieval.DenseRetrieverVersion
		}
	}
	return merged
}

type mergedRouteSummary struct {
	DenseParticipation  int
	SparseParticipation int
	PrimaryDenseCount   int
	PrimarySparseCount  int
	DualRouteFinalCount int
}

func summarizeMergedKnowledgeBaseRoutes(docs []*schema.Document) mergedRouteSummary {
	summary := mergedRouteSummary{}
	for _, doc := range docs {
		if doc == nil || doc.MetaData == nil {
			continue
		}
		routeContrib, _ := doc.MetaData["route_contrib"].(map[string]interface{})
		if _, ok := routeContrib["dense"]; ok {
			summary.DenseParticipation++
		}
		if _, ok := routeContrib["sparse"]; ok {
			summary.SparseParticipation++
		}
		if _, ok := routeContrib["dense"]; ok {
			if _, dual := routeContrib["sparse"]; dual {
				summary.DualRouteFinalCount++
			}
		}
		switch strings.ToLower(strings.TrimSpace(getStringMetadata(doc.MetaData, "route"))) {
		case "sparse":
			summary.PrimarySparseCount++
		default:
			summary.PrimaryDenseCount++
		}
	}
	return summary
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

func sortRetrieveDocsByDisplayScore(docs []*schema.Document) {
	sort.SliceStable(docs, func(i, j int) bool {
		return readRetrieveDocScore(docs[i]) > readRetrieveDocScore(docs[j])
	})
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

	kb, err := mustKnowledgeBaseExistForActor(c, kbID)
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
