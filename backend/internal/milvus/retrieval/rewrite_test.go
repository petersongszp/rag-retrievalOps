package retrieval

import (
	"context"
	"strings"
	"testing"
)

type stubModelRewriteAssistant struct {
	suggestion ModelRewriteSuggestion
	err        error
}

func (s stubModelRewriteAssistant) Assist(ctx context.Context, request ModelRewriteRequest) (ModelRewriteSuggestion, error) {
	return s.suggestion, s.err
}

func TestControlledQueryRewriterExpandsAbbreviationAndAlias(t *testing.T) {
	rewriter := NewControlledQueryRewriter(&QueryRewriterConfig{MaxExpansions: 3})

	result := rewriter.Rewrite(context.Background(), QueryRewriteRequest{Query: "jvm gc"})

	if !result.Applied {
		t.Fatalf("expected rewrite to be applied")
	}
	if result.Strategy != RewriteStrategyRuleBased {
		t.Fatalf("strategy = %q, want %q", result.Strategy, RewriteStrategyRuleBased)
	}
	if !strings.Contains(result.FinalQuery, "java virtual machine") {
		t.Fatalf("expected final query to contain abbreviation expansion, got %q", result.FinalQuery)
	}
	if !strings.Contains(result.FinalQuery, "garbage collection") {
		t.Fatalf("expected final query to contain alias expansion, got %q", result.FinalQuery)
	}
}

func TestControlledQueryRewriterHonorsExpansionLimit(t *testing.T) {
	rewriter := NewControlledQueryRewriter(&QueryRewriterConfig{MaxExpansions: 1})

	result := rewriter.Rewrite(context.Background(), QueryRewriteRequest{Query: "mq rpc"})

	if len(result.ExpansionTerms) != 1 {
		t.Fatalf("expansions = %d, want 1", len(result.ExpansionTerms))
	}
}

func TestControlledQueryRewriterCorrectsTypos(t *testing.T) {
	rewriter := NewControlledQueryRewriter(nil)

	result := rewriter.Rewrite(context.Background(), QueryRewriteRequest{Query: "sprinboot interview"})

	if !result.Applied {
		t.Fatalf("expected typo correction to apply")
	}
	if !strings.Contains(result.FinalQuery, "springboot") {
		t.Fatalf("final query = %q, want springboot correction", result.FinalQuery)
	}
}

func TestControlledQueryRewriterBlacklistSkipsRewrite(t *testing.T) {
	rewriter := NewControlledQueryRewriter(nil)

	result := rewriter.Rewrite(context.Background(), QueryRewriteRequest{Query: `site:example.com "jvm"`})

	if result.Applied {
		t.Fatalf("expected blacklist query to skip rewrite")
	}
	if result.Strategy != RewriteStrategyBlacklist {
		t.Fatalf("strategy = %q, want %q", result.Strategy, RewriteStrategyBlacklist)
	}
	if result.FinalQuery != `site:example.com "jvm"` {
		t.Fatalf("final query = %q, want original query", result.FinalQuery)
	}
}

func TestControlledQueryRewriterTimeoutFallsBack(t *testing.T) {
	rewriter := NewControlledQueryRewriter(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := rewriter.Rewrite(ctx, QueryRewriteRequest{Query: "jvm"})

	if result.Applied {
		t.Fatalf("expected canceled context to skip rewrite")
	}
	if result.Strategy != RewriteStrategyTimeout {
		t.Fatalf("strategy = %q, want %q", result.Strategy, RewriteStrategyTimeout)
	}
}

func TestControlledQueryRewriterAppliesDomainTermsAndRouteSpecificQueries(t *testing.T) {
	provider := NewStaticDomainTermProvider("test-v1")
	provider.RegisterScope("language:java", map[string][]string{
		"juc": {"java.util.concurrent", "abstract queued synchronizer"},
	})
	rewriter := NewControlledQueryRewriter(&QueryRewriterConfig{
		MaxExpansions:              4,
		EnableDomainTerms:          true,
		EnableRouteSpecificRewrite: true,
		DomainTerms:                provider,
	})

	result := rewriter.Rewrite(context.Background(), QueryRewriteRequest{
		Query:    "juc lock",
		Language: LanguageJava,
	})

	if !result.Applied {
		t.Fatalf("expected domain term rewrite to apply")
	}
	if result.TermDictVersion != "test-v1" {
		t.Fatalf("term dict version = %q, want test-v1", result.TermDictVersion)
	}
	if !strings.Contains(result.DenseQuery, "java.util.concurrent") {
		t.Fatalf("dense query = %q, want canonical domain term", result.DenseQuery)
	}
	if !strings.Contains(result.SparseQuery, "abstract queued synchronizer") {
		t.Fatalf("sparse query = %q, want aggressive domain term expansion", result.SparseQuery)
	}
	if result.DenseQuery == result.SparseQuery {
		t.Fatalf("expected route-specific queries to differ, got %q", result.DenseQuery)
	}
}

func TestControlledQueryRewriterAppliesModelAssistedShadow(t *testing.T) {
	rewriter := NewControlledQueryRewriter(&QueryRewriterConfig{
		MaxExpansions:              4,
		EnableRouteSpecificRewrite: true,
		EnableModelAssistedRewrite: true,
		ModelRewriteShadowRatio:    1,
		ModelAssistant: stubModelRewriteAssistant{
			suggestion: ModelRewriteSuggestion{
				NormalizedTerms: []string{"compare and swap"},
				Aliases:         []string{"atomic compare swap"},
				MustKeepTerms:   []string{"cas"},
				RiskLevel:       modelRewriteRiskLow,
			},
		},
	})

	result := rewriter.Rewrite(context.Background(), QueryRewriteRequest{Query: "cas retry"})

	if !result.ModelRewriteApplied {
		t.Fatalf("expected model-assisted rewrite to be applied")
	}
	if !result.ModelRewriteShadow {
		t.Fatalf("expected model-assisted rewrite to stay in shadow mode")
	}
	if !strings.Contains(result.SparseQuery, "compare and swap") {
		t.Fatalf("sparse query = %q, want model expansion", result.SparseQuery)
	}
	if !strings.Contains(result.Strategy, RewriteStrategyModelAssistedShadow) {
		t.Fatalf("strategy = %q, want %q marker", result.Strategy, RewriteStrategyModelAssistedShadow)
	}
}

func TestHybridSearchRequestApplyControlledRewriteFallback(t *testing.T) {
	req := &HybridSearchRequest{
		Query:         "mq rpc",
		OriginalQuery: "mq rpc",
	}

	req.applyControlledRewrite(context.Background(), NewControlledQueryRewriter(&QueryRewriterConfig{
		MaxExpansions:              3,
		EnableRouteSpecificRewrite: true,
	}))

	if !req.RewriteApplied {
		t.Fatalf("expected request rewrite to be applied")
	}
	if req.FinalQuery == req.OriginalQuery {
		t.Fatalf("expected final query to differ from original query")
	}
	if req.DenseQuery == "" || req.SparseQuery == "" {
		t.Fatalf("expected route queries to be populated")
	}
	if req.DenseQuery == req.SparseQuery {
		t.Fatalf("expected dense and sparse route queries to differ for route-specific rewrite")
	}
}
