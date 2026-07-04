package ratelimiter

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGinMemRatelimiter(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(GinMemRatelimiter(GinRatelimiterConfig{
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
