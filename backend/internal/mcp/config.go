package mcp

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultTransport       = "http"
	defaultHost            = "127.0.0.1"
	defaultPort            = 8898
	defaultEndpoint        = "/mcp"
	defaultUpstreamTimeout = 5 * time.Second
)

type Config struct {
	Enabled             bool
	AppEnv              string
	Transport           string
	Host                string
	Port                int
	Endpoint            string
	AllowedOrigins      []string
	RAGBaseURL          string
	RAGAccessToken      string
	Timeout             time.Duration
	EnableLegacyAppID   bool
	DisableMetrics      bool
	DisableHTTPAuth     bool
	SessionTimeout      time.Duration
	RequireOriginHeader bool
}

func LoadConfigFromEnv() (Config, error) {
	timeout := defaultUpstreamTimeout
	if raw := strings.TrimSpace(os.Getenv("MCP_UPSTREAM_TIMEOUT_MS")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return Config{}, fmt.Errorf("MCP_UPSTREAM_TIMEOUT_MS must be a positive integer")
		}
		timeout = time.Duration(parsed) * time.Millisecond
	}

	port := defaultPort
	if raw := strings.TrimSpace(os.Getenv("MCP_PORT")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 65535 {
			return Config{}, fmt.Errorf("MCP_PORT must be between 1 and 65535")
		}
		port = parsed
	}

	sessionTimeout := 5 * time.Minute
	if raw := strings.TrimSpace(os.Getenv("MCP_SESSION_TIMEOUT_MS")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return Config{}, fmt.Errorf("MCP_SESSION_TIMEOUT_MS must be a positive integer")
		}
		sessionTimeout = time.Duration(parsed) * time.Millisecond
	}

	cfg := Config{
		Enabled:             readBoolEnv("MCP_ENABLED", true),
		AppEnv:              strings.ToLower(strings.TrimSpace(defaultStringEnv("APP_ENV", "dev"))),
		Transport:           strings.ToLower(strings.TrimSpace(defaultStringEnv("MCP_TRANSPORT", defaultTransport))),
		Host:                strings.TrimSpace(defaultStringEnv("MCP_HOST", defaultHost)),
		Port:                port,
		Endpoint:            normalizeEndpoint(defaultStringEnv("MCP_ENDPOINT", defaultEndpoint)),
		AllowedOrigins:      parseCSVEnv("MCP_ALLOWED_ORIGINS"),
		RAGBaseURL:          strings.TrimSpace(os.Getenv("RAG_BASE_URL")),
		RAGAccessToken:      strings.TrimSpace(os.Getenv("RAG_ACCESS_TOKEN")),
		Timeout:             timeout,
		EnableLegacyAppID:   readBoolEnv("MCP_ENABLE_LEGACY_APP_ID", false),
		DisableMetrics:      readBoolEnv("MCP_DISABLE_METRICS", false),
		DisableHTTPAuth:     readBoolEnv("MCP_DISABLE_HTTP_AUTH", false),
		SessionTimeout:      sessionTimeout,
		RequireOriginHeader: readBoolEnv("MCP_REQUIRE_ORIGIN_HEADER", false),
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if !c.Enabled {
		return fmt.Errorf("MCP_ENABLED is false")
	}
	if c.Transport != "stdio" && c.Transport != "http" {
		return fmt.Errorf("MCP_TRANSPORT must be either \"stdio\" or \"http\"")
	}
	if c.isProd() {
		if c.Transport == "stdio" {
			return fmt.Errorf("MCP_TRANSPORT=stdio is not allowed when APP_ENV=prod; use MCP_TRANSPORT=http for production")
		}
		if c.Transport == "http" && len(c.AllowedOrigins) == 0 {
			return fmt.Errorf("MCP_ALLOWED_ORIGINS is required when APP_ENV=prod and MCP_TRANSPORT=http")
		}
	}
	if c.RAGBaseURL == "" {
		return fmt.Errorf("RAG_BASE_URL is required")
	}
	parsed, err := url.Parse(c.RAGBaseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("RAG_BASE_URL must be an absolute HTTP(S) URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("RAG_BASE_URL must use http or https")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("upstream timeout must be positive")
	}
	if c.Endpoint == "" || !strings.HasPrefix(c.Endpoint, "/") {
		return fmt.Errorf("MCP_ENDPOINT must start with /")
	}
	if c.Transport == "stdio" && c.RAGAccessToken == "" {
		return fmt.Errorf("RAG_ACCESS_TOKEN is required for stdio transport")
	}
	if c.EnableLegacyAppID {
		return fmt.Errorf("MCP_ENABLE_LEGACY_APP_ID is not supported in MCP Server V1")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("MCP_PORT must be between 1 and 65535")
	}
	if c.SessionTimeout <= 0 {
		return fmt.Errorf("MCP_SESSION_TIMEOUT_MS must be positive")
	}
	return nil
}

func (c Config) ListenAddress() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func (c Config) UpstreamReadyURL() string {
	return joinURLPath(c.RAGBaseURL, "/readyz")
}

func (c Config) UpstreamHealthURL() string {
	return joinURLPath(c.RAGBaseURL, "/healthz")
}

func (c Config) isProd() bool {
	return strings.EqualFold(strings.TrimSpace(c.AppEnv), "prod")
}

func defaultStringEnv(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func readBoolEnv(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseCSVEnv(name string) []string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func normalizeEndpoint(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultEndpoint
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return value
}

func joinURLPath(base, path string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	path = "/" + strings.TrimLeft(strings.TrimSpace(path), "/")
	return base + path
}
