package prediction

import (
	"context"
	"fmt"
	"interview-agents/internal/agents/llm"

	"github.com/cloudwego/eino/adk"
)

const (
	DirectionCount = 5
	// todo 更改
	QuestionsPerDirection    = 1
	TotalPredictionQuestions = DirectionCount * QuestionsPerDirection
)

type DirectionModule struct {
	Direction string `json:"direction"`
	Content   string `json:"content"`
}

type ResumeSplitResult struct {
	Directions []DirectionModule `json:"directions"`
}

// PredictionQuestion 结构体用于解析 AI 返回的 JSON
type PredictionQuestion struct {
	Question        string `json:"question"`
	Content         string `json:"content"`
	Focus           string `json:"focus"`
	ThinkingPath    string `json:"thinking_path"`
	ReferenceAnswer string `json:"reference_answer"`
	FollowUp        any    `json:"follow_up"` // 兼容 string 或 []string
}

type PredictionResult struct {
	Questions []PredictionQuestion `json:"questions"`
}

// NewResumeSplitAgent 创建简历方向拆分智能体
func NewResumeSplitAgent(userId uint) (adk.Agent, error) {
	ctx := context.Background()

	model, err := llm.CreatOpenAiChatModel(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI chat model: %w", err)
	}

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "ResumeDirectionSplitAgent",
		Description: "将简历拆分为押题方向模块",
		Instruction: `你是一个资深面试策略分析师。你的任务是把输入的简历内容拆分成 4 个可独立出题的方向模块。

【重要要求】
1. 只返回 JSON，不要 markdown 代码块，不要额外解释。
2. 必须严格返回 4 个方向，且方向名称不能重复。
3. 每个方向必须包含与该方向强相关的简历内容摘要，便于下游 Agent 独立出题。
4. 方向应覆盖候选人的核心技术能力、项目实践、基础原理、系统设计/性能优化等维度。

【JSON 格式模板】
{
	"directions": [
    {
		  "direction": "方向名称",
		  "content": "该方向的简历关键信息摘要"
    },
		... (共4个)
  ]
}
`,
		Model: model,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create resume split agent: %w", err)
	}
	return agent, nil
}

// NewDirectionalPredictionAgent 创建方向押题智能体
func NewDirectionalPredictionAgent(userId uint, direction string) (adk.Agent, error) {
	ctx := context.Background()

	model, err := llm.CreatOpenAiChatModel(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI chat model: %w", err)
	}

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "DirectionalPredictionAgent_" + direction,
		Description: "根据指定方向生成面试押题",
		Instruction: `你是一个资深的面试官和技术专家。你的任务是根据给定的“单一方向模块内容”和押题要求，生成该方向下的 3 道面试题。

【重要要求】
1. 必须严格生成 3 道题目，少于或多于 3 道都不允许。
2. 只返回标准 JSON，不要 markdown 标记（如 '''json），不要解释性文字。
3. 题目内容必须紧扣输入的方向模块，不要偏离方向。

【JSON 格式模板】
{
  "questions": [
    {
      "question": "问题内容",
      "content": "【重点考察】考察方向标题",
      "focus": "重点考察（例如：项目经历真实性验证、基础知识掌握等）",
      "thinking_path": "回答思路",
      "reference_answer": "参考答案",
      "follow_up": "可能追问（如果是多个追问，请用数组格式；如果是单个，请用字符串）"
    },
    ... (共3个)
  ]
}
`,
		Model: model,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create directional prediction agent: %w", err)
	}
	return agent, nil
}
