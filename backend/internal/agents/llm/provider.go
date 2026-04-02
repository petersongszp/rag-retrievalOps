package llm

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/model"

	"interview-agents/internal/config"
	"interview-agents/internal/errors"
	"interview-agents/internal/observability/looptrace"
	"interview-agents/pkg/circuitbreaker"
)

var (
	baseTransport *http.Transport
	transportOnce sync.Once
)

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

func getBaseTransport() *http.Transport {
	transportOnce.Do(func() {
		baseTransport = &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   20,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			DisableKeepAlives:     false,
			ForceAttemptHTTP2:     false,
		}
	})
	return baseTransport
}

type providerConfig struct {
	apiKey       string
	baseURL      string
	modelName    string
	providerName string
	protocol     string
}

func parseLLMConfig() (*providerConfig, error) {
	cfg := config.Global.LLM

	key := strings.TrimSpace(cfg.APIKey)
	if len(key) < 10 {
		return nil, errors.NewInvalidParamError("LLM API Key not configured or too short. Please check LLM_API_KEY in .env file.")
	}

	modelName := strings.TrimSpace(cfg.ModelName)
	if modelName == "" {
		return nil, errors.NewInvalidParamError("LLM Model Name not configured. Please check LLM_MODEL_NAME in .env file.")
	}

	providerName := strings.TrimSpace(cfg.ProviderName)
	protocol := resolveProtocol(providerName)

	rawURL := cfg.BaseURL
	cleanedURL := strings.Map(func(r rune) rune {
		if r > 126 || r < 33 {
			return -1
		}
		return r
	}, rawURL)

	cleanedURL = strings.TrimSuffix(cleanedURL, "/chat/completions")
	cleanedURL = strings.TrimSuffix(cleanedURL, "/")

	fmt.Printf("[LLM] Provider: %s, Protocol: %s, BaseURL: %q, Model: %s\n",
		providerName, protocol, cleanedURL, modelName)

	return &providerConfig{
		apiKey:       key,
		baseURL:      cleanedURL,
		modelName:    modelName,
		providerName: providerName,
		protocol:     protocol,
	}, nil
}

func resolveProtocol(providerName string) string {
	switch strings.ToLower(strings.TrimSpace(providerName)) {
	case "gemini":
		return "gemini"
	case "ark", "volcengine", "doubao":
		return "ark"
	case "openai", "deepseek", "ollama", "qwen", "groq", "grok":
		return "openai"
	default:
		return "openai"
	}
}

func buildHTTPClient(cacheKey string, modelName string) *http.Client {
	httpClient := getCachedClient(cacheKey)
	if httpClient == nil {
		cb := circuitbreaker.NewCircuitBreaker(fmt.Sprintf("llm-%s", modelName))
		httpClient = &http.Client{
			Timeout: 0,
			Transport: &loggingTransport{
				Transport:      getBaseTransport(),
				CircuitBreaker: cb,
			},
		}
		setCachedClient(cacheKey, httpClient)
	}
	return httpClient
}

type loggingTransport struct {
	Transport      http.RoundTripper
	CircuitBreaker *circuitbreaker.CircuitBreaker
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	fmt.Printf("[LLM Debug] Requesting: %s %s\n", req.Method, req.URL.String())

	if traceHeaders, err := looptrace.TraceHeaders(req.Context()); err == nil {
		for key, value := range traceHeaders {
			req.Header.Set(key, value)
		}
	}

	result, err := t.CircuitBreaker.Execute(func() (interface{}, error) {
		resp, err := t.Transport.RoundTrip(req)
		if err != nil {
			fmt.Printf("[LLM Debug] Request failed: %v\n", err)
			return nil, err
		}
		if resp.StatusCode >= 500 || resp.StatusCode == 429 {
			return resp, fmt.Errorf("server error or rate limit: %s", resp.Status)
		}
		return resp, nil
	})

	if err != nil {
		if err == circuitbreaker.ErrOpen || err == circuitbreaker.ErrTooMany {
			return nil, err
		}
		return nil, err
	}

	resp := result.(*http.Response)
	fmt.Printf("[LLM Debug] Response Status: %s\n", resp.Status)
	return resp, nil
}

func wrapModelErr(err error, msg string) error {
	if err == nil {
		return nil
	}
	errMsg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errMsg, "insufficient_quota") ||
		strings.Contains(errMsg, "billing_not_active") ||
		strings.Contains(errMsg, "quota_exceeded") ||
		strings.Contains(errMsg, "insufficient tokens"):
		return errors.NewInsufficientTokensError("Model API: Insufficient tokens or quota exceeded.", err)
	case strings.Contains(errMsg, "rate_limit_exceeded") ||
		strings.Contains(errMsg, "too_many_requests") ||
		strings.Contains(errMsg, "rate limit"):
		return errors.NewRateLimitExceededError("Model API: Rate limit exceeded.", err)
	case strings.Contains(errMsg, "context_length_exceeded") ||
		strings.Contains(errMsg, "maximum context length") ||
		strings.Contains(errMsg, "token limit"):
		return errors.NewContextLengthExceededError("Model API: Context length exceeded.", err)
	default:
		return errors.NewOpenAIError(msg, err)
	}
}

func CreatOpenAiChatModel(ctx context.Context, userId uint) (model.ToolCallingChatModel, error) {
	pc, err := parseLLMConfig()
	if err != nil {
		return nil, err
	}

	var chatModel model.ToolCallingChatModel

	switch pc.protocol {
	case "gemini":
		chatModel, err = createGeminiModel(ctx, pc)
	case "ark":
		chatModel, err = createArkModel(ctx, pc)
	default:
		chatModel, err = createOpenAIModel(ctx, pc)
	}

	if err != nil {
		return nil, err
	}

	return &tracedChatModel{
		inner:        chatModel,
		userID:       userId,
		modelName:    pc.modelName,
		protocol:     pc.protocol,
		providerName: pc.providerName,
		baseURL:      pc.baseURL,
	}, nil
}
