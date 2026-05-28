package kb

import (
	"context"

	"interview-agents/api/response"
	myerrors "interview-agents/internal/errors"
	"interview-agents/internal/model"

	"github.com/cloudwego/hertz/pkg/app"
)

type auditEventListResponse struct {
	Items []*model.KBAuditEvent `json:"items"`
}

func ListAuditEvents(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}
	items, err := model.KBAuditEventDao.List(100)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to list audit events", err))
		return
	}
	response.Success(ctx, c, auditEventListResponse{Items: items})
}
