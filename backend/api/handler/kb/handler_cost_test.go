package kb

import (
	"testing"
	"time"

	"interview-agents/internal/model"
)

func TestBuildCostSummaryResponseWithData(t *testing.T) {
	now := time.Now().UTC()
	traces := []*model.KBCostTrace{
		{
			RequestID:               "req-1",
			TotalCost:               1.5,
			EmbeddingCost:           0.2,
			LLMCost:                 0.9,
			RerankCost:              0.1,
			VectorStorageCost:       0.3,
			ContextTokens:           1200,
			RetrievalCandidateCount: 20,
			RerankCandidateCount:    10,
			CreatedAt:               now,
		},
		{
			RequestID:               "req-2",
			TotalCost:               0.5,
			EmbeddingCost:           0.1,
			LLMCost:                 0.3,
			RerankCost:              0.05,
			VectorStorageCost:       0.05,
			ContextTokens:           800,
			RetrievalCandidateCount: 8,
			RerankCandidateCount:    4,
			CreatedAt:               now,
		},
	}

	resp := buildCostSummaryResponse("24h", traces)
	if resp.TotalEstimatedCost == nil || *resp.TotalEstimatedCost != 2.0 {
		t.Fatalf("TotalEstimatedCost = %#v, want 2.0", resp.TotalEstimatedCost)
	}
	if resp.CostPer1KQueries == nil || *resp.CostPer1KQueries != 1000 {
		t.Fatalf("CostPer1KQueries = %#v, want 1000", resp.CostPer1KQueries)
	}
	if resp.AvgCandidateCount == nil || *resp.AvgCandidateCount != 7 {
		t.Fatalf("AvgCandidateCount = %#v, want 7", resp.AvgCandidateCount)
	}
	if len(resp.ContractGaps) != 1 || resp.ContractGaps[0] != "index_rebuild_cost" {
		t.Fatalf("ContractGaps = %#v, want index_rebuild_cost", resp.ContractGaps)
	}
}

func TestBuildCostSummaryResponseWithoutDataMarksContractGaps(t *testing.T) {
	resp := buildCostSummaryResponse("24h", nil)
	if len(resp.ContractGaps) == 0 {
		t.Fatal("ContractGaps = empty, want visible gaps")
	}
}

func TestBuildCostBreakdownItemsSortsByCost(t *testing.T) {
	traces := []*model.KBCostTrace{
		{KBID: 2, TotalCost: 1.2},
		{KBID: 1, TotalCost: 3.4},
		{KBID: 2, TotalCost: 0.3},
	}

	items := buildCostBreakdownItems(traces, func(trace *model.KBCostTrace) (string, string) {
		return uint64ToString(trace.KBID), uint64ToString(trace.KBID)
	})
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].Key != "1" || items[1].Key != "2" {
		t.Fatalf("items order = %#v, want key 1 then 2", items)
	}
}

func TestBuildCostTimeseriesResponseBucketsQueries(t *testing.T) {
	start := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	traces := []*model.KBCostTrace{
		{RequestID: "req-1", TotalCost: 1, ContextTokens: 100, RerankCandidateCount: 5, CreatedAt: start.Add(10 * time.Minute)},
		{RequestID: "req-2", TotalCost: 3, ContextTokens: 300, RerankCandidateCount: 15, CreatedAt: start.Add(70 * time.Minute)},
	}

	items := buildCostTimeseriesResponse(traces, start, start.Add(2*time.Hour-time.Nanosecond), time.Hour)
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].TotalEstimatedCost != 1 || items[1].TotalEstimatedCost != 3 {
		t.Fatalf("unexpected bucket totals: %#v", items)
	}
	if items[1].AvgCandidateCount != 15 {
		t.Fatalf("AvgCandidateCount = %v, want 15", items[1].AvgCandidateCount)
	}
}
