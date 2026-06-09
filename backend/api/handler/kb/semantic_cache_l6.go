package kb

import (
	"context"
	"time"

	"interview-agents/api/response"

	"github.com/cloudwego/hertz/pkg/app"
)

type semanticCacheImplementationStep struct {
	Layer        string   `json:"layer"`
	Goal         string   `json:"goal"`
	Delivered    []string `json:"delivered"`
	WhyThisOrder string   `json:"why_this_order"`
	StarterHint  string   `json:"starter_hint"`
}

type semanticCacheArtifactSummary struct {
	ImplementationGuide string   `json:"implementation_guide"`
	AcceptanceReport    string   `json:"acceptance_report"`
	MeetingBrief        string   `json:"meeting_brief"`
	AdminEndpoints      []string `json:"admin_endpoints"`
}

type semanticCacheTestSummary struct {
	FocusedCoverage  []string `json:"focused_coverage"`
	AcceptanceChecks []string `json:"acceptance_checks"`
	RecommendedSmoke []string `json:"recommended_smoke"`
}

type semanticCacheBenefitSummary struct {
	LookupCount          int     `json:"lookup_count"`
	HitCount             int     `json:"hit_count"`
	HitRate              float64 `json:"hit_rate"`
	LookupP95Ms          int64   `json:"lookup_p95_ms"`
	FalseHitCount        int     `json:"false_hit_count"`
	SavedRetrievalCost   float64 `json:"saved_retrieval_cost"`
	SavedRerankCost      float64 `json:"saved_rerank_cost"`
	TotalSavedCost       float64 `json:"total_saved_cost"`
	ObservabilityHealthy bool    `json:"observability_healthy"`
}

type semanticCacheReportResponse struct {
	GeneratedAt           time.Time                         `json:"generated_at"`
	Phase                 string                            `json:"phase"`
	Accepted              bool                              `json:"accepted"`
	Artifacts             semanticCacheArtifactSummary      `json:"artifacts"`
	ImplementationSummary []semanticCacheImplementationStep `json:"implementation_summary"`
	TestSummary           semanticCacheTestSummary          `json:"test_summary"`
	Gate                  semanticCacheGateResponse         `json:"gate"`
	ReleaseSummary        releaseSummaryResponse            `json:"release_summary"`
	CanaryPlan            []string                          `json:"canary_plan"`
	RollbackPlan          []string                          `json:"rollback_plan"`
	BenefitSummary        semanticCacheBenefitSummary       `json:"benefit_summary"`
	Risks                 []string                          `json:"risks"`
	NextActions           []string                          `json:"next_actions"`
}

func GetSemanticCacheReport(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}
	response.Success(ctx, c, buildSemanticCacheReport(time.Now().UTC()))
}

func buildSemanticCacheReport(now time.Time) semanticCacheReportResponse {
	since := now.Add(-24 * time.Hour)
	retrieveLogs, _ := listSemanticCacheRetrieveLogs(since, now, nil)
	gate := computeSemanticCacheGate(now)
	releaseSummary := buildReleaseSummary(24*60, since, retrieveLogs)
	accepted := gate.Passed && !releaseSummary.RollbackRecommended

	risks := uniqueSemanticCacheStrings(append(append([]string{}, gate.Risks...), releaseSummary.Risks...))
	nextActions := buildSemanticCacheNextActions(accepted, gate, releaseSummary)

	return semanticCacheReportResponse{
		GeneratedAt: now,
		Phase:       "L6",
		Accepted:    accepted,
		Artifacts: semanticCacheArtifactSummary{
			ImplementationGuide: "backend/docs/rag/semantic-cache-implementation-tutorial.md",
			AcceptanceReport:    "backend/docs/rag/semantic-cache-l6-acceptance-report.md",
			MeetingBrief:        "backend/docs/rag/semantic-cache-l0-to-l6-meeting-brief.md",
			AdminEndpoints: []string{
				"/api/admin/kb/semantic-cache/gate",
				"/api/admin/kb/semantic-cache/acceptance",
				"/api/admin/kb/semantic-cache/report",
			},
		},
		ImplementationSummary: semanticCacheImplementationFlow(),
		TestSummary:           semanticCacheReportTestSummary(accepted),
		Gate:                  gate,
		ReleaseSummary:        releaseSummary,
		CanaryPlan:            semanticCacheCanaryPlan(),
		RollbackPlan:          semanticCacheRollbackPlan(),
		BenefitSummary: semanticCacheBenefitSummary{
			LookupCount:          gate.LookupCount,
			HitCount:             gate.HitCount,
			HitRate:              gate.HitRate,
			LookupP95Ms:          gate.LookupP95Ms,
			FalseHitCount:        gate.FalseHitCount,
			SavedRetrievalCost:   gate.SavedRetrievalCost,
			SavedRerankCost:      gate.SavedRerankCost,
			TotalSavedCost:       gate.SavedRetrievalCost + gate.SavedRerankCost,
			ObservabilityHealthy: gate.ObservabilityGuardPassed,
		},
		Risks:       risks,
		NextActions: nextActions,
	}
}

func semanticCacheImplementationFlow() []semanticCacheImplementationStep {
	return []semanticCacheImplementationStep{
		{
			Layer: "L0",
			Goal:  "先冻结边界、开关和命中契约，避免一上来就把缓存做成跨租户或跨策略污染。",
			Delivered: []string{
				"定义 enable_semantic_cache、阈值、TTL、候选数量等配置",
				"冻结 tenant_id、kb_ids、strategy_version、query_type、top_k 的命中边界",
				"约定哪些请求必须绕过缓存",
			},
			WhyThisOrder: "缓存最怕的不是慢，而是错。边界不先定，后面的 Redis、命中、回填都会建立在不安全前提上。",
			StarterHint:  "把它理解成先写交通规则，再让车上路。",
		},
		{
			Layer: "L1",
			Goal:  "把缓存项长什么样、Redis 里怎么存、怎么按 scope 找候选先定义好。",
			Delivered: []string{
				"定义缓存 entry、lookup result 和 scope",
				"建立 payload、候选索引、TTL、按知识库删除协议",
				"让 L2/L3 可以在同一份协议上读写缓存",
			},
			WhyThisOrder: "L2 要读缓存、L3 要写缓存，它们都依赖同一份数据结构。先定协议，后面读写才不会各说各话。",
			StarterHint:  "把它理解成先定数据库表结构，再写业务代码。",
		},
		{
			Layer: "L2",
			Goal:  "在检索入口增加缓存查询与短路返回，让命中请求直接复用历史结果。",
			Delivered: []string{
				"检索入口先判断当前请求能不能查缓存",
				"生成 query embedding，拉候选，算相似度，筛选最高命中项",
				"命中后校验 scope、top_k 和 payload，再直接返回缓存结果",
			},
			WhyThisOrder: "只有先把读路径跑通，我们才知道系统能不能安全命中；没有消费方，回填再多数据也没人用。",
			StarterHint:  "这里是在主流程前面加一个更便宜的岔路口。",
		},
		{
			Layer: "L3",
			Goal:  "在未命中时回填结果，并在知识库变化时及时失效和清理旧缓存。",
			Delivered: []string{
				"只对安全、成功、非空的结果执行回填",
				"把本次检索结果写回 Redis，供后续相似请求复用",
				"支持按知识库失效，配合 TTL 自动过期",
			},
			WhyThisOrder: "先有稳定的命中读路径，再写回填，才能确保写进去的数据真会被后续命中，而且不会把脏结果放大。",
			StarterHint:  "L2 负责先查旧答案，L3 负责把这次新答案存起来。",
		},
		{
			Layer: "L4",
			Goal:  "把命中、耗时、节省的成本和调试信息全部记下来，方便解释缓存行为。",
			Delivered: []string{
				"retrieve log 记录 enabled、hit、reason、entry_id、similarity、lookup_ms",
				"cost trace 记录 cache saved retrieval/rerank cost",
				"debug trace 能把这次请求为什么命中或未命中讲清楚",
			},
			WhyThisOrder: "没有 L2/L3 的真实行为，就没有可观察对象。先让缓存跑起来，再补可观测，数据才有意义。",
			StarterHint:  "如果线上出了问题，L4 就是我们第一时间看懂现场的工具箱。",
		},
		{
			Layer: "L5",
			Goal:  "把缓存能力变成可以灰度、可以 Gate、可以回滚的上线能力。",
			Delivered: []string{
				"新增 semantic-cache gate 与 acceptance 接口",
				"检查命中率、lookup P95、误命中、收益、可观测性、回滚准备度",
				"生成灰度计划与回滚步骤",
			},
			WhyThisOrder: "Gate 要基于 L4 的日志和成本数据来判断；如果前面没有可观测数据，这里的通过与否就没有依据。",
			StarterHint:  "L5 解决的不是能不能写代码，而是敢不敢上线。",
		},
		{
			Layer: "L6",
			Goal:  "把前面做过的事情整理成文档、验收报告和会议材料，形成可交付闭环。",
			Delivered: []string{
				"新增 semantic-cache report 接口",
				"输出 L6 验收报告文档",
				"输出从 L0 到 L6 的会议讲稿文档",
			},
			WhyThisOrder: "文档收口必须放在最后，因为它要引用 L0-L5 已经落地的代码、测试结果、灰度与回滚事实。",
			StarterHint:  "L6 不是新功能，而是把前面零散证据收成一份能交付的答案。",
		},
	}
}

func semanticCacheReportTestSummary(accepted bool) semanticCacheTestSummary {
	checks := []string{
		"验证开关关闭时检索主链路保持原行为",
		"验证命中只发生在正确 scope 与 exact top_k 下",
		"验证未命中只回填安全、成功、非空结果",
		"验证知识库变更后旧缓存可按 kb 失效",
		"验证 retrieve log、cost trace、debug trace 都能解释缓存行为",
	}
	if accepted {
		checks = append(checks, "当前 Gate 通过，可按灰度计划继续推进验收")
	} else {
		checks = append(checks, "当前 Gate 未完全通过，先修风险项，再继续放量")
	}

	return semanticCacheTestSummary{
		FocusedCoverage: []string{
			"L2：bypass reason、exact top_k、scope 过滤后的短路返回",
			"L3：unsafe result 不回填、成功结果回填、按 knowledge base 失效",
			"L4：成本节省记账、结构化 debug trace 展示缓存命中信息",
			"L5：gate 与 acceptance 接口返回上线判定、灰度计划、回滚计划",
			"L6：report 接口汇总实现、测试、收益、风险和下一步动作",
		},
		AcceptanceChecks: checks,
		RecommendedSmoke: []string{
			"先跑 kb 包内 semantic-cache 相关单测",
			"再跑 go test ./... -run TestDoesNotExist 做全量编译检查",
			"最后在本地用 admin 接口查看 gate、acceptance、report 三个响应是否自洽",
		},
	}
}

func buildSemanticCacheNextActions(accepted bool, gate semanticCacheGateResponse, releaseSummary releaseSummaryResponse) []string {
	if !accepted {
		actions := make([]string, 0, 4)
		if !gate.IsolationGuardPassed {
			actions = append(actions, "优先排查误命中样本，确认 scope、top_k、payload 过滤是否仍有遗漏")
		}
		if !gate.ObservabilityGuardPassed {
			actions = append(actions, "补齐 retrieve log / cost trace / debug trace 缺失字段，再重新观察 24 小时窗口")
		}
		if !gate.LatencyGuardPassed {
			actions = append(actions, "检查 embedding、Redis 候选数量和 lookup 路径，先把 lookup P95 拉回阈值内")
		}
		if releaseSummary.RollbackRecommended {
			actions = append(actions, "按回滚计划先降级或关闭 enable_semantic_cache，确保主链路稳定")
		}
		if len(actions) == 0 {
			actions = append(actions, "继续补采样和验证，再决定是否进入下一阶段灰度")
		}
		return actions
	}

	return []string{
		"按当前灰度阶段继续观察命中率、lookup P95 和节省成本曲线",
		"沉淀高频 query 样本，为下一阶段 Answer Cache 或多级缓存提供输入",
		"保留回滚入口和观测面板，直到本轮复盘完成",
	}
}

func uniqueSemanticCacheStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}
