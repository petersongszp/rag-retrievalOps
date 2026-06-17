package chunking

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"

	"interview-agents/internal/documentparser"
)

func TestOCRAwareStrategyMarksWeakEvidenceChunks(t *testing.T) {
	doc := &documentparser.NormalizedDocument{
		ContentMarkdown: "scanned page text",
		Source:          documentparser.NormalizedSource{FileName: "scan.pdf", FileType: "pdf", PageCount: 1},
		Blocks: []documentparser.NormalizedBlock{
			{ID: "ocr-b1", Page: 1, MarkdownStart: 0, MarkdownEnd: 17, Confidence: 0.52},
		},
		Quality:   documentparser.ParseQuality{Status: "ok", Score: 0.52, Warnings: []string{"ocr confidence below threshold"}},
		Extractor: documentparser.ExtractorInfo{Provider: "docling", Version: "v1"},
	}
	delegate := &fixedChunkStrategy{
		chunks: []*schema.Document{
			{
				Content:  "scanned page text",
				MetaData: map[string]interface{}{"child_start_offset": 0, "child_end_offset": 17},
			},
		},
	}
	strategy := NewOCRAwareStrategy(delegate, 0.8)

	chunks, err := strategy.Split(context.Background(), Request{Document: doc})
	if err != nil {
		t.Fatalf("Split returned error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected one chunk, got %d", len(chunks))
	}
	if chunks[0].MetaData["chunking_strategy"] != "ocr-aware" {
		t.Fatalf("chunking_strategy = %v", chunks[0].MetaData["chunking_strategy"])
	}
	if chunks[0].MetaData["weak_evidence"] != true {
		t.Fatalf("weak_evidence = %v", chunks[0].MetaData["weak_evidence"])
	}
	if chunks[0].MetaData["weak_evidence_reason"] != "low_ocr_confidence" {
		t.Fatalf("weak_evidence_reason = %v", chunks[0].MetaData["weak_evidence_reason"])
	}
	if chunks[0].MetaData["ocr_confidence"] != 0.52 {
		t.Fatalf("ocr_confidence = %v", chunks[0].MetaData["ocr_confidence"])
	}
}

type fixedChunkStrategy struct {
	chunks []*schema.Document
}

func (s *fixedChunkStrategy) Split(_ context.Context, _ Request) ([]*schema.Document, error) {
	return s.chunks, nil
}
