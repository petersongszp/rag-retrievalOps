package chunking

import (
	"context"

	"github.com/cloudwego/eino/schema"

	"interview-agents/internal/documentparser"
)

// Request carries the normalized document and ingest metadata needed to build chunks.
type Request struct {
	Document       *documentparser.NormalizedDocument
	BaseMeta       map[string]interface{}
	NormalizedPath string
}

// Strategy converts a normalized document into indexable schema documents.
type Strategy interface {
	Split(ctx context.Context, req Request) ([]*schema.Document, error)
}
