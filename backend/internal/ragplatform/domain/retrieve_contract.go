package domain

import (
	"context"

	"interview-agents/internal/ragplatform/application"
)

// RetrieveContract 定义检索服务的接口
// 后续可被不同实现替换（本地、远程、mock）
type RetrieveContract interface {
	// Retrieve 执行检索
	Retrieve(ctx context.Context, req *application.RetrieveRequest) (*application.RetrieveResponse, error)
}
