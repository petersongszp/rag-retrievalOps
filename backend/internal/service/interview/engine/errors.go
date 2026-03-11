package core

import "errors"

var (
	// ErrSessionNotFound 会话未找到
	ErrSessionNotFound = errors.New("session not found")
	// ErrInvalidSessionID 无效的会话ID
	ErrInvalidSessionID = errors.New("invalid session id")
	// ErrUnauthorized 未授权
	ErrUnauthorized = errors.New("unauthorized")
)
