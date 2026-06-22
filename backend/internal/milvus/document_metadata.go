package milvus

import (
	"path/filepath"
	"strings"
	"time"

	"interview-agents/internal/milvus/chunkmeta"
	"interview-agents/internal/milvus/retrieval"

	"github.com/cloudwego/eino/schema"
)

// 使用 retrieval 包中定义的类型
type DocumentLanguage = retrieval.DocumentLanguage
type DocumentCategory = retrieval.DocumentCategory
type RetrieveOptions = retrieval.RetrieveOptions

// 重新导出常量
const (
	LanguageGolang        = retrieval.LanguageGolang
	LanguageJava          = retrieval.LanguageJava
	LanguageMiddleware    = retrieval.LanguageMiddleware
	CategorySpecialized   = retrieval.CategorySpecialized
	CategoryComprehensive = retrieval.CategoryComprehensive
	CategoryBasic         = retrieval.CategoryBasic
)

// DocumentMetadata 文档元数据结构
// 用于存储到 Milvus 的 metadata 字段中
type DocumentMetadata struct {
	Language DocumentLanguage `json:"language"`

	Category DocumentCategory `json:"category"`

	FilePath string `json:"file_path"`

	FileName string `json:"file_name"`

	Title string `json:"title"`

	Source string `json:"source,omitempty"`

	OperatorAdminID uint   `json:"operator_admin_id"`
	KBScope         string `json:"kb_scope"`
	TenantID        uint64 `json:"tenant_id"`

	KBID uint64 `json:"kb_id"`

	DocumentID uint64 `json:"document_id"`

	ChunkIndex int `json:"chunk_index,omitempty"`

	TotalChunks int `json:"total_chunks,omitempty"`

	ChunkID string `json:"chunk_id,omitempty"`

	ParentID string `json:"parent_id,omitempty"`

	ChildID string `json:"child_id,omitempty"`

	ChildStartOffset int `json:"child_start_offset,omitempty"`

	ChildEndOffset int `json:"child_end_offset,omitempty"`

	ParentStartOffset int `json:"parent_start_offset,omitempty"`

	ParentEndOffset int `json:"parent_end_offset,omitempty"`

	SectionTitle string `json:"section_title,omitempty"`

	HierarchyPath string `json:"hierarchy_path,omitempty"`

	ParentTokenCount int `json:"parent_token_count,omitempty"`

	ParentBuildStrategy string `json:"parent_build_strategy,omitempty"`

	ParentBuildVersion string `json:"parent_build_version,omitempty"`

	ParentTruncated bool `json:"parent_truncated,omitempty"`

	ParentChildAvailable bool `json:"parent_child_available,omitempty"`

	SplitStrategy string `json:"split_strategy,omitempty"`

	SplitVersion string `json:"split_version,omitempty"`

	SplitStage string `json:"split_stage,omitempty"`

	SourceFileType string `json:"source_file_type,omitempty"`

	SemanticSplitEnabled bool `json:"semantic_split_enabled,omitempty"`

	SemanticSplitScore float64 `json:"semantic_split_score,omitempty"`

	SemanticParentSectionID string `json:"semantic_parent_section_id,omitempty"`

	EmbeddingBuildStrategy string `json:"embedding_build_strategy,omitempty"`

	ContextVersion string `json:"context_version,omitempty"`

	CreatedAt string `json:"created_at"`

	Extra map[string]interface{} `json:"extra,omitempty"`
}

// NewDocumentMetadata 创建新的文档元数据
func NewDocumentMetadata(filePath string, language DocumentLanguage, category DocumentCategory) *DocumentMetadata {
	fileName := filepath.Base(filePath)
	// 从文件名提取标题（去掉扩展名）
	title := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	return &DocumentMetadata{
		Language:               language,
		Category:               category,
		FilePath:               filePath,
		FileName:               fileName,
		Title:                  title,
		SplitStage:             chunkmeta.SplitStagePrimary,
		EmbeddingBuildStrategy: chunkmeta.EmbeddingBuildStrategyRaw,
		ContextVersion:         chunkmeta.ContextVersionRawContent,
		CreatedAt:              time.Now().Format(time.RFC3339),
		Extra:                  make(map[string]interface{}),
	}
}

func NewKBDocumentMetadata(tenantID uint64, operatorAdminID uint, kbID, documentID uint64, fileName string) *DocumentMetadata {
	return &DocumentMetadata{
		OperatorAdminID:        operatorAdminID,
		KBScope:                "global",
		TenantID:               tenantID,
		KBID:                   kbID,
		DocumentID:             documentID,
		FileName:               fileName,
		ChunkIndex:             0,
		TotalChunks:            0,
		SplitStage:             chunkmeta.SplitStagePrimary,
		EmbeddingBuildStrategy: chunkmeta.EmbeddingBuildStrategyRaw,
		ContextVersion:         chunkmeta.ContextVersionRawContent,
		CreatedAt:              time.Now().UTC().Format(time.RFC3339),
		Extra:                  make(map[string]interface{}),
	}
}

// ToMap 将 DocumentMetadata 转换为 map[string]interface{}，用于 schema.Document.MetaData
func (m *DocumentMetadata) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"language":   string(m.Language),
		"category":   string(m.Category),
		"file_path":  m.FilePath,
		"file_name":  m.FileName,
		"title":      m.Title,
		"created_at": m.CreatedAt,
	}

	if m.Source != "" {
		result["source"] = m.Source
	}

	if m.OperatorAdminID > 0 {
		result["operator_admin_id"] = m.OperatorAdminID
	}
	if m.KBScope != "" {
		result["kb_scope"] = m.KBScope
	}
	if m.TenantID > 0 {
		result["tenant_id"] = m.TenantID
	}

	if m.KBID > 0 {
		result["kb_id"] = m.KBID
	}

	if m.DocumentID > 0 {
		result["document_id"] = m.DocumentID
	}

	if m.ChunkIndex >= 0 {
		result["chunk_index"] = m.ChunkIndex
	}

	if m.TotalChunks > 0 {
		result["total_chunks"] = m.TotalChunks
	}
	if m.ChunkID != "" {
		result["chunk_id"] = m.ChunkID
	}
	if m.ChildID != "" {
		result["child_id"] = m.ChildID
	}
	if m.ChildID != "" || m.ChunkID != "" {
		result["child_start_offset"] = m.ChildStartOffset
		result["child_end_offset"] = m.ChildEndOffset
	}
	if m.ParentID != "" {
		result["parent_id"] = m.ParentID
		result["parent_start_offset"] = m.ParentStartOffset
		result["parent_end_offset"] = m.ParentEndOffset
		result["parent_token_count"] = m.ParentTokenCount
	}
	if m.SectionTitle != "" {
		result["section_title"] = m.SectionTitle
	}
	if m.HierarchyPath != "" {
		result["hierarchy_path"] = m.HierarchyPath
	}
	if m.ParentBuildStrategy != "" {
		result["parent_build_strategy"] = m.ParentBuildStrategy
	}
	if m.ParentBuildVersion != "" {
		result["parent_build_version"] = m.ParentBuildVersion
	}
	if m.ParentTruncated {
		result["parent_truncated"] = true
	}
	if m.ParentChildAvailable {
		result["parent_child_available"] = true
	}
	if m.SplitStrategy != "" {
		result[chunkmeta.KeySplitStrategy] = m.SplitStrategy
	}
	if m.SplitVersion != "" {
		result[chunkmeta.KeySplitVersion] = m.SplitVersion
	}
	if m.SplitStage != "" {
		result[chunkmeta.KeySplitStage] = m.SplitStage
	}
	if m.SourceFileType != "" {
		result[chunkmeta.KeySourceFileType] = m.SourceFileType
	}
	if m.SemanticSplitEnabled {
		result[chunkmeta.KeySemanticSplitEnabled] = true
	}
	if m.SemanticSplitScore != 0 {
		result[chunkmeta.KeySemanticSplitScore] = m.SemanticSplitScore
	}
	if m.SemanticParentSectionID != "" {
		result[chunkmeta.KeySemanticParentSection] = m.SemanticParentSectionID
	}
	if m.EmbeddingBuildStrategy != "" {
		result[chunkmeta.KeyEmbeddingBuildStrategy] = m.EmbeddingBuildStrategy
	}
	if m.ContextVersion != "" {
		result[chunkmeta.KeyContextVersion] = m.ContextVersion
	}

	for k, v := range m.Extra {
		result[k] = v
	}

	return result
}

// EnrichDocumentsWithMetadata 为文档块添加元数据
// 将原始文档的元数据复制到每个分割后的块中，并添加块索引信息
func EnrichDocumentsWithMetadata(chunks []*schema.Document, baseMetadata *DocumentMetadata) []*schema.Document {
	totalChunks := len(chunks)

	for i, chunk := range chunks {
		if chunk == nil {
			continue
		}
		// 创建块的元数据副本
		chunkMetadata := *baseMetadata
		chunkMetadata.ChunkIndex = i
		chunkMetadata.TotalChunks = totalChunks

		// 如果 chunk 已经有 MetaData，合并它们；否则创建新的
		if chunk.MetaData == nil {
			chunk.MetaData = make(map[string]interface{})
		}

		// 合并元数据
		baseMap := chunkMetadata.ToMap()
		for k, v := range baseMap {
			if _, exists := chunk.MetaData[k]; !exists {
				chunk.MetaData[k] = v
			}
		}
	}

	return chunks
}
