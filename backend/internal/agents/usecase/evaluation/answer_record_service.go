package evaluation

import (
	"encoding/json"
	"errors"
	"fmt"
	"interview-agents/internal/agents/evaluation"
	"interview-agents/internal/agents/pkg"
	"interview-agents/internal/model"
	"log"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"golang.org/x/net/context"
	"gorm.io/gorm"
)

func GenerateAnswerRecordEvaluation(ctx context.Context, userId uint, reportId uint64) (*model.AnswerReport, error) {
	unlock := acquireGenerationLock("topic_evaluation", userId, reportId)
	defer unlock()

	existing, err := model.AnswerReportDao.GetAnswerReportByUserIDAndReportID(userId, reportId)
	if err == nil && existing != nil {
		return existing, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// 添加 300 秒超时（5分钟）- 评估需要调用工具和生成详细内容
	timeoutCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	// 创建主题评估智能体
	agent, err := evaluation.NewAnswerRecordAgent(userId)
	if err != nil {
		log.Printf("[GenerateAnswerRecordEvaluation] 创建智能体失败: %v", err)
		return nil, err
	}

	// 创建 runner
	runner := adk.NewRunner(timeoutCtx, adk.RunnerConfig{
		Agent: agent,
	})

	// 构建中文查询以生成中文评估报告。
	query := fmt.Sprintf(`请评估user_id=%d和report_id=%d的面试记录，按话题级别进行评估。

流程：
1. 首先调用get_mianshi_info获取完整面试对话。
2. 按时间顺序评估每个话题。
3. 为每个话题生成详细的反馈，包含评分和依据。

输出约束：
- 仅按要求的records格式返回JSON。
- 所有生成的字符串值请使用与面试记录相同的语言（中文面试用中文，英文面试用英文）。
- difficulty必须为easy、medium、hard之一。
- score必须为0-100的整数。
- 将所有相关的追问对话归入同一话题记录下。`, userId, reportId)

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
			log.Printf("[GenerateInterviewTopicEvaluation] 超时：等待智能体响应超过 300 秒")
			return nil, fmt.Errorf("timeout waiting for topic evaluation (300s)")
		default:
		}

		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			log.Printf("[GenerateInterviewTopicEvaluation] 错误: %v", event.Err)
			return nil, fmt.Errorf("error during topic evaluation: %w", event.Err)
		}

		// 收集最后一条消息
		if event.Output != nil && event.Output.MessageOutput != nil {
			lastMessage = event.Output.MessageOutput.Message.Content
		}
	}

	// 构建答题报告响应
	report := buildAnswerReportResponse(lastMessage, userId, reportId)

	// 保存答题报告数据到数据库
	if err := saveAnswerReportToDatabase(ctx, report); err != nil {
		log.Printf("Warning: Failed to save answer report: %v", err)
	}

	return report, nil
}

// buildAnswerReportResponse 从智能体响应构建答题报告响应
// 直接反序列化智能体返回的 JSON
func buildAnswerReportResponse(agentResponse string, userId uint, reportId uint64) *model.AnswerReport {
	report := &model.AnswerReport{
		UserID:   userId,
		ReportID: reportId,
		Records:  make([]*model.AnswerRecordItem, 0),
		Deleted:  0,
	}

	// 定义临时结构体用于解析
	type TempResponse struct {
		Records []map[string]interface{} `json:"records"`
	}

	var tempResp TempResponse

	// 尝试直接解析 JSON
	if err := json.Unmarshal([]byte(agentResponse), &tempResp); err != nil {
		// 尝试从文本中提取 JSON
		jsonStr := pkg.ExtractJSONFromResponse(agentResponse)
		if jsonStr == "" {
			log.Printf("[buildAnswerReportResponse] 无法提取 JSON，使用默认响应")
			return buildDefaultAnswerReport(userId, reportId)
		}

		// 尝试解析提取的 JSON
		if err := json.Unmarshal([]byte(jsonStr), &tempResp); err != nil {
			log.Printf("[buildAnswerReportResponse] 解析提取的 JSON 失败: %v", err)
			return buildDefaultAnswerReport(userId, reportId)
		}
	}

	// 转换临时结构体为最终的 AnswerRecordItem 列表
	for _, recordMap := range tempResp.Records {
		record := convertMapToAnswerRecordItem(recordMap)
		if record != nil {
			report.Records = append(report.Records, record)
		}
	}

	return report
}

// convertMapToAnswerRecordItem 将 map 转换为 AnswerRecordItem
func convertMapToAnswerRecordItem(recordMap map[string]interface{}) *model.AnswerRecordItem {
	record := &model.AnswerRecordItem{}

	// 解析 order
	if order, ok := recordMap["order"].(float64); ok {
		record.Order = int32(order)
	}

	// 解析 content
	if content, ok := recordMap["content"].(string); ok {
		record.Content = content
	}

	// 解析 comment
	if commentMap, ok := recordMap["comment"].(map[string]interface{}); ok {
		record.Comment = convertMapToAnswerRecordComment(commentMap)
	}

	// 解析 message
	if messageList, ok := recordMap["message"].([]interface{}); ok {
		record.Message = make([]*model.AnswerRecordMsg, 0)
		for _, msg := range messageList {
			if msgMap, ok := msg.(map[string]interface{}); ok {
				record.Message = append(record.Message, convertMapToAnswerRecordMsg(msgMap))
			}
		}
	}

	return record
}

// convertMapToAnswerRecordComment 将 map 转换为 AnswerRecordComment
func convertMapToAnswerRecordComment(commentMap map[string]interface{}) *model.AnswerRecordComment {
	comment := &model.AnswerRecordComment{}

	// 解析 score
	if score, ok := commentMap["score"].(float64); ok {
		comment.Score = int32(score)
	}

	// 解析 key_points
	if keyPoints, ok := commentMap["key_points"].(string); ok {
		comment.KeyPoints = keyPoints
	}

	// 解析 difficulty
	if difficulty, ok := commentMap["difficulty"].(string); ok {
		comment.Difficulty = difficulty
	}

	// 解析 strengths
	if strengths, ok := commentMap["strengths"].(string); ok {
		comment.Strengths = strengths
	}

	// 解析 weaknesses
	if weaknesses, ok := commentMap["weaknesses"].(string); ok {
		comment.Weaknesses = weaknesses
	}

	// 解析 suggestion
	if suggestion, ok := commentMap["suggestion"].(string); ok {
		comment.Suggestion = suggestion
	}

	// 解析 know_points
	if knowPoints, ok := commentMap["know_points"].(string); ok {
		comment.KnowPoints = knowPoints
	}

	// 解析 thinking
	if thinking, ok := commentMap["thinking"].(string); ok {
		comment.Thinking = thinking
	}

	// 解析 reference
	if reference, ok := commentMap["reference"].(string); ok {
		comment.Reference = reference
	}

	return comment
}

// convertMapToAnswerRecordMsg 将 map 转换为 AnswerRecordMsg
func convertMapToAnswerRecordMsg(msgMap map[string]interface{}) *model.AnswerRecordMsg {
	msg := &model.AnswerRecordMsg{}

	// 解析 order
	if order, ok := msgMap["order"].(float64); ok {
		msg.Order = int32(order)
	}

	// 解析 question
	if question, ok := msgMap["question"].(string); ok {
		msg.Question = question
	}

	// 解析 answer
	if answer, ok := msgMap["answer"].(string); ok {
		msg.Answer = answer
	}

	return msg
}

// buildDefaultAnswerReport 构建默认答题报告
func buildDefaultAnswerReport(userId uint, reportId uint64) *model.AnswerReport {
	return &model.AnswerReport{
		UserID:   userId,
		ReportID: reportId,
		Records:  make([]*model.AnswerRecordItem, 0),
		Deleted:  0,
	}
}

// saveAnswerReportToDatabase 将答题报告数据保存到数据库
func saveAnswerReportToDatabase(_ context.Context, report *model.AnswerReport) error {
	// 直接调用 DAO 方法保存到数据库
	err := model.AnswerReportDao.UpsertAnswerReport(report)
	if err != nil {
		log.Printf("[saveAnswerReportToDatabase] 保存答题报告失败: %v", err)
		return fmt.Errorf("failed to save answer report: %w", err)
	}

	log.Printf("[saveAnswerReportToDatabase] 答题报告保存成功，UserID: %d, ReportID: %d, 记录数: %d", report.UserID, report.ReportID, len(report.Records))
	return nil
}
