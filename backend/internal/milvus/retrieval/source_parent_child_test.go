package retrieval

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestAnnotateParentChildSourceCopiesCitationFields(t *testing.T) {
	doc := &schema.Document{
		MetaData: map[string]interface{}{
			"source":                 "feishu",
			"document_id":            uint64(9),
			"chunk_id":               "doc-9-child-000",
			"child_id":               "doc-9-child-000",
			"parent_id":              "doc-9-parent-001",
			"section_title":          "Storage",
			"hierarchy_path":         "Guide > Storage",
			"parent_child_available": true,
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
}
