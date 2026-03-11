package evaluation

// NextAction 评分后建议的下一步动作
type NextAction string

const (
	// ActionContinue 继续当前话题（评分 4-8）
	ActionContinue NextAction = "continue"
	// ActionDeepen 深入追问（评分 >= 8）
	ActionDeepen NextAction = "deepen"
	// ActionSwitch 切换到新话题（当前话题已充分覆盖）
	ActionSwitch NextAction = "switch"
	// ActionLower 降低难度（评分 < 4）
	ActionLower NextAction = "lower"
)

// DimensionScores 各维度评分
type DimensionScores struct {
	Correctness  float64 `json:"correctness"`  // 正确性 1-10，权重 40%
	Depth        float64 `json:"depth"`        // 深度 1-10，权重 25%
	Completeness float64 `json:"completeness"` // 完整性 1-10，权重 20%
	Practicality float64 `json:"practicality"` // 实践性 1-10，权重 15%
}

// CalculateOverall 计算加权总分
func (s *DimensionScores) CalculateOverall() float64 {
	// 使用权重：正确性40%，深度25%，完整性20%，实践性15%
	weighted := s.Correctness*0.40 +
		s.Depth*0.25 +
		s.Completeness*0.20 +
		s.Practicality*0.15

	return weighted
}

// EvaluationResult 评分结果
type EvaluationResult struct {
	Scores        DimensionScores `json:"scores"`         // 各维度评分
	Overall       float64         `json:"overall"`        // 加权总分 1-10
	CoveredTopics []string        `json:"covered_topics"` // 本次回答覆盖的知识点
	NextAction    NextAction      `json:"next_action"`    // 建议的下一步动作
	Reason        string          `json:"reason"`         // 评分理由（简短）
}

// EvaluationRequest 评分请求
type EvaluationRequest struct {
	Domain       string // 面试领域: Go/Java/MySQL/Redis/MQ
	CurrentTopic string // 当前话题
	Question     string // 当前问题
	Answer       string // 候选人回答
}

// EvaluatorConfig 评分器配置
type EvaluatorConfig struct {
	// 分数阈值
	DeepenThreshold int // 深入追问阈值，默认 8
	LowerThreshold  int // 降低难度阈值，默认 4

	// 话题切换条件
	MaxQuestionsPerTopic int // 单话题最大问题数，默认 5

	// 连续判断
	ConsecutiveCount int // 连续 N 题满足条件才触发，默认 2
}

// DefaultEvaluatorConfig 返回默认配置
func DefaultEvaluatorConfig() *EvaluatorConfig {
	return &EvaluatorConfig{
		DeepenThreshold:      8,
		LowerThreshold:       4,
		MaxQuestionsPerTopic: 5,
		ConsecutiveCount:     2,
	}
}

// ScoreHistory 评分历史（用于连续判断）
type ScoreHistory struct {
	Scores []float64 // 历史评分列表
	Topics []string  // 对应的话题
}

// AddScore 添加评分
func (h *ScoreHistory) AddScore(score float64, topic string) {
	h.Scores = append(h.Scores, score)
	h.Topics = append(h.Topics, topic)
}

// GetRecentScores 获取最近 N 个评分
func (h *ScoreHistory) GetRecentScores(n int) []float64 {
	if len(h.Scores) < n {
		return h.Scores
	}
	return h.Scores[len(h.Scores)-n:]
}

// CheckConsecutiveHigh 检查是否连续高分
func (h *ScoreHistory) CheckConsecutiveHigh(n int, threshold float64) bool {
	recent := h.GetRecentScores(n)
	if len(recent) < n {
		return false
	}
	for _, s := range recent {
		if s < threshold {
			return false
		}
	}
	return true
}

// CheckConsecutiveLow 检查是否连续低分
func (h *ScoreHistory) CheckConsecutiveLow(n int, threshold float64) bool {
	recent := h.GetRecentScores(n)
	if len(recent) < n {
		return false
	}
	for _, s := range recent {
		if s >= threshold {
			return false
		}
	}
	return true
}
