package errors

import (
	"fmt"
	"net/http"
	"strings"
)

// ErrorCode 错误码类型
type ErrorCode string

const (
	ErrCodeInternal     ErrorCode = "INTERNAL_ERROR"
	ErrCodeInvalidParam ErrorCode = "INVALID_PARAM"
	ErrCodeNotFound     ErrorCode = "NOT_FOUND"
	ErrCodeUnauthorized ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden    ErrorCode = "FORBIDDEN"
	ErrCodeDBError      ErrorCode = "DATABASE_ERROR"
	ErrCodeRedisError   ErrorCode = "REDIS_ERROR"
	ErrCodeMilvusError  ErrorCode = "MILVUS_ERROR"
	ErrCodeModelError   ErrorCode = "MODEL_ERROR"
	ErrCodeValidation   ErrorCode = "VALIDATION_ERROR"
	ErrCodeFeishuError  ErrorCode = "FEISHU_ERROR"
	ErrCodeOpenAIError  ErrorCode = "OPENAI_ERROR"
	// 模型API相关错误码
	ErrCodeInsufficientTokens    ErrorCode = "INSUFFICIENT_TOKENS"     // 模型API配额不足或令牌用尽
	ErrCodeRateLimitExceeded     ErrorCode = "RATE_LIMIT_EXCEEDED"     // 模型API请求频率限制超出
	ErrCodeContextLengthExceeded ErrorCode = "CONTEXT_LENGTH_EXCEEDED" // 模型API上下文长度超出限制
)

// AppError 应用错误结构
// 简化设计：只保留必要的字段
type AppError struct {
	Code       ErrorCode `json:"code"`    // 错误码，用于程序识别
	Message    string    `json:"message"` // 用户友好的错误消息
	HTTPStatus int       `json:"-"`       // HTTP状态码，不返回给客户端
	Err        error     `json:"-"`       // 底层错误，用于错误链追踪
}

// Error 实现 error 接口
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 返回底层错误（用于错误链追踪）
func (e *AppError) Unwrap() error {
	return e.Err
}

// NewAppError 创建新的应用错误（不带底层错误）
func NewAppError(code ErrorCode, message string, httpStatus int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
	}
}

// WrapError 包装现有错误
// 作用：保留原始错误信息，同时添加业务层面的错误码和消息
// 例如：数据库错误 -> 包装成 DBError，保留原始错误用于调试
func WrapError(err error, code ErrorCode, message string, httpStatus int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
		Err:        err, // 保留原始错误，可以通过 Unwrap() 获取
	}
}

// 预定义错误构造函数
func NewInternalError(message string, err error) *AppError {
	return WrapError(err, ErrCodeInternal, message, http.StatusInternalServerError)
}

func NewInvalidParamError(message string) *AppError {
	return NewAppError(ErrCodeInvalidParam, message, http.StatusBadRequest)
}

func NewNotFoundError(resource string) *AppError {
	msg := fmt.Sprintf("%s not found", resource)
	return NewAppError(ErrCodeNotFound, msg, http.StatusNotFound)
}

func NewDBError(message string, err error) *AppError {
	statusCode := http.StatusInternalServerError
	if err != nil {
		if strings.Contains(err.Error(), "record not found") {
			statusCode = http.StatusNotFound
		}
	}
	return WrapError(err, ErrCodeDBError, message, statusCode)
}

func NewValidationError(message string) *AppError {
	return NewAppError(ErrCodeValidation, message, http.StatusBadRequest)
}

func NewMilvusError(message string, err error) *AppError {
	return WrapError(err, ErrCodeMilvusError, message, http.StatusInternalServerError)
}

func NewFeishuError(message string, err error) *AppError {
	return WrapError(err, ErrCodeFeishuError, message, http.StatusBadGateway)
}

func NewOpenAIError(message string, err error) *AppError {
	return WrapError(err, ErrCodeOpenAIError, message, http.StatusBadGateway)
}

func NewModelError(message string, err error) *AppError {
	statusCode := http.StatusInternalServerError
	if err != nil {
		errMsg := strings.ToLower(err.Error())
		if strings.Contains(errMsg, "429") || strings.Contains(errMsg, "too many requests") || strings.Contains(errMsg, "rate limit") {
			statusCode = http.StatusTooManyRequests
		} else if strings.Contains(errMsg, "401") || strings.Contains(errMsg, "unauthorized") || strings.Contains(errMsg, "api key") {
			statusCode = http.StatusBadRequest
		} else if strings.Contains(errMsg, "empty choices") {
			statusCode = http.StatusBadGateway
		}
	}
	return WrapError(err, ErrCodeModelError, message, statusCode)
}

// NewInsufficientTokensError 创建令牌不足错误
// 用于处理模型API配额不足或令牌用尽的情况
// 返回HTTP 402 (Payment Required)状态码
// 前端可根据此错误提示用户检查账户余额或充值
func NewInsufficientTokensError(message string, err error) *AppError {
	return WrapError(err, ErrCodeInsufficientTokens, message, http.StatusPaymentRequired)
}

// NewRateLimitExceededError 创建请求频率限制超出错误
// 用于处理模型API请求频率限制被超出的情况
// 返回HTTP 429 (Too Many Requests)状态码
// 前端可根据此错误提示用户稍后重试
func NewRateLimitExceededError(message string, err error) *AppError {
	return WrapError(err, ErrCodeRateLimitExceeded, message, http.StatusTooManyRequests)
}

// NewContextLengthExceededError 创建上下文长度超出错误
// 用于处理模型API上下文长度超出限制的情况
// 返回HTTP 413 (Payload Too Large)状态码
// 前端可根据此错误提示用户输入内容过长，需要简化或分割
func NewContextLengthExceededError(message string, err error) *AppError {
	return WrapError(err, ErrCodeContextLengthExceeded, message, http.StatusRequestEntityTooLarge)
}

// As 检查错误链中是否存在指定类型的错误
func As(err error, target interface{}) bool {
	if err == nil {
		return false
	}
	if appErr, ok := err.(*AppError); ok {
		if t, ok := target.(**AppError); ok {
			*t = appErr
			return true
		}
	}
	// 检查错误链
	if unwrapped := Unwrap(err); unwrapped != nil {
		return As(unwrapped, target)
	}
	return false
}

// Unwrap 返回底层错误
func Unwrap(err error) error {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Unwrap()
	}
	return nil
}
