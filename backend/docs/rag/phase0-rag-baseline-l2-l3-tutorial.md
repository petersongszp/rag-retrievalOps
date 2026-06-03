# Phase 0 L2 + L3 实战教程（可照抄实现）

本文是 `RAG 基线` 的 **L2（API 与路由）+ L3（异步任务编排）** 落地教程。  
目标是：新同学不看历史提交，也能按本文一步步把功能写出来并验证通过。

---

## 1. 本教程实现目标

实现以下能力：

1. 提供知识库域 API：
`/api/kb/bases`、`/api/kb/documents/upload`、`/api/kb/jobs/:job_id`、`/api/kb/retrieve` 等。
2. 上传文档后不阻塞请求，改为异步入库。
3. 入库任务状态可追踪：
`pending -> processing -> completed/failed`。
4. 消费失败时有可读 `error_msg`，并包含错误分类。

---

## 2. 前置条件

在开始 L2/L3 前，确保：

1. L0/L1 已完成（配置、Milvus 初始化、3 张表 `kb_knowledge_base/kb_document/kb_ingest_job`）。
2. 服务启动时已经初始化 MQ 并启动 Consumer（见第 7 节）。
3. `.env` / `config.yaml` 已正确配置 MySQL、Redis、Milvus、Embedding。

---

## 3. L2：路由注册

### 3.1 新建文件 `backend/api/router/custom_kb.go`

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

### 3.2 修改 `backend/api/router/custom_asr.go`

确保 `RegisterCustomRoutes` 里调用了 `registerKnowledgeBaseRoutes(r)`：

```go
func RegisterCustomRoutes(r *server.Hertz) {
	asr := r.Group("/api/interview/asr")
	asr.GET("/capability", interview.GetASRCapability)
	asr.POST("/transcribe", interview.TranscribeInterviewAudio)

	prediction := r.Group("/api/prediction")
	prediction.POST("/delete", interview.DeletePredictionRecords)

	registerKnowledgeBaseRoutes(r)
}
```

---

## 4. L3：MQ 消息定义与发布

### 4.1 替换 `backend/internal/mq/mq.go`

> 这版包含 `knowledge_ingest` 消息类型和发布函数。

```go
package mq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
)

// MessageType is the message kind in MQ.
type MessageType string

const (
	MessageTypeEvaluationReport MessageType = "evaluation_report"
	MessageTypeTopicEvaluation  MessageType = "topic_evaluation"
	MessageTypeResumeParse      MessageType = "resume_parse"
	MessageTypeKnowledgeIngest  MessageType = "knowledge_ingest"
)

// Message is the envelope for all MQ tasks.
type Message struct {
	Type    MessageType            `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

type EvaluationReportPayload struct {
	UserID   uint   `json:"user_id"`
	ReportID uint64 `json:"report_id"`
}

type TopicEvaluationPayload struct {
	UserID   uint   `json:"user_id"`
	ReportID uint64 `json:"report_id"`
}

type ResumeParsePayload struct {
	UserID   uint   `json:"user_id"`
	ResumeID uint64 `json:"resume_id"`
	FilePath string `json:"file_path"`
	FileSize int64  `json:"file_size"`
}

type KnowledgeIngestPayload struct {
	UserID     uint   `json:"user_id"`
	KBID       uint64 `json:"kb_id"`
	DocumentID uint64 `json:"document_id"`
	JobID      uint64 `json:"job_id"`
	FilePath   string `json:"file_path"`
	FileType   string `json:"file_type"`
}

// MessageQueue defines queue behavior.
type MessageQueue interface {
	Publish(ctx context.Context, message *Message) error
	Subscribe(ctx context.Context, handler MessageHandler) error
	Close() error
}

type MessageHandler func(ctx context.Context, message *Message) error

// InMemoryQueue is mainly for dev and tests.
type InMemoryQueue struct {
	mu       sync.RWMutex
	messages chan *Message
	handlers []MessageHandler
	done     chan struct{}
}

func NewInMemoryQueue(bufferSize int) *InMemoryQueue {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	return &InMemoryQueue{
		messages: make(chan *Message, bufferSize),
		done:     make(chan struct{}),
	}
}

func (q *InMemoryQueue) Publish(ctx context.Context, message *Message) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-q.done:
		return fmt.Errorf("message queue is closed")
	case q.messages <- message:
		return nil
	}
}

func (q *InMemoryQueue) Subscribe(ctx context.Context, handler MessageHandler) error {
	q.mu.Lock()
	q.handlers = append(q.handlers, handler)
	q.mu.Unlock()
	go q.processMessages(ctx)
	return nil
}

func (q *InMemoryQueue) processMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-q.done:
			return
		case message := <-q.messages:
			if message == nil {
				continue
			}

			q.mu.RLock()
			handlers := q.handlers
			q.mu.RUnlock()

			for _, handler := range handlers {
				go func(h MessageHandler, msg *Message) {
					if err := h(ctx, msg); err != nil {
						log.Printf("[MQ] error processing message: %v, type=%s", err, msg.Type)
					}
				}(handler, message)
			}
		}
	}
}

func (q *InMemoryQueue) Close() error {
	close(q.done)
	close(q.messages)
	return nil
}

var (
	globalMQ MessageQueue
	mqMutex  sync.RWMutex
)

func InitMessageQueue(mq MessageQueue) {
	mqMutex.Lock()
	defer mqMutex.Unlock()
	globalMQ = mq
}

func GetMessageQueue() MessageQueue {
	mqMutex.RLock()
	defer mqMutex.RUnlock()
	if globalMQ == nil {
		return NewInMemoryQueue(100)
	}
	return globalMQ
}

func PublishEvaluationReport(ctx context.Context, userID uint, reportID uint64) error {
	payload := EvaluationReportPayload{
		UserID:   userID,
		ReportID: reportID,
	}
	return publishByPayload(ctx, MessageTypeEvaluationReport, payload)
}

func PublishTopicEvaluation(ctx context.Context, userID uint, reportID uint64) error {
	payload := TopicEvaluationPayload{
		UserID:   userID,
		ReportID: reportID,
	}
	return publishByPayload(ctx, MessageTypeTopicEvaluation, payload)
}

func PublishResumeParse(ctx context.Context, userID uint, resumeID uint64, filePath string, fileSize int64) error {
	payload := ResumeParsePayload{
		UserID:   userID,
		ResumeID: resumeID,
		FilePath: filePath,
		FileSize: fileSize,
	}
	return publishByPayload(ctx, MessageTypeResumeParse, payload)
}

func PublishKnowledgeIngest(ctx context.Context, payload KnowledgeIngestPayload) error {
	return publishByPayload(ctx, MessageTypeKnowledgeIngest, payload)
}

func publishByPayload(ctx context.Context, messageType MessageType, payload interface{}) error {
	mq := GetMessageQueue()

	payloadBytes, _ := json.Marshal(payload)
	var payloadMap map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &payloadMap); err != nil {
		return errors.New("failed to marshal message payload")
	}

	message := &Message{
		Type:    messageType,
		Payload: payloadMap,
	}

	log.Printf("[MQ] publishing message type=%s", messageType)
	return mq.Publish(ctx, message)
}
```

---

## 5. L3：Consumer 异步编排

### 5.1 替换 `backend/internal/mq/consumer.go`

> 这版包含 `handleKnowledgeIngest` 全流程：  
> 读文件 -> 提取文本 -> 切块 -> 注入 metadata -> 向量入库 -> 回写状态。

```go
package mq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"interview-agents/internal/agents/usecase/evaluation"
	"interview-agents/internal/agents/usecase/resume"
	"interview-agents/internal/milvus"
	"interview-agents/internal/model"

	"github.com/cloudwego/eino/schema"
	"github.com/ledongthuc/pdf"
)

type knowledgeIngestErrorType string

const (
	knowledgeIngestErrorTypeParse     knowledgeIngestErrorType = "parse_error"
	knowledgeIngestErrorTypeEmbedding knowledgeIngestErrorType = "embedding_error"
	knowledgeIngestErrorTypeMilvus    knowledgeIngestErrorType = "milvus_write_error"
	knowledgeIngestErrorTypeUnknown   knowledgeIngestErrorType = "unknown_error"
)

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
	userID, ok := message.Payload["user_id"].(float64)
	if !ok {
		return fmt.Errorf("invalid user_id in payload")
	}
	reportID, ok := message.Payload["report_id"].(float64)
	if !ok {
		return fmt.Errorf("invalid report_id in payload")
	}
	_, err := evaluation.GenerateRecordEvaluation(ctx, uint(userID), uint64(reportID))
	return err
}

func (h *ConsumerHandler) handleTopicEvaluation(ctx context.Context, message *Message) error {
	userID, ok := message.Payload["user_id"].(float64)
	if !ok {
		return fmt.Errorf("invalid user_id in payload")
	}
	reportID, ok := message.Payload["report_id"].(float64)
	if !ok {
		return fmt.Errorf("invalid report_id in payload")
	}
	_, err := evaluation.GenerateAnswerRecordEvaluation(ctx, uint(userID), uint64(reportID))
	return err
}

func (h *ConsumerHandler) handleResumeParse(ctx context.Context, message *Message) error {
	payloadBytes, err := json.Marshal(message.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	var payload ResumeParsePayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return fmt.Errorf("invalid resume_parse payload: %w", err)
	}
	if payload.UserID == 0 || payload.ResumeID == 0 || payload.FilePath == "" {
		return fmt.Errorf("invalid resume_parse payload values")
	}
	if err = model.ResumeDao.UpdateResumeStatus(payload.ResumeID, "processing", ""); err != nil {
		log.Printf("[Consumer] failed to update resume status to processing: %v", err)
	}
	parseResult, err := resume.ParseResume(ctx, payload.UserID, payload.FilePath)
	if err != nil {
		_ = model.ResumeDao.UpdateResumeStatus(payload.ResumeID, "failed", err.Error())
		return err
	}
	contentJSON, err := json.Marshal(parseResult)
	if err != nil {
		_ = model.ResumeDao.UpdateResumeStatus(payload.ResumeID, "failed", "failed to marshal parse result")
		return err
	}
	return model.ResumeDao.UpdateResumeContent(payload.ResumeID, string(contentJSON))
}

func (h *ConsumerHandler) handleKnowledgeIngest(ctx context.Context, message *Message) error {
	start := time.Now()

	payloadBytes, err := json.Marshal(message.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	var payload KnowledgeIngestPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return fmt.Errorf("invalid knowledge_ingest payload: %w", err)
	}
	if payload.UserID == 0 || payload.KBID == 0 || payload.DocumentID == 0 || payload.JobID == 0 || strings.TrimSpace(payload.FilePath) == "" {
		return fmt.Errorf("invalid knowledge_ingest payload values")
	}

	writeStatus := func(jobStatus model.KBIngestJobStatus, docStatus model.KBDocumentStatus, chunkCount int, errorMsg string) {
		if chunkCount >= 0 {
			_ = model.KBDocumentDao.UpdateChunkCount(payload.DocumentID, chunkCount)
		}
		_ = model.KBIngestJobDao.UpdateStatus(payload.JobID, jobStatus, errorMsg)
		_ = model.KBDocumentDao.UpdateStatus(payload.DocumentID, docStatus, errorMsg)
	}

	writeStatus(model.KBIngestJobStatusProcessing, model.KBDocumentStatusProcessing, -1, "")

	rawText, err := extractKnowledgeRawText(ctx, payload.FilePath, payload.FileType)
	if err != nil {
		finalErr := fmt.Errorf("%s: %v", classifyKnowledgeIngestError(err), err)
		writeStatus(model.KBIngestJobStatusFailed, model.KBDocumentStatusFailed, 0, finalErr.Error())
		logKnowledgeIngest(payload, string(model.KBIngestJobStatusFailed), finalErr.Error(), 0, time.Since(start))
		return finalErr
	}

	manager, err := milvus.GetMilvusManager()
	if err != nil || manager.GetSplitterService() == nil || manager.GetIndexerService() == nil {
		finalErr := fmt.Errorf("%s: milvus manager/services not ready: %v", knowledgeIngestErrorTypeMilvus, err)
		writeStatus(model.KBIngestJobStatusFailed, model.KBDocumentStatusFailed, 0, finalErr.Error())
		logKnowledgeIngest(payload, string(model.KBIngestJobStatusFailed), finalErr.Error(), 0, time.Since(start))
		return finalErr
	}

	doc := &schema.Document{Content: rawText}
	chunks, err := manager.GetSplitterService().Split(ctx, []*schema.Document{doc})
	if err != nil || len(chunks) == 0 {
		finalErr := fmt.Errorf("%s: split failed: %v", knowledgeIngestErrorTypeParse, err)
		writeStatus(model.KBIngestJobStatusFailed, model.KBDocumentStatusFailed, 0, finalErr.Error())
		logKnowledgeIngest(payload, string(model.KBIngestJobStatusFailed), finalErr.Error(), 0, time.Since(start))
		return finalErr
	}

	docRecord, err := model.KBDocumentDao.GetByID(payload.DocumentID)
	if err != nil {
		finalErr := fmt.Errorf("%s: failed to load document: %v", knowledgeIngestErrorTypeUnknown, err)
		writeStatus(model.KBIngestJobStatusFailed, model.KBDocumentStatusFailed, 0, finalErr.Error())
		logKnowledgeIngest(payload, string(model.KBIngestJobStatusFailed), finalErr.Error(), 0, time.Since(start))
		return finalErr
	}

	now := time.Now().UTC().Format(time.RFC3339)
	totalChunks := len(chunks)
	for i, chunk := range chunks {
		if chunk == nil {
			continue
		}
		if chunk.ID == "" {
			chunk.ID = fmt.Sprintf("kb_%d_doc_%d_chunk_%d_%d", payload.KBID, payload.DocumentID, i, time.Now().UnixNano())
		}
		if chunk.MetaData == nil {
			chunk.MetaData = map[string]interface{}{}
		}
		chunk.MetaData["user_id"] = payload.UserID
		chunk.MetaData["kb_id"] = payload.KBID
		chunk.MetaData["document_id"] = payload.DocumentID
		chunk.MetaData["chunk_index"] = i
		chunk.MetaData["total_chunks"] = totalChunks
		chunk.MetaData["file_name"] = docRecord.FileName
		chunk.MetaData["created_at"] = now
	}

	if _, err := manager.GetIndexerService().Store(ctx, chunks); err != nil {
		finalErr := fmt.Errorf("%s: %v", classifyKnowledgeIngestError(err), err)
		writeStatus(model.KBIngestJobStatusFailed, model.KBDocumentStatusFailed, 0, finalErr.Error())
		logKnowledgeIngest(payload, string(model.KBIngestJobStatusFailed), finalErr.Error(), 0, time.Since(start))
		return finalErr
	}

	writeStatus(model.KBIngestJobStatusCompleted, model.KBDocumentStatusCompleted, totalChunks, "")
	logKnowledgeIngest(payload, string(model.KBIngestJobStatusCompleted), "", totalChunks, time.Since(start))
	return nil
}

func logKnowledgeIngest(payload KnowledgeIngestPayload, status, errorMsg string, chunkCount int, duration time.Duration) {
	log.Printf(
		"[KB Ingest] job_id=%d document_id=%d kb_id=%d user_id=%d status=%s error_msg=%q chunk_count=%d duration_ms=%d",
		payload.JobID, payload.DocumentID, payload.KBID, payload.UserID, status, errorMsg, chunkCount, duration.Milliseconds(),
	)
}

func classifyKnowledgeIngestError(err error) knowledgeIngestErrorType {
	if err == nil {
		return knowledgeIngestErrorTypeUnknown
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "embed"), strings.Contains(msg, "embedding"), strings.Contains(msg, "vector"):
		return knowledgeIngestErrorTypeEmbedding
	case strings.Contains(msg, "milvus"), strings.Contains(msg, "indexer"), strings.Contains(msg, "store"):
		return knowledgeIngestErrorTypeMilvus
	case strings.Contains(msg, "parse"), strings.Contains(msg, "pdf"), strings.Contains(msg, "read"), strings.Contains(msg, "extract"), strings.Contains(msg, "split"):
		return knowledgeIngestErrorTypeParse
	default:
		return knowledgeIngestErrorTypeUnknown
	}
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

func extractTextFromPDF(filePath string) (string, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open pdf: %w", err)
	}
	defer func() { _ = f.Close() }()
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

func StartConsumer(ctx context.Context) error {
	mq := GetMessageQueue()
	handler := NewConsumerHandler()
	log.Printf("[Consumer] starting message consumer")
	log.Printf("[Consumer] MQ type: %T", mq)
	err := mq.Subscribe(ctx, handler.HandleMessage)
	log.Printf("[Consumer] consumer stopped: %v", err)
	return err
}
```

---

## 6. L2 -> L3：上传接口入队

在 `backend/api/handler/kb/handler.go` 的 `UploadDocument` 中，创建 `doc/job` 成功后新增以下代码：

```go
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
	_ = model.KBIngestJobDao.UpdateStatus(job.ID, model.KBIngestJobStatusFailed, errMsg)
	_ = model.KBDocumentDao.UpdateStatus(doc.ID, model.KBDocumentStatusFailed, errMsg)
	response.InternalServerError(ctx, c, "failed to enqueue ingest task")
	return
}
```

并增加 import：

```go
"interview-agents/internal/mq"
```

---

## 7. 订阅队列类型补全

### 7.1 `backend/internal/mq/redis_queue.go`

`channels` 里增加：

```go
fmt.Sprintf("interview:messages:%s", MessageTypeResumeParse),
fmt.Sprintf("interview:messages:%s", MessageTypeKnowledgeIngest),
```

### 7.2 `backend/internal/mq/redis_stream.go`

`streams` 里增加：

```go
fmt.Sprintf("interview:stream:%s", MessageTypeKnowledgeIngest),
```

---

## 8. 启动时确保 Consumer 在跑

`backend/cmd/server/main.go` 必须有这两段：

1. MQ 初始化：

```go
redisClient := repository.GetRedis()
messageQueue := mq.NewRedisStreamQueue(redisClient, "interview-consumer-group", fmt.Sprintf("consumer-%s", cfg.Host))
mq.InitMessageQueue(messageQueue)
```

2. 启动消费者：

```go
consumerCtx, cancelConsumer := context.WithCancel(context.Background())
go func() {
	if err := mq.StartConsumer(consumerCtx); err != nil {
		log.Printf("Error starting consumer: %v", err)
	}
}()
time.Sleep(500 * time.Millisecond)
defer cancelConsumer()
```

---

## 9. 编译与格式化

在 `backend` 目录执行：

```bash
gofmt -w internal/mq/mq.go internal/mq/consumer.go internal/mq/redis_queue.go internal/mq/redis_stream.go api/handler/kb/handler.go api/router/custom_kb.go api/router/custom_asr.go
go test ./... -run TestDoesNotExist -count=0
```

---

## 10. 功能验证（必须跑）

## 10.1 创建知识库

```bash
curl -X POST "http://localhost:8888/api/kb/bases" \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"name":"kb-demo","description":"phase0 test"}'
```

记录返回 `kb_id`。

## 10.2 上传文档（触发异步任务）

```bash
curl -X POST "http://localhost:8888/api/kb/documents/upload" \
  -H "Authorization: Bearer <TOKEN>" \
  -F "kb_id=<KB_ID>" \
  -F "file=@./test.md"
```

预期返回：
`document_id`、`job_id`、`status=pending`。

## 10.3 轮询任务状态

```bash
curl "http://localhost:8888/api/kb/jobs/<JOB_ID>" \
  -H "Authorization: Bearer <TOKEN>"
```

预期状态变化：
`pending -> processing -> completed`。  
若失败，预期：
`status=failed` 且 `error_msg` 可读。

## 10.4 检索验证

```bash
curl -X POST "http://localhost:8888/api/kb/retrieve" \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"kb_id":<KB_ID>,"query":"你的关键词","top_k":5}'
```

预期每条结果都带：
`content/score/citation/source`。

## 10.5 日志验证（L3 关键）

检查服务日志是否出现类似：

```text
[KB Ingest] job_id=... document_id=... kb_id=... user_id=... status=completed error_msg="" chunk_count=... duration_ms=...
```

---

## 11. 常见问题排查

1. 一直 `pending`：通常是 Consumer 没启动，先看 `main.go` 的 `StartConsumer` 是否执行。
2. `failed: parse_error`：文件路径错误、PDF 无法提取文本、文本为空。
3. `failed: embedding_error`：Embedding 配置错误（模型/APIKey/endpoint）。
4. `failed: milvus_write_error`：Milvus 连接/collection/维度不一致问题。
5. 上传成功但检索不到：先看任务是否 `completed`，再检查 `metadata` 过滤表达式（`user_id/kb_id`）。

---

## 12. 验收清单（L2/L3）

1. API 均可访问（创建 KB、上传、任务查询、检索、删除）。
2. 上传后请求立即返回，不阻塞。
3. job/document 状态正确流转。
4. 失败任务有可读 `error_msg`。
5. 日志具备 `job_id/document_id/kb_id/user_id/status/error_msg/duration_ms`。
6. 检索结果结构稳定包含 `content/score/citation/source`。

