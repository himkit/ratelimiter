package ratelimiter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func BenchmarkMemRatelimiterAllow(b *testing.B) {
	limiter := NewMemRatelimiter()
	defer limiter.Stop()
	ctx := context.Background()
	for b.Loop() {
		limiter.Allow(ctx, "bench-key", time.Second, 100)
	}
}

func BenchmarkMemRatelimiterAllowParallel(b *testing.B) {
	limiter := NewMemRatelimiter()
	defer limiter.Stop()
	ctx := context.Background()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			limiter.Allow(ctx, "bench-key-parallel", time.Second, 100)
			i++
		}
	})
}

func BenchmarkGinMemRatelimiter(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(GinMemRatelimiter(GinRatelimiterConfig{
		TokenBucketConfig: func(c *gin.Context) (time.Duration, int) {
			return time.Second, 100
		},
	}))
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, "hi")
	})

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}
	})
}

func BenchmarkRedisRatelimiterAllow(b *testing.B) {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		b.Skipf("redis not available: %v", err)
	}

	limiter := NewRedisRatelimiter(rdb)
	key := "BenchmarkRedisRatelimiterAllow:" + time.Now().Format(time.RFC3339Nano)
	for b.Loop() {
		limiter.Allow(ctx, key, time.Second, 100)
	}
}
