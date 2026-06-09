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

func TestComputeSemanticCacheGate(t *testing.T) {
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
		Stage:         release.StageSmall,
		CanaryPercent: 10,
		BatchPercent:  25,
	}

	now := time.Now().UTC()
	listSemanticCacheRetrieveLogs = func(startTime, endTime time.Time, kbID *uint64) ([]*model.KBRetrieveLog, error) {
		return []*model.KBRetrieveLog{
			{
				RequestID:               "req-hit",
				SemanticCacheEnabled:    true,
				SemanticCacheHit:        true,
				SemanticCacheLookupMs:   12,
				SemanticCacheReason:     "hit",
				SemanticCacheEntryID:    "entry-1",
				SemanticCacheSimilarity: 0.98,
				Routes:                  "semantic_cache",
				FinalCount:              2,
				ResultStatus:            model.RetrieveResultStatusSuccess,
				CreatedAt:               now,
			},
			{
				RequestID:             "req-miss",
				SemanticCacheEnabled:  true,
				SemanticCacheHit:      false,
				SemanticCacheLookupMs: 18,
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
				CacheSavedRetrievalCost: 0.12,
				CacheSavedRerankCost:    0.08,
				CreatedAt:               now,
			},
		}, nil
	}

	gate := computeSemanticCacheGate(now)
	if !gate.Passed {
		t.Fatalf("gate.Passed = false, want true, risks=%v", gate.Risks)
	}
	if gate.HitCount != 1 || gate.LookupCount != 2 {
		t.Fatalf("unexpected hit/lookup count: %+v", gate)
	}
	if gate.FalseHitCount != 0 {
		t.Fatalf("FalseHitCount = %d, want 0", gate.FalseHitCount)
	}
	if gate.LookupP95Ms != 12 {
		t.Fatalf("LookupP95Ms = %d, want 12", gate.LookupP95Ms)
	}
	if gate.SavedRetrievalCost != 0.12 || gate.SavedRerankCost != 0.08 {
		t.Fatalf("unexpected saved cost: %+v", gate)
	}
}

func TestSemanticCacheAcceptanceEndpoint(t *testing.T) {
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
				RequestID:             "req-acceptance",
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
	h.GET("/api/admin/kb/semantic-cache/gate", GetSemanticCacheGate)
	h.GET("/api/admin/kb/semantic-cache/acceptance", GetSemanticCacheAcceptance)

	gateResp := ut.PerformRequest(h.Engine, http.MethodGet, "/api/admin/kb/semantic-cache/gate", nil).Result()
	if gateResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected gate 200, got %d body=%s", gateResp.StatusCode(), string(gateResp.Body()))
	}
	var gatePayload struct {
		Code int                       `json:"code"`
		Data semanticCacheGateResponse `json:"data"`
	}
	decodeJSONResponse(t, gateResp.Body(), &gatePayload)
	if !gatePayload.Data.ObservabilityGuardPassed {
		t.Fatalf("unexpected gate payload: %+v", gatePayload.Data)
	}

	acceptanceResp := ut.PerformRequest(h.Engine, http.MethodGet, "/api/admin/kb/semantic-cache/acceptance", nil).Result()
	if acceptanceResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected acceptance 200, got %d body=%s", acceptanceResp.StatusCode(), string(acceptanceResp.Body()))
	}
	var acceptancePayload struct {
		Code int                             `json:"code"`
		Data semanticCacheAcceptanceResponse `json:"data"`
	}
	decodeJSONResponse(t, acceptanceResp.Body(), &acceptancePayload)
	if acceptancePayload.Data.Phase != "L5" {
		t.Fatalf("Phase = %q, want L5", acceptancePayload.Data.Phase)
	}
	if len(acceptancePayload.Data.CanaryPlan) == 0 || len(acceptancePayload.Data.RollbackPlan) == 0 {
		t.Fatalf("acceptance plans should not be empty: %+v", acceptancePayload.Data)
	}
}
