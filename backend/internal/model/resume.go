package model

import (
	"fmt"
	"time"
)

var ResumeDao _Resume

// Resume 简历模型
type (
	_Resume struct {
	}
	Resume struct {
		ID        uint64    `json:"id" gorm:"primaryKey;autoIncrement;comment:简历ID"`
		UserID    uint      `json:"user_id" gorm:"index;not null;comment:用户ID"`
		Content   string    `json:"content" gorm:"type:longtext;comment:简历内容(解析后的JSON)"`
		Status    string    `json:"status" gorm:"size:20;default:'pending';comment:处理状态(pending/processing/completed/failed)"`
		ErrorMsg  string    `json:"error_msg" gorm:"size:500;comment:错误信息"`
		FilePath  string    `json:"file_path" gorm:"size:500;comment:文件存储路径"`
		FileName  string    `json:"file_name" gorm:"size:255;comment:原始文件名"`
		FileSize  int64     `json:"file_size" gorm:"comment:文件大小（字节）"`
		FileType  string    `json:"file_type" gorm:"size:50;comment:文件类型(pdf/doc/txt等)"`
		IsDefault int       `json:"is_default" gorm:"default:0;comment:是否为默认简历(0-否，1-是)"`
		Deleted   int       `json:"deleted" gorm:"default:0;index;comment:删除标记(0-未删除，1-已删除)"`
		CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime:milli;comment:创建时间"`
		UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime:milli;comment:更新时间"`
	}
)

// TableName 指定表名
func (r *Resume) TableName() string {
	return "resume"
}

// CreateResume 创建简历记录，返回简历ID
func (r *_Resume) CreateResume(resume *Resume) (uint64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	if err := getDB().Create(resume).Error; err != nil {
		return 0, err
	}
	return resume.ID, nil
}

// GetResumeByID 根据ID查询简历
func (r *_Resume) GetResumeByID(id uint64) (*Resume, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var resume Resume
	err := getDB().Where("id = ? AND deleted = 0", id).First(&resume).Error
	if err != nil {
		return nil, err
	}
	return &resume, nil
}

// GetResumeByUserID 根据用户ID查询用户的简历列表
func (r *_Resume) GetResumeByUserID(userID uint) ([]*Resume, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var resumes []*Resume
	err := getDB().
		Where("user_id = ? AND deleted = 0", userID).
		Order("is_default DESC, created_at DESC").
		Find(&resumes).Error
	if err != nil {
		return nil, err
	}
	return resumes, nil
}

// GetDefaultResumeByUserID 根据用户ID查询默认简历
func (r *_Resume) GetDefaultResumeByUserID(userID uint) (*Resume, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var resume Resume
	err := getDB().
		Where("user_id = ? AND deleted = 0 AND is_default = 1", userID).
		First(&resume).Error
	if err != nil {
		return nil, err
	}
	return &resume, nil
}

// UpdateResume 更新简历
func (r *_Resume) UpdateResume(id uint64, updates map[string]interface{}) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	if len(updates) == 0 {
		return nil
	}
	return getDB().
		Model(&Resume{}).
		Where("id = ? AND deleted = 0", id).
		Updates(updates).Error
}

// DeleteResume 软删除简历
func (r *_Resume) DeleteResume(id uint64) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().
		Model(&Resume{}).
		Where("id = ?", id).
		Update("deleted", 1).Error
}

// SetDefaultResume 设置用户的默认简历
func (r *_Resume) SetDefaultResume(userID uint, resumeID uint64) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	// 先将该用户的所有简历设置为非默认
	if err := getDB().
		Model(&Resume{}).
		Where("user_id = ? AND deleted = 0", userID).
		Update("is_default", 0).Error; err != nil {
		return err
	}
	// 再将指定简历设置为默认
	return getDB().
		Model(&Resume{}).
		Where("id = ? AND user_id = ? AND deleted = 0", resumeID, userID).
		Update("is_default", 1).Error
}

// ListResumesByUserID 分页查询用户的简历列表
func (r *_Resume) ListResumesByUserID(userID uint, page, pageSize int32) ([]*Resume, int64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var resumes []*Resume
	var total int64

	query := getDB().Where("user_id = ? AND deleted = 0", userID)

	// 获取总数
	if err := query.Model(&Resume{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	if err := query.
		Offset(int((page - 1) * pageSize)).
		Limit(int(pageSize)).
		Order("is_default DESC, created_at DESC").
		Find(&resumes).Error; err != nil {
		return nil, 0, err
	}

	return resumes, total, nil
}

// CountResumesByUserID 统计用户的简历数量
func (r *_Resume) CountResumesByUserID(userID uint) (int64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var count int64
	err := getDB().
		Model(&Resume{}).
		Where("user_id = ? AND deleted = 0", userID).
		Count(&count).Error
	return count, err
}

// UpdateResumeStatus 更新简历处理状态
func (r *_Resume) UpdateResumeStatus(id uint64, status string, errorMsg string) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	updates := map[string]interface{}{
		"status": status,
	}
	if errorMsg != "" {
		updates["error_msg"] = errorMsg
	}
	tx := getDB().
		Model(&Resume{}).
		Where("id = ?", id).
		Updates(updates)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return fmt.Errorf("resume not found or not updated: id=%d", id)
	}
	return nil
}

// UpdateResumeContent 更新简历解析内容（解析成功时调用）
func (r *_Resume) UpdateResumeContent(id uint64, content string) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	tx := getDB().
		Model(&Resume{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"content": content,
			"status":  "completed",
		})
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return fmt.Errorf("resume not found or content not updated: id=%d", id)
	}
	return nil
}
