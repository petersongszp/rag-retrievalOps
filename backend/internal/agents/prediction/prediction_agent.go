package prediction

import (
	"context"
	"fmt"


	"github.com/cloudwego/eino/adk"
)

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

// NewPredictionAgent 创建押题智能体
func NewPredictionAgent(userId uint) (adk.Agent, error) {
	ctx := context.Background()

	model, err := llm.CreatOpenAiChatModel(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI chat model: %w", err)
	}

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "PredictionAgent",
		Description: "根据简历生成面试押题",
		Instruction: `你是一个资深的面试官和技术专家。你的任务是根据求职者的简历和指定的要求（如岗位、难度、目标公司），预测可能会问到的5道面试题。

【重要要求】
1. 必须严格生成 5 道题目，少于 5 道是不允许的。
2. 必须返回标准的 JSON 格式，不要包含 markdown 标记（如 '''json），也不要包含任何解释性文字。
3. 题目内容必须结合简历中的项目经历和技能点。
4. 所有题目与字段（question、content、focus、thinking_path、reference_answer、follow_up）均须使用英文（English）撰写。

【JSON 格式模板】
{
  "questions": [
    {
      "question": "问题内容",
      "content": "【重点考察】考察方向标题
      "focus": "重点考察（例如：项目经历真实性验证、基础知识掌握等）",
      "thinking_path": "回答思路",
      "reference_answer": "参考答案",
      "follow_up": "可能追问（如果是多个追问，请用数组格式；如果是单个，请用字符串）"
    },
    ... (共5个)
  ]
}
`,
		Model: model,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create prediction agent: %w", err)
	}
	return agent, nil
}
