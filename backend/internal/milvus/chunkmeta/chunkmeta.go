package chunkmeta

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	KeySplitStrategy            = "split_strategy"
	KeySplitVersion             = "split_version"
	KeySplitStage               = "split_stage"
	KeySourceFileType           = "source_file_type"
	KeySemanticSplitEnabled     = "semantic_split_enabled"
	KeySemanticSplitScore       = "semantic_split_score"
	KeySemanticParentSection    = "semantic_parent_section_id"
	KeySemanticBreakpointMethod = "semantic_breakpoint_method"
	KeyEmbeddingBuildStrategy   = "embedding_build_strategy"
	KeyContextVersion           = "context_version"

	SplitStrategyRecursiveV1      = "recursive_v1"
	SplitStrategyMarkdownV1       = "markdown_structure_v1"
	SplitStrategyLegacyRecursive  = "legacy_recursive"
	SplitStagePrimary             = "primary"
	EmbeddingBuildStrategyRaw     = "raw_content_embedding"
	ContextVersionRawContent      = "raw_content_v1"
	SemanticBreakpointEmbeddingV1 = "embedding_similarity_v1"
	SourceFileTypeMarkdown        = "markdown"
	SourceFileTypeTXT             = "txt"
	SourceFileTypePDF             = "pdf"
	SourceFileTypeUnknownFallback = "unknown"
)

func NormalizeSourceFileType(fileType, fileName string) string {
	normalized := strings.ToLower(strings.TrimSpace(fileType))
	if normalized == "" && strings.TrimSpace(fileName) != "" {
		normalized = strings.TrimPrefix(strings.ToLower(filepath.Ext(fileName)), ".")
	}
	switch normalized {
	case "md", "markdown":
		return SourceFileTypeMarkdown
	case "txt", "text":
		return SourceFileTypeTXT
	case "pdf":
		return SourceFileTypePDF
	case "":
		return SourceFileTypeUnknownFallback
	default:
		return normalized
	}
}

func DefaultSplitStrategyForSourceType(fileType string) string {
	switch NormalizeSourceFileType(fileType, "") {
	case SourceFileTypeMarkdown:
		return SplitStrategyMarkdownV1
	default:
		return SplitStrategyRecursiveV1
	}
}

func VersionForStrategy(strategy string) string {
	normalized := strings.TrimSpace(strategy)
	if normalized == "" {
		return ""
	}
	if normalized == SplitStrategyLegacyRecursive {
		return "legacy"
	}
	parts := strings.Split(normalized, "_")
	last := parts[len(parts)-1]
	if strings.HasPrefix(last, "v") {
		return last
	}
	return "v1"
}

func ApplyDefaults(metadata map[string]interface{}, defaultSplitStrategy string) map[string]interface{} {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	splitStrategy := readString(metadata, KeySplitStrategy)
	if splitStrategy == "" {
		splitStrategy = strings.TrimSpace(defaultSplitStrategy)
	}
	if splitStrategy == "" {
		splitStrategy = SplitStrategyLegacyRecursive
	}
	metadata[KeySplitStrategy] = splitStrategy

	if readString(metadata, KeySplitVersion) == "" {
		metadata[KeySplitVersion] = VersionForStrategy(splitStrategy)
	}
	if readString(metadata, KeySplitStage) == "" {
		metadata[KeySplitStage] = SplitStagePrimary
	}
	if _, exists := metadata[KeySemanticSplitEnabled]; !exists {
		metadata[KeySemanticSplitEnabled] = false
	}
	if readString(metadata, KeyEmbeddingBuildStrategy) == "" {
		metadata[KeyEmbeddingBuildStrategy] = EmbeddingBuildStrategyRaw
	}
	if readString(metadata, KeyContextVersion) == "" {
		metadata[KeyContextVersion] = ContextVersionRawContent
	}

	return metadata
}

func readString(metadata map[string]interface{}, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}
