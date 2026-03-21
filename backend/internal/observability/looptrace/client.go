package looptrace

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	cozeloop "github.com/coze-dev/cozeloop-go"
)

const (
	envEnabled       = "COZELOOP_ENABLED"
	envServiceName   = "COZELOOP_SERVICE_NAME"
	envDeploymentEnv = "COZELOOP_DEPLOYMENT_ENV"
)

var (
	initOnce sync.Once
	initErr  error

	client          cozeloop.Client
	enabled         bool
	serviceName     string
	deploymentEnv   string
)

// InitFromEnv 按环境变量初始化 CozeLoop 客户端。
// SDK 自身会读取 COZELOOP_API_BASE_URL / COZELOOP_WORKSPACE_ID / COZELOOP_API_TOKEN，
// 这里额外处理业务层的启用开关与公共维度。
func InitFromEnv() error {
	initOnce.Do(func() {
		rawEnabled := strings.TrimSpace(os.Getenv(envEnabled))
		if rawEnabled == "" {
			return
		}

		parsedEnabled, err := strconv.ParseBool(rawEnabled)
		if err != nil {
			initErr = fmt.Errorf("parse %s failed: %w", envEnabled, err)
			return
		}
		if !parsedEnabled {
			return
		}

		serviceName = strings.TrimSpace(os.Getenv(envServiceName))
		deploymentEnv = strings.TrimSpace(os.Getenv(envDeploymentEnv))

		client, err = cozeloop.NewClient(
			cozeloop.WithPromptTrace(true),
		)
		if err != nil {
			initErr = err
			return
		}

		enabled = true
		log.Printf("[CozeLoop] initialized, service=%q env=%q", serviceName, deploymentEnv)
	})

	return initErr
}

func Enabled() bool {
	_ = InitFromEnv()
	return enabled && client != nil
}

func Close(ctx context.Context) {
	if !Enabled() {
		return
	}
	client.Close(ctx)
}

func StartSpan(ctx context.Context, name, spanType string, opts ...cozeloop.StartSpanOption) (context.Context, cozeloop.Span, bool) {
	if !Enabled() {
		return ctx, nil, false
	}
	ctx, span := client.StartSpan(ctx, name, spanType, opts...)
	return ctx, span, true
}

func GetSpanFromContext(ctx context.Context) cozeloop.Span {
	if !Enabled() {
		return nil
	}
	return client.GetSpanFromContext(ctx)
}

func TraceHeaders(ctx context.Context) (map[string]string, error) {
	span := GetSpanFromContext(ctx)
	if span == nil {
		return nil, nil
	}
	return span.ToHeader()
}

// ApplyCommonFields 给 span 补齐服务级公共字段，避免调用方重复写样板代码。
func ApplyCommonFields(ctx context.Context, span cozeloop.Span, userID, threadID string, tags map[string]interface{}) {
	if span == nil {
		return
	}
	if serviceName != "" {
		span.SetServiceName(ctx, serviceName)
	}
	if deploymentEnv != "" {
		span.SetDeploymentEnv(ctx, deploymentEnv)
	}
	if userID != "" {
		span.SetUserID(ctx, userID)
	}
	if threadID != "" {
		span.SetThreadID(ctx, threadID)
	}
	if len(tags) > 0 {
		span.SetTags(ctx, tags)
	}
}
