package prediction

import (
	"context"
	"fmt"
	"interview-agents/internal/agents/llm"

	"github.com/cloudwego/eino/adk"
)

const (
	DirectionCount           = 5
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
		Description: "将简历内容拆分为独立的押题方向",
		Instruction: `你是一位资深面试策略分析师。将输入的简历拆分为5个独立的方向模块，用于后续生成面试题目。

[重要要求]
1. 仅返回JSON。不要使用markdown代码块，不要添加解释。
2. 必须返回恰好5个方向，方向名称不得重复。
3. 每个方向必须包含与该方向高度相关的简洁简历摘要。
4. 方向应覆盖核心技术技能、项目经验、基础知识和系统设计/性能优化。
5. JSON输出中的所有值请使用中文填写。

[JSON模板]
{
	"directions": [
    {
		  "direction": "方向名称",
		  "content": "该方向的关键简历摘要"
    },
		... (共5个)
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
		Description: "为特定方向生成面试押题",
		Instruction: `你是一位资深面试官和技术专家。根据给定的单个方向模块和要求，为该方向生成1道面试押题。

		[重要要求]
		1. 必须生成恰好1道题，不能多也不能少。
		2. 仅返回有效的JSON，不要使用markdown，不要添加解释。
		3. 题目必须与输入的方向模块紧密对齐。
		4. JSON输出中的所有字段请使用中文填写。

		[JSON模板]
		{
		  "questions": [
			{
			  "question": "题目文本",
			  "content": "[核心考察] 考察主题",
			  "focus": "本题评估什么",
			  "thinking_path": "建议答题思路",
			  "reference_answer": "参考答案",
			  "follow_up": "可能的追问"
			},
			... (共1个)
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
