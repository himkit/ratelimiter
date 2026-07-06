# Agent Notes

## Verification
- Run tests: `go test -race -v .`
  - Do **not** use `./...`; `example/` contains multiple `package main` files in one directory and will fail to build.
  - Redis-backed tests (`TestRedisRatelimiter`, `TestGinRedisRatelimiter`) require Redis on `localhost:6379` and skip gracefully if unavailable.
  - If Redis is unavailable, run memory-only tests: `go test -race -v -run 'TestMem|TestGinMem' .`
  - CI source of truth: `.github/workflows/go.yml` tests Go 1.23/1.24/1.25 with a Redis service.

## Runtime
- This repo requires **Go 1.25** because `github.com/gin-gonic/gin v1.12.0` requires `go >= 1.25.0`.
- The active Go toolchain is at `/Users/phamdat/.gvm/gos/go1.25.11/bin/go`. When running commands directly, prepend that to `PATH` and set `GOROOT` if wrappers default to an older Go version.

## Project Layout
- Single Go package library at repo root: `github.com/himkit/ratelimiter`.
- `mem_ratelimiter.go` — process-local token bucket using `golang.org/x/time/rate` + a sharded in-memory TTL map.
- `redis_ratelimiter.go` + `redis_lua.go` — distributed token bucket backed by a Redis Lua script.
- `gin_mem_ratelimiter.go` / `gin_redis_ratelimiter.go` — Gin middleware wrappers.
- `config_gin.go` — shared Gin config, default limit key prefix `pink-lady:ratelimiter:<ip>:<path>`, and default 429 handler.
- `example/` — runnable `package main` examples. Run one file at a time, e.g. `go run example/mem_ratelimiter.go`.

## Dependencies
- Removed from the library: `github.com/axiaoxin-com/logging`, `github.com/axiaoxin-com/goutils`, `github.com/patrickmn/go-cache`, `github.com/json-iterator/go`.
- `go.uber.org/zap` is used directly for error logging.
- Redis client is `github.com/redis/go-redis/v9`.

## Implementation Gotchas
- `GinRatelimiterConfig.TokenBucketConfig` is required; the middleware panics if it is nil.
- `MemRatelimiter` starts a lazy background sweeper goroutine; call `Stop()` when the instance is no longer needed to avoid leaking goroutines in tests.
- `RedisRatelimiter.Allow` returns `true` on Redis/Lua errors (fail-open).
- The Redis Lua script uses `TIME` and `redis.replicate_commands()`; it requires Redis with Lua support (3.2+).
- Redis tests connect to `localhost:6379` via `redis.NewClient(&redis.Options{Addr: "localhost:6379"})` and skip if Redis is unavailable.
