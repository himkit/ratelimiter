# ratelimiter vNext Upgrade + Performance Optimization

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the spec in `docs/superpowers/specs/2026-07-04-ratelimiter-upgrade-design.md`.

**Architecture:** Upgrade Go toolchain and dependencies, replace custom/internal deps with standard/popular equivalents, rewrite the memory limiter with a sharded TTL map, simplify the Redis Lua script, and add benchmarks.

**Tech stack:** Go 1.24, gin v1.12.0, `github.com/redis/go-redis/v9`, zap, testify, `golang.org/x/time`.

## Global constraints

- Go module version changes from `github.com/axiaoxin-com/ratelimiter` to `github.com/himkit/ratelimiter` (no `/v2`).
- Target Go directive: `go 1.24`.
- All existing tests must pass after changes; benchmarks are additive.
- Redis-backed tests require `localhost:6379`.
- Fail-open behavior for memory type assertion and Redis errors must be preserved.
- No `go test ./...`; root package only because `example/` contains multiple `main` files.

---

### Task 1: Upgrade Go directive and dependencies

**Files:**
- Modify: `go.mod`
- Test: `go mod tidy` + `go test -race -v .`

**Interfaces:**
- Produces: updated `go.mod` / `go.sum` with new dependency versions.

- [ ] **Step 1: Edit `go.mod`**
  Set:
  ```go
  module github.com/himkit/ratelimiter

  go 1.24

  require (
      github.com/gin-gonic/gin v1.12.0
      github.com/redis/go-redis/v9 v9.21.0
      github.com/stretchr/testify v1.11.0
      go.uber.org/zap v1.27.0
      golang.org/x/time v0.11.0
  )
  ```

- [ ] **Step 2: Run `go mod tidy`**
  Command: `go mod tidy`
  Expected: `go.mod` and `go.sum` refreshed, no build errors.

- [ ] **Step 3: Commit**
  ```bash
  git add go.mod go.sum
  git commit -m "chore(deps): upgrade Go to 1.24 and dependencies to latest"
  ```

---

### Task 2: Replace custom logging with zap in Redis limiter

**Files:**
- Modify: `redis_ratelimiter.go`
- Test: `redis_ratelimiter_test.go`

**Interfaces:**
- Consumes: `*redis.Client` from `github.com/redis/go-redis/v9`.
- Produces: `NewRedisRatelimiter(*redis.Client) *RedisRatelimiter`.

- [ ] **Step 1: Update imports**
  Replace `github.com/axiaoxin-com/logging` with `go.uber.org/zap`.
  Replace `github.com/go-redis/redis/v8` with `github.com/redis/go-redis/v9`.

- [ ] **Step 2: Update `Allow` error logging**
  ```go
  zap.L().Error("ratelimiter: redis script failed", zap.Error(err))
  ```

- [ ] **Step 3: Run Redis tests**
  Command: `go test -race -v -run TestRedisRatelimiter .`
  Expected: pass.

---

### Task 3: Simplify Redis Lua script

**Files:**
- Modify: `redis_lua.go`

**Interfaces:**
- Produces: `tokenBucketRedisLuaIsLimitedScript` returning integer `1` (limited) or `0` (allowed).

- [ ] **Step 1: Rewrite script**
  ```go
  var tokenBucketRedisLuaIsLimitedScript = redis.NewScript(`
      redis.replicate_commands()
      local key = KEYS[1]
      local capacity = tonumber(ARGV[1])
      local fill_count = tonumber(ARGV[2])
      local interval_micros = tonumber(ARGV[3])
      local expire_seconds = tonumber(ARGV[4])

      if fill_count <= 0 or capacity <= 0 then
          return 1
      end

      if redis.call("EXISTS", key) == 0 then
          local t = redis.call("TIME")
          local now = tonumber(t[1]) * 1000000 + tonumber(t[2])
          redis.call("HMSET", key, "last", now, "remain", capacity - 1)
          redis.call("EXPIRE", key, expire_seconds)
          return 0
      end

      local data = redis.call("HMGET", key, "last", "remain")
      if data == nil then
          return 0
      end

      local last = tonumber(data[1])
      local remain = tonumber(data[2])
      local t = redis.call("TIME")
      local now = tonumber(t[1]) * 1000000 + tonumber(t[2])
      local fill_rate = fill_count / interval_micros
      local filled = math.floor(math.max(now - last, 0) * fill_rate)
      local count = math.min(remain + filled, capacity)

      if count <= 0 then
          redis.call("HMSET", key, "last", last, "remain", 0)
          redis.call("EXPIRE", key, expire_seconds)
          return 1
      end

      redis.call("HMSET", key, "last", now, "remain", count - 1)
      redis.call("EXPIRE", key, expire_seconds)
      return 0
  `)
  ```

- [ ] **Step 2: Update `redis_ratelimiter.go` result parsing**
  ```go
  limited, err := r.script.Run(ctx, r.Client, keys, args...).Int64()
  if err != nil { ... return true }
  return limited == 0
  ```

- [ ] **Step 3: Run Redis tests**
  Command: `go test -race -v -run TestRedisRatelimiter .`
  Expected: pass.

---

### Task 4: Rewrite MemRatelimiter with sharded TTL map

**Files:**
- Modify: `mem_ratelimiter.go`
- Test: `mem_ratelimiter_test.go`, new benchmark file

**Interfaces:**
- Produces: `NewMemRatelimiter() *MemRatelimiter` with `Allow(ctx, key, interval, size) bool`.

- [ ] **Step 1: Implement sharded map**
  ```go
  const shardCount = 256

  type limiterEntry struct {
      limiter *rate.Limiter
      expire  int64 // unix nano
  }

  type memShard struct {
      mu   sync.RWMutex
      data map[string]*limiterEntry
  }

  type MemRatelimiter struct {
      shards [shardCount]*memShard
      ttl    time.Duration
      sweepInterval time.Duration
      once   sync.Once
      stop   chan struct{}
  }
  ```

- [ ] **Step 2: Implement Allow**
  - Hash key with `xxhash` or `fnv` to pick shard.
  - Lock shard.
  - Get or create `rate.Limiter`.
  - Call `Allow()`.
  - Update expiry only on creation to avoid write amplification.

- [ ] **Step 3: Add background sweeper**
  - `once.Do` start a goroutine that sweeps expired entries every sweep interval.
  - Provide `Stop()` method to stop sweeper; update tests to call it.

- [ ] **Step 4: Update tests**
  - Add `defer limiter.Stop()` where appropriate.
  - Ensure existing behavior unchanged.

- [ ] **Step 5: Run memory tests**
  Command: `go test -race -v -run TestMemRatelimiter .`
  Expected: pass.

---

### Task 5: Update Gin wrappers and tests

**Files:**
- Modify: `gin_mem_ratelimiter.go`, `gin_redis_ratelimiter.go`
- Test: `gin_mem_ratelimiter_test.go`, `gin_redis_ratelimiter_test.go`

**Interfaces:**
- `GinMemRatelimiter(GinRatelimiterConfig) gin.HandlerFunc`
- `GinRedisRatelimiter(*redis.Client, GinRatelimiterConfig) gin.HandlerFunc`

- [ ] **Step 1: Update redis import**
  Replace `github.com/go-redis/redis/v8` with `github.com/redis/go-redis/v9`.

- [ ] **Step 2: Replace goutils test helper**
  In tests:
  ```go
  w := httptest.NewRecorder()
  req := httptest.NewRequest("GET", "/", nil)
  r.ServeHTTP(w, req)
  assert.Equal(t, 200, w.Code)
  ```

- [ ] **Step 3: Run Gin tests**
  Commands:
  - `go test -race -v -run TestGinMemRatelimiter .`
  - `go test -race -v -run TestGinRedisRatelimiter .`
  Expected: pass.

---

### Task 6: Add benchmarks

**Files:**
- Create: `ratelimiter_bench_test.go`

- [ ] **Step 1: Add benchmarks**
  ```go
  func BenchmarkMemRatelimiterAllow(b *testing.B) { ... }
  func BenchmarkMemRatelimiterAllowParallel(b *testing.B) { ... }
  func BenchmarkGinMemRatelimiter(b *testing.B) { ... }
  func BenchmarkRedisRatelimiterAllow(b *testing.B) { ... } // skip if no redis
  ```

- [ ] **Step 2: Run benchmarks**
  Command: `go test -bench=. -run=^$ -benchmem .`
  Expected: benchmarks execute and report ns/op and allocs/op.

---

### Task 7: Replace Travis CI with GitHub Actions

**Files:**
- Delete: `.travis.yml`
- Create: `.github/workflows/go.yml`

- [ ] **Step 1: Create workflow**
  ```yaml
  name: Go
  on: [push, pull_request]
  jobs:
    test:
      runs-on: ubuntu-latest
      services:
        redis:
          image: redis:7
          ports:
            - 6379:6379
      strategy:
        matrix:
          go-version: ['1.22', '1.23', '1.24']
      steps:
        - uses: actions/checkout@v4
        - uses: actions/setup-go@v5
          with:
            go-version: ${{ matrix.go-version }}
        - run: go mod tidy
        - run: go test -race -v .
  ```

- [ ] **Step 2: Validate workflow syntax**
  Use `actionlint` if available, or inspect manually.

---

### Task 8: Update examples

**Files:**
- Modify: `example/gin_redis_ratelimiter.go`, `example/redis_ratelimiter.go`

- [ ] **Step 1: Replace redis import**
  Use `github.com/redis/go-redis/v9`.

- [ ] **Step 2: Replace goutils.NewRedisClient**
  Use `redis.NewClient(&redis.Options{Addr: "localhost:6379"})` + `Ping`.

- [ ] **Step 3: Verify examples build individually**
  Commands:
  - `go run example/mem_ratelimiter.go`
  - `go run example/gin_mem_ratelimiter.go`
  - `go run example/redis_ratelimiter.go`
  - `go run example/gin_redis_ratelimiter.go`
  Expected: each compiles successfully.

---

### Task 9: Final verification and documentation

**Files:**
- Modify: `AGENTS.md`
- Modify: `README.md` if dependency examples reference old versions

- [ ] **Step 1: Run full root test suite**
  Command: `go test -race -v .`
  Expected: all tests pass.

- [ ] **Step 2: Run memory-only suite**
  Command: `go test -race -v -run 'TestMem|TestGinMem' .`
  Expected: pass.

- [ ] **Step 3: Update AGENTS.md**
  Refresh dependency versions, Redis import path, and CI command.

- [ ] **Step 4: Commit final changes**
  ```bash
  git add .
  git commit -m "feat: upgrade deps and optimize ratelimiter performance"
  ```

---

## Plan completion check

- Spec coverage: all dependency upgrades, perf changes, tests, CI, examples covered.
- Placeholder scan: no TBD/TODO in plan steps.
- Type consistency: `*redis.Client` consistently from v9, zap logging consistent.
