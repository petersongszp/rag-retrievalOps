package router

import (
	"log"

	"github.com/cloudwego/hertz/pkg/app/server"
)

// RegisterRAGRoutes 只注册 RAG 相关路由
// 用于 cmd/rag-server 独立启动，不依赖面试吧业务路由
func RegisterRAGRoutes(h *server.Hertz) {
	// 注册知识库路由（包含 /api/kb 和 /api/admin/kb）
	registerKnowledgeBaseRoutes(h)

	log.Println("[RAG] RAG routes registered (rag-server mode)")
}
