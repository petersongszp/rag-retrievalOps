package transport

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RunStdio(ctx context.Context, server *mcp.Server) error {
	return server.Run(ctx, &mcp.StdioTransport{})
}
