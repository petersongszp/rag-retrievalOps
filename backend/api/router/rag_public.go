package router

import (
	rag "interview-agents/api/handler/rag"

	"github.com/cloudwego/hertz/pkg/route"
)

// registerRAGPublicRoutes registers the v1 public RAG API routes.
// These routes provide a stable platform-level entry point for external agents.
func registerRAGPublicRoutes(r *route.RouterGroup) {
	v1 := r.Group("/v1")
	{
		v1.POST("/retrieve", rag.Retrieve)
	}
}
