package impl

import (
	"context"
	"encoding/json"
	"fmt"
	predictionIDL "interview-agents/api/model/prediction"
	predictionAgent "interview-agents/internal/agents/prediction"
	"interview-agents/internal/model"
	"interview-agents/internal/service/prediction"
	"log"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"golang.org/x/sync/errgroup"
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

	// 2. 先拆分方向，再并行生成题目
	splitAgent, err := predictionAgent.NewResumeSplitAgent(userID)
	if err != nil {
		log.Printf("[Prediction] Failed to create resume split agent: %v", err)
		return nil, err
	}

	splitPrompt := s.buildSplitPrompt(resume.Content, req)
	log.Printf("[Prediction] Calling split agent with prompt length: %d", len(splitPrompt))

	splitContent, err := runAgentAndCollectContent(ctx, splitAgent, splitPrompt)
	if err != nil {
		return nil, err
	}

	var splitResult predictionAgent.ResumeSplitResult
	if err = json.Unmarshal([]byte(cleanJSONContent(splitContent)), &splitResult); err != nil {
		log.Printf("[Prediction] Split JSON Unmarshal failed: %v", err)
		return nil, fmt.Errorf("failed to parse split agent response: %v", err)
	}

	if len(splitResult.Directions) != predictionAgent.DirectionCount {
		return nil, fmt.Errorf("split directions mismatch: got %d, expect %d", len(splitResult.Directions), predictionAgent.DirectionCount)
	}

	seenDirections := make(map[string]struct{}, predictionAgent.DirectionCount)
	for _, module := range splitResult.Directions {
		direction := strings.TrimSpace(module.Direction)
		content := strings.TrimSpace(module.Content)
		if direction == "" || content == "" {
			return nil, fmt.Errorf("invalid split result: direction and content must be non-empty")
		}
		if _, exists := seenDirections[direction]; exists {
			return nil, fmt.Errorf("invalid split result: duplicated direction %q", direction)
		}
		seenDirections[direction] = struct{}{}
	}

	baseRequirements := s.buildRequirements(req)

	directionResults := make([][]predictionAgent.PredictionQuestion, len(splitResult.Directions))
	g, gctx := errgroup.WithContext(ctx)
	for i, module := range splitResult.Directions {
		i, module := i, module
		g.Go(func() error {
			agent, err := predictionAgent.NewDirectionalPredictionAgent(userID, module.Direction)
			if err != nil {
				return err
			}

			directionPrompt := s.buildDirectionPrompt(module, baseRequirements)
			content, err := runAgentAndCollectContent(gctx, agent, directionPrompt)
			if err != nil {
				return err
			}

			var result predictionAgent.PredictionResult
			if err := json.Unmarshal([]byte(cleanJSONContent(content)), &result); err != nil {
				return fmt.Errorf("failed to parse direction %q response: %w", module.Direction, err)
			}

			if len(result.Questions) != predictionAgent.QuestionsPerDirection {
				return fmt.Errorf("direction %q questions mismatch: got %d, expect %d", module.Direction, len(result.Questions), predictionAgent.QuestionsPerDirection)
			}

			for idx := range result.Questions {
				if strings.TrimSpace(result.Questions[idx].Content) == "" {
					result.Questions[idx].Content = fmt.Sprintf("Direction: %s", module.Direction)
				} else {
					result.Questions[idx].Content = fmt.Sprintf("[%s] %s", module.Direction, result.Questions[idx].Content)
				}
				if strings.TrimSpace(result.Questions[idx].Focus) == "" {
					result.Questions[idx].Focus = module.Direction
				} else {
					result.Questions[idx].Focus = fmt.Sprintf("[%s] %s", module.Direction, result.Questions[idx].Focus)
				}
			}

			directionResults[i] = result.Questions
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		log.Printf("[Prediction] Parallel generation failed: %v", err)
		return nil, err
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
	sort := 1
	for _, moduleQuestions := range directionResults {
		for _, q := range moduleQuestions {
			// 处理 FollowUp
			followUpStr := normalizeFollowUp(q.FollowUp)

			questions = append(questions, model.PredictionQuestion{
				RecordID:        record.ID,
				Question:        q.Question,
				Content:         q.Content,
				Focus:           q.Focus,
				ThinkingPath:    q.ThinkingPath,
				ReferenceAnswer: q.ReferenceAnswer,
				FollowUp:        followUpStr,
				Sort:            sort,
			})
			sort++
		}
	}

	if len(questions) != predictionAgent.TotalPredictionQuestions {
		return nil, fmt.Errorf("generated questions mismatch: got %d, expect %d", len(questions), predictionAgent.TotalPredictionQuestions)
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
			Content:         q.Content,
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

func (s *PredictionServiceImpl) buildRequirements(req *predictionIDL.PredictRequest) string {
	requirements := fmt.Sprintf(`题目生成要求：
- 类型：%s
- 输出语言：中文
- 职位：%s
- 难度：%s
`, req.PredictionType, req.JobTitle, req.Difficulty)

	if req.CompanyName != nil {
		requirements += fmt.Sprintf("- 目标公司：%s\n", *req.CompanyName)
	}
	requirements += "- 所有生成内容请使用中文。\n"

	return requirements
}

func (s *PredictionServiceImpl) buildSplitPrompt(resumeContent string, req *predictionIDL.PredictRequest) string {
	return fmt.Sprintf(`简历内容：
%s

请将上述简历拆分为%d个方向模块。
每个方向模块应支持独立的面试题目生成。

%s

重要：JSON中的值请使用中文填写。`, resumeContent, predictionAgent.DirectionCount, s.buildRequirements(req))
}

func (s *PredictionServiceImpl) buildDirectionPrompt(module predictionAgent.DirectionModule, requirements string) string {
	return fmt.Sprintf(`Direction: %s

方向模块内容：
%s

%s
请为该方向生成恰好%d道面试题目。
所有内容请使用中文。`, module.Direction, module.Content, requirements, predictionAgent.QuestionsPerDirection)
}

func runAgentAndCollectContent(ctx context.Context, agent adk.Agent, prompt string) (string, error) {
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent})
	messages := []adk.Message{schema.UserMessage(prompt)}
	iter := runner.Run(ctx, messages)

	var content string
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			return "", fmt.Errorf("agent generation failed: %w", event.Err)
		}

		if event.Output != nil && event.Output.MessageOutput != nil {
			msg := event.Output.MessageOutput.Message.Content
			if msg != "" {
				content = msg
			}
		}
	}

	if content == "" {
		return "", fmt.Errorf("agent returned empty response")
	}

	return content, nil
}

func normalizeFollowUp(v any) string {
	switch follow := v.(type) {
	case string:
		return follow
	default:
		b, err := json.Marshal(follow)
		if err != nil {
			log.Printf("[Prediction] Warning: failed to marshal follow_up: %v", err)
			return fmt.Sprintf("%v", follow)
		}
		return string(b)
	}
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
			Content:         q.Content,
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
