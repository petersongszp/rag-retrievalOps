package kb

import (
	"context"
	"net/http"
	"testing"
	"time"

	"interview-agents/internal/config"
	"interview-agents/internal/rag/phase3"
	"interview-agents/internal/rag/phase3admin"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	ut "github.com/cloudwego/hertz/pkg/common/ut"
)

func TestStrategyHandlersLifecycle(t *testing.T) {
	resetStrategyHandlerState(t)

	h := newAdminStrategyTestServer()

	flagsResp := ut.PerformRequest(h.Engine, http.MethodGet, "/api/admin/kb/strategy/flags", nil).Result()
	if flagsResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected list flags 200, got %d", flagsResp.StatusCode())
	}
	var flagsPayload struct {
		Code int `json:"code"`
		Data struct {
			Items []phase3admin.FlagState `json:"items"`
		} `json:"data"`
	}
	decodeJSONResponse(t, flagsResp.Body(), &flagsPayload)
	if len(flagsPayload.Data.Items) != len(phase3.ManagedFeatureFlags()) {
		t.Fatalf("flags len = %d, want %d", len(flagsPayload.Data.Items), len(phase3.ManagedFeatureFlags()))
	}

	updateShadowResp := performJSONRequest(t, h, http.MethodPatch, "/api/admin/kb/strategy/flags/"+phase3.FlagModelAssistedRewrite, map[string]interface{}{
		"enabled":            true,
		"status":             phase3.StatusShadow,
		"rollout_percentage": 10,
		"reason":             "shadow rollout",
	})
	if updateShadowResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected shadow update 200, got %d body=%s", updateShadowResp.StatusCode(), string(updateShadowResp.Body()))
	}

	versionsResp := ut.PerformRequest(h.Engine, http.MethodGet, "/api/admin/kb/strategy/versions?flag_key="+phase3.FlagModelAssistedRewrite, nil).Result()
	if versionsResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected list versions 200, got %d", versionsResp.StatusCode())
	}
	var versionsPayload struct {
		Code int `json:"code"`
		Data struct {
			Items []phase3admin.VersionRecord `json:"items"`
			Total int                         `json:"total"`
		} `json:"data"`
	}
	decodeJSONResponse(t, versionsResp.Body(), &versionsPayload)
	if versionsPayload.Data.Total < 2 {
		t.Fatalf("versions total = %d, want at least 2", versionsPayload.Data.Total)
	}
	targetVersionID := versionsPayload.Data.Items[0].VersionID

	versionDetailResp := ut.PerformRequest(h.Engine, http.MethodGet, "/api/admin/kb/strategy/versions/"+targetVersionID, nil).Result()
	if versionDetailResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected version detail 200, got %d", versionDetailResp.StatusCode())
	}
	var versionDetailPayload struct {
		Code int                       `json:"code"`
		Data phase3admin.VersionRecord `json:"data"`
	}
	decodeJSONResponse(t, versionDetailResp.Body(), &versionDetailPayload)
	if versionDetailPayload.Data.VersionID != targetVersionID {
		t.Fatalf("version detail id = %q, want %q", versionDetailPayload.Data.VersionID, targetVersionID)
	}

	updateEnabledResp := performJSONRequest(t, h, http.MethodPatch, "/api/admin/kb/strategy/flags/"+phase3.FlagModelAssistedRewrite, map[string]interface{}{
		"enabled":            true,
		"status":             phase3.StatusEnabled,
		"rollout_percentage": 100,
		"reason":             "promote to enabled",
	})
	if updateEnabledResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected enabled update 200, got %d body=%s", updateEnabledResp.StatusCode(), string(updateEnabledResp.Body()))
	}

	rollbackResp := performJSONRequest(t, h, http.MethodPost, "/api/admin/kb/strategy/rollback", map[string]interface{}{
		"target_version": targetVersionID,
		"reason":         "rollback to shadow",
	})
	if rollbackResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected rollback 200, got %d body=%s", rollbackResp.StatusCode(), string(rollbackResp.Body()))
	}
	var rollbackPayload struct {
		Code int `json:"code"`
		Data struct {
			Status       string                  `json:"status"`
			ChangedFlags []phase3admin.FlagState `json:"changed_flags"`
		} `json:"data"`
	}
	decodeJSONResponse(t, rollbackResp.Body(), &rollbackPayload)
	if rollbackPayload.Data.Status != "succeeded" {
		t.Fatalf("rollback status = %q, want succeeded", rollbackPayload.Data.Status)
	}
	if len(rollbackPayload.Data.ChangedFlags) != 1 {
		t.Fatalf("changed flags len = %d, want 1", len(rollbackPayload.Data.ChangedFlags))
	}
	if rollbackPayload.Data.ChangedFlags[0].Status != phase3.StatusShadow || rollbackPayload.Data.ChangedFlags[0].RolloutPercentage != 10 {
		t.Fatalf("changed flag = %#v, want shadow/10", rollbackPayload.Data.ChangedFlags[0])
	}
}

func TestListStrategyVersionsRejectsInvalidFlagKey(t *testing.T) {
	resetStrategyHandlerState(t)

	h := newAdminStrategyTestServer()
	resp := ut.PerformRequest(h.Engine, http.MethodGet, "/api/admin/kb/strategy/versions?flag_key=NOT_A_PHASE3_FLAG", nil).Result()
	if resp.StatusCode() != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", resp.StatusCode(), string(resp.Body()))
	}
}

func newAdminStrategyTestServer() *server.Hertz {
	h := server.Default()
	h.Use(func(ctx context.Context, c *app.RequestContext) {
		c.Set("user_id", uint(7))
		c.Set("role", "admin")
		c.Next(ctx)
	})
	h.GET("/api/admin/kb/strategy/flags", ListStrategyFlags)
	h.PATCH("/api/admin/kb/strategy/flags/:flag_key", UpdateStrategyFlag)
	h.GET("/api/admin/kb/strategy/versions", ListStrategyVersions)
	h.GET("/api/admin/kb/strategy/versions/:version_id", GetStrategyVersion)
	h.POST("/api/admin/kb/strategy/rollback", RollbackStrategy)
	h.GET("/api/admin/kb/strategy/impact", GetStrategyImpact)
	h.GET("/api/admin/kb/strategy/gates", GetStrategyGates)
	h.GET("/api/admin/kb/strategy/operations", ListStrategyOperations)
	return h
}

func resetStrategyHandlerState(t *testing.T) {
	t.Helper()

	originalConfig := config.Global
	config.Global = config.Config{
		ConfigVersion: "handler-strategy-test-" + time.Now().UTC().Format("150405.000000000"),
		RAG: config.RAGConfig{
			FeatureFlags: config.RAGFeatureFlags{
				EnableParentChildRetrieval: true,
				EnableStrategicTopK:        true,
				EnableModelAssistedRewrite: false,
			},
		},
	}
	t.Cleanup(func() {
		config.Global = originalConfig
	})
}
