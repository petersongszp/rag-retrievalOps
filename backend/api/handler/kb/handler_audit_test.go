package kb

import (
	"testing"

	"interview-agents/internal/model"
)

func TestParseSensitiveFieldsMasked(t *testing.T) {
	items := parseSensitiveFieldsMasked(`["query","content"]`)
	if len(items) != 2 || items[0] != "query" || items[1] != "content" {
		t.Fatalf("unexpected parsed items: %#v", items)
	}
}

func TestMaskAuditPayload(t *testing.T) {
	if got := maskAuditPayload(`{"query":"secret"}`); got != "[masked]" {
		t.Fatalf("maskAuditPayload = %q, want [masked]", got)
	}
	if got := maskAuditPayload(`{"status":"ok"}`); got == "" {
		t.Fatal("maskAuditPayload should preserve non-sensitive payload")
	}
}

func TestBuildAuditContractGaps(t *testing.T) {
	item := &model.KBAuditEvent{}
	gaps := buildAuditContractGaps(item)
	if len(gaps) < 4 {
		t.Fatalf("gaps = %#v, want actor/ip/ua/masked gaps", gaps)
	}
}
