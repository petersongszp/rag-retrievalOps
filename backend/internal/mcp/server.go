package mcp

import (
	"fmt"
	"strings"

	ragclient "interview-agents/internal/mcp/client"
	"interview-agents/internal/mcp/handler"
	"interview-agents/internal/mcp/tools"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const serverVersion = "0.2.0"

func NewServer(retrieverFactory handler.RetrieverFactory) (*mcpsdk.Server, error) {
	if retrieverFactory == nil {
		return nil, fmt.Errorf("retrieverFactory is required")
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
	retrieveHandler := handler.NewRetrieveHandler(retrieverFactory)
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
	return NewServer(NewRetrieverFactoryFromConfig(cfg))
}

func NewRetrieverFactoryFromConfig(cfg Config) handler.RetrieverFactory {
	return handler.RetrieverFactoryFunc(func(req *mcpsdk.CallToolRequest) (handler.Retriever, error) {
		authorization := strings.TrimSpace(cfg.StdioAuthorizationHeader())
		if req != nil && req.Extra != nil && req.Extra.Header != nil {
			if header := strings.TrimSpace(req.Extra.Header.Get("Authorization")); header != "" {
				authorization = header
			}
		}
		if authorization == "" {
			return nil, fmt.Errorf("unauthorized: Authorization header is required")
		}
		return ragclient.NewWithAuthorization(cfg.RAGBaseURL, authorization, cfg.Timeout)
	})
}

func (c Config) StdioAuthorizationHeader() string {
	if strings.TrimSpace(c.RAGAccessToken) == "" {
		return ""
	}
	return "Bearer " + strings.TrimSpace(c.RAGAccessToken)
}
