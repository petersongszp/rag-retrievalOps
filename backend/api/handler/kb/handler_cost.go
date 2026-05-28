package kb

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"interview-agents/api/response"
	myerrors "interview-agents/internal/errors"
	"interview-agents/internal/model"

	"github.com/cloudwego/hertz/pkg/app"
)

type costSummaryResponse struct {
	Range              string    `json:"range"`
	TotalEstimatedCost *float64  `json:"total_estimated_cost,omitempty"`
	Currency           string    `json:"currency,omitempty"`
	CostPer1KQueries   *float64  `json:"cost_per_1k_queries,omitempty"`
	EmbeddingCost      *float64  `json:"embedding_cost,omitempty"`
	LLMCost            *float64  `json:"llm_cost,omitempty"`
	RerankCost         *float64  `json:"rerank_cost,omitempty"`
	VectorStorageCost  *float64  `json:"vector_storage_cost,omitempty"`
	IndexRebuildCost   *float64  `json:"index_rebuild_cost,omitempty"`
	AvgContextTokens   *float64  `json:"avg_context_tokens,omitempty"`
	AvgCandidateCount  *float64  `json:"avg_candidate_count,omitempty"`
	HighCostQueryCount *int      `json:"high_cost_query_count,omitempty"`
	ContractGaps       []string  `json:"contract_gaps,omitempty"`
	GeneratedAt        time.Time `json:"generated_at"`
}

type costTimeseriesPointResponse struct {
	Bucket             time.Time `json:"bucket"`
	TotalEstimatedCost float64   `json:"total_estimated_cost"`
	CostPer1KQueries   float64   `json:"cost_per_1k_queries"`
	EmbeddingCost      float64   `json:"embedding_cost"`
	LLMCost            float64   `json:"llm_cost"`
	RerankCost         float64   `json:"rerank_cost"`
	VectorStorageCost  float64   `json:"vector_storage_cost"`
	AvgContextTokens   float64   `json:"avg_context_tokens"`
	AvgCandidateCount  float64   `json:"avg_candidate_count"`
}

type costTimeseriesResponse struct {
	Range        string                        `json:"range"`
	Bucket       string                        `json:"bucket"`
	Items        []costTimeseriesPointResponse `json:"items"`
	ContractGaps []string                      `json:"contract_gaps,omitempty"`
}

type costBreakdownItemResponse struct {
	Key                string  `json:"key"`
	Label              string  `json:"label"`
	TotalEstimatedCost float64 `json:"total_estimated_cost"`
	CostPer1KQueries   float64 `json:"cost_per_1k_queries"`
	RequestCount       int     `json:"request_count"`
	Share              float64 `json:"share"`
}

type costBreakdownListResponse struct {
	Range        string                      `json:"range"`
	Items        []costBreakdownItemResponse `json:"items"`
	ContractGaps []string                    `json:"contract_gaps,omitempty"`
}

type highCostQueryItemResponse struct {
	RequestID       string    `json:"request_id"`
	KBID            uint64    `json:"kb_id,omitempty"`
	QueryType       string    `json:"query_type,omitempty"`
	StrategyVersion string    `json:"strategy_version,omitempty"`
	ExperimentID    string    `json:"experiment_id,omitempty"`
	ModelName       string    `json:"model_name,omitempty"`
	EstimatedCost   float64   `json:"estimated_cost"`
	Currency        string    `json:"currency"`
	ContextTokens   int       `json:"context_tokens,omitempty"`
	CandidateCount  int       `json:"candidate_count,omitempty"`
	FinalCount      int       `json:"final_count,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type highCostQueryListResponse struct {
	Items        []highCostQueryItemResponse `json:"items"`
	Total        int                         `json:"total"`
	Page         int                         `json:"page"`
	PageSize     int                         `json:"page_size"`
	ContractGaps []string                    `json:"contract_gaps,omitempty"`
}

type costQueryFilter struct {
	kbID            *uint64
	strategyVersion string
	experimentID    string
	modelName       string
	queryType       string
}

func GetCostSummary(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	rangeName, startInclusive, queryEnd, _, _, err := resolveCostWindow(c)
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}
	filter, err := parseCostQueryFilter(c)
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}
	traces, err := listCostTracesForFilter(startInclusive, queryEnd, filter)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to load cost summary", err))
		return
	}

	summary := buildCostSummaryResponse(rangeName, traces)
	response.Success(ctx, c, summary)
}

func GetCostTimeseries(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	rangeName, startInclusive, queryEnd, bucketSize, bucketLabel, err := resolveCostWindow(c)
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}
	filter, err := parseCostQueryFilter(c)
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}
	traces, err := listCostTracesForFilter(startInclusive, queryEnd, filter)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to load cost timeseries", err))
		return
	}

	items := buildCostTimeseriesResponse(traces, startInclusive, queryEnd, bucketSize)
	resp := costTimeseriesResponse{
		Range:  rangeName,
		Bucket: bucketLabel,
		Items:  items,
	}
	if len(traces) == 0 {
		resp.ContractGaps = []string{"cost_trace"}
	}
	response.Success(ctx, c, resp)
}

func GetCostByKB(ctx context.Context, c *app.RequestContext) {
	getCostBreakdown(ctx, c, func(trace *model.KBCostTrace) (string, string) {
		if trace == nil || trace.KBID == 0 {
			return "unknown", "unknown"
		}
		key := uint64ToString(trace.KBID)
		return key, key
	})
}

func GetCostByStrategy(ctx context.Context, c *app.RequestContext) {
	getCostBreakdown(ctx, c, func(trace *model.KBCostTrace) (string, string) {
		if trace == nil || strings.TrimSpace(trace.StrategyVersion) == "" {
			return "unknown", "unknown"
		}
		value := strings.TrimSpace(trace.StrategyVersion)
		return value, value
	})
}

func GetCostByModel(ctx context.Context, c *app.RequestContext) {
	getCostBreakdown(ctx, c, func(trace *model.KBCostTrace) (string, string) {
		if trace == nil || strings.TrimSpace(trace.LLMModel) == "" {
			return "unknown", "unknown"
		}
		value := strings.TrimSpace(trace.LLMModel)
		return value, value
	})
}

func ListHighCostQueries(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	rangeName, startInclusive, queryEnd, _, _, err := resolveCostWindow(c)
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}
	filter, err := parseCostQueryFilter(c)
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}
	traces, err := listCostTracesForFilter(startInclusive, queryEnd, filter)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to load high cost queries", err))
		return
	}

	sort.SliceStable(traces, func(i, j int) bool {
		if traces[i] == nil {
			return false
		}
		if traces[j] == nil {
			return true
		}
		if traces[i].TotalCost == traces[j].TotalCost {
			return traces[i].CreatedAt.After(traces[j].CreatedAt)
		}
		return traces[i].TotalCost > traces[j].TotalCost
	})

	page, pageSize := getPagination(c)
	total := len(traces)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	items := make([]highCostQueryItemResponse, 0, end-start)
	for _, trace := range traces[start:end] {
		if trace == nil {
			continue
		}
		items = append(items, highCostQueryItemResponse{
			RequestID:       trace.RequestID,
			KBID:            trace.KBID,
			QueryType:       trace.QueryType,
			StrategyVersion: trace.StrategyVersion,
			ExperimentID:    trace.ExperimentID,
			ModelName:       trace.LLMModel,
			EstimatedCost:   trace.TotalCost,
			Currency:        "USD",
			ContextTokens:   trace.ContextTokens,
			CandidateCount:  candidateCountForTrace(trace),
			FinalCount:      0,
			CreatedAt:       trace.CreatedAt,
		})
	}

	resp := highCostQueryListResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
	if total == 0 {
		resp.ContractGaps = []string{"cost_trace"}
	}
	_ = rangeName
	response.Success(ctx, c, resp)
}

func getCostBreakdown(ctx context.Context, c *app.RequestContext, resolveKey func(*model.KBCostTrace) (string, string)) {
	if !requireAdmin(ctx, c) {
		return
	}

	rangeName, startInclusive, queryEnd, _, _, err := resolveCostWindow(c)
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}
	filter, err := parseCostQueryFilter(c)
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}
	traces, err := listCostTracesForFilter(startInclusive, queryEnd, filter)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to load cost breakdown", err))
		return
	}

	resp := costBreakdownListResponse{
		Range: rangeName,
		Items: buildCostBreakdownItems(traces, resolveKey),
	}
	if len(traces) == 0 {
		resp.ContractGaps = []string{"cost_trace"}
	}
	response.Success(ctx, c, resp)
}

func buildCostSummaryResponse(rangeName string, traces []*model.KBCostTrace) costSummaryResponse {
	resp := costSummaryResponse{
		Range:       rangeName,
		Currency:    "USD",
		GeneratedAt: time.Now().UTC(),
	}
	if len(traces) == 0 {
		resp.ContractGaps = []string{"cost_trace", "index_rebuild_cost"}
		return resp
	}

	var totalCost float64
	var embeddingCost float64
	var llmCost float64
	var rerankCost float64
	var vectorStorageCost float64
	var contextTokens int
	var candidateCount int
	var highCostCount int

	for _, trace := range traces {
		if trace == nil {
			continue
		}
		totalCost += trace.TotalCost
		embeddingCost += trace.EmbeddingCost
		llmCost += trace.LLMCost
		rerankCost += trace.RerankCost
		vectorStorageCost += trace.VectorStorageCost
		contextTokens += trace.ContextTokens
		candidateCount += candidateCountForTrace(trace)
		if trace.TotalCost >= 0.01 {
			highCostCount++
		}
	}

	count := len(nonNilCostTraces(traces))
	if count == 0 {
		resp.ContractGaps = []string{"cost_trace", "index_rebuild_cost"}
		return resp
	}

	costPer1K := totalCost / float64(count) * 1000
	avgContextTokens := float64(contextTokens) / float64(count)
	avgCandidateCount := float64(candidateCount) / float64(count)

	resp.TotalEstimatedCost = float64Ptr(totalCost)
	resp.CostPer1KQueries = float64Ptr(costPer1K)
	resp.EmbeddingCost = float64Ptr(embeddingCost)
	resp.LLMCost = float64Ptr(llmCost)
	resp.RerankCost = float64Ptr(rerankCost)
	resp.VectorStorageCost = float64Ptr(vectorStorageCost)
	resp.AvgContextTokens = float64Ptr(avgContextTokens)
	resp.AvgCandidateCount = float64Ptr(avgCandidateCount)
	resp.HighCostQueryCount = intPtr(highCostCount)
	resp.ContractGaps = []string{"index_rebuild_cost"}
	return resp
}

func buildCostTimeseriesResponse(
	traces []*model.KBCostTrace,
	startInclusive time.Time,
	queryEnd time.Time,
	bucketSize time.Duration,
) []costTimeseriesPointResponse {
	if len(traces) == 0 {
		return []costTimeseriesPointResponse{}
	}
	bucketCount := int(queryEnd.Add(time.Nanosecond).Sub(startInclusive) / bucketSize)
	if bucketCount <= 0 {
		return []costTimeseriesPointResponse{}
	}

	type aggregate struct {
		totalCost         float64
		embeddingCost     float64
		llmCost           float64
		rerankCost        float64
		vectorStorageCost float64
		contextTokens     int
		candidateCount    int
		queries           int
	}

	aggregates := make([]aggregate, bucketCount)
	for _, trace := range traces {
		if trace == nil {
			continue
		}
		index := bucketIndex(trace.CreatedAt.UTC(), startInclusive, bucketSize, bucketCount)
		if index < 0 {
			continue
		}
		aggregates[index].totalCost += trace.TotalCost
		aggregates[index].embeddingCost += trace.EmbeddingCost
		aggregates[index].llmCost += trace.LLMCost
		aggregates[index].rerankCost += trace.RerankCost
		aggregates[index].vectorStorageCost += trace.VectorStorageCost
		aggregates[index].contextTokens += trace.ContextTokens
		aggregates[index].candidateCount += candidateCountForTrace(trace)
		aggregates[index].queries++
	}

	items := make([]costTimeseriesPointResponse, 0, bucketCount)
	for i := 0; i < bucketCount; i++ {
		item := costTimeseriesPointResponse{
			Bucket:             startInclusive.Add(time.Duration(i) * bucketSize),
			TotalEstimatedCost: aggregates[i].totalCost,
			EmbeddingCost:      aggregates[i].embeddingCost,
			LLMCost:            aggregates[i].llmCost,
			RerankCost:         aggregates[i].rerankCost,
			VectorStorageCost:  aggregates[i].vectorStorageCost,
		}
		if aggregates[i].queries > 0 {
			item.CostPer1KQueries = aggregates[i].totalCost / float64(aggregates[i].queries) * 1000
			item.AvgContextTokens = float64(aggregates[i].contextTokens) / float64(aggregates[i].queries)
			item.AvgCandidateCount = float64(aggregates[i].candidateCount) / float64(aggregates[i].queries)
		}
		items = append(items, item)
	}
	return items
}

func buildCostBreakdownItems(
	traces []*model.KBCostTrace,
	resolveKey func(*model.KBCostTrace) (string, string),
) []costBreakdownItemResponse {
	type aggregate struct {
		key       string
		label     string
		totalCost float64
		requests  int
	}

	grouped := make(map[string]*aggregate)
	totalCost := 0.0
	for _, trace := range traces {
		if trace == nil {
			continue
		}
		key, label := resolveKey(trace)
		entry, ok := grouped[key]
		if !ok {
			entry = &aggregate{key: key, label: label}
			grouped[key] = entry
		}
		entry.totalCost += trace.TotalCost
		entry.requests++
		totalCost += trace.TotalCost
	}

	items := make([]costBreakdownItemResponse, 0, len(grouped))
	for _, entry := range grouped {
		item := costBreakdownItemResponse{
			Key:                entry.key,
			Label:              entry.label,
			TotalEstimatedCost: entry.totalCost,
			RequestCount:       entry.requests,
		}
		if entry.requests > 0 {
			item.CostPer1KQueries = entry.totalCost / float64(entry.requests) * 1000
		}
		if totalCost > 0 {
			item.Share = entry.totalCost / totalCost
		}
		items = append(items, item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].TotalEstimatedCost == items[j].TotalEstimatedCost {
			return items[i].Key < items[j].Key
		}
		return items[i].TotalEstimatedCost > items[j].TotalEstimatedCost
	})
	return items
}

func resolveCostWindow(c *app.RequestContext) (string, time.Time, time.Time, time.Duration, string, error) {
	rangeName := strings.TrimSpace(string(c.Query("range")))
	if rangeName == "" {
		rangeName = "24h"
	}

	window := 24 * time.Hour
	defaultBucket := time.Hour
	switch rangeName {
	case "1h":
		window = time.Hour
		defaultBucket = 5 * time.Minute
	case "24h":
		window = 24 * time.Hour
		defaultBucket = time.Hour
	case "7d":
		window = 7 * 24 * time.Hour
		defaultBucket = 6 * time.Hour
	case "30d":
		window = 30 * 24 * time.Hour
		defaultBucket = 24 * time.Hour
	default:
		return "", time.Time{}, time.Time{}, 0, "", myerrors.NewValidationError("range must be one of 1h, 24h, 7d, 30d")
	}

	bucketLabel := strings.TrimSpace(string(c.Query("bucket")))
	bucketSize := defaultBucket
	if bucketLabel != "" {
		switch bucketLabel {
		case "5m":
			bucketSize = 5 * time.Minute
		case "1h":
			bucketSize = time.Hour
		case "6h":
			bucketSize = 6 * time.Hour
		case "1d":
			bucketSize = 24 * time.Hour
		default:
			return "", time.Time{}, time.Time{}, 0, "", myerrors.NewValidationError("bucket must be one of 5m, 1h, 6h, 1d")
		}
	} else {
		bucketLabel = formatDurationBucket(defaultBucket)
	}

	endExclusive := alignTimeBucket(time.Now().UTC(), bucketSize).Add(bucketSize)
	startInclusive := endExclusive.Add(-window)
	queryEnd := endExclusive.Add(-time.Nanosecond)
	return rangeName, startInclusive, queryEnd, bucketSize, bucketLabel, nil
}

func parseCostQueryFilter(c *app.RequestContext) (costQueryFilter, error) {
	var filter costQueryFilter

	kbIDRaw := strings.TrimSpace(string(c.Query("kb_id")))
	if kbIDRaw != "" {
		parsed, err := parseUint64(kbIDRaw, "kb_id")
		if err != nil {
			return filter, myerrors.NewValidationError(err.Error())
		}
		filter.kbID = &parsed
	}

	filter.strategyVersion = strings.TrimSpace(string(c.Query("strategy_version")))
	filter.experimentID = strings.TrimSpace(string(c.Query("experiment_id")))
	filter.modelName = strings.TrimSpace(string(c.Query("model_name")))
	filter.queryType = strings.TrimSpace(string(c.Query("query_type")))
	return filter, nil
}

func listCostTracesForFilter(startInclusive, queryEnd time.Time, filter costQueryFilter) ([]*model.KBCostTrace, error) {
	items, err := model.KBCostTraceDao.ListByCreatedAt(startInclusive, queryEnd, filter.kbID)
	if err != nil {
		return nil, err
	}
	if filter.strategyVersion == "" && filter.experimentID == "" && filter.modelName == "" && filter.queryType == "" {
		return items, nil
	}

	filtered := make([]*model.KBCostTrace, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		if filter.strategyVersion != "" && !strings.EqualFold(strings.TrimSpace(item.StrategyVersion), filter.strategyVersion) {
			continue
		}
		if filter.experimentID != "" && !strings.EqualFold(strings.TrimSpace(item.ExperimentID), filter.experimentID) {
			continue
		}
		if filter.modelName != "" && !strings.EqualFold(strings.TrimSpace(item.LLMModel), filter.modelName) {
			continue
		}
		if filter.queryType != "" && !strings.EqualFold(strings.TrimSpace(item.QueryType), filter.queryType) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered, nil
}

func candidateCountForTrace(trace *model.KBCostTrace) int {
	if trace == nil {
		return 0
	}
	if trace.RerankCandidateCount > 0 {
		return trace.RerankCandidateCount
	}
	return trace.RetrievalCandidateCount
}

func nonNilCostTraces(items []*model.KBCostTrace) []*model.KBCostTrace {
	out := make([]*model.KBCostTrace, 0, len(items))
	for _, item := range items {
		if item != nil {
			out = append(out, item)
		}
	}
	return out
}

func intPtr(value int) *int {
	return &value
}

func uint64ToString(value uint64) string {
	return strconv.FormatUint(value, 10)
}

func formatDurationBucket(value time.Duration) string {
	switch value {
	case 5 * time.Minute:
		return "5m"
	case time.Hour:
		return "1h"
	case 6 * time.Hour:
		return "6h"
	case 24 * time.Hour:
		return "1d"
	default:
		return value.String()
	}
}
