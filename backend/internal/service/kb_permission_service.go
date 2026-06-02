package service

import (
	stderrors "errors"
	"fmt"
	"strings"

	apperrors "interview-agents/internal/errors"
	"interview-agents/internal/model"
	"interview-agents/internal/repository"

	"gorm.io/gorm"
)

type KBPermissionService struct {
	repo *repository.RAGTenantKBPermissionRepository
}

func NewKBPermissionService(repo *repository.RAGTenantKBPermissionRepository) *KBPermissionService {
	return &KBPermissionService{repo: repo}
}

func (s *KBPermissionService) GrantPermission(tenantID, kbID uint64, permission string) (*model.RAGTenantKBPermission, error) {
	if err := s.validateIDs(tenantID, kbID); err != nil {
		return nil, err
	}
	normalized, err := normalizeKBPermission(permission)
	if err != nil {
		return nil, err
	}
	if err := s.ensureRepo(); err != nil {
		return nil, err
	}

	existing, err := s.repo.GetByTenantAndKB(tenantID, kbID)
	switch {
	case err == nil:
		if existing.Permission == normalized {
			return existing, nil
		}
		existing.Permission = normalized
		if err := s.repo.Update(existing); err != nil {
			return nil, apperrors.NewDBError("failed to update kb permission", err)
		}
		return existing, nil
	case stderrors.Is(err, gorm.ErrRecordNotFound):
		record := &model.RAGTenantKBPermission{
			TenantID:   tenantID,
			KBID:       kbID,
			Permission: normalized,
		}
		if err := s.repo.Create(record); err != nil {
			return nil, apperrors.NewDBError("failed to create kb permission", err)
		}
		return record, nil
	default:
		return nil, apperrors.NewDBError("failed to query kb permission", err)
	}
}

func (s *KBPermissionService) RevokePermission(tenantID, kbID uint64) error {
	if err := s.validateIDs(tenantID, kbID); err != nil {
		return err
	}
	if err := s.ensureRepo(); err != nil {
		return err
	}
	if err := s.repo.Delete(tenantID, kbID); err != nil {
		return apperrors.NewDBError("failed to revoke kb permission", err)
	}
	return nil
}

func (s *KBPermissionService) CheckPermission(tenantID, kbID uint64, required string) (bool, error) {
	if err := s.validateIDs(tenantID, kbID); err != nil {
		return false, err
	}
	requiredPermission, err := normalizeKBPermission(required)
	if err != nil {
		return false, err
	}
	if err := s.ensureRepo(); err != nil {
		return false, err
	}

	record, err := s.repo.GetByTenantAndKB(tenantID, kbID)
	if err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, apperrors.NewDBError("failed to query kb permission", err)
	}

	grantedPermission, err := normalizeKBPermission(record.Permission)
	if err != nil {
		return false, apperrors.NewInternalError(
			fmt.Sprintf("invalid stored kb permission for tenant_id=%d kb_id=%d", tenantID, kbID),
			err,
		)
	}

	return kbPermissionRank(grantedPermission) >= kbPermissionRank(requiredPermission), nil
}

func (s *KBPermissionService) ListByTenant(tenantID uint64) ([]*model.RAGTenantKBPermission, error) {
	if tenantID == 0 {
		return nil, apperrors.NewValidationError("tenant_id must be greater than 0")
	}
	if err := s.ensureRepo(); err != nil {
		return nil, err
	}
	permissions, err := s.repo.ListByTenantID(tenantID)
	if err != nil {
		return nil, apperrors.NewDBError("failed to list kb permissions by tenant", err)
	}
	return permissions, nil
}

func (s *KBPermissionService) ListByKB(kbID uint64) ([]*model.RAGTenantKBPermission, error) {
	if kbID == 0 {
		return nil, apperrors.NewValidationError("kb_id must be greater than 0")
	}
	if err := s.ensureRepo(); err != nil {
		return nil, err
	}
	permissions, err := s.repo.ListByKBID(kbID)
	if err != nil {
		return nil, apperrors.NewDBError("failed to list kb permissions by knowledge base", err)
	}
	return permissions, nil
}

func (s *KBPermissionService) ensureRepo() error {
	if s == nil || s.repo == nil {
		return apperrors.NewInternalError("kb permission repository is nil", nil)
	}
	return nil
}

func (s *KBPermissionService) validateIDs(tenantID, kbID uint64) error {
	if tenantID == 0 {
		return apperrors.NewValidationError("tenant_id must be greater than 0")
	}
	if kbID == 0 {
		return apperrors.NewValidationError("kb_id must be greater than 0")
	}
	return nil
}

func normalizeKBPermission(permission string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(permission))
	switch normalized {
	case model.RAGTenantKBPermissionRead, model.RAGTenantKBPermissionWrite, model.RAGTenantKBPermissionAdmin:
		return normalized, nil
	default:
		return "", apperrors.NewValidationError("permission must be one of: read, write, admin")
	}
}

func kbPermissionRank(permission string) int {
	switch permission {
	case model.RAGTenantKBPermissionRead:
		return 1
	case model.RAGTenantKBPermissionWrite:
		return 2
	case model.RAGTenantKBPermissionAdmin:
		return 3
	default:
		return 0
	}
}
