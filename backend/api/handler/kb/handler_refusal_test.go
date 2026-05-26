package kb

import (
	"encoding/json"
	"testing"

	"interview-agents/internal/milvus/retrieval"
)

func TestBuildStandardRefusalPayload(t *testing.T) {
	payload := buildStandardRefusalPayload(retrieval.EvidenceGateOutcome{
		Result:               retrieval.EvidenceGateResultRefused,
		RefusalReason:        retrieval.RefusalReasonOutOfKBScope,
		CitationSupportScore: 0.25,
	})
	if payload == nil {
		t.Fatal("expected refusal payload")
	}
	if payload.Reason != retrieval.RefusalReasonOutOfKBScope {
		t.Fatalf("reason = %q, want %q", payload.Reason, retrieval.RefusalReasonOutOfKBScope)
	}
	if payload.CitationSupportScore != 0.25 {
		t.Fatalf("citation_support_score = %.2f, want 0.25", payload.CitationSupportScore)
	}
	if len(payload.Suggestions) == 0 {
		t.Fatal("expected suggestions")
	}
}

func TestRetrieveResponseWithRefusalJSON(t *testing.T) {
	resp := retrieveResponse{
		RequestID:          "req-1",
		Items:              []retrieveItem{},
		EvidenceGateResult: retrieval.EvidenceGateResultRefused,
		CitationCheck: &citationCheckResponse{
			Supported:             false,
			SupportScore:          0.3,
			UnsupportedClaims:     []string{"scheduler detail"},
			UnsupportedClaimCount: 1,
			Version:               "phase3-citation-v1",
			LatencyMs:             8,
		},
		Refusal: &refusalPayload{
			Reason:               retrieval.RefusalReasonLowRerankConfidence,
			Message:              "evidence too weak",
			Suggestions:          []string{"narrow the question"},
			CitationSupportScore: 0.3,
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if parsed["evidence_gate_result"] != retrieval.EvidenceGateResultRefused {
		t.Fatalf("evidence_gate_result = %v, want %q", parsed["evidence_gate_result"], retrieval.EvidenceGateResultRefused)
	}
	refusal, ok := parsed["refusal"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected refusal object, got %T", parsed["refusal"])
	}
	if refusal["reason"] != retrieval.RefusalReasonLowRerankConfidence {
		t.Fatalf("reason = %v, want %q", refusal["reason"], retrieval.RefusalReasonLowRerankConfidence)
	}
	citationCheck, ok := parsed["citation_check"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected citation_check object, got %T", parsed["citation_check"])
	}
	if citationCheck["unsupported_claim_count"] != float64(1) {
		t.Fatalf("unsupported_claim_count = %v, want 1", citationCheck["unsupported_claim_count"])
	}
}
