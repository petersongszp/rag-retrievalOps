package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"interview-agents/api/ragrouter"
	"interview-agents/internal/config"
	appMiddleware "interview-agents/internal/middleware"
	"interview-agents/internal/milvus"
	ragmq "interview-agents/internal/ragplatform/mq"
	"interview-agents/internal/ragqueue"
	"interview-agents/internal/repository"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/joho/godotenv"
)

func main() {
	log.Println("[RAG-Server] Starting RAG Platform server...")

	envPath := findEnvFile()
	if envPath != "" {
		if err := godotenv.Load(envPath); err != nil {
			log.Printf("[RAG-Server] Warning: Could not load .env file at %s: %v", envPath, err)
		} else {
			log.Printf("[RAG-Server] Loaded .env file: %s", envPath)
		}
	}

	configPath := findConfigFile()
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("[RAG-Server] Failed to load config: %v", err)
	}
	if err := cfg.ValidateRAGPrerequisites(); err != nil {
		log.Fatalf("[RAG-Server] Invalid RAG configuration: %v", err)
	}
	log.Printf("[RAG-Server] Config loaded: env=%s rag.enabled=%t", cfg.RAG.Environment, cfg.RAG.Enabled)

	log.Println("[RAG-Server] Initializing database...")
	if err := repository.InitDatabaseOnly(cfg.Database); err != nil {
		log.Fatalf("[RAG-Server] Failed to init database: %v", err)
	}
	if err := repository.MigrateRAGDatabase(repository.GetDB()); err != nil {
		log.Fatalf("[RAG-Server] Failed to migrate RAG database: %v", err)
	}
	log.Println("[RAG-Server] Database initialized (RAG tables only)")

	log.Println("[RAG-Server] Initializing Redis...")
	if err := repository.InitRedis(cfg.Redis); err != nil {
		log.Fatalf("[RAG-Server] Failed to init Redis: %v", err)
	}
	log.Println("[RAG-Server] Redis initialized")

	var messageQueue ragqueue.MessageQueue
	var consumerCtx context.Context
	var cancelConsumer context.CancelFunc
	redisClient := repository.GetRedis()
	if redisClient != nil {
		log.Println("[RAG-Server] Initializing MQ...")
		messageQueue = ragqueue.NewRedisStreamQueue(redisClient, "rag-consumer-group", fmt.Sprintf("rag-consumer-%s", cfg.Host))
		ragqueue.InitMessageQueue(messageQueue)

		ragConsumer := ragmq.NewRAGConsumer(messageQueue)
		consumerCtx, cancelConsumer = context.WithCancel(context.Background())
		go func() {
			if err := ragConsumer.Start(consumerCtx); err != nil {
				log.Printf("[RAG-Server] RAG consumer stopped: %v", err)
			}
		}()
		log.Println("[RAG-Server] RAG MQ consumer started")
	} else {
		log.Println("[RAG-Server] Warning: Redis client not available, skipping MQ initialization")
	}

	var milvusManager *milvus.MilvusManager
	if cfg.RAG.Enabled {
		log.Println("[RAG-Server] Initializing Milvus Manager...")
		milvusCtx := context.Background()
		milvusManager, err = milvus.InitMilvusManager(milvusCtx, cfg)
		if err != nil {
			log.Fatalf("[RAG-Server] Failed to init Milvus Manager: %v", err)
		}
		if err := milvusManager.HealthCheck(milvusCtx); err != nil {
			log.Fatalf("[RAG-Server] Milvus health check failed: %v", err)
		}
		log.Println("[RAG-Server] Milvus Manager initialized and healthy")
	} else {
		log.Println("[RAG-Server] RAG disabled, skipping Milvus initialization")
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	h := server.Default(
		server.WithHostPorts(addr),
		server.WithReadTimeout(3*time.Second),
		server.WithWriteTimeout(3*time.Second),
	)

	// Keep admin routes usable in rag-server mode and populate user context
	// for token-based requests handled by shared KB handlers.
	h.Use(func(ctx context.Context, c *app.RequestContext) {
		if string(c.Method()) == consts.MethodOptions {
			c.Next(ctx)
			return
		}

		path := strings.TrimSuffix(string(c.Path()), "/")
		if path == "" {
			path = "/"
		}

		if strings.HasPrefix(path, "/api/admin/") {
			c.Set("user_id", uint(1))
			c.Set("role", "admin")
			c.Set("username", "admin")
			c.Next(ctx)
			return
		}

		appMiddleware.ParseAndSetUserFromToken(c)
		c.Next(ctx)
	})

	ragrouter.Register(h)

	h.GET("/healthz", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	h.GET("/readyz", func(ctx context.Context, c *app.RequestContext) {
		status := "ready"
		if milvusManager != nil {
			if err := milvusManager.HealthCheck(ctx); err != nil {
				status = "degraded"
				log.Printf("[RAG-Server] Readyz check: Milvus unhealthy: %v", err)
			}
		}
		c.JSON(http.StatusOK, map[string]string{"status": status})
	})

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("[RAG-Server] Server listening on %s", addr)
		if err := h.Run(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[RAG-Server] Failed to start server: %v", err)
		}
	}()

	<-quit
	log.Println("[RAG-Server] Shutting down...")

	if cancelConsumer != nil {
		cancelConsumer()
		log.Println("[RAG-Server] MQ consumer stopped")
	}
	if messageQueue != nil {
		if err := messageQueue.Close(); err != nil {
			log.Printf("[RAG-Server] Warning: Failed to close MQ: %v", err)
		}
	}

	if milvusManager != nil {
		if err := milvusManager.Close(); err != nil {
			log.Printf("[RAG-Server] Warning: Failed to close Milvus: %v", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("[RAG-Server] Server forced to shutdown: %v", err)
	}

	log.Println("[RAG-Server] Server exited")
}

func findConfigFile() string {
	if wd, err := os.Getwd(); err == nil {
		path1 := filepath.Join(wd, "config.yaml")
		if _, err := os.Stat(path1); err == nil {
			return path1
		}
		path2 := filepath.Join(wd, "..", "..", "config.yaml")
		if _, err := os.Stat(path2); err == nil {
			return path2
		}
	}

	_, currentFile, _, ok := runtime.Caller(0)
	if ok {
		serverDir := filepath.Dir(currentFile)
		cmdDir := filepath.Dir(serverDir)
		backendDir := filepath.Dir(cmdDir)
		configPath := filepath.Join(backendDir, "config.yaml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}
	}

	log.Println("[RAG-Server] Warning: Could not find config.yaml, defaulting to 'config.yaml'")
	return "config.yaml"
}

func findEnvFile() string {
	if wd, err := os.Getwd(); err == nil {
		candidates := []string{
			filepath.Join(wd, ".env"),
			filepath.Join(wd, "..", ".env"),
			filepath.Join(wd, "..", "..", ".env"),
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}

	_, currentFile, _, ok := runtime.Caller(0)
	if ok {
		projectRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
		envPath := filepath.Join(projectRoot, ".env")
		if _, err := os.Stat(envPath); err == nil {
			return envPath
		}
	}
	return ""
}
