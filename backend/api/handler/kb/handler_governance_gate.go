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

type governanceGateResponse struct {
	GeneratedAt               time.Time `json:"generated_at"`
	Passed                    bool      `json:"passed"`
	CostGuardPassed           bool      `json:"cost_guard_passed"`
	AuditGuardPassed          bool      `json:"audit_guard_passed"`
	IndexGuardPassed          bool      `json:"index_guard_passed"`
	ExperimentGuardPassed     bool      `json:"experiment_guard_passed"`
	ReleaseGuardPassed        bool      `json:"release_guard_passed"`
	CollectionHealthScore     float64   `json:"collection_health_score"`
	AuditCoverageRate         float64   `json:"audit_coverage_rate"`
	RollbackSuccessRate       float64   `json:"rollback_success_rate"`
	StrategyRegressionRate    float64   `json:"strategy_regression_rate"`
	CostPer1KQueries          float64   `json:"cost_per_1k_queries"`
	Risks                     []string  `json:"risks"`
}

func GetGovernanceGate(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}
	now := time.Now().UTC()
	since := now.Add(-24 * time.Hour)

	retrieveLogs, err := model.KBRetrieveLogDao.ListByCreatedAt(since, now, nil)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to load retrieve logs", err))
		return
	}
	costTraces, err := model.KBCostTraceDao.ListByCreatedAt(since, now, nil)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to load cost traces", err))
		return
	}
	indexRegistry, err := indexlifecycle.ListRegistry()
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to load index registry", err))
		return
	}
	auditEvents, err := model.KBAuditEventDao.List(200)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to load audit events", err))
		return
	}

	costSeries := buildCostOverviewSeries(costTraces, alignTimeBucket(since, time.Hour), time.Hour, 24)
	latestCostPer1K := 0.0
	if len(costSeries) > 0 {
		latestCostPer1K = costSeries[len(costSeries)-1].CostPer1KQueries
	}
	costGuardPassed := latestCostPer1K < 25 || latestCostPer1K == 0

	auditCoverageRate := 1.0
	if len(retrieveLogs) > 0 {
		auditCoverageRate = float64(len(auditEvents)) / float64(len(retrieveLogs))
		if auditCoverageRate > 1 {
			auditCoverageRate = 1
		}
	}
	auditGuardPassed := auditCoverageRate >= 0.6

	collectionHealthScore := 0.0
	if len(indexRegistry) > 0 {
		healthy := 0
		for _, item := range indexRegistry {
			if item != nil && (item.BuildStatus == model.IndexBuildStatusReady || item.BuildStatus == model.IndexBuildStatusSwitched) {
				healthy++
			}
		}
		collectionHealthScore = float64(healthy) / float64(len(indexRegistry))
	}
	indexGuardPassed := collectionHealthScore >= 0.5

	experimentLogs := make([]experiment.RetrieveLogLike, 0, len(retrieveLogs))
	for _, item := range retrieveLogs {
		if item != nil {
			experimentLogs = append(experimentLogs, retrieveLogAdapter{item})
		}
	}
	experimentSummary := experiment.BuildSummary(experimentLogs)
	experimentGuardPassed := true
	strategyRegressionRate := 0.0
	for _, item := range experimentSummary {
		if item.ErrorRateDelta > 0.2 {
			experimentGuardPassed = false
			strategyRegressionRate = item.ErrorRateDelta
			break
		}
	}

	releaseSummary := buildReleaseSummary(24*60, since, retrieveLogs)
	releaseGuardPassed := !releaseSummary.RollbackRecommended
	rollbackSuccessRate := 1.0
	if releaseSummary.RollbackRecommended {
		rollbackSuccessRate = 0.0
	}

	risks := make([]string, 0, 6)
	if !costGuardPassed {
		risks = append(risks, "成本门禁失败：每千次问答成本超出阈值")
	}
	if !auditGuardPassed {
		risks = append(risks, "审计门禁失败：审计覆盖率不足")
	}
	if !indexGuardPassed {
		risks = append(risks, "索引门禁失败：collection 健康分过低")
	}
	if !experimentGuardPassed {
		risks = append(risks, "实验门禁失败：candidate 错误率回归过高")
	}
	if !releaseGuardPassed {
		risks = append(risks, "发布门禁失败：release summary 建议回滚")
	}

	resp := governanceGateResponse{
		GeneratedAt:            now,
		Passed:                 costGuardPassed && auditGuardPassed && indexGuardPassed && experimentGuardPassed && releaseGuardPassed,
		CostGuardPassed:        costGuardPassed,
		AuditGuardPassed:       auditGuardPassed,
		IndexGuardPassed:       indexGuardPassed,
		ExperimentGuardPassed:  experimentGuardPassed,
		ReleaseGuardPassed:     releaseGuardPassed,
		CollectionHealthScore:  collectionHealthScore,
		AuditCoverageRate:      auditCoverageRate,
		RollbackSuccessRate:    rollbackSuccessRate,
		StrategyRegressionRate: strategyRegressionRate,
		CostPer1KQueries:       latestCostPer1K,
		Risks:                  risks,
	}
	response.Success(ctx, c, resp)
}
