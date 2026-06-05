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

type ProfileBundle struct {
	ProfileVersion string            `json:"profile_version,omitempty"`
	Description    string            `json:"description,omitempty"`
	Profiles       []StrategyProfile `json:"profiles"`
}

type DatasetCase struct {
	ID                          string           `json:"id"`
	Question                    string           `json:"question,omitempty"`
	Query                       string           `json:"query"`
	Context                     string           `json:"context,omitempty"`
	GroundTruth                 string           `json:"ground_truth,omitempty"`
	TopK                        int              `json:"top_k"`
	RelevantIDs                 []string         `json:"relevant_ids"`
	CitationTargets             []CitationTarget `json:"citation_targets,omitempty"`
	QueryType                   string           `json:"query_type,omitempty"`
	Scenario                    string           `json:"scenario,omitempty"`
	ExpectedBehavior            string           `json:"expected_behavior,omitempty"`
	ExpectedPrimaryRoute        string           `json:"expected_primary_route,omitempty"`
	ExpectedParticipatingRoutes []string         `json:"expected_participating_routes,omitempty"`
	MustContainTerms            []string         `json:"must_contain_terms,omitempty"`
	Difficulty                  string           `json:"difficulty,omitempty"`
	Tags                        []string         `json:"tags,omitempty"`
	KBIDs                       []uint64         `json:"kb_ids,omitempty"`
	Collection                  string           `json:"collection,omitempty"`
	Notes                       string           `json:"notes,omitempty"`
}

type StrategyProfile struct {
	Name                        string  `json:"name"`
	Label                       string  `json:"label,omitempty"`
	Family                      string  `json:"family,omitempty"`
	FusionStrategy              string  `json:"fusion_strategy,omitempty"`
	Notes                       string  `json:"notes,omitempty"`
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

type SearchOutcome struct {
	Items                 []RetrievedItem
	Refused               bool
	RefusalReason         string
	CitationSupportScore  float64
	ParentFillCount       int
	RewriteApplied        bool
	ModelRewriteApplied   bool
	DenseHits             int
	SparseHits            int
	DenseParticipation    int
	SparseParticipation   int
	PrimaryDenseCount     int
	PrimarySparseCount    int
	DualRouteFinalCount   int
	EmptyReason           string
	DenseContribution     int
	SparseContribution    int
	SparseCandidateBefore int
	SparseCandidateAfter  int
}

type QueryMetrics struct {
	QueryID                     string           `json:"query_id"`
	Query                       string           `json:"query"`
	QueryType                   string           `json:"query_type,omitempty"`
	Tags                        []string         `json:"tags,omitempty"`
	TopK                        int              `json:"top_k"`
	Latency                     time.Duration    `json:"latency"`
	RecallAtK                   float64          `json:"recall_at_k"`
	MRR                         float64          `json:"mrr"`
	NDCG                        float64          `json:"ndcg"`
	CitationAccuracy            float64          `json:"citation_accuracy"`
	CitationPrecision           float64          `json:"citation_precision"`
	CitationRecall              float64          `json:"citation_recall"`
	LongDocCompleteness         float64          `json:"long_doc_completeness"`
	Refused                     bool             `json:"refused"`
	RefusalReason               string           `json:"refusal_reason,omitempty"`
	RefusalExpected             bool             `json:"refusal_expected"`
	RefusalCorrect              bool             `json:"refusal_correct"`
	RefusalFalsePositive        bool             `json:"refusal_false_positive"`
	ParentFillCount             int              `json:"parent_fill_count"`
	RewriteApplied              bool             `json:"rewrite_applied"`
	ModelRewriteApplied         bool             `json:"model_rewrite_applied"`
	ExpectedPrimaryRoute        string           `json:"expected_primary_route,omitempty"`
	ExpectedParticipatingRoutes []string         `json:"expected_participating_routes,omitempty"`
	MustContainTerms            []string         `json:"must_contain_terms,omitempty"`
	DenseHits                   int              `json:"dense_hits"`
	SparseHits                  int              `json:"sparse_hits"`
	DenseParticipation          int              `json:"dense_participation"`
	SparseParticipation         int              `json:"sparse_participation"`
	PrimaryDenseCount           int              `json:"primary_dense_count"`
	PrimarySparseCount          int              `json:"primary_sparse_count"`
	DualRouteFinalCount         int              `json:"dual_route_final_count"`
	PrimaryRoute                string           `json:"primary_route,omitempty"`
	EmptyResult                 bool             `json:"empty_result"`
	EmptyReason                 string           `json:"empty_reason,omitempty"`
	DenseContribution           int              `json:"dense_contribution"`
	SparseContribution          int              `json:"sparse_contribution"`
	SparseCandidateBefore       int              `json:"sparse_candidate_before_bm25"`
	SparseCandidateAfter        int              `json:"sparse_candidate_after_bm25"`
	ResultIDs                   []string         `json:"result_ids"`
	RelevantIDs                 []string         `json:"relevant_ids"`
	CitationTargets             []CitationTarget `json:"citation_targets,omitempty"`
}

type AggregateMetrics struct {
	RecallAtK                float64       `json:"recall_at_k"`
	MRR                      float64       `json:"mrr"`
	NDCG                     float64       `json:"ndcg"`
	CitationAccuracy         float64       `json:"citation_accuracy"`
	CitationPrecision        float64       `json:"citation_precision"`
	CitationRecall           float64       `json:"citation_recall"`
	LongDocCompleteness      float64       `json:"long_doc_completeness"`
	EvidenceRefusalRate      float64       `json:"evidence_refusal_rate"`
	RefusalFalsePositiveRate float64       `json:"refusal_false_positive_rate"`
	ParentFillGain           float64       `json:"parent_fill_gain"`
	RewriteAppliedRate       float64       `json:"rewrite_applied_rate"`
	ModelRewriteRate         float64       `json:"model_rewrite_rate"`
	DenseHitRate             float64       `json:"dense_hit_rate"`
	SparseHitRate            float64       `json:"sparse_hit_rate"`
	DenseParticipationRate   float64       `json:"dense_participation_rate"`
	SparseParticipationRate  float64       `json:"sparse_participation_rate"`
	PrimaryDenseRate         float64       `json:"primary_dense_rate"`
	PrimarySparseRate        float64       `json:"primary_sparse_rate"`
	DualRouteRate            float64       `json:"dual_route_rate"`
	EmptyRate                float64       `json:"empty_rate"`
	DenseRouteContribution   float64       `json:"dense_route_contribution"`
	SparseRouteContribution  float64       `json:"sparse_route_contribution"`
	P50LatencyMS             float64       `json:"p50_latency_ms"`
	P95LatencyMS             float64       `json:"p95_latency_ms"`
	AvgLatencyMS             float64       `json:"avg_latency_ms"`
	TotalLatency             time.Duration `json:"total_latency"`
}

type StrategyResult struct {
	Strategy StrategyProfile  `json:"strategy"`
	Metrics  AggregateMetrics `json:"metrics"`
	Queries  []QueryMetrics   `json:"queries"`
}

type StrategyDelta struct {
	Strategy                  string  `json:"strategy"`
	ComparedTo                string  `json:"compared_to"`
	RecallDelta               float64 `json:"recall_delta"`
	MRRDelta                  float64 `json:"mrr_delta"`
	NDCGDelta                 float64 `json:"ndcg_delta"`
	CitationAccuracyDelta     float64 `json:"citation_accuracy_delta"`
	CitationPrecisionDelta    float64 `json:"citation_precision_delta"`
	CitationRecallDelta       float64 `json:"citation_recall_delta"`
	LongDocCompletenessDelta  float64 `json:"long_doc_completeness_delta"`
	ParentFillGainDelta       float64 `json:"parent_fill_gain_delta"`
	RefusalFalsePositiveDelta float64 `json:"refusal_false_positive_delta"`
	DenseHitRateDelta         float64 `json:"dense_hit_rate_delta"`
	SparseHitRateDelta        float64 `json:"sparse_hit_rate_delta"`
	DenseParticipationDelta   float64 `json:"dense_participation_delta"`
	SparseParticipationDelta  float64 `json:"sparse_participation_delta"`
	PrimaryDenseRateDelta     float64 `json:"primary_dense_rate_delta"`
	PrimarySparseRateDelta    float64 `json:"primary_sparse_rate_delta"`
	EmptyRateDelta            float64 `json:"empty_rate_delta"`
	P95LatencyDeltaMS         float64 `json:"p95_latency_delta_ms"`
}

type ComparisonSummary struct {
	Baseline                     string  `json:"baseline"`
	Candidate                    string  `json:"candidate"`
	RecallDelta                  float64 `json:"recall_delta"`
	MRRDelta                     float64 `json:"mrr_delta"`
	NDCGDelta                    float64 `json:"ndcg_delta"`
	CitationAccuracyDelta        float64 `json:"citation_accuracy_delta"`
	CitationPrecisionDelta       float64 `json:"citation_precision_delta"`
	CitationRecallDelta          float64 `json:"citation_recall_delta"`
	LongDocCompletenessDelta     float64 `json:"long_doc_completeness_delta"`
	ParentFillGainDelta          float64 `json:"parent_fill_gain_delta"`
	EvidenceRefusalRateDelta     float64 `json:"evidence_refusal_rate_delta"`
	RefusalFalsePositiveRate     float64 `json:"refusal_false_positive_rate"`
	RewriteGainDelta             float64 `json:"rewrite_gain_delta"`
	DenseRouteContributionDelta  float64 `json:"dense_route_contribution_delta"`
	SparseRouteContributionDelta float64 `json:"sparse_route_contribution_delta"`
	DenseHitRateDelta            float64 `json:"dense_hit_rate_delta"`
	SparseHitRateDelta           float64 `json:"sparse_hit_rate_delta"`
	DenseParticipationRateDelta  float64 `json:"dense_participation_rate_delta"`
	SparseParticipationRateDelta float64 `json:"sparse_participation_rate_delta"`
	PrimaryDenseRateDelta        float64 `json:"primary_dense_rate_delta"`
	PrimarySparseRateDelta       float64 `json:"primary_sparse_rate_delta"`
	EmptyRateDelta               float64 `json:"empty_rate_delta"`
	P95LatencyDeltaMS            float64 `json:"p95_latency_delta_ms"`
	P95LatencyDeltaRatio         float64 `json:"p95_latency_delta_ratio"`
	CandidateModelRewrite        bool    `json:"candidate_model_rewrite"`
}

type GateThresholds struct {
	MinRecallDelta               float64 `json:"min_recall_delta"`
	MinMRRDelta                  float64 `json:"min_mrr_delta"`
	MinNDCGDelta                 float64 `json:"min_ndcg_delta"`
	MinCitationAccuracyDelta     float64 `json:"min_citation_accuracy_delta"`
	MinCitationPrecisionDelta    float64 `json:"min_citation_precision_delta"`
	MinCitationRecallDelta       float64 `json:"min_citation_recall_delta"`
	MaxP95LatencyRegressionMS    float64 `json:"max_p95_latency_regression_ms"`
	MaxP95LatencyRegressionRatio float64 `json:"max_p95_latency_regression_ratio"`
	MaxRefusalFalsePositiveRate  float64 `json:"max_refusal_false_positive_rate"`
	MinRewriteGainDelta          float64 `json:"min_rewrite_gain_delta"`
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
	ProfileVersion string            `json:"profile_version,omitempty"`
	GeneratedAt    time.Time         `json:"generated_at"`
	Results        []StrategyResult  `json:"results"`
	Contribution   []StrategyDelta   `json:"contribution"`
	Comparison     ComparisonSummary `json:"comparison"`
	Gate           GateResult        `json:"gate"`
	Baseline       string            `json:"baseline"`
	Candidate      string            `json:"candidate"`
}
