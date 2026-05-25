package evaluation

func DefaultProfiles() []StrategyProfile {
	return []StrategyProfile{
		{
			Name:     "dense_only",
			Label:    "Dense Only",
			Baseline: true,
			Mode:     "dense",
		},
		{
			Name:          "hybrid",
			Label:         "Hybrid",
			Mode:          "hybrid",
			CandidateTopK: 10,
		},
		{
			Name:               "hybrid_rewrite",
			Label:              "Hybrid + Rewrite",
			Mode:               "hybrid",
			EnableQueryRewrite: true,
			CandidateTopK:      10,
		},
		{
			Name:               "hybrid_rewrite_dynamic_topk",
			Label:              "Hybrid + Rewrite + DynamicTopK",
			Candidate:          true,
			Mode:               "hybrid",
			EnableQueryRewrite: true,
			EnableDynamicTopK:  true,
			CandidateTopK:      10,
		},
	}
}
