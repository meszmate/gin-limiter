package ratelimit

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Store interface {
	Limit(ctx context.Context, key string) Info
}

type Info struct {
	Limit         uint
	RemainingHits uint
	ResetTime     time.Time
	RateLimited   bool
}

// --- RedisStore -------------------------------------------------------------

type redisStore struct {
	rdb        *redis.Client
	rateSec    int64
	limit      uint
	panicOnErr bool
}

func NewRedisStore(rdb *redis.Client, rate time.Duration, limit uint, panicOnErr bool) Store {
	return &redisStore{
		rdb:        rdb,
		rateSec:    int64(rate / time.Second),
		limit:      limit,
		panicOnErr: panicOnErr,
	}
}

const lua = `
local tsKey   = KEYS[1]
local hitsKey = KEYS[2]
local rate    = tonumber(ARGV[1])
local limit   = tonumber(ARGV[2])
local now     = tonumber(ARGV[3])

local ts   = tonumber(redis.call("GET", tsKey) or now)
local hits = tonumber(redis.call("GET", hitsKey) or 0)

if ts + rate < now then
    ts   = now
    hits = 0
end

if hits >= limit then
    return {1, limit - hits, ts + rate}
else
    hits = hits + 1
    redis.call("SET", tsKey, ts, "EX", rate * 2)
    redis.call("SET", hitsKey, hits, "EX", rate * 2)
    return {0, limit - hits, ts + rate}
end
`

func (s *redisStore) Limit(ctx context.Context, key string) Info {
	now := time.Now().Unix()
	res, err := s.rdb.Eval(ctx, lua,
		[]string{key + ":ts", key + ":hits"},
		s.rateSec, s.limit, now).Result()
	if err != nil {
		if s.panicOnErr {
			panic(err)
		}
		// fallback: don't block the request
		return Info{
			Limit:         s.limit,
			RemainingHits: s.limit,
			ResetTime:     time.Now().Add(time.Duration(s.rateSec) * time.Second),
			RateLimited:   false,
		}
	}

	arr := res.([]interface{})
	return Info{
		Limit:         s.limit,
		RemainingHits: uint(arr[1].(int64)),
		ResetTime:     time.Unix(arr[2].(int64), 0),
		RateLimited:   arr[0].(int64) == 1,
	}
}
