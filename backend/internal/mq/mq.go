package mq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
)

// MessageType 消息类型
type MessageType string

const (
	// MessageTypeEvaluationReport 评估报告生成消息
	MessageTypeEvaluationReport MessageType = "evaluation_report"
	// MessageTypeTopicEvaluation 主题评估消息
	MessageTypeTopicEvaluation MessageType = "topic_evaluation"
	// MessageTypeResumeParse 简历解析消息
	MessageTypeResumeParse MessageType = "resume_parse"
)

// Message 消息结构
type Message struct {
	Type    MessageType            `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

// EvaluationReportPayload 评估报告消息负载
type EvaluationReportPayload struct {
	UserID   uint   `json:"user_id"`
	ReportID uint64 `json:"report_id"`
}

// TopicEvaluationPayload 主题评估消息负载
type TopicEvaluationPayload struct {
	UserID   uint   `json:"user_id"`
	ReportID uint64 `json:"report_id"`
}

// ResumeParsePayload 简历解析消息负载
type ResumeParsePayload struct {
	UserID   uint   `json:"user_id"`
	ResumeID uint64 `json:"resume_id"`
	FilePath string `json:"file_path"`
	FileSize int64  `json:"file_size"`
}

// MessageQueue 消息队列接口
type MessageQueue interface {
	// Publish 发布消息
	Publish(ctx context.Context, message *Message) error
	// Subscribe 订阅消息
	Subscribe(ctx context.Context, handler MessageHandler) error
	// Close 关闭消息队列
	Close() error
}

// MessageHandler 消息处理器
type MessageHandler func(ctx context.Context, message *Message) error

// InMemoryQueue 内存消息队列（用于开发和测试）
type InMemoryQueue struct {
	mu       sync.RWMutex
	messages chan *Message
	handlers []MessageHandler
	done     chan struct{}
}

// NewInMemoryQueue 创建内存消息队列
func NewInMemoryQueue(bufferSize int) *InMemoryQueue {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	return &InMemoryQueue{
		messages: make(chan *Message, bufferSize),
		done:     make(chan struct{}),
	}
}

// Publish 发布消息
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

// Subscribe 订阅消息
func (q *InMemoryQueue) Subscribe(ctx context.Context, handler MessageHandler) error {
	q.mu.Lock()
	q.handlers = append(q.handlers, handler)
	q.mu.Unlock()

	// 启动消息处理 goroutine
	go q.processMessages(ctx)
	return nil
}

// processMessages 处理消息
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

			// 异步调用所有处理器
			for _, handler := range handlers {
				go func(h MessageHandler, msg *Message) {
					if err := h(ctx, msg); err != nil {
						log.Printf("[MQ] Error processing message: %v, type: %s", err, msg.Type)
					}
				}(handler, message)
			}
		}
	}
}

// Close 关闭消息队列
func (q *InMemoryQueue) Close() error {
	close(q.done)
	close(q.messages)
	return nil
}

// Global message queue instance
var (
	globalMQ MessageQueue
	mqMutex  sync.RWMutex
)

// InitMessageQueue 初始化全局消息队列
func InitMessageQueue(mq MessageQueue) {
	mqMutex.Lock()
	defer mqMutex.Unlock()
	globalMQ = mq
}

// GetMessageQueue 获取全局消息队列
func GetMessageQueue() MessageQueue {
	mqMutex.RLock()
	defer mqMutex.RUnlock()
	if globalMQ == nil {
		// 默认使用内存队列
		return NewInMemoryQueue(100)
	}
	return globalMQ
}

// PublishEvaluationReport 发布评估报告生成消息
func PublishEvaluationReport(ctx context.Context, userID uint, reportID uint64) error {
	mq := GetMessageQueue()
	log.Printf("[MQ] GetMessageQueue type: %T", mq)

	payload := EvaluationReportPayload{
		UserID:   userID,
		ReportID: reportID,
	}

	payloadBytes, _ := json.Marshal(payload)
	var payloadMap map[string]interface{}
	err := json.Unmarshal(payloadBytes, &payloadMap)
	if err != nil {
		return errors.New("反序列化失败")
	}

	message := &Message{
		Type:    MessageTypeEvaluationReport,
		Payload: payloadMap,
	}

	log.Printf("[MQ] Publishing evaluation report message: userID=%d, reportID=%d, MQ type: %T", userID, reportID, mq)
	err = mq.Publish(ctx, message)
	if err != nil {
		log.Printf("[MQ] Failed to publish evaluation report message: %v", err)
	} else {
		log.Printf("[MQ] Successfully published evaluation report message")
	}
	return err
}

// PublishTopicEvaluation 发布主题评估消息
func PublishTopicEvaluation(ctx context.Context, userID uint, reportID uint64) error {
	mq := GetMessageQueue()
	payload := TopicEvaluationPayload{
		UserID:   userID,
		ReportID: reportID,
	}

	payloadBytes, _ := json.Marshal(payload)
	var payloadMap map[string]interface{}
	err := json.Unmarshal(payloadBytes, &payloadMap)
	if err != nil {
		return errors.New("反序列化失败")
	}
	message := &Message{
		Type:    MessageTypeTopicEvaluation,
		Payload: payloadMap,
	}

	log.Printf("[MQ] Publishing topic evaluation message: userID=%d, reportID=%d", userID, reportID)
	return mq.Publish(ctx, message)
}

// PublishResumeParse 发布简历解析消息
func PublishResumeParse(ctx context.Context, userID uint, resumeID uint64, filePath string, fileSize int64) error {
	mq := GetMessageQueue()
	payload := ResumeParsePayload{
		UserID:   userID,
		ResumeID: resumeID,
		FilePath: filePath,
		FileSize: fileSize,
	}

	payloadBytes, _ := json.Marshal(payload)
	var payloadMap map[string]interface{}
	err := json.Unmarshal(payloadBytes, &payloadMap)
	if err != nil {
		return errors.New("反序列化失败")
	}
	message := &Message{
		Type:    MessageTypeResumeParse,
		Payload: payloadMap,
	}

	log.Printf("[MQ] Publishing resume parse message: userID=%d, resumeID=%d, filePath=%s", userID, resumeID, filePath)
	return mq.Publish(ctx, message)
}
