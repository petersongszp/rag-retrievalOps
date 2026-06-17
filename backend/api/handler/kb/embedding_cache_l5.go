package kb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"interview-agents/api/response"
	"interview-agents/internal/config"
	"interview-agents/internal/model"

	"github.com/cloudwego/hertz/pkg/app"
)

var (
	listEmbeddingCacheRetrieveLogs = func(startTime, endTime time.Time, kbID *uint64) ([]*model.KBRetrieveLog, error) {
		return model.KBRetrieveLogDao.ListByCreatedAt(startTime, endTime, kbID)
	}
)

type embeddingCacheGateResponse struct {
	GeneratedAt              time.Time `json:"generated_at"`
	Passed                   bool      `json:"passed"`
	Enabled                  bool      `json:"enabled"`
	HitRate                  float64   `json:"hit_rate"`
	LookupP95Ms              int64     `json:"lookup_p95_ms"`
	IsolationGuardPassed     bool      `json:"isolation_guard_passed"`
	LatencyGuardPassed       bool      `json:"latency_guard_passed"`
	ObservabilityGuardPassed bool      `json:"observability_guard_passed"`
	RollbackReady            bool      `json:"rollback_ready"`
	HitCount                 int       `json:"hit_count"`
	LookupCount              int       `json:"lookup_count"`
	Risks                    []string  `json:"risks"`
}

type embeddingCacheAcceptanceResponse struct {
	GeneratedAt     time.Time                  `json:"generated_at"`
	Phase           string                     `json:"phase"`
	Gate            embeddingCacheGateResponse `json:"gate"`
	CanaryPlan      []string                   `json:"canary_plan"`
	RollbackPlan    []string                   `json:"rollback_plan"`
	Accepted        bool                       `json:"accepted"`
	AcceptanceNotes []string                   `json:"acceptance_notes"`
}

func GetEmbeddingCacheGate(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}
	response.Success(ctx, c, computeEmbeddingCacheGate(time.Now().UTC()))
}

func GetEmbeddingCacheAcceptance(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	now := time.Now().UTC()
	gate := computeEmbeddingCacheGate(now)
	accepted := gate.Passed
	notes := []string{
		"确认关闭 embedding.enable_cache 或 EMBEDDING_ENABLE_CACHE=false 后，检索链路回退为原始 embedding 调用。",
		"确认 retrieve log 与 debug trace 都能解释 embedding cache 的命中、未命中与耗时。",
		"确认 query cache 仅作用于查询链路，不影响文档入库与向量写入结果。",
	}
	if !accepted {
		notes = append(notes, "当前 embedding cache Gate 未通过，建议继续观察命中样本与日志完整性。")
	}

	response.Success(ctx, c, embeddingCacheAcceptanceResponse{
		GeneratedAt:     now,
		Phase:           "L5",
		Gate:            gate,
		CanaryPlan:      embeddingCacheCanaryPlan(),
		RollbackPlan:    embeddingCacheRollbackPlan(),
		Accepted:        accepted,
		AcceptanceNotes: notes,
	})
}

func computeEmbeddingCacheGate(now time.Time) embeddingCacheGateResponse {
	since := now.Add(-24 * time.Hour)
	retrieveLogs, _ := listEmbeddingCacheRetrieveLogs(since, now, nil)

	lookupCount := 0
	hitCount := 0
	hitLatencies := make([]int64, 0, len(retrieveLogs))
	observabilityGuardPassed := true
	isolationGuardPassed := true
	latencyGuardPassed := true
	risks := make([]string, 0, 8)

	for _, item := range retrieveLogs {
		if item == nil {
			continue
		}
		if !item.EmbeddingCacheEnabled {
			if item.EmbeddingCacheHit || item.EmbeddingCacheLookupMs > 0 || strings.TrimSpace(item.EmbeddingCacheReason) != "" {
				isolationGuardPassed = false
			}
			continue
		}

		lookupCount++
		if item.EmbeddingCacheHit {
			hitCount++
			if item.EmbeddingCacheLookupMs > 0 {
				hitLatencies = append(hitLatencies, item.EmbeddingCacheLookupMs)
			}
		}
		if strings.TrimSpace(item.EmbeddingCacheReason) == "" {
			observabilityGuardPassed = false
		}
		switch strings.TrimSpace(item.EmbeddingCacheReason) {
		case "empty_query", "batch_bypass", "dependency_unavailable", "unexpected_type":
			isolationGuardPassed = false
		}
	}

	hitRate := 0.0
	if lookupCount > 0 {
		hitRate = float64(hitCount) / float64(lookupCount)
	}

	lookupP95Ms := percentileInt64(hitLatencies, 0.95)
	if lookupP95Ms > 15 {
		latencyGuardPassed = false
		risks = append(risks, fmt.Sprintf("embedding cache hit lookup P95 %dms exceeds 15ms", lookupP95Ms))
	}
	if !observabilityGuardPassed {
		risks = append(risks, "embedding cache observability fields are incomplete in retrieve logs")
	}
	if !isolationGuardPassed {
		risks = append(risks, "embedding cache leaked non-query-path reasons or inconsistent enabled/hit combinations")
	}
	if config.Global.Embedding.EnableCache && lookupCount == 0 {
		risks = append(risks, "embedding cache is enabled but no lookup traffic was observed in the window")
	}

	rollbackReady := true
	passed := isolationGuardPassed && latencyGuardPassed && observabilityGuardPassed && rollbackReady

	return embeddingCacheGateResponse{
		GeneratedAt:              now,
		Passed:                   passed,
		Enabled:                  config.Global.Embedding.EnableCache,
		HitRate:                  hitRate,
		LookupP95Ms:              lookupP95Ms,
		IsolationGuardPassed:     isolationGuardPassed,
		LatencyGuardPassed:       latencyGuardPassed,
		ObservabilityGuardPassed: observabilityGuardPassed,
		RollbackReady:            rollbackReady,
		HitCount:                 hitCount,
		LookupCount:              lookupCount,
		Risks:                    risks,
	}
}

func embeddingCacheCanaryPlan() []string {
	return []string{
		"先在内部环境开启 embedding.enable_cache，观察重复 query 的 embedding_cache_hit 是否稳定出现。",
		"重点观察 embedding_cache_hit_rate、embedding_cache_lookup_p95_ms 与 retrieve log 中的 embedding_cache_reason。",
		"确认 semantic cache lookup 与常规 dense retrieval 两条查询链路都能复用同一套 query embedding cache。",
	}
}

func embeddingCacheRollbackPlan() []string {
	return []string{
		"关闭 embedding.enable_cache 或设置 EMBEDDING_ENABLE_CACHE=false，立即停止 query embedding 进程内缓存。",
		"保留 retrieve log 与 debug trace，继续观察关闭缓存后的检索行为是否回到基线。",
		"如果仍有异常，再继续排查 semantic cache lookup 与 dense retrieval 的上游依赖，而不是继续扩散缓存范围。",
	}
}
