package socket

import (
	"fmt"
	"net/http"

	socketio "github.com/googollee/go-socket.io"
	"github.com/rs/cors"
)

func CreateSocketIOServer() {

	server, err := socketio.NewServer(nil)
	if err != nil {
		panic(err)
	}

	server.OnConnect("/", func(s socketio.Conn) error {
		s.SetContext("")
		fmt.Println("connected:", s.ID())
		return nil
	})

	server.OnEvent("/", "ping", func(s socketio.Conn) {
		fmt.Println("Received ping")
	})

	server.OnError("/", func(s socketio.Conn, e error) {
		fmt.Println("meet error:", e)
	})

	server.OnDisconnect("/", func(s socketio.Conn, reason string) {
		fmt.Println("closed", reason)
	})

	go server.Serve()
	defer server.Close()

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowCredentials: true,
	})

	mux := http.NewServeMux()
	mux.Handle("/socket.io/", server)
	http.ListenAndServe(":3334", c.Handler(mux))
}
