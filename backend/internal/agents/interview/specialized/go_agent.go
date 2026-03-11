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

// NewGoSpecializedAgent 创建 Go 专项面试官智能体
// 专注于评估候选人在 Go 语言方面的专业技能和深度
func NewGoSpecializedAgent(userId uint, needResumeTool bool) (adk.Agent, error) {
	ctx := context.Background()

	milvusTool, err := tool2.GetMilvusRetrieverTool()
	if err != nil {
		return nil, fmt.Errorf("failed to create Milvus retriever tool: %w", err)
	}

	var tools []componenttool.BaseTool
	tools = append(tools, milvusTool)

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
		Name:          "GoSpecializedAgent",
		Description:   "Go 专项面试官智能体，专注于评估候选人在 Go 语言方面的专业技能和深度",
		Instruction:   GoSpecializedAgentInstruction,
		Model:         model,
		ToolsConfig:   toolsConfig,
		MaxIterations: 15,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Go specialized agent: %w", err)
	}
	return baseAgent, nil
}
