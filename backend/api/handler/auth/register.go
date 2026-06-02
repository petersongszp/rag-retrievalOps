package auth

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"interview-agents/api/response"
	authpkg "interview-agents/internal/auth"
	"interview-agents/internal/config"
	"interview-agents/internal/model"
	"interview-agents/internal/repository"

	"github.com/cloudwego/hertz/pkg/app"
	"gorm.io/gorm"
)

var (
	tenantRepo *repository.RAGTenantRepository
	userRepo   *repository.RAGUserRepository
	jwtManager *authpkg.JWTManager
)

// InitAuthHandler 初始化 auth handler 依赖
func InitAuthHandler(db *gorm.DB, cfg *config.Config) {
	tenantRepo = repository.NewRAGTenantRepository(db)
	userRepo = repository.NewRAGUserRepository(db)

	jwtManager = authpkg.NewJWTManager(authpkg.JWTConfig{
		Secret:     cfg.RAG.Auth.JWTSecret,
		AccessTTL:  cfg.RAG.Auth.GetAccessTokenTTL(),
		RefreshTTL: cfg.RAG.Auth.GetRefreshTokenTTL(),
	})
}

// Register 用户注册
func Register(ctx context.Context, c *app.RequestContext) {
	var req authpkg.RegisterRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.BadRequest(ctx, c, "Invalid request: "+err.Error())
		return
	}

	// 校验密码强度
	if err := authpkg.ValidatePasswordStrength(req.Password); err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}

	// 检查邮箱是否已存在
	existingUser, err := userRepo.GetByEmail(req.Email)
	if err == nil && existingUser != nil && existingUser.ID > 0 {
		response.Error(ctx, c, 409, "Email already registered")
		return
	}

	// 哈希密码
	passwordHash, err := authpkg.HashPassword(req.Password)
	if err != nil {
		log.Printf("[Auth] Hash password failed: %v", err)
		response.InternalServerError(ctx, c, "Failed to process password")
		return
	}

	// 生成租户 slug
	tenantName := strings.TrimSpace(req.TenantName)
	if tenantName == "" {
		tenantName = req.Name + "'s Workspace"
	}
	slug := generateSlug(tenantName) + "-" + fmt.Sprintf("%d", time.Now().UnixMilli()%100000)

	// 创建租户
	tenant := &model.RAGTenant{
		Name:   tenantName,
		Slug:   slug,
		Plan:   "free",
		Status: "active",
	}
	if err := tenantRepo.Create(tenant); err != nil {
		log.Printf("[Auth] Create tenant failed: %v", err)
		response.InternalServerError(ctx, c, "Failed to create tenant")
		return
	}

	// 创建用户（owner）
	user := &model.RAGUser{
		TenantID:     tenant.ID,
		Email:        req.Email,
		PasswordHash: passwordHash,
		Name:         req.Name,
		Role:         "owner",
		Status:       "active",
	}
	if err := userRepo.Create(user); err != nil {
		log.Printf("[Auth] Create user failed: %v", err)
		response.InternalServerError(ctx, c, "Failed to create user")
		return
	}

	log.Printf("[Auth] User registered: email=%s, tenant_id=%d, user_id=%d", req.Email, tenant.ID, user.ID)

	response.Success(ctx, c, authpkg.RegisterResponse{
		UserID:   uint(user.ID),
		Email:    user.Email,
		TenantID: tenant.ID,
	})
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
	if slug == "" {
		slug = "tenant"
	}
	return slug
}
