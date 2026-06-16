package retrieval

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestApplyTitleMatchBoostPromotesHierarchyTitleMatch(t *testing.T) {
	docs := []*schema.Document{
		{
			ID:      "doc-5-child-005",
			Content: "计算规格费用 = 实例购买后的服务时长（小时） × 规格单价（元/小时）",
			MetaData: map[string]interface{}{
				"document_id":    uint64(5),
				"chunk_id":       "doc-5-child-005",
				"section_title":  "2. 计费规则",
				"hierarchy_path": "2. 计费规则",
				"score":          0.70,
			},
		},
		{
			ID:      "doc-5-child-001",
			Content: "### 1.1 计算规格能力约束\n\n云消息队列 RocketMQ 版计算规格的大小，实际是处理消息收发的TPS上限。",
			MetaData: map[string]interface{}{
				"document_id":    uint64(5),
				"chunk_id":       "doc-5-child-001",
				"section_title":  "1.1 计算规格能力约束",
				"hierarchy_path": "1. 计算规格说明 > 1.1 计算规格能力约束",
				"score":          0.52,
			},
		},
	}

	boosted := applyTitleMatchBoost("计算规格说明", docs)
	if boosted[0].ID != "doc-5-child-001" {
		t.Fatalf("expected hierarchy title match to rank first, got %s", boosted[0].ID)
	}
	if boosted[0].MetaData["title_match_boost_applied"] != true {
		t.Fatalf("expected title_match_boost_applied metadata")
	}
	if boosted[0].MetaData["score"].(float64) <= 0.70 {
		t.Fatalf("expected boosted score to outrank billing candidate, got %v", boosted[0].MetaData["score"])
	}
}

func TestApplyTitleMatchBoostDoesNotPromoteDomainSpecificIntentWithoutTitleMatch(t *testing.T) {
	docs := []*schema.Document{
		{
			ID:      "doc-11-child-000",
			Content: "# 云消息队列 RocketMQ 版 - 消息收发计算规格计费说明\n\n云消息队列 RocketMQ 版根据消息收发 TPS 上限提供不同的计算规格。",
			MetaData: map[string]interface{}{
				"document_id":    uint64(11),
				"chunk_id":       "doc-11-child-000",
				"section_title":  "云消息队列 RocketMQ 版 - 消息收发计算规格计费说明",
				"hierarchy_path": "云消息队列 RocketMQ 版 - 消息收发计算规格计费说明",
				"score":          0.0833,
			},
		},
		{
			ID:      "doc-11-child-001",
			Content: "### 1.1 计算规格能力约束\n\n消息收发 TPS 是云消息队列 RocketMQ 版处理消息收发的 TPS 上限。",
			MetaData: map[string]interface{}{
				"document_id":    uint64(11),
				"chunk_id":       "doc-11-child-001",
				"section_title":  "1.1 计算规格能力约束",
				"hierarchy_path": "1. 计算规格说明 > 1.1 计算规格能力约束",
				"score":          0.0588,
			},
		},
		{
			ID:      "doc-11-child-002",
			Content: "### 1.2 超过规格限制后行为\n\n若实际使用的消息收发 TPS 超过了购买的规格上限，开启弹性 TPS 可在弹性区间内正常运行，超过弹性能力上限后实例还是会被限流。",
			MetaData: map[string]interface{}{
				"document_id":    uint64(11),
				"chunk_id":       "doc-11-child-002",
				"section_title":  "1.2 超过规格限制后行为",
				"hierarchy_path": "1. 计算规格说明 > 1.2 超过规格限制后行为",
				"score":          0.0455,
			},
		},
	}

	boosted := applyTitleMatchBoost("超过 TPS 规格上限后会怎样？", docs)
	if boosted[0].ID != "doc-11-child-000" {
		t.Fatalf("expected original ranking to be preserved, got %s", boosted[0].ID)
	}
	for _, doc := range boosted {
		if doc.MetaData["title_match_boost_applied"] == true {
			t.Fatalf("expected no title boost for domain-specific intent query, got %v", doc.MetaData["title_match_boost_reason"])
		}
	}
}

func TestApplyTitleMatchBoostDoesNotPromoteContentOnlyFormulaEvidence(t *testing.T) {
	docs := []*schema.Document{
		{
			ID:      "doc-15-child-000",
			Content: "**云消息队列 RocketMQ 版 - 消息收发计算规格计费说明** 包年包月或按量付费计费方式下，消息收发计算规格为云消息队列 RocketMQ 版的必选计费项。",
			MetaData: map[string]interface{}{
				"document_id":    uint64(15),
				"chunk_id":       "doc-15-child-000",
				"section_title":  "云消息队列 RocketMQ 版 - 消息收发计算规格计费说明",
				"hierarchy_path": "云消息队列 RocketMQ 版 - 消息收发计算规格计费说明",
				"score":          0.5233,
			},
		},
		{
			ID:      "doc-15-child-004",
			Content: "## 2. 计费规则 云消息队列 RocketMQ 版的基础计算规格费用按照规格大小计费。计费公式：计算规格费用 = 实例购买后的服务时长（小时） × 规格单价（元/小时）。",
			MetaData: map[string]interface{}{
				"document_id":    uint64(15),
				"chunk_id":       "doc-15-child-004",
				"section_title":  "2. 计费规则",
				"hierarchy_path": "2. 计费规则",
				"score":          0.4595,
			},
		},
	}

	boosted := applyTitleMatchBoost("RocketMQ 计算规格费用怎么计算?", docs)
	if boosted[0].ID != "doc-15-child-000" {
		t.Fatalf("expected original ranking to be preserved, got %s", boosted[0].ID)
	}
	for _, doc := range boosted {
		if doc.MetaData["title_match_boost_applied"] == true {
			t.Fatalf("expected no title boost for content-only formula evidence, got %v", doc.MetaData["title_match_boost_reason"])
		}
	}
}
