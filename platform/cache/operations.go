package cache

import (
	"fmt"

	"github.com/gomodule/redigo/redis"
)

func Get(key string, conn *redis.Conn) (string, error) {
	data, err := redis.String((*conn).Do("GET", key))
	if err != nil {
		fmt.Println("Error")
		fmt.Println(err)
		return "", err
	}
	return data, nil
}

func Del(key string, conn *redis.Conn) error {
	_, err := (*conn).Do("DEL", key)
	return err
}

func Set(key string, value interface{}, conn *redis.Conn) bool {
	reply, err := redis.String((*conn).Do("SET", key, value))
	if reply != "OK" || err != nil {
		fmt.Println(err)
		fmt.Println(reply)
		return false
	}
	return true
}

func HSET(key string, field string, value interface{}, conn *redis.Conn) error {
	_, err := (*conn).Do("HSET", key, field, value)
	if err != nil {
		return err
	}
	return nil
}

func HGET(key string, field string, conn *redis.Conn) (string, error) {
	res, err := redis.String((*conn).Do("HGET", key, field))
	if err != nil {
		return "", err
	}
	return res, nil
}

func HINCRBY(key string, field string, n int, conn *redis.Conn) (int, error) {
	res, err := redis.Int((*conn).Do("HINCRBY", key, field, n))
	if err != nil {
		return -1, err
	}
	return res, nil
}

func RPUSH(key string, values []interface{}, conn *redis.Conn) bool {
	_, err := (*conn).Do("RPUSH", redis.Args{}.Add(key).AddFlat(values)...)
	if err != nil {
		panic(err)
	}
	return true
}

func LLEN(key string, conn *redis.Conn) (int, error) {
	num, err := redis.Int((*conn).Do("LLEN", key))
	if err != nil {
		return -1, err
	}
	return num, nil
}

func LGET(key string, conn *redis.Conn) ([]interface{}, error) {
	val, _ := LLEN(key, conn)

	values, err := redis.Values((*conn).Do("LRANGE", key, 0, val))
	if err != nil {
		return make([]interface{}, 0), err
	}
	return values, nil

}

func LINDEX(key string, idx int, conn *redis.Conn) (interface{}, error) {
	val, err := redis.String((*conn).Do("LINDEX", key, idx))
	return val, err
}

func LREM(key string, val string, conn *redis.Conn) error {
	_, err := (*conn).Do("LREM", key, 0, val)
	return err
}
