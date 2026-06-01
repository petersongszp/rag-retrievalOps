package mq

import (
	"context"
	"log"

	"interview-agents/internal/mq"
)

// RAGConsumer 只处理知识入库任务，不承载简历解析、评估报告等业务消息。
// 它包装了底层 mq.MessageQueue，使用独立的 handler 过滤消息类型。
type RAGConsumer struct {
	queue mq.MessageQueue
}

// NewRAGConsumer 创建 RAG 专用消费者
func NewRAGConsumer(queue mq.MessageQueue) *RAGConsumer {
	return &RAGConsumer{queue: queue}
}

// Start 启动 RAG 消费者，只处理 knowledge_ingest 消息
func (c *RAGConsumer) Start(ctx context.Context) error {
	log.Println("[RAG-Consumer] Starting RAG ingest consumer...")
	return c.queue.Subscribe(ctx, c.handleMessage)
}

// handleMessage 只处理知识入库任务，忽略其他消息类型
func (c *RAGConsumer) handleMessage(ctx context.Context, message *mq.Message) error {
	if message.Type != mq.MessageTypeKnowledgeIngest {
		log.Printf("[RAG-Consumer] Ignoring non-RAG message type: %s", message.Type)
		return nil
	}

	log.Printf("[RAG-Consumer] Processing knowledge_ingest message")
	return mq.HandleKnowledgeIngestForRAG(ctx, message)
}
