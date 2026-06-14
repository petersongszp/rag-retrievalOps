package retrieval

import (
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestDecideStrategicTopK_CliffDistributionShrinksTopK(t *testing.T) {
	cfg := DynamicTopKConfig{
		Enabled:              true,
		MinTopK:              3,
		MaxTopK:              8,
		TokenBudget:          0,
		MinAnswerChunks:      1,
		StrategicEnabled:     true,
		StrategicMinTopK:     2,
		StrategicMaxTopK:     6,
		StrategicBudgetRatio: 0.6,
	}
	docs := []*schema.Document{
		makeStrategicDoc("doc-1", "p-1", 0.93, 120),
		makeStrategicDoc("doc-2", "p-1", 0.62, 120),
		makeStrategicDoc("doc-3", "p-1", 0.41, 120),
		makeStrategicDoc("doc-4", "p-1", 0.39, 120),
	}

	ruleDecision := DecideDynamicTopK("compare go interface design tradeoff overview", 8, 0, cfg)
	decision := DecideStrategicTopK("compare go interface design tradeoff overview", 8, 0, docs, cfg)

	if decision.PolicyVersion != TopKPolicyVersionStrategic {
		t.Fatalf("expected strategic policy version, got %q", decision.PolicyVersion)
	}
	if decision.FinalTopK >= ruleDecision.FinalTopK {
		t.Fatalf("expected strategic topk to shrink below rule decision, got strategic=%d rule=%d", decision.FinalTopK, ruleDecision.FinalTopK)
	}
	if decision.FinalTopK > 4 {
		t.Fatalf("expected cliff distribution to keep topk tight, got %d", decision.FinalTopK)
	}
	if !strings.Contains(decision.DecisionReason, "score_cliff") {
		t.Fatalf("expected decision reason to include score_cliff, got %q", decision.DecisionReason)
	}
}

func TestDecideStrategicTopK_FlatDistributionExpandsWithinBudgetRatio(t *testing.T) {
	cfg := DynamicTopKConfig{
		Enabled:              true,
		MinTopK:              3,
		MaxTopK:              8,
		TokenBudget:          200,
		MinAnswerChunks:      2,
		StrategicEnabled:     true,
		StrategicMinTopK:     2,
		StrategicMaxTopK:     7,
		StrategicBudgetRatio: 0.5,
	}
	docs := []*schema.Document{
		makeStrategicDoc("doc-1", "p-1", 0.82, 80),
		makeStrategicDoc("doc-2", "p-2", 0.81, 80),
		makeStrategicDoc("doc-3", "p-3", 0.80, 80),
		makeStrategicDoc("doc-4", "p-4", 0.79, 80),
		makeStrategicDoc("doc-5", "p-5", 0.78, 80),
	}

	ruleDecision := DecideDynamicTopK("go interface", 8, 0, cfg)
	decision := DecideStrategicTopK("go interface", 8, 0, docs, cfg)

	if decision.PolicyVersion != TopKPolicyVersionStrategic {
		t.Fatalf("expected strategic policy version, got %q", decision.PolicyVersion)
	}
	if decision.TokenBudget != 100 {
		t.Fatalf("expected strategic token budget ratio to apply, got %d", decision.TokenBudget)
	}
	if decision.FinalTopK < ruleDecision.FinalTopK {
		t.Fatalf("expected flat diverse evidence to keep or expand topk, got strategic=%d rule=%d", decision.FinalTopK, ruleDecision.FinalTopK)
	}
	if !strings.Contains(decision.ScoreDistribution, "flat") {
		t.Fatalf("expected flat score distribution summary, got %q", decision.ScoreDistribution)
	}
	if decision.TokenBudgetRemaining != 0 {
		t.Fatalf("expected flat case to consume the strategic token budget, got remaining=%d", decision.TokenBudgetRemaining)
	}
	if decision.EstimatedContextTokens != 100 {
		t.Fatalf("expected estimated context tokens to match the effective strategic budget, got %d", decision.EstimatedContextTokens)
	}
}

func TestApplyTokenBudgetGuard_TracksRemainingBudget(t *testing.T) {
	cfg := DynamicTopKConfig{
		MinAnswerChunks: 1,
	}
	docs := []*schema.Document{
		makeStrategicDoc("doc-1", "p-1", 0.91, 40),
		makeStrategicDoc("doc-2", "p-2", 0.88, 40),
		makeStrategicDoc("doc-3", "p-3", 0.80, 40),
	}
	decision := TopKDecision{
		FinalTopK:   3,
		TokenBudget: 25,
	}

	budgeted, out := ApplyTokenBudgetGuard(docs, decision, cfg)
	if len(budgeted) != 2 {
		t.Fatalf("expected two docs to fit the token budget, got %d", len(budgeted))
	}
	if out.TruncateReason != TruncateReasonTokenBudget {
		t.Fatalf("expected token budget truncate reason, got %q", out.TruncateReason)
	}
	if out.TokenBudgetRemaining != 5 {
		t.Fatalf("expected token budget remaining to be 5, got %d", out.TokenBudgetRemaining)
	}
	if out.EstimatedContextTokens != 20 {
		t.Fatalf("expected estimated context tokens to be 20, got %d", out.EstimatedContextTokens)
	}
}

func TestApplyScoreCliffGuard_ShortPreciseQueryKeepsOnlyStrongMatch(t *testing.T) {
	docs := []*schema.Document{
		makeStrategicDocWithContent("doc-1", "p-1", 0.70, "### 1.3 消息收发TPS计算规则\n\n针对高级特性消息，事务消息的调用次数需要在普通消息基础上乘以5倍倍率。"),
		makeStrategicDocWithContent("doc-2", "p-1", 0.38, "### 1.2 超过规格限制后行为\n\n超过弹性能力上限后，实例还是会被限流。"),
		makeStrategicDocWithContent("doc-3", "p-2", 0.37, "购买实例时，选择指定的计算规格即可创建实例，此时无需支付费用。"),
	}
	decision := TopKDecision{
		FinalTopK:      3,
		DecisionReason: "short_precise_query",
	}

	filtered, out := ApplyScoreCliffGuard("高级特性消息", docs, decision)

	if len(filtered) != 1 {
		t.Fatalf("expected score cliff guard to keep only the strong title/content match, got %d", len(filtered))
	}
	if filtered[0].ID != "doc-1" {
		t.Fatalf("expected doc-1 to be retained, got %s", filtered[0].ID)
	}
	if out.FinalTopK != 1 {
		t.Fatalf("expected final topk to shrink to 1, got %d", out.FinalTopK)
	}
	if out.TruncateReason != TruncateReasonScoreCliff {
		t.Fatalf("expected score cliff truncate reason, got %q", out.TruncateReason)
	}
	if !strings.Contains(out.DecisionReason, "score_cliff") {
		t.Fatalf("expected decision reason to include score_cliff, got %q", out.DecisionReason)
	}
}

func makeStrategicDoc(id string, parentID string, score float64, contentLen int) *schema.Document {
	return &schema.Document{
		ID:      id,
		Content: strings.Repeat("a", contentLen),
		MetaData: map[string]interface{}{
			"parent_id":    parentID,
			"rerank_score": score,
			"fusion_score": score,
			"score":        score,
		},
	}
}

func makeStrategicDocWithContent(id string, parentID string, score float64, content string) *schema.Document {
	doc := makeStrategicDoc(id, parentID, score, len(content))
	doc.Content = content
	return doc
}
