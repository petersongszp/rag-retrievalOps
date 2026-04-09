package interview

import (
	"context"

	"interview-agents/api/response"
	"interview-agents/internal/middleware"
	"interview-agents/internal/model"

	"github.com/cloudwego/hertz/pkg/app"
)

type deletePredictionRequest struct {
	IDs []uint64 `json:"ids"`
}

// DeletePredictionRecords handles batch deletion for prediction records.
func DeletePredictionRecords(ctx context.Context, c *app.RequestContext) {
	var req deletePredictionRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}

	if len(req.IDs) == 0 {
		response.BadRequest(ctx, c, "ids is required")
		return
	}

	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(ctx, c, "Authorization token is required")
		return
	}

	if err := model.PredictionDao.DeletePredictionRecordsByUserID(userID, req.IDs); err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}

	response.Success(ctx, c, map[string]any{"deleted_ids": req.IDs})
}
