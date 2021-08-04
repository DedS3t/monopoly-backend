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
	"github.com/go-pg/pg/v10"
	"github.com/gomodule/redigo/redis"
	socketio "github.com/googollee/go-socket.io"
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

func PossibleMortgage(game_id string, user_id string, needed int, b *map[string]models.Property, server *socketio.Server, conn *redis.Conn) (bool, int) {
	propertiesR, _ := cache.HGETALL(fmt.Sprintf("%s.%s.cards", game_id, user_id), conn)
	propertiesRaw := cache.ParseHGETALL(propertiesR)
	gain := 0

	for _, tpl := range propertiesRaw {
		if len(tpl) == 2 {
			prop := tpl[1]
			var propState models.PropertyState
			json.Unmarshal([]byte(prop.(string)), &propState)
			propVal, _ := board.GetByPos(propState.Posistion, b)

			if propState.Houses > 0 {

				enough := false
				for propState.Houses >= 0 {
					gain += (propState.HouseCost / 2)
					propState.Houses -= 1
					if gain >= needed {
						enough = true
						break
					}
				}

				if enough {
					data, _ := json.Marshal(propState)
					err := cache.HSET(fmt.Sprintf("%s.%s.cards", game_id, user_id), strconv.Itoa(propVal.Posistion), string(data), conn)

					returnData := make(map[string]interface{})
					returnData["update"] = string(data)
					returnData["user"] = user_id

					returnDataString, _ := json.Marshal(returnData)

					server.BroadcastToRoom("/", game_id, "mortgage", returnDataString)

					if err != nil {
						panic(err)
					}
					i, err := cache.HINCRBY(fmt.Sprintf("%s.%s", game_id, user_id), "bal", gain, conn)
					if err != nil {
						panic(err)
					}

					return true, i
				}
			}

			gain += propVal.Mortgage
			propState.Mortgaged = true
			data, _ := json.Marshal(propState)

			err := cache.HSET(fmt.Sprintf("%s.%s.cards", game_id, user_id), strconv.Itoa(propState.Posistion), string(data), conn)

			returnData := make(map[string]interface{})
			returnData["update"] = string(data)
			returnData["user"] = user_id

			returnDataString, _ := json.Marshal(returnData)

			server.BroadcastToRoom("/", game_id, "mortgage", returnDataString)

			if err != nil {
				panic(err)
			}

			if gain >= needed {
				i, err := cache.HINCRBY(fmt.Sprintf("%s.%s", game_id, user_id), "bal", gain, conn)
				if err != nil {
					panic(err)
				}
				return true, i
			}
		}
	}

	return false, 0

}

func CanAfford(game_id string, user_id string, cost int, b *map[string]models.Property, server *socketio.Server, conn *redis.Conn, auto bool) (bool, int) {
	val, err := cache.HGET(fmt.Sprintf("%s.%s", game_id, user_id), "bal", conn)
	if err != nil {
		return false, 0
	}
	bal, _ := strconv.Atoi(val)
	if bal < cost {
		if auto {
			if can, i := PossibleMortgage(game_id, user_id, cost-bal, b, server, conn); can {
				return true, i
			}
		}
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
			Info:    "jail",
			Action:  "jail",
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

func AllProperties(game_id string, user_id string, val models.Property, Board *map[string]models.Property, conn *redis.Conn) bool {
	for i := 0; i < 40; i++ {
		prop := (*Board)[strconv.Itoa(i)]
		if prop.Group == val.Group && prop.Posistion != val.Posistion {
			val, err := cache.HGET(fmt.Sprintf("%s.%s.cards", game_id, user_id), strconv.Itoa(prop.Posistion), conn)
			if err != nil {
				// if card isnt found
				return false
			}

			var propState models.PropertyState
			json.Unmarshal([]byte(val), &propState)
			if propState.Mortgaged {
				// if property is mortgaged you are unable to do operations
				return false
			}

		}
	}
	return true
}

func CalculateRent(game_id string, user_id string, id string, dice_roll int, val models.Property, Board *map[string]models.Property, conn *redis.Conn) int {
	if val.Group == "railroad" {
		// handle railroads
		railroadPositions := []string{"5", "15", "25", "35"}
		number := 1
		for _, pos := range railroadPositions {
			if pos != strconv.Itoa(val.Posistion) {
				_, err := cache.HGET(fmt.Sprintf("%s.%s.cards", game_id, user_id), pos, conn)
				if err == nil {
					number++
				}
			}
		}
		if number <= 1 {
			return val.Rent
		}

		return val.Mulriplied_Rent[number-2]
	} else if val.Group == "utility" {
		// handle utility cards
		utilityPositions := []string{"12", "28"}
		number := 1
		for _, pos := range utilityPositions {
			if pos != strconv.Itoa(val.Posistion) {
				_, err := cache.HGET(fmt.Sprintf("%s.%s.cards", game_id, user_id), pos, conn)
				if err == nil {
					number++
				}
			}
		}
		if number <= 1 {
			return dice_roll * 4
		}
		return dice_roll * 10
	}
	// check for houseses
	strState, err := cache.HGET(fmt.Sprintf("%s.%s.cards", game_id, id), strconv.Itoa(val.Posistion), conn)
	if err != nil {
		panic(err)
	}
	var propState models.PropertyState
	err = json.Unmarshal([]byte(strState), &propState)
	if err != nil {
		panic(err)
	}

	if propState.Houses > 0 {
		return val.Mulriplied_Rent[propState.Houses-1]
	} else {
		// check for all of a certain group
		for i := 0; i < 40; i++ {
			prop := (*Board)[strconv.Itoa(i)]
			if prop.Group == val.Group && prop.Posistion != val.Posistion {
				_, err := cache.HGET(fmt.Sprintf("%s.%s.cards", game_id, user_id), strconv.Itoa(prop.Posistion), conn)
				if err != nil {
					// if card isnt found
					return val.Rent
				}
			}
		}
		// if all cards are found
		return val.Rent * 2
	}

}

func Jailed(game_id string, user_id string, conn *redis.Conn) (bool, int) {
	res, err := cache.HGET(fmt.Sprintf("%s.%s", game_id, user_id), "jailed", conn)
	if err != nil {
		panic(err) // TODO delete this
		// return false
	}
	intVal, _ := strconv.Atoi(res)
	return intVal != 0, intVal
}

// handle move of
func HandleMove(nPos int, game_id string, user_id string, conn *redis.Conn, Board *map[string]models.Property, dice_roll int, server *socketio.Server, db *pg.DB) {
	if nPos >= 40 || (dice_roll == -1 && nPos == 0) {
		// add 200 since passed goal
		// alert
		newBalance, err := cache.HINCRBY(fmt.Sprintf("%s.%s", game_id, user_id), "bal", 200, conn)
		if err != nil {
			panic(err)
		}
		server.BroadcastToRoom("/", game_id, "passed-go", fmt.Sprintf("%s.%d", user_id, newBalance))
		if nPos >= 40 {
			nPos -= 40
		}
	}

	if nPos == 0 {
		return
	}
	val, err := board.GetByPos(nPos, Board)
	if err == nil {
		id := CheckWhoOwns(game_id, val.Posistion, conn)
		if id == "" {
			// send buy request
			if val.Type == "property" {
				encoded, _ := json.Marshal(&val)
				server.BroadcastToRoom("/", game_id, "buy-request", string(encoded))
			} else if specialDto := GetSpecial(nPos, Board); specialDto.Action != "" {
				// handle special
				if specialDto.Action == "change" {
					// update money
					NewBal, err := cache.HINCRBY(fmt.Sprintf("%s.%s", game_id, user_id), "bal", specialDto.Payload, conn)
					if err != nil {
						panic(err)
					}
					dto := make(map[string]interface{})
					dto["Info"] = specialDto.Info
					dto["Action"] = specialDto.Action
					dto["Payload"] = specialDto.Payload
					dto["User"] = user_id
					dto["Balance"] = NewBal
					jsonResult, err := json.Marshal(dto)
					if err != nil {
						panic(err)
					}
					server.BroadcastToRoom("/", game_id, "special", string(jsonResult))
				} else if specialDto.Action == "move" {
					// move
					err = cache.HSET(fmt.Sprintf("%s.%s", game_id, user_id), "pos", specialDto.Payload, conn)
					if err != nil {
						panic(err)
					}
					dto := make(map[string]interface{})
					dto["User"] = user_id
					dto["Info"] = specialDto.Info
					dto["Action"] = specialDto.Action
					dto["Pos"] = specialDto.Payload
					jsonResult, err := json.Marshal(dto)
					if err != nil {
						panic(err)
					}
					server.BroadcastToRoom("/", game_id, "special", string(jsonResult))
					HandleMove(specialDto.Payload, game_id, user_id, conn, Board, -1, server, db)

				} else if specialDto.Action == "jail" {
					/*
						TODO JAIL:
							1. Add to user hashmap "jailed" default 0. Then on a per turn basis - 1 , 2 , 3, 4.
							2. when jailed change pos to 10
							3. Can pay 50 fine to get out of jail.
					*/
					// send to jail
					cache.HSET(fmt.Sprintf("%s.%s", game_id, user_id), "jailed", 1, conn)
					cache.HSET(fmt.Sprintf("%s.%s", game_id, user_id), "pos", 10, conn)
					server.BroadcastToRoom("/", game_id, "jail", user_id)
					return
				}
			}
		} else if id != user_id {
			// pay rent
			// calculate rent
			if dice_roll == -1 {
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
				dice_roll = (dice1 + dice2)
			}
			rent := CalculateRent(game_id, user_id, id, dice_roll, val, Board, conn)
			// check if user can afford
			can, _ := CanAfford(game_id, user_id, rent, Board, server, conn, true)
			if !can {
				server.BroadcastToRoom("/", game_id, "bankrupt", can)
				DeletePlayer(user_id, game_id, db, server, false)
				return
			}
			nBal, err := cache.HINCRBY(fmt.Sprintf("%s.%s", game_id, user_id), "bal", (-1 * rent), conn)
			if err != nil {
				panic(err)
			}
			nBal2, err := cache.HINCRBY(fmt.Sprintf("%s.%s", game_id, id), "bal", rent, conn)
			if err != nil {
				panic(err)
			}
			server.BroadcastToRoom("/", game_id, "payed-rent", fmt.Sprintf("%s.%s.%d.%d.%d", user_id, id, nBal, nBal2, rent))
		}
	}
}
