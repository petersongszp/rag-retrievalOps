package release

import (
	"testing"

	"interview-agents/internal/config"
)

func TestDecideLegacyFlags(t *testing.T) {
	decision := Decide(config.RAGConfig{}, true, 100, "user")
	if !decision.UsePhase2 || decision.Strategy != StrategyPhase2 {
		t.Fatalf("expected legacy decision to use phase2, got %+v", decision)
	}
}

func TestDecideInternalStage(t *testing.T) {
	cfg := config.RAGConfig{
		Release: config.RAGReleaseConfig{
			Enabled:       true,
			Stage:         "internal",
			InternalRoles: []string{"admin"},
		},
	}
	if decision := Decide(cfg, true, 100, "admin"); !decision.UsePhase2 {
		t.Fatalf("expected admin to receive phase2, got %+v", decision)
	}
	if decision := Decide(cfg, true, 100, "user"); decision.UsePhase2 {
		t.Fatalf("expected normal user to stay on phase1, got %+v", decision)
	}
}

func TestDecideCanaryStageAllowlist(t *testing.T) {
	cfg := config.RAGConfig{
		Release: config.RAGReleaseConfig{
			Enabled:       true,
			Stage:         "small_flow",
			CanaryPercent: 1,
			UserAllowlist: []uint{42},
		},
	}
	decision := Decide(cfg, true, 42, "user")
	if !decision.UsePhase2 || decision.Reason != "allowlist" {
		t.Fatalf("expected allowlisted user to receive phase2, got %+v", decision)
	}
}

func TestRuntimeOverridePhase1(t *testing.T) {
	ClearRuntimeOverride()
	SetRuntimeOverride("phase1", "test", 1)
	defer ClearRuntimeOverride()

	decision := Decide(config.RAGConfig{}, true, 100, "admin")
	if decision.UsePhase2 || decision.Strategy != StrategyPhase1 {
		t.Fatalf("expected runtime override to force phase1, got %+v", decision)
	}
}
