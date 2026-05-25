# Phase 0 L0 Contract Freeze

This document freezes the current admin frontend baseline before the route/layout refactor.

## Current Functional Surface

Source of truth: `admin/src/app/page.tsx` before the L1 split.

Implemented user-visible actions:

1. List knowledge bases
2. Create a knowledge base
3. Upload documents into the current knowledge base
4. List documents for the current knowledge base
5. Delete a document
6. List ingest jobs
7. Retry a failed ingest job
8. Cancel an ingest job
9. Run a retrieval test for the current knowledge base

## Frozen P0 API Inventory

Source of truth: `admin/src/config/api.ts` and `admin/src/services/api/client.ts`.

1. `GET /api/admin/kb/bases`
2. `POST /api/admin/kb/bases`
3. `POST /api/admin/kb/documents/upload`
4. `GET /api/admin/kb/documents?kb_id={kbId}`
5. `DELETE /api/admin/kb/documents/{id}`
6. `GET /api/admin/kb/jobs`
7. `POST /api/admin/kb/jobs/{id}/retry`
8. `POST /api/admin/kb/jobs/{id}/cancel`
9. `POST /api/admin/kb/retrieve`

Transport and response assumptions:

1. The axios client unwraps `{ code: 200, data, message }` responses.
2. Multipart upload must omit the explicit `Content-Type` header so the browser can add the boundary.
3. Existing pages expect `items` arrays from list endpoints.

## Frozen Type Baseline

Source of truth: `admin/src/types/kb.ts`.

Stable entities already used by the UI:

1. `KnowledgeBase`
2. `KBDocument`
3. `KBIngestJob`
4. `RetrieveResponse`
5. `RetrieveItem`

## Contract Gaps To Keep Visible

These fields are required or expected by the detailed roadmap, but are not fully protected in the current UI baseline:

1. `RetrieveResponse.request_id`
2. `RetrieveItem.score`
3. `RetrieveItem.citation.file_name`
4. `RetrieveItem.citation.chunk_index`
5. `RetrieveItem.citation.chunk_id`
6. `RetrieveItem.source.route`
7. `RetrieveItem.source.collection`
8. `RetrieveItem.source.retriever_version`

Known extension fields expected later and still optional from the backend side:

1. Document: `last_ingest_job_id`, `chunk_count`, `file_hash`, `ingest_duration_ms`
2. Job: `stage`, `progress`, `retry_count`, `error_code`, `error_msg`, `started_at`, `finished_at`

Rule for Phase 0 onward:

Missing contract fields must be shown as explicit contract gaps in the UI instead of being silently fabricated.

## Non-Regression Behaviors

The L1 split must preserve these flows:

1. After upload succeeds, the document list and ingest job list can be refreshed.
2. After document deletion succeeds, the current document list updates.
3. Failed jobs can still be retried.
4. Incomplete jobs can still be canceled.
5. Retrieval test results can still be rendered for the current knowledge base.

## Smoke Baseline For L8

1. Visit `/dashboard`
2. Visit `/knowledge-bases`
3. Create a knowledge base
4. Visit `/knowledge-bases/{kbId}`
5. Upload a valid document
6. See the document in the document list
7. See the corresponding ingest job
8. Retry a failed job when available
9. Cancel a running job when available
10. Delete a document
11. Visit `/retrieval-lab`
12. Run a retrieval test
13. Confirm retrieval results render without a page crash
