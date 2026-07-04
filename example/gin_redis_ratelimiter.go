package main

import (
	"context"
	"time"

	"github.com/himkit/ratelimiter"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func main() {
	r := gin.New()
	// Put a token into the token bucket every 1s
	// Maximum 1 request allowed per second
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		panic(err)
	}
	r.Use(ratelimiter.GinRedisRatelimiter(rdb, ratelimiter.GinRatelimiterConfig{
		// config: how to generate a limit key
		LimitKey: func(c *gin.Context) string {
			return c.ClientIP()
		},
		// config: how to respond when limiting
		LimitedHandler: func(c *gin.Context) {
			c.JSON(200, "too many requests!!!")
			c.Abort()
			return
		},
		// config: return ratelimiter token fill interval and bucket size
		TokenBucketConfig: func(*gin.Context) (time.Duration, int) {
			return time.Second * 1, 1
		},
	}))
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, "hi")
	})
	r.Run()
}
