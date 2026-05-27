package kb

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"interview-agents/api/response"
	"interview-agents/internal/config"
	myerrors "interview-agents/internal/errors"
	"interview-agents/internal/milvus/evaluation"
	"interview-agents/internal/model"
	"interview-agents/internal/rag/phase3"
	"interview-agents/internal/rag/phase3admin"

	"github.com/cloudwego/hertz/pkg/app"
)

const (
	defaultStrategyImpactRange          = "24h"
	minStrategyComparisonSampleSize     = 5
	maxStrategyEvalRunSearchPageSize    = 50
	strategyGateStatusPassed            = "passed"
	strategyGateStatusFailed            = "failed"
	strategyGateStatusPending           = "pending"
	strategyGateStatusMissingReport     = "report_missing"
	strategyGateStatusSelectionMismatch = "selection_mismatch"
)

type strategyImpactResponse struct {
	FlagKey                string             `json:"flag_key,omitempty"`
	Version                string             `json:"version,omitempty"`
	Range                  string             `json:"range"`
	From                   time.Time          `json:"from"`
	To                     time.Time          `json:"to"`
	SampleSize             int                `json:"sample_size"`
	BaselineSampleSize     int                `json:"baseline_sample_size"`
	CandidateSampleSize    int                `json:"candidate_sample_size"`
	SampleSizeTooSmall     bool               `json:"sample_size_too_small"`
	ParentFillGain         *float64           `json:"parent_fill_gain,omitempty"`
	RewriteGain            *float64           `json:"rewrite_gain,omitempty"`
	RouteContribution      map[string]float64 `json:"route_contribution,omitempty"`
	EvidenceRefusalRate    *float64           `json:"evidence_refusal_rate,omitempty"`
	RefusalFalsePositive   *float64           `json:"refusal_false_positive_rate,omitempty"`
	CitationSupportScore   *float64           `json:"citation_support_score,omitempty"`
	CitationPrecisionDelta *float64           `json:"citation_precision_delta,omitempty"`
	P95LatencyDeltaMS      *float64           `json:"p95_latency_delta_ms,omitempty"`
	AvgContextTokensDelta  *float64           `json:"avg_context_tokens_delta,omitempty"`
	EmptyRateDelta         *float64           `json:"empty_rate_delta,omitempty"`
	ErrorRateDelta         *float64           `json:"error_rate_delta,omitempty"`
	ContractGaps           []string           `json:"contract_gaps,omitempty"`
}

type strategyGateSummaryResponse struct {
	FlagKey           string   `json:"flag_key,omitempty"`
	Version           string   `json:"version,omitempty"`
	GateStatus        string   `json:"gate_status"`
	Passed            bool     `json:"passed"`
	FailedRules       []string `json:"failed_rules,omitempty"`
	BaselineReportID  string   `json:"baseline_report_id,omitempty"`
	CandidateReportID string   `json:"candidate_report_id,omitempty"`
	LastEvalRunID     string   `json:"last_eval_run_id,omitempty"`
	ContractGaps      []string `json:"contract_gaps,omitempty"`
}

type strategyOperationListResponse struct {
	Items    []phase3admin.OperationRecord `json:"items"`
	Total    int                           `json:"total"`
	Page     int                           `json:"page"`
	PageSize int                           `json:"page_size"`
}

type strategyEvalContext struct {
	Run              *model.KBEvalRun
	Report           *evaluation.Report
	BaselineResult   *evaluation.StrategyResult
	CandidateResult  *evaluation.StrategyResult
	BaselineProfile  evaluation.StrategyProfile
	CandidateProfile evaluation.StrategyProfile
}

var (
	listStrategyRetrieveLogs = func(startTime, endTime time.Time, kbID *uint64) ([]*model.KBRetrieveLog, error) {
		return model.KBRetrieveLogDao.ListByCreatedAt(startTime, endTime, kbID)
	}
	listStrategyEvalRuns = func(filter model.KBEvalRunListFilter) ([]*model.KBEvalRun, int64, error) {
		return model.KBEvalRunDao.ListWithFilter(filter)
	}
)

func GetStrategyImpact(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	flagKey, versionID, err := resolveStrategySelection(c)
	if err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}

	rangeLabel, from, to, err := parseStrategyRange(strings.TrimSpace(string(c.Query("range"))))
	if err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}

	var kbID *uint64
	if raw := strings.TrimSpace(string(c.Query("kb_id"))); raw != "" {
		parsed, parseErr := parseUint64(raw, "kb_id")
		if parseErr != nil {
			response.BadRequest(ctx, c, parseErr.Error())
			return
		}
		kbID = &parsed
	}

	logs, err := listStrategyRetrieveLogs(from, to, kbID)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to load strategy impact logs", err))
		return
	}

	baselineLogs, candidateLogs := partitionStrategyLogs(logs, flagKey)
	evalCtx, evalErr := findLatestRelevantEvalContext(flagKey)
	if evalErr != nil {
		response.ErrorFromErr(ctx, c, evalErr)
		return
	}

	resp := strategyImpactResponse{
		FlagKey:             flagKey,
		Version:             versionID,
		Range:               rangeLabel,
		From:                from,
		To:                  to,
		SampleSize:          len(logs),
		BaselineSampleSize:  len(baselineLogs),
		CandidateSampleSize: len(candidateLogs),
		RouteContribution:   map[string]float64{},
	}
	contractGaps := make([]string, 0, 8)

	if evalCtx != nil {
		if resp.BaselineSampleSize == 0 {
			resp.BaselineSampleSize = len(evalCtx.BaselineResult.Queries)
		}
		if resp.CandidateSampleSize == 0 {
			resp.CandidateSampleSize = len(evalCtx.CandidateResult.Queries)
		}
		if resp.SampleSize == 0 {
			resp.SampleSize = resp.BaselineSampleSize + resp.CandidateSampleSize
		}
		resp.ParentFillGain = float64Ptr(evalCtx.Report.Comparison.ParentFillGainDelta)
		resp.RewriteGain = float64Ptr(evalCtx.Report.Comparison.RewriteGainDelta)
		resp.RefusalFalsePositive = float64Ptr(evalCtx.Report.Comparison.RefusalFalsePositiveRate)
		resp.CitationPrecisionDelta = float64Ptr(evalCtx.Report.Comparison.CitationPrecisionDelta)
		resp.P95LatencyDeltaMS = float64Ptr(evalCtx.Report.Comparison.P95LatencyDeltaMS)
	}

	if len(candidateLogs) > 0 {
		resp.EvidenceRefusalRate = float64Ptr(strategyRate(candidateLogs, isEvidenceRefusalLog))
		resp.CitationSupportScore = float64Ptr(strategyAverageFloat(candidateLogs, func(logEntry *model.KBRetrieveLog) float64 {
			return logEntry.CitationSupportScore
		}))
	} else {
		contractGaps = append(contractGaps, "candidate_logs")
	}

	if len(candidateLogs) > 0 && len(baselineLogs) > 0 {
		if resp.ParentFillGain == nil {
			resp.ParentFillGain = float64Ptr(
				strategyAverageInt(candidateLogs, func(logEntry *model.KBRetrieveLog) int {
					return logEntry.ParentFillCount
				}) - strategyAverageInt(baselineLogs, func(logEntry *model.KBRetrieveLog) int {
					return logEntry.ParentFillCount
				}),
			)
		}
		if resp.RewriteGain == nil {
			resp.RewriteGain = float64Ptr(strategyRewriteGain(candidateLogs) - strategyRewriteGain(baselineLogs))
		}
		resp.RouteContribution = map[string]float64{
			"dense": strategyAverageInt(candidateLogs, func(logEntry *model.KBRetrieveLog) int {
				return logEntry.DenseContribution
			}) - strategyAverageInt(baselineLogs, func(logEntry *model.KBRetrieveLog) int {
				return logEntry.DenseContribution
			}),
			"sparse": strategyAverageInt(candidateLogs, func(logEntry *model.KBRetrieveLog) int {
				return logEntry.SparseContribution
			}) - strategyAverageInt(baselineLogs, func(logEntry *model.KBRetrieveLog) int {
				return logEntry.SparseContribution
			}),
		}
		if resp.P95LatencyDeltaMS == nil {
			resp.P95LatencyDeltaMS = float64Ptr(
				float64(strategyP95Latency(candidateLogs) - strategyP95Latency(baselineLogs)),
			)
		}
		resp.EmptyRateDelta = float64Ptr(strategyRate(candidateLogs, isEmptyResultLog) - strategyRate(baselineLogs, isEmptyResultLog))
		resp.ErrorRateDelta = float64Ptr(strategyRate(candidateLogs, isErrorResultLog) - strategyRate(baselineLogs, isErrorResultLog))
	} else {
		contractGaps = append(contractGaps,
			"baseline_candidate_comparison",
			"route_contribution",
			"empty_rate_delta",
			"error_rate_delta",
		)
		if resp.ParentFillGain == nil {
			contractGaps = append(contractGaps, "parent_fill_gain")
		}
		if resp.RewriteGain == nil {
			contractGaps = append(contractGaps, "rewrite_gain")
		}
		if resp.P95LatencyDeltaMS == nil {
			contractGaps = append(contractGaps, "p95_latency_delta_ms")
		}
		resp.RouteContribution = nil
	}

	if resp.RefusalFalsePositive == nil {
		contractGaps = append(contractGaps, "refusal_false_positive_rate")
	}
	if resp.CitationPrecisionDelta == nil {
		contractGaps = append(contractGaps, "citation_precision_delta")
	}
	contractGaps = append(contractGaps, "avg_context_tokens_delta")
	if evalCtx == nil {
		contractGaps = append(contractGaps, "eval_report")
	}

	resp.SampleSizeTooSmall = strategySampleTooSmall(resp.BaselineSampleSize, resp.CandidateSampleSize, flagKey != "")
	resp.ContractGaps = uniqueSortedStrings(contractGaps)
	response.Success(ctx, c, resp)
}

func GetStrategyGates(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	flagKey, versionID, err := resolveStrategySelection(c)
	if err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}

	evalCtx, evalErr := findLatestRelevantEvalContext(flagKey)
	if evalErr != nil {
		response.ErrorFromErr(ctx, c, evalErr)
		return
	}

	resp := strategyGateSummaryResponse{
		FlagKey:    flagKey,
		Version:    versionID,
		GateStatus: strategyGateStatusMissingReport,
	}

	if versionID != "" {
		if record, ok := phase3admin.GetVersion(&config.Global, versionID); ok && strings.TrimSpace(record.GateStatus) != "" {
			resp.GateStatus = strings.TrimSpace(record.GateStatus)
		}
	}

	if evalCtx == nil {
		resp.ContractGaps = []string{"eval_report"}
		response.Success(ctx, c, resp)
		return
	}

	resp.Passed = evalCtx.Report.Gate.Passed
	resp.GateStatus = strategyGateStatusFailed
	if evalCtx.Report.Gate.Passed {
		resp.GateStatus = strategyGateStatusPassed
	}
	resp.LastEvalRunID = evalCtx.Run.RunID
	resp.BaselineReportID = buildStrategyReportID(evalCtx.Run.RunID, evalCtx.Report.Baseline)
	resp.CandidateReportID = buildStrategyReportID(evalCtx.Run.RunID, evalCtx.Report.Candidate)
	for _, check := range evalCtx.Report.Gate.Checks {
		if !check.Passed {
			resp.FailedRules = append(resp.FailedRules, check.Name)
		}
	}
	response.Success(ctx, c, resp)
}

func ListStrategyOperations(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	page, pageSize := getPagination(c)
	flagKey := strings.TrimSpace(string(c.Query("flag_key")))
	if flagKey != "" && !phase3.IsManagedFeatureFlag(flagKey) {
		response.ErrorFromErr(ctx, c, myerrors.NewValidationError("unsupported flag_key: "+flagKey))
		return
	}

	items := phase3admin.ListOperations(&config.Global, flagKey)
	total := len(items)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	response.Success(ctx, c, strategyOperationListResponse{
		Items:    items[start:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func resolveStrategySelection(c *app.RequestContext) (string, string, error) {
	flagKey := strings.TrimSpace(string(c.Query("flag_key")))
	versionID := strings.TrimSpace(string(c.Query("version")))

	if flagKey != "" && !phase3.IsManagedFeatureFlag(flagKey) {
		return "", "", myerrors.NewValidationError("unsupported flag_key: " + flagKey)
	}
	if versionID == "" {
		return flagKey, versionID, nil
	}

	record, ok := phase3admin.GetVersion(&config.Global, versionID)
	if !ok {
		return "", "", myerrors.NewNotFoundError("strategy version")
	}
	if flagKey != "" && flagKey != record.FlagKey {
		return "", "", myerrors.NewValidationError(fmt.Sprintf("version %s does not belong to flag %s", versionID, flagKey))
	}
	return record.FlagKey, versionID, nil
}

func parseStrategyRange(raw string) (string, time.Time, time.Time, error) {
	label := strings.TrimSpace(strings.ToLower(raw))
	if label == "" {
		label = defaultStrategyImpactRange
	}

	now := time.Now().UTC()
	switch label {
	case "1h":
		return label, now.Add(-1 * time.Hour), now, nil
	case "24h":
		return label, now.Add(-24 * time.Hour), now, nil
	case "7d":
		return label, now.Add(-7 * 24 * time.Hour), now, nil
	default:
		return "", time.Time{}, time.Time{}, myerrors.NewValidationError("range must be one of 1h, 24h, 7d")
	}
}

func findLatestRelevantEvalContext(flagKey string) (*strategyEvalContext, error) {
	status := model.KBEvalRunStatusSucceeded
	runs, _, err := listStrategyEvalRuns(model.KBEvalRunListFilter{
		Status:   &status,
		Page:     1,
		PageSize: maxStrategyEvalRunSearchPageSize,
	})
	if err != nil {
		return nil, myerrors.NewDBError("failed to list evaluation runs", err)
	}

	for _, run := range runs {
		evalCtx, buildErr := buildStrategyEvalContext(run, flagKey)
		if buildErr != nil || evalCtx == nil {
			continue
		}
		return evalCtx, nil
	}
	return nil, nil
}

func buildStrategyEvalContext(run *model.KBEvalRun, flagKey string) (*strategyEvalContext, error) {
	if run == nil || strings.TrimSpace(run.ReportPath) == "" {
		return nil, nil
	}

	report, err := loadEvalReport(run.ReportPath)
	if err != nil {
		return nil, err
	}
	baselineResult, candidateResult, err := resolveBaselineCandidateResults(report)
	if err != nil {
		return nil, err
	}

	baselineProfile, baselineOK := strategyProfileByName([]evaluation.StrategyProfile(run.Profiles), run.BaselineProfile)
	if !baselineOK {
		baselineProfile = baselineResult.Strategy
	}
	candidateProfile, candidateOK := strategyProfileByName([]evaluation.StrategyProfile(run.Profiles), run.CandidateProfile)
	if !candidateOK {
		candidateProfile = candidateResult.Strategy
	}

	if flagKey != "" && !strategyProfileChangesFlag(baselineProfile, candidateProfile, flagKey) {
		return nil, nil
	}

	return &strategyEvalContext{
		Run:              run,
		Report:           report,
		BaselineResult:   baselineResult,
		CandidateResult:  candidateResult,
		BaselineProfile:  baselineProfile,
		CandidateProfile: candidateProfile,
	}, nil
}

func strategyProfileByName(profiles []evaluation.StrategyProfile, name string) (evaluation.StrategyProfile, bool) {
	for _, profile := range profiles {
		if strings.EqualFold(strings.TrimSpace(profile.Name), strings.TrimSpace(name)) {
			return profile, true
		}
	}
	return evaluation.StrategyProfile{}, false
}

func strategyProfileChangesFlag(baseline, candidate evaluation.StrategyProfile, flagKey string) bool {
	return strategyProfileFlagEnabled(candidate, flagKey) != strategyProfileFlagEnabled(baseline, flagKey)
}

func strategyProfileFlagEnabled(profile evaluation.StrategyProfile, flagKey string) bool {
	switch flagKey {
	case phase3.FlagParentChildRetrieval:
		return profile.EnableParentChildRetrieval
	case phase3.FlagStrategicTopK:
		return profile.EnableStrategicTopK
	case phase3.FlagEvidenceRefusal:
		return profile.EnableEvidenceRefusal
	case phase3.FlagCitationConsistency:
		return profile.EnableCitationConsistency
	case phase3.FlagDomainTerms:
		return profile.EnableDomainTerms
	case phase3.FlagRouteSpecificRewrite:
		return profile.EnableRouteSpecificRewrite
	case phase3.FlagModelAssistedRewrite:
		return profile.EnableModelAssistedRewrite
	default:
		return false
	}
}

func partitionStrategyLogs(logs []*model.KBRetrieveLog, flagKey string) ([]*model.KBRetrieveLog, []*model.KBRetrieveLog) {
	if flagKey == "" {
		return nil, cloneRetrieveLogs(logs)
	}

	baseline := make([]*model.KBRetrieveLog, 0, len(logs))
	candidate := make([]*model.KBRetrieveLog, 0, len(logs))
	for _, logEntry := range logs {
		if logEntry == nil {
			continue
		}
		if strategyCandidateLog(flagKey, logEntry) {
			candidate = append(candidate, logEntry)
			continue
		}
		baseline = append(baseline, logEntry)
	}
	return baseline, candidate
}

func strategyCandidateLog(flagKey string, logEntry *model.KBRetrieveLog) bool {
	if logEntry == nil {
		return false
	}

	rewriteStrategy := strings.ToLower(strings.TrimSpace(logEntry.RewriteStrategy))
	rewriteBucket := strings.ToLower(strings.TrimSpace(logEntry.RewriteGainBucket))
	switch flagKey {
	case phase3.FlagParentChildRetrieval:
		return logEntry.ParentChildEnabled
	case phase3.FlagStrategicTopK:
		return strings.Contains(strings.ToLower(logEntry.TopKDecisionReason), "strategic") ||
			strings.Contains(strings.ToLower(logEntry.Strategy), "phase3")
	case phase3.FlagEvidenceRefusal:
		return strings.TrimSpace(logEntry.EvidenceGateResult) != "" &&
			!strings.EqualFold(strings.TrimSpace(logEntry.EvidenceGateResult), "disabled")
	case phase3.FlagCitationConsistency:
		return strings.TrimSpace(logEntry.CitationCheckVersion) != ""
	case phase3.FlagDomainTerms:
		return strings.Contains(rewriteStrategy, "domain_terms")
	case phase3.FlagRouteSpecificRewrite:
		return strings.Contains(rewriteStrategy, "route_specific")
	case phase3.FlagModelAssistedRewrite:
		return strings.Contains(rewriteStrategy, "model_assisted_shadow") ||
			strings.Contains(rewriteBucket, "model_gain_candidate")
	default:
		return false
	}
}

func cloneRetrieveLogs(logs []*model.KBRetrieveLog) []*model.KBRetrieveLog {
	cloned := make([]*model.KBRetrieveLog, 0, len(logs))
	for _, logEntry := range logs {
		if logEntry != nil {
			cloned = append(cloned, logEntry)
		}
	}
	return cloned
}

func strategyRewriteGain(logs []*model.KBRetrieveLog) float64 {
	return strategyRate(logs, func(logEntry *model.KBRetrieveLog) bool {
		bucket := strings.ToLower(strings.TrimSpace(logEntry.RewriteGainBucket))
		return bucket == "gain_candidate" || bucket == "route_gain_candidate" || bucket == "model_gain_candidate"
	})
}

func strategyRate(logs []*model.KBRetrieveLog, matched func(*model.KBRetrieveLog) bool) float64 {
	if len(logs) == 0 {
		return 0
	}
	total := 0
	hits := 0
	for _, logEntry := range logs {
		if logEntry == nil {
			continue
		}
		total++
		if matched(logEntry) {
			hits++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

func strategyAverageFloat(logs []*model.KBRetrieveLog, value func(*model.KBRetrieveLog) float64) float64 {
	total := 0.0
	count := 0
	for _, logEntry := range logs {
		if logEntry == nil {
			continue
		}
		total += value(logEntry)
		count++
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

func strategyAverageInt(logs []*model.KBRetrieveLog, value func(*model.KBRetrieveLog) int) float64 {
	total := 0.0
	count := 0
	for _, logEntry := range logs {
		if logEntry == nil {
			continue
		}
		total += float64(value(logEntry))
		count++
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

func strategyP95Latency(logs []*model.KBRetrieveLog) int64 {
	values := make([]int64, 0, len(logs))
	for _, logEntry := range logs {
		if logEntry == nil || logEntry.DurationMs <= 0 {
			continue
		}
		values = append(values, logEntry.DurationMs)
	}
	return percentileInt64(values, 0.95)
}

func strategySampleTooSmall(baselineSize, candidateSize int, requiresComparison bool) bool {
	if requiresComparison {
		return baselineSize < minStrategyComparisonSampleSize || candidateSize < minStrategyComparisonSampleSize
	}
	return candidateSize < minStrategyComparisonSampleSize
}

func isEvidenceRefusalLog(logEntry *model.KBRetrieveLog) bool {
	return logEntry != nil && strings.EqualFold(strings.TrimSpace(logEntry.EvidenceGateResult), "refused")
}

func isEmptyResultLog(logEntry *model.KBRetrieveLog) bool {
	if logEntry == nil {
		return false
	}
	return logEntry.ResultStatus == model.RetrieveResultStatusNoResult ||
		(logEntry.ResultStatus == model.RetrieveResultStatusFilteredOut && logEntry.FinalCount == 0)
}

func isErrorResultLog(logEntry *model.KBRetrieveLog) bool {
	if logEntry == nil {
		return false
	}
	return logEntry.ResultStatus == model.RetrieveResultStatusError ||
		logEntry.ResultStatus == model.RetrieveResultStatusTimeout
}

func buildStrategyReportID(runID, profileName string) string {
	return strings.TrimSpace(runID) + ":" + strings.TrimSpace(profileName)
}

func uniqueSortedStrings(items []string) []string {
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
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	if len(result) == 0 {
		return nil
	}
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if strings.Compare(result[j], result[i]) < 0 {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

func float64Ptr(value float64) *float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return nil
	}
	return &value
}
