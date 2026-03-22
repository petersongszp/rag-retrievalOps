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
		Description: "Split resume content into independent prediction directions",
		Instruction: `You are a senior interview strategy analyst. Split the input resume into 5 independent direction modules for question generation.

[Important Requirements]
1. Return JSON only. Do not use markdown code fences and do not add explanations.
		2. Return exactly 5 directions, and direction names must be unique.
3. Each direction must include a concise resume summary that is highly relevant to that direction.
4. Directions should cover core technical skills, project experience, fundamentals, and system design/performance optimization.
5. All values in the JSON output must be written in English.

[JSON Template]
{
	"directions": [
    {
		  "direction": "Direction name",
		  "content": "Key resume summary for this direction"
    },
		... (total 5)
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
		Description: "Generate interview prediction questions for a specific direction",
		Instruction: `You are a senior interviewer and technical expert. Based on the given single direction module and requirements, generate 1 interview question for that direction.

		[Important Requirements]
		1. Generate exactly 1 question. Fewer or more are not allowed.
		2. Return valid JSON only. Do not use markdown and do not add explanations.
		3. Questions must stay tightly aligned with the input direction module.
		4. All fields in the JSON output must be written in English.

		[JSON Template]
		{
		  "questions": [
			{
			  "question": "Question text",
			  "content": "[Key Assessment] Assessment topic",
			  "focus": "What this question evaluates",
			  "thinking_path": "Suggested answering approach",
			  "reference_answer": "Reference answer",
			  "follow_up": "Possible follow-up question(s)"
			},
			... (total 1)
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
