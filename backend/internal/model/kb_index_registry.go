package model

import (
	"time"
)

var KBIndexRegistryDao _KBIndexRegistry

type CollectionRole string

const (
	CollectionRoleActive     CollectionRole = "active"
	CollectionRoleCandidate  CollectionRole = "candidate"
	CollectionRoleStandby    CollectionRole = "standby"
	CollectionRoleRollback   CollectionRole = "rollback"
	CollectionRoleDeprecated CollectionRole = "deprecated"
)

type IndexBuildStatus string

const (
	IndexBuildStatusPending    IndexBuildStatus = "pending"
	IndexBuildStatusBuilding   IndexBuildStatus = "building"
	IndexBuildStatusReady      IndexBuildStatus = "ready"
	IndexBuildStatusFailed     IndexBuildStatus = "failed"
	IndexBuildStatusSwitched   IndexBuildStatus = "switched"
	IndexBuildStatusRolledBack IndexBuildStatus = "rolled_back"
)

type (
	_KBIndexRegistry struct{}
	KBIndexRegistry  struct {
		ID                 uint64           `json:"id" gorm:"primaryKey;autoIncrement"`
		IndexVersion       string           `json:"index_version" gorm:"uniqueIndex;size:128;not null"`
		CollectionName     string           `json:"collection_name" gorm:"size:255;index;not null"`
		CollectionRole     CollectionRole   `json:"collection_role" gorm:"size:32;index;not null"`
		EmbeddingModel     string           `json:"embedding_model" gorm:"size:255"`
		EmbeddingDimension int              `json:"embedding_dimension"`
		MetricType         string           `json:"metric_type" gorm:"size:64"`
		IndexType          string           `json:"index_type" gorm:"size:64"`
		IndexParams        string           `json:"index_params" gorm:"type:text"`
		BuildStatus        IndexBuildStatus `json:"build_status" gorm:"size:32;index;not null"`
		BuildStartedAt     *time.Time       `json:"build_started_at"`
		BuildFinishedAt    *time.Time       `json:"build_finished_at"`
		CreatedBy          string           `json:"created_by" gorm:"size:128"`
		CreatedAt          time.Time        `json:"created_at" gorm:"autoCreateTime:milli;index"`
		UpdatedAt          time.Time        `json:"updated_at" gorm:"autoUpdateTime:milli"`
	}
)

func (KBIndexRegistry) TableName() string {
	return "kb_index_registry"
}

func (d *_KBIndexRegistry) Create(record *KBIndexRegistry) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().Create(record).Error
}

func (d *_KBIndexRegistry) Save(record *KBIndexRegistry) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().Save(record).Error
}

func (d *_KBIndexRegistry) List() ([]*KBIndexRegistry, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var records []*KBIndexRegistry
	err := getDB().Model(&KBIndexRegistry{}).Order("updated_at DESC").Find(&records).Error
	return records, err
}

func (d *_KBIndexRegistry) GetByVersion(indexVersion string) (*KBIndexRegistry, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var record KBIndexRegistry
	if err := getDB().Where("index_version = ?", indexVersion).First(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func (d *_KBIndexRegistry) GetByRole(role CollectionRole) (*KBIndexRegistry, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var record KBIndexRegistry
	if err := getDB().Where("collection_role = ?", role).Order("updated_at DESC").First(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}
