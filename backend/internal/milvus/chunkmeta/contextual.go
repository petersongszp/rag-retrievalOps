package chunkmeta

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"

	"github.com/cloudwego/eino/schema"
)

const (
	KeyEmbeddingContent     = "embedding_content"
	KeyEmbeddingContentHash = "embedding_content_hash"
	KeyContextPrefix        = "context_prefix"
	KeyContextPrefixHash    = "context_prefix_hash"

	EmbeddingBuildStrategyTitleSection = "title_section_v1"
	ContextVersionChunkContextV1       = "chunk_context_v1"
)

type ContextOptions struct {
	Enabled               bool
	Strategy              string
	MaxPrefixChars        int
	MaxContentChars       int
	SaveContentForDebug   bool
	StoredContentMaxChars int
}

func PrepareChunksForIndexing(docs []*schema.Document, opts ContextOptions) {
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		if doc.MetaData == nil {
			doc.MetaData = make(map[string]interface{})
		}

		embeddingContent, prefix := BuildEmbeddingContent(doc.Content, doc.MetaData, opts)
		doc.MetaData[KeyEmbeddingContent] = embeddingContent
		doc.MetaData[KeyEmbeddingContentHash] = shortHash(embeddingContent)
		if prefix != "" {
			doc.MetaData[KeyContextPrefix] = prefix
			doc.MetaData[KeyContextPrefixHash] = shortHash(prefix)
		}

		if opts.Enabled {
			doc.MetaData[KeyEmbeddingBuildStrategy] = firstNonEmpty(opts.Strategy, EmbeddingBuildStrategyTitleSection)
			doc.MetaData[KeyContextVersion] = ContextVersionChunkContextV1
		} else {
			doc.MetaData[KeyEmbeddingBuildStrategy] = EmbeddingBuildStrategyRaw
			doc.MetaData[KeyContextVersion] = ContextVersionRawContent
		}
	}
}

func StripIndexOnlyMetadata(docs []*schema.Document, opts ContextOptions) {
	for _, doc := range docs {
		if doc == nil || doc.MetaData == nil {
			continue
		}
		if !opts.SaveContentForDebug {
			delete(doc.MetaData, KeyEmbeddingContent)
			delete(doc.MetaData, KeyContextPrefix)
			continue
		}

		if value, ok := doc.MetaData[KeyEmbeddingContent].(string); ok {
			doc.MetaData[KeyEmbeddingContent] = truncateRunes(value, opts.StoredContentMaxChars)
		}
		if value, ok := doc.MetaData[KeyContextPrefix].(string); ok {
			doc.MetaData[KeyContextPrefix] = truncateRunes(value, opts.StoredContentMaxChars)
		}
	}
}

func BuildEmbeddingContent(raw string, metadata map[string]interface{}, opts ContextOptions) (string, string) {
	if !opts.Enabled {
		return raw, ""
	}

	title := firstNonEmpty(readString(metadata, "title"), readString(metadata, "file_name"))
	section := firstNonEmpty(readString(metadata, "hierarchy_path"), readString(metadata, "section_title"))

	lines := make([]string, 0, 3)
	if title != "" {
		lines = append(lines, "[Document]: "+title)
	}
	if section != "" {
		lines = append(lines, "[Section]: "+section)
	}
	if len(lines) > 0 {
		lines = append(lines, "[Chunk]:")
	}

	prefix := strings.Join(lines, "\n")
	prefix = truncateRunes(prefix, opts.MaxPrefixChars)
	content := truncateRunes(raw, opts.MaxContentChars)
	if prefix == "" {
		return content, ""
	}
	return strings.TrimSpace(prefix + "\n" + content), prefix
}

func ResolveEmbeddingText(doc *schema.Document) string {
	if doc == nil {
		return ""
	}
	if doc.MetaData != nil {
		if value, ok := doc.MetaData[KeyEmbeddingContent].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return doc.Content
}

func shortHash(value string) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:8])
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return strings.TrimSpace(value)
	}
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
