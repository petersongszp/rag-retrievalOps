package llm

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/openai"
)

func createOpenAIModel(ctx context.Context, pc *providerConfig) (*openai.ChatModel, error) {
	cacheKey := fmt.Sprintf("openai:%s:%s", pc.modelName, pc.baseURL)
	httpClient := buildHTTPClient(cacheKey, pc.modelName)

	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:     pc.apiKey,
		Model:      pc.modelName,
		BaseURL:    pc.baseURL,
		HTTPClient: httpClient,
	})
	if err != nil {
		return nil, wrapModelErr(err, "Failed to create OpenAI chat model")
	}

	return chatModel, nil
}
