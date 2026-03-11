package storage

import (
	"context"
	"io"
)

// OSS 定义对象存储通用接口
// 可以在 LocalOSS (本地文件系统) 和 MinIO/S3 等对象存储之间切换
type OSS interface {
	// PutObject 上传文件
	// key: 文件名或路径标识
	// reader: 文件内容
	// size: 文件大小 (可选，某些实现可能需要)
	// contentType: 文件类型 (可选)
	// 返回: 访问该文件的路径或URL，以及错误信息
	PutObject(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (string, error)

	// GetObject 获取文件内容
	GetObject(ctx context.Context, key string) (io.ReadCloser, error)

	// DeleteObject 删除文件
	DeleteObject(ctx context.Context, key string) error

	// GetURL 获取文件的访问地址 (可能是本地绝对路径，也可能是 HTTP URL)
	GetURL(ctx context.Context, key string) (string, error)
}
