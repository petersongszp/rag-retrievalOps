package retrieval

import (
	"context"
	"strings"
	"testing"
)

func TestControlledQueryRewriterExpandsAbbreviationAndAlias(t *testing.T) {
	rewriter := NewControlledQueryRewriter(&QueryRewriterConfig{MaxExpansions: 3})

	result := rewriter.Rewrite(context.Background(), "jvm gc")

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

	result := rewriter.Rewrite(context.Background(), "mq rpc")

	if len(result.ExpansionTerms) != 1 {
		t.Fatalf("expansions = %d, want 1", len(result.ExpansionTerms))
	}
}

func TestControlledQueryRewriterCorrectsTypos(t *testing.T) {
	rewriter := NewControlledQueryRewriter(nil)

	result := rewriter.Rewrite(context.Background(), "sprinboot interview")

	if !result.Applied {
		t.Fatalf("expected typo correction to apply")
	}
	if !strings.Contains(result.FinalQuery, "springboot") {
		t.Fatalf("final query = %q, want springboot correction", result.FinalQuery)
	}
}

func TestControlledQueryRewriterBlacklistSkipsRewrite(t *testing.T) {
	rewriter := NewControlledQueryRewriter(nil)

	result := rewriter.Rewrite(context.Background(), `site:example.com "jvm"`)

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

	result := rewriter.Rewrite(ctx, "jvm")

	if result.Applied {
		t.Fatalf("expected canceled context to skip rewrite")
	}
	if result.Strategy != RewriteStrategyTimeout {
		t.Fatalf("strategy = %q, want %q", result.Strategy, RewriteStrategyTimeout)
	}
}

func TestHybridSearchRequestApplyControlledRewriteFallback(t *testing.T) {
	req := &HybridSearchRequest{
		Query:         "jvm",
		OriginalQuery: "jvm",
	}

	req.applyControlledRewrite(context.Background(), NewControlledQueryRewriter(nil))

	if !req.RewriteApplied {
		t.Fatalf("expected request rewrite to be applied")
	}
	if req.FinalQuery == req.OriginalQuery {
		t.Fatalf("expected final query to differ from original query")
	}
}
