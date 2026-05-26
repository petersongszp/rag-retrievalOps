package kb

import (
	"fmt"
	"strings"

	"interview-agents/internal/config"
	"interview-agents/internal/milvus/retrieval"

	"github.com/cloudwego/eino/schema"
)

func resolveEvidenceGateOutcome(query string, docs []*schema.Document, metrics retrieval.SearchMetrics) retrieval.EvidenceGateOutcome {
	if strings.TrimSpace(metrics.EvidenceGateResult) != "" {
		return retrieval.EvidenceGateOutcome{
			Result:               metrics.EvidenceGateResult,
			RefusalReason:        metrics.RefusalReason,
			CitationSupportScore: metrics.CitationSupportScore,
			Error:                metrics.EvidenceGateError,
		}
	}

	return retrieval.EvaluateEvidenceGate(query, docs, metrics, retrieval.EvidenceGateConfig{
		Enabled:             config.Global.RAG.FeatureFlags.EnableEvidenceRefusal,
		MinRerankScore:      config.Global.RAG.Phase3.EvidenceMinRerankScore,
		MinEvidenceDensity:  config.Global.RAG.Phase3.EvidenceMinDensity,
		MinCitationCoverage: config.Global.RAG.Phase3.EvidenceMinCitationCoverage,
	})
}

func buildStandardRefusalPayload(outcome retrieval.EvidenceGateOutcome) *refusalPayload {
	if !outcome.Refused() {
		return nil
	}

	reasonLabel := normalizeRefusalReason(outcome.RefusalReason)
	return &refusalPayload{
		Reason:               outcome.RefusalReason,
		Message:              fmt.Sprintf("当前知识库证据不足，暂时不能可靠回答这个问题。触发原因：%s。", reasonLabel),
		Suggestions:          refusalSuggestions(outcome.RefusalReason),
		CitationSupportScore: outcome.CitationSupportScore,
	}
}

func normalizeRefusalReason(reason string) string {
	switch strings.TrimSpace(reason) {
	case retrieval.RefusalReasonNoRetrievalHit:
		return "未检索到可用证据"
	case retrieval.RefusalReasonLowRerankConfidence:
		return "候选证据置信度不足"
	case retrieval.RefusalReasonInsufficientCitationCover:
		return "引用覆盖不足"
	case retrieval.RefusalReasonContradictoryEvidence:
		return "候选证据存在冲突"
	case retrieval.RefusalReasonOutOfKBScope:
		return "问题超出知识库覆盖范围"
	default:
		if strings.TrimSpace(reason) == "" {
			return "证据校验未通过"
		}
		return strings.TrimSpace(reason)
	}
}

func refusalSuggestions(reason string) []string {
	base := []string{
		"补充更相关的知识库文档后再试。",
		"把问题缩小到更具体的模块、版本或场景。",
	}

	switch strings.TrimSpace(reason) {
	case retrieval.RefusalReasonNoRetrievalHit, retrieval.RefusalReasonOutOfKBScope:
		return append(base, "确认问题是否属于当前知识库范围。")
	case retrieval.RefusalReasonInsufficientCitationCover:
		return append(base, "优先上传包含明确章节、文件名或原始出处的材料。")
	case retrieval.RefusalReasonContradictoryEvidence:
		return append(base, "补充权威版本文档，或明确你希望采用的规范来源。")
	default:
		return base
	}
}
