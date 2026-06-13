package documentparser

import (
	"strings"
	"testing"
	"time"
)

func TestNewProviderHTTPAliases(t *testing.T) {
	for _, name := range []string{"", "http", "docling-http"} {
		t.Run(name, func(t *testing.T) {
			provider, err := NewProvider(ProviderConfig{
				Provider: name,
				Endpoint: "http://parser-provider:9000/parse",
				Timeout:  2 * time.Second,
			})
			if err != nil {
				t.Fatalf("NewProvider returned error: %v", err)
			}
			if _, ok := provider.(*HTTPProvider); !ok {
				t.Fatalf("provider type = %T, want *HTTPProvider", provider)
			}
		})
	}
}

func TestNewProviderRejectsUnsupportedProvider(t *testing.T) {
	_, err := NewProvider(ProviderConfig{
		Provider: "llamaparse",
		Endpoint: "http://parser-provider:9000/parse",
	})
	if err == nil {
		t.Fatalf("expected unsupported provider error")
	}
	if !strings.Contains(err.Error(), "unsupported parser provider") {
		t.Fatalf("error = %v", err)
	}
}
