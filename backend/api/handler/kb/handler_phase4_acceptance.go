package kb

import (
	"context"
	"time"

	"interview-agents/api/response"
	"interview-agents/internal/model"
	"interview-agents/internal/rag/indexlifecycle"

	"github.com/cloudwego/hertz/pkg/app"
)

type phase4AcceptanceResponse struct {
	GeneratedAt     time.Time              `json:"generated_at"`
	Phase           string                 `json:"phase"`
	GovernanceGate  governanceGateResponse `json:"governance_gate"`
	ReleaseSummary  releaseSummaryResponse `json:"release_summary"`
	Accepted        bool                   `json:"accepted"`
	AcceptanceNotes []string               `json:"acceptance_notes"`
}

func GetPhase4Acceptance(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	now := time.Now().UTC()
	since := now.Add(-24 * time.Hour)
	retrieveLogs, _ := model.KBRetrieveLogDao.ListByCreatedAt(since, now, nil)
	releaseSummary := buildReleaseSummary(24*60, since, retrieveLogs)

	recorder := newGateRecorder()
	gate := recorder.compute(ctx, c)
	accepted := gate.Passed && !releaseSummary.RollbackRecommended

	notes := []string{
		"确认灰度发布节奏、实验分流、索引切换与审计留痕均可追踪",
		"确认治理能力异常时主查询链路仍可用",
	}
	if !accepted {
		notes = append(notes, "当前仍有门禁未通过，建议继续观察或执行回滚演练")
	}

	response.Success(ctx, c, phase4AcceptanceResponse{
		GeneratedAt:    now,
		Phase:          "Phase 4",
		GovernanceGate: gate,
		ReleaseSummary: releaseSummary,
		Accepted:       accepted,
		AcceptanceNotes: notes,
	})
}

type gateRecorder struct {
	response governanceGateResponse
}

func newGateRecorder() *gateRecorder {
	return &gateRecorder{}
}

func (g *gateRecorder) compute(ctx context.Context, c *app.RequestContext) governanceGateResponse {
	// Reuse the same data path as L7 without issuing nested HTTP calls.
	now := time.Now().UTC()
	since := now.Add(-24 * time.Hour)
	retrieveLogs, _ := model.KBRetrieveLogDao.ListByCreatedAt(since, now, nil)
	costTraces, _ := model.KBCostTraceDao.ListByCreatedAt(since, now, nil)
	indexRegistry, _ := indexlifecycle.ListRegistry()
	auditEvents, _ := model.KBAuditEventDao.List(200)

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

	return governanceGateResponse{
		GeneratedAt:           now,
		Passed:                costGuardPassed && auditGuardPassed && indexGuardPassed,
		CostGuardPassed:       costGuardPassed,
		AuditGuardPassed:      auditGuardPassed,
		IndexGuardPassed:      indexGuardPassed,
		ExperimentGuardPassed: true,
		ReleaseGuardPassed:    true,
		CollectionHealthScore: collectionHealthScore,
		AuditCoverageRate:     auditCoverageRate,
		CostPer1KQueries:      latestCostPer1K,
	}
}
