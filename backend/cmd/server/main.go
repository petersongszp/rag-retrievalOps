package main

import (
	"context"
	"errors"
	"fmt"
	"interview-agents/api/router"
	interviewRouter "interview-agents/api/router/interview"
	routerMiddleware "interview-agents/api/router/middleware"
	"interview-agents/internal/agents/llm"
	"interview-agents/internal/config"
	appMiddleware "interview-agents/internal/middleware"
	"interview-agents/internal/mq"
	"interview-agents/internal/repository"
	"interview-agents/pkg/eino/callbacks"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/joho/godotenv"
	"golang.org/x/time/rate"
)

func main() {
	// 1. 加载 .env 文件（如果存在）
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
		log.Println("Application will use system environment variables or config.yaml defaults")
	} else {
		log.Println("Successfully loaded .env file")
	}

	// 2. 加载配置文件
	// 获取配置文件路径（相对于 main.go 所在目录）
	configPath := findConfigFile()
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// 3. 展开配置中的环境变量引用（${VAR_NAME}）
	// 这使得 config.yaml 可以使用环境变量作为占位符，支持多环境配置（如 .env.local, .env.prod）
	cfg.ExpandEnv()
	log.Println("Environment variables expanded in configuration")

	// 4. 初始化数据库
	log.Println("Initializing database connection...")
	err = repository.InitDatabase(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	log.Println("Database initialized successfully")

	//5. 初始化Redis
	log.Println("Initializing Redis connection...")
	err = repository.InitRedis(cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to initialize Redis: %v", err)
	}
	log.Println("Redis initialized successfully")

	// 11.2.3 推理成本控制：Token 消耗监控与配额（Reasoning Model R1/o3 等），0 表示不限制
	if cfg.Eino.TokenQuotaPerUserPerDay > 0 {
		callbacks.DefaultTokenRecorder = llm.NewTokenRecorder(repository.GetRedis(), cfg.Eino.TokenQuotaPerUserPerDay)
		log.Printf("[11.2.3] Token quota enabled: %d per user per day", cfg.Eino.TokenQuotaPerUserPerDay)
	}

	//7. 初始化 Milvus Manager（向量数据库、Embedding、检索等服务）
	//log.Println("Initializing Milvus Manager...")
	//ctx := context.Background()
	//milvusManager, err := milvus.InitMilvusManager(ctx, cfg)
	//if err != nil {
	//	log.Fatalf("Failed to initialize Milvus Manager: %v", err)
	//}
	//// 进行健康检查
	//if err := milvusManager.HealthCheck(ctx); err != nil {
	//	log.Printf("Warning: Milvus health check failed: %v", err)
	//}
	//log.Println("Milvus Manager initialized successfully")

	// 8. 初始化消息队列（使用 Redis Stream）
	log.Println("Initializing Redis Stream message queue...")
	redisClient := repository.GetRedis()
	if redisClient == nil {
		log.Fatalf("Redis client not initialized")
	}

	// 创建 Redis Stream 消息队列
	messageQueue := mq.NewRedisStreamQueue(redisClient, "interview-consumer-group", fmt.Sprintf("consumer-%s", cfg.Host))
	mq.InitMessageQueue(messageQueue)
	log.Println("Redis Stream initialized successfully")

	// 9. 启动消费者
	log.Println("Starting message consumer...")
	consumerCtx, cancelConsumer := context.WithCancel(context.Background())
	go func() {
		if err := mq.StartConsumer(consumerCtx); err != nil {
			log.Printf("Error starting consumer: %v", err)
		}
	}()
	// 给消费者一点时间启动
	time.Sleep(500 * time.Millisecond)
	defer cancelConsumer()

	// 11.1.2 协程模型优化：启动时设置 GOMAXPROCS（0 表示不修改，使用默认）
	if cfg.Hertz.GOMAXPROCS > 0 {
		prev := runtime.GOMAXPROCS(cfg.Hertz.GOMAXPROCS)
		log.Printf("[11.1.2] GOMAXPROCS set to %d (was %d)", cfg.Hertz.GOMAXPROCS, prev)
	}

	// 11.1.1 Hertz 连接池/网关：从配置解析超时与 KeepAlive
	readTimeout := 3 * time.Second
	if d, err := time.ParseDuration(cfg.Hertz.ReadTimeout); err == nil && d > 0 {
		readTimeout = d
	}
	writeTimeout := 3 * time.Second
	if d, err := time.ParseDuration(cfg.Hertz.WriteTimeout); err == nil && d > 0 {
		writeTimeout = d
	}
	idleTimeout := 1 * time.Minute
	if d, err := time.ParseDuration(cfg.Hertz.IdleTimeout); err == nil && d > 0 {
		idleTimeout = d
	}
	keepAlive := cfg.Hertz.KeepAlive
	// 兼容旧配置：未配置 rate_limit 时视为旧配置，默认开启 KeepAlive。若需显式关闭，须同时配置 rate_limit（如 rps: 20）
	if !keepAlive && cfg.Hertz.RateLimit.RPS == 0 && cfg.Hertz.RateLimit.Burst == 0 {
		keepAlive = true
	}

	// 初始化 Hertz 服务器（11.1.1 高并发配置：KeepAlive、超时从 config 读取）
	s := server.Default(
		server.WithHostPorts(fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)),
		server.WithReadTimeout(readTimeout),
		server.WithWriteTimeout(writeTimeout),
		server.WithIdleTimeout(idleTimeout),
		server.WithKeepAlive(keepAlive),
		server.WithMaxRequestBodySize(4*1024*1024), // 4MB Limit
	)

	// 添加错误处理中间件（必须在最前面）
	// Recovery: 捕获请求处理中的 panic，防止服务崩溃
	// ErrorHandler: 统一处理业务错误，返回标准格式的错误响应
	s.Use(routerMiddleware.Recovery()) // 捕获 Panic

	// 11.1.3 限流中间件：支持单机（内存）或分布式（Redis）
	rps, burst := cfg.Hertz.RateLimit.RPS, cfg.Hertz.RateLimit.Burst
	if rps <= 0 {
		rps = 20
	}
	if burst <= 0 {
		burst = 50
	}
	if cfg.Hertz.RateLimit.UseRedis {
		log.Println("[11.1.3] Using Redis distributed rate limiter")
		prefix := cfg.Hertz.RateLimit.RedisKeyPrefix
		if prefix == "" {
			prefix = "rl:"
		}
		s.Use(appMiddleware.NewRedisRateLimiter(redisClient, prefix, burst).Handler())
	} else {
		s.Use(appMiddleware.NewIPRateLimiter(rate.Limit(rps), burst).Handler())
	}

	// 添加全局CORS中间件，处理OPTIONS预检请求
	s.Use(func(ctx context.Context, c *app.RequestContext) {
		// 设置CORS头
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, Cache-Control, X-Auth-Token")
		c.Header("Access-Control-Max-Age", "86400")

		// 如果是OPTIONS请求，直接返回204
		if string(c.Method()) == "OPTIONS" {
			log.Printf("[CORS] OPTIONS request: %s", c.Path())
			c.AbortWithStatus(204)
			return
		}

		c.Next(ctx)
	})

	s.Use(appMiddleware.JWTMiddlewareWithSkipper(interviewRouter.AuthSkipper()))

	router.GeneratedRegister(s)
	router.RegisterCustomRoutes(s)

	s.GET("/health", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
	})

	// 创建一个通道来监听中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 在单独的goroutine中启动服务器
	go func() {
		log.Printf("Server is running on %s:%d", cfg.Host, cfg.Port)
		if err := s.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 等待中断信号
	<-quit
	log.Println("Shutting down server...")

	// 关闭消费者
	cancelConsumer()
	log.Println("Message consumer stopped")

	// 关闭消息队列
	if err := messageQueue.Close(); err != nil {
		log.Printf("Warning: Failed to close message queue: %v", err)
	}
	log.Println("Message queue closed")

	// 关闭 Milvus Manager
	//if milvusManager != nil {
	//	if err := milvusManager.Close(); err != nil {
	//		log.Printf("Warning: Failed to close Milvus Manager: %v", err)
	//	}
	//}

	// 创建一个带有超时的上下文，用于关闭
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 关闭服务器
	if err := s.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}

// findConfigFile 查找配置文件路径
// 优先使用相对于 main.go 所在目录的 config.yaml
func findConfigFile() string {
	// 尝试当前工作目录（适用于 Docker 容器和项目根目录运行）
	if wd, err := os.Getwd(); err == nil {
		// 场景 1: 在 Docker 容器中运行 (WORKDIR /app, config.yaml 在 /app)
		// 或者在 backend 目录下直接运行 go run cmd/server/main.go
		path1 := filepath.Join(wd, "config.yaml")
		if _, err := os.Stat(path1); err == nil {
			log.Printf("Found config file at: %s (via workdir)", path1)
			return path1
		}

		// 场景 2: 在 backend/cmd/server 目录下运行
		path2 := filepath.Join(wd, "..", "..", "config.yaml")
		if _, err := os.Stat(path2); err == nil {
			log.Printf("Found config file at: %s (via workdir relative)", path2)
			return path2
		}
	}

	// 回退机制：使用 runtime.Caller 获取源码位置（适用于 IDE 调试）
	_, currentFile, _, ok := runtime.Caller(0)
	if ok {
		// currentFile 是 backend/cmd/server/main.go
		// config.yaml 在 backend/config.yaml
		// 需要向上找两级
		serverDir := filepath.Dir(currentFile)
		cmdDir := filepath.Dir(serverDir)
		backendDir := filepath.Dir(cmdDir)
		configPath := filepath.Join(backendDir, "config.yaml")

		if _, err := os.Stat(configPath); err == nil {
			log.Printf("Found config file at: %s (via runtime caller)", configPath)
			return configPath
		}
	}

	// 最后的兜底
	log.Println("Warning: Could not find config.yaml, defaulting to 'config.yaml'")
	return "config.yaml"
}
