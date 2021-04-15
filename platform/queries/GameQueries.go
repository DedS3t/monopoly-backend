package queries

import (
	"encoding/json"
	"fmt"

	"github.com/DedS3t/monopoly-backend/app/models"
	"github.com/DedS3t/monopoly-backend/platform/database"
	"github.com/go-pg/pg/v10"
	"github.com/gomodule/redigo/redis"
)

func VerifyGame(id string, db *pg.DB) bool {
	game := &models.Game{Id: id}
	err := db.Model(game).WherePK().Select()
	if err != nil {
		return false
	} else {
		return true
	}
}

// TODO check conccurency

func CreatePlayer(player models.Player, db *pg.DB) error {
	_, err := db.Model(&player).Insert()
	return err
}

func DeletePlayer(user_id string, game string, db *pg.DB) error {

	player := new(models.Player)
	_, err := db.Model(player).Where("user_id = ? and game_id = ?", user_id, game).Delete()
	CheckDB(game, db)
	return err
}

func CheckDB(game_id string, db *pg.DB) {
	var players []models.Player
	err := db.Model(&players).Where("game_id = ?", game_id).Select()
	if err != nil || len(players) == 0 {
		// means there are 0 rows returned

		game := new(models.Game)
		_, err = db.Model(game).Where("id = ?", game_id).Delete()
	}
}

func createRedisValue(key string, value interface{}, conn *redis.Conn) bool {
	reply, err := redis.String((*conn).Do("SET", key, value))
	if reply != "OK" || err != nil {
		fmt.Println(err)
		fmt.Println(reply)
		return false
	}
	return true
}

func createRedisPlayer(game_id string, player models.Player, conn *redis.Conn) {
	var cards []models.Card
	str, err := json.Marshal(cards)
	if err != nil {
		panic(err)
	}
	createRedisValue(fmt.Sprintf("%s.%s", player.Game_id, player.User_id), str, conn)   // set to empty array
	createRedisValue(fmt.Sprintf("%s.%s.pos", player.Game_id, player.User_id), 0, conn) // set initial pos to 0
	createRedisValue(fmt.Sprintf("%s.%s.bal", player.Game_id, player.User_id), 0, conn) // set intiial bal to 0
}

func StartGame(game_id string, conn *redis.Conn) bool {
	db := database.PostgreSQLConnection()
	var players []models.Player

	err := db.Model(&players).Where("game_id = ?", game_id).Select()

	if err != nil || len(players) <= 1 {
		return false
	}

	createRedisValue(game_id, players[0].User_id, conn)

	for _, player := range players {
		createRedisPlayer(game_id, player, conn)
	}

	game := &models.Game{
		Id: game_id,
	}
	_, err = db.Model(game).WherePK().Set("status = ?", "in progress").Update()
	if err != nil {
		panic(err)
	}

	return true
}
