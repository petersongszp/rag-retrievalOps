package storage

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/embedding"
)

type countingEmbedder struct {
	mu      sync.Mutex
	calls   int
	vectors [][]float64
	delay   time.Duration
}

func (c *countingEmbedder) EmbedStrings(_ context.Context, texts []string, _ ...embedding.Option) ([][]float64, error) {
	if c.delay > 0 {
		time.Sleep(c.delay)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.calls++
	out := make([][]float64, 0, len(texts))
	for i := range texts {
		index := i
		if index >= len(c.vectors) {
			index = len(c.vectors) - 1
		}
		if index < 0 {
			index = 0
		}
		out = append(out, append([]float64(nil), c.vectors[index]...))
	}
	return out, nil
}

func TestCachedQueryEmbedderReusesSameQuery(t *testing.T) {
	base := &countingEmbedder{vectors: [][]float64{{0.1, 0.9}}}
	embedder := newCachedQueryEmbedder(base, 5*time.Minute, 8)

	ctx := WithEmbeddingCacheTrace(context.Background())
	first, err := embedder.EmbedStrings(ctx, []string{"what is embedding cache"})
	if err != nil {
		t.Fatalf("first EmbedStrings err: %v", err)
	}
	firstTrace := ConsumeEmbeddingCacheTrace(ctx)

	ctx = WithEmbeddingCacheTrace(context.Background())
	second, err := embedder.EmbedStrings(ctx, []string{"what is embedding cache"})
	if err != nil {
		t.Fatalf("second EmbedStrings err: %v", err)
	}
	secondTrace := ConsumeEmbeddingCacheTrace(ctx)

	if base.calls != 1 {
		t.Fatalf("base calls = %d, want 1", base.calls)
	}
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("unexpected vector count: first=%d second=%d", len(first), len(second))
	}
	if !secondTrace.Enabled || !secondTrace.Hit || secondTrace.Reason != "hit" {
		t.Fatalf("unexpected second trace: %+v", secondTrace)
	}
	if firstTrace.Reason != "miss" {
		t.Fatalf("unexpected first trace: %+v", firstTrace)
	}
}

func TestCachedQueryEmbedderExpiresEntry(t *testing.T) {
	base := &countingEmbedder{vectors: [][]float64{{0.2, 0.8}}}
	now := time.Now().UTC()
	embedder := newCachedQueryEmbedder(base, time.Minute, 8)
	embedder.now = func() time.Time { return now }

	if _, err := embedder.EmbedStrings(context.Background(), []string{"cache ttl"}); err != nil {
		t.Fatalf("first EmbedStrings err: %v", err)
	}
	now = now.Add(2 * time.Minute)
	if _, err := embedder.EmbedStrings(context.Background(), []string{"cache ttl"}); err != nil {
		t.Fatalf("second EmbedStrings err: %v", err)
	}

	if base.calls != 2 {
		t.Fatalf("base calls = %d, want 2 after ttl expiry", base.calls)
	}
}

func TestCachedQueryEmbedderBypassesBatchRequests(t *testing.T) {
	base := &countingEmbedder{vectors: [][]float64{{0.3, 0.7}, {0.4, 0.6}}}
	embedder := newCachedQueryEmbedder(base, 5*time.Minute, 8)

	ctx := WithEmbeddingCacheTrace(context.Background())
	vectors, err := embedder.EmbedStrings(ctx, []string{"a", "b"})
	if err != nil {
		t.Fatalf("EmbedStrings err: %v", err)
	}
	trace := ConsumeEmbeddingCacheTrace(ctx)

	if len(vectors) != 2 {
		t.Fatalf("len(vectors) = %d, want 2", len(vectors))
	}
	if base.calls != 1 {
		t.Fatalf("base calls = %d, want 1", base.calls)
	}
	if trace.Hit || trace.Reason != "batch_bypass" {
		t.Fatalf("unexpected trace: %+v", trace)
	}
}

func TestCachedQueryEmbedderEvictsOldestEntry(t *testing.T) {
	base := &countingEmbedder{vectors: [][]float64{{0.5, 0.5}}}
	now := time.Now().UTC()
	embedder := newCachedQueryEmbedder(base, 5*time.Minute, 2)
	embedder.now = func() time.Time { return now }

	if _, err := embedder.EmbedStrings(context.Background(), []string{"first"}); err != nil {
		t.Fatalf("embed first err: %v", err)
	}
	now = now.Add(time.Second)
	if _, err := embedder.EmbedStrings(context.Background(), []string{"second"}); err != nil {
		t.Fatalf("embed second err: %v", err)
	}
	now = now.Add(time.Second)
	if _, err := embedder.EmbedStrings(context.Background(), []string{"third"}); err != nil {
		t.Fatalf("embed third err: %v", err)
	}
	now = now.Add(time.Second)
	if _, err := embedder.EmbedStrings(context.Background(), []string{"first"}); err != nil {
		t.Fatalf("embed first again err: %v", err)
	}

	if base.calls != 4 {
		t.Fatalf("base calls = %d, want 4 after eviction", base.calls)
	}
}

func TestCachedQueryEmbedderCoalescesConcurrentMisses(t *testing.T) {
	base := &countingEmbedder{
		vectors: [][]float64{{0.6, 0.4}},
		delay:   50 * time.Millisecond,
	}
	embedder := newCachedQueryEmbedder(base, 5*time.Minute, 8)

	var wg sync.WaitGroup
	traces := make([]EmbeddingCacheTrace, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			ctx := WithEmbeddingCacheTrace(context.Background())
			if _, err := embedder.EmbedStrings(ctx, []string{"same-query"}); err != nil {
				t.Errorf("goroutine %d EmbedStrings err: %v", index, err)
				return
			}
			traces[index] = ConsumeEmbeddingCacheTrace(ctx)
		}(i)
	}
	wg.Wait()

	if base.calls != 1 {
		t.Fatalf("base calls = %d, want 1", base.calls)
	}
	hitLikeCount := 0
	for _, trace := range traces {
		if trace.Reason == "hit" || trace.Reason == "singleflight_shared" || trace.Reason == "miss" {
			hitLikeCount++
		}
	}
	if hitLikeCount != 2 {
		t.Fatalf("unexpected traces: %+v", traces)
	}
}
