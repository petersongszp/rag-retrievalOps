package mcp

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

const defaultUpstreamTimeout = 10 * time.Second

type Config struct {
	RAGBaseURL     string
	RAGAccessToken string
	Timeout        time.Duration
}

func LoadConfigFromEnv() (Config, error) {
	cfg := Config{
		RAGBaseURL:     strings.TrimSpace(os.Getenv("RAG_BASE_URL")),
		RAGAccessToken: strings.TrimSpace(os.Getenv("RAG_ACCESS_TOKEN")),
		Timeout:        defaultUpstreamTimeout,
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
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
	if c.RAGAccessToken == "" {
		return fmt.Errorf("RAG_ACCESS_TOKEN is required")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("upstream timeout must be positive")
	}
	return nil
}
