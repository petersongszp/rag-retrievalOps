package repository

import (
	"context"
	"log"
	"time"

	"interview-agents/internal/config"

	"github.com/redis/go-redis/v9"
)

// RedisClient 全局Redis客户端实例
var RedisClient *redis.Client

// InitRedis 初始化Redis连接
func InitRedis(redisCfg config.RedisConfig) error {
	// 这里做的是“建立一个全局可复用的 Redis client”。
	// 业务层不会每次请求都重新拨号，而是都复用这个 client。
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     redisCfg.Addr,
		Password: redisCfg.Password,
		DB:       redisCfg.DB,
	})

	// 启动时先 Ping 一次，尽早发现配置错误或 Redis 未启动，
	// 避免服务起来后请求进来才报连接问题。
	ctx := context.Background()
	_, err := RedisClient.Ping(ctx).Result()
	if err != nil {
		return err
	}

	log.Println("Redis连接成功")
	return nil
}

// GetRedis 获取Redis客户端实例
func GetRedis() *redis.Client {
	// 语义缓存、限流、消息队列等模块都通过这里复用同一个 Redis 连接池。
	return RedisClient
}

// SetCache 设置缓存
func SetCache(ctx context.Context, key string, value interface{}, expiration int) error {
	return RedisClient.Set(ctx, key, value, time.Duration(expiration)*time.Second).Err()
}

// GetCache 获取缓存
func GetCache(ctx context.Context, key string) (string, error) {
	return RedisClient.Get(ctx, key).Result()
}

// DeleteCache 删除缓存
func DeleteCache(ctx context.Context, key string) error {
	return RedisClient.Del(ctx, key).Err()
}
