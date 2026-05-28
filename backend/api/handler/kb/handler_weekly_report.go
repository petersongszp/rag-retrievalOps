package kb

import (
	"context"

	"interview-agents/api/response"

	"github.com/cloudwego/hertz/pkg/app"
)

func GetWeeklyReport(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}
	report, err := buildWeeklyReport()
	if err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}
	response.Success(ctx, c, report)
}
