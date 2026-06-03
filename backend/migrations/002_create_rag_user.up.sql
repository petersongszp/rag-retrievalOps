-- Phase 1: 创建用户表
CREATE TABLE IF NOT EXISTS rag_user (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '所属租户',
    email VARCHAR(255) NOT NULL COMMENT '邮箱',
    password_hash VARCHAR(255) NOT NULL COMMENT '密码哈希',
    name VARCHAR(128) NOT NULL COMMENT '用户名',
    role VARCHAR(32) DEFAULT 'member' COMMENT '角色',
    status VARCHAR(16) DEFAULT 'active' COMMENT '状态',
    last_login_at TIMESTAMP NULL COMMENT '最后登录时间',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_email (email),
    KEY idx_tenant_id (tenant_id),
    KEY idx_status (status),
    KEY idx_role (role),
    CONSTRAINT fk_user_tenant FOREIGN KEY (tenant_id) REFERENCES rag_tenant(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
