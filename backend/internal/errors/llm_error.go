package errors

import "fmt"

// ModelUnavailableError 表示大模型调用遇到不可用故障（超时、5xx、熔断等）
// 用于区分普通的业务参数错误与需要触发外部人工/系统 Failover 的错误
type ModelUnavailableError struct {
	OriginalErr error
	ModelName   string
}

// Error 实现 error 接口
func (e *ModelUnavailableError) Error() string {
	return fmt.Sprintf("model %s unavailable: %v", e.ModelName, e.OriginalErr)
}

// Unwrap 支持 errors.Unwrap
func (e *ModelUnavailableError) Unwrap() error {
	return e.OriginalErr
}

// NewModelUnavailableError 创建新的实例
func NewModelUnavailableError(modelName string, err error) *ModelUnavailableError {
	return &ModelUnavailableError{
		OriginalErr: err,
		ModelName:   modelName,
	}
}
