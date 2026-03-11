package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/redis/go-redis/v9"
)

// containsAny 检查字符串是否包含任意一个子串
func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if len(substr) > 0 && len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

// RedisStreamQueue Redis Stream 消息队列实现
// 相比 Pub/Sub，Stream 提供：
// 1. 消息持久化
// 2. 消费组 (Consumer Group) 支持
// 3. ACK 机制，确保至少一次消费 (At-least-once)
// 4. 断点续传能力
type RedisStreamQueue struct {
	client        *redis.Client
	mu            sync.RWMutex
	handlers      []MessageHandler
	done          chan struct{}
	pool          *ants.Pool
	consumerGroup string // 消费组名称
	consumerName  string // 消费者名称
}

// NewRedisStreamQueue 创建 Redis Stream 消息队列
func NewRedisStreamQueue(client *redis.Client, consumerGroup, consumerName string) *RedisStreamQueue {
	if client == nil {
		log.Fatal("Redis client cannot be nil")
	}

	if consumerGroup == "" {
		consumerGroup = "interview-consumer-group"
	}

	if consumerName == "" {
		consumerName = fmt.Sprintf("consumer-%d", time.Now().UnixNano())
	}

	// 初始化协程池，限制最大并发数为 1000
	pool, err := ants.NewPool(1000)
	if err != nil {
		log.Fatalf("Failed to create goroutine pool: %v", err)
	}

	return &RedisStreamQueue{
		client:        client,
		done:          make(chan struct{}),
		pool:          pool,
		consumerGroup: consumerGroup,
		consumerName:  consumerName,
	}
}

// Publish 发布消息到 Redis Stream
func (q *RedisStreamQueue) Publish(ctx context.Context, message *Message) error {
	// 序列化消息 Payload
	payloadJSON, err := json.Marshal(message.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal message payload: %w", err)
	}

	// 根据消息类型使用不同的 Stream
	streamKey := fmt.Sprintf("interview:stream:%s", message.Type)

	// 构建 Stream 消息体
	values := map[string]interface{}{
		"type":      string(message.Type),
		"payload":   string(payloadJSON),
		"timestamp": time.Now().Unix(),
	}

	log.Printf("[RedisStream] Publishing message to stream %s", streamKey)

	// 使用 XADD 添加消息到 Stream
	// * 表示自动生成消息 ID
	result := q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		MaxLen: 10000, // 限制 Stream 最大长度，防止无限增长
		Approx: true,  // 使用近似裁剪，性能更好
		Values: values,
	})

	if err := result.Err(); err != nil {
		return fmt.Errorf("failed to add message to stream: %w", err)
	}

	messageID := result.Val()
	log.Printf("[RedisStream] Message published successfully with ID: %s", messageID)

	return nil
}

// Subscribe 订阅消息（使用消费组）
func (q *RedisStreamQueue) Subscribe(ctx context.Context, handler MessageHandler) error {
	q.mu.Lock()
	q.handlers = append(q.handlers, handler)
	q.mu.Unlock()

	// 所有需要订阅的 Stream
	streams := []string{
		fmt.Sprintf("interview:stream:%s", MessageTypeEvaluationReport),
		fmt.Sprintf("interview:stream:%s", MessageTypeTopicEvaluation),
		fmt.Sprintf("interview:stream:%s", MessageTypeResumeParse),
	}

	log.Printf("[RedisStream] Consumer %s subscribing to streams: %v", q.consumerName, streams)

	// 为每个 Stream 创建消费组（如果不存在）
	for _, stream := range streams {
		err := q.createConsumerGroup(ctx, stream)
		if err != nil {
			log.Printf("[RedisStream] Warning: failed to create consumer group for %s: %v", stream, err)
			// 继续执行，可能消费组已存在
		}
	}

	log.Printf("[RedisStream] Successfully initialized consumer groups, starting to read messages...")

	// 使用 XREADGROUP 读取消息
	return q.readMessages(ctx, streams)
}

// createConsumerGroup 创建消费组
func (q *RedisStreamQueue) createConsumerGroup(ctx context.Context, stream string) error {
	// 使用 $ 表示从当前最新位置开始消费
	// 对于已有消息的 Stream，使用 0 表示从头开始消费
	err := q.client.XGroupCreateMkStream(ctx, stream, q.consumerGroup, "0").Err()
	if err != nil {
		// 如果消费组已存在，忽略错误
		if err.Error() == "BUSYGROUP Consumer Group name already exists" {
			log.Printf("[RedisStream] Consumer group %s already exists for stream %s", q.consumerGroup, stream)
			return nil
		}
		return err
	}
	log.Printf("[RedisStream] Created consumer group %s for stream %s", q.consumerGroup, stream)
	return nil
}

// readMessages 读取消息
func (q *RedisStreamQueue) readMessages(ctx context.Context, streams []string) error {
	messageCount := 0

	for {
		select {
		case <-ctx.Done():
			log.Printf("[RedisStream] Context cancelled, stopping subscription (received %d messages)", messageCount)
			return ctx.Err()
		case <-q.done:
			log.Printf("[RedisStream] Queue closed, stopping subscription (received %d messages)", messageCount)
			return nil
		default:
		}

		// 使用 XREADGROUP 读取消息
		// > 表示读取尚未被该消费组消费的新消息
		streamArgs := make([]string, 0, len(streams)*2)
		for _, stream := range streams {
			streamArgs = append(streamArgs, stream)
		}

		for range streams {
			streamArgs = append(streamArgs, ">")
		}

		result, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    q.consumerGroup,
			Consumer: q.consumerName,
			Streams:  streamArgs,
			Count:    10,              // 每次最多读取 10 条消息
			Block:    1 * time.Second, // 阻塞等待 1 秒
			NoAck:    false,
		}).Result()

		if err != nil {
			// redis.Nil 表示没有新消息，这是正常情况
			if err == redis.Nil {
				continue
			}

			// 忽略 Stream 不存在或 ID 无效的错误（Stream 可能还未创建）
			errMsg := err.Error()
			if containsAny(errMsg, "NOGROUP", "Invalid stream ID", "stream not found") {
				log.Printf("[RedisStream] Stream not ready yet (will retry): %v", err)
				time.Sleep(2 * time.Second) // 等待 Stream 被创建
				continue
			}

			log.Printf("[RedisStream] Error reading messages: %v", err)
			time.Sleep(1 * time.Second) // 出错后等待一段时间再重试
			continue
		}

		// 处理读取到的消息
		for _, stream := range result {
			for _, msg := range stream.Messages {
				messageCount++
				log.Printf("[RedisStream] Received message #%d from stream %s (ID: %s)",
					messageCount, stream.Stream, msg.ID)

				// 提取消息数据
				messageType, ok := msg.Values["type"].(string)
				if !ok {
					log.Printf("[RedisStream] Invalid message type")
					// 即使解析失败也要 ACK，避免阻塞
					q.ackMessage(ctx, stream.Stream, msg.ID)
					continue
				}

				payloadStr, ok := msg.Values["payload"].(string)
				if !ok {
					log.Printf("[RedisStream] Invalid payload")
					q.ackMessage(ctx, stream.Stream, msg.ID)
					continue
				}

				// 反序列化 Payload
				var payload map[string]interface{}
				if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
					log.Printf("[RedisStream] Failed to unmarshal payload: %v", err)
					q.ackMessage(ctx, stream.Stream, msg.ID)
					continue
				}

				// 构建 Message 对象
				message := &Message{
					Type:    MessageType(messageType),
					Payload: payload,
				}

				// 异步调用所有处理器
				q.mu.RLock()
				handlers := q.handlers
				q.mu.RUnlock()

				// 使用协程池处理消息
				for _, h := range handlers {
					handler := h
					msgCopy := message
					streamKey := stream.Stream
					msgID := msg.ID

					err := q.pool.Submit(func() {
						// 处理消息
						if err := handler(ctx, msgCopy); err != nil {
							log.Printf("[RedisStream] Error processing message: %v, type: %s, ID: %s",
								err, msgCopy.Type, msgID)
							// 处理失败不 ACK，消息会保留在 Pending 列表中
							// 可以通过 XPENDING 和 XCLAIM 进行后续处理
							return
						}

						// 处理成功，ACK 消息
						q.ackMessage(ctx, streamKey, msgID)
					})

					if err != nil {
						log.Printf("[RedisStream] Failed to submit task to pool: %v", err)
						// Pool 满了，直接 ACK 避免阻塞
						q.ackMessage(ctx, stream.Stream, msg.ID)
					}
				}
			}
		}
	}
}

// ackMessage 确认消息已处理
func (q *RedisStreamQueue) ackMessage(ctx context.Context, stream, messageID string) {
	err := q.client.XAck(ctx, stream, q.consumerGroup, messageID).Err()
	if err != nil {
		log.Printf("[RedisStream] Failed to ACK message %s: %v", messageID, err)
	} else {
		log.Printf("[RedisStream] Message ACKed: %s", messageID)
	}
}

// Close 关闭消息队列
func (q *RedisStreamQueue) Close() error {
	close(q.done)
	q.pool.Release()
	return nil
}

// GetRedisClient 获取 Redis 客户端（用于测试和其他用途）
func (q *RedisStreamQueue) GetRedisClient() *redis.Client {
	return q.client
}

// CleanupOldMessages 清理旧消息（可选的维护操作）
// 通过 XTRIM 定期清理已确认的旧消息
func (q *RedisStreamQueue) CleanupOldMessages(ctx context.Context, stream string, maxLen int64) error {
	err := q.client.XTrimMaxLen(ctx, stream, maxLen).Err()
	if err != nil {
		return fmt.Errorf("failed to trim stream %s: %w", stream, err)
	}
	log.Printf("[RedisStream] Trimmed stream %s to max length %d", stream, maxLen)
	return nil
}
