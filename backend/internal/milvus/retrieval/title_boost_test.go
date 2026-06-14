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
