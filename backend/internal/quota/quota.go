package quota

import (
	"fmt"

	apperrors "interview-agents/internal/errors"
	"interview-agents/internal/model"

	"gorm.io/gorm"
)

const bytesPerMB int64 = 1024 * 1024

type QuotaChecker struct {
	db *gorm.DB
}

type tenantQuotaLimits struct {
	MaxKBCount        int
	MaxDocCount       int
	MaxStorageMB      int
	MaxAPICallsPerDay int
}

func NewQuotaChecker(db *gorm.DB) *QuotaChecker {
	return &QuotaChecker{db: db}
}

func (q *QuotaChecker) CheckKBLimit(tenantID uint64) error {
	if err := q.validate(); err != nil {
		return err
	}

	limits, err := q.loadTenantLimits(tenantID)
	if err != nil {
		return err
	}

	var current int64
	if err := q.db.Model(&model.KBKnowledgeBase{}).
		Where("tenant_id = ?", tenantID).
		Count(&current).Error; err != nil {
		return apperrors.NewDBError("failed to count tenant knowledge bases", err)
	}

	if current >= int64(limits.MaxKBCount) {
		return apperrors.NewTooManyRequestsError(
			fmt.Sprintf("knowledge base quota exceeded: current=%d limit=%d", current, limits.MaxKBCount),
		)
	}

	return nil
}

func (q *QuotaChecker) CheckDocLimit(tenantID uint64, count int) error {
	if err := q.validate(); err != nil {
		return err
	}
	if count < 0 {
		return apperrors.NewValidationError("count must be greater than or equal to 0")
	}

	limits, err := q.loadTenantLimits(tenantID)
	if err != nil {
		return err
	}

	var current int64
	if err := q.db.Model(&model.KBDocument{}).
		Where("tenant_id = ? AND deleted = 0", tenantID).
		Count(&current).Error; err != nil {
		return apperrors.NewDBError("failed to count tenant documents", err)
	}

	if current+int64(count) > int64(limits.MaxDocCount) {
		return apperrors.NewTooManyRequestsError(
			fmt.Sprintf("document quota exceeded: current=%d requested=%d limit=%d", current, count, limits.MaxDocCount),
		)
	}

	return nil
}

func (q *QuotaChecker) CheckStorageLimit(tenantID uint64, sizeMB int) error {
	if err := q.validate(); err != nil {
		return err
	}
	if sizeMB < 0 {
		return apperrors.NewValidationError("sizeMB must be greater than or equal to 0")
	}

	limits, err := q.loadTenantLimits(tenantID)
	if err != nil {
		return err
	}

	var totalBytes int64
	if err := q.db.Model(&model.KBDocument{}).
		Where("tenant_id = ? AND deleted = 0", tenantID).
		Select("COALESCE(SUM(file_size), 0)").
		Scan(&totalBytes).Error; err != nil {
		return apperrors.NewDBError("failed to sum tenant storage usage", err)
	}

	requestedBytes := int64(sizeMB) * bytesPerMB
	limitBytes := int64(limits.MaxStorageMB) * bytesPerMB
	if totalBytes+requestedBytes > limitBytes {
		return apperrors.NewTooManyRequestsError(
			fmt.Sprintf("storage quota exceeded: current_mb=%d requested_mb=%d limit_mb=%d", bytesToMB(totalBytes), sizeMB, limits.MaxStorageMB),
		)
	}

	return nil
}

func (q *QuotaChecker) CheckAPICallLimit(tenantID uint64) error {
	if err := q.validate(); err != nil {
		return err
	}

	limits, err := q.loadTenantLimits(tenantID)
	if err != nil {
		return err
	}

	current := GetAPICallCount(tenantID)
	if current >= limits.MaxAPICallsPerDay {
		return apperrors.NewTooManyRequestsError(
			fmt.Sprintf("api call quota exceeded: current=%d limit=%d", current, limits.MaxAPICallsPerDay),
		)
	}

	return nil
}

func (q *QuotaChecker) validate() error {
	if q == nil || q.db == nil {
		return apperrors.NewInternalError("quota checker database is nil", nil)
	}
	return nil
}

func (q *QuotaChecker) loadTenantLimits(tenantID uint64) (*tenantQuotaLimits, error) {
	if tenantID == 0 {
		return nil, apperrors.NewValidationError("tenant_id must be greater than 0")
	}

	var tenant model.RAGTenant
	if err := q.db.Select("max_kb_count", "max_doc_count", "max_storage_mb", "max_api_calls_per_day").
		Where("id = ?", tenantID).
		First(&tenant).Error; err != nil {
		return nil, apperrors.NewDBError("failed to load tenant quota limits", err)
	}

	return &tenantQuotaLimits{
		MaxKBCount:        tenant.MaxKBCount,
		MaxDocCount:       tenant.MaxDocCount,
		MaxStorageMB:      tenant.MaxStorageMB,
		MaxAPICallsPerDay: tenant.MaxAPICallsPerDay,
	}, nil
}

func bytesToMB(totalBytes int64) int64 {
	if totalBytes <= 0 {
		return 0
	}
	return (totalBytes + bytesPerMB - 1) / bytesPerMB
}
