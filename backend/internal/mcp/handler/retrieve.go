package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	ragclient "interview-agents/internal/mcp/client"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultTopK            = 5
	maxTopK                = 20
	maxQueryRunes          = 2000
	maxKBIDs               = 100
	maxMetadataFilterBytes = 16 << 10
	maxMetadataFilterDepth = 8
)

type Retriever interface {
	Retrieve(context.Context, ragclient.RetrieveRequest) (*ragclient.RetrieveResponse, error)
}

type RetrieveKnowledgeInput struct {
	Query           string                 `json:"query"`
	KBIDs           []uint64               `json:"kb_ids,omitempty"`
	KBID            uint64                 `json:"kb_id,omitempty"`
	TopK            int                    `json:"top_k,omitempty"`
	StrategyProfile string                 `json:"strategy_profile,omitempty"`
	MetadataFilter  map[string]interface{} `json:"metadata_filter,omitempty"`
}

type RetrieveKnowledgeOutput struct {
	RequestID          string                   `json:"request_id"`
	Items              []ragclient.RetrieveItem `json:"items"`
	StrategyVersion    string                   `json:"strategy_version,omitempty"`
	RequestCost        interface{}              `json:"request_cost,omitempty"`
	CitationCheck      interface{}              `json:"citation_check,omitempty"`
	Refusal            interface{}              `json:"refusal,omitempty"`
	EvidenceGateResult string                   `json:"evidence_gate_result,omitempty"`
}

type RetrieveHandler struct {
	retriever Retriever
}

func NewRetrieveHandler(retriever Retriever) *RetrieveHandler {
	return &RetrieveHandler{retriever: retriever}
}

func (h *RetrieveHandler) Handle(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input RetrieveKnowledgeInput,
) (*mcp.CallToolResult, RetrieveKnowledgeOutput, error) {
	normalized, err := normalizeInput(input)
	if err != nil {
		return nil, RetrieveKnowledgeOutput{}, err
	}

	response, err := h.retriever.Retrieve(ctx, normalized)
	if err != nil {
		return nil, RetrieveKnowledgeOutput{}, err
	}

	output := RetrieveKnowledgeOutput{
		RequestID:          response.RequestID,
		Items:              response.Items,
		StrategyVersion:    response.StrategyVersion,
		RequestCost:        response.RequestCost,
		CitationCheck:      response.CitationCheck,
		Refusal:            response.Refusal,
		EvidenceGateResult: response.EvidenceGateResult,
	}
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: formatReadableResult(response)},
		},
	}
	return result, output, nil
}

func normalizeInput(input RetrieveKnowledgeInput) (ragclient.RetrieveRequest, error) {
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return ragclient.RetrieveRequest{}, fmt.Errorf("invalid_request: query is required")
	}
	if utf8.RuneCountInString(query) > maxQueryRunes {
		return ragclient.RetrieveRequest{}, fmt.Errorf("invalid_request: query must not exceed %d characters", maxQueryRunes)
	}

	kbIDs := make([]uint64, 0, len(input.KBIDs)+1)
	seen := make(map[uint64]struct{}, len(input.KBIDs)+1)
	for _, kbID := range input.KBIDs {
		if kbID == 0 {
			return ragclient.RetrieveRequest{}, fmt.Errorf("invalid_request: kb_ids must contain positive integers")
		}
		if _, exists := seen[kbID]; exists {
			continue
		}
		seen[kbID] = struct{}{}
		kbIDs = append(kbIDs, kbID)
	}
	if input.KBID > 0 {
		if _, exists := seen[input.KBID]; !exists {
			kbIDs = append(kbIDs, input.KBID)
		}
	}
	if len(kbIDs) == 0 {
		return ragclient.RetrieveRequest{}, fmt.Errorf("invalid_request: kb_id or kb_ids is required")
	}
	if len(kbIDs) > maxKBIDs {
		return ragclient.RetrieveRequest{}, fmt.Errorf("invalid_request: at most %d knowledge bases are allowed", maxKBIDs)
	}

	topK := input.TopK
	if topK == 0 {
		topK = defaultTopK
	}
	if topK < 1 || topK > maxTopK {
		return ragclient.RetrieveRequest{}, fmt.Errorf("invalid_request: top_k must be between 1 and %d", maxTopK)
	}
	if err := validateMetadataFilter(input.MetadataFilter); err != nil {
		return ragclient.RetrieveRequest{}, err
	}

	return ragclient.RetrieveRequest{
		Query:           query,
		KBIDs:           kbIDs,
		TopK:            topK,
		StrategyProfile: strings.TrimSpace(input.StrategyProfile),
		MetadataFilter:  input.MetadataFilter,
	}, nil
}

func validateMetadataFilter(filter map[string]interface{}) error {
	if filter == nil {
		return nil
	}
	data, err := json.Marshal(filter)
	if err != nil {
		return fmt.Errorf("invalid_request: metadata_filter must be valid JSON")
	}
	if len(data) > maxMetadataFilterBytes {
		return fmt.Errorf("invalid_request: metadata_filter exceeds %d bytes", maxMetadataFilterBytes)
	}
	if valueDepth(filter) > maxMetadataFilterDepth {
		return fmt.Errorf("invalid_request: metadata_filter nesting exceeds %d levels", maxMetadataFilterDepth)
	}
	return nil
}

func valueDepth(value interface{}) int {
	switch typed := value.(type) {
	case map[string]interface{}:
		maxChild := 0
		for _, child := range typed {
			if depth := valueDepth(child); depth > maxChild {
				maxChild = depth
			}
		}
		return maxChild + 1
	case []interface{}:
		maxChild := 0
		for _, child := range typed {
			if depth := valueDepth(child); depth > maxChild {
				maxChild = depth
			}
		}
		return maxChild + 1
	default:
		return 0
	}
}

func formatReadableResult(response *ragclient.RetrieveResponse) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "检索结果（共 %d 条）：", len(response.Items))
	if response.RequestID != "" {
		fmt.Fprintf(&builder, "\n请求 ID: %s", response.RequestID)
	}
	for index, item := range response.Items {
		fmt.Fprintf(
			&builder,
			"\n\n[%d] 相关度: %.4f | 来源: %s | 知识库: %d | 文档: %d | 分块: %d\n%s",
			index+1,
			item.Score,
			fallback(item.Citation.FileName, "未知"),
			item.Citation.KBID,
			item.Citation.DocumentID,
			item.Citation.ChunkIndex,
			item.Content,
		)
	}
	if response.Refusal != nil {
		builder.WriteString("\n\n检索结果包含证据门禁拒答信息，请结合结构化结果处理。")
	}
	return builder.String()
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}
