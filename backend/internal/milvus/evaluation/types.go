package evaluation

import "time"

type CitationTarget struct {
	DocumentID uint64 `json:"document_id,omitempty"`
	ChunkID    string `json:"chunk_id,omitempty"`
	FileName   string `json:"file_name,omitempty"`
}

type DatasetBundle struct {
	DatasetVersion string        `json:"dataset_version,omitempty"`
	Description    string        `json:"description,omitempty"`
	Cases          []DatasetCase `json:"cases"`
}

type DatasetCase struct {
	ID               string           `json:"id"`
	Question         string           `json:"question,omitempty"`
	Query            string           `json:"query"`
	Context          string           `json:"context,omitempty"`
	GroundTruth      string           `json:"ground_truth,omitempty"`
	TopK             int              `json:"top_k"`
	RelevantIDs      []string         `json:"relevant_ids"`
	CitationTargets  []CitationTarget `json:"citation_targets,omitempty"`
	QueryType        string           `json:"query_type,omitempty"`
	Scenario         string           `json:"scenario,omitempty"`
	ExpectedBehavior string           `json:"expected_behavior,omitempty"`
	Tags             []string         `json:"tags,omitempty"`
	KBIDs            []uint64         `json:"kb_ids,omitempty"`
	Collection       string           `json:"collection,omitempty"`
	Notes            string           `json:"notes,omitempty"`
}

type StrategyProfile struct {
	Name                        string  `json:"name"`
	Label                       string  `json:"label,omitempty"`
	Baseline                    bool    `json:"baseline,omitempty"`
	Candidate                   bool    `json:"candidate,omitempty"`
	Mode                        string  `json:"mode"`
	EnableQueryRewrite          bool    `json:"enable_query_rewrite,omitempty"`
	EnableDynamicTopK           bool    `json:"enable_dynamic_topk,omitempty"`
	EnableAdvancedRerank        bool    `json:"enable_advanced_rerank,omitempty"`
	EnableParentChildRetrieval  bool    `json:"enable_parent_child_retrieval,omitempty"`
	EnableStrategicTopK         bool    `json:"enable_strategic_topk,omitempty"`
	EnableEvidenceRefusal       bool    `json:"enable_evidence_refusal,omitempty"`
	EnableCitationConsistency   bool    `json:"enable_citation_consistency,omitempty"`
	EnableDomainTerms           bool    `json:"enable_domain_terms,omitempty"`
	EnableRouteSpecificRewrite  bool    `json:"enable_route_specific_rewrite,omitempty"`
	EnableModelAssistedRewrite  bool    `json:"enable_model_assisted_rewrite,omitempty"`
	CandidateTopK               int     `json:"candidate_top_k,omitempty"`
	DenseWeight                 float64 `json:"dense_weight,omitempty"`
	SparseWeight                float64 `json:"sparse_weight,omitempty"`
	MinTopK                     int     `json:"min_top_k,omitempty"`
	MaxTopK                     int     `json:"max_top_k,omitempty"`
	TokenBudget                 int     `json:"token_budget,omitempty"`
	MinAnswerChunks             int     `json:"min_answer_chunks,omitempty"`
	RewriteMaxExpansions        int     `json:"rewrite_max_expansions,omitempty"`
	RerankTimeoutMS             int     `json:"rerank_timeout_ms,omitempty"`
	RerankModel                 string  `json:"rerank_model,omitempty"`
	ParentChildFillStrategy     string  `json:"parent_child_fill_strategy,omitempty"`
	ParentChildWindowSize       int     `json:"parent_child_window_size,omitempty"`
	ParentChildMaxTokens        int     `json:"parent_child_max_tokens,omitempty"`
	StrategicTopKMinK           int     `json:"strategic_topk_min_k,omitempty"`
	StrategicTopKMaxK           int     `json:"strategic_topk_max_k,omitempty"`
	StrategicTopKBudgetRatio    float64 `json:"strategic_topk_budget_ratio,omitempty"`
	EvidenceMinRerankScore      float64 `json:"evidence_min_rerank_score,omitempty"`
	EvidenceMinDensity          float64 `json:"evidence_min_density,omitempty"`
	EvidenceMinCitationCoverage float64 `json:"evidence_min_citation_coverage,omitempty"`
	CitationCheckThreshold      float64 `json:"citation_check_threshold,omitempty"`
	CitationCheckVersion        string  `json:"citation_check_version,omitempty"`
	DomainTermTimeoutMS         int     `json:"domain_term_timeout_ms,omitempty"`
	ModelRewriteTimeoutMS       int     `json:"model_rewrite_timeout_ms,omitempty"`
	ModelRewriteShadowRatio     float64 `json:"model_rewrite_shadow_ratio,omitempty"`
}

type RetrievedItem struct {
	ResultID  string
	Score     float64
	Citation  CitationTarget
	Route     string
	Source    map[string]interface{}
	RawFields map[string]interface{}
}

type QueryMetrics struct {
	QueryID          string           `json:"query_id"`
	Query            string           `json:"query"`
	QueryType        string           `json:"query_type,omitempty"`
	Tags             []string         `json:"tags,omitempty"`
	TopK             int              `json:"top_k"`
	Latency          time.Duration    `json:"latency"`
	RecallAtK        float64          `json:"recall_at_k"`
	MRR              float64          `json:"mrr"`
	NDCG             float64          `json:"ndcg"`
	CitationAccuracy float64          `json:"citation_accuracy"`
	ResultIDs        []string         `json:"result_ids"`
	RelevantIDs      []string         `json:"relevant_ids"`
	CitationTargets  []CitationTarget `json:"citation_targets,omitempty"`
}

type AggregateMetrics struct {
	RecallAtK        float64       `json:"recall_at_k"`
	MRR              float64       `json:"mrr"`
	NDCG             float64       `json:"ndcg"`
	CitationAccuracy float64       `json:"citation_accuracy"`
	P50LatencyMS     float64       `json:"p50_latency_ms"`
	P95LatencyMS     float64       `json:"p95_latency_ms"`
	AvgLatencyMS     float64       `json:"avg_latency_ms"`
	TotalLatency     time.Duration `json:"total_latency"`
}

type StrategyResult struct {
	Strategy StrategyProfile  `json:"strategy"`
	Metrics  AggregateMetrics `json:"metrics"`
	Queries  []QueryMetrics   `json:"queries"`
}

type StrategyDelta struct {
	Strategy              string  `json:"strategy"`
	ComparedTo            string  `json:"compared_to"`
	RecallDelta           float64 `json:"recall_delta"`
	MRRDelta              float64 `json:"mrr_delta"`
	NDCGDelta             float64 `json:"ndcg_delta"`
	CitationAccuracyDelta float64 `json:"citation_accuracy_delta"`
	P95LatencyDeltaMS     float64 `json:"p95_latency_delta_ms"`
}

type ComparisonSummary struct {
	Baseline              string  `json:"baseline"`
	Candidate             string  `json:"candidate"`
	RecallDelta           float64 `json:"recall_delta"`
	MRRDelta              float64 `json:"mrr_delta"`
	NDCGDelta             float64 `json:"ndcg_delta"`
	CitationAccuracyDelta float64 `json:"citation_accuracy_delta"`
	P95LatencyDeltaMS     float64 `json:"p95_latency_delta_ms"`
	P95LatencyDeltaRatio  float64 `json:"p95_latency_delta_ratio"`
}

type GateThresholds struct {
	MinRecallDelta               float64 `json:"min_recall_delta"`
	MinMRRDelta                  float64 `json:"min_mrr_delta"`
	MinNDCGDelta                 float64 `json:"min_ndcg_delta"`
	MinCitationAccuracyDelta     float64 `json:"min_citation_accuracy_delta"`
	MaxP95LatencyRegressionMS    float64 `json:"max_p95_latency_regression_ms"`
	MaxP95LatencyRegressionRatio float64 `json:"max_p95_latency_regression_ratio"`
}

type GateCheck struct {
	Name     string  `json:"name"`
	Actual   float64 `json:"actual"`
	Expected float64 `json:"expected"`
	Passed   bool    `json:"passed"`
	Message  string  `json:"message"`
}

type GateResult struct {
	Passed     bool           `json:"passed"`
	Thresholds GateThresholds `json:"thresholds"`
	Checks     []GateCheck    `json:"checks"`
}

type Report struct {
	DatasetSize    int               `json:"dataset_size"`
	DatasetVersion string            `json:"dataset_version,omitempty"`
	GeneratedAt    time.Time         `json:"generated_at"`
	Results        []StrategyResult  `json:"results"`
	Contribution   []StrategyDelta   `json:"contribution"`
	Comparison     ComparisonSummary `json:"comparison"`
	Gate           GateResult        `json:"gate"`
	Baseline       string            `json:"baseline"`
	Candidate      string            `json:"candidate"`
}
