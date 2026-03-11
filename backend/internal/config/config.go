package config

import (
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 应用程序配置结构
type Config struct {
	Host             string             `yaml:"host"`
	Port             int                `yaml:"port"`
	Database         DatabaseConfig     `yaml:"database"`
	Redis            RedisConfig        `yaml:"redis"`
	Hertz            HertzConfig        `yaml:"hertz"`
	Eino             EinoConfig         `yaml:"eino"`
	Interview        InterviewConfig    `yaml:"interview"`
	Security         SecurityConfig     `yaml:"security"`
	GoogleSearch     GoogleConfig       `yaml:"google_search"`
	OpenAI           OpenAIConfig       `yaml:"openai"`
	Embedding        EmbeddingConfig    `yaml:"Embedding"`
	Milvus           MilvusConfig       `yaml:"Milvus"`
	DocumentSplitter SplitterConfig     `yaml:"DocumentSplitter"`
	Wechat           WechatConfig       `yaml:"wechat"`     // 微信配置
	Feishu           FeishuConfig       `yaml:"feishu"`     // 飞书配置
	Email            EmailConfig        `yaml:"email"`      // 邮件配置
	RateLimit        LLMRateLimitConfig `yaml:"rate_limit"` // LLM API 限流配置
}

// EmailConfig 邮件配置
type EmailConfig struct {
	SMTPHost  string `yaml:"smtp_host"`
	SMTPPort  int    `yaml:"smtp_port"`
	SMTPUser  string `yaml:"smtp_user"`
	SMTPPass  string `yaml:"smtp_pass"`
	FromEmail string `yaml:"from_email"`
}

// WechatConfig 微信配置
type WechatConfig struct {
	AppID       string `yaml:"app_id"`
	AppSecret   string `yaml:"app_secret"`
	RedirectURL string `yaml:"redirect_url"`
}

// FeishuConfig 飞书配置
type FeishuConfig struct {
	WebhookURL string `yaml:"webhook_url"` // 飞书机器人 Webhook URL
	Enabled    bool   `yaml:"enabled"`     // 是否启用飞书告警
}

// CORSConfig CORS配置
type CORSConfig struct {
	AllowOrigins     []string `yaml:"allow_origins"`
	AllowMethods     []string `yaml:"allow_methods"`
	AllowHeaders     []string `yaml:"allow_headers"`
	ExposeHeaders    []string `yaml:"expose_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Driver          string `yaml:"driver"`
	DSN             string `yaml:"dsn"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime string `yaml:"conn_max_lifetime"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Addr         string `yaml:"addr"`
	Password     string `yaml:"password"`
	DB           int    `yaml:"db"`
	DialTimeout  string `yaml:"dial_timeout"`
	ReadTimeout  string `yaml:"read_timeout"`
	WriteTimeout string `yaml:"write_timeout"`
	PoolSize     int    `yaml:"pool_size"`
	MinIdleConns int    `yaml:"min_idle_conns"`
}

// HertzConfig Hertz框架配置（含 11.1 高并发相关）
type HertzConfig struct {
	LogLevel     string          `yaml:"log_level"`
	LogPath      string          `yaml:"log_path"`
	ReadTimeout  string          `yaml:"read_timeout"`
	WriteTimeout string          `yaml:"write_timeout"`
	IdleTimeout  string          `yaml:"idle_timeout"`
	KeepAlive    bool            `yaml:"keep_alive"` // 11.1.1 连接池：是否开启 KeepAlive
	GOMAXPROCS   int             `yaml:"gomaxprocs"` // 11.1.2 协程模型：P 数量，0 表示不设置（使用默认）
	RateLimit    RateLimitConfig `yaml:"rate_limit"` // 11.1.3 限流：支持分布式 Redis 限流
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	RPS            int    `yaml:"rps"`              // 每秒请求数
	Burst          int    `yaml:"burst"`            // 突发容量
	UseRedis       bool   `yaml:"use_redis"`        // 是否使用 Redis 分布式限流
	RedisKeyPrefix string `yaml:"redis_key_prefix"` // Redis 键前缀，空则用 "rl:"
}

// EinoConfig Eino框架配置
type EinoConfig struct {
	Model       string  `yaml:"model"`
	APIKey      string  `yaml:"api_key"`
	BaseURL     string  `yaml:"base_url"`
	MaxTokens   int     `yaml:"max_tokens"`
	Temperature float64 `yaml:"temperature"`
	RetryCount  int     `yaml:"retry_count"`
	RetryDelay  string  `yaml:"retry_delay"`
	// 11.2.3 推理成本控制：每用户每日 Token 配额（含 Reasoning Model R1/o3 等），0 表示不限制
	TokenQuotaPerUserPerDay int `yaml:"token_quota_per_user_per_day"`
}

// InterviewConfig 面试系统配置
type InterviewConfig struct {
	MaxDuration     string `yaml:"max_duration"`
	QuestionTimeout string `yaml:"question_timeout"`
	MaxQuestions    int    `yaml:"max_questions"`
	MinQuestions    int    `yaml:"min_questions"`
}

// SecurityConfig 安全性配置
type SecurityConfig struct {
	JWTSecret     string     `yaml:"jwt_secret"`
	JWTExpiration string     `yaml:"jwt_expiration"`
	CORS          CORSConfig `yaml:"cors"`
}

// GoogleConfig Google搜索配置
type GoogleConfig struct {
	APIKey         string `yaml:"api_key"`
	SearchEngineID string `yaml:"search_engine_id"`
}

// OpenAIConfig OpenAI配置
type OpenAIConfig struct {
	APIKey    string `yaml:"api_key"`
	ModelName string `yaml:"model_name"`
	BaseURL   string `yaml:"base_url"`
}

// RateLimitModelConfig 单个模型的限流配置
type RateLimitModelConfig struct {
	RPM int `yaml:"rpm"` // 每分钟最大请求数
	TPM int `yaml:"tpm"` // 每分钟最大 Token 数
}

// LLMRateLimitConfig LLM API 限流配置（与 Hertz 的 RateLimitConfig 区分）
type LLMRateLimitConfig struct {
	Enabled    bool                            `yaml:"enabled"`
	DefaultRPM int                             `yaml:"default_rpm"` // 默认 RPM 限制
	DefaultTPM int                             `yaml:"default_tpm"` // 默认 TPM 限制
	Models     map[string]RateLimitModelConfig `yaml:"models"`      // 按模型自定义限制
}

// Global 全局配置实例
var Global Config

// LoadConfig 从文件加载配置
func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}

	Global = cfg
	log.Println("配置加载成功")
	return &cfg, nil
}

// EmbeddingConfig Embedding服务配置
type EmbeddingConfig struct {
	// 认证配置（二选一）
	APIKey    string `yaml:"APIKey"`    // 使用 API Key 认证
	AccessKey string `yaml:"AccessKey"` // 使用 AK 认证
	SecretKey string `yaml:"SecretKey"` // 使用 SK 认证

	// 服务配置
	Model   string `yaml:"Model"`   // Ark 平台的端点 ID
	BaseURL string `yaml:"BaseURL"` // API 基础 URL
	Region  string `yaml:"Region"`  // 服务区域

	// 高级配置
	Timeout    time.Duration `yaml:"Timeout"`    // 请求超时时间
	RetryTimes int           `yaml:"RetryTimes"` // 重试次数
	Dimensions int           `yaml:"Dimensions"` // 输出向量维度
	User       string        `yaml:"User"`       // 用户标识
}

// MilvusConfig Milvus向量数据库配置
type MilvusConfig struct {
	// 连接配置
	Address        string `yaml:"Address"`        // Milvus 服务地址
	Username       string `yaml:"Username"`       // 用户名（可选）
	Password       string `yaml:"Password"`       // 密码（可选）
	DatabaseName   string `yaml:"DatabaseName"`   // 数据库名称
	CollectionName string `yaml:"CollectionName"` // 默认集合名称

	// 多集合配置
	Collections map[string]string `yaml:"Collections"` // 多个集合的命名映射

	// 检索配置
	TopK       int    `yaml:"TopK"`       // 返回的最相似文档数量
	MetricType string `yaml:"MetricType"` // 距离度量类型: L2, IP, COSINE

	// 超时配置
	ConnectTimeout time.Duration `yaml:"ConnectTimeout"` // 连接超时
	SearchTimeout  time.Duration `yaml:"SearchTimeout"`  // 搜索超时
}

// GetCollection 获取指定名称的集合，如果不存在则返回默认集合
func (c *MilvusConfig) GetCollection(name string) string {
	if c.Collections != nil {
		if col, ok := c.Collections[name]; ok {
			return col
		}
	}
	return c.CollectionName
}

// SplitterConfig 文档分割器配置
type SplitterConfig struct {
	ChunkSize   int      `yaml:"ChunkSize"`   // 目标片段大小（字符数）
	OverlapSize int      `yaml:"OverlapSize"` // 片段重叠大小（字符数）
	Separators  []string `yaml:"Separators"`  // 分隔符列表
	KeepType    int      `yaml:"KeepType"`    // 分隔符保留策略：0=不保留, 1=保留在开头, 2=保留在结尾
}

// ExpandEnv 展开配置中的环境变量引用
// 支持 ${VAR_NAME} 和 $VAR_NAME 两种语法
func (c *Config) ExpandEnv() {
	// 展开 Embedding 配置
	c.Embedding.APIKey = expandEnvVar(c.Embedding.APIKey)
	c.Embedding.AccessKey = expandEnvVar(c.Embedding.AccessKey)
	c.Embedding.SecretKey = expandEnvVar(c.Embedding.SecretKey)
	c.Embedding.Model = expandEnvVar(c.Embedding.Model)
	c.Embedding.BaseURL = expandEnvVar(c.Embedding.BaseURL)
	c.Embedding.Region = expandEnvVar(c.Embedding.Region)
	c.Embedding.User = expandEnvVar(c.Embedding.User)

	// 展开 Database 配置
	c.Database.DSN = expandEnvVar(c.Database.DSN)
	c.Database.Driver = expandEnvVar(c.Database.Driver)

	// 展开 Redis 配置
	c.Redis.Addr = expandEnvVar(c.Redis.Addr)
	c.Redis.Password = expandEnvVar(c.Redis.Password)

	// 展开 Security 配置
	c.Security.JWTSecret = expandEnvVar(c.Security.JWTSecret)

	// 展开 OpenAI 配置
	c.OpenAI.APIKey = expandEnvVar(c.OpenAI.APIKey)
	c.OpenAI.BaseURL = expandEnvVar(c.OpenAI.BaseURL)
	c.OpenAI.ModelName = expandEnvVar(c.OpenAI.ModelName)

	// 展开 Google 配置
	c.GoogleSearch.APIKey = expandEnvVar(c.GoogleSearch.APIKey)
	c.GoogleSearch.SearchEngineID = expandEnvVar(c.GoogleSearch.SearchEngineID)

	// 展开 Milvus 配置
	c.Milvus.Address = expandEnvVar(c.Milvus.Address)
	c.Milvus.Username = expandEnvVar(c.Milvus.Username)
	c.Milvus.Password = expandEnvVar(c.Milvus.Password)
	c.Milvus.DatabaseName = expandEnvVar(c.Milvus.DatabaseName)
	c.Milvus.CollectionName = expandEnvVar(c.Milvus.CollectionName)
	c.Milvus.MetricType = expandEnvVar(c.Milvus.MetricType)

	// 展开 Feishu 配置
	c.Feishu.WebhookURL = expandEnvVar(c.Feishu.WebhookURL)
}

// expandEnvVar 展开字符串中的环境变量引用
// 支持 ${VAR_NAME} 和 $VAR_NAME 两种语法
func expandEnvVar(s string) string {
	if s == "" {
		return s
	}

	// 匹配 ${VAR_NAME} 或 $VAR_NAME
	re := regexp.MustCompile(`\$\{([^}]+)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

	result := re.ReplaceAllStringFunc(s, func(match string) string {
		// 提取变量名
		varName := ""
		if strings.HasPrefix(match, "${") {
			// ${VAR_NAME} 格式
			varName = match[2 : len(match)-1]
		} else {
			// $VAR_NAME 格式
			varName = match[1:]
		}

		// 获取环境变量值
		value := os.Getenv(varName)
		if value != "" {
			return value
		}

		// 如果环境变量不存在，保持原样
		return match
	})

	return result
}
