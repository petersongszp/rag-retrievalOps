-- Phase 1: 创建租户表
CREATE TABLE IF NOT EXISTS rag_tenant (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(128) NOT NULL COMMENT '租户名称',
    slug VARCHAR(64) NOT NULL COMMENT '租户标识',
    plan VARCHAR(32) DEFAULT 'free' COMMENT '套餐',
    status VARCHAR(16) DEFAULT 'active' COMMENT '状态',
    max_kb_count INT DEFAULT 5 COMMENT '最大知识库数',
    max_doc_count INT DEFAULT 100 COMMENT '最大文档数',
    max_storage_mb INT DEFAULT 1024 COMMENT '最大存储MB',
    max_api_calls_per_day INT DEFAULT 10000 COMMENT '每日API调用上限',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_slug (slug),
    KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
