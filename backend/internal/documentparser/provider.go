package documentparser

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Provider interface {
	Parse(ctx context.Context, req ProviderRequest) (*NormalizedDocument, error)
}

type ProviderConfig struct {
	Provider string
	Endpoint string
	Timeout  time.Duration
	Client   *http.Client
}

func NewProvider(cfg ProviderConfig) (Provider, error) {
	name := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if name == "" {
		name = "http"
	}

	switch name {
	case "http", "http-parser", "docling-http":
		if isUnsetEndpoint(cfg.Endpoint) {
			return nil, fmt.Errorf("parser provider endpoint is empty")
		}
		return NewHTTPProvider(HTTPProviderConfig{
			Endpoint: strings.TrimSpace(cfg.Endpoint),
			Timeout:  cfg.Timeout,
			Client:   cfg.Client,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported parser provider: %s", cfg.Provider)
	}
}

func isUnsetEndpoint(endpoint string) bool {
	trimmed := strings.TrimSpace(endpoint)
	return trimmed == "" || strings.Contains(trimmed, "$")
}
