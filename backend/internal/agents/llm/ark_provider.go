package llm

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/ark"
)

func createArkModel(ctx context.Context, pc *providerConfig) (*ark.ChatModel, error) {
	cacheKey := fmt.Sprintf("ark:%s:%s", pc.modelName, pc.baseURL)
	httpClient := buildHTTPClient(cacheKey, pc.modelName)

	chatModel, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
		APIKey:     pc.apiKey,
		Model:      pc.modelName,
		BaseURL:    pc.baseURL,
		HTTPClient: httpClient,
	})
	if err != nil {
		return nil, wrapModelErr(err, "Failed to create Ark chat model")
	}

	return chatModel, nil
}
