package ragqueue

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"interview-agents/internal/config"
	"interview-agents/internal/documentparser"
)

func TestExtractNormalizedKnowledgeDocumentLocalMarkdown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "api.md")
	if err := os.WriteFile(path, []byte("| 字段 | 含义 |\n|---|---|\n| id | 编号 |\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, sidecarPath, err := extractNormalizedKnowledgeDocument(context.Background(), path, "md", "api.md")
	if err != nil {
		t.Fatalf("extractNormalizedKnowledgeDocument returned error: %v", err)
	}
	if len(doc.Tables) != 1 {
		t.Fatalf("expected markdown table sidecar, got %d", len(doc.Tables))
	}
	if filepath.Base(sidecarPath) != "api.md.normalized.json" {
		t.Fatalf("sidecarPath = %q", sidecarPath)
	}
	if _, err := os.Stat(sidecarPath); err != nil {
		t.Fatalf("sidecar should exist: %v", err)
	}
}

func TestExtractNormalizedKnowledgeDocumentPDFUsesPypdfium2AndCanonicalizes(t *testing.T) {
	oldConfig := config.Global
	t.Cleanup(func() {
		config.Global = oldConfig
	})

	var gotOptions map[string]interface{}
	parser := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		if got := r.FormValue("file_type"); got != "pdf" {
			t.Fatalf("file_type = %q, want pdf", got)
		}
		if err := json.Unmarshal([]byte(r.FormValue("options")), &gotOptions); err != nil {
			t.Fatalf("decode options: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(documentparser.NormalizedDocument{
			ContentMarkdown: "## 1.3 TPS\n\n## 消息收发 计算规则\n\n- 消息大小以 4 KB 为最小计量单位，不足 4 KB 按照 4 KB 计算。例如，消息收发 TPS 为（ 16/4 ） × （ 5000+5000 ） =40000 次 / 秒。",
			Source: documentparser.NormalizedSource{
				FileName: "billing.pdf",
				FileType: "pdf",
			},
			Quality: documentparser.ParseQuality{Status: "ok", Score: 1},
			Extractor: documentparser.ExtractorInfo{
				Provider: "docling",
				Version:  "test",
			},
		})
	}))
	defer parser.Close()

	config.Global.RAG.DocumentParser = config.DocumentParserConfig{
		Provider:    "http",
		Engine:      "docling",
		Endpoint:    parser.URL,
		TimeoutMS:   5000,
		StrictMode:  true,
		SaveSidecar: true,
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "billing.pdf")
	if err := os.WriteFile(path, []byte("%PDF"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, sidecarPath, err := extractNormalizedKnowledgeDocument(context.Background(), path, "pdf", "billing.pdf")
	if err != nil {
		t.Fatalf("extractNormalizedKnowledgeDocument returned error: %v", err)
	}
	if gotOptions["pdf_backend"] != "pypdfium2" {
		t.Fatalf("pdf_backend option = %v, want pypdfium2; options=%v", gotOptions["pdf_backend"], gotOptions)
	}
	if !hasHeadingWithAll(doc.ContentMarkdown, "1.3", "消息收发", "TPS", "计算规则") {
		t.Fatalf("canonical markdown did not merge PDF heading:\n%s", doc.ContentMarkdown)
	}
	if strings.Contains(doc.ContentMarkdown, "## 1.3 TPS\n\n## 消息收发") {
		t.Fatalf("canonical markdown should repair split PDF heading boundaries, got:\n%s", doc.ContentMarkdown)
	}
	if doc.ContentMarkdownRaw == "" || doc.Canonicalization.Version == "" {
		t.Fatalf("canonical metadata should be populated: raw=%q canonical=%+v", doc.ContentMarkdownRaw, doc.Canonicalization)
	}
	if filepath.Base(sidecarPath) != "billing.pdf.normalized.json" {
		t.Fatalf("sidecarPath = %q", sidecarPath)
	}
}

func hasHeadingWithAll(markdown string, terms ...string) bool {
	for _, line := range strings.Split(markdown, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		matched := true
		for _, term := range terms {
			if !strings.Contains(line, term) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func TestBuildParseErrorSidecar(t *testing.T) {
	err := &documentparser.ProviderError{
		Code:    "parse_failed",
		Message: "failed to parse page 3",
		Stage:   "ocr",
		Page:    3,
	}
	sidecar := buildParseErrorSidecar(err)
	if sidecar.ErrorCode != "parse_failed" {
		t.Fatalf("ErrorCode = %q", sidecar.ErrorCode)
	}
	if sidecar.Stage != "ocr" || sidecar.Page != 3 {
		t.Fatalf("stage/page = %q/%d", sidecar.Stage, sidecar.Page)
	}
}
