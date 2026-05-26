package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"

	"interview-agents/internal/config"
	"interview-agents/internal/milvus"
	"interview-agents/internal/milvus/evaluation"
	"interview-agents/internal/milvus/retrieval"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to backend config file")
	datasetPath := flag.String("dataset", "scripts/evaluation/dataset.json", "Retrieval evaluation dataset JSON path")
	profilesPath := flag.String("profiles", "", "Optional retrieval strategy profile JSON path")
	gatesPath := flag.String("gates", "", "Optional gate threshold JSON path")
	outputPrefix := flag.String("output", "docs/retrieval-regression-report", "Output file prefix without extension")
	baseline := flag.String("baseline", "", "Baseline strategy name override")
	candidate := flag.String("candidate", "", "Candidate strategy name override")
	collection := flag.String("collection", "", "Override collection name")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	exitIfErr(err)

	ctx := context.Background()
	manager, err := milvus.InitMilvusManager(ctx, cfg)
	exitIfErr(err)
	defer manager.Close()

	datasetBundle, err := evaluation.LoadDatasetBundle(*datasetPath)
	exitIfErr(err)

	profiles := evaluation.DefaultProfiles()
	if *profilesPath != "" {
		profiles, err = evaluation.LoadProfiles(*profilesPath)
		exitIfErr(err)
	}

	thresholds := evaluation.DefaultGateThresholds()
	if *gatesPath != "" {
		thresholds, err = evaluation.LoadGateThresholds(*gatesPath)
		exitIfErr(err)
	}

	targetCollection := cfg.Milvus.CollectionName
	if *collection != "" {
		targetCollection = *collection
	}

	runner := &evaluation.Runner{
		Factory: func(profile evaluation.StrategyProfile) (evaluation.Searcher, error) {
			return buildSearcher(cfg, manager, profile, targetCollection)
		},
	}
	report, err := runner.Run(ctx, datasetBundle.Cases, profiles, thresholds, *baseline, *candidate)
	exitIfErr(err)
	report.DatasetVersion = datasetBundle.DatasetVersion

	jsonPath := *outputPrefix + ".json"
	mdPath := *outputPrefix + ".md"
	exitIfErr(evaluation.SaveReportJSON(jsonPath, report))
	exitIfErr(evaluation.SaveReportMarkdown(mdPath, report))
	fmt.Printf("retrieval evaluation report written to %s and %s\n", filepath.Clean(jsonPath), filepath.Clean(mdPath))

	if gateErr := report.Gate.Error(); gateErr != nil {
		fmt.Fprintf(os.Stderr, "retrieval gate failed: %v\n", gateErr)
		os.Exit(2)
	}
}

type retrievalSearcher struct {
	profile    evaluation.StrategyProfile
	retriever  *retrieval.RetrieverService
	hybrid     *retrieval.HybridRetriever
	collection string
	timeout    time.Duration
}

func buildSearcher(cfg *config.Config, manager *milvus.MilvusManager, profile evaluation.StrategyProfile, collection string) (evaluation.Searcher, error) {
	searcher := &retrievalSearcher{
		profile:    profile,
		retriever:  manager.GetRetrieverService(),
		collection: collection,
		timeout:    time.Duration(cfg.RAG.Thresholds.RetrieveTimeoutMS) * time.Millisecond,
	}
	if searcher.timeout <= 0 {
		searcher.timeout = 3 * time.Second
	}

	if strings.EqualFold(profile.Mode, "hybrid") {
		candidateTopK := cfg.RAG.Phase2.CandidateTopK
		if candidateTopK <= 0 {
			candidateTopK = cfg.Milvus.TopK * 2
		}
		if profile.CandidateTopK > 0 {
			candidateTopK = profile.CandidateTopK
		}
		denseWeight := cfg.RAG.Phase2.HybridDenseWeight
		if profile.DenseWeight > 0 {
			denseWeight = profile.DenseWeight
		}
		sparseWeight := cfg.RAG.Phase2.HybridSparseWeight
		if profile.SparseWeight > 0 {
			sparseWeight = profile.SparseWeight
		}
		hybridConfig := &retrieval.HybridRetrieverConfig{
			CandidateTopK: candidateTopK,
			DenseWeight:   denseWeight,
			SparseWeight:  sparseWeight,
			SparseConfig: &retrieval.SparseRetrieverConfig{
				DefaultTopK: candidateTopK,
			},
			DynamicTopK: retrieval.DynamicTopKConfig{
				Enabled:              profile.EnableDynamicTopK,
				MinTopK:              fallbackInt(profile.MinTopK, cfg.RAG.Phase2.MinTopK),
				MaxTopK:              fallbackInt(profile.MaxTopK, cfg.RAG.Phase2.MaxTopK),
				TokenBudget:          fallbackInt(profile.TokenBudget, cfg.RAG.Phase2.TokenBudget),
				MinAnswerChunks:      fallbackInt(profile.MinAnswerChunks, cfg.RAG.Phase2.MinAnswerChunks),
				StrategicEnabled:     profile.EnableStrategicTopK,
				StrategicMinTopK:     fallbackInt(profile.StrategicTopKMinK, cfg.RAG.Phase3.StrategicTopKMinK),
				StrategicMaxTopK:     fallbackInt(profile.StrategicTopKMaxK, cfg.RAG.Phase3.StrategicTopKMaxK),
				StrategicBudgetRatio: fallbackFloat(profile.StrategicTopKBudgetRatio, cfg.RAG.Phase3.StrategicTopKBudgetRatio),
			},
			ParentChild: retrieval.ParentChildConfig{
				Enabled:      profile.EnableParentChildRetrieval,
				FillStrategy: firstNonEmpty(profile.ParentChildFillStrategy, cfg.RAG.Phase3.ParentChildFillStrategy),
				WindowSize:   fallbackInt(profile.ParentChildWindowSize, cfg.RAG.Phase3.ParentChildWindowSize),
				MaxTokens:    fallbackInt(profile.ParentChildMaxTokens, cfg.RAG.Phase3.ParentChildMaxTokens),
			},
		}
		hybridConfig.RerankerImpl = retrieval.NewJaccardReranker(&retrieval.JaccardRerankerConfig{
			TopK:      candidateTopK,
			ModelName: retrieval.DefaultRerankModelJaccardV1,
			Version:   retrieval.DefaultRerankVersion,
		})
		if profile.EnableAdvancedRerank {
			timeout := time.Duration(fallbackInt(profile.RerankTimeoutMS, cfg.RAG.Phase2.RerankTimeoutMS)) * time.Millisecond
			modelName := strings.TrimSpace(profile.RerankModel)
			if modelName == "" {
				modelName = cfg.RAG.Phase2.RerankModel
			}
			hybridConfig.RerankerImpl = retrieval.NewConfigurableReranker(
				modelName,
				timeout,
				retrieval.NewJaccardReranker(&retrieval.JaccardRerankerConfig{
					TopK:      candidateTopK,
					ModelName: modelName,
					Version:   modelName,
				}),
				hybridConfig.RerankerImpl,
			)
		}
		if profile.EnableQueryRewrite {
			hybridConfig.QueryRewriter = retrieval.NewControlledQueryRewriter(&retrieval.QueryRewriterConfig{
				MaxExpansions: fallbackInt(profile.RewriteMaxExpansions, cfg.RAG.Phase2.RewriteMaxExpansions),
			})
		}
		hybridRetriever, err := retrieval.NewHybridRetriever(manager.GetRetrieverService(), hybridConfig)
		if err != nil {
			return nil, err
		}
		searcher.hybrid = hybridRetriever
	}
	return searcher, nil
}

func (s *retrievalSearcher) Search(ctx context.Context, item evaluation.DatasetCase) ([]evaluation.RetrievedItem, error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	topK := item.TopK
	if topK <= 0 {
		topK = 5
	}
	collection := s.collection
	if strings.TrimSpace(item.Collection) != "" {
		collection = strings.TrimSpace(item.Collection)
	}
	opts := &retrieval.RetrieveOptions{
		TopK:             topK,
		CandidateTopK:    max(topK, s.profile.CandidateTopK),
		Collection:       collection,
		KBScope:          "global",
		ActiveGlobalKBID: firstKBID(item.KBIDs),
		OriginalQuery:    item.Query,
		Expr:             buildKBFilterExpr(item.KBIDs),
		RequestID:        fmt.Sprintf("eval-%s-%s", s.profile.Name, item.ID),
	}

	var docs []*schema.Document
	var err error
	if s.hybrid != nil {
		docs, err = s.hybrid.Search(queryCtx, item.Query, opts)
	} else {
		docs, err = s.retriever.RetrieveWithOptions(queryCtx, item.Query, opts)
	}
	if err != nil {
		return nil, err
	}
	results := make([]evaluation.RetrievedItem, 0, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		results = append(results, evaluation.RetrievedItem{
			ResultID: normalizeResultID(doc),
			Score:    readFloat(doc.MetaData, "score"),
			Route:    readString(doc.MetaData, "route"),
			Citation: evaluation.CitationTarget{
				DocumentID: readUint64(doc.MetaData, "document_id"),
				ChunkID:    firstNonEmpty(doc.ID, readString(doc.MetaData, "chunk_id")),
				FileName:   readString(doc.MetaData, "file_name"),
			},
			Source:    readSource(doc.MetaData),
			RawFields: cloneMap(doc.MetaData),
		})
	}
	return results, nil
}

func normalizeResultID(doc *schema.Document) string {
	if doc == nil {
		return ""
	}
	if strings.TrimSpace(doc.ID) != "" {
		return strings.TrimSpace(doc.ID)
	}
	if chunkID := readString(doc.MetaData, "chunk_id"); chunkID != "" {
		return chunkID
	}
	if documentID := readUint64(doc.MetaData, "document_id"); documentID > 0 {
		return strconv.FormatUint(documentID, 10)
	}
	return ""
}

func buildKBFilterExpr(kbIDs []uint64) string {
	if len(kbIDs) == 0 {
		return ""
	}
	conditions := make([]string, 0, len(kbIDs))
	for _, id := range kbIDs {
		if id > 0 {
			conditions = append(conditions, fmt.Sprintf("metadata['kb_id'] == %d", id))
		}
	}
	if len(conditions) == 0 {
		return ""
	}
	if len(conditions) == 1 {
		return conditions[0]
	}
	return "(" + strings.Join(conditions, " || ") + ")"
}

func firstKBID(ids []uint64) uint64 {
	for _, id := range ids {
		if id > 0 {
			return id
		}
	}
	return 0
}

func fallbackInt(primary, fallback int) int {
	if primary > 0 {
		return primary
	}
	return fallback
}

func fallbackFloat(primary, fallback float64) float64 {
	if primary > 0 {
		return primary
	}
	return fallback
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func readFloat(metadata map[string]interface{}, key string) float64 {
	if metadata == nil {
		return 0
	}
	switch value := metadata[key].(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	}
	return 0
}

func readString(metadata map[string]interface{}, key string) string {
	if metadata == nil {
		return ""
	}
	if value, ok := metadata[key]; ok && value != nil {
		return strings.TrimSpace(fmt.Sprint(value))
	}
	return ""
}

func readUint64(metadata map[string]interface{}, key string) uint64 {
	if metadata == nil {
		return 0
	}
	switch value := metadata[key].(type) {
	case uint64:
		return value
	case uint32:
		return uint64(value)
	case int:
		if value >= 0 {
			return uint64(value)
		}
	case int64:
		if value >= 0 {
			return uint64(value)
		}
	case float64:
		if value >= 0 {
			return uint64(value)
		}
	}
	return 0
}

func readSource(metadata map[string]interface{}) map[string]interface{} {
	if metadata == nil {
		return nil
	}
	source, ok := metadata["source"].(map[string]interface{})
	if !ok || source == nil {
		return nil
	}
	return cloneMap(source)
}

func cloneMap(source map[string]interface{}) map[string]interface{} {
	if source == nil {
		return nil
	}
	cloned := make(map[string]interface{}, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func exitIfErr(err error) {
	if err != nil {
		panic(err)
	}
}
