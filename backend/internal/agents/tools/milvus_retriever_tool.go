package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"interview-agents/internal/errors"
	"interview-agents/internal/milvus"
	"log"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// MilvusRetrieverInput 检索工具的输入参数
type MilvusRetrieverInput struct {
	// 查询文本
	Query string `json:"query" description:"要检索的查询文本"`
}

// MilvusRetrieverOutput 检索工具的输出结果
type MilvusRetrieverOutput struct {
	// 检索到的文档列表
	Documents []DocumentInfo `json:"documents" description:"检索到的相关文档列表"`
	// 检索结果数量
	Count int `json:"count" description:"检索到的文档数量"`
	// 错误信息（如果有）
	Error string `json:"error,omitempty" description:"错误信息"`
}

// DocumentInfo 文档信息
type DocumentInfo struct {
	// 文档ID
	ID string `json:"id"`
	// 文档内容
	Content string `json:"content"`
	// 文档元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	// 相似度分数
	Score float32 `json:"score,omitempty"`
}

// TestMilvusCallCallback 用于单元测试验证工具是否被大模型调用的回调钩子
var TestMilvusCallCallback func(query string)

// GetMilvusRetrieverWithInput 检索向量数据库中的数据（包装函数，用于 Eino 工具）
func GetMilvusRetrieverWithInput(ctx context.Context, input MilvusRetrieverInput) (string, error) {
	if TestMilvusCallCallback != nil {
		TestMilvusCallCallback(input.Query)
	}
	return GetMilvusRetriever(ctx, input.Query)
}

// GetMilvusRetriever 检索向量数据库中的数据
func GetMilvusRetriever(ctx context.Context, query string) (string, error) {
	if query == "" {
		return "", fmt.Errorf("query cannot be empty")
	}

	// 获取 Milvus 管理器
	mgr, err := milvus.GetMilvusManager()
	if err != nil {
		log.Printf("向量数据库管理器初始化失败: %v", err)
		return formatErrorOutput(fmt.Sprintf("向量数据库管理器初始化失败: %v", err), 0)
	}

	// 获取检索器服务
	retriever := mgr.RetrieverService
	if retriever == nil {
		log.Printf("检索器服务未初始化")
		return formatErrorOutput("检索器服务未初始化", 0)
	}

	// 执行检索 (添加 3 秒超时控制)
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	documents, err := retriever.Retrieve(ctxWithTimeout, query)
	if err != nil {
		log.Printf("检索失败或超时: %v", err)
		return formatErrorOutput(fmt.Sprintf("检索失败或超时: %v", err), 0)
	}

	// 检查是否有检索结果
	if len(documents) == 0 {
		log.Printf("未找到相关文档，查询: %s", query)
		return formatOutput([]DocumentInfo{}, 0)
	}

	// 转换文档格式
	docInfos := make([]DocumentInfo, 0, len(documents))
	for _, doc := range documents {
		// 提取相似度分数
		var score float32
		if s, ok := doc.MetaData["score"].(float32); ok {
			score = s
		}

		docInfo := DocumentInfo{
			ID:       doc.ID,
			Content:  doc.Content,
			Metadata: doc.MetaData,
			Score:    score,
		}
		docInfos = append(docInfos, docInfo)
	}

	log.Printf("检索成功，找到 %d 个相关文档", len(docInfos))
	return formatOutput(docInfos, len(docInfos))
}

// formatOutput 格式化成功的输出
func formatOutput(documents []DocumentInfo, count int) (string, error) {
	output := MilvusRetrieverOutput{
		Documents: documents,
		Count:     count,
	}
	jsonBytes, err := json.Marshal(output)
	if err != nil {
		log.Printf("JSON 序列化失败: %v", err)
		return "", err
	}
	return string(jsonBytes), nil
}

// formatErrorOutput 格式化错误的输出
func formatErrorOutput(errMsg string, count int) (string, error) {
	output := MilvusRetrieverOutput{
		Documents: []DocumentInfo{},
		Count:     count,
		Error:     errMsg,
	}
	jsonBytes, err := json.Marshal(output)
	if err != nil {
		log.Printf("JSON 序列化失败: %v", err)
		return "", err
	}
	return string(jsonBytes), nil
}

// GetMilvusRetrieverTool 创建向量数据库检索工具
func GetMilvusRetrieverTool() (tool.InvokableTool, error) {
	t, err := utils.InferTool(
		"get_milvus_retriever",
		"从向量数据库中检索出对应相关的数据。输入查询文本，返回最相关的文档列表。",
		GetMilvusRetrieverWithInput, // 使用包装函数，接收 MilvusRetrieverInput 结构体
	)
	if err != nil {
		return nil, errors.NewMilvusError("创建检索工具失败", err)
	}
	return t, nil
}
