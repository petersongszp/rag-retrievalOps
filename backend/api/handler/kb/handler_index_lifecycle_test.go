package kb

import (
	"testing"

	"interview-agents/internal/milvus/benchmark"
)

func TestBenchmarkProfileByName(t *testing.T) {
	profile, ok := benchmarkProfileByName("phase1-hnsw-baseline")
	if !ok {
		t.Fatal("expected phase1-hnsw-baseline profile")
	}
	if profile.Family != benchmark.IndexFamilyHNSW {
		t.Fatalf("profile family = %s, want %s", profile.Family, benchmark.IndexFamilyHNSW)
	}

	if _, ok := benchmarkProfileByName("not-exists"); ok {
		t.Fatal("unexpected profile for unknown name")
	}
}
