package kb

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"interview-agents/internal/milvus/evaluation"
	"interview-agents/internal/model"
	"interview-agents/internal/rag/phase3"

	ut "github.com/cloudwego/hertz/pkg/common/ut"
)

func TestStrategyImpactAndGatesEndpoints(t *testing.T) {
	resetStrategyHandlerState(t)
	seedStrategyInsightsFixtures(t)

	h := newAdminStrategyTestServer()

	impactResp := ut.PerformRequest(
		h.Engine,
		http.MethodGet,
		"/api/admin/kb/strategy/impact?flag_key="+phase3.FlagModelAssistedRewrite+"&range=24h",
		nil,
	).Result()
	if impactResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected impact 200, got %d body=%s", impactResp.StatusCode(), string(impactResp.Body()))
	}

	var impactPayload struct {
		Code int                    `json:"code"`
		Data strategyImpactResponse `json:"data"`
	}
	decodeJSONResponse(t, impactResp.Body(), &impactPayload)
	if impactPayload.Data.SampleSize != 12 || impactPayload.Data.BaselineSampleSize != 6 || impactPayload.Data.CandidateSampleSize != 6 {
		t.Fatalf("unexpected sample sizes: %+v", impactPayload.Data)
	}
	if impactPayload.Data.SampleSizeTooSmall {
		t.Fatalf("expected sample_size_too_small=false, got %+v", impactPayload.Data)
	}
	if impactPayload.Data.ParentFillGain == nil || *impactPayload.Data.ParentFillGain != 0.4 {
		t.Fatalf("parent_fill_gain = %#v, want 0.4", impactPayload.Data.ParentFillGain)
	}
	if impactPayload.Data.RewriteGain == nil || *impactPayload.Data.RewriteGain != 0.2 {
		t.Fatalf("rewrite_gain = %#v, want 0.2", impactPayload.Data.RewriteGain)
	}
	if impactPayload.Data.CitationPrecisionDelta == nil || *impactPayload.Data.CitationPrecisionDelta != 0.1 {
		t.Fatalf("citation_precision_delta = %#v, want 0.1", impactPayload.Data.CitationPrecisionDelta)
	}
	if impactPayload.Data.P95LatencyDeltaMS == nil || *impactPayload.Data.P95LatencyDeltaMS != 50 {
		t.Fatalf("p95_latency_delta_ms = %#v, want 50", impactPayload.Data.P95LatencyDeltaMS)
	}
	if impactPayload.Data.EvidenceRefusalRate == nil || *impactPayload.Data.EvidenceRefusalRate <= 0 {
		t.Fatalf("evidence_refusal_rate = %#v, want > 0", impactPayload.Data.EvidenceRefusalRate)
	}
	if impactPayload.Data.CitationSupportScore == nil || *impactPayload.Data.CitationSupportScore <= 0.7 {
		t.Fatalf("citation_support_score = %#v, want > 0.7", impactPayload.Data.CitationSupportScore)
	}
	if impactPayload.Data.RouteContribution["sparse"] <= 0 {
		t.Fatalf("route contribution = %#v, want sparse > 0", impactPayload.Data.RouteContribution)
	}
	if !stringSliceContains(impactPayload.Data.ContractGaps, "avg_context_tokens_delta") {
		t.Fatalf("contract_gaps = %#v, want avg_context_tokens_delta", impactPayload.Data.ContractGaps)
	}

	gatesResp := ut.PerformRequest(
		h.Engine,
		http.MethodGet,
		"/api/admin/kb/strategy/gates?flag_key="+phase3.FlagModelAssistedRewrite,
		nil,
	).Result()
	if gatesResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected gates 200, got %d body=%s", gatesResp.StatusCode(), string(gatesResp.Body()))
	}

	var gatesPayload struct {
		Code int                         `json:"code"`
		Data strategyGateSummaryResponse `json:"data"`
	}
	decodeJSONResponse(t, gatesResp.Body(), &gatesPayload)
	if gatesPayload.Data.GateStatus != strategyGateStatusFailed || gatesPayload.Data.Passed {
		t.Fatalf("unexpected gate payload: %+v", gatesPayload.Data)
	}
	if !stringSliceContains(gatesPayload.Data.FailedRules, "p95_latency_regression_ms") {
		t.Fatalf("failed_rules = %#v, want p95_latency_regression_ms", gatesPayload.Data.FailedRules)
	}
	if gatesPayload.Data.LastEvalRunID == "" || !strings.Contains(gatesPayload.Data.CandidateReportID, gatesPayload.Data.LastEvalRunID) {
		t.Fatalf("unexpected gate identifiers: %+v", gatesPayload.Data)
	}
}

func TestStrategyOperationsEndpointReturnsLatestChanges(t *testing.T) {
	resetStrategyHandlerState(t)

	h := newAdminStrategyTestServer()

	updateResp := performJSONRequest(t, h, http.MethodPatch, "/api/admin/kb/strategy/flags/"+phase3.FlagModelAssistedRewrite, map[string]interface{}{
		"enabled":            true,
		"status":             phase3.StatusShadow,
		"rollout_percentage": 15,
		"reason":             "ops shadow rollout",
	})
	if updateResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected update 200, got %d body=%s", updateResp.StatusCode(), string(updateResp.Body()))
	}

	rollbackResp := performJSONRequest(t, h, http.MethodPost, "/api/admin/kb/strategy/rollback", map[string]interface{}{
		"target_version": phase3.StrategyTargetPhase2Baseline,
		"flag_keys":      []string{phase3.FlagModelAssistedRewrite},
		"reason":         "ops rollback",
	})
	if rollbackResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected rollback 200, got %d body=%s", rollbackResp.StatusCode(), string(rollbackResp.Body()))
	}

	operationsResp := ut.PerformRequest(
		h.Engine,
		http.MethodGet,
		"/api/admin/kb/strategy/operations?flag_key="+phase3.FlagModelAssistedRewrite,
		nil,
	).Result()
	if operationsResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected operations 200, got %d body=%s", operationsResp.StatusCode(), string(operationsResp.Body()))
	}

	var operationsPayload struct {
		Code int                           `json:"code"`
		Data strategyOperationListResponse `json:"data"`
	}
	decodeJSONResponse(t, operationsResp.Body(), &operationsPayload)
	if operationsPayload.Data.Total < 2 || len(operationsPayload.Data.Items) < 2 {
		t.Fatalf("operations payload too small: %+v", operationsPayload.Data)
	}
	if operationsPayload.Data.Items[0].Operation != "rollback" || operationsPayload.Data.Items[0].Reason != "ops rollback" {
		t.Fatalf("latest operation = %+v, want rollback", operationsPayload.Data.Items[0])
	}
	if operationsPayload.Data.Items[1].Operation != "update_flag" || operationsPayload.Data.Items[1].Reason != "ops shadow rollout" {
		t.Fatalf("second operation = %+v, want update_flag", operationsPayload.Data.Items[1])
	}
}

func setupStrategyInsightStubs(t *testing.T, runs []*model.KBEvalRun, logs []*model.KBRetrieveLog) {
	t.Helper()

	originalRuns := listStrategyEvalRuns
	originalLogs := listStrategyRetrieveLogs
	listStrategyEvalRuns = func(filter model.KBEvalRunListFilter) ([]*model.KBEvalRun, int64, error) {
		return runs, int64(len(runs)), nil
	}
	listStrategyRetrieveLogs = func(startTime, endTime time.Time, kbID *uint64) ([]*model.KBRetrieveLog, error) {
		filtered := make([]*model.KBRetrieveLog, 0, len(logs))
		for _, item := range logs {
			if item == nil {
				continue
			}
			if item.CreatedAt.Before(startTime) || item.CreatedAt.After(endTime) {
				continue
			}
			filtered = append(filtered, item)
		}
		return filtered, nil
	}
	t.Cleanup(func() {
		listStrategyEvalRuns = originalRuns
		listStrategyRetrieveLogs = originalLogs
	})
}

func seedStrategyInsightsFixtures(t *testing.T) {
	t.Helper()

	profiles := []evaluation.StrategyProfile{
		{
			Name:                 "phase2_baseline",
			Baseline:             true,
			Mode:                 "hybrid",
			EnableQueryRewrite:   true,
			EnableDynamicTopK:    true,
			EnableAdvancedRerank: true,
		},
		{
			Name:                       "parent_child+advanced_rewrite",
			Candidate:                  true,
			Mode:                       "hybrid",
			EnableQueryRewrite:         true,
			EnableDynamicTopK:          true,
			EnableAdvancedRerank:       true,
			EnableParentChildRetrieval: true,
			EnableDomainTerms:          true,
			EnableRouteSpecificRewrite: true,
			EnableModelAssistedRewrite: true,
		},
	}
	report := evaluation.Report{
		GeneratedAt: time.Now().UTC(),
		Results: []evaluation.StrategyResult{
			{
				Strategy: profiles[0],
				Queries:  buildEvalQueries("baseline", 6),
			},
			{
				Strategy: profiles[1],
				Queries:  buildEvalQueries("candidate", 6),
			},
		},
		Comparison: evaluation.ComparisonSummary{
			Baseline:                 profiles[0].Name,
			Candidate:                profiles[1].Name,
			ParentFillGainDelta:      0.4,
			RewriteGainDelta:         0.2,
			CitationPrecisionDelta:   0.1,
			P95LatencyDeltaMS:        50,
			RefusalFalsePositiveRate: 0.03,
			CandidateModelRewrite:    true,
		},
		Gate: evaluation.GateResult{
			Passed: false,
			Checks: []evaluation.GateCheck{
				{Name: "rewrite_gain_delta", Passed: true},
				{Name: "p95_latency_regression_ms", Passed: false},
			},
		},
		Baseline:  profiles[0].Name,
		Candidate: profiles[1].Name,
	}
	reportPath := writeStrategyReportFixture(t, report)

	run := &model.KBEvalRun{
		RunID:            "run-strategy-l3",
		DatasetID:        1,
		BaselineProfile:  profiles[0].Name,
		CandidateProfile: profiles[1].Name,
		Profiles:         model.EvalStrategyProfileList(profiles),
		GateThresholds:   model.EvalGateThresholds(evaluation.DefaultGateThresholds()),
		Status:           model.KBEvalRunStatusSucceeded,
		ReportPath:       reportPath,
		CreatedBy:        7,
	}
	now := time.Now().UTC()
	logs := make([]*model.KBRetrieveLog, 0, 12)
	for i := 0; i < 6; i++ {
		logEntry := &model.KBRetrieveLog{
			RequestID:            fmtStrategyRequestID("baseline", i),
			UserID:               7,
			KBIDs:                "1",
			Query:                "baseline query",
			RewriteStrategy:      "rule_based",
			RewriteApplied:       true,
			RewriteGainBucket:    "gain_candidate",
			DenseContribution:    1,
			SparseContribution:   1,
			ParentFillCount:      0,
			EvidenceGateResult:   "disabled",
			CitationSupportScore: 0.45,
			ResultStatus:         model.RetrieveResultStatusSuccess,
			DurationMs:           120 + int64(i),
			CreatedAt:            now.Add(-time.Duration(i+1) * time.Hour),
		}
		logs = append(logs, logEntry)
	}
	candidateStatuses := []model.RetrieveResultStatus{
		model.RetrieveResultStatusSuccess,
		model.RetrieveResultStatusSuccess,
		model.RetrieveResultStatusFilteredOut,
		model.RetrieveResultStatusSuccess,
		model.RetrieveResultStatusNoResult,
		model.RetrieveResultStatusError,
	}
	candidateGateResults := []string{"pass", "pass", "refused", "pass", "pass", "pass"}
	for i := 0; i < 6; i++ {
		logEntry := &model.KBRetrieveLog{
			RequestID:            fmtStrategyRequestID("candidate", i),
			UserID:               7,
			KBIDs:                "1",
			Query:                "candidate query",
			RewriteStrategy:      "rule_based+model_assisted_shadow",
			RewriteApplied:       true,
			RewriteGainBucket:    "model_gain_candidate",
			DenseContribution:    1,
			SparseContribution:   3,
			ParentChildEnabled:   true,
			ParentFillCount:      2,
			EvidenceGateResult:   candidateGateResults[i],
			CitationSupportScore: 0.82,
			ResultStatus:         candidateStatuses[i],
			DurationMs:           170 + int64(i*5),
			CreatedAt:            now.Add(-time.Duration(i+1) * 30 * time.Minute),
		}
		logs = append(logs, logEntry)
	}
	setupStrategyInsightStubs(t, []*model.KBEvalRun{run}, logs)
}

func buildEvalQueries(prefix string, count int) []evaluation.QueryMetrics {
	queries := make([]evaluation.QueryMetrics, 0, count)
	for i := 0; i < count; i++ {
		queries = append(queries, evaluation.QueryMetrics{
			QueryID:           fmtStrategyRequestID(prefix, i),
			Query:             prefix + " query",
			CitationPrecision: 0.7,
		})
	}
	return queries
}

func writeStrategyReportFixture(t *testing.T, report evaluation.Report) string {
	t.Helper()

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("failed to marshal report: %v", err)
	}
	path := filepath.Join(t.TempDir(), "strategy-report.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("failed to write report fixture: %v", err)
	}
	return path
}

func fmtStrategyRequestID(prefix string, index int) string {
	return prefix + "-" + strconv.Itoa(index)
}
