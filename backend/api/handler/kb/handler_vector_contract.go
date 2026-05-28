package kb

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)

func ListVectorCollections(ctx context.Context, c *app.RequestContext) {
	ListIndexRegistry(ctx, c)
}

func GetVectorCollectionHealth(ctx context.Context, c *app.RequestContext) {
	GetIndexHealth(ctx, c)
}

func RebuildVectorCollection(ctx context.Context, c *app.RequestContext) {
	BuildCandidateIndex(ctx, c)
}

func SwitchVectorCollection(ctx context.Context, c *app.RequestContext) {
	SwitchActiveIndex(ctx, c)
}

func RollbackVectorCollection(ctx context.Context, c *app.RequestContext) {
	RollbackActiveIndex(ctx, c)
}

func ListVectorOperations(ctx context.Context, c *app.RequestContext) {
	ListIndexOperations(ctx, c)
}
