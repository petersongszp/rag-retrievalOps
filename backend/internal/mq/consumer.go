package mq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"interview-agents/internal/agents/usecase/evaluation"
	"interview-agents/internal/agents/usecase/resume"
	"interview-agents/internal/config"
	"interview-agents/internal/milvus"
	"interview-agents/internal/model"
	"interview-agents/internal/observability/metrics"

	"github.com/cloudwego/eino/schema"
	"github.com/ledongthuc/pdf"
)

type knowledgeIngestErrorType string

const (
	knowledgeIngestErrorTypePayload   knowledgeIngestErrorType = "invalid_payload"
	knowledgeIngestErrorTypeParse     knowledgeIngestErrorType = "parse_error"
	knowledgeIngestErrorTypeEmbedding knowledgeIngestErrorType = "embedding_error"
	knowledgeIngestErrorTypeMilvus    knowledgeIngestErrorType = "milvus_write_error"
	knowledgeIngestErrorTypeUnknown   knowledgeIngestErrorType = "unknown_error"
)

const (
	knowledgeRetryScannerBatchSize       = 50
	knowledgeRetryScannerFallbackBackoff = 5 * time.Second
	maxKnowledgeRetryBackoff             = 5 * time.Minute
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// ConsumerHandler handles async message processing.
type ConsumerHandler struct{}

func NewConsumerHandler() *ConsumerHandler {
	return &ConsumerHandler{}
}

func (h *ConsumerHandler) HandleMessage(ctx context.Context, message *Message) error {
	switch message.Type {
	case MessageTypeEvaluationReport:
		return h.handleEvaluationReport(ctx, message)
	case MessageTypeTopicEvaluation:
		return h.handleTopicEvaluation(ctx, message)
	case MessageTypeResumeParse:
		return h.handleResumeParse(ctx, message)
	case MessageTypeKnowledgeIngest:
		return h.handleKnowledgeIngest(ctx, message)
	default:
		return fmt.Errorf("unknown message type: %s", message.Type)
	}
}

func (h *ConsumerHandler) handleEvaluationReport(ctx context.Context, message *Message) error {
	log.Printf("[Consumer] processing evaluation report message")

	userID, ok := message.Payload["user_id"].(float64)
	if !ok {
		return fmt.Errorf("invalid user_id in payload")
	}
	reportID, ok := message.Payload["report_id"].(float64)
	if !ok {
		return fmt.Errorf("invalid report_id in payload")
	}

	log.Printf("[Consumer] generating evaluation report: userID=%d, reportID=%d", uint(userID), uint64(reportID))
	_, err := evaluation.GenerateRecordEvaluation(ctx, uint(userID), uint64(reportID))
	if err != nil {
		log.Printf("[Consumer] failed to generate evaluation report: %v", err)
		return err
	}
	log.Printf("[Consumer] evaluation report generated successfully: userID=%d, reportID=%d", uint(userID), uint64(reportID))
	return nil
}

func (h *ConsumerHandler) handleTopicEvaluation(ctx context.Context, message *Message) error {
	log.Printf("[Consumer] processing topic evaluation message")

	userID, ok := message.Payload["user_id"].(float64)
	if !ok {
		return fmt.Errorf("invalid user_id in payload")
	}
	reportID, ok := message.Payload["report_id"].(float64)
	if !ok {
		return fmt.Errorf("invalid report_id in payload")
	}

	log.Printf("[Consumer] generating topic evaluation: userID=%d, reportID=%d", uint(userID), uint64(reportID))
	_, err := evaluation.GenerateAnswerRecordEvaluation(ctx, uint(userID), uint64(reportID))
	if err != nil {
		log.Printf("[Consumer] failed to generate topic evaluation: %v", err)
		return err
	}
	log.Printf("[Consumer] topic evaluation generated successfully: userID=%d, reportID=%d", uint(userID), uint64(reportID))
	return nil
}

func (h *ConsumerHandler) handleResumeParse(ctx context.Context, message *Message) error {
	log.Printf("[Consumer] processing resume parse message")

	payloadBytes, err := json.Marshal(message.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	var payload ResumeParsePayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return fmt.Errorf("invalid resume_parse payload: %w", err)
	}
	if payload.UserID == 0 || payload.ResumeID == 0 || payload.FilePath == "" {
		return fmt.Errorf("invalid resume_parse payload values: user_id=%d resume_id=%d file_path=%q", payload.UserID, payload.ResumeID, payload.FilePath)
	}

	log.Printf("[Consumer] parsing resume: userID=%d, resumeID=%d, filePath=%s", payload.UserID, payload.ResumeID, payload.FilePath)

	if err = model.ResumeDao.UpdateResumeStatus(payload.ResumeID, "processing", ""); err != nil {
		log.Printf("[Consumer] failed to update resume status to processing: %v", err)
	}

	parseResult, err := resume.ParseResume(ctx, payload.UserID, payload.FilePath)
	if err != nil {
		log.Printf("[Consumer] failed to parse resume: %v", err)
		updateErr := model.ResumeDao.UpdateResumeStatus(payload.ResumeID, "failed", err.Error())
		if updateErr != nil {
			log.Printf("[Consumer] failed to update resume status to failed: %v", updateErr)
		}
		return err
	}

	contentJSON, err := json.Marshal(parseResult)
	if err != nil {
		log.Printf("[Consumer] failed to marshal parse result: %v", err)
		updateErr := model.ResumeDao.UpdateResumeStatus(payload.ResumeID, "failed", "failed to marshal parse result")
		if updateErr != nil {
			log.Printf("[Consumer] failed to update resume status: %v", updateErr)
		}
		return err
	}

	if err = model.ResumeDao.UpdateResumeContent(payload.ResumeID, string(contentJSON)); err != nil {
		log.Printf("[Consumer] failed to update resume content: %v", err)
		return err
	}

	log.Printf("[Consumer] resume parsed successfully: userID=%d, resumeID=%d, content_size=%d", payload.UserID, payload.ResumeID, len(contentJSON))
	return nil
}

func (h *ConsumerHandler) handleKnowledgeIngest(ctx context.Context, message *Message) error {
	if IsKnowledgeIngestPaused() {
		log.Printf("[KB Ingest] Skipping knowledge ingest because it's paused")
		return nil
	}

	start := time.Now()

	payload, err := parseKnowledgeIngestPayload(message.Payload)
	if err != nil {
		log.Printf("[KB Ingest] payload parse failed: err=%v", err)
		metrics.ObserveIngest(time.Since(start), string(model.KBIngestJobStatusFailed), string(knowledgeIngestErrorTypePayload))
		return nil
	}

	if !shouldHandleKnowledgeJob(payload.JobID) {
		return nil
	}

	claimed, claimErr := model.KBIngestJobDao.ClaimForProcessing(payload.JobID)
	if claimErr != nil {
		log.Printf("[KB Ingest] failed to claim job for processing: job_id=%d err=%v", payload.JobID, claimErr)
		return nil
	}
	if !claimed {
		log.Printf("[KB Ingest] job claim ignored due to state mismatch: job_id=%d", payload.JobID)
		return nil
	}

	if updateErr := model.KBDocumentDao.UpdateStatus(payload.DocumentID, model.KBDocumentStatusProcessing, ""); updateErr != nil {
		log.Printf("[KB Ingest] failed to update document status to processing: document_id=%d err=%v", payload.DocumentID, updateErr)
	}

	totalChunks, ingestErr := ingestKnowledgeDocument(ctx, payload)
	if ingestErr != nil {
		handleKnowledgeIngestFailure(payload, ingestErr, start)
		return nil
	}

	if updateErr := model.KBDocumentDao.UpdateChunkCount(payload.DocumentID, totalChunks); updateErr != nil {
		log.Printf("[KB Ingest] failed to update chunk_count: job_id=%d document_id=%d err=%v", payload.JobID, payload.DocumentID, updateErr)
	}

	updated, updateErr := model.KBIngestJobDao.UpdateStatusFrom(
		payload.JobID,
		model.KBIngestJobStatusCompleted,
		"",
		model.KBIngestJobStatusProcessing,
	)
	if updateErr != nil {
		log.Printf("[KB Ingest] failed to update job completed: job_id=%d err=%v", payload.JobID, updateErr)
	} else if updated {
		if docErr := model.KBDocumentDao.UpdateStatus(payload.DocumentID, model.KBDocumentStatusCompleted, ""); docErr != nil {
			log.Printf("[KB Ingest] failed to update document completed: document_id=%d err=%v", payload.DocumentID, docErr)
		}
	} else {
		log.Printf("[KB Ingest] skip complete update due to non-processing state: job_id=%d", payload.JobID)
	}

	logKnowledgeIngest(payload, string(model.KBIngestJobStatusCompleted), "", totalChunks, time.Since(start))
	metrics.ObserveIngest(time.Since(start), string(model.KBIngestJobStatusCompleted), "none")
	return nil
}

func parseKnowledgeIngestPayload(raw map[string]interface{}) (KnowledgeIngestPayload, error) {
	payloadBytes, err := json.Marshal(raw)
	if err != nil {
		return KnowledgeIngestPayload{}, fmt.Errorf("failed to marshal payload: %w", err)
	}

	var payload KnowledgeIngestPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return KnowledgeIngestPayload{}, fmt.Errorf("invalid knowledge_ingest payload: %w", err)
	}
	if payload.OperatorAdminID == 0 {
		payload.OperatorAdminID = payload.UserID
	}

	if payload.OperatorAdminID == 0 || payload.KBID == 0 || payload.DocumentID == 0 || payload.JobID == 0 || strings.TrimSpace(payload.FilePath) == "" {
		return KnowledgeIngestPayload{}, fmt.Errorf(
			"invalid knowledge_ingest payload values: operator_admin_id=%d kb_id=%d document_id=%d job_id=%d file_path=%q",
			payload.OperatorAdminID, payload.KBID, payload.DocumentID, payload.JobID, payload.FilePath,
		)
	}
	payload.UserID = payload.OperatorAdminID
	return payload, nil
}

func shouldHandleKnowledgeJob(jobID uint64) bool {
	job, err := model.KBIngestJobDao.GetByID(jobID)
	if err != nil {
		log.Printf("[KB Ingest] failed to load job, skip message: job_id=%d err=%v", jobID, err)
		return false
	}

	switch job.Status {
	case model.KBIngestJobStatusCompleted, model.KBIngestJobStatusDead, model.KBIngestJobStatusCanceled:
		log.Printf("[KB Ingest] job already terminal, skip: job_id=%d status=%s", jobID, job.Status)
		return false
	default:
		return true
	}
}

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

func handleKnowledgeIngestFailure(payload KnowledgeIngestPayload, ingestErr error, startedAt time.Time) {
	errorCode := getKnowledgeIngestErrorCode(ingestErr)
	errorDetail := ingestErr.Error()
	retryable := isKnowledgeIngestRetryable(errorCode, ingestErr)

	if !config.Global.RAG.FeatureFlags.EnableIngestRetry || !retryable {
		if err := model.KBIngestJobDao.UpdateFailureState(
			payload.JobID,
			model.KBIngestJobStatusFailed,
			errorDetail,
			string(errorCode),
			errorDetail,
			nil,
			false,
		); err != nil {
			log.Printf("[KB Ingest] failed to mark failed: job_id=%d err=%v", payload.JobID, err)
		}
		_ = model.KBDocumentDao.UpdateStatus(payload.DocumentID, model.KBDocumentStatusFailed, errorDetail)
		logKnowledgeIngest(payload, string(model.KBIngestJobStatusFailed), errorDetail, 0, time.Since(startedAt))
		metrics.ObserveIngest(time.Since(startedAt), string(model.KBIngestJobStatusFailed), string(errorCode))
		return
	}

	job, err := model.KBIngestJobDao.GetByID(payload.JobID)
	if err != nil {
		log.Printf("[KB Ingest Retry] failed to load job, fallback to failed: job_id=%d err=%v", payload.JobID, err)
		_ = model.KBIngestJobDao.UpdateFailureState(
			payload.JobID,
			model.KBIngestJobStatusFailed,
			errorDetail,
			string(errorCode),
			errorDetail,
			nil,
			false,
		)
		_ = model.KBDocumentDao.UpdateStatus(payload.DocumentID, model.KBDocumentStatusFailed, errorDetail)
		logKnowledgeIngest(payload, string(model.KBIngestJobStatusFailed), errorDetail, 0, time.Since(startedAt))
		metrics.ObserveIngest(time.Since(startedAt), string(model.KBIngestJobStatusFailed), string(errorCode))
		return
	}

	nextRetryCount := job.RetryCount + 1
	maxRetryCount := resolveKnowledgeRetryCount()
	if nextRetryCount > maxRetryCount {
		if err := model.KBIngestJobDao.UpdateFailureState(
			payload.JobID,
			model.KBIngestJobStatusFailed,
			errorDetail,
			string(errorCode),
			errorDetail,
			nil,
			false,
		); err != nil {
			log.Printf("[KB Ingest Retry] failed to mark failed before dead: job_id=%d err=%v", payload.JobID, err)
		}
		if _, err := model.KBIngestJobDao.UpdateStatusFrom(
			payload.JobID,
			model.KBIngestJobStatusDead,
			errorDetail,
			model.KBIngestJobStatusFailed,
		); err != nil {
			log.Printf("[KB Ingest Retry] failed to mark dead: job_id=%d err=%v", payload.JobID, err)
		}
		_ = model.KBDocumentDao.UpdateStatus(payload.DocumentID, model.KBDocumentStatusFailed, errorDetail)
		logKnowledgeIngest(payload, string(model.KBIngestJobStatusDead), errorDetail, 0, time.Since(startedAt))
		metrics.ObserveIngest(time.Since(startedAt), string(model.KBIngestJobStatusDead), string(errorCode))
		return
	}

	delay := calculateKnowledgeRetryBackoff(nextRetryCount)
	nextRetryAt := time.Now().Add(delay)
	if err := model.KBIngestJobDao.UpdateFailureState(
		payload.JobID,
		model.KBIngestJobStatusRetrying,
		errorDetail,
		string(errorCode),
		errorDetail,
		&nextRetryAt,
		true,
	); err != nil {
		log.Printf("[KB Ingest Retry] failed to mark retrying: job_id=%d err=%v", payload.JobID, err)
	}
	_ = model.KBDocumentDao.UpdateStatus(payload.DocumentID, model.KBDocumentStatusFailed, errorDetail)

	log.Printf(
		"[KB Ingest Retry] job_id=%d retry_count=%d max_retry=%d next_retry_at=%s backoff_ms=%d error_code=%s error=%q",
		payload.JobID,
		nextRetryCount,
		maxRetryCount,
		nextRetryAt.Format(time.RFC3339),
		delay.Milliseconds(),
		errorCode,
		errorDetail,
	)
	logKnowledgeIngest(payload, string(model.KBIngestJobStatusRetrying), errorDetail, 0, time.Since(startedAt))
	metrics.ObserveIngest(time.Since(startedAt), string(model.KBIngestJobStatusRetrying), string(errorCode))
}

func startKnowledgeRetryCompensator(ctx context.Context) {
	interval := resolveKnowledgeRetryScannerInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	log.Printf("[KB Retry Scanner] started, interval=%s batch=%d", interval, knowledgeRetryScannerBatchSize)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[KB Retry Scanner] stopped")
			return
		case <-ticker.C:
			runKnowledgeRetryCompensation(ctx)
		}
	}
}

func runKnowledgeRetryCompensation(ctx context.Context) {
	now := time.Now()
	jobs, err := model.KBIngestJobDao.ListRetryDueJobs(knowledgeRetryScannerBatchSize, now)
	if err != nil {
		log.Printf("[KB Retry Scanner] failed to list due jobs: err=%v", err)
		return
	}
	if len(jobs) == 0 {
		return
	}

	for _, job := range jobs {
		if job == nil {
			continue
		}

		claimed, err := model.KBIngestJobDao.MarkPendingForRetry(job.ID, now)
		if err != nil {
			log.Printf("[KB Retry Scanner] failed to claim job: job_id=%d err=%v", job.ID, err)
			continue
		}
		if !claimed {
			continue
		}

		doc, err := model.KBDocumentDao.GetByID(job.DocumentID)
		if err != nil {
			detail := fmt.Sprintf("source document not found during retry compensation: %v", err)
			_ = model.KBIngestJobDao.UpdateFailureState(
				job.ID,
				model.KBIngestJobStatusDead,
				detail,
				string(knowledgeIngestErrorTypeUnknown),
				detail,
				nil,
				false,
			)
			log.Printf("[KB Retry Scanner] mark job dead because document missing: job_id=%d document_id=%d", job.ID, job.DocumentID)
			continue
		}

		payload := KnowledgeIngestPayload{
			UserID:     job.UserID,
			KBID:       job.KbID,
			DocumentID: job.DocumentID,
			JobID:      job.ID,
			FilePath:   doc.StoragePath,
			FileType:   doc.FileType,
			Collection: resolveKnowledgeBaseCollectionNameForRetry(job.KbID),
		}

		if err := PublishKnowledgeIngest(ctx, payload); err != nil {
			detail := fmt.Sprintf("failed to republish ingest task: %v", err)
			nextRetryAt := time.Now().Add(knowledgeRetryScannerFallbackBackoff)
			_ = model.KBIngestJobDao.UpdateFailureState(
				job.ID,
				model.KBIngestJobStatusFailed,
				detail,
				string(knowledgeIngestErrorTypeUnknown),
				detail,
				&nextRetryAt,
				false,
			)
			log.Printf("[KB Retry Scanner] failed to republish: job_id=%d err=%v", job.ID, err)
			continue
		}

		_ = model.KBDocumentDao.UpdateStatus(job.DocumentID, model.KBDocumentStatusPending, "")
		log.Printf("[KB Retry Scanner] republished job: job_id=%d document_id=%d retry_count=%d", job.ID, job.DocumentID, job.RetryCount)
	}
}

func logKnowledgeIngest(payload KnowledgeIngestPayload, status, errorMsg string, chunkCount int, duration time.Duration) {
	log.Printf(
		"[KB Ingest] job_id=%d document_id=%d kb_id=%d user_id=%d status=%s error_msg=%q chunk_count=%d duration_ms=%d",
		payload.JobID,
		payload.DocumentID,
		payload.KBID,
		payload.UserID,
		status,
		errorMsg,
		chunkCount,
		duration.Milliseconds(),
	)
}

func buildKnowledgeIngestError(errorCode knowledgeIngestErrorType, detail string, cause error) error {
	if cause == nil {
		return fmt.Errorf("%s: %s", errorCode, detail)
	}
	return fmt.Errorf("%s: %s: %w", errorCode, detail, cause)
}

func getKnowledgeIngestErrorCode(err error) knowledgeIngestErrorType {
	if err == nil {
		return knowledgeIngestErrorTypeUnknown
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, string(knowledgeIngestErrorTypePayload)):
		return knowledgeIngestErrorTypePayload
	case strings.Contains(msg, string(knowledgeIngestErrorTypeParse)):
		return knowledgeIngestErrorTypeParse
	case strings.Contains(msg, string(knowledgeIngestErrorTypeEmbedding)):
		return knowledgeIngestErrorTypeEmbedding
	case strings.Contains(msg, string(knowledgeIngestErrorTypeMilvus)):
		return knowledgeIngestErrorTypeMilvus
	default:
		return classifyKnowledgeIngestError(err)
	}
}

func classifyKnowledgeIngestError(err error) knowledgeIngestErrorType {
	if err == nil {
		return knowledgeIngestErrorTypeUnknown
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "embed"),
		strings.Contains(msg, "embedding"),
		strings.Contains(msg, "vector"):
		return knowledgeIngestErrorTypeEmbedding
	case strings.Contains(msg, "milvus"),
		strings.Contains(msg, "indexer"),
		strings.Contains(msg, "store"):
		return knowledgeIngestErrorTypeMilvus
	case strings.Contains(msg, "parse"),
		strings.Contains(msg, "pdf"),
		strings.Contains(msg, "read"),
		strings.Contains(msg, "extract"),
		strings.Contains(msg, "split"):
		return knowledgeIngestErrorTypeParse
	default:
		return knowledgeIngestErrorTypeUnknown
	}
}

func isKnowledgeIngestRetryable(errorCode knowledgeIngestErrorType, err error) bool {
	switch errorCode {
	case knowledgeIngestErrorTypePayload, knowledgeIngestErrorTypeParse:
		return false
	case knowledgeIngestErrorTypeEmbedding, knowledgeIngestErrorTypeMilvus:
		return hasTransientFailureSignal(err)
	case knowledgeIngestErrorTypeUnknown:
		return hasTransientFailureSignal(err)
	default:
		return false
	}
}

func hasTransientFailureSignal(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	transientKeywords := []string{
		"timeout",
		"timed out",
		"deadline exceeded",
		"temporarily unavailable",
		"temporary failure",
		"connection reset",
		"connection refused",
		"broken pipe",
		"eof",
		"i/o timeout",
		"network",
	}
	for _, keyword := range transientKeywords {
		if strings.Contains(msg, keyword) {
			return true
		}
	}
	return false
}

func resolveKnowledgeRetryCount() int {
	maxRetry := config.Global.RAG.Thresholds.MaxRetryCount
	if maxRetry <= 0 {
		return 3
	}
	return maxRetry
}

func resolveKnowledgeRetryBackoff() time.Duration {
	backoffMS := config.Global.RAG.Thresholds.RetryBackoffMS
	if backoffMS <= 0 {
		backoffMS = 500
	}
	return time.Duration(backoffMS) * time.Millisecond
}

func resolveKnowledgeRetryScannerInterval() time.Duration {
	base := resolveKnowledgeRetryBackoff()
	if base < 5*time.Second {
		return 5 * time.Second
	}
	if base > 30*time.Second {
		return 30 * time.Second
	}
	return base
}

func calculateKnowledgeRetryBackoff(retryCount int) time.Duration {
	if retryCount <= 0 {
		retryCount = 1
	}

	base := resolveKnowledgeRetryBackoff()
	exponent := math.Pow(2, float64(retryCount-1))
	delay := time.Duration(float64(base) * exponent)
	if delay > maxKnowledgeRetryBackoff {
		delay = maxKnowledgeRetryBackoff
	}

	jitter := 0.8 + rand.Float64()*0.4
	jittered := time.Duration(float64(delay) * jitter)
	if jittered < time.Second {
		return time.Second
	}
	return jittered
}

func extractKnowledgeRawText(ctx context.Context, filePath, fileType string) (string, error) {
	_ = ctx

	if strings.TrimSpace(filePath) == "" {
		return "", fmt.Errorf("file path is empty")
	}
	if _, err := os.Stat(filePath); err != nil {
		return "", fmt.Errorf("failed to access file: %w", err)
	}

	normalizedType := strings.ToLower(strings.TrimSpace(fileType))
	if normalizedType == "" {
		normalizedType = strings.TrimPrefix(strings.ToLower(filepath.Ext(filePath)), ".")
	}

	switch normalizedType {
	case "txt", "md", "markdown":
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to read text file: %w", err)
		}
		text := strings.TrimSpace(string(data))
		if text == "" {
			return "", fmt.Errorf("empty text content")
		}
		return text, nil
	case "pdf":
		return extractTextFromPDF(filePath)
	default:
		return "", fmt.Errorf("unsupported file type: %s", normalizedType)
	}
}

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

func extractTextFromPDF(filePath string) (string, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open pdf: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	var builder strings.Builder
	totalPages := r.NumPage()

	for pageIndex := 1; pageIndex <= totalPages; pageIndex++ {
		p := r.Page(pageIndex)
		if p.V.IsNull() {
			continue
		}

		pageText, err := p.GetPlainText(nil)
		if err != nil {
			return "", fmt.Errorf("failed to extract pdf text on page %d: %w", pageIndex, err)
		}
		builder.WriteString(pageText)
		builder.WriteString("\n")
	}

	text := strings.TrimSpace(builder.String())
	if text == "" {
		return "", errors.New("empty text extracted from pdf")
	}
	return text, nil
}

// StartConsumer starts MQ consumer loop.
func StartConsumer(ctx context.Context) error {
	mq := GetMessageQueue()
	handler := NewConsumerHandler()

	log.Printf("[Consumer] starting message consumer")
	log.Printf("[Consumer] MQ type: %T", mq)

	if config.Global.RAG.FeatureFlags.EnableIngestRetry {
		go startKnowledgeRetryCompensator(ctx)
	}
	if reporter, ok := mq.(interface {
		StartMetricsReporter(context.Context)
	}); ok {
		go reporter.StartMetricsReporter(ctx)
	}

	err := mq.Subscribe(ctx, handler.HandleMessage)
	log.Printf("[Consumer] consumer stopped: %v", err)
	return err
}
