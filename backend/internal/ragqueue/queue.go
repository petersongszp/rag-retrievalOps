package ragqueue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
)

type MessageType string

const (
	MessageTypeKnowledgeIngest MessageType = "knowledge_ingest"
)

type Message struct {
	Type    MessageType            `json:"type"`
	Payload map[string]interface{} `json:"payload"`
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

type MessageQueue interface {
	Publish(ctx context.Context, message *Message) error
	Subscribe(ctx context.Context, handler MessageHandler) error
	Close() error
}

type MessageHandler func(ctx context.Context, message *Message) error

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
						log.Printf("[RAG Queue] error processing message: %v, type=%s", err, msg.Type)
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

func GetMessageQueue() MessageQueue {
	mqMutex.RLock()
	defer mqMutex.RUnlock()
	if globalMQ == nil {
		return NewInMemoryQueue(100)
	}
	return globalMQ
}

func PauseKnowledgeIngest() {
	ingestPausedMutex.Lock()
	defer ingestPausedMutex.Unlock()
	if !ingestPaused {
		ingestPaused = true
		log.Println("[RAG Queue] Knowledge ingest paused")
	}
}

func ResumeKnowledgeIngest() {
	ingestPausedMutex.Lock()
	defer ingestPausedMutex.Unlock()
	if ingestPaused {
		ingestPaused = false
		log.Println("[RAG Queue] Knowledge ingest resumed")
	}
}

func IsKnowledgeIngestPaused() bool {
	ingestPausedMutex.RLock()
	defer ingestPausedMutex.RUnlock()
	return ingestPaused
}

func PublishKnowledgeIngest(ctx context.Context, payload KnowledgeIngestPayload) error {
	mq := GetMessageQueue()

	payloadBytes, _ := json.Marshal(payload)
	var payloadMap map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &payloadMap); err != nil {
		return errors.New("failed to marshal message payload")
	}

	message := &Message{
		Type:    MessageTypeKnowledgeIngest,
		Payload: payloadMap,
	}

	log.Printf("[RAG Queue] publishing message type=%s", MessageTypeKnowledgeIngest)
	return mq.Publish(ctx, message)
}
