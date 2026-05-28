package kb

import (
	"context"
	"net/http"
	"testing"
	"time"

	"interview-agents/internal/config"
	"interview-agents/internal/rag/experiment"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	ut "github.com/cloudwego/hertz/pkg/common/ut"
)

func TestExperimentHandlersLifecycle(t *testing.T) {
	resetExperimentHandlerState(t)

	h := server.Default()
	h.Use(func(ctx context.Context, c *app.RequestContext) {
		c.Set("user_id", uint(9))
		c.Set("role", "admin")
		c.Next(ctx)
	})
	h.GET("/api/admin/kb/experiments", ListExperiments)
	h.POST("/api/admin/kb/experiments", SaveExperiment)
	h.POST("/api/admin/kb/experiments/:experiment_id/rollback", RollbackExperiment)

	saveResp := performJSONRequest(t, h, http.MethodPost, "/api/admin/kb/experiments", map[string]interface{}{
		"experiment_name":    "rewrite shadow",
		"strategy_type":      experiment.StrategyTypeRewrite,
		"baseline_version":   "rewrite_on",
		"candidate_version":  "rewrite_off",
		"traffic_ratio":      20,
		"target_environment": experiment.EnvAll,
		"shadow_mode":        true,
		"owner":              "admin",
		"status":             experiment.StatusRunning,
		"start_time":         time.Now().UTC().Add(-time.Hour).Format(time.RFC3339),
		"end_time":           time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
	})
	if saveResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected save 200, got %d body=%s", saveResp.StatusCode(), string(saveResp.Body()))
	}

	listResp := ut.PerformRequest(h.Engine, http.MethodGet, "/api/admin/kb/experiments", nil).Result()
	if listResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected list 200, got %d", listResp.StatusCode())
	}
	var listPayload struct {
		Code int `json:"code"`
		Data struct {
			Items []experiment.ConfigRecord `json:"items"`
		} `json:"data"`
	}
	decodeJSONResponse(t, listResp.Body(), &listPayload)
	if len(listPayload.Data.Items) != 1 {
		t.Fatalf("experiments len = %d, want 1", len(listPayload.Data.Items))
	}

	rollbackResp := performJSONRequest(t, h, http.MethodPost, "/api/admin/kb/experiments/"+listPayload.Data.Items[0].ExperimentID+"/rollback", map[string]interface{}{
		"reason": "manual rollback",
	})
	if rollbackResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected rollback 200, got %d body=%s", rollbackResp.StatusCode(), string(rollbackResp.Body()))
	}
}

func TestExperimentDecideTopKCandidate(t *testing.T) {
	resetExperimentHandlerState(t)

	_, err := experiment.Save(experiment.ConfigRecord{
		ExperimentName:    "topk candidate",
		StrategyType:      experiment.StrategyTypeCandidateTopK,
		BaselineVersion:   "topk:5",
		CandidateVersion:  "topk:11",
		TrafficRatio:      100,
		TargetEnvironment: experiment.EnvAll,
		Status:            experiment.StatusRunning,
		StartTime:         time.Now().UTC().Add(-time.Hour),
		EndTime:           time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("save experiment failed: %v", err)
	}

	decision := experiment.Decide(&config.Global, 12, "user", []uint64{1}, "how does topk work", "req-topk", 5)
	if !decision.Matched || decision.Group != experiment.GroupCandidate {
		t.Fatalf("unexpected decision: %+v", decision)
	}
	if decision.Override.CandidateTopK != 11 {
		t.Fatalf("CandidateTopK = %d, want 11", decision.Override.CandidateTopK)
	}
}

func resetExperimentHandlerState(t *testing.T) {
	t.Helper()

	originalConfig := config.Global
	config.Global = config.Config{
		ConfigVersion: "handler-experiment-test-" + time.Now().UTC().Format("150405.000000000"),
		RAG: config.RAGConfig{
			FeatureFlags: config.RAGFeatureFlags{
				EnableExperimentPlatform: true,
			},
		},
	}
	t.Cleanup(func() {
		config.Global = originalConfig
	})
}
