package evaluation

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cloudwego/eino/components/model"
	"interview-agents/internal/agents/llm"
	"log"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// Evaluator 面试回答评分器
type Evaluator struct {
	userId uint
	model  model.ToolCallingChatModel
	config *EvaluatorConfig
}

// NewEvaluator 创建评分器
func NewEvaluator(ctx context.Context, userId uint, config *EvaluatorConfig) (*Evaluator, error) {
	if config == nil {
		config = DefaultEvaluatorConfig()
	}
	// 创建 LLM 模型
	m, err := llm.CreatOpenAiChatModel(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat model for evaluator: %w", err)
	}

	return &Evaluator{
		userId: userId,
		model:  m,
		config: config,
	}, nil
}

// Evaluate 对候选人回答进行评分
func (e *Evaluator) Evaluate(ctx context.Context, req *EvaluationRequest) (*EvaluationResult, error) {
	// 验证请求
	if req == nil {
		return nil, fmt.Errorf("evaluation request cannot be nil")
	}
	if req.Question == "" || req.Answer == "" {
		return nil, fmt.Errorf("question and answer cannot be empty")
	}

	// 构建评分 Prompt
	prompt := BuildEvaluationPrompt(req)

	// 调用 LLM 获取评分
	messages := []*schema.Message{
		schema.SystemMessage(EvaluatorInstruction),
		schema.UserMessage(prompt),
	}

	response, err := e.model.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("failed to generate evaluation: %w", err)
	}

	// 解析 JSON 响应
	result, err := parseEvaluationResponse(response.Content)
	if err != nil {
		log.Printf("[Evaluator] Failed to parse response, raw content: %s", response.Content)
		return nil, fmt.Errorf("failed to parse evaluation response: %w", err)
	}

	// 验证并修正评分
	e.validateAndFixResult(result)

	return result, nil
}

// parseEvaluationResponse 解析 LLM 返回的 JSON 响应
func parseEvaluationResponse(content string) (*EvaluationResult, error) {
	// 清理响应内容（移除可能的 markdown 代码块标记）
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var result EvaluationResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("json unmarshal failed: %w, content: %s", err, content)
	}

	return &result, nil
}

// validateAndFixResult 验证并修正评分结果
func (e *Evaluator) validateAndFixResult(result *EvaluationResult) {
	// 确保分数在 1-10 范围内
	result.Scores.Correctness = clampScore(result.Scores.Correctness)
	result.Scores.Depth = clampScore(result.Scores.Depth)
	result.Scores.Completeness = clampScore(result.Scores.Completeness)
	result.Scores.Practicality = clampScore(result.Scores.Practicality)

	// 如果 overall 未设置或不合理，重新计算
	if result.Overall < 1 || result.Overall > 10 {
		result.Overall = result.Scores.CalculateOverall()
	}

	// 验证 NextAction
	switch result.NextAction {
	case ActionContinue, ActionDeepen, ActionSwitch, ActionLower:
		// 有效值，保持不变
	default:
		// 根据分数自动推断
		result.NextAction = e.inferNextAction(result.Overall)
	}

	// 确保 CoveredTopics 不为 nil
	if result.CoveredTopics == nil {
		result.CoveredTopics = []string{}
	}
}

// inferNextAction 根据分数推断下一步动作
func (e *Evaluator) inferNextAction(score float64) NextAction {
	if score >= float64(e.config.DeepenThreshold) {
		return ActionDeepen
	}
	if score < float64(e.config.LowerThreshold) {
		return ActionLower
	}
	return ActionContinue
}

// clampScore 将分数限制在 1-10 范围内
func clampScore(score float64) float64 {
	if score < 1 {
		return 1
	}
	if score > 10 {
		return 10
	}
	return score
}

// EvaluateWithHistory 评分并更新历史记录
func (e *Evaluator) EvaluateWithHistory(ctx context.Context, req *EvaluationRequest, history *ScoreHistory) (*EvaluationResult, error) {
	result, err := e.Evaluate(ctx, req)
	if err != nil {
		return nil, err
	}

	// 更新历史记录
	if history != nil {
		history.AddScore(result.Overall, req.CurrentTopic)

		// 基于历史记录调整 NextAction
		if history.CheckConsecutiveHigh(e.config.ConsecutiveCount, float64(e.config.DeepenThreshold)) {
			result.NextAction = ActionDeepen
		} else if history.CheckConsecutiveLow(e.config.ConsecutiveCount, float64(e.config.LowerThreshold)) {
			result.NextAction = ActionLower
		}
	}

	return result, nil
}
