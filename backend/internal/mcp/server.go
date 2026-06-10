package mcp

import (
	"fmt"

	ragclient "interview-agents/internal/mcp/client"
	"interview-agents/internal/mcp/handler"
	"interview-agents/internal/mcp/tools"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const serverVersion = "0.1.0"

func NewServer(retriever handler.Retriever) (*mcpsdk.Server, error) {
	if retriever == nil {
		return nil, fmt.Errorf("retriever is required")
	}

	server := mcpsdk.NewServer(
		&mcpsdk.Implementation{
			Name:    "rag-mcp-server",
			Title:   "RAG Retrieval MCP Server",
			Version: serverVersion,
		},
		&mcpsdk.ServerOptions{
			Capabilities: &mcpsdk.ServerCapabilities{},
		},
	)
	retrieveHandler := handler.NewRetrieveHandler(retriever)
	mcpsdk.AddTool[handler.RetrieveKnowledgeInput, handler.RetrieveKnowledgeOutput](
		server,
		tools.RetrieveKnowledge(),
		retrieveHandler.Handle,
	)
	return server, nil
}

func NewServerFromConfig(cfg Config) (*mcpsdk.Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	ragClient, err := ragclient.New(cfg.RAGBaseURL, cfg.RAGAccessToken, cfg.Timeout)
	if err != nil {
		return nil, fmt.Errorf("create RAG client: %w", err)
	}
	return NewServer(ragClient)
}
