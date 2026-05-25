package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"

	milvusRetriever "github.com/cloudwego/eino-ext/components/retriever/milvus"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"

	"interview-agents/internal/config"
	"interview-agents/internal/milvus"
	"interview-agents/internal/milvus/benchmark"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to backend config file")
	datasetPath := flag.String("dataset", "scripts/evaluation/index_benchmark_dataset.example.json", "Benchmark dataset JSON path")
	profilesPath := flag.String("profiles", "", "Optional profile JSON path")
	outputPrefix := flag.String("output", "docs/index-benchmark-report", "Output file prefix without extension")
	collection := flag.String("collection", "", "Override collection name")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	exitIfErr(err)

	ctx := context.Background()
	manager, err := milvus.InitMilvusManager(ctx, cfg)
	exitIfErr(err)
	defer manager.Close()

	profiles := benchmark.DefaultProfiles()
	if *profilesPath != "" {
		profiles, err = benchmark.LoadProfiles(*profilesPath)
		exitIfErr(err)
	}

	targetCollection := cfg.Milvus.CollectionName
	if *collection != "" {
		targetCollection = *collection
	}

	dataset, err := benchmark.LoadDataset(*datasetPath)
	exitIfErr(err)

	baseRetriever := &milvusRetriever.RetrieverConfig{
		Client:       manager.Client,
		Collection:   targetCollection,
		VectorField:  "vector",
		OutputFields: []string{"id", "content", "metadata"},
		MetricType:   entity.MetricType(cfg.Milvus.MetricType),
		TopK:         cfg.Milvus.TopK,
		Embedding:    manager.EmbeddingService.GetEmbedder(),
	}
	harness := &benchmark.MilvusProfileHarness{
		Client:        manager.Client,
		Collection:    targetCollection,
		VectorField:   "vector",
		IndexName:     benchmark.DefaultIndexName,
		BaseRetriever: baseRetriever,
	}

	runner := &benchmark.Runner{
		Factory: harness.SearcherFactory(),
		Applier: harness,
		Warmup:  1,
	}
	report, err := runner.Run(ctx, dataset, profiles)
	exitIfErr(err)

	jsonPath := *outputPrefix + ".json"
	mdPath := *outputPrefix + ".md"
	exitIfErr(benchmark.SaveReportJSON(jsonPath, report))
	exitIfErr(benchmark.SaveReportMarkdown(mdPath, report))

	fmt.Printf("benchmark report written to %s and %s\n", filepath.Clean(jsonPath), filepath.Clean(mdPath))
}

func exitIfErr(err error) {
	if err != nil {
		panic(err)
	}
}
