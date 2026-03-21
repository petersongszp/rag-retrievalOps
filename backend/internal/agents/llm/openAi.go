package llm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"

	"interview-agents/internal/errors"
	usermodel "interview-agents/internal/model"
	"interview-agents/internal/observability/looptrace"
	"interview-agents/internal/service/common"
	"interview-agents/pkg/circuitbreaker"
)

var (
	baseTransport *http.Transport
	transportOnce sync.Once
)

// clientCacheEntry 11.2.1 Client 复用：缓存条目，带 TTL 避免配置变更后长期用旧 Client
type clientCacheEntry struct {
	client    *http.Client
	createdAt time.Time
}

const clientCacheTTL = 10 * time.Minute

var (
	clientCache   = make(map[string]*clientCacheEntry)
	clientCacheMu sync.RWMutex
)

func getCachedClient(cacheKey string) *http.Client {
	clientCacheMu.RLock()
	ent := clientCache[cacheKey]
	clientCacheMu.RUnlock()
	if ent == nil {
		return nil
	}
	if time.Since(ent.createdAt) > clientCacheTTL {
		clientCacheMu.Lock()
		delete(clientCache, cacheKey)
		clientCacheMu.Unlock()
		return nil
	}
	return ent.client
}

func setCachedClient(cacheKey string, client *http.Client) {
	clientCacheMu.Lock()
	defer clientCacheMu.Unlock()
	clientCache[cacheKey] = &clientCacheEntry{client: client, createdAt: time.Now()}
}

// getBaseTransport returns a singleton http.Transport with optimized connection pooling settings
func getBaseTransport() *http.Transport {
	transportOnce.Do(func() {
		baseTransport = &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			MaxIdleConns:          100,              // Total max idle connections
			MaxIdleConnsPerHost:   20,               // Max idle connections per host
			IdleConnTimeout:       90 * time.Second, // Idle connection timeout
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			DisableKeepAlives:     false, // Enable KeepAlives for connection reuse
			ForceAttemptHTTP2:     false, // Avoid HTTP/2 framing errors
		}
	})
	return baseTransport
}

func CreatOpenAiChatModel(ctx context.Context, userId uint) (model.ToolCallingChatModel, error) {
	result, err := usermodel.UserModelDao.GetDefaultUserModel(int64(userId))
	if err != nil {

		return nil, errors.NewDBError("Failed to get user model", err)
	}
	apiKey, err := common.DecryptAPIKey(result.APIKeyEncrypted)
	if err != nil {
		return nil, errors.NewInternalError("Failed to decrypt API key", err)
	}
	key := apiKey

	// Validate API Key format (basic check)
	key = strings.TrimSpace(key)
	if len(key) < 10 || key == "123456" {
		return nil, errors.NewInvalidParamError("Invalid API Key detected (too short or default value). Please update your API Key in settings.")
	}

	//模型名称
	modelName := strings.TrimSpace(result.ModelKey)
	//api url
	rawURL := result.BaseURL
	// Filter out non-printable and non-ASCII characters to prevent "invisible char" issues
	url := strings.Map(func(r rune) rune {
		if r > 126 || r < 33 { // Keep only printable ASCII (33-126)
			return -1
		}
		return r
	}, rawURL)

	fmt.Printf("[OpenAI Debug] Raw BaseURL bytes: %v\n", []byte(rawURL))
	fmt.Printf("[OpenAI Debug] Cleaned URL: %q\n", url)
	// Remove /chat/completions if present (to avoid duplication when SDK adds it)
	url = strings.TrimSuffix(url, "/chat/completions")
	url = strings.TrimSuffix(url, "/") // Trim again in case it was .../chat/completions/

	// Log the configuration (masking key)
	// fmt.Printf("Creating OpenAI Chat Model: BaseURL=%s, Model=%s, Key=...%s\n", url, modelName, key[len(key)-4:])

	// Check if using Volcengine but model ID doesn't look like an endpoint ID
	if strings.Contains(url, "volces.com") && !strings.HasPrefix(modelName, "ep-") {
		// Just a warning log, or maybe we can hint the user in error message later
		// fmt.Printf("Warning: Volcengine usually requires Endpoint ID (starting with ep-) as Model, but got: %s\n", modelName)
	}

	// 11.2.1 Client 复用：相同 (userId, model, baseURL) 复用同一 http.Client，减少频繁创建
	cacheKey := fmt.Sprintf("%d:%s:%s", userId, modelName, url)
	httpClient := getCachedClient(cacheKey)
	if httpClient == nil {
		cb := circuitbreaker.NewCircuitBreaker(fmt.Sprintf("openai-%s", modelName))
		httpClient = &http.Client{
			Timeout: 0, // No timeout, use context deadline
			Transport: &loggingTransport{
				Transport:      getBaseTransport(),
				CircuitBreaker: cb,
			},
		}
		setCachedClient(cacheKey, httpClient)
	}

	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:     key,
		Model:      modelName,
		BaseURL:    url,
		HTTPClient: httpClient,
	})
	if err != nil {
		// Check for specific error types and wrap them appropriately
		errMsg := strings.ToLower(err.Error())
		switch {
		case strings.Contains(errMsg, "insufficient_quota") ||
			strings.Contains(errMsg, "billing_not_active") ||
			strings.Contains(errMsg, "quota_exceeded") ||
			strings.Contains(errMsg, "insufficient tokens"):
			return nil, errors.NewInsufficientTokensError("Model API: Insufficient tokens or quota exceeded. Please check your account balance.", err)

		case strings.Contains(errMsg, "rate_limit_exceeded") ||
			strings.Contains(errMsg, "too_many_requests") ||
			strings.Contains(errMsg, "rate limit"):
			return nil, errors.NewRateLimitExceededError("Model API: Rate limit exceeded. Please try again later.", err)

		case strings.Contains(errMsg, "context_length_exceeded") ||
			strings.Contains(errMsg, "maximum context length") ||
			strings.Contains(errMsg, "token limit"):
			return nil, errors.NewContextLengthExceededError("Model API: Context length exceeded. Please try with shorter input.", err)

		default:
			return nil, errors.NewOpenAIError("Failed to create OpenAI chat model", err)
		}
	}

	return &tracedChatModel{
		inner:        chatModel,
		userID:       userId,
		modelName:    modelName,
		protocol:     result.Protocol,
		providerName: result.ProviderName,
		baseURL:      url,
	}, nil
}

// loggingTransport logs the actual request URL for debugging
type loggingTransport struct {
	Transport      http.RoundTripper
	CircuitBreaker *circuitbreaker.CircuitBreaker
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	fmt.Printf("[OpenAI Debug] Requesting: %s %s\n", req.Method, req.URL.String())

	if traceHeaders, err := looptrace.TraceHeaders(req.Context()); err == nil {
		for key, value := range traceHeaders {
			req.Header.Set(key, value)
		}
	}

	// Peek at body if present
	if req.Body != nil {
		bodyBytes, err := io.ReadAll(req.Body)
		if err == nil {
			fmt.Printf("[OpenAI Debug] Body Size: %d bytes\n", len(bodyBytes))
			if len(bodyBytes) > 1000 {
				fmt.Printf("[OpenAI Debug] Body Preview: %s...\n", string(bodyBytes[:200]))
			} else {
				fmt.Printf("[OpenAI Debug] Body: %s\n", string(bodyBytes))
			}
			// Restore body
			req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
	}

	// 使用熔断器执行请求
	result, err := t.CircuitBreaker.Execute(func() (interface{}, error) {
		resp, err := t.Transport.RoundTrip(req)
		if err != nil {
			fmt.Printf("[OpenAI Debug] Request failed: %v\n", err)
			return nil, err
		}
		// 如果状态码是 5xx 或 429，视为失败，触发熔断计数
		if resp.StatusCode >= 500 || resp.StatusCode == 429 {
			return resp, fmt.Errorf("server error or rate limit: %s", resp.Status)
		}
		return resp, nil
	})

	if err != nil {
		// 如果是熔断器错误，直接返回
		if err == circuitbreaker.ErrOpen || err == circuitbreaker.ErrTooMany {
			return nil, err
		}
		// 如果是我们自己包装的错误，需要解包，但这里简单处理，如果 result 不为空，说明虽然报错但有响应（比如 500）
		// 不过 gobreaker 的 Execute 如果返回 err，result 可能是 nil。
		// 这里我们简化逻辑：如果熔断器执行报错，且不是我们预期的业务错误，就返回错误。

		// 特殊处理：上面我们在闭包里对 500/429 返回了 error，目的是让熔断器计数。
		// 但对于调用方，我们可能还是想返回 resp 让上层处理（虽然上层可能也会报错）。
		// 不过 http.Client 的 Transport 如果返回 error，Client.Do 会返回 error。
		// 所以这里如果熔断器捕获到 500/429，会导致上层收到 error。这是符合熔断预期的。
		return nil, err
	}

	resp := result.(*http.Response)
	fmt.Printf("[OpenAI Debug] Response Status: %s\n", resp.Status)
	return resp, nil
}
