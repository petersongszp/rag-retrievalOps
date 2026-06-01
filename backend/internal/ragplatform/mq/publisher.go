package mq

import (
	"context"
	"log"

	"interview-agents/internal/ragqueue"
)

// RAGPublisher 承接文档入库任务发布，复用现有 MQ 基础设施。
type RAGPublisher struct{}

// NewRAGPublisher 创建 RAG 专用发布者
func NewRAGPublisher() *RAGPublisher {
	return &RAGPublisher{}
}

// PublishIngestTask 发布知识入库任务
func (p *RAGPublisher) PublishIngestTask(ctx context.Context, payload ragqueue.KnowledgeIngestPayload) error {
	log.Printf("[RAG-Publisher] Publishing ingest task for doc %d (kb=%d job=%d)",
		payload.DocumentID, payload.KBID, payload.JobID)
	return ragqueue.PublishKnowledgeIngest(ctx, payload)
}
