package release

import (
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"time"

	"interview-agents/internal/config"
)

const (
	StrategyPhase1 = "phase1"
	StrategyPhase2 = "phase2"

	StagePhase1   = "phase1"
	StageInternal = "internal"
	StageSmall    = "small_flow"
	StageBatch    = "batch"
	StageFull     = "full"
)

type Decision struct {
	Strategy  string `json:"strategy"`
	Stage     string `json:"stage"`
	Reason    string `json:"reason"`
	UsePhase2 bool   `json:"use_phase2"`
}

type RuntimeOverride struct {
	Active    bool      `json:"active"`
	Stage     string    `json:"stage"`
	Reason    string    `json:"reason"`
	UpdatedBy uint      `json:"updated_by"`
	UpdatedAt time.Time `json:"updated_at"`
}

var (
	runtimeOverrideMu sync.RWMutex
	runtimeOverride   RuntimeOverride
)

func Decide(ragCfg config.RAGConfig, phase2Available bool, userID uint, userRole string) Decision {
	if !phase2Available {
		return Decision{
			Strategy: StrategyPhase1,
			Stage:    StagePhase1,
			Reason:   "phase2_unavailable",
		}
	}

	override := GetRuntimeOverride()
	if override.Active {
		stage := NormalizeStage(override.Stage)
		if stage == StagePhase1 {
			return Decision{
				Strategy: StrategyPhase1,
				Stage:    StagePhase1,
				Reason:   firstNonEmpty(strings.TrimSpace(override.Reason), "runtime_override_phase1"),
			}
		}
		return evaluateStage(ragCfg.Release, stage, userID, userRole, firstNonEmpty(strings.TrimSpace(override.Reason), "runtime_override"))
	}

	if !ragCfg.Release.Enabled {
		return Decision{
			Strategy:  StrategyPhase2,
			Stage:     StageFull,
			Reason:    "legacy_flags",
			UsePhase2: true,
		}
	}

	return evaluateStage(ragCfg.Release, NormalizeStage(ragCfg.Release.Stage), userID, userRole, "")
}

func SetRuntimeOverride(stage, reason string, updatedBy uint) RuntimeOverride {
	override := RuntimeOverride{
		Active:    true,
		Stage:     NormalizeStage(stage),
		Reason:    strings.TrimSpace(reason),
		UpdatedBy: updatedBy,
		UpdatedAt: time.Now().UTC(),
	}
	runtimeOverrideMu.Lock()
	runtimeOverride = override
	runtimeOverrideMu.Unlock()
	return override
}

func ClearRuntimeOverride() {
	runtimeOverrideMu.Lock()
	runtimeOverride = RuntimeOverride{}
	runtimeOverrideMu.Unlock()
}

func GetRuntimeOverride() RuntimeOverride {
	runtimeOverrideMu.RLock()
	defer runtimeOverrideMu.RUnlock()
	return runtimeOverride
}

func NormalizeStage(stage string) string {
	switch strings.ToLower(strings.TrimSpace(stage)) {
	case "", "full", "all":
		return StageFull
	case "phase1", "rollback", "off":
		return StagePhase1
	case "internal":
		return StageInternal
	case "small_flow", "small-flow", "canary":
		return StageSmall
	case "batch", "batch_flow", "batch-flow":
		return StageBatch
	default:
		return strings.ToLower(strings.TrimSpace(stage))
	}
}

func RollbackPlan() []string {
	return []string{
		"Set runtime release stage to phase1 via /api/admin/kb/release/rollback",
		"Disable RAG_ENABLE_HYBRID_RETRIEVAL in persistent config after emergency rollback",
		"Disable RAG_ENABLE_QUERY_REWRITE and RAG_ENABLE_DYNAMIC_TOPK if online metrics still regress",
		"Disable RAG_ENABLE_ADVANCED_RERANK when rerank latency keeps rising",
		"Confirm retrieve P95, empty-after-filter, and error rate return to Phase 1 baseline",
	}
}

func StagePlan(stage string, releaseCfg config.RAGReleaseConfig) []string {
	stage = NormalizeStage(stage)
	switch stage {
	case StageInternal:
		return []string{
			"Internal environment full traffic validation",
			"Only internal roles receive Phase 2 strategy",
			"Verify rewrite hit rate, route contribution, and rerank latency before public traffic",
		}
	case StageSmall:
		return []string{
			"Keep internal roles fully on Phase 2",
			fmt.Sprintf("Expose Phase 2 to %d%% stable user bucket", releaseCfg.CanaryPercent),
			"Watch error rate, P95 latency, and Empty-After-Filter ratio for at least one observation window",
		}
	case StageBatch:
		return []string{
			"Keep internal roles fully on Phase 2",
			fmt.Sprintf("Expand Phase 2 to %d%% stable user bucket", releaseCfg.BatchPercent),
			"Run acceptance summary before moving to full traffic",
		}
	case StageFull:
		return []string{
			"Enable Phase 2 for all traffic",
			"Keep rollback endpoint and alert rules active until post-release review finishes",
			"Archive acceptance summary and release retrospective",
		}
	default:
		return []string{
			"Stay on Phase 1 baseline",
			"Use release status endpoint to confirm no user is routed to Phase 2",
		}
	}
}

func evaluateStage(releaseCfg config.RAGReleaseConfig, stage string, userID uint, userRole string, reasonPrefix string) Decision {
	if isAllowlisted(userID, releaseCfg.UserAllowlist) {
		return phase2Decision(stage, firstNonEmpty(reasonPrefix, "allowlist"))
	}
	if isInternalRole(userRole, releaseCfg.InternalRoles) {
		return phase2Decision(stage, firstNonEmpty(reasonPrefix, "internal_role"))
	}

	switch stage {
	case StagePhase1:
		return Decision{Strategy: StrategyPhase1, Stage: StagePhase1, Reason: firstNonEmpty(reasonPrefix, "release_stage_phase1")}
	case StageInternal:
		return Decision{Strategy: StrategyPhase1, Stage: StageInternal, Reason: firstNonEmpty(reasonPrefix, "not_internal")}
	case StageSmall:
		if matchesStableBucket(userID, releaseCfg.CanaryPercent) {
			return phase2Decision(stage, firstNonEmpty(reasonPrefix, "canary_bucket"))
		}
		return Decision{Strategy: StrategyPhase1, Stage: stage, Reason: firstNonEmpty(reasonPrefix, "outside_canary_bucket")}
	case StageBatch:
		if matchesStableBucket(userID, releaseCfg.BatchPercent) {
			return phase2Decision(stage, firstNonEmpty(reasonPrefix, "batch_bucket"))
		}
		return Decision{Strategy: StrategyPhase1, Stage: stage, Reason: firstNonEmpty(reasonPrefix, "outside_batch_bucket")}
	case StageFull:
		return phase2Decision(stage, firstNonEmpty(reasonPrefix, "full_release"))
	default:
		return Decision{Strategy: StrategyPhase1, Stage: StagePhase1, Reason: firstNonEmpty(reasonPrefix, "invalid_stage")}
	}
}

func phase2Decision(stage, reason string) Decision {
	return Decision{
		Strategy:  StrategyPhase2,
		Stage:     stage,
		Reason:    reason,
		UsePhase2: true,
	}
}

func isAllowlisted(userID uint, allowlist []uint) bool {
	if userID == 0 {
		return false
	}
	for _, id := range allowlist {
		if id == userID {
			return true
		}
	}
	return false
}

func isInternalRole(role string, internalRoles []string) bool {
	role = strings.TrimSpace(strings.ToLower(role))
	if role == "" {
		return false
	}
	if len(internalRoles) == 0 {
		internalRoles = []string{"admin"}
	}
	for _, candidate := range internalRoles {
		if role == strings.TrimSpace(strings.ToLower(candidate)) {
			return true
		}
	}
	return false
}

func matchesStableBucket(userID uint, percent int) bool {
	if userID == 0 || percent <= 0 {
		return false
	}
	if percent >= 100 {
		return true
	}
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(fmt.Sprintf("%d", userID)))
	return int(hasher.Sum32()%100) < percent
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
