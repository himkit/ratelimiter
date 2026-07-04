package ratelimiter

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// DefaultGinLimitKey returns the default rate-limit key using the client IP and request path.
func DefaultGinLimitKey(c *gin.Context) string {
	return fmt.Sprintf("pink-lady:ratelimiter:%s:%s", c.ClientIP(), c.FullPath())
}

// DefaultGinLimitedHandler aborts the request with 429 Too Many Requests.
func DefaultGinLimitedHandler(c *gin.Context) {
	c.AbortWithStatus(http.StatusTooManyRequests)
}

// GinRatelimiterConfig configures the Gin rate-limiting middleware.
type GinRatelimiterConfig struct {
	// LimitKey returns the rate-limit key for a request.
	// Defaults to DefaultGinLimitKey when nil.
	LimitKey func(*gin.Context) string
	// LimitedHandler runs when the request is rate-limited.
	// Defaults to DefaultGinLimitedHandler when nil.
	LimitedHandler func(*gin.Context)
	// TokenBucketConfig returns the token refill interval and bucket size for a request.
	TokenBucketConfig func(*gin.Context) (time.Duration, int)
}
