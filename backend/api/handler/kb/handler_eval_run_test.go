package kb

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"interview-agents/internal/milvus/evaluation"
	"interview-agents/internal/model"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
)

type fakeEvalRunExecutor struct {
	report *evaluation.Report
	err    error
}

func (f *fakeEvalRunExecutor) Execute(ctx context.Context, run *model.KBEvalRun, dataset []evaluation.DatasetCase) (*evaluation.Report, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.report != nil {
		return f.report, nil
	}
	return &evaluation.Report{
		DatasetSize: len(dataset),
		GeneratedAt: time.Now(),
		Results: []evaluation.StrategyResult{
			{
				Strategy: evaluation.StrategyProfile{Name: run.BaselineProfile, Baseline: true, Mode: "dense"},
				Metrics:  evaluation.AggregateMetrics{RecallAtK: 0.5, MRR: 0.4, NDCG: 0.45, CitationAccuracy: 0.6, P50LatencyMS: 10, P95LatencyMS: 20},
			},
			{
				Strategy: evaluation.StrategyProfile{Name: run.CandidateProfile, Candidate: true, Mode: "hybrid"},
				Metrics:  evaluation.AggregateMetrics{RecallAtK: 0.6, MRR: 0.5, NDCG: 0.55, CitationAccuracy: 0.7, P50LatencyMS: 12, P95LatencyMS: 24},
			},
		},
		Comparison: evaluation.ComparisonSummary{
			Baseline:              run.BaselineProfile,
			Candidate:             run.CandidateProfile,
			RecallDelta:           0.1,
			MRRDelta:              0.1,
			NDCGDelta:             0.1,
			CitationAccuracyDelta: 0.1,
			P95LatencyDeltaMS:     4,
			P95LatencyDeltaRatio:  0.2,
		},
		Gate: evaluation.GateResult{
			Passed: true,
			Checks: []evaluation.GateCheck{},
		},
		Baseline:  run.BaselineProfile,
		Candidate: run.CandidateProfile,
	}, nil
}

func TestEvalRunLifecycleSuccess(t *testing.T) {
	setupEvalDatasetTestDB(t)
	dataset := seedReadyEvalDataset(t)

	oldExecutor := evalRunExecutor
	evalRunExecutor = &fakeEvalRunExecutor{}
	defer func() { evalRunExecutor = oldExecutor }()

	h := newAdminEvalRunTestServer()
	resp := performJSONRequest(t, h, http.MethodPost, "/api/admin/kb/eval/runs", map[string]interface{}{
		"dataset_id":        dataset.ID,
		"baseline_profile":  "phase1_baseline",
		"candidate_profile": "phase2_hybrid_rewrite_topk",
		"profiles": []map[string]interface{}{
			{"name": "phase1_baseline", "baseline": true, "mode": "dense"},
			{"name": "phase2_hybrid_rewrite_topk", "candidate": true, "mode": "hybrid", "enable_query_rewrite": true},
		},
		"gate_thresholds": map[string]interface{}{
			"min_recall_delta":                 0.08,
			"max_p95_latency_regression_ratio": 0.2,
		},
	})
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("expected create run 200, got %d", resp.StatusCode())
	}

	var payload struct {
		Code int `json:"code"`
		Data struct {
			RunID  string `json:"run_id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	decodeJSONResponse(t, resp.Body(), &payload)
	if payload.Code != 200 || payload.Data.RunID == "" {
		t.Fatalf("unexpected create run payload: %+v", payload)
	}

	run := waitForEvalRunStatus(t, h, payload.Data.RunID, 5*time.Second)
	t.Cleanup(func() {
		if run.ReportPath != "" {
			_ = os.Remove(run.ReportPath)
		}
	})
	if run.Status != string(model.KBEvalRunStatusSucceeded) {
		t.Fatalf("expected succeeded run, got %+v", run)
	}
	if run.ReportPath == "" {
		t.Fatalf("expected report_path to be persisted, got %+v", run)
	}
}

func TestEvalRunLifecycleFailureAndDraftGuard(t *testing.T) {
	setupEvalDatasetTestDB(t)
	draftDataset := &model.KBEvalDataset{
		Name:        "draft-dataset",
		Description: "draft dataset",
		Status:      model.KBEvalDatasetStatusDraft,
		CreatedBy:   7,
	}
	if err := model.KBEvalDatasetDao.Create(draftDataset); err != nil {
		t.Fatalf("failed to seed draft dataset: %v", err)
	}

	h := newAdminEvalRunTestServer()
	guardResp := performJSONRequest(t, h, http.MethodPost, "/api/admin/kb/eval/runs", map[string]interface{}{
		"dataset_id": draftDataset.ID,
		"profiles": []map[string]interface{}{
			{"name": "dense_only", "baseline": true, "mode": "dense"},
			{"name": "hybrid", "candidate": true, "mode": "hybrid"},
		},
	})
	if guardResp.StatusCode() != http.StatusBadRequest {
		t.Fatalf("expected draft dataset guard 400, got %d", guardResp.StatusCode())
	}

	readyDataset := seedReadyEvalDataset(t)
	oldExecutor := evalRunExecutor
	evalRunExecutor = &fakeEvalRunExecutor{err: errors.New("mock evaluation failure")}
	defer func() { evalRunExecutor = oldExecutor }()

	failResp := performJSONRequest(t, h, http.MethodPost, "/api/admin/kb/eval/runs", map[string]interface{}{
		"dataset_id":        readyDataset.ID,
		"baseline_profile":  "baseline",
		"candidate_profile": "candidate",
		"profiles": []map[string]interface{}{
			{"name": "baseline", "baseline": true, "mode": "dense"},
			{"name": "candidate", "candidate": true, "mode": "hybrid"},
		},
	})
	if failResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected failed run create 200, got %d", failResp.StatusCode())
	}

	var failPayload struct {
		Code int `json:"code"`
		Data struct {
			RunID string `json:"run_id"`
		} `json:"data"`
	}
	decodeJSONResponse(t, failResp.Body(), &failPayload)
	run := waitForEvalRunStatus(t, h, failPayload.Data.RunID, 5*time.Second)
	if run.Status != string(model.KBEvalRunStatusFailed) {
		t.Fatalf("expected failed run, got %+v", run)
	}
	if run.ErrorMsg == "" {
		t.Fatalf("expected error_msg on failed run, got %+v", run)
	}
}

func newAdminEvalRunTestServer() *server.Hertz {
	h := server.Default()
	h.Use(func(ctx context.Context, c *app.RequestContext) {
		c.Set("user_id", uint(7))
		c.Set("role", "admin")
		c.Next(ctx)
	})
	h.GET("/api/admin/kb/eval/runs", ListEvalRuns)
	h.POST("/api/admin/kb/eval/runs", CreateEvalRun)
	h.GET("/api/admin/kb/eval/runs/:run_id", GetEvalRun)
	return h
}

func seedReadyEvalDataset(t *testing.T) *model.KBEvalDataset {
	t.Helper()

	dataset := &model.KBEvalDataset{
		Name:        "ready-dataset-" + time.Now().Format("150405.000"),
		Description: "ready dataset",
		Status:      model.KBEvalDatasetStatusReady,
		CaseCount:   1,
		CreatedBy:   7,
	}
	if err := model.KBEvalDatasetDao.Create(dataset); err != nil {
		t.Fatalf("failed to seed ready dataset: %v", err)
	}

	evalCase := &model.KBEvalCase{
		DatasetID:        dataset.ID,
		CaseKey:          "ready-case",
		Query:            "golang interface nil",
		TopK:             5,
		RelevantIDs:      model.StringList{"chunk-1"},
		ValidationStatus: model.KBEvalCaseValidationStatusValid,
		ValidationErrors: model.StringList{},
	}
	if err := model.KBEvalCaseDao.Create(evalCase); err != nil {
		t.Fatalf("failed to seed ready eval case: %v", err)
	}
	return dataset
}

func waitForEvalRunStatus(t *testing.T, h *server.Hertz, runID string, timeout time.Duration) struct {
	RunID      string `json:"run_id"`
	Status     string `json:"status"`
	ReportPath string `json:"report_path"`
	ErrorMsg   string `json:"error_msg"`
} {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp := ut.PerformRequest(h.Engine, http.MethodGet, "/api/admin/kb/eval/runs/"+runID, nil).Result()
		if resp.StatusCode() != http.StatusOK {
			t.Fatalf("expected get run 200, got %d", resp.StatusCode())
		}

		var payload struct {
			Code int `json:"code"`
			Data struct {
				RunID      string `json:"run_id"`
				Status     string `json:"status"`
				ReportPath string `json:"report_path"`
				ErrorMsg   string `json:"error_msg"`
			} `json:"data"`
		}
		decodeJSONResponse(t, resp.Body(), &payload)
		if payload.Data.Status == string(model.KBEvalRunStatusSucceeded) || payload.Data.Status == string(model.KBEvalRunStatusFailed) {
			return payload.Data
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for run %s", runID)
	return struct {
		RunID      string `json:"run_id"`
		Status     string `json:"status"`
		ReportPath string `json:"report_path"`
		ErrorMsg   string `json:"error_msg"`
	}{}
}
