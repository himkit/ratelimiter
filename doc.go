// Package ratelimiter provides token-bucket rate limiters for Go.
//
// Two limiter implementations are included:
//
//   - [MemRatelimiter]: process-local token bucket built on golang.org/x/time/rate
//     and a sharded in-memory TTL map.
//   - [RedisRatelimiter]: distributed token bucket backed by Redis and a Lua script.
//
// For Gin applications, use [GinMemRatelimiter] or [GinRedisRatelimiter].
// Configure them with [GinRatelimiterConfig].
//
// For examples, benchmarks, installation instructions, and the algorithm
// description, see the README at
// https://github.com/himkit/ratelimiter/blob/main/README.md.
package ratelimiter
