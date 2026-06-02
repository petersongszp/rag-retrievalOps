package auth

import (
	"log"
	"strings"

	"interview-agents/internal/config"
)

// BootstrapAdmin 创建或验证首个管理员用户
// L2 阶段仅做配置校验和日志输出，不实际创建用户（Phase 1 实现）
func BootstrapAdmin(cfg *config.Config) error {
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
	if len(password) < 12 {
		log.Println("[Bootstrap] Password too short (minimum 12 characters)")
		return nil
	}

	weakPasswords := []string{"admin", "admin123", "password", "123456", "Admin@123"}
	for _, weak := range weakPasswords {
		if strings.EqualFold(password, weak) {
			log.Println("[Bootstrap] Weak password rejected")
			return nil
		}
	}

	// TODO: Phase 1 实现时，这里会：
	// 1. 检查 rag_tenant 表是否存在
	// 2. 检查是否已有同邮箱用户
	// 3. 创建租户和用户
	// 4. 设置角色为 owner

	log.Printf("[Bootstrap] Config validated: email=%s, tenant=%s", email, tenantName)
	log.Println("[Bootstrap] Bootstrap user creation will be implemented in Phase 1")

	return nil
}
