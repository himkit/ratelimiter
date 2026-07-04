# Spec: ratelimiter vNext Upgrade + Performance Optimization

> Date: 2026-07-04

## Goal
Modernize the `github.com/himkit/ratelimiter` library to current Go ecosystem versions, remove custom/internal dependencies, and improve request hot-path performance without redesigning the public API beyond the unavoidable Redis client type change.

## Target versions

| Component | From | To |
|---|---|---|
| Go | 1.14 | 1.24 |
| gin | v1.9.0 | v1.12.0 |
| redis client | `github.com/go-redis/redis/v8` v8.0.0-beta.10 | `github.com/redis/go-redis/v9` v9.21.0 |
| logging | `github.com/axiaoxin-com/logging` v1.2.3 | `go.uber.org/zap` v1.27.0 |
| json parsing | `github.com/json-iterator/go` v1.1.12 | removed |
| in-memory cache | `github.com/patrickmn/go-cache` v2.1.0+incompatible | removed (custom sharded map) |
| test assertions | `github.com/stretchr/testify` v1.8.1 | v1.11.0 |
| rate limiter core | `golang.org/x/time` v0.0.0-... | latest stable (v0.11.0) |
| helper utilities | `github.com/axiaoxin-com/goutils` v0.0.0-... | removed |

## Public API

- `NewMemRatelimiter() *MemRatelimiter` — unchanged signature.
- `(*MemRatelimiter) Allow(ctx, key, interval, size) bool` — unchanged signature.
- `NewRedisRatelimiter(rdb *redis.Client) *RedisRatelimiter` — `*redis.Client` now from `github.com/redis/go-redis/v9`.
- `(*RedisRatelimiter) Allow(ctx, key, interval, size) bool` — unchanged signature.
- `GinMemRatelimiter(conf GinRatelimiterConfig) gin.HandlerFunc` — unchanged.
- `GinRedisRatelimiter(rdb *redis.Client, conf GinRatelimiterConfig) gin.HandlerFunc` — `*redis.Client` from v9.
- `GinRatelimiterConfig`, `DefaultGinLimitKey`, `DefaultGinLimitedHandler` — unchanged.

## Architecture changes

### MemRatelimiter

- Remove `go-cache` dependency.
- Implement a sharded TTL map (256 shards) where each shard holds:
  - `map[string]*limiterEntry`
  - `sync.RWMutex`
  - `*rate.Limiter`
  - expiry timestamp
- `Allow` logic:
  - Select shard by hash of key.
  - Lock shard.
  - Lookup entry; if missing, create `rate.NewLimiter(rate.Every(interval), size)`, call `Allow()`, store with expiry.
  - If exists and not expired, call `Allow()` and return.
  - If expired, recreate.
  - Do not update expiry on every access to avoid write amplification; keep TTL long enough since stale limiters will be cleaned by background sweeper.
- Cleanup: start one background goroutine per `MemRatelimiter` instance (lazy start) that sweeps expired entries every cleanup interval. Provide `Stop()` to stop sweeper.

### RedisRatelimiter

- Rewrite Lua script in `redis_lua.go` to return only integer `1` (limited) or `0` (allowed). Remove all debug fields and `cjson` usage.
- `Allow` Go code:
  - Validate parameters.
  - Run script with `bucketSize, 1, intervalMicros, expireSeconds`.
  - On error, log with `zap.L().Error(...)` and return `true` (fail-open).
  - Parse result as int64.
  - Return `result == 0`.
- Remove `github.com/json-iterator/go` and `go.uber.org/zap` field logging of full JSON result.

### Logging

- All log calls use `zap.L().Error(...)`.
- No logger initialization required; zap default no-op logger is safe if caller does not configure it.

### Tests

- Replace `goutils.RequestHTTPHandler` with `httptest.NewRecorder()` + `r.ServeHTTP(...)`.
- Replace `goutils.NewRedisClient` with `redis.NewClient(&redis.Options{Addr: "localhost:6379"})` + `client.Ping(ctx).Err()`.
- Add benchmarks:
  - `BenchmarkMemRatelimiterAllow`
  - `BenchmarkMemRatelimiterAllowParallel`
  - `BenchmarkGinMemRatelimiter`
  - `BenchmarkRedisRatelimiterAllow` (skip if Redis unavailable)
- Existing tests remain the behavioral contract.

### CI

- Replace `.travis.yml` with `.github/workflows/go.yml`:
  - Trigger: push, pull_request.
  - Go versions: 1.22, 1.23, 1.24.
  - Services: Redis on localhost:6379.
  - Steps: checkout, setup-go, `go mod tidy`, `go test -race -v .`.

## Non-goals

- No v2 module path bump.
- No new generic `samber/lo` dependency unless a concrete need appears during implementation.
- No redesign of `GinRatelimiterConfig` or default limit key format.
- No behavioral change to fail-open semantics.
