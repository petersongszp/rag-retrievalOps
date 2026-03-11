package model

import (
	"time"
)

var AnswerReportDao _AnswerReport

// AnswerReport 答题报告（单表存储）
type (
	_AnswerReport struct{}
	AnswerReport  struct {
		ID        uint64              `json:"id" gorm:"primaryKey;autoIncrement;comment:主键"`
		UserID    uint                `json:"user_id" gorm:"not null;index:idx_user_id;comment:用户ID"`
		ReportID  uint64              `json:"report_id" gorm:"not null;index:idx_report_id;comment:关联的面试报告ID"`
		Records   []*AnswerRecordItem `json:"records" gorm:"type:json;serializer:json;comment:答题记录列表"`
		Deleted   int                 `json:"deleted" gorm:"default:0;index:idx_deleted;comment:是否删除"`
		CreatedAt time.Time           `json:"created_at" gorm:"autoCreateTime:milli;comment:创建时间"`
		UpdatedAt time.Time           `json:"updated_at" gorm:"autoUpdateTime:milli;comment:更新时间"`
	}
)

// AnswerRecordItem 单个答题记录
type AnswerRecordItem struct {
	Order   int32                `json:"order" gorm:"type:int;comment:问题顺序"`
	Content string               `json:"content" gorm:"type:text;comment:问题内容"`
	Comment *AnswerRecordComment `json:"comment" gorm:"type:json;serializer:json;comment:评论信息"`
	Message []*AnswerRecordMsg   `json:"message" gorm:"type:json;serializer:json;comment:对话列表"`
}

// AnswerRecordComment 答题记录中的评论信息
type AnswerRecordComment struct {
	Score      int32  `json:"score" gorm:"type:int;comment:评分"`
	KeyPoints  string `json:"key_points" gorm:"type:text;comment:关键点"`
	Difficulty string `json:"difficulty" gorm:"type:varchar(50);comment:难度等级"`
	Strengths  string `json:"strengths" gorm:"type:text;comment:优势"`
	Weaknesses string `json:"weaknesses" gorm:"type:text;comment:不足"`
	Suggestion string `json:"suggestion" gorm:"type:text;comment:建议"`
	KnowPoints string `json:"know_points" gorm:"type:text;comment:知识点"`
	Thinking   string `json:"thinking" gorm:"type:text;comment:思考过程"`
	Reference  string `json:"reference" gorm:"type:text;comment:参考答案"`
}

// AnswerRecordMsg 答题记录中的单条对话
type AnswerRecordMsg struct {
	Order    int32  `json:"order" gorm:"type:int;comment:对话顺序"`
	Question string `json:"question" gorm:"type:text;comment:提问内容"`
	Answer   string `json:"answer" gorm:"type:text;comment:回答内容"`
}

// TableName 指定表名
func (AnswerReport) TableName() string {
	return "answer_report"
}

// CreateAnswerReport 创建答题报告
func (dao *_AnswerReport) CreateAnswerReport(report *AnswerReport) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().Create(report).Error
}

// GetAnswerReportByID 根据ID获取答题报告
func (dao *_AnswerReport) GetAnswerReportByID(id uint64) (*AnswerReport, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var report *AnswerReport
	err := getDB().Where("id = ? AND deleted = ?", id, 0).First(&report).Error
	return report, err
}

// GetAnswerReportByReportID 根据报告ID获取答题报告
func (dao *_AnswerReport) GetAnswerReportByReportID(reportID uint64) (*AnswerReport, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var report *AnswerReport
	err := getDB().Where("report_id = ? AND deleted = ?", reportID, 0).First(&report).Error
	return report, err
}

// GetAnswerReportByUserIDAndReportID 根据用户ID和报告ID获取答题报告
func (dao *_AnswerReport) GetAnswerReportByUserIDAndReportID(userID uint, reportID uint64) (*AnswerReport, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var report *AnswerReport
	err := getDB().Where("user_id = ? AND report_id = ? AND deleted = ?", userID, reportID, 0).First(&report).Error
	return report, err
}

// DeleteAnswerReport 软删除答题报告
func (dao *_AnswerReport) DeleteAnswerReport(id uint64) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().Model(&AnswerReport{}).Where("id = ?", id).Update("deleted", 1).Error
}

// CountByReportID 统计报告下的答题报告数量
func (dao *_AnswerReport) CountByReportID(reportID uint64) (int64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var count int64
	err := getDB().Model(&AnswerReport{}).Where("report_id = ? AND deleted = ?", reportID, 0).Count(&count).Error
	return count, err
}

// ListByUserIDAndReportID 根据用户ID和报告ID获取所有答题报告
func (dao *_AnswerReport) ListByUserIDAndReportID(userID uint, reportID uint64) ([]*AnswerReport, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var reports []*AnswerReport
	err := getDB().Where("user_id = ? AND report_id = ? AND deleted = ?", userID, reportID, 0).Find(&reports).Error
	return reports, err
}
