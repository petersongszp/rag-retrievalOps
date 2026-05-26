# Phase 1 L0 Contract Freeze

This document freezes the Phase 1 admin contract before implementation starts.

## Scope

Phase 1 covers:

1. Dashboard metrics overview
2. Retrieval trace log list and detail
3. Ingest log list and detail
4. Retrieval lab trace jump-through
5. Navigation activation for `/trace-logs` and placeholder activation for `/quality-monitor`

Phase 1 does not cover:

1. Prometheus or Grafana integration
2. Realtime push via WebSocket or SSE
3. Real content for `/quality-monitor`

## Frozen Backend Entities

### `KBRetrieveLog`

Source of truth: `backend/internal/model/kb_retrieve_log.go`

P1 depends on these fields staying stable:

1. `request_id`
2. `user_id`
3. `kb_ids`
4. `query`
5. `final_query`
6. `expr`
7. `top_k`
8. `candidate_topk`
9. `final_topk`
10. `token_budget`
11. `truncate_reason`
12. `rewrite`
13. `rewrite_strategy`
14. `rewrite_applied`
15. `strategy`
16. `release_stage`
17. `release_reason`
18. `routes`
19. `collection`
20. `retriever_version`
21. `empty_reason`
22. `final_count`
23. `truncated_count`
24. `dense_hits`
25. `sparse_hits`
26. `dense_contribution`
27. `sparse_contribution`
28. `result_status`
29. `error_code`
30. `error_msg`
31. `embedding_ms`
32. `search_ms`
33. `postprocess_ms`
34. `rerank_ms`
35. `rerank_model`
36. `duration_ms`
37. `timeout_ms`
38. `created_at`

### `KBJobOperationLog`

Source of truth: `backend/internal/model/kb_job_operation_log.go`

P1 depends on these fields staying stable:

1. `id`
2. `job_id`
3. `operator_id`
4. `operation`
5. `operation_reason`
6. `from_status`
7. `to_status`
8. `created_at`

### `KBIngestJob`

Source of truth: `backend/internal/model/kb_ingest_job.go`

P1 depends on these fields being available in ingest log list/detail views:

1. `id`
2. `kb_id`
3. `document_id`
4. `user_id`
5. `status`
6. `retry_count`
7. `error_msg`
8. `last_error_code`
9. `last_error_detail`
10. `operator_id`
11. `operation`
12. `operation_reason`
13. `operated_at`
14. `started_at`
15. `finished_at`
16. `created_at`
17. `updated_at`

## Frozen API Additions

### Retrieval audit list

`GET /api/admin/kb/retrieve/audit`

Supported filters for P1:

1. `kb_id`
2. `result_status`
3. `start_time`
4. `end_time`
5. `query_keyword`
6. `request_id`
7. `page`
8. `page_size`

### Metrics overview

`GET /api/admin/kb/metrics/overview`

Supported query params:

1. `kb_id`
2. `range` with allowed values `1h`, `24h`, `7d`

Frozen response shape:

1. `range`
2. `ingest_success_rate`
3. `retrieve_request_count`
4. `retrieve_p95_ms`
5. `retrieve_empty_rate`
6. `error_type_topn`

### Ingest logs

1. `GET /api/admin/kb/logs/ingest`
2. `GET /api/admin/kb/logs/ingest/:job_id`

## Frontend Contract Rules

1. Missing contract fields must be shown explicitly instead of silently hidden.
2. Monitoring pages must show readable error states instead of blank screens.
3. Type definitions in `admin/src/types/kb.ts` must mirror backend response fields.
4. `/trace-logs` becomes clickable in P1.
5. `/quality-monitor` becomes clickable but remains a placeholder page in P1.

## P1 Non-Negotiable Behaviors

1. Retrieval logs can be filtered by `request_id`.
2. Retrieval logs can be filtered by knowledge base, status, and time range.
3. Retrieval trace detail shows stage durations.
4. Ingest log detail shows operation audit history.
5. Retrieval lab can jump to `/trace-logs/retrieval?request_id=...`.
