package queries

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/DedS3t/monopoly-backend/app/models"
	"github.com/DedS3t/monopoly-backend/platform/board"
	"github.com/DedS3t/monopoly-backend/platform/cache"
	"github.com/DedS3t/monopoly-backend/platform/database"
	"github.com/go-pg/pg/v10"
	"github.com/gomodule/redigo/redis"
	socketio "github.com/googollee/go-socket.io"
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

func DeletePlayer(user_id string, game string, db *pg.DB, server *socketio.Server) error {
	// TODO add the leave system
	conn, _ := cache.CreateRedisConnection()

	player := new(models.Player)
	_, err1 := db.Model(player).Where("user_id = ? and game_id = ?", user_id, game).Delete()

	CheckDB(game, db)

	val, err := cache.Get(game, &conn)
	if err != nil || val == "" {
		return err1
	}
	if val == user_id {
		new_turn := GetNextTurn(game, user_id, &conn)
		server.BroadcastToRoom("/", game, "change-turn", new_turn)
	}
	cache.Del(fmt.Sprintf("%s.%s", game, user_id), &conn)
	cache.Del(fmt.Sprintf("%s.%s.cards", game, user_id), &conn)
	cache.LREM(fmt.Sprintf("%s.order", game), user_id, &conn)

	length, err := cache.LLEN(fmt.Sprintf("%s.order", game), &conn)
	if length <= 1 {
		cleanUp(game, db, &conn)
		server.BroadcastToRoom("/", game, "game-over")
	}

	return err
}

func CheckDB(game_id string, db *pg.DB) {
	var players []models.Player
	err := db.Model(&players).Where("game_id = ?", game_id).Select()
	if err != nil || len(players) == 0 {
		// means there are 0 rows returned

		game := new(models.Game)
		db.Model(game).Where("id = ?", game_id).Delete()
	}
}

func cleanUp(game_id string, db *pg.DB, conn *redis.Conn) {
	// db cleanup
	player := new(models.Player)
	game := new(models.Game)
	db.Model(player).Where("game_id = ?", game_id).Delete()
	db.Model(game).Where("id = ?", game_id).Delete()
	// redis cleanup
	res, _ := cache.LGET(fmt.Sprintf("%s.order", game_id), conn)
	for _, id := range res {
		cache.Del(fmt.Sprintf("%s.%s", game, string(id.([]byte))), conn)
		cache.Del(fmt.Sprintf("%s.%s.cards", game, string(id.([]byte))), conn)
	}
	cache.Del(game_id, conn)
	cache.Del(fmt.Sprintf("%s.order", game_id), conn)
}

func createRedisPlayer(game_id string, player models.Player, conn *redis.Conn) {
	cache.HSET(fmt.Sprintf("%s.%s", player.Game_id, player.User_id), "bal", 1500, conn)
	cache.HSET(fmt.Sprintf("%s.%s", player.Game_id, player.User_id), "pos", 0, conn)
	cache.HSET(fmt.Sprintf("%s.%s", player.Game_id, player.User_id), "hasRolled", "false", conn)
	// add color
	//cache.Set(fmt.Sprintf("%s.%s.cards", player.Game_id, player.User_id), str, conn) cards is an empty array
}

func GetNextTurn(game_id string, user_id string, conn *redis.Conn) string {
	res, err := cache.LGET(fmt.Sprintf("%s.order", game_id), conn)
	if err != nil {
		panic(err)
	}

	for idx, id := range res {
		if user_id == string(id.([]byte)) {
			if idx == (len(res) - 1) {
				val, err := cache.LINDEX(fmt.Sprintf("%s.order", game_id), 0, conn)
				if err != nil {
					fmt.Println("Failed getting next turn")
					panic(err)
				}
				cache.Set(game_id, val, conn)
				return val.(string)
			} else {
				val, err := cache.LINDEX(fmt.Sprintf("%s.order", game_id), idx+1, conn)
				if err != nil {
					fmt.Println("Failed getting next turn")
					panic(err)
				}
				cache.Set(game_id, val, conn)
				return val.(string)
			}
		}
	}
	return ""
}

func RollDice(game_id string, user_id string, Board *[]models.Property, conn *redis.Conn, server *socketio.Server) {
	rand.Seed(time.Now().UnixNano())
	dice1 := rand.Intn(7) + 1
	dice2 := rand.Intn(7) + 1

	curPosS, err := cache.HGET(fmt.Sprintf("%s.%s", game_id, user_id), "pos", conn)
	if err != nil {
		panic(err)
	}
	curPos, _ := strconv.Atoi(curPosS)

	nPos := (dice1 + dice2 + curPos)
	if nPos >= 40 {
		// add 200 since passed goal
		// alert
		newBalance, err := cache.HINCRBY(fmt.Sprintf("%s.%s", game_id, user_id), "bal", 200, conn)
		if err != nil {
			panic(err)
		}
		server.BroadcastToRoom("/", game_id, "passed-go", fmt.Sprintf("%s.%d", user_id, newBalance))
		nPos -= 40
	}

	err = cache.HSET(fmt.Sprintf("%s.%s", game_id, user_id), "pos", nPos, conn)
	if err != nil {
		panic(err)
	}
	if dice1 != dice2 {
		cache.HSET(fmt.Sprintf("%s.%s", game_id, user_id), "hasRolled", "true", conn)
	}

	server.BroadcastToRoom("/", game_id, "dice-roll", fmt.Sprintf("%d.%d.%d", dice1, dice2, nPos))

	val, err := board.GetByPos(nPos, Board)
	if err == nil {
		id := CheckWhoOwns(game_id, val.Id, conn)
		if id == "" {
			// send buy request
			encoded, _ := json.Marshal(&val)
			server.BroadcastToRoom("/", game_id, "buy-request", string(encoded))
		} else {
			// pay rent
			// TODO check for 0
			/*
				bal, err := cache.HGET(fmt.Sprintf("%s.%s", game_id, user_id), "bal", conn)
				if err != nil {
					panic(err)
				}
				nBal, _ := strconv.Atoi(bal)
				cache.HSET(fmt.Sprintf("%s.%s", game_id, user_id), "bal", (nBal - val.Rent), conn)
			*/
			nBal, err := cache.HINCRBY(fmt.Sprintf("%s.%s", game_id, user_id), "bal", (-1 * val.Rent), conn)
			if err != nil {
				panic(err)
			}
			nBal2, err := cache.HINCRBY(fmt.Sprintf("%s.%s", game_id, id), "bal", val.Rent, conn)
			if err != nil {
				panic(err)
			}
			server.BroadcastToRoom("/", game_id, "payed-rent", fmt.Sprintf("%s.%s.%d.%d", user_id, id, nBal, nBal2))
		}
	}
}

func BuyProperty(game_id string, user_id string, conn *redis.Conn, Board *[]models.Property, server *socketio.Server) {
	// get pos
	val, err := cache.HGET(fmt.Sprintf("%s.%s", game_id, user_id), "pos", conn)
	if err != nil {
		panic(err)
	}
	pos, _ := strconv.Atoi(val)
	// get card at pos
	property, err := board.GetByPos(pos, Board)
	if err != nil {
		panic(err)
	}
	// check if is owned by someone
	id := CheckWhoOwns(game_id, property.Id, conn)
	if id != "" {
		return
	}
	// check if enough bal to buy
	val, err = cache.HGET(fmt.Sprintf("%s.%s", game_id, user_id), "bal", conn)
	if err != nil {
		panic(err)
	}
	bal, _ := strconv.Atoi(val)
	if bal < property.Price {
		return
	}
	// sub bal
	err = cache.HSET(fmt.Sprintf("%s.%s", game_id, user_id), "bal", (bal - property.Price), conn)
	if err != nil {
		panic(err)
	}
	// add card to user hash map
	cache.HSET(fmt.Sprintf("%s.%s.cards", game_id, user_id), property.Id, 1, conn) // for now set to 1
	// broadcast
	server.BroadcastToRoom("/", game_id, "property-bought", fmt.Sprintf("%s.%d", user_id, (bal-property.Price)))
}

func StartGame(game_id string, conn *redis.Conn) bool {
	db := database.PostgreSQLConnection()
	var players []models.Player

	err := db.Model(&players).Where("game_id = ?", game_id).Select()

	if err != nil || len(players) <= 1 {
		return false
	}

	cache.Set(game_id, players[0].User_id, conn)
	var ids []interface{}
	for _, player := range players {
		createRedisPlayer(game_id, player, conn)
		//cache.Set(fmt.Sprintf("%s.%d", game_id, idx), player.User_id, conn)
		ids = append(ids, player.User_id)
	}
	cache.RPUSH(fmt.Sprintf("%s.order", game_id), ids, conn)
	fmt.Println("Failed creating array")
	game := &models.Game{
		Id: game_id,
	}
	_, err = db.Model(game).WherePK().Set("status = ?", "in progress").Update()
	if err != nil {
		panic(err)
	}

	return true
}
