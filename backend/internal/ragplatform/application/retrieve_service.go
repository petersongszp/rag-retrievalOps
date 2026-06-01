package application

import (
	"context"
	"fmt"
)

// RetrieveRequest 检索请求
type RetrieveRequest struct {
	Query           string                 `json:"query"`
	KBIDs           []uint64               `json:"kb_ids"`
	UserID          uint64                 `json:"user_id"`
	AppID           string                 `json:"app_id"`
	SourceAPI       string                 `json:"source_api"`
	TopK            int                    `json:"top_k"`
	StrategyProfile string                 `json:"strategy_profile"`
	MetadataFilter  map[string]interface{} `json:"metadata_filter"`
}

// RetrieveResponse 检索响应
type RetrieveResponse struct {
	RequestID       string         `json:"request_id"`
	Items           []RetrieveItem `json:"items"`
	StrategyVersion string         `json:"strategy_version"`
}

// RetrieveItem 检索结果项
type RetrieveItem struct {
	Content  string      `json:"content"`
	Score    float64     `json:"score"`
	Citation interface{} `json:"citation"`
	Source   interface{} `json:"source"`
}

// RetrieveService 检索服务
type RetrieveService struct {
	// 依赖注入：后续可替换为 repository 接口
}

// NewRetrieveService 创建检索服务
func NewRetrieveService() *RetrieveService {
	return &RetrieveService{}
}

// Retrieve 执行检索
func (s *RetrieveService) Retrieve(ctx context.Context, req *RetrieveRequest) (*RetrieveResponse, error) {
	// 第一版：委托给现有的检索逻辑
	// 后续 L4 阶段会从 handler 中提取核心逻辑到这里
	return nil, fmt.Errorf("not implemented: use kb.Retrieve handler for now")
}
