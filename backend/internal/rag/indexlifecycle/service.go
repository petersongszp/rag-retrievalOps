package indexlifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	milvusClient "github.com/milvus-io/milvus-sdk-go/v2/client"

	"interview-agents/internal/config"
	"interview-agents/internal/milvus"
	"interview-agents/internal/milvus/benchmark"
	"interview-agents/internal/model"
)

type HealthReport struct {
	IndexVersion      string    `json:"index_version"`
	CollectionName    string    `json:"collection_name"`
	CollectionRole    string    `json:"collection_role"`
	CollectionExists  bool      `json:"collection_exists"`
	DimensionMatch    bool      `json:"dimension_match"`
	MetricTypeMatch   bool      `json:"metric_type_match"`
	LoadHealthy       bool      `json:"load_healthy"`
	QuerySmokeHealthy bool      `json:"query_smoke_healthy"`
	CheckedAt         time.Time `json:"checked_at"`
	Message           string    `json:"message,omitempty"`
}

type SwitchResult struct {
	ActiveIndexVersion   string    `json:"active_index_version"`
	PreviousIndexVersion string    `json:"previous_index_version,omitempty"`
	SwitchedAt           time.Time `json:"switched_at"`
}

func ListRegistry() ([]*model.KBIndexRegistry, error) {
	return model.KBIndexRegistryDao.List()
}

func RegisterFromConfig(cfg *config.Config, createdBy string) (*model.KBIndexRegistry, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	now := time.Now().UTC()
	record := &model.KBIndexRegistry{
		IndexVersion:       fmt.Sprintf("idx-%d", now.Unix()),
		CollectionName:     cfg.Milvus.GetCollection("knowledge"),
		CollectionRole:     model.CollectionRoleActive,
		EmbeddingModel:     cfg.Embedding.Model,
		EmbeddingDimension: cfg.Embedding.Dimensions,
		MetricType:         cfg.Milvus.MetricType,
		IndexType:          "AUTOINDEX",
		IndexParams:        "{}",
		BuildStatus:        model.IndexBuildStatusReady,
		BuildStartedAt:     &now,
		BuildFinishedAt:    &now,
		CreatedBy:          createdBy,
	}
	if record.CollectionName == "" {
		record.CollectionName = cfg.Milvus.CollectionName
	}
	if err := model.KBIndexRegistryDao.Create(record); err != nil {
		return nil, err
	}
	_ = model.KBIndexOperationLogDao.Create(&model.KBIndexOperationLog{
		IndexVersion:    record.IndexVersion,
		CollectionName:  record.CollectionName,
		Operation:       "register",
		ToRole:          string(record.CollectionRole),
		OperationReason: "register_from_config",
	})
	return record, nil
}

func BuildCandidate(ctx context.Context, cfg *config.Config, indexVersion string, profile benchmark.IndexProfile, operatorID uint, reason string) (*model.KBIndexRegistry, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	manager, err := milvus.GetMilvusManager()
	if err != nil {
		return nil, err
	}
	collectionName := cfg.Milvus.GetCollection("knowledge")
	if collectionName == "" {
		collectionName = cfg.Milvus.CollectionName
	}
	now := time.Now().UTC()
	record := &model.KBIndexRegistry{
		IndexVersion:       indexVersion,
		CollectionName:     collectionName,
		CollectionRole:     model.CollectionRoleCandidate,
		EmbeddingModel:     cfg.Embedding.Model,
		EmbeddingDimension: cfg.Embedding.Dimensions,
		MetricType:         cfg.Milvus.MetricType,
		IndexType:          string(profile.Family),
		BuildStatus:        model.IndexBuildStatusBuilding,
		BuildStartedAt:     &now,
		CreatedBy:          fmt.Sprintf("user:%d", operatorID),
	}
	if params, err := json.Marshal(profile); err == nil {
		record.IndexParams = string(params)
	}
	if err := model.KBIndexRegistryDao.Create(record); err != nil {
		return nil, err
	}
	_ = model.KBIndexOperationLogDao.Create(&model.KBIndexOperationLog{
		IndexVersion:    record.IndexVersion,
		CollectionName:  record.CollectionName,
		Operation:       "build_candidate",
		ToRole:          string(record.CollectionRole),
		OperatorID:      operatorID,
		OperationReason: reason,
	})

	harness := &benchmark.MilvusProfileHarness{
		Client:        manager.GetClient(),
		Collection:    collectionName,
		BaseRetriever: manager.GetRetrieverService().GetConfig(),
	}
	if err := harness.ApplyProfile(ctx, profile); err != nil {
		record.BuildStatus = model.IndexBuildStatusFailed
		_ = model.KBIndexRegistryDao.Save(record)
		return nil, err
	}
	finishedAt := time.Now().UTC()
	record.BuildStatus = model.IndexBuildStatusReady
	record.BuildFinishedAt = &finishedAt
	if err := model.KBIndexRegistryDao.Save(record); err != nil {
		return nil, err
	}
	return record, nil
}

func HealthCheck(ctx context.Context, cfg *config.Config, indexVersion string) (HealthReport, error) {
	record, err := model.KBIndexRegistryDao.GetByVersion(indexVersion)
	if err != nil {
		return HealthReport{}, err
	}
	manager, err := milvus.GetMilvusManager()
	if err != nil {
		return HealthReport{}, err
	}
	report := HealthReport{
		IndexVersion:    record.IndexVersion,
		CollectionName:  record.CollectionName,
		CollectionRole:  string(record.CollectionRole),
		DimensionMatch:  record.EmbeddingDimension == cfg.Embedding.Dimensions,
		MetricTypeMatch: record.MetricType == cfg.Milvus.MetricType,
		CheckedAt:       time.Now().UTC(),
	}
	report.CollectionExists, report.LoadHealthy, report.QuerySmokeHealthy, report.Message = inspectCollection(ctx, manager.GetClient(), record.CollectionName)
	if report.CollectionExists && report.DimensionMatch && report.MetricTypeMatch && report.LoadHealthy {
		report.QuerySmokeHealthy = true
	}
	_ = model.KBIndexOperationLogDao.Create(&model.KBIndexOperationLog{
		IndexVersion:    record.IndexVersion,
		CollectionName:  record.CollectionName,
		Operation:       "health_check",
		ToRole:          string(record.CollectionRole),
		HealthStatus:    report.Message,
		OperationReason: "manual_health_check",
	})
	return report, nil
}

func SwitchActive(ctx context.Context, cfg *config.Config, indexVersion string, operatorID uint, reason string) (SwitchResult, error) {
	if !cfg.RAG.FeatureFlags.EnableCollectionSwitchGuard {
		return SwitchResult{}, fmt.Errorf("collection switch guard is disabled")
	}
	record, err := model.KBIndexRegistryDao.GetByVersion(indexVersion)
	if err != nil {
		return SwitchResult{}, err
	}
	active, _ := model.KBIndexRegistryDao.GetByRole(model.CollectionRoleActive)
	if active != nil && active.IndexVersion == record.IndexVersion {
		return SwitchResult{ActiveIndexVersion: record.IndexVersion, PreviousIndexVersion: record.IndexVersion, SwitchedAt: time.Now().UTC()}, nil
	}
	if active != nil {
		active.CollectionRole = model.CollectionRoleRollback
		active.BuildStatus = model.IndexBuildStatusRolledBack
		_ = model.KBIndexRegistryDao.Save(active)
	}
	record.CollectionRole = model.CollectionRoleActive
	record.BuildStatus = model.IndexBuildStatusSwitched
	if err := model.KBIndexRegistryDao.Save(record); err != nil {
		return SwitchResult{}, err
	}
	cfg.Milvus.Collections["knowledge"] = record.CollectionName
	if err := milvus.ReconfigureGlobalManager(ctx, cfg); err != nil {
		return SwitchResult{}, err
	}
	_ = model.KBIndexOperationLogDao.Create(&model.KBIndexOperationLog{
		IndexVersion:    record.IndexVersion,
		CollectionName:  record.CollectionName,
		Operation:       "switch_active",
		FromRole:        roleString(active),
		ToRole:          string(model.CollectionRoleActive),
		OperatorID:      operatorID,
		OperationReason: reason,
		RollbackTarget:  fallbackIndexVersion(active),
	})
	return SwitchResult{
		ActiveIndexVersion:   record.IndexVersion,
		PreviousIndexVersion: fallbackIndexVersion(active),
		SwitchedAt:           time.Now().UTC(),
	}, nil
}

func RollbackActive(ctx context.Context, cfg *config.Config, operatorID uint, reason string) (SwitchResult, error) {
	target, err := model.KBIndexRegistryDao.GetByRole(model.CollectionRoleRollback)
	if err != nil {
		return SwitchResult{}, err
	}
	return SwitchActive(ctx, cfg, target.IndexVersion, operatorID, reason)
}

func ListOperations(limit int) ([]*model.KBIndexOperationLog, error) {
	return model.KBIndexOperationLogDao.List(limit)
}

func inspectCollection(ctx context.Context, client milvusClient.Client, collection string) (bool, bool, bool, string) {
	if client == nil {
		return false, false, false, "client_nil"
	}
	collections, err := client.ListCollections(ctx)
	if err != nil {
		return false, false, false, err.Error()
	}
	exists := false
	for _, item := range collections {
		if item != nil && item.Name == collection {
			exists = true
			break
		}
	}
	if !exists {
		return false, false, false, "collection_not_found"
	}
	return true, true, true, "healthy"
}

func roleString(record *model.KBIndexRegistry) string {
	if record == nil {
		return ""
	}
	return string(record.CollectionRole)
}

func fallbackIndexVersion(record *model.KBIndexRegistry) string {
	if record == nil {
		return ""
	}
	return record.IndexVersion
}
