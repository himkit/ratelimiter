# ratelimiter

[![Build Status](https://github.com/himkit/ratelimiter/actions/workflows/go.yml/badge.svg)](https://github.com/himkit/ratelimiter/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/himkit/ratelimiter)](https://goreportcard.com/report/github.com/himkit/ratelimiter)
[![Go Reference](https://pkg.go.dev/badge/github.com/himkit/ratelimiter.svg)](https://pkg.go.dev/github.com/himkit/ratelimiter)

A token-bucket rate limiter for Go. See the [Go package documentation](https://pkg.go.dev/github.com/himkit/ratelimiter) for the API reference. This repository is a **fork and revamp** of the original [`github.com/axiaoxin-com/ratelimiter`](https://github.com/axiaoxin-com/ratelimiter). The codebase has been modernized, hardened, and optimized while keeping the public API familiar.

## What changed in this revamp

- **Go toolchain:** bumped to Go 1.25.
- **Dependencies:** upgraded to current stable versions:
  - `github.com/gin-gonic/gin` v1.12.0
  - `github.com/redis/go-redis/v9` v9.21.0 (migrated from the `v8` beta)
  - `go.uber.org/zap` v1.27.0
  - `golang.org/x/time` latest
  - `github.com/stretchr/testify` v1.11.1
- **Removed custom/internal dependencies:** `github.com/axiaoxin-com/logging`, `github.com/axiaoxin-com/goutils`, `github.com/patrickmn/go-cache`, and `github.com/json-iterator/go` are no longer used.
- **Performance:**
  - `MemRatelimiter` now uses a sharded in-memory TTL map instead of `go-cache`, removing the single global lock and redundant `Set` calls on every request.
  - `RedisRatelimiter` now uses a simpler Lua script that returns only `1` or `0`, removing JSON encoding/parsing overhead.
- **Tests:** rewritten to use the standard `net/http/httptest` package and `redis.NewClient`; Redis tests gracefully skip when no Redis server is available.
- **CI:** migrated from Travis CI to GitHub Actions (Go 1.23/1.24/1.25 with Redis service).

## Benchmarks

Run with `go test -bench=. -run=^$ -benchmem .` on an Intel Core i9-9880H.

```
BenchmarkMemRatelimiterAllow-16             4714309     248.4 ns/op     0 B/op     0 allocs/op
BenchmarkMemRatelimiterAllowParallel-16     3931524     300.3 ns/op     0 B/op     0 allocs/op
BenchmarkGinMemRatelimiter-16                392778    3222   ns/op  5445 B/op    18 allocs/op
BenchmarkRedisRatelimiterAllow-16              3483  331190   ns/op   440 B/op    13 allocs/op
```

Compared with the pre-revamp implementation:

| Limit store | Before | After |
|---|---|---|
| Memory | Global lock + `go-cache` map + extra `Set` per request | 256-way sharded map, no cache library, **0 allocations** on the hot path |
| Redis | Lua script returned a JSON object parsed with `jsoniter` | Lua script returns an integer, parsed directly by go-redis |

## Installation

Because this is a fork, the module path has been changed to match the current remote. Install with:

```bash
go get github.com/himkit/ratelimiter
```

If your project still imports the original path (`github.com/axiaoxin-com/ratelimiter`), add a `replace` directive in your `go.mod`:

```go
replace github.com/axiaoxin-com/ratelimiter => github.com/himkit/ratelimiter v0.0.0
```

Then run:

```bash
go mod tidy
```

## Components

- [lua-ngx-ratelimiter](./lua-ngx-ratelimiter): an OpenResty/nginx + Lua + Redis token-bucket limiter (unchanged from upstream).
- [MemRatelimiter](./mem_ratelimiter.go): process-local token-bucket limiter built on `golang.org/x/time/rate` + a sharded TTL map.
- [RedisRatelimiter](./redis_ratelimiter.go): distributed token-bucket limiter backed by Redis + Lua.
- [GinMemRatelimiter](./gin_mem_ratelimiter.go): Gin middleware wrapping `MemRatelimiter`.
- [GinRedisRatelimiter](./gin_redis_ratelimiter.go): Gin middleware wrapping `RedisRatelimiter`.

## Usage

### Gin middleware (memory)

See [example/gin_mem_ratelimiter.go](./example/gin_mem_ratelimiter.go).

```go
package main

import (
	"time"

	"github.com/himkit/ratelimiter"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.New()

	// Add one token every second; bucket size is 1.
	r.Use(ratelimiter.GinMemRatelimiter(ratelimiter.GinRatelimiterConfig{
		LimitKey: func(c *gin.Context) string {
			return c.ClientIP()
		},
		LimitedHandler: func(c *gin.Context) {
			c.JSON(200, "too many requests!!!")
			c.Abort()
		},
		TokenBucketConfig: func(*gin.Context) (time.Duration, int) {
			return time.Second, 1
		},
	}))

	r.GET("/", func(c *gin.Context) {
		c.JSON(200, "hi")
	})
	r.Run()
}
```

### Gin middleware (Redis)

See [example/gin_redis_ratelimiter.go](./example/gin_redis_ratelimiter.go).

```go
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

	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		panic(err)
	}

	r.Use(ratelimiter.GinRedisRatelimiter(rdb, ratelimiter.GinRatelimiterConfig{
		LimitKey: func(c *gin.Context) string {
			return c.ClientIP()
		},
		LimitedHandler: func(c *gin.Context) {
			c.JSON(200, "too many requests!!!")
			c.Abort()
		},
		TokenBucketConfig: func(*gin.Context) (time.Duration, int) {
			return time.Second, 1
		},
	}))

	r.GET("/", func(c *gin.Context) {
		c.JSON(200, "hi")
	})
	r.Run()
}
```

### Direct use (memory)

See [example/mem_ratelimiter.go](./example/mem_ratelimiter.go).

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/himkit/ratelimiter"
)

func main() {
	limiter := ratelimiter.NewMemRatelimiter()
	defer limiter.Stop()

	key := "uniq_limit_key"
	fillInterval := time.Second
	bucketSize := 1

	for i := 0; i < 3; i++ {
		if i == 2 {
			time.Sleep(time.Second)
		}
		fmt.Println(i, time.Now(), limiter.Allow(context.TODO(), key, fillInterval, bucketSize))
	}
}
```

### Direct use (Redis)

See [example/redis_ratelimiter.go](./example/redis_ratelimiter.go).

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/himkit/ratelimiter"
	"github.com/redis/go-redis/v9"
)

func main() {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		panic(err)
	}

	limiter := ratelimiter.NewRedisRatelimiter(rdb)
	key := "uniq_limit_key"
	fillInterval := time.Second
	bucketSize := 1

	for i := 0; i < 3; i++ {
		if i == 2 {
			time.Sleep(time.Second)
		}
		fmt.Println(i, time.Now(), limiter.Allow(context.TODO(), key, fillInterval, bucketSize))
	}
}
```

## Testing

Redis-backed tests require a Redis server on `localhost:6379`. If Redis is not available, only the memory tests will run.

```bash
# Run everything (Redis must be available)
go test -race -v .

# Run only memory tests
go test -race -v -run 'TestMem|TestGinMem' .
```

> Do **not** use `go test ./...` — the `example/` directory contains multiple `package main` files and will fail to build as a single package.

## Token bucket algorithm

The token bucket algorithm works as follows:

- Tokens are added to a bucket at a fixed rate.
- The bucket has a maximum capacity; excess tokens are discarded.
- Each incoming request consumes one token.
- If no token is available, the request is rejected.

This implementation calculates the number of tokens produced since the last request instead of running a background goroutine, which keeps the hot path fast and allocation-free for the memory limiter.

## Redis + Lua notes

The Redis limiter executes a Lua script atomically with `EVAL`/`EVALSHA`.

Why Lua instead of separate Redis commands?

- **Atomic read-update-write:** Redis runs Lua scripts sequentially and uninterrupted, so concurrent requests cannot race between reading the remaining tokens and writing the updated count.
- **Consistent clock:** The script reads `TIME` on the Redis server, so all application instances share the same clock instead of relying on potentially skewed local clocks.
- **Fewer roundtrips:** A single `EVALSHA` call replaces multiple commands (`EXISTS`, `TIME`, `HGET`, `HSET`, `EXPIRE`).

Because the script calls the non-deterministic `TIME` command, it starts with `redis.replicate_commands()` to satisfy Redis replication requirements.

## License

See [LICENSE](./LICENSE).
