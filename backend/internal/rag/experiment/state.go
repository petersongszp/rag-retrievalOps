package experiment

import (
	"context"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"sync"
	"time"

	"interview-agents/internal/config"
	"interview-agents/internal/rag/governance"
)

const (
	StrategyTypeRewrite       = "rewrite"
	StrategyTypeCandidateTopK = "candidate_topk"

	GroupBaseline  = "baseline"
	GroupCandidate = "candidate"
	GroupShadow    = "shadow"

	StatusDraft    = "draft"
	StatusRunning  = "running"
	StatusPaused   = "paused"
	StatusStopped  = "stopped"
	StatusFinished = "finished"

	EnvAll      = "all"
	EnvInternal = "internal"
	EnvExternal = "external"
)

type ConfigRecord struct {
	ExperimentID      string    `json:"experiment_id"`
	ExperimentName    string    `json:"experiment_name"`
	StrategyType      string    `json:"strategy_type"`
	BaselineVersion   string    `json:"baseline_version"`
	CandidateVersion  string    `json:"candidate_version"`
	TrafficRatio      int       `json:"traffic_ratio"`
	TargetKBIDs       []uint64  `json:"target_kb_ids,omitempty"`
	TargetQueryTypes  []string  `json:"target_query_types,omitempty"`
	TargetEnvironment string    `json:"target_environment,omitempty"`
	ShadowMode        bool      `json:"shadow_mode"`
	StartTime         time.Time `json:"start_time"`
	EndTime           time.Time `json:"end_time"`
	Owner             string    `json:"owner"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type Override struct {
	QueryType       string
	CandidateTopK   int
	ForceRewriteOff bool
}

type Decision struct {
	ExperimentID      string   `json:"experiment_id"`
	ExperimentName    string   `json:"experiment_name,omitempty"`
	StrategyType      string   `json:"strategy_type,omitempty"`
	Group             string   `json:"group,omitempty"`
	BaselineVersion   string   `json:"baseline_version,omitempty"`
	CandidateVersion  string   `json:"candidate_version,omitempty"`
	TrafficRatio      int      `json:"traffic_ratio,omitempty"`
	TargetKBIDs       []uint64 `json:"target_kb_ids,omitempty"`
	TargetQueryTypes  []string `json:"target_query_types,omitempty"`
	TargetEnvironment string   `json:"target_environment,omitempty"`
	ShadowMode        bool     `json:"shadow_mode"`
	Matched           bool     `json:"matched"`
	Reason            string   `json:"reason,omitempty"`
	Override          Override `json:"override"`
}

type Summary struct {
	ExperimentID          string    `json:"experiment_id"`
	ExperimentName        string    `json:"experiment_name"`
	StrategyType          string    `json:"strategy_type"`
	Status                string    `json:"status"`
	ShadowMode            bool      `json:"shadow_mode"`
	TrafficRatio          int       `json:"traffic_ratio"`
	BaselineVersion       string    `json:"baseline_version"`
	CandidateVersion      string    `json:"candidate_version"`
	StartTime             time.Time `json:"start_time"`
	EndTime               time.Time `json:"end_time"`
	Owner                 string    `json:"owner"`
	BaselineSampleSize    int       `json:"baseline_sample_size"`
	CandidateSampleSize   int       `json:"candidate_sample_size"`
	ShadowSampleSize      int       `json:"shadow_sample_size"`
	AvgDurationDeltaMS    float64   `json:"avg_duration_delta_ms"`
	AvgContextTokensDelta float64   `json:"avg_context_tokens_delta"`
	ErrorRateDelta        float64   `json:"error_rate_delta"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type requestScope struct {
	userID    uint
	userRole  string
	kbIDs     []uint64
	queryType string
	requestID string
	query     string
	topK      int
	now       time.Time
}

var (
	stateMu       sync.RWMutex
	experiments   = map[string]ConfigRecord{}
	order         []string
	experimentSeq int
)

func List() []ConfigRecord {
	stateMu.RLock()
	defer stateMu.RUnlock()

	items := make([]ConfigRecord, 0, len(order))
	for _, id := range order {
		record, ok := experiments[id]
		if ok {
			items = append(items, cloneRecord(record))
		}
	}
	return items
}

func Get(experimentID string) (ConfigRecord, bool) {
	stateMu.RLock()
	defer stateMu.RUnlock()

	record, ok := experiments[strings.TrimSpace(experimentID)]
	if !ok {
		return ConfigRecord{}, false
	}
	return cloneRecord(record), true
}

func Save(record ConfigRecord) (ConfigRecord, error) {
	stateMu.Lock()
	defer stateMu.Unlock()

	now := time.Now().UTC()
	if record.ExperimentID == "" {
		experimentSeq++
		record.ExperimentID = fmt.Sprintf("exp-%d", experimentSeq)
		record.CreatedAt = now
		order = append([]string{record.ExperimentID}, order...)
	} else if existing, ok := experiments[record.ExperimentID]; ok {
		record.CreatedAt = existing.CreatedAt
	}
	record.ExperimentName = strings.TrimSpace(record.ExperimentName)
	record.StrategyType = normalizeStrategyType(record.StrategyType)
	record.BaselineVersion = strings.TrimSpace(record.BaselineVersion)
	record.CandidateVersion = strings.TrimSpace(record.CandidateVersion)
	record.TargetEnvironment = normalizeEnvironment(record.TargetEnvironment)
	record.Owner = strings.TrimSpace(record.Owner)
	record.Status = normalizeStatus(record.Status)
	record.TargetQueryTypes = normalizeStrings(record.TargetQueryTypes)
	record.TargetKBIDs = append([]uint64(nil), record.TargetKBIDs...)
	record.UpdatedAt = now

	if err := validateRecord(record); err != nil {
		return ConfigRecord{}, err
	}
	experiments[record.ExperimentID] = record
	return cloneRecord(record), nil
}

func Rollback(ctx context.Context, cfg *config.Config, experimentID, reason string) (ConfigRecord, error) {
	stateMu.Lock()
	defer stateMu.Unlock()

	record, ok := experiments[strings.TrimSpace(experimentID)]
	if !ok {
		return ConfigRecord{}, fmt.Errorf("experiment not found: %s", experimentID)
	}
	record.Status = StatusStopped
	record.TrafficRatio = 0
	record.UpdatedAt = time.Now().UTC()
	experiments[record.ExperimentID] = record

	governance.EnqueueCompensation(
		"experiment_rollback",
		record.ExperimentID,
		"",
		firstNonEmpty(strings.TrimSpace(reason), "experiment_stopped"),
		map[string]interface{}{
			"experiment_id":     record.ExperimentID,
			"candidate_version": record.CandidateVersion,
		},
	)
	return cloneRecord(record), nil
}

func Decide(cfg *config.Config, userID uint, userRole string, kbIDs []uint64, query, requestID string, topK int) Decision {
	stateMu.RLock()
	defer stateMu.RUnlock()

	scope := requestScope{
		userID:    userID,
		userRole:  userRole,
		kbIDs:     append([]uint64(nil), kbIDs...),
		queryType: classifyQueryType(query),
		requestID: requestID,
		query:     query,
		topK:      topK,
		now:       time.Now().UTC(),
	}

	for _, id := range order {
		record, ok := experiments[id]
		if !ok {
			continue
		}
		decision := decideOne(cfg, record, scope)
		if decision.Matched {
			return decision
		}
	}
	return Decision{Reason: "no_active_experiment"}
}

func BuildSummary(logs []RetrieveLogLike) []Summary {
	stateMu.RLock()
	defer stateMu.RUnlock()

	summaries := make([]Summary, 0, len(order))
	for _, id := range order {
		record, ok := experiments[id]
		if !ok {
			continue
		}
		summaries = append(summaries, summarizeRecord(record, logs))
	}
	return summaries
}

type RetrieveLogLike interface {
	GetExperimentID() string
	GetExperimentGroup() string
	GetDurationMS() int64
	GetContextTokens() int
	GetResultStatus() string
}

func decideOne(cfg *config.Config, record ConfigRecord, scope requestScope) Decision {
	decision := Decision{
		ExperimentID:      record.ExperimentID,
		ExperimentName:    record.ExperimentName,
		StrategyType:      record.StrategyType,
		BaselineVersion:   record.BaselineVersion,
		CandidateVersion:  record.CandidateVersion,
		TrafficRatio:      record.TrafficRatio,
		TargetKBIDs:       append([]uint64(nil), record.TargetKBIDs...),
		TargetQueryTypes:  append([]string(nil), record.TargetQueryTypes...),
		TargetEnvironment: record.TargetEnvironment,
		ShadowMode:        record.ShadowMode,
	}
	if !cfg.RAG.FeatureFlags.EnableExperimentPlatform {
		decision.Reason = "experiment_platform_disabled"
		return decision
	}
	if record.Status != StatusRunning {
		decision.Reason = "experiment_not_running"
		return decision
	}
	if !record.StartTime.IsZero() && scope.now.Before(record.StartTime) {
		decision.Reason = "before_start_time"
		return decision
	}
	if !record.EndTime.IsZero() && scope.now.After(record.EndTime) {
		decision.Reason = "after_end_time"
		return decision
	}
	if !matchesEnvironment(scope.userRole, record.TargetEnvironment) {
		decision.Reason = "environment_mismatch"
		return decision
	}
	if len(record.TargetKBIDs) > 0 && !matchesAnyKB(scope.kbIDs, record.TargetKBIDs) {
		decision.Reason = "kb_mismatch"
		return decision
	}
	if len(record.TargetQueryTypes) > 0 && !containsFold(record.TargetQueryTypes, scope.queryType) {
		decision.Reason = "query_type_mismatch"
		return decision
	}
	if record.ShadowMode {
		decision.Matched = true
		decision.Group = GroupShadow
		decision.Reason = "shadow_mode"
		decision.Override = buildOverride(record, scope)
		return decision
	}
	if !matchesBucket(scope.userID, scope.requestID, record.TrafficRatio) {
		decision.Matched = true
		decision.Group = GroupBaseline
		decision.Reason = "baseline_bucket"
		return decision
	}
	decision.Matched = true
	decision.Group = GroupCandidate
	decision.Reason = "candidate_bucket"
	decision.Override = buildOverride(record, scope)
	return decision
}

func buildOverride(record ConfigRecord, scope requestScope) Override {
	override := Override{
		QueryType: scope.queryType,
	}
	switch record.StrategyType {
	case StrategyTypeCandidateTopK:
		if value, err := parseCandidateTopK(record.CandidateVersion); err == nil {
			override.CandidateTopK = value
		}
	case StrategyTypeRewrite:
		if strings.EqualFold(strings.TrimSpace(record.CandidateVersion), "rewrite_off") {
			override.ForceRewriteOff = true
		}
	}
	return override
}

func summarizeRecord(record ConfigRecord, logs []RetrieveLogLike) Summary {
	summary := Summary{
		ExperimentID:     record.ExperimentID,
		ExperimentName:   record.ExperimentName,
		StrategyType:     record.StrategyType,
		Status:           record.Status,
		ShadowMode:       record.ShadowMode,
		TrafficRatio:     record.TrafficRatio,
		BaselineVersion:  record.BaselineVersion,
		CandidateVersion: record.CandidateVersion,
		StartTime:        record.StartTime,
		EndTime:          record.EndTime,
		Owner:            record.Owner,
		UpdatedAt:        record.UpdatedAt,
	}

	var (
		baselineDuration  int64
		candidateDuration int64
		baselineTokens    int
		candidateTokens   int
		baselineErrors    int
		candidateErrors   int
	)
	for _, logEntry := range logs {
		if logEntry == nil || strings.TrimSpace(logEntry.GetExperimentID()) != record.ExperimentID {
			continue
		}
		switch classifyLogGroup(logEntry.GetExperimentGroup(), record.ShadowMode) {
		case GroupShadow:
			summary.ShadowSampleSize++
		case GroupCandidate:
			summary.CandidateSampleSize++
			candidateDuration += logEntry.GetDurationMS()
			candidateTokens += logEntry.GetContextTokens()
			if !strings.EqualFold(logEntry.GetResultStatus(), "success") {
				candidateErrors++
			}
		default:
			summary.BaselineSampleSize++
			baselineDuration += logEntry.GetDurationMS()
			baselineTokens += logEntry.GetContextTokens()
			if !strings.EqualFold(logEntry.GetResultStatus(), "success") {
				baselineErrors++
			}
		}
	}
	if summary.BaselineSampleSize > 0 && summary.CandidateSampleSize > 0 {
		summary.AvgDurationDeltaMS = avgInt64(candidateDuration, summary.CandidateSampleSize) - avgInt64(baselineDuration, summary.BaselineSampleSize)
		summary.AvgContextTokensDelta = avgInt(candidateTokens, summary.CandidateSampleSize) - avgInt(baselineTokens, summary.BaselineSampleSize)
		summary.ErrorRateDelta = rate(candidateErrors, summary.CandidateSampleSize) - rate(baselineErrors, summary.BaselineSampleSize)
	}
	return summary
}

func classifyLogGroup(experimentGroup string, shadowMode bool) string {
	if strings.TrimSpace(experimentGroup) != "" {
		return strings.TrimSpace(experimentGroup)
	}
	if shadowMode {
		return GroupShadow
	}
	return GroupBaseline
}

func validateRecord(record ConfigRecord) error {
	if record.ExperimentName == "" {
		return fmt.Errorf("experiment_name is required")
	}
	if record.StrategyType != StrategyTypeRewrite && record.StrategyType != StrategyTypeCandidateTopK {
		return fmt.Errorf("unsupported strategy_type: %s", record.StrategyType)
	}
	if record.BaselineVersion == "" {
		return fmt.Errorf("baseline_version is required")
	}
	if record.CandidateVersion == "" {
		return fmt.Errorf("candidate_version is required")
	}
	if record.TrafficRatio < 0 || record.TrafficRatio > 100 {
		return fmt.Errorf("traffic_ratio must be between 0 and 100")
	}
	if record.TargetEnvironment != EnvAll && record.TargetEnvironment != EnvInternal && record.TargetEnvironment != EnvExternal {
		return fmt.Errorf("unsupported target_environment: %s", record.TargetEnvironment)
	}
	if record.Status != StatusDraft && record.Status != StatusRunning && record.Status != StatusPaused &&
		record.Status != StatusStopped && record.Status != StatusFinished {
		return fmt.Errorf("unsupported status: %s", record.Status)
	}
	if !record.StartTime.IsZero() && !record.EndTime.IsZero() && record.EndTime.Before(record.StartTime) {
		return fmt.Errorf("end_time cannot be earlier than start_time")
	}
	if record.StrategyType == StrategyTypeCandidateTopK {
		if _, err := parseCandidateTopK(record.CandidateVersion); err != nil {
			return err
		}
	}
	return nil
}

func parseCandidateTopK(version string) (int, error) {
	value := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(version), "topk:"))
	switch value {
	case "":
		return 0, fmt.Errorf("candidate_version must be like topk:8")
	default:
		var parsed int
		_, err := fmt.Sscanf(value, "%d", &parsed)
		if err != nil || parsed <= 0 {
			return 0, fmt.Errorf("candidate_version must be like topk:8")
		}
		return parsed, nil
	}
}

func classifyQueryType(query string) string {
	trimmed := strings.TrimSpace(strings.ToLower(query))
	switch {
	case trimmed == "":
		return "unknown"
	case strings.Contains(trimmed, "how") || strings.Contains(trimmed, "为什么") || strings.Contains(trimmed, "怎么"):
		return "reasoning"
	case strings.Contains(trimmed, "what") || strings.Contains(trimmed, "是什么"):
		return "entity"
	default:
		return "general"
	}
}

func matchesEnvironment(userRole, target string) bool {
	switch normalizeEnvironment(target) {
	case EnvInternal:
		return strings.EqualFold(strings.TrimSpace(userRole), "admin")
	case EnvExternal:
		return !strings.EqualFold(strings.TrimSpace(userRole), "admin")
	default:
		return true
	}
}

func matchesAnyKB(requestKBs, targetKBs []uint64) bool {
	lookup := make(map[uint64]struct{}, len(targetKBs))
	for _, kbID := range targetKBs {
		lookup[kbID] = struct{}{}
	}
	for _, kbID := range requestKBs {
		if _, ok := lookup[kbID]; ok {
			return true
		}
	}
	return false
}

func matchesBucket(userID uint, requestID string, percent int) bool {
	if percent <= 0 {
		return false
	}
	if percent >= 100 {
		return true
	}
	key := strings.TrimSpace(requestID)
	if key == "" {
		key = fmt.Sprintf("%d", userID)
	}
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(key))
	return int(hasher.Sum32()%100) < percent
}

func containsFold(items []string, target string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func normalizeEnvironment(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", EnvAll:
		return EnvAll
	case EnvInternal:
		return EnvInternal
	case EnvExternal:
		return EnvExternal
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func normalizeStrategyType(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", StatusDraft:
		return StatusDraft
	case StatusRunning:
		return StatusRunning
	case StatusPaused:
		return StatusPaused
	case StatusStopped:
		return StatusStopped
	case StatusFinished:
		return StatusFinished
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func normalizeStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	sort.Strings(result)
	return result
}

func cloneRecord(record ConfigRecord) ConfigRecord {
	record.TargetKBIDs = append([]uint64(nil), record.TargetKBIDs...)
	record.TargetQueryTypes = append([]string(nil), record.TargetQueryTypes...)
	return record
}

func avgInt64(total int64, count int) float64 {
	if count <= 0 {
		return 0
	}
	return float64(total) / float64(count)
}

func avgInt(total int, count int) float64 {
	if count <= 0 {
		return 0
	}
	return float64(total) / float64(count)
}

func rate(numerator, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
