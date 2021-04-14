package socket

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/DedS3t/monopoly-backend/app/models"
	"github.com/DedS3t/monopoly-backend/platform/cache"
	"github.com/DedS3t/monopoly-backend/platform/database"
	"github.com/DedS3t/monopoly-backend/platform/queries"
	socketio "github.com/googollee/go-socket.io"
	"github.com/rs/cors"
)

func CreateSocketIOServer() {

	server, err := socketio.NewServer(nil)
	db := database.PostgreSQLConnection()
	pool := cache.CreateRedisPool()

	if err != nil {
		panic(err)
	}

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
			err = queries.CreatePlayer(models.Player{
				Game_id: id,
				User_id: user_id,
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
		go queries.DeletePlayer(result["user_id"], result["game_id"], db)
		server.BroadcastToRoom("/", result["game_id"], "player-left")
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
