package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"interview-agents/internal/agents/usecase/evaluation"
	"interview-agents/internal/agents/usecase/resume"
	"interview-agents/internal/model"
	"log"
)

// ConsumerHandler 消费者处理器
type ConsumerHandler struct {
}

// NewConsumerHandler 创建消费者处理器
func NewConsumerHandler() *ConsumerHandler {
	return &ConsumerHandler{}
}

// HandleMessage 处理消息
func (h *ConsumerHandler) HandleMessage(ctx context.Context, message *Message) error {
	switch message.Type {
	case MessageTypeEvaluationReport:
		return h.handleEvaluationReport(ctx, message)
	case MessageTypeTopicEvaluation:
		return h.handleTopicEvaluation(ctx, message)
	case MessageTypeResumeParse:
		return h.handleResumeParse(ctx, message)
	default:
		return fmt.Errorf("unknown message type: %s", message.Type)
	}
}

// handleEvaluationReport 处理评估报告消息
func (h *ConsumerHandler) handleEvaluationReport(ctx context.Context, message *Message) error {
	log.Printf("[Consumer] Processing evaluation report message")

	// 提取负载
	userID, ok := message.Payload["user_id"].(float64)
	if !ok {
		return fmt.Errorf("invalid user_id in payload")
	}

	reportID, ok := message.Payload["report_id"].(float64)
	if !ok {
		return fmt.Errorf("invalid report_id in payload")
	}

	log.Printf("[Consumer] Generating evaluation report: userID=%d, reportID=%d", uint(userID), uint64(reportID))

	// 调用评估服务生成报告
	// 这里使用 evaluation.GenerateRecordEvaluation 生成整体评估
	_, err := evaluation.GenerateRecordEvaluation(ctx, uint(userID), uint64(reportID))
	if err != nil {
		log.Printf("[Consumer] Failed to generate evaluation report: %v", err)
		return err
	}

	log.Printf("[Consumer] Evaluation report generated successfully: userID=%d, reportID=%d", uint(userID), uint64(reportID))
	return nil
}

// handleTopicEvaluation 处理主题评估消息
func (h *ConsumerHandler) handleTopicEvaluation(ctx context.Context, message *Message) error {
	log.Printf("[Consumer] Processing topic evaluation message")

	// 提取负载
	userID, ok := message.Payload["user_id"].(float64)
	if !ok {
		return fmt.Errorf("invalid user_id in payload")
	}

	reportID, ok := message.Payload["report_id"].(float64)
	if !ok {
		return fmt.Errorf("invalid report_id in payload")
	}

	log.Printf("[Consumer] Generating topic evaluation: userID=%d, reportID=%d", uint(userID), uint64(reportID))

	// 调用评估服务生成主题评估
	// 这里使用 GenerateAnswerRecordEvaluation 生成答题记录的评估
	_, err := evaluation.GenerateAnswerRecordEvaluation(ctx, uint(userID), uint64(reportID))
	if err != nil {
		log.Printf("[Consumer] Failed to generate topic evaluation: %v", err)
		return err
	}

	log.Printf("[Consumer] Topic evaluation generated successfully: userID=%d, reportID=%d", uint(userID), uint64(reportID))
	return nil
}

// handleResumeParse 处理简历解析消息
func (h *ConsumerHandler) handleResumeParse(ctx context.Context, message *Message) error {
	log.Printf("[Consumer] Processing resume parse message")

	// 提取负载
	userID, ok := message.Payload["user_id"].(float64)
	if !ok {
		return fmt.Errorf("invalid user_id in payload")
	}

	resumeID, ok := message.Payload["resume_id"].(float64)
	if !ok {
		return fmt.Errorf("invalid resume_id in payload")
	}

	filePath, ok := message.Payload["file_path"].(string)
	if !ok {
		return fmt.Errorf("invalid file_path in payload")
	}

	log.Printf("[Consumer] Parsing resume: userID=%d, resumeID=%d, filePath=%s", uint(userID), uint64(resumeID), filePath)

	// 更新状态为 processing
	err := model.ResumeDao.UpdateResumeStatus(uint64(resumeID), "processing", "")
	if err != nil {
		log.Printf("[Consumer] Failed to update resume status to processing: %v", err)
		// 继续处理，不因状态更新失败而中断
	}

	// 调用简历解析服务
	parseResult, err := resume.ParseResume(ctx, uint(userID), filePath)
	if err != nil {
		log.Printf("[Consumer] Failed to parse resume: %v", err)
		// 更新状态为 failed
		updateErr := model.ResumeDao.UpdateResumeStatus(uint64(resumeID), "failed", err.Error())
		if updateErr != nil {
			log.Printf("[Consumer] Failed to update resume status to failed: %v", updateErr)
		}
		return err
	}

	// 将解析结果序列化为 JSON
	contentJSON, err := json.Marshal(parseResult)
	if err != nil {
		log.Printf("[Consumer] Failed to marshal parse result: %v", err)
		updateErr := model.ResumeDao.UpdateResumeStatus(uint64(resumeID), "failed", "JSON序列化失败")
		if updateErr != nil {
			log.Printf("[Consumer] Failed to update resume status: %v", updateErr)
		}
		return err
	}

	// 更新简历内容（状态会自动变为 completed）
	err = model.ResumeDao.UpdateResumeContent(uint64(resumeID), string(contentJSON))
	if err != nil {
		log.Printf("[Consumer] Failed to update resume content: %v", err)
		return err
	}

	log.Printf("[Consumer] Resume parsed successfully: userID=%d, resumeID=%d", uint(userID), uint64(resumeID))
	return nil
}

// StartConsumer 启动消费者
func StartConsumer(ctx context.Context) error {
	mq := GetMessageQueue()
	handler := NewConsumerHandler()

	log.Printf("[Consumer] Starting message consumer")
	log.Printf("[Consumer] MQ type: %T", mq)

	err := mq.Subscribe(ctx, handler.HandleMessage)
	log.Printf("[Consumer] Consumer stopped: %v", err)
	return err
}
