package kb

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"interview-agents/api/response"
	myerrors "interview-agents/internal/errors"
	"interview-agents/internal/model"
	"interview-agents/internal/rag/experiment"
	"interview-agents/internal/rag/indexlifecycle"

	"github.com/cloudwego/hertz/pkg/app"
)

type weeklyReportRecord struct {
	ID          string               `json:"id"`
	GeneratedAt time.Time            `json:"generated_at"`
	WindowStart time.Time            `json:"window_start"`
	WindowEnd   time.Time            `json:"window_end"`
	Report      weeklyReportResponse `json:"report"`
}

type weeklyReportResponse struct {
	GeneratedAt       time.Time                    `json:"generated_at"`
	WindowStart       time.Time                    `json:"window_start"`
	WindowEnd         time.Time                    `json:"window_end"`
	QualitySummary    metricsOverviewResponse      `json:"quality_summary"`
	ReleaseSummary    releaseSummaryResponse       `json:"release_summary"`
	ExperimentSummary []experiment.Summary         `json:"experiment_summary"`
	IndexRegistry     []*model.KBIndexRegistry     `json:"index_registry"`
	IndexOperations   []*model.KBIndexOperationLog `json:"index_operations"`
	AuditEvents       []*model.KBAuditEvent        `json:"audit_events"`
	Risks             []string                     `json:"risks"`
	NextActions       []string                     `json:"next_actions"`
}

type weeklyReportListResponse struct {
	Items    []weeklyReportRecord `json:"items"`
	Total    int                  `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
}

var (
	weeklyReportMu      sync.Mutex
	weeklyReportRecords []weeklyReportRecord
)

func CreateWeeklyReport(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}
	report, err := buildWeeklyReport()
	if err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}
	record := weeklyReportRecord{
		ID:          fmt.Sprintf("weekly-%d", report.GeneratedAt.Unix()),
		GeneratedAt: report.GeneratedAt,
		WindowStart: report.WindowStart,
		WindowEnd:   report.WindowEnd,
		Report:      report,
	}
	weeklyReportMu.Lock()
	weeklyReportRecords = append([]weeklyReportRecord{record}, weeklyReportRecords...)
	if len(weeklyReportRecords) > 20 {
		weeklyReportRecords = weeklyReportRecords[:20]
	}
	weeklyReportMu.Unlock()
	response.Success(ctx, c, record)
}

func ListWeeklyReports(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	page, pageSize := getPagination(c)
	items := snapshotWeeklyReports()
	total := len(items)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	response.Success(ctx, c, weeklyReportListResponse{
		Items:    items[start:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func GetWeeklyReportDetail(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}
	reportID := c.Param("report_id")
	for _, item := range snapshotWeeklyReports() {
		if item.ID == reportID {
			response.Success(ctx, c, item)
			return
		}
	}
	response.NotFound(ctx, c, "weekly report not found")
}

func ExportWeeklyReport(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}
	reportID := c.Param("report_id")
	for _, item := range snapshotWeeklyReports() {
		if item.ID == reportID {
			response.Success(ctx, c, map[string]interface{}{
				"report_id":      reportID,
				"exported_at":    time.Now().UTC(),
				"contract_gaps":  []string{"download_url"},
				"report_summary": item.Report.ReleaseSummary,
			})
			return
		}
	}
	response.NotFound(ctx, c, "weekly report not found")
}

func buildWeeklyReport() (weeklyReportResponse, error) {
	windowEnd := time.Now().UTC()
	windowStart := windowEnd.Add(-7 * 24 * time.Hour)

	retrieveLogs, err := model.KBRetrieveLogDao.ListByCreatedAt(windowStart, windowEnd, nil)
	if err != nil {
		return weeklyReportResponse{}, myerrors.NewDBError("failed to load retrieve logs", err)
	}
	ingestJobs, err := model.KBIngestJobDao.ListByCreatedAt(windowStart, windowEnd, nil)
	if err != nil {
		return weeklyReportResponse{}, myerrors.NewDBError("failed to load ingest jobs", err)
	}
	costTraces, err := model.KBCostTraceDao.ListByCreatedAt(windowStart, windowEnd, nil)
	if err != nil {
		return weeklyReportResponse{}, myerrors.NewDBError("failed to load cost traces", err)
	}
	indexRegistry, err := indexlifecycle.ListRegistry()
	if err != nil {
		return weeklyReportResponse{}, myerrors.NewDBError("failed to load index registry", err)
	}
	indexOperations, err := indexlifecycle.ListOperations(50)
	if err != nil {
		return weeklyReportResponse{}, myerrors.NewDBError("failed to load index operations", err)
	}
	auditEvents, err := model.KBAuditEventDao.List(100)
	if err != nil {
		return weeklyReportResponse{}, myerrors.NewDBError("failed to load audit events", err)
	}

	qualitySummary := metricsOverviewResponse{
		Range:                        "7d",
		IngestSuccessRate:            buildIngestSuccessRateSeries(ingestJobs, alignTimeBucket(windowStart, 6*time.Hour), 6*time.Hour, 28),
		RetrieveRequestCount:         buildRetrieveRequestCountSeries(retrieveLogs, alignTimeBucket(windowStart, 6*time.Hour), 6*time.Hour, 28),
		RetrieveP95Ms:                buildRetrieveP95Series(retrieveLogs, alignTimeBucket(windowStart, 6*time.Hour), 6*time.Hour, 28),
		RetrieveEmptyRate:            buildRetrieveEmptyRateSeries(retrieveLogs, alignTimeBucket(windowStart, 6*time.Hour), 6*time.Hour, 28),
		ParentFillAppliedRate:        buildParentFillAppliedRateSeries(retrieveLogs, alignTimeBucket(windowStart, 6*time.Hour), 6*time.Hour, 28),
		EvidenceRefusalRate:          buildEvidenceRefusalRateSeries(retrieveLogs, alignTimeBucket(windowStart, 6*time.Hour), 6*time.Hour, 28),
		RouteSpecificRewriteGainRate: buildRouteSpecificRewriteGainRateSeries(retrieveLogs, alignTimeBucket(windowStart, 6*time.Hour), 6*time.Hour, 28),
		ModelRewriteErrorRate:        buildModelRewriteErrorRateSeries(retrieveLogs, alignTimeBucket(windowStart, 6*time.Hour), 6*time.Hour, 28),
		CitationSupportScore:         buildCitationSupportScoreSeries(retrieveLogs, alignTimeBucket(windowStart, 6*time.Hour), 6*time.Hour, 28),
		RouteContributionTotal:       buildRouteContributionTotal(retrieveLogs),
		RewriteGainBucketCounts:      buildRewriteGainBucketCounts(retrieveLogs),
		ErrorTypeTopN:                buildRetrieveErrorTopN(retrieveLogs),
		CostOverview:                 buildCostOverviewSeries(costTraces, alignTimeBucket(windowStart, 6*time.Hour), 6*time.Hour, 28),
	}

	experimentLogs := make([]experiment.RetrieveLogLike, 0, len(retrieveLogs))
	for _, item := range retrieveLogs {
		if item != nil {
			experimentLogs = append(experimentLogs, retrieveLogAdapter{item})
		}
	}
	experimentSummary := experiment.BuildSummary(experimentLogs)
	releaseSummary := buildReleaseSummary(7*24*60, windowStart, retrieveLogs)

	risks := append([]string(nil), releaseSummary.Risks...)
	if len(auditEvents) == 0 {
		risks = append(risks, "审计事件为空，需要确认治理链路是否正常写入")
	}
	if len(indexRegistry) == 0 {
		risks = append(risks, "索引注册表为空，需要补齐 index registry 初始化")
	}

	return weeklyReportResponse{
		GeneratedAt:       time.Now().UTC(),
		WindowStart:       windowStart,
		WindowEnd:         windowEnd,
		QualitySummary:    qualitySummary,
		ReleaseSummary:    releaseSummary,
		ExperimentSummary: experimentSummary,
		IndexRegistry:     indexRegistry,
		IndexOperations:   indexOperations,
		AuditEvents:       auditEvents,
		Risks:             risks,
		NextActions: []string{
			"检查高成本 query 与高空结果率时间段",
			"复盘最近一次实验 candidate 与 baseline 差异",
			"确认 active/rollback collection 是否完成演练",
		},
	}, nil
}

func snapshotWeeklyReports() []weeklyReportRecord {
	weeklyReportMu.Lock()
	defer weeklyReportMu.Unlock()

	items := append([]weeklyReportRecord(nil), weeklyReportRecords...)
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].GeneratedAt.After(items[j].GeneratedAt)
	})
	return items
}
