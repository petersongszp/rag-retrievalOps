package model

import "testing"

func TestCollectionRoleConstants(t *testing.T) {
	if CollectionRoleActive != "active" || CollectionRoleRollback != "rollback" {
		t.Fatalf("unexpected collection role constants")
	}
	if IndexBuildStatusReady != "ready" || IndexBuildStatusRolledBack != "rolled_back" {
		t.Fatalf("unexpected build status constants")
	}
}

func TestKBIndexRegistryTableNames(t *testing.T) {
	if (KBIndexRegistry{}).TableName() != "kb_index_registry" {
		t.Fatalf("unexpected index registry table name")
	}
	if (KBIndexOperationLog{}).TableName() != "kb_index_operation_log" {
		t.Fatalf("unexpected index operation log table name")
	}
}
