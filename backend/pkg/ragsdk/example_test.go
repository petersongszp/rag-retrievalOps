package ragsdk_test

import (
	"context"
	"fmt"

	"interview-agents/pkg/ragsdk"
)

func ExampleClient_Retrieve() {
	client := ragsdk.NewClient(ragsdk.ClientConfig{
		BaseURL: "http://localhost:8081",
		AppID:   "example-app",
	})

	resp, err := client.Retrieve(context.Background(), ragsdk.RetrieveRequest{
		Query: "Go 并发编程最佳实践",
		KBIDs: []uint64{1},
		TopK:  3,
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Request ID: %s\n", resp.RequestID)
	for _, item := range resp.Items {
		fmt.Printf("- [%.2f] %s\n", item.Score, item.Content[:50])
	}
}
