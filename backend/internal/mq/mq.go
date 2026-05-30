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
	UserID          uint   `json:"user_id"`
	OperatorAdminID uint   `json:"operator_admin_id,omitempty"`
	KBID            uint64 `json:"kb_id"`
	DocumentID      uint64 `json:"document_id"`
	JobID           uint64 `json:"job_id"`
	FilePath        string `json:"file_path"`
	FileType        string `json:"file_type"`
	Collection      string `json:"collection,omitempty"`
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
	globalMQ          MessageQueue
	mqMutex           sync.RWMutex
	ingestPaused      bool
	ingestPausedMutex sync.RWMutex
)

func InitMessageQueue(mq MessageQueue) {
	mqMutex.Lock()
	defer mqMutex.Unlock()
	globalMQ = mq
}

// PauseKnowledgeIngest pauses knowledge ingest consumption
func PauseKnowledgeIngest() {
	ingestPausedMutex.Lock()
	defer ingestPausedMutex.Unlock()
	if !ingestPaused {
		ingestPaused = true
		log.Println("[MQ] Knowledge ingest paused")
	}
}

// ResumeKnowledgeIngest resumes knowledge ingest consumption
func ResumeKnowledgeIngest() {
	ingestPausedMutex.Lock()
	defer ingestPausedMutex.Unlock()
	if ingestPaused {
		ingestPaused = false
		log.Println("[MQ] Knowledge ingest resumed")
	}
}

// IsKnowledgeIngestPaused checks if knowledge ingest is paused
func IsKnowledgeIngestPaused() bool {
	ingestPausedMutex.RLock()
	defer ingestPausedMutex.RUnlock()
	return ingestPaused
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
