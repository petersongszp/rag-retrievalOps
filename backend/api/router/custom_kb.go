package router

import (
	kb "interview-agents/api/handler/kb"
	"interview-agents/internal/config"
	"log"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/route"
)

func registerKnowledgeBaseRoutes(r *server.Hertz) {
	if !config.Global.RAG.Enabled {
		log.Println("[RAG] RAG is disabled, skipping knowledge base routes registration")
		return
	}
	log.Println("[RAG] Registering knowledge base routes")
	registerKBGroup(r.Group("/api/kb"), false)
	registerKBGroup(r.Group("/api/admin/kb"), true)
}

func registerKBGroup(group *route.RouterGroup, adminOnly bool) {
	group.GET("/dashboard/stats", kb.GetDashboardStats)
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
	group.GET("/metrics/overview", kb.GetMetricsOverview)
	group.POST("/ingest/pause", kb.PauseIngest)
	group.POST("/ingest/resume", kb.ResumeIngest)
	group.GET("/ingest/status", kb.GetIngestStatus)
	if adminOnly {
		group.GET("/release/status", kb.GetReleaseStatus)
		group.GET("/release/summary", kb.GetReleaseSummary)
		group.POST("/release/rollback", kb.RollbackRelease)
		group.POST("/release/activate", kb.ActivateRelease)
	}
}
