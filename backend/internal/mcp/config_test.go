package mcp

import (
	"strings"
	"testing"
	"time"
)

func TestConfigValidateRejectsLegacyAppID(t *testing.T) {
	cfg := validTestConfig()
	cfg.EnableLegacyAppID = true

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("error = %q, want not supported", err.Error())
	}
}

func TestConfigValidateRejectsProdHTTPWithoutAllowedOrigins(t *testing.T) {
	cfg := validTestConfig()
	cfg.AppEnv = "prod"
	cfg.Transport = "http"
	cfg.AllowedOrigins = nil

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "MCP_ALLOWED_ORIGINS is required") {
		t.Fatalf("error = %q, want MCP_ALLOWED_ORIGINS message", err.Error())
	}
}

func TestConfigValidateRejectsProdStdio(t *testing.T) {
	cfg := validTestConfig()
	cfg.AppEnv = "prod"
	cfg.Transport = "stdio"
	cfg.RAGAccessToken = "rag_prod_token"

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "MCP_TRANSPORT=stdio is not allowed") {
		t.Fatalf("error = %q, want stdio not allowed message", err.Error())
	}
}

func TestConfigValidateAllowsProdHTTPWithAllowedOrigins(t *testing.T) {
	cfg := validTestConfig()
	cfg.AppEnv = "prod"
	cfg.Transport = "http"
	cfg.AllowedOrigins = []string{"https://agent.example.com"}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func validTestConfig() Config {
	return Config{
		Enabled:          true,
		AppEnv:           "dev",
		Transport:        "http",
		Host:             "127.0.0.1",
		Port:             8898,
		Endpoint:         "/mcp",
		AllowedOrigins:   []string{"http://localhost:3000"},
		RAGBaseURL:       "http://rag-server:8899",
		RAGAccessToken:   "rag_local_debug_token",
		Timeout:          time.Second,
		SessionTimeout:   time.Minute,
		DisableHTTPAuth:  false,
		DisableMetrics:   false,
		RequireOriginHeader: false,
	}
}
