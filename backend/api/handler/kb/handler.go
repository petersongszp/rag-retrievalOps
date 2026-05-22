package kb

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"interview-agents/api/response"
	"interview-agents/internal/config"
	myerrors "interview-agents/internal/errors"
	"interview-agents/internal/middleware"
	"interview-agents/internal/milvus"
	"interview-agents/internal/model"
	"interview-agents/internal/mq"
	"interview-agents/internal/observability/metrics"
	"interview-agents/internal/repository"
	"interview-agents/internal/storage"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
	"gorm.io/gorm"
)

const (
	defaultKBPageSize      = 10
	defaultRetrieveTopK    = 5
	defaultRetrieveTimeout = 3 * time.Second
	maxRetrieveTopK        = 20
	maxKnowledgeFileSize   = 20 * 1024 * 1024
	knowledgeUploadFormKey = "file"
)

var (
	retrieveLimiterMu sync.Mutex
	retrieveLimiters  = map[uint]*rate.Limiter{}
)

type createKnowledgeBaseRequest struct {
	Name        string `json:"name" vd:"len($)>0"`
	Description string `json:"description"`
}

type retrieveRequest struct {
	KBID  uint64   `json:"kb_id"`
	KBIDs []uint64 `json:"kb_ids"`
	Query string   `json:"query" vd:"len($)>0"`
	TopK  int      `json:"top_k"`
}

type knowledgeBaseListResponse struct {
	Items    []*model.KBKnowledgeBase `json:"items"`
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
}

type documentListResponse struct {
	Items    []*model.KBDocument `json:"items"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

type uploadDocumentResponse struct {
	DocumentID uint64 `json:"document_id"`
	JobID      uint64 `json:"job_id"`
	Status     string `json:"status"`
	Reused     bool   `json:"reused,omitempty"`
}

type jobListResponse struct {
	Items    []*model.KBIngestJob `json:"items"`
	Total    int64                `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
}

type citation struct {
	KBID         uint64 `json:"kb_id"`
	DocumentID   uint64 `json:"document_id"`
	ChunkID      string `json:"chunk_id"`
	FileName     string `json:"file_name"`
	ChunkIndex   int    `json:"chunk_index"`
	SnippetOffset int   `json:"snippet_offset,omitempty"`
}

type source struct {
	Route            string `json:"route"`
	Collection       string `json:"collection"`
	RetrieverVersion string `json:"retriever_version"`
}

type retrieveItem struct {
	Content  string   `json:"content"`
	Score    float64  `json:"score"`
	Citation citation `json:"citation"`
	Source   source   `json:"source"`
}

type retrieveResponse struct {
	RequestID string         `json:"request_id"`
	Items     []retrieveItem `json:"items"`
}

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

	response.Success(ctx, c, kb)
}

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

	response.Success(ctx, c, knowledgeBaseListResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

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

	if _, err := mustKnowledgeBaseExist(kbID); err != nil {
		response.ErrorFromErr(ctx, c, err)
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

func ListDocuments(ctx context.Context, c *app.RequestContext) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(ctx, c, "Authorization token is required")
		return
	}

	kbID, err := parseUint64(string(c.Query("kb_id")), "kb_id")
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}

	if _, err := mustKnowledgeBaseExist(kbID); err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}

	page, pageSize := getPagination(c)
	items, total, err := model.KBDocumentDao.ListByKbID(kbID, page, pageSize)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to list documents", err))
		return
	}

	response.Success(ctx, c, documentListResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func ListJobs(ctx context.Context, c *app.RequestContext) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(ctx, c, "Authorization token is required")
		return
	}

	var status *model.KBIngestJobStatus
	statusRaw := strings.TrimSpace(string(c.Query("status")))
	if statusRaw != "" {
		parsed, ok := model.ParseKBIngestJobStatus(statusRaw)
		if !ok {
			response.BadRequest(ctx, c, "status is invalid")
			return
		}
		status = &parsed
	}

	page, pageSize := getPagination(c)
	items, total, err := model.KBIngestJobDao.List(status, page, pageSize)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to list jobs", err))
		return
	}

	response.Success(ctx, c, jobListResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func GetJob(ctx context.Context, c *app.RequestContext) {
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
	response.Success(ctx, c, job)
}

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
	}); err != nil {
		errMsg := "failed to enqueue retry task: " + err.Error()
		_, _ = model.KBIngestJobDao.UpdateStatusFrom(jobID, model.KBIngestJobStatusFailed, errMsg, model.KBIngestJobStatusRetrying)
		_ = model.KBDocumentDao.UpdateStatus(doc.ID, model.KBDocumentStatusFailed, errMsg)
		response.InternalServerError(ctx, c, "failed to enqueue retry task")
		return
	}

	_ = model.KBDocumentDao.UpdateStatus(doc.ID, model.KBDocumentStatusPending, "")
	_ = model.KBJobOperationLogDao.Create(&model.KBJobOperationLog{
		JobID:           job.ID,
		OperatorID:      userID,
		Operation:       "retry",
		OperationReason: reason,
		FromStatus:      string(job.Status),
		ToStatus:        string(model.KBIngestJobStatusRetrying),
	})

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

func CancelJob(ctx context.Context, c *app.RequestContext) {
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
	if job.Status == model.KBIngestJobStatusCompleted || job.Status == model.KBIngestJobStatusCanceled {
		response.BadRequest(ctx, c, "job cannot be canceled in current status")
		return
	}

	reason := getOperationReason(c)
	transitioned, err := model.KBIngestJobDao.MarkCanceled(jobID, userID, reason)
	if err != nil {
		if errors.Is(err, model.ErrInvalidKBIngestJobTransition) {
			response.BadRequest(ctx, c, "job status transition is invalid")
			return
		}
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to cancel job", err))
		return
	}
	if !transitioned {
		response.BadRequest(ctx, c, "job status transition is invalid")
		return
	}

	if doc, err := model.KBDocumentDao.GetByID(job.DocumentID); err == nil {
		_ = model.KBDocumentDao.UpdateStatus(doc.ID, model.KBDocumentStatusFailed, firstNonEmptyString(reason, "canceled by operator"))
	}

	_ = model.KBJobOperationLogDao.Create(&model.KBJobOperationLog{
		JobID:           job.ID,
		OperatorID:      userID,
		Operation:       "cancel",
		OperationReason: reason,
		FromStatus:      string(job.Status),
		ToStatus:        string(model.KBIngestJobStatusCanceled),
	})

	updatedJob, getErr := model.KBIngestJobDao.GetByID(jobID)
	if getErr != nil {
		response.Success(ctx, c, map[string]interface{}{
			"job_id":  jobID,
			"status":  string(model.KBIngestJobStatusCanceled),
			"message": "job canceled",
		})
		return
	}
	response.Success(ctx, c, updatedJob)
}

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

	if _, err := model.KBDocumentDao.GetByID(documentID); err != nil {
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
			collection := config.Global.Milvus.GetCollection("knowledge")
			if collection == "" {
				collection = config.Global.Milvus.CollectionName
			}
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

func Retrieve(ctx context.Context, c *app.RequestContext) {
	requestID := uuid.New().String()
	metricsStartedAt := time.Now()
	metricsStatus := "success"
	metricsErrorCode := "none"
	metricsResultCount := 0
	defer func() {
		metrics.ObserveRetrieve(time.Since(metricsStartedAt), metricsStatus, metricsErrorCode, metricsResultCount)
	}()

	userID := middleware.GetUserID(c)
	if userID == 0 {
		metricsStatus = "error"
		metricsErrorCode = "unauthorized"
		response.Unauthorized(ctx, c, "Authorization token is required")
		return
	}
	if !allowRetrieveForUser(userID) {
		metricsStatus = "error"
		metricsErrorCode = "rate_limit_exceeded"
		response.Error(ctx, c, 429, "retrieve rate limit exceeded")
		return
	}

	var req retrieveRequest
	if err := c.BindAndValidate(&req); err != nil {
		metricsStatus = "error"
		metricsErrorCode = "invalid_param"
		response.BadRequest(ctx, c, "Invalid request: "+err.Error())
		return
	}
	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		metricsStatus = "error"
		metricsErrorCode = "invalid_param"
		response.BadRequest(ctx, c, "query is required")
		return
	}

	if !config.Global.RAG.Enabled {
		metricsStatus = "error"
		metricsErrorCode = "rag_disabled"
		response.Error(ctx, c, 503, "RAG is disabled")
		return
	}

	manager, err := milvus.GetMilvusManager()
	if err != nil {
		metricsStatus = "error"
		metricsErrorCode = "milvus_not_initialized"
		response.Error(ctx, c, 503, "Milvus is not initialized")
		return
	}
	retriever := manager.GetRetrieverService()
	if retriever == nil {
		metricsStatus = "error"
		metricsErrorCode = "retriever_not_initialized"
		response.Error(ctx, c, 503, "Retriever is not initialized")
		return
	}

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

	collection := config.Global.Milvus.GetCollection("knowledge")
	if collection == "" {
		collection = config.Global.Milvus.CollectionName
	}
	activeGlobalKBID := uint64(0)
	if len(kbIDs) > 0 {
		activeGlobalKBID = kbIDs[0]
	}
	expr := buildKBFilterExpr(kbIDs)
	retrieveTimeout := resolveRetrieveTimeout()
	retrieveCtx, cancel := context.WithTimeout(ctx, retrieveTimeout)
	defer cancel()

	start := time.Now()
	searchResult, err := retriever.RetrieveKnowledgeWithMetrics(retrieveCtx, req.Query, activeGlobalKBID, topK, collection)
	durationMs := time.Since(start).Milliseconds()
	if err != nil {
		metricsStatus, metricsErrorCode = classifyRetrieveError(err, retrieveCtx)
		persistRetrieveLog(&model.KBRetrieveLog{
			RequestID:    requestID,
			UserID:       userID,
			KBIDs:        formatKBIDs(kbIDs),
			Query:        req.Query,
			Expr:         expr,
			TopK:         topK,
			Routes:       "dense",
			Collection:   collection,
			ResultStatus: classifyRetrieveResultStatus(metricsStatus),
			ErrorCode:    metricsErrorCode,
			ErrorMsg:     err.Error(),
			DurationMs:   durationMs,
			TimeoutMs:    retrieveTimeout.Milliseconds(),
		})
		response.ErrorFromErr(ctx, c, myerrors.NewMilvusError("knowledge retrieve failed", err))
		return
	}

	docs := searchResult.Documents
	searchMetrics := searchResult.Metrics

	allowedKBs := make(map[uint64]struct{}, len(kbIDs))
	for _, id := range kbIDs {
		allowedKBs[id] = struct{}{}
	}

	queryLower := strings.ToLower(req.Query)
	items := make([]retrieveItem, 0, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}

		documentID := getUint64Metadata(doc.MetaData, "document_id")
		if documentID == 0 {
			continue
		}
		storedDoc, err := model.KBDocumentDao.GetByID(documentID)
		if err != nil || storedDoc == nil {
			continue
		}
		if _, ok := allowedKBs[storedDoc.KbID]; !ok {
			continue
		}

		items = append(items, retrieveItem{
			Content: doc.Content,
			Score:   getFloat64Metadata(doc.MetaData, "score"),
			Citation: citation{
				KBID:         storedDoc.KbID,
				DocumentID:   documentID,
				ChunkID:      firstNonEmptyString(doc.ID, getStringMetadata(doc.MetaData, "chunk_id")),
				FileName:     firstNonEmptyString(getStringMetadata(doc.MetaData, "file_name"), storedDoc.FileName),
				ChunkIndex:   getIntMetadata(doc.MetaData, "chunk_index"),
				SnippetOffset: computeSnippetOffset(doc.Content, queryLower),
			},
			Source: source{
				Route:            "dense",
				Collection:       collection,
				RetrieverVersion: "v1",
			},
		})
	}

	resultStatus := model.RetrieveResultStatusSuccess
	if len(items) == 0 {
		if searchMetrics.HitCount > 0 {
			resultStatus = model.RetrieveResultStatusFilteredOut
		} else {
			resultStatus = model.RetrieveResultStatusNoResult
		}
	}

	retrieveLog := &model.KBRetrieveLog{
		RequestID:       requestID,
		UserID:          userID,
		KBIDs:           formatKBIDs(kbIDs),
		Query:           req.Query,
		Expr:            expr,
		TopK:            topK,
		Routes:          "dense",
		Collection:      collection,
		RetrieverVersion: "v1",
		FinalCount:      len(items),
		TruncatedCount:  searchMetrics.TruncatedCount,
		ResultStatus:    resultStatus,
		EmbeddingMs:     searchMetrics.EmbeddingMs,
		SearchMs:        searchMetrics.SearchMs,
		PostprocessMs:   searchMetrics.PostprocessMs,
		DurationMs:      durationMs,
		TimeoutMs:       retrieveTimeout.Milliseconds(),
	}
	persistRetrieveLog(retrieveLog)

	if config.Global.RAG.FeatureFlags.EnableRetrieveAudit {
		log.Printf(
			"[KB Retrieve] request_id=%s query=%q user_id=%d kb_ids=%v kb_scope=%q expr=%q topk=%d rewrite=%q routes=%q final_count=%d hit_count=%d truncated_count=%d duration_ms=%d embedding_ms=%d search_ms=%d postprocess_ms=%d timeout_ms=%d result_status=%s",
			requestID,
			req.Query,
			userID,
			kbIDs,
			"global",
			expr,
			topK,
			"",
			"dense",
			len(items),
			searchMetrics.HitCount,
			searchMetrics.TruncatedCount,
			durationMs,
			searchMetrics.EmbeddingMs,
			searchMetrics.SearchMs,
			searchMetrics.PostprocessMs,
			retrieveTimeout.Milliseconds(),
			string(resultStatus),
		)
	}

	metricsResultCount = len(items)
	response.Success(ctx, c, retrieveResponse{RequestID: requestID, Items: items})
}

func resolveAppErrorCode(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	var appErr *myerrors.AppError
	if errors.As(err, &appErr) && appErr != nil {
		return strings.ToLower(string(appErr.Code))
	}
	return fallback
}

func classifyRetrieveError(err error, retrieveCtx context.Context) (string, string) {
	if err == nil {
		return "success", "none"
	}
	errText := strings.ToLower(err.Error())
	if errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(retrieveCtx.Err(), context.DeadlineExceeded) ||
		strings.Contains(errText, "timeout") ||
		strings.Contains(errText, "deadline exceeded") {
		return "timeout", "timeout"
	}
	return "error", "milvus_error"
}

func resolveRetrieveTimeout() time.Duration {
	timeoutMs := config.Global.RAG.Thresholds.RetrieveTimeoutMS
	if timeoutMs <= 0 {
		return defaultRetrieveTimeout
	}
	return time.Duration(timeoutMs) * time.Millisecond
}

func allowRetrieveForUser(userID uint) bool {
	limit := config.Global.RAG.Thresholds.UserQPSLimit
	if limit <= 0 {
		return true
	}

	retrieveLimiterMu.Lock()
	limiter, exists := retrieveLimiters[userID]
	if !exists {
		burst := limit
		if burst < 1 {
			burst = 1
		}
		limiter = rate.NewLimiter(rate.Limit(limit), burst)
		retrieveLimiters[userID] = limiter
	}
	retrieveLimiterMu.Unlock()

	return limiter.Allow()
}

func mustKnowledgeBaseExist(kbID uint64) (*model.KBKnowledgeBase, error) {
	kb, err := model.KBKnowledgeBaseDao.GetByID(kbID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, myerrors.NewNotFoundError("knowledge base")
		}
		return nil, myerrors.NewDBError("failed to get knowledge base", err)
	}
	return kb, nil
}

func getPagination(c *app.RequestContext) (int, int) {
	page := getIntWithDefault(string(c.Query("page")), 1)
	pageSize := getIntWithDefault(string(c.Query("page_size")), defaultKBPageSize)
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultKBPageSize
	}
	return page, pageSize
}

func clampTopK(topK int) int {
	if topK <= 0 {
		return defaultRetrieveTopK
	}
	if topK > maxRetrieveTopK {
		return maxRetrieveTopK
	}
	return topK
}

func getKnowledgeOSS() (storage.OSS, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return storage.NewLocalOSS(filepath.Join(wd, "uploads", "knowledge"))
}

func validateKnowledgeFile(fileHeader *multipart.FileHeader) (string, error) {
	if fileHeader == nil {
		return "", fmt.Errorf("file is required")
	}
	if fileHeader.Size > maxKnowledgeFileSize {
		return "", fmt.Errorf("file size cannot exceed 20MB")
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	switch ext {
	case ".pdf":
		return "pdf", nil
	case ".txt":
		return "txt", nil
	case ".md":
		return "md", nil
	case ".markdown":
		return "markdown", nil
	default:
		return "", fmt.Errorf("only pdf/txt/md/markdown files are supported")
	}
}

func readKnowledgeFile(fileHeader *multipart.FileHeader) ([]byte, string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, "", fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer func(file multipart.File) {
		_ = file.Close()
	}(file)

	content, err := io.ReadAll(io.LimitReader(file, maxKnowledgeFileSize+1))
	if err != nil {
		return nil, "", fmt.Errorf("failed to read uploaded file: %w", err)
	}
	if len(content) == 0 {
		return nil, "", fmt.Errorf("uploaded file is empty")
	}
	if len(content) > maxKnowledgeFileSize {
		return nil, "", fmt.Errorf("file size cannot exceed 20MB")
	}

	sum := sha256.Sum256(content)
	return content, hex.EncodeToString(sum[:]), nil
}

func buildKnowledgeObjectKey(kbID uint64, fileName string) string {
	safeName := strings.ReplaceAll(filepath.Base(fileName), " ", "_")
	return fmt.Sprintf("kb_%d_%d_%s", kbID, time.Now().UnixNano(), safeName)
}

func resolveRetrieveKBIDs(req retrieveRequest, activeKBIDs []uint64) []uint64 {
	activeSet := make(map[uint64]struct{}, len(activeKBIDs))
	for _, id := range activeKBIDs {
		if id > 0 {
			activeSet[id] = struct{}{}
		}
	}
	if len(activeSet) == 0 {
		return nil
	}

	candidates := make([]uint64, 0, len(req.KBIDs)+1)
	for _, id := range req.KBIDs {
		if id > 0 {
			candidates = append(candidates, id)
		}
	}
	if req.KBID > 0 {
		candidates = append(candidates, req.KBID)
	}

	if len(candidates) == 0 {
		result := make([]uint64, 0, len(activeSet))
		for id := range activeSet {
			result = append(result, id)
		}
		return result
	}

	result := make([]uint64, 0, len(candidates))
	seen := make(map[uint64]struct{}, len(candidates))
	for _, id := range candidates {
		if _, ok := activeSet[id]; !ok {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func buildKBFilterExpr(kbIDs []uint64) string {
	if len(kbIDs) == 0 {
		return ""
	}
	if len(kbIDs) == 1 {
		return fmt.Sprintf("metadata['kb_id'] == %d", kbIDs[0])
	}

	conditions := make([]string, 0, len(kbIDs))
	for _, id := range kbIDs {
		if id == 0 {
			continue
		}
		conditions = append(conditions, fmt.Sprintf("metadata['kb_id'] == %d", id))
	}
	if len(conditions) == 0 {
		return ""
	}
	return "(" + strings.Join(conditions, " || ") + ")"
}

func parseUint64(raw string, field string) (uint64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("%s is required", field)
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid integer", field)
	}
	return parsed, nil
}

func getIntWithDefault(raw string, fallback int) int {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getUint64Metadata(metadata map[string]interface{}, key string) uint64 {
	if metadata == nil {
		return 0
	}
	switch value := metadata[key].(type) {
	case uint64:
		return value
	case uint32:
		return uint64(value)
	case uint:
		return uint64(value)
	case int:
		if value >= 0 {
			return uint64(value)
		}
	case int64:
		if value >= 0 {
			return uint64(value)
		}
	case float64:
		if value >= 0 {
			return uint64(value)
		}
	case float32:
		if value >= 0 {
			return uint64(value)
		}
	case string:
		parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
		if err == nil {
			return parsed
		}
	}
	return 0
}

func getIntMetadata(metadata map[string]interface{}, key string) int {
	if metadata == nil {
		return 0
	}
	switch value := metadata[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case uint64:
		return int(value)
	case float64:
		return int(value)
	case float32:
		return int(value)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err == nil {
			return parsed
		}
	}
	return 0
}

func getFloat64Metadata(metadata map[string]interface{}, key string) float64 {
	if metadata == nil {
		return 0
	}
	switch value := metadata[key].(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	case uint64:
		return float64(value)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err == nil {
			return parsed
		}
	}
	return 0
}

func getStringMetadata(metadata map[string]interface{}, key string) string {
	if metadata == nil {
		return ""
	}
	switch value := metadata[key].(type) {
	case string:
		return value
	case []byte:
		return string(value)
	default:
		if value != nil {
			return fmt.Sprint(value)
		}
	}
	return ""
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func getOperationReason(c *app.RequestContext) string {
	reason := strings.TrimSpace(string(c.Query("operation_reason")))
	if reason != "" {
		return reason
	}
	return strings.TrimSpace(c.PostForm("operation_reason"))
}

func computeSnippetOffset(content, queryLower string) int {
	if queryLower == "" || content == "" {
		return -1
	}
	idx := strings.Index(strings.ToLower(content), queryLower)
	if idx >= 0 {
		return idx
	}
	words := strings.Fields(queryLower)
	if len(words) > 0 {
		idx = strings.Index(strings.ToLower(content), words[0])
		if idx >= 0 {
			return idx
		}
	}
	return -1
}

func persistRetrieveLog(entry *model.KBRetrieveLog) {
	go func() {
		if err := model.KBRetrieveLogDao.Create(entry); err != nil {
			log.Printf("[KB Retrieve Audit] failed to persist retrieve log request_id=%s err=%v", entry.RequestID, err)
		}
	}()
}

func classifyRetrieveResultStatus(metricsStatus string) model.RetrieveResultStatus {
	switch metricsStatus {
	case "timeout":
		return model.RetrieveResultStatusTimeout
	case "error":
		return model.RetrieveResultStatusError
	default:
		return model.RetrieveResultStatusSuccess
	}
}

func formatKBIDs(ids []uint64) string {
	if len(ids) == 0 {
		return ""
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatUint(id, 10)
	}
	return strings.Join(parts, ",")
}

func GetRetrieveAuditLog(ctx context.Context, c *app.RequestContext) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(ctx, c, "Authorization token is required")
		return
	}

	requestID := strings.TrimSpace(c.Param("request_id"))
	if requestID == "" {
		response.BadRequest(ctx, c, "request_id is required")
		return
	}

	logEntry, err := model.KBRetrieveLogDao.GetByRequestID(requestID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(ctx, c, "retrieve log not found")
			return
		}
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to get retrieve log", err))
		return
	}

	response.Success(ctx, c, logEntry)
}

type retrieveAuditListResponse struct {
	Items    []*model.KBRetrieveLog `json:"items"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

func ListRetrieveAuditLogs(ctx context.Context, c *app.RequestContext) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(ctx, c, "Authorization token is required")
		return
	}

	page, pageSize := getPagination(c)

	statusRaw := strings.TrimSpace(string(c.Query("result_status")))
	if statusRaw != "" {
		status, ok := model.ParseRetrieveResultStatus(statusRaw)
		if !ok {
			response.BadRequest(ctx, c, "result_status is invalid")
			return
		}
		items, total, err := model.KBRetrieveLogDao.ListByStatus(status, page, pageSize)
		if err != nil {
			response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to list retrieve logs", err))
			return
		}
		response.Success(ctx, c, retrieveAuditListResponse{
			Items:    items,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		})
		return
	}

	items, total, err := model.KBRetrieveLogDao.ListByUserID(userID, page, pageSize)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to list retrieve logs", err))
		return
	}

	response.Success(ctx, c, retrieveAuditListResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}
