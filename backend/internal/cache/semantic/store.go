package semantic

import (
	"context"
	"fmt"
	"sort"
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
	if err := s.client.ZAdd(ctx, scopeKey, redis.Z{
		Score:  float64(entry.CreatedAt.Unix()),
		Member: entry.EntryID,
	}).Err(); err != nil {
		return err
	}
	if err := s.client.Expire(ctx, scopeKey, ttl).Err(); err != nil {
		return err
	}

	card, err := s.client.ZCard(ctx, scopeKey).Result()
	if err != nil {
		return err
	}
	if int(card) > maxEntries {
		overflow := int(card) - maxEntries
		if overflow > 0 {
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
	return s.client.Del(ctx, keys...).Err()
}
