package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LocalOSS 本地文件系统实现的 OSS
type LocalOSS struct {
	BasePath string
}

// NewLocalOSS 创建一个新的 LocalOSS 实例
func NewLocalOSS(basePath string) (*LocalOSS, error) {
	// 确保目录存在
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base path %s: %w", basePath, err)
	}
	return &LocalOSS{BasePath: basePath}, nil
}

// PutObject 实现上传到本地文件系统
func (l *LocalOSS) PutObject(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (string, error) {
	fullPath := filepath.Join(l.BasePath, key)

	// 创建文件
	out, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file %s: %w", fullPath, err)
	}
	defer out.Close()

	// 写入内容
	if _, err := io.Copy(out, reader); err != nil {
		// 如果写入失败，尝试清理
		_ = os.Remove(fullPath)
		return "", fmt.Errorf("failed to write file content: %w", err)
	}

	return fullPath, nil
}

// GetObject 获取本地文件
func (l *LocalOSS) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	fullPath := filepath.Join(l.BasePath, key)
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// DeleteObject 删除本地文件
func (l *LocalOSS) DeleteObject(ctx context.Context, key string) error {
	fullPath := filepath.Join(l.BasePath, key)
	return os.Remove(fullPath)
}

// GetURL 获取本地文件的绝对路径
func (l *LocalOSS) GetURL(ctx context.Context, key string) (string, error) {
	return filepath.Join(l.BasePath, key), nil
}
