package mq

import "context"

// HandleKnowledgeIngestForRAG 是 ConsumerHandler.handleKnowledgeIngest 的导出包装，
// 供 RAG 专用消费者调用。只处理 knowledge_ingest 消息，不引入
// evaluation / resume 等业务依赖。
//
// 新增文件，未修改任何现有 MQ 代码。
func HandleKnowledgeIngestForRAG(ctx context.Context, message *Message) error {
	h := NewConsumerHandler()
	return h.handleKnowledgeIngest(ctx, message)
}

// StartRAGRetryCompensator 导出知识入库重试补偿器，供 rag-server 按需启动。
func StartRAGRetryCompensator(ctx context.Context) {
	startKnowledgeRetryCompensator(ctx)
}
