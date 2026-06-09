package ragrouter

import (
	"log"

	authhandler "interview-agents/api/handler/auth"
	kb "interview-agents/api/handler/kb"
	rag "interview-agents/api/handler/rag"
	authpkg "interview-agents/internal/auth"
	"interview-agents/internal/config"
	"interview-agents/internal/middleware"
	"interview-agents/internal/rag/phase3"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/route"
	"gorm.io/gorm"
)

var jwtManager *authpkg.JWTManager

// SetJWTManager 设置路由层使用的 JWT 管理器（在 Register 之前调用）
func SetJWTManager(m *authpkg.JWTManager) {
	jwtManager = m
}

// Register wires the RAG admin and public retrieval routes.
// adminGroup is the /api/admin group with auth middleware already applied.
func Register(h *server.Hertz, adminGroup *route.RouterGroup) {
	if !config.Global.RAG.Enabled {
		log.Println("[RAG] rag.enabled=false, skip route registration")
		return
	}

	log.Println("[RAG] Registering knowledge base routes")
	registerKBGroup(h.Group("/api/kb"), false)
	registerKBGroup(adminGroup.Group("/kb"), true)

	log.Println("[RAG] Registering v1 public RAG routes")
	registerRAGPublicRoutes(h.Group(""))

	log.Println("[RAG] Registering auth routes")
	// 公开路由（不需要认证）
	publicAuth := h.Group("/v1/auth")
	{
		publicAuth.POST("/register", authhandler.Register)
		publicAuth.POST("/login", authhandler.Login)
		publicAuth.POST("/refresh", authhandler.Refresh)
	}
	// 需要认证的路由
	if jwtManager != nil {
		protectedAuth := h.Group("/v1/auth")
		protectedAuth.Use(middleware.JWTAuth(jwtManager))
		{
			protectedAuth.GET("/me", authhandler.Me)
			protectedAuth.PUT("/password", authhandler.ChangePassword)
		}
	} else {
		log.Println("[RAG] Warning: jwtManager not set, protected auth routes registered without JWT middleware")
		authGroup := h.Group("/v1/auth")
		{
			authGroup.GET("/me", authhandler.Me)
			authGroup.PUT("/password", authhandler.ChangePassword)
		}
	}

	// API Key 管理路由（需要 JWT 认证）
	if jwtManager != nil {
		log.Println("[RAG] Registering API key management routes")
		apiKeyGroup := h.Group("/v1/api-keys")
		apiKeyGroup.Use(middleware.JWTAuth(jwtManager))
		{
			apiKeyGroup.GET("", authhandler.ListAPIKeys)
			apiKeyGroup.POST("", authhandler.CreateAPIKey)
			apiKeyGroup.PUT("/:id", authhandler.UpdateAPIKey)
			apiKeyGroup.POST("/:id/rotate", authhandler.RotateAPIKey)
			apiKeyGroup.DELETE("/:id", authhandler.DeleteAPIKey)
		}

		log.Println("[RAG] Registering tenant routes")
		tenantGroup := h.Group("/v1/tenant")
		tenantGroup.Use(middleware.JWTAuth(jwtManager))
		{
			tenantGroup.GET("", authhandler.GetTenant)
			tenantGroup.GET("/usage", authhandler.GetTenantUsage)
		}
	}
}

func registerRAGPublicRoutes(r *route.RouterGroup) {
	v1 := r.Group("/v1")
	{
		// /v1/retrieve 支持多种认证：API Key > JWT > Legacy app_id
		// 使用 OptionalAuth 中间件，不强制认证，让 handler 内部判断
		v1.POST("/retrieve", rag.Retrieve)
	}
}

// InitAuthHandler initializes auth handler dependencies (call after DB is ready)
func InitAuthHandler(db *gorm.DB, cfg *config.Config) {
	authhandler.InitAuthHandler(db, cfg)
	authhandler.InitAPIKeyHandler(db)
}

func registerKBGroup(group *route.RouterGroup, adminOnly bool) {
	group.GET("/dashboard/stats", kb.GetDashboardStats)
	group.POST("/bases", kb.CreateKnowledgeBase)
	group.GET("/bases", kb.ListKnowledgeBases)
	group.DELETE("/bases/:kb_id", kb.DeleteKnowledgeBase)
	group.POST("/documents/upload", kb.UploadDocument)
	group.GET("/documents", kb.ListDocuments)
	group.GET("/jobs", kb.ListJobs)
	group.GET("/jobs/:job_id", kb.GetJob)
	group.POST("/jobs/:job_id/retry", kb.RetryJob)
	group.POST("/jobs/:job_id/cancel", kb.CancelJob)
	group.DELETE("/documents/:document_id", kb.DeleteDocument)
	group.POST("/retrieve", kb.Retrieve)
	group.GET("/retrieve/audit/:request_id", kb.GetRetrieveAuditLog)
	group.GET(phase3.LegacyRetrievalDebugRoute, kb.GetRetrieveDebugView)
	group.GET(phase3.RetrievalDebugRoute, kb.GetRetrieveDebugView)
	group.GET("/retrieve/audit", kb.ListRetrieveAuditLogs)
	group.GET("/metrics/overview", kb.GetMetricsOverview)
	group.GET("/logs/ingest", kb.ListIngestLogs)
	group.GET("/logs/ingest/:job_id", kb.GetIngestLogDetail)
	group.POST("/ingest/pause", kb.PauseIngest)
	group.POST("/ingest/resume", kb.ResumeIngest)
	group.GET("/ingest/status", kb.GetIngestStatus)
	if adminOnly {
		group.GET("/eval/datasets", kb.ListEvalDatasets)
		group.POST("/eval/datasets", kb.CreateEvalDataset)
		group.GET("/eval/datasets/:dataset_id/items", kb.ListEvalCases)
		group.POST("/eval/datasets/:dataset_id/items", kb.CreateEvalCase)
		group.POST("/eval/datasets/:dataset_id/items/import", kb.ImportEvalCases)
		group.GET("/eval/datasets/:dataset_id/items/export", kb.ExportEvalCases)
		group.POST("/eval/datasets/:dataset_id/validate", kb.ValidateEvalDataset)
		group.GET("/eval/runs", kb.ListEvalRuns)
		group.POST("/eval/runs", kb.CreateEvalRun)
		group.GET("/eval/runs/:run_id", kb.GetEvalRun)
		group.GET("/eval/runs/:run_id/report", kb.GetEvalReport)
		group.GET("/eval/runs/:run_id/cases", kb.ListEvalFailureCases)
		group.GET("/eval/runs/:run_id/report/export", kb.ExportEvalReport)
		group.GET("/strategy/flags", kb.ListStrategyFlags)
		group.PATCH("/strategy/flags/:flag_key", kb.UpdateStrategyFlag)
		group.GET("/strategy/versions", kb.ListStrategyVersions)
		group.GET("/strategy/versions/:version_id", kb.GetStrategyVersion)
		group.POST("/strategy/rollback", kb.RollbackStrategy)
		group.GET("/strategy/impact", kb.GetStrategyImpact)
		group.GET("/strategy/gates", kb.GetStrategyGates)
		group.GET("/strategy/operations", kb.ListStrategyOperations)
		group.GET("/experiments", kb.ListExperiments)
		group.POST("/experiments", kb.SaveExperiment)
		group.POST("/experiments/:experiment_id/rollback", kb.RollbackExperiment)
		group.GET("/experiments/summary", kb.GetExperimentSummaries)
		group.GET("/index-lifecycle/registry", kb.ListIndexRegistry)
		group.POST("/index-lifecycle/register", kb.RegisterIndexFromConfig)
		group.POST("/index-lifecycle/build", kb.BuildCandidateIndex)
		group.GET("/index-lifecycle/health/:index_version", kb.GetIndexHealth)
		group.POST("/index-lifecycle/switch/:index_version", kb.SwitchActiveIndex)
		group.POST("/index-lifecycle/rollback", kb.RollbackActiveIndex)
		group.GET("/index-lifecycle/operations", kb.ListIndexOperations)
		group.GET("/cost/summary", kb.GetCostSummary)
		group.GET("/cost/timeseries", kb.GetCostTimeseries)
		group.GET("/cost/by-kb", kb.GetCostByKB)
		group.GET("/cost/by-strategy", kb.GetCostByStrategy)
		group.GET("/cost/by-model", kb.GetCostByModel)
		group.GET("/cost/high-cost-queries", kb.ListHighCostQueries)
		group.GET("/vector/collections", kb.ListVectorCollections)
		group.GET("/vector/collections/:name/health", kb.GetVectorCollectionHealth)
		group.GET("/vector/collections/:name/capacity", kb.GetVectorCollectionCapacity)
		group.POST("/vector/collections/:name/rebuild", kb.RebuildVectorCollection)
		group.POST("/vector/collections/:name/switch", kb.SwitchVectorCollection)
		group.POST("/vector/collections/:name/rollback", kb.RollbackVectorCollection)
		group.GET("/vector/operations", kb.ListVectorOperations)
		group.GET("/audit/events", kb.ListAuditEvents)
		group.GET("/audit/events/:event_id", kb.GetAuditEventDetail)
		group.POST("/audit/events/export", kb.ExportAuditEvents)
		group.GET("/alerts", kb.ListAlerts)
		group.PATCH("/alerts/:alert_id/ack", kb.AckAlert)
		group.PATCH("/alerts/:alert_id/resolve", kb.ResolveAlert)
		group.POST("/reports/weekly", kb.CreateWeeklyReport)
		group.GET("/reports/weekly", kb.ListWeeklyReports)
		group.GET("/reports/weekly/:report_id", kb.GetWeeklyReportDetail)
		group.POST("/reports/weekly/:report_id/export", kb.ExportWeeklyReport)
		group.GET("/weekly-report", kb.GetWeeklyReport)
		group.GET("/governance/gates", kb.GetGovernanceGate)
		group.POST("/governance/gates/check", kb.CheckGovernanceGates)
		group.GET("/governance/gate", kb.GetGovernanceGate)
		group.GET("/phase4/acceptance", kb.GetPhase4Acceptance)
		group.GET("/semantic-cache/gate", kb.GetSemanticCacheGate)
		group.GET("/semantic-cache/acceptance", kb.GetSemanticCacheAcceptance)
		group.GET("/semantic-cache/report", kb.GetSemanticCacheReport)
		group.GET("/release/status", kb.GetReleaseStatus)
		group.GET("/release/summary", kb.GetReleaseSummary)
		group.POST("/release/rollback", kb.RollbackRelease)
		group.POST("/release/activate", kb.ActivateRelease)
	}
}
