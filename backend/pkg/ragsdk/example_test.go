package ragsdk_test

import (
	"context"
	"errors"
	"fmt"

	"interview-agents/pkg/ragsdk"
)

func ExampleClient_Retrieve() {
	client := ragsdk.NewClient(ragsdk.ClientConfig{
		BaseURL: "http://localhost:8081",
		APIKey:  "rag_xxxxxxxxxxxx",
	})

	resp, err := client.Retrieve(context.Background(), ragsdk.RetrieveRequest{
		Query: "Go 并发编程最佳实践是什么？",
		KBIDs: []uint64{1},
		TopK:  3,
	})
	if err != nil {
		var apiErr *ragsdk.APIError
		if errors.As(err, &apiErr) {
			fmt.Printf("API Error: %d %s\n", apiErr.StatusCode, apiErr.Body)
			return
		}
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Request ID: %s\n", resp.RequestID)
	for _, item := range resp.Items {
		fmt.Printf("- [%.2f] %s\n", item.Score, item.Content)
	}
}
