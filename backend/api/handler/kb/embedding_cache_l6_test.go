package kb

import (
	"context"
	"net/http"
	"testing"
	"time"

	"interview-agents/internal/config"
	"interview-agents/internal/model"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	ut "github.com/cloudwego/hertz/pkg/common/ut"
)

func TestBuildEmbeddingCacheReport(t *testing.T) {
	originalConfig := config.Global
	originalListLogs := listEmbeddingCacheRetrieveLogs
	t.Cleanup(func() {
		config.Global = originalConfig
		listEmbeddingCacheRetrieveLogs = originalListLogs
	})

	config.Global.Embedding.EnableCache = true

	now := time.Now().UTC()
	listEmbeddingCacheRetrieveLogs = func(startTime, endTime time.Time, kbID *uint64) ([]*model.KBRetrieveLog, error) {
		return []*model.KBRetrieveLog{
			{
				RequestID:              "req-hit",
				EmbeddingCacheEnabled:  true,
				EmbeddingCacheHit:      true,
				EmbeddingCacheLookupMs: 2,
				EmbeddingCacheReason:   "hit",
				ResultStatus:           model.RetrieveResultStatusSuccess,
				CreatedAt:              now,
			},
			{
				RequestID:              "req-miss",
				EmbeddingCacheEnabled:  true,
				EmbeddingCacheHit:      false,
				EmbeddingCacheLookupMs: 5,
				EmbeddingCacheReason:   "miss",
				ResultStatus:           model.RetrieveResultStatusSuccess,
				CreatedAt:              now,
			},
		}, nil
	}

	report := buildEmbeddingCacheReport(now)
	if report.Phase != "L6-L9" {
		t.Fatalf("Phase = %q, want L6-L9", report.Phase)
	}
	if !report.Accepted {
		t.Fatalf("Accepted = false, want true, risks=%v", report.Risks)
	}
	if report.BenefitSummary.HitRate != 0.5 {
		t.Fatalf("HitRate = %v, want 0.5", report.BenefitSummary.HitRate)
	}
	if len(report.ImplementationSummary) != 7 {
		t.Fatalf("ImplementationSummary len = %d, want 7", len(report.ImplementationSummary))
	}
	if len(report.Artifacts.AdminEndpoints) != 3 {
		t.Fatalf("expected 3 admin endpoints, got %d", len(report.Artifacts.AdminEndpoints))
	}
}

func TestEmbeddingCacheReportEndpoint(t *testing.T) {
	originalConfig := config.Global
	originalListLogs := listEmbeddingCacheRetrieveLogs
	t.Cleanup(func() {
		config.Global = originalConfig
		listEmbeddingCacheRetrieveLogs = originalListLogs
	})

	config.Global.Embedding.EnableCache = true

	now := time.Now().UTC()
	listEmbeddingCacheRetrieveLogs = func(startTime, endTime time.Time, kbID *uint64) ([]*model.KBRetrieveLog, error) {
		return []*model.KBRetrieveLog{
			{
				RequestID:              "req-report",
				EmbeddingCacheEnabled:  true,
				EmbeddingCacheHit:      true,
				EmbeddingCacheLookupMs: 4,
				EmbeddingCacheReason:   "singleflight_shared",
				ResultStatus:           model.RetrieveResultStatusSuccess,
				CreatedAt:              now,
			},
		}, nil
	}

	h := server.Default()
	h.Use(func(ctx context.Context, c *app.RequestContext) {
		c.Set("user_id", uint(7))
		c.Set("role", "admin")
		c.Next(ctx)
	})
	h.GET("/api/admin/kb/embedding-cache/report", GetEmbeddingCacheReport)

	resp := ut.PerformRequest(h.Engine, http.MethodGet, "/api/admin/kb/embedding-cache/report", nil).Result()
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("expected report 200, got %d body=%s", resp.StatusCode(), string(resp.Body()))
	}

	var payload struct {
		Code int                          `json:"code"`
		Data embeddingCacheReportResponse `json:"data"`
	}
	decodeJSONResponse(t, resp.Body(), &payload)
	if payload.Data.Phase != "L6-L9" {
		t.Fatalf("Phase = %q, want L6-L9", payload.Data.Phase)
	}
	if payload.Data.Artifacts.ImplementationGuide == "" {
		t.Fatalf("ImplementationGuide should not be empty: %+v", payload.Data.Artifacts)
	}
	if len(payload.Data.TestSummary.FocusedCoverage) == 0 {
		t.Fatalf("FocusedCoverage should not be empty: %+v", payload.Data.TestSummary)
	}
}
