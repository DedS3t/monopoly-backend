package cache

import (
	"time"

	"github.com/gomodule/redigo/redis"
)

func CreateRedisPool() *redis.Pool {
	return &redis.Pool{
		MaxIdle:     10,
		IdleTimeout: 60 * time.Second,
		Dial:        func() (redis.Conn, error) { return redis.Dial("tcp", "localhost:6379") },
	}
}

func CreateRedisConnection() (redis.Conn, error) {
	return redis.Dial("tcp", "localhost:6379")
}
