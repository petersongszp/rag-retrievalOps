package retrieval

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestParentChildPostProcessorFillSectionWindow(t *testing.T) {
	processor := &parentChildPostProcessor{
		defaultCollection: "kb_chunks",
		config: ParentChildConfig{
			Enabled:      true,
			FillStrategy: parentFillStrategySectionWindow,
			WindowSize:   1,
			MaxTokens:    200,
		},
		query: func(ctx context.Context, collection string, expr string, limit int) ([]*schema.Document, error) {
			if collection != "kb_chunks" {
				t.Fatalf("unexpected collection %q", collection)
			}
			if !strings.Contains(expr, `metadata["hierarchy_path"]`) && !strings.Contains(expr, `metadata["parent_id"]`) {
				t.Fatalf("expected hierarchy or parent expr, got %q", expr)
			}
			return []*schema.Document{
				makeParentChildDoc("doc-1-child-000", 0, "Overview of the API gateway and auth flow."),
				makeParentChildDoc("doc-1-child-001", 1, "The storage layer persists vectors and metadata for retrieval."),
				makeParentChildDoc("doc-1-child-002", 2, "Caching and throttling are enforced before dispatch."),
			}, nil
		},
	}

	child := makeParentChildDoc("doc-1-child-001", 1, "The storage layer persists vectors and metadata for retrieval.")
	filled, stats := processor.Fill(context.Background(), []*schema.Document{child})
	if len(filled) != 1 {
		t.Fatalf("expected one filled doc, got %d", len(filled))
	}
	if stats.FilledCount != 1 {
		t.Fatalf("expected filled_count=1, got %d", stats.FilledCount)
	}
	if !strings.Contains(filled[0].Content, "Overview of the API gateway") {
		t.Fatalf("expected filled content to include sibling context, got %q", filled[0].Content)
	}
	if filled[0].MetaData["parent_fill_applied"] != true {
		t.Fatalf("expected parent_fill_applied=true, got %v", filled[0].MetaData["parent_fill_applied"])
	}
	if filled[0].MetaData["parent_fill_strategy"] != parentFillStrategySectionWindow {
		t.Fatalf("expected parent_fill_strategy=%q, got %v", parentFillStrategySectionWindow, filled[0].MetaData["parent_fill_strategy"])
	}
	source := ensureSourceMetadata(filled[0])
	if source["parent_fill_tokens"] == nil {
		t.Fatalf("expected source.parent_fill_tokens to be populated")
	}
}

func TestParentChildPostProcessorHonorsBudget(t *testing.T) {
	processor := &parentChildPostProcessor{
		defaultCollection: "kb_chunks",
		config: ParentChildConfig{
			Enabled:      true,
			FillStrategy: parentFillStrategyChildFirstWithParent,
			WindowSize:   2,
			MaxTokens:    18,
		},
		query: func(ctx context.Context, collection string, expr string, limit int) ([]*schema.Document, error) {
			return []*schema.Document{
				makeParentChildDoc("doc-2-child-000", 0, "Short child evidence."),
				makeParentChildDoc("doc-2-child-001", 1, "This context block is intentionally much longer and should not fit in the remaining token budget."),
				makeParentChildDoc("doc-2-child-002", 2, "Tiny note."),
			}, nil
		},
	}

	child := makeParentChildDoc("doc-2-child-000", 0, "Short child evidence.")
	filled, stats := processor.Fill(context.Background(), []*schema.Document{child})
	if stats.FilledCount != 1 {
		t.Fatalf("expected filled_count=1, got %d", stats.FilledCount)
	}
	if strings.Contains(filled[0].Content, "intentionally much longer") {
		t.Fatalf("expected oversized context to be excluded, got %q", filled[0].Content)
	}
	if !strings.Contains(filled[0].Content, "Tiny note.") {
		t.Fatalf("expected smaller parent context to remain, got %q", filled[0].Content)
	}
	if filled[0].MetaData["parent_fill_reason"] != ParentFillReasonBudgetLimited && filled[0].MetaData["parent_fill_reason"] != ParentFillReasonApplied {
		t.Fatalf("unexpected parent_fill_reason=%v", filled[0].MetaData["parent_fill_reason"])
	}
}

func TestParentChildPostProcessorFallsBackToChildOnly(t *testing.T) {
	processor := &parentChildPostProcessor{
		defaultCollection: "kb_chunks",
		config: ParentChildConfig{
			Enabled:      true,
			FillStrategy: parentFillStrategyParentOnly,
			WindowSize:   0,
			MaxTokens:    120,
		},
		query: func(ctx context.Context, collection string, expr string, limit int) ([]*schema.Document, error) {
			return nil, fmt.Errorf("query failed")
		},
	}

	child := makeParentChildDoc("doc-3-child-000", 0, "Only child evidence.")
	filled, stats := processor.Fill(context.Background(), []*schema.Document{child})
	if len(filled) != 1 {
		t.Fatalf("expected one doc, got %d", len(filled))
	}
	if stats.FallbackCount != 1 {
		t.Fatalf("expected fallback_count=1, got %d", stats.FallbackCount)
	}
	if filled[0].Content != "Only child evidence." {
		t.Fatalf("expected child content to remain unchanged, got %q", filled[0].Content)
	}
	if filled[0].MetaData["parent_fill_applied"] != false {
		t.Fatalf("expected parent_fill_applied=false, got %v", filled[0].MetaData["parent_fill_applied"])
	}
	if filled[0].MetaData["parent_fill_reason"] != ParentFillReasonQueryFailed {
		t.Fatalf("expected parent_fill_reason=%q, got %v", ParentFillReasonQueryFailed, filled[0].MetaData["parent_fill_reason"])
	}
}

func TestParentChildFetchCandidatesMergesFallbackExprs(t *testing.T) {
	processor := &parentChildPostProcessor{
		defaultCollection: "kb_chunks",
		config: ParentChildConfig{
			Enabled:      true,
			FillStrategy: parentFillStrategySectionWindow,
			WindowSize:   1,
			MaxTokens:    200,
		},
	}

	child := makeParentChildDoc("doc-1-child-001", 1, "The storage layer persists vectors and metadata for retrieval.")
	queryCalls := 0
	processor.query = func(ctx context.Context, collection string, expr string, limit int) ([]*schema.Document, error) {
		queryCalls++
		switch queryCalls {
		case 1:
			return []*schema.Document{
				makeParentChildDoc("doc-1-child-001", 1, "The storage layer persists vectors and metadata for retrieval."),
			}, nil
		case 2:
			return []*schema.Document{
				makeParentChildDoc("doc-1-child-000", 0, "Overview of the API gateway and auth flow."),
			}, nil
		default:
			return nil, nil
		}
	}

	candidates, err := processor.fetchCandidates(context.Background(), child, "kb_chunks")
	if err != nil {
		t.Fatalf("fetchCandidates failed: %v", err)
	}
	if queryCalls != 2 {
		t.Fatalf("expected both exprs to be queried, got %d calls", queryCalls)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected 2 merged candidates, got %d", len(candidates))
	}
	if readMetadataString(candidates[0], "hierarchy_path") != "Guide > Storage" {
		t.Fatalf("expected merged candidates to preserve metadata, got %v", candidates[0].MetaData)
	}
}

func TestParentChildPostProcessorDeduplicatesFilledSameParent(t *testing.T) {
	processor := &parentChildPostProcessor{
		defaultCollection: "kb_chunks",
		config: ParentChildConfig{
			Enabled:      true,
			FillStrategy: parentFillStrategySectionWindow,
			WindowSize:   2,
			MaxTokens:    200,
		},
		query: func(ctx context.Context, collection string, expr string, limit int) ([]*schema.Document, error) {
			return []*schema.Document{
				makeParentChildDoc("doc-1-child-004", 4, "Billing rules introduction."),
				makeParentChildDoc("doc-1-child-005", 5, "Billing formula table continuation."),
				makeParentChildDoc("doc-1-child-006", 6, "Billing cycle details."),
			}, nil
		},
	}

	first := makeParentChildDoc("doc-1-child-004", 4, "Billing rules introduction.")
	second := makeParentChildDoc("doc-1-child-005", 5, "Billing formula table continuation.")
	second.MetaData["score"] = 0.4

	filled, _ := processor.Fill(context.Background(), []*schema.Document{first, second})
	if len(filled) != 1 {
		t.Fatalf("expected duplicate filled parent results to collapse to one, got %d", len(filled))
	}
	if filled[0].ID != "doc-1-child-004" {
		t.Fatalf("expected highest scored child to be retained, got %q", filled[0].ID)
	}
	if filled[0].MetaData["parent_fill_applied"] != true {
		t.Fatalf("expected retained doc to keep parent fill metadata, got %v", filled[0].MetaData["parent_fill_applied"])
	}
}

func TestParentChildPostProcessorTrimsLegacyTableRowsDuplication(t *testing.T) {
	processor := &parentChildPostProcessor{
		defaultCollection: "kb_chunks",
		config: ParentChildConfig{
			Enabled:      true,
			FillStrategy: parentFillStrategySectionWindow,
			WindowSize:   1,
			MaxTokens:    200,
		},
	}
	doc := &schema.Document{
		ID: "doc-1-child-007",
		Content: strings.TrimSpace(`Table table-001

| 项目 | 按量付费 |
| --- | --- |
| 计费公式 | 计算规格费用=实例购买后的服务时长（小时）×规格单价（元/小时） |

Rows:
项目: 计费公式 | 按量付费: 计算规格费用=实例购买后的服务时长（小时）×规格单价（元/小时）`),
		MetaData: map[string]interface{}{
			"document_id":            uint64(1),
			"chunk_id":               "doc-1-child-007",
			"child_id":               "doc-1-child-007",
			"chunking_unit":          "table",
			"parent_child_available": false,
			"score":                  0.58,
		},
	}

	filled, _ := processor.Fill(context.Background(), []*schema.Document{doc})
	if len(filled) != 1 {
		t.Fatalf("expected one table doc, got %d", len(filled))
	}
	if strings.Contains(filled[0].Content, "\nRows:\n") {
		t.Fatalf("expected legacy rendered Rows block to be trimmed, got %q", filled[0].Content)
	}
	if count := strings.Count(filled[0].Content, "计算规格费用=实例购买后的服务时长"); count != 1 {
		t.Fatalf("expected formula to appear once after trim, got %d in %q", count, filled[0].Content)
	}
}

func makeParentChildDoc(childID string, chunkIndex int, content string) *schema.Document {
	documentID := uint64(1)
	return &schema.Document{
		ID:      childID,
		Content: content,
		MetaData: map[string]interface{}{
			"document_id":            documentID,
			"chunk_id":               childID,
			"child_id":               childID,
			"parent_id":              "doc-1-parent-001",
			"chunk_index":            chunkIndex,
			"hierarchy_path":         "Guide > Storage",
			"section_title":          "Storage",
			"parent_child_available": true,
			"collection":             "kb_chunks",
			"child_start_offset":     chunkIndex * 100,
			"child_end_offset":       chunkIndex*100 + len(content),
			"score":                  0.9 - float64(chunkIndex)*0.1,
		},
	}
}
