package kb

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	authpkg "interview-agents/internal/auth"
	"interview-agents/internal/config"
	myerrors "interview-agents/internal/errors"
	"interview-agents/internal/middleware"
	"interview-agents/internal/milvus"
	"interview-agents/internal/milvus/retrieval"
	"interview-agents/internal/model"
	"interview-agents/internal/observability/metrics"
	"interview-agents/internal/rag/experiment"
	"interview-agents/internal/rag/governance"
	"interview-agents/internal/rag/release"
	"interview-agents/internal/ragqueue"
	"interview-agents/internal/repository"
	"interview-agents/internal/storage"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
	"gorm.io/gorm"
)

const (
	defaultKBPageSize            = 10
	defaultRetrieveTopK          = 5
	defaultRetrieveTimeout       = 3 * time.Second
	maxRetrieveTopK              = 20
	maxKnowledgeFileSize         = 20 * 1024 * 1024
	knowledgeUploadFormKey       = "file"
	defaultReleaseSummaryMinutes = 60
	maxReleaseSummaryMinutes     = 7 * 24 * 60
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
	KBID          uint64 `json:"kb_id"`
	DocumentID    uint64 `json:"document_id"`
	ChunkID       string `json:"chunk_id"`
	FileName      string `json:"file_name"`
	ChunkIndex    int    `json:"chunk_index"`
	SnippetOffset int    `json:"snippet_offset,omitempty"`
}

type source struct {
	Route                string  `json:"route"`
	Collection           string  `json:"collection"`
	RetrieverVersion     string  `json:"retriever_version"`
	ParentID             string  `json:"parent_id"`
	ChildID              string  `json:"child_id"`
	SectionTitle         string  `json:"section_title"`
	HierarchyPath        string  `json:"hierarchy_path"`
	ParentFillStrategy   string  `json:"parent_fill_strategy"`
	ParentFillTokens     int     `json:"parent_fill_tokens"`
	CitationSupported    bool    `json:"citation_supported"`
	CitationSupportScore float64 `json:"citation_support_score"`
	CitationCheckVersion string  `json:"citation_check_version"`
	LowSupportCitation   bool    `json:"low_support_citation"`
}

type retrieveItem struct {
	Content  string   `json:"content"`
	Score    float64  `json:"score"`
	Citation citation `json:"citation"`
	Source   source   `json:"source"`
}

type refusalPayload struct {
	Reason               string   `json:"reason"`
	Message              string   `json:"message"`
	Suggestions          []string `json:"suggestions,omitempty"`
	CitationSupportScore float64  `json:"citation_support_score,omitempty"`
}

type citationCheckResponse struct {
	Supported             bool     `json:"supported"`
	SupportScore          float64  `json:"support_score"`
	UnsupportedClaims     []string `json:"unsupported_claims,omitempty"`
	UnsupportedClaimCount int      `json:"unsupported_claim_count"`
	Version               string   `json:"version,omitempty"`
	LatencyMs             int64    `json:"latency_ms,omitempty"`
	Error                 string   `json:"error,omitempty"`
}

type retrieveResponse struct {
	RequestID          string                 `json:"request_id"`
	Items              []retrieveItem         `json:"items"`
	EvidenceGateResult string                 `json:"evidence_gate_result,omitempty"`
	CitationCheck      *citationCheckResponse `json:"citation_check,omitempty"`
	Refusal            *refusalPayload        `json:"refusal,omitempty"`
}

type releaseStatusResponse struct {
	Config          config.RAGReleaseConfig `json:"config"`
	RuntimeOverride release.RuntimeOverride `json:"runtime_override"`
	EffectiveStage  string                  `json:"effective_stage"`
	CurrentStrategy string                  `json:"current_strategy"`
	StagePlan       []string                `json:"stage_plan"`
	RollbackPlan    []string                `json:"rollback_plan"`
}

type releaseSummaryResponse struct {
	WindowMinutes         int            `json:"window_minutes"`
	Since                 time.Time      `json:"since"`
	TotalRequests         int            `json:"total_requests"`
	StrategyCounts        map[string]int `json:"strategy_counts"`
	ReleaseStageCounts    map[string]int `json:"release_stage_counts"`
	ResultStatusCounts    map[string]int `json:"result_status_counts"`
	EmptyReasonCounts     map[string]int `json:"empty_reason_counts"`
	RouteContribution     map[string]int `json:"route_contribution"`
	RewriteGainBuckets    map[string]int `json:"rewrite_gain_buckets"`
	RewriteAppliedRate    float64        `json:"rewrite_applied_rate"`
	ParentFillRate        float64        `json:"parent_fill_rate"`
	EvidenceRefusalRate   float64        `json:"evidence_refusal_rate"`
	ModelRewriteErrorRate float64        `json:"model_rewrite_error_rate"`
	AvgCitationSupport    float64        `json:"avg_citation_support"`
	P95DurationMs         int64          `json:"p95_duration_ms"`
	P95RerankMs           int64          `json:"p95_rerank_ms"`
	RollbackRecommended   bool           `json:"rollback_recommended"`
	Risks                 []string       `json:"risks"`
	AcceptanceTemplate    string         `json:"acceptance_template"`
}

type retrieveDebugQueryView struct {
	Original string `json:"original"`
	Rewrite  string `json:"rewrite,omitempty"`
	Final    string `json:"final,omitempty"`
}

type retrieveDebugRouteView struct {
	Route           string `json:"route"`
	FinalQuery      string `json:"final_query,omitempty"`
	RewriteStrategy string `json:"rewrite_strategy,omitempty"`
	Hits            int    `json:"hits"`
	Contribution    int    `json:"contribution"`
}

type retrieveDebugTopKView struct {
	CandidateTopK        int     `json:"candidate_topk"`
	FinalTopK            int     `json:"final_topk"`
	TokenBudget          int     `json:"token_budget"`
	TokenBudgetRemaining int     `json:"token_budget_remaining"`
	ContextTokens        int     `json:"context_tokens"`
	TruncateReason       string  `json:"truncate_reason,omitempty"`
	PolicyVersion        string  `json:"policy_version,omitempty"`
	ScoreDistribution    string  `json:"score_distribution,omitempty"`
	RerankGap            float64 `json:"rerank_gap,omitempty"`
	EvidenceDensity      float64 `json:"evidence_density,omitempty"`
	DecisionReason       string  `json:"decision_reason,omitempty"`
}

type retrieveDebugParentFillItem struct {
	ChunkID          string `json:"chunk_id,omitempty"`
	ParentID         string `json:"parent_id,omitempty"`
	Strategy         string `json:"strategy,omitempty"`
	Reason           string `json:"reason,omitempty"`
	Applied          bool   `json:"applied"`
	FillCount        int    `json:"fill_count"`
	FillTokens       int    `json:"fill_tokens"`
	BeforeContent    string `json:"before_content,omitempty"`
	AfterContent     string `json:"after_content,omitempty"`
	OriginalChildLen int    `json:"original_child_len"`
	FilledContentLen int    `json:"filled_content_len"`
}

type retrieveDebugParentChildView struct {
	Enabled       bool                          `json:"enabled"`
	Strategy      string                        `json:"strategy,omitempty"`
	FillCount     int                           `json:"fill_count"`
	FallbackCount int                           `json:"fallback_count"`
	FillTokens    int                           `json:"fill_tokens"`
	Items         []retrieveDebugParentFillItem `json:"items,omitempty"`
}

type retrieveDebugRewriteView struct {
	Applied             bool     `json:"applied"`
	Strategy            string   `json:"strategy,omitempty"`
	GainBucket          string   `json:"gain_bucket,omitempty"`
	DenseQuery          string   `json:"dense_query,omitempty"`
	SparseQuery         string   `json:"sparse_query,omitempty"`
	TermDictScope       string   `json:"term_dict_scope,omitempty"`
	TermDictVersion     string   `json:"term_dict_version,omitempty"`
	TermHits            []string `json:"term_hits,omitempty"`
	ModelApplied        bool     `json:"model_applied"`
	ModelShadow         bool     `json:"model_shadow"`
	ModelRiskLevel      string   `json:"model_risk_level,omitempty"`
	ModelTerms          []string `json:"model_terms,omitempty"`
	RouteDenseStrategy  string   `json:"route_dense_strategy,omitempty"`
	RouteSparseStrategy string   `json:"route_sparse_strategy,omitempty"`
}

type retrieveDebugEvidenceView struct {
	Result               string   `json:"result,omitempty"`
	RefusalReason        string   `json:"refusal_reason,omitempty"`
	CitationSupportScore float64  `json:"citation_support_score,omitempty"`
	UnsupportedClaims    []string `json:"unsupported_claims,omitempty"`
	UnsupportedCount     int      `json:"unsupported_count"`
	Error                string   `json:"error,omitempty"`
}

type retrieveDebugCitationView struct {
	Supported        bool    `json:"supported"`
	SupportScore     float64 `json:"support_score,omitempty"`
	UnsupportedCount int     `json:"unsupported_count"`
	Version          string  `json:"version,omitempty"`
	LatencyMs        int64   `json:"latency_ms,omitempty"`
	Error            string  `json:"error,omitempty"`
}

type retrieveDebugTrace struct {
	RequestID          string                       `json:"request_id"`
	Strategy           string                       `json:"strategy,omitempty"`
	ReleaseStage       string                       `json:"release_stage,omitempty"`
	ReleaseReason      string                       `json:"release_reason,omitempty"`
	ResultStatus       string                       `json:"result_status,omitempty"`
	EmptyReason        string                       `json:"empty_reason,omitempty"`
	FinalCount         int                          `json:"final_count"`
	Query              retrieveDebugQueryView       `json:"query"`
	Routes             []retrieveDebugRouteView     `json:"routes"`
	TopK               retrieveDebugTopKView        `json:"topk"`
	ParentChild        retrieveDebugParentChildView `json:"parent_child"`
	Rewrite            retrieveDebugRewriteView     `json:"rewrite"`
	EvidenceGate       retrieveDebugEvidenceView    `json:"evidence_gate"`
	CitationCheck      retrieveDebugCitationView    `json:"citation_check"`
	FinalItems         []retrieveItem               `json:"final_items,omitempty"`
	Collection         string                       `json:"collection,omitempty"`
	RetrieverVersion   string                       `json:"retriever_version,omitempty"`
	DenseHits          int                          `json:"dense_hits"`
	SparseHits         int                          `json:"sparse_hits"`
	DenseContribution  int                          `json:"dense_contribution"`
	SparseContribution int                          `json:"sparse_contribution"`
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

	// 获取 tenant_id
	var tenantID uint64
	if tid, ok := c.Get("tenant_id"); ok {
		switch v := tid.(type) {
		case uint64:
			tenantID = v
		case uint:
			tenantID = uint64(v)
		case float64:
			tenantID = uint64(v)
		}
	}

	kb := &model.KBKnowledgeBase{
		TenantID:    tenantID,
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

	// 自动授予创建者 admin 权限
	if tenantID > 0 {
		permRepo := repository.NewRAGTenantKBPermissionRepository(repository.GetDB())
		perm := &model.RAGTenantKBPermission{
			TenantID:   tenantID,
			KBID:       kb.ID,
			Permission: model.RAGTenantKBPermissionAdmin,
		}
		if err := permRepo.Create(perm); err != nil {
			log.Printf("[KB Create] Warning: failed to grant admin permission: kb_id=%d tenant_id=%d err=%v", kb.ID, tenantID, err)
		}
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

	publishErr := ragqueue.PublishKnowledgeIngest(ctx, ragqueue.KnowledgeIngestPayload{
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
	persistAuditEvent(&model.KBAuditEvent{
		AuditTraceID: fmt.Sprintf("audit-upload-%d-%d", job.ID, doc.ID),
		OperatorID:   userID,
		UserID:       userID,
		KBID:         kbID,
		DocumentID:   doc.ID,
		Action:       governance.ActionDocumentUpload,
		ResourceType: "document",
		ResourceID:   strconv.FormatUint(doc.ID, 10),
		AfterData:    fmt.Sprintf(`{"file_name":"%s","job_id":%d}`, doc.FileName, job.ID),
		Result:       "accepted",
	})

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

	var kbID *uint64
	kbIDRaw := strings.TrimSpace(string(c.Query("kb_id")))
	if kbIDRaw != "" {
		parsed, err := parseUint64(kbIDRaw, "kb_id")
		if err != nil {
			response.BadRequest(ctx, c, err.Error())
			return
		}
		kbID = &parsed
	}

	page, pageSize := getPagination(c)

	var items []*model.KBIngestJob
	var total int64
	var err error
	if kbID != nil {
		items, total, err = model.KBIngestJobDao.ListByKbID(*kbID, status, page, pageSize)
	} else {
		items, total, err = model.KBIngestJobDao.List(status, page, pageSize)
	}
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

	if err := ragqueue.PublishKnowledgeIngest(ctx, ragqueue.KnowledgeIngestPayload{
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
	persistAuditEvent(&model.KBAuditEvent{
		AuditTraceID: firstNonEmptyString("audit-delete-"+c.Param("document_id"), c.Param("document_id")),
		OperatorID:   userID,
		UserID:       userID,
		KBID:         mustDocumentKBID(documentID),
		DocumentID:   documentID,
		Action:       governance.ActionDocumentDelete,
		ResourceType: "document",
		ResourceID:   strconv.FormatUint(documentID, 10),
		Result:       "deleted",
		Reason:       getOperationReason(c),
	})

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
	phase2Available := config.Global.RAG.FeatureFlags.EnableHybridRetrieval && manager.GetHybridRetriever() != nil
	releaseDecision := release.Decide(config.Global.RAG, phase2Available, userID, middleware.GetUserRole(c))
	useHybrid := releaseDecision.UsePhase2 && manager.GetHybridRetriever() != nil

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
	expr := buildKBFilterExpr(kbIDs)
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
	if searchErr != nil {
		metricsStatus, metricsErrorCode = classifyRetrieveError(searchErr, retrieveCtx)
		metrics.ObserveRetrieveStrategy(searchResult.Metrics.Strategy, searchResult.Metrics.ReleaseStage, releaseDecision.Reason, metricsStatus)
		metrics.ObserveRetrieveEmptyReason(searchResult.Metrics.Strategy, searchResult.Metrics.ReleaseStage, firstNonEmptyString(searchResult.Metrics.EmptyReason, retrieval.EmptyReasonAfterRetrieve))
		errorStatus := classifyRetrieveResultStatus(metricsStatus)
		retrieveLog := &model.KBRetrieveLog{
			RequestID:              requestID,
			ExperimentID:           searchResult.Metrics.ExperimentID,
			ExperimentGroup:        searchResult.Metrics.ExperimentGroup,
			StrategyVersion:        searchResult.Metrics.StrategyVersion,
			IndexVersion:           searchResult.Metrics.IndexVersion,
			CollectionVersion:      searchResult.Metrics.CollectionVersion,
			CostTraceID:            searchResult.Metrics.CostTraceID,
			AuditTraceID:           searchResult.Metrics.AuditTraceID,
			ReleaseID:              searchResult.Metrics.ReleaseID,
			UserID:                 userID,
			KBIDs:                  formatKBIDs(kbIDs),
			Query:                  req.Query,
			FinalQuery:             firstNonEmptyString(searchResult.Metrics.FinalQuery, req.Query),
			Expr:                   expr,
			TopK:                   topK,
			CandidateTopK:          searchResult.Metrics.CandidateTopK,
			FinalTopK:              searchResult.Metrics.FinalTopK,
			TokenBudget:            searchResult.Metrics.TokenBudget,
			ContextTokens:          searchResult.Metrics.ContextTokens,
			QueryType:              queryType,
			TruncateReason:         searchResult.Metrics.TruncateReason,
			Rewrite:                searchResult.Metrics.RewriteQuery,
			RewriteStrategy:        searchResult.Metrics.RewriteStrategy,
			RewriteApplied:         searchResult.Metrics.RewriteApplied,
			Strategy:               searchResult.Metrics.Strategy,
			ReleaseStage:           searchResult.Metrics.ReleaseStage,
			ReleaseReason:          searchResult.Metrics.ReleaseReason,
			Routes:                 resolveRetrieveRoutes(useHybrid),
			Collection:             collection,
			RetrieverVersion:       searchResult.Metrics.RetrieverVersion,
			EmptyReason:            firstNonEmptyString(searchResult.Metrics.EmptyReason, retrieval.EmptyReasonAfterRetrieve),
			ParentChildEnabled:     searchResult.Metrics.ParentChildEnabled,
			ParentFillStrategy:     searchResult.Metrics.ParentFillStrategy,
			ParentFillCount:        searchResult.Metrics.ParentFillCount,
			ParentFillFallback:     searchResult.Metrics.ParentFillFallback,
			ParentFillTokens:       searchResult.Metrics.ParentFillTokens,
			TopKDecisionReason:     searchResult.Metrics.TopKDecisionReason,
			EvidenceGateResult:     searchResult.Metrics.EvidenceGateResult,
			RefusalReason:          searchResult.Metrics.RefusalReason,
			CitationSupported:      searchResult.Metrics.CitationSupported,
			CitationSupportScore:   searchResult.Metrics.CitationSupportScore,
			RewriteGainBucket:      classifyRewriteGainBucket(searchResult.Metrics, 0, errorStatus),
			UnsupportedClaimCount:  searchResult.Metrics.UnsupportedClaimCount,
			CitationCheckVersion:   searchResult.Metrics.CitationCheckVersion,
			CitationCheckLatencyMs: searchResult.Metrics.CitationCheckLatencyMs,
			EvidenceGateError:      searchResult.Metrics.EvidenceGateError,
			CitationCheckError:     searchResult.Metrics.CitationCheckError,
			ResultStatus:           errorStatus,
			ErrorCode:              metricsErrorCode,
			ErrorMsg:               searchErr.Error(),
			EmbeddingMs:            searchResult.Metrics.EmbeddingMs,
			SearchMs:               searchResult.Metrics.SearchMs,
			PostprocessMs:          searchResult.Metrics.PostprocessMs,
			RerankMs:               searchResult.Metrics.RerankMs,
			RerankModel:            searchResult.Metrics.RerankModel,
			DenseHits:              searchResult.Metrics.DenseHits,
			SparseHits:             searchResult.Metrics.SparseHits,
			DenseContribution:      searchResult.Metrics.DenseContribution,
			SparseContribution:     searchResult.Metrics.SparseContribution,
			DurationMs:             durationMs,
			TimeoutMs:              retrieveTimeout.Milliseconds(),
		}
		enrichRetrieveLogWithPlatformContext(ctx, c, retrieveLog, "allowed")
		retrieveLog.DebugTrace = encodeRetrievalDebugTraceResponse(buildRetrievalDebugTraceResponse(
			retrieveLog,
			searchResult.Metrics,
			nil,
			searchResult.Debug,
		))
		persistRetrieveLog(retrieveLog)
		persistCostTrace(buildRetrieveCostTrace(retrieveLog))
		response.ErrorFromErr(ctx, c, myerrors.NewMilvusError("knowledge retrieve failed", searchErr))
		return
	}

	docs := searchResult.Documents
	searchMetrics := searchResult.Metrics
	annotateRetrieveDocs(docs, collection, searchMetrics.RetrieverVersion, useHybrid)

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

		route := firstNonEmptyString(getStringMetadata(doc.MetaData, "route"), resolvePrimaryRoute(useHybrid))
		items = append(items, retrieveItem{
			Content: doc.Content,
			Score:   getFloat64Metadata(doc.MetaData, "score"),
			Citation: citation{
				KBID:          storedDoc.KbID,
				DocumentID:    documentID,
				ChunkID:       firstNonEmptyString(doc.ID, getStringMetadata(doc.MetaData, "chunk_id")),
				FileName:      firstNonEmptyString(getStringMetadata(doc.MetaData, "file_name"), storedDoc.FileName),
				ChunkIndex:    getIntMetadata(doc.MetaData, "chunk_index"),
				SnippetOffset: computeSnippetOffset(doc.Content, queryLower),
			},
			Source: source{
				Route:                route,
				Collection:           firstNonEmptyString(getStringMetadata(doc.MetaData, "collection"), collection),
				RetrieverVersion:     firstNonEmptyString(getStringMetadata(doc.MetaData, "retriever_version"), searchMetrics.RetrieverVersion),
				ParentID:             getStringMetadata(doc.MetaData, "parent_id"),
				ChildID:              firstNonEmptyString(getStringMetadata(doc.MetaData, "child_id"), firstNonEmptyString(doc.ID, getStringMetadata(doc.MetaData, "chunk_id"))),
				SectionTitle:         getStringMetadata(doc.MetaData, "section_title"),
				HierarchyPath:        getStringMetadata(doc.MetaData, "hierarchy_path"),
				ParentFillStrategy:   getStringMetadata(doc.MetaData, "parent_fill_strategy"),
				ParentFillTokens:     getIntMetadata(doc.MetaData, "parent_fill_tokens"),
				CitationSupported:    getBoolMetadata(doc.MetaData, "citation_supported"),
				CitationSupportScore: getFloat64Metadata(doc.MetaData, "citation_support_score"),
				CitationCheckVersion: getStringMetadata(doc.MetaData, "citation_check_version"),
				LowSupportCitation:   getBoolMetadata(doc.MetaData, "low_support_citation"),
			},
		})
	}

	evidenceOutcome := resolveEvidenceGateOutcome(req.Query, docs, searchMetrics)
	searchMetrics.EvidenceGateResult = evidenceOutcome.Result
	searchMetrics.RefusalReason = evidenceOutcome.RefusalReason
	searchMetrics.CitationSupportScore = evidenceOutcome.CitationSupportScore
	searchMetrics.EvidenceGateError = evidenceOutcome.Error
	citationCheck := buildCitationCheckResponse(searchMetrics)
	refusal := buildStandardRefusalPayload(evidenceOutcome)

	resultStatus := model.RetrieveResultStatusSuccess
	emptyReason := searchMetrics.EmptyReason
	if refusal != nil {
		items = []retrieveItem{}
		resultStatus = model.RetrieveResultStatusFilteredOut
		emptyReason = retrieval.EmptyReasonEvidenceRefusal
	} else if len(items) == 0 {
		if searchMetrics.HitCount > 0 {
			resultStatus = model.RetrieveResultStatusFilteredOut
			emptyReason = firstNonEmptyString(emptyReason, retrieval.EmptyReasonAfterFilter)
		} else {
			resultStatus = model.RetrieveResultStatusNoResult
			emptyReason = firstNonEmptyString(emptyReason, retrieval.EmptyReasonAfterRetrieve)
		}
	} else {
		emptyReason = firstNonEmptyString(emptyReason, retrieval.EmptyReasonNone)
	}
	searchMetrics.EmptyReason = emptyReason

	metrics.ObserveRetrieveStrategy(searchMetrics.Strategy, searchMetrics.ReleaseStage, searchMetrics.ReleaseReason, string(resultStatus))
	metrics.ObserveRetrieveEmptyReason(searchMetrics.Strategy, searchMetrics.ReleaseStage, emptyReason)
	metrics.ObserveRetrieveRewrite(searchMetrics.Strategy, searchMetrics.ReleaseStage, searchMetrics.RewriteApplied || extractRewriteApplied(docs))
	if searchMetrics.RerankMs > 0 {
		metrics.ObserveRetrieveRerank(searchMetrics.Strategy, searchMetrics.ReleaseStage, firstNonEmptyString(searchMetrics.RerankModel, "none"), "ok", time.Duration(searchMetrics.RerankMs)*time.Millisecond)
	}
	metrics.ObserveRetrieveRouteContribution("dense", searchMetrics.Strategy, searchMetrics.ReleaseStage, countRoute(items, "dense"))
	metrics.ObserveRetrieveRouteContribution("sparse", searchMetrics.Strategy, searchMetrics.ReleaseStage, countRoute(items, "sparse"))

	retrieveLog := &model.KBRetrieveLog{
		RequestID:              requestID,
		ExperimentID:           searchMetrics.ExperimentID,
		ExperimentGroup:        searchMetrics.ExperimentGroup,
		StrategyVersion:        searchMetrics.StrategyVersion,
		IndexVersion:           searchMetrics.IndexVersion,
		CollectionVersion:      searchMetrics.CollectionVersion,
		CostTraceID:            searchMetrics.CostTraceID,
		AuditTraceID:           searchMetrics.AuditTraceID,
		ReleaseID:              searchMetrics.ReleaseID,
		UserID:                 userID,
		KBIDs:                  formatKBIDs(kbIDs),
		Query:                  req.Query,
		FinalQuery:             firstNonEmptyString(searchMetrics.FinalQuery, extractFinalQuery(docs), req.Query),
		Expr:                   expr,
		TopK:                   topK,
		CandidateTopK:          searchMetrics.CandidateTopK,
		FinalTopK:              searchMetrics.FinalTopK,
		TokenBudget:            searchMetrics.TokenBudget,
		ContextTokens:          searchMetrics.ContextTokens,
		QueryType:              queryType,
		TruncateReason:         searchMetrics.TruncateReason,
		Rewrite:                firstNonEmptyString(searchMetrics.RewriteQuery, extractRewriteQuery(docs)),
		RewriteStrategy:        firstNonEmptyString(searchMetrics.RewriteStrategy, extractRewriteStrategy(docs)),
		RewriteApplied:         searchMetrics.RewriteApplied || extractRewriteApplied(docs),
		Strategy:               searchMetrics.Strategy,
		ReleaseStage:           searchMetrics.ReleaseStage,
		ReleaseReason:          searchMetrics.ReleaseReason,
		Routes:                 resolveRetrieveRoutes(useHybrid),
		Collection:             collection,
		RetrieverVersion:       searchMetrics.RetrieverVersion,
		EmptyReason:            emptyReason,
		ParentChildEnabled:     searchMetrics.ParentChildEnabled,
		ParentFillStrategy:     searchMetrics.ParentFillStrategy,
		ParentFillCount:        searchMetrics.ParentFillCount,
		ParentFillFallback:     searchMetrics.ParentFillFallback,
		ParentFillTokens:       searchMetrics.ParentFillTokens,
		TopKDecisionReason:     searchMetrics.TopKDecisionReason,
		FinalCount:             len(items),
		TruncatedCount:         searchMetrics.TruncatedCount,
		DenseHits:              searchMetrics.DenseHits,
		SparseHits:             searchMetrics.SparseHits,
		DenseContribution:      searchMetrics.DenseContribution,
		SparseContribution:     searchMetrics.SparseContribution,
		EvidenceGateResult:     searchMetrics.EvidenceGateResult,
		RefusalReason:          searchMetrics.RefusalReason,
		CitationSupported:      searchMetrics.CitationSupported,
		CitationSupportScore:   searchMetrics.CitationSupportScore,
		RewriteGainBucket:      classifyRewriteGainBucket(searchMetrics, len(items), resultStatus),
		UnsupportedClaimCount:  searchMetrics.UnsupportedClaimCount,
		CitationCheckVersion:   searchMetrics.CitationCheckVersion,
		CitationCheckLatencyMs: searchMetrics.CitationCheckLatencyMs,
		EvidenceGateError:      searchMetrics.EvidenceGateError,
		CitationCheckError:     searchMetrics.CitationCheckError,
		ResultStatus:           resultStatus,
		EmbeddingMs:            searchMetrics.EmbeddingMs,
		SearchMs:               searchMetrics.SearchMs,
		PostprocessMs:          searchMetrics.PostprocessMs,
		RerankMs:               searchMetrics.RerankMs,
		RerankModel:            searchMetrics.RerankModel,
		DurationMs:             durationMs,
		TimeoutMs:              retrieveTimeout.Milliseconds(),
	}
	enrichRetrieveLogWithPlatformContext(ctx, c, retrieveLog, "allowed")
	retrieveLog.DebugTrace = encodeRetrievalDebugTraceResponse(buildRetrievalDebugTraceResponse(
		retrieveLog,
		searchMetrics,
		items,
		searchResult.Debug,
	))
	persistRetrieveLog(retrieveLog)
	persistCostTrace(buildRetrieveCostTrace(retrieveLog))

	if config.Global.RAG.FeatureFlags.EnableRetrieveAudit {
		log.Printf(
			"[KB Retrieve] source_api=legacy_kb request_id=%s strategy=%s release_stage=%s release_reason=%q query=%q final_query=%q rewrite=%q rewrite_strategy=%q rewrite_applied=%t rewrite_gain_bucket=%q user_id=%d kb_ids=%v kb_scope=%q expr=%q topk=%d candidate_topk=%d final_topk=%d token_budget=%d truncate_reason=%q topk_decision_reason=%q parent_child_enabled=%t parent_fill_strategy=%q parent_fill_count=%d evidence_gate_result=%q refusal_reason=%q citation_supported=%t citation_support_score=%.4f unsupported_claim_count=%d citation_check_version=%q citation_check_latency_ms=%d citation_check_error=%q evidence_gate_error=%q routes=%q final_count=%d hit_count=%d truncated_count=%d empty_reason=%s dense_hits=%d sparse_hits=%d dense_contrib=%d sparse_contrib=%d rerank_ms=%d duration_ms=%d embedding_ms=%d search_ms=%d postprocess_ms=%d timeout_ms=%d result_status=%s",
			requestID,
			retrieveLog.Strategy,
			retrieveLog.ReleaseStage,
			retrieveLog.ReleaseReason,
			req.Query,
			retrieveLog.FinalQuery,
			retrieveLog.Rewrite,
			retrieveLog.RewriteStrategy,
			retrieveLog.RewriteApplied,
			retrieveLog.RewriteGainBucket,
			userID,
			kbIDs,
			"global",
			expr,
			topK,
			searchMetrics.CandidateTopK,
			searchMetrics.FinalTopK,
			searchMetrics.TokenBudget,
			searchMetrics.TruncateReason,
			retrieveLog.TopKDecisionReason,
			retrieveLog.ParentChildEnabled,
			retrieveLog.ParentFillStrategy,
			retrieveLog.ParentFillCount,
			retrieveLog.EvidenceGateResult,
			retrieveLog.RefusalReason,
			retrieveLog.CitationSupported,
			retrieveLog.CitationSupportScore,
			retrieveLog.UnsupportedClaimCount,
			retrieveLog.CitationCheckVersion,
			retrieveLog.CitationCheckLatencyMs,
			retrieveLog.CitationCheckError,
			retrieveLog.EvidenceGateError,
			retrieveLog.Routes,
			len(items),
			searchMetrics.HitCount,
			searchMetrics.TruncatedCount,
			retrieveLog.EmptyReason,
			retrieveLog.DenseHits,
			retrieveLog.SparseHits,
			retrieveLog.DenseContribution,
			retrieveLog.SparseContribution,
			retrieveLog.RerankMs,
			durationMs,
			searchMetrics.EmbeddingMs,
			searchMetrics.SearchMs,
			searchMetrics.PostprocessMs,
			retrieveTimeout.Milliseconds(),
			string(resultStatus),
		)
	}

	metricsResultCount = len(items)
	response.Success(ctx, c, retrieveResponse{
		RequestID:          requestID,
		Items:              items,
		EvidenceGateResult: searchMetrics.EvidenceGateResult,
		CitationCheck:      citationCheck,
		Refusal:            refusal,
	})
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

func resolveRetrieveRoutes(useHybrid bool) string {
	if useHybrid {
		return "dense+sparse"
	}
	return "dense"
}

func resolvePrimaryRoute(useHybrid bool) string {
	if useHybrid {
		return "hybrid"
	}
	return "dense"
}

func extractFinalQuery(docs []*schema.Document) string {
	return getStringMetadataFromDocs(docs, "final_query")
}

func extractRewriteQuery(docs []*schema.Document) string {
	return getStringMetadataFromDocs(docs, "rewrite_query")
}

func extractRewriteStrategy(docs []*schema.Document) string {
	return getStringMetadataFromDocs(docs, "rewrite_strategy")
}

func extractRewriteApplied(docs []*schema.Document) bool {
	for _, doc := range docs {
		if doc == nil || doc.MetaData == nil {
			continue
		}
		if value, ok := doc.MetaData["rewrite_applied"]; ok {
			switch v := value.(type) {
			case bool:
				return v
			case string:
				return strings.EqualFold(strings.TrimSpace(v), "true")
			}
		}
	}
	return false
}

func getStringMetadataFromDocs(docs []*schema.Document, key string) string {
	for _, doc := range docs {
		if value := getStringMetadata(doc.MetaData, key); value != "" {
			return value
		}
	}
	return ""
}

func classifyRewriteGainBucket(metrics retrieval.SearchMetrics, finalCount int, resultStatus model.RetrieveResultStatus) string {
	if !metrics.RewriteApplied {
		return "not_applied"
	}
	switch resultStatus {
	case model.RetrieveResultStatusError, model.RetrieveResultStatusTimeout:
		return "error"
	case model.RetrieveResultStatusFilteredOut:
		if strings.EqualFold(metrics.EvidenceGateResult, retrieval.EvidenceGateResultRefused) {
			return "risk_refusal"
		}
		return "risk_filtered"
	}
	if finalCount <= 0 {
		return "risk_no_result"
	}
	if metrics.ModelRewriteApplied {
		return "model_gain_candidate"
	}
	if metrics.RouteRewriteDense != "" || metrics.RouteRewriteSparse != "" {
		return "route_gain_candidate"
	}
	return "gain_candidate"
}

func buildRetrieveDebugTrace(
	logEntry *model.KBRetrieveLog,
	searchMetrics retrieval.SearchMetrics,
	docs []*schema.Document,
	items []retrieveItem,
) retrieveDebugTrace {
	trace := retrieveDebugTrace{
		Query: retrieveDebugQueryView{
			Original: firstNonEmptyString(searchMetrics.OriginalQuery, searchMetrics.FinalQuery, searchMetrics.RewriteQuery, func() string {
				if logEntry != nil {
					return logEntry.Query
				}
				return ""
			}()),
			Rewrite: firstNonEmptyString(searchMetrics.RewriteQuery, func() string {
				if logEntry != nil {
					return logEntry.Rewrite
				}
				return ""
			}()),
			Final: firstNonEmptyString(searchMetrics.FinalQuery, func() string {
				if logEntry != nil {
					return logEntry.FinalQuery
				}
				return ""
			}()),
		},
		Routes: []retrieveDebugRouteView{
			{
				Route:           "dense",
				FinalQuery:      firstNonEmptyString(searchMetrics.DenseQuery, searchMetrics.FinalQuery),
				RewriteStrategy: searchMetrics.RouteRewriteDense,
				Hits:            searchMetrics.DenseHits,
				Contribution:    searchMetrics.DenseContribution,
			},
			{
				Route:           "sparse",
				FinalQuery:      firstNonEmptyString(searchMetrics.SparseQuery, searchMetrics.FinalQuery),
				RewriteStrategy: searchMetrics.RouteRewriteSparse,
				Hits:            searchMetrics.SparseHits,
				Contribution:    searchMetrics.SparseContribution,
			},
		},
		TopK: retrieveDebugTopKView{
			CandidateTopK:        searchMetrics.CandidateTopK,
			FinalTopK:            searchMetrics.FinalTopK,
			TokenBudget:          searchMetrics.TokenBudget,
			TokenBudgetRemaining: searchMetrics.TokenBudgetRemain,
			ContextTokens:        searchMetrics.ContextTokens,
			TruncateReason:       searchMetrics.TruncateReason,
			PolicyVersion:        searchMetrics.TopKPolicyVersion,
			ScoreDistribution:    searchMetrics.ScoreDistribution,
			RerankGap:            searchMetrics.RerankGap,
			EvidenceDensity:      searchMetrics.EvidenceDensity,
			DecisionReason: firstNonEmptyString(searchMetrics.TopKDecisionReason, func() string {
				if logEntry != nil {
					return logEntry.TopKDecisionReason
				}
				return ""
			}()),
		},
		ParentChild: retrieveDebugParentChildView{
			Enabled: searchMetrics.ParentChildEnabled,
			Strategy: firstNonEmptyString(searchMetrics.ParentFillStrategy, func() string {
				if logEntry != nil {
					return logEntry.ParentFillStrategy
				}
				return ""
			}()),
			FillCount: maxInt(searchMetrics.ParentFillCount, func() int {
				if logEntry != nil {
					return logEntry.ParentFillCount
				}
				return 0
			}()),
			FallbackCount: maxInt(searchMetrics.ParentFillFallback, func() int {
				if logEntry != nil {
					return logEntry.ParentFillFallback
				}
				return 0
			}()),
			FillTokens: maxInt(searchMetrics.ParentFillTokens, func() int {
				if logEntry != nil {
					return logEntry.ParentFillTokens
				}
				return 0
			}()),
			Items: buildParentFillDebugItems(docs),
		},
		Rewrite: retrieveDebugRewriteView{
			Applied: searchMetrics.RewriteApplied,
			Strategy: firstNonEmptyString(searchMetrics.RewriteStrategy, func() string {
				if logEntry != nil {
					return logEntry.RewriteStrategy
				}
				return ""
			}()),
			GainBucket: func() string {
				if logEntry != nil {
					return logEntry.RewriteGainBucket
				}
				return ""
			}(),
			DenseQuery:          searchMetrics.DenseQuery,
			SparseQuery:         searchMetrics.SparseQuery,
			TermDictScope:       searchMetrics.TermDictScope,
			TermDictVersion:     searchMetrics.TermDictVersion,
			TermHits:            append([]string(nil), searchMetrics.TermHits...),
			ModelApplied:        searchMetrics.ModelRewriteApplied,
			ModelShadow:         searchMetrics.ModelRewriteShadow,
			ModelRiskLevel:      searchMetrics.ModelRewriteRiskLevel,
			ModelTerms:          append([]string(nil), searchMetrics.ModelRewriteTerms...),
			RouteDenseStrategy:  searchMetrics.RouteRewriteDense,
			RouteSparseStrategy: searchMetrics.RouteRewriteSparse,
		},
		EvidenceGate: retrieveDebugEvidenceView{
			Result: firstNonEmptyString(searchMetrics.EvidenceGateResult, func() string {
				if logEntry != nil {
					return logEntry.EvidenceGateResult
				}
				return ""
			}()),
			RefusalReason: firstNonEmptyString(searchMetrics.RefusalReason, func() string {
				if logEntry != nil {
					return logEntry.RefusalReason
				}
				return ""
			}()),
			CitationSupportScore: firstNonEmptyFloat(searchMetrics.CitationSupportScore, func() float64 {
				if logEntry != nil {
					return logEntry.CitationSupportScore
				}
				return 0
			}()),
			UnsupportedClaims: append([]string(nil), searchMetrics.UnsupportedClaims...),
			UnsupportedCount: maxInt(searchMetrics.UnsupportedClaimCount, func() int {
				if logEntry != nil {
					return logEntry.UnsupportedClaimCount
				}
				return 0
			}()),
			Error: firstNonEmptyString(searchMetrics.EvidenceGateError, func() string {
				if logEntry != nil {
					return logEntry.EvidenceGateError
				}
				return ""
			}()),
		},
		CitationCheck: retrieveDebugCitationView{
			Supported: searchMetrics.CitationSupported,
			SupportScore: firstNonEmptyFloat(searchMetrics.CitationSupportScore, func() float64 {
				if logEntry != nil {
					return logEntry.CitationSupportScore
				}
				return 0
			}()),
			UnsupportedCount: maxInt(searchMetrics.UnsupportedClaimCount, func() int {
				if logEntry != nil {
					return logEntry.UnsupportedClaimCount
				}
				return 0
			}()),
			Version: firstNonEmptyString(searchMetrics.CitationCheckVersion, func() string {
				if logEntry != nil {
					return logEntry.CitationCheckVersion
				}
				return ""
			}()),
			LatencyMs: maxInt64(searchMetrics.CitationCheckLatencyMs, func() int64 {
				if logEntry != nil {
					return logEntry.CitationCheckLatencyMs
				}
				return 0
			}()),
			Error: firstNonEmptyString(searchMetrics.CitationCheckError, func() string {
				if logEntry != nil {
					return logEntry.CitationCheckError
				}
				return ""
			}()),
		},
		FinalItems: items,
	}
	if logEntry != nil {
		trace.RequestID = logEntry.RequestID
		trace.Strategy = logEntry.Strategy
		trace.ReleaseStage = logEntry.ReleaseStage
		trace.ReleaseReason = logEntry.ReleaseReason
		trace.ResultStatus = string(logEntry.ResultStatus)
		trace.EmptyReason = logEntry.EmptyReason
		trace.FinalCount = logEntry.FinalCount
		trace.Collection = logEntry.Collection
		trace.RetrieverVersion = logEntry.RetrieverVersion
		trace.DenseHits = logEntry.DenseHits
		trace.SparseHits = logEntry.SparseHits
		trace.DenseContribution = logEntry.DenseContribution
		trace.SparseContribution = logEntry.SparseContribution
	}
	if trace.RequestID == "" {
		trace.RequestID = "unknown"
	}
	return trace
}

func buildParentFillDebugItems(docs []*schema.Document) []retrieveDebugParentFillItem {
	if len(docs) == 0 {
		return nil
	}
	items := make([]retrieveDebugParentFillItem, 0, len(docs))
	for _, doc := range docs {
		if doc == nil || doc.MetaData == nil {
			continue
		}
		before := getStringMetadata(doc.MetaData, "original_child_content")
		after := strings.TrimSpace(doc.Content)
		items = append(items, retrieveDebugParentFillItem{
			ChunkID:          firstNonEmptyString(getStringMetadata(doc.MetaData, "child_id"), getStringMetadata(doc.MetaData, "chunk_id"), doc.ID),
			ParentID:         getStringMetadata(doc.MetaData, "parent_id"),
			Strategy:         getStringMetadata(doc.MetaData, "parent_fill_strategy"),
			Reason:           getStringMetadata(doc.MetaData, "parent_fill_reason"),
			Applied:          getBoolMetadata(doc.MetaData, "parent_fill_applied"),
			FillCount:        getIntMetadata(doc.MetaData, "parent_fill_count"),
			FillTokens:       getIntMetadata(doc.MetaData, "parent_fill_tokens"),
			BeforeContent:    before,
			AfterContent:     after,
			OriginalChildLen: len([]rune(before)),
			FilledContentLen: len([]rune(after)),
		})
	}
	return items
}

func encodeRetrieveDebugTrace(trace retrieveDebugTrace) string {
	data, err := json.Marshal(trace)
	if err != nil {
		return ""
	}
	return string(data)
}

func decodeRetrieveDebugTrace(raw string) (*retrieveDebugTrace, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var trace retrieveDebugTrace
	if err := json.Unmarshal([]byte(raw), &trace); err != nil {
		return nil, err
	}
	return &trace, nil
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

func parseOptionalRFC3339Query(c *app.RequestContext, field string) (*time.Time, error) {
	raw := strings.TrimSpace(string(c.Query(field)))
	if raw == "" {
		return nil, nil
	}

	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be a valid RFC3339 timestamp", field)
	}

	parsed = parsed.UTC()
	return &parsed, nil
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

func getBoolMetadata(metadata map[string]interface{}, key string) bool {
	if metadata == nil {
		return false
	}
	value, ok := metadata[key]
	if !ok || value == nil {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return strings.EqualFold(strings.TrimSpace(fmt.Sprint(v)), "true")
	}
}

func buildCitationCheckResponse(metrics retrieval.SearchMetrics) *citationCheckResponse {
	if metrics.CitationCheckVersion == "" && metrics.CitationCheckLatencyMs == 0 && metrics.CitationCheckError == "" && metrics.UnsupportedClaimCount == 0 {
		return nil
	}
	return &citationCheckResponse{
		Supported:             metrics.CitationSupported,
		SupportScore:          metrics.CitationSupportScore,
		UnsupportedClaims:     append([]string(nil), metrics.UnsupportedClaims...),
		UnsupportedClaimCount: metrics.UnsupportedClaimCount,
		Version:               metrics.CitationCheckVersion,
		LatencyMs:             metrics.CitationCheckLatencyMs,
		Error:                 metrics.CitationCheckError,
	}
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

func firstNonEmptyFloat(primary float64, fallback float64) float64 {
	if primary > 0 {
		return primary
	}
	return fallback
}

func maxInt(a int, b int) int {
	if b > a {
		return b
	}
	return a
}

func maxInt64(a int64, b int64) int64 {
	if b > a {
		return b
	}
	return a
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
	if repository.GetDB() == nil {
		log.Printf("[KB Retrieve Audit] skip persist because db is not initialized request_id=%s", entry.RequestID)
		return
	}
	go func() {
		if err := model.KBRetrieveLogDao.Create(entry); err != nil {
			log.Printf("[KB Retrieve Audit] failed to persist retrieve log request_id=%s err=%v", entry.RequestID, err)
			metrics.IncError("retrieve_audit", "persist_failed")
			traceID := firstNonEmptyString(entry.AuditTraceID, entry.RequestID)
			governance.EnqueueCompensation(
				"retrieve_audit",
				traceID,
				entry.RequestID,
				"persist_retrieve_log_failed",
				map[string]interface{}{
					"request_id":       entry.RequestID,
					"audit_trace_id":   entry.AuditTraceID,
					"strategy_version": entry.StrategyVersion,
				},
			)
		}
	}()
}

func persistCostTrace(entry *model.KBCostTrace) {
	if entry == nil {
		return
	}
	if repository.GetDB() == nil {
		log.Printf("[KB Cost] skip persist because db is not initialized cost_trace_id=%s", entry.CostTraceID)
		return
	}
	go func() {
		if err := model.KBCostTraceDao.Create(entry); err != nil {
			log.Printf("[KB Cost] failed to persist cost trace cost_trace_id=%s err=%v", entry.CostTraceID, err)
			metrics.IncError("cost_trace", "persist_failed")
		}
	}()
}

func persistAuditEvent(entry *model.KBAuditEvent) {
	if entry == nil {
		return
	}
	if repository.GetDB() == nil {
		log.Printf("[KB Audit] skip persist because db is not initialized audit_trace_id=%s action=%s", entry.AuditTraceID, entry.Action)
		return
	}
	go func() {
		if err := model.KBAuditEventDao.Create(entry); err != nil {
			log.Printf("[KB Audit] failed to persist audit event audit_trace_id=%s action=%s err=%v", entry.AuditTraceID, entry.Action, err)
			metrics.IncError("audit_event", "persist_failed")
			governance.EnqueueCompensation(
				"audit_event",
				firstNonEmptyString(entry.AuditTraceID, entry.RequestID),
				entry.RequestID,
				"persist_audit_event_failed",
				map[string]interface{}{
					"action":        entry.Action,
					"resource_type": entry.ResourceType,
					"resource_id":   entry.ResourceID,
				},
			)
		}
	}()
}

func enrichRetrieveLogWithPlatformContext(ctx context.Context, c *app.RequestContext, entry *model.KBRetrieveLog, permissionResult string) {
	if entry == nil {
		return
	}

	identity := authpkg.GetIdentity(ctx)
	if identity.TenantID > 0 {
		entry.TenantID = identity.TenantID
	}
	if identity.AppID != "" {
		entry.AppID = identity.AppID
	}
	if identity.APIKeyID > 0 {
		entry.APIKeyID = identity.APIKeyID
	}
	if identity.AuthType != "" {
		entry.AuthType = string(identity.AuthType)
	}
	if identity.IsLegacy {
		entry.IsLegacy = true
	}

	if tenantID, ok := c.Get("tenant_id"); ok && entry.TenantID == 0 {
		switch v := tenantID.(type) {
		case uint64:
			entry.TenantID = v
		case uint:
			entry.TenantID = uint64(v)
		case int:
			if v > 0 {
				entry.TenantID = uint64(v)
			}
		}
	}
	if appID, ok := c.Get("app_id"); ok && entry.AppID == "" {
		if v, ok := appID.(string); ok {
			entry.AppID = v
		}
	}
	if apiKeyID, ok := c.Get("api_key_id"); ok && entry.APIKeyID == 0 {
		switch v := apiKeyID.(type) {
		case uint64:
			entry.APIKeyID = v
		case uint:
			entry.APIKeyID = uint64(v)
		case int:
			if v > 0 {
				entry.APIKeyID = uint64(v)
			}
		}
	}
	if authType, ok := c.Get("auth_type"); ok && entry.AuthType == "" {
		if v, ok := authType.(string); ok {
			entry.AuthType = v
		}
	}
	if isLegacy, ok := c.Get("is_legacy"); ok && !entry.IsLegacy {
		if v, ok := isLegacy.(bool); ok {
			entry.IsLegacy = v
		}
	}

	if permissionResult != "" {
		entry.PermissionResult = permissionResult
	}

	path := string(c.Path())
	switch {
	case strings.HasPrefix(path, "/v1/retrieve"):
		entry.SourceAPI = "v1"
	case path != "":
		entry.SourceAPI = "legacy_kb"
	}
}

func buildRetrieveCostTrace(logEntry *model.KBRetrieveLog) *model.KBCostTrace {
	if logEntry == nil {
		return nil
	}
	kbID := firstKBIDFromCSV(logEntry.KBIDs)
	embeddingTokens := maxInt(logEntry.CandidateTopK*24, 0)
	completionTokens := maxInt(logEntry.FinalCount*48, 0)
	retrievalCost := float64(logEntry.DenseHits+logEntry.SparseHits) * 0.00002
	rerankCost := float64(maxInt(logEntry.CandidateTopK, logEntry.FinalTopK)) * 0.00003
	embeddingCost := float64(embeddingTokens) * 0.0000008
	llmCost := float64(logEntry.ContextTokens+completionTokens) * 0.0000015
	vectorStorageCost := float64(maxInt(logEntry.FinalCount, 1)) * 0.000005
	totalCost := embeddingCost + retrievalCost + rerankCost + llmCost + vectorStorageCost

	return &model.KBCostTrace{
		RequestID:               logEntry.RequestID,
		CostTraceID:             firstNonEmptyString(logEntry.CostTraceID, logEntry.RequestID),
		KBID:                    kbID,
		UserID:                  logEntry.UserID,
		ExperimentID:            logEntry.ExperimentID,
		StrategyVersion:         logEntry.StrategyVersion,
		QueryType:               logEntry.QueryType,
		EmbeddingTokens:         embeddingTokens,
		ContextTokens:           logEntry.ContextTokens,
		CompletionTokens:        completionTokens,
		RetrievalCandidateCount: logEntry.DenseHits + logEntry.SparseHits,
		RerankCandidateCount:    maxInt(logEntry.CandidateTopK, logEntry.FinalTopK),
		LLMModel:                firstNonEmptyString(logEntry.RerankModel, "rag-answer-estimator"),
		EmbeddingCost:           embeddingCost,
		RetrievalCost:           retrievalCost,
		RerankCost:              rerankCost,
		LLMCost:                 llmCost,
		VectorStorageCost:       vectorStorageCost,
		TotalCost:               totalCost,
	}
}

func firstKBIDFromCSV(raw string) uint64 {
	for _, part := range strings.Split(strings.TrimSpace(raw), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		value, err := strconv.ParseUint(part, 10, 64)
		if err == nil {
			return value
		}
	}
	return 0
}

func mustDocumentKBID(documentID uint64) uint64 {
	if documentID == 0 {
		return 0
	}
	doc, err := model.KBDocumentDao.GetByID(documentID)
	if err != nil || doc == nil {
		return 0
	}
	return doc.KbID
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

func annotateRetrieveDocs(docs []*schema.Document, collection, retrieverVersion string, useHybrid bool) {
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		if doc.MetaData == nil {
			doc.MetaData = make(map[string]interface{})
		}
		doc.MetaData["collection"] = firstNonEmptyString(getStringMetadata(doc.MetaData, "collection"), collection)
		doc.MetaData["retriever_version"] = firstNonEmptyString(getStringMetadata(doc.MetaData, "retriever_version"), retrieverVersion)
		if getStringMetadata(doc.MetaData, "route") == "" {
			if useHybrid {
				doc.MetaData["route"] = "hybrid"
			} else {
				doc.MetaData["route"] = "dense"
			}
		}
	}
}

func countRoute(items []retrieveItem, route string) int {
	total := 0
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Source.Route), route) {
			total++
		}
	}
	return total
}

func percentileInt64(values []int64, q float64) int64 {
	if len(values) == 0 {
		return 0
	}
	if q <= 0 {
		q = 0
	}
	if q >= 1 {
		q = 1
	}
	sorted := append([]int64(nil), values...)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] < sorted[i] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	index := int(float64(len(sorted)-1) * q)
	return sorted[index]
}

func resolveMetricsOverviewRange(rangeName string) (time.Duration, time.Duration, int, bool) {
	switch rangeName {
	case "1h":
		return time.Hour, 5 * time.Minute, 12, true
	case "24h":
		return 24 * time.Hour, time.Hour, 24, true
	case "7d":
		return 7 * 24 * time.Hour, 6 * time.Hour, 28, true
	default:
		return 0, 0, 0, false
	}
}

func alignTimeBucket(ts time.Time, bucketSize time.Duration) time.Time {
	if bucketSize <= 0 {
		return ts.UTC()
	}

	utc := ts.UTC()
	seconds := int64(bucketSize / time.Second)
	if seconds <= 0 {
		return utc
	}
	return time.Unix((utc.Unix()/seconds)*seconds, 0).UTC()
}

func buildIngestSuccessRateSeries(jobs []*model.KBIngestJob, start time.Time, bucketSize time.Duration, bucketCount int) []metricsOverviewBucketRate {
	if len(jobs) == 0 {
		return []metricsOverviewBucketRate{}
	}

	type aggregate struct {
		total   int
		success int
	}

	aggregates := make([]aggregate, bucketCount)
	for _, job := range jobs {
		if job == nil {
			continue
		}
		index := bucketIndex(job.CreatedAt.UTC(), start, bucketSize, bucketCount)
		if index < 0 {
			continue
		}
		switch job.Status {
		case model.KBIngestJobStatusCompleted:
			aggregates[index].total++
			aggregates[index].success++
		case model.KBIngestJobStatusFailed, model.KBIngestJobStatusDead:
			aggregates[index].total++
		}
	}

	series := make([]metricsOverviewBucketRate, 0, bucketCount)
	for i := 0; i < bucketCount; i++ {
		rate := 0.0
		if aggregates[i].total > 0 {
			rate = float64(aggregates[i].success) / float64(aggregates[i].total)
		}
		series = append(series, metricsOverviewBucketRate{
			Bucket:  start.Add(time.Duration(i) * bucketSize),
			Rate:    rate,
			Total:   aggregates[i].total,
			Success: aggregates[i].success,
		})
	}
	return series
}

func buildRetrieveRequestCountSeries(logs []*model.KBRetrieveLog, start time.Time, bucketSize time.Duration, bucketCount int) []metricsOverviewBucketCount {
	if len(logs) == 0 {
		return []metricsOverviewBucketCount{}
	}

	counts := make([]int, bucketCount)
	for _, logEntry := range logs {
		if logEntry == nil {
			continue
		}
		index := bucketIndex(logEntry.CreatedAt.UTC(), start, bucketSize, bucketCount)
		if index >= 0 {
			counts[index]++
		}
	}

	series := make([]metricsOverviewBucketCount, 0, bucketCount)
	for i := 0; i < bucketCount; i++ {
		series = append(series, metricsOverviewBucketCount{
			Bucket: start.Add(time.Duration(i) * bucketSize),
			Count:  counts[i],
		})
	}
	return series
}

func buildRetrieveP95Series(logs []*model.KBRetrieveLog, start time.Time, bucketSize time.Duration, bucketCount int) []metricsOverviewBucketP95 {
	if len(logs) == 0 {
		return []metricsOverviewBucketP95{}
	}

	valuesByBucket := make([][]int64, bucketCount)
	for _, logEntry := range logs {
		if logEntry == nil {
			continue
		}
		index := bucketIndex(logEntry.CreatedAt.UTC(), start, bucketSize, bucketCount)
		if index >= 0 {
			valuesByBucket[index] = append(valuesByBucket[index], logEntry.DurationMs)
		}
	}

	series := make([]metricsOverviewBucketP95, 0, bucketCount)
	for i := 0; i < bucketCount; i++ {
		series = append(series, metricsOverviewBucketP95{
			Bucket: start.Add(time.Duration(i) * bucketSize),
			P95Ms:  percentileInt64(valuesByBucket[i], 0.95),
		})
	}
	return series
}

func buildRetrieveEmptyRateSeries(logs []*model.KBRetrieveLog, start time.Time, bucketSize time.Duration, bucketCount int) []metricsOverviewBucketRate {
	if len(logs) == 0 {
		return []metricsOverviewBucketRate{}
	}

	type aggregate struct {
		total int
		empty int
	}

	aggregates := make([]aggregate, bucketCount)
	for _, logEntry := range logs {
		if logEntry == nil {
			continue
		}
		index := bucketIndex(logEntry.CreatedAt.UTC(), start, bucketSize, bucketCount)
		if index < 0 {
			continue
		}
		aggregates[index].total++
		if logEntry.ResultStatus == model.RetrieveResultStatusNoResult {
			aggregates[index].empty++
		}
	}

	series := make([]metricsOverviewBucketRate, 0, bucketCount)
	for i := 0; i < bucketCount; i++ {
		rate := 0.0
		if aggregates[i].total > 0 {
			rate = float64(aggregates[i].empty) / float64(aggregates[i].total)
		}
		series = append(series, metricsOverviewBucketRate{
			Bucket: start.Add(time.Duration(i) * bucketSize),
			Rate:   rate,
			Total:  aggregates[i].total,
			Empty:  aggregates[i].empty,
		})
	}
	return series
}

func buildParentFillAppliedRateSeries(logs []*model.KBRetrieveLog, start time.Time, bucketSize time.Duration, bucketCount int) []metricsOverviewBucketRate {
	return buildBucketRateSeries(logs, start, bucketSize, bucketCount, func(logEntry *model.KBRetrieveLog) bool {
		return logEntry.ParentFillCount > 0
	})
}

func buildEvidenceRefusalRateSeries(logs []*model.KBRetrieveLog, start time.Time, bucketSize time.Duration, bucketCount int) []metricsOverviewBucketRate {
	return buildBucketRateSeries(logs, start, bucketSize, bucketCount, func(logEntry *model.KBRetrieveLog) bool {
		return strings.EqualFold(logEntry.EvidenceGateResult, retrieval.EvidenceGateResultRefused)
	})
}

func buildRouteSpecificRewriteGainRateSeries(logs []*model.KBRetrieveLog, start time.Time, bucketSize time.Duration, bucketCount int) []metricsOverviewBucketRate {
	return buildBucketScopedRateSeries(logs, start, bucketSize, bucketCount,
		func(logEntry *model.KBRetrieveLog) bool {
			return strings.Contains(strings.ToLower(logEntry.RewriteStrategy), retrieval.RewriteStrategyRouteSpecific)
		},
		func(logEntry *model.KBRetrieveLog) bool {
			bucket := strings.TrimSpace(strings.ToLower(logEntry.RewriteGainBucket))
			return bucket == "gain_candidate" || bucket == "route_gain_candidate" || bucket == "model_gain_candidate"
		},
	)
}

func buildModelRewriteErrorRateSeries(logs []*model.KBRetrieveLog, start time.Time, bucketSize time.Duration, bucketCount int) []metricsOverviewBucketRate {
	return buildBucketScopedRateSeries(logs, start, bucketSize, bucketCount,
		func(logEntry *model.KBRetrieveLog) bool {
			return strings.Contains(strings.ToLower(logEntry.RewriteStrategy), retrieval.RewriteStrategyModelAssistedShadow)
		},
		func(logEntry *model.KBRetrieveLog) bool {
			bucket := strings.TrimSpace(strings.ToLower(logEntry.RewriteGainBucket))
			return logEntry.ResultStatus == model.RetrieveResultStatusError ||
				logEntry.ResultStatus == model.RetrieveResultStatusTimeout ||
				strings.HasPrefix(bucket, "risk_") ||
				bucket == "error"
		},
	)
}

func buildCitationSupportScoreSeries(logs []*model.KBRetrieveLog, start time.Time, bucketSize time.Duration, bucketCount int) []metricsOverviewBucketAverage {
	if len(logs) == 0 {
		return []metricsOverviewBucketAverage{}
	}

	type aggregate struct {
		total float64
		count int
	}
	aggregates := make([]aggregate, bucketCount)
	for _, logEntry := range logs {
		if logEntry == nil {
			continue
		}
		index := bucketIndex(logEntry.CreatedAt.UTC(), start, bucketSize, bucketCount)
		if index < 0 {
			continue
		}
		aggregates[index].total += logEntry.CitationSupportScore
		aggregates[index].count++
	}

	series := make([]metricsOverviewBucketAverage, 0, bucketCount)
	for i := 0; i < bucketCount; i++ {
		avg := 0.0
		if aggregates[i].count > 0 {
			avg = aggregates[i].total / float64(aggregates[i].count)
		}
		series = append(series, metricsOverviewBucketAverage{
			Bucket:  start.Add(time.Duration(i) * bucketSize),
			Average: avg,
			Count:   aggregates[i].count,
		})
	}
	return series
}

func buildRouteContributionTotal(logs []*model.KBRetrieveLog) map[string]int {
	total := map[string]int{
		"dense":  0,
		"sparse": 0,
	}
	for _, logEntry := range logs {
		if logEntry == nil {
			continue
		}
		total["dense"] += logEntry.DenseContribution
		total["sparse"] += logEntry.SparseContribution
	}
	return total
}

func buildRewriteGainBucketCounts(logs []*model.KBRetrieveLog) map[string]int {
	counts := make(map[string]int)
	for _, logEntry := range logs {
		if logEntry == nil {
			continue
		}
		bucket := strings.TrimSpace(logEntry.RewriteGainBucket)
		if bucket == "" {
			bucket = "unknown"
		}
		counts[bucket]++
	}
	return counts
}

func buildBucketRateSeries(logs []*model.KBRetrieveLog, start time.Time, bucketSize time.Duration, bucketCount int, matched func(*model.KBRetrieveLog) bool) []metricsOverviewBucketRate {
	return buildBucketScopedRateSeries(logs, start, bucketSize, bucketCount,
		func(*model.KBRetrieveLog) bool { return true },
		matched,
	)
}

func buildBucketScopedRateSeries(
	logs []*model.KBRetrieveLog,
	start time.Time,
	bucketSize time.Duration,
	bucketCount int,
	scoped func(*model.KBRetrieveLog) bool,
	matched func(*model.KBRetrieveLog) bool,
) []metricsOverviewBucketRate {
	if len(logs) == 0 {
		return []metricsOverviewBucketRate{}
	}

	type aggregate struct {
		total   int
		matched int
	}
	aggregates := make([]aggregate, bucketCount)
	for _, logEntry := range logs {
		if logEntry == nil || !scoped(logEntry) {
			continue
		}
		index := bucketIndex(logEntry.CreatedAt.UTC(), start, bucketSize, bucketCount)
		if index < 0 {
			continue
		}
		aggregates[index].total++
		if matched(logEntry) {
			aggregates[index].matched++
		}
	}

	series := make([]metricsOverviewBucketRate, 0, bucketCount)
	for i := 0; i < bucketCount; i++ {
		rate := 0.0
		if aggregates[i].total > 0 {
			rate = float64(aggregates[i].matched) / float64(aggregates[i].total)
		}
		series = append(series, metricsOverviewBucketRate{
			Bucket: start.Add(time.Duration(i) * bucketSize),
			Rate:   rate,
			Total:  aggregates[i].total,
			Empty:  aggregates[i].matched,
		})
	}
	return series
}

func buildRetrieveErrorTopN(logs []*model.KBRetrieveLog) []metricsOverviewErrorType {
	if len(logs) == 0 {
		return []metricsOverviewErrorType{}
	}

	counts := make(map[string]int)
	for _, logEntry := range logs {
		if logEntry == nil || logEntry.ResultStatus != model.RetrieveResultStatusError {
			continue
		}
		errorCode := strings.TrimSpace(logEntry.ErrorCode)
		if errorCode == "" {
			errorCode = "unknown"
		}
		counts[errorCode]++
	}
	if len(counts) == 0 {
		return []metricsOverviewErrorType{}
	}

	items := make([]metricsOverviewErrorType, 0, len(counts))
	for errorCode, count := range counts {
		items = append(items, metricsOverviewErrorType{
			ErrorCode: errorCode,
			Count:     count,
		})
	}

	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].Count > items[i].Count || (items[j].Count == items[i].Count && items[j].ErrorCode < items[i].ErrorCode) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	if len(items) > 5 {
		items = items[:5]
	}
	return items
}

func buildCostOverviewSeries(costs []*model.KBCostTrace, start time.Time, bucketSize time.Duration, bucketCount int) []metricsOverviewCostBreakdown {
	if len(costs) == 0 {
		return []metricsOverviewCostBreakdown{}
	}

	type aggregate struct {
		totalCost         float64
		embeddingCost     float64
		retrievalCost     float64
		rerankCost        float64
		llmCost           float64
		vectorStorageCost float64
		contextTokens     int
		queries           int
	}

	aggregates := make([]aggregate, bucketCount)
	for _, item := range costs {
		if item == nil {
			continue
		}
		index := bucketIndex(item.CreatedAt.UTC(), start, bucketSize, bucketCount)
		if index < 0 {
			continue
		}
		aggregates[index].totalCost += item.TotalCost
		aggregates[index].embeddingCost += item.EmbeddingCost
		aggregates[index].retrievalCost += item.RetrievalCost
		aggregates[index].rerankCost += item.RerankCost
		aggregates[index].llmCost += item.LLMCost
		aggregates[index].vectorStorageCost += item.VectorStorageCost
		aggregates[index].contextTokens += item.ContextTokens
		aggregates[index].queries++
	}

	series := make([]metricsOverviewCostBreakdown, 0, bucketCount)
	for i := 0; i < bucketCount; i++ {
		item := metricsOverviewCostBreakdown{
			Bucket:            start.Add(time.Duration(i) * bucketSize),
			TotalCost:         aggregates[i].totalCost,
			EmbeddingCost:     aggregates[i].embeddingCost,
			RetrievalCost:     aggregates[i].retrievalCost,
			RerankCost:        aggregates[i].rerankCost,
			LLMCost:           aggregates[i].llmCost,
			VectorStorageCost: aggregates[i].vectorStorageCost,
		}
		if aggregates[i].queries > 0 {
			item.CostPer1KQueries = aggregates[i].totalCost / float64(aggregates[i].queries) * 1000
			item.AvgContextTokens = float64(aggregates[i].contextTokens) / float64(aggregates[i].queries)
		}
		series = append(series, item)
	}
	return series
}

func bucketIndex(ts, start time.Time, bucketSize time.Duration, bucketCount int) int {
	if ts.Before(start) {
		return -1
	}

	offset := ts.Sub(start)
	if offset < 0 {
		return -1
	}

	index := int(offset / bucketSize)
	if index >= bucketCount {
		return -1
	}
	return index
}

func requireAdmin(ctx context.Context, c *app.RequestContext) bool {
	if middleware.GetUserID(c) == 0 {
		response.Unauthorized(ctx, c, "Authorization token is required")
		return false
	}
	role := strings.ToLower(strings.TrimSpace(middleware.GetUserRole(c)))
	if role != "admin" && role != "owner" {
		response.Error(ctx, c, 403, "admin role required")
		return false
	}
	return true
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

func GetRetrieveDebugView(ctx context.Context, c *app.RequestContext) {
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
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to get retrieve debug trace", err))
		return
	}

	trace, err := decodeRetrievalDebugTraceResponse(logEntry.DebugTrace)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewInternalError("failed to decode retrieve debug trace", err))
		return
	}
	if trace == nil {
		fallback := buildRetrievalDebugTraceResponse(logEntry, retrieval.SearchMetrics{
			OriginalQuery:          logEntry.Query,
			RewriteQuery:           logEntry.Rewrite,
			FinalQuery:             logEntry.FinalQuery,
			RewriteStrategy:        logEntry.RewriteStrategy,
			RewriteApplied:         logEntry.RewriteApplied,
			ParentChildEnabled:     logEntry.ParentChildEnabled,
			ParentFillStrategy:     logEntry.ParentFillStrategy,
			ParentFillCount:        logEntry.ParentFillCount,
			ParentFillFallback:     logEntry.ParentFillFallback,
			ParentFillTokens:       logEntry.ParentFillTokens,
			TopKDecisionReason:     logEntry.TopKDecisionReason,
			EvidenceGateResult:     logEntry.EvidenceGateResult,
			RefusalReason:          logEntry.RefusalReason,
			CitationSupported:      logEntry.CitationSupported,
			CitationSupportScore:   logEntry.CitationSupportScore,
			UnsupportedClaimCount:  logEntry.UnsupportedClaimCount,
			CitationCheckVersion:   logEntry.CitationCheckVersion,
			CitationCheckLatencyMs: logEntry.CitationCheckLatencyMs,
			EvidenceGateError:      logEntry.EvidenceGateError,
			CitationCheckError:     logEntry.CitationCheckError,
			CandidateTopK:          logEntry.CandidateTopK,
			FinalTopK:              logEntry.FinalTopK,
			TokenBudget:            logEntry.TokenBudget,
			TruncateReason:         logEntry.TruncateReason,
			DenseHits:              logEntry.DenseHits,
			SparseHits:             logEntry.SparseHits,
			DenseContribution:      logEntry.DenseContribution,
			SparseContribution:     logEntry.SparseContribution,
		}, nil, nil)
		trace = &fallback
	}
	if trace.RequestID == "" {
		trace.RequestID = logEntry.RequestID
	}
	if trace.CreatedAt.IsZero() {
		trace.CreatedAt = logEntry.CreatedAt
	}

	response.Success(ctx, c, trace)
}

type retrieveAuditListResponse struct {
	Items    []*model.KBRetrieveLog `json:"items"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

type metricsOverviewBucketRate struct {
	Bucket  time.Time `json:"bucket"`
	Rate    float64   `json:"rate"`
	Total   int       `json:"total"`
	Success int       `json:"success,omitempty"`
	Empty   int       `json:"empty,omitempty"`
}

type metricsOverviewBucketCount struct {
	Bucket time.Time `json:"bucket"`
	Count  int       `json:"count"`
}

type metricsOverviewBucketP95 struct {
	Bucket time.Time `json:"bucket"`
	P95Ms  int64     `json:"p95_ms"`
}

type metricsOverviewBucketAverage struct {
	Bucket  time.Time `json:"bucket"`
	Average float64   `json:"average"`
	Count   int       `json:"count"`
}

type metricsOverviewErrorType struct {
	ErrorCode string `json:"error_code"`
	Count     int    `json:"count"`
}

type metricsOverviewCostBreakdown struct {
	Bucket            time.Time `json:"bucket"`
	TotalCost         float64   `json:"total_cost"`
	CostPer1KQueries  float64   `json:"cost_per_1k_queries"`
	EmbeddingCost     float64   `json:"embedding_cost"`
	RetrievalCost     float64   `json:"retrieval_cost"`
	RerankCost        float64   `json:"rerank_cost"`
	LLMCost           float64   `json:"llm_cost"`
	VectorStorageCost float64   `json:"vector_storage_cost"`
	AvgContextTokens  float64   `json:"avg_context_tokens"`
}

type metricsOverviewResponse struct {
	Range                        string                         `json:"range"`
	IngestSuccessRate            []metricsOverviewBucketRate    `json:"ingest_success_rate"`
	RetrieveRequestCount         []metricsOverviewBucketCount   `json:"retrieve_request_count"`
	RetrieveP95Ms                []metricsOverviewBucketP95     `json:"retrieve_p95_ms"`
	RetrieveEmptyRate            []metricsOverviewBucketRate    `json:"retrieve_empty_rate"`
	ParentFillAppliedRate        []metricsOverviewBucketRate    `json:"parent_fill_applied_rate"`
	EvidenceRefusalRate          []metricsOverviewBucketRate    `json:"evidence_refusal_rate"`
	RouteSpecificRewriteGainRate []metricsOverviewBucketRate    `json:"route_specific_rewrite_gain_rate"`
	ModelRewriteErrorRate        []metricsOverviewBucketRate    `json:"model_rewrite_error_rate"`
	CitationSupportScore         []metricsOverviewBucketAverage `json:"citation_support_score"`
	RouteContributionTotal       map[string]int                 `json:"route_contribution_total"`
	RewriteGainBucketCounts      map[string]int                 `json:"rewrite_gain_bucket_counts"`
	ErrorTypeTopN                []metricsOverviewErrorType     `json:"error_type_topn"`
	CostOverview                 []metricsOverviewCostBreakdown `json:"cost_overview"`
}

type ingestLogDetailResponse struct {
	Job           *model.KBIngestJob         `json:"job"`
	OperationLogs []*model.KBJobOperationLog `json:"operation_logs"`
}

func ListRetrieveAuditLogs(ctx context.Context, c *app.RequestContext) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(ctx, c, "Authorization token is required")
		return
	}

	page, pageSize := getPagination(c)

	var kbID *uint64
	kbIDRaw := strings.TrimSpace(string(c.Query("kb_id")))
	if kbIDRaw != "" {
		parsed, err := parseUint64(kbIDRaw, "kb_id")
		if err != nil {
			response.BadRequest(ctx, c, err.Error())
			return
		}
		kbID = &parsed
	}

	statusRaw := strings.TrimSpace(string(c.Query("result_status")))
	var status *model.RetrieveResultStatus
	if statusRaw != "" {
		parsed, ok := model.ParseRetrieveResultStatus(statusRaw)
		if !ok {
			response.BadRequest(ctx, c, "result_status is invalid")
			return
		}
		status = &parsed
	}

	startTime, err := parseOptionalRFC3339Query(c, "start_time")
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}
	endTime, err := parseOptionalRFC3339Query(c, "end_time")
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}
	if startTime != nil && endTime != nil && startTime.After(*endTime) {
		response.BadRequest(ctx, c, "start_time cannot be later than end_time")
		return
	}

	queryKeywordRaw := strings.TrimSpace(string(c.Query("query_keyword")))
	var queryKeyword *string
	if queryKeywordRaw != "" {
		queryKeyword = &queryKeywordRaw
	}

	requestIDRaw := strings.TrimSpace(string(c.Query("request_id")))
	var requestID *string
	if requestIDRaw != "" {
		requestID = &requestIDRaw
	}

	items, total, err := model.KBRetrieveLogDao.ListWithFilter(model.KBRetrieveLogListFilter{
		UserID:       &userID,
		KBID:         kbID,
		ResultStatus: status,
		StartTime:    startTime,
		EndTime:      endTime,
		QueryKeyword: queryKeyword,
		RequestID:    requestID,
		Page:         page,
		PageSize:     pageSize,
	})
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

func GetMetricsOverview(ctx context.Context, c *app.RequestContext) {
	if middleware.GetUserID(c) == 0 {
		response.Unauthorized(ctx, c, "Authorization token is required")
		return
	}

	var kbID *uint64
	kbIDRaw := strings.TrimSpace(string(c.Query("kb_id")))
	if kbIDRaw != "" {
		parsed, err := parseUint64(kbIDRaw, "kb_id")
		if err != nil {
			response.BadRequest(ctx, c, err.Error())
			return
		}
		kbID = &parsed
	}

	rangeName := strings.TrimSpace(string(c.Query("range")))
	if rangeName == "" {
		rangeName = "24h"
	}
	window, bucketSize, bucketCount, ok := resolveMetricsOverviewRange(rangeName)
	if !ok {
		response.BadRequest(ctx, c, "range must be one of 1h, 24h, 7d")
		return
	}

	endExclusive := alignTimeBucket(time.Now().UTC(), bucketSize).Add(bucketSize)
	startInclusive := endExclusive.Add(-window)
	queryEnd := endExclusive.Add(-time.Nanosecond)

	retrieveLogs, err := model.KBRetrieveLogDao.ListByCreatedAt(startInclusive, queryEnd, kbID)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to load retrieve logs for metrics overview", err))
		return
	}
	costTraces, err := model.KBCostTraceDao.ListByCreatedAt(startInclusive, queryEnd, kbID)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to load cost traces for metrics overview", err))
		return
	}

	ingestJobs, err := model.KBIngestJobDao.ListByCreatedAt(startInclusive, queryEnd, kbID)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to load ingest jobs for metrics overview", err))
		return
	}

	response.Success(ctx, c, metricsOverviewResponse{
		Range:                        rangeName,
		IngestSuccessRate:            buildIngestSuccessRateSeries(ingestJobs, startInclusive, bucketSize, bucketCount),
		RetrieveRequestCount:         buildRetrieveRequestCountSeries(retrieveLogs, startInclusive, bucketSize, bucketCount),
		RetrieveP95Ms:                buildRetrieveP95Series(retrieveLogs, startInclusive, bucketSize, bucketCount),
		RetrieveEmptyRate:            buildRetrieveEmptyRateSeries(retrieveLogs, startInclusive, bucketSize, bucketCount),
		ParentFillAppliedRate:        buildParentFillAppliedRateSeries(retrieveLogs, startInclusive, bucketSize, bucketCount),
		EvidenceRefusalRate:          buildEvidenceRefusalRateSeries(retrieveLogs, startInclusive, bucketSize, bucketCount),
		RouteSpecificRewriteGainRate: buildRouteSpecificRewriteGainRateSeries(retrieveLogs, startInclusive, bucketSize, bucketCount),
		ModelRewriteErrorRate:        buildModelRewriteErrorRateSeries(retrieveLogs, startInclusive, bucketSize, bucketCount),
		CitationSupportScore:         buildCitationSupportScoreSeries(retrieveLogs, startInclusive, bucketSize, bucketCount),
		RouteContributionTotal:       buildRouteContributionTotal(retrieveLogs),
		RewriteGainBucketCounts:      buildRewriteGainBucketCounts(retrieveLogs),
		ErrorTypeTopN:                buildRetrieveErrorTopN(retrieveLogs),
		CostOverview:                 buildCostOverviewSeries(costTraces, startInclusive, bucketSize, bucketCount),
	})
}

func ListIngestLogs(ctx context.Context, c *app.RequestContext) {
	if middleware.GetUserID(c) == 0 {
		response.Unauthorized(ctx, c, "Authorization token is required")
		return
	}

	var kbID *uint64
	kbIDRaw := strings.TrimSpace(string(c.Query("kb_id")))
	if kbIDRaw != "" {
		parsed, err := parseUint64(kbIDRaw, "kb_id")
		if err != nil {
			response.BadRequest(ctx, c, err.Error())
			return
		}
		kbID = &parsed
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

	errorCodeRaw := strings.TrimSpace(string(c.Query("error_code")))
	var errorCode *string
	if errorCodeRaw != "" {
		errorCode = &errorCodeRaw
	}

	startTime, err := parseOptionalRFC3339Query(c, "start_time")
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}
	endTime, err := parseOptionalRFC3339Query(c, "end_time")
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}
	if startTime != nil && endTime != nil && startTime.After(*endTime) {
		response.BadRequest(ctx, c, "start_time cannot be later than end_time")
		return
	}

	page, pageSize := getPagination(c)
	items, total, err := model.KBIngestJobDao.ListWithFilter(model.KBIngestJobListFilter{
		KBID:      kbID,
		Status:    status,
		ErrorCode: errorCode,
		StartTime: startTime,
		EndTime:   endTime,
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to list ingest logs", err))
		return
	}

	response.Success(ctx, c, jobListResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func GetIngestLogDetail(ctx context.Context, c *app.RequestContext) {
	if middleware.GetUserID(c) == 0 {
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
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to get ingest log detail", err))
		return
	}

	operationLogs, err := model.KBJobOperationLogDao.ListByJobID(jobID)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to get ingest operation logs", err))
		return
	}

	response.Success(ctx, c, ingestLogDetailResponse{
		Job:           job,
		OperationLogs: operationLogs,
	})
}

func GetReleaseStatus(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	stage := config.Global.RAG.Release.Stage
	strategy := release.StrategyPhase1
	if config.Global.RAG.FeatureFlags.EnableHybridRetrieval {
		strategy = release.StrategyPhase2
	}
	override := release.GetRuntimeOverride()
	if override.Active {
		stage = override.Stage
		if release.NormalizeStage(stage) == release.StagePhase1 {
			strategy = release.StrategyPhase1
		}
	} else if !config.Global.RAG.Release.Enabled {
		stage = "legacy_full"
	} else if release.NormalizeStage(stage) == release.StagePhase1 {
		strategy = release.StrategyPhase1
	}

	response.Success(ctx, c, releaseStatusResponse{
		Config:          config.Global.RAG.Release,
		RuntimeOverride: override,
		EffectiveStage:  stage,
		CurrentStrategy: strategy,
		StagePlan:       release.StagePlan(stage, config.Global.RAG.Release),
		RollbackPlan:    release.RollbackPlan(),
	})
}

func RollbackRelease(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	userID := middleware.GetUserID(c)
	reason := firstNonEmptyString(strings.TrimSpace(string(c.Query("reason"))), strings.TrimSpace(c.PostForm("reason")), "manual_rollback")
	override := release.SetRuntimeOverride(release.StagePhase1, reason, userID)
	metrics.ObserveReleaseRollback("rollback", release.StagePhase1)
	response.Success(ctx, c, map[string]interface{}{
		"rolled_back":      true,
		"runtime_override": override,
		"rollback_plan":    release.RollbackPlan(),
	})
}

func ActivateRelease(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	release.ClearRuntimeOverride()
	metrics.ObserveReleaseRollback("resume", release.NormalizeStage(config.Global.RAG.Release.Stage))
	response.Success(ctx, c, map[string]interface{}{
		"rolled_back":      false,
		"runtime_override": release.GetRuntimeOverride(),
		"stage_plan":       release.StagePlan(config.Global.RAG.Release.Stage, config.Global.RAG.Release),
	})
}

func GetReleaseSummary(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	minutes := getIntWithDefault(string(c.Query("minutes")), defaultReleaseSummaryMinutes)
	if minutes <= 0 {
		minutes = defaultReleaseSummaryMinutes
	}
	if minutes > maxReleaseSummaryMinutes {
		minutes = maxReleaseSummaryMinutes
	}

	since := time.Now().Add(-time.Duration(minutes) * time.Minute)
	var logs []*model.KBRetrieveLog
	if err := repository.GetDB().Where("created_at >= ?", since).Order("created_at DESC").Find(&logs).Error; err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to load release summary logs", err))
		return
	}

	summary := buildReleaseSummary(minutes, since, logs)
	response.Success(ctx, c, summary)
}

func buildReleaseSummary(minutes int, since time.Time, logs []*model.KBRetrieveLog) releaseSummaryResponse {
	summary := releaseSummaryResponse{
		WindowMinutes:      minutes,
		Since:              since,
		StrategyCounts:     map[string]int{},
		ReleaseStageCounts: map[string]int{},
		ResultStatusCounts: map[string]int{},
		EmptyReasonCounts:  map[string]int{},
		RewriteGainBuckets: map[string]int{},
		RouteContribution: map[string]int{
			"dense":  0,
			"sparse": 0,
		},
		AcceptanceTemplate: "backend/docs/kb-l8-phase2-rollout-acceptance-template.md",
	}
	if len(logs) == 0 {
		return summary
	}

	rewriteApplied := 0
	durationValues := make([]int64, 0, len(logs))
	rerankValues := make([]int64, 0, len(logs))
	phase2Requests := 0
	phase2Failures := 0
	phase2EmptyAfterFilter := 0
	parentFillRequests := 0
	evidenceRefusals := 0
	modelRewriteRequests := 0
	modelRewriteErrors := 0
	totalCitationSupport := 0.0

	for _, item := range logs {
		if item == nil {
			continue
		}
		summary.TotalRequests++
		summary.StrategyCounts[firstNonEmptyString(item.Strategy, "unknown")]++
		summary.ReleaseStageCounts[firstNonEmptyString(item.ReleaseStage, "unknown")]++
		summary.ResultStatusCounts[string(item.ResultStatus)]++
		summary.EmptyReasonCounts[firstNonEmptyString(item.EmptyReason, retrieval.EmptyReasonNone)]++
		summary.RewriteGainBuckets[firstNonEmptyString(item.RewriteGainBucket, "unknown")]++
		summary.RouteContribution["dense"] += item.DenseContribution
		summary.RouteContribution["sparse"] += item.SparseContribution
		if item.RewriteApplied {
			rewriteApplied++
		}
		if item.ParentFillCount > 0 {
			parentFillRequests++
		}
		if strings.EqualFold(item.EvidenceGateResult, retrieval.EvidenceGateResultRefused) {
			evidenceRefusals++
		}
		if strings.Contains(strings.ToLower(item.RewriteStrategy), retrieval.RewriteStrategyModelAssistedShadow) {
			modelRewriteRequests++
			if item.ResultStatus == model.RetrieveResultStatusError ||
				item.ResultStatus == model.RetrieveResultStatusTimeout ||
				strings.HasPrefix(strings.ToLower(item.RewriteGainBucket), "risk_") ||
				strings.EqualFold(item.RewriteGainBucket, "error") {
				modelRewriteErrors++
			}
		}
		totalCitationSupport += item.CitationSupportScore
		if item.DurationMs > 0 {
			durationValues = append(durationValues, item.DurationMs)
		}
		if item.RerankMs > 0 {
			rerankValues = append(rerankValues, item.RerankMs)
		}
		if item.Strategy == release.StrategyPhase2 {
			phase2Requests++
			if item.ResultStatus == model.RetrieveResultStatusError || item.ResultStatus == model.RetrieveResultStatusTimeout {
				phase2Failures++
			}
			if item.EmptyReason == retrieval.EmptyReasonAfterFilter {
				phase2EmptyAfterFilter++
			}
		}
	}

	summary.RewriteAppliedRate = float64(rewriteApplied) / float64(len(logs))
	summary.ParentFillRate = float64(parentFillRequests) / float64(len(logs))
	summary.EvidenceRefusalRate = float64(evidenceRefusals) / float64(len(logs))
	summary.AvgCitationSupport = totalCitationSupport / float64(len(logs))
	if modelRewriteRequests > 0 {
		summary.ModelRewriteErrorRate = float64(modelRewriteErrors) / float64(modelRewriteRequests)
	}
	summary.P95DurationMs = percentileInt64(durationValues, 0.95)
	summary.P95RerankMs = percentileInt64(rerankValues, 0.95)
	if phase2Requests > 0 {
		errorRate := float64(phase2Failures) / float64(phase2Requests)
		emptyAfterFilterRate := float64(phase2EmptyAfterFilter) / float64(phase2Requests)
		if errorRate > 0.03 {
			summary.RollbackRecommended = true
			summary.Risks = append(summary.Risks, fmt.Sprintf("phase2 error rate %.2f%% exceeds 3%%", errorRate*100))
		}
		if emptyAfterFilterRate > 0.15 {
			summary.RollbackRecommended = true
			summary.Risks = append(summary.Risks, fmt.Sprintf("phase2 Empty-After-Filter ratio %.2f%% exceeds 15%%", emptyAfterFilterRate*100))
		}
	}
	if summary.P95DurationMs > 2000 {
		summary.RollbackRecommended = true
		summary.Risks = append(summary.Risks, fmt.Sprintf("retrieve P95 latency %dms exceeds 2000ms", summary.P95DurationMs))
	}
	if summary.P95RerankMs > 800 {
		summary.Risks = append(summary.Risks, fmt.Sprintf("rerank P95 latency %dms exceeds 800ms", summary.P95RerankMs))
	}
	if summary.ModelRewriteErrorRate > 0.2 {
		summary.Risks = append(summary.Risks, fmt.Sprintf("model rewrite error rate %.2f%% exceeds 20%%", summary.ModelRewriteErrorRate*100))
	}

	return summary
}

func PauseIngest(ctx context.Context, c *app.RequestContext) {
	if middleware.GetUserID(c) == 0 {
		response.Unauthorized(ctx, c, "Authorization token is required")
		return
	}

	ragqueue.PauseKnowledgeIngest()
	response.Success(ctx, c, map[string]interface{}{"paused": true})
}

func ResumeIngest(ctx context.Context, c *app.RequestContext) {
	if middleware.GetUserID(c) == 0 {
		response.Unauthorized(ctx, c, "Authorization token is required")
		return
	}

	ragqueue.ResumeKnowledgeIngest()
	response.Success(ctx, c, map[string]interface{}{"paused": false})
}

func GetIngestStatus(ctx context.Context, c *app.RequestContext) {
	if middleware.GetUserID(c) == 0 {
		response.Unauthorized(ctx, c, "Authorization token is required")
		return
	}

	paused := ragqueue.IsKnowledgeIngestPaused()
	response.Success(ctx, c, map[string]interface{}{"paused": paused})
}

type dashboardStatsResponse struct {
	KBCount            int64 `json:"kb_count"`
	DocumentCount      int64 `json:"document_count"`
	ProcessingJobCount int64 `json:"processing_job_count"`
	FailedJobCount     int64 `json:"failed_job_count"`
}

func GetDashboardStats(ctx context.Context, c *app.RequestContext) {
	if middleware.GetUserID(c) == 0 {
		response.Unauthorized(ctx, c, "Authorization token is required")
		return
	}

	kbCount, err := model.KBKnowledgeBaseDao.Count()
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to count knowledge bases", err))
		return
	}

	docCount, err := model.KBDocumentDao.CountNonDeleted()
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to count documents", err))
		return
	}

	processingCount, err := model.KBIngestJobDao.CountByStatuses([]model.KBIngestJobStatus{
		model.KBIngestJobStatusPending,
		model.KBIngestJobStatusProcessing,
		model.KBIngestJobStatusRetrying,
	})
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to count processing jobs", err))
		return
	}

	failedCount, err := model.KBIngestJobDao.CountByStatuses([]model.KBIngestJobStatus{
		model.KBIngestJobStatusFailed,
		model.KBIngestJobStatusDead,
	})
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to count failed jobs", err))
		return
	}

	response.Success(ctx, c, dashboardStatsResponse{
		KBCount:            kbCount,
		DocumentCount:      docCount,
		ProcessingJobCount: processingCount,
		FailedJobCount:     failedCount,
	})
}
