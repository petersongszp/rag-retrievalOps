package comprehensive

import (
	"context"
	"fmt"
	"interview-agents/internal/agents/llm"
	tool2 "interview-agents/internal/agents/tools"

	"github.com/cloudwego/eino/adk"
	componenttool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
)

// NewSocialComprehensiveAgent 创建社招综合面试官智能体
// 专注于评估有工作经验的候选人的综合能力，包括实战经验、架构设计和领导力
func NewSocialComprehensiveAgent(userId uint, needResumeTool bool) (adk.Agent, error) {
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
		Name:          "SocialComprehensiveAgent",
		Description:   "社招综合面试官智能体，全面评估有工作经验的候选人的综合能力",
		Instruction:   SocialComprehensiveAgentInstruction,
		Model:         model,
		ToolsConfig:   toolsConfig,
		MaxIterations: 15,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create social comprehensive agent: %w", err)
	}
	return baseAgent, nil
}
