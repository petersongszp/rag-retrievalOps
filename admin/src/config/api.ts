// API 配置文件
export const API_BASE_URL = process.env.NEXT_PUBLIC_API_BASE_URL || '/api';

// 知识库管理 API（管理员端）
export const KB_ADMIN_API = {
  // Dashboard
  DASHBOARD_STATS: `${API_BASE_URL}/admin/kb/dashboard/stats`,

  // 知识库
  CREATE_BASE: `${API_BASE_URL}/admin/kb/bases`,
  LIST_BASES: `${API_BASE_URL}/admin/kb/bases`,

  // 文档
  UPLOAD_DOCUMENT: `${API_BASE_URL}/admin/kb/documents/upload`,
  LIST_DOCUMENTS: `${API_BASE_URL}/admin/kb/documents`,
  DELETE_DOCUMENT: (id: number | string) => `${API_BASE_URL}/admin/kb/documents/${id}`,

  // 任务
  GET_JOB: (id: number | string) => `${API_BASE_URL}/admin/kb/jobs/${id}`,
  LIST_JOBS: `${API_BASE_URL}/admin/kb/jobs`,
  LIST_JOBS_BY_KB: (kbId: number | string) => `${API_BASE_URL}/admin/kb/jobs?kb_id=${kbId}`,
  RETRY_JOB: (id: number | string) => `${API_BASE_URL}/admin/kb/jobs/${id}/retry`,
  CANCEL_JOB: (id: number | string) => `${API_BASE_URL}/admin/kb/jobs/${id}/cancel`,

  // 检索
  RETRIEVE: `${API_BASE_URL}/admin/kb/retrieve`,
};
