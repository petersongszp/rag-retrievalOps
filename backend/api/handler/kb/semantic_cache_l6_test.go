package kb

import (
	"context"
	"net/http"
	"testing"
	"time"

	"interview-agents/internal/config"
	"interview-agents/internal/model"
	"interview-agents/internal/rag/release"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	ut "github.com/cloudwego/hertz/pkg/common/ut"
)

func TestBuildSemanticCacheReport(t *testing.T) {
	originalConfig := config.Global
	originalListLogs := listSemanticCacheRetrieveLogs
	originalListCosts := listSemanticCacheCostTraces
	t.Cleanup(func() {
		config.Global = originalConfig
		listSemanticCacheRetrieveLogs = originalListLogs
		listSemanticCacheCostTraces = originalListCosts
	})

	config.Global.RAG.FeatureFlags.EnableSemanticCache = true
	config.Global.RAG.Release = config.RAGReleaseConfig{
		Enabled:       true,
		Stage:         release.StageInternal,
		CanaryPercent: 5,
		BatchPercent:  20,
	}

	now := time.Now().UTC()
	listSemanticCacheRetrieveLogs = func(startTime, endTime time.Time, kbID *uint64) ([]*model.KBRetrieveLog, error) {
		return []*model.KBRetrieveLog{
			{
				RequestID:               "req-hit",
				SemanticCacheEnabled:    true,
				SemanticCacheHit:        true,
				SemanticCacheLookupMs:   9,
				SemanticCacheReason:     "hit",
				SemanticCacheEntryID:    "entry-1",
				SemanticCacheSimilarity: 0.99,
				Routes:                  "semantic_cache",
				FinalCount:              2,
				ResultStatus:            model.RetrieveResultStatusSuccess,
				CreatedAt:               now,
			},
			{
				RequestID:             "req-miss",
				SemanticCacheEnabled:  true,
				SemanticCacheHit:      false,
				SemanticCacheLookupMs: 13,
				SemanticCacheReason:   "miss",
				ResultStatus:          model.RetrieveResultStatusSuccess,
				CreatedAt:             now,
			},
		}, nil
	}
	listSemanticCacheCostTraces = func(startTime, endTime time.Time, kbID *uint64) ([]*model.KBCostTrace, error) {
		return []*model.KBCostTrace{
			{
				RequestID:               "req-hit",
				CacheSavedRetrievalCost: 0.21,
				CacheSavedRerankCost:    0.07,
				CreatedAt:               now,
			},
		}, nil
	}

	report := buildSemanticCacheReport(now)
	if report.Phase != "L6" {
		t.Fatalf("Phase = %q, want L6", report.Phase)
	}
	if !report.Accepted {
		t.Fatalf("Accepted = false, want true, risks=%v", report.Risks)
	}
	if report.BenefitSummary.TotalSavedCost != 0.28 {
		t.Fatalf("TotalSavedCost = %v, want 0.28", report.BenefitSummary.TotalSavedCost)
	}
	if len(report.ImplementationSummary) != 7 {
		t.Fatalf("ImplementationSummary len = %d, want 7", len(report.ImplementationSummary))
	}
	if len(report.Artifacts.AdminEndpoints) != 3 {
		t.Fatalf("expected 3 admin endpoints, got %d", len(report.Artifacts.AdminEndpoints))
	}
}

func TestSemanticCacheReportEndpoint(t *testing.T) {
	originalConfig := config.Global
	originalListLogs := listSemanticCacheRetrieveLogs
	originalListCosts := listSemanticCacheCostTraces
	t.Cleanup(func() {
		config.Global = originalConfig
		listSemanticCacheRetrieveLogs = originalListLogs
		listSemanticCacheCostTraces = originalListCosts
	})

	config.Global.RAG.FeatureFlags.EnableSemanticCache = true
	config.Global.RAG.Release = config.RAGReleaseConfig{
		Enabled:       true,
		Stage:         release.StageBatch,
		CanaryPercent: 10,
		BatchPercent:  30,
	}

	now := time.Now().UTC()
	listSemanticCacheRetrieveLogs = func(startTime, endTime time.Time, kbID *uint64) ([]*model.KBRetrieveLog, error) {
		return []*model.KBRetrieveLog{
			{
				RequestID:             "req-report",
				SemanticCacheEnabled:  true,
				SemanticCacheHit:      false,
				SemanticCacheLookupMs: 15,
				SemanticCacheReason:   "miss",
				ResultStatus:          model.RetrieveResultStatusSuccess,
				CreatedAt:             now,
			},
		}, nil
	}
	listSemanticCacheCostTraces = func(startTime, endTime time.Time, kbID *uint64) ([]*model.KBCostTrace, error) {
		return nil, nil
	}

	h := server.Default()
	h.Use(func(ctx context.Context, c *app.RequestContext) {
		c.Set("user_id", uint(7))
		c.Set("role", "admin")
		c.Next(ctx)
	})
	h.GET("/api/admin/kb/semantic-cache/report", GetSemanticCacheReport)

	resp := ut.PerformRequest(h.Engine, http.MethodGet, "/api/admin/kb/semantic-cache/report", nil).Result()
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("expected report 200, got %d body=%s", resp.StatusCode(), string(resp.Body()))
	}

	var payload struct {
		Code int                         `json:"code"`
		Data semanticCacheReportResponse `json:"data"`
	}
	decodeJSONResponse(t, resp.Body(), &payload)
	if payload.Data.Phase != "L6" {
		t.Fatalf("Phase = %q, want L6", payload.Data.Phase)
	}
	if payload.Data.Artifacts.MeetingBrief == "" {
		t.Fatalf("MeetingBrief should not be empty: %+v", payload.Data.Artifacts)
	}
	if len(payload.Data.TestSummary.FocusedCoverage) == 0 {
		t.Fatalf("FocusedCoverage should not be empty: %+v", payload.Data.TestSummary)
	}
}
