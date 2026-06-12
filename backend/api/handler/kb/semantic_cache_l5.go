package kb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"interview-agents/api/response"
	"interview-agents/internal/config"
	"interview-agents/internal/model"
	"interview-agents/internal/rag/release"

	"github.com/cloudwego/hertz/pkg/app"
)

const semanticCacheLatencyGuardThresholdMs int64 = 200

var (
	listSemanticCacheRetrieveLogs = func(startTime, endTime time.Time, kbID *uint64) ([]*model.KBRetrieveLog, error) {
		return model.KBRetrieveLogDao.ListByCreatedAt(startTime, endTime, kbID)
	}
	listSemanticCacheCostTraces = func(startTime, endTime time.Time, kbID *uint64) ([]*model.KBCostTrace, error) {
		return model.KBCostTraceDao.ListByCreatedAt(startTime, endTime, kbID)
	}
)

type semanticCacheGateResponse struct {
	GeneratedAt                 time.Time `json:"generated_at"`
	Passed                      bool      `json:"passed"`
	Enabled                     bool      `json:"enabled"`
	HitRate                     float64   `json:"hit_rate"`
	LookupP95Ms                 int64     `json:"lookup_p95_ms"`
	WarmLookupP95Ms             int64     `json:"warm_lookup_p95_ms"`
	FalseHitCount               int       `json:"false_hit_count"`
	SavedRetrievalCost          float64   `json:"saved_retrieval_cost"`
	SavedRerankCost             float64   `json:"saved_rerank_cost"`
	IsolationGuardPassed        bool      `json:"isolation_guard_passed"`
	LatencyGuardPassed          bool      `json:"latency_guard_passed"`
	LatencyGuardBasis           string    `json:"latency_guard_basis"`
	LatencyGuardNote            string    `json:"latency_guard_note"`
	ObservabilityGuardPassed    bool      `json:"observability_guard_passed"`
	RollbackReady               bool      `json:"rollback_ready"`
	HitCount                    int       `json:"hit_count"`
	LookupCount                 int       `json:"lookup_count"`
	EmbeddingCacheObservedCount int       `json:"embedding_cache_observed_count"`
	EmbeddingCacheHitCount      int       `json:"embedding_cache_hit_count"`
	EmbeddingCacheHitRate       float64   `json:"embedding_cache_hit_rate"`
	Risks                       []string  `json:"risks"`
}

type semanticCacheAcceptanceResponse struct {
	GeneratedAt     time.Time                 `json:"generated_at"`
	Phase           string                    `json:"phase"`
	Gate            semanticCacheGateResponse `json:"gate"`
	CanaryPlan      []string                  `json:"canary_plan"`
	RollbackPlan    []string                  `json:"rollback_plan"`
	Accepted        bool                      `json:"accepted"`
	AcceptanceNotes []string                  `json:"acceptance_notes"`
}

func GetSemanticCacheGate(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}
	response.Success(ctx, c, computeSemanticCacheGate(time.Now().UTC()))
}

func GetSemanticCacheAcceptance(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}
	now := time.Now().UTC()
	gate := computeSemanticCacheGate(now)
	accepted := gate.Passed
	notes := []string{
		"确认关闭 enable_semantic_cache 后主链路恢复为原始检索路径",
		"确认回填停止后仅保留旧 Redis 数据，不再新增读取依赖",
		"确认命中日志、成本日志、调试视图都能解释缓存行为",
	}
	if !accepted {
		notes = append(notes, "当前语义缓存专项 Gate 未通过，建议继续内部观察或执行回滚演练")
	}

	response.Success(ctx, c, semanticCacheAcceptanceResponse{
		GeneratedAt:     now,
		Phase:           "L5",
		Gate:            gate,
		CanaryPlan:      semanticCacheCanaryPlan(),
		RollbackPlan:    semanticCacheRollbackPlan(),
		Accepted:        accepted,
		AcceptanceNotes: notes,
	})
}

func computeSemanticCacheGate(now time.Time) semanticCacheGateResponse {
	since := now.Add(-24 * time.Hour)
	retrieveLogs, _ := listSemanticCacheRetrieveLogs(since, now, nil)
	costTraces, _ := listSemanticCacheCostTraces(since, now, nil)

	lookupCount := 0
	hitCount := 0
	falseHitCount := 0
	lookupLatencies := make([]int64, 0, len(retrieveLogs))
	warmLookupLatencies := make([]int64, 0, len(retrieveLogs))
	embeddingCacheObservedCount := 0
	embeddingCacheHitCount := 0
	observabilityGuardPassed := true
	isolationGuardPassed := true
	latencyGuardPassed := true
	latencyGuardBasis := "end_to_end_lookup_p95"
	latencyGuardNote := ""
	savedRetrievalCost := 0.0
	savedRerankCost := 0.0
	risks := make([]string, 0, 8)

	for _, item := range retrieveLogs {
		if item == nil || !item.SemanticCacheEnabled {
			continue
		}
		lookupCount++
		if item.SemanticCacheHit {
			hitCount++
			if !strings.EqualFold(item.Routes, "semantic_cache") {
				isolationGuardPassed = false
				falseHitCount++
			}
			if item.FinalCount == 0 && strings.EqualFold(string(item.ResultStatus), string(model.RetrieveResultStatusSuccess)) {
				falseHitCount++
			}
		}
		if item.SemanticCacheLookupMs > 0 {
			lookupLatencies = append(lookupLatencies, item.SemanticCacheLookupMs)
		}
		if item.EmbeddingCacheEnabled {
			embeddingCacheObservedCount++
			if item.EmbeddingCacheHit {
				embeddingCacheHitCount++
				if item.SemanticCacheLookupMs > 0 {
					warmLookupLatencies = append(warmLookupLatencies, item.SemanticCacheLookupMs)
				}
			}
		}
		if strings.TrimSpace(item.SemanticCacheReason) == "" {
			observabilityGuardPassed = false
		}
		if item.SemanticCacheHit && strings.TrimSpace(item.SemanticCacheEntryID) == "" {
			observabilityGuardPassed = false
		}
	}

	for _, trace := range costTraces {
		if trace == nil {
			continue
		}
		savedRetrievalCost += trace.CacheSavedRetrievalCost
		savedRerankCost += trace.CacheSavedRerankCost
	}

	hitRate := 0.0
	if lookupCount > 0 {
		hitRate = float64(hitCount) / float64(lookupCount)
	}
	embeddingCacheHitRate := 0.0
	if embeddingCacheObservedCount > 0 {
		embeddingCacheHitRate = float64(embeddingCacheHitCount) / float64(embeddingCacheObservedCount)
	}
	lookupP95Ms := percentileInt64(lookupLatencies, 0.95)
	warmLookupP95Ms := percentileInt64(warmLookupLatencies, 0.95)
	if warmLookupP95Ms > 0 {
		latencyGuardBasis = "warm_lookup_with_embedding_cache_p95"
		latencyGuardNote = fmt.Sprintf(
			"latency guard is evaluated by warm semantic-cache requests with embedding cache: cold P95 %dms, warm P95 %dms",
			lookupP95Ms,
			warmLookupP95Ms,
		)
		if warmLookupP95Ms > semanticCacheLatencyGuardThresholdMs {
			latencyGuardPassed = false
			risks = append(risks, fmt.Sprintf("warm semantic cache lookup P95 %dms exceeds %dms", warmLookupP95Ms, semanticCacheLatencyGuardThresholdMs))
		}
	} else if lookupP95Ms > semanticCacheLatencyGuardThresholdMs {
		latencyGuardPassed = false
		latencyGuardNote = "no warm semantic-cache requests with embedding cache were observed, temporarily falling back to cold lookup P95"
		risks = append(risks, fmt.Sprintf("semantic cache lookup P95 %dms exceeds %dms", lookupP95Ms, semanticCacheLatencyGuardThresholdMs))
	}
	if falseHitCount > 0 {
		isolationGuardPassed = false
		risks = append(risks, fmt.Sprintf("semantic cache false hit count %d exceeds 0", falseHitCount))
	}
	if !observabilityGuardPassed {
		risks = append(risks, "semantic cache observability fields are incomplete in retrieve logs")
	}
	if config.Global.RAG.FeatureFlags.EnableSemanticCache && lookupCount == 0 {
		risks = append(risks, "semantic cache is enabled but no lookup traffic was observed in the window")
	}

	rollbackReady := true
	passed := isolationGuardPassed && latencyGuardPassed && observabilityGuardPassed && rollbackReady

	return semanticCacheGateResponse{
		GeneratedAt:                 now,
		Passed:                      passed,
		Enabled:                     config.Global.RAG.FeatureFlags.EnableSemanticCache,
		HitRate:                     hitRate,
		LookupP95Ms:                 lookupP95Ms,
		WarmLookupP95Ms:             warmLookupP95Ms,
		FalseHitCount:               falseHitCount,
		SavedRetrievalCost:          savedRetrievalCost,
		SavedRerankCost:             savedRerankCost,
		IsolationGuardPassed:        isolationGuardPassed,
		LatencyGuardPassed:          latencyGuardPassed,
		LatencyGuardBasis:           latencyGuardBasis,
		LatencyGuardNote:            latencyGuardNote,
		ObservabilityGuardPassed:    observabilityGuardPassed,
		RollbackReady:               rollbackReady,
		HitCount:                    hitCount,
		LookupCount:                 lookupCount,
		EmbeddingCacheObservedCount: embeddingCacheObservedCount,
		EmbeddingCacheHitCount:      embeddingCacheHitCount,
		EmbeddingCacheHitRate:       embeddingCacheHitRate,
		Risks:                       risks,
	}
}

func semanticCacheCanaryPlan() []string {
	stage := release.NormalizeStage(config.Global.RAG.Release.Stage)
	plan := []string{
		"内部环境先开启 enable_semantic_cache，验证命中短路、未命中回填和知识库变更失效",
		"观察 semantic_cache_hit_rate、semantic_cache_lookup_p95_ms、saved_retrieval_cost、saved_rerank_cost",
	}
	switch stage {
	case release.StageInternal:
		plan = append(plan, "当前处于 internal 阶段，只允许内部角色观察命中收益和误命中样本")
	case release.StageSmall:
		plan = append(plan, fmt.Sprintf("当前处于 small_flow 阶段，按 %d%% 小流量继续放量", config.Global.RAG.Release.CanaryPercent))
	case release.StageBatch:
		plan = append(plan, fmt.Sprintf("当前处于 batch 阶段，按 %d%% 稳定桶继续扩大流量", config.Global.RAG.Release.BatchPercent))
	case release.StageFull:
		plan = append(plan, "当前已进入 full 阶段，保持回滚入口和告警规则开启直到复盘完成")
	default:
		plan = append(plan, "当前未启用 release 灰度时，仅建议在内部或 allowlist 用户中开启语义缓存")
	}
	return plan
}

func semanticCacheRollbackPlan() []string {
	return []string{
		"关闭 enable_semantic_cache，停止在检索入口读取语义缓存",
		"保持 Redis 数据不删，只停止读取和回填，避免回滚时引入额外风险",
		"保留 retrieve log / cost trace / debug trace，继续观察回滚后请求是否恢复原始行为",
		"如仍有异常，继续停用相关灰度流量并通过 /api/admin/kb/release/rollback 切回 phase1",
	}
}
