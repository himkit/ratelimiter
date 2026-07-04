package ratelimiter

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var (
	// RedisRatelimiterCacheExpiration redis ratelimiter 缓存过期时间
	RedisRatelimiterCacheExpiration = time.Minute * 60
)

// RedisRatelimiter redis limiter
type RedisRatelimiter struct {
	*redis.Client
	script *redis.Script
}

// NewRedisRatelimiter 根据配置创建 redis limiter
func NewRedisRatelimiter(rdb *redis.Client) *RedisRatelimiter {
	return &RedisRatelimiter{
		Client: rdb,
		script: tokenBucketRedisLuaIsLimitedScript,
	}
}

// Allow 判断给定 key 是否被允许
func (r *RedisRatelimiter) Allow(ctx context.Context, key string, tokenFillInterval time.Duration, bucketSize int) bool {
	// 参数小于等于 0 时直接限制
	if tokenFillInterval <= 0 || bucketSize <= 0 {
		return false
	}

	// 构造 lua 脚本参数
	keys := []string{key}
	args := []interface{}{
		bucketSize,
		1, // lua 脚本支持调整每次放入 token 的个数，这里全部统一使用每次放一个 token
		tokenFillInterval.Microseconds(),
		RedisRatelimiterCacheExpiration.Seconds(),
	}
	// 在 redis 中执行 lua 脚本计算当前 key 是否被限频
	// Run 会自动使用 evalsha 优化带宽
	limited, err := r.script.Run(ctx, r.Client, keys, args...).Int64()
	if err != nil {
		// 有 err 默认放行
		zap.L().Error("RedisRatelimiter run script error", zap.Error(err))
		return true
	}
	return limited == 0
}
