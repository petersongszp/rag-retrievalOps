# Phase 3 L0 隔离合约冻结记录

## 冻结日期
2026-06-03

## 隔离字段清单

| 字段 | 类型 | 说明 |
|------|------|------|
| tenant_id | uint64 | 租户 ID（第一隔离维度） |
| kb_id | uint64 | 知识库 ID |
| document_id | uint64 | 文档 ID |
| api_key_id | uint64 | API Key ID |
| app_id | string | 应用标识 |
| auth_type | string | 认证类型 |
| source_api | string | 来源 API |
| is_legacy | bool | 是否旧版 |

## 需要改造的数据表

- kb_knowledge_base
- kb_document
- kb_ingest_job
- kb_job_operation_log
- kb_audit_event
- kb_retrieve_log
- rag_tenant_kb_permission（新增）

## 需要强过滤的接口

- 知识库 CRUD
- 文档 CRUD
- 入库任务查询
- 检索日志和审计日志
- /v1/retrieve

## 身份来源优先级

1. JWT（Admin/UI）
2. API Key（Agent/SDK）
3. legacy app_id（兼容路径）
4. dev bypass（仅 dev/test 环境）

## 风险清单

- 只按 user_id 过滤导致跨租户误读
- legacy app_id 未映射租户
- Milvus 检索未加 tenant_id 过滤
