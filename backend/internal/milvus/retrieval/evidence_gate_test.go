package retrieval

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestEvaluateEvidenceGateDisabled(t *testing.T) {
	outcome := EvaluateEvidenceGate("what is go", nil, SearchMetrics{}, EvidenceGateConfig{})
	if outcome.Result != EvidenceGateResultDisabled {
		t.Fatalf("result = %q, want %q", outcome.Result, EvidenceGateResultDisabled)
	}
}

func TestEvaluateEvidenceGateNoRetrievalHit(t *testing.T) {
	outcome := EvaluateEvidenceGate("what is go", nil, SearchMetrics{}, EvidenceGateConfig{
		Enabled:             true,
		MinRerankScore:      0.55,
		MinEvidenceDensity:  0.2,
		MinCitationCoverage: 0.5,
	})
	if outcome.Result != EvidenceGateResultRefused {
		t.Fatalf("result = %q, want %q", outcome.Result, EvidenceGateResultRefused)
	}
	if outcome.RefusalReason != RefusalReasonNoRetrievalHit {
		t.Fatalf("refusal_reason = %q, want %q", outcome.RefusalReason, RefusalReasonNoRetrievalHit)
	}
}

func TestEvaluateEvidenceGateLowConfidence(t *testing.T) {
	docs := []*schema.Document{
		{
			ID:      "chunk-1",
			Content: "The Go scheduler runs goroutines, but this snippet is too generic to explain how scheduling actually works.",
			MetaData: map[string]interface{}{
				"document_id":  1,
				"chunk_id":     "chunk-1",
				"rerank_score": 0.32,
			},
		},
	}

	outcome := EvaluateEvidenceGate("how does go scheduler work", docs, SearchMetrics{
		EvidenceDensity: 0.1,
	}, EvidenceGateConfig{
		Enabled:             true,
		MinRerankScore:      0.55,
		MinEvidenceDensity:  0.2,
		MinCitationCoverage: 0.5,
	})
	if outcome.Result != EvidenceGateResultRefused {
		t.Fatalf("result = %q, want %q", outcome.Result, EvidenceGateResultRefused)
	}
	if outcome.RefusalReason != RefusalReasonLowRerankConfidence {
		t.Fatalf("refusal_reason = %q, want %q", outcome.RefusalReason, RefusalReasonLowRerankConfidence)
	}
}

func TestEvaluateEvidenceGateCitationCoverage(t *testing.T) {
	docs := []*schema.Document{
		{
			Content: "Go uses goroutines and a work-stealing scheduler.",
			MetaData: map[string]interface{}{
				"rerank_score": 0.91,
			},
		},
	}

	outcome := EvaluateEvidenceGate("go scheduler", docs, SearchMetrics{
		EvidenceDensity: 0.9,
	}, EvidenceGateConfig{
		Enabled:             true,
		MinRerankScore:      0.55,
		MinEvidenceDensity:  0.2,
		MinCitationCoverage: 0.5,
	})
	if outcome.Result != EvidenceGateResultRefused {
		t.Fatalf("result = %q, want %q", outcome.Result, EvidenceGateResultRefused)
	}
	if outcome.RefusalReason != RefusalReasonInsufficientCitationCover {
		t.Fatalf("refusal_reason = %q, want %q", outcome.RefusalReason, RefusalReasonInsufficientCitationCover)
	}
}

func TestEvaluateEvidenceGatePass(t *testing.T) {
	docs := []*schema.Document{
		{
			ID:      "chunk-1",
			Content: "The Go scheduler multiplexes goroutines onto system threads.",
			MetaData: map[string]interface{}{
				"document_id":  42,
				"chunk_id":     "chunk-1",
				"rerank_score": 0.92,
			},
		},
	}

	outcome := EvaluateEvidenceGate("go scheduler goroutines", docs, SearchMetrics{
		EvidenceDensity: 0.8,
	}, EvidenceGateConfig{
		Enabled:             true,
		MinRerankScore:      0.55,
		MinEvidenceDensity:  0.2,
		MinCitationCoverage: 0.5,
	})
	if outcome.Result != EvidenceGateResultPass {
		t.Fatalf("result = %q, want %q", outcome.Result, EvidenceGateResultPass)
	}
	if outcome.RefusalReason != "" {
		t.Fatalf("refusal_reason = %q, want empty", outcome.RefusalReason)
	}
}

func TestEvaluateEvidenceGateRespectsUnsupportedClaims(t *testing.T) {
	docs := []*schema.Document{
		{
			ID:      "chunk-1",
			Content: "The Go scheduler multiplexes goroutines onto system threads.",
			MetaData: map[string]interface{}{
				"document_id":  uint64(42),
				"chunk_id":     "chunk-1",
				"rerank_score": 0.92,
			},
		},
	}

	outcome := EvaluateEvidenceGate("go scheduler and garbage collector tuning", docs, SearchMetrics{
		EvidenceDensity:       0.8,
		CitationSupported:     false,
		CitationSupportScore:  0.82,
		UnsupportedClaimCount: 1,
		CitationCheckVersion:  "phase3-citation-v1",
	}, EvidenceGateConfig{
		Enabled:             true,
		MinRerankScore:      0.55,
		MinEvidenceDensity:  0.2,
		MinCitationCoverage: 0.5,
	})
	if outcome.Result != EvidenceGateResultRefused {
		t.Fatalf("result = %q, want %q", outcome.Result, EvidenceGateResultRefused)
	}
	if outcome.RefusalReason != RefusalReasonInsufficientCitationCover {
		t.Fatalf("refusal_reason = %q, want %q", outcome.RefusalReason, RefusalReasonInsufficientCitationCover)
	}
}
