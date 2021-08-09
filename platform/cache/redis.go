package cache

import (
	"os"
	"time"

	"github.com/gomodule/redigo/redis"
)

func CreateRedisPool() *redis.Pool {
	return &redis.Pool{
		MaxIdle:     10,
		IdleTimeout: 60 * time.Second,
		Dial:        func() (redis.Conn, error) { return redis.Dial("tcp", os.Getenv("REDIS_URL")) },
	}
}

func CreateRedisConnection() (redis.Conn, error) {
	return redis.Dial("tcp", os.Getenv("REDIS_URL"))
}
