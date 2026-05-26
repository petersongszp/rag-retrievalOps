export interface KnowledgeBase {
  id: number;
  name: string;
  description?: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface KBDocument {
  id: number;
  kb_id: number;
  file_name: string;
  file_type: string;
  file_size: number;
  file_hash: string;
  storage_path: string;
  status: 'pending' | 'processing' | 'completed' | 'failed';
  chunk_count: number;
  error_msg?: string;
  deleted: number;
  last_ingest_job_id?: number;
  ingest_duration_ms?: number;
  created_at: string;
  updated_at: string;
}

export interface KBIngestJob {
  id: number;
  kb_id: number;
  document_id: number;
  user_id: number;
  status: 'pending' | 'processing' | 'completed' | 'failed' | 'retrying' | 'dead' | 'canceled';
  retry_count: number;
  error_msg?: string;
  last_error_code?: string;
  last_error_detail?: string;
  operation?: string;
  operation_reason?: string;
  operated_at?: string;
  started_at?: string;
  finished_at?: string;
  created_at: string;
  updated_at: string;
}

export type RetrieveResultStatus =
  | 'success'
  | 'no_result'
  | 'filtered_out'
  | 'error'
  | 'timeout';

export interface RetrieveItem {
  content: string;
  score: number;
  citation: {
    kb_id: number;
    document_id: number;
    chunk_id: string;
    file_name: string;
    chunk_index: number;
  };
  source: {
    route: string;
    collection: string;
    retriever_version: string;
  };
}

export interface RetrieveResponse {
  request_id: string;
  items: RetrieveItem[];
}

export interface KBRetrieveLog {
  id: number;
  request_id: string;
  user_id: number;
  kb_ids: string;
  query: string;
  final_query?: string;
  expr?: string;
  top_k: number;
  candidate_topk: number;
  final_topk: number;
  token_budget: number;
  truncate_reason?: string;
  rewrite?: string;
  rewrite_strategy?: string;
  rewrite_applied: boolean;
  strategy?: string;
  release_stage?: string;
  release_reason?: string;
  routes?: string;
  collection?: string;
  retriever_version?: string;
  empty_reason?: string;
  final_count: number;
  truncated_count: number;
  dense_hits: number;
  sparse_hits: number;
  dense_contribution: number;
  sparse_contribution: number;
  result_status: RetrieveResultStatus;
  error_code?: string;
  error_msg?: string;
  embedding_ms: number;
  search_ms: number;
  postprocess_ms: number;
  rerank_ms: number;
  rerank_model?: string;
  duration_ms: number;
  timeout_ms: number;
  created_at: string;
}

export interface KBJobOperationLog {
  id: number;
  job_id: number;
  operator_id: number;
  operation: string;
  operation_reason?: string;
  from_status: string;
  to_status: string;
  created_at: string;
}

export interface IngestLogDetail {
  job: KBIngestJob;
  operation_logs: KBJobOperationLog[];
}

export type MetricsRange = '1h' | '24h' | '7d';

export interface MetricsOverviewBucketRate {
  bucket: string;
  rate: number;
  total: number;
  success?: number;
  empty?: number;
}

export interface MetricsOverviewBucketCount {
  bucket: string;
  count: number;
}

export interface MetricsOverviewBucketP95 {
  bucket: string;
  p95_ms: number;
}

export interface MetricsOverviewErrorType {
  error_code: string;
  count: number;
}

export interface MetricsOverview {
  range: MetricsRange;
  ingest_success_rate: MetricsOverviewBucketRate[];
  retrieve_request_count: MetricsOverviewBucketCount[];
  retrieve_p95_ms: MetricsOverviewBucketP95[];
  retrieve_empty_rate: MetricsOverviewBucketRate[];
  error_type_topn: MetricsOverviewErrorType[];
}

export interface ListResponse<T> {
  items: T[];
  total: number;
  page: number;
  page_size: number;
}
