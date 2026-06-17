package milvus

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	milvusClient "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"

	"interview-agents/internal/config"
	"interview-agents/internal/milvus/storage"
)

const (
	knowledgeCollectionKey = "knowledge"
	vectorFieldName        = "vector"
	maxCollectionNameLen   = 255
)

var nonAlphaNumericPattern = regexp.MustCompile(`[^a-z0-9]+`)

// resolveKnowledgeCollection aligns the runtime collection with the active embedding model.
// When the configured base collection already exists with a different vector dimension,
// a model/dimension-scoped collection name is selected so Milvus can create a fresh schema.
func resolveKnowledgeCollection(
	ctx context.Context,
	cfg *config.Config,
	client milvusClient.Client,
	embeddingService *storage.EmbeddingService,
) (string, int, error) {
	baseCollection := strings.TrimSpace(cfg.Milvus.GetCollection(knowledgeCollectionKey))
	if baseCollection == "" {
		baseCollection = strings.TrimSpace(cfg.Milvus.CollectionName)
	}
	if baseCollection == "" {
		return "", 0, fmt.Errorf("milvus collection name is empty")
	}

	actualDim, err := probeEmbeddingDimension(ctx, embeddingService)
	if err != nil {
		return "", 0, err
	}
	if cfg.Embedding.Dimensions != actualDim {
		log.Printf("[Milvus] embedding dimension updated from config value %d to runtime value %d for model %s",
			cfg.Embedding.Dimensions, actualDim, cfg.Embedding.Model)
		cfg.Embedding.Dimensions = actualDim
	}

	resolvedCollection, err := resolveCollectionNameForDimension(ctx, client, baseCollection, cfg.Embedding.Model, actualDim)
	if err != nil {
		return "", 0, err
	}

	if cfg.Milvus.Collections == nil {
		cfg.Milvus.Collections = make(map[string]string)
	}
	cfg.Milvus.Collections[knowledgeCollectionKey] = resolvedCollection
	cfg.Milvus.CollectionName = resolvedCollection

	return resolvedCollection, actualDim, nil
}

func probeEmbeddingDimension(ctx context.Context, embeddingService *storage.EmbeddingService) (int, error) {
	if embeddingService == nil {
		return 0, fmt.Errorf("embedding service is nil")
	}

	vectors, err := embeddingService.GetEmbedder().EmbedStrings(ctx, []string{"dimension probe"})
	if err != nil {
		return 0, fmt.Errorf("failed to probe embedding dimension: %w", err)
	}
	if len(vectors) == 0 || len(vectors[0]) == 0 {
		return 0, fmt.Errorf("embedding dimension probe returned empty vectors")
	}

	return len(vectors[0]), nil
}

func resolveCollectionNameForDimension(ctx context.Context, client milvusClient.Client, baseCollection string, model string, expectedDim int) (string, error) {
	if client == nil {
		return "", fmt.Errorf("milvus client is nil")
	}
	if expectedDim <= 0 {
		return "", fmt.Errorf("embedding dimension must be positive, got %d", expectedDim)
	}

	exists, err := client.HasCollection(ctx, baseCollection)
	if err != nil {
		return "", fmt.Errorf("failed to check milvus collection %s: %w", baseCollection, err)
	}
	if !exists {
		return baseCollection, nil
	}

	currentDim, err := getCollectionVectorDimension(ctx, client, baseCollection, vectorFieldName)
	if err != nil {
		return "", err
	}
	if currentDim == expectedDim {
		return baseCollection, nil
	}

	candidate := buildDimensionScopedCollectionName(baseCollection, model, expectedDim)
	candidateExists, err := client.HasCollection(ctx, candidate)
	if err != nil {
		return "", fmt.Errorf("failed to check milvus collection %s: %w", candidate, err)
	}
	if candidateExists {
		candidateDim, err := getCollectionVectorDimension(ctx, client, candidate, vectorFieldName)
		if err != nil {
			return "", err
		}
		if candidateDim != expectedDim {
			return "", fmt.Errorf("collection %s already exists with dim=%d, expected dim=%d", candidate, candidateDim, expectedDim)
		}
	}

	log.Printf("[Milvus] collection %s uses dim=%d but model %s outputs dim=%d, switching runtime collection to %s",
		baseCollection, currentDim, cfgSafeModelName(model), expectedDim, candidate)
	return candidate, nil
}

func getCollectionVectorDimension(ctx context.Context, client milvusClient.Client, collectionName string, fieldName string) (int, error) {
	collection, err := client.DescribeCollection(ctx, collectionName)
	if err != nil {
		return 0, fmt.Errorf("failed to describe milvus collection %s: %w", collectionName, err)
	}
	if collection == nil || collection.Schema == nil {
		return 0, fmt.Errorf("collection %s schema is empty", collectionName)
	}

	for _, field := range collection.Schema.Fields {
		if field == nil || field.Name != fieldName {
			continue
		}
		dimStr := strings.TrimSpace(field.TypeParams[entity.TypeParamDim])
		if dimStr == "" {
			return 0, fmt.Errorf("collection %s field %s has no dimension metadata", collectionName, fieldName)
		}
		dim, err := strconv.Atoi(dimStr)
		if err != nil {
			return 0, fmt.Errorf("collection %s field %s has invalid dimension %q: %w", collectionName, fieldName, dimStr, err)
		}
		return dim, nil
	}

	return 0, fmt.Errorf("collection %s does not contain vector field %s", collectionName, fieldName)
}

func buildDimensionScopedCollectionName(baseCollection string, model string, dim int) string {
	base := sanitizeCollectionName(baseCollection)
	modelPart := sanitizeCollectionName(model)
	if modelPart == "" {
		modelPart = "embedding"
	}
	modelPart = shortenIdentifier(modelPart, 96)
	name := fmt.Sprintf("%s_%s_dim%d", base, modelPart, dim)
	if len(name) <= maxCollectionNameLen {
		return name
	}

	hash := sha1.Sum([]byte(name))
	suffix := "_" + hex.EncodeToString(hash[:6])
	trimmedBase := base
	maxBaseLen := maxCollectionNameLen - len(suffix) - len(modelPart) - len("_dim") - len(strconv.Itoa(dim)) - 2
	if maxBaseLen < 8 {
		maxBaseLen = 8
	}
	trimmedBase = shortenIdentifier(trimmedBase, maxBaseLen)
	return fmt.Sprintf("%s_%s_dim%d%s", trimmedBase, modelPart, dim, suffix)
}

func sanitizeCollectionName(value string) string {
	sanitized := strings.ToLower(strings.TrimSpace(value))
	sanitized = nonAlphaNumericPattern.ReplaceAllString(sanitized, "_")
	sanitized = strings.Trim(sanitized, "_")
	if sanitized == "" {
		return "documents"
	}
	return sanitized
}

func shortenIdentifier(value string, maxLen int) string {
	if len(value) <= maxLen || maxLen <= 0 {
		return value
	}
	if maxLen <= 10 {
		return value[:maxLen]
	}

	hash := sha1.Sum([]byte(value))
	hashSuffix := "_" + hex.EncodeToString(hash[:4])
	prefixLen := maxLen - len(hashSuffix)
	if prefixLen < 1 {
		prefixLen = 1
	}
	return value[:prefixLen] + hashSuffix
}

func cfgSafeModelName(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return "unknown"
	}
	return model
}
