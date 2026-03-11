package model

import (
	"time"

	"gorm.io/gorm"
)

var PredictionDao _Prediction

// PredictionRecord 押题记录
type PredictionRecord struct {
	ID         uint64               `json:"id" gorm:"primaryKey;autoIncrement;comment:押题记录ID"`
	UserID     uint                 `json:"user_id" gorm:"index;not null;comment:用户ID"`
	ResumeID   uint64               `json:"resume_id" gorm:"index;not null;comment:简历ID"`
	Type       string               `json:"type" gorm:"size:20;comment:押题类型(校招/社招)"`
	Language   string               `json:"language" gorm:"size:20;comment:语言类型(java/go)"`
	JobTitle   string               `json:"job_title" gorm:"size:50;comment:岗位名称(前端/后端)"`
	Difficulty string               `json:"difficulty" gorm:"size:20;comment:难度等级(入门/进阶)"`
	Company    string               `json:"company" gorm:"size:100;comment:公司名称(字节/阿里等)"`
	Questions  []PredictionQuestion `json:"questions" gorm:"foreignKey:RecordID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	CreatedAt  time.Time            `json:"created_at" gorm:"autoCreateTime;comment:创建时间"`
}

// TableName 指定表名
func (r *PredictionRecord) TableName() string {
	return "prediction_record"
}

// PredictionQuestion 押题题目
type PredictionQuestion struct {
	ID              uint64    `json:"id" gorm:"primaryKey;autoIncrement;comment:题目ID"`
	RecordID        uint64    `json:"record_id" gorm:"index;not null;comment:押题记录ID"`
	Question        string    `json:"question" gorm:"type:text;not null;comment:问题"`
	Content         string    `json:"content" gorm:"type:text;comment:重点考察内容"`
	Focus           string    `json:"focus" gorm:"type:text;comment:重点考察"`
	ThinkingPath    string    `json:"thinking_path" gorm:"type:text;comment:回答思路"`
	ReferenceAnswer string    `json:"reference_answer" gorm:"type:text;comment:参考答案"`
	FollowUp        string    `json:"follow_up" gorm:"type:text;comment:可能追问(JSON或文本)"`
	Sort            int       `json:"sort" gorm:"comment:题目排序"`
	CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime;comment:创建时间"`
}

// TableName 指定表名
func (q *PredictionQuestion) TableName() string {
	return "prediction_question"
}

type _Prediction struct{}

// CreatePredictionRecord 创建押题记录
func (d *_Prediction) CreatePredictionRecord(record *PredictionRecord) error {
	if getDB == nil {
		panic("getDB function not initialized")
	}
	return getDB().Create(record).Error
}

// CreatePredictionQuestions 批量创建押题题目
func (d *_Prediction) CreatePredictionQuestions(questions []PredictionQuestion) error {
	if getDB == nil {
		panic("getDB function not initialized")
	}
	if len(questions) == 0 {
		return nil
	}
	return getDB().Create(&questions).Error
}

// GetPredictionRecordByID 根据ID查询押题记录
func (d *_Prediction) GetPredictionRecordByID(id uint64) (*PredictionRecord, error) {
	if getDB == nil {
		panic("getDB function not initialized")
	}
	var record PredictionRecord
	// 预加载 Questions 并按 Sort 排序
	err := getDB().Preload("Questions", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort ASC")
	}).First(&record, id).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// GetPredictionRecordsByUserID 查询用户的押题记录
func (d *_Prediction) GetPredictionRecordsByUserID(userID uint, page, pageSize int) ([]*PredictionRecord, int64, error) {
	if getDB == nil {
		panic("getDB function not initialized")
	}
	var records []*PredictionRecord
	var total int64

	db := getDB().Model(&PredictionRecord{}).Where("user_id = ?", userID)
	db.Count(&total)

	err := db.Order("created_at asc").
		Offset((page-1)*pageSize).
		Limit(pageSize).
		Preload("Questions", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort ASC")
		}).
		Find(&records).Error

	if err != nil {
		return nil, 0, err
	}
	return records, total, nil
}
