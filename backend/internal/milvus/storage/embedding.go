package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	einoopenai "github.com/cloudwego/eino-ext/components/embedding/openai"
	"github.com/cloudwego/eino/components/embedding"

	"interview-agents/internal/config"
)

// EmbeddingService Embedding服务包装，支持多种向量模型提供商
type EmbeddingService struct {
	embedder embedding.Embedder
	model    string
}

// NewEmbeddingService 根据配置中的 Provider 字段创建对应的 Embedding 服务
// 支持: ark (火山引擎), openai (OpenAI 及兼容接口), ollama (本地)
func NewEmbeddingService(ctx context.Context, cfg *config.EmbeddingConfig) (*EmbeddingService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("embedding config is nil")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("embedding model is required")
	}

	var embedder embedding.Embedder
	var err error

	switch cfg.Provider {
	case "ark":
		embedder, err = newArkEmbedder(ctx, cfg)
	case "openai", "":
		// openai 兼容接口，也是默认值
		// 国内大多数 API (DashScope/阿里云、智谱、百度千帆等) 均兼容 OpenAI 接口
		// 只需设置对应的 BaseURL 和 APIKey 即可
		embedder, err = newOpenAIEmbedder(ctx, cfg)
	default:
		return nil, fmt.Errorf("unsupported embedding provider %q, supported: ark, openai", cfg.Provider)
	}
	if err != nil {
		return nil, err
	}

	return &EmbeddingService{
		embedder: embedder,
		model:    cfg.Model,
	}, nil
}

func newArkEmbedder(ctx context.Context, cfg *config.EmbeddingConfig) (embedding.Embedder, error) {
	if cfg.APIKey == "" && (cfg.AccessKey == "" || cfg.SecretKey == "") {
		return nil, fmt.Errorf("ark provider requires APIKey or AccessKey/SecretKey")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://ark.cn-beijing.volces.com/api/v3"
	}
	region := cfg.Region
	if region == "" {
		region = "cn-beijing"
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	retryTimes := cfg.RetryTimes
	if retryTimes == 0 {
		retryTimes = 3
	}

	arkCfg := &ark.EmbeddingConfig{
		Model:      cfg.Model,
		BaseURL:    baseURL,
		Region:     region,
		APIKey:     cfg.APIKey,
		AccessKey:  cfg.AccessKey,
		SecretKey:  cfg.SecretKey,
		Timeout:    &timeout,
		RetryTimes: &retryTimes,
	}
	return ark.NewEmbedder(ctx, arkCfg)
}

func newOpenAIEmbedder(ctx context.Context, cfg *config.EmbeddingConfig) (embedding.Embedder, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("openai provider requires APIKey")
	}

	var dimensions *int
	if cfg.Dimensions > 0 {
		dimensions = &cfg.Dimensions
	}

	var user *string
	if cfg.User != "" {
		user = &cfg.User
	}

	openaiCfg := &einoopenai.EmbeddingConfig{
		Model:      cfg.Model,
		APIKey:     cfg.APIKey,
		BaseURL:    cfg.BaseURL,
		Dimensions: dimensions,
		User:       user,
	}
	return einoopenai.NewEmbedder(ctx, openaiCfg)
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
	return s.model
}

// GetEmbedder 获取底层的 Embedder 实例（用于 Retriever 等组件）
func (s *EmbeddingService) GetEmbedder() embedding.Embedder {
	return s.embedder
}

// Close 关闭服务
func (s *EmbeddingService) Close() error {
	return nil
}
