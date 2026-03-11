package middleware

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"

	"interview-agents/api/response"
	"interview-agents/internal/alert"
	"interview-agents/internal/errors"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
)

// Recovery 恢复中间件，捕获 Panic
func Recovery() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				log.Printf("[PANIC] Recovered: %v\n%s", r, stack)

				// 发送飞书告警
				go func() {
					method := string(c.Method())
					path := string(c.Path())
					clientIP := c.ClientIP()
					timestamp := time.Now().Format("2006-01-02 15:04:05")

					content := fmt.Sprintf("**时间**: %s\n**方法**: %s\n**路径**: %s\n**IP**: %s\n**错误**: %v\n\n**堆栈**:\n```go\n%s\n```",
						timestamp, method, path, clientIP, r, string(stack))

					_ = alert.SendFeishuCard("🔴 系统发生 Panic", content, "red")
				}()

				appErr := errors.NewInternalError(
					"Internal server error",
					fmt.Errorf("panic: %v\nStack:\n%s", r, string(stack)),
				)

				response.Error(ctx, c, appErr.HTTPStatus, appErr.Message)
			}
		}()

		c.Next(ctx)
	}
}
