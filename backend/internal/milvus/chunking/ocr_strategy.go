package chunking

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/schema"

	"interview-agents/internal/documentparser"
)

const defaultOCRConfidenceThreshold = 0.8

type OCRAwareStrategy struct {
	delegate            Strategy
	confidenceThreshold float64
}

func NewOCRAwareStrategy(delegate Strategy, confidenceThreshold float64) *OCRAwareStrategy {
	if confidenceThreshold <= 0 {
		confidenceThreshold = defaultOCRConfidenceThreshold
	}
	return &OCRAwareStrategy{
		delegate:            delegate,
		confidenceThreshold: confidenceThreshold,
	}
}

func (s *OCRAwareStrategy) Split(ctx context.Context, req Request) ([]*schema.Document, error) {
	if s == nil || s.delegate == nil {
		return nil, fmt.Errorf("ocr-aware strategy delegate is not initialized")
	}
	chunks, err := s.delegate.Split(ctx, req)
	if err != nil {
		return nil, err
	}
	annotateOCRQuality(chunks, req.Document, s.confidenceThreshold, true)
	finalizeChunks(req, chunks)
	return chunks, nil
}

func annotateOCRQuality(chunks []*schema.Document, doc *documentparser.NormalizedDocument, confidenceThreshold float64, markStrategy bool) {
	if doc == nil {
		return
	}
	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}
		if chunk.MetaData == nil {
			chunk.MetaData = map[string]interface{}{}
		}
		if markStrategy {
			if previous := fmt.Sprint(chunk.MetaData["chunking_strategy"]); previous != "" && previous != "<nil>" {
				chunk.MetaData["chunking_base_strategy"] = previous
			}
			chunk.MetaData["chunking_strategy"] = StrategyOCRAware
		}

		span := chunkSpan(chunk)
		confidence := averageBlockConfidence(doc.Blocks, span.start, span.end)
		if confidence <= 0 {
			continue
		}
		chunk.MetaData["ocr_confidence"] = confidence
		if confidence < confidenceThreshold {
			chunk.MetaData["weak_evidence"] = true
			chunk.MetaData["weak_evidence_reason"] = "low_ocr_confidence"
		}
	}
}
