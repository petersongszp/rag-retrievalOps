package kb

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	// 这层缓存是“语义缓存查找前的 query embedding 缓存”，
	// 目标是减少重复 query 反复调用向量模型的成本。
	semanticCacheEmbeddingMemoryTTL        = 15 * time.Minute
	semanticCacheEmbeddingMemoryMaxEntries = 2048
)

type semanticCacheEmbeddingCacheEntry struct {
	vector     []float64
	expiresAt  time.Time
	lastAccess time.Time
}

type cachedSemanticCacheEmbedder struct {
	base       semanticCacheEmbedding
	ttl        time.Duration
	maxEntries int
	now        func() time.Time

	mu      sync.RWMutex
	entries map[string]semanticCacheEmbeddingCacheEntry
	group   singleflight.Group
}

var (
	semanticCacheEmbedderWrapperMu   sync.Mutex
	semanticCacheEmbedderWrapperRaw  semanticCacheEmbedding
	semanticCacheEmbedderWrapperInst semanticCacheEmbedding
)

func newCachedSemanticCacheEmbedder(base semanticCacheEmbedding, ttl time.Duration, maxEntries int) *cachedSemanticCacheEmbedder {
	return &cachedSemanticCacheEmbedder{
		base:       base,
		ttl:        ttl,
		maxEntries: maxEntries,
		now: func() time.Time {
			return time.Now().UTC()
		},
		entries: make(map[string]semanticCacheEmbeddingCacheEntry, maxEntries),
	}
}

func (c *cachedSemanticCacheEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	if c == nil || c.base == nil {
		return nil, fmt.Errorf("semantic cache embedder is not initialized")
	}
	if len(texts) == 0 {
		return nil, fmt.Errorf("texts is empty")
	}

	result := make([][]float64, 0, len(texts))
	for _, text := range texts {
		vector, err := c.embedOne(ctx, text)
		if err != nil {
			return nil, err
		}
		result = append(result, vector)
	}
	return result, nil
}

func (c *cachedSemanticCacheEmbedder) embedOne(ctx context.Context, text string) ([]float64, error) {
	normalized := strings.TrimSpace(text)
	if normalized == "" {
		return nil, fmt.Errorf("text is empty")
	}

	if vector, ok := c.get(normalized); ok {
		return vector, nil
	}

	value, err, _ := c.group.Do(normalized, func() (interface{}, error) {
		if vector, ok := c.get(normalized); ok {
			return vector, nil
		}

		vectors, embedErr := c.base.EmbedBatch(ctx, []string{normalized})
		if embedErr != nil {
			return nil, embedErr
		}
		if len(vectors) == 0 || len(vectors[0]) == 0 {
			return nil, fmt.Errorf("semantic cache embedding is empty")
		}

		vector := append([]float64(nil), vectors[0]...)
		c.put(normalized, vector)
		return append([]float64(nil), vector...), nil
	})
	if err != nil {
		return nil, err
	}

	vector, ok := value.([]float64)
	if !ok {
		return nil, fmt.Errorf("semantic cache embedding cache returned unexpected type %T", value)
	}
	return append([]float64(nil), vector...), nil
}

func (c *cachedSemanticCacheEmbedder) get(key string) ([]float64, bool) {
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

func (c *cachedSemanticCacheEmbedder) put(key string, vector []float64) {
	if c.maxEntries <= 0 {
		return
	}

	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = semanticCacheEmbeddingCacheEntry{
		vector:     append([]float64(nil), vector...),
		expiresAt:  now.Add(c.ttl),
		lastAccess: now,
	}
	c.pruneLocked(now)
}

func (c *cachedSemanticCacheEmbedder) pruneLocked(now time.Time) {
	for key, entry := range c.entries {
		if !entry.expiresAt.After(now) {
			delete(c.entries, key)
		}
	}
	if len(c.entries) <= c.maxEntries {
		return
	}

	var oldestKey string
	var oldestTime time.Time
	first := true
	for key, entry := range c.entries {
		if first || entry.lastAccess.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.lastAccess
			first = false
		}
	}
	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

func wrapSemanticCacheEmbedderWithMemoryCache(base semanticCacheEmbedding) semanticCacheEmbedding {
	if base == nil {
		return nil
	}

	semanticCacheEmbedderWrapperMu.Lock()
	defer semanticCacheEmbedderWrapperMu.Unlock()

	// 底层 embedding service 在进程生命周期里通常是单例。
	// 这里按“同一个底层服务 -> 同一个内存缓存包装器”复用，
	// 避免每次请求 resolve 时都新建一个空缓存。
	if semanticCacheEmbedderWrapperRaw == base && semanticCacheEmbedderWrapperInst != nil {
		return semanticCacheEmbedderWrapperInst
	}

	semanticCacheEmbedderWrapperRaw = base
	semanticCacheEmbedderWrapperInst = newCachedSemanticCacheEmbedder(
		base,
		semanticCacheEmbeddingMemoryTTL,
		semanticCacheEmbeddingMemoryMaxEntries,
	)
	return semanticCacheEmbedderWrapperInst
}
