package auth

import (
	"context"

	"interview-agents/api/response"
	authpkg "interview-agents/internal/auth"
	apperrors "interview-agents/internal/errors"
	"interview-agents/internal/quota"
	"interview-agents/internal/repository"

	"github.com/cloudwego/hertz/pkg/app"
	"gorm.io/gorm"
)

var tenantUsageRepo *repository.RAGTenantUsageRepository

func InitTenantHandler(db *gorm.DB) {
	tenantUsageRepo = repository.NewRAGTenantUsageRepository(db)
}

func GetTenant(ctx context.Context, c *app.RequestContext) {
	identity := authpkg.GetIdentity(ctx)
	if identity.TenantID == 0 {
		response.Error(ctx, c, 401, "Not authenticated")
		return
	}

	tenant, err := tenantRepo.GetByID(identity.TenantID)
	if err != nil {
		response.Error(ctx, c, 404, "Tenant not found")
		return
	}

	response.Success(ctx, c, authpkg.TenantResponse{
		TenantID:          tenant.ID,
		Name:              tenant.Name,
		Slug:              tenant.Slug,
		Plan:              tenant.Plan,
		Status:            tenant.Status,
		CreatedAt:         tenant.CreatedAt,
		UpdatedAt:         tenant.UpdatedAt,
		MaxKBCount:        tenant.MaxKBCount,
		MaxDocCount:       tenant.MaxDocCount,
		MaxStorageMB:      tenant.MaxStorageMB,
		MaxAPICallsPerDay: tenant.MaxAPICallsPerDay,
	})
}

func GetTenantUsage(ctx context.Context, c *app.RequestContext) {
	identity := authpkg.GetIdentity(ctx)
	if identity.TenantID == 0 {
		response.Error(ctx, c, 401, "Not authenticated")
		return
	}

	tenant, err := tenantRepo.GetByID(identity.TenantID)
	if err != nil {
		response.Error(ctx, c, 404, "Tenant not found")
		return
	}
	if tenantUsageRepo == nil {
		response.Error(ctx, c, 500, "Tenant usage repository not initialized")
		return
	}

	kbCount, err := tenantUsageRepo.CountKnowledgeBases(identity.TenantID)
	if err != nil {
		response.ErrorFromErr(ctx, c, apperrors.NewDBError("failed to count tenant knowledge bases", err))
		return
	}

	docCount, err := tenantUsageRepo.CountDocuments(identity.TenantID)
	if err != nil {
		response.ErrorFromErr(ctx, c, apperrors.NewDBError("failed to count tenant documents", err))
		return
	}

	totalBytes, err := tenantUsageRepo.SumStorageBytes(identity.TenantID)
	if err != nil {
		response.ErrorFromErr(ctx, c, apperrors.NewDBError("failed to sum tenant storage usage", err))
		return
	}

	response.Success(ctx, c, authpkg.TenantUsageResponse{
		APICallsToday: quota.GetAPICallCount(identity.TenantID),
		KBCount:       int(kbCount),
		DocCount:      int(docCount),
		StorageMB:     storageBytesToMB(totalBytes),
		Limits: authpkg.TenantUsageLimits{
			MaxKBCount:        tenant.MaxKBCount,
			MaxDocCount:       tenant.MaxDocCount,
			MaxStorageMB:      tenant.MaxStorageMB,
			MaxAPICallsPerDay: tenant.MaxAPICallsPerDay,
		},
	})
}

func storageBytesToMB(totalBytes int64) int64 {
	if totalBytes <= 0 {
		return 0
	}

	const bytesPerMB int64 = 1024 * 1024
	return (totalBytes + bytesPerMB - 1) / bytesPerMB
}
