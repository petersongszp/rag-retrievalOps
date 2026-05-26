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

  LIST_INGEST_LOGS: `${API_BASE_URL}/admin/kb/logs/ingest`,
  GET_INGEST_LOG_DETAIL: (jobId: number | string) =>
    `${API_BASE_URL}/admin/kb/logs/ingest/${jobId}`,
};
