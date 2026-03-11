package comprehensive

import (
	"interview-agents/internal/agents/llm"
	tool2 "interview-agents/internal/agents/tools"
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	componenttool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
)

// NewSchoolComprehensiveAgent 创建校招综合面试官智能体
// 专注于评估应届毕业生的综合能力，包括基础知识、学习潜力和职业素养
func NewSchoolComprehensiveAgent(userId uint, needResumeTool bool) (adk.Agent, error) {
	ctx := context.Background()

	var toolsConfig adk.ToolsConfig
	if needResumeTool {
		toolsConfig = adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []componenttool.BaseTool{
					tool2.GetResumeInfoTool(),
				},
			},
		}
	}

	model, err := llm.CreatOpenAiChatModel(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI chat model: %w", err)
	}

	baseAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "SchoolComprehensiveAgent",
		Description:   "校招综合面试官智能体，全面评估应届毕业生的综合能力",
		Instruction:   SchoolComprehensiveAgentInstruction,
		Model:         model,
		ToolsConfig:   toolsConfig,
		MaxIterations: 15,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create school comprehensive agent: %w", err)
	}
	return baseAgent, nil
}
