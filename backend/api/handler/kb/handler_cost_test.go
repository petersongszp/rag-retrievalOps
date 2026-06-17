package kb

import (
	"testing"
	"time"

	"interview-agents/internal/model"

	"github.com/cloudwego/hertz/pkg/app"
)

func TestBuildCostSummaryResponseWithData(t *testing.T) {
	now := time.Now().UTC()
	traces := []*model.KBCostTrace{
		{
			RequestID:               "req-1",
			TotalCost:               1.5,
			EmbeddingTokens:         100,
			EmbeddingCost:           0.2,
			CompletionTokens:        200,
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
			EmbeddingTokens:         50,
			EmbeddingCost:           0.1,
			CompletionTokens:        100,
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
	if resp.TotalTokens == nil || *resp.TotalTokens != 2450 {
		t.Fatalf("TotalTokens = %#v, want 2450", resp.TotalTokens)
	}
	if resp.TokensPer1KQueries == nil || *resp.TokensPer1KQueries != 1225000 {
		t.Fatalf("TokensPer1KQueries = %#v, want 1225000", resp.TokensPer1KQueries)
	}
	if resp.AvgTokensPerQuery == nil || *resp.AvgTokensPerQuery != 1225 {
		t.Fatalf("AvgTokensPerQuery = %#v, want 1225", resp.AvgTokensPerQuery)
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
		{RequestID: "req-1", TotalCost: 1, EmbeddingTokens: 10, ContextTokens: 100, CompletionTokens: 30, RerankCandidateCount: 5, CreatedAt: start.Add(10 * time.Minute)},
		{RequestID: "req-2", TotalCost: 3, EmbeddingTokens: 20, ContextTokens: 300, CompletionTokens: 40, RerankCandidateCount: 15, CreatedAt: start.Add(70 * time.Minute)},
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
	if items[0].TotalTokens != 140 || items[1].TotalTokens != 360 {
		t.Fatalf("unexpected token totals: %#v", items)
	}
	if items[1].AvgTokensPerQuery != 360 || items[1].TokensPer1KQueries != 360000 {
		t.Fatalf("unexpected token averages: %#v", items[1])
	}
}

func TestResolveCostWindowPrefersExplicitWindowParams(t *testing.T) {
	c := &app.RequestContext{}
	c.Request.SetRequestURI("/api/admin/kb/cost/timeseries?range=24h&start_time=2026-06-08T00:00:00%2B08:00&end_time=2026-06-08T23:59:59.999999999%2B08:00&bucket=1h&tz=Asia%2FShanghai")

	rangeName, startInclusive, queryEnd, bucketSize, bucketLabel, err := resolveCostWindow(c)
	if err != nil {
		t.Fatalf("resolveCostWindow returned error: %v", err)
	}
	if rangeName != "custom" {
		t.Fatalf("rangeName = %q, want custom", rangeName)
	}
	if bucketLabel != "1h" || bucketSize != time.Hour {
		t.Fatalf("bucket = (%q, %v), want (1h, 1h)", bucketLabel, bucketSize)
	}
	expectedStart := time.Date(2026, 6, 8, 0, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	expectedEnd := time.Date(2026, 6, 8, 23, 59, 59, 999999999, time.FixedZone("CST", 8*60*60))
	if !startInclusive.Equal(expectedStart) {
		t.Fatalf("startInclusive = %s, want %s", startInclusive, expectedStart)
	}
	if !queryEnd.Equal(expectedEnd) {
		t.Fatalf("queryEnd = %s, want %s", queryEnd, expectedEnd)
	}
}

func TestResolveExplicitCostWindowSupportsNaturalMonthInShanghai(t *testing.T) {
	c := &app.RequestContext{}
	c.Request.SetRequestURI("/api/admin/kb/cost/timeseries?start_time=2026-06-01T00:00:00%2B08:00&end_time=2026-06-30T23:59:59.999999999%2B08:00&bucket=1d&tz=Asia%2FShanghai")

	rangeName, startInclusive, queryEnd, bucketSize, bucketLabel, err := resolveCostWindow(c)
	if err != nil {
		t.Fatalf("resolveCostWindow returned error: %v", err)
	}
	if rangeName != "custom" || bucketLabel != "1d" || bucketSize != 24*time.Hour {
		t.Fatalf("unexpected custom window metadata: range=%q bucket=%q size=%v", rangeName, bucketLabel, bucketSize)
	}

	bucketCount := int(queryEnd.Add(time.Nanosecond).Sub(startInclusive) / bucketSize)
	if bucketCount != 30 {
		t.Fatalf("bucketCount = %d, want 30", bucketCount)
	}
}

func TestResolveExplicitCostWindowRejectsInvalidTimezone(t *testing.T) {
	c := &app.RequestContext{}
	c.Request.SetRequestURI("/api/admin/kb/cost/timeseries?start_time=2026-06-01T00:00:00&end_time=2026-06-01T23:59:59&bucket=1h&tz=Mars%2FOlympus")

	_, _, _, _, _, err := resolveCostWindow(c)
	if err == nil {
		t.Fatal("resolveCostWindow error = nil, want validation error")
	}
}

func TestBuildRetrieveCostTraceMarksSemanticCacheSavings(t *testing.T) {
	logEntry := &model.KBRetrieveLog{
		RequestID:           "req-cache",
		KBIDs:               "7",
		UserID:              3,
		CandidateTopK:       8,
		FinalTopK:           4,
		FinalCount:          4,
		DenseHits:           4,
		SparseHits:          2,
		ContextTokens:       200,
		SemanticCacheHit:    true,
		SemanticCacheReason: "hit",
		StrategyVersion:     "phase4",
		QueryType:           "general",
	}

	trace := buildRetrieveCostTrace(logEntry)
	if trace == nil {
		t.Fatal("buildRetrieveCostTrace() = nil")
	}
	if trace.RetrievalCost != 0 || trace.RerankCost != 0 {
		t.Fatalf("expected actual retrieval/rerank cost to be zero on cache hit, got retrieval=%v rerank=%v", trace.RetrievalCost, trace.RerankCost)
	}
	if trace.CacheSavedRetrievalCost <= 0 || trace.CacheSavedRerankCost <= 0 {
		t.Fatalf("expected saved cost to be recorded, got %+v", trace)
	}
}
