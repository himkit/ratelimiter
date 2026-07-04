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
	limitKey := "uniq_limit_key"
	tokenFillInterval := time.Second * 1
	bucketSize := 1
	for i := 0; i < 3; i++ {
		// 1st and 3nd is allowed
		if i == 2 {
			time.Sleep(time.Second * 1)
		}
		isAllow := limiter.Allow(context.TODO(), limitKey, tokenFillInterval, bucketSize)
		fmt.Println(i, time.Now(), isAllow)
	}
}
