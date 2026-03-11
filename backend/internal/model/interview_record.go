package model

import (
	"time"
)

var InterviewRecordDao _InterviewRecord

// InterviewRecord 面试记录模型
type (
	_InterviewRecord struct {
	}
	InterviewRecord struct {
		ID           uint64    `json:"id" gorm:"primaryKey;autoIncrement;comment:面试主表"`
		UserID       uint      `json:"user_id" gorm:"index;not null;comment:用户ID"`
		Type         string    `json:"type" gorm:"size:255;not null;comment:面试类型(综合面试、专项面试)"`
		Difficulty   string    `json:"difficulty" gorm:"size:128;not null;comment:难度级别（简单、中等、困难）"`
		Domain       string    `json:"domain" gorm:"size:255;not null;comment:面试领域(校招、社招；java、golang)"`
		CompanyName  string    `json:"company_name" gorm:"size:128;comment:公司名称"`
		PositionName string    `json:"position_name" gorm:"size:128;comment:岗位名称"`
		Status       string    `json:"status" gorm:"size:50;not null;default:'pending';comment:面试状态（pending/completed）"`
		Duration     int64     `json:"duration" gorm:"comment:面试耗时（秒）"`
		CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime:milli"`
		UpdatedAt    time.Time `json:"updated_at" gorm:"autoUpdateTime:milli"`
	}
)

// TableName 指定表名
func (i *InterviewRecord) TableName() string {
	return "interview_record"
}

// CreateInterviewRecord 创建面试记录，返回记录ID
func (i *_InterviewRecord) CreateInterviewRecord(record *InterviewRecord) (uint64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	if err := getDB().Create(record).Error; err != nil {
		return 0, err
	}
	return record.ID, nil
}

// GetInterviewRecordByID 根据ID查询面试记录
func (i *_InterviewRecord) GetInterviewRecordByID(id uint64) (*InterviewRecord, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var record InterviewRecord
	err := getDB().Where("id = ?", id).First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// ListInterviewRecords 查询用户的面试记录列表
func (i *_InterviewRecord) ListInterviewRecords(userID uint, page, pageSize *int32) ([]*InterviewRecord, int64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var records []*InterviewRecord
	var total int64

	query := getDB().Where("user_id = ?", userID)

	// 获取总数
	if err := query.Model(&InterviewRecord{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	if err := query.Offset(int((*page - 1) * *pageSize)).
		Limit(int(*pageSize)).
		Order("created_at DESC").
		Find(&records).Error; err != nil {
		return nil, 0, err
	}

	return records, total, nil
}

// UpdateInterviewRecord 更新面试记录（只更新非零值字段）
func (i *_InterviewRecord) UpdateInterviewRecord(record *InterviewRecord) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	// 使用 map 方式更新，只更新非零值字段，避免覆盖原有数据
	updates := make(map[string]interface{})

	if record.Type != "" {
		updates["type"] = record.Type
	}
	if record.Difficulty != "" {
		updates["difficulty"] = record.Difficulty
	}
	if record.Domain != "" {
		updates["domain"] = record.Domain
	}
	if record.CompanyName != "" {
		updates["company_name"] = record.CompanyName
	}
	if record.PositionName != "" {
		updates["position_name"] = record.PositionName
	}
	if record.Status != "" {
		updates["status"] = record.Status
	}
	if record.Duration != 0 {
		updates["duration"] = record.Duration
	}

	// 如果没有需要更新的字段，直接返回
	if len(updates) == 0 {
		return nil
	}

	return getDB().Model(&InterviewRecord{}).Where("id = ?", record.ID).Updates(updates).Error
}

// DeleteInterviewRecord 删除面试记录
func (i *_InterviewRecord) DeleteInterviewRecord(id uint64) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().Where("id = ?", id).Delete(&InterviewRecord{}).Error
}
