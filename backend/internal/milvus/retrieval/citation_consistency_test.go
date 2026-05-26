package retrieval

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestCitationConsistencyCheckerSupported(t *testing.T) {
	checker := NewCitationConsistencyChecker(CitationConsistencyConfig{
		Enabled:   true,
		Threshold: 0.7,
		Version:   "phase3-citation-v1",
	})
	docs := []*schema.Document{
		{
			ID:      "chunk-1",
			Content: "The Go scheduler multiplexes goroutines onto system threads with work-stealing.",
			MetaData: map[string]interface{}{
				"document_id": uint64(7),
				"chunk_id":    "chunk-1",
			},
		},
	}

	outcome := checker.Check("How does the Go scheduler multiplex goroutines?", docs)
	if !outcome.Supported {
		t.Fatalf("supported = false, want true; outcome=%+v", outcome)
	}
	if outcome.SupportScore < 0.7 {
		t.Fatalf("support_score = %.3f, want >= 0.7", outcome.SupportScore)
	}
	if docs[0].MetaData["citation_supported"] != true {
		t.Fatalf("doc citation_supported = %v, want true", docs[0].MetaData["citation_supported"])
	}
}

func TestCitationConsistencyCheckerUnsupportedClaim(t *testing.T) {
	checker := NewCitationConsistencyChecker(CitationConsistencyConfig{
		Enabled:   true,
		Threshold: 0.72,
		Version:   "phase3-citation-v1",
	})
	docs := []*schema.Document{
		{
			ID:      "chunk-2",
			Content: "The storage layer persists vectors and metadata for retrieval.",
			MetaData: map[string]interface{}{
				"document_id": uint64(8),
				"chunk_id":    "chunk-2",
			},
		},
	}

	outcome := checker.Check("How does the Go scheduler multiplex goroutines?", docs)
	if outcome.Supported {
		t.Fatalf("supported = true, want false; outcome=%+v", outcome)
	}
	if len(outcome.UnsupportedClaims) == 0 {
		t.Fatalf("unsupported_claims empty, want at least one")
	}
	if docs[0].MetaData["low_support_citation"] != true {
		t.Fatalf("doc low_support_citation = %v, want true", docs[0].MetaData["low_support_citation"])
	}
}

func TestCitationConsistencyCheckerMissingVersionDegrades(t *testing.T) {
	checker := NewCitationConsistencyChecker(CitationConsistencyConfig{
		Enabled:   true,
		Threshold: 0.7,
	})

	outcome := checker.Check("go scheduler", nil)
	if outcome.Error == "" {
		t.Fatal("expected error when citation checker version is missing")
	}
	if !outcome.Supported {
		t.Fatalf("supported = false, want degraded true outcome; outcome=%+v", outcome)
	}
}
