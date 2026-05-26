package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"interview-agents/internal/milvus/evaluation"

	"gorm.io/gorm"
)

type KBEvalCaseValidationStatus string

const (
	KBEvalCaseValidationStatusValid     KBEvalCaseValidationStatus = "valid"
	KBEvalCaseValidationStatusInvalid   KBEvalCaseValidationStatus = "invalid"
	KBEvalCaseValidationStatusUnchecked KBEvalCaseValidationStatus = "unchecked"
)

var KBEvalCaseDao _KBEvalCase

type StringList []string
type Uint64List []uint64
type CitationTargetList []evaluation.CitationTarget

type KBEvalCaseListFilter struct {
	DatasetID        uint64
	QueryType        *string
	Tag              *string
	ValidationStatus *KBEvalCaseValidationStatus
	Keyword          *string
	Page             int
	PageSize         int
}

type (
	_KBEvalCase struct{}
	KBEvalCase  struct {
		ID               uint64                     `json:"id" gorm:"primaryKey;autoIncrement"`
		DatasetID        uint64                     `json:"dataset_id" gorm:"uniqueIndex:idx_kb_eval_case_dataset_key;index;not null"`
		CaseKey          string                     `json:"case_key" gorm:"size:128;uniqueIndex:idx_kb_eval_case_dataset_key;not null"`
		Query            string                     `json:"query" gorm:"size:4000;not null"`
		TopK             int                        `json:"top_k" gorm:"not null"`
		RelevantIDs      StringList                 `json:"relevant_ids" gorm:"type:text"`
		CitationTargets  CitationTargetList         `json:"citation_targets" gorm:"type:text"`
		QueryType        string                     `json:"query_type" gorm:"size:64;index"`
		Tags             StringList                 `json:"tags" gorm:"type:text"`
		KBIDs            Uint64List                 `json:"kb_ids" gorm:"type:text"`
		Collection       string                     `json:"collection" gorm:"size:255"`
		Notes            string                     `json:"notes" gorm:"size:2000"`
		ValidationStatus KBEvalCaseValidationStatus `json:"validation_status" gorm:"size:20;not null;default:'unchecked';index"`
		ValidationErrors StringList                 `json:"validation_errors" gorm:"type:text"`
		CreatedAt        time.Time                  `json:"created_at" gorm:"autoCreateTime:milli"`
		UpdatedAt        time.Time                  `json:"updated_at" gorm:"autoUpdateTime:milli"`
	}
)

func (KBEvalCase) TableName() string {
	return "kb_eval_case"
}

func ParseKBEvalCaseValidationStatus(raw string) (KBEvalCaseValidationStatus, bool) {
	status := KBEvalCaseValidationStatus(strings.TrimSpace(raw))
	switch status {
	case KBEvalCaseValidationStatusValid,
		KBEvalCaseValidationStatusInvalid,
		KBEvalCaseValidationStatusUnchecked:
		return status, true
	default:
		return "", false
	}
}

func (d *_KBEvalCase) Create(evalCase *KBEvalCase) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	normalizeEvalCase(evalCase)
	return getDB().Create(evalCase).Error
}

func (d *_KBEvalCase) GetByID(id uint64) (*KBEvalCase, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var evalCase KBEvalCase
	if err := getDB().Where("id = ?", id).First(&evalCase).Error; err != nil {
		return nil, err
	}
	normalizeEvalCase(&evalCase)
	return &evalCase, nil
}

func (d *_KBEvalCase) ExistsByDatasetIDAndCaseKey(datasetID uint64, caseKey string) (bool, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var count int64
	if err := getDB().Model(&KBEvalCase{}).
		Where("dataset_id = ? AND case_key = ?", datasetID, caseKey).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (d *_KBEvalCase) CountByDatasetID(datasetID uint64) (int64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var count int64
	if err := getDB().Model(&KBEvalCase{}).Where("dataset_id = ?", datasetID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (d *_KBEvalCase) ListAllByDatasetID(datasetID uint64) ([]*KBEvalCase, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var list []*KBEvalCase
	if err := getDB().Where("dataset_id = ?", datasetID).
		Order("created_at ASC").
		Find(&list).Error; err != nil {
		return nil, err
	}
	for _, item := range list {
		normalizeEvalCase(item)
	}
	return list, nil
}

func (d *_KBEvalCase) ListWithFilter(filter KBEvalCaseListFilter) ([]*KBEvalCase, int64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}

	query := getDB().Model(&KBEvalCase{}).Where("dataset_id = ?", filter.DatasetID)
	if filter.QueryType != nil {
		query = query.Where("query_type = ?", strings.TrimSpace(*filter.QueryType))
	}
	if filter.ValidationStatus != nil {
		query = query.Where("validation_status = ?", *filter.ValidationStatus)
	}
	if filter.Keyword != nil {
		keyword := strings.TrimSpace(*filter.Keyword)
		if keyword != "" {
			query = query.Where(
				"case_key LIKE ? OR query LIKE ? OR notes LIKE ?",
				"%"+keyword+"%",
				"%"+keyword+"%",
				"%"+keyword+"%",
			)
		}
	}

	var list []*KBEvalCase
	if err := query.Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, 0, err
	}

	for _, item := range list {
		normalizeEvalCase(item)
	}

	if filter.Tag != nil {
		tag := strings.TrimSpace(*filter.Tag)
		if tag != "" {
			filtered := make([]*KBEvalCase, 0, len(list))
			for _, item := range list {
				if item.Tags.Contains(tag) {
					filtered = append(filtered, item)
				}
			}
			list = filtered
		}
	}

	total := int64(len(list))
	start := (filter.Page - 1) * filter.PageSize
	if start >= len(list) {
		return []*KBEvalCase{}, total, nil
	}
	end := start + filter.PageSize
	if end > len(list) {
		end = len(list)
	}
	return list[start:end], total, nil
}

func (d *_KBEvalCase) UpdateValidation(datasetID uint64, caseID uint64, status KBEvalCaseValidationStatus, errors StringList) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	if errors == nil {
		errors = StringList{}
	}
	return getDB().Model(&KBEvalCase{}).
		Where("dataset_id = ? AND id = ?", datasetID, caseID).
		Updates(map[string]interface{}{
			"validation_status": status,
			"validation_errors": errors,
		}).Error
}

func (d *_KBEvalCase) ToDatasetCase(evalCase *KBEvalCase) evaluation.DatasetCase {
	normalizeEvalCase(evalCase)
	return evaluation.DatasetCase{
		ID:              evalCase.CaseKey,
		Query:           evalCase.Query,
		TopK:            evalCase.TopK,
		RelevantIDs:     []string(evalCase.RelevantIDs),
		CitationTargets: []evaluation.CitationTarget(evalCase.CitationTargets),
		QueryType:       evalCase.QueryType,
		Tags:            []string(evalCase.Tags),
		KBIDs:           []uint64(evalCase.KBIDs),
		Collection:      evalCase.Collection,
		Notes:           evalCase.Notes,
	}
}

func (s StringList) Value() (driver.Value, error) {
	return marshalJSONValue([]string(s))
}

func (s *StringList) Scan(value interface{}) error {
	var items []string
	if err := scanJSONValue(value, &items); err != nil {
		return err
	}
	*s = StringList(items)
	return nil
}

func (s StringList) Contains(target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, item := range s {
		if strings.EqualFold(strings.TrimSpace(item), target) {
			return true
		}
	}
	return false
}

func (u Uint64List) Value() (driver.Value, error) {
	return marshalJSONValue([]uint64(u))
}

func (u *Uint64List) Scan(value interface{}) error {
	var items []uint64
	if err := scanJSONValue(value, &items); err != nil {
		return err
	}
	*u = Uint64List(items)
	return nil
}

func (u Uint64List) Contains(target uint64) bool {
	for _, item := range u {
		if item == target {
			return true
		}
	}
	return false
}

func (c CitationTargetList) Value() (driver.Value, error) {
	return marshalJSONValue([]evaluation.CitationTarget(c))
}

func (c *CitationTargetList) Scan(value interface{}) error {
	var items []evaluation.CitationTarget
	if err := scanJSONValue(value, &items); err != nil {
		return err
	}
	*c = CitationTargetList(items)
	return nil
}

func marshalJSONValue(value interface{}) (driver.Value, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

func scanJSONValue(value interface{}, target interface{}) error {
	switch v := value.(type) {
	case nil:
		return json.Unmarshal([]byte("[]"), target)
	case []byte:
		if len(v) == 0 {
			return json.Unmarshal([]byte("[]"), target)
		}
		return json.Unmarshal(v, target)
	case string:
		if strings.TrimSpace(v) == "" {
			return json.Unmarshal([]byte("[]"), target)
		}
		return json.Unmarshal([]byte(v), target)
	default:
		return fmt.Errorf("unsupported JSON storage type %T", value)
	}
}

func normalizeEvalCase(evalCase *KBEvalCase) {
	if evalCase == nil {
		return
	}
	if evalCase.RelevantIDs == nil {
		evalCase.RelevantIDs = StringList{}
	}
	if evalCase.CitationTargets == nil {
		evalCase.CitationTargets = CitationTargetList{}
	}
	if evalCase.Tags == nil {
		evalCase.Tags = StringList{}
	}
	if evalCase.KBIDs == nil {
		evalCase.KBIDs = Uint64List{}
	}
	if evalCase.ValidationErrors == nil {
		evalCase.ValidationErrors = StringList{}
	}
}

func applyEvalCasePagination(list []*KBEvalCase, page, pageSize int) ([]*KBEvalCase, int64) {
	total := int64(len(list))
	start := (page - 1) * pageSize
	if start >= len(list) {
		return []*KBEvalCase{}, total
	}
	end := start + pageSize
	if end > len(list) {
		end = len(list)
	}
	return list[start:end], total
}

func listEvalCasesByDataset(tx *gorm.DB, datasetID uint64) ([]*KBEvalCase, error) {
	var list []*KBEvalCase
	if err := tx.Where("dataset_id = ?", datasetID).Order("created_at ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	for _, item := range list {
		normalizeEvalCase(item)
	}
	return list, nil
}
