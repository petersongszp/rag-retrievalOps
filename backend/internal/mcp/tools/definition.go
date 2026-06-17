package tools

import "github.com/modelcontextprotocol/go-sdk/mcp"

const RetrieveKnowledgeName = "retrieve_knowledge"

func RetrieveKnowledge() *mcp.Tool {
	return &mcp.Tool{
		Name: RetrieveKnowledgeName,
		Description: "根据用户问题，在授权知识库范围内检索可引用的知识证据，" +
			"返回结构化片段供 Agent 生成答案或展示引用。",
		InputSchema: map[string]interface{}{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"query"},
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "用户问题或检索 query",
					"maxLength":   2000,
				},
				"kb_ids": map[string]interface{}{
					"type":        "array",
					"description": "知识库 ID 列表，推荐使用",
					"maxItems":    100,
					"items": map[string]interface{}{
						"type":    "integer",
						"minimum": 1,
					},
				},
				"kb_id": map[string]interface{}{
					"type":        "integer",
					"description": "单知识库 ID，兼容字段",
					"minimum":     1,
				},
				"top_k": map[string]interface{}{
					"type":        "integer",
					"description": "期望召回数量，最终上限由 RAG 平台控制",
					"minimum":     1,
					"maximum":     20,
				},
				"strategy_profile": map[string]interface{}{
					"type":        "string",
					"description": "V1 预留字段，只有底层检索链路支持时才保证生效",
				},
				"metadata_filter": map[string]interface{}{
					"type":        "object",
					"description": "V1 预留字段，只有底层检索链路支持时才保证生效",
				},
			},
		},
	}
}
