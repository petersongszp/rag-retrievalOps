package benchmark

import (
	"fmt"
	"sort"

	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

const (
	DefaultIndexName   = "idx_vector"
	DefaultVectorField = "vector"
)

func DefaultProfiles() []IndexProfile {
	return []IndexProfile{
		{
			Name:       "phase1-hnsw-baseline",
			Label:      "Phase 1 HNSW baseline",
			Family:     IndexFamilyHNSW,
			MetricType: string(entity.COSINE),
			IsBaseline: true,
			Notes:      "Current production-safe baseline before Phase 2 tuning.",
			HNSW: &HNSWParams{
				M:              16,
				EfConstruction: 200,
				EfSearch:       64,
			},
		},
		{
			Name:       "phase2-hnsw-balanced",
			Label:      "Phase 2 HNSW balanced",
			Family:     IndexFamilyHNSW,
			MetricType: string(entity.COSINE),
			Notes:      "Balanced recall and latency candidate for general RAG traffic.",
			HNSW: &HNSWParams{
				M:              24,
				EfConstruction: 320,
				EfSearch:       96,
			},
		},
		{
			Name:       "phase2-hnsw-high-recall",
			Label:      "Phase 2 HNSW high recall",
			Family:     IndexFamilyHNSW,
			MetricType: string(entity.COSINE),
			Notes:      "Aggressive recall profile for long-tail and entity-heavy queries.",
			HNSW: &HNSWParams{
				M:              32,
				EfConstruction: 360,
				EfSearch:       128,
			},
		},
		{
			Name:       "phase2-ivf-balanced",
			Label:      "Phase 2 IVF balanced",
			Family:     IndexFamilyIVF,
			MetricType: string(entity.COSINE),
			Notes:      "IVF option for lower memory footprint while maintaining acceptable recall.",
			IVF: &IVFParams{
				NList:  2048,
				NProbe: 32,
			},
		},
		{
			Name:       "phase2-ivf-low-latency",
			Label:      "Phase 2 IVF low latency",
			Family:     IndexFamilyIVF,
			MetricType: string(entity.COSINE),
			Notes:      "Latency-biased IVF option for bursty traffic.",
			IVF: &IVFParams{
				NList:  1024,
				NProbe: 16,
			},
		},
	}
}

func ValidateProfile(profile IndexProfile) error {
	if profile.Name == "" {
		return fmt.Errorf("profile name is required")
	}
	if profile.MetricType == "" {
		profile.MetricType = string(entity.COSINE)
	}
	switch profile.Family {
	case IndexFamilyHNSW:
		if profile.HNSW == nil {
			return fmt.Errorf("profile %s requires hnsw params", profile.Name)
		}
		if profile.HNSW.M < 4 || profile.HNSW.EfConstruction < 8 || profile.HNSW.EfSearch < 1 {
			return fmt.Errorf("profile %s contains invalid hnsw params", profile.Name)
		}
	case IndexFamilyIVF:
		if profile.IVF == nil {
			return fmt.Errorf("profile %s requires ivf params", profile.Name)
		}
		if profile.IVF.NList < 1 || profile.IVF.NProbe < 1 {
			return fmt.Errorf("profile %s contains invalid ivf params", profile.Name)
		}
	default:
		return fmt.Errorf("profile %s uses unsupported family %s", profile.Name, profile.Family)
	}
	return nil
}

func SortResults(results []ProfileResult) {
	sort.SliceStable(results, func(i, j int) bool {
		scoreI := recommendationScore(results[i])
		scoreJ := recommendationScore(results[j])
		if scoreI == scoreJ {
			return results[i].Metrics.P95LatencyMS < results[j].Metrics.P95LatencyMS
		}
		return scoreI > scoreJ
	})
}

func recommendationScore(result ProfileResult) float64 {
	return result.Metrics.RecallAtK*0.45 + result.Metrics.MRR*0.30 + result.Metrics.NDCG*0.25
}
