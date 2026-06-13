package storage

import (
	"context"
	"fmt"
	"testing"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino/components/embedding"

	"interview-agents/internal/config"
)

type fakeEmbedder struct {
	callSizes []int
}

func (f *fakeEmbedder) EmbedStrings(_ context.Context, texts []string, _ ...embedding.Option) ([][]float64, error) {
	f.callSizes = append(f.callSizes, len(texts))
	result := make([][]float64, 0, len(texts))
	for i := range texts {
		result = append(result, []float64{float64(len(texts)), float64(i)})
	}
	return result, nil
}

func TestBatchingEmbedderSplitsRequests(t *testing.T) {
	inner := &fakeEmbedder{}
	embedder := &batchingEmbedder{
		embedder:     inner,
		maxBatchSize: 10,
	}

	texts := make([]string, 25)
	for i := range texts {
		texts[i] = fmt.Sprintf("text-%d", i)
	}

	vectors, err := embedder.EmbedStrings(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedStrings returned error: %v", err)
	}
	if len(vectors) != len(texts) {
		t.Fatalf("expected %d vectors, got %d", len(texts), len(vectors))
	}

	wantSizes := []int{10, 10, 5}
	if len(inner.callSizes) != len(wantSizes) {
		t.Fatalf("expected %d calls, got %d", len(wantSizes), len(inner.callSizes))
	}
	for i, want := range wantSizes {
		if inner.callSizes[i] != want {
			t.Fatalf("call %d expected batch size %d, got %d", i, want, inner.callSizes[i])
		}
	}
}

func TestResolveEmbeddingBatchSizeDashScopeDefault(t *testing.T) {
	cfg := &config.EmbeddingConfig{
		Provider: "openai",
		BaseURL:  "https://dashscope.aliyuncs.com/compatible-mode/v1",
	}
	if got := resolveEmbeddingBatchSize(cfg); got != 10 {
		t.Fatalf("expected dashscope batch size 10, got %d", got)
	}
}

func TestResolveEmbeddingBatchSizeHonorsExplicitConfig(t *testing.T) {
	cfg := &config.EmbeddingConfig{
		Provider:  "openai",
		BaseURL:   "https://dashscope.aliyuncs.com/compatible-mode/v1",
		BatchSize: 8,
	}
	if got := resolveEmbeddingBatchSize(cfg); got != 8 {
		t.Fatalf("expected explicit batch size 8, got %d", got)
	}
}

func TestResolveArkAPITypeMapsMultiModalAliases(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "env short form", input: "multi_modal"},
		{name: "ark env form", input: "multimodal"},
		{name: "sdk form", input: "multi_modal_api"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveArkAPIType(tt.input)
			if err != nil {
				t.Fatalf("resolveArkAPIType returned error: %v", err)
			}
			if got == nil || *got != ark.APITypeMultiModal {
				t.Fatalf("expected APITypeMultiModal, got %v", got)
			}
		})
	}
}

func TestResolveArkAPITypeRejectsUnknownValue(t *testing.T) {
	_, err := resolveArkAPIType("unsupported")
	if err == nil {
		t.Fatal("expected unsupported API type to return error")
	}
}

func TestResolveArkAPITypeUsesArkSpecificField(t *testing.T) {
	cfg := &config.EmbeddingConfig{
		ArkAPIType: "multimodal",
	}

	got, err := resolveArkAPIType(cfg.ArkAPIType)
	if err != nil {
		t.Fatalf("resolveArkAPIType returned error: %v", err)
	}
	if got == nil || *got != ark.APITypeMultiModal {
		t.Fatalf("expected APITypeMultiModal, got %v", got)
	}
}
