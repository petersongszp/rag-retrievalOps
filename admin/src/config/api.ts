export const API_BASE_URL = process.env.NEXT_PUBLIC_API_BASE_URL || '/api';

export const KB_ADMIN_API = {
  DASHBOARD_STATS: `${API_BASE_URL}/admin/kb/dashboard/stats`,
  METRICS_OVERVIEW: `${API_BASE_URL}/admin/kb/metrics/overview`,

  CREATE_BASE: `${API_BASE_URL}/admin/kb/bases`,
  LIST_BASES: `${API_BASE_URL}/admin/kb/bases`,

  UPLOAD_DOCUMENT: `${API_BASE_URL}/admin/kb/documents/upload`,
  LIST_DOCUMENTS: `${API_BASE_URL}/admin/kb/documents`,
  DELETE_DOCUMENT: (id: number | string) => `${API_BASE_URL}/admin/kb/documents/${id}`,

  GET_JOB: (id: number | string) => `${API_BASE_URL}/admin/kb/jobs/${id}`,
  LIST_JOBS: `${API_BASE_URL}/admin/kb/jobs`,
  LIST_JOBS_BY_KB: (kbId: number | string) => `${API_BASE_URL}/admin/kb/jobs?kb_id=${kbId}`,
  RETRY_JOB: (id: number | string) => `${API_BASE_URL}/admin/kb/jobs/${id}/retry`,
  CANCEL_JOB: (id: number | string) => `${API_BASE_URL}/admin/kb/jobs/${id}/cancel`,

  RETRIEVE: `${API_BASE_URL}/admin/kb/retrieve`,
  LIST_RETRIEVE_AUDIT_LOGS: `${API_BASE_URL}/admin/kb/retrieve/audit`,
  GET_RETRIEVE_AUDIT_LOG: (requestId: string) =>
    `${API_BASE_URL}/admin/kb/retrieve/audit/${requestId}`,
  GET_RETRIEVE_DEBUG_TRACE: (requestId: string) =>
    `${API_BASE_URL}/admin/kb/retrieve/audit/${requestId}/debug`,

  LIST_INGEST_LOGS: `${API_BASE_URL}/admin/kb/logs/ingest`,
  GET_INGEST_LOG_DETAIL: (jobId: number | string) =>
    `${API_BASE_URL}/admin/kb/logs/ingest/${jobId}`,

  LIST_EVAL_DATASETS: `${API_BASE_URL}/admin/kb/eval/datasets`,
  CREATE_EVAL_DATASET: `${API_BASE_URL}/admin/kb/eval/datasets`,
  LIST_EVAL_CASES: (datasetId: number | string) =>
    `${API_BASE_URL}/admin/kb/eval/datasets/${datasetId}/items`,
  CREATE_EVAL_CASE: (datasetId: number | string) =>
    `${API_BASE_URL}/admin/kb/eval/datasets/${datasetId}/items`,
  IMPORT_EVAL_CASES: (datasetId: number | string) =>
    `${API_BASE_URL}/admin/kb/eval/datasets/${datasetId}/items/import`,
  EXPORT_EVAL_CASES: (datasetId: number | string) =>
    `${API_BASE_URL}/admin/kb/eval/datasets/${datasetId}/items/export`,
  VALIDATE_EVAL_DATASET: (datasetId: number | string) =>
    `${API_BASE_URL}/admin/kb/eval/datasets/${datasetId}/validate`,

  LIST_EVAL_RUNS: `${API_BASE_URL}/admin/kb/eval/runs`,
  CREATE_EVAL_RUN: `${API_BASE_URL}/admin/kb/eval/runs`,
  GET_EVAL_RUN: (runId: string) => `${API_BASE_URL}/admin/kb/eval/runs/${runId}`,
  GET_EVAL_REPORT: (runId: string) => `${API_BASE_URL}/admin/kb/eval/runs/${runId}/report`,
  LIST_EVAL_FAILURE_CASES: (runId: string) =>
    `${API_BASE_URL}/admin/kb/eval/runs/${runId}/cases`,
  EXPORT_EVAL_REPORT: (runId: string, format: 'json' | 'markdown') =>
    `${API_BASE_URL}/admin/kb/eval/runs/${runId}/report/export?format=${format}`,

  LIST_STRATEGY_FLAGS: `${API_BASE_URL}/admin/kb/strategy/flags`,
  UPDATE_STRATEGY_FLAG: (flagKey: string) =>
    `${API_BASE_URL}/admin/kb/strategy/flags/${flagKey}`,
  LIST_STRATEGY_VERSIONS: `${API_BASE_URL}/admin/kb/strategy/versions`,
  GET_STRATEGY_VERSION: (versionId: string) =>
    `${API_BASE_URL}/admin/kb/strategy/versions/${versionId}`,
  ROLLBACK_STRATEGY: `${API_BASE_URL}/admin/kb/strategy/rollback`,
  GET_STRATEGY_IMPACT: `${API_BASE_URL}/admin/kb/strategy/impact`,
  GET_STRATEGY_GATES: `${API_BASE_URL}/admin/kb/strategy/gates`,
  LIST_STRATEGY_OPERATIONS: `${API_BASE_URL}/admin/kb/strategy/operations`,

  LIST_EXPERIMENTS: `${API_BASE_URL}/admin/kb/experiments`,
  SAVE_EXPERIMENT: `${API_BASE_URL}/admin/kb/experiments`,
  ROLLBACK_EXPERIMENT: (experimentId: string) =>
    `${API_BASE_URL}/admin/kb/experiments/${experimentId}/rollback`,
  GET_EXPERIMENT_SUMMARY: `${API_BASE_URL}/admin/kb/experiments/summary`,

  LIST_INDEX_REGISTRY: `${API_BASE_URL}/admin/kb/index-lifecycle/registry`,
  REGISTER_INDEX_FROM_CONFIG: `${API_BASE_URL}/admin/kb/index-lifecycle/register`,
  BUILD_CANDIDATE_INDEX: `${API_BASE_URL}/admin/kb/index-lifecycle/build`,
  GET_INDEX_HEALTH: (indexVersion: string) =>
    `${API_BASE_URL}/admin/kb/index-lifecycle/health/${indexVersion}`,
  SWITCH_ACTIVE_INDEX: (indexVersion: string) =>
    `${API_BASE_URL}/admin/kb/index-lifecycle/switch/${indexVersion}`,
  ROLLBACK_ACTIVE_INDEX: `${API_BASE_URL}/admin/kb/index-lifecycle/rollback`,
  LIST_INDEX_OPERATIONS: `${API_BASE_URL}/admin/kb/index-lifecycle/operations`,
  LIST_AUDIT_EVENTS: `${API_BASE_URL}/admin/kb/audit/events`,
};
