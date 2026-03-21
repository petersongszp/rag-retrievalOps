package llm

import (
	"context"
	"io"
	"net/url"
	"strconv"
	"strings"

	cozeloop "github.com/coze-dev/cozeloop-go"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"
	"github.com/cloudwego/eino/components/model"
	"interview-agents/internal/observability/looptrace"
	mycallbacks "interview-agents/pkg/eino/callbacks"

	"github.com/cloudwego/eino/schema"
)

type tracedChatModel struct {
	inner        model.ToolCallingChatModel
	userID       uint
	modelName    string
	protocol     string
	providerName string
	baseURL      string
}

func (t *tracedChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	ctx, span := t.startModelSpan(ctx, "llm.generate", input)
	resp, err := t.inner.Generate(ctx, input, opts...)
	t.finishModelSpan(ctx, span, resp, err)
	return resp, err
}

func (t *tracedChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	ctx, span := t.startModelSpan(ctx, "llm.stream", input)
	stream, err := t.inner.Stream(ctx, input, opts...)
	if err != nil {
		t.finishModelSpan(ctx, span, nil, err)
		return nil, err
	}

	out, writer := schema.Pipe[*schema.Message](16)
	go func() {
		defer writer.Close()
		defer stream.Close()

		chunks := make([]*schema.Message, 0, 16)
		for {
			chunk, recvErr := stream.Recv()
			switch {
			case recvErr == nil:
				chunks = append(chunks, chunk)
				if closed := writer.Send(chunk, nil); closed {
					t.finishModelSpan(ctx, span, nil, nil)
					return
				}
			case recvErr == io.EOF:
				if len(chunks) > 0 {
					if merged, mergeErr := schema.ConcatMessages(chunks); mergeErr == nil {
						t.finishModelSpan(ctx, span, merged, nil)
					} else {
						t.finishModelSpan(ctx, span, nil, mergeErr)
					}
				} else {
					t.finishModelSpan(ctx, span, nil, nil)
				}
				return
			default:
				_ = writer.Send(nil, recvErr)
				t.finishModelSpan(ctx, span, nil, recvErr)
				return
			}
		}
	}()

	return out, nil
}

func (t *tracedChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	toolModel, err := t.inner.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return &tracedChatModel{
		inner:        toolModel,
		userID:       t.userID,
		modelName:    t.modelName,
		protocol:     t.protocol,
		providerName: t.providerName,
		baseURL:      t.baseURL,
	}, nil
}

func (t *tracedChatModel) startModelSpan(ctx context.Context, spanName string, input []*schema.Message) (context.Context, cozeloop.Span) {
	nextCtx, span, ok := looptrace.StartSpan(ctx, spanName, tracespec.VModelSpanType)
	if !ok || span == nil {
		return ctx, nil
	}

	userID := mycallbacks.UserIDFromContext(nextCtx)
	if userID == 0 {
		userID = t.userID
	}

	provider := strings.TrimSpace(t.protocol)
	if provider == "" {
		provider = strings.ToLower(strings.TrimSpace(t.providerName))
	}
	if provider == "" {
		provider = "openai"
	}

	looptrace.ApplyCommonFields(nextCtx, span, strconv.FormatUint(uint64(userID), 10), mycallbacks.TraceIDFromContext(nextCtx), map[string]interface{}{
		"provider_name": t.providerName,
		"protocol":      t.protocol,
		"base_url_host": parseBaseURLHost(t.baseURL),
	})
	span.SetModelProvider(nextCtx, provider)
	span.SetModelName(nextCtx, t.modelName)
	span.SetInput(nextCtx, traceMessages(input))

	return nextCtx, span
}

func (t *tracedChatModel) finishModelSpan(ctx context.Context, span cozeloop.Span, resp *schema.Message, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.SetError(ctx, err)
		span.Finish(ctx)
		return
	}
	if resp != nil {
		span.SetOutput(ctx, traceMessage(resp))
		if usage := resp.ResponseMeta; usage != nil && usage.Usage != nil {
			span.SetInputTokens(ctx, usage.Usage.PromptTokens)
			span.SetOutputTokens(ctx, usage.Usage.CompletionTokens)
		}
	}
	span.Finish(ctx)
}

func traceMessages(messages []*schema.Message) []map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			continue
		}
		items = append(items, traceMessage(message))
	}
	return items
}

func traceMessage(message *schema.Message) map[string]interface{} {
	if message == nil {
		return map[string]interface{}{}
	}
	item := map[string]interface{}{
		"role":    string(message.Role),
		"content": message.Content,
	}
	if message.Name != "" {
		item["name"] = message.Name
	}
	if message.ReasoningContent != "" {
		item["reasoning_content"] = message.ReasoningContent
	}
	if len(message.ToolCalls) > 0 {
		item["tool_calls"] = len(message.ToolCalls)
	}
	if message.ResponseMeta != nil && message.ResponseMeta.Usage != nil {
		item["usage"] = map[string]int{
			"prompt_tokens":     message.ResponseMeta.Usage.PromptTokens,
			"completion_tokens": message.ResponseMeta.Usage.CompletionTokens,
			"total_tokens":      message.ResponseMeta.Usage.TotalTokens,
		}
	}
	return item
}

func parseBaseURLHost(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return parsed.Host
}
