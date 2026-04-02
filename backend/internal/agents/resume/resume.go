package resume

import (
	"context"
	"fmt"
	"interview-agents/internal/agents/llm"
	tool2 "interview-agents/internal/agents/tools"

	"github.com/cloudwego/eino/adk"
	componenttool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
)

// NewResumeParserAgent 创建简历解析智能体
// 用于解析简历内容，提取关键信息用于面试准备
func NewResumeParserAgent(userId uint) (adk.Agent, error) {
	ctx := context.Background()
	model, err := llm.CreatOpenAiChatModel(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI chat model: %w", err)
	}

	// 初始化 Milvus 检索工具 (用于 RAG 联动)
	milvusTool, err := tool2.GetMilvusRetrieverTool()
	if err != nil {
		return nil, fmt.Errorf("failed to create Milvus retriever tool: %w", err)
	}

	baseAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "ResumeParserAgent",
		Description: "一个专业的简历解析智能体，支持 RAG 知识库联动，用于提取简历并对齐行业标准",
		Instruction: `你是一个专业的简历分析专家。你的任务是解析候选人的简历，提取关键信息，并结合内部知识库进行行业标准对齐。

重要提示：
- 你必须使用 resume_extraction 工具来解析简历文件（支持PDF/DOCX格式）。
- 对于简历中提到的关键技术栈、证书或行业术语，必须使用 get_milvus_retriever 工具在知识库中检索相关的行业标准描述、考察重点或等级要求。
- 严禁空回复，必须基于工具返回的内容进行深层画像。
- 只返回 JSON 格式结果。

任务步骤（必须按顺序执行）：
1. 【获取原文】调用 resume_extraction 获取简历完整文本及结构化字段。
2. 【基础提取】初步提取基本信息、工作经历、项目和技术栈。
3. 【RAG 增强】针对提取出的“核心技术栈”和“工作经验”，调用 get_milvus_retriever 检索内部专业面试库。
   - 识别是否有不熟悉或模糊的行业术语。
   - 检索该技术在本项目（如大厂面试）中的考察权重和常见评级标准。
4. 【对齐分析】结合检索到的行业知识，重新修正候选人的“简历画像”：
   - 技能特长：是否符合行业术语描述？
   - 面试难度：结合知识库中的标准，给出更客观的推荐难度。
   - 深度洞察：在画像中加入“行业对齐”后的评价。

5. 返回完整的 JSON 结果。

必须返回的 JSON 格式：
{
  "basic_info": { "name": "", "work_years": "", "contact": "" },
  "education": [ { "school": "", "major": "", "degree": "", "graduation_year": "" } ],
  "work_experience": [ { "company": "", "position": "", "duration": "", "responsibilities": "" } ],
  "tech_stack": ["技术名"],
  "projects": [ { "name": "", "description": "", "tech_stack": [], "contribution": "" } ],
  "skills": ["行业术语对齐后的技能点"],
  "certifications": ["证书名"],
  "strengths": "通过 RAG 对比后的核心技术优势",
  "potential_weaknesses": "与行业标准对比后的不足点",
  "recommended_difficulty": "初级/中级/高级/专家（参考知识库标准）",
  "interview_focus_areas": ["基于检索结果的重点考察方向"],
  "suggested_questions_directions": ["针对性提问引导"]
}`,

		Model: model,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []componenttool.BaseTool{
					tool2.CreateResumeExtractionTool(),
					milvusTool,
				},
			},
		},
		MaxIterations: 20,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create resume parser agent: %w", err)
	}
	return baseAgent, nil
}
