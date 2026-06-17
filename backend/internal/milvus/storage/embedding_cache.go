package storage

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/embedding"
	"golang.org/x/sync/singleflight"
)

type EmbeddingCacheTrace struct {
	Enabled  bool
	Hit      bool
	LookupMs int64
	Reason   string
}

type embeddingCacheTraceContextKey struct{}

type embeddingCacheTraceCollector struct {
	mu    sync.Mutex
	trace EmbeddingCacheTrace
}

type embeddingCacheEntry struct {
	vector     []float64
	expiresAt  time.Time
	lastAccess time.Time
}

type cachedQueryEmbedder struct {
	base       embedding.Embedder
	ttl        time.Duration
	maxEntries int
	now        func() time.Time

	mu      sync.RWMutex
	entries map[string]embeddingCacheEntry
	group   singleflight.Group
}

// WithEmbeddingCacheTrace 会往 ctx 里注入一个轻量级 trace 收集器，
// 让检索链路后续可以解释这次 embedding 请求是命中缓存、未命中，
// 还是回退到了底层 embedder。
func WithEmbeddingCacheTrace(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Value(embeddingCacheTraceContextKey{}).(*embeddingCacheTraceCollector); ok {
		return ctx
	}
	return context.WithValue(ctx, embeddingCacheTraceContextKey{}, &embeddingCacheTraceCollector{})
}

func ConsumeEmbeddingCacheTrace(ctx context.Context) EmbeddingCacheTrace {
	collector, ok := ctx.Value(embeddingCacheTraceContextKey{}).(*embeddingCacheTraceCollector)
	if !ok || collector == nil {
		return EmbeddingCacheTrace{}
	}

	collector.mu.Lock()
	defer collector.mu.Unlock()
	return collector.trace
}

func recordEmbeddingCacheTrace(ctx context.Context, trace EmbeddingCacheTrace) {
	collector, ok := ctx.Value(embeddingCacheTraceContextKey{}).(*embeddingCacheTraceCollector)
	if !ok || collector == nil {
		return
	}

	collector.mu.Lock()
	collector.trace = trace
	collector.mu.Unlock()
}

// newCachedQueryEmbedder 用一个进程内小缓存包装底层 embedder。
// 这个缓存只面向“短时间内高频重复”的 query embedding，
// 文档写入或索引构建的 embedding 仍然应直接走底层 embedder。
func newCachedQueryEmbedder(base embedding.Embedder, ttl time.Duration, maxEntries int) *cachedQueryEmbedder {
	if maxEntries <= 0 {
		maxEntries = 1
	}
	return &cachedQueryEmbedder{
		base:       base,
		ttl:        ttl,
		maxEntries: maxEntries,
		now: func() time.Time {
			return time.Now().UTC()
		},
		entries: make(map[string]embeddingCacheEntry, maxEntries),
	}
}

func (c *cachedQueryEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	start := time.Now()
	trace := EmbeddingCacheTrace{Enabled: true}

	if c == nil || c.base == nil {
		trace.Reason = "dependency_unavailable"
		trace.LookupMs = time.Since(start).Milliseconds()
		recordEmbeddingCacheTrace(ctx, trace)
		return nil, fmt.Errorf("query embedder is not initialized")
	}
	if len(texts) == 0 {
		trace.Reason = "empty_batch"
		trace.LookupMs = time.Since(start).Milliseconds()
		recordEmbeddingCacheTrace(ctx, trace)
		return nil, fmt.Errorf("texts is empty")
	}
	if len(texts) != 1 {
		trace.Reason = "batch_bypass"
		vectors, err := c.base.EmbedStrings(ctx, texts, opts...)
		trace.LookupMs = time.Since(start).Milliseconds()
		if err != nil {
			trace.Reason = "embed_failed"
			recordEmbeddingCacheTrace(ctx, trace)
			return nil, err
		}
		recordEmbeddingCacheTrace(ctx, trace)
		return cloneVectors(vectors), nil
	}

	// 这个缓存只覆盖“一条 query -> 一个 embedding”这条热点链路。
	// 多文本 batch 直接绕过缓存，避免把 query 缓存语义和其他 embedding
	// 工作负载混在一起。
	normalized := strings.TrimSpace(texts[0])
	if normalized == "" {
		trace.Reason = "empty_query"
		vectors, err := c.base.EmbedStrings(ctx, texts, opts...)
		trace.LookupMs = time.Since(start).Milliseconds()
		if err != nil {
			trace.Reason = "embed_failed"
			recordEmbeddingCacheTrace(ctx, trace)
			return nil, err
		}
		recordEmbeddingCacheTrace(ctx, trace)
		return cloneVectors(vectors), nil
	}

	if vector, ok := c.get(normalized); ok {
		trace.Hit = true
		trace.Reason = "hit"
		trace.LookupMs = time.Since(start).Milliseconds()
		recordEmbeddingCacheTrace(ctx, trace)
		return [][]float64{vector}, nil
	}

	// singleflight 用来收敛并发请求：
	// 同一个 normalized query 在同一时刻只计算一次 embedding，
	// 其余并发请求直接复用首个请求的结果。
	value, err, shared := c.group.Do(normalized, func() (interface{}, error) {
		if vector, ok := c.get(normalized); ok {
			return vector, nil
		}

		vectors, embedErr := c.base.EmbedStrings(ctx, []string{normalized}, opts...)
		if embedErr != nil {
			return nil, embedErr
		}
		if len(vectors) == 0 || len(vectors[0]) == 0 {
			return nil, fmt.Errorf("embedding is empty")
		}

		vector := append([]float64(nil), vectors[0]...)
		c.put(normalized, vector)
		return append([]float64(nil), vector...), nil
	})

	trace.LookupMs = time.Since(start).Milliseconds()
	if err != nil {
		trace.Reason = "embed_failed"
		recordEmbeddingCacheTrace(ctx, trace)
		return nil, err
	}

	vector, ok := value.([]float64)
	if !ok {
		trace.Reason = "unexpected_type"
		recordEmbeddingCacheTrace(ctx, trace)
		return nil, fmt.Errorf("embedding cache returned unexpected type %T", value)
	}

	if shared {
		trace.Hit = true
		trace.Reason = "singleflight_shared"
	} else {
		trace.Reason = "miss"
	}
	recordEmbeddingCacheTrace(ctx, trace)

	return [][]float64{append([]float64(nil), vector...)}, nil
}

func (c *cachedQueryEmbedder) get(key string) ([]float64, bool) {
	now := c.now()

	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if !entry.expiresAt.After(now) {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		return nil, false
	}

	c.mu.Lock()
	entry.lastAccess = now
	c.entries[key] = entry
	c.mu.Unlock()

	return append([]float64(nil), entry.vector...), true
}

func (c *cachedQueryEmbedder) put(key string, vector []float64) {
	now := c.now()

	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = embeddingCacheEntry{
		vector:     append([]float64(nil), vector...),
		expiresAt:  now.Add(c.ttl),
		lastAccess: now,
	}
	c.pruneLocked(now)
}

// pruneLocked 会先清理过期项，再按 lastAccess 做一个轻量级的 LRU 风格淘汰，
// 把缓存大小控制在 maxEntries 以内，避免为了这个场景引入更重的数据结构。
func (c *cachedQueryEmbedder) pruneLocked(now time.Time) {
	for key, entry := range c.entries {
		if !entry.expiresAt.After(now) {
			delete(c.entries, key)
		}
	}
	if len(c.entries) <= c.maxEntries {
		return
	}

	var (
		oldestKey string
		oldestAt  time.Time
		first     = true
	)
	for key, entry := range c.entries {
		if first || entry.lastAccess.Before(oldestAt) {
			oldestKey = key
			oldestAt = entry.lastAccess
			first = false
		}
	}
	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// cloneVectors 会复制一份向量结果返回给调用方，
// 避免外部修改底层 slice，反向污染缓存中的数据。
func cloneVectors(vectors [][]float64) [][]float64 {
	if len(vectors) == 0 {
		return nil
	}
	cloned := make([][]float64, 0, len(vectors))
	for _, vector := range vectors {
		cloned = append(cloned, append([]float64(nil), vector...))
	}
	return cloned
}
