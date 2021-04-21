package queries

import (
	"fmt"

	"github.com/DedS3t/monopoly-backend/platform/cache"
	"github.com/gomodule/redigo/redis"
)

func IsUserTurn(game_id string, user_id string, conn *redis.Conn) bool {
	val, err := cache.Get(game_id, conn)
	if err != nil {
		return false
	}
	return val == user_id
}

func HasRolledDice(game_id string, user_id string, conn *redis.Conn) bool {
	val, err := cache.HGET(fmt.Sprintf("%s.%s", game_id, user_id), "hasRolled", conn)
	if err != nil {
		panic(err) // TODO change to return
	}
	return val == "true"
}

func ResetRolledDice(game_id string, user_id string, conn *redis.Conn) bool {
	err := cache.HSET(fmt.Sprintf("%s.%s", game_id, user_id), "hasRolled", "false", conn)
	if err != nil {
		panic(err)
	}
	return true
}

func CheckWhoOwns(game_id string, card_id string, conn *redis.Conn) string {
	res, err := cache.LGET(fmt.Sprintf("%s.order", game_id), conn)
	if err != nil {
		panic(err)
	}
	for _, id := range res {
		// check if contains card
		_, err := cache.HGET(fmt.Sprintf("%s.%s.cards", game_id, string(id.([]byte))), card_id, conn)
		if err == nil {
			return string(id.([]byte))
		}
	}
	return ""
}
