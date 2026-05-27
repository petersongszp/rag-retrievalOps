package phase3

import "testing"

func TestManagedFeatureFlagsMatchesRollbackContract(t *testing.T) {
	flags := ManagedFeatureFlags()
	order := RollbackOrder()

	if len(flags) != 7 {
		t.Fatalf("ManagedFeatureFlags len = %d, want 7", len(flags))
	}
	if len(order) != len(flags) {
		t.Fatalf("RollbackOrder len = %d, want %d", len(order), len(flags))
	}

	seen := make(map[string]bool, len(order))
	for _, flagKey := range order {
		if !IsManagedFeatureFlag(flagKey) {
			t.Fatalf("RollbackOrder contains unmanaged flag %q", flagKey)
		}
		if seen[flagKey] {
			t.Fatalf("RollbackOrder contains duplicate flag %q", flagKey)
		}
		seen[flagKey] = true
	}

	for _, flagKey := range flags {
		if !seen[flagKey] {
			t.Fatalf("managed flag %q missing from RollbackOrder", flagKey)
		}
	}
}

func TestStrategyStatusesAreFrozen(t *testing.T) {
	want := []string{
		StatusEnabled,
		StatusDisabled,
		StatusShadow,
		StatusCanary,
		StatusRollingBack,
		StatusError,
	}
	got := StrategyStatuses()
	if len(got) != len(want) {
		t.Fatalf("StrategyStatuses len = %d, want %d", len(got), len(want))
	}
	for index, status := range want {
		if got[index] != status {
			t.Fatalf("StrategyStatuses[%d] = %q, want %q", index, got[index], status)
		}
		if !IsStrategyStatus(status) {
			t.Fatalf("IsStrategyStatus(%q) = false, want true", status)
		}
	}
	if IsStrategyStatus("unknown") {
		t.Fatal("IsStrategyStatus(\"unknown\") = true, want false")
	}
}
