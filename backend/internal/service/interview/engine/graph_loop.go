package core

import (
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"time"

	interviewsapi "interview-agents/api/model/resume"
	evaluator "interview-agents/internal/agents/evaluation"
	"interview-agents/internal/agents/evaluation/topic"
	"interview-agents/internal/agents/usecase/interview"
	"interview-agents/internal/errors"
	"interview-agents/internal/model"
	"interview-agents/internal/observability/looptrace"
	interviewservice "interview-agents/internal/service/resume"
	mycallbacks "interview-agents/pkg/eino/callbacks"

	"github.com/cloudwego/eino/compose"
)

// =============================================================================
// InterviewState - 面试状态（Graph 编排的核心数据结构）
// =============================================================================

// InterviewState 面试状态，在 Graph 各节点间传递
type InterviewState struct {
	// 基础信息
	Session        *InterviewSession
	SessionManager *SessionManager
	InterviewSvc   interviewservice.InterviewManager
	Writer         io.Writer

	// Agent 相关
	AgentSvc       *interview.InterviewAgentService
	AgentType      interview.InterviewAgentType
	NeedResumeTool bool

	// 评分相关
	Evaluator    *evaluator.Evaluator
	ScoreHistory *evaluator.ScoreHistory
	TopicTracker *topic.TopicTracker

	// 当前轮次数据
	QuestionIndex int
	QuestionText  string
	Answer        string
	EvalResult    *evaluator.EvaluationResult

	// 分支节点设置的下一题提示（由 deepen/continue/lower/switch 节点设置）
	NextActionHint string // 下一题的 Prompt 提示

	// 历史记录
	AllDialogues  []*InterviewDialogueData
	RecentHistory []historyItem

	// 配置
	MaxQuestions       int
	HistoryContextSize int
	AnswerTimeout      time.Duration
	HeartbeatInterval  time.Duration

	// 控制标志
	ShouldStop bool
	Error      error
}

type historyItem struct {
	Question string
	Answer   string
}

// =============================================================================
// Graph 节点定义
// =============================================================================

const (
	NodeStart      = "start_init"
	NodeQuestion   = "question"
	NodeWaitAnswer = "wait_answer"
	NodeEvaluate   = "evaluate"
	NodeBranch     = "branch"
	NodeDeepen     = "deepen"
	NodeContinue   = "continue"
	NodeLower      = "lower"
	NodeSwitch     = "switch"
	NodeEnd        = "end_loop"
)

// =============================================================================
// 节点函数实现
// =============================================================================

// startNode 初始化节点
func startNode(ctx context.Context, state *InterviewState) (*InterviewState, error) {
	state.QuestionIndex = 1
	log.Printf("[Graph] Start interview, sessionID: %s", state.Session.SessionID)
	return state, nil
}

func buildQuestionPrompt(state *InterviewState) (prompt string) {
	isGroupInterview := state.AgentType == interview.GroupInterview

	if state.QuestionIndex == 1 {
		prompt = buildFirstQuestionPrompt(state, isGroupInterview, true)
		return
	}

	prompt = buildFollowUpPrompt(state, isGroupInterview, true)
	// 清空 NextActionHint，避免影响下一轮
	state.NextActionHint = ""
	return
}

// buildFirstQuestionPrompt 构建首题提示词
func buildFirstQuestionPrompt(state *InterviewState, isGroupInterview bool, isEnglishInterview bool) string {
	if isEnglishInterview {
		return fmt.Sprintf(`请作为面试官，根据简历和难度等级开始面试。

简历ID：%d
难度：%s

要求：
1. 简短问候候选人，然后直接提出第一个技术问题
2. 仅提出一个问题
3. 请根据候选人使用的语言来回复，如果候选人用中文提问则用中文回复，如果用英文提问则用英文回复
`, state.Session.ResumeId, state.Session.Difficulty)
	}

	if isGroupInterview {
		return fmt.Sprintf(`请作为主面试官，根据简历和难度等级开始这场团面。

		简历ID：%d
		难度：%s

		要求：
		1. 简短问候候选人并介绍面试团
		2. 然后提出第一个技术问题
		3. 包含面试官身份前缀（例如，"我是主面试官："）
		4. 请根据候选人使用的语言来回复，如果候选人用中文提问则用中文回复，如果用英文提问则用英文回复
`, state.Session.ResumeId, state.Session.Difficulty)
	}

	return fmt.Sprintf(`请作为面试官，根据简历和难度等级开始面试。

	简历ID：%d
	难度：%s

	要求：
	1. 简短问候候选人，然后直接提出第一个技术问题
	2. 仅提出一个问题
	3. 请根据候选人使用的语言来回复，如果候选人用中文提问则用中文回复，如果用英文提问则用英文回复
`, state.Session.ResumeId, state.Session.Difficulty)
}

// buildFollowUpPrompt 构建后续问题提示词
func buildFollowUpPrompt(state *InterviewState, isGroupInterview bool, isEnglishInterview bool) string {
	// 构建历史记录文本
	historyText := buildHistoryText(state.RecentHistory, isEnglishInterview)
	actionHint := state.NextActionHint

	if isEnglishInterview {
		return fmt.Sprintf(`根据简历、难度和最近的问答历史，继续面试。

简历ID：%d
难度：%s
当前话题：%s
%s

最近的问答历史（最近%d个问题）：
%s

要求：
1. 根据上述指导提出下一个问题
2. 避免重复之前问过的问题
3. 请根据候选人使用的语言来回复，如果候选人用中文提问则用中文回复，如果用英文提问则用英文回复
`, state.Session.ResumeId, state.Session.Difficulty, state.TopicTracker.CurrentTopic, actionHint, len(state.RecentHistory), historyText)
	}

	if isGroupInterview {
		return fmt.Sprintf(`根据简历、难度和最近的问答历史，继续团面。

		简历ID：%d
		难度：%s
		当前话题：%s
%s

		最近的问答历史（最近%d个问题）：
%s

		要求：
		1. 根据上述指导提出下一个问题
		2. 避免重复之前问过的问题
		3. 包含面试官身份前缀（例如，"我是主面试官："或"我是技术面试官："）
		4. 请根据候选人使用的语言来回复，如果候选人用中文提问则用中文回复，如果用英文提问则用英文回复
`, state.Session.ResumeId, state.Session.Difficulty, state.TopicTracker.CurrentTopic, actionHint, len(state.RecentHistory), historyText)
	}

	return fmt.Sprintf(`根据简历、难度和最近的问答历史，继续面试。

	简历ID：%d
	难度：%s
	当前话题：%s
%s

	最近的问答历史（最近%d个问题）：
%s

	要求：
	1. 根据上述指导提出下一个问题
	2. 避免重复之前问过的问题
	3. 请根据候选人使用的语言来回复，如果候选人用中文提问则用中文回复，如果用英文提问则用英文回复
`, state.Session.ResumeId, state.Session.Difficulty, state.TopicTracker.CurrentTopic, actionHint, len(state.RecentHistory), historyText)
}

// buildHistoryText 构建问答历史文本
func buildHistoryText(history []historyItem, isEnglishInterview bool) string {
	if len(history) == 0 {
		return "（暂无历史记录）"
	}
	var sb strings.Builder
	for i, h := range history {
		sb.WriteString(fmt.Sprintf("问题%d：%s\n回答%d：%s\n\n", i+1, h.Question, i+1, h.Answer))
	}
	return sb.String()
}

// questionNode 生成问题节点
func questionNode(ctx context.Context, state *InterviewState) (*InterviewState, error) {
	if state.QuestionIndex > state.MaxQuestions || state.ShouldStop {
		state.ShouldStop = true
		return state, nil
	}

	// 构建提示词
	prompt := buildQuestionPrompt(state)

	// 调用智能体生成问题
	state.QuestionText = ""
	currentRole := RoleMainInterviewer // 默认主面试官

	err := state.AgentSvc.RunInterviewWithCallback(ctx, state.AgentType, state.NeedResumeTool, prompt, func(message string) error {
		state.QuestionText += message

		// 动态检测角色（从消息内容中）
		if state.QuestionText != "" {
			currentRole = DetectRoleFromContent(state.QuestionText)
		}

		// 发送流式分块消息（支持多路复用）
		return SendChunkMessage(state.Writer, currentRole, message, state.QuestionIndex)
	})

	if err != nil {
		log.Printf("[Graph] Failed to generate question %d: %v", state.QuestionIndex, err)
		state.Error = err
		state.ShouldStop = true
		return state, nil
	}

	if len(state.QuestionText) == 0 {
		state.Error = fmt.Errorf("agent returned empty result")
		state.ShouldStop = true
	}

	// 发送最终完整的结构化消息
	finalMessage := NewMessageSchema(currentRole, state.QuestionText, ActionSpeaking)
	finalMessage.Status = StatusComplete
	finalMessage.Metadata = map[string]interface{}{
		"index":      state.QuestionIndex,
		"total":      state.MaxQuestions,
		"session_id": state.Session.SessionID,
	}
	_ = SendStructuredMessage(state.Writer, finalMessage)

	return state, nil
}

// waitAnswerNode 等待回答节点
func waitAnswerNode(ctx context.Context, state *InterviewState) (*InterviewState, error) {
	if state.ShouldStop {
		return state, nil
	}

	// 发送就绪事件
	SendReadyEventWithSession(state.Writer, state.QuestionIndex, state.Session.SessionID)
	state.SessionManager.ClearAnswer(state.Session.SessionID)

	// 等待用户回答
	log.Printf("[Graph] Waiting for answer, question: %d/%d", state.QuestionIndex, state.MaxQuestions)
	answer, received := WaitForAnswerWithHeartbeat(ctx, state.SessionManager, state.Session.SessionID,
		state.AnswerTimeout, state.HeartbeatInterval, state.Writer)

	if !received {
		log.Printf("[Graph] Answer timeout, question: %d", state.QuestionIndex)
		state.Error = fmt.Errorf("answer timeout")
		state.ShouldStop = true
		return state, nil
	}

	state.Answer = answer
	return state, nil
}

// evaluateNode 评分节点
func evaluateNode(ctx context.Context, state *InterviewState) (*InterviewState, error) {
	if state.ShouldStop {
		return state, nil
	}

	// 保存对话
	dialogue := &InterviewDialogueData{
		Question: state.QuestionText,
		Answer:   state.Answer,
	}

	// 评分
	result, err := state.Evaluator.EvaluateWithHistory(ctx, &evaluator.EvaluationRequest{
		Domain:       state.Session.Domain,
		CurrentTopic: state.TopicTracker.CurrentTopic,
		Question:     state.QuestionText,
		Answer:       state.Answer,
	}, state.ScoreHistory)

	if err != nil {
		log.Printf("[Graph] Evaluation failed: %v", err)
		// 评分失败不影响主流程
		state.EvalResult = &evaluator.EvaluationResult{
			NextAction: evaluator.ActionContinue,
		}
	} else {
		state.EvalResult = result
		dialogue.Score = result.Overall
		dialogue.Topics = result.CoveredTopics

		state.TopicTracker.UpdateCoverage(result.CoveredTopics, result.Overall, state.TopicTracker.CurrentTopic)

		log.Printf("[Graph] Evaluation for Q%d: score=%.2f, action=%s, topics=%v",
			state.QuestionIndex, result.Overall, result.NextAction, result.CoveredTopics)
	}

	state.AllDialogues = append(state.AllDialogues, dialogue)
	// 同步到 Session 快照：用户主动结束或 context cancel 时 Invoke 可能拿不到 finalState，仍可依此落库。
	if state.Session != nil {
		state.Session.AppendDialogueSnapshot(dialogue)
	}

	// 更新历史
	state.RecentHistory = append(state.RecentHistory, historyItem{
		Question: state.QuestionText,
		Answer:   state.Answer,
	})
	if len(state.RecentHistory) > state.HistoryContextSize {
		state.RecentHistory = state.RecentHistory[len(state.RecentHistory)-state.HistoryContextSize:]
	}

	// 更新会话计数
	state.Session.QuestionCount = int32(state.QuestionIndex)

	// 保存 dialogue 数据
	err = saveDialogue(ctx, state.Session, dialogue)
	if err != nil {
		return state, err
	}

	// 发送进度
	_ = SendSSEEvent(state.Writer, map[string]interface{}{
		"type":     "answer_received",
		"index":    state.QuestionIndex,
		"total":    state.MaxQuestions,
		"progress": float64(state.QuestionIndex) / float64(state.MaxQuestions) * 100,
	})

	return state, nil
}

// deepenNode 深入追问节点 (高分分支)
func deepenNode(ctx context.Context, state *InterviewState) (*InterviewState, error) {
	log.Printf("[Graph] Branch: DEEPEN - Q%d score=%.2f, going deeper", state.QuestionIndex, state.EvalResult.Overall)
	// 设置下一题提示：深入追问（自然过渡，不暴露策略）
	state.NextActionHint = `根据候选人最近的回答，就相关的技术细节或原理提出更深入的追问。
语气要求：
- 保持自然过渡，就像真实对话中一样
- 不要使用生硬的元描述如"让我们深入讨论"
- 直接提出问题，保持流畅
追问方向（选择一个）：
- 回答中提到的概念的底层实现
- 边界情况下的行为
- 性能优化或最佳实践`

	state.QuestionIndex++
	return state, nil
}

// continueNode 继续当前话题节点 (中等分数分支)
func continueNode(ctx context.Context, state *InterviewState) (*InterviewState, error) {
	log.Printf("[Graph] Branch: CONTINUE - Q%d score=%.2f, continuing topic", state.QuestionIndex, state.EvalResult.Overall)

	// 设置下一题提示：继续当前话题（自然过渡）
	state.NextActionHint = `继续评估当前话题下的其他知识点。
语气要求：
- 使用自然过渡，如同正常技术讨论
- 避免元描述如"继续当前话题"
- 保持对话感和流畅性
评估方向：
- 当前话题下尚未覆盖的子话题
- 同一技术在不同实际场景中的应用
- 相关的工程实践问题`

	state.QuestionIndex++
	return state, nil
}

// lowerNode 降低难度节点 (低分分支)
func lowerNode(ctx context.Context, state *InterviewState) (*InterviewState, error) {
	log.Printf("[Graph] Branch: LOWER - Q%d score=%.2f, lowering difficulty", state.QuestionIndex, state.EvalResult.Overall)
	// 设置下一题提示：降低难度（自然过渡，不暴露意图）
	state.NextActionHint = `切换到一个更基础的问题来继续评估候选人。
语气要求（重要）：
- 绝不要暗示上一个回答不好
- 使用自然过渡，保持友好语气
- 避免说"让我们简化这个问题"之类的话
问题选择：
- 同一话题下更基础的概念
- 候选人在日常工作中应该使用的知识
- 从具体用例出发而非抽象理论`

	state.QuestionIndex++
	return state, nil
}

// switchNode 切换话题节点
func switchNode(ctx context.Context, state *InterviewState) (*InterviewState, error) {
	newTopic := state.TopicTracker.SuggestNextTopic()

	log.Printf("[Graph] Branch: SWITCH - Q%d switching to topic: %s", state.QuestionIndex, newTopic)
	state.TopicTracker.CurrentTopic = newTopic

	// 更新当前话题
	state.TopicTracker.CurrentTopic = newTopic

	// 设置下一题提示：切换话题（自然过渡）
	state.NextActionHint = fmt.Sprintf(`切换到新话题"%s"并提出下一个问题。
语气要求（重要）：
- 自然过渡，不要明确说你在切换话题
- 不要解释为什么话题改变了
- 保持过渡的对话感和流畅性
问题选择：
- 新话题下的入门级问题
- 优先选择与候选人简历/经验相关的角度
- 使用实际问题来鼓励具体回答`, newTopic)

	state.QuestionIndex++
	return state, nil
}

// endNode 结束节点
func endNode(ctx context.Context, state *InterviewState) (*InterviewState, error) {
	log.Printf("[Graph] End interview, total questions: %d", len(state.AllDialogues))
	return state, nil
}

// =============================================================================
// Branch 条件函数
// =============================================================================

// branchCondition 根据评分结果决定下一步
func branchCondition(ctx context.Context, state *InterviewState) (string, error) {
	// 检查是否应该结束
	if state.ShouldStop || state.QuestionIndex >= state.MaxQuestions {
		return NodeEnd, nil
	}

	// 根据评分结果路由
	if state.EvalResult == nil {
		return NodeContinue, nil
	}

	switch state.EvalResult.NextAction {
	case evaluator.ActionDeepen:
		return NodeDeepen, nil
	case evaluator.ActionLower:
		return NodeLower, nil
	case evaluator.ActionSwitch:
		return NodeSwitch, nil
	default:
		return NodeContinue, nil
	}
}

// shouldContinueLoop 检查是否继续循环
func shouldContinueLoop(ctx context.Context, state *InterviewState) (string, error) {
	if state.ShouldStop || state.QuestionIndex > state.MaxQuestions {
		return NodeEnd, nil
	}
	return NodeQuestion, nil
}

// =============================================================================
// RunInterviewLoopWithGraph - Graph 编排版本的面试循环
// =============================================================================

// RunInterviewLoopWithGraph 使用 Eino Graph 编排运行面试循环
func (e *InterviewEngine) RunInterviewLoopWithGraph(ctx context.Context, session *InterviewSession) {
	// 12.1.1 全链路监控：整场面试共享同一 TraceID，便于日志聚合与回溯
	ctx = mycallbacks.WithTraceID(ctx, session.SessionID)
	// 11.2.3 Token 监控与配额：注入 UserID 供 OnEnd 记录消耗；开场前检查当日配额
	ctx = mycallbacks.WithUserID(ctx, session.UserID)
	if mycallbacks.DefaultTokenRecorder != nil {
		if err := mycallbacks.DefaultTokenRecorder.CheckQuota(ctx, session.UserID); err != nil {
			log.Printf("[Graph] Token quota exceeded for user %d: %v", session.UserID, err)
			SendErrorEvent(e.writer, err.Error())
			return
		}
	}

	const answerTimeout = 30 * time.Minute
	const heartbeatInterval = 15 * time.Second
	const maxQuestions = 10      // 最多生成10道问题
	const historyContextSize = 2 // 保留前2道题作为历史上下文

	// 创建智能体服务
	agentSvc := interview.NewInterviewAgentService(session.UserID)
	agentType := e.selectAgentType(session)
	if nextCtx, span, ok := looptrace.StartSpan(ctx, "interview.loop", "custom"); ok && span != nil {
		ctx = nextCtx
		looptrace.ApplyCommonFields(ctx, span, strconv.FormatUint(uint64(session.UserID), 10), session.SessionID, map[string]interface{}{
			"scene":      "interview",
			"agent_type": agentType,
		})
		defer span.Finish(ctx)
	}

	// 创建评分器
	evaluatorInstance, err := evaluator.NewEvaluator(ctx, session.UserID, nil)
	if err != nil {
		log.Printf("[Graph] Failed to create evaluator: %v", err)
		SendErrorEvent(e.writer, fmt.Sprintf("Failed to create evaluator: %s", err.Error()))
		return
	}

	// 初始化状态
	state := &InterviewState{
		Session:            session,
		SessionManager:     e.sessionManager,
		InterviewSvc:       e.interviewSvc,
		Writer:             e.writer,
		AgentSvc:           agentSvc,
		AgentType:          agentType,
		NeedResumeTool:     session.HasResume,
		Evaluator:          evaluatorInstance,
		ScoreHistory:       &evaluator.ScoreHistory{},
		TopicTracker:       topic.NewTopicTracker(session.Domain),
		AllDialogues:       make([]*InterviewDialogueData, 0),
		RecentHistory:      make([]historyItem, 0),
		MaxQuestions:       maxQuestions,
		HistoryContextSize: historyContextSize,
		AnswerTimeout:      answerTimeout,
		HeartbeatInterval:  heartbeatInterval,
	}

	// 构建 Graph
	graph := compose.NewGraph[*InterviewState, *InterviewState]()

	// 添加节点
	_ = graph.AddLambdaNode(NodeStart, compose.InvokableLambda(startNode))       // 面试开场, 初始化
	_ = graph.AddLambdaNode(NodeQuestion, compose.InvokableLambda(questionNode)) // 生成/发送问题
	_ = graph.AddLambdaNode(NodeWaitAnswer, compose.InvokableLambda(waitAnswerNode))
	_ = graph.AddLambdaNode(NodeEvaluate, compose.InvokableLambda(evaluateNode))
	_ = graph.AddLambdaNode(NodeDeepen, compose.InvokableLambda(deepenNode))
	_ = graph.AddLambdaNode(NodeContinue, compose.InvokableLambda(continueNode))
	_ = graph.AddLambdaNode(NodeLower, compose.InvokableLambda(lowerNode))
	_ = graph.AddLambdaNode(NodeSwitch, compose.InvokableLambda(switchNode))
	_ = graph.AddLambdaNode(NodeEnd, compose.InvokableLambda(endNode))

	// 添加边
	_ = graph.AddEdge(compose.START, NodeStart)
	_ = graph.AddEdge(NodeStart, NodeQuestion)
	_ = graph.AddEdge(NodeQuestion, NodeWaitAnswer)
	_ = graph.AddEdge(NodeWaitAnswer, NodeEvaluate)

	// 添加 Branch: Evaluate -> (Deepen/Continue/Lower/Switch/End)
	evalBranch := compose.NewGraphBranch(branchCondition, map[string]bool{
		NodeDeepen:   true,
		NodeContinue: true,
		NodeLower:    true,
		NodeSwitch:   true,
		NodeEnd:      true,
	})
	_ = graph.AddBranch(NodeEvaluate, evalBranch)

	// 各分支回到 Question 或结束
	loopBranch := compose.NewGraphBranch(shouldContinueLoop, map[string]bool{
		NodeQuestion: true,
		NodeEnd:      true,
	})
	_ = graph.AddBranch(NodeDeepen, loopBranch)
	_ = graph.AddBranch(NodeContinue, loopBranch)
	_ = graph.AddBranch(NodeLower, loopBranch)
	_ = graph.AddBranch(NodeSwitch, loopBranch)

	// End 节点连接到 END
	_ = graph.AddEdge(NodeEnd, compose.END)

	// 编译 Graph（设置足够大的 MaxRunSteps 以支持循环）
	// 每个问题大约经过 6 个节点，maxQuestions * 6 + 余量
	maxRunSteps := maxQuestions*10 + 20
	runnable, err := graph.Compile(ctx, compose.WithMaxRunSteps(maxRunSteps))
	if err != nil {
		log.Printf("[Graph] Failed to compile graph: %v", err)
		SendErrorEvent(e.writer, fmt.Sprintf("Failed to compile graph: %s", err.Error()))
		return
	}

	// 运行 Graph
	log.Printf("[Graph] Starting interview graph, sessionID: %s", session.SessionID)
	finalState, err := runnable.Invoke(ctx, state)
	if finalState != nil && finalState.Error != nil {
		err = finalState.Error
	}
	if err != nil {
		log.Printf("[Graph] Graph execution failed: %v", err)

		// 图中途失败或 context cancel 时，尽力保存已产生的问答：
		// - 优先用 finalState.AllDialogues（模型错误等非 cancel 场景）
		// - cancel 时 Invoke 常返回 nil finalState，改用 Session 上每轮 evaluate 写入的快照
		if !session.IsDialoguesPersisted() {
			var dlg []*InterviewDialogueData
			if finalState != nil && len(finalState.AllDialogues) > 0 {
				dlg = finalState.AllDialogues
			} else {
				dlg = session.GetDialoguesSnapshot()
			}
			if len(dlg) > 0 {
				if saveErr := e.saveAllDialogues(ctx, session, dlg); saveErr != nil {
					log.Printf("[Graph] Failed to save dialogues after graph error: %v", saveErr)
				} else {
					log.Printf("[Graph] Saved %d dialogue(s) after graph error, reportID: %d", len(dlg), session.RecordID)
					session.MarkDialoguesPersisted()
					endTime := time.Now()
					session.LastActivity = endTime
					duration := int64(endTime.Sub(session.StartTime).Seconds())
					st := "failed"
					if stderrors.Is(err, context.Canceled) {
						st = "aborted"
					}
					updateDTO := &interviewsapi.InterviewRecordDTO{
						ID:       int64(session.RecordID),
						UserID:   int32(session.UserID),
						Status:   st,
						Duration: &duration,
					}
					if upErr := e.interviewSvc.UpdateInterviewRecord(ctx, updateDTO); upErr != nil {
						log.Printf("[Graph] Failed to update interview record after partial save: %v", upErr)
					}
				}
			}
		}

		// 检查是否是大模型不可用错误
		// 注意: 引发的 error 被 eino wrapping 为 [NodeRunError], default errors.As 可能匹配不到深层结构
		// 因此增加 string contain 判断，或者直接尝试 errors.As
		var unavailableErr *errors.ModelUnavailableError

		isUnavailable := false

		if strings.Contains(err.Error(), "NodeRunError") {
			// Fallback: 如果被 wrap 到透支，但 string 包含，我们当做 failover 切 (临时构建一个空 name 的 err，避免进程强行退)
			isUnavailable = true
			unavailableErr = &errors.ModelUnavailableError{OriginalErr: err, ModelName: "Current Model"}
		}

		if isUnavailable {
			// 查出候选备用模型 (我们不知道具体 failedModelID，传 0 就全查出来让前端排除)
			backupModels, listErr := model.UserModelDao.ListBackupModels(int64(session.UserID), 0)

			if listErr != nil || len(backupModels) == 0 {
				log.Printf("[Graph] No backup models available for failover for user %d", session.UserID)
				SendErrorEvent(e.writer, fmt.Sprintf("大模型调用失败，且您没有备用的可用模型可供切换，请前往设置。原始错误：%v", err))
			} else {
				// 构造带备用模型的事件
				failoverData := map[string]interface{}{
					"failed_model_name": unavailableErr.ModelName,
					"error_reason":      err.Error(),
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
			return
		}

		SendErrorEvent(e.writer, fmt.Sprintf("Interview failed: %s", err.Error()))
		return
	}

	// 保存到数据库
	log.Printf("[Graph] Saving %d questions to database", len(finalState.AllDialogues))
	if err := e.saveAllDialogues(ctx, session, finalState.AllDialogues); err != nil {
		log.Printf("[Graph] Failed to save dialogues: %v", err)
		SendErrorEvent(e.writer, "Failed to save interview data: "+err.Error())
		SendCompleteEvent(e.writer)
		return
	}
	session.MarkDialoguesPersisted()
	endTime := time.Now()
	session.LastActivity = endTime
	session.Status = "completed"
	duration := int64(endTime.Sub(session.StartTime).Seconds())
	updateDTO := &interviewsapi.InterviewRecordDTO{
		ID:       int64(session.RecordID),
		UserID:   int32(session.UserID),
		Status:   "completed",
		Duration: &duration,
	}
	if err := e.interviewSvc.UpdateInterviewRecord(ctx, updateDTO); err != nil {
		log.Printf("[Graph] Failed to update interview record status: %v", err)
	}

	// 发送完成事件
	_ = SendSSEEvent(e.writer, map[string]interface{}{
		"type": "complete",
		"data": map[string]interface{}{
			"total_questions": len(finalState.AllDialogues),
		},
	})
	SendCompleteEvent(e.writer)

	// 发布后续消息
	// mq.PublishEvaluationReport(ctx, session.UserID, session.RecordID)
	// mq.PublishTopicEvaluation(ctx, session.UserID, session.RecordID)
	e.sessionManager.DeleteSession(session.SessionID)
	log.Printf("[Graph] Interview completed, sessionID: %s", session.SessionID)
}

// saveAllDialoguesFromGraph 保存对话（复用现有方法）
// Deprecated 暂时没看到有用到的地方，冗余代码，建议删除
func (e *InterviewEngine) saveAllDialoguesFromGraph(ctx context.Context, session *InterviewSession, dialogues []*InterviewDialogueData) error {
	for i, q := range dialogues {
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
	}
	return nil
}

// saveDialogue 保存 Dialogue 数据
func saveDialogue(ctx context.Context, session *InterviewSession, dialogueData *InterviewDialogueData) error {
	dialogue := &model.InterviewDialogue{
		UserID:    session.UserID,
		ReportID:  session.RecordID,
		Question:  dialogueData.Question,
		Answer:    dialogueData.Answer,
		CreatedAt: time.Now(),
	}
	if err := model.InterviewDialogueDao.Create(dialogue); err != nil {
		return fmt.Errorf("failed to save question %w", err)
	}
	return nil
}
