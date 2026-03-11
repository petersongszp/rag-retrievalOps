package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino/components/embedding"
)

// 参考文档：https://www.cloudwego.io/zh/docs/eino/ecosystem_integration/embedding/embedding_ark/

// EmbeddingService Embedding服务包装
type EmbeddingService struct {
	embedder embedding.Embedder
	config   *ark.EmbeddingConfig
}

// NewArkEmbeddingService 创建新的Embedding服务
func NewArkEmbeddingService(ctx context.Context, config *ark.EmbeddingConfig) (*EmbeddingService, error) {
	if config == nil {
		return nil, fmt.Errorf("embedding config is nil")
	}
	// 验证配置
	if config.APIKey == "" && (config.AccessKey == "" || config.SecretKey == "") {
		return nil, fmt.Errorf("must provide APIKey or AccessKey/SecretKey")
	}
	if config.Model == "" {
		return nil, fmt.Errorf("model is required")
	}
	// 设置默认值
	if config.BaseURL == "" {
		config.BaseURL = "https://ark.cn-beijing.volces.com/api/v3"
	}
	if config.Region == "" {
		config.Region = "cn-beijing"
	}
	defaultTimeout := 30 * time.Second
	if config.Timeout == nil {
		config.Timeout = &defaultTimeout
	}
	defaultRetryTimes := 3
	if config.RetryTimes == nil {
		config.RetryTimes = &defaultRetryTimes
	}
	// 构建 Ark Embedding 配置
	arkConfig := &ark.EmbeddingConfig{
		Model:   config.Model,
		BaseURL: config.BaseURL,
		Region:  config.Region,
	}
	// 设置认证方式
	if config.APIKey != "" {
		arkConfig.APIKey = config.APIKey
	} else {
		arkConfig.AccessKey = config.AccessKey
		arkConfig.SecretKey = config.SecretKey
	}
	// 设置可选配置
	timeout := config.Timeout
	arkConfig.Timeout = timeout
	retryTimes := config.RetryTimes
	arkConfig.RetryTimes = retryTimes
	// 创建 embedder
	embedder, err := ark.NewEmbedder(ctx, arkConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create ark embedder: %w", err)
	}
	return &EmbeddingService{
		embedder: embedder,
		config:   config,
	}, nil
}

// EmbedBatch 批量将文本转换为向量
func (s *EmbeddingService) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("texts is empty")
	}

	vectors, err := s.embedder.EmbedStrings(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("failed to embed texts: %w", err)
	}

	return vectors, nil
}

// GetModel 获取模型名称
func (s *EmbeddingService) GetModel() string {
	return s.config.Model
}

// GetEmbedder 获取底层的 Embedder 实例（用于 Retriever 等组件）
func (s *EmbeddingService) GetEmbedder() embedding.Embedder {
	return s.embedder
}

// Close 关闭服务
func (s *EmbeddingService) Close() error {
	// 如果 embedder 有 Close 方法，可以在这里调用
	return nil
}
