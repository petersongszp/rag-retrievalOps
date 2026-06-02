package retrieval

import (
	"fmt"
	"strings"
)

// BuildFilterExpr 构建过滤表达式。
func BuildFilterExpr(opts *RetrieveOptions) string {
	if opts == nil {
		return ""
	}

	conditions := make([]string, 0, 6)
	if opts.TenantID > 0 {
		conditions = append(conditions, fmt.Sprintf("metadata['tenant_id'] == %d", opts.TenantID))
	}
	if kbExpr := buildAllowedKBIDsExpr(opts.AllowedKBIDs); kbExpr != "" {
		conditions = append(conditions, kbExpr)
	}
	if opts.KBScope != "" {
		conditions = append(conditions, fmt.Sprintf("metadata[\"kb_scope\"] == '%s'", opts.KBScope))
	}
	if opts.ActiveGlobalKBID > 0 {
		conditions = append(conditions, fmt.Sprintf("metadata[\"kb_id\"] == %d", opts.ActiveGlobalKBID))
	}
	if opts.Language != "" {
		conditions = append(conditions, fmt.Sprintf("metadata['language'] == '%s'", string(opts.Language)))
	}
	if opts.Category != "" {
		conditions = append(conditions, fmt.Sprintf("metadata['category'] == '%s'", string(opts.Category)))
	}

	baseExpr := strings.Join(conditions, " && ")
	customExpr := strings.TrimSpace(opts.Expr)
	switch {
	case baseExpr == "":
		return customExpr
	case customExpr == "":
		return baseExpr
	default:
		return fmt.Sprintf("(%s) && (%s)", baseExpr, customExpr)
	}
}

func buildAllowedKBIDsExpr(ids []uint64) string {
	values := make([]string, 0, len(ids))
	seen := make(map[uint64]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		values = append(values, fmt.Sprintf("%d", id))
	}
	if len(values) == 0 {
		return ""
	}
	return fmt.Sprintf("metadata['kb_id'] in [%s]", strings.Join(values, ", "))
}
