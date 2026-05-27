package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"interview-agents/internal/milvus/evaluation"
)

type KBEvalRunStatus string

const (
	KBEvalRunStatusPending   KBEvalRunStatus = "pending"
	KBEvalRunStatusRunning   KBEvalRunStatus = "running"
	KBEvalRunStatusSucceeded KBEvalRunStatus = "succeeded"
	KBEvalRunStatusFailed    KBEvalRunStatus = "failed"
	KBEvalRunStatusCanceled  KBEvalRunStatus = "canceled"
)

var KBEvalRunDao _KBEvalRun

type EvalStrategyProfileList []evaluation.StrategyProfile
type EvalGateThresholds evaluation.GateThresholds

type KBEvalRunListFilter struct {
	DatasetID *uint64
	Status    *KBEvalRunStatus
	Page      int
	PageSize  int
}

type (
	_KBEvalRun struct{}
	KBEvalRun  struct {
		ID               uint64                  `json:"id" gorm:"primaryKey;autoIncrement"`
		RunID            string                  `json:"run_id" gorm:"size:64;uniqueIndex;not null"`
		DatasetID        uint64                  `json:"dataset_id" gorm:"index;not null"`
		BaselineProfile  string                  `json:"baseline_profile" gorm:"size:128;not null"`
		CandidateProfile string                  `json:"candidate_profile" gorm:"size:128;not null"`
		Profiles         EvalStrategyProfileList `json:"profiles,omitempty" gorm:"type:text"`
		GateThresholds   EvalGateThresholds      `json:"gate_thresholds" gorm:"type:text"`
		Status           KBEvalRunStatus         `json:"status" gorm:"size:20;not null;default:'pending';index"`
		Progress         int                     `json:"progress" gorm:"default:0"`
		CaseTotal        int                     `json:"case_total" gorm:"default:0"`
		CaseFinished     int                     `json:"case_finished" gorm:"default:0"`
		ReportPath       string                  `json:"report_path" gorm:"size:1000"`
		ErrorMsg         string                  `json:"error_msg" gorm:"size:2000"`
		CreatedBy        uint                    `json:"created_by" gorm:"index;not null"`
		StartedAt        *time.Time              `json:"started_at"`
		FinishedAt       *time.Time              `json:"finished_at"`
		CreatedAt        time.Time               `json:"created_at" gorm:"autoCreateTime:milli"`
		UpdatedAt        time.Time               `json:"updated_at" gorm:"autoUpdateTime:milli"`
	}
)

func (KBEvalRun) TableName() string {
	return "kb_eval_run"
}

func ParseKBEvalRunStatus(raw string) (KBEvalRunStatus, bool) {
	status := KBEvalRunStatus(strings.TrimSpace(raw))
	switch status {
	case KBEvalRunStatusPending,
		KBEvalRunStatusRunning,
		KBEvalRunStatusSucceeded,
		KBEvalRunStatusFailed,
		KBEvalRunStatusCanceled:
		return status, true
	default:
		return "", false
	}
}

func (d *_KBEvalRun) Create(run *KBEvalRun) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	return getDB().Create(run).Error
}

func (d *_KBEvalRun) GetByRunID(runID string) (*KBEvalRun, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	var run KBEvalRun
	if err := getDB().Where("run_id = ?", strings.TrimSpace(runID)).First(&run).Error; err != nil {
		return nil, err
	}
	return &run, nil
}

func (d *_KBEvalRun) ListWithFilter(filter KBEvalRunListFilter) ([]*KBEvalRun, int64, error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}

	var list []*KBEvalRun
	var total int64

	query := getDB().Model(&KBEvalRun{})
	if filter.DatasetID != nil {
		query = query.Where("dataset_id = ?", *filter.DatasetID)
	}
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Offset((filter.Page - 1) * filter.PageSize).
		Limit(filter.PageSize).
		Order("created_at DESC").
		Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (d *_KBEvalRun) UpdateByRunID(runID string, updates map[string]interface{}) error {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}
	if len(updates) == 0 {
		return nil
	}
	return getDB().Model(&KBEvalRun{}).Where("run_id = ?", strings.TrimSpace(runID)).Updates(updates).Error
}

func (p EvalStrategyProfileList) Value() (driver.Value, error) {
	data, err := json.Marshal([]evaluation.StrategyProfile(p))
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

func (p *EvalStrategyProfileList) Scan(value interface{}) error {
	var items []evaluation.StrategyProfile
	if err := scanJSONValue(value, &items); err != nil {
		return err
	}
	*p = EvalStrategyProfileList(items)
	return nil
}

func (g EvalGateThresholds) Value() (driver.Value, error) {
	data, err := json.Marshal(evaluation.GateThresholds(g))
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

func (g *EvalGateThresholds) Scan(value interface{}) error {
	var thresholds evaluation.GateThresholds
	switch v := value.(type) {
	case nil:
		*g = EvalGateThresholds(evaluation.GateThresholds{})
		return nil
	case []byte:
		if len(v) == 0 {
			*g = EvalGateThresholds(evaluation.GateThresholds{})
			return nil
		}
		if err := json.Unmarshal(v, &thresholds); err != nil {
			return err
		}
	case string:
		if strings.TrimSpace(v) == "" {
			*g = EvalGateThresholds(evaluation.GateThresholds{})
			return nil
		}
		if err := json.Unmarshal([]byte(v), &thresholds); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported gate thresholds storage type %T", value)
	}
	*g = EvalGateThresholds(thresholds)
	return nil
}
