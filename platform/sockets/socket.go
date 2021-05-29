package socket

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/DedS3t/monopoly-backend/app/models"
	"github.com/DedS3t/monopoly-backend/platform/board"
	"github.com/DedS3t/monopoly-backend/platform/cache"
	"github.com/DedS3t/monopoly-backend/platform/database"
	"github.com/DedS3t/monopoly-backend/platform/queries"
	socketio "github.com/googollee/go-socket.io"
	"github.com/rs/cors"
)

// TODO add chat

func CreateSocketIOServer() {

	server, err := socketio.NewServer(nil)
	if err != nil {
		panic(err)
	}
	db := database.PostgreSQLConnection()
	defer db.Close()

	pool := cache.CreateRedisPool()
	defer pool.Close()

	Board := board.LoadProperties()

	server.OnConnect("/", func(s socketio.Conn) error {
		s.SetContext("")
		return nil
	})

	server.OnEvent("/", "see", func(s socketio.Conn) {
		fmt.Println("pinged")
	})

	server.OnEvent("/", "join-game", func(s socketio.Conn, jsonStr string) {

		var result map[string]string

		json.Unmarshal([]byte(jsonStr), &result)
		if id, ok := result["game_id"]; ok {
			if !queries.VerifyGame(id, db) {
				s.Emit("error-message", "Invalid game")
				s.Emit("failed")
				return
			}
			user_id, ok := result["user_id"]
			if !ok {
				s.Emit("error-message", "User not authenticated")
				s.Emit("failed")
				return
			}

			user, err := queries.GetUserData(user_id, db)
			if err != nil {
				s.Emit("error-message", "User retrieval failed")
				s.Emit("failed")
				panic(err)
			}
			err = queries.CreatePlayer(models.Player{
				Game_id:  id,
				User_id:  user_id,
				Username: user.Email,
			}, db)

			if err != nil {
				fmt.Println(err)
				s.Emit("error-message", "Failed creating player")
				s.Emit("failed")
				return
			}

			server.BroadcastToRoom("/", id, "player-join")
			s.Join(id)
			players := server.RoomLen("/", id)

			s.Emit("joined-game", strconv.Itoa(players))
			fmt.Printf("%s joined room %s", s.ID(), id)
		} else {
			fmt.Println("Game_id not passed")
		}
	})

	server.OnEvent("/", "leave-game", func(s socketio.Conn, jsonStr string) {
		var result map[string]string
		json.Unmarshal([]byte(jsonStr), &result)

		s.Leave(result["game_id"])
		go queries.DeletePlayer(result["user_id"], result["game_id"], db, server)
		server.BroadcastToRoom("/", result["game_id"], "player-left")
	})

	server.OnEvent("/", "start-game", func(s socketio.Conn, game_id string) {
		/* Set go timeout for 3 mins before deletion of player data.
		If player decides to within 3 mins he can
		Have new event join-back
		*/
		conn := pool.Get()
		defer conn.Close()
		if result := queries.StartGame(game_id, &conn); result != nil {
			userJson, err := json.Marshal(result)
			if err != nil {
				fmt.Println("Here")
				panic(err)
			}
			server.BroadcastToRoom("/", game_id, "game-start", string(userJson))
			time.Sleep(100 * time.Millisecond)
			val, err := cache.Get(game_id, &conn)
			if err != nil {
				panic(err)

			}

			server.BroadcastToRoom("/", game_id, "change-turn", val)
		} else {
			// failed to start game
			s.Emit("error-message", "Unable to start game")
			fmt.Println("Failed to start game")
		}
	})

	server.OnEvent("/", "roll-dice", func(s socketio.Conn, jsonStr string) {
		conn := pool.Get()
		defer conn.Close()
		var result map[string]string
		json.Unmarshal([]byte(jsonStr), &result)

		if queries.IsUserTurn(result["game_id"], result["user_id"], &conn) {
			// check if has rolled dice
			if !queries.HasRolledDice(result["game_id"], result["user_id"], &conn) {
				queries.RollDice(result["game_id"], result["user_id"], &Board, &conn, server, db)
			} else {
				s.Emit("error-message", "You have already rolled the dice")
			}
		} else {
			s.Emit("error-message", "Not your turn")
		}
	})

	server.OnEvent("/", "request-buy", func(s socketio.Conn, jsonStr string) {
		conn := pool.Get()
		defer conn.Close()
		var result map[string]string
		json.Unmarshal([]byte(jsonStr), &result)

		if queries.IsUserTurn(result["game_id"], result["user_id"], &conn) {
			if result := queries.BuyProperty(result["game_id"], result["user_id"], &conn, &Board, server); result != "" {
				s.Emit("error-message", result)
			}
		} else {
			s.Emit("error-message", "Not your turn")
		}

	})

	server.OnEvent("/", "pay-out-jail", func(s socketio.Conn, jsonStr string) {
		conn := pool.Get()
		defer conn.Close()
		var result map[string]string
		json.Unmarshal([]byte(jsonStr), &result)

		if queries.IsUserTurn(result["game_id"], result["user_id"], &conn) && !queries.HasRolledDice(result["game_id"], result["user_id"], &conn) {
			if result := queries.PayOutOfJail(result["game_id"], result["user_id"], &conn, db, server); result != "" {
				s.Emit("error-message", result)
			}
		} else {
			s.Emit("error-message", "To pay out of jail you must not have thrown the dice and it must be your turn ")
		}
	})

	server.OnEvent("/", "buy-house", func(s socketio.Conn, jsonStr string) {
		conn := pool.Get()
		defer conn.Close()
		var result map[string]string
		json.Unmarshal([]byte(jsonStr), &result)
		card_pos, err := strconv.Atoi(result["card_pos"])
		if err != nil {
			panic(err)
		}
		if queries.IsUserTurn(result["game_id"], result["user_id"], &conn) && queries.CheckWhoOwns(result["game_id"], card_pos, &conn) == result["user_id"] {
			property, err := board.GetByPos(card_pos, &Board)
			if err != nil {
				panic(err)
			}

			if result := queries.BuildHouse(result["game_id"], result["user_id"], property, &Board, &conn, server); result != "" {
				s.Emit("error-message", result)
			}
		} else {
			s.Emit("error-message", "It must be your turn and your property to perform this action")
		}

	})

	server.OnEvent("/", "end-turn", func(s socketio.Conn, jsonStr string) {
		conn := pool.Get()
		defer conn.Close()
		var result map[string]string
		json.Unmarshal([]byte(jsonStr), &result)

		if queries.IsUserTurn(result["game_id"], result["user_id"], &conn) {
			// check if has rolled dice
			if queries.HasRolledDice(result["game_id"], result["user_id"], &conn) {
				new_id := queries.GetNextTurn(result["game_id"], result["user_id"], &conn)
				server.BroadcastToRoom("/", result["game_id"], "change-turn", new_id)
				queries.ResetRolledDice(result["game_id"], result["user_id"], &conn)
			} else {
				s.Emit("error-message", "You must roll the die first!")
			}
		} else {
			s.Emit("error-message", "Not your turn")
		}

	})

	server.OnError("/", func(s socketio.Conn, e error) {
		fmt.Println("meet error:", e)
	})

	server.OnDisconnect("/", func(s socketio.Conn, reason string) {
		// shoudlnt be called
		rooms := s.Rooms()
		for _, room := range rooms {
			server.BroadcastToRoom("/", room, "player-left")
		}
		s.LeaveAll()
	})

	go server.Serve()
	defer server.Close()

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowCredentials: true,
	})

	mux := http.NewServeMux()
	mux.Handle("/socket.io/", server)
	http.ListenAndServe(":8000", c.Handler(mux))
}
