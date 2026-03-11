package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

// Logging Transport from openAi.go
type loggingTransport struct {
	Transport http.RoundTripper
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	fmt.Printf("[Debug] Requesting: %s %s\n", req.Method, req.URL.String())
	resp, err := t.Transport.RoundTrip(req)
	if err != nil {
		fmt.Printf("[Debug] Request failed: %v\n", err)
		return nil, err
	}
	fmt.Printf("[Debug] Response Status: %s\n", resp.Status)
	return resp, nil
}

func main() {
	fmt.Println("=== SDK Diagnostic Tool ===")

	baseURL := "https://ark.cn-beijing.volces.com/api/v3" // SDK appends /chat/completions
	modelName := "ep-20240604123456-12345"                // Dummy
	apiKey := "sk-dummy-key"

	// Create a custom HTTP client EXACTLY like openAi.go
	httpClient := &http.Client{
		Timeout: 120 * time.Second,
		Transport: &loggingTransport{
			Transport: &http.Transport{
				DisableKeepAlives:     true,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 60 * time.Second,
				ForceAttemptHTTP2:     false,
			},
		},
	}

	fmt.Println("Initializing ChatModel...")
	chatModel, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
		APIKey:     apiKey,
		Model:      modelName,
		BaseURL:    baseURL,
		HTTPClient: httpClient,
	})
	if err != nil {
		fmt.Printf("Failed to create chat model: %v\n", err)
		return
	}

	largeContent := strings.Repeat("This is a test sentence to make the payload larger. ", 200) // ~10KB
	msgs := []*schema.Message{
		{
			Role:    schema.User,
			Content: largeContent,
		},
	}

	fmt.Println("Generating...")
	resp, err := chatModel.Generate(context.Background(), msgs)
	if err != nil {
		fmt.Printf("Generate ERROR: %v\n", err)
		// Check if it's EOF
		if strings.Contains(err.Error(), "EOF") {
			fmt.Println("!!! REPRODUCED EOF !!!")
		}
	} else {
		fmt.Printf("Generate SUCCESS: %s\n", resp.Content)
	}
}
