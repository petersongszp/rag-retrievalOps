package topic

import (
	"sort"
)

// TopicTracker 话题覆盖度追踪器
type TopicTracker struct {
	Domain         string               // 面试领域
	CurrentTopic   string               // 当前主话题
	CoveredTopics  map[string]int       // topic -> 问过次数
	TopicScores    map[string][]float64 // topic -> 历史评分列表
	TopicQuestions map[string]int       // mainTopic -> 该话题下已问问题数
	TotalQuestions int                  // 总问题数
}

// NewTopicTracker 创建话题追踪器
func NewTopicTracker(domain string) *TopicTracker {
	return &TopicTracker{
		Domain:         domain,
		CurrentTopic:   "",
		CoveredTopics:  make(map[string]int),
		TopicScores:    make(map[string][]float64),
		TopicQuestions: make(map[string]int),
		TotalQuestions: 0,
	}
}

// UpdateCoverage 更新话题覆盖度
// topics: 本次回答覆盖的知识点列表
// score: 本次评分
// currentMainTopic: 当前主话题
func (t *TopicTracker) UpdateCoverage(topics []string, score float64, currentMainTopic string) {
	// 更新当前话题
	if currentMainTopic != "" {
		t.CurrentTopic = currentMainTopic
		t.TopicQuestions[currentMainTopic]++
	}

	// 更新覆盖的知识点
	for _, topic := range topics {
		t.CoveredTopics[topic]++
		t.TopicScores[topic] = append(t.TopicScores[topic], score)
	}

	t.TotalQuestions++
}

// GetTopicCoverage 获取某个话题的覆盖信息
func (t *TopicTracker) GetTopicCoverage(topic string) (count int, avgScore float64) {
	count = t.CoveredTopics[topic]
	scores := t.TopicScores[topic]
	if len(scores) == 0 {
		return count, 0
	}

	sum := float64(0)
	for _, s := range scores {
		sum += s
	}
	avgScore = sum / float64(len(scores))
	return count, avgScore
}

// ShouldSwitchTopic 判断是否应该切换话题
// maxQuestionsPerTopic: 单话题最大问题数
func (t *TopicTracker) ShouldSwitchTopic(maxQuestionsPerTopic int) bool {
	if t.CurrentTopic == "" {
		return false
	}
	return t.TopicQuestions[t.CurrentTopic] >= maxQuestionsPerTopic
}

// SuggestNextTopic 建议下一个话题
// 优先选择覆盖度低的话题
func (t *TopicTracker) SuggestNextTopic() string {
	tree := GetTopicTree(t.Domain)
	if tree == nil {
		return ""
	}

	// 统计每个主话题的覆盖情况
	type topicStats struct {
		name     string
		count    int     // 问过的次数
		avgScore float64 // 平均分
	}

	var stats []topicStats
	for mainTopic := range tree {
		count := t.TopicQuestions[mainTopic]

		// 计算该主话题下所有子话题的平均分
		var totalScore float64
		var scoreCount int
		for _, subTopic := range tree[mainTopic] {
			if scores, ok := t.TopicScores[subTopic]; ok {
				for _, s := range scores {
					totalScore += s
					scoreCount++
				}
			}
		}

		var avgScore float64
		if scoreCount > 0 {
			avgScore = float64(totalScore) / float64(scoreCount)
		}

		stats = append(stats, topicStats{
			name:     mainTopic,
			count:    count,
			avgScore: avgScore,
		})
	}

	// 排序：优先选择问得少的，其次选择平均分低的（需要加强的）
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].count != stats[j].count {
			return stats[i].count < stats[j].count
		}
		return stats[i].avgScore < stats[j].avgScore
	})

	// 返回第一个不是当前话题的
	for _, s := range stats {
		if s.name != t.CurrentTopic {
			return s.name
		}
	}

	// 如果所有话题都问过了，返回问得最少的
	if len(stats) > 0 {
		return stats[0].name
	}

	return ""
}

// GetCoverageReport 获取覆盖度报告
func (t *TopicTracker) GetCoverageReport() *CoverageReport {
	tree := GetTopicTree(t.Domain)
	if tree == nil {
		return nil
	}

	report := &CoverageReport{
		Domain:         t.Domain,
		TotalQuestions: t.TotalQuestions,
		MainTopics:     make([]MainTopicCoverage, 0),
	}

	// 统计每个主话题
	var totalCovered, totalTopics int
	for mainTopic, subTopics := range tree {
		totalTopics += len(subTopics)

		mtc := MainTopicCoverage{
			Name:      mainTopic,
			Questions: t.TopicQuestions[mainTopic],
			SubTopics: make([]SubTopicCoverage, 0),
		}

		var coveredCount, scoreCount int
		var totalScore float64
		for _, subTopic := range subTopics {
			stc := SubTopicCoverage{
				Name:    subTopic,
				Covered: t.CoveredTopics[subTopic] > 0,
				Count:   t.CoveredTopics[subTopic],
			}

			if scores, ok := t.TopicScores[subTopic]; ok && len(scores) > 0 {
				sum := float64(0)
				for _, s := range scores {
					sum += s
					totalScore += s
					scoreCount++
				}
				stc.AvgScore = sum / float64(len(scores))
			}

			if stc.Covered {
				coveredCount++
				totalCovered++
			}

			mtc.SubTopics = append(mtc.SubTopics, stc)
		}

		mtc.CoveredCount = coveredCount
		mtc.TotalCount = len(subTopics)
		if scoreCount > 0 {
			mtc.AvgScore = float64(totalScore) / float64(scoreCount)
		}

		report.MainTopics = append(report.MainTopics, mtc)
	}

	// 计算总覆盖率
	if totalTopics > 0 {
		report.CoverageRate = float64(totalCovered) / float64(totalTopics) * 100
	}

	return report
}

// CoverageReport 覆盖度报告
type CoverageReport struct {
	Domain         string              // 面试领域
	TotalQuestions int                 // 总问题数
	CoverageRate   float64             // 覆盖率 (%)
	MainTopics     []MainTopicCoverage // 各主话题覆盖情况
}

// MainTopicCoverage 主话题覆盖情况
type MainTopicCoverage struct {
	Name         string             // 主话题名称
	Questions    int                // 该话题下问题数
	CoveredCount int                // 已覆盖子话题数
	TotalCount   int                // 子话题总数
	AvgScore     float64            // 平均分
	SubTopics    []SubTopicCoverage // 子话题详情
}

// SubTopicCoverage 子话题覆盖情况
type SubTopicCoverage struct {
	Name     string  // 子话题名称
	Covered  bool    // 是否已覆盖
	Count    int     // 问过次数
	AvgScore float64 // 平均分
}
