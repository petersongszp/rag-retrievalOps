package kb

import (
	"context"
	"sync"
	"testing"
	"time"
)

type countingSemanticCacheEmbedder struct {
	mu      sync.Mutex
	calls   int
	vectors [][]float64
	err     error
}

func (c *countingSemanticCacheEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	if c.err != nil {
		return nil, c.err
	}
	out := make([][]float64, 0, len(texts))
	for i := range texts {
		idx := 0
		if i < len(c.vectors) {
			idx = i
		}
		out = append(out, append([]float64(nil), c.vectors[idx]...))
	}
	return out, nil
}

func TestCachedSemanticCacheEmbedderReusesSameQuery(t *testing.T) {
	base := &countingSemanticCacheEmbedder{
		vectors: [][]float64{{0.2, 0.8}},
	}
	embedder := newCachedSemanticCacheEmbedder(base, 5*time.Minute, 8)

	first, err := embedder.EmbedBatch(context.Background(), []string{"what is semantic cache"})
	if err != nil {
		t.Fatalf("first EmbedBatch err: %v", err)
	}
	second, err := embedder.EmbedBatch(context.Background(), []string{"what is semantic cache"})
	if err != nil {
		t.Fatalf("second EmbedBatch err: %v", err)
	}

	if base.calls != 1 {
		t.Fatalf("base calls = %d, want 1", base.calls)
	}
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("unexpected vector count: first=%d second=%d", len(first), len(second))
	}
	if first[0][0] != second[0][0] || first[0][1] != second[0][1] {
		t.Fatalf("cached vector mismatch: first=%v second=%v", first[0], second[0])
	}
}

func TestCachedSemanticCacheEmbedderExpiresEntry(t *testing.T) {
	base := &countingSemanticCacheEmbedder{
		vectors: [][]float64{{0.4, 0.6}},
	}
	now := time.Now().UTC()
	embedder := newCachedSemanticCacheEmbedder(base, time.Minute, 8)
	embedder.now = func() time.Time { return now }

	if _, err := embedder.EmbedBatch(context.Background(), []string{"cache ttl"}); err != nil {
		t.Fatalf("first EmbedBatch err: %v", err)
	}
	now = now.Add(2 * time.Minute)
	if _, err := embedder.EmbedBatch(context.Background(), []string{"cache ttl"}); err != nil {
		t.Fatalf("second EmbedBatch err: %v", err)
	}

	if base.calls != 2 {
		t.Fatalf("base calls = %d, want 2 after ttl expiry", base.calls)
	}
}
