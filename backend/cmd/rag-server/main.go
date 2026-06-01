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
	"syscall"
	"time"

	"interview-agents/api/router"
	"interview-agents/internal/config"
	"interview-agents/internal/milvus"
	"interview-agents/internal/repository"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/joho/godotenv"
)

func main() {
	log.Println("[RAG-Server] Starting RAG Platform server...")

	// 1. 加载 .env 文件
	envPath := findEnvFile()
	if envPath != "" {
		if err := godotenv.Load(envPath); err != nil {
			log.Printf("[RAG-Server] Warning: Could not load .env file at %s: %v", envPath, err)
		} else {
			log.Printf("[RAG-Server] Loaded .env file: %s", envPath)
		}
	}

	// 2. 加载配置
	configPath := findConfigFile()
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("[RAG-Server] Failed to load config: %v", err)
	}
	if err := cfg.ValidateRAGPrerequisites(); err != nil {
		log.Fatalf("[RAG-Server] Invalid RAG configuration: %v", err)
	}
	log.Printf("[RAG-Server] Config loaded: env=%s rag.enabled=%t", cfg.RAG.Environment, cfg.RAG.Enabled)

	// 3. 初始化数据库（不执行全量迁移）
	log.Println("[RAG-Server] Initializing database...")
	if err := repository.InitDatabaseOnly(cfg.Database); err != nil {
		log.Fatalf("[RAG-Server] Failed to init database: %v", err)
	}
	// 只迁移 RAG 相关表
	if err := repository.MigrateRAGDatabase(repository.GetDB()); err != nil {
		log.Fatalf("[RAG-Server] Failed to migrate RAG database: %v", err)
	}
	log.Println("[RAG-Server] Database initialized (RAG tables only)")

	// 4. 初始化 Redis
	log.Println("[RAG-Server] Initializing Redis...")
	if err := repository.InitRedis(cfg.Redis); err != nil {
		log.Fatalf("[RAG-Server] Failed to init Redis: %v", err)
	}
	log.Println("[RAG-Server] Redis initialized")

	// 5. 初始化 Milvus Manager（如果 RAG 启用）
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

	// 6. 创建 Hertz 服务
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	h := server.Default(
		server.WithHostPorts(addr),
		server.WithReadTimeout(3*time.Second),
		server.WithWriteTimeout(3*time.Second),
	)

	// 7. 只注册 RAG 路由（不注册面试、支付等业务路由）
	router.RegisterRAGRoutes(h)

	// 8. 健康检查
	h.GET("/healthz", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	h.GET("/readyz", func(ctx context.Context, c *app.RequestContext) {
		status := "ready"
		// 检查 Milvus 连接
		if milvusManager != nil {
			if err := milvusManager.HealthCheck(ctx); err != nil {
				status = "degraded"
				log.Printf("[RAG-Server] Readyz check: Milvus unhealthy: %v", err)
			}
		}
		c.JSON(http.StatusOK, map[string]string{"status": status})
	})

	// 9. 优雅关闭
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

	// 关闭 Milvus
	if milvusManager != nil {
		if err := milvusManager.Close(); err != nil {
			log.Printf("[RAG-Server] Warning: Failed to close Milvus: %v", err)
		}
	}

	// 关闭 HTTP 服务
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("[RAG-Server] Server forced to shutdown: %v", err)
	}

	log.Println("[RAG-Server] Server exited")
}

// findConfigFile 查找配置文件路径
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

// findEnvFile 查找 .env 路径
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
