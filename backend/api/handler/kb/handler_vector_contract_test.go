package kb

import (
	"testing"
	"time"

	"interview-agents/internal/model"
)

func TestBuildVectorOperationTimestamps(t *testing.T) {
	now := time.Now().UTC()
	switchAt, rebuildAt := buildVectorOperationTimestamps([]*model.KBIndexOperationLog{
		{IndexVersion: "idx-1", Operation: "register", CreatedAt: now.Add(-2 * time.Hour)},
		{IndexVersion: "idx-1", Operation: "switch_active", CreatedAt: now.Add(-time.Hour)},
		{IndexVersion: "idx-1", Operation: "build_candidate", CreatedAt: now},
	})

	if rebuildAt["idx-1"] == nil || !rebuildAt["idx-1"].Equal(now) {
		t.Fatalf("rebuildAt[idx-1] = %#v, want %v", rebuildAt["idx-1"], now)
	}
	if switchAt["idx-1"] == nil || !switchAt["idx-1"].Equal(now.Add(-time.Hour)) {
		t.Fatalf("switchAt[idx-1] = %#v, want %v", switchAt["idx-1"], now.Add(-time.Hour))
	}
}

func TestBuildVectorListContractGapsMarksDuplicateCollectionName(t *testing.T) {
	registry := []*model.KBIndexRegistry{
		{IndexVersion: "idx-1", CollectionName: "knowledge_docs"},
		{IndexVersion: "idx-2", CollectionName: "knowledge_docs"},
	}

	gaps := buildVectorListContractGaps(registry[0], registry)
	found := false
	for _, item := range gaps {
		if item == "collection_name_not_unique_use_index_version" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("gaps = %#v, want duplicate collection name gap", gaps)
	}
}

func TestDeriveVectorHealthStatus(t *testing.T) {
	if deriveVectorHealthStatus(model.IndexBuildStatusReady) != "healthy" {
		t.Fatal("ready should be healthy")
	}
	if deriveVectorHealthStatus(model.IndexBuildStatusBuilding) != "degraded" {
		t.Fatal("building should be degraded")
	}
	if deriveVectorHealthStatus(model.IndexBuildStatusFailed) != "unhealthy" {
		t.Fatal("failed should be unhealthy")
	}
}
