package specialized

import (
	"context"
	"fmt"
	"interview-agents/internal/agents/llm"
	tool2 "interview-agents/internal/agents/tools"

	"github.com/cloudwego/eino/adk"
	componenttool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
)

// NewJavaSpecializedAgent 创建 Java 专项面试官智能体
// 专注于评估候选人在 Java 方面的专业技能和深度
func NewJavaSpecializedAgent(userId uint, needResumeTool bool) (adk.Agent, error) {
	ctx := context.Background()

	ragTool := tool2.GetRAGRetrieveTool()

	var tools []componenttool.BaseTool
	tools = append(tools, ragTool)

	if needResumeTool {
		tools = append(tools, tool2.GetResumeInfoTool())
	}

	toolsConfig := adk.ToolsConfig{
		ToolsNodeConfig: compose.ToolsNodeConfig{
			Tools: tools,
		},
	}

	model, err := llm.CreatOpenAiChatModel(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI chat model: %w", err)
	}

	baseAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "JavaSpecializedAgent",
		Description:   "Java 专项面试官智能体，专注于评估候选人在 Java 方面的专业技能和深度",
		Instruction:   JavaSpecializedAgentInstruction,
		Model:         model,
		ToolsConfig:   toolsConfig,
		MaxIterations: 15,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Java specialized agent: %w", err)
	}
	return baseAgent, nil
}
