package semantic

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCommander interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	ZAdd(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd
	ZRemRangeByRank(ctx context.Context, key string, start, stop int64) *redis.IntCmd
	ZRevRange(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd
	ZCard(ctx context.Context, key string) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	TTL(ctx context.Context, key string) *redis.DurationCmd
	Incr(ctx context.Context, key string) *redis.IntCmd
	SAdd(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
	SMembers(ctx context.Context, key string) *redis.StringSliceCmd
	SRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
}

type Store struct {
	client RedisCommander
	now    func() time.Time
}

func NewStore(client RedisCommander) *Store {
	return &Store{
		client: client,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *Store) Put(ctx context.Context, scope Scope, entry *Entry, ttl time.Duration, maxEntries int) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("semantic cache store is not initialized")
	}
	if ttl <= 0 {
		return fmt.Errorf("ttl must be > 0")
	}
	if maxEntries <= 0 {
		return fmt.Errorf("max_entries must be > 0")
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	if err := entry.Validate(); err != nil {
		return err
	}

	normalizedScope := scope.Normalize()
	entry.TenantID = normalizedScope.TenantID
	entry.KBIDs = append([]uint64(nil), normalizedScope.KBIDs...)
	entry.KBScope = joinUint64s(normalizedScope.KBIDs)
	entry.StrategyVersion = normalizedScope.StrategyVersion
	entry.QueryType = normalizedScope.QueryType

	now := s.now()
	if entry.EntryID == "" {
		entryID, err := BuildEntryID(normalizedScope, entry.Query, entry.TopK)
		if err != nil {
			return err
		}
		entry.EntryID = entryID
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	entry.LastHitAt = now
	entry.ExpiresAt = now.Add(ttl)

	entryKey, err := EntryKey(entry.EntryID)
	if err != nil {
		return err
	}
	scopeKey, err := ScopeKey(normalizedScope)
	if err != nil {
		return err
	}
	payload, err := EncodeEntry(entry)
	if err != nil {
		return err
	}

	if err := s.client.Set(ctx, entryKey, payload, ttl).Err(); err != nil {
		return err
	}
	// scopeKey 是一个 ZSET：
	// member = entry_id，score = created_at
	// 作用是“按 scope 维护最近的候选 entry 列表”。
	if err := s.client.ZAdd(ctx, scopeKey, redis.Z{
		Score:  float64(entry.CreatedAt.Unix()),
		Member: entry.EntryID,
	}).Err(); err != nil {
		return err
	}
	if err := s.client.Expire(ctx, scopeKey, ttl).Err(); err != nil {
		return err
	}
	for _, kbID := range normalizedScope.KBIDs {
		indexKey, keyErr := KnowledgeBaseScopeIndexKey(normalizedScope.TenantID, kbID)
		if keyErr != nil {
			return keyErr
		}
		// kb_scope_index 是一个 Set：
		// 记录“某个 kb 关联了哪些 scopeKey”。
		// 这样知识库变更时，就能从 kb 反向找到所有相关缓存并清理。
		if err := s.client.SAdd(ctx, indexKey, scopeKey).Err(); err != nil {
			return err
		}
		if err := s.client.Expire(ctx, indexKey, ttl).Err(); err != nil {
			return err
		}
	}

	card, err := s.client.ZCard(ctx, scopeKey).Result()
	if err != nil {
		return err
	}
	if int(card) > maxEntries {
		overflow := int(card) - maxEntries
		if overflow > 0 {
			// 超过每个 scope 的上限后，删除更旧的 entry，避免某个热点 scope 无限膨胀。
			staleIDs, err := s.client.ZRevRange(ctx, scopeKey, int64(maxEntries), int64(maxEntries+overflow-1)).Result()
			if err != nil {
				return err
			}
			if len(staleIDs) > 0 {
				keys := make([]string, 0, len(staleIDs))
				for _, staleID := range staleIDs {
					key, keyErr := EntryKey(staleID)
					if keyErr != nil {
						return keyErr
					}
					keys = append(keys, key)
				}
				if err := s.client.ZRemRangeByRank(ctx, scopeKey, 0, int64(overflow-1)).Err(); err != nil {
					return err
				}
				if err := s.client.Del(ctx, keys...).Err(); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *Store) GetCandidates(ctx context.Context, scope Scope, maxCandidates int) (*LookupResult, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("semantic cache store is not initialized")
	}
	if maxCandidates <= 0 {
		return nil, fmt.Errorf("max_candidates must be > 0")
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}

	normalizedScope := scope.Normalize()
	scopeKey, err := ScopeKey(normalizedScope)
	if err != nil {
		return nil, err
	}

	// 先从 scopeKey 里拿最近 N 个 entry_id，再逐个取 entry payload。
	entryIDs, err := s.client.ZRevRange(ctx, scopeKey, 0, int64(maxCandidates-1)).Result()
	if err != nil {
		return nil, err
	}
	entries := make([]*Entry, 0, len(entryIDs))
	for _, entryID := range entryIDs {
		entryKey, keyErr := EntryKey(entryID)
		if keyErr != nil {
			return nil, keyErr
		}
		payload, getErr := s.client.Get(ctx, entryKey).Bytes()
		if getErr != nil {
			if getErr == redis.Nil {
				continue
			}
			return nil, getErr
		}
		entry, decodeErr := DecodeEntry(payload)
		if decodeErr != nil {
			return nil, decodeErr
		}
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CreatedAt.After(entries[j].CreatedAt)
	})

	return &LookupResult{
		Scope:          normalizedScope,
		Candidates:     entries,
		CandidateCount: len(entries),
	}, nil
}

func (s *Store) Touch(ctx context.Context, entry *Entry, ttl time.Duration) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("semantic cache store is not initialized")
	}
	if ttl <= 0 {
		return fmt.Errorf("ttl must be > 0")
	}
	if err := entry.Validate(); err != nil {
		return err
	}

	entry.LastHitAt = s.now()
	entry.HitCount++
	entry.ExpiresAt = entry.LastHitAt.Add(ttl)
	// Touch 只刷新 entry 本身，不改 scope 排名。
	// 这里的设计重点是“热点续期”，不是“重新排序候选时间线”。
	entryKey, err := EntryKey(entry.EntryID)
	if err != nil {
		return err
	}
	payload, err := EncodeEntry(entry)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, entryKey, payload, ttl).Err()
}

func (s *Store) DeleteScope(ctx context.Context, scope Scope) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("semantic cache store is not initialized")
	}
	if err := scope.Validate(); err != nil {
		return err
	}

	normalizedScope := scope.Normalize()
	scopeKey, err := ScopeKey(normalizedScope)
	if err != nil {
		return err
	}
	entryIDs, err := s.client.ZRevRange(ctx, scopeKey, 0, -1).Result()
	if err != nil {
		return err
	}

	keys := make([]string, 0, len(entryIDs)+1)
	keys = append(keys, scopeKey)
	for _, entryID := range entryIDs {
		entryKey, keyErr := EntryKey(entryID)
		if keyErr != nil {
			return keyErr
		}
		keys = append(keys, entryKey)
	}
	if err := s.client.Del(ctx, keys...).Err(); err != nil {
		return err
	}
	return s.removeScopeKeyFromKBIndexes(ctx, normalizedScope.TenantID, normalizedScope.KBIDs, scopeKey)
}

func (s *Store) DeleteByKnowledgeBase(ctx context.Context, tenantID uint64, kbID uint64) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("semantic cache store is not initialized")
	}
	indexKey, err := KnowledgeBaseScopeIndexKey(tenantID, kbID)
	if err != nil {
		return err
	}
	scopeKeys, err := s.client.SMembers(ctx, indexKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return err
	}
	if len(scopeKeys) == 0 {
		return nil
	}

	// 清理流程是：
	// 1. 先从 kb_scope_index 找到相关 scopeKey
	// 2. 再从每个 scopeKey 找到所有 entry
	// 3. 删除 scope 和 entry，并把反向索引也一起移除
	for _, scopeKey := range scopeKeys {
		entryIDs, rangeErr := s.client.ZRevRange(ctx, scopeKey, 0, -1).Result()
		if rangeErr != nil && rangeErr != redis.Nil {
			return rangeErr
		}
		keys := make([]string, 0, len(entryIDs)+1)
		keys = append(keys, scopeKey)
		for _, entryID := range entryIDs {
			entryKey, keyErr := EntryKey(entryID)
			if keyErr != nil {
				return keyErr
			}
			keys = append(keys, entryKey)
		}
		if err := s.client.Del(ctx, keys...).Err(); err != nil {
			return err
		}
		scopeTenantID, scopeKBIDs := extractScopeIndexFields(scopeKey)
		if removeErr := s.removeScopeKeyFromKBIndexes(ctx, scopeTenantID, scopeKBIDs, scopeKey); removeErr != nil {
			return removeErr
		}
	}
	return s.client.SRem(ctx, indexKey, toInterfaceSlice(scopeKeys)...).Err()
}

func (s *Store) removeScopeKeyFromKBIndexes(ctx context.Context, tenantID uint64, kbIDs []uint64, scopeKey string) error {
	if tenantID == 0 || len(kbIDs) == 0 || strings.TrimSpace(scopeKey) == "" {
		return nil
	}
	for _, kbID := range kbIDs {
		indexKey, err := KnowledgeBaseScopeIndexKey(tenantID, kbID)
		if err != nil {
			return err
		}
		if err := s.client.SRem(ctx, indexKey, scopeKey).Err(); err != nil {
			return err
		}
	}
	return nil
}

func extractScopeIndexFields(scopeKey string) (uint64, []uint64) {
	parts := strings.Split(strings.TrimSpace(scopeKey), ":")
	var (
		tenantID uint64
		kbIDs    []uint64
	)
	for _, part := range parts {
		if strings.HasPrefix(part, "t") {
			if parsed, err := strconv.ParseUint(strings.TrimPrefix(part, "t"), 10, 64); err == nil && parsed > 0 {
				tenantID = parsed
			}
		}
		if strings.HasPrefix(part, "k") {
			rawIDs := strings.Split(strings.TrimPrefix(part, "k"), "-")
			kbIDs = make([]uint64, 0, len(rawIDs))
			for _, rawID := range rawIDs {
				parsed, err := strconv.ParseUint(strings.TrimSpace(rawID), 10, 64)
				if err == nil && parsed > 0 {
					kbIDs = append(kbIDs, parsed)
				}
			}
		}
	}
	return tenantID, kbIDs
}

func toInterfaceSlice(values []string) []interface{} {
	if len(values) == 0 {
		return nil
	}
	out := make([]interface{}, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}
