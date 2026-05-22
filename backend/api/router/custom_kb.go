package router

import (
	kb "interview-agents/api/handler/kb"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/route"
)

func registerKnowledgeBaseRoutes(r *server.Hertz) {
	registerKBGroup(r.Group("/api/kb"))
	registerKBGroup(r.Group("/api/admin/kb"))
}

func registerKBGroup(group *route.RouterGroup) {
	group.POST("/bases", kb.CreateKnowledgeBase)
	group.GET("/bases", kb.ListKnowledgeBases)
	group.POST("/documents/upload", kb.UploadDocument)
	group.GET("/documents", kb.ListDocuments)
	group.GET("/jobs", kb.ListJobs)
	group.GET("/jobs/:job_id", kb.GetJob)
	group.POST("/jobs/:job_id/retry", kb.RetryJob)
	group.POST("/jobs/:job_id/cancel", kb.CancelJob)
	group.DELETE("/documents/:document_id", kb.DeleteDocument)
	group.POST("/retrieve", kb.Retrieve)
	group.GET("/retrieve/audit/:request_id", kb.GetRetrieveAuditLog)
	group.GET("/retrieve/audit", kb.ListRetrieveAuditLogs)
}
