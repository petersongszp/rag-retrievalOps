package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	internalmcp "interview-agents/internal/mcp"
	"interview-agents/internal/mcp/transport"
)

func main() {
	logger := log.New(os.Stderr, "[rag-mcp-server] ", log.LstdFlags)
	if err := run(logger); err != nil {
		logger.Printf("fatal: %v", err)
		os.Exit(1)
	}
}

func run(logger *log.Logger) error {
	flags := flag.NewFlagSet("rag-mcp-server", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	transportOverride := flags.String("transport", "", "MCP transport (stdio or http)")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return err
	}

	cfg, err := internalmcp.LoadConfigFromEnv()
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	if *transportOverride != "" {
		cfg.Transport = *transportOverride
		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("invalid configuration: %w", err)
		}
	}

	server, err := internalmcp.NewServerFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("initialize MCP server: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cfg.Transport {
	case "stdio":
		logger.Printf("starting stdio transport")
		if err := transport.RunStdio(ctx, server); err != nil && ctx.Err() == nil {
			return fmt.Errorf("run stdio transport: %w", err)
		}
		return nil
	case "http":
		if err := transport.RunHTTP(ctx, cfg, server, logger); err != nil && ctx.Err() == nil {
			return fmt.Errorf("run http transport: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported transport %q", cfg.Transport)
	}
}
