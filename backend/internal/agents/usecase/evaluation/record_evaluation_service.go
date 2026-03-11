package evaluation

import (
	"interview-agents/api/model/interview"
	"interview-agents/internal/agents/evaluation"
	"interview-agents/internal/agents/pkg"
	"interview-agents/internal/model"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

// GenerateRecordEvaluation 调用答题记录评估智能体生成评估
// 返回答题评估响应数据
func GenerateRecordEvaluation(ctx context.Context, userId uint, reportId uint64) (*interview.GetInterviewEvaluationResponse, error) {
	// 添加 120 秒超时
	timeoutCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	// 创建答题记录评估智能体
	agent, err := evaluation.NewRecordEvaluationAgent(userId)
	if err != nil {
		log.Printf("[GenerateRecordEvaluation] 创建智能体失败: %v", err)
		return nil, err
	}

	// 创建 runner
	runner := adk.NewRunner(timeoutCtx, adk.RunnerConfig{
		Agent: agent,
	})

	// 构建查询消息
	query := fmt.Sprintf(`请对用户ID为 %d、报告ID为 %d 的答题记录进行详细评估。

请按照以下步骤进行：
1. 首先调用 get_mianshi_info 工具获取面试的完整问题和对话记录
2. 仔细分析候选人的回答内容
3. 根据回答质量进行综合评估
4. 生成详细的评估反馈

评估应包含：
- 评分（0-100分）
- 关键知识点的掌握情况
- 问题难度评估
- 回答的优势
- 回答的不足
- 改进建议
- 相关知识点总结
- 思考过程分析
- 参考答案或最佳实践`, userId, reportId)

	// 创建用户消息
	userMsg := &schema.Message{
		Role:    schema.User,
		Content: query,
	}

	messages := []adk.Message{
		userMsg,
	}

	// 运行智能体
	iter := runner.Run(timeoutCtx, messages)

	var lastMessage string
	for {
		select {
		case <-timeoutCtx.Done():
			log.Printf("[GenerateAnswerRecordEvaluation] 超时：等待智能体响应超过 120 秒")
			return nil, fmt.Errorf("timeout waiting for answer record evaluation (120s)")
		default:
		}

		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			log.Printf("[GenerateAnswerRecordEvaluation] 错误: %v", event.Err)
			return nil, fmt.Errorf("error during answer record evaluation: %w", event.Err)
		}

		// 收集最后一条消息
		if event.Output != nil && event.Output.MessageOutput != nil {
			lastMessage = event.Output.MessageOutput.Message.Content
		}
	}

	// 构建答题记录评估响应
	records := buildEvaluationResponse(lastMessage)

	// 检查 records 是否为 nil
	if records == nil {
		log.Printf("[GenerateRecordEvaluation] 评估响应为 nil，返回错误")
		return nil, fmt.Errorf("failed to build evaluation response: invalid agent response")
	}

	// 保存评估数据到数据库
	if err := saveEvaluationToDatabase(ctx, userId, reportId, records); err != nil {
		log.Printf("Warning: Failed to save evaluation: %v", err)
	}
	return records, nil
}

// buildEvaluationResponse 从智能体响应构建评估响应
// 直接反序列化智能体返回的 JSON
func buildEvaluationResponse(agentResponse string) *interview.GetInterviewEvaluationResponse {
	response := &interview.GetInterviewEvaluationResponse{
		Comment:    "",
		Dimensions: make([]*interview.InterviewEvaluationDimension, 0),
	}

	// 尝试直接解析 JSON
	if err := json.Unmarshal([]byte(agentResponse), response); err != nil {
		// 尝试从文本中提取 JSON
		jsonStr := pkg.ExtractJSONFromResponse(agentResponse)
		if jsonStr == "" {
			log.Printf("[buildEvaluationResponse] 无法提取 JSON，使用默认响应")
			return response // 返回默认值
		}

		// 尝试解析提取的 JSON
		if err := json.Unmarshal([]byte(jsonStr), response); err != nil {
			log.Printf("[buildEvaluationResponse] 解析提取的 JSON 失败: %v", err)
			return response // 返回默认值
		}
	}

	return response
}

// saveEvaluationToDatabase 将评估数据保存到数据库
func saveEvaluationToDatabase(ctx context.Context, userId uint, reportId uint64, response *interview.GetInterviewEvaluationResponse) error {
	// 检查 response 是否为 nil
	if response == nil {
		log.Printf("[saveEvaluationToDatabase] response 为 nil，无法保存评估")
		return fmt.Errorf("response is nil")
	}

	// 将维度数据转换为 []*model.EvaluationDimension
	var dimensionList []*model.EvaluationDimension
	for _, dim := range response.Dimensions {
		dimensionList = append(dimensionList, &model.EvaluationDimension{
			DimensionName: dim.DimensionName,
			Evaluation:    dim.Evaluation,
			Score:         float64(dim.Score),
		})
	}

	// 计算总体评分（各维度评分的平均值）
	var totalScore float64
	if len(response.Dimensions) > 0 {
		for _, dim := range response.Dimensions {
			totalScore += float64(dim.Score)
		}
		totalScore = totalScore / float64(len(response.Dimensions))
	}

	// 创建评估记录
	evaluation := &model.InterviewEvaluation{
		UserID:     userId,
		ReportID:   reportId,
		Comment:    response.Comment,
		Score:      totalScore,
		Dimensions: dimensionList,
		Deleted:    0,
	}

	// 直接调用 DAO 方法保存到数据库
	err := model.InterviewEvaluationDao.CreateEvaluation(evaluation)
	if err != nil {
		log.Printf("[saveEvaluationToDatabase] 保存评估失败: %v", err)
		return fmt.Errorf("failed to save evaluation: %w", err)
	}

	return nil
}
