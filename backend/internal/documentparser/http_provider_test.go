package documentparser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPProviderParseSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		if got := r.FormValue("file_type"); got != "pdf" {
			t.Fatalf("file_type = %q", got)
		}
		_ = json.NewEncoder(w).Encode(NormalizedDocument{
			ContentMarkdown: "# Parsed\n\nBody",
			Source: NormalizedSource{
				FileName: "scan.pdf",
				FileType: "pdf",
			},
			Quality:   ParseQuality{Status: "ok", Score: 0.91},
			Extractor: ExtractorInfo{Provider: "http-parser", Version: NormalizerVersion},
		})
	}))
	defer server.Close()

	provider := NewHTTPProvider(HTTPProviderConfig{
		Endpoint: server.URL,
		Timeout:  2 * time.Second,
		Client:   server.Client(),
	})
	doc, err := provider.Parse(context.Background(), ProviderRequest{
		FileName: "scan.pdf",
		FileType: "pdf",
		Content:  []byte("%PDF"),
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if doc.ContentMarkdown != "# Parsed\n\nBody" {
		t.Fatalf("ContentMarkdown = %q", doc.ContentMarkdown)
	}
}

func TestHTTPProviderParseProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(ProviderError{
			Code:      "parse_failed",
			Message:   "failed to parse page 3",
			Stage:     "ocr",
			Page:      3,
			Retryable: false,
		})
	}))
	defer server.Close()

	provider := NewHTTPProvider(HTTPProviderConfig{
		Endpoint: server.URL,
		Timeout:  2 * time.Second,
		Client:   server.Client(),
	})
	_, err := provider.Parse(context.Background(), ProviderRequest{
		FileName: "scan.pdf",
		FileType: "pdf",
		Content:  []byte("%PDF"),
	})
	if err == nil {
		t.Fatalf("expected provider error")
	}
	if !strings.Contains(err.Error(), "parse_failed") {
		t.Fatalf("error = %v", err)
	}
}
