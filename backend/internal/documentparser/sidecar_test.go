package documentparser

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveNormalizedSidecar(t *testing.T) {
	dir := t.TempDir()
	path, err := SaveNormalizedSidecar(context.Background(), filepath.Join(dir, "doc.pdf"), &NormalizedDocument{
		ContentMarkdown: "Body",
		Source: NormalizedSource{
			FileName: "doc.pdf",
			FileType: "pdf",
		},
		Quality:   ParseQuality{Status: "ok"},
		Extractor: ExtractorInfo{Provider: "test", Version: NormalizerVersion},
	})
	if err != nil {
		t.Fatalf("SaveNormalizedSidecar returned error: %v", err)
	}
	if filepath.Base(path) != "doc.pdf.normalized.json" {
		t.Fatalf("sidecar base = %q", filepath.Base(path))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("sidecar should not be empty")
	}
}

func TestSaveErrorSidecar(t *testing.T) {
	dir := t.TempDir()
	path, err := SaveErrorSidecar(context.Background(), filepath.Join(dir, "doc.pdf"), ErrorSidecar{
		ErrorCode: "parse_error",
		Message:   "failed to parse page 3",
		Stage:     "ocr",
		Page:      3,
	})
	if err != nil {
		t.Fatalf("SaveErrorSidecar returned error: %v", err)
	}
	if filepath.Base(path) != "doc.pdf.normalized.error.json" {
		t.Fatalf("error sidecar base = %q", filepath.Base(path))
	}
}
