package evaluation

import (
	"testing"
)

// TestParseEvaluationResponse 校验 LLM 输出解析（12.3.1 单元测试策略）
func TestParseEvaluationResponse(t *testing.T) {
	validJSON := `{
  "scores": {
    "correctness": 7,
    "depth": 6,
    "completeness": 8,
    "practicality": 5
  },
  "overall": 6.5,
  "covered_topics": ["goroutine", "channel"],
  "next_action": "continue",
  "reason": "理解正确，可继续追问"
}`
	res, err := parseEvaluationResponse(validJSON)
	if err != nil {
		t.Fatalf("parseEvaluationResponse(valid): %v", err)
	}
	if res.Scores.Correctness != 7 || res.Overall != 6.5 || res.NextAction != ActionContinue {
		t.Errorf("parseEvaluationResponse: scores/overall/next_action 解析不符")
	}
	if len(res.CoveredTopics) != 2 || res.Reason == "" {
		t.Errorf("parseEvaluationResponse: covered_topics 或 reason 解析不符")
	}
}

func TestParseEvaluationResponse_WithMarkdownBlock(t *testing.T) {
	wrapped := "```json\n{\"scores\":{\"correctness\":5,\"depth\":5,\"completeness\":5,\"practicality\":5},\"overall\":5,\"covered_topics\":[],\"next_action\":\"continue\",\"reason\":\"一般\"}\n```"
	res, err := parseEvaluationResponse(wrapped)
	if err != nil {
		t.Fatalf("parseEvaluationResponse(markdown): %v", err)
	}
	if res.Overall != 5 {
		t.Errorf("parseEvaluationResponse(markdown): overall 应为 5, got %v", res.Overall)
	}
}

func TestParseEvaluationResponse_InvalidJSON(t *testing.T) {
	_, err := parseEvaluationResponse("not json at all")
	if err == nil {
		t.Error("parseEvaluationResponse(invalid): 期望返回错误")
	}
}
