package impl

import (
	predictionIDL "interview-agents/api/model/prediction"
	predictionAgent "interview-agents/internal/agents/prediction"
	"interview-agents/internal/model"
	"interview-agents/internal/service/prediction"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

type PredictionServiceImpl struct{}

func NewPredictionService() prediction.PredictionService {
	return &PredictionServiceImpl{}
}

func (s *PredictionServiceImpl) Predict(ctx context.Context, req *predictionIDL.PredictRequest, userID uint) (resp *predictionIDL.PredictResponse, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Prediction] Panic recovered: %v", r)
			err = fmt.Errorf("internal server error: panic recovered")
		}
	}()

	log.Printf("[Prediction] Start predicting for user %d, resume %d", userID, req.ResumeID)

	// 1. Get Resume
	resume, err := model.ResumeDao.GetResumeByID(uint64(req.ResumeID))
	if err != nil {
		log.Printf("[Prediction] Error getting resume: %v", err)
		return nil, fmt.Errorf("resume not found: %w", err)
	}

	// 2. Construct Prompt
	prompt := fmt.Sprintf(`
简历内容：
%s

押题要求：
- 类型：%s
- 语言：%s
- 岗位：%s
- 难度：%s
`, resume.Content, req.PredictionType, req.Language, req.JobTitle, req.Difficulty)

	if req.CompanyName != nil {
		prompt += fmt.Sprintf("- 目标公司：%s\n", *req.CompanyName)
	}

	log.Printf("[Prediction] Calling Agent with prompt length: %d", len(prompt))

	// 3. Call Agent
	agent, err := predictionAgent.NewPredictionAgent(userID)
	if err != nil {
		log.Printf("[Prediction] Failed to create prediction agent: %v", err)
		return nil, err
	}

	// 使用 Runner 运行 Agent
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent: agent,
	})

	// 构建消息
	messages := []adk.Message{
		schema.UserMessage(prompt),
	}

	// 运行智能体
	iter := runner.Run(ctx, messages)

	var content string
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			log.Printf("[Prediction] Agent generation failed: %v", event.Err)
			return nil, fmt.Errorf("agent generation failed: %w", event.Err)
		}

		// 处理消息事件，收集最后一条消息内容
		if event.Output != nil && event.Output.MessageOutput != nil {
			message := event.Output.MessageOutput.Message.Content
			if message != "" {
				content = message
			}
		}
	}

	if content == "" {
		log.Printf("[Prediction] Agent returned empty response")
		return nil, fmt.Errorf("agent returned empty response")
	}

	log.Printf("[Prediction] Agent Raw Response: %s", content) // 打印原始响应，方便调试

	// 清理可能存在的 Markdown 代码块标记
	content = cleanJSONContent(content)
	log.Printf("[Prediction] Cleaned JSON Content: %s", content)

	var result predictionAgent.PredictionResult
	err = json.Unmarshal([]byte(content), &result)
	if err != nil {
		log.Printf("[Prediction] JSON Unmarshal failed: %v", err)
		return nil, fmt.Errorf("failed to parse agent response: %v. Content: %s", err, content)
	}

	if len(result.Questions) == 0 {
		log.Printf("[Prediction] No questions parsed from response")
		return nil, fmt.Errorf("no questions generated")
	}

	// 4. Save to DB
	// 分步保存，以便排查问题
	record := &model.PredictionRecord{
		UserID:     userID,
		ResumeID:   uint64(req.ResumeID),
		Type:       req.PredictionType,
		Language:   req.Language,
		JobTitle:   req.JobTitle,
		Difficulty: req.Difficulty,
	}
	if req.CompanyName != nil {
		record.Company = *req.CompanyName
	}

	// 先只保存主记录
	if err := model.PredictionDao.CreatePredictionRecord(record); err != nil {
		log.Printf("[Prediction] DB Create Main Record failed: %v", err)
		return nil, fmt.Errorf("failed to save main record: %w", err)
	}

	if record.ID == 0 {
		log.Printf("[Prediction] DB Create Main Record Success but ID is 0")
		return nil, fmt.Errorf("database error: record created but ID is 0")
	}

	log.Printf("[Prediction] Main Record Saved, ID: %d", record.ID)

	// 准备保存问题
	var questions []model.PredictionQuestion
	for i, q := range result.Questions {
		// 处理 FollowUp
		var followUpStr string
		switch v := q.FollowUp.(type) {
		case string:
			followUpStr = v
		default:
			// 将其他类型（如数组、对象）转换为 JSON 字符串
			b, err := json.Marshal(v)
			if err != nil {
				log.Printf("[Prediction] Warning: failed to marshal follow_up: %v", err)
				followUpStr = fmt.Sprintf("%v", v)
			} else {
				followUpStr = string(b)
			}
		}

		questions = append(questions, model.PredictionQuestion{
			RecordID:        record.ID, // 显式设置 RecordID
			Question:        q.Question,
			Content:         q.Content,
			Focus:           q.Focus,
			ThinkingPath:    q.ThinkingPath,
			ReferenceAnswer: q.ReferenceAnswer,
			FollowUp:        followUpStr,
			Sort:            i + 1,
		})
	}

	// 保存问题列表
	if err := model.PredictionDao.CreatePredictionQuestions(questions); err != nil {
		log.Printf("[Prediction] DB Create Questions failed: %v", err)
		return nil, fmt.Errorf("failed to save questions: %w", err)
	}

	// 将问题赋值回 record，以便返回
	record.Questions = questions

	log.Printf("[Prediction] Successfully saved questions count: %d", len(questions))

	// 5. Build Response
	var responseQuestions []*predictionIDL.PredictionQuestion
	for _, q := range record.Questions {
		responseQuestions = append(responseQuestions, &predictionIDL.PredictionQuestion{
			ID:              int64(q.ID),
			Question:        q.Question,
			Focus:           q.Focus,
			ThinkingPath:    q.ThinkingPath,
			ReferenceAnswer: q.ReferenceAnswer,
			FollowUp:        q.FollowUp,
			Sort:            int32(q.Sort),
		})
	}

	return &predictionIDL.PredictResponse{
		RecordID:  int64(record.ID),
		Questions: responseQuestions,
	}, nil
}

// cleanJSONContent 辅助函数：清理 markdown 标记
func cleanJSONContent(content string) string {
	content = strings.TrimSpace(content)
	// 去除开头的 ```json 或 ```
	if strings.HasPrefix(content, "```") {
		if idx := strings.Index(content, "\n"); idx != -1 {
			content = content[idx+1:]
		}
	}
	// 去除结尾的 ```
	if strings.HasSuffix(content, "```") {
		content = content[:len(content)-3]
	}
	return strings.TrimSpace(content)
}

func (s *PredictionServiceImpl) ListPredictions(ctx context.Context, req *predictionIDL.ListPredictionRequest, userID uint) (*predictionIDL.ListPredictionResponse, error) {
	page := 1
	size := 10
	if req.Page != nil {
		page = int(*req.Page)
	}
	if req.Size != nil {
		size = int(*req.Size)
	}

	records, total, err := model.PredictionDao.GetPredictionRecordsByUserID(userID, page, size)
	if err != nil {
		return nil, err
	}

	var list []*predictionIDL.PredictionRecordItem
	for _, r := range records {
		list = append(list, &predictionIDL.PredictionRecordItem{
			ID:             int64(r.ID),
			CreatedAt:      r.CreatedAt.Format(time.DateTime),
			JobTitle:       r.JobTitle,
			Difficulty:     r.Difficulty,
			Company:        r.Company,
			PredictionType: r.Type,
			Language:       r.Language,
		})
	}

	return &predictionIDL.ListPredictionResponse{
		List:  list,
		Total: total,
		Page:  int32(page),
		Size:  int32(size),
	}, nil
}

func (s *PredictionServiceImpl) GetPredictionDetail(ctx context.Context, req *predictionIDL.GetPredictionDetailRequest, userID uint) (*predictionIDL.GetPredictionDetailResponse, error) {
	record, err := model.PredictionDao.GetPredictionRecordByID(uint64(req.ID))
	if err != nil {
		return nil, err
	}

	if record.UserID != userID {
		return nil, fmt.Errorf("unauthorized")
	}

	var questions []*predictionIDL.PredictionQuestion
	for _, q := range record.Questions {
		questions = append(questions, &predictionIDL.PredictionQuestion{
			ID:              int64(q.ID),
			Question:        q.Question,
			Focus:           q.Focus,
			ThinkingPath:    q.ThinkingPath,
			ReferenceAnswer: q.ReferenceAnswer,
			FollowUp:        q.FollowUp,
			Sort:            int32(q.Sort),
		})
	}

	return &predictionIDL.GetPredictionDetailResponse{
		ID:        int64(record.ID),
		Questions: questions,
	}, nil
}
