package splitter

import (
	"context"
	"math"
	"sort"
	"strings"

	"interview-agents/internal/milvus/chunkmeta"

	"github.com/cloudwego/eino/schema"
)

type semanticSentence struct {
	Text string
}

func (s *DocumentSplitterService) applySemanticSecondarySplit(ctx context.Context, chunks []*schema.Document) []*schema.Document {
	if s == nil || !s.semanticConfig.Enabled || s.semanticConfig.Embedder == nil || len(chunks) == 0 {
		return chunks
	}

	results := make([]*schema.Document, 0, len(chunks))
	for _, chunk := range chunks {
		results = append(results, s.semanticSplitChunk(ctx, chunk)...)
	}
	return results
}

func (s *DocumentSplitterService) semanticSplitChunk(ctx context.Context, chunk *schema.Document) []*schema.Document {
	if chunk == nil {
		return nil
	}
	content := strings.TrimSpace(chunk.Content)
	if len([]rune(content)) < s.semanticConfig.MinBlockSize {
		return []*schema.Document{chunk}
	}

	sentences := splitSemanticSentences(content)
	if len(sentences) < s.semanticConfig.MinSentencesPerChunk*2 || len(sentences) > 128 {
		return []*schema.Document{chunk}
	}

	texts := make([]string, 0, len(sentences))
	for _, sentence := range sentences {
		texts = append(texts, sentence.Text)
	}
	vectors, err := s.semanticConfig.Embedder.EmbedStrings(ctx, texts)
	if err != nil || len(vectors) != len(sentences) {
		return []*schema.Document{chunk}
	}

	similarities := adjacentSimilarities(vectors)
	threshold := percentileFloat64(similarities, s.semanticConfig.BreakpointPercentile)
	parts := rebuildSemanticChunks(sentences, similarities, threshold, s.semanticConfig)
	if len(parts) <= 1 {
		return []*schema.Document{chunk}
	}

	results := make([]*schema.Document, 0, len(parts))
	for _, part := range parts {
		meta := cloneMetadataMap(chunk.MetaData)
		meta[chunkmeta.KeySemanticSplitEnabled] = true
		meta[chunkmeta.KeySemanticSplitScore] = threshold
		meta[chunkmeta.KeySemanticBreakpointMethod] = chunkmeta.SemanticBreakpointEmbeddingV1
		results = append(results, &schema.Document{
			Content:  part,
			MetaData: meta,
		})
	}
	return results
}

func splitSemanticSentences(content string) []semanticSentence {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) == 0 {
		return nil
	}

	sentences := make([]semanticSentence, 0)
	start := 0
	for i, r := range runes {
		if !isSentenceBoundary(r) {
			continue
		}
		piece := strings.TrimSpace(string(runes[start : i+1]))
		if piece != "" {
			sentences = append(sentences, semanticSentence{Text: piece})
		}
		start = i + 1
	}
	if start < len(runes) {
		piece := strings.TrimSpace(string(runes[start:]))
		if piece != "" {
			sentences = append(sentences, semanticSentence{Text: piece})
		}
	}
	return sentences
}

func isSentenceBoundary(r rune) bool {
	switch r {
	case '.', '!', '?', '。', '！', '？', '\n':
		return true
	default:
		return false
	}
}

func adjacentSimilarities(vectors [][]float64) []float64 {
	if len(vectors) < 2 {
		return nil
	}
	scores := make([]float64, 0, len(vectors)-1)
	for i := 0; i < len(vectors)-1; i++ {
		scores = append(scores, cosineSimilarity(vectors[i], vectors[i+1]))
	}
	return scores
}

func cosineSimilarity(left, right []float64) float64 {
	if len(left) == 0 || len(right) == 0 || len(left) != len(right) {
		return 0
	}
	dot := 0.0
	leftNorm := 0.0
	rightNorm := 0.0
	for i := range left {
		dot += left[i] * right[i]
		leftNorm += left[i] * left[i]
		rightNorm += right[i] * right[i]
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0
	}
	return dot / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm))
}

func rebuildSemanticChunks(sentences []semanticSentence, similarities []float64, threshold float64, cfg SemanticSplitConfig) []string {
	parts := make([]string, 0)
	current := make([]string, 0, cfg.MinSentencesPerChunk)
	currentLen := 0

	flush := func(force bool) {
		if len(current) == 0 {
			return
		}
		if !force && len(current) < cfg.MinSentencesPerChunk {
			return
		}
		parts = append(parts, strings.TrimSpace(strings.Join(current, " ")))
		current = nil
		currentLen = 0
	}

	for idx, sentence := range sentences {
		current = append(current, sentence.Text)
		currentLen += len([]rune(sentence.Text))

		shouldSplit := false
		if idx == len(sentences)-1 {
			shouldSplit = true
		} else if len(current) >= cfg.MinSentencesPerChunk && currentLen >= cfg.TargetChunkSize {
			nextScore := similarities[idx]
			if nextScore <= threshold || currentLen >= cfg.MaxChunkSize {
				shouldSplit = true
			}
		} else if currentLen >= cfg.MaxChunkSize && len(current) >= cfg.MinSentencesPerChunk {
			shouldSplit = true
		}

		if shouldSplit {
			flush(true)
		}
	}

	if len(parts) > 1 && len([]rune(parts[len(parts)-1])) < cfg.MinBlockSize/4 {
		parts[len(parts)-2] = strings.TrimSpace(parts[len(parts)-2] + " " + parts[len(parts)-1])
		parts = parts[:len(parts)-1]
	}
	return parts
}

func percentileFloat64(values []float64, percentile int) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)
	rank := int(math.Ceil((float64(percentile)/100.0)*float64(len(sorted)))) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= len(sorted) {
		rank = len(sorted) - 1
	}
	return sorted[rank]
}
