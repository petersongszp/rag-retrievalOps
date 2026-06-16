package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	ragclient "interview-agents/internal/mcp/client"
	internalmetrics "interview-agents/internal/mcp/metrics"

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

type RetrieverFactory interface {
	RetrieverFor(*mcp.CallToolRequest) (Retriever, error)
}

type RetrieverFactoryFunc func(*mcp.CallToolRequest) (Retriever, error)

func (f RetrieverFactoryFunc) RetrieverFor(req *mcp.CallToolRequest) (Retriever, error) {
	return f(req)
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
	retrieverFactory RetrieverFactory
}

func NewRetrieveHandler(retrieverFactory RetrieverFactory) *RetrieveHandler {
	return &RetrieveHandler{retrieverFactory: retrieverFactory}
}

func (h *RetrieveHandler) Handle(
	ctx context.Context,
	request *mcp.CallToolRequest,
	input RetrieveKnowledgeInput,
) (*mcp.CallToolResult, RetrieveKnowledgeOutput, error) {
	startedAt := time.Now()
	normalized, err := normalizeInput(input)
	if err != nil {
		internalmetrics.ObserveToolCall("retrieve_knowledge", "invalid_request", "invalid_request", durationMs(startedAt), 0)
		return toolErrorResult(err), RetrieveKnowledgeOutput{}, nil
	}

	retriever, err := h.retrieverFactory.RetrieverFor(request)
	if err != nil {
		internalmetrics.IncAuthMissing()
		internalmetrics.ObserveToolCall("retrieve_knowledge", "error", "unauthorized", durationMs(startedAt), 0)
		return toolErrorResult(err), RetrieveKnowledgeOutput{}, nil
	}

	response, err := retriever.Retrieve(ctx, normalized)
	if err != nil {
		return mapToolError(err, startedAt), RetrieveKnowledgeOutput{}, nil
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
	internalmetrics.ObserveToolCall("retrieve_knowledge", "success", "none", durationMs(startedAt), len(output.Items))
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
	fmt.Fprintf(&builder, "Retrieved %d item(s).", len(response.Items))
	if response.RequestID != "" {
		fmt.Fprintf(&builder, "\nrequest_id: %s", response.RequestID)
	}
	for index, item := range response.Items {
		fmt.Fprintf(
			&builder,
			"\n\n[%d] score=%.4f source=%s kb_id=%d document_id=%d chunk_index=%d\n%s",
			index+1,
			item.Score,
			fallback(item.Citation.FileName, "unknown"),
			item.Citation.KBID,
			item.Citation.DocumentID,
			item.Citation.ChunkIndex,
			item.Content,
		)
	}
	if response.Refusal != nil {
		builder.WriteString("\n\nResponse contains refusal metadata; inspect the structured payload before answering.")
	}
	return builder.String()
}

func mapToolError(err error, startedAt time.Time) *mcp.CallToolResult {
	upstreamErr, ok := err.(*ragclient.UpstreamError)
	if !ok {
		internalmetrics.ObserveToolCall("retrieve_knowledge", "error", "backend_error", durationMs(startedAt), 0)
		return toolErrorResult(fmt.Errorf("backend_error: %s", err.Error()))
	}

	switch upstreamErr.Code {
	case "unauthorized":
		internalmetrics.IncAuthMissing()
	case "forbidden":
		internalmetrics.IncForbidden()
	case "backend_timeout":
		internalmetrics.IncBackendTimeout()
	}
	internalmetrics.IncUpstreamError(upstreamErr.Code)
	internalmetrics.ObserveToolCall("retrieve_knowledge", "error", upstreamErr.Code, durationMs(startedAt), 0)

	payload := map[string]interface{}{
		"code":      upstreamErr.Code,
		"message":   upstreamErr.Message,
		"retryable": upstreamErr.Retryable,
	}
	if upstreamErr.RequestID != "" {
		payload["request_id"] = upstreamErr.RequestID
	}
	body, _ := json.Marshal(payload)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(body)},
		},
		StructuredContent: payload,
		IsError:           true,
	}
}

func toolErrorResult(err error) *mcp.CallToolResult {
	message := strings.TrimSpace(err.Error())
	code, detail := splitErrorCode(message)
	payload := map[string]interface{}{
		"code":      code,
		"message":   detail,
		"retryable": false,
	}
	body, _ := json.Marshal(payload)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(body)},
		},
		StructuredContent: payload,
		IsError:           true,
	}
}

func splitErrorCode(message string) (string, string) {
	code := "invalid_request"
	detail := message
	if head, tail, ok := strings.Cut(message, ":"); ok {
		code = strings.TrimSpace(head)
		if strings.TrimSpace(tail) != "" {
			detail = strings.TrimSpace(tail)
		}
	}
	return code, detail
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}

func durationMs(startedAt time.Time) float64 {
	return float64(time.Since(startedAt).Milliseconds())
}
