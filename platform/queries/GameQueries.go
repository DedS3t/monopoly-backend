package queries

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"time"

	"github.com/DedS3t/monopoly-backend/app/models"
	"github.com/DedS3t/monopoly-backend/platform/board"
	"github.com/DedS3t/monopoly-backend/platform/cache"
	"github.com/DedS3t/monopoly-backend/platform/database"
	"github.com/DedS3t/monopoly-backend/platform/logging"
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

func PlayerExists(user_id string, game_id string, db *pg.DB) bool {
	player := &models.Player{}
	err := db.Model(player).Where("user_id = ? and game_id = ?", user_id, game_id).Select()
	if err != nil {
		return false
	} else {
		return true
	}
}

func HandlePossibleRejoin(user_id string, game_id string, db *pg.DB, conn *redis.Conn, s *socketio.Conn) {
	player := &models.Player{}
	err := db.Model(player).Where("user_id = ? and game_id = ?", user_id, game_id).Select()
	if err != nil {
		logging.Error(err.Error())
		return
	}
	if player.Active == "false" {
		// rejoin
		_, err = db.Model(player).Where("user_id = ? and game_id = ?", user_id, game_id).Set("active = true").Update()
		if err != nil {
			logging.Error(err.Error())
			return
		}

		data := make(map[string]interface{})

		turn, _ := cache.Get(game_id, conn) // current user turn

		data["turn"] = turn

		players := make(map[string]interface{})

		res, _ := cache.LGET(fmt.Sprintf("%s.order", game_id), conn)

		for _, id := range res {

			player := make(map[string]interface{})
			/*
				Pos
				Balance
				Jailed
				Properties
				Color
				Username
			*/

			pos, _ := cache.HGET(fmt.Sprintf("%s.%s", game_id, id), "pos", conn)
			bal, _ := cache.HGET(fmt.Sprintf("%s.%s", game_id, id), "bal", conn)
			jailedStr, _ := cache.HGET(fmt.Sprintf("%s.%s", game_id, id), "jailed", conn)
			jailed, _ := strconv.Atoi(jailedStr)
			color, _ := cache.HGET(fmt.Sprintf("%s.%s", game_id, id), "color", conn)
			username, _ := cache.HGET(fmt.Sprintf("%s.%s", game_id, id), "username", conn)

			propertiesRaw, _ := cache.HGETALL(fmt.Sprintf("%s.%s.cards", game_id, id), conn)

			properties := make([]map[string]interface{}, int(math.Ceil(float64(len(propertiesRaw)/2))))
			for _, prop := range propertiesRaw {
				var propState models.PropertyState
				json.Unmarshal([]byte(prop.(string)), &propState)
				temp := make(map[string]interface{})
				temp["Name"] = propState.Name
				temp["Houses"] = propState.Houses
				temp["Mortgaged"] = propState.Mortgaged
				properties = append(properties, temp)
			}

			player["Username"] = username
			player["Balance"] = bal
			player["jailed"] = (jailed != 0)
			player["Pos"] = pos
			player["Color"] = color
			player["Properties"] = properties
			players[string(id.([]byte))] = player
		}

		data["data"] = players

		body, err := json.Marshal(data)
		if err != nil {
			logging.Error(err.Error())
			return
		}

		logging.Info(string(body))

		(*s).Emit("rejoined", string(body))

	}
}

// TODO check conccurency

func CreatePlayer(player models.Player, db *pg.DB) error {
	_, err := db.Model(&player).Insert()
	return err
}

func GetUserData(user_id string, db *pg.DB) (*models.User, error) {
	user := &models.User{
		Id: user_id,
	}
	err := db.Model(user).WherePK().Select()
	if err != nil {
		return user, err
	}
	return user, nil
}

func DeletePlayerTemp(user_id string, game string, db *pg.DB, server *socketio.Server) {
	playerO := &models.Player{}

	_, err := db.Model(playerO).Where("user_id = ? and game_id = ?", user_id, game).Set("active = ?", "false").Update()
	if err != nil {
		panic(err) // TODO change to logging
	}

	server.BroadcastToRoom("/", game, "temp-leave")

	time.Sleep(time.Minute * 1)

	err = db.Model(playerO).Where("user_id = ? and game_id = ?", user_id, game).Select()

	if err != nil {
		logging.Error(fmt.Sprintf("Error on retrieving player status: %s", err.Error()))
	}

	if playerO.Active != "true" {
		DeletePlayer(user_id, game, db, server, true)
	}
}

func DeletePlayer(user_id string, game string, db *pg.DB, server *socketio.Server, left bool) error {
	if left {
		server.BroadcastToRoom("/", game, "player-left")
	}

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
		if length == 10 { // TODO fix this
			// someone won
			winner, err := cache.LINDEX(fmt.Sprintf("%s.order", game), 0, &conn)
			if err != nil {
				panic(err)
			}
			server.BroadcastToRoom("/", game, "game-done", winner.(string))
		} else {
			server.BroadcastToRoom("/", game, "game-over")
		}

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

func createRedisPlayer(game_id string, player models.Player, color string, conn *redis.Conn) {
	cache.HSET(fmt.Sprintf("%s.%s", player.Game_id, player.User_id), "bal", 1500, conn)
	cache.HSET(fmt.Sprintf("%s.%s", player.Game_id, player.User_id), "pos", 0, conn)
	cache.HSET(fmt.Sprintf("%s.%s", player.Game_id, player.User_id), "hasRolled", "false", conn)
	cache.HSET(fmt.Sprintf("%s.%s", player.Game_id, player.User_id), "jailed", 0, conn)
	cache.HSET(fmt.Sprintf("%s.%s", player.Game_id, player.User_id), "color", color, conn)
	cache.HSET(fmt.Sprintf("%s.%s", player.Game_id, player.User_id), "username", player.Username, conn)
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

func RollDice(game_id string, user_id string, Board *map[string]models.Property, conn *redis.Conn, server *socketio.Server, db *pg.DB) {
	rand.Seed(time.Now().UnixNano())
	dice1 := rand.Intn(7) + 1
	dice2 := rand.Intn(7) + 1
	// extra cheaks
	if dice1 > 6 {
		dice1 = 6
	}
	if dice2 > 6 {
		dice2 = 6
	}
	// check if user is in jail
	if isJailed, jailVal := Jailed(game_id, user_id, conn); isJailed {
		if dice1 != dice2 {
			if jailVal >= 4 {
				// last throw
				// pay 50 and then move
				can, _ := CanAfford(game_id, user_id, 50, Board, server, conn, true)
				if !can {
					server.BroadcastToRoom("/", game_id, "bankrupt", can)
					DeletePlayer(user_id, game_id, db, server, false)
					return
				}
				newBal, _ := cache.HINCRBY(fmt.Sprintf("%s.%s", game_id, user_id), "bal", -50, conn)
				cache.HSET(fmt.Sprintf("%s.%s", game_id, user_id), "jailed", 0, conn)
				dto := make(map[string]interface{})
				dto["User"] = user_id
				dto["Balance"] = newBal
				dto["Info"] = "Payed $50 to get out of jail"
				jsonResult, err := json.Marshal(dto)
				if err != nil {
					panic(err)
				}
				server.BroadcastToRoom("/", game_id, "free-jail", user_id)
				server.BroadcastToRoom("/", game_id, "payment", string(jsonResult))
			} else {
				// increment jailVal by 1
				cache.HINCRBY(fmt.Sprintf("%s.%s", game_id, user_id), "jailed", 1, conn)
				cache.HSET(fmt.Sprintf("%s.%s", game_id, user_id), "hasRolled", "true", conn)
				server.BroadcastToRoom("/", game_id, "dice-roll", fmt.Sprintf("%d.%d.%d", dice1, dice2, 10))
				return
			}
		} else {
			cache.HSET(fmt.Sprintf("%s.%s", game_id, user_id), "jailed", 0, conn)
			server.BroadcastToRoom("/", game_id, "free-jail", user_id)
		}
	}

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

	HandleMove(nPos, game_id, user_id, conn, Board, (dice1 + dice2), server, db)
}

func PayOutOfJail(game_id string, user_id string, b *map[string]models.Property, conn *redis.Conn, db *pg.DB, server *socketio.Server) string {
	if isJailed, _ := Jailed(game_id, user_id, conn); isJailed {
		can, _ := CanAfford(game_id, user_id, 50, b, server, conn, false)
		if !can {
			return "You can't afford this action"
		}
		newBal, _ := cache.HINCRBY(fmt.Sprintf("%s.%s", game_id, user_id), "bal", -50, conn)
		cache.HSET(fmt.Sprintf("%s.%s", game_id, user_id), "jailed", 0, conn)
		dto := make(map[string]interface{})
		dto["User"] = user_id
		dto["Balance"] = newBal
		dto["Info"] = "Payed $50 to get out of jail"
		jsonResult, err := json.Marshal(dto)
		if err != nil {
			panic(err)
		}
		server.BroadcastToRoom("/", game_id, "free-jail", user_id)
		server.BroadcastToRoom("/", game_id, "payment", string(jsonResult))
	}
	return ""
}

func BuyProperty(game_id string, user_id string, conn *redis.Conn, Board *map[string]models.Property, server *socketio.Server) string {
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
	id := CheckWhoOwns(game_id, property.Posistion, conn)
	if id != "" {
		return "Someone owns this property"
	}
	// check if enough bal to buy
	can, bal := CanAfford(game_id, user_id, property.Price, Board, server, conn, false)
	if !can {
		return "You can't afford this property!"
	}

	// sub bal
	err = cache.HSET(fmt.Sprintf("%s.%s", game_id, user_id), "bal", (bal - property.Price), conn)
	if err != nil {
		panic(err)
	}
	// add card to user hash map
	jsonProperty, err := json.Marshal(models.PropertyState{
		Name:      property.Name,
		Houses:    0,
		HouseCost: property.HouseCost,
		Mortgaged: false,
		Posistion: property.Posistion,
	})
	if err != nil {
		panic(err)
	}
	cache.HSET(fmt.Sprintf("%s.%s.cards", game_id, user_id), strconv.Itoa(property.Posistion), string(jsonProperty), conn) // set to card
	// broadcast
	server.BroadcastToRoom("/", game_id, "property-bought", fmt.Sprintf("%s.%d.%s", user_id, (bal-property.Price), property.Name))

	return ""
}

func Mortgage(game_id string, user_id string, pos int, Board *map[string]models.Property, conn *redis.Conn, server *socketio.Server) string {

	val, err := cache.HGET(fmt.Sprintf("%s.%s.cards", game_id, user_id), strconv.Itoa(pos), conn)
	if err != nil {
		// user doesnt own property
		return "Property not owned"
	}

	var propState models.PropertyState
	json.Unmarshal([]byte(val), &propState)

	if !propState.Mortgaged {
		propState.Mortgaged = true
		data, _ := json.Marshal(propState)
		err = cache.HSET(fmt.Sprintf("%s.%s.cards", game_id, user_id), strconv.Itoa(pos), data, conn)
		if err != nil {
			return "Failed in retrieval of property"
		}

		returnData := make(map[string]interface{})
		returnData["update"] = string(data)
		returnData["user"] = user_id

		returnDataString, _ := json.Marshal(returnData)

		server.BroadcastToRoom("/", game_id, "mortgage", returnDataString)
		return ""
	} else {
		return "Property already mortgaged"
	}

}

func BuyBack(game_id string, user_id string, pos int, Board *map[string]models.Property, conn *redis.Conn, server *socketio.Server) string {
	val, err := cache.HGET(fmt.Sprintf("%s.%s.cards", game_id, user_id), strconv.Itoa(pos), conn)
	if err != nil {
		// user doesnt own property
		return "Property not owned"
	}

	var propState models.PropertyState
	json.Unmarshal([]byte(val), &propState)

	if propState.Mortgaged {
		propVal, err := board.GetByPos(pos, Board)
		if err != nil {
			panic(err)
		}
		if can, _ := CanAfford(game_id, user_id, propVal.Price, Board, server, conn, false); can {
			_, err := cache.HINCRBY(fmt.Sprintf("%s.%s", game_id, user_id), "bal", -1*propVal.Price, conn)
			if err != nil {
				return "Failed to update balance"
			}
			propState.Mortgaged = false

			data, _ := json.Marshal(propState)
			err = cache.HSET(fmt.Sprintf("%s.%s.cards", game_id, user_id), strconv.Itoa(pos), data, conn)
			if err != nil {
				return "Failed in retrieval of property"
			}

			returnData := make(map[string]interface{})
			returnData["update"] = string(data)
			returnData["user"] = user_id

			returnDataString, _ := json.Marshal(returnData)

			server.BroadcastToRoom("/", game_id, "bought-back", returnDataString)

			return ""
		} else {
			return "Cant afford"
		}
	} else {
		return "Property not mortgaged"
	}

}

func BuildHouse(game_id string, user_id string, property models.Property, Board *map[string]models.Property, conn *redis.Conn, server *socketio.Server) string {

	if !AllProperties(game_id, user_id, property, Board, conn) {
		// doesnt own all the properties
		return "You need to own all properties (not mortgaged) of the group to build houses"
	}
	if !board.CanBuildHouses(property) {
		// not valid property to build on
		return "You are unable to build on this property"
	}

	// can afford
	if can, bal := CanAfford(game_id, user_id, property.HouseCost, Board, server, conn, false); can {
		// update redis
		res, err := cache.HGET(fmt.Sprintf("%s.%s.cards", game_id, user_id), strconv.Itoa(property.Posistion), conn)
		if err != nil {
			panic(err)
		}
		var propState models.PropertyState
		json.Unmarshal([]byte(res), &propState)

		if propState.Houses < 5 {
			propState.Houses += 1
			jsonProperty, err := json.Marshal(propState)
			if err != nil {
				panic(err)
			}
			err = cache.HSET(fmt.Sprintf("%s.%s.cards", game_id, user_id), strconv.Itoa(property.Posistion), string(jsonProperty), conn)
			if err != nil {
				panic(err)
			}
			// subtract from balance
			err = cache.HSET(fmt.Sprintf("%s.%s", game_id, user_id), "bal", (bal - propState.HouseCost), conn)
			if err != nil {
				panic(err)
			}

			dto := map[string]interface{}{
				"user_id":  user_id,
				"property": property.Name,
				"houses":   propState.Houses,
			}

			jsonDto, err := json.Marshal(dto)
			if err != nil {
				panic(err)
			}

			server.BroadcastToRoom("/", game_id, "bought-house", string(jsonDto))
		}

	} else {
		return "You can't afford this action"
	}
	return ""
}

func StartGame(game_id string, conn *redis.Conn) *map[string]models.PlayerDto {
	db := database.PostgreSQLConnection()
	var players []models.Player
	playersDto := make(map[string]models.PlayerDto)

	err := db.Model(&players).Where("game_id = ?", game_id).Select()

	if err != nil || len(players) <= 1 {
		return nil
	}

	cache.Set(game_id, players[0].User_id, conn)
	var ids []interface{}
	arrColors := []string{"#63b598", "#ce7d78", "#ea9e70", "#a48a9e", "#c6e1e8", "#648177", "#0d5ac1", "#f205e6", "#1c0365", "#14a9ad", "#4ca2f9", "#a4e43f", "#d298e2", "#6119d0", "#d2737d", "#c0a43c", "#f2510e", "#651be6", "#79806e", "#61da5e", "#cd2f00", "#9348af", "#01ac53", "#c5a4fb", "#996635", "#b11573", "#4bb473", "#75d89e"}
	for idx, player := range players {
		createRedisPlayer(game_id, player, arrColors[idx], conn)
		playersDto[player.User_id] = models.PlayerDto{
			Username:   player.Username,
			Balance:    1500,
			Pos:        0,
			Color:      arrColors[idx],
			Properties: make([]interface{}, 0),
			Jail:       false,
		}
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

	return &playersDto
}
