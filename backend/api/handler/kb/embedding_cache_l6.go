package kb

import (
	"context"
	"time"

	"interview-agents/api/response"

	"github.com/cloudwego/hertz/pkg/app"
)

type embeddingCacheImplementationStep struct {
	Layer        string   `json:"layer"`
	Goal         string   `json:"goal"`
	Delivered    []string `json:"delivered"`
	WhyThisOrder string   `json:"why_this_order"`
	StarterHint  string   `json:"starter_hint"`
}

type embeddingCacheArtifactSummary struct {
	ImplementationGuide string   `json:"implementation_guide"`
	AcceptanceReport    string   `json:"acceptance_report"`
	MeetingBrief        string   `json:"meeting_brief"`
	AdminEndpoints      []string `json:"admin_endpoints"`
}

type embeddingCacheTestSummary struct {
	FocusedCoverage  []string `json:"focused_coverage"`
	AcceptanceChecks []string `json:"acceptance_checks"`
	RecommendedSmoke []string `json:"recommended_smoke"`
}

type embeddingCacheBenefitSummary struct {
	LookupCount          int     `json:"lookup_count"`
	HitCount             int     `json:"hit_count"`
	HitRate              float64 `json:"hit_rate"`
	LookupP95Ms          int64   `json:"lookup_p95_ms"`
	ObservabilityHealthy bool    `json:"observability_healthy"`
}

type embeddingCacheReportResponse struct {
	GeneratedAt           time.Time                          `json:"generated_at"`
	Phase                 string                             `json:"phase"`
	Accepted              bool                               `json:"accepted"`
	Artifacts             embeddingCacheArtifactSummary      `json:"artifacts"`
	ImplementationSummary []embeddingCacheImplementationStep `json:"implementation_summary"`
	TestSummary           embeddingCacheTestSummary          `json:"test_summary"`
	Gate                  embeddingCacheGateResponse         `json:"gate"`
	ReleaseSummary        releaseSummaryResponse             `json:"release_summary"`
	CanaryPlan            []string                           `json:"canary_plan"`
	RollbackPlan          []string                           `json:"rollback_plan"`
	BenefitSummary        embeddingCacheBenefitSummary       `json:"benefit_summary"`
	Risks                 []string                           `json:"risks"`
	NextActions           []string                           `json:"next_actions"`
}

func GetEmbeddingCacheReport(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}
	response.Success(ctx, c, buildEmbeddingCacheReport(time.Now().UTC()))
}

func buildEmbeddingCacheReport(now time.Time) embeddingCacheReportResponse {
	since := now.Add(-24 * time.Hour)
	retrieveLogs, _ := listEmbeddingCacheRetrieveLogs(since, now, nil)
	gate := computeEmbeddingCacheGate(now)
	releaseSummary := buildReleaseSummary(24*60, since, retrieveLogs)
	accepted := gate.Passed && !releaseSummary.RollbackRecommended

	risks := uniqueSemanticCacheStrings(append(append([]string{}, gate.Risks...), releaseSummary.Risks...))
	nextActions := buildEmbeddingCacheNextActions(accepted, gate, releaseSummary)

	return embeddingCacheReportResponse{
		GeneratedAt: now,
		Phase:       "L6-L9",
		Accepted:    accepted,
		Artifacts: embeddingCacheArtifactSummary{
			ImplementationGuide: "docs/embedding-cache-l0-to-l9-implementation-guide.md",
			AcceptanceReport:    "docs/embedding-cache-l6-to-l9-acceptance-report.md",
			MeetingBrief:        "docs/embedding-cache-l0-to-l9-meeting-brief.md",
			AdminEndpoints: []string{
				"/api/admin/kb/embedding-cache/gate",
				"/api/admin/kb/embedding-cache/acceptance",
				"/api/admin/kb/embedding-cache/report",
			},
		},
		ImplementationSummary: embeddingCacheImplementationFlow(),
		TestSummary:           embeddingCacheReportTestSummary(accepted),
		Gate:                  gate,
		ReleaseSummary:        releaseSummary,
		CanaryPlan:            embeddingCacheCanaryPlan(),
		RollbackPlan:          embeddingCacheRollbackPlan(),
		BenefitSummary: embeddingCacheBenefitSummary{
			LookupCount:          gate.LookupCount,
			HitCount:             gate.HitCount,
			HitRate:              gate.HitRate,
			LookupP95Ms:          gate.LookupP95Ms,
			ObservabilityHealthy: gate.ObservabilityGuardPassed,
		},
		Risks:       risks,
		NextActions: nextActions,
	}
}

func embeddingCacheImplementationFlow() []embeddingCacheImplementationStep {
	return []embeddingCacheImplementationStep{
		{
			Layer: "L0",
			Goal:  "确认 Gate 问题与 query embedding 延迟相关，并冻结只优化查询链路、不动入库链路的边界。",
			Delivered: []string{
				"梳理 Gate 未通过的观察口径与基线数据",
				"确认 embedding cache 仅作用于 query/retrieval path",
				"区分 semantic cache 命中收益与 embedding cache 命中收益",
			},
			WhyThisOrder: "边界不先定清楚，后面缓存越做越容易串到入库、回填或其他非目标路径。",
			StarterHint:  "先回答优化谁、不能影响谁，再开始写缓存。",
		},
		{
			Layer: "L1",
			Goal:  "补齐 embedding cache 的配置、默认值、校验与环境变量覆盖。",
			Delivered: []string{
				"新增 enable_cache、cache_ttl_seconds、cache_max_entries",
				"支持 YAML 默认值与 ENV 覆盖",
				"补充 config 测试，确保可一键关闭",
			},
			WhyThisOrder: "没有开关就没有安全上线与回滚能力。",
			StarterHint:  "任何缓存都应该先可控，再追求命中率。",
		},
		{
			Layer: "L2-L3",
			Goal:  "在 storage 层实现通用 query embedding 进程内缓存，并只接到查询链路。",
			Delivered: []string{
				"实现 TTL、最大容量、LRU 风格淘汰和 singleflight 去重",
				"EmbeddingService 同时保留 base embedder 与 query embedder",
				"Retriever 与 semantic cache lookup 走 query embedder，Indexer 继续走 base embedder",
			},
			WhyThisOrder: "先把缓存本体做对，再接链路，能减少误伤面。",
			StarterHint:  "这里的关键不是把缓存包上去，而是只包查询，不包入库。",
		},
		{
			Layer: "L4",
			Goal:  "把 embedding cache 命中、耗时和原因写进日志与调试视图。",
			Delivered: []string{
				"SearchMetrics 新增 embedding_cache_* 观测字段",
				"KBRetrieveLog 持久化 embedding_cache_enabled/hit/lookup_ms/reason",
				"Retrieval debug trace v2 新增 embedding_cache 片段",
			},
			WhyThisOrder: "没有观测，就无法解释为什么快了，也无法支撑后续 Gate 判定。",
			StarterHint:  "命中本身不是结果，可观测才是可上线能力。",
		},
		{
			Layer: "L5",
			Goal:  "新增 embedding cache gate 与 acceptance 接口，让能力可验收、可回滚。",
			Delivered: []string{
				"新增 /embedding-cache/gate 与 /embedding-cache/acceptance",
				"判定命中率、P95、观测完整性与回滚准备度",
				"沉淀 canary plan 与 rollback plan",
			},
			WhyThisOrder: "Gate 依赖前面的日志与观测字段，没有数据就无法判断通过与否。",
			StarterHint:  "先让代码工作，再让上线决策有依据。",
		},
		{
			Layer: "L6-L7",
			Goal:  "补 report 输出与测试，形成后端闭环。",
			Delivered: []string{
				"新增 /embedding-cache/report 汇总报告接口",
				"覆盖 storage、retrieval、model、kb handler 的相关测试",
				"让 report 能汇总 Gate、收益、风险与下一步动作",
			},
			WhyThisOrder: "报告是把分散的事实串成结论，测试是把结论变成可重复验证。",
			StarterHint:  "L6-L7 解决的是‘别人怎么确认你真的做完了’。",
		},
		{
			Layer: "L8-L9",
			Goal:  "补前端页面、导航、日志展示和实施文档，让人能看见、能解释、能交接。",
			Delivered: []string{
				"新增 embedding cache 管理页与 report 展示",
				"Retrieval logs 展示 embedding cache 命中详情",
				"生成中文 MD，总结 L0-L9 全链路实现与验证方式",
			},
			WhyThisOrder: "最后一层不是写更多逻辑，而是把能力真正交到使用者手里。",
			StarterHint:  "技术闭环的最后一步，是让人看得懂、用得上。",
		},
	}
}

func embeddingCacheReportTestSummary(accepted bool) embeddingCacheTestSummary {
	checks := []string{
		"验证 query 重复请求能从 miss 进入 hit 或 singleflight_shared",
		"验证 TTL 过期、容量淘汰和 batch bypass 行为符合预期",
		"验证 query embedder 只接在检索链路，不影响文档入库",
		"验证 retrieve log 与 debug trace 都能解释 embedding cache 命中情况",
		"验证 gate、acceptance、report 三个 admin 接口响应自洽",
	}
	if accepted {
		checks = append(checks, "当前 report 与 gate 结论允许继续做前端展示与后续灰度验证")
	} else {
		checks = append(checks, "当前 report 提示仍需补观察样本或修复风险项，再继续放量")
	}

	return embeddingCacheTestSummary{
		FocusedCoverage: []string{
			"L2-L3: key 归一化、TTL、容量淘汰、singleflight 与查询链路接线",
			"L4: retrieval metrics、retrieve log、debug trace 的 embedding_cache_* 字段贯通",
			"L5: gate 与 acceptance 接口判定逻辑",
			"L6: report 接口汇总收益、风险、后续动作",
			"L8-L9: 前端页面、日志详情、中文总结文档",
		},
		AcceptanceChecks: checks,
		RecommendedSmoke: []string{
			"先跑 go test ./backend/internal/milvus/storage/...",
			"再跑 go test ./backend/internal/milvus/retrieval/... ./backend/internal/model/... ./backend/api/handler/kb/...",
			"最后在前端管理页检查 embedding cache 的 gate、acceptance、report 与 retrieval logs 展示是否一致",
		},
	}
}

func buildEmbeddingCacheNextActions(accepted bool, gate embeddingCacheGateResponse, releaseSummary releaseSummaryResponse) []string {
	if !accepted {
		actions := make([]string, 0, 4)
		if !gate.ObservabilityGuardPassed {
			actions = append(actions, "先补齐 embedding cache 的 retrieve log / debug trace 样本，保证 24h 窗口内字段完整。")
		}
		if !gate.LatencyGuardPassed {
			actions = append(actions, "继续观察高频 query 样本，确认命中请求的 lookup P95 是否还能下降。")
		}
		if !gate.IsolationGuardPassed {
			actions = append(actions, "排查是否有非查询链路错误写入 embedding cache 观测字段。")
		}
		if releaseSummary.RollbackRecommended {
			actions = append(actions, "优先保证主链路稳定，必要时关闭 embedding.enable_cache 后继续观察。")
		}
		if len(actions) == 0 {
			actions = append(actions, "继续补流量样本与验收数据，再重新评估 Gate。")
		}
		return actions
	}

	return []string{
		"继续在重复 query 较多的场景观察 hit rate 与 lookup P95 变化。",
		"让前端侧的 embedding cache 页面作为后续验收与演示入口。",
		"保留关闭开关与日志面板，直到这轮改造复盘完成。",
	}
}
