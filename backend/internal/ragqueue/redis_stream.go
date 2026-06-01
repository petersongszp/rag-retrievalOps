package ragqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"interview-agents/internal/observability/metrics"

	"github.com/panjf2000/ants/v2"
	"github.com/redis/go-redis/v9"
)

// RedisStreamQueue implements queueing on Redis Stream with a RAG-only stream set.
type RedisStreamQueue struct {
	client        *redis.Client
	mu            sync.RWMutex
	handlers      []MessageHandler
	done          chan struct{}
	pool          *ants.Pool
	consumerGroup string
	consumerName  string
}

func NewRedisStreamQueue(client *redis.Client, consumerGroup, consumerName string) *RedisStreamQueue {
	if client == nil {
		log.Fatal("Redis client cannot be nil")
	}
	if consumerGroup == "" {
		consumerGroup = "rag-consumer-group"
	}
	if consumerName == "" {
		consumerName = fmt.Sprintf("rag-consumer-%d", time.Now().UnixNano())
	}

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

func (q *RedisStreamQueue) Publish(ctx context.Context, message *Message) error {
	payloadJSON, err := json.Marshal(message.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal message payload: %w", err)
	}

	streamKey := streamKeyForType(message.Type)
	values := map[string]interface{}{
		"type":      string(message.Type),
		"payload":   string(payloadJSON),
		"timestamp": time.Now().Unix(),
	}

	result := q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		MaxLen: 10000,
		Approx: true,
		Values: values,
	})
	if err := result.Err(); err != nil {
		return fmt.Errorf("failed to add message to stream: %w", err)
	}

	log.Printf("[RAG RedisStream] Message published to %s with ID: %s", streamKey, result.Val())
	return nil
}

func (q *RedisStreamQueue) Subscribe(ctx context.Context, handler MessageHandler) error {
	q.mu.Lock()
	q.handlers = append(q.handlers, handler)
	q.mu.Unlock()

	streams := q.streamKeys()
	log.Printf("[RAG RedisStream] Consumer %s subscribing to streams: %v", q.consumerName, streams)

	for _, stream := range streams {
		if err := q.createConsumerGroup(ctx, stream); err != nil {
			log.Printf("[RAG RedisStream] Warning: failed to create consumer group for %s: %v", stream, err)
		}
	}

	log.Printf("[RAG RedisStream] Consumer groups initialized")
	return q.readMessages(ctx, streams)
}

func (q *RedisStreamQueue) createConsumerGroup(ctx context.Context, stream string) error {
	err := q.client.XGroupCreateMkStream(ctx, stream, q.consumerGroup, "0").Err()
	if err != nil {
		if strings.Contains(err.Error(), "BUSYGROUP") {
			return nil
		}
		return err
	}
	return nil
}

func (q *RedisStreamQueue) readMessages(ctx context.Context, streams []string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-q.done:
			return nil
		default:
		}

		streamArgs := make([]string, 0, len(streams)*2)
		streamArgs = append(streamArgs, streams...)
		for range streams {
			streamArgs = append(streamArgs, ">")
		}

		result, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    q.consumerGroup,
			Consumer: q.consumerName,
			Streams:  streamArgs,
			Count:    10,
			Block:    1 * time.Second,
			NoAck:    false,
		}).Result()
		if err != nil {
			if err == redis.Nil {
				continue
			}

			errMsg := err.Error()
			if containsAny(errMsg, "NOGROUP", "Invalid stream ID", "stream not found") {
				time.Sleep(2 * time.Second)
				continue
			}

			log.Printf("[RAG RedisStream] Error reading messages: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		for _, stream := range result {
			for _, msg := range stream.Messages {
				messageType, ok := msg.Values["type"].(string)
				if !ok {
					q.ackMessage(ctx, stream.Stream, msg.ID)
					continue
				}

				payloadStr, ok := msg.Values["payload"].(string)
				if !ok {
					q.ackMessage(ctx, stream.Stream, msg.ID)
					continue
				}

				var payload map[string]interface{}
				if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
					q.ackMessage(ctx, stream.Stream, msg.ID)
					continue
				}

				message := &Message{Type: MessageType(messageType), Payload: payload}

				q.mu.RLock()
				handlers := q.handlers
				q.mu.RUnlock()

				for _, h := range handlers {
					handlerFn := h
					msgCopy := message
					streamKey := stream.Stream
					msgID := msg.ID

					err := q.pool.Submit(func() {
						if err := handlerFn(ctx, msgCopy); err != nil {
							log.Printf("[RAG RedisStream] Error processing message: %v, type: %s, ID: %s", err, msgCopy.Type, msgID)
							return
						}
						q.ackMessage(ctx, streamKey, msgID)
					})
					if err != nil {
						log.Printf("[RAG RedisStream] Failed to submit task to pool: %v", err)
						q.ackMessage(ctx, stream.Stream, msg.ID)
					}
				}
			}
		}
	}
}

func (q *RedisStreamQueue) StartMetricsReporter(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	q.reportBacklogMetrics(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-q.done:
			return
		case <-ticker.C:
			q.reportBacklogMetrics(ctx)
		}
	}
}

func (q *RedisStreamQueue) reportBacklogMetrics(ctx context.Context) {
	for _, stream := range q.streamKeys() {
		messageType := streamTypeFromKey(stream)
		groups, err := q.client.XInfoGroups(ctx, stream).Result()
		if err != nil {
			metrics.SetConsumerBacklog("redis_stream", stream, q.consumerGroup, messageType, 0, 0, -1)
			continue
		}

		var target *redis.XInfoGroup
		for i := range groups {
			if groups[i].Name == q.consumerGroup {
				target = &groups[i]
				break
			}
		}
		if target == nil {
			metrics.SetConsumerBacklog("redis_stream", stream, q.consumerGroup, messageType, 0, 0, -1)
			continue
		}

		lag := target.Lag
		backlog := target.Pending
		if lag > 0 {
			backlog += lag
		}
		metrics.SetConsumerBacklog("redis_stream", stream, q.consumerGroup, messageType, backlog, target.Pending, lag)
	}
}

func (q *RedisStreamQueue) Close() error {
	close(q.done)
	q.pool.Release()
	return nil
}

func (q *RedisStreamQueue) streamKeys() []string {
	return []string{streamKeyForType(MessageTypeKnowledgeIngest)}
}

func streamKeyForType(messageType MessageType) string {
	// Keep the legacy stream key for backward compatibility with existing data and tooling.
	return fmt.Sprintf("interview:stream:%s", messageType)
}

func streamTypeFromKey(stream string) string {
	stream = strings.TrimSpace(stream)
	if stream == "" {
		return "unknown"
	}
	idx := strings.LastIndex(stream, ":")
	if idx < 0 || idx >= len(stream)-1 {
		return stream
	}
	return stream[idx+1:]
}

func (q *RedisStreamQueue) ackMessage(ctx context.Context, stream, messageID string) {
	err := q.client.XAck(ctx, stream, q.consumerGroup, messageID).Err()
	if err != nil {
		log.Printf("[RAG RedisStream] Failed to ACK message %s: %v", messageID, err)
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
