# Phase 4 L0 Contract Freeze

本文档冻结 RAG Admin Phase 4（P4）“企业治理、成本、审计、告警与周报”的管理台契约，作为 L1-L8 的唯一实现基线。

## 1. 目标与边界

P4 必须完成：

1. 统一成本、向量运维、审计、告警、周报、治理门禁的管理台 API 前缀。
2. 冻结 P4 的核心字段、事件分类、feature flags 和降级策略。
3. 页面遇到字段缺失、接口失败或治理模块异常时必须显式展示 contract gap 或局部降级。
4. P4 任一模块异常不得阻塞 P0-P3 主链路。

P4 明确不做：

1. 不在前端计算真实成本账单。
2. 不让前端直接执行底层 Milvus 命令。
3. 不把敏感 query、文档片段、未脱敏 before/after 原文直接暴露给管理台。
4. 不绕过现有 P3 策略中心与回滚体系。

## 2. 冻结的 API 前缀

P4 后端管理台接口统一挂在：

- `/api/admin/kb/cost/*`
- `/api/admin/kb/vector/*`
- `/api/admin/kb/audit/*`
- `/api/admin/kb/alerts/*`
- `/api/admin/kb/reports/*`
- `/api/admin/kb/governance/*`

兼容规则：

1. 现有 `/api/admin/kb/index-lifecycle/*` 与 `/api/admin/kb/weekly-report`、`/api/admin/kb/governance/gate` 在 P4 期间允许保留为兼容别名。
2. 新增前端页面与后续 L1-L8 contract 一律优先使用上述 canonical 前缀。

## 3. 冻结的 Phase 4 Feature Flags

P4 canonical flags 固定为：

- `RAG_ENABLE_COST_GOVERNANCE`
- `RAG_ENABLE_AUDIT_CENTER`
- `RAG_ENABLE_VECTOR_OPS`
- `RAG_ENABLE_GOVERNANCE_ALERTS`
- `RAG_ENABLE_WEEKLY_REPORT`

兼容映射：

1. `RAG_ENABLE_COST_GOVERNANCE` 兼容既有 `RAG_ENABLE_COST_DASHBOARD`
2. `RAG_ENABLE_AUDIT_CENTER` 兼容既有 `RAG_ENABLE_COMPLIANCE_AUDIT`
3. `RAG_ENABLE_VECTOR_OPS` 兼容既有 `RAG_ENABLE_INDEX_LIFECYCLE`、`RAG_ENABLE_MILVUS_OPS_TOOLING`、`RAG_ENABLE_COLLECTION_SWITCH_GUARD`
4. `RAG_ENABLE_GOVERNANCE_ALERTS` 兼容既有 `RAG_ENABLE_EXPERIMENT_PLATFORM`

## 4. 冻结的数据对象

### 4.1 Cost Summary

字段基线：

- `range`
- `total_estimated_cost`
- `currency`
- `cost_per_1k_queries`
- `embedding_cost`
- `llm_cost`
- `rerank_cost`
- `vector_storage_cost`
- `index_rebuild_cost`
- `avg_context_tokens`
- `avg_candidate_count`
- `high_cost_query_count`
- `contract_gaps`

### 4.2 Audit Event

字段基线：

- `id`
- `audit_trace_id`
- `request_id`
- `kb_id`
- `document_id`
- `action`
- `resource_type`
- `resource_id`
- `before`
- `after`
- `reason`
- `created_at`

详情扩展字段：

- `actor_name`
- `ip`
- `user_agent`
- `sensitive_fields_masked`
- `trace_id`

### 4.3 Governance Alert

分类枚举固定为：

- `quality`
- `stability`
- `cost`
- `capacity`
- `audit`

### 4.4 Governance Gate Summary

字段基线：

- `generated_at`
- `passed`
- `cost_guard_passed`
- `audit_guard_passed`
- `index_guard_passed`
- `experiment_guard_passed`
- `release_guard_passed`
- `collection_health_score`
- `audit_coverage_rate`
- `rollback_success_rate`
- `strategy_regression_rate`
- `cost_per_1k_queries`
- `risks`
- `contract_gaps`

## 5. 冻结的审计动作

P4 canonical action 固定为：

- `kb_create`
- `document_upload`
- `document_delete`
- `ingest_retry`
- `ingest_cancel`
- `retrieve_query`
- `trace_view`
- `eval_run_create`
- `report_export`
- `strategy_flag_update`
- `strategy_rollback`
- `collection_rebuild`
- `collection_switch`
- `collection_rollback`
- `alert_ack`
- `alert_resolve`
- `permission_change`

说明：

1. 历史记录可保留既有动作值。
2. 自 L1 起新增 P4 审计事件按上述 canonical action 写入。

## 6. 降级契约

P4 降级策略固定为：

1. 成本采集失败：记录告警或 contract gap，不影响检索回答。
2. 审计写入失败：进入补偿队列，不阻塞主链路。
3. 审计查询失败：只影响 `/audit` 页面。
4. 告警聚合失败：Dashboard 或 Alerts 页面显式展示模块异常。
5. 周报生成失败：保留最近成功版本并展示失败原因。
6. Collection 操作失败：不得修改 active 状态，并记录失败审计。

## 7. 非回退要求

1. P3 策略中心继续可用。
2. P3 Retrieval Debug 继续可用。
3. P2 Evaluation 页面继续可用。
4. P1 Trace Logs 继续可用。
5. P0 知识库管理闭环继续可用。
