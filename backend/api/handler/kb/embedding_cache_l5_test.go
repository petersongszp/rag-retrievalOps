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

func TestComputeEmbeddingCacheGate(t *testing.T) {
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
				EmbeddingCacheLookupMs: 6,
				EmbeddingCacheReason:   "miss",
				ResultStatus:           model.RetrieveResultStatusSuccess,
				CreatedAt:              now,
			},
		}, nil
	}

	gate := computeEmbeddingCacheGate(now)
	if !gate.Passed {
		t.Fatalf("gate.Passed = false, want true, risks=%v", gate.Risks)
	}
	if gate.HitCount != 1 || gate.LookupCount != 2 {
		t.Fatalf("unexpected hit/lookup count: %+v", gate)
	}
	if gate.LookupP95Ms != 2 {
		t.Fatalf("LookupP95Ms = %d, want 2", gate.LookupP95Ms)
	}
}

func TestEmbeddingCacheAcceptanceEndpoint(t *testing.T) {
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
				RequestID:              "req-acceptance",
				EmbeddingCacheEnabled:  true,
				EmbeddingCacheHit:      true,
				EmbeddingCacheLookupMs: 3,
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
	h.GET("/api/admin/kb/embedding-cache/gate", GetEmbeddingCacheGate)
	h.GET("/api/admin/kb/embedding-cache/acceptance", GetEmbeddingCacheAcceptance)

	gateResp := ut.PerformRequest(h.Engine, http.MethodGet, "/api/admin/kb/embedding-cache/gate", nil).Result()
	if gateResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected gate 200, got %d body=%s", gateResp.StatusCode(), string(gateResp.Body()))
	}
	var gatePayload struct {
		Code int                        `json:"code"`
		Data embeddingCacheGateResponse `json:"data"`
	}
	decodeJSONResponse(t, gateResp.Body(), &gatePayload)
	if !gatePayload.Data.ObservabilityGuardPassed {
		t.Fatalf("unexpected gate payload: %+v", gatePayload.Data)
	}

	acceptanceResp := ut.PerformRequest(h.Engine, http.MethodGet, "/api/admin/kb/embedding-cache/acceptance", nil).Result()
	if acceptanceResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected acceptance 200, got %d body=%s", acceptanceResp.StatusCode(), string(acceptanceResp.Body()))
	}
	var acceptancePayload struct {
		Code int                              `json:"code"`
		Data embeddingCacheAcceptanceResponse `json:"data"`
	}
	decodeJSONResponse(t, acceptanceResp.Body(), &acceptancePayload)
	if acceptancePayload.Data.Phase != "L5" {
		t.Fatalf("Phase = %q, want L5", acceptancePayload.Data.Phase)
	}
	if len(acceptancePayload.Data.CanaryPlan) == 0 || len(acceptancePayload.Data.RollbackPlan) == 0 {
		t.Fatalf("acceptance plans should not be empty: %+v", acceptancePayload.Data)
	}
}
