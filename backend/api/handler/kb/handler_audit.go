package kb

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"interview-agents/api/response"
	myerrors "interview-agents/internal/errors"
	"interview-agents/internal/middleware"
	"interview-agents/internal/model"
	"interview-agents/internal/rag/governance"

	"github.com/cloudwego/hertz/pkg/app"
	"gorm.io/gorm"
)

type auditEventListResponse struct {
	Items    []*auditEventResponse `json:"items"`
	Total    int64                 `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
}

type auditEventResponse struct {
	ID                    uint64    `json:"id"`
	AuditTraceID          string    `json:"audit_trace_id"`
	RequestID             string    `json:"request_id,omitempty"`
	OperatorID            uint      `json:"operator_id,omitempty"`
	UserID                uint      `json:"user_id,omitempty"`
	KBID                  uint64    `json:"kb_id,omitempty"`
	DocumentID            uint64    `json:"document_id,omitempty"`
	Action                string    `json:"action"`
	ResourceType          string    `json:"resource_type"`
	ResourceID            string    `json:"resource_id,omitempty"`
	Before                string    `json:"before,omitempty"`
	After                 string    `json:"after,omitempty"`
	Result                string    `json:"result,omitempty"`
	Reason                string    `json:"reason,omitempty"`
	ActorName             string    `json:"actor_name,omitempty"`
	IP                    string    `json:"ip,omitempty"`
	UserAgent             string    `json:"user_agent,omitempty"`
	SensitiveFieldsMasked []string  `json:"sensitive_fields_masked,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
	ContractGaps          []string  `json:"contract_gaps,omitempty"`
}

type auditEventExportRequest struct {
	Action       string `json:"action"`
	ResourceType string `json:"resource_type"`
	ActorID      uint   `json:"actor_id"`
	KBID         uint64 `json:"kb_id"`
	RequestID    string `json:"request_id"`
	DocumentID   uint64 `json:"document_id"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	Reason       string `json:"reason"`
}

type auditEventExportResponse struct {
	ExportedAt   time.Time             `json:"exported_at"`
	ExportReason string                `json:"export_reason"`
	Items        []*auditEventResponse `json:"items"`
	ContractGaps []string              `json:"contract_gaps,omitempty"`
}

func ListAuditEvents(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	page, pageSize := getPagination(c)
	filter, err := parseAuditEventFilter(c)
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}
	filter.Page = page
	filter.PageSize = pageSize

	items, total, err := model.KBAuditEventDao.ListWithFilter(filter)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to list audit events", err))
		return
	}
	response.Success(ctx, c, auditEventListResponse{
		Items:    buildAuditEventResponses(items),
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func GetAuditEventDetail(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	eventID, err := parseUint64(c.Param("event_id"), "event_id")
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}
	item, err := model.KBAuditEventDao.GetByID(eventID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFound(ctx, c, "audit event not found")
			return
		}
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to get audit event", err))
		return
	}
	respItems := buildAuditEventResponses([]*model.KBAuditEvent{item})
	if len(respItems) == 0 {
		response.NotFound(ctx, c, "audit event not found")
		return
	}
	response.Success(ctx, c, respItems[0])
}

func ExportAuditEvents(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	var req auditEventExportRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.BadRequest(ctx, c, "invalid request: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Reason) == "" {
		response.BadRequest(ctx, c, "reason is required")
		return
	}

	filter, err := buildAuditEventFilterFromExport(req)
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}
	filter.Page = 1
	filter.PageSize = 500

	items, _, err := model.KBAuditEventDao.ListWithFilter(filter)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to export audit events", err))
		return
	}

	persistAuditEvent(&model.KBAuditEvent{
		AuditTraceID: "audit-export-" + time.Now().UTC().Format("20060102150405"),
		OperatorID:   middleware.GetUserID(c),
		UserID:       middleware.GetUserID(c),
		Action:       governance.ActionReportExport,
		ResourceType: "audit_event",
		ResourceID:   "audit_export",
		Reason:       strings.TrimSpace(req.Reason),
		Result:       "exported",
	})

	response.Success(ctx, c, auditEventExportResponse{
		ExportedAt:   time.Now().UTC(),
		ExportReason: strings.TrimSpace(req.Reason),
		Items:        buildAuditEventResponses(items),
		ContractGaps: []string{"export_download_url"},
	})
}

func buildAuditEventResponses(items []*model.KBAuditEvent) []*auditEventResponse {
	result := make([]*auditEventResponse, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		maskedFields := parseSensitiveFieldsMasked(item.SensitiveFieldsMasked)
		resp := &auditEventResponse{
			ID:                    item.ID,
			AuditTraceID:          item.AuditTraceID,
			RequestID:             item.RequestID,
			OperatorID:            item.OperatorID,
			UserID:                item.UserID,
			KBID:                  item.KBID,
			DocumentID:            item.DocumentID,
			Action:                item.Action,
			ResourceType:          item.ResourceType,
			ResourceID:            item.ResourceID,
			Before:                maskAuditPayload(item.BeforeData),
			After:                 maskAuditPayload(item.AfterData),
			Result:                item.Result,
			Reason:                item.Reason,
			ActorName:             item.ActorName,
			IP:                    maskIP(item.IP),
			UserAgent:             maskUserAgent(item.UserAgent),
			SensitiveFieldsMasked: maskedFields,
			CreatedAt:             item.CreatedAt,
			ContractGaps:          buildAuditContractGaps(item),
		}
		result = append(result, resp)
	}
	return result
}

func parseAuditEventFilter(c *app.RequestContext) (model.KBAuditEventListFilter, error) {
	var filter model.KBAuditEventListFilter

	if action := strings.TrimSpace(string(c.Query("action"))); action != "" {
		filter.Action = &action
	}
	if resourceType := strings.TrimSpace(string(c.Query("resource_type"))); resourceType != "" {
		filter.ResourceType = &resourceType
	}
	if actorRaw := strings.TrimSpace(string(c.Query("actor_id"))); actorRaw != "" {
		value, err := parseUint64(actorRaw, "actor_id")
		if err != nil {
			return filter, err
		}
		actorID := uint(value)
		filter.ActorID = &actorID
	}
	if kbRaw := strings.TrimSpace(string(c.Query("kb_id"))); kbRaw != "" {
		value, err := parseUint64(kbRaw, "kb_id")
		if err != nil {
			return filter, err
		}
		filter.KBID = &value
	}
	if requestID := strings.TrimSpace(string(c.Query("request_id"))); requestID != "" {
		filter.RequestID = &requestID
	}
	if documentRaw := strings.TrimSpace(string(c.Query("document_id"))); documentRaw != "" {
		value, err := parseUint64(documentRaw, "document_id")
		if err != nil {
			return filter, err
		}
		filter.DocumentID = &value
	}
	startTime, err := parseOptionalRFC3339Query(c, "start_time")
	if err != nil {
		return filter, err
	}
	endTime, err := parseOptionalRFC3339Query(c, "end_time")
	if err != nil {
		return filter, err
	}
	if startTime != nil && endTime != nil && startTime.After(*endTime) {
		return filter, myerrors.NewValidationError("start_time cannot be later than end_time")
	}
	filter.StartTime = startTime
	filter.EndTime = endTime
	return filter, nil
}

func buildAuditEventFilterFromExport(req auditEventExportRequest) (model.KBAuditEventListFilter, error) {
	filter := model.KBAuditEventListFilter{}
	if value := strings.TrimSpace(req.Action); value != "" {
		filter.Action = &value
	}
	if value := strings.TrimSpace(req.ResourceType); value != "" {
		filter.ResourceType = &value
	}
	if req.ActorID > 0 {
		actorID := req.ActorID
		filter.ActorID = &actorID
	}
	if req.KBID > 0 {
		kbID := req.KBID
		filter.KBID = &kbID
	}
	if value := strings.TrimSpace(req.RequestID); value != "" {
		filter.RequestID = &value
	}
	if req.DocumentID > 0 {
		documentID := req.DocumentID
		filter.DocumentID = &documentID
	}
	if value := strings.TrimSpace(req.StartTime); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return filter, myerrors.NewValidationError("start_time must use RFC3339")
		}
		parsed = parsed.UTC()
		filter.StartTime = &parsed
	}
	if value := strings.TrimSpace(req.EndTime); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return filter, myerrors.NewValidationError("end_time must use RFC3339")
		}
		parsed = parsed.UTC()
		filter.EndTime = &parsed
	}
	if filter.StartTime != nil && filter.EndTime != nil && filter.StartTime.After(*filter.EndTime) {
		return filter, myerrors.NewValidationError("start_time cannot be later than end_time")
	}
	return filter, nil
}

func parseSensitiveFieldsMasked(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err == nil {
		return items
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, item := range parts {
		value := strings.TrimSpace(item)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func buildAuditContractGaps(item *model.KBAuditEvent) []string {
	gaps := make([]string, 0, 4)
	if strings.TrimSpace(item.ActorName) == "" {
		gaps = append(gaps, "actor_name")
	}
	if strings.TrimSpace(item.IP) == "" {
		gaps = append(gaps, "ip")
	}
	if strings.TrimSpace(item.UserAgent) == "" {
		gaps = append(gaps, "user_agent")
	}
	if strings.TrimSpace(item.SensitiveFieldsMasked) == "" {
		gaps = append(gaps, "sensitive_fields_masked")
	}
	return gaps
}

func maskAuditPayload(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	replacements := []string{"query", "content", "snippet", "before", "after"}
	masked := value
	for _, keyword := range replacements {
		if strings.Contains(lower, keyword) {
			masked = "[masked]"
			break
		}
	}
	return masked
}

func maskIP(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.Count(value, ".") == 3 {
		parts := strings.Split(value, ".")
		if len(parts) == 4 {
			return parts[0] + "." + parts[1] + ".*.*"
		}
	}
	return "[masked]"
}

func maskUserAgent(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if len(value) <= 24 {
		return "[masked]"
	}
	return value[:24] + "..."
}
