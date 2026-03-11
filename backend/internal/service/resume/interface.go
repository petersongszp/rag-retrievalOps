package resume

import (
	interviewsapi "interview-agents/api/model/resume"
	"interview-agents/internal/model"
	"interview-agents/internal/service/resume/impl"
	"context"
)

// NewInterviewService  返回面试服务的实现
func NewInterviewService() InterviewManager {
	return impl.NewInterviewServiceImpl()
}

// NewResumeService 返回简历服务的实现
func NewResumeService() ResumeManager {
	return impl.NewResumeServer()
}

// InterviewManager 面试管理接口
type InterviewManager interface {
	// CreateInterviewRecord 创建面试记录，返回记录ID
	CreateInterviewRecord(
		ctx context.Context,
		record *interviewsapi.InterviewRecordDTO,
	) (uint64, error)

	// UpdateInterviewRecord 更新面试记录
	UpdateInterviewRecord(
		ctx context.Context,
		record *interviewsapi.InterviewRecordDTO,
	) error

	// ListInterviewRecords 获取面试记录列表
	ListInterviewRecords(
		ctx context.Context,
		userID uint,
		page, pageSize *int32,
	) ([]*interviewsapi.InterviewRecordDTO, int64, error)

	// GetInterviewEvaluation 根据用户ID和报告ID获取面试评估报告
	GetInterviewEvaluation(
		ctx context.Context,
		userID uint,
		reportID uint64,
	) (interface{}, error)

	// GetAnswerReport 根据用户ID和报告ID获取答题报告
	GetAnswerReport(
		ctx context.Context,
		userID uint,
		reportID uint64,
	) (interface{}, error)

	// SaveInterviewDialogueWithParent 保存面试对话（支持父子关系）
	SaveInterviewDialogueWithParent(
		ctx context.Context,
		userID uint,
		reportID uint64,
		mainQuestion *model.InterviewDialogue,
		followUpQuestions []*model.InterviewDialogue,
	) error
}

// ResumeManager 简历管理接口
type ResumeManager interface {
	// UploadResume 上传简历，返回简历ID
	UploadResume(
		ctx context.Context,
		userID uint,
		fileName string,
		fileType string,
		fileSize int64,
		content string,
	) (uint64, error)

	// SetDefaultResume 设置用户的默认简历
	SetDefaultResume(
		ctx context.Context,
		userID uint,
		resumeID uint64,
	) error

	// UpdateResume 更新简历信息
	UpdateResume(
		ctx context.Context,
		resumeID uint64,
		fileName string,
		content string,
	) error

	// DeleteResume 删除简历
	DeleteResume(
		ctx context.Context,
		userID uint,
		resumeID uint64,
	) error

	// GetResumeInfoByID 根据简历ID获取简历详情，返回强类型 ResumeInfo
	GetResumeInfoByID(
		ctx context.Context,
		resumeID uint64,
	) (interface{}, error)

	// GetDefaultResumeInfo 获取用户的默认简历，返回强类型 ResumeInfo
	GetDefaultResumeInfo(
		ctx context.Context,
		userID uint,
	) (interface{}, error)

	// ListResumeInfosByUserID 分页获取用户的简历列表，返回强类型 ResumeInfo 列表
	ListResumeInfosByUserID(
		ctx context.Context,
		userID uint,
		page, pageSize int32,
	) (interface{}, int64, error)
}
