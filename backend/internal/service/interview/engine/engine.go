package core

import (
	"context"
	"errors"
	"fmt"
	"interview-agents/internal/mq"
	"io"
	"log"
	"time"

	"interview-agents/internal/agents/usecase/interview"
	intErr "interview-agents/internal/errors"
	"interview-agents/internal/model"
	interviewservice "interview-agents/internal/service/resume"
)

// InterviewDialogueData 对话数据结构
type InterviewDialogueData struct {
	Question string
	Answer   string
	Score    float64  // 评分 (1-10)
	Topics   []string // 覆盖的知识点
}

// InterviewEngine 面试引擎 - 处理核心面试逻辑
type InterviewEngine struct {
	sessionManager *SessionManager
	interviewSvc   interviewservice.InterviewManager
	writer         io.Writer
}

// NewInterviewEngine 创建面试引擎
func NewInterviewEngine(sessionManager *SessionManager, interviewSvc interviewservice.InterviewManager, writer io.Writer) *InterviewEngine {
	return &InterviewEngine{
		sessionManager: sessionManager,
		interviewSvc:   interviewSvc,
		writer:         writer,
	}
}

// RunInterviewLoop 运行面试循环
// 新逻辑：逐个生成问题，每次生成一道，用户回答后再生成下一道
// 保留前2道题的历史作为上下文
func (e *InterviewEngine) RunInterviewLoop(ctx context.Context, session *InterviewSession) {
	const answerTimeout = 30 * time.Minute
	const heartbeatInterval = 15 * time.Second
	const maxQuestions = 10      // 最多生成10道问题
	const historyContextSize = 2 // 保留前2道题作为历史上下文

	// 创建智能体服务
	agentSvc := interview.NewInterviewAgentService(session.UserID)

	// 确定智能体类型（移除 Planner 依赖，直接使用会话信息）
	var agentType interview.InterviewAgentType
	var needResumeTool bool
	agentType = e.selectAgentType(session)
	needResumeTool = session.HasResume

	// 用于存储所有问题和回答
	var allDialogues []*InterviewDialogueData

	// 用于存储最近的历史记录（前2道题）
	type HistoryItem struct {
		Question string
		Answer   string
	}
	var recentHistory []HistoryItem

	// 循环生成30道问题
	for questionIndex := 1; questionIndex <= maxQuestions; questionIndex++ {
		select {
		case <-ctx.Done():
			log.Printf("[Interview Engine] Context cancelled, sessionID: %s, questions generated: %d", session.SessionID, questionIndex-1)
			return
		default:
		}

		// 构建提示词
		var prompt string
		if questionIndex == 1 {
			prompt = fmt.Sprintf(`Please act as the lead interviewer and start this interview based on the resume and difficulty level.

		Resume ID: %d
		Difficulty: %s

		Requirements:
		1. Briefly greet the candidate and introduce the interview panel, then ask the first technical question
		2. Include an interviewer identity prefix (for example, "I am the main interviewer:")
		3. Keep the output in English only
`, session.ResumeId, session.Difficulty)
		} else {
			// 后续问题：包含最近2道题的历史上下文
			historyText := ""
			for i, h := range recentHistory {
				historyText += fmt.Sprintf("Question %d: %s\nAnswer %d: %s\n\n", i+1, h.Question, i+1, h.Answer)
			}

			prompt = fmt.Sprintf(`Based on the resume ID, difficulty, and recent Q&A history, continue the interview.
If you (the main interviewer) decide this turn should be handled by the technical interviewer or project interviewer, call the corresponding tool to generate the next question.

		Resume ID: %d
		Difficulty: %s

		Recent Q&A history (latest %d question(s)):
%s

		Requirements:
		1. Use the candidate's previous answers to move to the next stage or deepen the current topic
		2. Avoid repeating previously asked questions
		3. Increase depth progressively
		4. Include an interviewer identity prefix (for example, "I am the main interviewer:" or another interviewer prefix)
		5. Keep the output in English only
`, session.ResumeId, session.Difficulty, len(recentHistory), historyText)
		}

		// 调用智能体生成一道问题
		questionText := ""
		currentRole := RoleMainInterviewer // 默认主面试官

		err := agentSvc.RunInterviewWithCallback(ctx, agentType, needResumeTool, prompt, func(message string) error {
			questionText += message

			// 动态检测角色（从消息内容中）
			if questionText != "" {
				currentRole = DetectRoleFromContent(questionText)
			}

			// 发送流式分块消息（支持多路复用）
			return SendChunkMessage(e.writer, currentRole, message, questionIndex)
		})

		if err != nil {
			log.Printf("[Interview Engine] Failed to generate question %d: %v, sessionID: %s", questionIndex, err, session.SessionID)

			// 检查是否是大模型不可用错误
			var unavailableErr *intErr.ModelUnavailableError
			if errors.As(err, &unavailableErr) {
				// 查出候选备用模型
				backupModels, listErr := model.UserModelDao.ListBackupModels(int64(session.UserID), 0)
				if listErr != nil || len(backupModels) == 0 {
					log.Printf("[Interview Engine] No backup models available for failover for user %d", session.UserID)
					SendErrorEvent(e.writer, fmt.Sprintf("大模型调用失败，且您没有备用的可用模型可供切换，请前往设置。原始错误：%v", unavailableErr.OriginalErr))
				} else {
					// 构造带备用模型的事件
					failoverData := map[string]interface{}{
						"failed_model_name": unavailableErr.ModelName,
						"error_reason":      unavailableErr.OriginalErr.Error(),
						"available_models":  make([]map[string]interface{}, 0, len(backupModels)),
					}
					for _, bm := range backupModels {
						failoverData["available_models"] = append(failoverData["available_models"].([]map[string]interface{}), map[string]interface{}{
							"id":   bm.ID,
							"name": bm.Name,
						})
					}
					_ = SendSSEEvent(e.writer, map[string]interface{}{
						"type": "model_failover_required",
						"data": failoverData,
					})
				}
				break
			}

			SendErrorEvent(e.writer, fmt.Sprintf("Failed to generate question %d: %s", questionIndex, err.Error()))
			break
		}

		// 发送最终完整的结构化消息
		if len(questionText) > 0 {
			finalMessage := NewMessageSchema(currentRole, questionText, ActionSpeaking)
			finalMessage.Status = StatusComplete
			finalMessage.Metadata = map[string]interface{}{
				"index":      questionIndex,
				"total":      maxQuestions,
				"session_id": session.SessionID,
			}
			_ = SendStructuredMessage(e.writer, finalMessage)
		}

		if len(questionText) == 0 {
			log.Printf("[Interview Engine] Agent returned empty result for question %d, sessionID: %s", questionIndex, session.SessionID)
			SendErrorEvent(e.writer, fmt.Sprintf("Agent returned empty result for question %d", questionIndex))
			break
		}

		// 发送就绪事件
		SendReadyEventWithSession(e.writer, questionIndex, session.SessionID)
		e.sessionManager.ClearAnswer(session.SessionID)

		// 等待用户回答
		log.Printf("[Interview Engine] Waiting for answer, sessionID: %s, question: %d/%d", session.SessionID, questionIndex, maxQuestions)
		answer, received := WaitForAnswerWithHeartbeat(ctx, e.sessionManager, session.SessionID, answerTimeout, heartbeatInterval, e.writer)
		if !received {
			log.Printf("[Interview Engine] Answer timeout, sessionID: %s, question: %d", session.SessionID, questionIndex)
			SendErrorEvent(e.writer, fmt.Sprintf("Question %d timeout", questionIndex))
			break
		}

		// 保存当前问题和回答
		dialogue := &InterviewDialogueData{
			Question: questionText,
			Answer:   answer,
		}
		allDialogues = append(allDialogues, dialogue)

		// 更新会话中的问题计数
		session.QuestionCount = int32(questionIndex)

		// 更新最近的历史记录（保留最近2道题）
		recentHistory = append(recentHistory, HistoryItem{
			Question: questionText,
			Answer:   answer,
		})
		if len(recentHistory) > historyContextSize {
			recentHistory = recentHistory[len(recentHistory)-historyContextSize:]
		}

		// 发送进度事件
		err = SendSSEEvent(e.writer, map[string]interface{}{
			"type":     "answer_received",
			"index":    questionIndex,
			"total":    maxQuestions,
			"progress": float64(questionIndex) / float64(maxQuestions) * 100,
		})
		if err != nil {
			log.Printf("[Interview Engine] Failed to send answer_received event: %v", err)
		}

		log.Printf("[Interview Engine] Question %d answered, sessionID: %s", questionIndex, session.SessionID)
	}

	// 所有问题都已回答，保存到数据库
	log.Printf("[Interview Engine] All questions answered, saving %d questions to database, sessionID: %s", len(allDialogues), session.SessionID)
	err := e.saveAllDialogues(ctx, session, allDialogues)
	if err != nil {
		log.Printf("[Interview Engine] Failed to save dialogues: %v, sessionID: %s", err, session.SessionID)
		SendErrorEvent(e.writer, "Failed to save interview data: "+err.Error())
		SendCompleteEvent(e.writer)
		return
	}

	// 发送完成事件
	SendCompleteEvent(e.writer)

	// 发布评估报告生成消息
	if err := mq.PublishEvaluationReport(ctx, session.UserID, session.RecordID); err != nil {
		log.Printf("[Interview Loop] Failed to publish evaluation report message: %v, sessionID: %s", err, session.SessionID)
	}

	// 发布主题评估消息
	if err := mq.PublishTopicEvaluation(ctx, session.UserID, session.RecordID); err != nil {
		log.Printf("[Interview Loop] Failed to publish topic evaluation message: %v, sessionID: %s", err, session.SessionID)
	}
}

// PersistInterviewDialogues 将问答写入 interview_dialogues（正常结束、图中途失败、用户主动结束等路径复用）。
func PersistInterviewDialogues(ctx context.Context, session *InterviewSession, questions []*InterviewDialogueData) error {
	if session == nil || len(questions) == 0 {
		return nil
	}
	log.Printf("[Interview Engine] Saving %d question(s) to database, sessionID: %s", len(questions), session.SessionID)
	for i, q := range questions {
		dialogue := &model.InterviewDialogue{
			UserID:    session.UserID,
			ReportID:  session.RecordID,
			Question:  q.Question,
			Answer:    q.Answer,
			CreatedAt: time.Now(),
		}
		if err := model.InterviewDialogueDao.Create(dialogue); err != nil {
			return fmt.Errorf("failed to save question %d: %w", i+1, err)
		}
		if (i+1)%10 == 0 {
			log.Printf("[Interview Engine] Saved %d/%d questions, sessionID: %s", i+1, len(questions), session.SessionID)
		}
	}
	log.Printf("[Interview Engine] Successfully saved %d question(s), sessionID: %s", len(questions), session.SessionID)
	return nil
}

// saveAllDialogues 保存所有问题到数据库
func (e *InterviewEngine) saveAllDialogues(ctx context.Context, session *InterviewSession, questions []*InterviewDialogueData) error {
	return PersistInterviewDialogues(ctx, session, questions)
}

// selectAgentType 根据面试类型和领域选择智能体类型
func (e *InterviewEngine) selectAgentType(session *InterviewSession) interview.InterviewAgentType {
	// 多人面试（多智能体协作）
	if session.Type == "多人模拟面试" {
		return interview.GroupInterview
	}

	// 综合面试
	if session.Type == "综合面试" {
		switch session.Domain {
		case "校招简历面试":
			return interview.ComprehensiveSchool
		case "社招简历面试":
			return interview.ComprehensiveSocial
		default:
			// 社招简历面试为默认选项
			return interview.ComprehensiveSocial
		}
	}

	// 专项面试
	switch session.Domain {
	case "Java":
		return interview.SpecializedJava
	case "MQ":
		return interview.SpecializedMQ
	case "MySQL":
		return interview.SpecializedMySQL
	case "Redis":
		return interview.SpecializedRedis
	case "Go":
		fallthrough
	default:
		return interview.SpecializedGo
	}
}
