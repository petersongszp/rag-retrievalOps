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
	// 步骤1：记录处理开始时间，用于统计耗时
	start := time.Now()
	// 步骤2：序列化+反序列化，解析消息载荷为业务结构体
	// 把消息的Payload转成JSON字节
	payloadBytes, err := json.Marshal(message.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	// 把JSON字节解析为 KnowledgeIngestPayload（知识录入的参数结构体）
	var payload KnowledgeIngestPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return fmt.Errorf("invalid knowledge_ingest payload: %w", err)
	}
	// 步骤3：必填参数校验（核心参数不能为空/0）
	// 校验：用户ID、知识库ID、文档ID、任务ID、文件路径 必须合法
	if payload.UserID == 0 || payload.KBID == 0 || payload.DocumentID == 0 || payload.JobID == 0 || strings.TrimSpace(payload.FilePath) == "" {
		return fmt.Errorf("invalid knowledge_ingest payload values: user_id=%d kb_id=%d document_id=%d job_id=%d file_path=%q",
			payload.UserID, payload.KBID, payload.DocumentID, payload.JobID, payload.FilePath)
	}

	// =====================================================================
	// 步骤4：定义内部闭包函数 writeStatus（核心：更新数据库状态）
	// 作用：统一更新「录入任务状态」「文档状态」「文档分块数」
	// 参数：任务状态、文档状态、分块数量、错误信息
	// 特点：失败只打日志，不中断主流程（保证状态尽量落地）
	// =====================================================================
	writeStatus := func(jobStatus model.KBIngestJobStatus, docStatus model.KBDocumentStatus, chunkCount int, errorMsg string) {
		// 1. 如果分块数≥0，更新数据库中文档的分块总数
		if chunkCount >= 0 {
			if err := model.KBDocumentDao.UpdateChunkCount(payload.DocumentID, chunkCount); err != nil {
				log.Printf("[KB Ingest] failed to update chunk_count: job_id=%d document_id=%d err=%v", payload.JobID, payload.DocumentID, err)
			}
		}
		// 2. 更新数据库中「录入任务」的状态+错误信息
		if err := model.KBIngestJobDao.UpdateStatus(payload.JobID, jobStatus, errorMsg); err != nil {
			log.Printf("[KB Ingest] failed to update job status: job_id=%d status=%s err=%v", payload.JobID, jobStatus, err)
		}
		// 3. 更新数据库中「文档」的状态+错误信息
		if err := model.KBDocumentDao.UpdateStatus(payload.DocumentID, docStatus, errorMsg); err != nil {
			log.Printf("[KB Ingest] failed to update document status: document_id=%d status=%s err=%v", payload.DocumentID, docStatus, err)
		}
	}
	// 步骤5：初始状态：标记「任务+文档」为 处理中(Processing)
	writeStatus(model.KBIngestJobStatusProcessing, model.KBDocumentStatusProcessing, -1, "")
	// 步骤6：提取文件原始文本
	// 调用工具方法：根据文件路径+文件类型，读取并提取纯文本内容
	rawText, err := extractKnowledgeRawText(ctx, payload.FilePath, payload.FileType)
	if err != nil {
		// 提取失败：分类错误类型 → 更新状态为失败 → 记录日志 → 返回错误
		errType := classifyKnowledgeIngestError(err)
		finalErr := fmt.Errorf("%s: %v", errType, err)
		writeStatus(model.KBIngestJobStatusFailed, model.KBDocumentStatusFailed, 0, finalErr.Error())
		logKnowledgeIngest(payload, string(model.KBIngestJobStatusFailed), finalErr.Error(), 0, time.Since(start))
		return finalErr
	}
	// 步骤7：获取向量数据库(Milvus)管理器
	manager, err := milvus.GetMilvusManager()
	if err != nil {
		// Milvus连接失败：标记失败 → 日志 → 返回
		finalErr := fmt.Errorf("%s: %v", knowledgeIngestErrorTypeMilvus, err)
		writeStatus(model.KBIngestJobStatusFailed, model.KBDocumentStatusFailed, 0, finalErr.Error())
		logKnowledgeIngest(payload, string(model.KBIngestJobStatusFailed), finalErr.Error(), 0, time.Since(start))
		return finalErr
	}
	// 校验Milvus的两个核心服务是否初始化完成
	if manager.GetSplitterService() == nil || manager.GetIndexerService() == nil {
		finalErr := fmt.Errorf("%s: milvus manager services are not initialized", knowledgeIngestErrorTypeMilvus)
		writeStatus(model.KBIngestJobStatusFailed, model.KBDocumentStatusFailed, 0, finalErr.Error())
		logKnowledgeIngest(payload, string(model.KBIngestJobStatusFailed), finalErr.Error(), 0, time.Since(start))
		return finalErr
	}
	// 步骤8：文本分块（知识库核心：把长文本切成小片段）
	// 封装文档对象 → 调用拆分服务切割文本
	doc := &schema.Document{Content: rawText}
	chunks, err := manager.GetSplitterService().Split(ctx, []*schema.Document{doc})
	if err != nil {
		errType := classifyKnowledgeIngestError(err)
		finalErr := fmt.Errorf("%s: %v", errType, err)
		writeStatus(model.KBIngestJobStatusFailed, model.KBDocumentStatusFailed, 0, finalErr.Error())
		logKnowledgeIngest(payload, string(model.KBIngestJobStatusFailed), finalErr.Error(), 0, time.Since(start))
		return finalErr
	}
	// 分块失败：标记失败 → 日志 → 返回
	if len(chunks) == 0 {
		finalErr := fmt.Errorf("%s: empty chunks after split", knowledgeIngestErrorTypeParse)
		writeStatus(model.KBIngestJobStatusFailed, model.KBDocumentStatusFailed, 0, finalErr.Error())
		logKnowledgeIngest(payload, string(model.KBIngestJobStatusFailed), finalErr.Error(), 0, time.Since(start))
		return finalErr
	}
	// 步骤9：从数据库查询文档原始信息（主要拿文件名）
	docRecord, err := model.KBDocumentDao.GetByID(payload.DocumentID)
	if err != nil {
		finalErr := fmt.Errorf("%s: failed to load document: %v", knowledgeIngestErrorTypeUnknown, err)
		writeStatus(model.KBIngestJobStatusFailed, model.KBDocumentStatusFailed, 0, finalErr.Error())
		logKnowledgeIngest(payload, string(model.KBIngestJobStatusFailed), finalErr.Error(), 0, time.Since(start))
		return finalErr
	}
	// 步骤10：填充分块的元数据（关键：给每个文本块加标识，方便后续检索）
	baseMeta := milvus.NewKBDocumentMetadata(payload.UserID, payload.KBID, payload.DocumentID, docRecord.FileName)
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
		baseMeta.ChunkIndex = i
		baseMeta.TotalChunks = totalChunks
		for k, v := range baseMeta.ToMap() {
			chunk.MetaData[k] = v
		}
	}
	// 步骤11：将分块存入向量数据库(Milvus)
	if _, err := manager.GetIndexerService().Store(ctx, chunks); err != nil {
		errType := classifyKnowledgeIngestError(err)
		finalErr := fmt.Errorf("%s: %v", errType, err)
		writeStatus(model.KBIngestJobStatusFailed, model.KBDocumentStatusFailed, 0, finalErr.Error())
		logKnowledgeIngest(payload, string(model.KBIngestJobStatusFailed), finalErr.Error(), 0, time.Since(start))
		return finalErr
	}
	// 步骤12：处理完成！更新状态为成功
	writeStatus(model.KBIngestJobStatusCompleted, model.KBDocumentStatusCompleted, totalChunks, "")
	logKnowledgeIngest(payload, string(model.KBIngestJobStatusCompleted), "", totalChunks, time.Since(start))
	return nil
}

// 参数解释：
// payload: 知识录入请求参数（含任务ID、文档ID等）
// status: 任务状态（处理中/失败/完成）
// errorMsg: 错误信息（成功为空字符串）
// chunkCount: 文本分块数量
// duration: 任务总耗时
func logKnowledgeIngest(payload KnowledgeIngestPayload, status, errorMsg string, chunkCount int, duration time.Duration) {
	// 格式化打印日志，所有核心字段一一对应
	log.Printf(
		"[KB Ingest] job_id=%d document_id=%d kb_id=%d user_id=%d status=%s error_msg=%q chunk_count=%d duration_ms=%d",
		payload.JobID,           // 唯一任务ID
		payload.DocumentID,      // 文档ID
		payload.KBID,            // 知识库ID
		payload.UserID,          // 用户ID
		status,                  // 处理状态
		errorMsg,                // 错误信息（空则为""）
		chunkCount,              // 文本分块总数
		duration.Milliseconds(), // 耗时（转换为毫秒，方便统计）
	)
}

// 返回值：自定义枚举类型 knowledgeIngestErrorType（定义了：嵌入失败/向量库失败/解析失败/未知错误）
func classifyKnowledgeIngestError(err error) knowledgeIngestErrorType {
	// 空错误直接返回未知类型
	if err == nil {
		return knowledgeIngestErrorTypeUnknown
	}
	// 把错误信息转小写，避免大小写导致匹配失败
	msg := strings.ToLower(err.Error())
	// 关键词匹配分类
	switch {
	// 匹配：嵌入、向量生成相关错误
	case strings.Contains(msg, "embed"), strings.Contains(msg, "embedding"), strings.Contains(msg, "vector"):
		return knowledgeIngestErrorTypeEmbedding
	// 匹配：向量数据库Milvus、存储相关错误
	case strings.Contains(msg, "milvus"), strings.Contains(msg, "indexer"), strings.Contains(msg, "store"):
		return knowledgeIngestErrorTypeMilvus
	// 匹配：文件解析、读取、切割、PDF处理相关错误
	case strings.Contains(msg, "parse"), strings.Contains(msg, "pdf"), strings.Contains(msg, "read"), strings.Contains(msg, "extract"), strings.Contains(msg, "split"):
		return knowledgeIngestErrorTypeParse
	// 其他所有错误：未知类型
	default:
		return knowledgeIngestErrorTypeUnknown
	}
}

// 参数：ctx上下文、文件路径、文件类型
// 返回：提取的纯文本、错误
func extractKnowledgeRawText(ctx context.Context, filePath, fileType string) (string, error) {
	_ = ctx // 上下文暂未使用，保留用于未来扩展（超时、链路追踪）

	// 1. 基础校验：文件路径不能为空
	if strings.TrimSpace(filePath) == "" {
		return "", fmt.Errorf("file path is empty")
	}
	// 2. 校验文件是否存在、是否可访问
	if _, err := os.Stat(filePath); err != nil {
		return "", fmt.Errorf("failed to access file: %w", err)
	}

	// 3. 标准化文件类型（转小写、去空格）
	normalizedType := strings.ToLower(strings.TrimSpace(fileType))
	// 如果未传入文件类型，自动从文件路径中提取后缀（如 .pdf → pdf）
	if normalizedType == "" {
		normalizedType = strings.TrimPrefix(strings.ToLower(filepath.Ext(filePath)), ".")
	}

	// 4. 根据文件类型分支处理
	switch normalizedType {
	// 文本类文件：直接读取
	case "txt", "md", "markdown":
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to read text file: %w", err)
		}
		// 转字符串并去除首尾空白
		text := strings.TrimSpace(string(data))
		// 空文本直接报错
		if text == "" {
			return "", fmt.Errorf("empty text content")
		}
		return text, nil
	// PDF文件：调用专用PDF提取函数
	case "pdf":
		return extractTextFromPDF(filePath)
	// 不支持的文件类型：报错
	default:
		return "", fmt.Errorf("unsupported file type: %s", normalizedType)
	}
}

// 参数：PDF文件路径
// 返回：PDF纯文本、错误
func extractTextFromPDF(filePath string) (string, error) {
	// 打开PDF文件（使用第三方pdf库）
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open pdf: %w", err)
	}
	// 延迟关闭文件句柄，防止资源泄漏
	defer func() {
		_ = f.Close()
	}()

	// 字符串Builder：高效拼接文本
	var builder strings.Builder
	// 获取PDF总页数
	totalPages := r.NumPage()

	// 逐页遍历提取文本
	for pageIndex := 1; pageIndex <= totalPages; pageIndex++ {
		p := r.Page(pageIndex)
		// 跳过空页面
		if p.V.IsNull() {
			continue
		}
		// 提取当前页纯文本
		pageText, err := p.GetPlainText(nil)
		if err != nil {
			// 报错携带页码，方便定位损坏页面
			return "", fmt.Errorf("failed to extract pdf text on page %d: %w", pageIndex, err)
		}
		// 拼接文本，每页换行
		builder.WriteString(pageText)
		builder.WriteString("\n")
	}

	// 最终文本去空白
	text := strings.TrimSpace(builder.String())
	// 空内容报错
	if text == "" {
		return "", errors.New("empty text extracted from pdf")
	}
	return text, nil
}

// StartConsumer starts MQ consumer loop.
// 注释：启动MQ消费者循环
// 参数：ctx上下文（用于优雅关闭消费者）
func StartConsumer(ctx context.Context) error {
	// 1. 获取全局消息队列实例（单例）
	mq := GetMessageQueue()
	// 2. 创建消息处理器（就是包含handleKnowledgeIngest的结构体）
	handler := NewConsumerHandler()

	// 打印启动日志
	log.Printf("[Consumer] starting message consumer")
	// 打印MQ类型，方便调试（确认使用的是Kafka/RabbitMQ等）
	log.Printf("[Consumer] MQ type: %T", mq)

	// 3. 核心：订阅消息队列，绑定HandleMessage处理方法
	// 程序会阻塞在这里，持续消费消息，直到上下文关闭/异常退出
	err := mq.Subscribe(ctx, handler.HandleMessage)
	// 消费者停止后打印日志
	log.Printf("[Consumer] consumer stopped: %v", err)
	return err
}
