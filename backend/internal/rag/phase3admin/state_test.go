package phase3admin

import (
	"context"
	"errors"
	"testing"

	"interview-agents/internal/config"
	"interview-agents/internal/rag/phase3"
)

func resetStateForTest() {
	initialized = false
	flagStates = map[string]FlagState{}
	versionRecords = map[string][]VersionRecord{}
	versionByID = map[string]VersionRecord{}
	operationRecords = nil
	versionCounter = 0
	operationCounter = 0
	rollbackCounter = 0
	lastConfigVersion = ""
	reconfigureManager = func(ctx context.Context, cfg *config.Config) error { return nil }
}

func baseConfig() *config.Config {
	return &config.Config{
		ConfigVersion: "cfg-test-v1",
		RAG: config.RAGConfig{
			FeatureFlags: config.RAGFeatureFlags{
				EnableParentChildRetrieval: true,
				EnableStrategicTopK:        true,
				EnableModelAssistedRewrite: false,
			},
		},
	}
}

func TestListFlagsBootstrapsManagedFlags(t *testing.T) {
	resetStateForTest()
	cfg := baseConfig()

	items := ListFlags(cfg)
	if len(items) != len(phase3.ManagedFeatureFlags()) {
		t.Fatalf("ListFlags len = %d, want %d", len(items), len(phase3.ManagedFeatureFlags()))
	}
	if items[0].FlagKey != phase3.FlagParentChildRetrieval {
		t.Fatalf("first flag = %q", items[0].FlagKey)
	}
}

func TestUpdateFlagRejectsDirectEnableOfHighRiskFlag(t *testing.T) {
	resetStateForTest()
	cfg := baseConfig()

	_, err := UpdateFlag(context.Background(), cfg, phase3.FlagModelAssistedRewrite, true, phase3.StatusEnabled, 100, "enable directly", 1)
	if err == nil {
		t.Fatal("expected high-risk direct enable to fail")
	}
}

func TestUpdateFlagRejectsInvalidRolloutPercentage(t *testing.T) {
	resetStateForTest()
	cfg := baseConfig()

	_, err := UpdateFlag(context.Background(), cfg, phase3.FlagModelAssistedRewrite, true, phase3.StatusShadow, 101, "bad rollout", 1)
	if err == nil {
		t.Fatal("expected invalid rollout to fail")
	}
}

func TestUpdateFlagAppliesConfigMutation(t *testing.T) {
	resetStateForTest()
	cfg := baseConfig()

	state, err := UpdateFlag(context.Background(), cfg, phase3.FlagModelAssistedRewrite, true, phase3.StatusShadow, 25, "shadow rollout", 7)
	if err != nil {
		t.Fatalf("UpdateFlag failed: %v", err)
	}
	if !state.Enabled || state.Status != phase3.StatusShadow || state.RolloutPercentage != 25 {
		t.Fatalf("state = %#v", state)
	}
	value, ok := cfg.RAG.FeatureFlags.GetPhase3StrategyFlag(phase3.FlagModelAssistedRewrite)
	if !ok || !value {
		t.Fatalf("config flag = (%t, %t), want (true, true)", value, ok)
	}
	if cfg.RAG.Phase3.ModelRewriteShadowRatio != 0.25 {
		t.Fatalf("ModelRewriteShadowRatio = %.2f, want 0.25", cfg.RAG.Phase3.ModelRewriteShadowRatio)
	}
}

func TestRollbackPhase2BaselineDisablesManagedFlags(t *testing.T) {
	resetStateForTest()
	cfg := baseConfig()
	_, _ = UpdateFlag(context.Background(), cfg, phase3.FlagModelAssistedRewrite, true, phase3.StatusShadow, 20, "shadow rollout", 7)

	result, err := Rollback(context.Background(), cfg, phase3.StrategyTargetPhase2Baseline, nil, "rollback all", 9)
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}
	if result.Status != "succeeded" {
		t.Fatalf("rollback status = %q", result.Status)
	}
	for _, flagKey := range phase3.ManagedFeatureFlags() {
		value, ok := cfg.RAG.FeatureFlags.GetPhase3StrategyFlag(flagKey)
		if !ok {
			t.Fatalf("flag %q missing after rollback", flagKey)
		}
		if value {
			t.Fatalf("flag %q still enabled after rollback", flagKey)
		}
	}
}

func TestRollbackToVersionRestoresSavedState(t *testing.T) {
	resetStateForTest()
	cfg := baseConfig()

	shadowState, err := UpdateFlag(context.Background(), cfg, phase3.FlagModelAssistedRewrite, true, phase3.StatusShadow, 25, "shadow rollout", 7)
	if err != nil {
		t.Fatalf("UpdateFlag shadow failed: %v", err)
	}
	shadowVersion := ListVersions(cfg, phase3.FlagModelAssistedRewrite)[0]

	_, err = UpdateFlag(context.Background(), cfg, phase3.FlagModelAssistedRewrite, true, phase3.StatusEnabled, 100, "full rollout", 7)
	if err != nil {
		t.Fatalf("UpdateFlag enabled failed: %v", err)
	}

	result, err := Rollback(context.Background(), cfg, shadowVersion.VersionID, nil, "rollback to saved version", 9)
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}
	if result.Status != "succeeded" {
		t.Fatalf("rollback status = %q", result.Status)
	}
	if len(result.ChangedFlags) != 1 {
		t.Fatalf("changed flags len = %d, want 1", len(result.ChangedFlags))
	}
	if result.ChangedFlags[0].FlagKey != phase3.FlagModelAssistedRewrite {
		t.Fatalf("changed flag = %q", result.ChangedFlags[0].FlagKey)
	}
	if result.ChangedFlags[0].Status != shadowState.Status || result.ChangedFlags[0].RolloutPercentage != shadowState.RolloutPercentage {
		t.Fatalf("changed flag = %#v, want status=%s rollout=%d", result.ChangedFlags[0], shadowState.Status, shadowState.RolloutPercentage)
	}
	if cfg.RAG.Phase3.ModelRewriteShadowRatio != 0.25 {
		t.Fatalf("ModelRewriteShadowRatio = %.2f, want 0.25", cfg.RAG.Phase3.ModelRewriteShadowRatio)
	}
}

func TestRollbackFailureKeepsStateUntouched(t *testing.T) {
	resetStateForTest()
	cfg := baseConfig()

	_, err := UpdateFlag(context.Background(), cfg, phase3.FlagModelAssistedRewrite, true, phase3.StatusShadow, 20, "shadow rollout", 7)
	if err != nil {
		t.Fatalf("UpdateFlag failed: %v", err)
	}
	beforeFlags := ListFlags(cfg)
	beforeOps := len(ListOperations(cfg, ""))
	reconfigureManager = func(ctx context.Context, cfg *config.Config) error {
		return errors.New("reconfigure failed")
	}

	result, err := Rollback(context.Background(), cfg, phase3.StrategyTargetPhase2Baseline, nil, "force failure", 9)
	if err != nil {
		t.Fatalf("Rollback returned error: %v", err)
	}
	if result.Status != "failed" {
		t.Fatalf("rollback status = %q, want failed", result.Status)
	}

	afterFlags := ListFlags(cfg)
	if len(afterFlags) != len(beforeFlags) {
		t.Fatalf("after flags len = %d, want %d", len(afterFlags), len(beforeFlags))
	}
	for i := range beforeFlags {
		if afterFlags[i].FlagKey != beforeFlags[i].FlagKey || afterFlags[i].Status != beforeFlags[i].Status || afterFlags[i].RolloutPercentage != beforeFlags[i].RolloutPercentage || afterFlags[i].Enabled != beforeFlags[i].Enabled {
			t.Fatalf("flag[%d] changed after failed rollback: before=%#v after=%#v", i, beforeFlags[i], afterFlags[i])
		}
	}
	if got := len(ListOperations(cfg, "")); got != beforeOps {
		t.Fatalf("operation count = %d, want %d", got, beforeOps)
	}
	if cfg.RAG.Phase3.ModelRewriteShadowRatio != 0.20 {
		t.Fatalf("ModelRewriteShadowRatio = %.2f, want 0.20", cfg.RAG.Phase3.ModelRewriteShadowRatio)
	}
}
