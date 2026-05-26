package kb

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"interview-agents/internal/milvus/evaluation"
	"interview-agents/internal/model"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
)

type detailedEvalRunExecutor struct{}

func (d *detailedEvalRunExecutor) Execute(ctx context.Context, run *model.KBEvalRun, dataset []evaluation.DatasetCase) (*evaluation.Report, error) {
	queryID := dataset[0].ID
	queryText := dataset[0].Query
	queryTags := dataset[0].Tags
	queryType := dataset[0].QueryType

	return &evaluation.Report{
		DatasetSize: len(dataset),
		GeneratedAt: time.Now(),
		Results: []evaluation.StrategyResult{
			{
				Strategy: evaluation.StrategyProfile{Name: run.BaselineProfile, Baseline: true, Mode: "dense"},
				Metrics:  evaluation.AggregateMetrics{RecallAtK: 1, MRR: 0.8, NDCG: 0.9, CitationAccuracy: 1, P50LatencyMS: 10, P95LatencyMS: 20, AvgLatencyMS: 15},
				Queries: []evaluation.QueryMetrics{
					{
						QueryID:          queryID,
						Query:            queryText,
						QueryType:        queryType,
						Tags:             queryTags,
						TopK:             5,
						Latency:          20 * time.Millisecond,
						RecallAtK:        1,
						MRR:              0.8,
						NDCG:             0.9,
						CitationAccuracy: 1,
						ResultIDs:        []string{"chunk-1"},
						RelevantIDs:      []string{"chunk-1"},
					},
				},
			},
			{
				Strategy: evaluation.StrategyProfile{Name: run.CandidateProfile, Candidate: true, Mode: "hybrid"},
				Metrics:  evaluation.AggregateMetrics{RecallAtK: 0, MRR: 0, NDCG: 0, CitationAccuracy: 0, P50LatencyMS: 12, P95LatencyMS: 25, AvgLatencyMS: 18},
				Queries: []evaluation.QueryMetrics{
					{
						QueryID:          queryID,
						Query:            queryText,
						QueryType:        queryType,
						Tags:             queryTags,
						TopK:             5,
						Latency:          25 * time.Millisecond,
						RecallAtK:        0,
						MRR:              0,
						NDCG:             0,
						CitationAccuracy: 0,
						ResultIDs:        []string{},
						RelevantIDs:      []string{"chunk-1"},
					},
				},
			},
		},
		Contribution: []evaluation.StrategyDelta{
			{
				Strategy:    run.CandidateProfile,
				ComparedTo:  run.BaselineProfile,
				RecallDelta: -1,
				MRRDelta:    -0.8,
				NDCGDelta:   -0.9,
			},
		},
		Comparison: evaluation.ComparisonSummary{
			Baseline:              run.BaselineProfile,
			Candidate:             run.CandidateProfile,
			RecallDelta:           -1,
			MRRDelta:              -0.8,
			NDCGDelta:             -0.9,
			CitationAccuracyDelta: -1,
			P95LatencyDeltaMS:     5,
			P95LatencyDeltaRatio:  0.25,
		},
		Gate: evaluation.GateResult{
			Passed: false,
			Checks: []evaluation.GateCheck{
				{Name: "recall_at_k_delta", Actual: -1, Expected: 0.08, Passed: false, Message: "candidate recall gain must stay above threshold"},
			},
		},
		Baseline:  run.BaselineProfile,
		Candidate: run.CandidateProfile,
	}, nil
}

func TestEvalReportEndpoints(t *testing.T) {
	setupEvalDatasetTestDB(t)
	dataset := seedReadyEvalDataset(t)

	if err := model.KBRetrieveLogDao.Create(&model.KBRetrieveLog{
		RequestID:    buildEvalRequestID("baseline-report", "ready-case"),
		UserID:       7,
		Query:        "golang interface nil",
		ResultStatus: model.RetrieveResultStatusSuccess,
	}); err != nil {
		t.Fatalf("failed to seed baseline retrieve log: %v", err)
	}
	if err := model.KBRetrieveLogDao.Create(&model.KBRetrieveLog{
		RequestID:    buildEvalRequestID("candidate-report", "ready-case"),
		UserID:       7,
		Query:        "golang interface nil",
		ResultStatus: model.RetrieveResultStatusSuccess,
	}); err != nil {
		t.Fatalf("failed to seed candidate retrieve log: %v", err)
	}

	oldExecutor := evalRunExecutor
	evalRunExecutor = &detailedEvalRunExecutor{}
	defer func() { evalRunExecutor = oldExecutor }()

	h := newAdminEvalReportTestServer()
	resp := performJSONRequest(t, h, http.MethodPost, "/api/admin/kb/eval/runs", map[string]interface{}{
		"dataset_id":        dataset.ID,
		"baseline_profile":  "baseline-report",
		"candidate_profile": "candidate-report",
		"profiles": []map[string]interface{}{
			{"name": "baseline-report", "baseline": true, "mode": "dense"},
			{"name": "candidate-report", "candidate": true, "mode": "hybrid"},
		},
	})
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("expected create run 200, got %d", resp.StatusCode())
	}

	var createPayload struct {
		Code int `json:"code"`
		Data struct {
			RunID string `json:"run_id"`
		} `json:"data"`
	}
	decodeJSONResponse(t, resp.Body(), &createPayload)
	run := waitForEvalRunStatus(t, h, createPayload.Data.RunID, 5*time.Second)
	t.Cleanup(func() {
		if run.ReportPath != "" {
			_ = os.Remove(run.ReportPath)
		}
	})

	reportResp := ut.PerformRequest(h.Engine, http.MethodGet, "/api/admin/kb/eval/runs/"+createPayload.Data.RunID+"/report", nil).Result()
	if reportResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected report 200, got %d", reportResp.StatusCode())
	}
	var reportPayload struct {
		Code int               `json:"code"`
		Data evaluation.Report `json:"data"`
	}
	decodeJSONResponse(t, reportResp.Body(), &reportPayload)
	if reportPayload.Code != 200 || reportPayload.Data.Baseline != "baseline-report" || reportPayload.Data.Candidate != "candidate-report" {
		t.Fatalf("unexpected report payload: %+v", reportPayload)
	}

	casesResp := ut.PerformRequest(h.Engine, http.MethodGet, "/api/admin/kb/eval/runs/"+createPayload.Data.RunID+"/cases?failure_reason=recall_miss", nil).Result()
	if casesResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected failure cases 200, got %d", casesResp.StatusCode())
	}
	var casesPayload struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				CaseID             string `json:"case_id"`
				FailureReason      string `json:"failure_reason"`
				BaselineRequestID  string `json:"baseline_request_id"`
				CandidateRequestID string `json:"candidate_request_id"`
			} `json:"items"`
			Total int `json:"total"`
		} `json:"data"`
	}
	decodeJSONResponse(t, casesResp.Body(), &casesPayload)
	if casesPayload.Code != 200 || casesPayload.Data.Total == 0 {
		t.Fatalf("unexpected cases payload: %+v", casesPayload)
	}
	if casesPayload.Data.Items[0].FailureReason != "recall_miss" {
		t.Fatalf("expected recall_miss, got %+v", casesPayload.Data.Items[0])
	}
	if casesPayload.Data.Items[0].BaselineRequestID == "" || casesPayload.Data.Items[0].CandidateRequestID == "" {
		t.Fatalf("expected trace request ids, got %+v", casesPayload.Data.Items[0])
	}

	exportJSONResp := ut.PerformRequest(h.Engine, http.MethodGet, "/api/admin/kb/eval/runs/"+createPayload.Data.RunID+"/report/export?format=json", nil).Result()
	if exportJSONResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected export json 200, got %d", exportJSONResp.StatusCode())
	}
	var exportedReport evaluation.Report
	if err := json.Unmarshal(exportJSONResp.Body(), &exportedReport); err != nil {
		t.Fatalf("failed to parse exported json report: %v", err)
	}
	if exportedReport.Baseline != "baseline-report" {
		t.Fatalf("unexpected exported report baseline: %+v", exportedReport)
	}

	exportMarkdownResp := ut.PerformRequest(h.Engine, http.MethodGet, "/api/admin/kb/eval/runs/"+createPayload.Data.RunID+"/report/export?format=markdown", nil).Result()
	if exportMarkdownResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected export markdown 200, got %d", exportMarkdownResp.StatusCode())
	}
	if !strings.Contains(string(exportMarkdownResp.Body()), "Baseline") {
		t.Fatalf("expected markdown export content, got %s", string(exportMarkdownResp.Body()))
	}
}

func newAdminEvalReportTestServer() *server.Hertz {
	h := server.Default()
	h.Use(func(ctx context.Context, c *app.RequestContext) {
		c.Set("user_id", uint(7))
		c.Set("role", "admin")
		c.Next(ctx)
	})
	h.POST("/api/admin/kb/eval/runs", CreateEvalRun)
	h.GET("/api/admin/kb/eval/runs/:run_id", GetEvalRun)
	h.GET("/api/admin/kb/eval/runs/:run_id/report", GetEvalReport)
	h.GET("/api/admin/kb/eval/runs/:run_id/cases", ListEvalFailureCases)
	h.GET("/api/admin/kb/eval/runs/:run_id/report/export", ExportEvalReport)
	return h
}
