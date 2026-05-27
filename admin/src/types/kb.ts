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
  parent_child_enabled?: boolean;
  parent_fill_strategy?: string;
  parent_fill_count?: number;
  parent_fill_fallback?: number;
  parent_fill_tokens?: number;
  topk_decision_reason?: string;
  evidence_gate_result?: string;
  refusal_reason?: string;
  citation_supported?: boolean;
  citation_support_score?: number;
  rewrite_gain_bucket?: string;
  unsupported_claim_count?: number;
  citation_check_version?: string;
  citation_check_latency_ms?: number;
  evidence_gate_error?: string;
  citation_check_error?: string;
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

export interface RetrievalDebugDocument {
  document_id?: number;
  chunk_id?: string;
  parent_id?: string;
  file_name?: string;
  route?: string;
  score?: number;
  rerank_score?: number;
  content?: string;
  collection?: string;
  section_title?: string;
  hierarchy_path?: string;
  parent_fill_applied?: boolean;
  parent_fill_strategy?: string;
  parent_fill_reason?: string;
  metadata?: Record<string, unknown>;
}

export interface RetrievalRouteHit {
  route: string;
  query?: string;
  hits?: RetrievalDebugDocument[];
  contribution?: number;
  latency_ms?: number;
  error?: string;
}

export interface RetrievalFusionResult {
  before?: RetrievalDebugDocument[];
  after?: RetrievalDebugDocument[];
}

export interface RetrievalDedupeResult {
  before_count?: number;
  after_count?: number;
  removed?: RetrievalDebugDocument[];
}

export interface RetrievalRerankResult {
  before?: RetrievalDebugDocument[];
  after?: RetrievalDebugDocument[];
  rerank_model?: string;
  rerank_version?: string;
  fallback?: boolean;
  reason?: string;
}

export interface RetrievalFilterResult {
  before_count?: number;
  after_count?: number;
  removed?: RetrievalDebugDocument[];
  truncate_reason?: string;
}

export interface ParentChildDebugInfo {
  parent_child_enabled?: boolean;
  parent_fill_strategy?: string;
  parent_fill_count?: number;
  parent_fill_tokens?: number;
  child_hits?: RetrievalDebugDocument[];
  parent_contexts?: RetrievalDebugDocument[];
  parent_child_available?: boolean;
  fallback_reason?: string;
}

export interface TopKDecisionDebugInfo {
  topk_policy_version?: string;
  candidate_topk?: number;
  final_topk?: number;
  score_distribution?: string;
  rerank_gap?: number;
  evidence_density?: number;
  token_budget?: number;
  token_budget_remaining?: number;
  topk_decision_reason?: string;
}

export interface EvidenceGateDebugInfo {
  evidence_gate_result?: string;
  refusal_reason?: string;
  thresholds?: {
    min_rerank_score?: number;
    min_density?: number;
    min_citation_coverage?: number;
  };
  evidence_gate_error?: string;
  refusal_template_version?: string;
}

export interface CitationCheckDebugInfo {
  citation_supported?: boolean;
  citation_support_score?: number;
  unsupported_claims?: string[];
  citation_check_version?: string;
  citation_check_latency_ms?: number;
}

export interface RetrievalDebugDegradation {
  enabled?: boolean;
  reason?: string;
  fallback_strategy?: string;
  error_code?: string;
}

export interface RetrievalDebugTrace {
  request_id?: string;
  debug_available?: boolean;
  kb_ids?: number[];
  original_query?: string;
  rewritten_query?: string;
  route_final_queries?: Record<string, string>;
  route_hits?: RetrievalRouteHit[];
  fusion_results?: RetrievalFusionResult;
  dedupe_results?: RetrievalDedupeResult;
  rerank_results?: RetrievalRerankResult;
  filter_results?: RetrievalFilterResult;
  parent_child?: ParentChildDebugInfo;
  topk_decision?: TopKDecisionDebugInfo;
  evidence_gate?: EvidenceGateDebugInfo;
  citation_check?: CitationCheckDebugInfo;
  final_results?: RetrieveItem[];
  stage_durations?: Record<string, number>;
  degradation?: RetrievalDebugDegradation;
  contract_gaps?: string[];
  created_at?: string;
}

export type StrategyFlagStatus =
  | 'enabled'
  | 'disabled'
  | 'shadow'
  | 'canary'
  | 'rolling_back'
  | 'error';

export interface StrategyFlag {
  flag_key: string;
  label?: string;
  status?: StrategyFlagStatus;
  enabled?: boolean;
  rollout_percentage?: number;
  strategy_version?: string;
  risk_level?: string;
  updated_at?: string;
}

export interface StrategyVersion {
  version_id: string;
  flag_key: string;
  label?: string;
  created_at?: string;
  created_by?: string;
  gate_status?: string;
  baseline_report_id?: string;
  candidate_report_id?: string;
  metadata?: Record<string, unknown>;
}

export interface StrategyImpact {
  flag_key?: string;
  version?: string;
  range: MetricsRange;
  from?: string;
  to?: string;
  sample_size?: number;
  baseline_sample_size?: number;
  candidate_sample_size?: number;
  sample_size_too_small?: boolean;
  parent_fill_gain?: number;
  rewrite_gain?: number;
  route_contribution?: Record<string, number>;
  evidence_refusal_rate?: number;
  refusal_false_positive_rate?: number;
  citation_support_score?: number;
  citation_precision_delta?: number;
  p95_latency_delta_ms?: number;
  avg_context_tokens_delta?: number;
  empty_rate_delta?: number;
  error_rate_delta?: number;
  contract_gaps?: string[];
}

export interface StrategyGateSummary {
  flag_key?: string;
  version?: string;
  gate_status?: string;
  passed?: boolean;
  failed_rules?: string[];
  baseline_report_id?: string;
  candidate_report_id?: string;
  last_eval_run_id?: string;
  contract_gaps?: string[];
}

export interface StrategyOperationLog {
  id: string;
  operator_id?: number;
  operation?: string;
  flag_key?: string;
  from_status?: StrategyFlagStatus;
  to_status?: StrategyFlagStatus;
  from_rollout_percentage?: number;
  to_rollout_percentage?: number;
  reason?: string;
  created_at?: string;
}

export interface StrategyRollbackRequest {
  target_version?: string;
  flag_keys?: string[];
  reason: string;
}

export interface StrategyRollbackResult {
  rollback_id?: string;
  status?: string;
  changed_flags?: StrategyFlag[];
  target_version?: string;
  started_at?: string;
  finished_at?: string;
  error_msg?: string;
}
