package semantic

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
)

func ScopeKey(scope Scope) (string, error) {
	normalized := scope.Normalize()
	if err := normalized.Validate(); err != nil {
		return "", err
	}
	// scopeKey 代表一组“同边界候选集”。
	// 后续会在 Redis 的有序集合里维护这个 scope 下最近写入的 entry_id。
	return fmt.Sprintf(
		"%s:scope:t%d:k%s:s%s:q%s",
		KeyPrefix,
		normalized.TenantID,
		joinUint64s(normalized.KBIDs),
		safeToken(normalized.StrategyVersion),
		safeToken(normalized.QueryType),
	), nil
}

func EntryKey(entryID string) (string, error) {
	token := strings.TrimSpace(entryID)
	if token == "" {
		return "", fmt.Errorf("entry_id must not be empty")
	}
	return fmt.Sprintf("%s:entry:%s", KeyPrefix, token), nil
}

func KnowledgeBaseScopeIndexKey(tenantID uint64, kbID uint64) (string, error) {
	if tenantID == 0 {
		return "", fmt.Errorf("tenant_id must be > 0")
	}
	if kbID == 0 {
		return "", fmt.Errorf("kb_id must be > 0")
	}
	return fmt.Sprintf("%s:kb_scope_index:t%d:k%d", KeyPrefix, tenantID, kbID), nil
}

func BuildEntryID(scope Scope, query string, topK int) (string, error) {
	normalized := scope.Normalize()
	if err := normalized.Validate(); err != nil {
		return "", err
	}
	if strings.TrimSpace(query) == "" {
		return "", fmt.Errorf("query must not be empty")
	}
	if topK <= 0 {
		return "", fmt.Errorf("top_k must be > 0")
	}
	base := fmt.Sprintf(
		"tenant=%d|kb=%s|strategy=%s|query_type=%s|topk=%d|query=%s",
		normalized.TenantID,
		joinUint64s(normalized.KBIDs),
		normalized.StrategyVersion,
		normalized.QueryType,
		topK,
		strings.TrimSpace(query),
	)
	// 用 scope + query + top_k 做稳定哈希，
	// 这样相同边界下的相同问题会得到同一个 entry_id。
	sum := sha1.Sum([]byte(base))
	return hex.EncodeToString(sum[:]), nil
}

func safeToken(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.ReplaceAll(trimmed, " ", "_")
	trimmed = strings.ReplaceAll(trimmed, ":", "_")
	trimmed = strings.ReplaceAll(trimmed, "|", "_")
	if trimmed == "" {
		return "empty"
	}
	return trimmed
}

func joinUint64s(values []uint64) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, fmt.Sprintf("%d", value))
	}
	return strings.Join(parts, "-")
}
