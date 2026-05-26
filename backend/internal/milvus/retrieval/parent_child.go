package retrieval

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cloudwego/eino/schema"
	milvusClient "github.com/milvus-io/milvus-sdk-go/v2/client"
)

const (
	parentFillStrategyParentOnly           = "parent_only"
	parentFillStrategySiblingWindow        = "sibling_window"
	parentFillStrategySectionWindow        = "section_window"
	parentFillStrategyChildFirstWithParent = "child_first_with_parent_summary"
	parentFillStrategyChildOnly            = "child_only"

	ParentFillReasonApplied             = "applied"
	ParentFillReasonSkippedUnavailable  = "parent_child_unavailable"
	ParentFillReasonSkippedInsufficient = "insufficient_context"
	ParentFillReasonQueryFailed         = "query_failed"
	ParentFillReasonBudgetLimited       = "budget_limited"
)

type ParentChildConfig struct {
	Enabled      bool
	FillStrategy string
	WindowSize   int
	MaxTokens    int
}

type ParentChildFillStats struct {
	FilledCount   int
	FallbackCount int
	FilledTokens  int
	Strategy      string
}

type parentChildQueryExecutor func(ctx context.Context, collection string, expr string, limit int) ([]*schema.Document, error)

type parentChildPostProcessor struct {
	defaultCollection string
	config            ParentChildConfig
	query             parentChildQueryExecutor
}

func newParentChildPostProcessor(client milvusClient.Client, defaultCollection string, cfg ParentChildConfig) *parentChildPostProcessor {
	if !cfg.Enabled {
		return nil
	}
	cfg = normalizeParentChildConfig(cfg)
	return &parentChildPostProcessor{
		defaultCollection: strings.TrimSpace(defaultCollection),
		config:            cfg,
		query:             buildParentChildQueryExecutor(client),
	}
}

func normalizeParentChildConfig(cfg ParentChildConfig) ParentChildConfig {
	if strings.TrimSpace(cfg.FillStrategy) == "" {
		cfg.FillStrategy = parentFillStrategySectionWindow
	}
	switch cfg.FillStrategy {
	case parentFillStrategyParentOnly, parentFillStrategySiblingWindow, parentFillStrategySectionWindow, parentFillStrategyChildFirstWithParent:
	default:
		cfg.FillStrategy = parentFillStrategySectionWindow
	}
	if cfg.WindowSize < 0 {
		cfg.WindowSize = 0
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 1200
	}
	return cfg
}

func buildParentChildQueryExecutor(client milvusClient.Client) parentChildQueryExecutor {
	if client == nil {
		return nil
	}
	return func(ctx context.Context, collection string, expr string, limit int) ([]*schema.Document, error) {
		if strings.TrimSpace(collection) == "" {
			return nil, fmt.Errorf("collection is empty")
		}
		resultSet, err := client.Query(
			ctx,
			collection,
			nil,
			expr,
			[]string{"id", "content", "metadata"},
			milvusClient.WithLimit(int64(limit)),
		)
		if err != nil {
			return nil, err
		}
		return parseQueryResultSet(resultSet), nil
	}
}

func (p *parentChildPostProcessor) Fill(ctx context.Context, docs []*schema.Document) ([]*schema.Document, ParentChildFillStats) {
	stats := ParentChildFillStats{
		Strategy: p.config.FillStrategy,
	}
	if p == nil || !p.config.Enabled || len(docs) == 0 {
		return docs, stats
	}

	filled := make([]*schema.Document, 0, len(docs))
	for _, doc := range docs {
		out, applied, tokens, err := p.fillDocument(ctx, doc)
		if err != nil {
			stats.FallbackCount++
			fallbackDoc := cloneDocumentWithMetadata(doc)
			annotateParentFillMetadata(fallbackDoc, parentFillStrategyChildOnly, 0, 0, readDocScore(doc), false, ParentFillReasonQueryFailed)
			filled = append(filled, fallbackDoc)
			continue
		}
		if applied {
			stats.FilledCount++
			stats.FilledTokens += tokens
		}
		filled = append(filled, out)
	}

	return filled, stats
}

func (p *parentChildPostProcessor) fillDocument(ctx context.Context, doc *schema.Document) (*schema.Document, bool, int, error) {
	if doc == nil {
		return nil, false, 0, nil
	}

	base := cloneDocumentWithMetadata(doc)
	if base.MetaData == nil {
		base.MetaData = make(map[string]interface{})
	}

	parentID := readMetadataString(base, "parent_id")
	if !castBool(base.MetaData["parent_child_available"]) || parentID == "" {
		annotateParentFillMetadata(base, parentFillStrategyChildOnly, 0, 0, readDocScore(doc), false, ParentFillReasonSkippedUnavailable)
		return base, false, 0, nil
	}

	collection := readCollectionFromDoc(base)
	if collection == "" {
		collection = p.defaultCollection
	}
	if collection == "" || p.query == nil {
		annotateParentFillMetadata(base, parentFillStrategyChildOnly, 0, 0, readDocScore(doc), false, ParentFillReasonQueryFailed)
		return base, false, 0, nil
	}

	candidates, err := p.fetchCandidates(ctx, base, collection)
	if err != nil {
		return nil, false, 0, err
	}

	selected := p.selectCandidates(base, candidates)
	if len(selected) <= 1 {
		annotateParentFillMetadata(base, p.config.FillStrategy, 0, 0, readDocScore(doc), false, ParentFillReasonSkippedInsufficient)
		return base, false, 0, nil
	}

	filledContent, addedCount, addedTokens, reason := p.buildFilledContent(base, selected)
	if strings.TrimSpace(filledContent) == "" || addedCount == 0 || addedTokens == 0 {
		annotateParentFillMetadata(base, p.config.FillStrategy, 0, 0, readDocScore(doc), false, reason)
		return base, false, 0, nil
	}

	base.Content = filledContent
	annotateParentFillMetadata(base, p.config.FillStrategy, addedCount, addedTokens, readDocScore(doc), true, reason)
	return base, true, addedTokens, nil
}

func (p *parentChildPostProcessor) fetchCandidates(ctx context.Context, doc *schema.Document, collection string) ([]*schema.Document, error) {
	exprs := p.buildCandidateExprs(doc)
	if len(exprs) == 0 {
		return []*schema.Document{cloneDocumentWithMetadata(doc)}, nil
	}

	seen := make(map[string]*schema.Document, 8)
	for _, expr := range exprs {
		if strings.TrimSpace(expr) == "" {
			continue
		}
		docs, err := p.query(ctx, collection, expr, p.resolveQueryLimit())
		if err != nil {
			return nil, err
		}
		for _, candidate := range docs {
			if candidate == nil {
				continue
			}
			key := buildDedupeKey(candidate)
			if key == "" {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = candidate
		}
		if len(seen) > 0 {
			break
		}
	}

	childKey := buildDedupeKey(doc)
	if _, exists := seen[childKey]; !exists {
		seen[childKey] = cloneDocumentWithMetadata(doc)
	}

	out := make([]*schema.Document, 0, len(seen))
	for _, candidate := range seen {
		out = append(out, cloneDocumentWithMetadata(candidate))
	}
	sortParentChildDocs(out)
	return out, nil
}

func (p *parentChildPostProcessor) buildCandidateExprs(doc *schema.Document) []string {
	switch p.config.FillStrategy {
	case parentFillStrategySectionWindow:
		if expr := buildHierarchyExpr(doc); expr != "" {
			parentExpr := buildParentExpr(doc)
			if parentExpr != "" && parentExpr != expr {
				return []string{expr, parentExpr}
			}
			return []string{expr}
		}
	}

	if expr := buildParentExpr(doc); expr != "" {
		return []string{expr}
	}
	return nil
}

func buildParentExpr(doc *schema.Document) string {
	parentID := readMetadataString(doc, "parent_id")
	if parentID == "" {
		return ""
	}
	conditions := []string{
		fmt.Sprintf(`metadata["parent_id"] == "%s"`, escapeExprString(parentID)),
	}
	if documentID := readMetadataUint64(doc, "document_id"); documentID > 0 {
		conditions = append(conditions, fmt.Sprintf(`metadata["document_id"] == %d`, documentID))
	}
	return joinExprConditions(conditions...)
}

func buildHierarchyExpr(doc *schema.Document) string {
	hierarchyPath := readMetadataString(doc, "hierarchy_path")
	if hierarchyPath == "" {
		return ""
	}
	conditions := []string{
		fmt.Sprintf(`metadata["hierarchy_path"] == "%s"`, escapeExprString(hierarchyPath)),
	}
	if documentID := readMetadataUint64(doc, "document_id"); documentID > 0 {
		conditions = append(conditions, fmt.Sprintf(`metadata["document_id"] == %d`, documentID))
	}
	return joinExprConditions(conditions...)
}

func joinExprConditions(conditions ...string) string {
	filtered := make([]string, 0, len(conditions))
	for _, condition := range conditions {
		trimmed := strings.TrimSpace(condition)
		if trimmed != "" {
			filtered = append(filtered, "("+trimmed+")")
		}
	}
	return strings.Join(filtered, " && ")
}

func escapeExprString(value string) string {
	replacer := strings.NewReplacer(
		`\\`, `\\\\`,
		`"`, `\\"`,
	)
	return replacer.Replace(strings.TrimSpace(value))
}

func (p *parentChildPostProcessor) resolveQueryLimit() int {
	limit := 24
	switch p.config.FillStrategy {
	case parentFillStrategyParentOnly, parentFillStrategyChildFirstWithParent:
		limit = 48
	case parentFillStrategySiblingWindow:
		limit = maxInt(8, (p.config.WindowSize*2)+5)
	case parentFillStrategySectionWindow:
		limit = maxInt(12, (p.config.WindowSize*4)+8)
	}
	return limit
}

func (p *parentChildPostProcessor) selectCandidates(doc *schema.Document, candidates []*schema.Document) []*schema.Document {
	if len(candidates) == 0 {
		return []*schema.Document{cloneDocumentWithMetadata(doc)}
	}

	childKey := buildDedupeKey(doc)
	childIndex := readMetadataInt(doc, "chunk_index")
	parentID := readMetadataString(doc, "parent_id")
	hierarchyPath := readMetadataString(doc, "hierarchy_path")

	selected := make([]*schema.Document, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		switch p.config.FillStrategy {
		case parentFillStrategySiblingWindow:
			if parentID != "" && readMetadataString(candidate, "parent_id") != parentID {
				continue
			}
			if !withinChunkWindow(candidate, childKey, childIndex, p.config.WindowSize) {
				continue
			}
		case parentFillStrategySectionWindow:
			if hierarchyPath != "" && readMetadataString(candidate, "hierarchy_path") != hierarchyPath {
				continue
			}
			if p.config.WindowSize > 0 && !withinChunkWindow(candidate, childKey, childIndex, p.config.WindowSize) {
				continue
			}
		default:
			if parentID != "" && readMetadataString(candidate, "parent_id") != parentID {
				continue
			}
		}
		selected = append(selected, candidate)
	}

	if len(selected) == 0 {
		selected = append(selected, cloneDocumentWithMetadata(doc))
	}
	ensureChildIncluded(&selected, doc)
	sortParentChildDocs(selected)
	return selected
}

func withinChunkWindow(candidate *schema.Document, childKey string, childIndex int, window int) bool {
	if candidate == nil {
		return false
	}
	if buildDedupeKey(candidate) == childKey {
		return true
	}
	if window <= 0 {
		return false
	}
	candidateIndex := readMetadataInt(candidate, "chunk_index")
	return absInt(candidateIndex-childIndex) <= window
}

func ensureChildIncluded(target *[]*schema.Document, doc *schema.Document) {
	if target == nil || doc == nil {
		return
	}
	childKey := buildDedupeKey(doc)
	for _, candidate := range *target {
		if buildDedupeKey(candidate) == childKey {
			return
		}
	}
	*target = append(*target, cloneDocumentWithMetadata(doc))
}

func sortParentChildDocs(docs []*schema.Document) {
	sort.SliceStable(docs, func(i, j int) bool {
		leftStart := readMetadataInt(docs[i], "child_start_offset")
		rightStart := readMetadataInt(docs[j], "child_start_offset")
		if leftStart != rightStart {
			return leftStart < rightStart
		}
		leftIndex := readMetadataInt(docs[i], "chunk_index")
		rightIndex := readMetadataInt(docs[j], "chunk_index")
		if leftIndex != rightIndex {
			return leftIndex < rightIndex
		}
		return buildDedupeKey(docs[i]) < buildDedupeKey(docs[j])
	})
}

func (p *parentChildPostProcessor) buildFilledContent(childDoc *schema.Document, candidates []*schema.Document) (string, int, int, string) {
	switch p.config.FillStrategy {
	case parentFillStrategyChildFirstWithParent:
		return p.buildChildFirstContent(childDoc, candidates)
	default:
		return p.buildWindowedContent(childDoc, candidates)
	}
}

func (p *parentChildPostProcessor) buildWindowedContent(childDoc *schema.Document, candidates []*schema.Document) (string, int, int, string) {
	selected := selectDocsAroundChild(candidates, childDoc, p.config.MaxTokens)
	if len(selected) <= 1 {
		return strings.TrimSpace(childDoc.Content), 0, 0, ParentFillReasonSkippedInsufficient
	}

	parts := make([]string, 0, len(selected))
	childKey := buildDedupeKey(childDoc)
	addedCount := 0
	totalTokens := 0
	childTokens := estimateDocumentTokens(childDoc)

	for _, doc := range selected {
		content := strings.TrimSpace(doc.Content)
		if content == "" {
			continue
		}
		parts = append(parts, content)
		totalTokens += estimateDocumentTokens(doc)
		if buildDedupeKey(doc) != childKey {
			addedCount++
		}
	}

	if len(parts) <= 1 || addedCount == 0 {
		return strings.TrimSpace(childDoc.Content), 0, 0, ParentFillReasonSkippedInsufficient
	}

	addedTokens := totalTokens - childTokens
	if addedTokens <= 0 {
		return strings.TrimSpace(childDoc.Content), 0, 0, ParentFillReasonSkippedInsufficient
	}

	reason := ParentFillReasonApplied
	if totalTokens >= p.config.MaxTokens {
		reason = ParentFillReasonBudgetLimited
	}
	return strings.Join(parts, "\n\n"), addedCount, addedTokens, reason
}

func (p *parentChildPostProcessor) buildChildFirstContent(childDoc *schema.Document, candidates []*schema.Document) (string, int, int, string) {
	selected := selectDocsAroundChild(candidates, childDoc, p.config.MaxTokens)
	childKey := buildDedupeKey(childDoc)
	childContent := strings.TrimSpace(childDoc.Content)
	childTokens := estimateDocumentTokens(childDoc)
	if childContent == "" {
		return "", 0, 0, ParentFillReasonSkippedInsufficient
	}

	contextParts := make([]string, 0, len(selected))
	addedCount := 0
	addedTokens := 0
	for _, doc := range selected {
		if buildDedupeKey(doc) == childKey {
			continue
		}
		content := strings.TrimSpace(doc.Content)
		if content == "" {
			continue
		}
		docTokens := estimateDocumentTokens(doc)
		if childTokens+addedTokens+docTokens > p.config.MaxTokens {
			break
		}
		contextParts = append(contextParts, content)
		addedCount++
		addedTokens += docTokens
	}

	if len(contextParts) == 0 {
		return childContent, 0, 0, ParentFillReasonSkippedInsufficient
	}

	reason := ParentFillReasonApplied
	if childTokens+addedTokens >= p.config.MaxTokens {
		reason = ParentFillReasonBudgetLimited
	}
	content := "Primary evidence:\n" + childContent + "\n\nParent context:\n" + strings.Join(contextParts, "\n\n")
	return content, addedCount, addedTokens, reason
}

func selectDocsAroundChild(candidates []*schema.Document, childDoc *schema.Document, maxTokens int) []*schema.Document {
	if len(candidates) == 0 {
		return []*schema.Document{cloneDocumentWithMetadata(childDoc)}
	}
	if maxTokens <= 0 {
		maxTokens = 1200
	}

	childKey := buildDedupeKey(childDoc)
	childPos := 0
	for i, candidate := range candidates {
		if buildDedupeKey(candidate) == childKey {
			childPos = i
			break
		}
	}

	selectedIndexes := map[int]struct{}{
		childPos: {},
	}
	totalTokens := estimateDocumentTokens(candidates[childPos])
	left := childPos - 1
	right := childPos + 1
	takeLeft := true

	for left >= 0 || right < len(candidates) {
		next := -1
		if takeLeft && left >= 0 {
			next = left
			left--
		} else if !takeLeft && right < len(candidates) {
			next = right
			right++
		} else if left >= 0 {
			next = left
			left--
		} else if right < len(candidates) {
			next = right
			right++
		}
		takeLeft = !takeLeft
		if next < 0 {
			continue
		}

		docTokens := estimateDocumentTokens(candidates[next])
		if totalTokens+docTokens > maxTokens {
			continue
		}
		selectedIndexes[next] = struct{}{}
		totalTokens += docTokens
	}

	selected := make([]*schema.Document, 0, len(selectedIndexes))
	for idx, candidate := range candidates {
		if _, exists := selectedIndexes[idx]; exists {
			selected = append(selected, candidate)
		}
	}
	sortParentChildDocs(selected)
	return selected
}

func annotateParentFillMetadata(doc *schema.Document, strategy string, fillCount int, fillTokens int, originalScore float64, applied bool, reason string) {
	if doc == nil {
		return
	}
	if doc.MetaData == nil {
		doc.MetaData = make(map[string]interface{})
	}

	doc.MetaData["original_child_score"] = originalScore
	doc.MetaData["parent_fill_strategy"] = strategy
	doc.MetaData["parent_fill_count"] = fillCount
	doc.MetaData["parent_fill_tokens"] = fillTokens
	doc.MetaData["parent_fill_applied"] = applied
	doc.MetaData["parent_fill_reason"] = reason

	source := ensureSourceMetadata(doc)
	source["original_child_score"] = originalScore
	source["parent_fill_strategy"] = strategy
	source["parent_fill_count"] = fillCount
	source["parent_fill_tokens"] = fillTokens
	source["parent_fill_applied"] = applied
	source["parent_fill_reason"] = reason
	doc.MetaData["source"] = source
	annotateParentChildSource(doc)
}

func readMetadataInt(doc *schema.Document, key string) int {
	if doc == nil || doc.MetaData == nil {
		return 0
	}
	value, ok := doc.MetaData[key]
	if !ok || value == nil {
		return 0
	}
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float32:
		return int(v)
	case float64:
		return int(v)
	default:
		var out int
		fmt.Sscanf(strings.TrimSpace(fmt.Sprint(v)), "%d", &out)
		return out
	}
}

func readMetadataUint64(doc *schema.Document, key string) uint64 {
	if doc == nil || doc.MetaData == nil {
		return 0
	}
	value, ok := doc.MetaData[key]
	if !ok || value == nil {
		return 0
	}
	switch v := value.(type) {
	case uint64:
		return v
	case uint32:
		return uint64(v)
	case uint:
		return uint64(v)
	case int:
		if v > 0 {
			return uint64(v)
		}
	case int64:
		if v > 0 {
			return uint64(v)
		}
	case float64:
		if v > 0 {
			return uint64(v)
		}
	}
	return 0
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
