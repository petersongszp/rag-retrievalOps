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
	"interview-agents/internal/repository"
	"interview-agents/internal/storage"

	"github.com/cloudwego/hertz/pkg/app"
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
	KBID  uint64 `json:"kb_id" vd:"$>0"`
	Query string `json:"query" vd:"len($)>0"`
	TopK  int    `json:"top_k"`
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

type citation struct {
	KBID       uint64 `json:"kb_id"`
	DocumentID uint64 `json:"document_id"`
	ChunkID    string `json:"chunk_id"`
	FileName   string `json:"file_name"`
	ChunkIndex int    `json:"chunk_index"`
}

type source struct {
	Route      string `json:"route"`
	Collection string `json:"collection"`
}

type retrieveItem struct {
	Content  string   `json:"content"`
	Score    float64  `json:"score"`
	Citation citation `json:"citation"`
	Source   source   `json:"source"`
}

type retrieveResponse struct {
	Items []retrieveItem `json:"items"`
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

	existing, err := model.KBKnowledgeBaseDao.GetByUserIDAndName(userID, req.Name)
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
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(ctx, c, "Authorization token is required")
		return
	}

	page, pageSize := getPagination(c)
	items, total, err := model.KBKnowledgeBaseDao.ListByUserID(userID, page, pageSize)
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

	if _, err := mustOwnKnowledgeBase(userID, kbID); err != nil {
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
	if err == nil && existingDoc != nil && existingDoc.UserID == userID {
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

	objectKey := buildKnowledgeObjectKey(userID, kbID, fileName)
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
		UserID:     userID,
		KBID:       kbID,
		DocumentID: doc.ID,
		JobID:      job.ID,
		FilePath:   storagePath,
		FileType:   fileType,
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

	if _, err := mustOwnKnowledgeBase(userID, kbID); err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}

	page, pageSize := getPagination(c)
	items, total, err := model.KBDocumentDao.ListByKbID(kbID, page, pageSize)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to list documents", err))
		return
	}

	filtered := make([]*model.KBDocument, 0, len(items))
	for _, item := range items {
		if item != nil && item.UserID == userID {
			filtered = append(filtered, item)
		}
	}

	response.Success(ctx, c, documentListResponse{
		Items:    filtered,
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
	if job.UserID != userID {
		response.Forbidden(ctx, c, "forbidden")
		return
	}

	response.Success(ctx, c, job)
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

	doc, err := model.KBDocumentDao.GetByID(documentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(ctx, c, "document not found")
			return
		}
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to get document", err))
		return
	}
	if doc.UserID != userID {
		response.Forbidden(ctx, c, "forbidden")
		return
	}

	if err := model.KBDocumentDao.SoftDelete(documentID); err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to delete document", err))
		return
	}

	response.Success(ctx, c, map[string]interface{}{
		"document_id": documentID,
		"deleted":     true,
	})
}

func Retrieve(ctx context.Context, c *app.RequestContext) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(ctx, c, "Authorization token is required")
		return
	}
	if !allowRetrieveForUser(userID) {
		response.Error(ctx, c, 429, "retrieve rate limit exceeded")
		return
	}

	var req retrieveRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.BadRequest(ctx, c, "Invalid request: "+err.Error())
		return
	}
	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		response.BadRequest(ctx, c, "query is required")
		return
	}

	if _, err := mustOwnKnowledgeBase(userID, req.KBID); err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}

	if !config.Global.RAG.Enabled {
		response.Error(ctx, c, 503, "RAG is disabled")
		return
	}

	manager, err := milvus.GetMilvusManager()
	if err != nil {
		response.Error(ctx, c, 503, "Milvus is not initialized")
		return
	}
	retriever := manager.GetRetrieverService()
	if retriever == nil {
		response.Error(ctx, c, 503, "Retriever is not initialized")
		return
	}

	topK := clampTopK(req.TopK)
	collection := config.Global.Milvus.GetCollection("knowledge")
	if collection == "" {
		collection = config.Global.Milvus.CollectionName
	}
	expr := fmt.Sprintf("metadata[\"user_id\"] == %d && metadata[\"kb_id\"] == %d", userID, req.KBID)
	retrieveTimeout := resolveRetrieveTimeout()
	retrieveCtx, cancel := context.WithTimeout(ctx, retrieveTimeout)
	defer cancel()

	start := time.Now()
	docs, err := retriever.RetrieveWithDatabaseAndCollection(retrieveCtx, req.Query, config.Global.Milvus.DatabaseName, collection, &milvus.RetrieveOptions{
		Expr:       expr,
		TopK:       topK,
		Collection: collection,
	})
	durationMs := time.Since(start).Milliseconds()
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewMilvusError("knowledge retrieve failed", err))
		return
	}

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
		if err != nil || storedDoc.UserID != userID || storedDoc.KbID != req.KBID {
			continue
		}

		items = append(items, retrieveItem{
			Content: doc.Content,
			Score:   getFloat64Metadata(doc.MetaData, "score"),
			Citation: citation{
				KBID:       req.KBID,
				DocumentID: documentID,
				ChunkID:    firstNonEmptyString(doc.ID, getStringMetadata(doc.MetaData, "chunk_id")),
				FileName:   firstNonEmptyString(getStringMetadata(doc.MetaData, "file_name"), storedDoc.FileName),
				ChunkIndex: getIntMetadata(doc.MetaData, "chunk_index"),
			},
			Source: source{
				Route:      "dense",
				Collection: collection,
			},
		})
	}

	if config.Global.RAG.FeatureFlags.EnableRetrieveAudit {
		log.Printf(
			"[KB Retrieve] query=%q user_id=%d kb_id=%d expr=%q topk=%d rewrite=%q routes=%q final_count=%d duration_ms=%d timeout_ms=%d",
			req.Query,
			userID,
			req.KBID,
			expr,
			topK,
			"",
			"dense",
			len(items),
			durationMs,
			retrieveTimeout.Milliseconds(),
		)
	}

	response.Success(ctx, c, retrieveResponse{Items: items})
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

func mustOwnKnowledgeBase(userID uint, kbID uint64) (*model.KBKnowledgeBase, error) {
	kb, err := model.KBKnowledgeBaseDao.GetByID(kbID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, myerrors.NewNotFoundError("knowledge base")
		}
		return nil, myerrors.NewDBError("failed to get knowledge base", err)
	}
	if kb.UserID != userID {
		return nil, myerrors.NewAppError(myerrors.ErrCodeForbidden, "forbidden", 403)
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

func buildKnowledgeObjectKey(userID uint, kbID uint64, fileName string) string {
	safeName := strings.ReplaceAll(filepath.Base(fileName), " ", "_")
	return fmt.Sprintf("kb_%d_%d_%d_%s", userID, kbID, time.Now().UnixNano(), safeName)
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
