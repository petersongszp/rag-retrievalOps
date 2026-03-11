package multiagent

import (
	"context"
	"fmt"
	"interview-agents/internal/agents/llm"
	tool2 "interview-agents/internal/agents/tools"

	"github.com/cloudwego/eino/adk"
	componenttool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
)

// NewCoInterviewerAgent 创建技术面试官智能体
// 专注于技术深度挖掘和连环追问
func NewCoInterviewerAgent(ctx context.Context, userId uint) (adk.Agent, error) {
	model, err := llm.CreatOpenAiChatModel(ctx, userId)
	if err != nil {
		return nil, err
	}
	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "co_interviewer",
		Description: "技术面试官工具。当候选人提到技术栈（如Redis、MySQL、Go、分布式系统、微服务等）或需要深入技术细节时，必须调用此工具。技术面试官会进行深度技术追问和考察。",
		Instruction: CoInterviewerInstruction,
		Model:       model,
	})
}

// NewProjectInterviewerAgent 创建项目面试官智能体
// 专注于考察候选人的项目真实性、落地能力和解决问题的思路
func NewProjectInterviewerAgent(ctx context.Context, userId uint) (adk.Agent, error) {
	model, err := llm.CreatOpenAiChatModel(ctx, userId)
	if err != nil {
		return nil, err
	}
	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "project_interviewer",
		Description: "项目面试官工具。当候选人描述项目经验、架构设计、遇到的问题或需要评估实际落地能力时，必须调用此工具。项目面试官会深入挖掘项目细节和真实性。",
		Instruction: ProjectInterviewerInstruction,
		Model:       model,
	})
}

// NewInterviewHostAgent 创建面试主控智能体 (主面试官)
// 负责统筹整个面试流程，调度副面试官 and 项目面试官
func NewInterviewHostAgent(ctx context.Context, userId uint, needResumeTool bool) (adk.Agent, error) {
	model, err := llm.CreatOpenAiChatModel(ctx, userId)
	if err != nil {
		return nil, err
	}

	// 1. 初始化协作 Agent
	coAgent, err := NewCoInterviewerAgent(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to create co-interviewer: %w", err)
	}

	projectAgent, err := NewProjectInterviewerAgent(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to create project-interviewer: %w", err)
	}

	// 2. 将协作 Agent 包装成工具 (AgentAsTool)
	coTool := adk.NewAgentTool(ctx, coAgent)
	projectTool := adk.NewAgentTool(ctx, projectAgent)

	// 3. 构建工具列表
	tools := []componenttool.BaseTool{
		coTool,
		projectTool,
	}

	// 如果需要，添加简历工具
	if needResumeTool {
		tools = append(tools, tool2.GetResumeInfoTool())
	}

	// 4. 创建主面试官 (Host)
	hostAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "MainInterviewer",
		Description: "主面试官，负责面试全流程的引导、节奏控制 and 最终评价。",
		Instruction: MainInterviewerInstruction,
		Model:       model,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: tools,
			},
		},
		MaxIterations: 30,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create main interviewer agent: %w", err)
	}

	return hostAgent, nil
}
