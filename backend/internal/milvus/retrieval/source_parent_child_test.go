package retrieval

import (
	"testing"

	"interview-agents/internal/milvus/chunkmeta"

	"github.com/cloudwego/eino/schema"
)

func TestAnnotateParentChildSourceCopiesCitationFields(t *testing.T) {
	doc := &schema.Document{
		MetaData: map[string]interface{}{
			"source":                   "feishu",
			"document_id":              uint64(9),
			"chunk_id":                 "doc-9-child-000",
			"child_id":                 "doc-9-child-000",
			"parent_id":                "doc-9-parent-001",
			"section_title":            "Storage",
			"hierarchy_path":           "Guide > Storage",
			"parent_child_available":   true,
			"split_strategy":           chunkmeta.SplitStrategyMarkdownV1,
			"split_version":            "v1",
			"source_file_type":         chunkmeta.SourceFileTypeMarkdown,
			"embedding_build_strategy": chunkmeta.EmbeddingBuildStrategyRaw,
			"context_version":          chunkmeta.ContextVersionRawContent,
		},
	}

	annotateParentChildSource(doc)

	source, ok := doc.MetaData["source"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected source map, got %T", doc.MetaData["source"])
	}
	if source["origin"] != "feishu" {
		t.Fatalf("expected source origin to preserve legacy string value, got %v", source["origin"])
	}
	if source["child_id"] != "doc-9-child-000" {
		t.Fatalf("expected child_id in source, got %v", source["child_id"])
	}
	if source["parent_id"] != "doc-9-parent-001" {
		t.Fatalf("expected parent_id in source, got %v", source["parent_id"])
	}
	if source["section_title"] != "Storage" {
		t.Fatalf("expected section_title in source, got %v", source["section_title"])
	}
	if source["hierarchy_path"] != "Guide > Storage" {
		t.Fatalf("expected hierarchy_path in source, got %v", source["hierarchy_path"])
	}
	if source["parent_child_available"] != true {
		t.Fatalf("expected parent_child_available=true, got %v", source["parent_child_available"])
	}
	if source["split_strategy"] != chunkmeta.SplitStrategyMarkdownV1 {
		t.Fatalf("expected split_strategy in source, got %v", source["split_strategy"])
	}
	if source["embedding_build_strategy"] != chunkmeta.EmbeddingBuildStrategyRaw {
		t.Fatalf("expected embedding_build_strategy in source, got %v", source["embedding_build_strategy"])
	}
}

func TestAnnotateParentChildSourceMarksLegacyChunksUnavailable(t *testing.T) {
	doc := &schema.Document{
		MetaData: map[string]interface{}{
			"document_id": uint64(3),
			"chunk_id":    "legacy-3",
		},
	}

	annotateParentChildSource(doc)

	source, ok := doc.MetaData["source"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected source map, got %T", doc.MetaData["source"])
	}
	if source["parent_child_available"] != false {
		t.Fatalf("expected legacy chunk to be marked parent_child_available=false, got %v", source["parent_child_available"])
	}
	if doc.MetaData["parent_child_available"] != false {
		t.Fatalf("expected doc metadata to record parent_child_available=false, got %v", doc.MetaData["parent_child_available"])
	}
	if source["split_strategy"] != chunkmeta.SplitStrategyLegacyRecursive {
		t.Fatalf("expected legacy split strategy fallback, got %v", source["split_strategy"])
	}
	if source["embedding_build_strategy"] != chunkmeta.EmbeddingBuildStrategyRaw {
		t.Fatalf("expected raw embedding fallback, got %v", source["embedding_build_strategy"])
	}
}

func TestParseMilvusMetadataSupportsCommonEncodings(t *testing.T) {
	inputs := []interface{}{
		`{"hierarchy_path":"Guide > Storage","chunk_index":16,"source":{"route":"dense"}}`,
		[]byte(`{"hierarchy_path":"Guide > Storage","chunk_index":16,"source":{"route":"dense"}}`),
		map[string]interface{}{
			"hierarchy_path": "Guide > Storage",
			"chunk_index":    16,
			"source": map[string]interface{}{
				"route": "dense",
			},
		},
	}

	for _, input := range inputs {
		metadata := parseMilvusMetadata(input)
		if metadata["hierarchy_path"] != "Guide > Storage" {
			t.Fatalf("expected hierarchy_path to be preserved, got %v", metadata["hierarchy_path"])
		}
		source, ok := metadata["source"].(map[string]interface{})
		if !ok || source["route"] != "dense" {
			t.Fatalf("expected nested source metadata, got %T %v", metadata["source"], metadata["source"])
		}
	}
}

func TestCloneDocumentWithMetadataDeepCopiesNestedMaps(t *testing.T) {
	original := &schema.Document{
		ID:      "doc-1-child-001",
		Content: "storage layer",
		MetaData: map[string]interface{}{
			"hierarchy_path": "Guide > Storage",
			"source": map[string]interface{}{
				"route": "dense",
			},
		},
	}

	cloned := cloneDocumentWithMetadata(original)
	source, ok := cloned.MetaData["source"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected cloned nested source map, got %T", cloned.MetaData["source"])
	}
	source["route"] = "sparse"

	originalSource, ok := original.MetaData["source"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected original nested source map, got %T", original.MetaData["source"])
	}
	if originalSource["route"] != "dense" {
		t.Fatalf("expected original metadata to remain unchanged, got %v", originalSource["route"])
	}
}
