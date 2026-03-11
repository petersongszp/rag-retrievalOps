package evaluation

import (
	"strings"
	"testing"
)

// TestBuildEvaluationPrompt 校验 Prompt 构建包含领域、话题、问题与回答（12.3.1 单元测试策略）
func TestBuildEvaluationPrompt(t *testing.T) {
	req := &EvaluationRequest{
		Domain:       "go",
		CurrentTopic: "并发",
		Question:     "请说说 goroutine 与 channel 的关系",
		Answer:       "goroutine 是轻量级线程，channel 用于通信",
	}
	out := BuildEvaluationPrompt(req)

	checks := []struct {
		name  string
		sub   string
		label string
	}{
		{"含领域", "go", "【面试领域】"},
		{"含话题", "并发", "【当前话题】"},
		{"含问题", req.Question, ""},
		{"含回答", req.Answer, ""},
	}
	for _, c := range checks {
		if c.sub != "" && !strings.Contains(out, c.sub) {
			t.Errorf("BuildEvaluationPrompt: 期望输出包含 %q", c.sub)
		}
		if c.label != "" && !strings.Contains(out, c.label) {
			t.Errorf("BuildEvaluationPrompt: 期望输出包含 %q", c.label)
		}
	}
}

func TestBuildEvaluationPrompt_EmptyTopic(t *testing.T) {
	req := &EvaluationRequest{
		Domain:       "MySQL",
		CurrentTopic: "",
		Question:     "MVCC 是什么？",
		Answer:       "多版本并发控制",
	}
	out := BuildEvaluationPrompt(req)
	if !strings.Contains(out, "MySQL") || !strings.Contains(out, "MVCC") {
		t.Errorf("BuildEvaluationPrompt(empty topic): 应包含领域与问题")
	}
}
