package tools

import (
	"interview-agents/internal/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"golang.org/x/net/context"
	"log"
)

// GetResumeInfoRequest 获取简历信息请求体
type GetResumeInfoRequest struct {
	ResumeID uint64 `json:"resume_id" jsonschema:"description=简历ID"`
}

// GetResumeInfoResponse 获取面试数据的响应结构体
type GetResumeInfoResponse struct {
	Data *model.Resume `json:"data" jsonschema:"description=简历解析后的内容"`
}

// GetResumeInfo 获取简历数据
func GetResumeInfo(_ context.Context, req *GetResumeInfoRequest) (*GetResumeInfoResponse, error) {
	if req == nil {
		return nil, nil
	}
	data, err := model.ResumeDao.GetResumeByID(req.ResumeID)
	if err != nil {
		log.Printf("get db data failed: %v", err)
		return nil, err
	}
	return &GetResumeInfoResponse{
		Data: data,
	}, nil
}

// GetResumeInfoTool 创建获取简历数据的工具
func GetResumeInfoTool() tool.InvokableTool {
	t, err := utils.InferTool(
		"get_resume_info",
		"获取用户的解析后的简历信息",
		GetResumeInfo,
	)
	if err != nil {
		log.Fatalf("infer tool failed: %v", err)
	}
	return t
}
