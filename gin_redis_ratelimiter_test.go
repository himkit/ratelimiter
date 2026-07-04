package ratelimiter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestGinRedisRatelimiter(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	keyPrefix := "TestGinRedisRatelimiter:" + time.Now().Format(time.RFC3339Nano)
	r.Use(GinRedisRatelimiter(rdb, GinRatelimiterConfig{
		LimitKey: func(c *gin.Context) string {
			return keyPrefix + ":" + c.ClientIP()
		},
		TokenBucketConfig: func(c *gin.Context) (time.Duration, int) {
			return 1 * time.Second, 1
		},
	}))
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, "hi")
	})
	time.Sleep(1 * time.Second)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 429, w.Code)

	time.Sleep(1 * time.Second)
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}
