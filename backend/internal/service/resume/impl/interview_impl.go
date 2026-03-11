package impl

import (
	interviewsapi "interview-agents/api/model/resume"
	"interview-agents/internal/model"
	"context"
	"fmt"
	"log"
)

// InterviewServiceImpl 面试服务实现
type InterviewServiceImpl struct{}

// NewInterviewServiceImpl 创建面试服务实例
func NewInterviewServiceImpl() *InterviewServiceImpl {
	return &InterviewServiceImpl{}
}

// CreateInterviewRecord 创建面试记录，返回记录ID
func (s *InterviewServiceImpl) CreateInterviewRecord(ctx context.Context, dto *interviewsapi.InterviewRecordDTO) (uint64, error) {
	// 处理指针类型字段，提取值或使用默认值
	companyName := ""
	if dto.CompanyName != nil {
		companyName = *dto.CompanyName
	}

	positionName := ""
	if dto.PositionName != nil {
		positionName = *dto.PositionName
	}

	// 初始化状态为pending（如果未提供）
	status := dto.Status
	if status == "" {
		status = "pending"
	}

	var duration int64 = 0
	if dto.Duration != nil {
		duration = *dto.Duration
	}

	record := &model.InterviewRecord{
		UserID:       uint(dto.UserID),
		Type:         dto.Type,
		Difficulty:   dto.Difficulty,
		Domain:       dto.Domain,
		CompanyName:  companyName,
		PositionName: positionName,
		Status:       status,
		Duration:     duration,
	}

	recordID, err := model.InterviewRecordDao.CreateInterviewRecord(record)
	if err != nil {
		log.Printf("[CreateInterviewRecord] 创建面试记录失败: %v", err)
		return 0, err
	}

	log.Printf("[CreateInterviewRecord] 面试记录创建成功，ID: %d，用户ID: %d", recordID, dto.UserID)
	return recordID, nil
}

// UpdateInterviewRecord 更新面试记录
func (s *InterviewServiceImpl) UpdateInterviewRecord(ctx context.Context, dto *interviewsapi.InterviewRecordDTO) error {
	// 处理指针类型字段，提取值或使用默认值
	companyName := ""
	if dto.CompanyName != nil {
		companyName = *dto.CompanyName
	}

	positionName := ""
	if dto.PositionName != nil {
		positionName = *dto.PositionName
	}

	var duration int64 = 0
	if dto.Duration != nil {
		duration = *dto.Duration
	}

	record := &model.InterviewRecord{
		ID:           uint64(dto.ID),
		UserID:       uint(dto.UserID),
		Type:         dto.Type,
		Difficulty:   dto.Difficulty,
		Domain:       dto.Domain,
		CompanyName:  companyName,
		PositionName: positionName,
		Status:       dto.Status,
		Duration:     duration,
	}

	err := model.InterviewRecordDao.UpdateInterviewRecord(record)
	if err != nil {
		log.Printf("[UpdateInterviewRecord] 更新面试记录失败: %v", err)
		return err
	}

	log.Printf("[UpdateInterviewRecord] 面试记录更新成功，ID: %d，用户ID: %d", dto.ID, dto.UserID)
	return nil
}

// ListInterviewRecords 获取面试记录列表
func (s *InterviewServiceImpl) ListInterviewRecords(ctx context.Context, userID uint, page, pageSize *int32) ([]*interviewsapi.InterviewRecordDTO, int64, error) {
	// 设置默认分页参数
	pageNum := int32(1)
	pageSz := int32(10)

	if page != nil && *page > 0 {
		pageNum = *page
	}
	if pageSize != nil && *pageSize > 0 {
		pageSz = *pageSize
	}

	// 调用 DAO 层获取数据
	records, total, err := model.InterviewRecordDao.ListInterviewRecords(userID, &pageNum, &pageSz)
	if err != nil {
		log.Printf("[ListInterviewRecords] 查询面试记录失败: %v", err)
		return nil, 0, err
	}

	// 转换为 DTO
	dtoList := make([]*interviewsapi.InterviewRecordDTO, 0, len(records))
	for _, record := range records {
		dto := convertToInterviewRecordDTO(record)
		dtoList = append(dtoList, dto)
	}

	log.Printf("[ListInterviewRecords] 查询成功，用户ID: %d，总数: %d，页码: %d，每页: %d", userID, total, pageNum, pageSz)
	return dtoList, total, nil
}

// convertToInterviewRecordDTO 将 model.InterviewRecord 转换为 interviewsapi.InterviewRecordDTO
func convertToInterviewRecordDTO(record *model.InterviewRecord) *interviewsapi.InterviewRecordDTO {
	dto := interviewsapi.NewInterviewRecordDTO()
	dto.ID = int64(record.ID)
	dto.UserID = int32(record.UserID)
	dto.Type = record.Type
	dto.Difficulty = record.Difficulty
	dto.Domain = record.Domain
	dto.Status = record.Status

	if record.CompanyName != "" {
		v := record.CompanyName
		dto.CompanyName = &v
	}
	if record.PositionName != "" {
		v := record.PositionName
		dto.PositionName = &v
	}
	if record.Duration != 0 {
		v := record.Duration
		dto.Duration = &v
	}
	if !record.CreatedAt.IsZero() {
		ms := record.CreatedAt.UnixNano() / int64(1000000)
		dto.CreatedAt = &ms
	}
	if !record.UpdatedAt.IsZero() {
		ms := record.UpdatedAt.UnixNano() / int64(1000000)
		dto.UpdatedAt = &ms
	}

	return dto
}

// GetInterviewEvaluation 根据用户ID和报告ID获取面试评估报告
func (s *InterviewServiceImpl) GetInterviewEvaluation(ctx context.Context, userID uint, reportID uint64) (interface{}, error) {
	evaluation, err := model.InterviewEvaluationDao.GetEvaluationByUserIDAndReportID(userID, reportID)
	if err != nil {
		log.Printf("Failed to get interview evaluation: %v", err)
		return nil, err
	}

	// 返回评估数据
	return map[string]interface{}{
		"id":         evaluation.ID,
		"user_id":    evaluation.UserID,
		"report_id":  evaluation.ReportID,
		"comment":    evaluation.Comment,
		"score":      evaluation.Score,
		"dimensions": evaluation.Dimensions,
		"created_at": evaluation.CreatedAt,
		"updated_at": evaluation.UpdatedAt,
	}, nil
}

// GetAnswerReport 根据用户ID和报告ID获取答题报告
func (s *InterviewServiceImpl) GetAnswerReport(ctx context.Context, userID uint, reportID uint64) (interface{}, error) {
	report, err := model.AnswerReportDao.GetAnswerReportByUserIDAndReportID(userID, reportID)
	if err != nil {
		log.Printf("[GetAnswerReport] 获取答题报告失败: %v", err)
		return nil, err
	}

	// 返回答题报告数据
	return map[string]interface{}{
		"id":         report.ID,
		"user_id":    report.UserID,
		"report_id":  report.ReportID,
		"records":    report.Records,
		"deleted":    report.Deleted,
		"created_at": report.CreatedAt,
		"updated_at": report.UpdatedAt,
	}, nil
}

// SaveInterviewDialogueWithParent 保存面试对话（支持父子关系）
// 主问题的 ParentID = 0
// 追问的 ParentID = 主问题的 ID
func (s *InterviewServiceImpl) SaveInterviewDialogueWithParent(
	ctx context.Context,
	userID uint,
	reportID uint64,
	mainQuestion *model.InterviewDialogue,
	followUpQuestions []*model.InterviewDialogue,
) error {
	// 1. 保存主问题（ParentID = 0）
	mainQuestion.UserID = userID
	mainQuestion.ReportID = reportID

	if err := model.InterviewDialogueDao.Create(mainQuestion); err != nil {
		return fmt.Errorf("failed to save main question: %w", err)
	}

	// 2. 保存追问（ParentID = 主问题的 ID）
	for _, followUp := range followUpQuestions {
		followUp.UserID = userID
		followUp.ReportID = reportID

		if err := model.InterviewDialogueDao.Create(followUp); err != nil {
			return fmt.Errorf("failed to save follow-up question: %w", err)
		}
	}

	return nil
}
