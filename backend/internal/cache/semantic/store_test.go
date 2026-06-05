package semantic

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type fakeRedis struct {
	values    map[string]string
	expiresAt map[string]time.Time
	zsets     map[string][]redis.Z
}

func newFakeRedis() *fakeRedis {
	return &fakeRedis{
		values:    make(map[string]string),
		expiresAt: make(map[string]time.Time),
		zsets:     make(map[string][]redis.Z),
	}
}

func (f *fakeRedis) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx)
	str, ok := value.(string)
	if !ok {
		bytes, bytesOK := value.([]byte)
		if bytesOK {
			str = string(bytes)
		}
	}
	f.values[key] = str
	if expiration > 0 {
		f.expiresAt[key] = time.Now().UTC().Add(expiration)
	}
	cmd.SetVal("OK")
	return cmd
}

func (f *fakeRedis) Get(ctx context.Context, key string) *redis.StringCmd {
	cmd := redis.NewStringCmd(ctx)
	value, ok := f.values[key]
	if !ok {
		cmd.SetErr(redis.Nil)
		return cmd
	}
	cmd.SetVal(value)
	return cmd
}

func (f *fakeRedis) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	var deleted int64
	for _, key := range keys {
		if _, ok := f.values[key]; ok {
			delete(f.values, key)
			delete(f.expiresAt, key)
			deleted++
		}
		if _, ok := f.zsets[key]; ok {
			delete(f.zsets, key)
			delete(f.expiresAt, key)
			deleted++
		}
	}
	cmd.SetVal(deleted)
	return cmd
}

func (f *fakeRedis) ZAdd(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	items := append([]redis.Z(nil), f.zsets[key]...)
	for _, member := range members {
		replaced := false
		for i := range items {
			if items[i].Member == member.Member {
				items[i] = member
				replaced = true
				break
			}
		}
		if !replaced {
			items = append(items, member)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Score < items[j].Score
	})
	f.zsets[key] = items
	cmd.SetVal(int64(len(members)))
	return cmd
}

func (f *fakeRedis) ZRemRangeByRank(ctx context.Context, key string, start, stop int64) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	items := append([]redis.Z(nil), f.zsets[key]...)
	if len(items) == 0 {
		cmd.SetVal(0)
		return cmd
	}
	if start < 0 {
		start = 0
	}
	if stop >= int64(len(items)) {
		stop = int64(len(items) - 1)
	}
	if start > stop {
		cmd.SetVal(0)
		return cmd
	}
	removed := stop - start + 1
	filtered := make([]redis.Z, 0, len(items)-int(removed))
	filtered = append(filtered, items[:start]...)
	filtered = append(filtered, items[stop+1:]...)
	f.zsets[key] = filtered
	cmd.SetVal(removed)
	return cmd
}

func (f *fakeRedis) ZRevRange(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd {
	cmd := redis.NewStringSliceCmd(ctx)
	items := append([]redis.Z(nil), f.zsets[key]...)
	sort.Slice(items, func(i, j int) bool {
		return items[i].Score > items[j].Score
	})
	if len(items) == 0 {
		cmd.SetVal([]string{})
		return cmd
	}
	if stop < 0 || stop >= int64(len(items)) {
		stop = int64(len(items) - 1)
	}
	if start < 0 {
		start = 0
	}
	if start > stop {
		cmd.SetVal([]string{})
		return cmd
	}
	values := make([]string, 0, stop-start+1)
	for _, item := range items[start : stop+1] {
		values = append(values, item.Member.(string))
	}
	cmd.SetVal(values)
	return cmd
}

func (f *fakeRedis) ZCard(ctx context.Context, key string) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	cmd.SetVal(int64(len(f.zsets[key])))
	return cmd
}

func (f *fakeRedis) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	cmd := redis.NewBoolCmd(ctx)
	if expiration > 0 {
		f.expiresAt[key] = time.Now().UTC().Add(expiration)
	}
	cmd.SetVal(true)
	return cmd
}

func (f *fakeRedis) TTL(ctx context.Context, key string) *redis.DurationCmd {
	cmd := redis.NewDurationCmd(ctx, time.Second)
	exp, ok := f.expiresAt[key]
	if !ok {
		cmd.SetVal(-1)
		return cmd
	}
	cmd.SetVal(time.Until(exp))
	return cmd
}

func (f *fakeRedis) Incr(ctx context.Context, key string) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	cmd.SetVal(1)
	return cmd
}

func TestScopeKeyNormalizesScope(t *testing.T) {
	key, err := ScopeKey(Scope{
		TenantID:        12,
		KBIDs:           []uint64{9, 2, 9, 4},
		StrategyVersion: "phase4-semantic-v1",
		QueryType:       "retrieve",
	})
	if err != nil {
		t.Fatalf("ScopeKey failed: %v", err)
	}
	expected := "rag:semantic_cache:scope:t12:k2-4-9:sphase4-semantic-v1:qretrieve"
	if key != expected {
		t.Fatalf("ScopeKey = %q, want %q", key, expected)
	}
}

func TestBuildEntryIDStable(t *testing.T) {
	scope := Scope{
		TenantID:        1,
		KBIDs:           []uint64{3, 2},
		StrategyVersion: "phase4-semantic-v1",
		QueryType:       "retrieve",
	}
	id1, err := BuildEntryID(scope, "what is cache", 5)
	if err != nil {
		t.Fatalf("BuildEntryID failed: %v", err)
	}
	id2, err := BuildEntryID(Scope{
		TenantID:        1,
		KBIDs:           []uint64{2, 3},
		StrategyVersion: "phase4-semantic-v1",
		QueryType:       "retrieve",
	}, "what is cache", 5)
	if err != nil {
		t.Fatalf("BuildEntryID failed: %v", err)
	}
	if id1 != id2 {
		t.Fatalf("expected stable entry id, got %q vs %q", id1, id2)
	}
}

func TestEncodeDecodeEntry(t *testing.T) {
	entry := &Entry{
		EntryID:          "entry-1",
		TenantID:         1,
		KBScope:          "2-3",
		KBIDs:            []uint64{2, 3},
		StrategyVersion:  "phase4-semantic-v1",
		RetrieverVersion: "phase1-dense-v1",
		QueryType:        "retrieve",
		Query:            "cache me",
		QueryEmbedding:   []float32{0.1, 0.2, 0.3},
		ResponsePayload:  json.RawMessage(`{"items":[{"content":"doc"}]}`),
		ResultPayload:    ResultPayloadTag,
		TopK:             5,
		CreatedAt:        time.Now().UTC(),
		ExpiresAt:        time.Now().UTC().Add(15 * time.Minute),
	}
	data, err := EncodeEntry(entry)
	if err != nil {
		t.Fatalf("EncodeEntry failed: %v", err)
	}
	decoded, err := DecodeEntry(data)
	if err != nil {
		t.Fatalf("DecodeEntry failed: %v", err)
	}
	if decoded.Query != entry.Query || decoded.TopK != entry.TopK {
		t.Fatalf("decoded entry mismatch: %+v", decoded)
	}
}

func TestStorePutGetCandidatesTouchAndDeleteScope(t *testing.T) {
	fake := newFakeRedis()
	store := NewStore(fake)
	now := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }

	scope := Scope{
		TenantID:        12,
		KBIDs:           []uint64{5, 1},
		StrategyVersion: "phase4-semantic-v1",
		QueryType:       "retrieve",
	}
	entry := &Entry{
		TenantID:         12,
		KBIDs:            []uint64{1, 5},
		StrategyVersion:  "phase4-semantic-v1",
		RetrieverVersion: "phase1-dense-v1",
		QueryType:        "retrieve",
		Query:            "what is semantic cache",
		QueryEmbedding:   []float32{0.1, 0.9},
		ResponsePayload:  json.RawMessage(`{"items":[{"content":"semantic cache intro"}]}`),
		ResultPayload:    ResultPayloadTag,
		TopK:             5,
	}
	if err := store.Put(context.Background(), scope, entry, 15*time.Minute, 5); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	result, err := store.GetCandidates(context.Background(), scope, 3)
	if err != nil {
		t.Fatalf("GetCandidates failed: %v", err)
	}
	if result.CandidateCount != 1 {
		t.Fatalf("expected 1 candidate, got %d", result.CandidateCount)
	}
	if result.Candidates[0].EntryID == "" {
		t.Fatal("expected entry id to be generated")
	}
	if !strings.Contains(string(result.Candidates[0].ResponsePayload), "semantic cache intro") {
		t.Fatalf("unexpected payload: %s", string(result.Candidates[0].ResponsePayload))
	}

	store.now = func() time.Time { return now.Add(5 * time.Minute) }
	if err := store.Touch(context.Background(), result.Candidates[0], 15*time.Minute); err != nil {
		t.Fatalf("Touch failed: %v", err)
	}
	touched, err := store.GetCandidates(context.Background(), scope, 1)
	if err != nil {
		t.Fatalf("GetCandidates after touch failed: %v", err)
	}
	if touched.Candidates[0].HitCount != 1 {
		t.Fatalf("expected hit_count=1, got %d", touched.Candidates[0].HitCount)
	}

	if err := store.DeleteScope(context.Background(), scope); err != nil {
		t.Fatalf("DeleteScope failed: %v", err)
	}
	deleted, err := store.GetCandidates(context.Background(), scope, 3)
	if err != nil {
		t.Fatalf("GetCandidates after delete failed: %v", err)
	}
	if deleted.CandidateCount != 0 {
		t.Fatalf("expected empty scope after delete, got %d", deleted.CandidateCount)
	}
}

func TestStorePutEvictsOldestEntriesWhenScopeIsFull(t *testing.T) {
	fake := newFakeRedis()
	store := NewStore(fake)
	scope := Scope{
		TenantID:        9,
		KBIDs:           []uint64{7},
		StrategyVersion: "phase4-semantic-v1",
		QueryType:       "retrieve",
	}
	baseTime := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)

	for i := 0; i < 3; i++ {
		current := baseTime.Add(time.Duration(i) * time.Minute)
		store.now = func() time.Time { return current }
		entry := &Entry{
			TenantID:         9,
			KBIDs:            []uint64{7},
			StrategyVersion:  "phase4-semantic-v1",
			RetrieverVersion: "phase1-dense-v1",
			QueryType:        "retrieve",
			Query:            "query-" + string(rune('A'+i)),
			QueryEmbedding:   []float32{float32(i + 1)},
			ResponsePayload:  json.RawMessage(`{"items":[{"content":"doc"}]}`),
			ResultPayload:    ResultPayloadTag,
			TopK:             5,
		}
		if err := store.Put(context.Background(), scope, entry, 15*time.Minute, 2); err != nil {
			t.Fatalf("Put #%d failed: %v", i, err)
		}
	}

	result, err := store.GetCandidates(context.Background(), scope, 5)
	if err != nil {
		t.Fatalf("GetCandidates failed: %v", err)
	}
	if result.CandidateCount != 2 {
		t.Fatalf("expected 2 candidates after eviction, got %d", result.CandidateCount)
	}
	if result.Candidates[0].Query != "query-C" || result.Candidates[1].Query != "query-B" {
		t.Fatalf("unexpected candidate order after eviction: %+v", []string{result.Candidates[0].Query, result.Candidates[1].Query})
	}
}
