package router

import (
	kb "interview-agents/api/handler/kb"

	"github.com/cloudwego/hertz/pkg/app/server"
)

func registerKnowledgeBaseRoutes(r *server.Hertz) {
	kbGroup := r.Group("/api/kb")
	kbGroup.POST("/bases", kb.CreateKnowledgeBase)
	kbGroup.GET("/bases", kb.ListKnowledgeBases)
	kbGroup.POST("/documents/upload", kb.UploadDocument)
	kbGroup.GET("/documents", kb.ListDocuments)
	kbGroup.GET("/jobs", kb.ListJobs)
	kbGroup.GET("/jobs/:job_id", kb.GetJob)
	kbGroup.POST("/jobs/:job_id/retry", kb.RetryJob)
	kbGroup.POST("/jobs/:job_id/cancel", kb.CancelJob)
	kbGroup.DELETE("/documents/:document_id", kb.DeleteDocument)
	kbGroup.POST("/retrieve", kb.Retrieve)
}
