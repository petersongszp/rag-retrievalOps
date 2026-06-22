package chunkmeta

import (
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestPrepareChunksForIndexingBuildsContextualEmbedding(t *testing.T) {
	doc := &schema.Document{
		Content: "Child chunk body.",
		MetaData: map[string]interface{}{
			"title":          "Guide",
			"hierarchy_path": "Guide > Operations",
		},
	}

	PrepareChunksForIndexing([]*schema.Document{doc}, ContextOptions{
		Enabled:               true,
		Strategy:              EmbeddingBuildStrategyTitleSection,
		MaxPrefixChars:        400,
		MaxContentChars:       3000,
		SaveContentForDebug:   false,
		StoredContentMaxChars: 0,
	})

	embeddingContent, _ := doc.MetaData[KeyEmbeddingContent].(string)
	if !strings.Contains(embeddingContent, "[Document]: Guide") {
		t.Fatalf("expected embedding content to include document title, got %q", embeddingContent)
	}
	if !strings.Contains(embeddingContent, "[Section]: Guide > Operations") {
		t.Fatalf("expected embedding content to include section path, got %q", embeddingContent)
	}
	if doc.MetaData[KeyEmbeddingBuildStrategy] != EmbeddingBuildStrategyTitleSection {
		t.Fatalf("expected contextual embedding strategy, got %v", doc.MetaData[KeyEmbeddingBuildStrategy])
	}
	if doc.MetaData[KeyContextVersion] != ContextVersionChunkContextV1 {
		t.Fatalf("expected contextual version, got %v", doc.MetaData[KeyContextVersion])
	}
	if ResolveEmbeddingText(doc) != embeddingContent {
		t.Fatalf("ResolveEmbeddingText should prefer embedding_content")
	}
}

func TestStripIndexOnlyMetadataRemovesEmbeddingContentWhenDebugDisabled(t *testing.T) {
	doc := &schema.Document{
		Content: "Raw body",
		MetaData: map[string]interface{}{
			KeyEmbeddingContent: "contextual body",
			KeyContextPrefix:    "[Document]: Guide",
		},
	}

	StripIndexOnlyMetadata([]*schema.Document{doc}, ContextOptions{})

	if _, ok := doc.MetaData[KeyEmbeddingContent]; ok {
		t.Fatalf("expected embedding_content to be stripped when debug is disabled")
	}
	if _, ok := doc.MetaData[KeyContextPrefix]; ok {
		t.Fatalf("expected context_prefix to be stripped when debug is disabled")
	}
}
