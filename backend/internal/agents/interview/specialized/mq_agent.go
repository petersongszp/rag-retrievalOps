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

// NewMQSpecializedAgent 创建 MQ 专项面试官智能体
// 专注于评估候选人在消息队列技术方面的专业能力和深度
func NewMQSpecializedAgent(userId uint, needResumeTool bool) (adk.Agent, error) {
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
		Name:          "MQSpecializedAgent",
		Description:   "MQ 专项面试官智能体，专注于评估候选人在消息队列技术方面的专业能力和深度",
		Instruction:   MQSpecializedAgentInstruction,
		Model:         model,
		ToolsConfig:   toolsConfig,
		MaxIterations: 15,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MQ specialized agent: %w", err)
	}
	return baseAgent, nil
}
