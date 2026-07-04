package ratelimiter

import "github.com/redis/go-redis/v9"

// tokenBucketRedisLuaIsLimitedScript returns 1 if the request should be limited,
// or 0 if it is allowed.
var tokenBucketRedisLuaIsLimitedScript = redis.NewScript(`
    redis.replicate_commands()

    local key = KEYS[1]
    local capacity = tonumber(ARGV[1])
    local fill_count = tonumber(ARGV[2])
    local interval_micros = tonumber(ARGV[3])
    local expire_seconds = tonumber(ARGV[4])

    local t = redis.call("TIME")
    local now = tonumber(t[1]) * 1000000 + tonumber(t[2])

    if fill_count <= 0 or capacity <= 0 then
        return 1
    end

    if redis.call("EXISTS", key) == 0 then
        redis.call("HSET", key, "last", now, "remain", capacity - 1)
        redis.call("EXPIRE", key, expire_seconds)
        return 0
    end

    local data = redis.call("HMGET", key, "last", "remain")
    if data[1] == false or data[2] == false then
        return 0
    end

    local last = tonumber(data[1])
    local remain = tonumber(data[2])
    local filled = math.floor(math.max(now - last, 0) / interval_micros) * fill_count
    local count = math.min(remain + filled, capacity)

    if count <= 0 then
        redis.call("HSET", key, "last", last, "remain", 0)
        redis.call("EXPIRE", key, expire_seconds)
        return 1
    end

    redis.call("HSET", key, "last", now, "remain", count - 1)
    redis.call("EXPIRE", key, expire_seconds)
    return 0
`)
