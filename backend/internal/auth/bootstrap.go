package auth

import (
	"log"
	"strings"

	"interview-agents/internal/config"
	"interview-agents/internal/model"
	"interview-agents/internal/repository"

	"gorm.io/gorm"
)

// BootstrapAdmin 创建或验证首个管理员用户
func BootstrapAdmin(cfg *config.Config, db *gorm.DB) error {
	if !cfg.RAG.Auth.BootstrapEnabled {
		log.Println("[Bootstrap] Bootstrap disabled, skip")
		return nil
	}

	// 检查环境
	if cfg.RAG.Environment == "prod" {
		log.Println("[Bootstrap] Bootstrap disabled in production")
		return nil
	}

	// 校验配置
	email := strings.TrimSpace(cfg.RAG.Auth.BootstrapAdminEmail)
	password := cfg.RAG.Auth.BootstrapAdminPassword
	name := strings.TrimSpace(cfg.RAG.Auth.BootstrapAdminName)
	tenantName := strings.TrimSpace(cfg.RAG.Auth.BootstrapTenantName)

	if email == "" || password == "" || name == "" {
		log.Println("[Bootstrap] Missing required bootstrap config (email/password/name)")
		return nil
	}

	// 校验密码强度
	if err := ValidatePasswordStrength(password); err != nil {
		log.Printf("[Bootstrap] Password validation failed: %v", err)
		return nil
	}

	// 初始化 repo
	tenantRepo := repository.NewRAGTenantRepository(db)
	userRepo := repository.NewRAGUserRepository(db)

	// 检查是否已有同邮箱用户
	existingUser, _ := userRepo.GetByEmail(email)
	if existingUser != nil {
		log.Printf("[Bootstrap] User already exists: email=%s, user_id=%d, tenant_id=%d",
			email, existingUser.ID, existingUser.TenantID)
		return nil
	}

	// 生成 slug
	slug := generateSlug(tenantName)
	if slug == "" {
		slug = "default"
	}

	// 创建租户
	tenant := &model.RAGTenant{
		Name:   tenantName,
		Slug:   slug,
		Plan:   "free",
		Status: "active",
	}
	if err := tenantRepo.Create(tenant); err != nil {
		log.Printf("[Bootstrap] Create tenant failed: %v", err)
		return err
	}

	// 哈希密码
	passwordHash, err := HashPassword(password)
	if err != nil {
		log.Printf("[Bootstrap] Hash password failed: %v", err)
		return err
	}

	// 创建用户（owner）
	user := &model.RAGUser{
		TenantID:     tenant.ID,
		Email:        email,
		PasswordHash: passwordHash,
		Name:         name,
		Role:         RoleOwner,
		Status:       "active",
	}
	if err := userRepo.Create(user); err != nil {
		log.Printf("[Bootstrap] Create user failed: %v", err)
		return err
	}

	log.Printf("[Bootstrap] Admin created: email=%s, user_id=%d, tenant_id=%d, tenant=%s",
		email, user.ID, tenant.ID, tenantName)

	return nil
}

// generateSlug 生成租户标识
func generateSlug(name string) string {
	slug := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	slug = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return -1
	}, slug)
	if len(slug) > 64 {
		slug = slug[:64]
	}
	return slug
}
