package documentparser

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestHTTPProviderParseSuccess(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		if got := r.FormValue("file_type"); got != "pdf" {
			t.Fatalf("file_type = %q", got)
		}
		body, err := json.Marshal(NormalizedDocument{
			ContentMarkdown: "# Parsed\n\nBody",
			Source: NormalizedSource{
				FileName: "scan.pdf",
				FileType: "pdf",
			},
			Quality:   ParseQuality{Status: "ok", Score: 0.91},
			Extractor: ExtractorInfo{Provider: "http-parser", Version: NormalizerVersion},
		})
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		return jsonResponse(http.StatusOK, string(body)), nil
	})}

	provider := NewHTTPProvider(HTTPProviderConfig{
		Endpoint: "http://parser.test/parse",
		Timeout:  2 * time.Second,
		Client:   client,
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
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, err := json.Marshal(ProviderError{
			Code:      "parse_failed",
			Message:   "failed to parse page 3",
			Stage:     "ocr",
			Page:      3,
			Retryable: false,
		})
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		return jsonResponse(http.StatusUnprocessableEntity, string(body)), nil
	})}

	provider := NewHTTPProvider(HTTPProviderConfig{
		Endpoint: "http://parser.test/parse",
		Timeout:  2 * time.Second,
		Client:   client,
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
