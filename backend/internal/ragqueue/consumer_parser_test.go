package ragqueue

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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
