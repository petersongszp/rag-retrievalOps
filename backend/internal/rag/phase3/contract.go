package phase3

const (
	RetrievalDebugRoute       = "/retrieve/audit/:request_id/debug"
	LegacyRetrievalDebugRoute = "/retrieve/debug/:request_id"
	StrategyRoutePrefix       = "/strategy"

	StrategyTargetPhase2Baseline = "phase2_baseline"

	FlagParentChildRetrieval = "RAG_ENABLE_PARENT_CHILD_RETRIEVAL"
	FlagStrategicTopK        = "RAG_ENABLE_STRATEGIC_TOPK"
	FlagEvidenceRefusal      = "RAG_ENABLE_EVIDENCE_REFUSAL"
	FlagCitationConsistency  = "RAG_ENABLE_CITATION_CONSISTENCY"
	FlagDomainTerms          = "RAG_ENABLE_DOMAIN_TERMS"
	FlagRouteSpecificRewrite = "RAG_ENABLE_ROUTE_SPECIFIC_REWRITE"
	FlagModelAssistedRewrite = "RAG_ENABLE_MODEL_ASSISTED_REWRITE"

	StatusEnabled     = "enabled"
	StatusDisabled    = "disabled"
	StatusShadow      = "shadow"
	StatusCanary      = "canary"
	StatusRollingBack = "rolling_back"
	StatusError       = "error"
)

var managedFeatureFlags = []string{
	FlagParentChildRetrieval,
	FlagStrategicTopK,
	FlagEvidenceRefusal,
	FlagCitationConsistency,
	FlagDomainTerms,
	FlagRouteSpecificRewrite,
	FlagModelAssistedRewrite,
}

var rollbackOrder = []string{
	FlagModelAssistedRewrite,
	FlagRouteSpecificRewrite,
	FlagDomainTerms,
	FlagEvidenceRefusal,
	FlagCitationConsistency,
	FlagStrategicTopK,
	FlagParentChildRetrieval,
}

var strategyStatuses = []string{
	StatusEnabled,
	StatusDisabled,
	StatusShadow,
	StatusCanary,
	StatusRollingBack,
	StatusError,
}

func ManagedFeatureFlags() []string {
	return append([]string(nil), managedFeatureFlags...)
}

func RollbackOrder() []string {
	return append([]string(nil), rollbackOrder...)
}

func StrategyStatuses() []string {
	return append([]string(nil), strategyStatuses...)
}

func IsManagedFeatureFlag(flagKey string) bool {
	for _, candidate := range managedFeatureFlags {
		if candidate == flagKey {
			return true
		}
	}
	return false
}

func IsStrategyStatus(status string) bool {
	for _, candidate := range strategyStatuses {
		if candidate == status {
			return true
		}
	}
	return false
}
