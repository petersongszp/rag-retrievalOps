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
	if gate.WarmLookupP95Ms != 0 {
		t.Fatalf("WarmLookupP95Ms = %d, want 0", gate.WarmLookupP95Ms)
	}
	if gate.EmbeddingCacheObservedCount != 0 {
		t.Fatalf("EmbeddingCacheObservedCount = %d, want 0", gate.EmbeddingCacheObservedCount)
	}
	if gate.SavedRetrievalCost != 0.12 || gate.SavedRerankCost != 0.08 {
		t.Fatalf("unexpected saved cost: %+v", gate)
	}
}

func TestComputeSemanticCacheGatePassesWithEmbeddingCacheWarmLatency(t *testing.T) {
	originalConfig := config.Global
	originalListLogs := listSemanticCacheRetrieveLogs
	originalListCosts := listSemanticCacheCostTraces
	t.Cleanup(func() {
		config.Global = originalConfig
		listSemanticCacheRetrieveLogs = originalListLogs
		listSemanticCacheCostTraces = originalListCosts
	})

	config.Global.RAG.FeatureFlags.EnableSemanticCache = true

	now := time.Now().UTC()
	listSemanticCacheRetrieveLogs = func(startTime, endTime time.Time, kbID *uint64) ([]*model.KBRetrieveLog, error) {
		return []*model.KBRetrieveLog{
			{
				RequestID:             "req-cold-1",
				SemanticCacheEnabled:  true,
				SemanticCacheHit:      true,
				SemanticCacheLookupMs: 406,
				SemanticCacheReason:   "hit",
				SemanticCacheEntryID:  "entry-cold-1",
				EmbeddingCacheEnabled: true,
				EmbeddingCacheHit:     false,
				EmbeddingCacheReason:  "miss",
				Routes:                "semantic_cache",
				FinalCount:            1,
				ResultStatus:          model.RetrieveResultStatusSuccess,
				CreatedAt:             now,
			},
			{
				RequestID:             "req-cold-2",
				SemanticCacheEnabled:  true,
				SemanticCacheHit:      false,
				SemanticCacheLookupMs: 390,
				SemanticCacheReason:   "miss",
				EmbeddingCacheEnabled: true,
				EmbeddingCacheHit:     false,
				EmbeddingCacheReason:  "miss",
				ResultStatus:          model.RetrieveResultStatusSuccess,
				CreatedAt:             now,
			},
			{
				RequestID:             "req-warm-1",
				SemanticCacheEnabled:  true,
				SemanticCacheHit:      true,
				SemanticCacheLookupMs: 14,
				SemanticCacheReason:   "hit",
				SemanticCacheEntryID:  "entry-warm-1",
				EmbeddingCacheEnabled: true,
				EmbeddingCacheHit:     true,
				EmbeddingCacheReason:  "hit",
				Routes:                "semantic_cache",
				FinalCount:            1,
				ResultStatus:          model.RetrieveResultStatusSuccess,
				CreatedAt:             now,
			},
			{
				RequestID:             "req-warm-2",
				SemanticCacheEnabled:  true,
				SemanticCacheHit:      false,
				SemanticCacheLookupMs: 16,
				SemanticCacheReason:   "miss",
				EmbeddingCacheEnabled: true,
				EmbeddingCacheHit:     true,
				EmbeddingCacheReason:  "hit",
				ResultStatus:          model.RetrieveResultStatusSuccess,
				CreatedAt:             now,
			},
		}, nil
	}
	listSemanticCacheCostTraces = func(startTime, endTime time.Time, kbID *uint64) ([]*model.KBCostTrace, error) {
		return nil, nil
	}

	gate := computeSemanticCacheGate(now)
	if !gate.Passed {
		t.Fatalf("gate.Passed = false, want true, risks=%v", gate.Risks)
	}
	if gate.LookupP95Ms <= semanticCacheLatencyGuardThresholdMs {
		t.Fatalf("LookupP95Ms = %d, want > %d for cold path", gate.LookupP95Ms, semanticCacheLatencyGuardThresholdMs)
	}
	if gate.WarmLookupP95Ms != 14 {
		t.Fatalf("WarmLookupP95Ms = %d, want 14", gate.WarmLookupP95Ms)
	}
	if gate.LatencyGuardBasis != "warm_lookup_with_embedding_cache_p95" {
		t.Fatalf("LatencyGuardBasis = %q, want warm_lookup_with_embedding_cache_p95", gate.LatencyGuardBasis)
	}
	if gate.EmbeddingCacheHitCount != 2 {
		t.Fatalf("EmbeddingCacheHitCount = %d, want 2", gate.EmbeddingCacheHitCount)
	}
	if gate.EmbeddingCacheObservedCount != 4 {
		t.Fatalf("EmbeddingCacheObservedCount = %d, want 4", gate.EmbeddingCacheObservedCount)
	}
}

func TestComputeSemanticCacheGateFailsWhenWarmLatencyExceedsThreshold(t *testing.T) {
	originalConfig := config.Global
	originalListLogs := listSemanticCacheRetrieveLogs
	originalListCosts := listSemanticCacheCostTraces
	t.Cleanup(func() {
		config.Global = originalConfig
		listSemanticCacheRetrieveLogs = originalListLogs
		listSemanticCacheCostTraces = originalListCosts
	})

	config.Global.RAG.FeatureFlags.EnableSemanticCache = true

	now := time.Now().UTC()
	listSemanticCacheRetrieveLogs = func(startTime, endTime time.Time, kbID *uint64) ([]*model.KBRetrieveLog, error) {
		return []*model.KBRetrieveLog{
			{
				RequestID:             "req-cold",
				SemanticCacheEnabled:  true,
				SemanticCacheHit:      false,
				SemanticCacheLookupMs: 120,
				SemanticCacheReason:   "miss",
				EmbeddingCacheEnabled: true,
				EmbeddingCacheHit:     false,
				EmbeddingCacheReason:  "miss",
				ResultStatus:          model.RetrieveResultStatusSuccess,
				CreatedAt:             now,
			},
			{
				RequestID:             "req-warm",
				SemanticCacheEnabled:  true,
				SemanticCacheHit:      true,
				SemanticCacheLookupMs: 260,
				SemanticCacheReason:   "hit",
				SemanticCacheEntryID:  "entry-warm",
				EmbeddingCacheEnabled: true,
				EmbeddingCacheHit:     true,
				EmbeddingCacheReason:  "hit",
				Routes:                "semantic_cache",
				FinalCount:            1,
				ResultStatus:          model.RetrieveResultStatusSuccess,
				CreatedAt:             now,
			},
		}, nil
	}
	listSemanticCacheCostTraces = func(startTime, endTime time.Time, kbID *uint64) ([]*model.KBCostTrace, error) {
		return nil, nil
	}

	gate := computeSemanticCacheGate(now)
	if gate.Passed {
		t.Fatalf("gate.Passed = true, want false when warm latency exceeds threshold: %+v", gate)
	}
	if gate.LatencyGuardBasis != "warm_lookup_with_embedding_cache_p95" {
		t.Fatalf("LatencyGuardBasis = %q, want warm_lookup_with_embedding_cache_p95", gate.LatencyGuardBasis)
	}
	if gate.WarmLookupP95Ms <= semanticCacheLatencyGuardThresholdMs {
		t.Fatalf("WarmLookupP95Ms = %d, want > %d", gate.WarmLookupP95Ms, semanticCacheLatencyGuardThresholdMs)
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
