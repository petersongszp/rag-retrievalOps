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
	Language DocumentLanguage `json:"language"`

	Category DocumentCategory `json:"category"`

	FilePath string `json:"file_path"`

	FileName string `json:"file_name"`

	Title string `json:"title"`

	Source string `json:"source,omitempty"`

	UserID uint `json:"user_id"`

	KBID uint64 `json:"kb_id"`

	DocumentID uint64 `json:"document_id"`

	ChunkIndex int `json:"chunk_index,omitempty"`

	TotalChunks int `json:"total_chunks,omitempty"`

	CreatedAt string `json:"created_at"`

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

func NewKBDocumentMetadata(userID uint, kbID, documentID uint64, fileName string) *DocumentMetadata {
	return &DocumentMetadata{
		UserID:      userID,
		KBID:        kbID,
		DocumentID:  documentID,
		FileName:    fileName,
		ChunkIndex:  0,
		TotalChunks: 0,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		Extra:       make(map[string]interface{}),
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

	if m.UserID > 0 {
		result["user_id"] = m.UserID
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
