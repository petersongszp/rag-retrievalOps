package tools

import (
	"interview-agents/internal/model"
	"context"
	"log"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// GetInterviewInfoRequest 获取面试对话记录的请求结构体
type GetInterviewInfoRequest struct {
	UserID   uint   `json:"user_id" jsonschema:"description=用户ID"`
	ReportID uint64 `json:"report_id" jsonschema:"description=报告ID"`
}

// GetInterviewInfoResponse 获取面试对话记录的响应结构体
type GetInterviewInfoResponse struct {
	Data []model.InterviewDialogue `json:"data" jsonschema:"description=面试对话记录列表"`
}

// GetInterviewInfo 获取面试对话记录
func GetInterviewInfo(_ context.Context, req *GetInterviewInfoRequest) (*GetInterviewInfoResponse, error) {
	if req == nil {
		return nil, nil
	}
	data, err := model.InterviewDialogueDao.GetInterviewDialoguesByUserIdAndRecordId(req.UserID, req.ReportID)
	if err != nil {
		log.Printf("get interview dialogues failed: %v", err)
		return nil, err
	}
	return &GetInterviewInfoResponse{
		Data: *data,
	}, nil
}

// GetInterviewInfoTool 创建获取面试对话记录的工具
func GetInterviewInfoTool() tool.InvokableTool {
	t, err := utils.InferTool(
		"get_mianshi_info",
		"获取用户的面试对话记录，包括所有问题和回答",
		GetInterviewInfo,
	)
	if err != nil {
		log.Fatalf("infer tool failed: %v", err)
	}
	return t
}
