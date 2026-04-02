package llm

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/gemini"
	"google.golang.org/genai"
)

func createGeminiModel(ctx context.Context, pc *providerConfig) (*gemini.ChatModel, error) {
	clientConfig := &genai.ClientConfig{
		APIKey: pc.apiKey,
	}
	if pc.baseURL != "" {
		clientConfig.HTTPOptions = genai.HTTPOptions{
			BaseURL: pc.baseURL,
		}
	}

	client, err := genai.NewClient(ctx, clientConfig)
	if err != nil {
		return nil, wrapModelErr(err, "Failed to create Gemini client")
	}

	cacheKey := fmt.Sprintf("gemini:%s:%s", pc.modelName, pc.baseURL)
	httpClient := buildHTTPClient(cacheKey, pc.modelName)
	_ = httpClient

	chatModel, err := gemini.NewChatModel(ctx, &gemini.Config{
		Client: client,
		Model:  pc.modelName,
	})
	if err != nil {
		return nil, wrapModelErr(err, "Failed to create Gemini chat model")
	}

	return chatModel, nil
}
