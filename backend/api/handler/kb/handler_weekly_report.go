package kb

import (
	"context"
	"time"

	"interview-agents/api/response"
	myerrors "interview-agents/internal/errors"
	"interview-agents/internal/model"
	"interview-agents/internal/rag/experiment"
	"interview-agents/internal/rag/indexlifecycle"

	"github.com/cloudwego/hertz/pkg/app"
)

type weeklyReportResponse struct {
	GeneratedAt        time.Time                       `json:"generated_at"`
	WindowStart        time.Time                       `json:"window_start"`
	WindowEnd          time.Time                       `json:"window_end"`
	QualitySummary     metricsOverviewResponse         `json:"quality_summary"`
	ReleaseSummary     releaseSummaryResponse         `json:"release_summary"`
	ExperimentSummary  []experiment.Summary            `json:"experiment_summary"`
	IndexRegistry      []*model.KBIndexRegistry        `json:"index_registry"`
	IndexOperations    []*model.KBIndexOperationLog    `json:"index_operations"`
	AuditEvents        []*model.KBAuditEvent           `json:"audit_events"`
	Risks              []string                        `json:"risks"`
	NextActions        []string                        `json:"next_actions"`
}

func GetWeeklyReport(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}
	windowEnd := time.Now().UTC()
	windowStart := windowEnd.Add(-7 * 24 * time.Hour)

	retrieveLogs, err := model.KBRetrieveLogDao.ListByCreatedAt(windowStart, windowEnd, nil)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to load retrieve logs", err))
		return
	}
	ingestJobs, err := model.KBIngestJobDao.ListByCreatedAt(windowStart, windowEnd, nil)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to load ingest jobs", err))
		return
	}
	costTraces, err := model.KBCostTraceDao.ListByCreatedAt(windowStart, windowEnd, nil)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to load cost traces", err))
		return
	}
	indexRegistry, err := indexlifecycle.ListRegistry()
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to load index registry", err))
		return
	}
	indexOperations, err := indexlifecycle.ListOperations(50)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to load index operations", err))
		return
	}
	auditEvents, err := model.KBAuditEventDao.List(100)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to load audit events", err))
		return
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

	response.Success(ctx, c, weeklyReportResponse{
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
	})
}
