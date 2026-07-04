package ratelimiter

import (
	"github.com/gin-gonic/gin"
)

// GinMemRatelimiter returns a Gin middleware that rate-limits requests using an in-memory token bucket.
func GinMemRatelimiter(conf GinRatelimiterConfig) gin.HandlerFunc {
	if conf.TokenBucketConfig == nil {
		panic("GinRatelimiterConfig must implement the TokenBucketConfig callback function")
	}
	limiter := NewMemRatelimiter()

	return func(c *gin.Context) {
		// Determine the limit key for this request.
		limitKey := DefaultGinLimitKey(c)
		if conf.LimitKey != nil {
			limitKey = conf.LimitKey(c)
		}

		limitedHandler := DefaultGinLimitedHandler
		if conf.LimitedHandler != nil {
			limitedHandler = conf.LimitedHandler
		}

		tokenFillInterval, bucketSize := conf.TokenBucketConfig(c)

		if !limiter.Allow(c, limitKey, tokenFillInterval, bucketSize) {
			limitedHandler(c)
			return
		}
		c.Next()
	}
}
