package ratelimiter

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestRedisRatelimiter(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}

	limiter := NewRedisRatelimiter(rdb)
	key := "TestRedisRatelimiter:" + time.Now().Format(time.RFC3339Nano)
	assert.True(t, limiter.Allow(ctx, key, time.Second*1, 1))
	assert.False(t, limiter.Allow(ctx, key, time.Second*1, 1))
	time.Sleep(1 * time.Second)
	assert.True(t, limiter.Allow(ctx, key, time.Second*1, 1))
}
