package model

import (
	"fmt"
	"time"
)

type KBDocumentStatus string

const (
	KBDocumentStatusPending    KBDocumentStatus = "pending"
	KBDocumentStatusProcessing KBDocumentStatus = "processing"
	KBDocumentStatusCompleted  KBDocumentStatus = "completed"
	KBDocumentStatusFailed     KBDocumentStatus = "failed"
)

var KBDocumentDao _KBDocument

type (
	_KBDocument struct{}
	KBDocument  struct {
		ID          uint64           `json:"id" gorm:"primaryKey;autoIncrement"`
		TenantID    uint64           `json:"tenant_id" gorm:"index"`
		KbID        uint64           `json:"kb_id" gorm:"index;not null"`
		UserID      uint             `json:"user_id" gorm:"index;not null"`
		FileName    string           `json:"file_name" gorm:"size:255;not null"`
		FileType    string           `json:"file_type" gorm:"size:50;not null"`
		FileSize    int64            `json:"file_size" gorm:"not null"`
		FileHash    string           `json:"file_hash" gorm:"index;size:64;not null"`
		StoragePath string           `json:"storage_path" gorm:"size:500;not null"`
		Status      KBDocumentStatus `json:"status" gorm:"size:20;not null;default:'pending';index"`
		ChunkCount  int              `json:"chunk_count" gorm:"default:0"`
		ErrorMsg    string           `json:"error_msg" gorm:"size:500"`
		Deleted     int              `json:"deleted" gorm:"default:0;index"`
		CreatedAt   time.Time        `json:"created_at" gorm:"autoCreateTime:milli"`
		UpdatedAt   time.Time        `json:"updated_at" gorm:"autoUpdateTime:milli"`
		// 以下字段由关联查询填充，不存储在 kb_document 表中
		LastIngestJobID  *uint64 `json:"last_ingest_job_id,omitempty" gorm:"-"`
		IngestDurationMs *int64  `json:"ingest_duration_ms,omitempty" gorm:"-"`
	}
)

func (KBDocument) TableName() string {
	return "kb_document"
}

func (d *_KBDocument) Create(doc *KBDocument) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().Create(doc).Error
}

func (d *_KBDocument) GetByID(id uint64) (*KBDocument, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var doc KBDocument
	err := getDB().Where("id = ? AND deleted = 0", id).First(&doc).Error
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (d *_KBDocument) ListByKbID(kbID uint64, page, pageSize int) ([]*KBDocument, int64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var list []*KBDocument
	var total int64

	query := getDB().Model(&KBDocument{}).Where("kb_id = ? AND deleted = 0", kbID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Offset((page - 1) * pageSize).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&list).Error
	if err != nil {
		return nil, 0, err
	}

	if err := enrichDocumentsWithIngestInfo(list); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (d *_KBDocument) ListByUserID(userID uint, page, pageSize int) ([]*KBDocument, int64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var list []*KBDocument
	var total int64

	query := getDB().Model(&KBDocument{}).Where("user_id = ? AND deleted = 0", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Offset((page - 1) * pageSize).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&list).Error
	if err != nil {
		return nil, 0, err
	}

	if err := enrichDocumentsWithIngestInfo(list); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (d *_KBDocument) GetByFileHash(kbID uint64, fileHash string) (*KBDocument, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var doc KBDocument
	err := getDB().Where("kb_id = ? AND file_hash = ? AND deleted = 0", kbID, fileHash).First(&doc).Error
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (d *_KBDocument) UpdateStatus(id uint64, status KBDocumentStatus, errorMsg string) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	updates := map[string]interface{}{
		"status": status,
	}
	if errorMsg != "" {
		updates["error_msg"] = errorMsg
	}
	tx := getDB().Model(&KBDocument{}).Where("id = ? AND deleted = 0", id).Updates(updates)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return fmt.Errorf("kb_document not found or not updated: id=%d", id)
	}
	return nil
}

func (d *_KBDocument) UpdateChunkCount(id uint64, chunkCount int) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().Model(&KBDocument{}).Where("id = ? AND deleted = 0", id).
		Update("chunk_count", chunkCount).Error
}

func (d *_KBDocument) SoftDelete(id uint64) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().Model(&KBDocument{}).Where("id = ?", id).
		Update("deleted", 1).Error
}

func (d *_KBDocument) CountNonDeleted() (int64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var count int64
	err := getDB().Model(&KBDocument{}).Where("deleted = 0").Count(&count).Error
	return count, err
}

// enrichDocumentsWithIngestInfo 批量查询每个文档的最新 ingest job，
// 填充 LastIngestJobID 和 IngestDurationMs（finished_at - started_at，单位毫秒）。
func enrichDocumentsWithIngestInfo(docs []*KBDocument) error {
	if len(docs) == 0 {
		return nil
	}

	ids := make([]uint64, len(docs))
	for i, doc := range docs {
		ids[i] = doc.ID
	}

	type jobRow struct {
		DocumentID uint64
		JobID      uint64
		StartedAt  *time.Time
		FinishedAt *time.Time
	}

	// 每个 document_id 取 created_at 最新的一条 job
	var rows []jobRow
	err := getDB().
		Raw(`SELECT j.document_id, j.id AS job_id, j.started_at, j.finished_at
		     FROM kb_ingest_job j
		     INNER JOIN (
		         SELECT document_id, MAX(created_at) AS max_created_at
		         FROM kb_ingest_job
		         WHERE document_id IN ?
		         GROUP BY document_id
		     ) latest ON j.document_id = latest.document_id AND j.created_at = latest.max_created_at`, ids).
		Scan(&rows).Error
	if err != nil {
		return err
	}

	jobMap := make(map[uint64]*jobRow, len(rows))
	for i := range rows {
		jobMap[rows[i].DocumentID] = &rows[i]
	}

	for _, doc := range docs {
		row, ok := jobMap[doc.ID]
		if !ok {
			continue
		}
		doc.LastIngestJobID = &row.JobID
		if row.StartedAt != nil && row.FinishedAt != nil {
			ms := row.FinishedAt.Sub(*row.StartedAt).Milliseconds()
			doc.IngestDurationMs = &ms
		}
	}
	return nil
}
