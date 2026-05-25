package benchmark

import "time"

type IndexFamily string

const (
	IndexFamilyHNSW IndexFamily = "HNSW"
	IndexFamilyIVF  IndexFamily = "IVF_FLAT"
)

type QueryCase struct {
	ID          string   `json:"id"`
	Query       string   `json:"query"`
	TopK        int      `json:"top_k"`
	RelevantIDs []string `json:"relevant_ids"`
}

type HNSWParams struct {
	M              int `json:"m"`
	EfConstruction int `json:"ef_construction"`
	EfSearch       int `json:"ef_search"`
}

type IVFParams struct {
	NList  int `json:"nlist"`
	NProbe int `json:"nprobe"`
}

type IndexProfile struct {
	Name       string      `json:"name"`
	Label      string      `json:"label"`
	Family     IndexFamily `json:"family"`
	MetricType string      `json:"metric_type"`
	IsBaseline bool        `json:"is_baseline"`
	Notes      string      `json:"notes,omitempty"`
	HNSW       *HNSWParams `json:"hnsw,omitempty"`
	IVF        *IVFParams  `json:"ivf,omitempty"`
}

type QueryMetrics struct {
	QueryID    string        `json:"query_id"`
	Query      string        `json:"query"`
	TopK       int           `json:"top_k"`
	Latency    time.Duration `json:"latency"`
	RecallAtK  float64       `json:"recall_at_k"`
	MRR        float64       `json:"mrr"`
	NDCG       float64       `json:"ndcg"`
	ResultIDs  []string      `json:"result_ids"`
	RelevantID []string      `json:"relevant_ids"`
}

type ResourceUsage struct {
	ProcessCPUUserMS   int64   `json:"process_cpu_user_ms"`
	ProcessCPUSystemMS int64   `json:"process_cpu_system_ms"`
	HeapAllocMB        float64 `json:"heap_alloc_mb"`
	HeapSysMB          float64 `json:"heap_sys_mb"`
}

type AggregateMetrics struct {
	RecallAtK    float64       `json:"recall_at_k"`
	MRR          float64       `json:"mrr"`
	NDCG         float64       `json:"ndcg"`
	P50LatencyMS float64       `json:"p50_latency_ms"`
	P95LatencyMS float64       `json:"p95_latency_ms"`
	AvgLatencyMS float64       `json:"avg_latency_ms"`
	TotalLatency time.Duration `json:"total_latency"`
	Resources    ResourceUsage `json:"resources"`
}

type ProfileResult struct {
	Profile IndexProfile     `json:"profile"`
	Metrics AggregateMetrics `json:"metrics"`
	Queries []QueryMetrics   `json:"queries"`
}

type Recommendation struct {
	RecommendedProfile string   `json:"recommended_profile"`
	BaselineProfile    string   `json:"baseline_profile"`
	Reasons            []string `json:"reasons"`
	Risks              []string `json:"risks"`
	RollbackSteps      []string `json:"rollback_steps"`
}

type Report struct {
	DatasetSize     int             `json:"dataset_size"`
	GeneratedAt     time.Time       `json:"generated_at"`
	Results         []ProfileResult `json:"results"`
	Recommendation  Recommendation  `json:"recommendation"`
	ProfilesScanned []string        `json:"profiles_scanned"`
}
