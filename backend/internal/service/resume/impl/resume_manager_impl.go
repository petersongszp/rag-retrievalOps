package impl

import (
	"context"
	interviewsapi "interview-agents/api/model/resume"
	"interview-agents/internal/model"
	"log"
)

// ResumeServer 简历管理服务实现
type ResumeServer struct{}

// NewResumeServer 创建简历管理服务实例
func NewResumeServer() *ResumeServer {
	return &ResumeServer{}
}

// toResumeInfo 将 map 转换为 ResumeInfo
func toResumeInfo(dataMap map[string]interface{}) *interviewsapi.ResumeInfo {
	resumeInfo := &interviewsapi.ResumeInfo{}

	// 提取 ID
	if id, ok := dataMap["id"]; ok {
		if idVal, ok := id.(uint64); ok {
			resumeInfo.ID = int64(idVal)
		} else if idVal, ok := id.(int64); ok {
			resumeInfo.ID = idVal
		}
	}

	// 提取 UserID
	if userID, ok := dataMap["user_id"]; ok {
		if userIDVal, ok := userID.(uint); ok {
			resumeInfo.UserID = int32(userIDVal)
		} else if userIDVal, ok := userID.(int); ok {
			resumeInfo.UserID = int32(userIDVal)
		}
	}

	// 提取 FileName
	if fileName, ok := dataMap["file_name"].(string); ok {
		resumeInfo.FileName = fileName
	}

	// 提取 FileSize
	if fileSize, ok := dataMap["file_size"].(int64); ok {
		resumeInfo.FileSize = fileSize
	}

	// 提取 FileType
	if fileType, ok := dataMap["file_type"].(string); ok {
		resumeInfo.FileType = fileType
	}

	// 提取 IsDefault
	if isDefault, ok := dataMap["is_default"]; ok {
		if isDefaultVal, ok := isDefault.(int); ok {
			resumeInfo.IsDefault = int32(isDefaultVal)
		} else if isDefaultVal, ok := isDefault.(int32); ok {
			resumeInfo.IsDefault = isDefaultVal
		}
	}

	// 提取 CreatedAt
	if createdAt, ok := dataMap["created_at"]; ok {
		if createdAtVal, ok := createdAt.(int64); ok {
			resumeInfo.CreatedAt = createdAtVal
		}
	}

	// 提取 UpdatedAt
	if updatedAt, ok := dataMap["updated_at"]; ok {
		if updatedAtVal, ok := updatedAt.(int64); ok {
			resumeInfo.UpdatedAt = updatedAtVal
		}
	}

	return resumeInfo
}

// UploadResume 上传简历，返回简历ID
func (s *ResumeServer) UploadResume(
	ctx context.Context,
	userID uint,
	fileName string,
	fileType string,
	fileSize int64,
	content string,
) (uint64, error) {
	resume := &model.Resume{
		UserID:    userID,
		Content:   content,
		FileName:  fileName,
		FileSize:  fileSize,
		FileType:  fileType,
		IsDefault: 0,
		Deleted:   0,
	}

	resumeID, err := model.ResumeDao.CreateResume(resume)
	if err != nil {
		log.Printf("[UploadResume] 创建简历失败: %v", err)
		return 0, err
	}

	log.Printf("[UploadResume] 简历上传成功: userID=%d, resumeID=%d, fileName=%s", userID, resumeID, fileName)
	return resumeID, nil
}

// SetDefaultResume 设置用户的默认简历
func (s *ResumeServer) SetDefaultResume(
	ctx context.Context,
	userID uint,
	resumeID uint64,
) error {
	err := model.ResumeDao.SetDefaultResume(userID, resumeID)
	if err != nil {
		log.Printf("[SetDefaultResume] 设置默认简历失败: %v", err)
		return err
	}

	log.Printf("[SetDefaultResume] 默认简历设置成功: userID=%d, resumeID=%d", userID, resumeID)
	return nil
}

// UpdateResume 更新简历信息
func (s *ResumeServer) UpdateResume(
	ctx context.Context,
	resumeID uint64,
	fileName string,
	content string,
) error {
	updates := make(map[string]interface{})
	if fileName != "" {
		updates["file_name"] = fileName
	}
	if content != "" {
		updates["content"] = content
	}

	if len(updates) == 0 {
		return nil
	}

	err := model.ResumeDao.UpdateResume(resumeID, updates)
	if err != nil {
		log.Printf("[UpdateResume] 更新简历失败: %v", err)
		return err
	}

	log.Printf("[UpdateResume] 简历更新成功: resumeID=%d", resumeID)
	return nil
}

// DeleteResume 删除简历
func (s *ResumeServer) DeleteResume(
	ctx context.Context,
	userID uint,
	resumeID uint64,
) error {
	// 验证简历属于该用户
	resume, err := model.ResumeDao.GetResumeByID(resumeID)
	if err != nil {
		log.Printf("[DeleteResume] 获取简历失败: %v", err)
		return err
	}

	if resume.UserID != userID {
		log.Printf("[DeleteResume] 用户无权删除该简历: userID=%d, resumeID=%d", userID, resumeID)
		return err
	}

	err = model.ResumeDao.DeleteResume(resumeID)
	if err != nil {
		log.Printf("[DeleteResume] 删除简历失败: %v", err)
		return err
	}

	log.Printf("[DeleteResume] 简历删除成功: userID=%d, resumeID=%d", userID, resumeID)
	return nil
}

// GetResumeInfoByID 根据简历ID获取简历详情，返回强类型 ResumeInfo
func (s *ResumeServer) GetResumeInfoByID(
	ctx context.Context,
	resumeID uint64,
) (interface{}, error) {
	resume, err := model.ResumeDao.GetResumeByID(resumeID)
	if err != nil {
		log.Printf("[GetResumeInfoByID] 获取简历失败: %v", err)
		return nil, err
	}

	// 直接返回 map，包含 status 字段
	return map[string]interface{}{
		"id":         resume.ID,
		"user_id":    resume.UserID,
		"file_name":  resume.FileName,
		"file_size":  resume.FileSize,
		"file_type":  resume.FileType,
		"is_default": resume.IsDefault,
		"status":     resume.Status,
		"error_msg":  resume.ErrorMsg,
		"created_at": resume.CreatedAt.Unix(),
		"updated_at": resume.UpdatedAt.Unix(),
	}, nil
}

// GetDefaultResumeInfo 获取用户的默认简历，返回强类型 ResumeInfo
func (s *ResumeServer) GetDefaultResumeInfo(
	ctx context.Context,
	userID uint,
) (interface{}, error) {
	resume, err := model.ResumeDao.GetDefaultResumeByUserID(userID)
	if err != nil {
		log.Printf("[GetDefaultResumeInfo] 获取默认简历失败: %v", err)
		return nil, err
	}

	// 直接返回 map，包含 status 字段
	return map[string]interface{}{
		"id":         resume.ID,
		"user_id":    resume.UserID,
		"file_name":  resume.FileName,
		"file_size":  resume.FileSize,
		"file_type":  resume.FileType,
		"is_default": resume.IsDefault,
		"status":     resume.Status,
		"error_msg":  resume.ErrorMsg,
		"created_at": resume.CreatedAt.Unix(),
		"updated_at": resume.UpdatedAt.Unix(),
	}, nil
}

// ListResumeInfosByUserID 分页获取用户的简历列表，返回强类型 ResumeInfo 列表
func (s *ResumeServer) ListResumeInfosByUserID(
	ctx context.Context,
	userID uint,
	page, pageSize int32,
) (interface{}, int64, error) {
	resumes, total, err := model.ResumeDao.ListResumesByUserID(userID, page, pageSize)
	if err != nil {
		log.Printf("[ListResumeInfosByUserID] 分页获取简历列表失败: %v", err)
		return nil, 0, err
	}

	// 直接返回 map 列表，包含 status 字段
	var resumeInfoList []map[string]interface{}
	for _, resume := range resumes {
		resumeInfoList = append(resumeInfoList, map[string]interface{}{
			"id":         resume.ID,
			"user_id":    resume.UserID,
			"file_name":  resume.FileName,
			"file_size":  resume.FileSize,
			"file_type":  resume.FileType,
			"is_default": resume.IsDefault,
			"status":     resume.Status,
			"error_msg":  resume.ErrorMsg,
			"created_at": resume.CreatedAt.Unix(),
			"updated_at": resume.UpdatedAt.Unix(),
		})
	}

	return resumeInfoList, total, nil
}
