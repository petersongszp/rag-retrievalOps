ALTER TABLE kb_audit_event
    DROP INDEX idx_kb_audit_event_tenant_id,
    DROP COLUMN tenant_id;

ALTER TABLE kb_job_operation_log
    DROP INDEX idx_kb_job_operation_log_tenant_id,
    DROP COLUMN tenant_id;

ALTER TABLE kb_ingest_job
    DROP INDEX idx_kb_ingest_job_tenant_id,
    DROP COLUMN tenant_id;

ALTER TABLE kb_document
    DROP INDEX idx_kb_document_tenant_id,
    DROP COLUMN tenant_id;

ALTER TABLE kb_knowledge_base
    DROP INDEX idx_kb_knowledge_base_tenant_id,
    DROP COLUMN tenant_id;
