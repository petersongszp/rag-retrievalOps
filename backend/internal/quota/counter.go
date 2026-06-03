package quota

import (
	"context"
	"log"
	"strconv"
	"time"

	"interview-agents/internal/repository"

	"github.com/redis/go-redis/v9"
)

const apiCallCounterLua = `
local k = KEYS[1]
local ttl = tonumber(ARGV[1])
local c = redis.call('INCR', k)
if c == 1 and ttl > 0 then
  redis.call('EXPIRE', k, ttl)
end
return c
`

func IncrAPICall(tenantID uint64) int {
	if tenantID == 0 {
		return 0
	}

	client := repository.GetRedis()
	if client == nil {
		log.Printf("[quota] redis client is nil, skip api call increment for tenant_id=%d", tenantID)
		return 0
	}

	key := apiCallCounterKey(tenantID, time.Now().UTC())
	ttl := secondsUntilNextUTCDay()
	count, err := client.Eval(context.Background(), apiCallCounterLua, []string{key}, ttl).Int()
	if err != nil {
		log.Printf("[quota] failed to increment api call counter for tenant_id=%d: %v", tenantID, err)
		return 0
	}

	return count
}

func GetAPICallCount(tenantID uint64) int {
	if tenantID == 0 {
		return 0
	}

	client := repository.GetRedis()
	if client == nil {
		log.Printf("[quota] redis client is nil, skip api call lookup for tenant_id=%d", tenantID)
		return 0
	}

	key := apiCallCounterKey(tenantID, time.Now().UTC())
	count, err := client.Get(context.Background(), key).Int()
	if err != nil {
		if err == redis.Nil {
			return 0
		}
		log.Printf("[quota] failed to get api call counter for tenant_id=%d: %v", tenantID, err)
		return 0
	}

	return count
}

func apiCallCounterKey(tenantID uint64, now time.Time) string {
	return "quota:api_calls:" + strconv.FormatUint(tenantID, 10) + ":" + now.Format("20060102")
}

func secondsUntilNextUTCDay() int {
	now := time.Now().UTC()
	next := now.Truncate(24 * time.Hour).Add(24 * time.Hour)
	ttl := int(next.Sub(now).Seconds())
	if ttl < 1 {
		return 1
	}
	return ttl
}
