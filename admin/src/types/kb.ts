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

export type EvalDatasetStatus = 'draft' | 'ready' | 'invalid' | 'archived';

export type EvalRunStatus = 'pending' | 'running' | 'succeeded' | 'failed' | 'canceled';

export type EvalFailureReason =
  | 'recall_miss'
  | 'citation_miss'
  | 'mrr_drop'
  | 'ndcg_drop'
  | 'latency_regression'
  | 'gate_failed'
  | 'trace_missing';

export interface CitationTarget {
  document_id?: number;
  chunk_id?: string;
  file_name?: string;
}

export interface EvalDataset {
  id: number;
  name: string;
  description?: string;
  kb_id?: number;
  case_count: number;
  status: EvalDatasetStatus;
  created_by?: number;
  created_at: string;
  updated_at: string;
}

export interface EvalCase {
  id: number;
  dataset_id: number;
  case_key: string;
  query: string;
  top_k: number;
  relevant_ids: string[];
  citation_targets: CitationTarget[];
  query_type?: string;
  tags?: string[];
  kb_ids?: number[];
  collection?: string;
  notes?: string;
  validation_status: 'valid' | 'invalid' | 'unchecked';
  validation_errors?: string[];
  created_at?: string;
  updated_at?: string;
}

export interface EvalStrategyProfile {
  name: string;
  label?: string;
  baseline?: boolean;
  candidate?: boolean;
  mode: string;
  enable_query_rewrite?: boolean;
  enable_dynamic_topk?: boolean;
  enable_advanced_rerank?: boolean;
  candidate_top_k?: number;
  dense_weight?: number;
  sparse_weight?: number;
  min_top_k?: number;
  max_top_k?: number;
  token_budget?: number;
  rewrite_max_expansions?: number;
  rerank_timeout_ms?: number;
  rerank_model?: string;
}

export interface EvalAggregateMetrics {
  recall_at_k: number;
  mrr: number;
  ndcg: number;
  citation_accuracy: number;
  p50_latency_ms: number;
  p95_latency_ms: number;
  avg_latency_ms: number;
}

export interface EvalQueryMetrics {
  query_id: string;
  query: string;
  query_type?: string;
  tags?: string[];
  top_k: number;
  latency: number;
  recall_at_k: number;
  mrr: number;
  ndcg: number;
  citation_accuracy: number;
  result_ids: string[];
  relevant_ids: string[];
  citation_targets?: CitationTarget[];
}

export interface EvalStrategyResult {
  strategy: EvalStrategyProfile;
  metrics: EvalAggregateMetrics;
  queries: EvalQueryMetrics[];
}

export interface EvalStrategyDelta {
  strategy: string;
  compared_to: string;
  recall_delta: number;
  mrr_delta: number;
  ndcg_delta: number;
  citation_accuracy_delta: number;
  p95_latency_delta_ms: number;
}

export interface EvalComparisonSummary {
  baseline: string;
  candidate: string;
  recall_delta: number;
  mrr_delta: number;
  ndcg_delta: number;
  citation_accuracy_delta: number;
  p95_latency_delta_ms: number;
  p95_latency_delta_ratio: number;
}

export interface EvalGateThresholds {
  min_recall_delta?: number;
  min_mrr_delta?: number;
  min_ndcg_delta?: number;
  min_citation_accuracy_delta?: number;
  max_p95_latency_regression_ms?: number;
  max_p95_latency_regression_ratio?: number;
}

export interface EvalGateCheck {
  name: string;
  actual: number;
  expected: number;
  passed: boolean;
  message: string;
}

export interface EvalGateResult {
  passed: boolean;
  thresholds: EvalGateThresholds;
  checks: EvalGateCheck[];
}

export interface EvalReport {
  dataset_size: number;
  generated_at: string;
  results: EvalStrategyResult[];
  contribution: EvalStrategyDelta[];
  comparison: EvalComparisonSummary;
  gate: EvalGateResult;
  baseline: string;
  candidate: string;
}

export interface EvalRun {
  id: number;
  run_id: string;
  dataset_id: number;
  baseline_profile: string;
  candidate_profile: string;
  profiles?: EvalStrategyProfile[];
  gate_thresholds: EvalGateThresholds;
  status: EvalRunStatus;
  progress: number;
  case_total: number;
  case_finished: number;
  report_path?: string;
  error_msg?: string;
  started_at?: string;
  finished_at?: string;
  created_at: string;
  updated_at?: string;
}

export interface EvalFailureMetrics {
  recall_at_k: number;
  mrr: number;
  ndcg: number;
  citation_accuracy: number;
  latency_ms: number;
}

export interface EvalFailureDelta {
  recall_delta: number;
  mrr_delta: number;
  ndcg_delta: number;
  citation_accuracy_delta: number;
  latency_delta_ms: number;
}

export interface EvalFailureCase {
  case_id: string;
  query: string;
  query_type?: string;
  tags?: string[];
  failure_reason: EvalFailureReason;
  baseline_metrics: EvalFailureMetrics;
  candidate_metrics: EvalFailureMetrics;
  delta: EvalFailureDelta;
  baseline_request_id?: string;
  candidate_request_id?: string;
}

export interface EvalDatasetValidationIssue {
  case_id: number;
  case_key: string;
  errors: string[];
}

export interface EvalDatasetValidationResult {
  dataset_id: number;
  status: EvalDatasetStatus;
  case_count: number;
  valid_count: number;
  invalid_count: number;
  unchecked_count: number;
  issues: EvalDatasetValidationIssue[];
}

export interface EvalCaseImportError {
  index: number;
  case_key?: string;
  message: string;
}

export interface EvalCaseImportResult {
  imported: number;
  failed: number;
  errors: EvalCaseImportError[];
}
