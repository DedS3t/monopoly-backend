package queries

import (
	"fmt"
	"math/rand"
	"strconv"

	"github.com/DedS3t/monopoly-backend/app/models"
	"github.com/DedS3t/monopoly-backend/platform/board"
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

func CanAfford(game_id string, user_id string, cost int, conn *redis.Conn) (bool, int) {
	val, err := cache.HGET(fmt.Sprintf("%s.%s", game_id, user_id), "bal", conn)
	if err != nil {
		return false, 0
	}
	bal, _ := strconv.Atoi(val)
	if bal < cost {
		return false, 0
	}
	return true, bal
}

func CheckWhoOwns(game_id string, card_pos int, conn *redis.Conn) string { // O(N) time complex
	res, err := cache.LGET(fmt.Sprintf("%s.order", game_id), conn)
	if err != nil {
		panic(err)
	}
	for _, id := range res {
		// check if contains card
		_, err := cache.HGET(fmt.Sprintf("%s.%s.cards", game_id, string(id.([]byte))), strconv.Itoa(card_pos), conn)
		if err == nil {
			return string(id.([]byte))
		}
	}
	return ""
}

func GetSpecial(card_pos int, Board *map[string]models.Property) models.Special { // parses special card
	val, exist := (*Board)[strconv.Itoa(card_pos)]
	if !exist || card_pos == 0 {
		return models.Special{
			Info: "",
		}
	}

	if val.Action == "chest" {
		chests := board.LoadSpecial()["chest"]
		return chests[rand.Intn(len(chests))]
		// handle community chest

	} else if val.Action == "chance" {
		chances := board.LoadSpecial()["chance"]
		return chances[rand.Intn(len(chances))]
		// handle chance
	} else if val.Action == "jail" {
		// handle jail
		return models.Special{
			Info:    "Jail",
			Action:  "Jail",
			Payload: 0,
		}
	} else if res, err := strconv.Atoi(val.Action); err == nil {
		// if is money change
		return models.Special{
			Info:    val.Name,
			Action:  "change",
			Payload: res,
		}
	}
	return models.Special{
		Info: "",
	}
}
