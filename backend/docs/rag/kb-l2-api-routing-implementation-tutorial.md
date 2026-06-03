# 知识库域 L2 API 与路由实现教程

## 1. 这篇教程是做什么的

这篇文档的目标很简单：

**让别人只看这篇文档，就能把“知识库域 L2 API 与路由”功能一步步敲出来。**

这篇教程不讨论代码 Review，也不展开架构争论，重点只有两件事：

1. 你需要改哪些文件。
2. 每个文件里需要写什么代码。

最后实现出来的接口有 7 个：

1. `POST /api/kb/bases`
2. `GET /api/kb/bases`
3. `POST /api/kb/documents/upload`
4. `GET /api/kb/documents?kb_id=`
5. `GET /api/kb/jobs/:job_id`
6. `DELETE /api/kb/documents/:document_id`
7. `POST /api/kb/retrieve`

---

## 2. 你开始之前要确认的前置条件

在开始写 L2 代码前，项目里最好已经有这些基础：

1. 已有知识库相关数据模型：
   - `backend/internal/model/kb_knowledge_base.go`
   - `backend/internal/model/kb_document.go`
   - `backend/internal/model/kb_ingest_job.go`
2. 这 3 个模型已经在 `backend/internal/repository/database.go` 的 `AutoMigrate` 里注册。
3. 项目里已经有统一响应工具：
   - `backend/api/response/response.go`
4. 项目里已经有 JWT 中间件，并且可以通过 `middleware.GetUserID(c)` 拿到当前用户 ID：
   - `backend/internal/middleware/jwt.go`
5. 项目里已经有自定义路由入口：
   - `backend/api/router/custom_asr.go`
6. 项目里已经有 Milvus 管理器与检索服务：
   - `backend/internal/milvus/init.go`
   - `backend/internal/milvus/retrieval/retriever.go`
7. 项目里已经有本地对象存储封装：
   - `backend/internal/storage/local_oss.go`

如果这些前置条件还没准备好，先把它们补齐，再继续往下做。

---

## 3. 这次最终会改哪些文件

这次只需要处理 3 个文件：

1. 新建 `backend/api/handler/kb/handler.go`
2. 新建 `backend/api/router/custom_kb.go`
3. 修改 `backend/api/router/custom_asr.go`

也就是说，这个功能的 L2 实现主要是：

1. 新增知识库域 handler
2. 新增知识库域自定义路由
3. 把知识库路由接入现有自定义路由入口

---

## 4. 实现顺序建议

请严格按这个顺序做：

1. 先新建知识库路由文件 `custom_kb.go`
2. 再修改 `custom_asr.go`，把新路由挂进去
3. 最后再写 `handler.go`

原因很简单：

1. 先把路由入口搭好，脑子里更清楚接口长什么样
2. handler 是最大文件，最后写更顺

---

## 5. 第一步：新建知识库路由文件

新建文件：

`backend/api/router/custom_kb.go`

把下面完整代码直接写进去：

```go
package router

import (
	kb "interview-agents/api/handler/kb"

	"github.com/cloudwego/hertz/pkg/app/server"
)

func registerKnowledgeBaseRoutes(r *server.Hertz) {
	kbGroup := r.Group("/api/kb")
	kbGroup.POST("/bases", kb.CreateKnowledgeBase)
	kbGroup.GET("/bases", kb.ListKnowledgeBases)
	kbGroup.POST("/documents/upload", kb.UploadDocument)
	kbGroup.GET("/documents", kb.ListDocuments)
	kbGroup.GET("/jobs/:job_id", kb.GetJob)
	kbGroup.DELETE("/documents/:document_id", kb.DeleteDocument)
	kbGroup.POST("/retrieve", kb.Retrieve)
}
```

### 这一步做完后你得到什么

你已经把知识库域的 7 个接口路由全部定义出来了，只是这些 handler 现在还没实现。

---

## 6. 第二步：修改自定义路由入口

打开文件：

`backend/api/router/custom_asr.go`

把它改成下面这样：

```go
package router

import (
	interview "interview-agents/api/handler/interview"

	"github.com/cloudwego/hertz/pkg/app/server"
)

func RegisterCustomRoutes(r *server.Hertz) {
	asr := r.Group("/api/interview/asr")
	asr.GET("/capability", interview.GetASRCapability)
	asr.POST("/transcribe", interview.TranscribeInterviewAudio)

	prediction := r.Group("/api/prediction")
	prediction.POST("/delete", interview.DeletePredictionRecords)

	registerKnowledgeBaseRoutes(r)
}
```

### 这一步做完后你得到什么

服务启动时，知识库域路由会跟 ASR、自定义 prediction 路由一起被注册。

---

## 7. 第三步：新建知识库 handler 文件

新建文件：

`backend/api/handler/kb/handler.go`

这个文件比较长，但你不用拆着猜，直接按下面完整内容写进去即可。

---

## 8. `handler.go` 完整代码

```go
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
	"time"

	"interview-agents/api/response"
	"interview-agents/internal/config"
	myerrors "interview-agents/internal/errors"
	"interview-agents/internal/middleware"
	"interview-agents/internal/milvus"
	"interview-agents/internal/model"
	"interview-agents/internal/repository"
	"interview-agents/internal/storage"

	"github.com/cloudwego/hertz/pkg/app"
	"gorm.io/gorm"
)

const (
	defaultKBPageSize      = 10
	defaultRetrieveTopK    = 5
	maxRetrieveTopK        = 20
	maxKnowledgeFileSize   = 20 * 1024 * 1024
	knowledgeUploadFormKey = "file"
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
	expr := fmt.Sprintf("metadata['user_id'] == %d && metadata['kb_id'] == %d", userID, req.KBID)

	start := time.Now()
	docs, err := retriever.RetrieveWithDatabaseAndCollection(ctx, req.Query, config.Global.Milvus.DatabaseName, collection, &milvus.RetrieveOptions{
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

	log.Printf(
		"[KB Retrieve] query=%q user_id=%d kb_id=%d expr=%q topk=%d rewrite=%q routes=%q final_count=%d duration_ms=%d",
		req.Query,
		userID,
		req.KBID,
		expr,
		topK,
		"",
		"dense",
		len(items),
		durationMs,
	)

	response.Success(ctx, c, retrieveResponse{Items: items})
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
```

---

## 9. 你刚刚写进去的代码分别实现了什么

虽然这篇教程的重点不是讲原理，但为了让你敲代码时不迷糊，还是把每个接口的职责说明一下。

### 9.1 `CreateKnowledgeBase`

作用：

1. 校验当前用户登录
2. 校验请求体 `name`
3. 检查当前用户下是否已有同名知识库
4. 创建一条 `kb_knowledge_base`

### 9.2 `ListKnowledgeBases`

作用：

1. 按 `user_id` 分页查询知识库
2. 返回 `items/total/page/page_size`

### 9.3 `UploadDocument`

作用：

1. 校验当前用户和 `kb_id`
2. 获取上传文件
3. 校验文件类型和大小
4. 读取文件内容并算 `file_hash`
5. 按 `kb_id + file_hash` 做去重
6. 把文件保存到本地 `uploads/knowledge`
7. 用事务创建：
   - `kb_document`
   - `kb_ingest_job`
8. 返回：
   - `document_id`
   - `job_id`
   - `status`

### 9.4 `ListDocuments`

作用：

1. 校验 `kb_id` 属于当前用户
2. 查询这个知识库下的文档列表

### 9.5 `GetJob`

作用：

1. 查询某个入库任务
2. 校验这个任务属于当前用户

### 9.6 `DeleteDocument`

作用：

1. 查询文档
2. 校验文档属于当前用户
3. 对文档做软删除

### 9.7 `Retrieve`

作用：

1. 校验当前用户和 `kb_id`
2. 检查 RAG 是否开启
3. 获取 Milvus retriever
4. 构造检索隔离表达式：
   - `metadata['user_id'] == 当前用户`
   - `metadata['kb_id'] == 当前知识库`
5. 限制 `top_k`
6. 调用 Milvus 检索
7. 标准化返回：
   - `content`
   - `score`
   - `citation`
   - `source`

---

## 10. 这个教程里最关键的 4 个实现点

如果你照着写时想抓重点，请特别留意这 4 个地方。

### 10.1 上传文件字段名固定是 `file`

这一行：

```go
knowledgeUploadFormKey = "file"
```

意味着上传接口用的是：

```text
multipart/form-data
file=<上传文件>
kb_id=<知识库ID>
```

### 10.2 重复上传是“复用已有结果”

这一段逻辑：

```go
existingDoc, err := model.KBDocumentDao.GetByFileHash(kbID, fileHash)
if err == nil && existingDoc != nil && existingDoc.UserID == userID {
	...
	response.Success(ctx, c, uploadDocumentResponse{
		DocumentID: existingDoc.ID,
		JobID:      jobID,
		Status:     string(existingDoc.Status),
		Reused:     true,
	})
	return
}
```

这表示如果同一个知识库里上传了同样内容的文件，就直接复用。

### 10.3 上传时 document 和 job 必须一起创建

这一段事务非常关键：

```go
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
	...
}
```

这样可以保证：

1. 不会只创建文档，不创建任务
2. 创建失败时还能回滚并删除落盘文件

### 10.4 检索必须强制带 `user_id + kb_id` 隔离

这一行是整个检索安全隔离的核心：

```go
expr := fmt.Sprintf("metadata['user_id'] == %d && metadata['kb_id'] == %d", userID, req.KBID)
```

没有这行，就可能出现：

1. 用户 A 检索到用户 B 的数据
2. 同一用户跨知识库串数据

---

## 11. 现在你已经完成了功能代码

到这里为止，L2 的代码已经全部写完。

你已经完成：

1. 路由注册
2. 知识库创建与列表
3. 文档上传与任务创建
4. 文档列表
5. 任务状态查询
6. 文档软删除
7. 检索接口

---

## 12. 最后一步：格式化和编译验证

写完以后，执行下面这些命令：

### 12.1 格式化

```bash
gofmt -w backend/api/handler/kb/handler.go backend/api/router/custom_kb.go backend/api/router/custom_asr.go
```

### 12.2 编译知识库相关代码

```bash
cd backend
go build ./api/handler/kb ./api/router
```

### 12.3 编译主服务

```bash
go build ./cmd/server
```

如果这两条 `go build` 都通过，说明这次 L2 代码已经接入成功。

---

## 13. 你可以直接用的联调请求示例

下面这些例子可以直接拿去本地联调。

### 13.1 创建知识库

```bash
curl -X POST http://localhost:8888/api/kb/bases \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Java Knowledge Base",
    "description": "Phase0 test knowledge base"
  }'
```

### 13.2 查询知识库列表

```bash
curl http://localhost:8888/api/kb/bases \
  -H "Authorization: Bearer <token>"
```

### 13.3 上传文档

```bash
curl -X POST http://localhost:8888/api/kb/documents/upload \
  -H "Authorization: Bearer <token>" \
  -F "kb_id=1" \
  -F "file=@D:/tmp/test.md"
```

### 13.4 查询文档列表

```bash
curl "http://localhost:8888/api/kb/documents?kb_id=1" \
  -H "Authorization: Bearer <token>"
```

### 13.5 查询任务状态

```bash
curl http://localhost:8888/api/kb/jobs/1 \
  -H "Authorization: Bearer <token>"
```

### 13.6 删除文档

```bash
curl -X DELETE http://localhost:8888/api/kb/documents/1 \
  -H "Authorization: Bearer <token>"
```

### 13.7 检索知识库

```bash
curl -X POST http://localhost:8888/api/kb/retrieve \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "kb_id": 1,
    "query": "What is JVM garbage collection?",
    "top_k": 5
  }'
```

---

## 14. 这次教程做到哪一步，没做到哪一步

这篇教程完成的是 **L2 API 与路由**。

已经做到的：

1. 能创建知识库
2. 能上传文件
3. 能生成文档记录和任务记录
4. 能查列表
5. 能查任务
6. 能删文档
7. 能检索 Milvus

还没做到的：

1. 上传后自动发布 `knowledge_ingest` MQ 消息
2. Consumer 消费后推进 `pending -> processing -> completed/failed`
3. 自动切块并入 Milvus
4. 删除文档时同步删除 Milvus chunk

这些属于后续 L3/L4，不在这篇教程范围内。

---

## 15. 一句话总结这篇教程

如果你只记一句话，那就是：

**按这篇文档新建 2 个文件、修改 1 个文件，再把完整代码照着敲进去，最后执行 `gofmt` 和 `go build`，你就能把知识库域的 L2 API 与路由功能实现出来。**
