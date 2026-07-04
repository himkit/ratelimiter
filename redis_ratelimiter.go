package ratelimiter

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var (
	// RedisRatelimiterCacheExpiration is the default TTL for Redis rate limiter keys.
	RedisRatelimiterCacheExpiration = time.Minute * 60
)

// RedisRatelimiter is a distributed token-bucket rate limiter backed by Redis.
type RedisRatelimiter struct {
	*redis.Client
	script *redis.Script
}

// NewRedisRatelimiter creates a Redis-backed rate limiter using the provided client.
func NewRedisRatelimiter(rdb *redis.Client) *RedisRatelimiter {
	return &RedisRatelimiter{
		Client: rdb,
		script: tokenBucketRedisLuaIsLimitedScript,
	}
}

// Allow reports whether the request identified by key is permitted under the token bucket.
func (r *RedisRatelimiter) Allow(ctx context.Context, key string, tokenFillInterval time.Duration, bucketSize int) bool {
	// Reject immediately when the bucket parameters are invalid.
	if tokenFillInterval <= 0 || bucketSize <= 0 {
		return false
	}

	// Build the Lua script arguments.
	keys := []string{key}
	args := []interface{}{
		bucketSize,
		1, // The script supports variable refill counts; the Go wrapper refills one token at a time.
		tokenFillInterval.Microseconds(),
		RedisRatelimiterCacheExpiration.Seconds(),
	}
	// Run the Lua script on Redis to decide if the key is limited.
	// go-redis.Script.Run uses EVALSHA automatically to save bandwidth.
	limited, err := r.script.Run(ctx, r.Client, keys, args...).Int64()
	if err != nil {
		// Fail open on Redis errors.
		zap.L().Error("RedisRatelimiter run script error", zap.Error(err))
		return true
	}
	return limited == 0
}
