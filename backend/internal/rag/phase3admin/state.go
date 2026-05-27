package phase3admin

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"interview-agents/internal/config"
	"interview-agents/internal/milvus"
	"interview-agents/internal/rag/phase3"
)

type FlagState struct {
	FlagKey           string    `json:"flag_key"`
	Label             string    `json:"label"`
	Status            string    `json:"status"`
	Enabled           bool      `json:"enabled"`
	RolloutPercentage int       `json:"rollout_percentage"`
	StrategyVersion   string    `json:"strategy_version"`
	RiskLevel         string    `json:"risk_level"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type VersionRecord struct {
	VersionID         string                 `json:"version_id"`
	FlagKey           string                 `json:"flag_key"`
	Label             string                 `json:"label"`
	CreatedAt         time.Time              `json:"created_at"`
	CreatedBy         string                 `json:"created_by"`
	GateStatus        string                 `json:"gate_status"`
	BaselineReportID  string                 `json:"baseline_report_id,omitempty"`
	CandidateReportID string                 `json:"candidate_report_id,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

type OperationRecord struct {
	ID                    string    `json:"id"`
	OperatorID            uint      `json:"operator_id"`
	Operation             string    `json:"operation"`
	FlagKey               string    `json:"flag_key,omitempty"`
	FromStatus            string    `json:"from_status,omitempty"`
	ToStatus              string    `json:"to_status,omitempty"`
	FromRolloutPercentage int       `json:"from_rollout_percentage,omitempty"`
	ToRolloutPercentage   int       `json:"to_rollout_percentage,omitempty"`
	Reason                string    `json:"reason"`
	CreatedAt             time.Time `json:"created_at"`
}

type RollbackResult struct {
	RollbackID    string      `json:"rollback_id"`
	Status        string      `json:"status"`
	ChangedFlags  []FlagState `json:"changed_flags"`
	TargetVersion string      `json:"target_version"`
	StartedAt     time.Time   `json:"started_at"`
	FinishedAt    time.Time   `json:"finished_at"`
	ErrorMsg      string      `json:"error_msg,omitempty"`
}

type pendingRollbackChange struct {
	Previous FlagState
	Next     FlagState
}

var (
	stateMu           sync.Mutex
	initialized       bool
	flagStates        = map[string]FlagState{}
	versionRecords    = map[string][]VersionRecord{}
	versionByID       = map[string]VersionRecord{}
	operationRecords  []OperationRecord
	versionCounter    int
	operationCounter  int
	rollbackCounter   int
	lastConfigVersion string

	reconfigureManager = milvus.ReconfigureGlobalManager

	flagLabels = map[string]string{
		phase3.FlagParentChildRetrieval: "Parent Child Retrieval",
		phase3.FlagStrategicTopK:        "Strategic TopK",
		phase3.FlagEvidenceRefusal:      "Evidence Refusal",
		phase3.FlagCitationConsistency:  "Citation Consistency",
		phase3.FlagDomainTerms:          "Domain Terms",
		phase3.FlagRouteSpecificRewrite: "Route Specific Rewrite",
		phase3.FlagModelAssistedRewrite: "Model Assisted Rewrite",
	}
	flagRiskLevels = map[string]string{
		phase3.FlagParentChildRetrieval: "medium",
		phase3.FlagStrategicTopK:        "medium",
		phase3.FlagEvidenceRefusal:      "medium",
		phase3.FlagCitationConsistency:  "medium",
		phase3.FlagDomainTerms:          "low",
		phase3.FlagRouteSpecificRewrite: "medium",
		phase3.FlagModelAssistedRewrite: "high",
	}
	flagVersions = map[string]string{
		phase3.FlagParentChildRetrieval: "p3-parent-child-v1",
		phase3.FlagStrategicTopK:        "p3-strategic-topk-v1",
		phase3.FlagEvidenceRefusal:      "p3-evidence-refusal-v1",
		phase3.FlagCitationConsistency:  "p3-citation-consistency-v1",
		phase3.FlagDomainTerms:          "p3-domain-terms-v1",
		phase3.FlagRouteSpecificRewrite: "p3-route-specific-rewrite-v1",
		phase3.FlagModelAssistedRewrite: "p3-model-assisted-rewrite-v1",
	}
)

func ListFlags(cfg *config.Config) []FlagState {
	stateMu.Lock()
	defer stateMu.Unlock()

	ensureStateLocked(cfg)
	items := make([]FlagState, 0, len(flagStates))
	for _, flagKey := range phase3.ManagedFeatureFlags() {
		items = append(items, flagStates[flagKey])
	}
	return items
}

func UpdateFlag(ctx context.Context, cfg *config.Config, flagKey string, enabled bool, status string, rolloutPercentage int, reason string, operatorID uint) (FlagState, error) {
	stateMu.Lock()
	defer stateMu.Unlock()

	ensureStateLocked(cfg)
	if !phase3.IsManagedFeatureFlag(flagKey) {
		return FlagState{}, fmt.Errorf("unsupported flag_key: %s", flagKey)
	}
	if strings.TrimSpace(reason) == "" {
		return FlagState{}, fmt.Errorf("reason is required")
	}
	if rolloutPercentage < 0 || rolloutPercentage > 100 {
		return FlagState{}, fmt.Errorf("rollout_percentage must be between 0 and 100")
	}
	if !phase3.IsStrategyStatus(status) {
		return FlagState{}, fmt.Errorf("unsupported status: %s", status)
	}

	current := flagStates[flagKey]
	fromStatus := current.Status
	fromRollout := current.RolloutPercentage
	if isHighRiskFlag(flagKey) && !current.Enabled && enabled && status != phase3.StatusShadow && status != phase3.StatusCanary {
		return FlagState{}, fmt.Errorf("high-risk flag %s must be enabled through shadow or canary first", flagKey)
	}

	snapshot := *cfg
	if !cfg.RAG.FeatureFlags.SetPhase3StrategyFlag(flagKey, enabled) {
		return FlagState{}, fmt.Errorf("failed to apply flag_key: %s", flagKey)
	}
	applyRolloutSideEffects(cfg, flagKey, status, rolloutPercentage)
	if err := reconfigureManager(ctx, cfg); err != nil {
		*cfg = snapshot
		return FlagState{}, err
	}

	current.Enabled = enabled
	current.Status = status
	current.RolloutPercentage = rolloutPercentage
	current.UpdatedAt = time.Now().UTC()
	flagStates[flagKey] = current
	recordOperationLocked("update_flag", flagKey, fromStatus, status, fromRollout, rolloutPercentage, reason, operatorID)
	appendVersionLocked(current, operatorID)
	return current, nil
}

func ListVersions(cfg *config.Config, flagKey string) []VersionRecord {
	stateMu.Lock()
	defer stateMu.Unlock()

	ensureStateLocked(cfg)
	if flagKey == "" {
		records := make([]VersionRecord, 0)
		for _, managedFlag := range phase3.ManagedFeatureFlags() {
			records = append(records, versionRecords[managedFlag]...)
		}
		sort.SliceStable(records, func(i, j int) bool {
			return records[i].CreatedAt.After(records[j].CreatedAt)
		})
		return records
	}
	return append([]VersionRecord(nil), versionRecords[flagKey]...)
}

func GetVersion(cfg *config.Config, versionID string) (VersionRecord, bool) {
	stateMu.Lock()
	defer stateMu.Unlock()

	ensureStateLocked(cfg)
	record, ok := versionByID[versionID]
	return record, ok
}

func Rollback(ctx context.Context, cfg *config.Config, targetVersion string, flagKeys []string, reason string, operatorID uint) (RollbackResult, error) {
	stateMu.Lock()
	defer stateMu.Unlock()

	ensureStateLocked(cfg)
	if strings.TrimSpace(reason) == "" {
		return RollbackResult{}, fmt.Errorf("reason is required")
	}

	startedAt := time.Now().UTC()
	rollbackCounter++
	result := RollbackResult{
		RollbackID:    fmt.Sprintf("rollback-%d", rollbackCounter),
		Status:        "running",
		TargetVersion: targetVersion,
		StartedAt:     startedAt,
	}

	changes, err := buildRollbackChangesLocked(targetVersion, flagKeys)
	if err != nil {
		return RollbackResult{}, err
	}

	snapshot := *cfg
	for _, change := range changes {
		if !cfg.RAG.FeatureFlags.SetPhase3StrategyFlag(change.Next.FlagKey, change.Next.Enabled) {
			*cfg = snapshot
			result.Status = "failed"
			result.ErrorMsg = fmt.Sprintf("failed to apply flag %s", change.Next.FlagKey)
			result.FinishedAt = time.Now().UTC()
			return result, nil
		}
		applyRolloutSideEffects(cfg, change.Next.FlagKey, change.Next.Status, change.Next.RolloutPercentage)
	}
	if err := reconfigureManager(ctx, cfg); err != nil {
		*cfg = snapshot
		result.Status = "failed"
		result.ErrorMsg = err.Error()
		result.FinishedAt = time.Now().UTC()
		return result, nil
	}

	changedFlags := make([]FlagState, 0, len(changes))
	for _, change := range changes {
		next := change.Next
		next.UpdatedAt = time.Now().UTC()
		flagStates[next.FlagKey] = next
		recordOperationLocked(
			"rollback",
			next.FlagKey,
			change.Previous.Status,
			next.Status,
			change.Previous.RolloutPercentage,
			next.RolloutPercentage,
			reason,
			operatorID,
		)
		appendVersionLocked(next, operatorID)
		changedFlags = append(changedFlags, next)
	}

	result.Status = "succeeded"
	result.ChangedFlags = changedFlags
	result.FinishedAt = time.Now().UTC()
	return result, nil
}

func ListOperations(cfg *config.Config, flagKey string) []OperationRecord {
	stateMu.Lock()
	defer stateMu.Unlock()

	ensureStateLocked(cfg)
	if flagKey == "" {
		return append([]OperationRecord(nil), operationRecords...)
	}
	filtered := make([]OperationRecord, 0)
	for _, record := range operationRecords {
		if record.FlagKey == flagKey {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func ensureStateLocked(cfg *config.Config) {
	if cfg == nil {
		return
	}
	if initialized && lastConfigVersion == cfg.ConfigVersion {
		return
	}

	now := time.Now().UTC()
	for _, flagKey := range phase3.ManagedFeatureFlags() {
		enabled, _ := cfg.RAG.FeatureFlags.GetPhase3StrategyFlag(flagKey)
		state := FlagState{
			FlagKey:           flagKey,
			Label:             flagLabels[flagKey],
			Status:            resolveStatus(flagKey, enabled, cfg),
			Enabled:           enabled,
			RolloutPercentage: resolveRollout(flagKey, enabled, cfg),
			StrategyVersion:   flagVersions[flagKey],
			RiskLevel:         flagRiskLevels[flagKey],
			UpdatedAt:         now,
		}
		if existing, ok := flagStates[flagKey]; ok {
			state.UpdatedAt = existing.UpdatedAt
			if existing.UpdatedAt.IsZero() {
				state.UpdatedAt = now
			}
		}
		flagStates[flagKey] = state
		if len(versionRecords[flagKey]) == 0 {
			appendVersionLocked(state, 0)
		}
	}
	initialized = true
	lastConfigVersion = cfg.ConfigVersion
}

func buildRollbackChangesLocked(targetVersion string, flagKeys []string) ([]pendingRollbackChange, error) {
	if targetVersion == phase3.StrategyTargetPhase2Baseline {
		targets, err := resolveRollbackTargets(flagKeys)
		if err != nil {
			return nil, err
		}
		changes := make([]pendingRollbackChange, 0, len(targets))
		for _, flagKey := range targets {
			current := flagStates[flagKey]
			next := current
			next.Enabled = false
			next.Status = phase3.StatusDisabled
			next.RolloutPercentage = 0
			changes = append(changes, pendingRollbackChange{
				Previous: current,
				Next:     next,
			})
		}
		return changes, nil
	}

	record, ok := versionByID[targetVersion]
	if !ok {
		return nil, fmt.Errorf("strategy version not found: %s", targetVersion)
	}
	targets := []string{}
	var err error
	if len(flagKeys) > 0 {
		targets, err = resolveRollbackTargets(flagKeys)
		if err != nil {
			return nil, err
		}
	}
	if len(targets) == 0 {
		targets = []string{record.FlagKey}
	}
	if len(targets) != 1 || targets[0] != record.FlagKey {
		return nil, fmt.Errorf("target_version %s only applies to flag %s", targetVersion, record.FlagKey)
	}

	current := flagStates[record.FlagKey]
	next, err := stateFromVersionRecord(current, record)
	if err != nil {
		return nil, err
	}
	return []pendingRollbackChange{{
		Previous: current,
		Next:     next,
	}}, nil
}

func stateFromVersionRecord(current FlagState, record VersionRecord) (FlagState, error) {
	metadata := record.Metadata
	if metadata == nil {
		return FlagState{}, fmt.Errorf("strategy version %s metadata is empty", record.VersionID)
	}

	enabled, ok := boolMetadata(metadata, "enabled")
	if !ok {
		return FlagState{}, fmt.Errorf("strategy version %s missing enabled metadata", record.VersionID)
	}
	status, ok := stringMetadata(metadata, "status")
	if !ok || !phase3.IsStrategyStatus(status) {
		return FlagState{}, fmt.Errorf("strategy version %s missing valid status metadata", record.VersionID)
	}
	rolloutPercentage, ok := intMetadata(metadata, "rollout_percentage")
	if !ok {
		return FlagState{}, fmt.Errorf("strategy version %s missing rollout_percentage metadata", record.VersionID)
	}
	if rolloutPercentage < 0 || rolloutPercentage > 100 {
		return FlagState{}, fmt.Errorf("strategy version %s has invalid rollout_percentage metadata", record.VersionID)
	}

	next := current
	next.Enabled = enabled
	next.Status = status
	next.RolloutPercentage = rolloutPercentage
	if strategyVersion, ok := stringMetadata(metadata, "strategy_version"); ok && strings.TrimSpace(strategyVersion) != "" {
		next.StrategyVersion = strategyVersion
	}
	return next, nil
}

func appendVersionLocked(state FlagState, operatorID uint) {
	versionCounter++
	record := VersionRecord{
		VersionID:  fmt.Sprintf("%s:v%d", state.FlagKey, versionCounter),
		FlagKey:    state.FlagKey,
		Label:      state.Label,
		CreatedAt:  time.Now().UTC(),
		CreatedBy:  createdByLabel(operatorID),
		GateStatus: "pending",
		Metadata: map[string]interface{}{
			"status":             state.Status,
			"enabled":            state.Enabled,
			"rollout_percentage": state.RolloutPercentage,
			"strategy_version":   state.StrategyVersion,
		},
	}
	versionRecords[state.FlagKey] = append([]VersionRecord{record}, versionRecords[state.FlagKey]...)
	versionByID[record.VersionID] = record
}

func recordOperationLocked(operation, flagKey, fromStatus, toStatus string, fromRollout, toRollout int, reason string, operatorID uint) {
	operationCounter++
	operationRecords = append([]OperationRecord{
		{
			ID:                    fmt.Sprintf("op-%d", operationCounter),
			OperatorID:            operatorID,
			Operation:             operation,
			FlagKey:               flagKey,
			FromStatus:            fromStatus,
			ToStatus:              toStatus,
			FromRolloutPercentage: fromRollout,
			ToRolloutPercentage:   toRollout,
			Reason:                reason,
			CreatedAt:             time.Now().UTC(),
		},
	}, operationRecords...)
}

func resolveRollbackTargets(flagKeys []string) ([]string, error) {
	if len(flagKeys) == 0 {
		return phase3.RollbackOrder(), nil
	}

	targets := make([]string, 0, len(flagKeys))
	allowed := make(map[string]struct{}, len(flagKeys))
	for _, flagKey := range flagKeys {
		trimmed := strings.TrimSpace(flagKey)
		if !phase3.IsManagedFeatureFlag(trimmed) {
			return nil, fmt.Errorf("unsupported flag_key: %s", trimmed)
		}
		allowed[trimmed] = struct{}{}
	}
	for _, flagKey := range phase3.RollbackOrder() {
		if _, ok := allowed[flagKey]; ok {
			targets = append(targets, flagKey)
		}
	}
	return targets, nil
}

func applyRolloutSideEffects(cfg *config.Config, flagKey, status string, rolloutPercentage int) {
	if cfg == nil {
		return
	}
	if flagKey == phase3.FlagModelAssistedRewrite {
		switch status {
		case phase3.StatusShadow, phase3.StatusCanary:
			cfg.RAG.Phase3.ModelRewriteShadowRatio = float64(rolloutPercentage) / 100
		case phase3.StatusEnabled:
			cfg.RAG.Phase3.ModelRewriteShadowRatio = 1
		default:
			cfg.RAG.Phase3.ModelRewriteShadowRatio = 0
		}
	}
}

func resolveStatus(flagKey string, enabled bool, cfg *config.Config) string {
	if !enabled {
		return phase3.StatusDisabled
	}
	if flagKey == phase3.FlagModelAssistedRewrite && cfg != nil && cfg.RAG.Phase3.ModelRewriteShadowRatio > 0 && cfg.RAG.Phase3.ModelRewriteShadowRatio < 1 {
		return phase3.StatusShadow
	}
	return phase3.StatusEnabled
}

func resolveRollout(flagKey string, enabled bool, cfg *config.Config) int {
	if !enabled {
		return 0
	}
	if flagKey == phase3.FlagModelAssistedRewrite && cfg != nil && cfg.RAG.Phase3.ModelRewriteShadowRatio > 0 && cfg.RAG.Phase3.ModelRewriteShadowRatio < 1 {
		return int(cfg.RAG.Phase3.ModelRewriteShadowRatio * 100)
	}
	return 100
}

func isHighRiskFlag(flagKey string) bool {
	return flagKey == phase3.FlagModelAssistedRewrite
}

func createdByLabel(operatorID uint) string {
	if operatorID == 0 {
		return "system"
	}
	return fmt.Sprintf("user:%d", operatorID)
}

func boolMetadata(metadata map[string]interface{}, key string) (bool, bool) {
	value, ok := metadata[key]
	if !ok {
		return false, false
	}
	typed, ok := value.(bool)
	return typed, ok
}

func intMetadata(metadata map[string]interface{}, key string) (int, bool) {
	value, ok := metadata[key]
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float32:
		return int(typed), true
	case float64:
		return int(typed), true
	default:
		return 0, false
	}
}

func stringMetadata(metadata map[string]interface{}, key string) (string, bool) {
	value, ok := metadata[key]
	if !ok {
		return "", false
	}
	typed, ok := value.(string)
	return typed, ok
}
