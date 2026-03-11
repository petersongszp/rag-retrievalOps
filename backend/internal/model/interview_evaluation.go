package model

import (
	"time"
)

var InterviewEvaluationDao _InterviewEvaluation

// InterviewEvaluation 面试评估信息（单表存储）
type (
	_InterviewEvaluation struct{}
	InterviewEvaluation  struct {
		ID         uint64                 `json:"id" gorm:"primaryKey;autoIncrement;comment:主键"`
		UserID     uint                   `json:"user_id" gorm:"not null;index:idx_user_id;comment:用户ID"`
		ReportID   uint64                 `json:"report_id" gorm:"not null;index:idx_report_id;comment:关联的面试报告ID"`
		Comment    string                 `json:"comment" gorm:"type:text;comment:总体评价"`
		Score      float64                `json:"score" gorm:"type:decimal(5,2);comment:总体评分"`
		Dimensions []*EvaluationDimension `json:"dimensions" gorm:"type:json;serializer:json;comment:各维度评估"`
		Deleted    int                    `json:"deleted" gorm:"default:0;index:idx_deleted;comment:是否删除"`
		CreatedAt  time.Time              `json:"created_at" gorm:"autoCreateTime:milli;comment:创建时间"`
		UpdatedAt  time.Time              `json:"updated_at" gorm:"autoUpdateTime:milli;comment:更新时间"`
	}
)

// EvaluationDimension 评估维度信息
type EvaluationDimension struct {
	DimensionName string  `json:"dimension_name" gorm:"type:varchar(100);comment:维度名称"`
	Evaluation    string  `json:"evaluation" gorm:"type:text;comment:维度评价"`
	Score         float64 `json:"score" gorm:"type:decimal(5,2);comment:维度评分"`
}

// TableName 指定表名
func (InterviewEvaluation) TableName() string {
	return "interview_evaluation"
}

// CreateEvaluation 创建评估记录
func (dao *_InterviewEvaluation) CreateEvaluation(evaluation *InterviewEvaluation) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().Create(evaluation).Error
}

// GetEvaluationByID 根据ID获取评估
func (dao *_InterviewEvaluation) GetEvaluationByID(id uint64) (*InterviewEvaluation, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var evaluation *InterviewEvaluation
	err := getDB().Where("id = ? AND deleted = ?", id, false).First(&evaluation).Error
	return evaluation, err
}

// GetEvaluationByReportID 根据报告ID获取评估
func (dao *_InterviewEvaluation) GetEvaluationByReportID(reportID uint64) (*InterviewEvaluation, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var evaluation *InterviewEvaluation
	err := getDB().Where("report_id = ? AND deleted = ?", reportID, false).First(&evaluation).Error
	return evaluation, err
}

// GetEvaluationByUserIDAndReportID 根据用户ID和报告ID获取评估
func (dao *_InterviewEvaluation) GetEvaluationByUserIDAndReportID(userID uint, reportID uint64) (*InterviewEvaluation, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var evaluation *InterviewEvaluation
	err := getDB().Where("user_id = ? AND report_id = ? AND deleted = ?", userID, reportID, false).First(&evaluation).Error
	return evaluation, err
}

// UpdateEvaluation 更新评估
func (dao *_InterviewEvaluation) UpdateEvaluation(evaluation *InterviewEvaluation) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().Model(evaluation).Updates(evaluation).Error
}

// DeleteEvaluation 软删除评估
func (dao *_InterviewEvaluation) DeleteEvaluation(id uint64) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().Model(&InterviewEvaluation{}).Where("id = ?", id).Update("deleted", true).Error
}

// CountByReportID 统计报告下的评估数量
func (dao *_InterviewEvaluation) CountByReportID(reportID uint64) (int64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var count int64
	err := getDB().Model(&InterviewEvaluation{}).Where("report_id = ? AND deleted = ?", reportID, false).Count(&count).Error
	return count, err
}
