package main

import (
	"os"
	"time"

	"interview-agents/internal/config"
)

func getTestConfig() *config.Config {
	// 尝试加载项目根目录的 config.yaml
	// 假设 milvusctl 运行在 backend/internal/milvus/cmd/milvusctl/
	// 或者项目根目录
	// 这里简化处理：优先使用环境变量，其次尝试加载 config.yaml，最后回退到默认值（但不包含敏感信息）

	// 获取 config.yaml 路径
	configPath := findConfigPath()

	var cfg *config.Config
	var err error

	if configPath != "" {
		cfg, err = config.LoadConfig(configPath)
		if err == nil {
			// 展开环境变量
			cfg.ExpandEnv()
			return cfg
		}
	}

	// 如果加载失败，返回基于环境变量的默认配置
	return &config.Config{
		Embedding: config.EmbeddingConfig{
			APIKey:     getEnvOrDefault("EMBEDDING_API_KEY", ""),
			Provider:   getEnvOrDefault("EMBEDDING_PROVIDER", "ark"),
			Model:      getEnvOrDefault("EMBEDDING_MODEL", "doubao-embedding-text-240715"),
			BaseURL:    getEnvOrDefault("EMBEDDING_BASE_URL", "https://ark.cn-beijing.volces.com/api/v3/"),
			Region:     getEnvOrDefault("EMBEDDING_REGION", "cn-beijing"),
			Timeout:    30 * time.Second,
			RetryTimes: 3,
			Dimensions: 2048,
		},
		DocumentSplitter: config.SplitterConfig{
			ChunkSize:   500,
			OverlapSize: 50,
			Separators:  []string{"\n\n", "\n", " "},
			KeepType:    0,
		},
		Milvus: config.MilvusConfig{
			Address:        getEnvOrDefault("MILVUS_ADDRESS", "localhost:19530"),
			CollectionName: "feishu_docs_20251126",
			DatabaseName:   "default",
			MetricType:     "COSINE",
			Username:       getEnvOrDefault("MILVUS_USERNAME", "minioadmin"),
			Password:       getEnvOrDefault("MILVUS_PASSWORD", "minioadmin"),
			TopK:           5,
			ConnectTimeout: 10 * time.Second,
			SearchTimeout:  30 * time.Second,
		},
	}
}

func findConfigPath() string {
	// 1. 尝试当前目录
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml"
	}
	// 2. 尝试 backend/config.yaml
	if _, err := os.Stat("backend/config.yaml"); err == nil {
		return "backend/config.yaml"
	}
	// 3. 尝试 ../../../../config.yaml (相对于源代码位置)
	// 注意：编译后的二进制文件可能无法使用相对路径查找源码位置，这里仅供开发便利
	return ""
}

// getEnvOrDefault 获取环境变量，如果不存在则返回默认值
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
