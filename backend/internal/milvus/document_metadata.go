package milvus

import (
	"path/filepath"
	"strings"
	"time"

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
	// 语言类型：golang, java, 中间件
	Language DocumentLanguage `json:"language"`

	// 文档分类
	Category DocumentCategory `json:"category"`

	// 文件路径（原始文件路径）
	FilePath string `json:"file_path"`

	// 文件名
	FileName string `json:"file_name"`

	// 文档标题（从 Markdown 中提取或文件名）
	Title string `json:"title"`

	// 来源（如：官方文档、教程、博客等）
	Source string `json:"source,omitempty"`

	// 块索引（如果文档被分割，这是第几个块，从 0 开始）
	ChunkIndex int `json:"chunk_index,omitempty"`

	// 总块数（如果文档被分割）
	TotalChunks int `json:"total_chunks,omitempty"`

	// 创建时间
	CreatedAt string `json:"created_at"`

	// 额外的自定义字段
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// NewDocumentMetadata 创建新的文档元数据
func NewDocumentMetadata(filePath string, language DocumentLanguage, category DocumentCategory) *DocumentMetadata {
	fileName := filepath.Base(filePath)
	// 从文件名提取标题（去掉扩展名）
	title := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	return &DocumentMetadata{
		Language:  language,
		Category:  category,
		FilePath:  filePath,
		FileName:  fileName,
		Title:     title,
		CreatedAt: time.Now().Format(time.RFC3339),
		Extra:     make(map[string]interface{}),
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

	if m.ChunkIndex >= 0 {
		result["chunk_index"] = m.ChunkIndex
	}

	if m.TotalChunks > 0 {
		result["total_chunks"] = m.TotalChunks
	}

	// 合并额外的字段
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
			chunk.MetaData[k] = v
		}
	}

	return chunks
}
