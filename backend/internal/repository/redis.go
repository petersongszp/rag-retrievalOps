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
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     redisCfg.Addr,
		Password: redisCfg.Password,
		DB:       redisCfg.DB,
	})

	// 测试连接
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
