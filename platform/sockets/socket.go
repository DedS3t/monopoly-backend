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

	board := board.LoadProperties()

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
				s.Emit("failed")
				return
			}
			user_id, ok := result["user_id"]
			if !ok {
				s.Emit("failed")
				return
			}

			user, err := queries.GetUserData(user_id, db)
			if err != nil {
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
				s.Emit("failed")
				return
			}

			server.BroadcastToRoom("/", id, "player-join")
			s.Join(id)
			players := 0
			server.ForEach("/", id, func(s socketio.Conn) {
				players += 1
			})

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
		// TODO check for double
		// TODO add save state
		/* Set go timeout for 3 mins before deletion of player data.
		If player decides to within 3 mins he can
		Have new event join-back
		*/
		conn := pool.Get()
		defer conn.Close()
		if result := queries.StartGame(game_id, &conn); result != nil {
			userJson, err := json.Marshal(result)
			if err != nil {
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
				queries.RollDice(result["game_id"], result["user_id"], &board, &conn, server, db)
			}
		}
	})

	server.OnEvent("/", "request-buy", func(s socketio.Conn, jsonStr string) {
		conn := pool.Get()
		defer conn.Close()
		var result map[string]string
		json.Unmarshal([]byte(jsonStr), &result)

		if queries.IsUserTurn(result["game_id"], result["user_id"], &conn) {
			queries.BuyProperty(result["game_id"], result["user_id"], &conn, &board, server)
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
			}
		}
		// ELSE End turn from wrong user

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
