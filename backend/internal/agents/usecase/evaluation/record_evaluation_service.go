package evaluation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"interview-agents/api/model/interview"
	"interview-agents/internal/agents/evaluation"
	"interview-agents/internal/agents/pkg"
	"interview-agents/internal/model"
	"log"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"gorm.io/gorm"
)

// GenerateRecordEvaluation 调用答题记录评估智能体生成评估
// 返回答题评估响应数据
func GenerateRecordEvaluation(ctx context.Context, userId uint, reportId uint64) (*interview.GetInterviewEvaluationResponse, error) {
	unlock := acquireGenerationLock("evaluation_report", userId, reportId)
	defer unlock()

	existing, err := model.InterviewEvaluationDao.GetEvaluationByUserIDAndReportID(userId, reportId)
	if err == nil && existing != nil {
		return buildEvaluationAPIResponse(existing), nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

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

	// Build an English-only query to avoid multilingual output drift.
	query := fmt.Sprintf(`Evaluate the interview record for user_id=%d and report_id=%d.

Process:
1. Call get_mianshi_info first to retrieve full interview dialogues.
2. Analyze candidate answers by quality, completeness, and depth.
3. Generate dimension-level scores and feedback.
4. Return final JSON only.

Output constraints:
- All generated string values must be in English.
- Keep score as integer 0-100.
- Keep dimension names in concise English phrases.
- Do not include any non-JSON text.`, userId, reportId)

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

func buildEvaluationAPIResponse(item *model.InterviewEvaluation) *interview.GetInterviewEvaluationResponse {
	if item == nil {
		return nil
	}
	response := &interview.GetInterviewEvaluationResponse{
		Comment:    item.Comment,
		Dimensions: make([]*interview.InterviewEvaluationDimension, 0, len(item.Dimensions)),
	}
	for _, dim := range item.Dimensions {
		if dim == nil {
			continue
		}
		response.Dimensions = append(response.Dimensions, &interview.InterviewEvaluationDimension{
			DimensionName: dim.DimensionName,
			Evaluation:    dim.Evaluation,
			Score:         int32(dim.Score),
		})
	}
	return response
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
func saveEvaluationToDatabase(_ context.Context, userId uint, reportId uint64, response *interview.GetInterviewEvaluationResponse) error {
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
	evalRecord := &model.InterviewEvaluation{
		UserID:     userId,
		ReportID:   reportId,
		Comment:    response.Comment,
		Score:      totalScore,
		Dimensions: dimensionList,
		Deleted:    0,
	}

	// 直接调用 DAO 方法保存到数据库
	err := model.InterviewEvaluationDao.UpsertEvaluation(evalRecord)
	if err != nil {
		log.Printf("[saveEvaluationToDatabase] 保存评估失败: %v", err)
		return fmt.Errorf("failed to save evaluation: %w", err)
	}

	return nil
}
