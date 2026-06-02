ALTER TABLE kb_knowledge_base
    ADD COLUMN tenant_id BIGINT UNSIGNED DEFAULT 0,
    ADD INDEX idx_kb_knowledge_base_tenant_id (tenant_id);

ALTER TABLE kb_document
    ADD COLUMN tenant_id BIGINT UNSIGNED DEFAULT 0,
    ADD INDEX idx_kb_document_tenant_id (tenant_id);

ALTER TABLE kb_ingest_job
    ADD COLUMN tenant_id BIGINT UNSIGNED DEFAULT 0,
    ADD INDEX idx_kb_ingest_job_tenant_id (tenant_id);

ALTER TABLE kb_job_operation_log
    ADD COLUMN tenant_id BIGINT UNSIGNED DEFAULT 0,
    ADD INDEX idx_kb_job_operation_log_tenant_id (tenant_id);

ALTER TABLE kb_audit_event
    ADD COLUMN tenant_id BIGINT UNSIGNED DEFAULT 0,
    ADD INDEX idx_kb_audit_event_tenant_id (tenant_id);
