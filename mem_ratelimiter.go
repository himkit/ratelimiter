package ratelimiter

import (
	"context"
	"hash/fnv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	// memShardCount is the number of shards for the in-memory limiter.
	// It should be a power of two so the shard index can be computed with a bitmask.
	memShardCount = 256
	memShardMask  = memShardCount - 1
)

var (
	// MemRatelimiterCacheExpiration is the TTL for each cached limiter entry.
	MemRatelimiterCacheExpiration = time.Minute * 60
	// MemRatelimiterCacheCleanInterval is the interval between sweeps of expired entries.
	MemRatelimiterCacheCleanInterval = time.Minute * 60
)

type limiterEntry struct {
	limiter *rate.Limiter
	expire  int64 // nanoseconds since Unix epoch
}

type memShard struct {
	mu   sync.RWMutex
	data map[string]*limiterEntry
}

// MemRatelimiter is a process-local token-bucket rate limiter.
type MemRatelimiter struct {
	shards        [memShardCount]*memShard
	ttl           time.Duration
	sweepInterval time.Duration
	once          sync.Once
	stop          chan struct{}
}

// NewMemRatelimiter creates a new process-local rate limiter.
func NewMemRatelimiter() *MemRatelimiter {
	r := &MemRatelimiter{
		ttl:           MemRatelimiterCacheExpiration,
		sweepInterval: MemRatelimiterCacheCleanInterval,
		stop:          make(chan struct{}),
	}
	for i := range memShardCount {
		r.shards[i] = &memShard{data: make(map[string]*limiterEntry)}
	}
	return r
}

func (r *MemRatelimiter) startCleaner() {
	r.once.Do(func() {
		go r.cleanLoop()
	})
}

func (r *MemRatelimiter) cleanLoop() {
	ticker := time.NewTicker(r.sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			r.sweep()
		}
	}
}

func (r *MemRatelimiter) sweep() {
	now := time.Now().UnixNano()
	for i := range memShardCount {
		shard := r.shards[i]
		shard.mu.Lock()
		for key, entry := range shard.data {
			if now > entry.expire {
				delete(shard.data, key)
			}
		}
		shard.mu.Unlock()
	}
}

// Stop halts the background cleaner goroutine.
func (r *MemRatelimiter) Stop() {
	close(r.stop)
}

func shardIndex(key string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return h.Sum32() & memShardMask
}

// Allow reports whether the request for the given key is allowed under the
// token-bucket configuration. It returns false when tokenFillInterval or
// bucketSize are non-positive.
func (r *MemRatelimiter) Allow(ctx context.Context, key string, tokenFillInterval time.Duration, bucketSize int) bool {
	if tokenFillInterval <= 0 || bucketSize <= 0 {
		return false
	}

	r.startCleaner()

	tokenRate := rate.Every(tokenFillInterval)
	idx := shardIndex(key)
	shard := r.shards[idx]
	now := time.Now()
	expire := now.Add(r.ttl).UnixNano()

	shard.mu.Lock()
	defer shard.mu.Unlock()

	entry, exists := shard.data[key]
	if !exists || now.UnixNano() > entry.expire {
		limiter := rate.NewLimiter(tokenRate, bucketSize)
		limiter.Allow()
		shard.data[key] = &limiterEntry{limiter: limiter, expire: expire}
		return true
	}

	// Existing limiter is still valid; just try to consume a token.
	return entry.limiter.Allow()
}
